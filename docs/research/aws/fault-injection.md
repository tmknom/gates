# AWS Fault Injection

本ドキュメントでは、デプロイした GATES に、運用者の操作で起こせて元に戻せる障害を 1 つずつ起こし、GitHub Actions のワークフローから Docker Action で発行を要求した結果を記述します。
障害は、KMS Key の無効化、Lambda の固定設定の環境変数の欠落、Reserved Concurrency 0、関数のタイムアウトの短縮の 4 つです。
障害ごとに、AWS の応答、GATES の応答と監査記録、Docker Action の注釈、関数のロググループの行、`terraform plan` の差分を掲載しています。

- 調査日: **2026/11/13**（関数のタイムアウトは **2026/11/14**）
- 要求: GitHub Actions の `ubuntu-latest` のランナーから、GATES の Docker Action で対象 1 件の `contents:read` を要求した。Docker Action は AWS SDK の再試行を止めて、関数 `gates` を同期 Invoke する
- 検証環境: 個人の AWS アカウント（`ap-northeast-1`）の関数 `gates`（`provided.al2023`、`arm64`、タイムアウト 45 秒、Reserved Concurrency 10）と、その関数が署名に使う `EXTERNAL` の KMS Key。障害の注入と復元は、運用者の管理 Role の一時認証情報で AWS CLI v2 と Terraform を実行した

AWS アカウント ID は `123456789012` に置き換えています。
KMS Key ARN、GATES 用 Audience、環境変数の値、トークンは記録していません。

## 取得方法

障害を 1 つ起こすごとに、ワークフローの `env` の障害の種類を書き換えて push し、Docker Action で発行を要求しました。
戻すごとにも同じ要求を 1 回行い、発行が成立することを確かめてから次の障害に進みました。
障害を起こした状態と戻した後に、`terraform plan` を実行しました。

ワークフローは、Docker Action のステップに `continue-on-error` を付け、後続のジョブでジョブのログを読んで次を確かめました。
一致した行そのものは出さず、件数と、期待した注釈の段階の文言だけを出しました。

- 障害ごとに期待した `::error::` の注釈が 1 つあること
- Docker Action のログの `Invoking the Lambda function at` が 1 回だけであること。Docker Action が Invoke を 1 回だけ試みたことを示す
- post-entrypoint が動き、失効を試みず、`::warning::` を出さないこと
- 4 つの Output がどれも空であること
- `Invoking the Lambda function at` の行から注釈の行までの秒数。ジョブのログの各行の時刻から計算した

## KMS Key Disabled

運用者の管理 Role で `aws kms disable-key` を実行し、`aws kms describe-key` で `KeyState` が `Disabled`、`Enabled` が `false` になったことを確かめてから要求しました。
要求は無効化の確認から約 1 分後で、最初の要求で無効化が反映されていました。

| 項目 | 結果 |
|---|---|
| KMS の応答 | `Sign` が `DisabledException`、HTTP ステータス `400` |
| GATES の応答 | `internal_error`、固定の文言 `An internal error occurred.`、`request_id` |
| Docker Action の注釈 | `GATES returned a failure response: error code "internal_error", message "An internal error occurred.", request id "<request_id>"` |
| Invoke から注釈まで | 2.548 秒 |
| 監査記録 | 1 件。`result` は `internal_error`。`caller` と `request` があり、`installation` はない |

監査記録の `reason` は次のとおりで、KMS のエラーコード、HTTP ステータス、KMS の request ID だけを含みました。

```text
internal failure: kms sign failed: code "DisabledException", status 400, request id "db5b9726-6712-4742-bc03-f7c9c24cc414"
```

`installation` がないのは、GitHub App JWT の署名に失敗し、Installation の取得を呼ばなかったためです。

`aws kms enable-key` で戻し、`KeyState` が `Enabled` になったことを確かめた後の要求は、成功しました。

## Lambda Initialization Failure

`aws lambda update-function-configuration --environment` で、6 つの固定設定の環境変数から `GATES_GITHUB_APP_CLIENT_ID` だけを除いた環境変数に置き換え、`aws lambda wait function-updated-v2` で更新を待ってから要求しました。

| 項目 | 結果 |
|---|---|
| Invoke の応答 | `FunctionError` が `Unhandled`、`errorType` が `Runtime.ExitError` |
| GATES の応答 | なし |
| Docker Action の注釈 | `the Lambda function returned FunctionError "Unhandled" with errorType "Runtime.ExitError"` |
| Invoke から注釈まで | 0.910 秒 |
| 監査記録 | なし |

要求の時間帯の関数のロググループの行は次のとおりです。
Runtime Version ARN は省略しています。

```text
INIT_START Runtime Version: provided:al2023.AmazonLinux2023.v228	Runtime Version ARN: ...
{"time":"2026-11-13T20:18:41.440521877Z","level":"ERROR","msg":"initialization failed","error":"invalid config: GATES_GITHUB_APP_CLIENT_ID: is not set"}
INIT_REPORT Init Duration: 98.39 ms	Phase: init	Status: error	Error Type: Runtime.ExitError
{"time":"2026-11-13T20:18:41.551421676Z","level":"ERROR","msg":"initialization failed","error":"invalid config: GATES_GITHUB_APP_CLIENT_ID: is not set"}
INIT_REPORT Init Duration: 89.23 ms	Phase: invoke	Status: error	Error Type: Runtime.ExitError
START RequestId: 4b5368f0-f74e-431b-bfee-912481b74de2 Version: $LATEST
END RequestId: 4b5368f0-f74e-431b-bfee-912481b74de2
REPORT RequestId: 4b5368f0-f74e-431b-bfee-912481b74de2	Duration: 107.18 ms	Billed Duration: 108 ms	Memory Size: 256 MB	Max Memory Used: 25 MB	Status: error	Error Type: Runtime.ExitError
```

初期化の失敗の行は、欠けた環境変数の名前だけを含み、ほかの 5 つの環境変数の名前と値を含みませんでした。

Docker Action の Invoke は 1 回でしたが、初期化の失敗は `Phase: init` と `Phase: invoke` の 2 回記録されました。
Lambda が、`Phase: init` で失敗した実行環境の初期化を、Invoke の処理の中でもう一度試みたと読めます。
Lambda Developer Guide は、Init のタイムアウトのときに最初の Invoke で Init をやり直すと記述していますが、Init での異常終了のときのやり直しは記述していません。

環境変数を `terraform apply` で戻した後の要求は、成功しました。
環境変数の変更で実行環境が入れ替わったため、この要求はコールドスタートで、`Invoking the Lambda function at` の行から発行のログの行まで 3.096 秒でした。

## Reserved Concurrency 0

`aws lambda put-function-concurrency --reserved-concurrent-executions 0` を実行し、応答の `ReservedConcurrentExecutions` が `0` であることを確かめてから要求しました。

| 項目 | 結果 |
|---|---|
| Invoke の応答 | `TooManyRequestsException`、HTTP ステータス `429` |
| GATES の応答 | なし |
| Docker Action の注釈 | `the Lambda Invoke failed: AWS error code "TooManyRequestsException", HTTP status 429` |
| Invoke から注釈まで | 0.457 秒 |
| 監査記録 | なし |

要求の時間帯の関数のロググループには、`INIT_START` と `START` を含め、行が 1 つもありませんでした。
関数は実行されず、Lambda の Invoke API が要求を拒否しました。

Lambda API Reference は、関数の単位の同時実行数の上限（`ReservedFunctionConcurrentInvocationLimitExceeded`）を超える Invoke に `TooManyRequestsException` を返すと記述しています。
Docker Action はエラーコードと HTTP ステータスだけを出すため、どの上限による拒否かは応答から観測していません。

関数に届かない要求の回数は、関数のロググループに残りません。
Docker Action のログの `Invoking the Lambda function at` は 1 回でしたが、この行は Docker Action が Invoke を始めたことを示すだけで、AWS SDK の中の再試行を数えません。
AWS SDK for Go v2 の既定の再試行は、`TooManyRequestsException` を再試行の対象とし、最大 3 回試み、1 回目の後に 0〜2 秒、2 回目の後に 0〜4 秒の乱数の待ちを入れます（v1.47.3 のソース）。
再試行していれば、注釈までの時間に 2 回の乱数の待ちが加わります。
注釈までの 0.457 秒に収まるのは 2 回の待ちの和が 0.457 秒未満の場合に限られ、その見込みは小さいものの、待ちが乱数のため確率的な読み取りにとどまります。
Docker Action が再試行を止めた設定で要求を送ることは、Docker Action のローカルの試験で確かめています。

Reserved Concurrency を `terraform apply` で 10 に戻した後の要求は、成功しました。

## Function Timeout

`aws lambda update-function-configuration --timeout 1` で関数のタイムアウトを 1 秒にし、`aws lambda wait function-updated-v2` で更新を待ってから要求しました。
タイムアウトの変更で実行環境が入れ替わったため、この要求はコールドスタートでした。

| 項目 | 結果 |
|---|---|
| Invoke の応答 | `FunctionError` が `Unhandled`、`errorType` が `Sandbox.Timedout` |
| GATES の応答 | なし |
| Docker Action の注釈 | `the Lambda function returned FunctionError "Unhandled" with errorType "Sandbox.Timedout"` |
| Invoke から注釈まで | 1.899 秒 |
| 監査記録 | 1 件。`result` は `identity_verification_unavailable`。`request` があり、`caller` と `installation` はない |

要求の時間帯の関数のロググループの行は次のとおりです。
Runtime Version ARN は省略し、要求の対象は `<owner>/<repo>` に置き換えています。

```text
INIT_START Runtime Version: provided:al2023.AmazonLinux2023.v228	Runtime Version ARN: ...
START RequestId: 1e262cb7-cc11-48da-a16d-479c1adb4ab5 Version: $LATEST
{"time":"2026-11-14T03:21:08.727618283Z","level":"INFO","msg":"audit record","request_id":"1e262cb7-cc11-48da-a16d-479c1adb4ab5","result":"identity_verification_unavailable","reason":"retrieve discovery: transient failure: request failed","request":{"repositories":["<owner>/<repo>"],"permissions":{"contents":"read"}}}
END RequestId: 1e262cb7-cc11-48da-a16d-479c1adb4ab5
REPORT RequestId: 1e262cb7-cc11-48da-a16d-479c1adb4ab5	Duration: 1000.00 ms	Billed Duration: 1090 ms	Memory Size: 256 MB	Max Memory Used: 37 MB	Init Duration: 89.34 ms	Status: timeout
```

関数は、GitHub の OIDC Discovery の取得の途中で 1 秒の期限に達しました。
Lambda は Invoke の期限を要求の ctx の期限として渡すため、Discovery の取得は期限切れで失敗し、GATES はこれを一時的な障害に分類して監査記録を書きました。
しかし応答を返す前に Lambda が関数を打ち切り、呼び出し元が受け取ったのは `Sandbox.Timedout` の `FunctionError` でした。
監査記録の `identity_verification_unavailable` は、呼び出し元に届いていません。

Installation の取得と発行には進んでおらず、GitHub 側でトークンは発行されていません。
打ち切りが発行の後に来れば、トークン文字列を受け取れず失効も試みられないまま、有効期限まで GitHub 側に残り得ますが、この要求では起きていません。

タイムアウトを `terraform apply` で 45 秒に戻した後の E2E テストは、成功しました。

## Terraform Plan

障害を起こした状態の `terraform plan` は次のとおりでした。

| 状態 | Plan |
|---|---|
| 障害の前 | `No changes` |
| KMS Key を無効にした状態 | `No changes` |
| `GATES_GITHUB_APP_CLIENT_ID` を除いた状態 | `aws_lambda_function.gates` の in-place の更新 1 件（`0 to add, 1 to change, 0 to destroy`）。差分は環境変数への `GATES_GITHUB_APP_CLIENT_ID` の追加だけ |
| Reserved Concurrency 0 の状態 | `aws_lambda_function.gates` の in-place の更新 1 件。差分は `reserved_concurrent_executions = 0 -> 10` だけ |
| タイムアウトを 1 秒にした状態 | `aws_lambda_function.gates` の in-place の更新 1 件。差分は `timeout = 1 -> 45` だけ |
| すべて戻した後 | `No changes` |

KMS Key の無効化は Plan に現れませんでした。
GATES の Terraform は `aws_kms_external_key` に `enabled` を書いていません。

## Restoration and Credential Search

すべて戻した後の状態は、KMS Key の `KeyState` が `Enabled`、関数の環境変数が 6 つの固定設定のキー、Reserved Concurrency が 10、タイムアウトが 45 秒で、障害の前と同じでした。
2 日のそれぞれで戻した後に、E2E テストの `_e2e-success.yml` と `_e2e-failure.yml` が成功しました。

障害を起こした時間帯の監査記録は、KMS Key の無効化の `internal_error` と、関数のタイムアウトの `identity_verification_unavailable` の 2 件だけで、`result` が `success` の記録はありませんでした。

各実行のジョブのログには、資格情報の文字列（`ghs_`、`eyJ`、`ASIA`、`FwoG`、`IQoJ`）がありませんでした。
試験の時間帯の関数のロググループ全体にも、資格情報の文字列、KMS Key ID の一部、GATES 用 Audience の値はありませんでした。

## Workflow File

障害の種類ごとに期待した注釈は、次のように拡張正規表現でジョブのログと照らしました。

```bash
declare -A expected=(
  [kms-disabled]='##\[error\]GATES returned a failure response: error code "internal_error", message "[^"]+", request id "[^"]+"'
  [missing-env]='##\[error\]the Lambda function returned FunctionError "[^"]+" with errorType "Runtime\.ExitError"'
  [zero-concurrency]='##\[error\]the Lambda Invoke failed: AWS error code "TooManyRequestsException", HTTP status 429'
  [function-timeout]='##\[error\]the Lambda function returned FunctionError "[^"]+"( with errorType "[^"]+")?'
)
```

障害の注入と復元のコマンドは次のとおりです。

```bash
aws kms disable-key --key-id <key-arn>
aws kms enable-key --key-id <key-arn>
aws kms describe-key --key-id <key-arn> --query 'KeyMetadata.{KeyState: KeyState, Enabled: Enabled}'
aws lambda update-function-configuration --function-name gates --environment file://env-without-client-id.json
aws lambda wait function-updated-v2 --function-name gates
aws lambda put-function-concurrency --function-name gates --reserved-concurrent-executions 0
aws lambda update-function-configuration --function-name gates --timeout 1
```

## References

- [AWS — Lambda API Reference: Invoke](https://docs.aws.amazon.com/lambda/latest/api/API_Invoke.html)
- [AWS — Understanding the Lambda execution environment lifecycle](https://docs.aws.amazon.com/lambda/latest/dg/lambda-runtime-environment.html)
- [AWS — AWS KMS API Reference: Sign](https://docs.aws.amazon.com/kms/latest/APIReference/API_Sign.html)
- [AWS — Enable and disable keys](https://docs.aws.amazon.com/kms/latest/developerguide/enabling-keys.html)
