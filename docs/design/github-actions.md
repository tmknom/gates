# GitHub Actions Design

## 1. Responsibility

Composite Action は、GitHub Actions の利用者にトークン取得手段を提供します。AWS 用 OIDC JWT による一時認証情報の取得、GATES 用 OIDC JWT の取得、Lambda の同期 Invoke、応答の処理と同一ジョブ内の後続ステップへのトークン出力を担当します。発行の可否の判定は Lambda 側が行います。

Composite Action は GATES 運用者が提供し、利用者向けの安定した境界とします。利用者に求めるのは、ワークフローでの `permissions: id-token: write` の宣言、対象リポジトリおよび権限の指定、運用者が用意した接続情報の参照だけです（セルフホストの Runner では、[5.1 Runner Requirements](#51-runner-requirements) のコマンドの用意も求めます）。

Composite Action は、Lambda と同じこのリポジトリの `.github/actions/issue-token/action.yml` に置きます。
Lambda の入出力契約と Composite Action を同じ PR で変更でき、両者の食い違いを防げるためです。
Composite Action はデプロイ固有の値を持たないため、公開リポジトリに置いても AWS アカウント ID は公開されません。
公開リポジトリであることから、許可オーナーのワークフローもこの Composite Action を参照できます。
名前の `issue` は、Composite Action が起動する処理が [Domain Model](domain.md) の発行であることを表します。

IAM Role と Lambda の AWS リソース設定は [AWS Infrastructure Design](aws-infrastructure.md) に記述します。Lambda Invoke の Payload、応答、エラーコードは [Lambda Design](lambda.md) を正本とします。

## 2. Composite Action Interface

### 2.1 Inputs and Outputs

| Input | 必須 | 意味 |
|---|---|---|
| `repositories` | はい | カンマ区切りの `owner/repo` リスト |
| `permissions` | はい | カンマ区切りの `permission:level`。値は `contents:read` |
| `role-arn` | はい | 引き受ける IAM Role の ARN |
| `function-name` | はい | 呼び出す Lambda の関数名 |
| `aws-region` | はい | 呼び出す Lambda を置く AWS リージョン。AWS STS の呼び出しにも使う |
| `audience` | はい | GATES 用 OIDC JWT の Audience |

`role-arn`、`function-name`、`aws-region`、`audience` を **接続情報** と呼びます。接続情報は Composite Action の中に固定せず、入力として受け取ります。Composite Action の定義にデプロイ固有の値を持たせないためです。全リポジトリを対象にする入力は設けません。GATES は対象を明示した要求だけを受け付けるためです。

接続情報は GATES 運用者が GitHub の Secrets と Variables に用意し、ワークフローはそれを参照します。AWS アカウント ID は Secrets に置き、その他の値は Variables に置きます。ワークフローに値を直接書かず参照だけにするのは、Role 名や関数名を変えたときに、Secrets と Variables の更新だけで済ませ、利用者のワークフロー定義を変えずに済ませるためです（[Requirements](../spec/requirements.md) AC-23）。

Secrets と Variables を置く単位は、許可オーナーの種別で決まります（GitHub 側の仕様は [GitHub Constraints](../spec/github-constraints.md#7-github-actions-integration)）。
オーナーが Organization なら Organization 単位に一度だけ登録します。
ただし、GitHub Free の private リポジトリと、ポリシーで利用を制限したリポジトリからは Organization 単位の値を参照できないため、それらのリポジトリには個別に登録します。
ユーザーアカウントには Organization 単位に相当する Secrets と Variables がないため、Composite Action を使う各リポジトリに登録します。
ワークフローからの参照の書き方は `secrets.*` と `vars.*` で共通なので、どちらの単位でも利用者のワークフロー定義は変わりません。

| Input | 参照する値の例 |
|---|---|
| `role-arn` | `arn:aws:iam::${{ secrets.GATES_AWS_ACCOUNT_ID }}:role/${{ vars.GATES_ROLE_NAME }}` |
| `function-name` | `${{ vars.GATES_FUNCTION_NAME }}` |
| `aws-region` | `${{ vars.GATES_AWS_REGION }}` |
| `audience` | `${{ vars.GATES_AUDIENCE }}` |

`function-name` には ARN ではなく関数名を渡します。関数名は引き受けた IAM Role と同じアカウントの関数として解決されるため、AWS アカウント ID を含めずに済みます。

`GATES_AWS_REGION` には、Lambda と KMS Key を置くリージョン（Terraform の入力の値）と同じ値を設定します（[AWS Infrastructure Design](aws-infrastructure.md#5-lambda-deployment-and-operation)）。

接続情報を利用者が入力しても、発行範囲は広がりません。IAM Role の引き受けは Trust Policy が不変 Owner ID 入りの `sub` で制限し、引き受けた Role に許可されるのは GATES 用 Lambda の Invoke だけです（[AWS Infrastructure Design](aws-infrastructure.md#31-github-actions-iam-role)）。Audience は Lambda が GATES 用の値との完全一致を検証します（[GOJ-AUD](gates-oidc-jwt-claims.md#goj-aud-aud)）。別の値を渡した要求は、いずれかの境界で失敗します。

| Output | 意味 |
|---|---|
| `token` | 発行された Installation Access Token |
| `expires-at` | GitHub が定めた有効期限（RFC 3339） |
| `repositories` | Lambda 応答が示す発行対象のリポジトリ。入力と同じ形式のカンマ区切りの `owner/repo` |
| `permissions` | Lambda 応答が示す実効権限。入力と同じ形式のカンマ区切りの `permission:level`。要求にかかわらず GitHub が付与する `metadata:read` を含み得る |

対象と実効権限を Output に含めるのは、利用者が受け取ったトークンの範囲を確認できるようにするためです（[Requirements](../spec/requirements.md) FR-016）。Output は呼び出したステップの `steps.<id>.outputs` から同一ジョブ内の後続ステップが利用できます。Action はジョブ間のトークン転送を提供しません。内部ステップの `GITHUB_OUTPUT` に設定した値は `action.yml` の `outputs` へマッピングします。

4 つの Output は、応答から `jq` で取り出した値を、`GITHUB_OUTPUT` に `name=value` の 1 行の形式で 1 つずつ書きます。
どの値も改行を含まないため、1 行の形式で足ります。
値が改行を含むと `GITHUB_OUTPUT` に別の Output の行を書き足せるため、成功応答の値が改行を含めば、不正な応答として失敗します（[3.2 Failure Output](#32-failure-output)）。
`repositories` は応答の `repositories` の順のまま `,` で結びます。
`permissions` は応答の `permissions` を権限名の昇順に並べ、`name:level` を `,` で結びます。

Composite Action の内部ステップでマスクしたトークンは、呼び出し元の後続ステップのログでもマスクされます（[GitHub Constraints](../spec/github-constraints.md#7-github-actions-integration)）。
そのため呼び出し元はトークンをもう一度マスクする必要はありません。
ただし、マスクはトークン全体の文字列に効くため、後続ステップがトークンの一部を切り出してログに出すと、その部分はマスクされません（[GitHub Composite Action Outputs](../research/github/composite-action-outputs.md)）。

### 2.2 List Syntax

```yaml
with:
  repositories: example-org/repo-a,example-org/repo-b
  permissions: contents:read
```

区切りは `,` とし、要素の前後の空白を除きます。
Composite Action 内では `repositories` を配列、`permissions` を JSON オブジェクトへ変換します。
独自のエスケープ構文は設けません。

Composite Action が検証するのは、リストの構造だけです。
次のどれかに当たる入力は、Lambda を呼ばずに失敗します。

- 空の要素がある
- `repositories` の要素が `/` をちょうど 1 つ含まないか、`/` の前後のどちらかが空である
- `repositories` に、ASCII の範囲で大文字小文字を区別せずに比べて同じ要素が 2 つ以上ある
- `permissions` の要素が `:` をちょうど 1 つ含まないか、`:` の前後のどちらかが空である
- `permissions` に、権限名が文字列として完全に一致する要素が 2 つ以上ある

空の要素や区切りの誤りは Lambda も `invalid_request` で拒否しますが、その `message` は固定の文言で、どの要素が不正かを示しません（[Lambda Design](lambda.md#32-failure)）。
Composite Action で先に止めると、利用者に該当する要素を示せます。
`permissions` の権限名の重複は、Lambda に任せることもできません。
JSON オブジェクトに変換すると後の要素が前の要素を上書きし、`contents:write,contents:read` が `{"contents":"read"}` として送られ、利用者が書いた要求と監査記録に残る要求が食い違うためです。

名前と権限の文字種、対象の件数の上限、許可条件に合うかは、Composite Action では判定しません。
これらの規則は [Lambda Design](lambda.md#22-validation) が定め、Lambda が `invalid_request` か `authorization_denied` で返します。
リストの構造は GATES が決めた文字種や上限に左右されませんが、これらの規則は GATES の判断で変わり得ます。
Composite Action に写すと規則の定義が 2 か所になり、Lambda の規則だけを変えたときに、Lambda が受け付ける要求を Composite Action が拒否し得ます。
許可条件の判定を Lambda に委ねるのは、認証済み Caller とともに許可外の要求を監査記録に残すためです（[Security Design](security.md#31-authorization-rule)）。

## 3. Processing Flow

Composite Action は、`action.yml` の次の手順を順に実行します。

1. 前提を確かめる。Runner の OS が Linux であること、必要なコマンドがあること（[5.1 Runner Requirements](#51-runner-requirements)）、`ACTIONS_ID_TOKEN_REQUEST_URL` と要求用トークン（`ACTIONS_ID_TOKEN_REQUEST_TOKEN`）があること、接続情報の 4 つの入力が空でないことである。要求用トークンがなければ、`permissions: id-token: write` の宣言を促して失敗する。
2. `umask 077` のもとで `mktemp -d "${RUNNER_TEMP}/gates.XXXXXX"` により **作業ディレクトリ** を作り、そのパスをステップの Output で後続の手順に渡す。GATES 用 OIDC JWT、Payload、応答のファイルは、作業ディレクトリの中にだけ置く。
3. `aws-actions/configure-aws-credentials` を用い、入力の `role-arn` と `aws-region` で AWS STS の一時認証情報を取得する。`output-env-credentials: false` と `output-credentials: true` を指定し、一時認証情報を環境変数ではなくステップの Output として受け取る。
4. `request-id-token.sh` が、入力の `audience` を Audience とする GATES 用 OIDC JWT を取得する。
5. `build-payload.sh` が、入力のリストを検証し（[§2.2](#22-list-syntax)）、Payload を作る。
6. `invoke.sh` が、AWS CLI `lambda invoke` で Lambda を同期実行する。
7. `handle-response.sh` が結果を判定し、成功ならトークン、有効期限、対象、実効権限を Output に書く。
8. `if: always()` の手順で作業ディレクトリを削除する。作業ディレクトリを作る前に失敗した場合は、何もせずに終える。

手順 1 と手順 4 から 7 の処理は `action.yml` の `run:` に書かず、[3.1 Scripts](#31-scripts) のスクリプトに書きます。
`run:` の手順には、入力を `env:` の環境変数で渡し、`${{ inputs.* }}` を展開しません。
展開すると、入力の値をシェルがコマンドとして解釈し得るためです。
各スクリプトは先頭で `set -euo pipefail` と `umask 077` を設定します。
手順はそれぞれ別のシェルで動き、手順 2 の umask は引き継がれないためです。
`set -x`、AWS CLI の `--debug`、`--log-type Tail` は使いません。

手順 1 で前提を先に確かめるのは、AWS STS の一時認証情報を取得する手順 3 が `id-token: write` を要し、手順 4 以降がコマンドを要するためです。
前提が欠けたまま進むと、手順 3 以降で原因の分かりにくい失敗になります。

手順 3 で一時認証情報を環境変数に書き出さないのは、Composite Action の内部の手順が書き出した環境変数が、呼び出し元のジョブの後続ステップにも残るためです。
残ると、後続ステップが一時認証情報を有効期限まで使え、利用者が先に設定した AWS の認証情報とリージョンも置き換わります（[AWS — configure-aws-credentials](https://github.com/aws-actions/configure-aws-credentials)）。
一時認証情報は、手順 3 の Output から `invoke.sh` の `env:` にだけ渡します。

OIDC JWT の取得、`GITHUB_OUTPUT`、マスク登録の GitHub 側の仕様は [GitHub Constraints](../spec/github-constraints.md#7-github-actions-integration) を参照します。

Composite Action は入力値を [Lambda Request](lambda.md#2-request) に従って変換し、[Lambda Responses](lambda.md#3-responses) に従って結果を判定します。GATES 用 OIDC JWT、AWS 一時認証情報、発行されたトークンは、成功と失敗のどちらでもログへ出しません。一時ファイルの削除は処理後の残留リスクを減らす対策であり、ランナーで動作する信頼できないコードから資格情報を保護するものではありません。

### 3.1 Scripts

通信するスクリプトと、ファイルを変換するだけのスクリプトを分けます。
変換の試験に通信を要らなくするためです。
各スクリプトは、入力の値とファイルのパスを環境変数で受け取ります。

| スクリプト | 環境変数 | 処理 | 通信 |
|---|---|---|---|
| `check-prerequisites.sh` | `ROLE_ARN`、`FUNCTION_NAME`、`REGION`、`AUDIENCE` | 手順 1 の前提を確かめ、満たさなければ、満たさなかった前提をすべて示して失敗する。要求用トークンは有無だけを確かめ、値を出さない | なし |
| `request-id-token.sh` | `AUDIENCE`、`OUT` | `ACTIONS_ID_TOKEN_REQUEST_URL` に URI エンコードした Audience を付けて GET し、応答の `value` を末尾の改行なしで `OUT` に書く。要求用トークンを含む `Authorization` ヘッダーは `curl -H @-` で標準入力から渡し、コマンドの引数に置かない | `curl` |
| `build-payload.sh` | `REPOSITORIES`、`PERMISSIONS`、`ID_TOKEN_FILE`、`OUT` | [§2.2](#22-list-syntax) の検証を行い、[Lambda Request](lambda.md#21-invoke-payload) の `version`、`id_token`、`repositories`、`permissions` を持つ Payload を `OUT` に書く。JWT は `jq --rawfile` でファイルから読み、コマンドの引数にもログにも出さない。終了時に、成否によらず `ID_TOKEN_FILE` を削除する | なし |
| `invoke.sh` | `FUNCTION_NAME`、`PAYLOAD`、`RESPONSE`、`METADATA`、`STDERR`、`STATUS`。AWS CLI 用に、手順 3 の Output の一時認証情報を `AWS_ACCESS_KEY_ID`、`AWS_SECRET_ACCESS_KEY`、`AWS_SESSION_TOKEN`、入力の `aws-region` を `AWS_REGION` | 一時認証情報をマスクし、Invoke の時刻をログに出してから、`aws lambda invoke --invocation-type RequestResponse --payload fileb://... --output json --no-cli-pager` を実行する。応答を `RESPONSE`、標準出力を `METADATA`、標準エラーを `STDERR`、終了コードを `STATUS` に書き、AWS CLI が失敗してもスクリプトは失敗しない。終了時に、成否によらず `PAYLOAD` を削除する | `aws` |
| `handle-response.sh` | `STATUS`、`STDERR`、`METADATA`、`RESPONSE` | AWS CLI の終了コード、`StatusCode`、`FunctionError`、応答の `ok` の順に判定する。成功なら Output を書き（[§2.1](#21-inputs-and-outputs)）、`request_id`、`expires_at`、対象、実効権限をログに出す。失敗なら [3.2 Failure Output](#32-failure-output) に従ってログに出し、終了コード 1 で終える。終了時に、成否によらず `RESPONSE` を削除する | なし |

要求用トークンと JWT をコマンドの引数に置かないのは、引数が同じ Runner のほかのプロセスから読めるためです。
Payload を `fileb://` で渡すのも同じ理由です。
`--output json` を指定するのは、Runner の AWS の設定で出力形式が変わっても、`METADATA` を JSON として読めるようにするためです。

### 3.2 Failure Output

Composite Action は、どの段階で失敗しても終了コード 1 で終え、`::error::` の注釈に失敗した段階と原因を書きます。
各段階でログに出すものと出さないものは次のとおりです。

| 段階 | ログに出すもの | 出さないもの |
|---|---|---|
| 前提の確認 | 満たさなかった前提（OS、足りないコマンドの名前、要求用トークンの欠落、空の入力の名前） | なし |
| GATES 用 OIDC JWT の取得 | HTTP ステータスと応答本文。通信に失敗して HTTP ステータスがなければ `curl` の終了コードと文言。`200` の応答に `value` がなければ、そのこと | 要求用トークン、JWT |
| 入力の検証 | 満たさなかった条件と、該当する要素 | なし。入力は資格情報ではない |
| AWS CLI の失敗（終了コードが 0 でない） | 標準エラーが `An error occurred (` を含めば、その括弧の中の例外の名前だけ。含まなければ標準エラーの全文 | AWS のエラー応答の文言 |
| `METADATA` を読めないか、`StatusCode` がないか `200` でない | そのことと、読み取れれば `StatusCode` の値 | なし |
| `FunctionError` がある | `FunctionError` の値と、読み取れれば応答の `errorType` | 応答の `errorMessage` |
| 応答のファイルがないか、JSON として読めないか、`ok` が真偽値でない | そのことと、応答の大きさ | 応答本文 |
| `ok` が `false` | `error.code`、`error.message`、`request_id`。どれも文字列のときだけ出す | 文字列でない値 |
| `ok` が `true` だが、成功応答の項目が欠けるか、型が違うか、値が改行を含む | そのことと、読み取れれば `request_id` | 応答本文 |

作業ディレクトリの作成や Payload の書き込みのように、外部に依存しない処理の失敗では、失敗したことと段階だけを出します。

AWS のエラー応答の文言を出さないのは、例外によっては文言に要求の一部が入り、Payload には GATES 用 OIDC JWT が入るためです。
Lambda Developer Guide の例では、JSON として読めない Payload への `InvalidRequestContentException` の文言に、読めなかった Payload の一部が入ります（[AWS Lambda Invoke Rejection](../research/aws/lambda-invoke-rejection.md)）。
どの例外の文言が要求の一部を含むかは列挙できないため、例外によらず名前だけを出します。
エラー応答かどうかを、標準エラーの形の一致ではなく `An error occurred (` を含むかで判定するのは、AWS CLI の文言の形が変わったときに、エラー応答の文言を全文として出さないためです。
エラー応答を含まない標準エラーは、接続の失敗やタイムアウトのように AWS CLI が手元で作る文言です。
AWS CLI は Payload をファイルから読んで送るだけで、手元で JSON として解釈しないため、この文言には Payload の内容が入らない見込みです（推論）。

`errorMessage` と、JSON として読めない応答本文を出さないのは、内容を GATES が定めておらず、資格情報を含まないと保証できないためです。
`FunctionError` のときは応答に `request_id` がないため、運用者は `invoke.sh` が出した Invoke の時刻と `run_id` で CloudWatch Logs の記録を探します（[Lambda Design](lambda.md#32-failure)）。
`ok` が `false` の `message` はエラーコードごとに固定の文言で、資格情報を含みません。

AWS STS の一時認証情報の取得の失敗は、`configure-aws-credentials` のログに現れ、Composite Action は出力を加えません。

### 3.3 Retry and Timeout

Composite Action は、どの要求も自動で送り直しません。
AWS CLI は既定で、接続の失敗、タイムアウト、`500`、`502`、`503`、`504`、スロットリングのときに同じ要求を送り直します（[AWS — AWS CLI retries](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-retries.html)）。
Lambda が発行を終えた後に応答が届かなかった場合、送り直した要求は 2 つ目のトークンを発行し、1 つ目のトークンは誰も持たないまま有効期限まで GitHub 側に残ります。
そのため Invoke では、環境変数 `AWS_MAX_ATTEMPTS=1` と `AWS_RETRY_MODE=standard` で AWS CLI の再試行を止めます。
`AWS_MAX_ATTEMPTS` は最初の呼び出しを含む試行の回数で、`1` は送り直さないことを表します。
再試行のモードも指定するのは、AWS CLI の資料の間で既定のモードの記述が異なり（同上、[AWS — AWS CLI environment variables](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html)）、既定に頼らずに挙動を決めるためです。
設定ファイルではなく環境変数で指定するのは、環境変数が設定ファイルの値より優先され（同上）、Runner の AWS の設定によらず再試行を止められるためです。

Lambda もどの呼び出しも再試行せず、一時的な障害を呼び出し元の再実行に任せます（[Lambda Design](lambda.md#64-call-time-budget)）。
Composite Action も、一時的な障害をステップの失敗として利用者に伝え、再実行するかは利用者が判断します。

AWS CLI の読み取りのタイムアウトは、既定の 60 秒のままとします（[AWS — AWS CLI command line options](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-options.html)）。
同期 Invoke では関数が終わるまで応答が届かず、新しい実行環境では関数の初期化の時間も待ちます。
読み取りのタイムアウトがこの合計より短いと、関数が応答を書く前に AWS CLI が打ち切り、発行したトークンを受け取れません。
関数のタイムアウト（[AWS Infrastructure Design](aws-infrastructure.md#5-lambda-deployment-and-operation)）に初期化の時間を足した値が 60 秒に近づくときは、`--cli-read-timeout` を合わせて延ばします。

### 3.4 Masking

資格情報は、変数に入れた直後、ほかのコマンドより先に `::add-mask::` でマスクします。
マスクより前に実行したコマンドの出力には、マスクが効かないためです。

| 値 | マスクする時点 |
|---|---|
| OIDC JWT の要求用トークン（`ACTIONS_ID_TOKEN_REQUEST_TOKEN`） | `request-id-token.sh` の先頭 |
| GATES 用 OIDC JWT | `request-id-token.sh` が応答から取り出した直後。ファイルに書く前 |
| AWS 一時認証情報（アクセスキー ID、シークレットアクセスキー、セッショントークン） | `invoke.sh` の先頭 |
| Installation Access Token | `handle-response.sh` が応答から取り出した直後。応答のほかの項目を判定する前 |

AWS 一時認証情報は `configure-aws-credentials` もマスクし得ますが、手順 3 の Output から受け取る経路でのマスクを確かめていないため、`invoke.sh` でもマスクします。

## 4. Usage

### 4.1 Workflow Example

`<PINNED_COMMIT>` は実際の固定コミット SHA に置き換えます。アクセス先は Contents 読み取り権限が必要な Private Repository とし、`README.md` が存在する前提です。

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

## 5. Technology Decisions

| 用途 | 採用技術・方針 | 理由 |
|---|---|---|
| AWS 一時認証情報の取得 | `aws-actions/configure-aws-credentials` | GitHub OIDC と AWS STS の処理を公式 Action に委譲 |
| Lambda Invoke | AWS CLI | SigV4 署名と同期呼び出しを AWS CLI に委譲 |
| JWT 取得と入力変換 | GitHub Actions の OIDC 取得機能、`curl`、Bash、`jq` | GitHub の標準機構を利用して Payload を組み立てる |

外部 Action は `aws-actions/configure-aws-credentials` だけとし、固定コミット SHA で参照します。

### 5.1 Runner Requirements

Composite Action は Linux の Runner で動かし、次のコマンドを使います。

| コマンド | 用途 | 条件 |
|---|---|---|
| `bash` | スクリプトの実行 | なし |
| `curl` | GATES 用 OIDC JWT の取得 | `-H @-` で標準入力からヘッダーを読める版（[curl — man page](https://curl.se/docs/manpage.html)） |
| `jq` | Payload の組み立てと応答の読み取り | 1.6 以上。`--rawfile` を使うため（[jq 1.6 Manual](https://jqlang.org/manual/v1.6/)） |
| `aws` | Lambda Invoke | AWS CLI v2 |

GitHub がホストする `ubuntu-latest` の Runner には、どのコマンドも条件を満たす版が入っており（[GitHub — runner-images](https://github.com/actions/runner-images)）、利用者の準備は要りません。
セルフホストの Runner など、ほかの Runner では、利用者がこれらのコマンドを用意します。
最初の手順（[§3](#3-processing-flow)）で OS も確かめるのは、Linux 以外の Runner では同じ名前のコマンドがあっても、`bash` の版や `mktemp` の挙動が異なり得るためです。

## 6. Testability

受け入れ条件は [Requirements](../spec/requirements.md) を正本とします。
GitHub の Installation Access Token は固定長と仮定せず、トークンの形式と長さによらず、マスク、非記録、Output への書き込みが正しく行われることを検証します。

### 6.1 Local Tests

[3.1 Scripts](#31-scripts) のスクリプトは、AWS と GitHub に接続しないローカルの試験で検証します。
試験は bats-core をコンテナで動かし、コンテナはネットワークに接続せず、Composite Action のディレクトリを読み取り専用でマウントします。
通信するスクリプトは、試験のときに PATH の先頭へ置いたスタブの `curl` と `aws` を呼びます。
スタブは受け取った引数をファイルに記録し、用意した応答と終了コードを返します。

ローカルの試験で確かめるのは次の事項です。

- 入力のリストの検証（[§2.2](#22-list-syntax)）と、Payload への変換
- GATES 用 OIDC JWT、要求用トークン、AWS 一時認証情報が、スタブが記録したコマンドの引数とログに現れないこと
- AWS CLI の終了コード、`StatusCode`、`FunctionError`、`ok` の判定と、各段階でログに出すものと出さないもの（[3.2 Failure Output](#32-failure-output)）
- 応答の配列とオブジェクトからカンマ区切りへの変換と、`GITHUB_OUTPUT` に書く 4 行
- 資格情報の `::add-mask::` が、その値を含むほかの出力より先にあること
- 成功と失敗のどちらでも、各スクリプトが受け持つファイルを削除すること

### 6.2 Tests on GitHub Actions

GitHub と AWS の挙動に依存する事項は、許可オーナーのワークフローから Composite Action を呼んで確かめます。

- 接続情報と対象と権限だけを渡してトークンを受け取り、同一ジョブ内の後続ステップで使えること
- ワークフローのログで、GATES 用 OIDC JWT、AWS 一時認証情報、トークンがマスクされているか現れないこと
- 成功と失敗のどちらでも、作業ディレクトリが Runner に残らないこと
- 呼び出し元の後続ステップの環境変数に、AWS 一時認証情報が現れないこと
- AWS STS の一時認証情報の取得の失敗と、Lambda が返す失敗で、Composite Action がエラー終了すること

ローカルの試験のコンテナの `mktemp` などの基本コマンドは、Runner のものと実装が異なり得ます。
基本コマンドの挙動に依存する作業ディレクトリの作成と削除は、GitHub Actions 上の試験で確かめます。

## 7. Sources

- [GitHub Constraints](../spec/github-constraints.md)
- [GitHub — Creating a composite action](https://docs.github.com/en/actions/tutorials/create-actions/create-a-composite-action)
- [AWS — configure-aws-credentials](https://github.com/aws-actions/configure-aws-credentials)
- [AWS — AWS CLI Lambda Invoke](https://docs.aws.amazon.com/cli/latest/reference/lambda/invoke.html)
- [AWS — AWS CLI retries](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-retries.html)
- [AWS — AWS CLI environment variables](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html)
- [AWS — AWS CLI command line options](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-options.html)
- [GitHub — Workflow commands for GitHub Actions](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands)
- [jq 1.6 Manual](https://jqlang.org/manual/v1.6/)
- [curl — man page](https://curl.se/docs/manpage.html)
- [GitHub — runner-images](https://github.com/actions/runner-images)
- [GitHub — REST API endpoints for repository contents](https://docs.github.com/en/rest/repos/contents)

## Appendix: Impact of Future Scope

[Future Scope](../spec/future-scope.md) が想定する機能を取り込む場合、本ドキュメントには次の影響が及びます。

- **細粒度の認可**：発行の可否の判定は Lambda 側が行う（§1）ため、利用者が指定する入力が増えない限り、Composite Action は変わらない
- **発行可能な権限の拡大**：`permissions` 入力の値（§2.1）と利用例（§4.1）が変わる。許可条件の判定を Lambda の認可に委ねる方針（§2.2）は変わらない。Account / Organization に関係する権限の対象を `repositories` 入力で表せないなら、入力も見直す
