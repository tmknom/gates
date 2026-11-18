# Troubleshooting

Action が失敗したときに、注釈から原因と直し方を引くためのリファレンスです。
ワークフロー利用者は §1 と §2 を、運用者は §3 以降を使います。

Action が失敗すると、ジョブのログとワークフロー実行のページに `::error::` の注釈が出ます。
注釈の先頭の文言が、失敗した段階を示します。
文言と段階の一覧は、[Issue Token](../../.github/actions/issue-token/README.md) の「失敗の読み方」にあります。

## 1. Failures the Workflow User Can Fix

| 注釈 | 原因 | 直し方 |
|---|---|---|
| `ACTIONS_ID_TOKEN_REQUEST_TOKEN is not set` で始まる | ジョブに `id-token: write` がない。ジョブに `permissions` を書くと、ワークフローの `permissions` は使われない | Action を置いたジョブの `permissions` に `id-token: write` を書く |
| `input <入力名> is empty` で始まる | 入力が空。参照した Secret か Variable がないか、参照できない | 入力名と、Secret と Variable の名前の綴りを見直す。正しければ、運用者に登録を確かめてもらう |
| `invalid inputs` で始まる | `repositories` か `permissions` の書式の誤り | 注釈の次の行に出る要素と理由に従って直す |
| `could not get the OIDC JWT for GATES` で始まり、`HTTP status 400` を含む | `audience` が、GitHub が受け付けない値 | `audience` に、運用者が登録した Variable を渡す |
| `could not get the OIDC JWT for` で始まり、`HTTP status 400` を含まない | GitHub の一時的な障害か、Runner から OIDC JWT の取得先に届かない | 再実行する。セルフホストの Runner なら、通信の許可を確かめる |
| `AssumeRoleWithWebIdentity failed` で始まる | `role-arn` の誤り、ワークフローを置くリポジトリが許可オーナーのものでない、またはリポジトリの OIDC JWT が不変の ID を含む形式でない | `role-arn` が `arn:aws:iam::${{ secrets.GATES_AWS_ACCOUNT_ID }}:role/${{ vars.GATES_ROLE_NAME }}` の形かを見直す。正しければ §2 に進む |
| `AssumeRoleWithWebIdentity failed` で始まり、Dependabot のワークフローでだけ起きる | Dependabot のワークフローは GitHub Actions の Secrets を参照できない | 運用者に、Dependabot の Secrets への登録を頼む |
| `the Lambda Invoke failed` で始まり、`AccessDeniedException` を含む | `function-name` か `aws-region` の誤り | 運用者が登録した Variables を渡す |
| `error code "invalid_request"` を含む | 要求の形が正しくない（名前に使えない文字、500 を超える `repositories`、大文字の権限名など） | `repositories` は `owner/repo` の形で書く。`permissions` は `contents:read` と書く |
| `error code "invalid_id_token"` を含む | `audience` が GATES の設定と一致しない | 運用者が登録した Variable を渡す。渡しているなら §2 に進む |
| `error code "authorization_denied"` を含む | 許可オーナー以外のリポジトリか、`contents:read` 以外の権限を要求した | 許可オーナーのリポジトリと `contents:read` だけを要求する。一つでも許可されないと、要求全体が失敗する |
| `error code "github_issuance_rejected"` を含む | インストール対象にないか、存在しないリポジトリを要求した | 綴りを確かめる。正しければ、インストール対象に加えるよう頼むか、要求から外す |
| ジョブの `outputs` に置いたトークンが空になる | GitHub はマスクされた値をジョブの外に渡さない | トークンが要るジョブごとに Action のステップを置く |
| トークンで呼んだ API が `403` か `404` を返す | `403` は `contents:read` でできない操作、`404` は要求に含めなかったリポジトリ | `403` の操作は GATES のトークンではできない。`404` のリポジトリは `repositories` に加える |

## 2. Reporting a Failure to the Operator

§1 に当たらない失敗と、§1 の直し方で解消しない失敗は、運用者に伝えます。
ただし、注釈の `message` が `Retry later.` で終わる失敗は一時的な障害のため、まず再実行します。
`the job was canceled` は、ジョブがキャンセルされただけです。

運用者には、次を伝えます。
注釈はトークンも認証情報も含まないため、そのまま渡せます。

- 注釈の全文
- 注釈にあれば、`request_id`
- ワークフロー実行の URL と、何回目の実行か
- 失敗した時刻（ログの `Invoking the Lambda function at` の行の時刻。なければ注釈の時刻）

## 3. Finding the Audit Record

ここからは運用者の作業です。
コマンドは、GATES を適用した AWS アカウントのログと設定を読める一時認証情報で、`AWS_REGION` を GATES のリージョンにして実行します。

`request_id` があれば、それで監査記録を引きます。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --filter-pattern '{ $.request_id = "<request_id>" }' \
  --query 'events[].message' --output json \
  | jq '.[] | fromjson | select(.msg == "audit record") | del(.claims_set)'
```

`result` がエラーコードで、`reason` が失敗した理由です。
`reason` は応答にも注釈にも出ないため、ワークフロー利用者には見えません。
エラーコードごとの原因は §4 にあります。

`request_id` がなく、注釈が `GATES returned a failure response` で始まるときは、ワークフロー実行の ID（URL の `.../actions/runs/<ID>`）で引きます。
呼び出し元を確かめる前に失敗した要求（`invalid_request`、`invalid_id_token` など）は、この方法では見つからないため、時刻で引きます。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --filter-pattern '{ $.caller.run_id = <ワークフロー実行の ID> }' \
  --query 'events[].message' --output json \
  | jq -c '.[] | fromjson | {time, request_id, result, reason, run_attempt: .caller.run_attempt}'
```

注釈に `request_id` がない失敗は §5 で調べます。

## 4. Causes and Fixes by Error Code

| エラーコード | 原因 | 確かめることと直し方 |
|---|---|---|
| `invalid_request` | 要求の形が正しくない。Action と GATES のリリースが違う場合にも起きる | `reason` で不正なフィールドを見る。ワークフロー利用者に §1 の直し方を伝える。Action の参照先がデプロイした GATES と同じリリースかを確かめる |
| `invalid_id_token` | GATES 用の OIDC JWT が受け付けられない。Audience の食い違い、期限切れ、署名の不正など | Variable の `GATES_AUDIENCE` が `gates_audience` と同じかを確かめる |
| `authorization_denied` | 許可オーナー以外のリポジトリか、`contents:read` 以外の権限の要求 | `request` の値を見て、ワークフロー利用者に §1 の直し方を伝える |
| `identity_verification_unavailable` | GitHub の OIDC の公開鍵を一時的に取得できない | 再実行で解消し得る。繰り返すなら [GitHub Status](https://www.githubstatus.com/) を見る |
| `signing_unavailable` | AWS KMS の署名が一時的に失敗した | 再実行で解消し得る。`reason` が `ThrottlingException` で繰り返すなら、KMS のクォータを確かめる |
| `installation_unavailable` | GATES が Installation を使えない。GitHub App のアンインストール（`status 404`）、Installation の停止（`installation is suspended`）、`github_app_client_id` の誤り、使用中の鍵と GitHub App の鍵の不一致、GitHub からの鍵の削除 | アンインストールなら、`Only select repositories` で再インストールする。停止なら、悪用への対処で意図して停止したものでないかを確かめてから解く。鍵の不一致は下記で確かめる |
| `github_api_unavailable` | GitHub API が一時的に失敗した | 再実行で解消し得る。繰り返すなら GitHub Status を見る |
| `github_issuance_rejected` | インストール対象にないか、存在しないリポジトリの要求 | `request.repositories` を Installation の画面のリポジトリと比べる。入っているのに拒否されるなら、`reason` の GitHub の request ID を控えて GitHub に問い合わせる |
| `issuance_verification_failed` | GitHub が返したトークンが要求と合わない | `reason` と GitHub の request ID を控え、GATES の開発者に報告する。`revocation` が `failed` なら、トークンは 1 時間まで残り得る |
| `internal_error` | ほかに当たらない失敗。使用中の KMS Key が使えない（無効、削除の予約、鍵素材のインポート前）、Lambda の権限の不足、Lambda の panic など | 下記で KMS Key の状態を確かめる。`Disabled` なら、悪用への対処や鍵の更新で意図して無効にしたものでないかを確かめてから有効にする。`PendingDeletion` なら削除の予約を取り消す。`PendingImport` なら [Key Material Import](import-key-material.md) で鍵素材をインポートする。panic なら、同じ `request_id` の `panic recovered` の行を見る |

使用中の KMS Key の状態を確かめます。

```bash
aws lambda get-function --function-name gates \
  --query 'Configuration.Environment.Variables.GATES_KMS_KEY_ARN' --output text
aws kms describe-key --key-id <KMS Key の ARN> --query 'KeyMetadata.KeyState' --output text
```

使用中の鍵と GitHub App の鍵が一致するかは、次の出力と、GitHub App の `Credentials` → `Key pairs` の `SHA256:` の後の値を比べて確かめます。
一致する鍵がなければ、鍵の更新で新しい鍵に替えます。

```bash
aws kms get-public-key --key-id <KMS Key の ARN> --query PublicKey --output text \
  | openssl base64 -d -A | openssl sha256 -binary | openssl base64
```

## 5. Failures without a request_id

要求が GATES に届かなかったか、GATES が応答を書けなかった失敗です。

### 5.1 AssumeRoleWithWebIdentity failed

Action が IAM Role を引き受けられませんでした。
§1 の原因のほかに、次を確かめます。

- AWS アカウントに IAM OIDC Provider があり、Client ID に `sts.amazonaws.com` がある
- `allowed_owner_id` と `allowed_owner_name` が、許可オーナーの値と一致する。許可オーナーが改名したなら、`allowed_owner_name` を新しい名前にして適用し直す

失敗した時刻の前後で、引き受けの記録を CloudTrail から引きます。
`--region` は、ワークフローの `aws-region` のリージョンにします。

```bash
aws cloudtrail lookup-events --region <リージョン> \
  --lookup-attributes AttributeKey=EventName,AttributeValue=AssumeRoleWithWebIdentity \
  --start-time <開始> --end-time <終了> \
  --query 'Events[].CloudTrailEvent' --output json \
  | jq -r '.[] | fromjson | [.eventTime, (.errorCode // "success"), .userIdentity.userName] | @tsv'
```

`userIdentity.userName` は、OIDC JWT の `sub` です。

- `repo:` の後に `@` と Owner ID がない：リポジトリの OIDC JWT が名前だけの形式。リポジトリか Organization の設定で切り替える
- オーナーが許可オーナーでない：許可オーナー以外のリポジトリからの呼び出し
- 記録がない：`role-arn` が別のアカウントを指しているか、`aws-region` が誤っている

### 5.2 the Lambda Invoke failed

AWS Lambda が要求を関数に渡しませんでした。
関数のログにも、既定の CloudTrail にも残らないため、注釈の AWS のエラーコードで見分けます。

| AWS のエラーコード | 原因 | 直し方 |
|---|---|---|
| `AccessDeniedException` | `function-name` か `aws-region` の誤り。存在しない関数もこのコードになる | ワークフロー利用者に、登録した Variables を渡しているかを確かめてもらう |
| `TooManyRequestsException` | 同時実行数の上限を超えた | `aws lambda get-function-concurrency --function-name gates` で予約を確かめる。`0` なら、悪用への対処で意図して止めたものでないかを確かめてから、`lambda_reserved_concurrency` を見直して適用する |

### 5.3 the Lambda function returned FunctionError

関数は動いたものの、応答を書けずに終わりました。
注釈の `errorType` で見分けます。

| `errorType` | 原因 | 直し方 |
|---|---|---|
| `Runtime.ExitError` | 関数の初期化の失敗。関数の設定の環境変数が欠けていると起きる | ログの `initialization failed` の行に、欠けた設定の名前が出る。`make tf-plan` で差分を確かめてから適用し直す |
| `Sandbox.Timedout` | 関数のタイムアウト | ログの `REPORT` の行に `Status: timeout` が出る。タイムアウトが Terraform の値（45 秒）から変わっていれば、適用し直す |

時刻でログを引きます。
`<開始>` と `<終了>` は、ミリ秒の UNIX 時刻です。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --start-time <開始> --end-time <終了> \
  --query 'events[].message' --output text
```

### 5.4 Other Annotations

`the Lambda response` か `the success response has no usable` で始まる注釈は、Action が GATES の応答を読めなかったことを示します。
Action の参照先が、デプロイした GATES と同じリリースのコミットかを確かめます。

Runner の環境（`invalid ACTIONS_ID_TOKEN_REQUEST_URL`、`could not write to GITHUB_OUTPUT` など）や、Action の想定外の失敗（`the child process`、`an unexpected failure occurred`）は、AWS に手がかりがありません。
注釈の全文とワークフロー実行の URL を添えて、Runner の管理者か GATES の開発者に伝えます。

## 6. Revocation Warning

`revocation failed:` で始まる `::warning::` は、ジョブの終了時にトークンを失効できなかったことを示します。
ジョブは失敗せず、トークンは期限（発行から 1 時間）を過ぎれば使えなくなります。
残ったトークンで読めるのは、要求したリポジトリだけです。

セルフホストの Runner では、`api.github.com` への通信の許可を確かめます。
GitHub がホストする Runner で繰り返し出るときは、注釈の全文とワークフロー実行の URL を運用者に伝えます。

同じステップに `::error::` の注釈もあれば、そちらから §1 以降を引きます。
