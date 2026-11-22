# Using GATES in Workflows

ワークフロー利用者に向けて、GATES から受け取ったトークンで別のリポジトリを操作するワークフローの書き方を示します。
ワークフローに書き足すのは、`id-token: write` の権限と Action のステップ一つだけです。

## 1. Minimal Workflow

次のワークフローは、許可オーナー `example-owner` のリポジトリ `example-owner/repo-a` を読みます。
`<コミット>` と、`secrets.` と `vars.` の名前は、運用者から受け取ったものにします。

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
        uses: tmknom/gates/actions/issue-token@<コミット>
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

- **`id-token: write`**：Action が OIDC JWT を受け取るために要る。Action を置くジョブにだけ与える
- **`repositories`**：トークンで操作するリポジトリ。複数ならカンマ区切りで並べる
- **`permissions`**：トークンの権限。`権限名:水準` をカンマ区切りで並べる（例：`contents:write,pull_requests:read`）。書ける権限名と水準は [Issue Token](../../actions/issue-token/README.md) の「要求できる権限」にある
- **`role-arn` から `audience`**：接続情報。値を直接書かず、Secrets と Variables を参照する
- **`steps.gates.outputs.token`**：トークン。同じジョブの後続のステップで使う

実行すると、`Read repository` のステップが `example-owner/repo-a` を出力します。
ジョブの最後には `Post Issue token` のステップが動き、トークンを失効させます。

## 2. What to Get from the Operator

ワークフローを書く前に、運用者から次を受け取ります。

| 受け取るもの | 使う場所 |
|---|---|
| 接続情報を登録した Secret と Variables の名前 | `role-arn`、`function-name`、`aws-region`、`audience` |
| Action のコミットの SHA | `uses` の `<コミット>` |
| インストール対象のリポジトリ | `repositories`。ここにないリポジトリは要求できない |
| 使える権限（GitHub App に付与され、発行上限が許す権限） | `permissions`。ここにない権限は要求できない |
| 要求する保護対象と、保護対象ごとに、ワークフローを置くリポジトリ（許可 Caller のリポジトリ）に許された権限の上限 | `repositories` と `permissions` |

ワークフローを置くリポジトリも、許可オーナーのものでなければなりません。
接続情報をそのリポジトリから参照できるかも、運用者に確かめます。
保護対象を要求するワークフローを新しく置くときは、先に運用者に、要求する保護対象ごとに、そのリポジトリを許可 Caller に加え、要る権限を権限の上限に含めてもらいます。
加える前の要求と、権限の上限を超える要求は、ほかのリポジトリと一緒に要求していても、要求全体が `authorization_denied` で失敗します。

## 3. Rules to Follow

### 3.1 Action はコミットの SHA で参照する

タグやブランチは、後から別のコミットを指すように変えられるため、改ざんされた Action を取り込むおそれがあります。
参照するのは、運用者がデプロイした GATES と同じリリースのコミットです。
運用者が GATES を更新したら、参照するコミットも運用者が示すものに更新します。
`actions/checkout` のようなほかの Action も、同じくコミットの SHA で参照します。

### 3.2 信頼できないコードは、トークンを受け取るジョブで動かさない

トークンを受け取るジョブの中のコードは、どれもトークンを読み、トークンの権限で書き込みを含む操作ができます。
Pull Request のコードや、ほかのワークフローの成果物は、`id-token: write` を持たない別のジョブで動かします。

次の例では、Pull Request のコードを `test` のジョブでだけ動かし、トークンを受け取る `publish` のジョブは、既定のブランチへの push のときだけ動かします。
`publish` は、`test` の成果物を受け取って実行しません。

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
        uses: tmknom/gates/actions/issue-token@<コミット>
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

**`pull_request_target` と `workflow_run` で起動するジョブは、fork からの Pull Request が起点でもトークンを受け取れます。**
これらのジョブでは、Pull Request の head のコードも、起点のワークフローの成果物も実行しません。

### 3.3 ジョブに必要な権限だけを要求する

`permissions` には、そのジョブで使う権限だけを、使う水準で書きます。
読むだけなら `read` にし、書き込みが要る権限だけを `write` にします。
トークンが漏れたときや、ジョブの中のコードが改ざんされていたときに、トークンでできる操作を減らすためです。

同じジョブのステップはどのトークンも読めるため、ステップごとにトークンを分けても、ジョブの中でできることは分かれません。
書き込みの要らない処理は、読み取りだけを要求する別のジョブに分けます。
同じジョブで使うリポジトリと権限は、一つの Action のステップにまとめて要求します。

## 4. Using the Token

この節の例は、どれも §1 の `Issue token` のステップの後に置きます。

### 4.1 GitHub CLI

`GH_TOKEN` にトークンを渡すと、GitHub CLI の API の呼び出しに使われます。

```yaml
      - name: Download a file
        env:
          GH_TOKEN: ${{ steps.gates.outputs.token }}
        run: |
          gh api -H "Accept: application/vnd.github.raw+json" \
            repos/example-owner/repo-a/contents/README.md > README.md
```

### 4.2 actions/checkout

`path` で、ワークフローのリポジトリと重ならないディレクトリを指定します。
`persist-credentials: false` にして、トークンを Runner のファイルに残さないようにします。
既定の `true` では、トークンが Runner のファイルに書かれ、後続のステップの Git がどれもトークンを使えるようになります。
push するときも `false` のままにし、次の `gh auth setup-git` を使います。

```yaml
      - name: Check out repo-a
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          repository: example-owner/repo-a
          token: ${{ steps.gates.outputs.token }}
          path: repo-a
          persist-credentials: false
```

### 4.3 Git

`gh auth setup-git` で、Git が GitHub CLI を通してトークンを使うようにします。
`https://x-access-token:<トークン>@github.com/...` のように URL にトークンを含めると、取得したリポジトリの `.git/config` にトークンが残るため、使いません。

```yaml
      - name: Clone repo-a
        env:
          GH_TOKEN: ${{ steps.gates.outputs.token }}
        run: |
          gh auth setup-git
          git clone https://github.com/example-owner/repo-a.git
```

### 4.4 Writing and Multiple Permissions

次の例は、`example-owner/repo-a` の Pull Request を一覧し、新しいブランチを push します。
§1 の `Issue token` のステップの `permissions` を、次の値にします。

```yaml
          permissions: contents:write,pull_requests:read
```

Action の Output の `permissions` は、要求した `contents:write,pull_requests:read` に、GitHub が `metadata:read` を加えた値（`contents:write,metadata:read,pull_requests:read`）になることがあります。

```yaml
      - name: List pull requests
        env:
          GH_TOKEN: ${{ steps.gates.outputs.token }}
        run: gh api repos/example-owner/repo-a/pulls --jq '.[].title'

      - name: Check out repo-a
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          repository: example-owner/repo-a
          token: ${{ steps.gates.outputs.token }}
          path: repo-a
          persist-credentials: false

      - name: Push a branch
        working-directory: repo-a
        env:
          GH_TOKEN: ${{ steps.gates.outputs.token }}
        run: |
          gh auth setup-git
          git config user.name "github-actions[bot]"
          git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
          git switch -c update-example
          date -u > example.txt
          git add example.txt
          git commit -m "Update example.txt"
          git push origin update-example
```

`persist-credentials: false` のまま `gh auth setup-git` を使わずに push すると、Git は資格情報を持たずに失敗します。

### 4.5 What the Token Cannot Do

- **要求しなかった権限の操作**：GitHub App に付与されていても、要求しなかった権限が要る API は `403` で拒否される。`contents:read` だけのトークンでは書き込めない
- **`.github/workflows/` のファイルを変える push**：`workflows:write` を要求しなかったトークンでは、`refusing to allow a GitHub App to create or update workflow` で拒否される
- **要求しなかったリポジトリの操作**：インストール対象のリポジトリでも、要求に含めなかったプライベートリポジトリは `404` になる
- **Organization、Enterprise、ユーザーアカウントの資源の操作**：GATES は Repository permissions だけを発行する
- **ほかのジョブでの利用**：ジョブの `outputs` に置いても、GitHub はマスクされた値を渡さない。トークンが要るジョブごとに Action のステップを置く

## 5. When the Action Fails

Action は、`::error::` の注釈で失敗の段階と原因を示します。
注釈をもとに [Troubleshooting](troubleshooting.md) で直し方を引き、直せなければ運用者に伝えます。
許可 Caller でないリポジトリからの保護対象の要求と、権限の上限を超える要求も、ほかの認可の拒否と同じ `error code "authorization_denied"` の注釈になり、注釈からは原因を見分けられません。
運用者は、注釈の `request_id` から原因を調べられます。

## 6. Self-Hosted Runners

GitHub がホストする `ubuntu-latest` では、準備は要りません。
セルフホストの Runner では、Linux と Docker を用意し、次の宛先への通信を許可します。
Windows と macOS の Runner では、Action は動きません。

- **イメージのビルド**（ジョブのたびに行う）：Docker Hub と `proxy.golang.org`
- **トークンの受け取り**：OIDC JWT の取得先（環境変数 `ACTIONS_ID_TOKEN_REQUEST_URL` の宛先）と、`GATES_AWS_REGION` のリージョンの AWS STS と Lambda の API
- **失効**：`api.github.com`

ジョブが多い Runner では、Docker Hub の取得回数の制限に備えて、Docker Hub の認証かミラーを用意します。

## 7. References

- [Issue Token](../../actions/issue-token/README.md)：Action の入力と出力の書式、注釈の一覧
- [Security Considerations](security-considerations.md)：トークンを受け取るジョブで信頼できないコードを動かさない理由
- [GitHub — OpenID Connect reference](https://docs.github.com/en/actions/reference/security/oidc)
- [GitHub — Workflow syntax: permissions](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#permissions)
- [GitHub CLI — gh auth setup-git](https://cli.github.com/manual/gh_auth_setup-git)
- [actions/checkout](https://github.com/actions/checkout)
