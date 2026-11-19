# Using GATES in Workflows

ワークフロー利用者に向けて、GATES から受け取ったトークンで別のリポジトリを読むワークフローの書き方を示します。
ワークフローに書き足すのは、`id-token: write` の権限と Action のステップ一つだけです。

## 1. Minimal Workflow

次のワークフローは、許可オーナー `example-owner` のリポジトリ `example-owner/repo-a` を読みます。
`<コミット>` と、`secrets.` と `vars.` の名前は、運用者から受け取ったものにします（§2）。

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

- **`id-token: write`**：Action が OIDC JWT を受け取るために要る。Action を置くジョブにだけ与える
- **`repositories`**：読むリポジトリ。複数ならカンマ区切りで並べる
- **`permissions`**：`contents:read` だけを書ける
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
| インストール対象のリポジトリ | `repositories`。ここにないリポジトリは読めない |

ワークフローを置くリポジトリも、許可オーナーのものでなければなりません。
接続情報をそのリポジトリから参照できるかも、運用者に確かめます。

## 3. Rules to Follow

### 3.1 Action はコミットの SHA で参照する

タグやブランチは、後から別のコミットを指すように変えられるため、改ざんされた Action を取り込むおそれがあります。
参照するのは、運用者がデプロイした GATES と同じリリースのコミットです。
運用者が GATES を更新したら、参照するコミットも運用者が示すものに更新します。
`actions/checkout` のようなほかの Action も、同じくコミットの SHA で参照します。

### 3.2 信頼できないコードは、トークンを受け取るジョブで動かさない

トークンを受け取るジョブの中のコードは、どれもトークンを読めます。
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

**`pull_request_target` と `workflow_run` で起動するジョブは、fork からの Pull Request が起点でもトークンを受け取れます。**
これらのジョブでは、Pull Request の head のコードも、起点のワークフローの成果物も実行しません。
理由は [Security Considerations](security-considerations.md) の §1.2 にあります。

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
`persist-credentials: false` にすると、トークンを Runner のファイルに残しません。
取得の後に Git でリモートと通信しないなら、こちらにします。

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

### 4.4 What the Token Cannot Do

- **書き込みなど `contents:read` 以外の操作**：`403` で拒否される
- **要求しなかったリポジトリの読み取り**：インストール対象のリポジトリでも、要求に含めなかったプライベートリポジトリは `404` になる
- **ほかのジョブでの利用**：ジョブの `outputs` に置いても、GitHub はマスクされた値を渡さない。トークンが要るジョブごとに Action のステップを置く

同じジョブのステップはどのトークンも読めるため、リポジトリごとにトークンを分けても、読める範囲は分かれません。
同じジョブで読むリポジトリは、`repositories` にまとめて要求します。

## 5. When the Action Fails

Action は、`::error::` の注釈で失敗の段階と原因を示します。
注釈から原因と直し方を引く表は、[Troubleshooting](troubleshooting.md) の §1 にあります。
そこで直せない失敗は、Troubleshooting の §2 にある情報を添えて運用者に伝えます。

入力の書式と、注釈の一覧は [Issue Token](../../.github/actions/issue-token/README.md) にあります。

## 6. Self-Hosted Runners

GitHub がホストする `ubuntu-latest` では、準備は要りません。
セルフホストの Runner では、Linux と Docker を用意し、次の宛先への通信を許可します。
Windows と macOS の Runner では、Action は動きません。

- **イメージのビルド**（ジョブのたびに行う）：Docker Hub と `proxy.golang.org`
- **トークンの受け取り**：OIDC JWT の取得先（環境変数 `ACTIONS_ID_TOKEN_REQUEST_URL` の宛先）と、`GATES_AWS_REGION` のリージョンの AWS STS と Lambda の API
- **失効**：`api.github.com`

ジョブが多い Runner では、Docker Hub の取得回数の制限に備えて、Docker Hub の認証かミラーを用意します。

## 7. References

- [GitHub — OpenID Connect reference](https://docs.github.com/en/actions/reference/security/oidc)
- [GitHub — Workflow syntax: permissions](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#permissions)
- [GitHub CLI — gh auth setup-git](https://cli.github.com/manual/gh_auth_setup-git)
- [actions/checkout](https://github.com/actions/checkout)
