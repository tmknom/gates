# Security Design

## 1. Security Model

本システムでは、AWS IAM による Lambda Invoke の入口制御と、Lambda 内で検証した GitHub OIDC JWT に基づくトークン発行認可を分離します。GitHub App Private Key は AWS KMS に置き、Lambda は署名操作を KMS に依頼します。返却する Installation Access Token は、要求されたリポジトリと、要求された Repository permissions の権限（許可された付随権限を除く）に限定します。

コンポーネントの全体構成、JWT の生成元、正常系の処理順序は [System Architecture](architecture.md) に記述します。本ドキュメントは各境界が保証する内容と、認可・資格情報保護・拒否条件を定義します。概念の定義は [Domain Model](domain.md) を正本とします。

## 2. Trust Boundaries

| 境界 | 保証すること | 保証しないこと |
|---|---|---|
| AWS IAM / STS | 許可オーナーの不変 Owner ID を `sub` に含む GitHub OIDC JWT からだけ AWS 一時認証情報を発行し、特定 Lambda の Invoke のみ許可する | ワークフロー・リポジトリ単位のトークン発行認可、資格情報の現在の利用者の特定 |
| OIDC Verifier | GitHub JWT の真正性、Issuer、Audience、期限、不変 Owner ID と Repository ID を検証する | JWT と AWS 一時認証情報が同じジョブで取得されたことの証明、トークン発行権限そのもの |
| Authorization | 許可オーナーの Owner ID、取得した Installation の `account.id`、許可権限、発行上限、保護対象の一覧、明示された要求を照合する | 保護対象の一覧にないリポジトリでの、アクセス先ごとの Caller の制限、対象リポジトリが Installation に含まれることの事前確認、要求した権限が Installation に付与されていることの事前確認 |
| AWS KMS | 非抽出可能な秘密鍵による RSA 署名を実行する | 署名対象 JWT の内容に対する GATES の認可判断 |
| GitHub API | Installation と App の権限上限に従い短命トークンを発行する | GATES 側のトークン発行認可 |

### 2.1 AWS IAM / STS

IAM Role の Trust Policy は、GitHub OIDC Provider を信頼先とし、`aud` と `sub` の条件で引き受けを制限します。
AWS 用 OIDC JWT の Claim に課す条件と、この入口制御が保証しない範囲は [AWS OIDC JWT](aws-oidc-jwt.md) に定めます。

- オーナー配下での共通利用を前提とし、`sub` の後半部分（`ref`、`pull_request`、`environment` など）による追加制限は設けない。
- 引き受けた IAM Role の権限は対象 Lambda への `lambda:InvokeFunction` のみとし、KMS `Sign`、IAM 管理、鍵インポートを許可しない。
- 許可オーナーのワークフローは共通の一つの IAM Role を引き受ける。

オーナー側で既存リポジトリの OIDC JWT に不変 ID を含める設定が済んでいることを前提とします。この設定は GitHub 側の操作であり、GATES は実施しません。設定前のリポジトリは旧形式の `sub` を持つため、IAM Role を引き受けられません。

IAM は認証情報の由来となるオーナーに対する入口制限であるため、IAM 認証情報の窃取・再利用により、元のジョブ以外から Lambda Invoke が行われる可能性は残ります。

### 2.2 GATES 用 OIDC JWT

AWS 用 OIDC JWT とは別に、GATES 用 Audience の OIDC JWT を取得します。
Lambda はこの JWT を検証し、Caller を確定します。
署名と検証鍵、Issuer、Audience、時間条件、必須 Claim、`sub` と個別の ID の Claim の整合の受理条件は [GATES OIDC JWT Claims](gates-oidc-jwt-claims.md) と [GATES OIDC JWT Signature](gates-oidc-jwt-signature.md) に定めます。
検証鍵は固定した Issuer の JWKS からだけ得ます。
JWT の発行者と署名検証鍵を結び付け、JWT や要求の内容で鍵の取得先を変えられないようにするためです。

JWT が受理条件を満たしても、発行要求を認可したことにはなりません。
発行の可否は、認可条件で別に判定します。

AWS 一時認証情報と GATES 用 OIDC JWT が同じジョブで取得されたことを、暗号学的に検証しません。AWS IAM による Invoke の許可と、GATES 用 OIDC JWT に基づく発行認可は独立した境界です。トークンの発行可否は GATES 用 OIDC JWT と固定された許可条件から判断し、Docker Action 側の自己申告による照合を認可根拠としません。

## 3. Authorization

### 3.1 Authorization Rule

リポジトリへの発行は、次の条件がすべて成立した場合だけ許可します。
いずれかが成立しなければ、認可拒否として扱います。

1. 検証済み Caller の `repository_owner_id` が許可オーナーの不変 Owner ID と一致する。オーナー名の一致では代用しない。
2. 要求した各 `owner/repo` のオーナー名が固定された許可オーナー名と一致する。
3. 要求の Permissions の Permission Entry が、**どれも許可権限に含まれ、発行上限に水準の包含で含まれる**（§3.3）。
4. 要求した完全名のうち、リポジトリ名が保護対象の名前と一致するものすべてについて、Caller がその保護対象の許可 Caller に当たり、要求の Permission Entry がどれもその許可 Caller の権限の上限に水準の包含で含まれる（§3.6）。

条件 1 から 3 は Caller と対象の組を見ず、条件 4 だけが組を見ます。
条件 4 を満たさない保護対象を一つでも含む要求は、保護対象でない対象を含めて全体を拒否します。

Caller の `repository_id` は、監査と条件 4 に用います。
Caller のリポジトリ名（`sub` のリポジトリ部の名前）は、条件 4 で ID の一致に重ねて照合するためだけに用います。
ワークフロー起動属性は監査のための識別に用い、認可条件には含めません。
署名検証済みの Claims Set も監査記録に書き写すだけで、認証と認可のどちらにも使いません。

条件 3 の Permissions は、形式が正しければ JWT 検証後に認可条件として判定し、許可外なら認可拒否として扱います。
許可外の Permission Entry を一つでも含む要求は、許可されるものだけに縮小せず、全体を拒否します。
形式検証で先に落とさないのは、`members:read` のような許可外の要求を、認証済み Caller とともに監査記録に残すためです。
形式検証が入力不正にするのは、欠落や空の Permissions のように、要求した権限を読み取れない形に限ります。

要求には、全リポジトリを対象にする項目を設けません。
対象は要求で明示したリポジトリだけであり、`selected` しか許可しない値を利用者に指定させても、許可条件を一つ増やすだけになるためです。

認可条件が成立した後、GitHub App JWT で許可オーナーの Installation を取得し、発行に使えるかを次の条件で確認します。

- Installation の `account.id` が許可オーナーの Owner ID と一致する。
- Installation が停止していない（`suspended_at` が `null` である）。`suspended_at` のキーがない、または `null` でも時刻でもない応答は、停止していないと読まず、Installation を作らない。

どちらかが成立しなければ発行しません。
この拒否は利用者の要求に起因せず、運用者かオーナーの管理者が GitHub 側か固定設定を直す必要があるため、認可拒否と区別し、Installation 利用不可として返します。
停止中の Installation に対する発行 API の応答は公式文書で確認できていないため、GitHub の振る舞いに Fail Closed を預けず、取得した時点で閉じます。

これらの確認の後、Installation ID、要求した完全名、要求した Permissions を含む発行許可を生成します。Installation に対象リポジトリが含まれること、要求した Permissions が Installation に付与されていること、発行結果が発行許可に一致すること、実効対象のリポジトリ名と Repository ID の対応が保護対象の一覧と食い違わないことは、GitHub API の応答を受けてから確認します。一部だけで発行することや権限を暗黙に広げることは禁止します。

### 3.2 Installation Scope as Authorization Boundary

Installation の GitHub 側の仕様は [GitHub App Constraints](../external/github/app.md) を参照します。

許可オーナーの有効なワークフローは、呼び出し元リポジトリ／チーム別の認可を経ず、GATES が許可する権限の範囲で GitHub App の Installation 対象リポジトリに横断的なトークン発行を要求できます。この権限は GitHub App / Installation 自体の権限上限にも拘束されます。GATES が許可する権限は §3.3 に定めます。

GitHub App のインストール対象が、トークンで操作できる範囲の上限となります。`Only select repositories` により対象を限定できます。`All repositories` を使用すると、そのオーナーにインストールされた App の対象リポジトリ全体が操作できる範囲に含まれます。インストール対象の変更は認可境界の変更として扱います。
GATES はインストール対象を保持せず、発行のたびに GitHub が判定するため、変更は GATES の設定を変えずに以後の発行要求へ反映されます。対象から外したリポジトリを含む要求は GitHub が拒否し、GATES はどの対象にもトークンを返しません。

許可オーナーが Organization の場合、この変更は Organization オーナーだけの操作ではありません。
既定では、リポジトリ管理者が自身の管理するリポジトリをインストール済みの App の対象に追加でき、`installation_repositories:write` を持つ呼び出し元も、そのリポジトリを Installation に追加または削除できます。
追加されたリポジトリは、GATES と Installation が許可する権限の範囲で、許可オーナーの有効なワークフローから横断的にアクセスされ得ます。懸念は、非公開コードや設定、誤って格納された秘密情報の読み取りと、GitHub App に付与された権限による操作です。
このため、Organization の許可オーナーには、リポジトリ管理者によるインストールを Organization オーナーが制限していることと、`installation_repositories:write` の付与先を管理していることを、運用の前提として求めます。
GATES はこの設定を実行時に確認しません。

ここでの上限は、Installation Access Token の対象に指定できる範囲を指します。
「インストール対象の外にある公開リポジトリは読めない」を意味しません。
選択対象外の公開リポジトリは、発行の要求に含めると拒否されますが、発行したトークンでは公開の内容として読めます（[GitHub App Constraints](../external/github/app.md)）。
公開の内容は誰でも読めるため、この振る舞いは読み取れる範囲を広げません。
`contents:write` で発行したトークンでは、要求した対象には書き込めましたが、選択対象外の公開リポジトリへの書き込みは `403`、要求していない選択リポジトリと選択対象外の private リポジトリへの書き込みは `404` で拒否されました（[GitHub App Constraints](../external/github/app.md)）。
ほかの読み取り以外の権限で、選択対象外の公開リポジトリに何ができるかは確かめていません。

GATES は、デプロイ時に固定した GitHub App と許可オーナーの組から Installation が一意に決まるものとして扱います。
これは、オーナーを指定して単一の Installation を返すエンドポイントがあることからの推論であり、GitHub が一意性を明示的に保証したものとは扱いません。
Lambda は Installation ID を設定として持たず、固定したオーナーの Installation を GitHub API から取得し、`account.id` の一致で確認します。この導出は [Requirements](../spec/requirements.md) FR-OPS-001 に従います。Installation ID を設定値として持たないため、アプリの再インストールで ID が変わっても設定変更は不要であり、実行時の入力から Installation を指定する余地もありません。オーナーの名前変更やアプリの再インストールが起きても、固定した不変 Owner ID と異なる Installation へ発行しません。

### 3.3 Permission Boundary

GATES が発行を許可する Permission Entry の集合を、許可権限と呼びます。
許可権限は、GitHub が Repository permissions に分類する権限の権限名と、GitHub が要求どおりに発行する水準の組です。
権限名、水準、分類の根拠は [GitHub App Constraints](../external/github/app.md) の「権限名と分類」を参照します。

許可権限は、次の規則で定めます。

- **列挙して固定する**：発行 API のスキーマは、権限名を Repository、Organization、Account、Enterprise に分けて示さない。そのため GATES は、許可する権限名と水準を列挙して持ち、列挙にない権限名と水準を認可拒否にする。GitHub が権限を加えても、GATES が列挙に加えるまで発行しない。権限名の形から分類を推定すると、GitHub が加えた Organization permissions を発行し得るためである
- **分類と水準を確かめたものだけを加える**：列挙に加えるのは、ユーザーアカウントの Installation の `permissions` に現れた権限名と、その権限名で要求どおりに発行されることを確かめた水準に限る。Organization permissions と Enterprise permissions は Organization か Enterprise にインストールした場合にだけ効き、Account permissions はインストールではなくユーザーの認可で付与されるため、ユーザーアカウントの Installation に現れた権限名を Repository permissions に当たるものとして扱う（[GitHub App Constraints](../external/github/app.md)）。現在の列挙は、GitHub App Constraints の「権限名と分類」が記録する 43 の権限名と、要求どおりに発行された水準の組である
- **発行 API のスキーマに権限名があることを条件にしない**：スキーマに権限名がない Repository permissions（`actions_variables` など）も、上の条件を確かめたものは列挙に含める。GitHub App に付与された権限を利用者が明示して要求したとき、スキーマの有無で拒否すると、GitHub が発行する権限を GATES の判断で狭めることになるためである。水準もスキーマではなく GitHub の応答で決める。スキーマが `write` だけを定める `workflows` の `read` は要求どおりに発行されたため含め、スキーマが定める `metadata` の `write` は GitHub App に付与できず拒否されたため含めない。スキーマにない権限名は、GitHub が名前や扱いを予告なく変える可能性がある。変わった場合も、発行 API の拒否か照合（§3.4）の失敗でトークンを返さずに終わる
- **Organization permissions、Account permissions、Enterprise permissions は発行しない**：これらは対象リポジトリを明示しても影響範囲が狭まらず、リポジトリを単位とする認可条件と照合（§3.4）では範囲を限定できない
- **Caller と対象によって変えない**：許可権限は、どの Caller にも、どの対象リポジトリにも同じである。一つの要求の Permissions は、その要求のすべての対象に同じく適用される。保護対象の一覧（§3.6）は、許可権限の中で、保護対象ごとに Caller と権限を狭めるだけである
- **設定で広げない**：許可権限を広げる設定を持たない。デプロイごとに狭める手段は、次に定める発行上限である

GATES 運用者は、許可権限のうち、そのデプロイで発行してよい Permission Entry の集合を、デプロイ時の設定として決めます。
この集合を **発行上限** と呼びます。
GitHub App に付与する権限は GitHub の画面だけで変えられ、コードのレビューにも Terraform の Plan にも現れません。
発行上限を Terraform の入力にすると、発行できる権限の変更がレビューと Plan の対象になり、GitHub App に誤って権限を付与しても、発行上限に含まれない権限は発行しません。

発行上限は、次の規則で定めます。

- **許可権限の一部に限る**：許可権限に含まれない Permission Entry を発行上限に含めると、Lambda の初期化が失敗し、どの要求も受け付けない。許可権限は Organization permissions などを発行しないための最後の上限であり、設定から広げる手段を持たない
- **水準の包含で照合する**：発行上限は許可権限と同じく Permission Entry の集合で、一つの権限名に複数の水準を含められる。要求の Permission Entry は、許可権限に含まれ、発行上限が同じ権限名の同じかより高い水準を含むときに許す（水準は `read`、`write`、`admin` の順に高い）。`contents:write` だけを含めた発行上限は、`contents:read` の要求を許す。GitHub App に付与した水準より低い水準の要求は要求どおりに発行されたため（[GitHub App Constraints](../external/github/app.md) §4.4 の観測）、高い水準を発行してよいデプロイで低い水準を拒否しても、守れる範囲は増えない
- **要求した水準で発行する**：発行上限は要求を許すかだけを決め、発行する水準を変えない。上限が `contents:write` でも、`contents:read` の要求には `contents:read` を発行し、発行結果の照合（§3.4）も要求した水準との一致を求める
- **空にしない**：空の発行上限はどの要求も認可しない設定で、書き損じと区別できないため、初期化の失敗にする
- **付随権限には適用しない**：付随権限 `metadata:read` は要求にかかわらず GitHub が付与するもので、発行結果の照合で許容する。発行上限に含まれなくても、保護対象を含む要求で許可 Caller の権限の上限（§3.6）に含まれなくても許容する。`metadata` を明示して要求するときは、ほかの権限と同じく発行上限に含まれている必要がある
- **Caller と対象によって変えない**：発行上限はデプロイに一つで、どの Caller にも、どの対象リポジトリにも同じに効く。Caller と対象の組による制限は、保護対象の一覧（§3.6）が担う
- **以後の発行要求にだけ効く**：発行上限を狭めても、発行済みのトークンは失効させない。発行済みのトークンは、GitHub が定めた有効期限まで、発行したときの権限で使える。Lambda の設定の更新が完了するまでの呼び出しは、更新前の発行上限で判定される（§5）

発行上限は Lambda の環境変数として渡すため、Lambda の設定を変更できる主体は、発行上限を許可権限まで広げられます。
この主体は関数のコードも変更して実行 Role の権限で署名させられるため、発行上限を広げられても、得られる能力は増えません。
発行上限が抑えるのは、GitHub App に誤って付与した権限と、レビューと Plan を経ずに GitHub の画面で広げた権限の発行です。
Lambda の設定やコードを変更できる主体による悪用は抑えません。

発行できる権限は、許可権限と発行上限に加えて、GitHub App と Installation に付与された権限にも拘束されます。
GATES は Installation に付与された権限を事前に確かめません。
付与されていない権限か水準を含む要求は、発行 API が拒否するか、発行結果の照合が実効権限の不一致を検出するかのどちらかで、トークンを返さずに終わります（[Requirements](../spec/requirements.md) FR-AUTHZ-007）。
GitHub App に付与されていない権限と、付与された水準より高い水準の要求は、発行 API が `422` で拒否しました。
GitHub App には付与され、Installation に現れない権限の要求は、拒否されずに、その権限を除いた発行になりました（[GitHub App Constraints](../external/github/app.md)）。
後者は照合で実効権限の欠落として検出します。
どちらになるかは GitHub の振る舞いに依存するため、トークンを返さないことは照合で保証します。
GitHub App / Installation の付与権限が要求より広くても、発行するトークンの権限は要求した Permissions から拡大しません。付随する `metadata:read` のみ、照合時に許容します。

### 3.4 Issuance Result Verification

- GitHub API に発行を要求するときは、発行許可の対象と Permissions を必ず指定する。
- 発行応答の対象リポジトリと実効権限を発行許可に照合する。Owner ID、Repository ID の有効性・一意性、対象の全件一致を確認し、必要な情報の欠落、対象外のリポジトリ、過大な権限があればトークンを返却しない。この拒否は GitHub API の通信障害とは別のエラーとして返し、監査記録に Caller とともに残す。
- GitHub の発行応答からトークン文字列を取得でき、照合に失敗した場合は、利用者へ返却せず失効 API の呼び出しを試みる。失効はベストエフォートとし、試行の成否を監査記録に残す。API 応答が失われた場合はトークンの発行有無が不明となり得る。
- 実効権限は、権限名と水準の組で発行許可の Permissions に照合する。発行許可の Permission Entry が実効権限に一つでも欠けているか、水準が異なれば、トークンを返却しない。水準が要求より低い場合も、要求を縮小した発行として拒否する。
- 要求にかかわらず GitHub が付与する `metadata:read` だけは、発行許可の Permissions になくても、応答に含まれていれば許容する。付与の条件は未確認であり（[GitHub App Constraints](../external/github/app.md)）、含まれなくても拒否しない。`metadata` を要求した場合は、ほかの権限と同じく発行許可の水準に照合する。許可権限の各権限名を単独で要求した発行と、複数の権限を要求した発行では、`metadata:read` のほかに権限が加わった応答はなかった（[GitHub App Constraints](../external/github/app.md)）。GitHub がほかの権限を加えるように変えた場合、その応答は照合の失敗になる。
- 発行応答の `repository_selection` が `selected` であることを確認する。`all` は Installation の全リポジトリを対象にしたことを意味し、要求外の対象を含むため返却しない。
- 各実効対象と各保護対象の組について、実効対象のリポジトリ名が保護対象の名前と一致することと、実効対象の Repository ID が保護対象の ID と一致することが、両方成り立つか両方成り立たないことを確認する。一方だけが成り立つ実効対象は、保護対象の一覧と GitHub の現在の状態が食い違っており（§3.6）、照合の失敗としてトークンを返却しない。
- 応答の Repository ID は、ID の有効性と一意性の確認、保護対象の一覧との照合、監査に使う。GATES は要求の時点で対象の Repository ID を確かめないため、応答の Repository ID が利用者の意図したリポジトリの ID であることは示さない。

対象の全件一致を照合するため、[Requirements](../spec/requirements.md) FR-AUTHZ-005 のうち範囲外の対象を含む要求にトークンを返さないことは、GitHub が範囲外の対象を拒否するかどうかに依存しません。
GitHub が範囲外の対象を黙って除いて発行した場合は、応答の対象と発行許可の不一致を検出し、トークンを返さず、取得したトークンの失効を試みます。
発行 API が呼び出しを `422` で拒否した場合は GitHub 発行拒否、発行応答を取得して照合が失敗した場合は照合失敗とします。どちらも、発行前の GATES の認可条件による認可拒否とは区別します。
GitHub 側での失効成功や、応答喪失で取得できなかったトークンの不存在は保証せず、GitHub 側で有効なまま残る可能性があります。

### 3.5 Accepted Risks for Repositories Not Protected

保護対象の一覧にないリポジトリについては、認可条件が Caller と対象の組み合わせを見ません。
そのため、許可オーナーのどのワークフローも、Installation に含まれるそれらのリポジトリに対して、GitHub App に付与され、発行上限に含まれる Repository permissions をすべて要求できます。
保護対象の一覧にないリポジトリは、自身に対する操作を GATES の認可では制限できません。
新しく作ったリポジトリも、一覧に加えるまでは同じです。
[Scope](../spec/scope.md) がこの組み合わせを現在のスコープとするため、次のリスクを受容します。
各権限で呼べる API は、GitHub の公式資料（[Permissions required for GitHub Apps](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps)）に基づきます。

- **ほかのリポジトリの変更と破壊**：`contents:write` はコミット、ブランチ、タグ、リリースを書き換えられる。`administration:write` はリポジトリの設定の変更と削除、ブランチ保護と ruleset の変更、コラボレーターとデプロイキーの追加、セルフホストの Runner の登録に届く。リポジトリの削除や履歴の書き換えのように、元に戻せない操作を含む
- **ほかのリポジトリの Secrets と OIDC への到達**：`contents:write` と `workflows:write` を持つトークンは、ほかのリポジトリのワークフローのファイルを書き換えられる。書き換えたワークフローは、そのリポジトリのワークフローとして動き、ブランチと Environment の保護が止めない範囲で、そのリポジトリの Secrets と、そのリポジトリを主体とする OIDC JWT を使える。`secrets:write` は Secrets の値を読めないが、上書きと削除ができる。`actions:write` は、ワークフローの起動、実行ログの削除、OIDC JWT の `sub` のカスタマイズの変更に届く。このため、Installation に含まれるリポジトリの Secrets と、それらのリポジトリを信頼する外部の権限（クラウドの Role など）は、許可オーナーのどのワークフローからも到達され得るものとして扱う
- **要求した対象を越える変更**：公式資料は、`installation_repositories:write` で使える API に、リポジトリを Installation に加える API と外す API を挙げる。Installation の対象は GATES の認可の境界であり、許可オーナーのワークフローが、その境界を変え得るトークンを要求できる。`repository_creation:write` で使える API には、リポジトリの作成、フォーク、テンプレートからの生成を挙げる（ほかの権限も要る場合がある）。どちらも、要求した対象のリポジトリに影響範囲を限れない。トークンで実際に何ができるかは確かめていない
- **トークンの有効期限より長く残る変更**：デプロイキー、Webhook、コラボレーター、書き換えたワークフローとコードは、トークンの失効と有効期限の後も残る。トークンが短命であることと、ジョブの終了時の失効は、トークンで行われた変更を取り消さない
- **信頼できないコードが得る権限の拡大**：トークンを取得するジョブで信頼できないコードが動くと、そのコードは読み取りにとどまらず、GitHub App に付与された権限で Installation のリポジトリを操作できる
- **機密の情報の読み取りの拡大**：`secret_scanning_alerts:read` は検出された秘密情報のアラートに、`actions:read` はほかのリポジトリの実行ログと成果物に届く。読み取れる範囲はリポジトリの内容にとどまらない
- **トークンで行われた操作の追跡**：GATES の監査記録が残すのは、Caller、要求対象、要求権限、発行の結果までである。発行したトークンでどの操作が行われたかは GATES に残らず、GitHub 側の記録に依存する

これらのリスクは、次の手段で抑えます。
発行上限と保護対象の一覧は GATES が発行のたびに確かめる設定で、残りは GATES の外の設定であり、GATES は実行時に確かめません。

- **守りたいリポジトリを保護対象にする**：操作できるワークフローを一部のリポジトリのものに限りたいリポジトリは、保護対象の一覧に加え、要求できる Caller と権限を限る。上の経路で到達されたくないリポジトリ（Secrets を持つリポジトリや、外部の権限に信頼されたリポジトリなど）も対象になる。一覧の変更は、発行上限と同じく Terraform の変更としてレビューと Plan を経る。保護対象に残るリスクは §3.6 に定める
- **発行上限を限る**：GitHub App に付与し、かつ発行上限に含まれる Repository permissions が、許可オーナーのすべてのワークフローに共通の上限になる。発行上限は、許可オーナーのワークフローが必要とする権限だけにする。発行上限の変更は、Terraform の変更としてレビューと Plan を経る
- **GitHub App に付与する権限を限る**：`administration`、`workflows`、`secrets`、`actions`、`installation_repositories`、`repository_creation` の `write` は、上の経路のうち影響の大きいものを開くため、必要な自動化がある場合に限って付与する。発行上限が狭くても、GitHub App に付与した権限は署名能力の悪用で得られるため、GitHub App にも必要な権限だけを付与する。GitHub App の権限の変更は、Installation の対象の変更と同じく、認可境界の変更として扱う
- **Installation の対象を限る**：保護対象にしないリポジトリは、GitHub App に付与した権限で、許可オーナーのどのワークフローから操作されてもよいものに限る。保護対象にするかどうかにかかわらず、GATES の Docker Action と Terraform のコードを置くリポジトリや、GATES の AWS リソースを変更できる Role に信頼されたリポジトリを対象に含めると、GATES 自身の改ざんに届く経路ができるため、含めない
- **アクセスされる側のリポジトリで保護する**：ブランチ保護と ruleset は、GitHub App を bypass の対象に加えない限り、トークンによる変更にも適用される見込みである（公式資料が bypass の対象に App を挙げることからの推論で、実環境では確かめていない）。`administration:write` を持つトークンはこの保護を変更できるため、保護に頼る場合は GitHub App に `administration:write` を付与しない

### 3.6 Protected Repositories

GATES 運用者は、操作できるワークフローを一部のリポジトリのものに限りたいリポジトリを **保護対象** として固定設定の保護対象の一覧に指定し、要求できる Caller と権限を限ります（[Requirements](../spec/requirements.md) FR-AUTHZ-009）。
保護対象、許可 Caller、権限の上限の定義は [Domain Model](domain.md) を正本とします。

保護対象の一覧は、次の規則で定めます。

- **Repository ID で同定し、名前で発行前に見分ける**：発行要求は対象を完全名だけで示すため、要求のリポジトリ名が保護対象の名前と一致する対象を保護対象として、発行前に §3.1 の条件 4 を判定する。発行の後は、実効対象の Repository ID を保護対象の ID と照合する（§3.4）。Repository ID を同定に使うのは、改名で変わらないと GitHub が定め（[GitHub Actions OIDC Constraints](../external/github/oidc.md) の GH-OIDC-REPOSITORY-ID-RENAME）、削除して作り直したリポジトリでは別の値だったため（[GitHub Owner and Repository Constraints](../external/github/account.md) §3 の観測）である。ほかのリポジトリに再び割り当てられないことは、GitHub が ID を「immutable」と呼ぶことからの推論で、明記はない（同 §3）
- **一覧と GitHub の食い違いを拒否に倒す**：名前で見分ける発行前の判定と、ID で突き合わせる発行後の照合は、片方だけでは保護にならない。組にすると、一覧と GitHub の現在の状態が食い違ったどの場合も、トークンを返さずに終わる（次の表）
- **許可 Caller は ID と名前の両方で照合する**：許可 Caller に当たるのは、Caller の `repository_id` と、`sub` のリポジトリ部の名前の両方が一致する Caller に限る。名前は `sub` から得る。`sub` は署名と形式の検証を経ており、しかも同じ文字列の中で名前と Repository ID が組になっているためである。名前の照合は ID の一致に重ねる条件であり、名前だけが一致する Caller を許可 Caller にしない。ID と名前のどちらを書き誤っても、その許可 Caller はどの Caller にも当たらず、拒否に倒れる
- **許可 Caller の権限の上限は許可権限の一部に限り、発行上限の一部であることは求めない**：許可権限に含まれない組を権限の上限に含めると、Lambda の初期化が失敗する。発行上限に含まれることを求めると、発行上限を狭めただけで初期化が失敗し、どの要求も受け付けなくなる。発行上限の外の組は §3.1 の条件 3 が拒否するため、許す範囲は広がらない
- **条件を加えるだけにする**：保護対象の一覧は、§3.1 の条件 1 から 3、Installation の条件、GitHub App に付与された権限の条件で拒否される要求を許可に変えず、保護対象を含まない要求の結果も変えない
- **欠落させない**：保護対象の一覧の欠落と不正な値は、Lambda の初期化の失敗にし、どの要求も受け付けない。保護対象を置かないデプロイは、空の一覧を明示する。欠落を「保護なし」と読むと、設定の漏れで保護が外れるためである
- **以後の発行要求にだけ効く**：一覧を変えても、発行済みのトークンは失効させない。Lambda の設定の更新が完了するまでの呼び出しは、更新前の一覧で判定される（§5）

一覧と GitHub の状態が食い違ったときの結果は次のとおりです。

| 出来事 | 発行前 | 発行後 | 結果 |
|---|---|---|---|
| 保護対象を改名し、新しい名前で要求 | 名前が一覧にないため通る | Repository ID が保護対象の ID で、名前が一致しない | 照合失敗 |
| 保護対象を改名し、旧名で要求 | 名前で保護対象と判定する | GitHub が `422` で拒否し、発行結果がない（[GitHub App Constraints](../external/github/app.md) §4.2 の観測。GitHub は保証していない） | 認可拒否か GitHub 発行拒否 |
| 保護対象を削除し、同じ名前で作り直して要求 | 名前で保護対象と判定する | 名前が一致し、Repository ID が一致しない | 認可拒否か照合失敗 |
| 一覧の保護対象の ID を書き誤る | 名前で保護対象と判定する | 名前が一致し、Repository ID が一致しない | 認可拒否か照合失敗 |
| 一覧の保護対象の名前を書き誤る | 名前が一覧にないため通る | Repository ID が保護対象の ID で、名前が一致しない | 照合失敗 |
| 許可 Caller のリポジトリを改名する | Repository ID は一致し、名前が一致しない | なし | 認可拒否 |
| 許可 Caller のリポジトリを削除し、同じ名前で作り直す | 名前は一致し、Repository ID が一致しない | なし | 認可拒否 |

照合失敗になる行では、GitHub 側でトークンが一度発行され、GATES は利用者に返さずに失効を試みます。
一覧を直すまで、その保護対象への発行は止まります。
保護対象の条件による認可拒否は、ほかの認可拒否と同じく、監査記録の拒否の理由に満たさなかった条件として残り、一覧の ID と名前は記録しません（[Domain Model](domain.md) の Audit Record）。

Caller の名前と ID の出どころには、GitHub が保証していない前提があります。
`sub` のリポジトリ部の名前は、観測では REST API の `name` と大文字小文字まで一致し、改名の後に発行された JWT では新しい名前でしたが、GitHub はこれを定めていません（[GitHub Actions OIDC Constraints](../external/github/oidc.md) の GH-OIDC-REPOSITORY-NAME-OBSERVED、GH-OIDC-REPOSITORY-NAME）。
運用者が REST API で調べて一覧に書く Repository ID が JWT の `repository_id` と同じ値であることも、観測だけに依ります（同 GH-OIDC-REPOSITORY-ID-REST-OBSERVED、GH-OIDC-REPOSITORY-ID-REST）。
どちらの前提が崩れても、許可 Caller に当たる Caller が減るだけで、拒否に倒れます。

保護対象の一覧は Lambda の環境変数として渡すため、Lambda の設定を変更できる主体は、一覧を書き換えて保護を外せます。
発行上限と同じく（§3.3）、この主体は関数のコードも変更できるため、得られる能力は増えません。
一覧が抑えるのは、保護対象への要求のうち、レビューと Plan を経た一覧に載っていない Caller や権限によるものです。

保護対象について、次のリスクを受容します。

- **許可 Caller を経由した到達**：許可 Caller のリポジトリが Installation の対象に含まれ、保護対象でない場合、許可オーナーのどのワークフローも、`contents:write` と `workflows:write` のトークンでそのリポジトリのワークフローを書き換え（§3.5）、許可 Caller として保護対象を要求できる。発行上限と GitHub App の権限が書き込みを含むデプロイでは、許可 Caller のリポジトリも保護対象にするか、Installation の対象から外す
- **Caller の粒度**：Caller はリポジトリ単位で、ブランチ、イベント、ワークフローを見分けない。許可 Caller のリポジトリでワークフローを動かせる者は、その許可 Caller に許した範囲を使える。`pull_request_target` のように信頼できないコードがトークンを得る経路（§5）も、許可 Caller のリポジトリでは保護対象に届く
- **許可 Caller の改名と作り直し**：許可 Caller のリポジトリを改名するか作り直すと、一覧を直すまで、そのリポジトリのワークフローは保護対象を要求できない
- **改名の直後の発行**：保護対象を改名してから一覧を直すまで、新しい名前への要求のたびに GitHub 側でトークンが発行され、照合失敗で失効を試みる。失効は成功を保証しない（§3.4）
- **Repository ID の再割り当て**：GitHub が Repository ID をほかのリポジトリに再び割り当て、そのリポジトリが許可 Caller と同じ名前を持つと、そのリポジトリは許可 Caller として扱われる。GitHub は再び割り当てないことを明記していない。名前も同じでなければ拒否に倒れるため、このリスクを受容する
- **YAML を読む外部のライブラリ**：Lambda は保護対象の一覧を読むため、外部の YAML のライブラリ一つに依存する。ライブラリの欠陥は、一覧の読み違いか初期化の失敗として現れ得る。読ませるのは運用者が書いた環境変数だけで、利用者の Payload は読ませない。認可に要らない YAML の機能（アンカー、エイリアス、タグ、重複したキーなど）を構文木の上で拒否し、構文木から作った値を不変条件（[Domain Model](domain.md) の INV-016）で確かめてから使うため、ライブラリの解釈が運用者の意図と分かれる範囲を狭める。ライブラリは脆弱性の検査（govulncheck、Trivy）と依存の更新の対象に含め、YAML のライブラリを import できる Go の package を一つに限る

## 4. Secret and Token Lifecycle

KMS Key の構成・IAM 権限は Terraform で管理します。GitHub App Private Key の取得・変換・鍵素材インポートは独立した運用で行い、Terraform の State / Plan と Go アプリケーションには鍵素材を渡しません。

| 資格情報 | 保存・利用の境界 |
|---|---|
| GitHub App Private Key | KMS インポート前は運用者が別の運用境界で扱う。Lambda と Runner には渡さない。インポート後、インポート前のコピーは運用者の手元から削除する。KMS にインポートした鍵素材は取り出せないため、KMS Key を失った場合は GitHub 側で新しい秘密鍵を発行して切り替える |
| AWS KMS Key | インポート済みの非対称署名鍵。Lambda 実行 Role に当該キーの `kms:Sign` のみ許可。鍵更新時は新 KMS Key を用意し、Lambda が参照する鍵（使用中の鍵）を切り替える。切り替えの確認の後に旧 KMS Key を無効にし、GitHub 側で旧 Private Key を削除した後に、旧 KMS Key を Terraform の管理から外して削除を予約する |
| GitHub App JWT | KMS 署名で Lambda が短時間生成し、GitHub API 呼び出しにのみ使用。外部返却・ログ出力禁止 |
| Installation Access Token | 要求・認可された対象と Permissions（付随する `metadata:read` を許容）に限定して Runner へ返す。短命でも窃取リスクは残る。照合に失敗したものは返さず、文字列を取得できた場合に失効を試みる。Runner へ返したものは、ジョブの終了時に Docker Action がトークン自身で認証して失効を試みる（ジョブの終了時の失効）。Docker Action がトークンを取り出した後に失敗するかキャンセルされ、後続ステップへ渡せなかったものは、その場で失効を試みる（失敗時の失効）。Runner の上では、Output（`GITHUB_OUTPUT`）と `GITHUB_STATE` の 2 つのファイルにだけ書く。どちらも Runner のファイルコマンドのディレクトリにある |
| AWS 一時認証情報・AWS 用 OIDC JWT・GATES 用 OIDC JWT | Runner の上で Docker Action の子のプロセスのメモリの中だけで扱い、ファイル、環境変数、Output に書かない。プロセスの終了とともに手放す。ログやデバッグ出力に含めない |

## 5. Failure, Audit, and Security Risks

- 入力が不正、または認証に失敗したら、GitHub API や KMS のトークン発行処理に進まない。Installation の解決や署名の失敗、発行結果の不一致は Fail Closed とする。
- 監査記録には相関 ID、時刻、処理結果（成功またはエラーコード）、拒否の理由に加え、認証済み Caller の Owner ID と Repository ID、ワークフロー実行 ID と試行番号、ワークフロー起動属性、要求対象（`owner/repo`）、要求権限、取得した Installation の Installation ID と `account.id`、照合に失敗した場合の失効の結果、署名検証済みの Claims Set を記録する。ワークフロー実行 ID と試行番号を残すのは、監査記録から GitHub 上のワークフロー実行にたどり着けるようにするためで、認可には使わない。認証前の失敗では Caller が確定していないため、未検証の値を認証済み主体として記録しない。秘密鍵、OIDC JWT、GitHub App JWT、Installation Access Token、AWS 一時認証情報は記録しない。
- ワークフロー起動属性（`actor`、`event_name`、`ref`、`workflow_ref`、`environment` など）を残すのは、後述の `pull_request_target` の悪用のように、どのイベントと ref でトークンを得たかを調べる場面に備えるためである。GitHub 側の記録は保持期間を過ぎたり削除されたりし得る。ワークフロー起動属性はベストエフォートで記録し、Claim が欠落していても拒否しない。IAM Role の引き受けと Lambda の呼び出しは AWS 側の記録（CloudTrail）が担い、GATES の監査記録はそれを補うアプリケーション層の記録と位置づける。IAM Role の引き受けは CloudTrail の管理イベントで、拒否された引き受けも既定で残る。Lambda の呼び出しは CloudTrail のデータイベントで、データイベントの記録を有効にしていない場合は、成功も拒否も残らない（[AWS Lambda Invoke Rejection](../research/aws/lambda-invoke-rejection.md#cloudtrail)、[AWS STS Legacy Subject](../research/aws/sts-legacy-subject.md#cloudtrail)）。成功した引き受けの記録には、発行された AWS 一時認証情報の `accessKeyId` と `sessionToken` が残る（`secretAccessKey` は残らない）。
- 署名検証済みの Claims Set の記録では、署名を検証する前の Payload は、攻撃者が内容と大きさを自由に作れるため残さない。JWT の文字列（署名付きの compact 形式）は有効期限内は Bearer の資格情報であるため残さず、Claims Set の JSON オブジェクトだけを残す。`head_ref`、`actor`、`repo_property_*` などは利用者が選んだ文字列を含むが、値ごとの検査はせず、JSON として再符号化するときのエスケープに任せる。署名検証済みの Claims Set は監査記録にだけ書き、Caller への応答やワークフローのログには出さない。パブリックリポジトリでは GitHub 側のログを誰でも読めるためである。
- 初期化の失敗（固定設定の欠落か不正、AWS SDK の設定の読み込みの失敗）のログには、満たさなかった条件と、どの値の条件かを示す環境変数の名前か固定設定の概念名だけを書き、値を書かない。GATES 用 Audience と KMS Key ARN は資格情報ではないが、ログに書く値を増やさない。
- Handler が回復した panic の値は、応答、監査記録、ログのどれにも書かない。値に資格情報が入っていない保証がないためである。
- 相関 ID は成功応答と失敗応答の両方に、同じ位置で含める。ワークフローのログに残った相関 ID から監査記録へたどれるようにするためである。
- AWS Invoke の成功と Lambda の業務成功を区別し、Docker Action は `FunctionError` と応答の両方を確認する。
- 許可オーナー配下のリポジトリであることは、ワークフロー内のコードの信頼性を保証しない。同じリポジトリのブランチからの Pull Request の `pull_request` と `pull_request_target`、Environment を使うジョブ、`workflow_run` のどれも、IAM Role を引き受けてトークンを得られた（[GitHub Workflow Event Contexts](../research/github/workflow-event-contexts.md)）。`pull_request_target` と `workflow_run` のジョブは、Pull Request の head や起点のワークフローのファイルではなく、base のリポジトリの既定のブランチのワークフローで動く。GitHub の公式資料によれば、fork からの Pull Request が起点でも Secrets と write の `GITHUB_TOKEN` を持つため、宣言した `id-token: write` も持つ見込みである（fork からの起動は実環境では未確認）。そのジョブが Pull Request のコードや起点のワークフローの成果物を実行すると、そのコードがトークンを得られる。`pull_request_target` の `sub` は `pull_request` と同じ形のため、Trust Policy ではこの 2 つを区別できず、GATES もイベントの種別で拒否しない。fork からの Pull Request の `pull_request` は、公式資料によれば既定では write の権限を read に下げられるため `id-token: write` を持たず、プライベートリポジトリで「Send write tokens to workflows from pull requests」を有効にすると持つ見込みである（実環境では未確認）。利用者は資格情報を取得するジョブで信頼できないコードを実行しない。
- `pull_request_target` と `workflow_run` の監査記録では、`ref` と `sha` は既定のブランチの値になる。`workflow_run` には起点のワークフロー実行を示す Claim がなく、署名検証済みの Claims Set にも起点は残らない。`pull_request_target` では署名検証済みの Claims Set に `head_ref` と `base_ref` が残るが、`head_ref` は Pull Request の元ブランチの名前だけで、Pull Request の番号、fork の持ち主、head の commit SHA を含まないため、Pull Request を一意に特定できるとは限らない。起点は、`caller.run_id` から GitHub のワークフロー実行をたどって調べる。GitHub 側の記録の保持期間を過ぎると、起点を調べられないリスクを受容する。
- JWT、AWS 認証情報、Installation Access Token の漏えい・再利用を完全には防止しない。許可範囲、期限、権限とインストール対象の限定で影響を抑える。
- Docker Action のコンテナは root で動き、Runner はそこに Docker ソケット（`/var/run/docker.sock`）をマウントするため、次のリスクを受容する。コンテナの中のコードは、ソケットを通じて Runner のホストの root に相当する権限を得られる。調査用のコンテナは、ソケットを通じてホストの `/` をマウントしたコンテナを起動し、ホストのファイルを読めた（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md#docker-socket)）。この権限は Docker Action のコンテナに固有のものではない。GitHub がホストする Linux の Runner では、同じジョブの `run` のステップもパスワードなしの `sudo` を使え、観測した Runner ではソケットの所有グループ（`docker`）にも属していた。コンテナの中で動くのは Docker Action のプログラムだけで、そのプログラムはソケットを使わない。このリスクが現れるのは、Docker Action のプログラム、その依存、ビルド用のイメージが改ざんされた場合である。マウントは Runner が決め、Docker Action からは外せない。依存とビルド用のイメージをバージョンと digest で固定し、利用者が Docker Action を commit SHA で参照することで、改ざんされたものが取り込まれる機会を抑える
- Docker Action のコンテナには、`RUNNER_TEMP` と `HOME` が、同じジョブのほかのステップと共有されるディレクトリとしてマウントされる。Docker Action がこれらに書く資格情報は、`RUNNER_TEMP` の下にある Output と `GITHUB_STATE` のファイルの中のトークンだけである。Docker Action の実行の前後で `RUNNER_TEMP` と `HOME` に増えたファイルのうち、トークンを含むのはこの 2 つだけだった（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md#files-left-by-the-gates-action)）。Runner はステップごとのファイルコマンドのファイルをステップの後も消さず、後続のステップはそれを読めた。前のステップが起動したバックグラウンドのプロセスはステップをまたいで動き続け、Docker Action のステップの最中にもそのファイルを読めた（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md#file-commands-across-steps)）。このため、トークンを同じジョブのほかのステップから隠すことはできない。Docker Action のコンテナは同じジョブのほかのステップに対する隔離の境界ではなく、資格情報を守る境界はジョブである。資格情報を取得するジョブで信頼できないコードを実行しない前提（前述）で、このリスクを受容する
- Runner は、`id-token: write` の有無によらず、GitHub Actions のランタイムの操作に使う `ACTIONS_RUNTIME_TOKEN` を Docker Action のコンテナに渡す。`id-token: write` を宣言したジョブでは要求用トークン（`ACTIONS_ID_TOKEN_REQUEST_TOKEN`）も渡り、その値は `ACTIONS_RUNTIME_TOKEN` と同じだった。同じジョブの `run` のステップには、`ACTIONS_RUNTIME_TOKEN` は渡らないが、同じ値の要求用トークンが渡る（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md#request-token-in-run-steps)）。Docker Action は `ACTIONS_RUNTIME_TOKEN` を読まず、要求用トークンを OIDC JWT の取得にだけ使うが、どちらも渡されることは止められない。改ざんされた Docker Action のプログラムは、このアクセストークンでジョブのランタイムの操作を行い得る。ランタイムの操作は試していないが、`id-token: write` のジョブでは `run` のステップも同じ値を持つため、受容の判断は操作の範囲によらない。上記の Docker ソケットと同じく、改ざんの機会を抑えることで対処する
- ジョブの終了時の失効と失敗時の失効はベストエフォートであり、次の場合に、Runner へ返したトークンが GitHub 側の有効期限まで有効なまま残るリスクを受容する。失効の要求が失敗した場合と、Runner がコンテナを強制終了するか Runner が停止して、失効を試みる前に Docker Action が終わった場合である。ジョブのキャンセルでは、Docker Action はシグナルを受けてから失効を試みて終えるため、このリスクは Runner がそれより先にコンテナを止めた場合に限られる。残ったトークンの範囲は、要求した対象と Permissions に限られる。読み取り以外の権限を要求していれば、残ったトークンでその操作ができる。失効の要求が `204` を返した後も、短い間はトークンが使えることがある。GitHub は失効が効くまでの時間を公開していない（[GitHub App Constraints](../external/github/app.md)）。その間にトークンを使えるのは同じジョブのステップとプロセスで、それらは失効の前からトークンを読めるため、このリスクを受容する
- 保持した Issuer の公開鍵に有効期限を設けず、次のリスクを受容する。Issuer が JWKS から鍵を取り下げても、保持した鍵で検証できない `kid` のある JWT が届いて JWKS の取得に成功するか、実行環境が入れ替わるまで、取り下げた鍵で検証できる JWT を受理し続ける。Lambda は呼び出しが続く関数でも実行環境を数時間ごとに終了するため、受理し続ける期間はその程度に収まる見込みだが、AWS は正確な間隔を公開しておらず、GATES はこの期間の上限を保証しない。取り下げた鍵で新しい JWT を偽造するには、対応する秘密鍵が必要である。一方、取り下げ前に発行された JWT の再提示には秘密鍵が要らず、その JWT の `exp` までは受理され得る。どちらの悪用にも、Lambda の Invoke を許可された IAM 認証情報が別に必要である。期限を設けると、期限切れのたびに取得が発生し、Issuer の障害で正当な JWT まで拒否し得る。この可用性の低下を避ける。Issuer の鍵の漏えいが判明したときは、運用者が関数のコードか設定を更新する。AWS は更新の完了まで呼び出しが更新前のコードと設定を使うと定めており、完了後の呼び出しが保持した鍵を持たない実行環境で動くことは、この定めからの推論である（公式文書に明記はない）。検証環境では、関数の環境変数とタイムアウトを変えた後の最初の呼び出しが、初期化から始まる新しい実行環境で動いた（[AWS Fault Injection](../research/aws/fault-injection.md)）。
- Discovery と JWKS の再取得の上限は実行環境ごとであり、関数全体の上限を設けず、次のリスクを受容する。IAM の認可を得た呼び出し元は、手元の鍵で検証できない JWT を送ることで、実行環境ごとに、再取得の上限が許す頻度まで取得を起こせる。署名の検証には鍵が要るため、取得の前に不正な JWT を見分けることはできない。Issuer がレート制限を返すと、鍵を持たない実行環境とローテーション後の鍵を要する JWT は、再試行できる失敗で拒否され続け得る（Issuer のレート制限の有無は未確認）。影響は可用性にとどまり、この場合も GATES はトークンを返さない。実行環境をまたいで上限を共有すると、そのためだけに外部の状態、権限、障害の経路が増えるため採らない。同時に動く実行環境の数は Reserved Concurrency で抑えられるが、実行環境が入れ替わると新しい環境は取得の履歴を持たないため、厳密な上限にはならない。

### 5.1 KMS Signing Key Risks

KMS Key の署名能力は、GitHub App として認証する手段そのものです。
そのため、Key Policy と IAM で抑える範囲と、それでは抑えられずに受容する範囲を分けて定めます。

次のリスクは、設定で抑えます。

| リスク | 抑える手段 |
|---|---|
| 秘密鍵の取り出し | KMS にインポートした鍵素材は取り出せない。Terraform の State / Plan、Git、CI の成果物、Lambda の環境変数に鍵素材を渡さない |
| 実行 Role による鍵の管理 | 実行 Role に許可する KMS の操作を `kms:Sign` だけにする |
| 実行 Role による別の方式での署名 | Key Policy で署名アルゴリズムを `RSASSA_PKCS1_V1_5_SHA_256` に限る |
| 実行 Role による別の鍵での署名 | 実行 Role の IAM ポリシーに使用中の鍵のほかの許可を含めず、GATES の鍵の集合（新旧の 2 つまで）のほかの鍵の Key Policy に実行 Role を書かない |
| IAM の管理権限を持つ主体による直接の署名 | Key Policy で root に許可する操作を列挙し、`kms:Sign`、`kms:*`、`kms:CreateGrant` を含めない |
| Terraform の操作による鍵の削除と置換 | `prevent_destroy` で該当する Plan を拒否する。鍵の集合から鍵を除いた Plan も拒否する |
| 予約した削除の実行 | Terraform が管理する鍵の削除の予約期間（`deletion_window_in_days`）を 30 日にし、その間は `kms:CancelKeyDeletion` で取り消せるようにする |
| CI からの権限の変更 | Terraform を運用者の手元から適用し、CI に適用権限を与えない |

これらの設定は「鍵を管理できる主体は署名できない」を意味しません。
保証するのは、実行 Role 以外の主体が署名するには、Key Policy、Lambda、実行 Role の Trust Policy のいずれかの変更か、実行 Role を別の関数に渡す操作が別に必要になるところまでです。

次のリスクは受容します。

- **実行 Role の侵害**：Lambda のコードの脆弱性などで実行 Role の権限を奪われると、`kms:Sign` で不正に署名できる。KMS は秘密鍵の取り出しを防ぐが、署名能力の悪用は防がない
- **`kms:PutKeyPolicy` を持つ主体**：Key Policy を書き換えて、自身に署名を許可できる。`kms:CreateGrant` を一時的に許可して作った Grant は、Key Policy を元に戻しても残る。この Grant を確認して取り消せるように、root に `kms:ListGrants` と `kms:RevokeGrant` を許可する
- **Lambda の更新権限を持つ主体**：関数のコードや設定を変更して、実行 Role の権限で署名させられる。`iam:PassRole` を持つ主体は、実行 Role を別の関数に渡して同じことができる
- **`iam:UpdateAssumeRolePolicy` を持つ主体**：実行 Role の Trust Policy を書き換えて Role を引き受け、署名できる
- **Permissions Boundary を通る別の鍵での署名**：Boundary は同じアカウントとリージョンの KMS Key のうち、鍵素材をインポートした鍵への `RSASSA_PKCS1_V1_5_SHA_256` の `kms:Sign` を許可する。GitHub Actions が引き受ける IAM Role と Lambda 実行 Role のどちらかが、この条件に合う別の鍵で署名できる経路は 2 つある。Role に `kms:Sign` を許可するポリシーが追加され、その鍵の Key Policy が IAM ポリシーへ委任している経路と、その鍵の Key Policy が Role を Principal として直接許可する経路である。後者は Role 側のポリシーが変わらない。GATES の署名鍵は Key Policy が root に `kms:Sign` を許可しないため、前者の経路では署名できない。Boundary を GATES の署名鍵に絞ると Terraform の依存が循環するため、この範囲を受容する
- **適用とインポートを同じ主体が担う構成**：Key Policy は鍵素材のインポートを許可する主体を区別しない。鍵素材のインポートの権限を通常の Terraform 適用から分離することは IAM の権限の割り当てに依存し、Key Policy では保証しない
- **インポート前の秘密鍵のコピー**：運用者の手元のコピーを削除することは手順で定めるだけで、設定では強制できない
- **鍵の更新の間の、使用中でない鍵での署名**：鍵の更新の間は、新旧どちらの鍵の Key Policy も実行 Role に `kms:Sign` を許可するため、実行 Role は使用中でない鍵でも署名できる。使用中でない鍵の Key Policy から実行 Role を外さないのは、外すと切り替えと切り戻しのたびに 2 つの Key Policy を書き換えることになり、上記の `kms:PutKeyPolicy` のリスクを持つ操作が増えるためである。2 つの鍵はどちらも同じ GitHub App の Private Key であり、実行 Role が侵害されたときに得られる能力（GitHub App としての認証）は 1 つの鍵のときと変わらない。新しい鍵は鍵素材のインポートまで署名できない。旧鍵は、切り替えの後の発行を確かめてから運用者が無効にし、この期間を短くする。GitHub 側で旧 Private Key を削除した後は、旧鍵での署名は GitHub に受理されない見込みである（削除した鍵の JWT を GitHub が拒否することは、公式資料からの推論で、GATES を通しては確かめられない）
- **Terraform の管理から外した旧鍵**：退役させる鍵を State から外すと、その鍵の Key Policy と状態は Terraform の Plan に現れなくなる。外す前に鍵を無効にし、外した後は削除を予約する手順で、残る期間を予約期間（7 日）に限る。予約期間のあいだは `kms:EnableKey` と `kms:CancelKeyDeletion` を持つ主体が鍵を戻せるが、GitHub 側で Private Key を削除した後のため、戻した鍵で署名しても GitHub App として認証できない見込みである
- **Terraform の設定の除去による鍵の削除**：`prevent_destroy` はリソースの設定自体を取り除いた場合の削除を防がない。Plan に現れた鍵の削除を運用者が確認して止める
- **KMS の API からの鍵の無効化と削除の予約**：`kms:DisableKey` や `kms:ScheduleKeyDeletion` を IAM で許可された主体は、Terraform を経ずに実行できる。`prevent_destroy` は効かず、Terraform は鍵の有効と無効を設定に書かないため、無効化は Plan にも現れない（[AWS Fault Injection](../research/aws/fault-injection.md)）。Lambda は署名できなくなってトークンを発行できない。影響は可用性にとどまる。鍵の悪用が判明したときに一時的に止める手段でもあるため、Key Policy から外さない

`kms:PutKeyPolicy`、Lambda の更新権限、`iam:UpdateAssumeRolePolicy` によるリスクを受容するのは、これらが Terraform の適用に必要な権限と重なり、Key Policy の記述では取り除けないためです。
`kms:PutKeyPolicy` を Key Policy から外すと Terraform で Key Policy を更新できなくなり、署名を拒否する `Deny` を加えても同じ権限で消せます。
これらの権限を持つ主体は、実行主体の限定と変更レビューで制御します。

不正に署名されると、GATES の認可を経ずに GitHub App JWT を作れるため、§3.1 と §3.3 の制限（発行上限と保護対象の一覧を含む）は効きません。
得られるトークンの上限は、GitHub App の権限と Installation の対象で決まります。
GATES が発行しない Organization permissions と Enterprise permissions も、GitHub App に付与され、GitHub App が Organization か Enterprise にインストールされていれば、この上限に含まれます。
Account permissions は、GitHub App JWT から得られる Installation Access Token には付かないため、署名能力の悪用だけでは得られません（[GitHub App Constraints](../external/github/app.md)）。
GitHub App に付与する権限を広げるほど、署名能力の悪用で得られる権限も広がります。
GATES は不正な変更や署名を通知する仕組みを持たないため、悪用に気付くまでの時間は保証できません。
悪用が判明したときは、GitHub 側で Private Key を失効させて止めます。
`kms:DisableKey` による停止は `kms:EnableKey` を持つ主体が元に戻せるため、これだけでは確実に止められません。
`kms:DeleteImportedKeyMaterial` は Key Policy に含めません。
インポート前のコピーを削除するため、鍵素材を消すと同じ鍵には戻せず、悪用を止める目的には GitHub 側の失効で足りるからです。

## 6. Traceability

| 機能 | 対応要求 |
|---|---|
| 入口制御と Caller の識別（§2） | FR-AUTHN-001〜005、FR-OPS-002 |
| 発行認可（§3.1〜3.3、§3.5、§3.6） | FR-AUTHZ-001〜010、FR-OPS-001 |
| 署名と発行結果の検証（§3.4、§4） | FR-ISSUANCE-001〜005、QR-003、QR-008 |
| 失敗時の扱いと監査（§5） | FR-OPS-003、FR-OPS-004、FR-ISSUANCE-006、QR-003、QR-004 |
| 署名鍵のリスクの抑制と受容（§5.1） | FR-ISSUANCE-001、FR-OPS-005、QR-008 |

受け入れ条件は [Acceptance Criteria](../spec/acceptance-criteria.md) を正本とします。

## 7. References

- [GitHub App Constraints](../external/github/app.md)
- [AWS OIDC JWT](aws-oidc-jwt.md)
- [GATES OIDC JWT Claims](gates-oidc-jwt-claims.md)
- [GATES OIDC JWT Signature](gates-oidc-jwt-signature.md)
- [External](../external/README.md)
- [GitHub — Securely using pull_request_target](https://docs.github.com/en/actions/reference/security/securely-using-pull_request_target)
- [GitHub — Permissions required for GitHub Apps](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps)
- [GitHub — GitHub-hosted runners reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)
- [AWS — IAM and AWS STS condition context keys](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_iam-condition-keys.html)
- [AWS — Lambda Invoke API](https://docs.aws.amazon.com/lambda/latest/api/API_Invoke.html)
- [AWS — Lambda execution environment lifecycle](https://docs.aws.amazon.com/lambda/latest/dg/lambda-runtime-environment.html)
- [AWS — Lambda function states](https://docs.aws.amazon.com/lambda/latest/dg/functions-states.html)
- [AWS — KMS Sign API](https://docs.aws.amazon.com/kms/latest/APIReference/API_Sign.html)
- [AWS — Imported key material](https://docs.aws.amazon.com/kms/latest/developerguide/importing-keys-import-key-material.html)
- [AWS — KMS key policies](https://docs.aws.amazon.com/kms/latest/developerguide/key-policies.html)

## Appendix: Impact of Future Scope

[Scope](../spec/scope.md) が将来取り込むことを想定する機能を導入する場合、本ドキュメントには次の影響が及びます。

- **細粒度の認可の残る部分**：アクセスされる側のリポジトリの管理者が制御するなら、中央の保護対象の一覧との重ね合わせと、管理者の設定を守る手段を評価する。保護対象の一覧にないリポジトリを既定で拒否するなら、§3.5 で受容したリスクを見直す。監査のためだけに用いているワークフロー起動属性を認可に使って Caller をリポジトリより細かく見分けるなら、属性の値を変えられる主体と、認可の根拠にできるかを評価する
- **Organization permissions、Account permissions、Enterprise permissions の発行**：認可条件 3、権限の境界、発行結果の照合が変わる。リポジトリを単位とする認可条件と照合をそのまま一般化しない。Account permissions は Installation Access Token に付かないため、発行するにはユーザーアクセストークンを扱う必要がある。ユーザーアクセストークンは本人のブラウザ操作で得て、リフレッシュトークン（有効期限 6 か月）で取り直すため、長期の資格情報を GATES の側で保管することになり、資格情報の扱いから見直す
- **OIDC JWT の再利用の検出とレート制限**：受容している JWT の再利用のリスクを見直す。どちらも実行環境をまたいで要求の履歴を共有する必要があり、再取得の上限の共有を採らない理由とした外部の状態、権限、障害の経路が増えるため、それらの脅威と、その状態に障害が起きたときに Fail Closed とするかを評価する

スコープを変更するときは、先に本ドキュメントで脅威と緩和策を評価し、その結果を踏まえて [Requirements](../spec/requirements.md) と各設計書を変更します。
