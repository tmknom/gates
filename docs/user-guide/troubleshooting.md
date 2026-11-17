# Troubleshooting

GATES（GitHub App Token Exchange System）は、GitHub Actions のワークフローに、指定したリポジトリだけを読める短命なトークンを発行する仕組みです。
ワークフローは GATES の Docker Action（以下、Action）のステップを置き、Action が、運用者の AWS アカウントで動く GATES の AWS Lambda の関数（以下、Lambda）を呼び出してトークンを受け取ります。
本文書は、Action が失敗したときに、失敗の手がかりから原因と直し方を引くためのリファレンスです。
読者は、**ワークフロー利用者**（GATES からトークンを受け取るワークフローを書く人）と、**GATES 運用者**（GATES を AWS にデプロイして運用する人。以下、運用者）です。

Action が失敗すると、ステップが失敗で終わり、ワークフロー実行のページとジョブのログに `::error::` の **注釈**（Action が失敗した段階と原因を書いたメッセージ）が出ます。
失敗を調べる手がかりは次のとおりです。

- **注釈の先頭の文言**：失敗した段階を示す。文言と段階の対応は [Issue Token](../../.github/actions/issue-token/README.md) の「失敗の読み方」にある
- **`request_id`**：Lambda が応答に付ける ID。注釈が `GATES returned a failure response` か `the success response has no usable` で始まるときに入る。運用者はこれで、Lambda が要求ごとに CloudWatch Logs に残す **監査記録**（呼び出し元、要求、結果を書いた 1 行の JSON）を探せる
- **ワークフロー実行の ID と時刻**：`request_id` がない失敗で、運用者が AWS の記録と突き合わせる手がかり。ID はワークフロー実行のページの URL（`.../actions/runs/<ワークフロー実行の ID>`）にある。時刻は、Action が GATES を呼び出す直前にログに出す `Invoking the Lambda function at <時刻>` の行の時刻（UTC）を使う

ワークフロー利用者は、§1 で自分で直せる失敗かを確かめ、直せなければ §2 に従って運用者に伝えます。
運用者は、`request_id` があれば §3 で監査記録を探して §4 で原因を絞り込み、`request_id` がなければ §5 を引きます。
`revocation failed:` の警告が出たときは、§6 を引きます。

## 1. Failures the Workflow User Can Fix

次の失敗は、ワークフローの書き方か Action への入力に原因があり、ワークフロー利用者が直せます。
**許可オーナー** は、GATES がトークンを発行してよいと運用者が設定した GitHub のユーザーアカウントか Organization です。
**インストール対象** は、GitHub App をオーナーにインストールするときに選ぶ、GitHub App がアクセスできるリポジトリです。
**OIDC JWT** は、ワークフローがどのオーナーのどのリポジトリで動いているかを GitHub が署名して示すトークンで、Action はこれで自分の身元を AWS と GATES に示します。

| 症状 | 原因 | 直し方 |
|---|---|---|
| 注釈が `ACTIONS_ID_TOKEN_REQUEST_TOKEN is not set` で始まる | Action を置いたジョブが `id-token: write` の権限を持たない。ジョブに `permissions` を書くと、ワークフローの先頭の `permissions` はそのジョブに使われない | Action を置いたジョブの `permissions` に `id-token: write` を書く |
| 注釈が `input <入力名> is empty` で始まる | Action の入力が空である。`function-name`、`aws-region`、`audience` が空なら、参照した Variable がないか、ワークフローを置くリポジトリから参照できない | 入力名と、参照する Secret と Variable の名前の綴りを見直す。綴りが正しいのに空なら、運用者に登録と参照の範囲を確かめてもらう |
| 注釈が `invalid inputs` で始まる | `repositories` か `permissions` の書式の誤り（空の要素、`/` や `:` の数、重複など） | 注釈の次の行に出る要素と理由に従って直す。書式は Issue Token の「入力」にある |
| 注釈が `could not get the OIDC JWT for GATES` で始まり、`HTTP status 400` を含む | `audience` が、GitHub が OIDC JWT の発行を拒否する値である（呼び出し元以外のオーナーやリポジトリを指す `https://github.com/` の URL など） | `audience` に、運用者が登録した Variable を渡す |
| 注釈が `could not get the OIDC JWT for AWS STS` か `could not get the OIDC JWT for GATES` で始まり、`HTTP status 400` を含まない | Runner から OIDC JWT の取得先に届かないか、GitHub の一時的な障害である（検証環境では確かめていない） | 再実行する。セルフホストの Runner なら、環境変数 `ACTIONS_ID_TOKEN_REQUEST_URL` が示す宛先への通信を許可する |
| 注釈が `AssumeRoleWithWebIdentity failed` で始まり、`AWS error code "AccessDenied"` を含む | `role-arn` が GATES の IAM Role を指していない。ワークフローを置くリポジトリが許可オーナーのものでない。リポジトリの OIDC JWT の `sub` が、不変の ID を含む形式でない | `role-arn` が `arn:aws:iam::${{ secrets.GATES_AWS_ACCOUNT_ID }}:role/${{ vars.GATES_ROLE_NAME }}` の形かを見直す。`sub` の形式は下記のコマンドで確かめる |
| 注釈が `AssumeRoleWithWebIdentity failed` で始まり、Dependabot が起動したワークフローでだけ起きる | Dependabot が起動したワークフローは GitHub Actions の Secrets を参照できず、`role-arn` の AWS アカウント ID が空になる | 運用者に、`GATES_AWS_ACCOUNT_ID` を Dependabot の Secrets にも登録してもらう |
| 注釈が `the Lambda Invoke failed` で始まり、`AWS error code "AccessDeniedException"` を含む | `function-name` が GATES の関数名でないか、`aws-region` が GATES のリージョンでない | `function-name` と `aws-region` に、運用者が登録した Variables を渡す |
| 注釈が `GATES returned a failure response` で始まり、`error code "invalid_request"` を含む | 要求が GATES の受け付ける形でない。たとえば、名前に使えない文字がある、`repositories` の要素が 500 を超える、`permissions` の権限名に大文字がある | `repositories` は `owner/repo` の形で、オーナー名は英数字とハイフン、リポジトリ名は 100 文字以下の英数字、ハイフン、アンダースコア、ピリオドで書く。`permissions` は `contents:read` と書く |
| 注釈が `GATES returned a failure response` で始まり、`error code "invalid_id_token"` を含む | `audience` が、GATES に設定した Audience（OIDC JWT の宛先を示す値）と一致しない | `audience` に、運用者が登録した Variable を渡す。渡しているなら §2 に進む |
| 注釈が `GATES returned a failure response` で始まり、`error code "authorization_denied"` を含む | `repositories` に許可オーナー以外のオーナーのリポジトリがあるか、`permissions` が `contents:read` 以外である | 許可オーナーのリポジトリと `contents:read` だけを要求する。一つでも許可されない要素があると、要求全体が失敗する |
| 注釈が `GATES returned a failure response` で始まり、`error code "github_issuance_rejected"` を含む | `repositories` に、インストール対象にないリポジトリか、存在しないリポジトリがある | リポジトリ名の綴りを確かめる。綴りが正しければ、インストール対象に加えるよう **GitHub App とオーナーの管理者**（GitHub App のインストール先と、オーナーの GitHub の設定を管理する人）に頼むか、要求から外す |
| ジョブの `outputs` に置いたトークンが、後続のジョブで空になる | GitHub は、マスクされた値を含むジョブの Output を設定しない。ジョブのログに `Skip output ... since it may contain secret` の警告が出る | トークンが要るジョブごとに、Action のステップを置く |
| トークンで呼んだ GitHub の API が `403` か `404` を返す | `403` は、`contents:read` 以外の権限が要る操作である。`404` は、要求に含めなかったプライベートリポジトリである | `403` の操作は、GATES のトークンではできない。`404` のリポジトリは `repositories` に加える |

表の原因を検証環境で確かめたかどうかは、次のとおりです。

- `AccessDenied` は、存在しない Role 名と、`sub` が名前だけの形式のリポジトリ（[AWS STS Legacy Subject](../research/aws/sts-legacy-subject.md)）で確かめた。AWS STS は、IAM Role の引き受けの条件（**Trust Policy**）のどれを満たさなかったかを示さないため、注釈からは原因を一つに絞れない
- `AccessDeniedException` は、存在しない関数名で確かめた。存在しない関数でも `ResourceNotFoundException` にはならない（[AWS Lambda Invoke Rejection](../research/aws/lambda-invoke-rejection.md)）。リージョンの誤りは、同じ注釈になる見込みだが確かめていない
- `invalid_id_token` は Audience の食い違いで、`authorization_denied` は許可オーナー以外のリポジトリと `contents:write` などの権限で、`github_issuance_rejected` はインストール対象にないリポジトリで確かめた。存在しないリポジトリは、GitHub の発行 API が `422` で拒否することを確かめた（[GitHub Constraints](../spec/github-constraints.md) の §6.2）。`422` は `github_issuance_rejected` に分類されるが、GATES を通した結果は確かめていない
- Dependabot の行は、GitHub の公式資料からの推論である。注釈に入る AWS のエラーコードは確かめていない（[GitHub Constraints](../spec/github-constraints.md) の §7）
- ジョブの `outputs` とトークンの `403` と `404` は確かめた

ワークフローを置くリポジトリの `sub` の形式は、次のコマンドで確かめます。
GATES が受け付けるのは、`sub` が `repo:<オーナー名>@<Owner ID>/<リポジトリ名>@<Repository ID>:...` のように不変の ID を含む形式だけです。

```bash
gh api repos/<オーナー名>/<リポジトリ名>/actions/oidc/customization/sub
```

出力の `use_immutable_subject` が `false` で、`sub_claim_prefix` に `@` がなければ、名前だけの形式です。
切り替えはリポジトリか Organization の設定で行うため、GitHub App とオーナーの管理者に頼みます（[GitHub Constraints](../spec/github-constraints.md) の §3.2）。

fork からの Pull Request で起動した `pull_request` のジョブは、GitHub の公式資料によれば既定では `id-token: write` を持たないため、`ACTIONS_ID_TOKEN_REQUEST_TOKEN is not set` の注釈で失敗する見込みです（検証環境では確かめていない）。
そのジョブには Pull Request のコードがあるため、トークンを受け取らせません。
トークンを受け取るジョブは別の起動イベントで動かし、そのジョブでも Pull Request のコードや、ほかのジョブやワークフローの成果物を実行しません。

## 2. Reporting a Failure to the Operator

§1 のどれにも当たらない失敗と、§1 の直し方で解消しない失敗は、ワークフロー利用者の側では直せません。
GATES の設定、GitHub App のインストール、AWS のリソースのように、運用者か GitHub App とオーナーの管理者が管理するものに原因があるためです。
たとえば、注釈が `the Lambda function returned FunctionError`、`the Lambda response`、`the success response has no usable` で始まる失敗や、`error code "installation_unavailable"`、`"issuance_verification_failed"`、`"internal_error"` を含む失敗がこれに当たります。
注釈が `the job was canceled` で始まるときは、ジョブがキャンセルされただけで、伝える必要はありません。

注釈の `message` の値が `Retry later.` で終わる失敗は、GATES が依存する GitHub か AWS の一時的な障害です（`identity_verification_unavailable`、`signing_unavailable`、`github_api_unavailable`）。
ワークフローを再実行すると解消することがあるため、まず再実行し、繰り返し失敗するときに運用者に伝えます。

運用者には、次の情報を伝えます。
注釈はトークンや AWS の認証情報を含まないため、全文をそのまま渡せます。

- 注釈の全文
- 注釈に入っていれば、`request_id`
- ワークフロー実行の URL。再実行した場合は、何回目の実行で失敗したか
- 失敗した時刻（ジョブのログの `Invoking the Lambda function at` の行の時刻。その行がなければ、注釈の行の時刻）

## 3. Finding the Audit Record

§3 から §5 は運用者の作業です。
コマンドは、GATES を適用した AWS アカウントの、ログと設定を読める一時認証情報で実行します。
AWS CLI のリージョン（`AWS_REGION`）は、GATES を適用したリージョンにしておきます。

監査記録は、ロググループ `/aws/lambda/gates` に、`msg` が `audit record` の 1 行の JSON として残ります。
関数が応答を返した要求は、成功でも失敗でも監査記録を 1 件残します。
IAM が拒否した呼び出しは Lambda に届かないため、監査記録を残しません。
関数が応答を書けずに終わった呼び出しは、残らないか、残っても結果が呼び出し元に届いていないことがあります（どちらも §5）。
監査記録の保持期間は Terraform の `log_retention_in_days` で決まり、それを過ぎた記録は探せません。

`request_id` で、その要求の監査記録を引きます。
フィルターは JSON の行の `request_id` の項目だけに一致させ、同じ ID を含む Lambda の `START`、`END`、`REPORT` の行を除きます。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --filter-pattern '{ $.request_id = "<request_id>" }' \
  --query 'events[].message' --output json \
  | jq '.[] | fromjson | select(.msg == "audit record") | del(.claims_set)'
```

```json
{
  "time": "<時刻>",
  "level": "INFO",
  "msg": "audit record",
  "request_id": "<request_id>",
  "result": "authorization_denied",
  "reason": "<部品のエラー文言>",
  "caller": {"owner_id": <Owner ID>, "repository_id": <Repository ID>, "run_id": <ワークフロー実行の ID>, "run_attempt": <試行の番号>, "...": "..."},
  "request": {"repositories": ["<要求したリポジトリ>"], "permissions": {"contents": "write"}}
}
```

注釈が `GATES returned a failure response` で始まる失敗で、ワークフロー利用者から URL だけを受け取ったときは、ワークフロー実行の ID で引きます。
`caller` は呼び出し元の認証に成功した後にだけ記録されるため、認証の前や途中で失敗した要求（`invalid_request`、`invalid_id_token`、`identity_verification_unavailable` など）はこの方法では見つかりません。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --filter-pattern '{ $.caller.run_id = <ワークフロー実行の ID> }' \
  --query 'events[].message' --output json \
  | jq -c '.[] | fromjson | {time, request_id, result, reason, run_attempt: .caller.run_attempt}'
```

監査記録のうち、原因を絞り込むのに使う項目は次のとおりです。
すべての項目は [Lambda Design](../design/lambda.md) の §7 にあります。

- **`result`**：成功なら `success`、失敗ならエラーコード。§4 でエラーコードごとの原因を引く
- **`reason`**：失敗した部品のエラー文言。満たさなかった条件や、失敗した外部の呼び出しの段階とステータスが入る。拒否の理由を許可されない呼び出し元に探られないよう、`reason` は応答にも注釈にも出さず、監査記録にだけ残す
- **`caller`**：呼び出し元のリポジトリとワークフロー実行。`caller.run_id` がワークフロー利用者の伝えた ID と一致することを確かめる
- **`request`**：要求したリポジトリと権限。要求の表記と並びのまま記録される
- **`installation`**：GATES が取得した Installation（GitHub App をオーナーにインストールした単位）。取得まで進んだ要求にだけある
- **`revocation`**：GitHub が発行したトークンを GATES の照合で拒否したとき（`issuance_verification_failed`）の失効の結果

## 4. Causes and Fixes by Error Code

エラーコードの定義は [Lambda Design](../design/lambda.md) の §3.2 にあります。
表の原因には、検証環境で確かめたかどうかを添えます。
**使用中の鍵** は、Lambda が署名に使う KMS Key で、`terraform.tfvars` の `active_kms_key` で選びます。

| エラーコード | 原因 | 確かめることと直し方 |
|---|---|---|
| `invalid_request` | 要求が GATES の形式検証に合わない。名前の文字種、500 を超える要求の数、権限名の文字種など、Action が検証しない形の誤りを確かめた。Action と GATES のリリースが違い、GATES の知らない形の要求を Action が送ることも考えられる（確かめていない） | `reason` の、不正なフィールドの名前と形を見る。ワークフロー利用者に、要求の直し方（§1）を伝える。リリースが違うなら、Action の参照先を、デプロイした GATES と同じリリースのコミットにしてもらう |
| `invalid_id_token` | GATES 用の OIDC JWT が受理条件を満たさない。Audience の食い違い、期限切れ、署名の改ざんを確かめた | `reason` からは、満たさなかった受理条件（Audience、期限、署名など）を区別できない。Audience の食い違いなら、ワークフローが参照する Variable の `GATES_AUDIENCE` を、`terraform.tfvars` の `gates_audience` と同じ値に直す |
| `authorization_denied` | 許可オーナー以外のオーナーのリポジトリか、`contents:read` 以外の権限を要求した。どちらも確かめた | `reason` の `owner name does not match allowed owner name` か `permissions are not equal to allowed permissions` と、`request` の値を見て、ワークフロー利用者に要求の直し方（§1）を伝える |
| `identity_verification_unavailable` | GitHub の OIDC の公開鍵の取得（OIDC Discovery と JWKS）が一時的に失敗した。関数のタイムアウトで取得が打ち切られた場合にも、このコードの監査記録が残ることを確かめた（§5.3） | 再実行で解消し得る。繰り返すなら、`reason` と [GitHub Status](https://www.githubstatus.com/) を見て、GitHub の障害か、Lambda から `token.actions.githubusercontent.com` への通信を確かめる |
| `signing_unavailable` | AWS KMS の署名が、通信の失敗、タイムアウト、5xx、スロットリングなどで一時的に失敗した（確かめていない） | 再実行で解消し得る。`reason` の KMS のエラーコードが `ThrottlingException` で繰り返すなら、KMS の要求数のクォータを確かめる |
| `installation_unavailable` | GATES が許可オーナーの Installation を発行に使えない。GitHub App のアンインストール（`reason` に `status 404`）と、Installation の停止（`installation is suspended`）を、ユーザーアカウントの許可オーナーで確かめた。`github_app_client_id` の誤り、使用中の鍵の鍵素材と GitHub App に登録した Private Key の不一致、GitHub App からの Private Key の削除、GitHub の API バージョンのサポート終了（`status 410`）も、このコードになる（確かめていない） | `reason` の段階（`resolve installation` は Installation の取得、`issue` は発行）と、GitHub の応答のステータスを見る。アンインストールなら、GitHub App を `Only select repositories` で再インストールする。GATES の設定は変えなくてよい。停止なら、悪用への対処のために意図して停止したものでないことを確かめてから、停止した側が停止を解く。Client ID は GitHub App の設定ページの値と比べる。鍵は下記のコマンドで照合する |
| `github_api_unavailable` | GitHub API が、通信の失敗、タイムアウト、5xx、レート制限で一時的に失敗した（確かめていない） | 再実行で解消し得る。繰り返すなら、`reason` のステータスと GitHub Status を見る |
| `github_issuance_rejected` | GitHub の発行 API が `422` で拒否した。インストール対象にないリポジトリの要求を確かめた。`422` は GitHub の検証の失敗やスパムの判定も含むため、ステータスからは原因を特定できない | `request.repositories` を、GitHub App の Installation の画面のリポジトリと比べる。足りなければインストール対象に加える。変更は、保存から約 1 分後の要求で反映されていた。インストール対象に入っているのに拒否されるなら、`reason` の GitHub の request ID を控え、GitHub に問い合わせる |
| `issuance_verification_failed` | GitHub が発行したトークンの対象、権限、応答の形が、GATES の照合に合わない（確かめていない） | `reason` の満たさなかった条件と GitHub の request ID を控え、GATES の開発者に報告する。`revocation` が `failed` なら、そのトークンは有効期限（発行から 1 時間）まで残り得る |
| `internal_error` | どの種別にも当たらない失敗。使用中の KMS Key の無効化（`reason` に `kms sign failed: code "DisabledException"`）を確かめた。鍵素材のインポート前、鍵の削除の予約、Lambda の権限の不足、Lambda の panic も、このコードになる（確かめていない） | `reason` を見る。KMS の失敗なら、下記のコマンドで使用中の鍵の状態を確かめる。`Disabled` なら、悪用への対処や鍵の更新のために意図して無効にしたものでないことを確かめてから有効にする。`PendingDeletion` なら、削除の予約を取り消してから有効にする。`PendingImport` なら、鍵素材をインポートする。panic なら、同じ `request_id` の、`msg` が `panic recovered` の行にスタックトレースが残る |

検証環境での観測は、[GitHub App Installation Changes](../research/github/installation-changes.md)、[AWS Fault Injection](../research/aws/fault-injection.md)、E2E テストの失敗の事例によります。

`internal_error` で KMS を疑うときは、使用中の鍵の ARN を関数の設定から読み、その状態を確かめます。
状態が `Enabled` でなければ、署名に使えません。

```bash
aws lambda get-function --function-name gates \
  --query 'Configuration.Environment.Variables.GATES_KMS_KEY_ARN' --output text
aws kms describe-key --key-id <KMS Key の ARN> --query 'KeyMetadata.KeyState' --output text
```

`installation_unavailable` で鍵の不一致を疑うときは、使用中の鍵の公開鍵のフィンガープリントを計算し、GitHub App の設定ページの `Credentials` → `Key pairs` に表示される鍵の、`SHA256:` の後の値と比べます。
一致する鍵がなければ、GitHub App は使用中の鍵の署名を受け付けません。

```bash
aws kms get-public-key --key-id <KMS Key の ARN> --query PublicKey --output text \
  | openssl base64 -d -A | openssl sha256 -binary | openssl base64
```

KMS Key にインポートした鍵素材は、別の鍵素材に入れ替えられません。
GitHub App の鍵と一致しないときは、GitHub App で新しい Private Key を生成し、新しい KMS Key にインポートして、使用中の鍵をその KMS Key に切り替えます（GitHub App Private Key の鍵の更新と同じ作業）。

## 5. Failures without a request_id

注釈が `AssumeRoleWithWebIdentity failed`、`the Lambda Invoke failed`、`the Lambda Invoke returned StatusCode`、`the Lambda function returned FunctionError` で始まる失敗では、`request_id` がありません。
要求が Lambda に届かなかったか、Lambda が応答を書けなかったためです。
運用者は、ワークフロー利用者から受け取ったワークフロー実行の ID と時刻で、AWS の記録を探します。

ほかに `request_id` がない注釈には、Runner の環境（`ACTIONS_ID_TOKEN_REQUEST_URL is not set`、`invalid ACTIONS_ID_TOKEN_REQUEST_URL`、`could not write to GITHUB_OUTPUT` など）と、Action の想定外の失敗（`the child process`、`an unexpected failure occurred`）があります。
これらは AWS の記録に手がかりがないため、注釈の全文とワークフロー実行の URL を添えて、Runner の管理者か GATES の開発者に伝えます。
`the Lambda response` で始まる注釈は、Action が GATES の応答を読めなかったことを示し、Action と GATES のリリースの食い違いが考えられます（確かめていない）。
`the success response has no usable` で始まる注釈も同じで、こちらは `request_id` を含み、§3 で引いた監査記録の `result` は `success` になります。
Action の参照先が、デプロイした GATES と同じリリースのコミットかを確かめます。

### 5.1 IAM Role Assumption

注釈が `AssumeRoleWithWebIdentity failed` で始まる失敗は、Action が IAM Role を引き受けられなかったことを示します。
ワークフロー利用者の側の原因（§1）のほかに、運用者の側には次の原因が考えられます。

- AWS アカウントに IAM OIDC Provider がないか、その Client ID に `sts.amazonaws.com` がない（確かめていない）
- `terraform.tfvars` の `allowed_owner_id` か `allowed_owner_name` が、許可オーナーの値と違う。`gh api users/<オーナー名> --jq '[.id, .login] | @tsv'` の出力と比べる（確かめていない）
- 許可オーナーの名前が変わり、Trust Policy の名前が古い。`allowed_owner_name` を新しい名前にして適用し直すまで、引き受けは拒否される（設計から導いた。確かめていない）

引き受けは CloudTrail の管理イベントで、拒否された引き受けも既定で記録されます。
検証環境では、Action が AWS STS を呼び出したリージョン（ワークフロー利用者が `aws-region` に渡したリージョン）のイベント履歴に残りました。
ほかのリージョンを渡した場合も、そのリージョンに残る見込みです（確かめていない）。
ワークフローが失敗した時刻の前後で、引き受けの記録を引きます。
`<開始>` と `<終了>` は、`YYYY-MM-DDThh:mm:ssZ` の形の ISO 8601 の時刻です。

```bash
aws cloudtrail lookup-events --region <リージョン> \
  --lookup-attributes AttributeKey=EventName,AttributeValue=AssumeRoleWithWebIdentity \
  --start-time <開始> --end-time <終了> \
  --query 'Events[].CloudTrailEvent' --output json \
  | jq -r '.[] | fromjson | [.eventTime, (.errorCode // "success"), .userIdentity.userName] | @tsv'
```

`userIdentity.userName` には、OIDC JWT の `sub` が入ります。
`sub` の `repo:` の後に `@` と Owner ID がなければ、名前だけの形式です。
オーナー名と Owner ID が許可オーナーのものでなければ、許可オーナー以外のリポジトリからの呼び出しです。
`sub` が正しい形式で許可オーナーのものなのに `AccessDenied` なら、Trust Policy のオーナー名か、`role-arn` を確かめます。
拒否の記録は、Trust Policy のどの条件で拒否したかを示しません（[AWS STS Legacy Subject](../research/aws/sts-legacy-subject.md)）。

その時刻に記録がないときは、`role-arn` が別の AWS アカウントを指しているか、`aws-region` が誤っていて AWS STS に届いていない可能性があります（確かめていない）。

### 5.2 Lambda Invocation

注釈が `the Lambda Invoke failed` か `the Lambda Invoke returned StatusCode` で始まる失敗は、AWS Lambda の Invoke API が要求を関数に渡さなかったことを示します。
関数は実行されないため、関数のロググループに行は残りません。
AWS Lambda の Invoke は CloudTrail のデータイベントで、データイベントの記録を有効にしていなければ、CloudTrail にも残りません（[AWS Lambda Invoke Rejection](../research/aws/lambda-invoke-rejection.md)）。
そのため、注釈の AWS のエラーコードと、関数の設定で原因を絞り込みます。

| AWS のエラーコード | 原因 | 確かめることと直し方 |
|---|---|---|
| `AccessDeniedException` | `function-name` か `aws-region` の誤り（§1）。存在しない関数でも、このコードになる | ワークフロー利用者に、運用者が登録した Variables を渡しているかを確かめてもらう |
| `TooManyRequestsException` | 関数の同時実行数の上限を超えた。`lambda_reserved_concurrency` が `0` の場合を確かめた（[AWS Fault Injection](../research/aws/fault-injection.md)）。予約した数を超える要求が重なった場合も、このコードになる見込みである（確かめていない） | 下記のコマンドで、関数の予約した同時実行数を確かめる。`0` なら、悪用への対処のために意図して `0` にしたものでないことを確かめてから、`terraform.tfvars` の `lambda_reserved_concurrency` を見直して適用し直す |

```bash
aws lambda get-function-concurrency --function-name gates
```

### 5.3 Unexpected Lambda Failures

注釈が `the Lambda function returned FunctionError` で始まる失敗は、関数が実行されたものの、応答を書けずに終わったことを示します。
注釈の `errorType` で、原因を見分けます。

| `errorType` | 原因 | 確かめることと直し方 |
|---|---|---|
| `Runtime.ExitError` | 関数の初期化に失敗した。関数の設定の環境変数が欠けた場合を確かめた（[AWS Fault Injection](../research/aws/fault-injection.md)） | ロググループの、`msg` が `initialization failed` の行の `error` に、欠けた設定の名前が出る。Terraform の外で関数の設定が変えられていれば、`make tf-plan` に差分が出るため、確かめてから適用し直す |
| `Sandbox.Timedout` | 関数がタイムアウトした。関数のタイムアウトを短くした場合を確かめた（同上） | 失敗の時刻のロググループの `REPORT` の行に `Status: timeout` が出る。関数のタイムアウトが Terraform の値（45 秒）から変わっていれば、適用し直す |

タイムアウトでは、関数が監査記録を書いた直後に打ち切られることがあり、そのときの監査記録の `result` は、関数の中で判定した失敗の種別で、呼び出し元には届いていません。
観測した例では、GitHub の OIDC Discovery の取得の途中で打ち切られたため、`caller` がなく、`result` は `identity_verification_unavailable` でした。
トークンを発行した後に打ち切られると、GitHub 側で発行されたトークンを Action が受け取れず、失効も試みられないまま有効期限まで残り得ます。
メモリ不足のような、表にない `errorType` は確かめていません。
その場合も、時刻で引いた `REPORT` の行の `Status` と `Error Type` を見ます。

`request_id` がないため、`Invoking the Lambda function at` の行の時刻の前後の行を、時刻で引きます。
`<開始>` と `<終了>` は、ミリ秒の UNIX 時刻です。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/gates \
  --start-time <開始> --end-time <終了> \
  --query 'events[].message' --output text
```

## 6. Revocation Warning

Action は、ジョブの終了時の `Post <ステップ名>` のステップで、トークンの失効を GitHub に要求します。
失効に成功すると、ログに `revocation succeeded: HTTP status 204` が出ます。
失効に失敗すると、`revocation failed:` で始まる `::warning::` の注釈が出ます。
失効の処理そのものが想定外に終わった場合は、Issue Token の「失敗の読み方」にある Action の想定外の失敗の文言が、`::warning::` で出ます。
Action がトークンを受け取った後、後続のステップに渡す前に失敗したときも、Action のステップの中で失効を試み、同じ警告が出ることがあります。
この場合は、同じステップの `::error::` の注釈から §1 以降を引きます。

この警告では、ジョブは失敗しません。
失効できなかったトークンも、Action の出力の `expires-at`（発行から 1 時間）を過ぎれば使えなくなります。
残ったトークンの対象と権限は、要求したリポジトリと `contents:read` に限られます。

セルフホストの Runner では、Runner から `api.github.com` への通信が許可されているかを確かめます。
GitHub がホストする Runner で警告が繰り返し出るときは、注釈の全文とワークフロー実行の URL を運用者に伝えます。

## 7. References

- [Issue Token](../../.github/actions/issue-token/README.md)
- [Lambda Design](../design/lambda.md)
- [GitHub Constraints](../spec/github-constraints.md)
- [GitHub App Installation Changes](../research/github/installation-changes.md)
- [AWS Fault Injection](../research/aws/fault-injection.md)
- [AWS STS Legacy Subject](../research/aws/sts-legacy-subject.md)
- [AWS Lambda Invoke Rejection](../research/aws/lambda-invoke-rejection.md)
- [AWS — Logging IAM and AWS STS API calls with AWS CloudTrail](https://docs.aws.amazon.com/IAM/latest/UserGuide/cloudtrail-integration.html)
- [AWS — Filter pattern syntax for metric filters, subscription filters, filter log events, and Live Tail](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/FilterAndPatternSyntax.html)
