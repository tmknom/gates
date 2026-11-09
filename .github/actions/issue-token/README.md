# Issue Token

GATES から、指定したリポジトリと権限に絞った GitHub App の Installation Access Token を受け取る Docker Action です。
受け取ったトークンは、同じジョブの後続ステップで使えます。
ジョブの終了時には、トークンの失効を試みます。

GATES の目的と全体像は [GATES](../../../README.md) を、Action の設計は [GitHub Actions Design](../../../docs/design/github-actions.md) を参照してください。

## 前提

- ワークフローかジョブの `permissions` で `id-token: write` を宣言する
- GATES の運用者が登録した接続情報（AWS アカウント ID の Secret と、Role 名、関数名、リージョン、Audience の Variables）を参照できる。登録の手順は [Testing Environment Setup](../../../docs/contributing/setup-environment.md#34-connection-information) にある
- Runner が Linux で、Docker を使える。GitHub がホストする `ubuntu-latest` はこれを満たす。セルフホストの Runner に求める通信の許可は [GitHub Actions Design](../../../docs/design/github-actions.md#54-runner-requirements) にある

Runner は、ジョブのたびに Action の `Dockerfile` からイメージをビルドします。

## 入力

すべて必須で、既定値はありません。

| 入力 | 意味 |
|---|---|
| `repositories` | トークンでアクセスするリポジトリ。カンマ区切りの `owner/repo` |
| `permissions` | トークンに付与する権限。カンマ区切りの `permission:level`。値は `contents:read` |
| `role-arn` | 引き受ける IAM Role の ARN |
| `function-name` | 呼び出す GATES の Lambda の関数名 |
| `aws-region` | GATES の Lambda を置く AWS リージョン |
| `audience` | GATES に送る OIDC JWT の Audience |

`role-arn`、`function-name`、`aws-region`、`audience` は、値を直接書かず、運用者が登録した Secrets と Variables を参照します（[利用例](#利用例)）。

`repositories` と `permissions` は、`,` で区切ったリストで書きます。
要素の前後の空白は除かれ、大文字小文字はそのまま GATES に送られます。

```yaml
repositories: example-org/repo-a,example-org/repo-b
permissions: contents:read
```

Action は、次のどれかに当たるリストを GATES に送らずに失敗させ、該当する要素を `::error::` に示します。

- 空の要素がある
- `repositories` の要素が `/` をちょうど 1 つ含まないか、`/` の前後のどちらかが空であるか、空白で始まるか終わる
- `repositories` に、大文字小文字を区別せずに比べて同じ要素が 2 つ以上ある
- `permissions` の要素が `:` をちょうど 1 つ含まないか、`:` の前後のどちらかが空であるか、空白で始まるか終わる
- `permissions` に、同じ権限名の要素が 2 つ以上ある
- 要素が UTF-8 として正しくない

名前の文字種、リポジトリの数の上限、要求が許可されるかは、GATES が判定します。

## 出力

| 出力 | 意味 |
|---|---|
| `token` | 発行された Installation Access Token。ログではマスクされる |
| `expires-at` | GitHub が定めたトークンの有効期限（RFC 3339） |
| `repositories` | トークンでアクセスできるリポジトリ。入力と同じ形式のカンマ区切りの `owner/repo` |
| `permissions` | トークンの実効権限。入力と同じ形式のカンマ区切りの `permission:level`。要求しなくても GitHub が付与する `metadata:read` を含み得る |

## 利用例

`<PINNED_COMMIT>` は、参照する commit SHA に置き換えます。

```yaml
name: Read another repository
on: workflow_dispatch

jobs:
  read-repository:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
    steps:
      - name: Issue token
        id: gates
        uses: tmknom/gates/.github/actions/issue-token@<PINNED_COMMIT>
        with:
          repositories: example-org/repo-a,example-org/repo-b
          permissions: contents:read
          role-arn: arn:aws:iam::${{ secrets.GATES_AWS_ACCOUNT_ID }}:role/${{ vars.GATES_ROLE_NAME }}
          function-name: ${{ vars.GATES_FUNCTION_NAME }}
          aws-region: ${{ vars.GATES_AWS_REGION }}
          audience: ${{ vars.GATES_AUDIENCE }}

      - name: Access repository
        env:
          GH_TOKEN: ${{ steps.gates.outputs.token }}
        run: gh api repos/example-org/repo-a/contents/README.md
```

Action はタグやブランチではなく commit SHA で参照します。
タグやブランチの参照先は、参照した後に別のコミットへ変わり得るためです。

## 失敗の読み方

Action は失敗すると、ステップを失敗で終え、失敗した段階と原因を `::error::` の注釈に書きます。
段階は注釈の先頭の文言で分かります。

| 注釈の先頭 | 段階 |
|---|---|
| `ACTIONS_ID_TOKEN_REQUEST_TOKEN is not set`、`... is not set`、`input ... is empty` | 前提の確認。`permissions` の宣言と、入力の指定を見直す |
| `invalid ACTIONS_ID_TOKEN_REQUEST_URL` | OIDC JWT の要求先の確認 |
| `invalid inputs` | 入力のリストの検証（[入力](#入力)） |
| `could not get the OIDC JWT for AWS STS`、`could not get the OIDC JWT for GATES` | OIDC JWT の取得 |
| `AssumeRoleWithWebIdentity failed` | IAM Role の引き受け。`role-arn` と `aws-region` を見直す |
| `the Lambda Invoke failed`、`the Lambda Invoke returned StatusCode` | Lambda の呼び出し。`function-name` と `aws-region` を見直す |
| `the Lambda function returned FunctionError` | GATES の想定外の失敗。`request_id` がないため、運用者にはワークフローの実行を伝える |
| `GATES returned a failure response` | GATES が要求を拒否したか、発行に失敗した |
| `the Lambda response ...`、`the success response has no usable ...` | GATES の応答を読めなかった |
| `could not write to GITHUB_STATE`、`could not write to GITHUB_OUTPUT` | Output の書き込み |
| `the job was canceled` | ジョブのキャンセル |
| `the child process could not be started`、`the child process terminated unexpectedly`、`an unexpected failure occurred` | Action の想定外の失敗。運用者に知らせる |

`GATES returned a failure response` の注釈には、`error.code`、`error.message`、`request_id` が入ります。
`error.code` の意味と、再実行で解消し得るかは [Lambda Design](../../../docs/design/lambda.md#32-failure) の一覧にあります。
運用者は `request_id` で GATES の監査記録を探せるため、運用者に問い合わせるときは `request_id` を伝えてください。

## トークンの失効

ジョブの終了時に、Runner は `Post <ステップ名>` のステップを動かし、Action はトークンの失効を試みます。
失効に失敗しても、Action は `::warning::` の注釈を書くだけで、ジョブを失敗させません。
失効の処理そのものが想定外に終わった場合も、[失敗の読み方](#失敗の読み方)の想定外の失敗の文言を `::warning::` に書くだけです。
失効できなかったトークンも、`expires-at` の時刻を過ぎれば使えなくなります。

Action がトークンを受け取った後、後続ステップに渡す前に失敗した場合は、その場で失効を試みてから失敗で終えます。

## 注意

- Output は、同じジョブの後続ステップでだけ使える。ジョブの `outputs` に置くと、GitHub はマスクした値を含むとして設定しない
- トークンを受け取るジョブで、信頼できないコード（`pull_request_target` で取得した Pull Request のコードなど）を実行しない。そのジョブは `id-token: write` を持つため、信頼できないコードが GATES からトークンを受け取れ、同じジョブのほかのステップに渡したトークンも読める（[Security Design](../../../docs/design/security.md#5-failure-audit-and-security-risks)）
- Dependabot が起動したワークフローは、GitHub Actions の Secrets を参照できない。`GATES_AWS_ACCOUNT_ID` が空になって `role-arn` が組み立てられないため、トークンの取得は失敗する。同じ名前で Dependabot の Secrets に登録すると、取得できる（[GitHub Constraints](../../../docs/spec/github-constraints.md#7-github-actions-integration)）
- Action がトークンをマスクするため、呼び出し元がもう一度マスクする必要はない。ただし、トークンの一部を切り出してログに出すと、その部分はマスクされない
