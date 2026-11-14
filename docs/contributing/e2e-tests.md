# E2E Tests

デプロイした GATES を Docker Action から呼び、発行、後続ステップでの利用、失効、失敗時の挙動、資格情報の非出力を確かめる E2E テストです。
Action の入力の検証が止める Payload を GATES がどう拒否するかは、Lambda を直接 Invoke する E2E テストで確かめます。
GitHub と AWS の挙動に依存する事項は、許可オーナーのリポジトリのワークフローでしか確かめられません（[GitHub Actions Design](../design/github-actions.md#62-tests-on-github-actions)）。
E2E テストのワークフローは本リポジトリの `.github/workflows/` に置き、試験用のリポジトリに写して実行します。
本リポジトリには接続情報を登録しないため、ワークフローは `workflow_dispatch` だけで起動するようにしています。

## Workflows

| ファイル | 確かめること |
| --- | --- |
| [_e2e-success.yml](../../.github/workflows/_e2e-success.yml) | 対象の件数、並び、表記、区切りの空白を変えた発行、トークンから見える対象と拒否される操作、Output の形、後続ステップの環境に AWS 一時認証情報がないこと、post-entrypoint が失効させたトークンが `401` で拒否されること |
| [_e2e-interval.yml](../../.github/workflows/_e2e-interval.yml) | 温まった Lambda の実行環境に、直前の Invoke から 1 分前後空けた Invoke を 5 回続け、すべて成功すること |
| [_e2e-failure.yml](../../.github/workflows/_e2e-failure.yml) | 成功する設定から 1 つの条件だけを変えた各事例で、Action が段階を示す注釈を書いて失敗し、Output を書かず、post-entrypoint が失効を試みずに終わること |
| [_e2e-payload-rejection.yml](../../.github/workflows/_e2e-payload-rejection.yml) | Action を使わずに Lambda を直接 Invoke し、成功する Payload から 1 つの条件だけを変えた各事例を、GATES が定めたエラーコードと固定の文言で拒否し、トークンを返さないこと |
| [_e2e-cancel.yml](../../.github/workflows/_e2e-cancel.yml) | 後続ステップが失敗したジョブと、後続ステップの実行中にキャンセルしたジョブで、post-entrypoint がトークンを失効させること。メインの処理の最中に SIGINT を受けた Action が、受けた時点に応じて終えること |
| [_e2e-reusable-credential-leak.yml](../../.github/workflows/_e2e-reusable-credential-leak.yml) | ほかのワークフローから呼ばれ、同じワークフロー実行の完了したジョブのログ全体に、資格情報の文字列がないこと |

ログの検索は、資格情報が漏れていないことを保証しません。
資格情報を出さない仕組みの本体は Action のマスクと出力の設計で、検索はそれが崩れたときに気づくための確認です。

Action を使うワークフローは、Action を commit SHA で参照します。
Action を変えたときは、各ファイルの `uses:` の SHA を、試す commit に変えます。

Lambda を呼ぶ E2E のワークフローは、どれも同じ `concurrency` のグループに入り、同時に動きません。
間隔の試験は、ほかの Invoke が同じ実行環境に割り込むと間隔が崩れるためです。

### Success Cases

`_e2e-success.yml` の各事例は、`repositories` の入力だけを変えます。

| 事例 | `repositories` の入力 |
| --- | --- |
| `single` | 1 つ目の対象 |
| `multiple` | 1 つ目と 2 つ目の対象 |
| `reversed` | 2 つ目と 1 つ目の対象 |
| `letter-case` | 1 つ目の対象のオーナー名とリポジトリ名を大文字にしたもの |
| `whitespace` | 1 つ目と 2 つ目の対象。要素の前後に改行、スペース、タブを置く |

大文字にした要求が通るのは、GATES が名前を ASCII の範囲で大文字小文字を区別せずに比べ（[Lambda Design](../design/lambda.md#名前の文字種と比較規則)）、GitHub の発行 API も大文字小文字の違う名前を受け付けるためです（[GitHub Installation Access Token](../research/github/installation-access-token.md#名前の大文字小文字を変えた発行)）。
区切りは `,` だけで、改行は区切りになりません。
前後の空白は Action が除きます（[GitHub Actions Design](../design/github-actions.md#22-list-syntax)）。

どの事例でも、発行したトークンで次を確かめます。
書き込みの試験のパスと ref の名前は実行ごとに一意にし、想定に反して作られた場合はジョブを失敗させて、パスか ref を注釈に示します。
作られたものは運用者が消します。

| 確認 | 方法 | 期待する結果 |
| --- | --- | --- |
| トークンから見える対象 | `GET /installation/repositories` | `total_count` が要求した件数で、`full_name` の集合が GitHub 上の表記の要求した対象と一致する |
| 内容の読み取り | 要求した各対象の `README.md` の取得 | `200` |
| 内容の書き込み | 要求した各対象への `PUT /repos/{owner}/{repo}/contents/{path}` | `403` |
| ref の作成 | 要求した各対象への `POST /repos/{owner}/{repo}/git/refs` | `403` |
| 別の権限の読み取り | 要求した各対象の `GET /repos/{owner}/{repo}/pulls` | `403`。対象が非公開であることを前提とする |
| 要求していない対象 | 対象 1 件の事例では Installation のもう 1 つの対象、どの事例でも試験用のリポジトリ自身の `README.md` の取得 | `404` |
| Installation の対象外の公開リポジトリ | `GATES_PUBLIC_UNINSTALLED` の `README.md` の取得 | `200`。トークンの対象ではないが、公開の内容として読める |
| Output | `repositories` と `permissions` | `repositories` の集合が GitHub 上の表記の要求した対象と一致し、`permissions` が `contents:read,metadata:read` |

`403` の応答の `X-Accepted-GitHub-Permissions` のヘッダーも、ログに出します。
Output の `repositories` は GitHub の応答の並びのため、並びは判定せずに出すだけにしています。

### Revoked Token

`_e2e-success.yml` の `Revoked token` のジョブは、post-entrypoint が失効させた後のトークンが使えないことを確かめます。
post-entrypoint はジョブの最後に動くため、後続ステップからは失効の後のトークンを試せません。
そこで、Docker Action のステップより前に、post を持つ試験用の JavaScript の Action（[_e2e-revoked-token](../../.github/actions/_e2e-revoked-token/action.yml)）を置きます。
Runner は post をメインの処理と逆の順で動かすため、この Action の post は Docker Action の post-entrypoint の後に動きます（[GitHub Actions Design](../design/github-actions.md#62-tests-on-github-actions)）。

1. 試験用の Action のメインの処理が、`RUNNER_TEMP` の下に、ほかのユーザーが読めないディレクトリを作る
2. Docker Action がトークンを発行する
3. 後続ステップが、トークンで要求した対象の `README.md` を読めること（`200`）を確かめ、`umask 077` でトークンをそのディレクトリのファイルに書く
4. post-entrypoint がトークンを失効させる
5. 試験用の Action の post が、ファイルのトークンで同じ `README.md` を読み、ディレクトリを成否によらず削除する

試験用の Action は、依存を持たず、Node.js の標準の `https` だけで GitHub API を呼びます。
トークンの値はログに出しません。

`Check revocation order` のジョブは、`Revoked token` のジョブのステップとログから次を確かめます。

| 確認 | 期待する結果 |
| --- | --- |
| post の順序 | `Post Issue token` のステップの番号が、`Post Check revoked token` より小さく、どちらも `success` |
| 失効 | `revocation succeeded: HTTP status 204` が 1 行あり、401 の行より前にある |
| 失効の後の利用 | `revoked token read README.md: HTTP status 401` が 1 行ある。`200` なら試験用の Action の post が失敗する。post の順序が想定と違うか、失効が効いていないかを、post の順序の行と失効の行で切り分ける |
| ファイルの削除 | `token file removed` が 1 行ある |

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

KMS Key の無効化による `internal_error`、関数の初期化の失敗とタイムアウトによる `FunctionError`、同時実行数 0 による Invoke API の拒否は、検証環境の設定を手で変えて戻す操作を要するため、繰り返し実行する E2E テストに含めません。
検証環境で一度ずつ起こした結果は、[AWS Fault Injection](../research/aws/fault-injection.md) に記録しています。
次の障害は、検証環境でも起こさず、Lambda のローカルの試験だけで確かめます。
どれも本番のコードに試験のための分岐や設定を加えれば起こせますが、その経路は本番で発行結果の照合や失敗の扱いを迂回する経路になり得るため、加えません。
表の受け入れ基準は [Requirements](../spec/requirements.md) の AC で、ローカルの試験はそれを `trace:checks` で宣言しています（[Documentation Policy](docs-policy.md)）。

| 障害 | 検証環境で起こさない理由 | 受け入れ基準 |
| --- | --- | --- |
| 発行結果の照合失敗と、その後の失効 | GitHub に、要求と異なる発行結果を返させる手段がない | AC-12 |
| 発行の要求の後の応答の喪失 | Lambda と GitHub の間の通信だけを切る手段がない | AC-30 |
| OIDC 公開鍵の取得の障害 | Lambda の外向きの通信を、宛先を選んで止める構成がない | AC-13、AC-28 |
| KMS の一時的な障害（スロットリング、`5xx`） | AWS 側で起きる事象で、意図して起こせない | AC-13 |
| GitHub API の一時的な障害とレート制限 | 通信障害は起こす手段がない。レート制限は超えると、同じ GitHub App を使うほかの試験がしばらく止まる | AC-13、AC-17 |

照合失敗と応答の喪失の後に、トークンを返さず、取得していないトークンを失効させないことは、[Alloy Models](../../formal/alloy/README.md) の発行の時間モデルでも確かめています。

### Payload Rejection Cases

`_e2e-payload-rejection.yml` は Action を使わず、Runner の AWS CLI、`curl`、`jq` で進めます。
`sts.amazonaws.com` を Audience とする OIDC JWT で GitHub Actions 用 IAM Role を引き受け、GATES 用 Audience の OIDC JWT を取得してから、事例ごとに Payload を組み立てて `aws lambda invoke` します。
JWT、AWS 一時認証情報、Payload、応答は、`umask 077` で `RUNNER_TEMP` の下に作った作業ディレクトリのファイルだけに置き、`if: always()` のステップで削除します。
ログに出すのは、事例ごとの判定と `request_id` だけです。

各事例は、`FunctionError` がなく、応答が `ok`（`false`）、`error.code`、`error.message`、`request_id` だけを持ち、`error.code` が期待と一致し、`error.message` がそのコードの固定の文言（[Lambda Design](../design/lambda.md#32-failure)）であるときに期待どおりとします。
応答に `token` があれば、ワークフローがその場で失効させ、期待どおりとしません。
AWS CLI の再試行は止め、1 つの事例で 1 回だけ Invoke します。

| 事例 | 成功する Payload から変える条件 | 期待するエラーコード |
| --- | --- | --- |
| `qualified-arn` | `--qualifier 1` を付けて Invoke する | IAM の `AccessDeniedException` |
| `invalid-json` | JSON として不正な Payload にする | Invoke API の `InvalidRequestContentException` |
| `field-installation-id` | `installation_id` を加える | `invalid_request` |
| `field-caller` | `repository_owner_id` と `caller` を加える | `invalid_request` |
| `field-owner` | `owner` と `owner_id` を加える | `invalid_request` |
| `field-github-app` | `client_id` と `app_id` を加える | `invalid_request` |
| `field-kms-key` | `kms_key_arn` を加える | `invalid_request` |
| `field-api-host` | `api_host` を加える | `invalid_request` |
| `unknown-field` | `unknown` を加える | `invalid_request` |
| `repository-selection` | `repository_selection` を加える | `invalid_request` |
| `duplicate-key` | 先頭に `permissions` の `{"contents":"write"}` を重ねる | `invalid_request` |
| `trailing-data` | オブジェクトの後に `{}` を続ける | `invalid_request` |
| `missing-id-token` | `id_token` を除く | `invalid_request` |
| `repositories-missing` | `repositories` を除く | `invalid_request` |
| `repositories-empty` | `repositories` を `[]` にする | `invalid_request` |
| `repositories-null` | `repositories` を `null` にする | `invalid_request` |
| `permissions-missing` | `permissions` を除く | `invalid_request` |
| `permissions-empty` | `permissions` を `{}` にする | `invalid_request` |
| `permission-unknown-name` | `permissions` を `{"issues":"read"}` にする | `authorization_denied` |
| `permission-unknown-level` | `permissions` を `{"contents":"admin"}` にする | `authorization_denied` |
| `permission-invalid-charset` | `permissions` を `{"Contents":"read"}` にする | `invalid_request` |
| `version-unknown` | `version` を `2` にする | `invalid_request` |
| `permission-added` | `permissions` に `metadata:read` を加える | `authorization_denied` |
| `permission-write` | `permissions` を `{"contents":"write"}` にする | `authorization_denied` |
| `other-owner` | `repositories` を許可オーナー以外のオーナーの公開リポジトリにする | `authorization_denied` |
| `partially-other-owner` | `repositories` に許可オーナー以外のオーナーの公開リポジトリを加える | `authorization_denied` |
| `too-many-repositories` | `repositories` を異なる名前の 501 件にする | `invalid_request` |
| `nested-name` | `repositories` を `<owner>/a/b` にする | `invalid_request` |
| `not-installed` | `repositories` を試験用のリポジトリ自身にする | `github_issuance_rejected` |
| `partially-not-installed` | `repositories` に試験用のリポジトリ自身を加える | `github_issuance_rejected` |
| `audience-mismatch` | `id_token` を `sts.amazonaws.com` の OIDC JWT にする | `invalid_id_token` |
| `tampered` | `id_token` の Payload の `jti` を 1 文字変え、署名を元のままにする | `invalid_id_token` |
| `expired` | `id_token` の `exp` から 15 秒過ぎるまで待つ | `invalid_id_token` |

`field-` で始まる事例のフィールドの名前は、Caller、許可オーナー、Installation ID、GitHub App、KMS Key、API Host を利用者が指定しようとする例です。
GATES はどの名前も未知のフィールドとして同じ経路で拒否するため、項目の種類ごとに 1 件ずつとしています。
許可オーナー以外のオーナーのリポジトリは、GATES がオーナー名の照合で GitHub API を呼ぶ前に拒否するため、公開か非公開か、実在するかで結果は変わりません。
期限切れの事例は `exp` まで 5 分ほど待つため、最後に置きます。

`qualified-arn` と `invalid-json` は関数に届く前に拒否され、GATES の応答も監査記録も残りません（[AWS Lambda Invoke Rejection](../research/aws/lambda-invoke-rejection.md)）。
この 2 つの事例は、エラーコードの代わりに AWS CLI が示す AWS のエラーコードで判定し、万一関数に届いても資格情報を送らないよう、Payload に `id_token` を含めません。
別の関数の Invoke は `_e2e-failure.yml` の `function` で確かめます。

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

ワークフローを試験用のリポジトリの `.github/workflows/` に、試験用の Action を `.github/actions/` に写します。
`_e2e-success.yml` は試験用の Action を相対パスで参照するためです。

```bash
cp .github/workflows/_e2e-*.yml <試験用のリポジトリ>/.github/workflows/
mkdir -p <試験用のリポジトリ>/.github/actions
cp -R .github/actions/_e2e-revoked-token <試験用のリポジトリ>/.github/actions/
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
| E2E Payload Rejection | `e2e/payload-rejection` |
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
| `GATES_PUBLIC_UNINSTALLED` | Variables | 許可オーナーの公開リポジトリのうち、GitHub App のインストール先に含めないものの完全名 |

対象のリポジトリと `GATES_PUBLIC_UNINSTALLED` のリポジトリには、それぞれ `README.md` を置きます。
トークンで読めるかを、`README.md` の取得のステータスで確かめるためです。
対象のリポジトリは非公開にします。
公開のリポジトリでは、権限のないトークンでも読める API があり、別の権限の読み取りの確認が意味を持たないためです。

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
| 監査記録 | CloudWatch Logs | Action が出した `request_id` ごとに `audit record` の行がちょうど 1 行あり、`caller.run_id` がワークフロー実行の ID と一致する。発行のワークフローでは `result` が `success` で、`request.repositories` が空白を除いた入力の表記と並びのまま。失敗のワークフローの `authorization` と `installation` では、`result` がエラーコードと一致し、`installation` は `installation` の事例にだけある |
| トークンの不発行 | CloudWatch Logs | 失敗のワークフローの時間帯に、`result` が `success` の監査記録がない |
| 引き受けと Invoke の数 | CloudTrail と CloudWatch Logs | 失敗のワークフローで、成功した `AssumeRoleWithWebIdentity` の記録は `audience`、`function`、`authorization`、`installation` の 4 つの事例の数だけ増え、Lambda の `START` の行は `authorization` と `installation` の 2 つの事例の数だけ増える。`function` の Invoke は IAM が拒否するため、関数に届かない |
| Invoke の間隔と実行時間 | CloudWatch Logs の `REPORT` 行 | 間隔の試験で、2 回目以降の Invoke に `Init Duration` がなく、直前の Invoke からの間隔が 1 分前後である |
| Payload の拒否の監査記録 | CloudWatch Logs | Payload の拒否のワークフローの `request_id` ごとに `audit record` の行がちょうど 1 行あり、`result` が期待したエラーコードと一致する。`invalid_request` は `caller`、`request`、`installation` がなく、`reason` に未知のフィールドの名前や Payload の断片がない。`invalid_id_token` は `caller` と `installation` がない。`authorization_denied` は `caller` と `request` があり `installation` がない。`github_issuance_rejected` だけに `installation` がある |
| KMS の署名の数 | CloudTrail と CloudWatch Logs | Payload の拒否のワークフローの時間帯の KMS の `Sign` の記録が、`github_issuance_rejected` の事例の数（2）と一致する。ほかの事例は KMS と GitHub API に進まない |
| 資格情報の非記録 | CloudWatch Logs | 同じ時間帯のロググループに `ghs_`、`eyJ`、`ASIA`、`FwoG`、`IQoJ` の文字列がない |

監査記録は、関数のロググループを `request_id` で検索して引きます。
失敗のワークフローの `request_id` は、`Check job logs` のジョブのログに出ます。
Payload の拒否のワークフローの `request_id` と、試験の時間帯（開始と終了の時刻）は、`Reject` のジョブのログに出ます。

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

KMS の `Sign` は管理イベントのため、同じ Event history から引けます。
時刻を Payload の拒否のワークフローの事例の時刻と並べると、どの事例が KMS に進んだかが分かります。

```bash
aws cloudtrail lookup-events --region <GATES_AWS_REGION> \
  --lookup-attributes AttributeKey=EventName,AttributeValue=Sign \
  --start-time <開始> --end-time <終了> \
  --query 'Events[].EventTime' --output text
```

Payload の拒否のワークフローの監査記録は、試験の時間帯の監査記録をまとめて引き、`Reject` のジョブのログの `request_id` と突き合わせます。
次のコマンドは、監査記録ごとに `request_id`、`result`、記録にある項目、`reason` を 1 行で出します。

```bash
aws logs filter-log-events --log-group-name /aws/lambda/<GATES_FUNCTION_NAME> \
  --start-time <開始> --end-time <終了> --filter-pattern '"audit record"' \
  --query 'events[].message' --output json \
  | jq -r '.[] | fromjson | [.request_id, .result, ([("caller", "request", "installation", "revocation") as $k | select(has($k)) | $k] | join(",")), .reason] | @tsv'
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
