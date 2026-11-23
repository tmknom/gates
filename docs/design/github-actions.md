# GitHub Actions Design

## 1. Responsibility

Docker Action は、GitHub Actions の利用者にトークンの取得手段を提供します。
AWS 用 OIDC JWT による AWS STS からの一時認証情報の取得、GATES 用 OIDC JWT の取得、Lambda の同期 Invoke、応答の処理と同一ジョブ内の後続ステップへのトークンの出力、取り出したトークンの失効を担当します。
発行の可否の判定は Lambda 側が行います。

Docker Action は GATES 運用者が提供し、利用者向けの安定した境界とします。
利用者に求めるのは、ワークフローでの `permissions: id-token: write` の宣言、対象リポジトリと権限の指定、運用者が用意した接続情報の参照だけです（セルフホストの Runner では [Runner Requirements](#43-runner-requirements) も求めます）。

Docker Action は、Lambda と同じこのリポジトリの `actions/issue-token/` に置きます。
Lambda の入出力契約と Docker Action を同じ PR で変更でき、両者の食い違いを防げるためです。
`.github/` の下に置かないのは、Dependabot が `github-actions` 以外の ecosystem では `.github/` の下のファイルを変える PR を作れず、Docker Action の Go の依存とビルド用のイメージが更新されなくなるためです（[dependabot-core#15237](https://github.com/dependabot/dependabot-core/issues/15237)）。
Docker Action はデプロイ固有の値を持たないため、公開リポジトリに置いても AWS アカウント ID は公開されず、許可オーナーのワークフローもこの Docker Action を参照できます。

IAM Role と Lambda の AWS リソース設定は [AWS Infrastructure Design](aws-infrastructure.md) に、Lambda Invoke の Payload、応答、エラーコードは [Lambda Design](lambda.md) に定めます。

## 2. Docker Action Interface

### 2.1 Inputs and Outputs

| Input | 意味 |
|---|---|
| `repositories` | カンマ区切りの `owner/repo` のリスト |
| `permissions` | カンマ区切りの `permission:level` のリスト |
| `role-arn` | 引き受ける IAM Role の ARN |
| `function-name` | 呼び出す Lambda の関数名 |
| `aws-region` | 呼び出す Lambda を置く AWS リージョン。AWS STS の呼び出しにも使う |
| `audience` | GATES 用 OIDC JWT の Audience |

入力はどれも必須です。
`permissions` に書ける組と、保護対象を含む要求の条件は Lambda の認可が決めます（[Security Design](security.md#31-authorization-rule)）。
全リポジトリを対象にする入力は設けません。
GATES は対象を明示した要求だけを受け付けるためです。

`role-arn`、`function-name`、`aws-region`、`audience` を **接続情報** と呼びます。
接続情報は Docker Action の中に固定せず、入力として受け取ります。
Docker Action の定義にデプロイ固有の値を持たせないためです。
接続情報は GATES 運用者が GitHub の Secrets と Variables に用意し（AWS アカウント ID は Secrets、その他は Variables）、ワークフローはそれを参照します。
ワークフローに値を直接書かないのは、Role 名や関数名を変えたときに、Secrets と Variables の更新だけで済ませ、利用者のワークフロー定義を変えずに済ませるためです（[Acceptance Criteria](../spec/acceptance-criteria.md) AC-OPS-010）。
Secrets と Variables を置ける単位はオーナーの種別とプランで異なりますが（[GitHub Actions Integration Constraints](../external/github/actions.md)）、`secrets.*` と `vars.*` の参照の書き方は同じため、どの単位でも利用者のワークフロー定義は変わりません。
`function-name` に ARN ではなく関数名を渡すのは、関数名は引き受けた IAM Role と同じアカウントの関数として解決され、AWS アカウント ID を含めずに済むためです。

接続情報を利用者が入力しても、発行範囲は広がりません。
IAM Role の引き受けは Trust Policy が不変 Owner ID 入りの `sub` で制限し、引き受けた Role に許可されるのは GATES 用 Lambda の Invoke だけです（[AWS Infrastructure Design](aws-infrastructure.md#31-github-actions-iam-role)）。
Audience は Lambda が GATES 用の値との完全一致を検証します（[OIDC JWT Claims](oidc-jwt-claims.md) の GOJ-AUD）。
別の値を渡した要求は、いずれかの境界で失敗します。

入力は、Runner が `INPUT_<大文字の入力名>` の環境変数としてコンテナに渡すものを読み、`action.yml` に `args` を書きません。
Runner はコンテナを起動する `docker run` の行に環境変数の名前だけを載せ、値を載せないためです（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md#inputs)）。

| Output | 意味 |
|---|---|
| `token` | 発行された Installation Access Token |
| `expires-at` | GitHub が定めた有効期限（RFC 3339） |
| `repositories` | Lambda の応答が示す発行対象。入力と同じ形式のカンマ区切りの `owner/repo` |
| `permissions` | Lambda の応答が示す実効権限。入力と同じ形式のカンマ区切りの `permission:level`。要求にかかわらず GitHub が付与する `metadata:read` を含み得る |

対象と実効権限を Output に含めるのは、利用者が受け取ったトークンの範囲を確かめられるようにするためです（[Requirements](../spec/requirements.md) FR-ISSUANCE-005）。
Output は同一ジョブ内の後続ステップが使い、Docker Action はジョブ間のトークンの転送を提供しません。

Output は `GITHUB_OUTPUT` に `name=value` の 1 行の形式で書くため、値を書き込む前に次の値を不正な応答として拒否します。

- **改行を含む値**：別の Output の行を書き足せるため
- **区切りの文字を含む要素**：`repositories` の要素の `,` と、`permissions` の権限名か水準の `,` と `:`。後続ステップは Output を入力と同じ形式として読むため、応答が示した範囲と食い違って読まれる
- **空の値と空の要素**：空の `expires-at` では利用者が有効期限を確かめられず、空の `request_id` では運用者が監査記録を探せない
- **`##[` を含む値**：`token` 以外の Output と `request_id`。これらは成功のログの行にも出し、Runner は `::` で始まるワークフローコマンドを先頭の空白を除いた行頭でだけ読む一方、`##[` で始まる旧形式のワークフローコマンドは行の途中からも読むため（[GitHub — actions/runner ActionCommand.cs](https://github.com/actions/runner/blob/v2.338.2/src/Runner.Common/ActionCommand.cs) の `TryParseV2` と `TryParse`）

トークンは、ジョブの終了時の失効（[Revocation](#35-revocation)）へ引き継ぐため、`GITHUB_STATE` にも書きます。
`GITHUB_STATE` は `GITHUB_OUTPUT` と同じ Runner のファイルコマンドのディレクトリにあり、トークンを書く場所は増えても、読める主体は増えません。

Output と `GITHUB_STATE` は、応答をすべて判定して成功が決まった後にだけ、`GITHUB_STATE`、`GITHUB_OUTPUT` の順に書きます。

- 成功が決まる前に書かないのは、メインの処理が失敗で終わっても、書いた Output は後続ステップに届くためである（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md#output-and-mask)）
- `GITHUB_STATE` を先に書くのは、後続ステップに渡したトークンが、ジョブの終了時の失効の対象から漏れないようにするためである
- どちらへの書き込みの失敗もエラー終了にする。ステップが成功で終わると、後続ステップは空のトークンで動き、失敗の原因を追えないためである

Docker Action の中でマスクしたトークンは、呼び出し元の後続ステップのログでもマスクされるため（[GitHub Actions Integration Constraints](../external/github/actions.md)）、呼び出し元がもう一度マスクする必要はありません。
ただし、マスクは登録した文字列の全体に一致する箇所に効くため、後続ステップがトークンの一部を切り出してログに出すと、その部分はマスクされません（[GitHub Composite Action Outputs](../research/github/composite-action-outputs.md#mask)）。

### 2.2 List Syntax

区切りは `,` とし、要素の前後の空白（スペース、タブ、CR、LF）を除きます。
空白を除くほかは要素を書き換えず、大文字小文字もそろえずに、入力の順のまま `repositories` を配列に、`permissions` を JSON オブジェクトに変換します。
独自のエスケープ構文は設けません。

Docker Action が検証するのは、リストの構造と、要素が UTF-8 として正しいことだけです。
次のどれかに当たる入力は、Lambda を呼ばずに失敗します。

- 空の要素がある
- `repositories` の要素が `/` をちょうど 1 つ含まないか、`/` の前後のどちらかが、空であるか、空白で始まるか終わる
- `repositories` に、ASCII の範囲で大文字小文字を区別せずに比べて同じ要素が 2 つ以上ある
- `permissions` の要素が `:` をちょうど 1 つ含まないか、`:` の前後のどちらかが、空であるか、空白で始まるか終わる
- `permissions` に、権限名が文字列として完全に一致する要素が 2 つ以上ある
- 要素が UTF-8 として正しくない

空の要素や区切りの誤りは Lambda も `invalid_request` で拒否しますが、その `message` は固定の文言で、どの要素が不正かを示しません。
Docker Action で先に止めると、利用者に該当する要素を示せます。
UTF-8 として正しくない要素は、Payload の JSON の文字列にすると別の文字に置き換わり、要素を書き換えない変換にならないため、Docker Action で止めます。
`permissions` の権限名の重複は、Lambda に任せられません。
JSON オブジェクトに変換すると後の要素が前の要素を上書きし、`contents:write,contents:read` が `{"contents":"read"}` として送られ、利用者が書いた要求と監査記録に残る要求が食い違うためです。

名前と権限の文字種、対象の件数の上限、許可条件に合うかは、Docker Action では判定せず、Lambda が `invalid_request` か `authorization_denied` で返します（[Lambda Design](lambda.md#22-validation)）。
リストの構造は GATES が決めた文字種や上限に左右されませんが、これらの規則は GATES の判断で変わり得ます。
Docker Action に写すと規則の定義が 2 か所になり、Lambda の規則だけを変えたときに、Lambda が受け付ける要求を Docker Action が拒否し得ます。
許可条件の判定を Lambda に委ねるのは、認証済み Caller とともに許可外の要求を監査記録に残すためです。

## 3. Processing Flow

Docker Action のメインの処理は、次の段階を 1 つの子のプロセスで順に行います。

1. 前提を確かめる。OIDC JWT の要求先と要求用トークン（`ACTIONS_ID_TOKEN_REQUEST_URL` と `ACTIONS_ID_TOKEN_REQUEST_TOKEN`）、`GITHUB_OUTPUT` と `GITHUB_STATE` があり、入力が空でないことである。要求用トークンがなければ、`permissions: id-token: write` の宣言を促して失敗する。要求先がホストを持つ `https` の URL であることも通信の前に確かめ、要求用トークンを平文で送らない
2. 入力のリストを検証し、Payload の `repositories` と `permissions` を作る
3. `sts.amazonaws.com` を Audience とする AWS 用 OIDC JWT を取得する
4. AWS STS の `AssumeRoleWithWebIdentity` で、入力の `role-arn` の IAM Role を引き受ける。AWS 用 OIDC JWT はこの段階の後に使わない
5. 入力の `audience` を Audience とする GATES 用 OIDC JWT を取得する
6. [Lambda Request](lambda.md#21-invoke-payload) の Payload を組み立て、Invoke の時刻をログに出してから、一時認証情報で Lambda を同期 Invoke する。`InvocationType` は `RequestResponse` を明示し、呼び方を Lambda の既定に任せない。`LogType` を指定せず、関数のログを応答に含めない
7. 結果を、SDK のエラー、`StatusCode`、`FunctionError`、応答の `ok` の順に判定し、成功ならトークンを取り出す。取り出すのは、空でなく ASCII の制御文字を含まない文字列だけである。改行は 1 行の形式で書く `GITHUB_STATE` と `GITHUB_OUTPUT` に別の行を書き足せ、ほかの制御文字は失効の要求のヘッダーに載らないためで、トークンの文字種に制御文字はない
8. トークンをマスクし、キャンセルの要求を確かめ（[Cancellation](#36-cancellation)）、成功応答の残りの項目を判定する。すべて満たせば、`GITHUB_STATE` と `GITHUB_OUTPUT` に書き（[Inputs and Outputs](#21-inputs-and-outputs)）、`request_id`、`expires_at`、対象、実効権限をログに出す。この段階で失敗すれば、失敗時の失効を試みてから終える

前提を最初に確かめるのは、OIDC JWT の取得と AWS STS が `id-token: write` を要し、欠けたまま進むと原因の分かりにくい失敗になるためです。
入力の検証を AWS STS より前に置くのは、通信の要らない失敗を先に返し、検証に失敗する入力で IAM Role を引き受けないためです。

GATES 用 OIDC JWT、AWS 用 OIDC JWT、AWS 一時認証情報、Payload、応答は、プロセスのメモリの中だけで扱い、ファイルに書きません。
資格情報は、成功と失敗のどちらでもログへ出しません。
AWS STS の一時認証情報は、環境変数にも `GITHUB_ENV` にも書きません。
書くと、後続ステップが一時認証情報を有効期限まで使え、利用者が先に設定した AWS の認証情報とリージョンも置き換わるためです。
`AssumeRoleWithWebIdentity` では有効期間に下限の 900 秒を指定します。
一時認証情報を使うのは 1 回の Invoke だけで、長く有効にしておく理由がないためです。
資格情報をプロセスのメモリの中だけで扱うことは、Runner で動く信頼できないコードから資格情報を守るものではなく、同じジョブで信頼できないコードを動かさないことは利用者の責任です（[Security Design](security.md#5-failure-audit-and-security-risks)）。

OIDC JWT の取得では、次の規則に従います。

- リダイレクトには従わず、`3xx` は失敗にする。従うと、要求用トークンを含むヘッダーを別の宛先へ送り得るためである
- 応答本文の大きさに上限を設ける。JWT は数 KB であり、上限がないと、想定と異なる応答でプロセスのメモリを使い切り得るためである
- 応答が JSON のオブジェクト 1 つでないか、`value` が改行を含めば失敗にする。JWS Compact Serialization の構成要素は改行を含められないため（[OIDC JWT Signature](oidc-jwt-signature.md)）、改行を含む `value` は JWT ではない

### 3.1 Packages

Docker Action は `actions/issue-token/` を独立した Go モジュールとし、`action.yml` と `Dockerfile` を同じディレクトリに置きます。
Runner は `Dockerfile` のあるディレクトリをビルドコンテキストにし、その外のファイルを `Dockerfile` から参照できないため（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md#runner-source)）、イメージのビルドに要る `go.mod` と `go.sum` もこのディレクトリに置く必要があるためです。

package は次の規則で分けます。

- **通信する package と値だけを扱う package を分ける**：入力の検証、Payload の組み立て、応答の判定、Output への変換の試験で通信を不要にするため
- **環境変数、引数、標準出力、終了コード、シグナルに触れるのはエントリーポイントだけにする**：ほかの package は読み取った値と書き込み先を引数で受け取るため、環境変数とファイルを用意せずに試験できる
- **資格情報の値を取り出せる package を、その値を送る箇所に限る**：要求用トークンは OIDC JWT の要求に、OIDC JWT は AWS STS の要求と Payload に、AWS 一時認証情報は Invoke の署名に、Installation Access Token は `GITHUB_STATE` と Output への書き込みと失効の要求に限る。資格情報は値を `fmt` と JSON で出さない型で持ち、取り出す口を呼べる package を、マスクの登録を除いて lint で限る。誤ってログや応答に値を出す箇所を、取り出せる package の中に閉じるため

メインと post-entrypoint、親と子は、同じ実行ファイルを起動したパスの末尾の名前で区別します。
イメージにはメイン用の実行ファイルと、それを指す post-entrypoint 用のシンボリックリンクを置き、`action.yml` の `entrypoint` と `post-entrypoint` がそれぞれを指します。
実行ファイルを 2 つ複製しないのは、AWS SDK を含む実行ファイルの大きさの分だけ、ジョブのたびにビルドするイメージが大きくなるためです。
親は、起動したパスの名前に `-child` を付けた名前を `argv[0]` にして、自身の実行ファイルを子として起動します。
子の名前は `action.yml` が固定するパスに現れないため、利用者は子を直接起動させられません。
ほかの手段で区別しないのは、次の理由からです。

- **環境変数**：利用者がステップの `env:` で同じ環境変数を渡すと、親の見張りを外せる
- **引数**：Runner は `runs.args` をメインと post-entrypoint で同じように評価して渡し、段階ごとに切り替えるのは `entrypoint` と `post-entrypoint` の値だけである（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md#runner-source)）
- **`STATE_` の環境変数の有無**：メインが失敗して `GITHUB_STATE` を書かなかったジョブの post-entrypoint が、メインの処理をもう一度行うことになる

### 3.2 Failure Output

Docker Action のメインの処理は、どの段階で失敗しても終了コード 1 で終え、`::error::` の注釈に、失敗した段階を示す固定の文言と原因を書きます。
原因として書くのは、次の値です。

- 満たさなかった前提と、入力の検証で満たさなかった条件と該当する要素
- HTTP ステータス、AWS のエラーコード（`AccessDenied`、`AccessDeniedException` など）、通信の失敗の種類（タイムアウトか、それ以外か）
- OIDC JWT の取得の `4xx` と `5xx` の応答本文の先頭の一部
- `StatusCode`、`FunctionError` の値と応答の `errorType`、`ok` が `false` の応答の `error.code`、`error.message`、`request_id`
- 書き込めなかったファイルの環境変数の名前

次の値は書きません。

- **資格情報と、`ACTIONS_ID_TOKEN_REQUEST_URL` の値**
- **AWS のエラー応答の文言、SDK とライブラリのエラー文言**：例外によっては文言に要求の一部が入り、Invoke の Payload には GATES 用 OIDC JWT が、`AssumeRoleWithWebIdentity` の要求には AWS 用 OIDC JWT が入る。Lambda Developer Guide の例では、JSON として読めない Payload への `InvalidRequestContentException` の文言に Payload の一部が入る（[AWS Lambda Invoke Rejection](../research/aws/lambda-invoke-rejection.md)）。どの例外の文言が要求の一部を含むかは列挙できないため、例外によらずエラーコードと HTTP ステータスだけを書く。ライブラリの文言にも、接続先の URL や要求の一部が入り得る
- **GATES が内容を定めていない応答の本文**：`FunctionError` の応答の `errorMessage`、JSON として読めない応答本文、OIDC JWT の取得の `200` 以外の `2xx` と `3xx` の応答本文。資格情報を含まないと保証できず、OIDC JWT の取得の応答本文はマスクする前の JWT を含み得る。`ok` が `false` の `message` はエラーコードごとに固定の文言で、資格情報を含まない
- **文字列でない値と、`##[` を含む値**：Runner が旧形式のワークフローコマンドを行の途中からも読むため
- **書こうとした値と panic の値**：値に資格情報が入っていない保証がない

`FunctionError` のときは応答に `request_id` がないため、運用者は段階 6 で出した Invoke の時刻と `run_id` で CloudWatch Logs の記録を探します。
トークンを取り出した後の失敗では、失敗の注釈に加えて、失敗時の失効の結果をログに出します。

注釈と `::add-mask::` に入れる値は、`%`、CR、LF を `%25`、`%0D`、`%0A` に符号化して 1 行に収めます（[GitHub — actions/toolkit command.ts](https://github.com/actions/toolkit/blob/main/packages/core/src/command.ts) の `escapeData`）。
値に改行が残ると、その後の行を Runner がワークフローコマンドとして読み得るためです。
マスクの値に `%` が符号化されずに残ると、Runner が符号を戻した後の別の文字列をマスクし、資格情報そのものはマスクされません。
入力の検証の注釈では、該当する要素を二重引用符で囲み、制御文字と UTF-8 として正しくないバイトをエスケープして、前後の空白や制御文字を見分けられるようにします。

子のコードが panic するのは、ほかのコードが保証している不変条件が破れた場合だけで、外部入力と外部呼び出しの失敗はエラーで返します。
メインの子は、トークンを取り出した後の panic を一度回復して失敗時の失効を試み、同じ panic を回復の外側に渡し直します。
回復が効くのは入口関数を呼ぶ goroutine の panic だけのため、子のコードは goroutine を起動しません。
一方、`net/http` と AWS SDK が内部で起動する goroutine の panic は回復できず、Go のランタイムが panic の値とスタックを標準エラー出力に書いて、終了コード 2 でプロセスを終えます。
panic の値には資格情報の一部が入り得て、一部だけではログのマスクが効きません。

この出力をログに出さず、落ちたことだけを残すため、資格情報を扱う処理は子のプロセスで行い、親のプロセスが子を見張ります。

- 親は資格情報を読まず、子を起動し、メインでは受けたシグナルを子へ送り、終了を待つだけとする。そのため親の panic の値は資格情報を含まない
- 子の標準出力は親の標準出力を引き継ぎ、子のワークフローコマンドと注釈はそのまま Runner に届く。子の標準エラー出力は捨てる
- 子が使う終了コードは、メインの子では 0 と 1、post-entrypoint の子では 0 だけとする。メインの子が 0 か 1 で終われば、親は同じ終了コードで終える
- メインの子がそれ以外の終了コードかシグナルで終わるか、起動できなければ、親は固定の文言を `::error::` に書き、終了コード 1 で終える。回復されない panic、Go のランタイムの致命的なエラー、メモリ不足による強制終了がこれに当たる

post-entrypoint では、子を起動できないか、子が 0 以外の終了コードかシグナルで終われば、親は固定の文言を `::warning::` に書き、終了コード 0 で終えます。
子が panic を回復した場合も、子は `::warning::` に書き、終了コード 0 で終えます。
失効の失敗を `::warning::` で知らせるのと同じ扱いにするためです。
`::error::` にすると、すべてのステップが成功したジョブにエラーの注釈だけが残り、利用者が存在しない失敗を探すことになります。

### 3.3 Retry and Timeout

Docker Action は、どの要求も自動で送り直しません。
Lambda が発行を終えた後に応答が届かなかった場合、送り直した要求は 2 つ目のトークンを発行し、1 つ目のトークンは誰も持たないまま有効期限まで GitHub 側に残るためです。
AWS SDK が既定で行う再試行も止めます。
一時的な障害はステップの失敗として利用者に伝え、再実行するかは利用者が判断します。

AWS のクライアントの Region、認証情報、再試行、接続先は Docker Action のプログラムが組み立て、共有の設定ファイルと `AWS_*` の環境変数から読みません。
読むと、利用者がステップに渡した環境変数や Runner の AWS の設定で、接続先、認証情報、再試行が変わり得るためです。
AWS STS の呼び出しは認証情報が要らないため、認証情報を持たないクライアントで行います。

どの呼び出しにも全体の時間の上限を設けます。
上限がないと、接続した相手が応答しないとき、ジョブのタイムアウトまで失敗を伝えられないためです。

- **失効**：Lambda の失効と同じ 5 秒とする。失効はベストエフォートで再試行しないため、応答しない相手を長く待っても結果は変わらず、ジョブの時間だけが延びる
- **Lambda Invoke**：関数のタイムアウト（[AWS Infrastructure Design](aws-infrastructure.md#5-lambda-deployment-and-operation)）に初期化の時間を足した値より長くする。同期 Invoke では関数が終わるまで応答が届かず、新しい実行環境では初期化の時間も待つため、上限がこれより短いと、関数が応答を書く前に打ち切り、発行したトークンを受け取れない。関数のタイムアウトを延ばすときは、この上限も見直す

### 3.4 Masking

資格情報は、値を得た直後、その値を含み得るほかの出力より先に `::add-mask::` でマスクします。
マスクより前の出力には、マスクが効かないためです。

| 値 | マスクする時点 |
|---|---|
| OIDC JWT の要求用トークン | 環境変数から読んだ直後。ほかのどの出力よりも先 |
| AWS 用 OIDC JWT、GATES 用 OIDC JWT | 応答から取り出した直後 |
| AWS 一時認証情報（アクセスキー ID、シークレットアクセスキー、セッショントークン） | AWS STS の応答を受け取った直後 |
| Installation Access Token | 応答から取り出した直後で、応答のほかの項目を判定する前。post-entrypoint では `STATE_` の環境変数から読んだ直後 |

マスクはジョブごとに効くため、メインで登録したトークンのマスクは、別のコンテナで動く post-entrypoint のログにも効きます（[GitHub Actions Integration Constraints](../external/github/actions.md)）。
post-entrypoint でももう一度マスクするのは、マスクがメインの登録の成否に依存しないようにするためです。

### 3.5 Revocation

Docker Action は、取り出したトークンの失効を次の時点に分けて試みます（[Domain Model](domain.md#contexts)）。

- **ジョブの終了時の失効**：`GITHUB_STATE` に書いて引き継いだトークンを、ジョブの終了時に post-entrypoint で失効させる。`GITHUB_STATE` を `GITHUB_OUTPUT` より先に書くため、後続ステップへ渡したトークンはすべて含む
- **失敗時の失効**：メインの処理がトークンを取り出した後に失敗で終える場合（キャンセルの要求で終える場合を含む）に、そのトークンをメインの処理がその場で失効させる

失敗時の失効をジョブの終了時に任せないのは、`GITHUB_STATE` に書く前の失敗では post-entrypoint にトークンを引き継げず、誰も持たないトークンが有効期限まで GitHub 側に残るためです。
失敗時の失効は、標準出力への書き込みの失敗と想定外の失敗（panic）を含め、トークンを取り出した後のどの失敗でも試み、試みた後も終了コード 1 で終えます。
失敗で終わるステップのトークンを残さないためです。
`GITHUB_STATE` に書いた後に `GITHUB_OUTPUT` への書き込みに失敗した場合は、post-entrypoint も失効済みのトークンで失効を試み、`204` 以外で終わり得ますが、警告を出すだけでジョブの結果は変えません。

post-entrypoint は、`runs.post-if` を書かず、既定の `always()` のまま、メインの成否によらず動きます。
引き継いだトークンがなければ、何もせずに終えます。
メインの処理が `GITHUB_STATE` に書く前に失敗かキャンセルで終えたジョブに当たります。
失効の成否によらず終了コード 0 で終えます。
失効はベストエフォートであり、トークンは GitHub が定めた有効期限を過ぎれば使えなくなるため、失効の失敗でジョブを失敗させるより、利用者の処理の結果をそのまま残すことを選びます。

失効の要求は、どちらの失効でも共通です。

- `https://api.github.com/installation/token` へ、トークン自身で認証して `DELETE` を送る。宛先は固定し、Runner の環境変数（`GITHUB_API_URL` など）から読まない。GATES は GitHub.com だけを対象とし（[Scope](../spec/scope.md) §4）、トークンを送る宛先を実行環境の値で変えられないようにするためである
- `X-GitHub-Api-Version` は、Lambda が GitHub API を確認したバージョンにそろえる（[Lambda Design](lambda.md#63-github-client)）
- リダイレクトには従わない
- HTTP ステータスをログに出し、`204` 以外か通信の失敗では `::warning::` で出す。応答本文は出さない

入力で失効を止める手段は設けません。
Docker Action はジョブ間のトークンの転送を提供せず、ジョブの終了後にトークンを使う正当な経路がないためです。

### 3.6 Cancellation

ジョブをキャンセルすると、Runner は `docker run` のプロセスに SIGINT を送り、終わらなければ SIGTERM を、SIGINT から 10 秒で強制終了を送ります（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md#runner-source)）。
`docker run` はシグナルをコンテナの最初のプロセスである親へ中継します（[Docker Docs — docker container run](https://docs.docker.com/reference/cli/docker/container/run/)）。
Runner はコンテナを止めないため、`docker run` のプロセスを強制終了した後もコンテナは動き続けます（[GitHub Docker Action Failure and Cancellation](../research/github/docker-action-cancellation.md#killing-the-docker-run-process)）。

シグナルを受けた子をその場で終えると、トークンが残り得ます。
Lambda がトークンを発行した後、子が `GITHUB_STATE` に書く前に終わると、メインの処理も post-entrypoint もそのトークンを失効できないためです。
そのため、Docker Action はシグナルをキャンセルの要求として受け取り、要求を受けた時点に応じて終えます。

- メインの親は、SIGINT か SIGTERM を受けると同じシグナルを子へ送り、子の終了を待ち続ける。親が先に終わると、カーネルが同じコンテナの子を強制終了するため
- メインの子は、Invoke を始める前に要求を受けたら、進行中の通信を打ち切り、キャンセルされたことを `::error::` に書いて終了コード 1 で終える。Lambda をまだ呼んでいないため、トークンは発行されていない
- メインの子は、Invoke の最中に要求を受けても Invoke を打ち切らず、[Retry and Timeout](#33-retry-and-timeout) の上限まで結果を待つ。打ち切ると、Lambda が発行したトークンを受け取れず、誰もそのトークンを失効できないため
- メインの子は、トークンを取り出してマスクした後、残りの項目を判定する前に要求の有無を確かめる。要求があれば、Output と `GITHUB_STATE` に書かずに失敗時の失効を試み、キャンセルされたことを `::error::` に書いて終了コード 1 で終える
- メインの子は、この確認の後に要求を受けたら、要求がない場合と同じく終える。Runner はキャンセルで終わったステップの `GITHUB_STATE` も読むため（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md#runner-source)）、書いたトークンはジョブの終了時の失効が扱う
- post-entrypoint の子は、SIGINT と SIGTERM を無視し、失効を終えてから終える。失効の上限の 5 秒は、SIGINT から強制終了までの 10 秒より短いため
- post-entrypoint の親も、SIGINT と SIGTERM を無視し、子へ送らない。子が無視を設定する前に届いたシグナルで子が終わると、失効を試みないまま終わるため

子がシグナルの受け方を設定しないと、Go のランタイムがシグナルを受けた子を終えるため、メインの子は受け取り、post-entrypoint の子は無視するよう設定します。
Invoke と失敗時の失効は、キャンセルの要求で打ち切りません。

Invoke の結果を待つ間に `docker run` のプロセスが強制終了されても、子はトークンを取り出せば失敗時の失効を試みますが、その結果はワークフローのログに残りません（[GitHub Docker Action Failure and Cancellation](../research/github/docker-action-cancellation.md#killing-the-docker-run-process)）。
Runner がコンテナを強制終了した場合や Runner が停止した場合は、子が失効を試みる前に終わり得ます。
このリスクは [Security Design](security.md#5-failure-audit-and-security-risks) が受容します。

## 4. Technology Decisions

| 用途 | 採用技術・方針 | 理由 |
|---|---|---|
| Action の形式 | Docker Action（`runs.using: docker`、`image: Dockerfile`） | 利用者の Runner がジョブのたびに、利用者が commit SHA で固定した参照先の `Dockerfile` からイメージをビルドする。レジストリにイメージを公開する作業と、公開したイメージを信頼する前提が要らない |
| AWS STS と Lambda Invoke | AWS SDK for Go v2 | SigV4 の署名と AWS STS の呼び出しは、誤りが脆弱性になるため SDK に委ねる |
| OIDC JWT の取得、失効、ワークフローコマンド、Payload と応答の JSON | Go の標準ライブラリ | 処理が小さく、外部ライブラリの依存を増やすほどの理由がない |

外部ライブラリは AWS SDK for Go v2 だけとし、ワークフローコマンドのライブラリ（actions/toolkit に相当する Go のライブラリ）も使いません。

### 4.1 Exceptions to the Go Coding Rules

Docker Action の Go のコードは、Go コーディングルールが既定とする方式のうち、次の方式を使いません。
これらはどれもモジュール全体に及ぶため、コードのコメントではなく本ドキュメントに理由を書きます。

- **cobra を使わない**：Docker Action は flag を受け取らず、入力は GitHub が名前を定める `INPUT_*` の環境変数で届く。外部の標準で名前が決まっている入力に当たる。メインと post-entrypoint の区別は、起動したパスで行う
- **slog を使わない**：Docker Action の出力は、Runner が読むワークフローコマンドと、利用者が読む数行の情報であり、構造化したログを読む主体がない

### 4.2 Image

- ビルド用と実行用を分けたマルチステージビルドとし、ビルド用のイメージはタグと digest で固定する
- ビルド用のイメージは `docker.io/library/golang` の Debian 版とする。Alpine 版は Go プロジェクトの公式サポートの対象外であり（[Docker Hub — golang](https://hub.docker.com/_/golang)）、CGO を使わずにビルドするため C ライブラリの違いは実行ファイルに影響しない
- ビルド用のイメージの Go は、`go.mod` の `go` のバージョンと同じマイナーバージョンの、最新のパッチバージョンとする。`go` のバージョンは言語の最低のバージョンで、パッチバージョンは標準ライブラリの脆弱性の修正を含むためである。ツールチェーンの自動取得を止めてビルドするため、`go.mod` のバージョンを上げるときはビルド用のイメージも合わせて上げる
- ビルドは `go.mod` と `go.sum` を書き換えず、`go.sum` にない依存を取り込まない
- 実行用のイメージは `scratch` とし、実行ファイルと、GitHub と AWS への HTTPS に要る CA 証明書だけを置く
- `.dockerignore` は、すべてを除外してから、ビルドに使うファイルだけを許可する
- Dependabot の `docker` で `FROM` のイメージを、`gomod` で Go の依存を更新する。`docker` が `FROM` の digest も更新するかは、GitHub の資料に明記がない（[GitHub Actions Integration Constraints](../external/github/actions.md)）

`USER` を置かず、root で動かします。
root 以外では `GITHUB_OUTPUT` と `GITHUB_STATE` に書けず、ファイルコマンドのファイルの所有者の uid は Runner の環境ごとに決まるため、イメージに固定の uid を置いても書き込める保証がないためです（[GitHub Actions Integration Constraints](../external/github/actions.md)）。
コンテナに入るのはこのリポジトリでビルドした実行ファイルと CA 証明書だけであり、root で動くことの影響は Security Design が受容するリスクの範囲にとどめます（[Security Design](security.md#5-failure-audit-and-security-risks)）。

### 4.3 Runner Requirements

Docker Action は、Linux の Runner と Docker を要します（[GitHub Actions Integration Constraints](../external/github/actions.md)）。
Windows と macOS の Runner では Docker Action が動かないため、対象外とします。
加えて、Runner から次の宛先へ通信できる必要があります。

- イメージのビルド：Docker Hub（ビルド用のイメージ）と Go module proxy（`proxy.golang.org`）
- メインの処理：`ACTIONS_ID_TOKEN_REQUEST_URL` の宛先と、入力の `aws-region` の AWS STS と Lambda の API
- 失効：`api.github.com`

GitHub がホストする `ubuntu-latest` の Runner はこれらをすべて満たし（[GitHub — runner-images](https://github.com/actions/runner-images)）、利用者の準備は要りません。
セルフホストの Runner では、利用者が Linux と Docker を用意し、これらの宛先への通信を許可します。
Docker Hub は認証しないイメージの取得の回数を制限するため、ジョブのたびにビルドするセルフホストの Runner では、利用者が Docker Hub の認証やミラーを用意します（[Docker Docs — Usage and limits](https://docs.docker.com/docker-hub/usage/)）。

## 5. Testability

受け入れ条件は [Acceptance Criteria](../spec/acceptance-criteria.md) を正本とします。
GitHub の Installation Access Token は固定長と仮定せず、トークンの形式と長さによらず、マスク、非記録、Output と `GITHUB_STATE` への書き込みが正しく行われることを検証します。

### 5.1 Local Tests

AWS と GitHub に接続せずに確かめられる事項は、Go の単体テストで確かめ、`internal/` のカバレッジは 100% とします。
通信する package の試験では、宛先の URL と AWS SDK の接続先を `httptest` のサーバーに差し替えます。

親の試験は、試験の実行ファイル自身を子として起動し、子の振る舞いを本番と同じく `argv[0]` の名前で選びます。
このため、親の試験のコードは、子の振る舞いのために環境変数、引数、終了コード、シグナル、`panic` を直接使います。
キャンセルの試験では、シグナルの代わりにキャンセルの要求を入口関数に渡します。

### 5.2 Tests on GitHub Actions

GitHub と AWS の挙動に依存する事項は、許可オーナーのワークフローから Docker Action を呼んで確かめます。
対象は、後続ステップでのトークンの利用、ワークフローのログと後続ステップの環境変数に資格情報が現れないこと、AWS STS、Invoke API、関数のそれぞれの失敗の出力、post-entrypoint による失効、キャンセルの各時点の終え方、別のリポジトリから commit SHA で参照したときのイメージのビルドです。

## 6. References

- [GitHub Actions Integration Constraints](../external/github/actions.md)
- [GitHub App Constraints](../external/github/app.md)
- [GitHub Docker Action Runtime](../research/github/docker-action-runtime.md)
- [GitHub Docker Action Failure and Cancellation](../research/github/docker-action-cancellation.md)
- [GitHub Composite Action Outputs](../research/github/composite-action-outputs.md)
- [AWS Lambda Invoke Rejection](../research/aws/lambda-invoke-rejection.md)
- [GitHub — Creating a Docker container action](https://docs.github.com/en/actions/tutorials/use-containerized-services/create-a-docker-container-action)
- [GitHub — Dockerfile support for GitHub Actions](https://docs.github.com/en/actions/reference/workflows-and-actions/dockerfile-support)
- [GitHub — Metadata syntax reference](https://docs.github.com/en/actions/reference/workflows-and-actions/metadata-syntax)
- [GitHub — Workflow commands for GitHub Actions](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands)
- [GitHub — actions/toolkit command.ts](https://github.com/actions/toolkit/blob/main/packages/core/src/command.ts)
- [GitHub — actions/runner ActionCommand.cs](https://github.com/actions/runner/blob/v2.338.2/src/Runner.Common/ActionCommand.cs)
- [GitHub — runner-images](https://github.com/actions/runner-images)
- [GitHub — REST API endpoints for GitHub App installations](https://docs.github.com/en/rest/apps/installations)
- [AWS — AssumeRoleWithWebIdentity](https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRoleWithWebIdentity.html)
- [AWS — Lambda Invoke API](https://docs.aws.amazon.com/lambda/latest/api/API_Invoke.html)
- [Docker Hub — golang](https://hub.docker.com/_/golang)
- [Docker Docs — docker container run](https://docs.docker.com/reference/cli/docker/container/run/)
- [Docker Docs — Usage and limits](https://docs.docker.com/docker-hub/usage/)

## Appendix: Impact of Future Scope

[Scope](../spec/scope.md) が将来取り込むことを想定する機能を導入する場合、本ドキュメントには次の影響が及びます。

- **細粒度の認可**：発行の可否の判定は Lambda 側が行うため、利用者が指定する入力が増えない限り、Docker Action は変わらない
- **Organization permissions、Account permissions、Enterprise permissions の発行**：`permissions` 入力の値が変わる。許可条件の判定を Lambda の認可に委ねる方針は変わらない。これらの権限の対象を `repositories` 入力で表せないなら、入力も見直す
