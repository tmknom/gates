# AWS Lambda Environment Size

本ドキュメントでは、AWS Lambda の環境変数の合計の上限の数え方と、デプロイした GATES の関数の環境変数の現在の合計を記述します。
YAML の文書を環境変数に置くときに、保護対象 1 件と許可 Caller 1 件が合計をどれだけ増やすかも計算しています。

- 調査日: **2026/11/21**
- 要求: AWS CLI v2 の `aws lambda get-function-configuration` に、JMESPath の `--query` で `length(to_string(Environment.Variables))` と `length(keys(Environment.Variables))` を指定した。環境変数の値は出力していない
- 検証環境: 個人の AWS アカウント（`ap-northeast-1`）の関数 `gates`。`GATES_ISSUABLE_PERMISSIONS` の値は `contents:read`

## 上限の数え方

AWS Lambda Developer Guide の [Lambda quotas](https://docs.aws.amazon.com/lambda/latest/dg/gettingstarted-limits.html) と [Troubleshoot configuration issues in Lambda](https://docs.aws.amazon.com/lambda/latest/dg/troubleshooting-configuration.html) は、関数の環境変数の上限を次のように書いています。

- **上限**：Quotas の表は、関数の環境変数を「4 KB, for all environment variables associated with the function, in aggregate」とする。変更できない上限である
- **数え方**：トラブルシューティングの節は、関数の設定に保存される変数のオブジェクトの大きさが 4096 バイトを超えてはならず、その大きさは「key names, values, quotes, commas, and brackets」を含むとする。例は、空白を除いた `{"BUCKET":"amzn-s3-demo-bucket","KEY":"file.txt"}` の 39 文字を 39 バイトと数える
- **文字**：同じ節は、標準の ASCII の文字は 1 文字 1 バイト、拡張 ASCII と Unicode の文字は 2 バイトから 4 バイトを使うとする
- **超えたとき**：`UpdateFunctionConfiguration` は `InvalidParameterValueException`（4KB の上限を超えたこと）か `RequestEntityTooLargeException`（要求の本文が 5120 バイトを超えたこと）を返す

資料は、値に含まれる改行と二重引用符を、JSON の文字列のエスケープ（`\n`、`\"`）の後の長さで数えるかを書いていません。
資料の例は、改行も二重引用符も含みません。
以下では、エスケープの前と後の両方を計算します。

## 現在の合計

関数 `gates` の環境変数は 7 個で、空白を除いた JSON にした長さは 338 バイトでした。
JMESPath の `to_string` は、空白を除いた JSON を返します。
`terraform/local/terraform.tfvars` の値から、Terraform が設定する 7 個の変数を空白を除いた JSON にして数えた長さも 338 バイトで、一致しました。

| 環境変数 | キーのバイト数 | 値のバイト数 |
|---|---|---|
| `GATES_ALLOWED_OWNER_ID` | 22 | 7 |
| `GATES_ALLOWED_OWNER_NAME` | 24 | 6 |
| `GATES_ALLOWED_OWNER_TYPE` | 24 | 4 |
| `GATES_GITHUB_APP_CLIENT_ID` | 26 | 20 |
| `GATES_AUDIENCE` | 14 | 12 |
| `GATES_KMS_KEY_ARN` | 17 | 80 |
| `GATES_ISSUABLE_PERMISSIONS` | 26 | 13 |

値のバイト数はデプロイごとに変わります。
KMS Key ARN は `arn:aws:kms:<リージョン>:<12 桁>:key/<36 文字の UUID>` の形で、`ap-northeast-1` では 80 バイトです。

## 保護対象の一覧が増やすバイト数

環境変数 `GATES_PROTECTED_REPOSITORIES` に、リポジトリ（保護対象）ごとに要求できる Caller（許可 Caller）と権限の上限を並べた、次の書式の YAML を置く場合を数えました。
ID は 10 桁で引用符で囲み、名前は `n` 文字の ASCII、行末は LF とします。
「JSON」の列は、改行を `\n`、二重引用符を `\"` にエスケープした後のバイト数です。

```yaml
repositories:
  # 共通基盤
  - id: "1234567890"
    name: platform-infra
    callers:
      - repository_id: "1234567891"
        repository_name: deploy-tools
        permissions:
          contents: write
          pull_requests: read
```

| 要素 | エスケープの前 | JSON |
|---|---|---|
| 変数を一つ加えること（キー 28 文字、キーと値の引用符、`:`、`,`） | 34 | 34 |
| `repositories:` の行 | 14 | 15 |
| 保護対象のない一覧（`repositories: []` の 1 行） | 17 | 18 |
| 保護対象 1 件（`id`、`name`、`callers:` の 3 行） | 45 + `n` | 50 + `n` |
| 許可 Caller 1 件（`repository_id`、`repository_name`、`permissions:`、権限 1 行の 4 行。権限は `contents: write`） | 109 + `n` | 115 + `n` |
| 権限 1 行の追加（`pull_requests: read`） | 30 | 31 |
| コメント 1 行（`  # ` と行末を含み、本文 `b` バイト） | 5 + `b` | 6 + `b` |

権限の行は、権限名と水準の長さに 13 バイト（字下げ 10、`: `、行末）を足した長さで、JSON では 1 バイト増えます。
コメントの日本語は、UTF-8 で 1 文字 3 バイトです（`共通基盤` は 12 バイト）。
Terraform のヒアドキュメントは最後の行の後にも改行を置くため、どの行も行末を含めて数えています。

現在の合計に変数と `repositories:` の行を加えると 387 バイト（JSON）で、残りは 3709 バイトです。
名前を 20 文字、許可 Caller を 1 件、権限を 1 行、コメントを置かないとすると、保護対象 1 件と許可 Caller 1 件の組は JSON で 205 バイトで、18 組まで置けます。
