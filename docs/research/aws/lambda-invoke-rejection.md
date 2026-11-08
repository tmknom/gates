# AWS Lambda Invoke Rejection

本ドキュメントでは、デプロイした GATES の関数を同期 Invoke し、IAM と Lambda の Invoke API が関数の実行前に拒否した呼び出しと、関数に渡した Payload を記述します。
AWS CLI の出力と、関数のロググループに残った行を掲載しています。

- 調査日: **2026/11/08**
- 要求: `aws lambda invoke --invocation-type RequestResponse --payload fileb://<file>`（AWS CLI v2）
- 検証環境: 個人の AWS アカウント（`ap-northeast-1`）の関数 `gates`（`provided.al2023`、`arm64`）。呼び出し元は、GitHub Actions の `ubuntu-latest` のランナーと、運用者の手元のシェル

AWS アカウント ID は `123456789012` に置き換えています。
GATES 用 OIDC JWT と AWS 一時認証情報は記録していません。

## Invoke Permission

関数の実行前に拒否された呼び出しは次のとおりです。
どの呼び出しでも、AWS CLI は応答のファイルを作りませんでした。

| 呼び出し元 | 呼び出し | AWS CLI のエラー |
|---|---|---|
| ランナー（AWS の認証情報を設定する前） | `--function-name gates` | `NoCredentials` |
| 手元。ポリシーを一つも付けない IAM Role を引き受けた一時認証情報 | `--function-name gates` | `AccessDeniedException` |
| ランナー。GATES の GitHub Actions 用 IAM Role（`lambda:InvokeFunction` の Resource は `gates` の修飾なしの ARN だけ） | `--function-name gates-rejection-test`（存在しない関数） | `AccessDeniedException` |
| 同上 | `--function-name gates --qualifier 1`（存在しないバージョン） | `AccessDeniedException` |

`NoCredentials` の出力は次のとおりです。

```text
aws: [ERROR]: An error occurred (NoCredentials): Unable to locate credentials. You can configure credentials by running "aws login".
```

`AccessDeniedException` の出力は、どれも `no identity-based policy allows the lambda:InvokeFunction action` で終わり、`resource` に要求した ARN が入りました。
手元からの呼び出しの出力は次のとおりで、終了コードは `254` でした。

```text
aws: [ERROR]: An error occurred (AccessDeniedException) when calling the Invoke operation: User: arn:aws:sts::123456789012:assumed-role/gates-rejection-test/rejection-test is not authorized to perform: lambda:InvokeFunction on resource: arn:aws:lambda:ap-northeast-1:123456789012:function:gates because no identity-based policy allows the lambda:InvokeFunction action
```

ランナーからの 2 件の `resource` は、`arn:aws:lambda:ap-northeast-1:123456789012:function:gates-rejection-test` と `arn:aws:lambda:ap-northeast-1:123456789012:function:gates:1` でした。
存在しない関数とバージョンへの呼び出しも `ResourceNotFoundException` ではなく `AccessDeniedException` で、存在の有無は出力に現れませんでした。

## Payload Parsing

GitHub Actions 用 IAM Role で、JSON として不正な Payload と、JSON の解釈が実装で分かれる Payload を送りました。

| Payload | Invoke の結果 | GATES の応答 |
|---|---|---|
| `{"version":1,`（JSON として不正） | `InvalidRequestContentException` で、関数は実行されない | なし |
| 同じキー `permissions` を 2 回持つオブジェクト | `StatusCode` `200`、`FunctionError` なし | `invalid_request` |
| オブジェクトの後に `{}` が続くもの | `StatusCode` `200`、`FunctionError` なし | `invalid_request` |
| 未知のフィールド `unknown` を持つオブジェクト | `StatusCode` `200`、`FunctionError` なし | `invalid_request` |

`InvalidRequestContentException` の出力は次のとおりです。

```text
aws: [ERROR]: An error occurred (InvalidRequestContentException) when calling the Invoke operation: Could not parse request body into json: Could not parse payload into json: Unexpected end-of-input within/between Object entries
```

Lambda API Reference は、この例外を「the request body could not be parsed as JSON」の場合に返すと記述しています。
Lambda Developer Guide の例では、エラー文言に読めなかった Payload の一部が入ります。

重複したキーと後続のデータの Payload は、GATES の形式検証が `malformed payload` の理由で拒否しました。
このため、関数に届いた Payload には、重複したキーと後続のデータが残っていたと読めます。
Lambda が Payload のバイト列を変えずに渡すかは、確かめていません。

## Function Logs

試験の時間帯の関数のロググループには、`START` の行が 13 行あり、どれもワークフローが応答から得た `request_id` と一致しました。
上の表で関数の実行前に拒否された 5 件（`NoCredentials`、`AccessDeniedException` の 3 件、`InvalidRequestContentException`）の行はありませんでした。

## CloudTrail

Lambda Developer Guide は、Lambda の `Invoke` を CloudTrail のデータイベントとし、データイベントの記録を有効にしていない場合は記録せず、イベント履歴にも残らないと記述しています。
検証環境ではデータイベントの記録を有効にしていないため、`Invoke` の記録は、成功も `AccessDeniedException` も取得できませんでした。

## Workflow File

ランナーからの呼び出しに使用したワークフローのうち、本ドキュメントの拒否に関わる Invoke の部分を抜き出したものは次のとおりです。

```yaml
name: GATES Rejection
on:
  workflow_dispatch:
jobs:
  reject:
    runs-on: ubuntu-latest
    timeout-minutes: 15
    permissions:
      id-token: write
      contents: read
    env:
      AWS_REGION: ap-northeast-1
    steps:
      # AWS の認証情報を設定する前に呼ぶ
      - name: Invoke without AWS credentials
        shell: bash
        run: |
          set -euo pipefail
          echo '{"version":1}' > "${RUNNER_TEMP}/payload.json"
          aws lambda invoke --function-name gates \
            --invocation-type RequestResponse \
            --payload "fileb://${RUNNER_TEMP}/payload.json" \
            "${RUNNER_TEMP}/response.json" || echo "exit: $?"

      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@e1253824e5c10ff9df46874f81ed3ec929e19cfd # v6.3.0
        with:
          role-to-assume: arn:aws:iam::${{ secrets.GATES_AWS_ACCOUNT_ID }}:role/gates-github-actions
          aws-region: ap-northeast-1

      - name: Invoke other function, qualified ARN, and malformed payload
        shell: bash
        run: |
          set -euo pipefail
          invoke() {
            aws lambda invoke --invocation-type RequestResponse \
              --payload "fileb://${RUNNER_TEMP}/payload.json" "$@" \
              "${RUNNER_TEMP}/response.json" || echo "exit: $?"
          }
          echo '{"version":1}' > "${RUNNER_TEMP}/payload.json"
          invoke --function-name gates-rejection-test
          invoke --function-name gates --qualifier 1
          # JSON として不正な Payload には資格情報を含めない。エラー文言に Payload の一部が入るため
          printf '{"version":1,' > "${RUNNER_TEMP}/payload.json"
          invoke --function-name gates
```

手元からの呼び出しは、ポリシーを一つも付けない IAM Role を引き受け、その一時認証情報で同じ `aws lambda invoke` を実行しました。

```bash
aws sts assume-role \
  --role-arn arn:aws:iam::123456789012:role/gates-rejection-test \
  --role-session-name rejection-test
aws lambda invoke --function-name gates --invocation-type RequestResponse \
  --payload fileb://payload.json response.json
```

## References

- [AWS — Lambda API Reference: Invoke](https://docs.aws.amazon.com/lambda/latest/api/API_Invoke.html)
- [AWS — Invoking a Lambda function synchronously](https://docs.aws.amazon.com/lambda/latest/dg/invocation-sync.html)
- [AWS — AWS CLI Lambda Invoke](https://docs.aws.amazon.com/cli/latest/reference/lambda/invoke.html)
- [AWS — Logging AWS Lambda API calls using AWS CloudTrail](https://docs.aws.amazon.com/lambda/latest/dg/logging-using-cloudtrail.html)
