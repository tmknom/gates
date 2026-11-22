# Issue Token

GATES から、指定したリポジトリを指定した権限でだけ操作できる GitHub App の Installation Access Token を受け取る Docker Action です。
トークンは同じジョブの後続ステップで使え、ジョブの終了時に失効させます。
GATES の目的と全体像は [GATES](../../README.md) にあります。

## 利用例

`<PINNED_COMMIT>` には、GATES の運用者が示すコミットの SHA を入れます。
`secrets.` と `vars.` の名前も、運用者が登録したものにします。

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
        uses: tmknom/gates/actions/issue-token@<PINNED_COMMIT>
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

## 前提

- ワークフローかジョブの `permissions` で `id-token: write` を宣言する
- GATES の運用者が登録した接続情報（AWS アカウント ID の Secret と、Role 名、関数名、リージョン、Audience の Variables）を参照できる
- Runner が Linux で、Docker を使える。GitHub がホストする `ubuntu-latest` はこれを満たす

セルフホストの Runner では、Docker Hub と `proxy.golang.org`（イメージのビルド）、OIDC JWT の取得先、AWS STS と Lambda の API、`api.github.com` への通信を許可します。

## 入力

すべて必須です。

| 入力 | 意味 |
|---|---|
| `repositories` | トークンで操作するリポジトリ。カンマ区切りの `owner/repo` |
| `permissions` | トークンの権限。カンマ区切りの `権限名:水準`（例：`contents:read,pull_requests:write`）。書ける組は [要求できる権限](#要求できる権限) にある |
| `role-arn` | 引き受ける IAM Role の ARN |
| `function-name` | GATES の Lambda の関数名 |
| `aws-region` | GATES の Lambda の AWS リージョン |
| `audience` | GATES に送る OIDC JWT の Audience |

`role-arn`、`function-name`、`aws-region`、`audience` は、値を直接書かず、運用者が登録した Secrets と Variables を参照します。

`repositories` と `permissions` の要素の前後の空白は除かれ、大文字小文字はそのまま GATES に送られます。
次のリストは、GATES に送る前に失敗します。

- 空の要素がある
- `repositories` の要素が `owner/repo` の形でない
- `permissions` の要素が `権限名:水準` の形でない
- 同じリポジトリ（大文字小文字を区別しない）か、同じ権限名が 2 つ以上ある
- UTF-8 として正しくない

GATES がトークンを発行するのは、要素がどれも次の条件を満たすときだけです。
一つでも満たさない要素があると、要求全体が失敗し、許可される要素だけのトークンも返りません。

- **`repositories`**：運用者が許可したオーナーのリポジトリで、GitHub App のインストール対象に含まれる
- **`permissions`**：[要求できる権限](#要求できる権限) の組で、GitHub App に付与され、発行上限（運用者が GATES のデプロイで決める、発行してよい権限の一覧）が許す。発行上限の高い水準は、同じ権限名の低い水準も許す（`contents:write` を含む発行上限は `contents:read` の要求も許す）。トークンは要求した水準で発行される
- **保護対象を含むとき**：保護対象（運用者が、要求できるワークフローと権限を限ったリポジトリ）を含む要求は、運用者が、ワークフローを置くリポジトリにその保護対象の要求を許していて、要求の権限がどれもそのリポジトリに許した範囲（権限の上限）にあるときだけ発行される。保護対象かどうかと、許された範囲は運用者に確かめる

## 要求できる権限

GitHub App の Repository permissions に当たる、次の権限名と水準の組です。
Organization permissions（`members` など）と Account permissions（`emails` など）は、GitHub App に付与されていても要求できません。

| 水準 | 権限名 |
|---|---|
| `read`、`write` | `actions`、`actions_variables`、`administration`、`agent_secrets`、`agent_tasks`、`agent_variables`、`artifact_metadata`、`attestations`、`checks`、`code_quality`、`codespaces`、`codespaces_lifecycle_admin`、`codespaces_secrets`、`contents`、`copilot_agent_settings`、`dependabot_secrets`、`deployments`、`discussions`、`environments`、`installation_repositories`、`issues`、`license_compliance_alerts`、`merge_queues`、`packages`、`pages`、`pull_requests`、`repo_secret_scanning_dismissal_requests`、`repository_advisories`、`repository_creation`、`repository_custom_properties`、`repository_hooks`、`secret_scanning_alerts`、`secret_scanning_bypass_requests`、`secrets`、`security_events`、`single_file`、`statuses`、`vulnerability_alerts`、`workflows` |
| `read`、`write`、`admin` | `repository_projects` |
| `read` | `codespaces_metadata`、`metadata`、`repository_license_policy` |

- **権限名は表の名前で書く**：GitHub App の設定画面の表示名（`Contents`、`Pull requests`、`Commit statuses` など）ではなく、`contents`、`pull_requests`、`statuses` のように書く。表示名と権限名は、`Commit statuses` と `statuses` のように一致しないことがある
- **水準は付与された水準以下にする**：GitHub App に `Read-only` で付与された権限の `write` は、GitHub に拒否される
- **`single_file` のファイルは選べない**：読み書きできるファイルのパスは GitHub App の設定で決まり、要求では指定できない
- **`metadata` は書かなくてよい**：GitHub が `metadata:read` を加えることがある。明示して要求するときは、ほかの権限と同じく発行上限が許している必要がある
- **機能が無効なリポジトリでは使えない**：リポジトリの設定で Pull requests の機能が無効なら、`pull_requests` の権限を持つトークンでも Pull Request の API は `404` になる

## 出力

| 出力 | 意味 |
|---|---|
| `token` | トークン。ログではマスクされる |
| `expires-at` | トークンの有効期限（RFC 3339）。発行から 1 時間 |
| `repositories` | トークンで操作できるリポジトリ。カンマ区切りの `owner/repo` |
| `permissions` | トークンの権限。カンマ区切りの `権限名:水準`。要求した権限に加えて、要求にかかわらず GitHub が加える `metadata:read` を含むことがある |

出力は、同じジョブの後続ステップでだけ使えます。
ジョブの `outputs` に置いても、GitHub はマスクされた値をほかのジョブに渡しません。

## 注意

- **コミットの SHA で参照する**：タグやブランチは後から別のコミットを指せるため、改ざんされた Action を取り込むおそれがある。参照するのは、運用者がデプロイした GATES と同じリリースのコミットにする
- **トークンを受け取るジョブで、信頼できないコードを実行しない**：そのジョブのコードは、トークンを読んでその権限で書き込みを含む操作ができるうえ、`id-token: write` で、要求しなかった権限を含むトークンも自分で受け取れる。Pull Request のコードや、ほかのワークフローの成果物は、別のジョブで動かす
- **ジョブに必要な権限だけを要求する**：読むだけの権限は `read` にする。書き込みの要らない処理は、読み取りだけを要求する別のジョブに分ける
- **トークンで行った変更は、失効の後も残る**：トークンの期限が短くても、ジョブの終了時に失効しても、書き換えたコードや設定は元に戻らない
- **`pull_request_target` と `workflow_run` に注意する**：fork からの Pull Request が起点でもトークンを受け取れる。これらのジョブでは、Pull Request のコードも起点のワークフローの成果物も実行しない
- **Dependabot のワークフローでは Secret の登録が要る**：Dependabot のワークフローは GitHub Actions の Secrets を参照できないため、運用者が `GATES_AWS_ACCOUNT_ID` を Dependabot の Secrets にも登録する
- **トークンの一部をログに出さない**：Action はトークンをマスクするが、切り出した一部はマスクされない

## 失敗の読み方

Action は失敗すると、ステップを失敗で終え、`::error::` の注釈に失敗した段階と原因を書きます。
段階は注釈の先頭の文言で分かります。

| 注釈の先頭 | 段階と見直す箇所 |
|---|---|
| `ACTIONS_ID_TOKEN_REQUEST_TOKEN is not set`、`... is not set`、`input ... is empty` | 前提の確認。`permissions` の宣言と、入力の指定 |
| `invalid ACTIONS_ID_TOKEN_REQUEST_URL` | OIDC JWT の要求先の確認。Runner の環境 |
| `invalid inputs` | 入力のリストの検証（[入力](#入力)） |
| `could not get the OIDC JWT for AWS STS`、`could not get the OIDC JWT for GATES` | OIDC JWT の取得。`audience` と、Runner の通信 |
| `AssumeRoleWithWebIdentity failed` | IAM Role の引き受け。`role-arn` と `aws-region` |
| `the Lambda Invoke failed`、`the Lambda Invoke returned StatusCode` | Lambda の呼び出し。`function-name` と `aws-region` |
| `the Lambda function returned FunctionError` | GATES の想定外の失敗。運用者に、ワークフロー実行の URL と時刻を伝える |
| `GATES returned a failure response` | GATES が要求を拒否したか、発行に失敗した。`error.code`、`error.message`、`request_id` が入る |
| `the Lambda response ...`、`the success response has no usable ...` | GATES の応答を読めなかった。Action の参照先が、デプロイした GATES と同じリリースかを確かめる |
| `could not write to GITHUB_STATE`、`could not write to GITHUB_OUTPUT` | Output の書き込み。Runner の環境 |
| `the job was canceled` | ジョブのキャンセル |
| `the child process could not be started`、`the child process terminated unexpectedly`、`an unexpected failure occurred` | Action の想定外の失敗。運用者に知らせる |

`GATES returned a failure response` のうち、`error.message` が `Retry later.` で終わるものは一時的な障害で、再実行で解消することがあります。
`error.code` が `authorization_denied` のものは、許されないリポジトリか権限を要求したことを示します。
保護対象を、許されていないリポジトリのワークフローから要求したか、許された範囲を超える権限で要求した場合も同じ注釈になり、注釈からは原因を見分けられません。
運用者は `request_id` で GATES の監査記録を探して原因を調べられるため、問い合わせるときは `request_id` を伝えます。

## トークンの失効

ジョブの終了時に、Runner は `Post <ステップ名>` のステップを動かし、Action はトークンの失効を試みます。
後続のステップが失敗した場合や、ジョブがキャンセルされた場合も試みます。
Action がトークンを受け取った後、後続ステップに渡す前に失敗した場合は、その場で失効を試みます。

失効に失敗しても、Action は `::warning::` の注釈を書くだけで、ジョブを失敗させません。
失効できなかったトークンも、`expires-at` を過ぎれば使えなくなります。
