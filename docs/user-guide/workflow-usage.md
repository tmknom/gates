# Using GATES in Workflows

GATES（GitHub App Token Exchange System）は、GitHub Actions のワークフローに、指定したリポジトリだけを読める短命なトークンを発行する仕組みです。
本文書は、**ワークフロー利用者**（GATES からトークンを受け取るワークフローを書く人）が、別のリポジトリを読むワークフローを書くための手引きです。

ワークフローに書き足すのは、ジョブの `id-token: write` の権限と、GATES の Docker Action（以下、Action）のステップ一つと、その出力のトークンを同じジョブの後続のステップで使う処理だけです。
トークンは GitHub App の **Installation Access Token**（GitHub が GitHub App に発行する、対象と権限を限ったトークン）で、発行から 1 時間で使えなくなり、ジョブの終了時に Action が失効を試みます。
ワークフローの側に、個人アクセストークンや GitHub App Private Key を置く必要はありません。

GATES の仕組みと現在の制約は [GATES Overview](overview.md) に、Action の入力、出力、失敗の注釈の一覧は [Issue Token](../../.github/actions/issue-token/README.md) にあります。

## 1. What to Prepare Before Writing a Workflow

GATES は、**GATES 運用者**（GATES を AWS にデプロイして運用する人。以下、運用者）が用意します。
ワークフローを書く前に、次のことを運用者に確かめます。

| 確かめること | 理由 |
|---|---|
| ワークフローを置くリポジトリが、**許可オーナー**（GATES がトークンを発行してよいと運用者が設定した GitHub のユーザーアカウントか Organization）のものである | GATES は、許可オーナーのワークフローにだけトークンを発行する |
| **接続情報**（Action が GATES を呼び出すのに使う値）を登録した Secret と Variables の名前と、それらをワークフローを置くリポジトリから参照できること | Action の入力に渡す。本文書の例は、運用者が `GATES_AWS_ACCOUNT_ID`（Secret）、`GATES_ROLE_NAME`、`GATES_FUNCTION_NAME`、`GATES_AWS_REGION`、`GATES_AUDIENCE`（Variables）の名前で登録したものとする |
| 読みたいリポジトリが、GitHub App の **インストール対象**（GitHub App をオーナーにインストールするときに選ぶ、GitHub App がアクセスできるリポジトリ）に入っている | インストール対象にないリポジトリのトークンは発行されない |
| 参照する Action のコミットの SHA | §2 の `uses` に入れる |

Action は、**OIDC JWT**（ワークフローがどのオーナーのどのリポジトリで動いているかを GitHub が署名して示すトークン）で、ワークフローの身元を GATES に示します。
ワークフローを置くリポジトリの OIDC JWT の形式は、**GitHub App とオーナーの管理者**（GitHub App のインストール先と、オーナーの GitHub の設定を管理する人）が設定します。
GATES は、その中の呼び出し元を示す値（`sub`）が、名前に加えて変わらない ID を含む形式のときだけ受け付けます。
新しく作ったリポジトリは既定でこの形式ですが、古いリポジトリは、Organization かリポジトリの設定（REST API では `use_immutable_subject`）で切り替える必要があります（[GitHub Constraints](../spec/github-constraints.md) の §3.2）。
この形式でないリポジトリと、`sub` の形式をカスタマイズして先頭がこの形式でなくなったリポジトリでは、Action は `AssumeRoleWithWebIdentity failed` の注釈で失敗します。

ジョブを動かす Runner は、Linux で Docker を使えるものにします。
GitHub がホストする `ubuntu-latest` はこれを満たし、準備は要りません。
Windows と macOS の Runner では、Action が動きません。

セルフホストの Runner では、Linux と Docker を用意し、Runner から次の宛先への通信を許可します。

- イメージのビルド（Runner がジョブのたびに行う）：Docker Hub と `proxy.golang.org`
- トークンの受け取り：OIDC JWT の取得先（Runner が環境変数 `ACTIONS_ID_TOKEN_REQUEST_URL` で示す宛先）と、`GATES_AWS_REGION` のリージョンの AWS STS と Lambda の API
- 失効：`api.github.com`

Docker Hub は、認証しないイメージの取得の回数を制限します。
ジョブが多いセルフホストの Runner では、Docker Hub の認証かミラーを用意します（[GitHub Actions Design](../design/github-actions.md) の Runner Requirements）。

## 2. Minimal Workflow

次のワークフローは、`example-owner` が許可オーナーで、そのインストール対象の `example-owner/repo-a` を読む例です。
`<コミット>` には、§1 で運用者から受け取ったコミットの SHA を入れます。

```yaml
name: Read another repository
on: workflow_dispatch

permissions: {}

jobs:
  read-repository:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
    steps:
      - name: Issue token
        id: gates
        uses: tmknom/gates/.github/actions/issue-token@<コミット>
        with:
          repositories: example-owner/repo-a
          permissions: contents:read
          role-arn: arn:aws:iam::${{ secrets.GATES_AWS_ACCOUNT_ID }}:role/${{ vars.GATES_ROLE_NAME }}
          function-name: ${{ vars.GATES_FUNCTION_NAME }}
          aws-region: ${{ vars.GATES_AWS_REGION }}
          audience: ${{ vars.GATES_AUDIENCE }}

      - name: Read repository
        env:
          GH_TOKEN: ${{ steps.gates.outputs.token }}
        run: gh api repos/example-owner/repo-a --jq .full_name
```

ワークフローの各部の役割は次のとおりです。

- `permissions` の `id-token: write`：Action が GitHub に OIDC JWT を要求するために要る。ワークフローの先頭の `permissions: {}` でほかの権限をすべて外し、Action を置くジョブにだけこの権限を与えている
- `repositories` と `permissions`：トークンで読むリポジトリと、トークンの権限。書き方は §4 にある
- `role-arn`、`function-name`、`aws-region`、`audience`：接続情報。値を直接書かず、運用者が登録した Secrets と Variables を参照する
- `id: gates`：後続のステップが `steps.gates.outputs.token` でトークンを参照するための名前
- `GH_TOKEN`：GitHub CLI（`gh`）が使うトークンの環境変数。GitHub CLI は、GitHub がホストする Runner に入っている

ワークフローを実行すると、`Read repository` のステップが `example-owner/repo-a` を出力します。
`Issue token` のステップのログには、発行の結果が次の形で出ます。
この結果はトークンを含まず、トークンはログでマスクされます。

```text
Issued the token: request_id=<request_id> expires_at=<有効期限> repositories=example-owner/repo-a permissions=contents:read,metadata:read
```

`metadata:read` は、リポジトリの基本情報を読む権限で、`contents:read` を要求すると GitHub が加えることがあります。
ジョブの最後には、Runner が `Post Issue token` のステップを動かし、Action がトークンの失効を試みます。

Action は、タグやブランチではなく、コミットの SHA で参照します。
タグやブランチは、参照した後に別のコミットを指すように変えられるため、改ざんされた Action を知らないうちに取り込むおそれがあるからです（[Security Considerations](security-considerations.md) の §3）。
参照するのは、運用者がデプロイした GATES と同じリリースのコミットです。
Action と、デプロイした GATES のリリースが異なる組み合わせで動くかは、確かめていないためです。
運用者が GATES を新しいリリースに更新したら、参照するコミットも運用者が示すものに更新します。

## 3. Using the Token

トークンは、GitHub の API と Git の HTTPS での通信に使えます。
この節の例は、どれも §2 の `Issue token` のステップの後に置くステップです。

### 3.1 Calling the API with GitHub CLI

次の例は、`example-owner/repo-a` の `README.md` を、Runner の作業ディレクトリに同じ名前で保存します。

```yaml
      - name: Download a file
        env:
          GH_TOKEN: ${{ steps.gates.outputs.token }}
        run: |
          gh api -H "Accept: application/vnd.github.raw+json" \
            repos/example-owner/repo-a/contents/README.md > README.md
```

### 3.2 Checking Out with actions/checkout

`actions/checkout` で別のプライベートリポジトリを取得するときは、`repository` に読むリポジトリを、`token` にトークンを渡します。
`path` を省くと、ワークスペース（`$GITHUB_WORKSPACE`）そのものに取得するため、先にワークフローのリポジトリを取得していればそれと重なります。
`path` で別のディレクトリを指定します。

```yaml
      - name: Check out repo-a
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          repository: example-owner/repo-a
          token: ${{ steps.gates.outputs.token }}
          path: repo-a
          persist-credentials: false
```

既定の `persist-credentials: true` では、後続のステップの Git がトークンを使えるように、`actions/checkout` がトークンを Runner の一時ディレクトリのファイルに保存し、ジョブの後処理まで残します。
取得の後に Git でリモートと通信しないなら、`false` にしてトークンを残さないようにします。
`actions/checkout` も、Action と同じくコミットの SHA で参照します。

### 3.3 Cloning with Git

Git でリポジトリを取得するときは、GitHub CLI を Git の **資格情報ヘルパー**（Git が HTTPS の通信で資格情報を問い合わせる先）に登録します。
登録の後は、`GH_TOKEN` を設定したステップの Git が、GitHub への HTTPS の通信にそのトークンを使います。

```yaml
      - name: Clone repo-a
        env:
          GH_TOKEN: ${{ steps.gates.outputs.token }}
        run: |
          gh auth setup-git
          git clone https://github.com/example-owner/repo-a.git
```

`gh auth setup-git` が Git の設定に書くのは GitHub CLI を呼び出す設定だけで、トークンは書きません。
`https://x-access-token:<トークン>@github.com/...` のように URL にトークンを含めて取得すると、取得したリポジトリの `.git/config` にリモートの URL としてトークンが残るため、この方法をとりません。

### 3.4 Requesting Several Repositories at Once

複数のリポジトリを読むときは、`repositories` にカンマ区切りで並べ、一つのトークンで読みます。

```yaml
        with:
          repositories: example-owner/repo-a,example-owner/repo-b
```

出力の `repositories` には、トークンで読めるリポジトリが入ります。
名前の大文字小文字は、要求の書き方ではなく GitHub 上の表記に揃います。
リポジトリごとに Action のステップを分けてトークンを別にしても、同じジョブのステップはどのトークンも読めるため（§5）、読める範囲を分けることにはなりません。

### 3.5 What the Token Cannot Do

次の操作は、トークンでできません。
GitHub の API の応答は、GATES の検証環境で確かめたものです。

- **書き込みなど `contents:read` 以外の権限が要る操作**：ファイルやブランチの作成、Pull Request の一覧の取得のような操作は、`403` で拒否される
- **要求しなかったリポジトリの読み取り**：インストール対象のリポジトリでも、要求に含めなかったプライベートリポジトリは `404` になる
- **ほかのジョブでの利用**：Action の出力は、同じジョブの後続のステップでだけ使える。ジョブの `outputs` に置いても、GitHub はマスクされた値を含むとして設定しない。トークンが要るジョブでは、それぞれ Action のステップを置く

## 4. Writing the Request

`repositories` に書けるのは、許可オーナーのリポジトリのうち、インストール対象に入っているものだけです。
一つの GATES の許可オーナーは一つで、ワークフローを置くリポジトリも許可オーナーのものであるため、要求できるのはワークフローを置くリポジトリと同じオーナーのリポジトリだけです。
`permissions` に書けるのは、現在は `contents:read` だけです。

要求したリポジトリのうち一つでも許可されなければ、要求全体が失敗し、許可されたリポジトリだけのトークンも発行されません（[Requirements](../spec/requirements.md) の FR-AUTHZ-006）。
許可オーナーでないオーナーのリポジトリや、`contents:read` 以外の権限を要求すると、GATES は要求を拒否します。
インストール対象にないリポジトリや存在しないリポジトリを要求すると、GitHub が発行を拒否します。
どちらの場合も、Action は §6 の注釈で失敗し、直し方は [Troubleshooting](troubleshooting.md) の §1 にあります。

リストの書式（区切り、空白、大文字小文字の扱い）と、Action が GATES に送る前に止める誤りは、[Issue Token](../../.github/actions/issue-token/README.md) の「入力」にあります。

## 5. Keeping Untrusted Code out of the Token Job

トークンを受け取るジョブの中のコードは、どれもトークンを読めます。
Action は、トークンを後続のステップに渡すため、Runner のファイルに書きます。
後続のステップはそのファイルを読め、`id-token: write` を持つジョブのステップは、自分で GATES からトークンを受け取ることもできます（[Security Considerations](security-considerations.md) の §3）。
そのため、**信頼できないコード**（Pull Request のコードや、ほかのワークフローの成果物など）を動かすジョブと、トークンを受け取るジョブは分けます。

次の例は、Pull Request と既定のブランチへの push の両方で動くワークフローです。
Pull Request のコードは `id-token: write` を持たない `test` のジョブでだけ動かし、トークンを受け取る `publish` のジョブは、既定のブランチへの push のときだけ動かします。

```yaml
name: Test and publish
on:
  pull_request:
  push:
    branches:
      - main

permissions: {}

jobs:
  test:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - run: make test

  publish:
    needs: test
    if: github.event_name == 'push'
    runs-on: ubuntu-latest
    permissions:
      id-token: write
    steps:
      - name: Issue token
        id: gates
        uses: tmknom/gates/.github/actions/issue-token@<コミット>
        with:
          repositories: example-owner/repo-a
          permissions: contents:read
          role-arn: arn:aws:iam::${{ secrets.GATES_AWS_ACCOUNT_ID }}:role/${{ vars.GATES_ROLE_NAME }}
          function-name: ${{ vars.GATES_FUNCTION_NAME }}
          aws-region: ${{ vars.GATES_AWS_REGION }}
          audience: ${{ vars.GATES_AUDIENCE }}

      - name: Read repository
        env:
          GH_TOKEN: ${{ steps.gates.outputs.token }}
        run: gh api repos/example-owner/repo-a --jq .full_name
```

ジョブを分けても、トークンを受け取るジョブが前のジョブの成果物（ビルドした実行ファイルやスクリプトなど）を実行すれば、信頼できないコードを同じジョブで動かすのと同じことになります。
`publish` のジョブは、`test` のジョブの成果物を受け取って実行しません。

`pull_request_target` と `workflow_run` で起動するジョブには、とくに注意が要ります。
どちらも、Pull Request の作者が変えられない既定のブランチのワークフローで動き、宣言した `id-token: write` を持つため、トークンを受け取れます。
GATES は、ワークフローを起動したイベントの種別で要求を拒否しません。
そのため、これらのイベントでトークンを受け取るジョブでは、Pull Request の head のコードも、起点のワークフロー（`workflow_run` のジョブを起動したワークフロー）の成果物も実行しません。
実行すると、そのコードがトークンを得られます。

ワークフローの書き方の規則の一覧（Dependabot のワークフローでの扱いやマスクの扱いを含む）は、[Issue Token](../../.github/actions/issue-token/README.md) の「注意」にあります。

## 6. When the Action Fails

Action が失敗したときは、`::error::` の注釈の先頭の文言で、失敗した段階が分かります。
たとえば `ACTIONS_ID_TOKEN_REQUEST_TOKEN is not set` は、ジョブに `id-token: write` がないことを示します。
文言ごとの段階と見直す箇所は、[Issue Token](../../.github/actions/issue-token/README.md) の「失敗の読み方」にあります。

GATES が要求を拒否したか発行に失敗したときは、注釈が `GATES returned a failure response` で始まり、`error.code`、`error.message`、`request_id` を含みます。
運用者は `request_id` で GATES の **監査記録**（GATES が要求ごとに、呼び出し元、要求、結果を残す記録）を探し、拒否の理由を確かめられます。

注釈から原因と直し方を引く表は、[Troubleshooting](troubleshooting.md) にあります。
ワークフロー利用者が自分で直せる失敗は Troubleshooting の §1 に、直せない失敗を運用者に伝えるときに添える情報（注釈の全文、`request_id`、ワークフロー実行の URL、時刻）は Troubleshooting の §2 にあります。

## 7. References

- [GitHub — OpenID Connect reference](https://docs.github.com/en/actions/reference/security/oidc)
- [GitHub — Workflow syntax: permissions](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#permissions)
- [GitHub CLI — gh auth setup-git](https://cli.github.com/manual/gh_auth_setup-git)
- [actions/checkout](https://github.com/actions/checkout)
