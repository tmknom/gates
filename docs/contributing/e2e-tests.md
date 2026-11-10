# E2E Tests

デプロイした GATES を Docker Action から呼び、発行、後続ステップでの利用、失効、失敗時の挙動、資格情報の非出力を確かめる E2E テストです。
GitHub と AWS の挙動に依存する事項は、許可オーナーのリポジトリのワークフローでしか確かめられません（[GitHub Actions Design](../design/github-actions.md#62-tests-on-github-actions)）。
E2E テストのワークフローは本リポジトリの `.github/workflows/` に置き、試験用のリポジトリに写して実行します。
本リポジトリには接続情報を登録しないため、ワークフローは `workflow_dispatch` だけで起動するようにしています。

## Workflows

| ファイル | 確かめること |
| --- | --- |
| [_e2e-success.yml](../../.github/workflows/_e2e-success.yml) | 対象 1 件と 2 件の発行、トークンの範囲、Output の形、後続ステップの環境に AWS 一時認証情報がないこと |
| [_e2e-interval.yml](../../.github/workflows/_e2e-interval.yml) | 温まった Lambda の実行環境に、直前の Invoke から 1 分前後空けた Invoke を 5 回続け、すべて成功すること |
| [_e2e-failure.yml](../../.github/workflows/_e2e-failure.yml) | 成功する設定から 1 つの条件だけを変えた各事例で、Action が段階を示す注釈を書いて失敗し、Output を書かず、post-entrypoint が失効を試みずに終わること |
| [_e2e-cancel.yml](../../.github/workflows/_e2e-cancel.yml) | 後続ステップが失敗したジョブと、後続ステップの実行中にキャンセルしたジョブで、post-entrypoint がトークンを失効させること。メインの処理の最中に SIGINT を受けた Action が、受けた時点に応じて終えること |
| [_e2e-reusable-credential-leak.yml](../../.github/workflows/_e2e-reusable-credential-leak.yml) | ほかのワークフローから呼ばれ、同じワークフロー実行の完了したジョブのログ全体に、資格情報の文字列がないこと |

ログの検索は、資格情報が漏れていないことを保証しません。
資格情報を出さない仕組みの本体は Action のマスクと出力の設計で、検索はそれが崩れたときに気づくための確認です。

どのワークフローも、Action を commit SHA で参照します。
Action を変えたときは、各ファイルの `uses:` の SHA を、試す commit に変えます。

Lambda を呼ぶ E2E のワークフローは、どれも同じ `concurrency` のグループに入り、同時に動きません。
間隔の試験は、ほかの Invoke が同じ実行環境に割り込むと間隔が崩れるためです。

### Failure Cases

`_e2e-failure.yml` の各事例が変える条件と、失敗する段階は次のとおりです。
条件は互いに重ならないように選んでいます。

| 事例 | 変える条件 | 失敗する段階 |
| --- | --- | --- |
| `no-id-token` | ジョブの `permissions` から `id-token: write` を外す | 前提の確認 |
| `empty-input` | `audience` を空にする | 前提の確認 |
| `empty-element` | `repositories` に空の要素を置く | 入力の検証 |
| `duplicate` | `repositories` に同じ対象を大文字に変えて 2 回書く | 入力の検証 |
| `syntax` | `permissions` を `contents` にする | 入力の検証 |
| `assume-role` | `role-arn` の Role 名を存在しない名前にする | AWS STS |
| `audience` | `audience` を、GitHub が発行を拒否する github.com の URL にする（[GitHub OIDC Custom Audience](../research/github/custom-audience.md)） | GATES 用 OIDC JWT の取得 |
| `function` | `function-name` を存在しない名前にする | Invoke の SDK の失敗 |
| `authorization` | `permissions` を `contents:write` にする | GATES の `authorization_denied` |
| `installation` | `repositories` に試験用のリポジトリ自身を含める | GATES の `github_issuance_rejected` |

ほかに、`continue-on-error` を付けない Action のステップを入力の検証で失敗させるジョブ（`no-continue-on-error`）を置き、ジョブが失敗しても post-entrypoint が動くことを確かめます。

`FunctionError`、一時的な障害、照合失敗は、実環境で意図して起こす手段がないため、Action のローカルの試験だけで確かめます。

### Cancellation

`_e2e-cancel.yml` は、2 つの方法でシグナルを届けます。

- **後続ステップの実行中のキャンセル**：GitHub は 1 つのジョブだけをキャンセルする手段を持たないため、ほかの試験のジョブが終わり、後続ステップで待つジョブだけが残った時点で、ワークフロー実行をキャンセルする
- **メインの処理の最中の SIGINT**：ワークフロー実行のキャンセルは、API を呼んでから Runner がステップにシグナルを送るまでに 10 秒以上かかり得て、1〜2 秒で終わるメインの処理に届けられない（[GitHub Docker Action Failure and Cancellation](../research/github/docker-action-cancellation.md)）。そのため、Runner がキャンセルで送るのと同じ SIGINT を、同じ Runner の上で切り離したプロセスから、Action のステップの `docker run` のプロセスへ送る。送るまでの待ちをジョブごとに変え、Invoke の前と Invoke の開始の後の両方に届ける

`if: always()` のジョブはキャンセルされないため（[GitHub — Workflow cancellation reference](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-cancellation)）、ログの確認と資格情報の検索はキャンセルの後に動きます。
ワークフロー実行の結果は常に `cancelled` になり、試験の合否は `Check job logs` と `Credential leak` のジョブの結果で読みます。

SIGINT が Invoke の前と後のどちらで届いたかは、`Check job logs` がログに Invoke の行があるかから判定して出します。
Invoke の前に届いたジョブと、Invoke の開始の後に届いたジョブのどちらかが 1 つもなければ、`Check job logs` が失敗し、待ちの値の見直しを促します。
Lambda の実行環境が温まっているかで Invoke の時間が変わるため、待ちに合う値は実行ごとに少しずつずれます。

## Setup

試験用のリポジトリは、許可オーナーのリポジトリで、GitHub App のインストール先に含めません。
GitHub Actions 用 IAM Role の信頼ポリシーは、不変 ID 入りの形式の `sub` だけを受け付けるため、試験用のリポジトリの OIDC の `sub` をその形式にしておきます（[AWS OIDC JWT](../design/aws-oidc-jwt.md)）。
ワークフローは試験用のリポジトリ自身を、要求していない対象として読めないことと、Installation の対象外として拒否されることを確かめるため、インストール先に含めると試験が失敗します。
GitHub App のインストール先は、対象として使う 2 つのリポジトリだけにします。

ワークフローを、試験用のリポジトリの `.github/workflows/` に写します。

```bash
cp .github/workflows/_e2e-*.yml <試験用のリポジトリ>/.github/workflows/
```

`workflow_dispatch` は、試験用のリポジトリの既定のブランチにあるワークフローにしか使えません。
既定のブランチに取り込む前に動かすときは、写したワークフローの `on:` に、決まった名前のブランチへの `push` を加えます。
`_e2e-reusable-credential-leak.yml` はほかのワークフローから呼ばれるだけのため、加えません。

```yaml
on:
  push:
    branches:
      - e2e/failure
  workflow_dispatch:
```

| ワークフロー | ブランチ |
| --- | --- |
| E2E Success | `e2e/success` |
| E2E Interval | `e2e/interval` |
| E2E Failure | `e2e/failure` |
| E2E Cancel | `e2e/cancel` |

試験用のリポジトリに、次の Secrets と Variables を登録します。
接続情報の値と登録のコマンドは [Testing Environment Setup](setup-environment.md) にあります。

| 名前 | 種類 | 値 |
| --- | --- | --- |
| `GATES_AWS_ACCOUNT_ID` | Secrets | AWS アカウント ID |
| `GATES_ROLE_NAME` | Variables | Terraform の Output の `github_actions_role_name` |
| `GATES_FUNCTION_NAME` | Variables | Terraform の Output の `lambda_function_name` |
| `GATES_AWS_REGION` | Variables | `terraform.tfvars` の `region` |
| `GATES_AUDIENCE` | Variables | `terraform.tfvars` の `gates_audience` |
| `GATES_TARGET_PRIMARY` | Variables | GitHub App のインストール先の 1 つ目のリポジトリの完全名 |
| `GATES_TARGET_SECONDARY` | Variables | GitHub App のインストール先の 2 つ目のリポジトリの完全名 |

対象のリポジトリには、それぞれ `README.md` を置きます。
トークンで読めるかを、`README.md` の取得のステータスで確かめるためです。

試験の前に、IAM OIDC Provider がアカウントにあることを確かめます。
ない間は、Action が IAM Role を引き受けられません。

## Running

`push` のトリガーを加えたワークフローは、表のブランチへの push で動きます。
同じ内容でもう一度動かすときは、空のコミットを push します。

```bash
git commit --allow-empty -m "chore: run the GATES E2E test"
git push origin HEAD:e2e/failure
```

ワークフローを動かさずに文書だけを push するときは、コミットのメッセージに `[skip ci]` を付けます。

## Checks

ワークフローの成否のほかに、次を確かめます。

| 確認 | 方法 | 期待する結果 |
| --- | --- | --- |
| 失効 | 発行のワークフローの各 `Post Issue token` のステップと、キャンセルのワークフローの `Check job logs` | 発行したトークンの数だけ `revocation succeeded: HTTP status 204` がある |
| 監査記録 | CloudWatch Logs | Action が出した `request_id` ごとに `audit record` の行がちょうど 1 行あり、`caller.run_id` がワークフロー実行の ID と一致する。発行のワークフローでは `result` が `success`。失敗のワークフローの `authorization` と `installation` では、`result` がエラーコードと一致し、`installation` は `installation` の事例にだけある |
| トークンの不発行 | CloudWatch Logs | 失敗のワークフローの時間帯に、`result` が `success` の監査記録がない |
| 引き受けと Invoke の数 | CloudTrail と CloudWatch Logs | 失敗のワークフローで、成功した `AssumeRoleWithWebIdentity` の記録は `audience`、`function`、`authorization`、`installation` の 4 つの事例の数だけ増え、Lambda の `START` の行は `authorization` と `installation` の 2 つの事例の数だけ増える。`function` の Invoke は IAM が拒否するため、関数に届かない |
| Invoke の間隔と実行時間 | CloudWatch Logs の `REPORT` 行 | 間隔の試験で、2 回目以降の Invoke に `Init Duration` がなく、直前の Invoke からの間隔が 1 分前後である |
| 資格情報の非記録 | CloudWatch Logs | 同じ時間帯のロググループに `ghs_`、`eyJ`、`ASIA`、`FwoG`、`IQoJ` の文字列がない |

監査記録は、関数のロググループを `request_id` で検索して引きます。
失敗のワークフローの `request_id` は、`Check job logs` のジョブのログに出ます。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/<GATES_FUNCTION_NAME> \
  --filter-pattern '"<request_id>"'
```

`REPORT` 行と `START` 行は、試験の時間帯をミリ秒の UNIX 時刻で指定して引きます。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/<GATES_FUNCTION_NAME> \
  --start-time <開始> --end-time <終了> --filter-pattern REPORT \
  --query 'events[].[timestamp,message]' --output text
```

`AssumeRoleWithWebIdentity` の記録は、CloudTrail の Event history を試験の時間帯で引きます。

```bash
aws cloudtrail lookup-events --region <GATES_AWS_REGION> \
  --lookup-attributes AttributeKey=EventName,AttributeValue=AssumeRoleWithWebIdentity \
  --start-time <開始> --end-time <終了> \
  --query 'Events[].[EventTime,Username]' --output text
```

## Troubleshooting

Action が失敗すると、ステップの `::error::` の注釈に、失敗した段階と原因が出ます。
GATES の失敗応答の注釈には `error.code` と `request_id` が入るため、`request_id` で監査記録の `reason` を引いて切り分けます。

`Check job logs` が失敗したら、ジョブごとの件数の行で、どの確認が期待と違ったかを確かめます。
`_e2e-reusable-credential-leak.yml` が失敗したら、ジョブごとの件数の行で、どのジョブのログに一致する行があったかを確かめます。
どちらも一致した行そのものはログに出さないため、そのジョブのログを手元に取得して確かめます。

```bash
gh run view <run_id> -R <試験用のリポジトリ> --log > run.log
grep -n -E 'ghs_|eyJ|ASIA|FwoG|IQoJ' run.log
```
