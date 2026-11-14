# GitHub Actions Design

## 1. Responsibility

Docker Action は、GitHub Actions の利用者にトークン取得手段を提供します。AWS 用 OIDC JWT による AWS STS からの一時認証情報の取得、GATES 用 OIDC JWT の取得、Lambda の同期 Invoke、応答の処理と同一ジョブ内の後続ステップへのトークン出力、受け取ったトークンの失効を担当します。発行の可否の判定は Lambda 側が行います。

これらの処理は、Docker Action のイメージに入れた Go のプログラムが 1 つのプロセスで行います。このプロセスは、同じプログラムの親のプロセスが起動して見張る子のプロセスです（[§3.2](#32-failure-output)）。
GATES 用 OIDC JWT、AWS 用 OIDC JWT、AWS 一時認証情報、Payload、応答は、プロセスのメモリの中だけで扱い、ファイルに書きません。

Docker Action は GATES 運用者が提供し、利用者向けの安定した境界とします。利用者に求めるのは、ワークフローでの `permissions: id-token: write` の宣言、対象リポジトリおよび権限の指定、運用者が用意した接続情報の参照だけです（セルフホストの Runner では、[5.4 Runner Requirements](#54-runner-requirements) の Linux と Docker の用意と、通信の許可も求めます）。

Docker Action は、Lambda と同じこのリポジトリの `.github/actions/issue-token/` に置きます。
Lambda の入出力契約と Docker Action を同じ PR で変更でき、両者の食い違いを防げるためです。
Docker Action はデプロイ固有の値を持たないため、公開リポジトリに置いても AWS アカウント ID は公開されません。
公開リポジトリであることから、許可オーナーのワークフローもこの Docker Action を参照できます。
名前の `issue` は、Docker Action が起動する処理が [Domain Model](domain.md) の発行であることを表します。

IAM Role と Lambda の AWS リソース設定は [AWS Infrastructure Design](aws-infrastructure.md) に記述します。Lambda Invoke の Payload、応答、エラーコードは [Lambda Design](lambda.md) を正本とします。

## 2. Docker Action Interface

### 2.1 Inputs and Outputs

| Input | 必須 | 意味 |
|---|---|---|
| `repositories` | はい | カンマ区切りの `owner/repo` リスト |
| `permissions` | はい | カンマ区切りの `permission:level`。値は `contents:read` |
| `role-arn` | はい | 引き受ける IAM Role の ARN |
| `function-name` | はい | 呼び出す Lambda の関数名 |
| `aws-region` | はい | 呼び出す Lambda を置く AWS リージョン。AWS STS の呼び出しにも使う |
| `audience` | はい | GATES 用 OIDC JWT の Audience |

`role-arn`、`function-name`、`aws-region`、`audience` を **接続情報** と呼びます。接続情報は Docker Action の中に固定せず、入力として受け取ります。Docker Action の定義にデプロイ固有の値を持たせないためです。全リポジトリを対象にする入力は設けません。GATES は対象を明示した要求だけを受け付けるためです。

入力は、Runner が `INPUT_<大文字の入力名>` の環境変数としてコンテナに渡すものを読みます（`INPUT_ROLE-ARN` のようにハイフンは残ります）。
`action.yml` に `args` を書かず、入力をコマンドの引数にしません。
Runner はコンテナを起動する `docker run` の行に環境変数の名前だけを載せ、値を載せないためです（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md#inputs)）。

接続情報は GATES 運用者が GitHub の Secrets と Variables に用意し、ワークフローはそれを参照します。AWS アカウント ID は Secrets に置き、その他の値は Variables に置きます。ワークフローに値を直接書かず参照だけにするのは、Role 名や関数名を変えたときに、Secrets と Variables の更新だけで済ませ、利用者のワークフロー定義を変えずに済ませるためです（[Requirements](../spec/requirements.md) AC-23）。

Secrets と Variables を置く単位は、許可オーナーの種別で決まります（GitHub 側の仕様は [GitHub Constraints](../spec/github-constraints.md#7-github-actions-integration)）。
オーナーが Organization なら Organization 単位に一度だけ登録します。
ただし、GitHub Free の private リポジトリと、ポリシーで利用を制限したリポジトリからは Organization 単位の値を参照できないため、それらのリポジトリには個別に登録します。
ユーザーアカウントには Organization 単位に相当する Secrets と Variables がないため、Docker Action を使う各リポジトリに登録します。
ワークフローからの参照の書き方は `secrets.*` と `vars.*` で共通なので、どちらの単位でも利用者のワークフロー定義は変わりません。

| Input | 参照する値の例 |
|---|---|
| `role-arn` | `arn:aws:iam::${{ secrets.GATES_AWS_ACCOUNT_ID }}:role/${{ vars.GATES_ROLE_NAME }}` |
| `function-name` | `${{ vars.GATES_FUNCTION_NAME }}` |
| `aws-region` | `${{ vars.GATES_AWS_REGION }}` |
| `audience` | `${{ vars.GATES_AUDIENCE }}` |

`function-name` には ARN ではなく関数名を渡します。関数名は引き受けた IAM Role と同じアカウントの関数として解決されるため、AWS アカウント ID を含めずに済みます。

`GATES_AWS_REGION` には、Lambda と KMS Key を置くリージョン（Terraform の入力の値）と同じ値を設定します（[AWS Infrastructure Design](aws-infrastructure.md#5-lambda-deployment-and-operation)）。

接続情報を利用者が入力しても、発行範囲は広がりません。IAM Role の引き受けは Trust Policy が不変 Owner ID 入りの `sub` で制限し、引き受けた Role に許可されるのは GATES 用 Lambda の Invoke だけです（[AWS Infrastructure Design](aws-infrastructure.md#31-github-actions-iam-role)）。Audience は Lambda が GATES 用の値との完全一致を検証します（[GATES OIDC JWT Claims](gates-oidc-jwt-claims.md) の GOJ-AUD）。別の値を渡した要求は、いずれかの境界で失敗します。

| Output | 意味 |
|---|---|
| `token` | 発行された Installation Access Token |
| `expires-at` | GitHub が定めた有効期限（RFC 3339） |
| `repositories` | Lambda 応答が示す発行対象のリポジトリ。入力と同じ形式のカンマ区切りの `owner/repo` |
| `permissions` | Lambda 応答が示す実効権限。入力と同じ形式のカンマ区切りの `permission:level`。要求にかかわらず GitHub が付与する `metadata:read` を含み得る |

対象と実効権限を Output に含めるのは、利用者が受け取ったトークンの範囲を確認できるようにするためです（[Requirements](../spec/requirements.md) FR-016）。Output は呼び出したステップの `steps.<id>.outputs` から同一ジョブ内の後続ステップが利用できます。Action はジョブ間のトークン転送を提供しません。

4 つの Output は、`GITHUB_OUTPUT` に `name=value` の 1 行の形式で 1 つずつ書きます。
どの値も改行を含まないため、1 行の形式で足ります。
値が改行を含むと `GITHUB_OUTPUT` に別の Output の行を書き足せるため、成功応答の値が改行を含めば、不正な応答として失敗します（[3.2 Failure Output](#32-failure-output)）。
`repositories` は応答の `repositories` の順のまま `,` で結びます。
`permissions` は応答の `permissions` を権限名の昇順に並べ、`name:level` を `,` で結びます。
`repositories` か `permissions` が空であるか、要素が空であるか、`repositories` の要素が `,` を含むか、`permissions` の権限名か水準が `,` か `:` を含めば、不正な応答として失敗します。
後続ステップは Output を入力と同じ形式として読むため、区切りの文字を含む要素は別の要素として読まれ、応答が示した範囲と食い違うためです。
`expires-at` か `request_id` が空である場合も、不正な応答として失敗します。
空の `expires-at` では利用者が有効期限を確かめられず、空の `request_id` では運用者が監査記録を探せないためです。
`token` 以外の Output の値と `request_id` が `##[` を含む場合も、不正な応答として失敗します。
これらの値は成功のログの行にも出し、Runner は `::` で始まるワークフローコマンドを行頭でだけ読む一方、`##[` で始まる旧形式のワークフローコマンドは行の途中からも読むためです（[GitHub — actions/runner ActionCommand.cs](https://github.com/actions/runner/blob/v2.338.2/src/Runner.Common/ActionCommand.cs) の `TryParseV2` と `TryParse`）。

トークンは、ジョブの終了時の失効（[3.5 Revocation](#35-revocation)）に渡すため、`GITHUB_STATE` にも `token=<トークン>` の 1 行で書きます。
`GITHUB_STATE` は `GITHUB_OUTPUT` と同じ Runner のファイルコマンドのディレクトリにあり、トークンを書く場所は増えても、読める主体は増えません。

Output と `GITHUB_STATE` は、応答をすべて判定して成功が決まった後にだけ書きます。
メインの処理が失敗で終わっても、それより前に書いた Output は呼び出し元の後続ステップに届くためです（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md#output-and-mask)）。
`GITHUB_STATE` を先に、`GITHUB_OUTPUT` を後に書きます。
後続ステップに渡したトークンが、ジョブの終了時の失効の対象から漏れないようにするためです。
どちらへの書き込みの失敗もエラー終了にします。
書き込みに失敗してもステップが成功で終わると、後続ステップは空のトークンで動き、失敗の原因を追えないためです。

Docker Action の中でマスクしたトークンは、呼び出し元の後続ステップのログでもマスクされます（[GitHub Constraints](../spec/github-constraints.md#7-github-actions-integration)）。
そのため呼び出し元はトークンをもう一度マスクする必要はありません。
ただし、マスクは登録した文字列の全体に一致する箇所に効くため、後続ステップがトークンの一部を切り出してログに出すと、その部分はマスクされません（[GitHub Composite Action Outputs](../research/github/composite-action-outputs.md#mask)）。

### 2.2 List Syntax

```yaml
with:
  repositories: example-org/repo-a,example-org/repo-b
  permissions: contents:read
```

区切りは `,` とし、要素の前後の空白（スペース、タブ、CR、LF）を除きます。
Docker Action 内では `repositories` を配列、`permissions` を JSON オブジェクトへ変換します。
空白を除くほかは要素を書き換えず、大文字小文字もそろえずに、入力の順のまま変換します。
独自のエスケープ構文は設けません。

Docker Action が検証するのは、リストの構造と、要素が UTF-8 として正しいことだけです。
次のどれかに当たる入力は、Lambda を呼ばずに失敗します。

- 空の要素がある
- `repositories` の要素が `/` をちょうど 1 つ含まないか、`/` の前後のどちらかが、空であるか、空白で始まるか終わる
- `repositories` に、ASCII の範囲で大文字小文字を区別せずに比べて同じ要素が 2 つ以上ある
- `permissions` の要素が `:` をちょうど 1 つ含まないか、`:` の前後のどちらかが、空であるか、空白で始まるか終わる
- `permissions` に、権限名が文字列として完全に一致する要素が 2 つ以上ある
- 要素が UTF-8 として正しくない

空の要素や区切りの誤りは Lambda も `invalid_request` で拒否しますが、その `message` は固定の文言で、どの要素が不正かを示しません（[Lambda Design](lambda.md#32-failure)）。
Docker Action で先に止めると、利用者に該当する要素を示せます。
UTF-8 として正しくない要素は、Payload の JSON の文字列にすると別の文字に置き換わり、要素を書き換えない変換にならないため、Docker Action で止めます。
`permissions` の権限名の重複は、Lambda に任せることもできません。
JSON オブジェクトに変換すると後の要素が前の要素を上書きし、`contents:write,contents:read` が `{"contents":"read"}` として送られ、利用者が書いた要求と監査記録に残る要求が食い違うためです。

名前と権限の文字種、対象の件数の上限、許可条件に合うかは、Docker Action では判定しません。
これらの規則は [Lambda Design](lambda.md#22-validation) が定め、Lambda が `invalid_request` か `authorization_denied` で返します。
リストの構造は GATES が決めた文字種や上限に左右されませんが、これらの規則は GATES の判断で変わり得ます。
Docker Action に写すと規則の定義が 2 か所になり、Lambda の規則だけを変えたときに、Lambda が受け付ける要求を Docker Action が拒否し得ます。
許可条件の判定を Lambda に委ねるのは、認証済み Caller とともに許可外の要求を監査記録に残すためです（[Security Design](security.md#31-authorization-rule)）。

## 3. Processing Flow

Docker Action のメインの処理は、次の段階を 1 つの子のプロセスで順に行います。
各段階で得た資格情報は、[3.4 Masking](#34-masking) に示す時点でマスクします。

1. 前提を確かめる。`ACTIONS_ID_TOKEN_REQUEST_URL` と要求用トークン（`ACTIONS_ID_TOKEN_REQUEST_TOKEN`）があること、`GITHUB_OUTPUT` と `GITHUB_STATE` があること、6 つの入力が空でないことである。要求用トークンがなければ、`permissions: id-token: write` の宣言を促して失敗する。続けて、`ACTIONS_ID_TOKEN_REQUEST_URL` がホストを持つ `https` の URL であることを通信の前に確かめる。要求用トークンを平文で送らないためである
2. 入力のリストを検証し（[§2.2](#22-list-syntax)）、`repositories` の配列と `permissions` のオブジェクトを作る。この段階は通信しない
3. `sts.amazonaws.com` を Audience とする AWS 用 OIDC JWT を取得する
4. AWS STS の `AssumeRoleWithWebIdentity` で、入力の `role-arn` の IAM Role を引き受ける。AWS 用 OIDC JWT はこの段階の後に使わない
5. 入力の `audience` を Audience とする GATES 用 OIDC JWT を取得する
6. [Lambda Request](lambda.md#21-invoke-payload) の `version`、`id_token`、`repositories`、`permissions` を持つ Payload を組み立て、Invoke の時刻をログに出してから、一時認証情報で Lambda を `RequestResponse` で Invoke する。`InvocationType` は省略せずに指定する。AWS SDK for Go v2 は省略すると `X-Amz-Invocation-Type` を送らず、呼び方を Lambda の既定に任せることになるためである。`LogType` を指定しない
7. 結果を判定する。SDK のエラー、`StatusCode`、`FunctionError`、応答の `ok` の順に見る。成功なら、トークンを取り出し、残りの項目を判定する。トークンとして取り出すのは、空でなく ASCII の制御文字を含まない文字列だけである。改行を含む値は 1 行の形式で書く `GITHUB_STATE` と `GITHUB_OUTPUT` に別の行を書き足せ（[§2.1](#21-inputs-and-outputs)）、タブ以外の制御文字を含む値は Go の `net/http` が失効の要求のヘッダーに載せない。トークンの文字種に制御文字はないため、タブを含む ASCII の制御文字をまとめて、取り出せない応答として失敗する。すべて満たせば、トークンを `GITHUB_STATE` に、4 つの Output を `GITHUB_OUTPUT` に書き（[§2.1](#21-inputs-and-outputs)）、`request_id`、`expires_at`、対象、実効権限をログに出す。トークンを取り出した後に失敗すれば、失敗時の失効を試みてから終える（[§3.5](#35-revocation)）

入力の検証を AWS STS より前に置くのは、通信を要らない失敗を先に返し、検証に失敗する入力で IAM Role を引き受けないためです。
前提を最初に確かめるのは、OIDC JWT の取得と AWS STS が `id-token: write` を要し、欠けたまま進むと原因の分かりにくい失敗になるためです。

AWS STS の一時認証情報はプロセスの中の Lambda のクライアントにだけ渡し、環境変数にも `GITHUB_ENV` にも書きません。
書くと、呼び出し元の後続ステップが一時認証情報を有効期限まで使え、利用者が先に設定した AWS の認証情報とリージョンも置き換わるためです。
`AssumeRoleWithWebIdentity` では有効期間に下限の 900 秒を指定します。
一時認証情報を使うのは 1 回の Invoke だけで、長く有効にしておく理由がないためです。
セッション名は固定の `gates-issue-token` とします。

Docker Action は入力値を [Lambda Request](lambda.md#2-request) に従って変換し、[Lambda Responses](lambda.md#3-responses) に従って結果を判定します。GATES 用 OIDC JWT、AWS 用 OIDC JWT、AWS 一時認証情報、発行されたトークンは、成功と失敗のどちらでもログへ出しません。
資格情報をプロセスのメモリの中だけで扱うことは、ランナーで動作する信頼できないコードから資格情報を保護するものではありません。
同じジョブで信頼できないコードを動かさないことは利用者の責任です（[Security Design](security.md#5-failure-audit-and-security-risks)）。

OIDC JWT の取得、`GITHUB_OUTPUT`、`GITHUB_STATE`、マスク登録、Docker Action の実行環境の GitHub 側の仕様は [GitHub Constraints](../spec/github-constraints.md#7-github-actions-integration) を参照します。

OIDC JWT の取得では、`ACTIONS_ID_TOKEN_REQUEST_URL` に URI エンコードした Audience を `audience` のパラメーターで付けて GET し、要求用トークンを `Authorization: Bearer` のヘッダーで送ります。
リダイレクトには従わず、`3xx` は失敗にします。
従うと、要求用トークンを含むヘッダーを別の宛先へ送り得るためです。
応答本文は 64 KiB までを読み、超えれば失敗にします。
JWT は数 KB であり、上限を設けないと、想定と異なる応答でプロセスのメモリを使い切り得るためです。
`200` の応答の `value` が改行を含むか、応答が JSON のオブジェクト 1 つでなければ失敗にします。
JWS Compact Serialization の構成要素は改行を含められないため（[GATES OIDC JWT Signature](gates-oidc-jwt-signature.md)）、改行を含む `value` は JWT ではないためです。

### 3.1 Packages

Docker Action は、`.github/actions/issue-token/` を独立した Go モジュールとし、`main.go` をその直下に、ほかの package を `internal/` に置きます。
ルートの Go モジュールと分けるのは、Go の `./...` がドットで始まる `.github` を対象にせず、ルートのモジュールに置くとビルド、試験、lint から黙って外れるためです。
`action.yml`、`Dockerfile`、`.dockerignore`、利用者向けの README も同じディレクトリに置きます。
Runner は `Dockerfile` のあるディレクトリをビルドコンテキストにし、その外のファイルを `Dockerfile` から参照できないためです（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md#runner-source)）。

通信する package と、値を受けて値を返すだけの package を分けます。
入力の検証、Payload の組み立て、応答の判定、Output への変換の試験に通信を要らなくするためです。

| package | 責務 | 通信 |
|---|---|---|
| `main.go` | 起動したパスによる親と子の選択。メインの親ならシグナルの受け口を作り、post-entrypoint の親ならシグナルを無視して、子の起動と見張りを呼び、子なら環境変数を読み取り、シグナルの受け方を設定して入口関数を呼ぶ（[§3.6](#36-cancellation)）。終了コード。panic を回復し、その値を出さずに失敗として終える | なし |
| `internal/supervise` | 親の処理。同じ実行ファイルを子として起動し、メインでは受けたシグナルを子へ送り、子の終了の仕方から親の終了コードと注釈を決める（[§3.2](#32-failure-output)、[§3.6](#36-cancellation)） | なし |
| `internal/issue` | メインの入口関数。前提の確認から Output の書き込みまでの段階（[§3](#3-processing-flow)）を順に呼び、トークンを取り出した後の失敗では失効を呼ぶ（[§3.5](#35-revocation)）。失敗の注釈を書く（[§3.2](#32-failure-output)）。キャンセルの要求を受けた時点に応じて終える（[§3.6](#36-cancellation)） | なし。部品に任せる |
| `internal/revoke` | post-entrypoint の入口関数。`GITHUB_STATE` から渡ったトークンがあれば失効を呼ぶ（[§3.5](#35-revocation)） | なし。部品に任せる |
| `internal/request` | 入力のリストの検証（[§2.2](#22-list-syntax)）と Payload の組み立て | なし |
| `internal/response` | Invoke の結果を SDK のエラー、`StatusCode`、`FunctionError`、`ok` の順に判定し、成功なら 4 つの Output の値とログに出す項目を、失敗なら段階とログに出す文言を返す。トークンを取り出す判定と残りの項目の判定を分け、その間で入口関数がマスクとキャンセルの確認を行う（[§3.4](#34-masking)、[§3.6](#36-cancellation)） | なし |
| `internal/revocation` | 失効の要求と、その結果のログへの出し方（[§3.5](#35-revocation)）。失敗時の失効とジョブの終了時の失効で共有する | なし。部品に任せる |
| `internal/communication` | 通信の失敗の error。タイムアウトとそれ以外の判定（[§3.2](#32-failure-output)）を、通信する package で共有する | なし |
| `internal/workflow` | ワークフローコマンド（`::add-mask::`、`::error::`、`::warning::`）の符号化と標準出力への書き込み、ファイルコマンド（`GITHUB_OUTPUT` と `GITHUB_STATE` への `name=value` の書き込み）と、改行を含む値の拒否 | なし |
| `internal/credential` | 資格情報の型。`fmt` と JSON で値を出さず、値を取り出す口を使う側に限る | なし |
| `internal/oidc` | OIDC JWT の取得。AWS 用と GATES 用の両方に使う | GitHub OIDC Provider |
| `internal/awsapi` | `AssumeRoleWithWebIdentity` と Lambda の Invoke。AWS SDK を使う唯一の package | AWS STS、Lambda Invoke API |
| `internal/githubapi` | `DELETE /installation/token` を呼び、HTTP ステータスを返す。失効の結果をログに出すのは `internal/revocation` | GitHub API |

`os.Getenv`、`os.Args`、`os.Exit`、`os.Stdout`、`os/signal` を参照するのは `main.go` だけとします。
`internal/` の package は、読み取った値と書き込み先を引数で受け取るため、環境変数とファイルを用意せずに試験できます。

メインと post-entrypoint は、同じ実行ファイルを置いた 2 つのパスで区別します。
イメージには実行ファイルを `/issue-token` に置き、`/revoke-token` をそれを指すシンボリックリンクにし、`action.yml` の `entrypoint` を `/issue-token`、`post-entrypoint` を `/revoke-token` にします。
シンボリックリンクで起動しても、起動したパスはリンクのパスのまま実行ファイルに渡ります。
実行ファイルを 2 つ複製しないのは、AWS SDK を含む実行ファイルの大きさの分だけ、ジョブのたびにビルドするイメージが大きくなるためです。
`main.go` は、起動したパスの末尾の名前で役割を選び、どれでもなければ終了コード 1 で終えます。

| 末尾の名前 | 役割 |
|---|---|
| `issue-token` | メインの親。`issue-token-child` の子を起動して見張る |
| `revoke-token` | post-entrypoint の親。`revoke-token-child` の子を起動して見張る |
| `issue-token-child` | メインの子。メインの入口関数を呼ぶ |
| `revoke-token-child` | post-entrypoint の子。post-entrypoint の入口関数を呼ぶ |

親は、自身の実行ファイルを、起動したパスの末尾の名前に `-child` を付けた名前を `argv[0]` にして起動します。
自身の実行ファイルは、Linux の `/proc/self/exe` で指します。Runner は Linux に限るためです（[§5.4](#54-runner-requirements)）。
子の名前は、`action.yml` が固定する `entrypoint` と `post-entrypoint` のパスには現れず、利用者は子を直接起動させられません。
子であることを環境変数で示さないのは、利用者がステップの `env:` で同じ環境変数を渡すと、親の見張りを外せるためです。
引数で区別しないのは、Runner が `runs.args` をメインと post-entrypoint に同じ値で渡し、段階ごとに切り替えるのは `docker run` の `--entrypoint` に渡す `entrypoint` と `post-entrypoint` の値だけだからです（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md#runner-source)）。
`STATE_` の環境変数の有無で区別しないのは、メインが失敗して `GITHUB_STATE` を書かなかったジョブの post-entrypoint が、メインの処理をもう一度行うことになるためです。

### 3.2 Failure Output

Docker Action のメインの処理は、どの段階で失敗しても終了コード 1 で終え、`::error::` の注釈に、失敗した段階を示す固定の文言と原因を書きます。
各段階でログに出すものと出さないものは次のとおりです。

| 段階 | ログに出すもの | 出さないもの |
|---|---|---|
| 前提の確認 | 満たさなかった前提（環境変数の欠落、空の入力の名前） | 要求用トークン |
| OIDC JWT の要求先の形 | `ACTIONS_ID_TOKEN_REQUEST_URL` がホストを持つ `https` の URL でないこと | URL の値 |
| 入力の検証 | 満たさなかった条件と、該当する要素 | なし。入力は資格情報ではない |
| OIDC JWT の取得（AWS 用、GATES 用） | HTTP ステータス。`4xx` と `5xx` なら、応答本文の先頭の 4096 バイトも。通信に失敗して HTTP ステータスがなければ、失敗の種類。`200` の応答に 1 行の `value` がなければ、そのこと | 要求用トークン、JWT。`200` 以外の `2xx` と `3xx` の応答本文は、マスクする前の JWT を含み得るため出さない |
| AWS STS | AWS のエラーコード（`AccessDenied`、`InvalidIdentityToken` など）と HTTP ステータス。エラーコードを読み取れない応答なら HTTP ステータスだけ。通信の失敗なら失敗の種類。要求を送る前の失敗なら、送れなかったこと。成功の応答に一時認証情報の 3 つの値がそろわなければ、そのこと | AWS のエラー応答の文言、要求の内容、SDK のエラー文言 |
| Invoke の SDK の失敗 | AWS のエラーコード（`AccessDeniedException`、`InvalidRequestContentException` など）と HTTP ステータス。エラーコードを読み取れない応答なら HTTP ステータスだけ。通信の失敗なら失敗の種類。要求を送る前の失敗なら、送れなかったこと | AWS のエラー応答の文言、SDK のエラー文言 |
| `StatusCode` が `200` でない | `StatusCode` の値 | なし |
| `FunctionError` がある | `FunctionError` の値と、読み取れれば応答の `errorType`。どちらも `##[` を含まないときだけ出す | 応答の `errorMessage`、`##[` を含む値 |
| 応答を JSON として読めないか、`ok` が真偽値でない | そのことと、応答の大きさ | 応答本文 |
| `ok` が `false` | `error.code`、`error.message`、`request_id`。どれも文字列で `##[` を含まないときだけ出す | 文字列でない値、`##[` を含む値 |
| `ok` が `true` だが、成功応答の項目が欠けるか、型が違うか、値が [§2.1](#21-inputs-and-outputs) と [§3](#3-processing-flow) の段階 7 の条件を満たさない | そのことと、文字列で `##[` を含まなければ `request_id` | 応答本文、`##[` を含む `request_id` |
| `GITHUB_STATE` か `GITHUB_OUTPUT` への書き込み | 書けなかったファイルの環境変数の名前 | 書こうとした値、ライブラリのエラー文言 |
| 想定外の失敗（panic） | 想定外の失敗が起きたこと | panic の値 |
| キャンセル（[§3.6](#36-cancellation)） | キャンセルされたこと | なし |

AWS のエラー応答の文言を出さないのは、例外によっては文言に要求の一部が入り、Invoke の Payload には GATES 用 OIDC JWT が、`AssumeRoleWithWebIdentity` の要求には AWS 用 OIDC JWT が入るためです。
Lambda Developer Guide の例では、JSON として読めない Payload への `InvalidRequestContentException` の文言に、読めなかった Payload の一部が入ります（[AWS Lambda Invoke Rejection](../research/aws/lambda-invoke-rejection.md)）。
どの例外の文言が要求の一部を含むかは列挙できないため、例外によらずエラーコードと HTTP ステータスだけを出します。

通信の失敗の種類は、タイムアウトとそれ以外の通信の失敗の 2 つに分け、Go の標準ライブラリの種別（`context.DeadlineExceeded` と、`net.Error` の `Timeout()`）で判定します。
SDK とライブラリのエラー文言は出しません。
文言には接続先の URL や要求の一部が入り得るためです。
panic の値を出さないのも、値に資格情報が入っていない保証がないためです。

子のコードが panic するのは、ほかのコードが保証している不変条件が破れた場合だけです。
外部入力と外部呼び出しの失敗は、panic にせずエラーで返します。
メインの入口関数は、トークンを取り出した後の panic を一度回復して失敗時の失効を試み（[§3.5](#35-revocation)）、同じ panic を `main.go` の回復に渡し直します。

`main.go` の回復が効くのは、入口関数を呼ぶ goroutine の panic だけです。
そのため子のコードは goroutine を起動せず、すべての処理を入口関数の goroutine で行います。
親は、子の終了を待つために goroutine を使います（[§3.6](#36-cancellation)）。
親は資格情報を読まないため、親の panic の値が資格情報を含むことはありません。
`net/http` と AWS SDK が内部で起動する goroutine の panic は回復できず、Go のランタイムが panic の値とスタックを標準エラー出力に書き、終了コード 2 でプロセスを終えます。
panic の値には資格情報の一部が入り得て、一部だけではログのマスクが効きません。

この出力をログに出さず、落ちたことだけをログに残すため、資格情報を扱う処理は子のプロセスで行い、親のプロセスが子を見張ります（[§3.1](#31-packages)）。

- 親は資格情報を読まず、子を起動し、メインでは受けたシグナルを子へ送り（[§3.6](#36-cancellation)）、終了を待つだけとする
- 子の標準出力は親の標準出力をそのまま引き継ぎ、子のワークフローコマンドと注釈はそのまま Runner に届く
- 子の標準エラー出力は `/dev/null` につなぎ、Go のランタイムが書く panic の値とスタックを捨てる
- 子のコードが使う終了コードは、メインの子では 0 と 1、post-entrypoint の子では 0 だけとする。メインの子が 0 か 1 で終われば、親は何も書かずに同じ終了コードで終える
- メインの子がそれ以外の終了コードかシグナルで終われば、親は想定外の失敗で終了したことを固定の文言で `::error::` に書き、終了コード 1 で終える。回復されない panic、Go のランタイムの致命的なエラー、メモリ不足による強制終了がこれに当たる
- メインの子を起動できなければ、親はそのことを固定の文言で `::error::` に書き、終了コード 1 で終える

post-entrypoint では、子を起動できなかった場合と、子が 0 以外の終了コードかシグナルで終わった場合に、親は同じ固定の文言を `::warning::` に書き、終了コード 0 で終えます。
post-entrypoint の子が panic を回復した場合も、子は想定外の失敗が起きたことを `::warning::` に書き、終了コード 0 で終えます。
失効の失敗でジョブを失敗させず、`::warning::` で知らせる（[§3.5](#35-revocation)）のと同じ扱いにするためです。
`::error::` にすると、すべてのステップが成功したジョブにエラーの注釈だけが残り、利用者が存在しない失敗を探すことになります。

親が SIGINT か SIGTERM を受けたときの扱いは [3.6 Cancellation](#36-cancellation) に記述します。

トークンを取り出した後の失敗では、失敗の注釈に加えて、失敗時の失効の結果をログに出します（[§3.5](#35-revocation)）。

`errorMessage` と、JSON として読めない応答本文を出さないのは、内容を GATES が定めておらず、資格情報を含まないと保証できないためです。
`FunctionError` のときは応答に `request_id` がないため、運用者は段階 6 で出した Invoke の時刻と `run_id` で CloudWatch Logs の記録を探します（[Lambda Design](lambda.md#32-failure)）。
`ok` が `false` の `message` はエラーコードごとに固定の文言で、資格情報を含みません。
応答から注釈に出す値のうち `##[` を含むものを出さないのは、Runner が旧形式のワークフローコマンドを行の途中からも読むためです（[§2.1](#21-inputs-and-outputs)）。

注釈は標準出力に書き、注釈と `::add-mask::` に入れる値は `%`、CR、LF をそれぞれ `%25`、`%0D`、`%0A` に符号化して 1 行に収めます（[GitHub — actions/toolkit command.ts](https://github.com/actions/toolkit/blob/main/packages/core/src/command.ts) の `escapeData`）。
値に改行が残ると、その後の行を Runner がワークフローコマンドとして読み得るためです。
マスクの値に `%` が符号化されずに残ると、Runner が `%25`、`%0D`、`%0A` を戻した後の別の文字列をマスクし、資格情報そのものはマスクされません。
入力の検証の注釈では、該当する要素を二重引用符で囲み、制御文字と UTF-8 として正しくないバイトをエスケープして示し、前後の空白や制御文字を見分けられるようにします。

### 3.3 Retry and Timeout

Docker Action は、どの要求も自動で送り直しません。
Lambda が発行を終えた後に応答が届かなかった場合、送り直した要求は 2 つ目のトークンを発行し、1 つ目のトークンは誰も持たないまま有効期限まで GitHub 側に残るためです。
AWS SDK for Go v2 が既定で行う再試行は、AWS STS と Lambda のクライアントの両方で `aws.NopRetryer` により止めます。
Lambda の KMS の呼び出しと同じ扱いです（[Lambda Design](lambda.md#64-call-time-budget)）。

AWS のクライアントの設定は、Region、一時認証情報、再試行、HTTP のクライアントを Docker Action のプログラムが組み立て、これらと接続先の設定を共有の設定ファイルと `AWS_*` の環境変数から読みません。
読むと、利用者がステップに渡した環境変数や Runner の AWS の設定で、接続先、認証情報、再試行が変わり得るためです。AWS STS の呼び出しは認証情報を要らないため、認証情報を持たないクライアントで行います。

Lambda もどの呼び出しも再試行せず、一時的な障害を呼び出し元の再実行に任せます。
Docker Action も、一時的な障害をステップの失敗として利用者に伝え、再実行するかは利用者が判断します。

| 呼び出し | タイムアウト |
|---|---|
| OIDC JWT の取得（AWS 用、GATES 用） | 接続 10 秒、全体 30 秒 |
| AWS STS | 全体 30 秒 |
| Lambda Invoke | 全体 90 秒 |
| 失効（失敗時の失効、ジョブの終了時の失効） | 全体 5 秒 |

どの呼び出しにも全体の時間の上限を設けます。
上限がないと、接続した相手が応答しないとき、ジョブのタイムアウトまで失敗を伝えられないためです。

失効の上限は、Lambda の失効と同じ 5 秒とします（[Lambda Design](lambda.md#64-call-time-budget)）。
失効はベストエフォートで失敗を再試行しないため、応答しない相手を長く待っても結果は変わらず、ジョブの時間だけが延びるためです。

Lambda Invoke の上限は、関数のタイムアウト（[AWS Infrastructure Design](aws-infrastructure.md#5-lambda-deployment-and-operation)）に初期化の時間を足した値より長くします。
同期 Invoke では関数が終わるまで応答が届かず、新しい実行環境では関数の初期化の時間も待ちます。
上限がこの合計より短いと、関数が応答を書く前に Docker Action が打ち切り、発行したトークンを受け取れません。
関数のタイムアウトを延ばすときは、この上限も合わせて見直します。

### 3.4 Masking

資格情報は、値を得た直後、その値を含み得るほかの出力より先に `::add-mask::` でマスクします。
マスクより前の出力には、マスクが効かないためです。

| 値 | マスクする時点 |
|---|---|
| OIDC JWT の要求用トークン（`ACTIONS_ID_TOKEN_REQUEST_TOKEN`） | 環境変数から読んだ直後。ほかのどの出力よりも先 |
| AWS 用 OIDC JWT | 応答から取り出した直後 |
| AWS 一時認証情報（アクセスキー ID、シークレットアクセスキー、セッショントークン） | AWS STS の応答を受け取った直後 |
| GATES 用 OIDC JWT | 応答から取り出した直後 |
| Installation Access Token | 応答から取り出した直後。応答のほかの項目を判定する前。post-entrypoint では `STATE_token` から読んだ直後 |

マスクはジョブごとに効くため、メインで登録したトークンのマスクは、別のコンテナで動く post-entrypoint のログにも効きます（[GitHub Constraints](../spec/github-constraints.md#7-github-actions-integration)）。
post-entrypoint でもトークンを読んだ直後にもう一度マスクするのは、マスクがメインの登録の成否に依存しないようにするためです。

### 3.5 Revocation

Docker Action は、受け取ったトークンの失効を 2 つの時点に分けて試みます（[Domain Model](domain.md#61-contexts)）。

- **ジョブの終了時の失効**：`GITHUB_STATE` に書いたトークンを、ジョブの終了時に post-entrypoint で失効させる。`GITHUB_STATE` を `GITHUB_OUTPUT` より先に書くため（[§2.1](#21-inputs-and-outputs)）、後続ステップへ渡したトークンはすべて含む
- **失敗時の失効**：メインの処理がトークンを取り出した後に、後続ステップへ渡す前に失敗するかキャンセルの要求を受けた場合に、そのトークンをメインの処理がその場で失効させる

失敗時の失効をジョブの終了時に任せないのは、`GITHUB_STATE` に書く前の失敗では post-entrypoint にトークンが渡らず、誰も持たないトークンが有効期限まで GitHub 側に残るためです。
失敗時の失効は、トークンを取り出した後のどの失敗でも試みます。成功応答の残りの項目が不正な場合、`GITHUB_STATE` か `GITHUB_OUTPUT` への書き込みに失敗した場合、`GITHUB_STATE` に書く前にキャンセルの要求を受けた場合（[§3.6](#36-cancellation)）がこれに当たり、トークンのマスクや成功のログを標準出力に書けなかった場合と、想定外の失敗（panic）も含みます。
失敗で終わるステップのトークンを残さないためです。
失効を試みた後も、メインの処理は終了コード 1 で終えます。
`GITHUB_STATE` に書いた後に `GITHUB_OUTPUT` への書き込みに失敗した場合は、post-entrypoint も同じトークンの失効を試みます。
この要求は失効済みのトークンで送るため `204` 以外で終わり得ます。その場合も警告を出すだけで、ジョブの結果は変えません。

post-entrypoint は、メインと同じイメージの新しいコンテナで動き、メインが `GITHUB_STATE` に書いたトークンを `STATE_token` の環境変数で受け取ります。
`runs.post-if` は書かず、既定の `always()` のまま、メインの成否によらず動かします。

1. `STATE_token` がないか空なら、何もせずに終える。メインの処理が `GITHUB_STATE` に書く前に失敗かキャンセルで終えたジョブに当たる
2. トークンをマスクする
3. 失効の要求を送り、結果をログに出す

失効の要求とログへの出し方は、2 つの失効で同じです。

- `https://api.github.com/installation/token` へ、トークンを `Authorization: Bearer` のヘッダーに入れて `DELETE` を送る。Lambda と同じ版の `X-GitHub-Api-Version` と、GATES の Docker Action を示す固定の `User-Agent`（`gates-issue-token`）を付ける。リダイレクトには従わない
- HTTP ステータスをログに出す。`204` 以外か、通信に失敗した場合は、HTTP ステータスか失敗の種類を `::warning::` で出す。応答本文は出さない

post-entrypoint は、失効の成否によらず終了コード 0 で終えます。
失効はベストエフォートであり、トークンは GitHub が定めた有効期限を過ぎれば使えなくなるため、失効の失敗でジョブを失敗させるより、利用者の処理の結果をそのまま残すことを選びます。

失効の宛先は固定し、Runner の環境変数（`GITHUB_API_URL` など）から読みません。
GATES は GitHub.com だけを対象とし（[Requirements](../spec/requirements.md) §2.2）、トークンを送る宛先を実行環境の値で変えられないようにするためです。
`X-GitHub-Api-Version` を Lambda と同じ版にするのは、Lambda Design が GitHub API を確認した版にそろえるためです（[Lambda Design](lambda.md#63-github-client)）。

入力で失効を止める手段は設けません。
Action はジョブ間のトークン転送を提供せず（[§2.1](#21-inputs-and-outputs)）、ジョブの終了後にトークンを使う正当な経路がないためです。

### 3.6 Cancellation

ジョブをキャンセルすると、Runner は Docker Action のコンテナを起動した `docker run` のプロセスに SIGINT を送ります。
7.5 秒で終わらなければ SIGTERM を送り、さらに 2.5 秒で終わらなければ `docker run` のプロセスを強制終了します（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md#runner-source)）。
`docker run` は、受け取ったシグナルをコンテナの最初のプロセスである親へ中継します（`--sig-proxy` の既定値。[Docker Docs — docker container run](https://docs.docker.com/reference/cli/docker/container/run/)）。
Runner はコンテナに `docker kill` も `docker stop` も送らないため、`docker run` のプロセスを強制終了した後もコンテナは動き続けます（推論：コンテナのプロセスは `docker run` のプロセスの子ではなく、強制終了するプロセスの木に含まれないため）。

シグナルを受けた子をその場で終えると、トークンが残り得ます。
Lambda がトークンを発行した後、子が `GITHUB_STATE` に書く前に終わると、メインの処理も post-entrypoint もそのトークンを失効できないためです。
そのため、Docker Action はシグナルをキャンセルの要求として受け取り、要求を受けた時点に応じて終えます。

- メインの親は、SIGINT か SIGTERM を受けると同じシグナルを子へ送り、子の終了を待ち続ける。親が先に終わると、カーネルが同じコンテナの子を強制終了するため
- メインの子は、Invoke を始める前に要求を受けたら、進行中の通信を打ち切り、キャンセルされたことを `::error::` に書いて終了コード 1 で終える。Lambda をまだ呼んでいないため、トークンは発行されていない
- メインの子は、Invoke の最中に要求を受けても Invoke を打ち切らず、[§3.3](#33-retry-and-timeout) の上限まで結果を待つ。打ち切ると、Lambda が発行したトークンを受け取れず、Docker Action も Lambda もそのトークンを失効できないため
- メインの子は、トークンを取り出してマスクした後、残りの項目を判定する前に要求の有無を確かめる。要求があれば、Output と `GITHUB_STATE` に書かずに失敗時の失効を試み（[§3.5](#35-revocation)）、キャンセルされたことを `::error::` に書いて終了コード 1 で終える
- メインの子は、この確認の後に要求を受けたら、残りの判定と書き込みを要求がない場合と同じく終える。Runner はキャンセルで終わったステップの `GITHUB_STATE` も読むため（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md#runner-source)）、書いたトークンはジョブの終了時の失効が扱う
- post-entrypoint の子は、SIGINT と SIGTERM を無視し、失効を終えてから終える。失効の上限の 5 秒は、SIGINT から強制終了までの 10 秒より短いため
- post-entrypoint の親も、SIGINT と SIGTERM を無視し、子へ送らない。子が無視を設定する前に届いたシグナルで子が終わると、失効を試みないまま終わるため

子のシグナルの受け方は `main.go` が Go の `os/signal` で設定します。
メインの子では SIGINT と SIGTERM を受け取ってキャンセルの要求として入口関数に渡し、post-entrypoint の子では無視します。
どちらも設定しないと、Go のランタイムがシグナルを受けた子を終えるためです。
入口関数はキャンセルの要求を ctx のキャンセルとして受け、Invoke と失敗時の失効には、キャンセルを引き継がない ctx を渡します。

Invoke の結果を待つ間に SIGINT から 10 秒を過ぎると、Runner は `docker run` のプロセスを強制終了します。
その後もコンテナは動き続けるため、子はトークンを受け取れば失敗時の失効を試みます。
ただしこの場合、失効の結果はワークフローのログに残りません（推論：Runner はコンテナの出力を `docker run` のプロセスの標準出力から読むため）。

キャンセルしても、Runner がコンテナを強制終了した場合や、Runner が停止した場合は、子が失効を試みる前に終わり得ます。
このリスクは [Security Design](security.md#5-failure-audit-and-security-risks) が受容します。

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

ジョブの最後に、Runner は `Post Issue token` のステップとして post-entrypoint を動かし、トークンの失効を試みます。

## 5. Technology Decisions

| 用途 | 採用技術・方針 | 理由 |
|---|---|---|
| Action の形式 | Docker Action（`runs.using: docker`、`image: Dockerfile`） | 利用者の Runner がジョブのたびに、利用者が commit SHA で固定した参照先の `Dockerfile` からイメージをビルドする。レジストリにイメージを公開する作業と、公開したイメージを信頼する前提が要らない |
| AWS STS と Lambda Invoke | AWS SDK for Go v2（`service/sts`、`service/lambda`） | SigV4 の署名と AWS STS の呼び出しは、誤りが脆弱性になるため SDK に委ねる |
| OIDC JWT の取得、失効、ワークフローコマンド、Payload と応答の JSON | Go の標準ライブラリ | 処理が小さく、外部ライブラリの依存を増やすほどの理由がない |

外部ライブラリは AWS SDK for Go v2 だけとします。
ワークフローコマンドのライブラリ（actions/toolkit に相当する Go のライブラリ）も使わず、符号化と書き込みを自前で書きます。

### 5.1 Exceptions to the Go Coding Rules

Docker Action の Go のコードは、Go コーディングルールが既定とする方式のうち、次の 2 つを使いません。
どちらもモジュール全体に及ぶため、コードのコメントではなく本ドキュメントに理由を書きます。

- **cobra を使わない**：Docker Action は flag を受け取らず、入力は GitHub が名前を定める `INPUT_*` の環境変数で届く。外部の標準で名前が決まっている入力に当たる。メインと post-entrypoint の区別は、起動したパスで行う（[§3.1](#31-packages)）
- **slog を使わない**：Docker Action の出力は、Runner が読むワークフローコマンド（`::add-mask::`、`::error::`、`::warning::`）と、利用者が読む数行の情報であり、構造化したログを読む主体がない。失敗は `::error::` の注釈を標準出力に書き、終了コード 1 で終える

### 5.2 Image

- マルチステージビルドとし、ビルド用のイメージは `docker.io/library/golang` の Debian 版（`<版>-trixie`）を、タグと digest で固定する。Go の版は、Docker Action の Go モジュールの `go.mod`（`.github/actions/issue-token/go.mod`）の `go` の版と同じマイナー版の、最新のパッチ版とする。`go` の版は言語の最低の版で、パッチ版は標準ライブラリの脆弱性の修正を含むため。`GOTOOLCHAIN=local` ではビルド用のイメージの Go が `go.mod` の版より古いとビルドが失敗するため、`go.mod` の版を上げるときはビルド用のイメージも合わせて上げる
- `CGO_ENABLED=0`、`-trimpath`、`-buildvcs=false`、`GOTOOLCHAIN=local` でビルドする。`go build` は既定で `go.mod` と `go.sum` を書き換えず、`go.sum` にない依存を取り込まない
- 実行用のイメージは `scratch` に、実行ファイル（`/issue-token` と、それを指すシンボリックリンクの `/revoke-token`）と、ビルド用のイメージから複製した CA 証明書（`/etc/ssl/certs/ca-certificates.crt`）だけを置く。GitHub と AWS への HTTPS に CA 証明書が要る
- `ENTRYPOINT` は exec 形式と絶対パスで書き、`WORKDIR` を使わない
- `.dockerignore` は `*` ですべて除外してから、ビルドに使うファイル（`go.mod`、`go.sum`、`main.go`、`internal/` の試験以外のファイル）だけを許可する
- `USER` を置かず、root で動かす
- Dependabot の `docker` で `FROM` のイメージを、`gomod` で Go の依存を更新する。`docker` が `FROM` の digest も更新するかは、GitHub の資料に明記がない（[GitHub Constraints](../spec/github-constraints.md#7-github-actions-integration)）

ビルド用のイメージを Alpine 版ではなく Debian 版にするのは、Alpine 版が Go プロジェクトの公式サポートの対象外だからです（[Docker Hub — golang](https://hub.docker.com/_/golang)）。
`CGO_ENABLED=0` でビルドするため、C ライブラリの違いは実行ファイルに影響しません。
イメージのビルドの時間は、Alpine 版と Debian 版でほぼ同じでした（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md#build-time)）。

実行ユーザーを root にするのは、root 以外では `GITHUB_OUTPUT` と `GITHUB_STATE` に書けないためです。
GitHub の資料は Docker Action を既定の Docker ユーザー（root）で動かすよう求めています（[GitHub Constraints](../spec/github-constraints.md#7-github-actions-integration)）。
ファイルコマンドのファイルの所有者は Runner を動かすユーザーで、ほかのユーザーに書き込みの権限がなく、その uid は Runner の環境ごとに決まるため、イメージに固定の uid を置いても書き込める保証がありません。
コンテナに入るのはこのリポジトリでビルドした実行ファイルと CA 証明書だけであり、root で動くことの影響は Security Design が受容するリスクの範囲にとどめます（[Security Design](security.md#5-failure-audit-and-security-risks)）。

### 5.3 Image Build Time

Docker Action のイメージは、利用者の Runner がジョブのたびにビルドします。
利用者向けの README には、ジョブのたびにイメージをビルドすることを書き、時間の値は書きません。
時間は Runner と回線によって変わり、値を書くと実測と食い違い得るためです。

### 5.4 Runner Requirements

Docker Action は、Linux の Runner と Docker を要します（[GitHub Constraints](../spec/github-constraints.md#7-github-actions-integration)）。
加えて、Runner から次の宛先へ通信できる必要があります。

- イメージのビルド：Docker Hub（ビルド用のイメージの取得）と Go module proxy（`proxy.golang.org`、Go の依存の取得）
- メインの処理：`ACTIONS_ID_TOKEN_REQUEST_URL` の宛先（OIDC JWT の取得）、入力の `aws-region` の AWS STS と Lambda の API
- 失効：`api.github.com`

GitHub がホストする `ubuntu-latest` の Runner はこれらをすべて満たし（[GitHub — runner-images](https://github.com/actions/runner-images)）、利用者の準備は要りません。
セルフホストの Runner では、利用者が Linux と Docker を用意し、これらの宛先への通信を許可します。
Docker Hub は認証しないイメージの取得の回数を制限するため、ジョブのたびにビルドするセルフホストの Runner では、利用者がこの制限に達しないように Docker Hub の認証やミラーを用意します（[Docker Docs — Usage and limits](https://docs.docker.com/docker-hub/usage/)）。
Windows と macOS の Runner では Docker Action が動かないため、対象外とします。

## 6. Testability

受け入れ条件は [Requirements](../spec/requirements.md) を正本とします。
GitHub の Installation Access Token は固定長と仮定せず、トークンの形式と長さによらず、マスク、非記録、Output と `GITHUB_STATE` への書き込みが正しく行われることを検証します。

### 6.1 Local Tests

[3.1 Packages](#31-packages) の `internal/` の package は、AWS と GitHub に接続しない Go の単体テストで検証します。
通信する package の試験では、`httptest` のサーバーを相手にします。
OIDC JWT の取得と失効は宛先の URL を、AWS STS と Lambda のクライアントは AWS SDK の `BaseEndpoint` を、試験のサーバーに差し替えます。
`internal/` のカバレッジは 100% とします。

ローカルの試験で確かめるのは次の事項です。

- 入力のリストの検証（[§2.2](#22-list-syntax)）と、Payload への変換
- OIDC JWT の取得の要求（Audience の URI エンコード、ヘッダー）と、応答の `value` の取り出し、リダイレクトと応答の大きさの上限
- SDK のエラー、`StatusCode`、`FunctionError`、`ok` の判定と、各段階でログに出すものと出さないもの（[3.2 Failure Output](#32-failure-output)）
- 応答の配列とオブジェクトからカンマ区切りへの変換と、`GITHUB_OUTPUT` に書く 4 行と `GITHUB_STATE` に書く 1 行、その書く順序
- `GITHUB_OUTPUT` と `GITHUB_STATE` への書き込みの失敗がエラー終了になること
- 資格情報の `::add-mask::` が、その値を含み得るほかの出力より先にあること
- 失効の要求（宛先、ヘッダー）と、HTTP ステータスごとのログ、トークンがないときに何もしないこと
- 通信の失敗の種類の判定が、OIDC JWT の取得、AWS STS、Invoke、失効で同じであること
- AWS のクライアントが、`AWS_*` の環境変数と共有の設定ファイルがあっても、組み立てた接続先、認証情報、再試行で要求を送ること
- トークンを取り出した後の失敗（成功応答の残りの項目が不正な場合、`GITHUB_STATE` と `GITHUB_OUTPUT` への書き込みの失敗、想定外の失敗（panic））のそれぞれで、メインの処理が失効を試みてから終了コード 1 で終えること
- トークンを取り出す前の失敗では、メインの処理が失効を試みないこと
- 試験のサーバーへの要求の回数が、失敗した場合も 1 回であること
- メインの親が、子の終了コード 0 と 1 をそのまま返し、それ以外の終了コードとシグナルによる終了と起動の失敗では固定の文言の `::error::` だけを書いて 1 を返すこと。post-entrypoint の親が、子の 0 以外の終わり方と起動の失敗では固定の文言の `::warning::` だけを書き、どの場合も 0 を返すこと。別の goroutine で panic する子を起動し、panic の値が親の出力に現れないことを確かめる
- メインの親が、受けた SIGINT と SIGTERM を子へ送り、子が終わるまで待つこと
- 親が起動した子に、親の環境変数がそのまま渡ること
- メインの子が、キャンセルの要求を受けた時点（Invoke を始める前、Invoke の最中、トークンを取り出した後、その後の確認を過ぎた後）ごとに、[§3.6](#36-cancellation) のとおりに終えること。試験では、シグナルの代わりにキャンセルの要求を入口関数に渡す

親の試験は、試験の実行ファイル自身を子として起動し、子の振る舞いを本番と同じく `argv[0]` の名前で選びます。
このため、親の試験のコードは、子の振る舞いのために `os.Args`、`os.Exit`、`os.Getenv`、`os/signal`、`panic` を使います。
メインの入口関数の試験も、段階の途中の panic を起こすために、標準出力の書き込み先で `panic` を使います。

### 6.2 Tests on GitHub Actions

GitHub と AWS の挙動に依存する事項は、許可オーナーのワークフローから Docker Action を呼んで確かめます。

- 接続情報と対象と権限だけを渡してトークンを受け取り、同一ジョブ内の後続ステップで使えること
- ワークフローのログで、GATES 用 OIDC JWT、AWS 用 OIDC JWT、AWS 一時認証情報、トークンがマスクされているか現れないこと
- 呼び出し元の後続ステップの環境変数に、AWS 一時認証情報が現れないこと
- AWS STS での引き受けの失敗、Lambda の Invoke API の拒否、関数の `FunctionError`、Lambda が返す失敗で、Docker Action がエラー終了すること。Invoke API の拒否と `FunctionError` は、検証環境で同時実行数を 0 にした状態、固定設定の環境変数を欠いた状態、関数のタイムアウトを短くした状態で起こし、Docker Action が [§3.2](#32-failure-output) の段階の文言だけを出すことを確かめる（[AWS Fault Injection](../research/aws/fault-injection.md)）
- メインと post-entrypoint のそれぞれで、`main.go` が起動したパス（`os.Args[0]`）から正しい入口関数を選ぶこと
- ジョブの終了時に post-entrypoint がトークンの失効を試み、HTTP ステータスをログに出し、失効したトークンが使えないこと
- 後続ステップが失敗したジョブと、後続ステップの実行中にキャンセルしたジョブでも、post-entrypoint がトークンの失効を試みること
- メインがトークンを取り出す前に失敗したジョブで、`continue-on-error` の有無によらず、post-entrypoint が何もせずに終わること
- メインの処理の最中に SIGINT を受けたジョブで、`docker run` がシグナルをコンテナへ中継し、親が子へ送り、子が [§3.6](#36-cancellation) のとおりに終えること。Invoke の前と Invoke の開始の後の両方で確かめる。`docker run` のプロセスを強制終了した後もコンテナが動き続けることは、実環境で確かめていない
- 別のリポジトリから commit SHA で参照したとき、Runner が `Dockerfile` からイメージをビルドして動かせること

失効したトークンが使えないことは、post-entrypoint より後に動くステップから確かめます。
post-entrypoint はジョブの最後に動くため、利用者のステップからはトークンを試せないためです。
試験のワークフローでは、Docker Action のステップより前に、post でファイルからトークンを読んで GitHub API を呼ぶ試験用の Action を置き、Docker Action の後続ステップがトークンを `RUNNER_TEMP` のファイルに書きます。
この確かめ方は、post がメインの処理と逆の順で動き、試験用の Action の post が Docker Action の post-entrypoint より後に動くことを前提とします（推論：GitHub の資料に明記はない）。
試験では、2 つの post の順序もログで確かめます。

## 7. Sources

- [GitHub Constraints](../spec/github-constraints.md)
- [GitHub Docker Action Runtime](../research/github/docker-action-runtime.md)
- [GitHub — Creating a Docker container action](https://docs.github.com/en/actions/tutorials/use-containerized-services/create-a-docker-container-action)
- [GitHub — Dockerfile support for GitHub Actions](https://docs.github.com/en/actions/reference/workflows-and-actions/dockerfile-support)
- [GitHub — Metadata syntax reference](https://docs.github.com/en/actions/reference/workflows-and-actions/metadata-syntax)
- [GitHub — Workflow commands for GitHub Actions](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands)
- [GitHub — actions/toolkit command.ts](https://github.com/actions/toolkit/blob/main/packages/core/src/command.ts)
- [GitHub — actions/runner ActionCommand.cs](https://github.com/actions/runner/blob/v2.338.2/src/Runner.Common/ActionCommand.cs)
- [GitHub — runner-images](https://github.com/actions/runner-images)
- [GitHub — REST API endpoints for repository contents](https://docs.github.com/en/rest/repos/contents)
- [GitHub — REST API endpoints for GitHub App installations](https://docs.github.com/en/rest/apps/installations)
- [AWS — AssumeRoleWithWebIdentity](https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRoleWithWebIdentity.html)
- [AWS — Lambda Invoke API](https://docs.aws.amazon.com/lambda/latest/api/API_Invoke.html)
- [Docker Hub — golang](https://hub.docker.com/_/golang)
- [Docker Docs — docker container run](https://docs.docker.com/reference/cli/docker/container/run/)

## Appendix: Impact of Future Scope

[Future Scope](../spec/future-scope.md) が想定する機能を取り込む場合、本ドキュメントには次の影響が及びます。

- **細粒度の認可**：発行の可否の判定は Lambda 側が行う（§1）ため、利用者が指定する入力が増えない限り、Docker Action は変わらない
- **発行可能な権限の拡大**：`permissions` 入力の値（§2.1）と利用例（§4.1）が変わる。許可条件の判定を Lambda の認可に委ねる方針（§2.2）は変わらない。Account / Organization に関係する権限の対象を `repositories` 入力で表せないなら、入力も見直す
