# GATES OIDC JWT Claims

本ドキュメントは、GATES 用 OIDC JWT の Claims Set に GATES が課す受理条件を定めます。
GATES 用 OIDC JWT は GitHub Actions OIDC Provider が発行し、Lambda の OIDC Verifier が検証する JWT です。
Caller の認証と監査（ワークフロー起動属性と、調査用に残す Claims Set の全体）に使い、GATES 固有の Audience を指定して取得します。
同じジョブで取得する AWS 用 OIDC JWT とは Audience が異なる別の JWT です。

Claim の意味と使い方、Claim 間の整合を記述します。
Go のパッケージ構成、Discovery / JWKS の取得・キャッシュ、エラー応答、信頼境界と残留リスクはそれぞれのコンポーネント設計の責務です。
Caller とその不変条件の概念的な定義は [Domain Model](domain.md) を正本とします。

本書の受理条件は、採用する設計契約です。
GitHub が定めていない事柄は [GitHub Actions OIDC Constraints](../external/github/oidc.md) の Evidence が Unconfirmed の条項に、GATES がそれによって拒否し得る発行は各規則の Possibly Rejects の行に書きます。

## Rule ID

各規則には `GOJ-` で始まる ID を付けます。
`GOJ` は GATES 用 OIDC JWT を表します。
一つの ID は一つの判定を表し、関係する外部仕様の条項の組、`formal/` の述語、検査のどれかが異なる判定は、別の ID に分けます。
Claim ごとの規則の ID は、Claim 名を大文字にして `_` を `-` に置き換えたものです（`repository_owner_id` なら `GOJ-REPOSITORY-OWNER-ID`）。
一つの Claim に複数の判定があるときは、Claim 名から作った ID の後に、判定の内容を表す名前を続けます（`sub` の形式の GOJ-SUB-FORMAT と、長さの GOJ-SUB-LENGTH）。
JOSE Header の規則の ID は `GOJ-HEADER-` で始め、Header Parameter ごとの規則では Header Parameter 名を大文字にして続けます（`alg` なら GOJ-HEADER-ALG）。
Claim ごとでない規則の ID は、規則の内容を表す名前です（GOJ-TIME-EXP、GOJ-ID-MATCH など）。
新しい Claim の ID が既存の ID と重なるときは、既存の ID を変えず、Claim 名から作った ID の末尾に `-CLAIM` を付けます。
既存の ID の参照元が、別の規則を指すようにならないためです。

## Sources and Acceptance Categories

### 関係の表

各規則の見出しの直下に、外部仕様の条項との関係を表で書きます。
1 行に一つの関係の種類（Relation）と一つの条項（Clause）を置き、補足を Note に書きます。

- **Satisfies**：規則に従って受理と拒否を決めれば、標準仕様の条項に従ったことになる
- **Narrows**：標準仕様の条項が許す入力の一部を拒否する。狭める理由を Note に書く
- **Accepts**：GitHub の条項が表す発行を受理する
- **Possibly Rejects**：GitHub の条項が許す発行の一部を拒否する。GATES が受け入れた既知の非互換を表し、条項の多くは Evidence が Unconfirmed で、GitHub がその発行を実際に行うかは確定していない。拒否し得る発行と、受け入れた理由を Note に書く
- **None**：関係する条項がない

条項の内容は、標準仕様は [External](../external/README.md) に挙げた標準仕様の文書、GitHub の仕様と観測は [GitHub Actions OIDC Constraints](../external/github/oidc.md) を正本とし、規則の本文には繰り返しません。
本文には、GATES の判断とその理由を書きます。
GitHub の条項の Evidence のうち、Published と Observed は取得・観測した時点と条件での値です。Published と Observed の条項だけを根拠にした Accepts の行は、GitHub が保証した受理ではありません。

GATES は、適用対象の標準仕様が要求する検証を省略しません。追加で受理対象を狭くする場合は、Narrows の行で狭める条項と理由を、Possibly Rejects の行で互換性への影響を示します。「理解しない Claim / Header は無視する」という標準規定も尊重し、無関係な新規 Claim の追加だけを拒否の理由にしません。Claims Set の JSON の名前重複、不正な UTF-8、既知 Claim の解釈の曖昧さは、未知 Claim と区別して [GOJ-CLAIMS-JSON](#goj-claims-json-claims-set-の-json)、[GOJ-CLAIM-NAME-MATCH](#goj-claim-name-match-claim-名の厳密一致)、[GOJ-CLAIM-NAME-ALIAS](#goj-claim-name-alias-claim-名の別名の拒否) で扱います。

GH-OIDC-DISCOVERY-CLAIMS の `claims_supported` は、サポートする Claim の一覧として読み、すべての JWT に全 Claim が存在する保証とは読みません（GH-OIDC-CLAIM-PRESENCE）。

### 受理の分類

- **必須**：欠落、`null`、型の不一致、個別条件の不一致を拒否する。必須 Claim が成立しなければ Caller を生成しない。
- **任意・検証**：欠落を許す。存在する場合は定めた型・値・時間条件を満たす必要がある。
- **任意・監査**：欠落や型の不一致を許し、正しい型の値だけをワークフロー起動属性として監査に転送する。認可の根拠には使用しない。監査する値は構造化形式でエスケープして記録する。
- **未使用**：Claim 自体の存在・値で追加の認可判断をしない。未知 Claim を含む JWT も、共通 JSON / 署名規則には従う。
- **拒否**：GATES 用 OIDC JWT の検証条件を満たさない場合は Caller を確定せず、認証失敗として扱う。鍵の取得や JWKS 全体の異常による失敗は、JWT 自体の検証失敗とは別の障害として扱う。

## Roles in Standard Clauses

### GATES の役割に当たる Actor

標準仕様の条項の Actor のうち、GATES が当たるものは次のとおりです。
Level が MUST、MUST NOT、REQUIRED、SHALL、SHALL NOT で、Actor がこの一覧にある条項は、いずれかの規則の Satisfies か Narrows の行に挙げるか、[適用しない標準の条項](#standard-clauses-not-applied) の表に挙げます。
この一覧は [GATES OIDC JWT Signature](gates-oidc-jwt-signature.md) にも適用します。

- **Recipient**：JWT を受け取って処理する主体。GATES は GATES 用 OIDC JWT の Recipient である
- **Relying Party**：OpenID Provider の設定と鍵を使って検証する主体。GATES は GitHub Actions OIDC Provider の Discovery と JWKS を使う主体として当たる。ID Token を受け取る主体としては当たらない（[ID Token として扱わないこと](#id-token-として扱わないこと)）
- **Application**：JWT を使うアプリケーション。GATES の Lambda が当たる
- **Implementation**：JWS、JWT、JWK、JWK Set を処理する実装。GATES のコードと、GATES が使うライブラリを合わせたものが当たる
- **Implementer**：標準仕様が許す選択（時刻の判定の猶予など）を決める実装者。GATES の設計が当たる
- **Library**：JWT の暗号操作を行うライブラリ。GATES が使うライブラリが当たる。ライブラリが備えない検証は、ライブラリを使う GATES のコードが補う（RFC8725-CRYPTO-VALIDATE の本文）
- **JSON Parser**、**JWS Parser**、**JWT Parser**、**JWK Parser**、**JWK Set Parser**：JOSE Header、Claims Set、JWK、JWK Set を読み取る処理。GATES のコードと、GATES が使うライブラリが当たる

Issuer、Producer、OpenID Provider、Signer は、GATES 用 OIDC JWT と鍵情報を作る GitHub が当たる Actor で、GATES の役割ではありません。
これらの Actor の条項も、GATES が受理する JWT がその条項を満たすことを規則が保証する場合は、Satisfies の行に挙げます（`exp` を NumericDate に限る RFC7519-EXP-NUMERICDATE など）。

### ID Token として扱わないこと

OpenID Connect Core の ID Token は、Client が送った認証要求に応えて、OpenID Provider が End-User の認証の結果を Client に伝えるトークンです（[OIDC Core §2](https://openid.net/specs/openid-connect-core-1_0.html#IDToken)）。
GATES 用 OIDC JWT は、GitHub Actions のジョブが実行環境から要求して受け取る JWT で（[GitHub Actions Integration Constraints](../external/github/actions.md)）、GATES は認証要求を送らず、GitHub に登録した Client でもありません。
Audience も `client_id` ではなく、ジョブが指定する値です（GH-OIDC-AUD-CUSTOM）。
GitHub の Discovery は、認証要求を受ける `authorization_endpoint` と `token_endpoint` を持ちません（GH-OIDC-DISCOVERY-MEMBERS）。
同じ Discovery は `response_types_supported` に `id_token` を、`id_token_signing_alg_values_supported` に `RS256` を挙げますが、Metadata の名前は GATES の扱いを決めません。GATES が受け取る JWT が、認証要求への応答として発行されたものではないためです。

そのため GATES は、GATES 用 OIDC JWT を ID Token として扱いません。
Condition が ID Token や認証のフローに限る OpenID Connect の条項は、[適用しない標準の条項](#standard-clauses-not-applied) に挙げます。
Condition が ID Token に限る条項の内容を GATES が採るとき（`sub` の長さの上限）は、その条項を Satisfies に挙げず、RFC 7519 の条項を狭める Narrows の行の Note に書きます。
Condition を持たない OpenID Connect の条項（文字列の比較など）は、ほかの標準仕様の条項と同じく関係の表に挙げます。

## Claim Summary

以下の Claim は、GH-OIDC-DISCOVERY-CLAIMS の `claims_supported` に一致します。表は GATES での用途を先に把握するための索引です。正確な受理条件・根拠・例外は各 Claim の規則と、[Time](#time)、[Cross-Claim Invariants](#cross-claim-invariants) に定めます。

| ID | Claim | 受理の分類 | GATES での用途 |
|---|---|---|---|
| GOJ-SUB-FORMAT、GOJ-SUB-LENGTH | `sub` | 必須 | 不変形式を解析し ID Claim と照合 |
| GOJ-AUD | `aud` | 必須 | GATES 用 Audience のみ |
| GOJ-EXP | `exp` | 必須 | 期限切れ拒否、猶予なし |
| GOJ-IAT | `iat` | 必須 | 未来時刻の上限を検証 |
| GOJ-ISS | `iss` | 必須 | 固定 GitHub Issuer と完全一致 |
| GOJ-JTI | `jti` | 未使用 | 再利用検出を行わない |
| GOJ-NBF | `nbf` | 任意・検証 | 存在時は時刻を確認 |
| GOJ-REF | `ref` | 任意・監査 | ワークフロー起動属性 |
| GOJ-SHA | `sha` | 任意・監査 | ワークフロー起動属性 |
| GOJ-REPOSITORY | `repository` | 未使用 | なし |
| GOJ-REPOSITORY-ID | `repository_id` | 必須 | `sub` と照合、Caller 識別・監査 |
| GOJ-REPOSITORY-OWNER | `repository_owner` | 未使用 | なし |
| GOJ-REPOSITORY-OWNER-ID | `repository_owner_id` | 必須 | `sub` と照合、認可 |
| GOJ-ENTERPRISE | `enterprise` | 未使用 | なし |
| GOJ-ENTERPRISE-ID | `enterprise_id` | 未使用 | なし |
| GOJ-RUN-ID | `run_id` | 必須 | 監査識別 |
| GOJ-RUN-NUMBER | `run_number` | 未使用 | なし |
| GOJ-RUN-ATTEMPT | `run_attempt` | 必須 | 監査識別 |
| GOJ-ACTOR | `actor` | 任意・監査 | ワークフロー起動属性 |
| GOJ-ACTOR-ID | `actor_id` | 任意・監査 | ワークフロー起動属性 |
| GOJ-WORKFLOW | `workflow` | 未使用 | なし |
| GOJ-WORKFLOW-REF | `workflow_ref` | 任意・監査 | ワークフロー起動属性 |
| GOJ-WORKFLOW-SHA | `workflow_sha` | 未使用 | なし |
| GOJ-HEAD-REF | `head_ref` | 未使用 | なし |
| GOJ-BASE-REF | `base_ref` | 未使用 | なし |
| GOJ-EVENT-NAME | `event_name` | 任意・監査 | ワークフロー起動属性 |
| GOJ-REF-TYPE | `ref_type` | 未使用 | なし |
| GOJ-REF-PROTECTED | `ref_protected` | 未使用 | なし |
| GOJ-ENVIRONMENT | `environment` | 任意・監査 | ワークフロー起動属性 |
| GOJ-ENVIRONMENT-NODE-ID | `environment_node_id` | 未使用 | なし |
| GOJ-JOB-WORKFLOW-REF | `job_workflow_ref` | 任意・監査 | ワークフロー起動属性 |
| GOJ-JOB-WORKFLOW-SHA | `job_workflow_sha` | 未使用 | なし |
| GOJ-REPOSITORY-VISIBILITY | `repository_visibility` | 未使用 | なし |
| GOJ-RUNNER-ENVIRONMENT | `runner_environment` | 未使用 | なし |
| GOJ-ISSUER-SCOPE | `issuer_scope` | 未使用 | なし |
| GOJ-CHECK-RUN-ID | `check_run_id` | 未使用 | なし |

表の必須・任意・未使用は GATES の設計分類であり、GitHub の発行保証ではありません。

この一覧は Discovery の取得時点の掲載名に限ります。Organization / Enterprise の設定に応じて追加される `repo_property_*` や、将来の未知 Claim はこの一覧に含みません。これらの追加 Claim の扱いは [GOJ-DYNAMIC-CLAIM](#goj-dynamic-claim-discovery-に掲載されない動的-claim) に定めます。

## Claims

### GOJ-SUB-FORMAT `sub` の形式

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC8725-SUB-VALIDATE | 固定した Issuer の JWT のうち、不変形式で、2 つの ID が GOJ-ID-MATCH の照合を満たす `sub` を有効な Subject とする |
| Narrows | RFC7519-SUB-FORMAT | 不変形式のほかの `sub` を拒否する。名前の再利用で同じ値を作れる `sub` を、Caller の識別に使わないため。受理する `sub` が URI であること（RFC7519-STRINGORURI-URI）は確かめない |
| Narrows | RFC7519-SUB-OPTIONAL | `sub` がない JWT を拒否する。Caller の不変な識別子を `sub` から得るため |
| Accepts | GH-OIDC-SUB-AT-SEPARATOR | オーナー部とリポジトリ部の `@` をちょうど 1 個とする判定は、名前が `@` を含まないことに依存する |
| Accepts | GH-OIDC-SUB-CONTEXT | リポジトリ部より後を解釈しない |
| Accepts | GH-OIDC-SUB-IMMUTABLE-DEFAULT | なし |
| Accepts | GH-OIDC-SUB-IMMUTABLE-FORMAT | なし |
| Accepts | GH-OIDC-SUB-OBSERVED | なし |
| Possibly Rejects | GH-OIDC-CLAIM-PRESENCE | `sub` がない JWT を拒否する。Caller の不変な識別子を得られないため |
| Possibly Rejects | GH-OIDC-SUB-CUSTOM-REPLACE | `repo:` で始まらないカスタマイズ形式を拒否する。カスタマイズ形式に対応しないのは、現在の GATES の対象範囲による |
| Possibly Rejects | GH-OIDC-SUB-CUSTOM-REPO-ONLY | リポジトリ部の後に `:` がない形式を拒否する。`repo` だけを指定するカスタマイズに対応しないのは、現在の GATES の対象範囲による |
| Possibly Rejects | GH-OIDC-SUB-CUSTOM-WITHOUT-REPO | `repo` の部分を含まない形式を拒否する。カスタマイズ形式に対応しないのは、現在の GATES の対象範囲による |
| Possibly Rejects | GH-OIDC-SUB-LEGACY-RETAINED | 旧形式を拒否する。名前の再利用で別のオーナーが同じ値を作れるため（GH-OIDC-SUB-IMMUTABLE-REASON）。許可オーナーのリポジトリには、不変形式の利用を選択させる |
| Possibly Rejects | GH-OIDC-SUB-LENGTH | オーナーの名前が `/` を含む `sub` と、リポジトリの名前が `:` を含む `sub` を拒否する。オーナー部を最初の `/` で、リポジトリ部をその後の最初の `:` で区切り、名前と ID の位置を一つに決めるため。新しく作るリポジトリの名前は、公式資料の名前の規則により `:` を含まない（[GitHub Owner and Repository Constraints](../external/github/account.md)）。オーナーの名前（個人アカウントと Organization）の文字と、既存のリポジトリの例外的な名前は定まっていない |

**GATES の受理条件**：

- `sub` は欠落不可の JSON 文字列である。`repo:` で始まり、以後の最初の `/` によってオーナー部とリポジトリ部を分ける。
- リポジトリ部は最初の `:` までとし、`:` がなければ拒否する。オーナー部・リポジトリ部はそれぞれ非空の `NAME@ID` で、`@` は各部にちょうど1個とする。
- 両方の ID は [GOJ-ID-FORMAT](#goj-id-format-id-の表記) の正の整数の文字列表現に合致する。`repository_owner_id`・`repository_id` との一致は [GOJ-ID-MATCH](#goj-id-match-subject-と識別子の一致) で判定する。
- 名前部分は ID の認可根拠とせず、Claim の名前部分と他の名前の一致を認証条件にしない。`:` 以降のコンテキストは解釈・認可に使用しない。
- 旧形式、`repo` を含まないカスタマイズ形式、末尾の `:` がない形式は拒否する。

**理由**：`sub` の名前部分だけを信頼すると、名前の再利用で別の主体を許す危険があります。不変 ID と個別 Claim の両方を確認し、主体の取り違えを防ぎます。`sub` はワークフロー実行 ID ではなく、Caller の不変な識別子を得るための根拠です。

### GOJ-SUB-LENGTH `sub` の文字種と長さ

| Relation | Clause | Note |
|---|---|---|
| Narrows | RFC7519-SUB-FORMAT | ASCII 以外の文字を含む `sub` と、255 文字を超える `sub` を拒否する。受理する `sub` の範囲を、OpenID Connect Core が ID Token の `sub` に課す上限（OIDCC-SUB-LENGTH）に合わせるため |
| Possibly Rejects | GH-OIDC-SUB-LENGTH | ASCII 以外の文字を含む `sub` と、255 文字を超える `sub` を拒否する。長い ref や Environment の名前、ASCII 以外の文字から GitHub がそのような `sub` を発行し得るかは定まっていない。受理する `sub` の範囲を明示することを優先し、互換性の影響を受け入れる |

**GATES の受理条件**：`sub` 全体を ASCII 文字のみに限定し、文字数が255を超えたら拒否します。

**理由**：長さと文字種の条件は ID の一致による認可を強めるものではなく、受理する `sub` の範囲を明示するためのものです。GATES 用 OIDC JWT は ID Token として扱わないため（[ID Token として扱わないこと](#id-token-として扱わないこと)）、255 文字の上限も ASCII 以外の拒否も GATES が加える制約です。

### GOJ-AUD `aud`

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7519-AUD-INTENDED | GATES 用 Audience を、GATES を識別する値とする |
| Satisfies | RFC7519-AUD-REJECT | なし |
| Satisfies | RFC7519-STRING-COMPARE | なし |
| Satisfies | RFC7519-STRINGORURI-COMPARE | なし |
| Satisfies | RFC8725-AUD-REQUIRED | なし |
| Satisfies | RFC8725-AUD-VALIDATE | なし |
| Satisfies | RFC8725-KIND-CONFUSION-MITIGATE | 同じ Issuer が発行する AWS 用 OIDC JWT と、Audience で区別する |
| Satisfies | RFC8725-KIND-EXCLUSIVE | GATES 用 Audience は AWS 用 OIDC JWT の Audience `sts.amazonaws.com` と異なる値に限られ（Domain Model の INV-005）、GATES 用 OIDC JWT と AWS 用 OIDC JWT の検証規則は同時には満たされない |
| Narrows | RFC7519-AUD-FORMAT | 空配列と、要素が複数の配列を拒否する。受信者を増やした JWT を資格情報として再利用させないため |
| Narrows | RFC7519-AUD-OPTIONAL | `aud` がない JWT を拒否する。GitHub Actions OIDC Provider は同じ Issuer で AWS 用 OIDC JWT も発行し、RFC8725-AUD-VALIDATE がその場合の拒否を求めるため |
| Accepts | GH-OIDC-AUD-CUSTOM | なし |
| Accepts | GH-OIDC-AUD-CUSTOM-STRING | なし |
| Possibly Rejects | GH-OIDC-CLAIM-PRESENCE | `aud` がない JWT を拒否する。RFC8725-AUD-VALIDATE が求める拒否である |

**GATES の受理条件**：必須。固定設定の GATES 用 Audience と大文字小文字を含め完全一致する文字列1個、またはそれだけを含む要素数1の文字列配列を許容し、要素が複数なら GATES 用 Audience を含んでも拒否します。空文字列、空配列、別の Audience は拒否します。AWS 用 JWT の `aud=sts.amazonaws.com` とは区別します。

### GOJ-EXP `exp`

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7519-EXP-NUMERICDATE | なし |
| Narrows | RFC7519-EXP-OPTIONAL | `exp` がない JWT を拒否する。有効期限のない JWT を資格情報として受理しないため |
| Narrows | RFC7519-NUMERICDATE-FORMAT | 整数でない値と、範囲 `0..253402300799` の外の値を拒否する。時刻の値を、Claim を読み取る処理によって異なる値に読めないようにするため（FR-AUTHN-006） |
| Accepts | GH-OIDC-CLAIM-TYPE-EXAMPLE | なし |
| Accepts | GH-OIDC-CLAIM-TYPE-OBSERVED | なし |
| Possibly Rejects | GH-OIDC-CLAIM-PRESENCE | `exp` がない JWT を拒否する。有効期限のない JWT を資格情報として受理しないため |
| Possibly Rejects | GH-OIDC-CLAIM-TYPE | JSON の整数でない `exp` を拒否する。時刻の値を、Claim を読み取る処理によって異なる値に読めないようにするため（FR-AUTHN-006） |

**GATES の受理条件**：必須。JSON 数値の整数（小数点・指数表記も拒否）、範囲 `0..253402300799` のみ。時刻の判定は [GOJ-TIME-EXP](#goj-time-exp-有効期限の判定) を正本とします。観測された5分寿命（GH-OIDC-TIME-OBSERVED）を GATES 固有の寿命上限にはしません。

### GOJ-IAT `iat`

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7519-IAT-NUMERICDATE | なし |
| Narrows | RFC7519-IAT-OPTIONAL | `iat` がない JWT を拒否する。GOJ-TIME-IAT の発行時刻の判定に使うため |
| Narrows | RFC7519-NUMERICDATE-FORMAT | 整数でない値と、範囲 `0..253402300799` の外の値を拒否する。時刻の値を、Claim を読み取る処理によって異なる値に読めないようにするため（FR-AUTHN-006） |
| Accepts | GH-OIDC-CLAIM-TYPE-EXAMPLE | なし |
| Accepts | GH-OIDC-CLAIM-TYPE-OBSERVED | なし |
| Possibly Rejects | GH-OIDC-CLAIM-PRESENCE | `iat` がない JWT を拒否する。GOJ-TIME-IAT の発行時刻の判定に使うため |
| Possibly Rejects | GH-OIDC-CLAIM-TYPE | JSON の整数でない `iat` を拒否する。時刻の値を、Claim を読み取る処理によって異なる値に読めないようにするため（FR-AUTHN-006） |

**GATES の受理条件**：必須。[GOJ-EXP](#goj-exp-exp) と同じ JSON 整数・範囲を要求し、[GOJ-TIME-IAT](#goj-time-iat-発行時刻の判定) の未来時刻の条件を適用します。

### GOJ-ISS `iss`

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7519-ISS-FORMAT | なし |
| Satisfies | RFC7519-STRING-COMPARE | なし |
| Satisfies | RFC7519-STRINGORURI-COMPARE | なし |
| Narrows | RFC7519-ISS-OPTIONAL | `iss` がない JWT を拒否する。検証鍵を固定した Issuer の鍵に結び付けるため（RFC8725-ISS-KEY-VALIDATE） |
| Accepts | GH-OIDC-ISS | なし |
| Possibly Rejects | GH-OIDC-CLAIM-PRESENCE | `iss` がない JWT を拒否する。検証鍵を固定した Issuer の鍵に結び付けるため |

**GATES の受理条件**：必須。固定文字列との完全一致。Issuer の自由な切替えを許可しません。
JWT の `iss` の値は照合だけに使い、Discovery / JWKS の取得先には使いません。
`iss` に別の Issuer を書いた JWT を、その Issuer が公開する鍵で検証させないためです。

### GOJ-JTI `jti`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-REGISTERED-CLAIMS | `jti` の値と有無を判定に使わない |

**GATES の設計**：未使用。`jti` の存在、値、過去に使用したことの有無を認証・認可に用いません。JWT の有効期間内の再利用の検出は、[Scope §3](../spec/scope.md#3-deferred-capabilities) で現在のスコープに含めていない機能です。

### GOJ-NBF `nbf`

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7519-NBF-NUMERICDATE | なし |
| Narrows | RFC7519-NUMERICDATE-FORMAT | 整数でない値と、範囲 `0..253402300799` の外の値を拒否する。時刻の値を、Claim を読み取る処理によって異なる値に読めないようにするため（FR-AUTHN-006） |
| Accepts | GH-OIDC-CLAIM-TYPE-EXAMPLE | なし |
| Accepts | GH-OIDC-CLAIM-TYPE-OBSERVED | なし |
| Possibly Rejects | GH-OIDC-CLAIM-TYPE | JSON の整数でない `nbf` を拒否する。時刻の値を、Claim を読み取る処理によって異なる値に読めないようにするため（FR-AUTHN-006） |

**GATES の受理条件**：任意・検証。欠落を許し、存在時は `exp` / `iat` と同じ JSON 整数・範囲および [GOJ-TIME-NBF](#goj-time-nbf-利用開始時刻の判定) の時刻条件を適用します。型違いを「欠落」として扱いません。

### GOJ-REF `ref`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-REF | なし |

**GATES の設計**：任意・監査。存在し、JSON 文字列であればそのままワークフロー起動属性に保持します。欠落や文字列でない値は「記録なし」にし、認証・認可を拒否しません。ref 名をブランチポリシーやワークフローの信頼性の根拠として扱いません。

### GOJ-SHA `sha`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIM-MEANING | 意味が定まっていなくても、値を記録するだけで判定に使わない |

**GATES の設計**：任意・監査。JSON 文字列ならワークフロー起動属性として記録し、欠落・型違いは拒否しません。40文字の16進数字であることを必須にせず、`workflow_sha` と同じであることも要求しません。

### GOJ-REPOSITORY `repository`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIMS-TOKEN-TABLE | なし |

**GATES の設計**：未使用。`repository_id` の代用として Caller を確定しません。名前変更・移管・再利用があり得る名前に基づく認可をしないためです。存在・値を検証する独立した要件は設けません。

### GOJ-REPOSITORY-ID `repository_id`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-REPOSITORY-ID | なし |
| Possibly Rejects | GH-OIDC-CLAIM-PRESENCE | `repository_id` がない JWT を拒否する。`sub` の Repository ID と照合して Caller を確定するため |

**GATES の受理条件**：必須。[GOJ-ID-FORMAT](#goj-id-format-id-の表記) の形式を満たす正の整数の **JSON 文字列**。`sub` 内の Repository ID と一致すること。Caller の識別・監査に用いますが、現在のオーナー単位の発行認可には使いません（[Requirements](../spec/requirements.md) の FR-AUTHN-002）。

### GOJ-REPOSITORY-OWNER `repository_owner`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIMS-TOKEN-TABLE | なし |

**GATES の設計**：未使用。認証・認可で使用する Owner ID を名前で代用しません。オーナーの名前変更で失敗する箇所（要求対象の完全名のオーナー部分と許可オーナー名の照合、オーナー名を含む IAM の `sub` 条件）は、この Claim を使わず、それぞれ別の設計の責務です（[Domain Model §4.4](domain.md#44-events-outside-gates-that-change-the-outcome)）。

### GOJ-REPOSITORY-OWNER-ID `repository_owner_id`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-REPOSITORY-OWNER-ID | なし |
| Accepts | GH-OIDC-REPOSITORY-OWNER-ID-OBSERVED | なし |
| Possibly Rejects | GH-OIDC-CLAIM-PRESENCE | `repository_owner_id` がない JWT を拒否する。Owner ID を名前で代用しないため |
| Possibly Rejects | GH-OIDC-REPOSITORY-OWNER-ID-USER | User が所有するリポジトリで `repository_owner_id` がない JWT を拒否する。Owner ID を名前で代用すると、名前の再利用で別の主体を許すため |

**GATES の受理条件**：必須。[GOJ-ID-FORMAT](#goj-id-format-id-の表記) の正の整数を示す JSON 文字列。`sub` 内の Owner ID と一致すること。Caller の Owner ID として認証し、固定した許可オーナーの Owner ID との照合に使用します（認可の条件は [Requirements](../spec/requirements.md) の FR-AUTHZ-001）。User 所有で取得できない場合は自動的に名前へフォールバックせず拒否します。

### GOJ-ENTERPRISE `enterprise`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIM-MEANING | 意味が定まっていなくても、判定に使わない |

**GATES の設計**：未使用。存在・値を検証せず、将来利用するときに仕様を確認します。

### GOJ-ENTERPRISE-ID `enterprise_id`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIM-MEANING | 意味が定まっていなくても、判定に使わない |

**GATES の設計**：未使用。Claim 名が `_id` で終わるというだけで整数文字列と決めつけたり、固定オーナー ID に代用したりしません。

### GOJ-RUN-ID `run_id`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-RUN-ID | なし |
| Accepts | GH-OIDC-RUN-ID-RERUN | 再実行で値が変わっても変わらなくても、形式の判定は変わらない |
| Possibly Rejects | GH-OIDC-CLAIM-PRESENCE | `run_id` がない JWT を拒否する。監査記録からワークフロー実行を特定できるようにするため |

**GATES の受理条件**：必須。[GOJ-ID-FORMAT](#goj-id-format-id-の表記) の正の整数文字列。監査上のワークフロー実行 ID に用い、発行認可には使用しません。`run_attempt` と組にして再実行の試行を区別します。

### GOJ-RUN-NUMBER `run_number`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIMS-TOKEN-TABLE | なし |

**GATES の設計**：未使用。欠落・値を検証しません。

### GOJ-RUN-ATTEMPT `run_attempt`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-RUN-ATTEMPT | 1 以上の値に限る。初回を `"0"` とする読み方の発行は、GH-OIDC-RUN-ATTEMPT-VALUE の Possibly Rejects の行に当たる |
| Accepts | GH-OIDC-RUN-ATTEMPT-OBSERVED | なし |
| Possibly Rejects | GH-OIDC-CLAIM-PRESENCE | `run_attempt` がない JWT を拒否する。監査記録から再実行の試行を区別できるようにするため |
| Possibly Rejects | GH-OIDC-RUN-ATTEMPT-VALUE | `"0"` の `run_attempt` を拒否する。初回を `"0"` とする発行があれば正当な JWT を拒否するが、GATES が対応する試行番号の形式を正の整数に限ることを優先する |

**GATES の受理条件**：必須。[GOJ-ID-FORMAT](#goj-id-format-id-の表記) の正の整数文字列。`run_id` とともに監査に保持し、認可には使いません。`"0"` の拒否は、GitHub の保証や認可上の必要条件ではなく、GATES が対応する試行番号の形式を限定する判断です。

### GOJ-ACTOR `actor`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-ACTOR | なし |

**GATES の設計**：任意・監査。JSON 文字列をワークフロー起動属性として保持します。欠落・型違いは拒否しません。Caller の所有者やリポジトリの所有者を `actor` の名前で代用しません。

### GOJ-ACTOR-ID `actor_id`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-ACTOR | なし |

**GATES の設計**：任意・監査。JSON 文字列ならそのまま記録します。必須 ID Claim と異なり正の整数への変換を必須にせず、欠落・型違いを拒否しません。認可には使いません。

### GOJ-WORKFLOW `workflow`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIMS-TOKEN-TABLE | なし |

**GATES の設計**：未使用。名前で発行を制限しません。

### GOJ-WORKFLOW-REF `workflow_ref`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-WORKFLOW-REF | なし |
| Accepts | GH-OIDC-WORKFLOW-REF-OBSERVED | なし |

**GATES の設計**：任意・監査。JSON 文字列を保存します。欠落・型違いは拒否せず、特定のワークフローファイルを要求する認可には使いません。`job_workflow_ref` と同一とは仮定しません。

### GOJ-WORKFLOW-SHA `workflow_sha`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-WORKFLOW-REF | なし |

**GATES の設計**：未使用。`sha`、`job_workflow_sha` との一致は検証しません。コミットの信頼性や認可の根拠に使いません。

### GOJ-HEAD-REF `head_ref`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIM-PRESENCE-OBSERVED | 空文字列でも拒否しない |
| Accepts | GH-OIDC-CLAIMS-TOKEN-TABLE | なし |

**GATES の設計**：未使用。空文字列を欠落と同じとみなす独自の必須条件は設けず、値で発行可否を制限しません。

### GOJ-BASE-REF `base_ref`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIM-PRESENCE-OBSERVED | 空文字列でも拒否しない |
| Accepts | GH-OIDC-CLAIMS-TOKEN-TABLE | なし |

**GATES の設計**：未使用。`head_ref` と同様に、値の有無で認証・認可を変えません。

### GOJ-EVENT-NAME `event_name`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-EVENT-NAME | なし |
| Accepts | GH-OIDC-EVENT-NAME-DEPENDABOT | なし |

**GATES の設計**：任意・監査。JSON 文字列を保持し、`pull_request_target` 等を後から調査できるようにします。イベント種別で認可を拒否するポリシーは現時点で設けません。イベント名を見ただけでジョブ内のコードが信頼できるとは解釈しません。

### GOJ-REF-TYPE `ref_type`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIMS-TOKEN-TABLE | なし |

**GATES の設計**：未使用。`ref` との値の整合性を確認せず、認証・認可にも使用しません。

### GOJ-REF-PROTECTED `ref_protected`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIM-MEANING | 意味が定まっていなくても、判定に使わない |
| Accepts | GH-OIDC-CLAIM-TYPE-OBSERVED | 真偽値でなく文字列の `"false"` でも拒否しない |

**GATES の設計**：未使用。Claim 名から保護ブランチ判定の正確な意味を推定して認可に使いません。将来使用する場合は、名前だけでなく GitHub 公式の保証と発行条件を確認します。

### GOJ-ENVIRONMENT `environment`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIM-PRESENCE-OBSERVED | Environment を使わないジョブの JWT に `environment` がなくても拒否しない |
| Accepts | GH-OIDC-ENVIRONMENT | なし |

**GATES の設計**：任意・監査。存在し、JSON 文字列なら記録します。`environment` Claim のないジョブを拒否せず、Environment の名前から承認や保護規則の適用結果を推定しません。`sub` の `environment:` コンテキストを認可するためには用いません。

### GOJ-ENVIRONMENT-NODE-ID `environment_node_id`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIM-MEANING | 意味が定まっていなくても、判定に使わない |
| Accepts | GH-OIDC-CLAIM-PRESENCE-OBSERVED | なくても拒否しない |

**GATES の設計**：未使用。Environment の認可に使用せず、欠落を拒否しません。

### GOJ-JOB-WORKFLOW-REF `job_workflow_ref`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-WORKFLOW-REF | なし |
| Accepts | GH-OIDC-WORKFLOW-REF-OBSERVED | なし |

**GATES の設計**：任意・監査。JSON 文字列なら保持し、欠落・型違いは拒否しません。再利用する場合としない場合の存在条件を、観測（GH-OIDC-WORKFLOW-REF-OBSERVED）から保証された仕様へ格上げしません。`workflow_ref` との一致も要求しません。

### GOJ-JOB-WORKFLOW-SHA `job_workflow_sha`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-WORKFLOW-REF | なし |
| Accepts | GH-OIDC-WORKFLOW-REF-OBSERVED | なし |

**GATES の設計**：未使用。必須にもせず `sha` と照合しません。ワークフローの出自を認可に使う機能は現在のスコープ外です。

### GOJ-REPOSITORY-VISIBILITY `repository_visibility`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIMS-TOKEN-TABLE | なし |

**GATES の設計**：未使用。private のみ許可する等の制約を導入せず、Claim の値や欠落を検証条件にしません。

### GOJ-RUNNER-ENVIRONMENT `runner_environment`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIMS-TOKEN-TABLE | なし |

**GATES の設計**：未使用。GitHub-hosted かどうかでトークン発行を制限しません。ランナーが信頼できることの証明としては扱いません。

### GOJ-ISSUER-SCOPE `issuer_scope`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIM-MEANING | 意味が定まっていなくても、判定に使わない |

**GATES の設計**：未使用。存在・値を検証しません。

### GOJ-CHECK-RUN-ID `check_run_id`

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIMS-TOKEN-TABLE | なし |

**GATES の設計**：未使用。`run_id` や `run_attempt` の代用にせず、認証・認可・監査へ取り込みません。

## Time

`now` を検証時の現在時刻（Unix 秒の整数）、`L=300` 秒とします。[GOJ-EXP](#goj-exp-exp)、[GOJ-IAT](#goj-iat-iat)、[GOJ-NBF](#goj-nbf-nbf) の値を、同一の `now` で判定します。
3つとも存在する場合でも、観測された `nbf=iat-300` や `exp=iat+300` の固定関係（GH-OIDC-TIME-OBSERVED）を要求しません。`exp > iat` のような追加の相互順序条件も、現行 GATES の受理規則としては設けません。これらを追加する場合は、外部仕様の要求か GATES の新しい制約かを明確にしたうえで別途判断します。

### GOJ-TIME-EXP 有効期限の判定

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7519-EXP-REJECT | なし |
| Narrows | RFC7519-EXP-LEEWAY | `exp` の判定に猶予を設けず、`exp` の時刻以後の JWT を拒否する。有効期限を過ぎた JWT を資格情報として受理しないため |
| Accepts | GH-OIDC-REGISTERED-CLAIMS | `exp` を JWT の有効期限として判定する |

`now < exp` を満たさない JWT を拒否します。等号は拒否し、猶予を設けません。

### GOJ-TIME-IAT 発行時刻の判定

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-REGISTERED-CLAIMS | `iat` を JWT の発行時刻として判定する。GitHub と Lambda の時計の差が `L` 以内なら受理する |

`iat <= now + L` を満たさない JWT を拒否します。発行時刻への5分の猶予は GATES 独自の判断です。

### GOJ-TIME-NBF 利用開始時刻の判定

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7519-NBF-LEEWAY | 猶予を 300 秒とする。GOJ-TIME-IAT と同じく、GitHub と Lambda の時計の差を吸収するため |
| Satisfies | RFC7519-NBF-REJECT | RFC7519-NBF-LEEWAY の猶予を含めて判定する |
| Accepts | GH-OIDC-REGISTERED-CLAIMS | `nbf` を、JWT をその時刻より前に使えないことを表す時刻として判定する |
| Accepts | GH-OIDC-TIME-EXAMPLE | `nbf` が `iat` より前なので、GOJ-TIME-IAT を満たす時刻には `nbf` の判定も満たす |
| Accepts | GH-OIDC-TIME-OBSERVED | `nbf` が `iat` より前なので、GOJ-TIME-IAT を満たす時刻には `nbf` の判定も満たす |

`nbf` が存在するとき、`nbf <= now + L` を満たさない JWT を拒否します。

## Cross-Claim Invariants

各 Claim の値を独立に受理しただけでは Caller を確定しません。次の条件が同時に成立したときに限り、[Domain Model](domain.md) の INV-009 が定める Caller を生成できます。

### GOJ-ID-FORMAT ID の表記

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIM-TYPE-EXAMPLE | `repository_id`、`repository_owner_id`、`run_attempt` の数字だけの文字列を受理する。例の `run_id` は置き換えの値で、形式を示さない |
| Accepts | GH-OIDC-CLAIM-TYPE-OBSERVED | `run_id` を数字だけの文字列に限る根拠は、この Observed の条項だけである |
| Possibly Rejects | GH-OIDC-CLAIM-ID-FORMAT | 数字以外の文字を含む ID、先頭に `0` を持つ ID、64 bit 符号付き整数の正の範囲を超える ID を拒否する。ID を正の 64 bit 符号付き整数の 10 進表記に限って、同じ ID を一つの文字列だけで表し、Claim を読み取る処理によって異なる値に読めないようにするため |
| Possibly Rejects | GH-OIDC-CLAIM-TYPE | JSON の文字列でない ID の Claim を拒否する。GitHub が型を変えたときに、暗黙に受理を広げないため |

`repository_owner_id`、`repository_id`、`run_id`、`run_attempt` と `sub` のオーナー ID・リポジトリ ID はすべて、次の条件を満たす必要があります。

- JSON の **文字列** である。JSON 数値、`null`、boolean は拒否する。
- ASCII 数字 `[0-9]` だけで構成し、先頭に `0` を持たず、`"0"` を拒否する。`+` / `-`、空白、小数点、指数表記、非 ASCII 数字は拒否する。
- 64 bit 符号付き整数の正の範囲 `1..9223372036854775807` に収まる。

この表記は GitHub が保証した型・範囲ではなく、公式の JWT 例と観測に基づいて GATES が固定した形式です。形式の変更は明示的な設計変更として扱います。

### GOJ-ID-MATCH Subject と識別子の一致

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7519-STRING-COMPARE | なし |
| Accepts | GH-OIDC-SUB-IMMUTABLE-FORMAT | `sub` の Owner ID と Repository ID は、`repository_owner_id`（GH-OIDC-REPOSITORY-OWNER-ID）と `repository_id`（GH-OIDC-REPOSITORY-ID）と同じオーナーとリポジトリの ID である |

`sub` の構文・値の形式は [GOJ-SUB-FORMAT](#goj-sub-format-sub-の形式)、共通の ID 表記は [GOJ-ID-FORMAT](#goj-id-format-id-の表記) を正本とします。Caller を確定するには、さらに次の **Claim 間の一致** を要求します。

| 条件 | 不一致時の扱い |
|---|---|
| `sub` の Owner ID = `repository_owner_id` | 拒否（認証失敗） |
| `sub` の Repository ID = `repository_id` | 拒否（認証失敗） |

Caller の確定後、固定した Allowed Owner ID との一致は認可で確認します。

### GOJ-CLAIMS-JSON Claims Set の JSON

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7515-JSON-STRICT | なし |
| Satisfies | RFC7515-JSON-TRAILING-INVALID | なし |
| Satisfies | RFC7519-CLAIM-NAME-DUPLICATE | 重複した Claim 名を持つ JWT を拒否する側を選ぶ |
| Satisfies | RFC7519-CLAIM-NAME-UNIQUE | なし |
| Satisfies | RFC7519-VALIDATE-REJECT | RFC 7519 §7.2 の手順のうち、Payload が UTF-8 で表した完全に有効な JSON オブジェクトであることの確認に当たる |
| Satisfies | RFC8725-UTF8-APPLICATION | なし |
| Satisfies | RFC8725-UTF8-IMPLEMENTATION | なし |

Claims Set は UTF-8 の JSON オブジェクトとし、不正な UTF-8 と、エスケープ解除後に同一となる Claim 名の重複を拒否します。未使用 Claim の重複も拒否します。`iss` と `\u0069ss` の同時存在は名前の重複として拒否します。

### GOJ-CLAIM-NAME-MATCH Claim 名の厳密一致

| Relation | Clause | Note |
|---|---|---|
| Satisfies | OIDCC-STRING-CODE-POINT | なし |
| Satisfies | OIDCC-STRING-NO-NORMALIZATION | なし |
| Satisfies | OIDCC-STRING-UNESCAPE | なし |
| Satisfies | RFC7519-STRING-COMPARE | なし |

- Claim 名はエスケープ解除後の **厳密一致** で識別する。識別では、大文字小文字の同一視、Unicode 正規化、Unicode の Case Folding は行わない。`iss`、`ISS`、`iſſ` はそれぞれ別の名前として扱う。
- 検証ライブラリと Claim 抽出処理で、同じ署名済み Claims Set の **同名・同値** を使用する。

### GOJ-CLAIM-NAME-ALIAS Claim 名の別名の拒否

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7519-CLAIM-UNKNOWN-IGNORE | 5 つの名前の別名の Claim は、FR-AUTHN-006 が拒否を求める Claim で、条項の Condition（アプリケーションが特定の処理を要求していない）に当たらない。JSON デコーダーが別名を同じ Claim として読む危険を防ぐため、JWT ごと拒否する |
| Accepts | GH-OIDC-DISCOVERY-CLAIMS | `claims_supported` の 36 個の名前は、どれも 5 つの名前の別名ではない |

- `iss`、`aud`、`exp`、`iat`、`nbf` の **別名** の Claim があれば、正しい `iss` などが同時にあっても拒否する。JSON デコーダーが別名を同じ Claim として読む危険を防ぐための、[Requirements](../spec/requirements.md) の FR-AUTHN-006 に従う GATES の追加制約である。
- 別名は、5 つの名前のどれかと Unicode の単純な Case Folding（[CaseFolding.txt](https://www.unicode.org/Public/UCD/latest/ucd/CaseFolding.txt) の状態 C と S）で同じ文字列に畳み込まれ、その名前自体とは異なる名前とする。5 つの名前の文字では、各 ASCII 英字の大文字と小文字、および `s`、`S`、`ſ`（U+017F）がそれぞれ同じ文字に畳み込まれる。別名は `iss` の 17 個（`ISS`、`iſſ`、`iſs` など）と、ほかの 4 つの名前の各 7 個（`AUD`、`eXp` など）の計 45 個である。
- 完全な Case Folding（状態 F）とトルコ語用の対応（状態 T）は使わない。そのため、`iß` と `İss` は別名ではなく、未知の Claim として扱う。`ıss` も、`ı`（U+0131）がどの Case Folding でも `i` に畳み込まれないため、未知の Claim として扱う。範囲を単純な Case Folding に揃えるのは、検証ライブラリの JSON デコーダーが Claim 名を同一視する範囲がこれと一致し、`iß` のような名前は `iss` として読まれないためである。

### GOJ-TRIGGER-ATTRIBUTES ワークフロー起動属性への転送

| Relation | Clause | Note |
|---|---|---|
| Accepts | GH-OIDC-CLAIM-PRESENCE | 任意・監査の Claim がなくても拒否しない |
| Accepts | GH-OIDC-CLAIM-TYPE | 任意・監査の Claim が JSON の文字列でなくても拒否しない |

任意・監査用 Claim は JSON 文字列として読み取れた場合だけワークフロー起動属性に転送します。型違い・欠落は拒否せず、構造化ログではエスケープします。未知 Claim をワークフロー起動属性に取り込みません。

### GOJ-VERIFIED-CLAIMS-SET 署名検証済みの Claims Set の記録

| Relation | Clause | Note |
|---|---|---|
| None | なし | Claims Set を記録するだけで、受理と拒否を決めない |

署名検証を通った Claims Set は、Claim の分類によらずすべての Claim を、ワークフロー起動属性とは別に調査用として監査記録へ残します（[Requirements](../spec/requirements.md) の FR-OPS-003）。残した Claim は、認証、認可、ワークフロー起動属性のどれにも使いません。各 Claim の分類はこの記録によって変わりません。

### GOJ-DYNAMIC-CLAIM Discovery に掲載されない動的 Claim

| Relation | Clause | Note |
|---|---|---|
| Satisfies | RFC7519-CLAIM-UNKNOWN-IGNORE | `repo_property_*` と、将来 GitHub が加える未知の Claim を無視する |
| Accepts | GH-OIDC-REPO-PROPERTY-PRESENCE | なし |
| Accepts | GH-OIDC-REPO-PROPERTY-VALUE | なし |

**GATES の設計**：`repo_property_*` は現在すべて未使用です。存在・欠落・値・型を認証、認可、ワークフロー起動属性の根拠とせず、存在するだけで JWT を拒否しません。署名検証を通った Claims Set の一部として、調査用に監査記録へ残すだけです（[GOJ-VERIFIED-CLAIMS-SET](#goj-verified-claims-set-署名検証済みの-claims-set-の記録)）。例えば `repo_property_business_unit` や `repo_property_pci_compliant` を追加しても、必須 Claim の検証結果は変えません。将来 GitHub が追加する未知 Claim も同様です。ただし Claims Set 全体の JSON の妥当性、名前の重複、署名検証は引き続き必要です。将来この属性を認可に使うときは、意味・型・信頼性・変更権限を別途評価し、受理条件を改訂します。

## Standard Clauses Not Applied

GATES の役割に当たる強い規範の条項のうち、Claims Set を扱い、GATES に適用しない条項です。
直列化、JOSE Header、Discovery、JWK を扱う条項は [GATES OIDC JWT Signature](gates-oidc-jwt-signature.md) に挙げます。

| Clause | Reason |
|---|---|
| OIDCC-CLAIM-UNKNOWN-IGNORE | ID Token に限る（[ID Token として扱わないこと](#id-token-として扱わないこと)）。GATES は未知の Claim を GOJ-DYNAMIC-CLAIM で無視する |
| OIDCC-CODE-AUD-CLIENT | Authorization Code Flow の Token Response に含まれる ID Token に限る。GATES は `client_id` を持たない |
| OIDCC-CODE-AUD-UNTRUSTED | Authorization Code Flow の Token Response に含まれる ID Token に限る。GATES は GOJ-AUD で要素が複数の `aud` を拒否する |
| OIDCC-CODE-EXP | Authorization Code Flow の Token Response に含まれる ID Token に限る。GATES は GOJ-TIME-EXP で `exp` を判定する |
| OIDCC-CODE-ISS-MATCH | Authorization Code Flow の Token Response に含まれる ID Token に限る。GATES は GOJ-ISS で `iss` を固定の Issuer と照合する |
| OIDCC-CODE-NONCE | 認証要求で `nonce` を送った場合に限る。GATES は認証要求を送らない |
| OIDCC-CODE-VALIDATE | Authorization Code Flow の Token Response に含まれる ID Token に限る |
| OIDCC-EXP-REJECT | ID Token に限る。GATES は GOJ-TIME-EXP で RFC7519-EXP-REJECT を満たす |
| OIDCC-HYBRID-AUTHORIZATION-ENDPOINT | Hybrid Flow の ID Token に限る |
| OIDCC-HYBRID-TOKEN-ENDPOINT | Hybrid Flow の ID Token に限る |
| OIDCC-IMPLICIT-NONCE | Implicit Flow の ID Token に限る。`nonce` の照合は認証要求を前提とする |
| OIDCC-IMPLICIT-VALIDATE | Implicit Flow の ID Token に限る |
| OIDCC-NONCE-VERIFY | ID Token に限る。`nonce` の照合は認証要求を前提とし、GATES は認証要求を送らない |
| OIDCC-SPACE-DELIMITER | GATES は空白区切りの文字列（`scope` など）を扱わない |
| RFC7519-JTI-ISSUERS-UNIQUE | 複数の Issuer を使うアプリケーションに限る。GATES の Issuer は一つに固定する（GOJ-ISS） |

## Traceability

| GATES の受理条件 | 要求・設計上の根拠 |
|---|---|
| 必須 Claim、Issuer、Audience、時間、ID の整合 | [Requirements](../spec/requirements.md) の FR-AUTHN-002、FR-AUTHN-003、[Domain Model](domain.md) の INV-009 |
| Claim 名の別名の拒否と、時刻の値の表記 | [Requirements](../spec/requirements.md) の FR-AUTHN-006 |
| 不変形式 `sub` | [Requirements](../spec/requirements.md) の FR-AUTHN-005、FR-OPS-007 |
| Caller の Owner ID による認可と Repository ID の監査利用 | [Requirements](../spec/requirements.md) の FR-AUTHN-002、FR-AUTHZ-001 |
| 起動属性のベストエフォート監査と、署名検証を通った Claims Set の調査用の記録 | [Requirements](../spec/requirements.md) の FR-OPS-003 |

## Verification Cases

| 対象 | 受理例・拒否例の焦点 | 期待結果 |
|---|---|---|
| `sub` | 不変形式、旧形式、`@` の過不足、`:` 欠落、カスタマイズ形 | 構文と ID 一致を満たす場合だけ受理 |
| `sub` の長さ・文字種 | ASCII 255文字、256文字、非 ASCII、長い ref / Environment | ASCII 255文字以内のみ受理 |
| `aud` | GATES 用1件、文字列1件と配列1件、配列に空要素や別 Audience を含む場合、AWS 用 Audience | GATES 用の1要素だけを受理 |
| NumericDate | 数値整数、数字文字列、小数、指数、範囲外、`exp=now`、`exp=now+1` | 整数・範囲・各時刻条件を満たすものだけ受理 |
| 不変 ID | `"1"`、`"0"`、`"01"`、JSON 数値、符号、64 bit 範囲外、`sub` との不一致 | 正の整数文字列および一致だけ受理 |
| `run_attempt` | `"1"`、`"0"`、欠落 | `"0"` と欠落は拒否 |
| 任意・監査 | `environment` 欠落、監査 Claim の型違い、制御文字入りの文字列 | 認証は継続し、ログでは安全に記録 |
| Claim 名の比較 | `iss`、`ISS`、`iſſ`、`iſS` のような混在、`iß`、`İss`、`ıss`、`\u0069ss`、同時存在 | 厳密一致。GOJ-CLAIM-NAME-ALIAS の 45 個の別名は拒否し、`iß`、`İss`、`ıss` は別名として扱わない。エスケープ解除後の重複は拒否 |
| 動的 Claim | `repo_property_*` の文字列・複数選択・真偽値文字列、未知 Claim | 追加 Claim があっても、必須 Claim が成立すれば認証を妨げない |
| Claims Set の記録 | 署名検証の前の拒否、署名検証の後の Claim の拒否、認証の成功、制御文字入りの値、未知 Claim | 署名検証を通った場合だけ、未知 Claim を含むすべての Claim を残し、制御文字はエスケープする |
| JSON の曖昧さ | 重複 Claim 名、Unicode エスケープによる重複、不正 UTF-8 | 署名が正しくても拒否 |

ケースの詳細なテスト実装とモック方式はコンポーネント設計が担当します。この表は受理条件のテスト可能性の索引であり、コードのテスト一覧の正本を二重に設けるものではありません。

## References

- [RFC 7515 JSON Web Signature](../external/standards/rfc7515.md) — `RFC7515-` の条項
- [RFC 7519 JSON Web Token](../external/standards/rfc7519.md) — `RFC7519-` の条項
- [RFC 8725 JWT Best Current Practices](../external/standards/rfc8725.md) — `RFC8725-` の条項
- [OpenID Connect Core 1.0](../external/standards/oidc-core.md) — `OIDCC-` の条項
- [GitHub Actions OIDC Constraints](../external/github/oidc.md) — `GH-OIDC-` の条項
- [GitHub Actions Integration Constraints](../external/github/actions.md) — OIDC JWT の要求
- [GitHub OIDC Discovery](../research/github/oidc-discovery.md) — `claims_supported`
- [GitHub OIDC JWT](../research/github/oidc-jwt.md) — 6件の JWT 観測
- [GitHub OIDC Custom Audience](../research/github/custom-audience.md) — Audience を指定した JWT の観測と、GitHub が発行を拒否した Audience
- [RFC 7519 — JSON Web Token](https://www.rfc-editor.org/rfc/rfc7519.html)
- [OpenID Connect Core 1.0](https://openid.net/specs/openid-connect-core-1_0.html)
- [Unicode Character Database — CaseFolding.txt](https://www.unicode.org/Public/UCD/latest/ucd/CaseFolding.txt)（Claim 名の別名の範囲）
- [GitHub — OpenID Connect reference](https://docs.github.com/en/actions/reference/security/oidc)
