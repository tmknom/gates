# GitHub Actions OIDC Constraints

この文書は、GitHub.com の GitHub Actions OIDC の仕様を、GitHub の公式資料と、GitHub から取得した値の調査記録を根拠に、ID の付いた条項に分けて整理したものです。
GitHub の公式資料そのものではなく、GitHub の仕様が変更された場合は該当する公式資料を確認します。

- **対象**：GitHub.com の GitHub Actions OIDC の Issuer、Discovery、JWKS、JOSE Header、Claim、時刻、Audience、`sub` とそのカスタマイズ
- **対象外**：GitHub Enterprise Server。OIDC JWT を取得するワークフローの権限と取得の手段は [GitHub Actions Integration Constraints](actions.md) が扱う
- **取得した値の調査記録**：[GitHub OIDC Discovery](../../research/github/oidc-discovery.md)、[GitHub OIDC JWKS](../../research/github/oidc-jwks.md)、[GitHub OIDC JWT](../../research/github/oidc-jwt.md)、[GitHub OIDC Custom Audience](../../research/github/custom-audience.md)。取得日、取得方法、記録した応答はこれらの記録にある

各条項の Evidence は、Statement がどのように知られたかを表します。
Documented は公式資料の記述、Published は GitHub が公開するエンドポイントから取得した値、Observed は実環境で観測した値、Inferred はそれらの条項から導いた解釈、Unconfirmed は公式資料で確定できない事項です。
Published と Observed は、記録した時点と条件での値で、将来やほかの条件でも同じ値であることを GitHub が保証したものではありません。
Unconfirmed の条項には、確定に要る確認を Confirmation に書きます。

Observed の条項が根拠にする JWT は、どれも個人アカウントの private リポジトリで発行されたものです。
[GitHub OIDC JWT](../../research/github/oidc-jwt.md) の 6 件は Audience を指定せずに取得し、起動イベント、再利用ワークフロー、Environment の条件を変えて記録しています。
[GitHub OIDC Custom Audience](../../research/github/custom-audience.md) の 2 件は、workflow_dispatch で起動したワークフローから、URL でない文字列を Audience に指定して取得し、値ではなく型だけを記録しています。

## 1. Issuer and Discovery

### GH-OIDC-ISS Issuer

- **Evidence**：Documented
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：OIDC JWT の `iss` は `https://token.actions.githubusercontent.com` である

### GH-OIDC-DISCOVERY-LOCATION Discovery の公開先

- **Evidence**：Documented
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：GitHub の OIDC Provider がサポートする Claim は、`https://token.actions.githubusercontent.com/.well-known/openid-configuration` の `claims_supported` に挙がる

### GH-OIDC-DISCOVERY-ISSUER Discovery の `issuer`

- **Evidence**：Published
- **Source**：[GitHub OIDC Discovery](../../research/github/oidc-discovery.md)
- **Statement**：Discovery の応答の `issuer` は `https://token.actions.githubusercontent.com` である

### GH-OIDC-DISCOVERY-JWKS-URI Discovery の `jwks_uri`

- **Evidence**：Published
- **Source**：[GitHub OIDC Discovery](../../research/github/oidc-discovery.md)
- **Statement**：Discovery の応答の `jwks_uri` は `https://token.actions.githubusercontent.com/.well-known/jwks` である

### GH-OIDC-DISCOVERY-ALG Discovery の署名アルゴリズム

- **Evidence**：Published
- **Source**：[GitHub OIDC Discovery](../../research/github/oidc-discovery.md)
- **Statement**：Discovery の応答の `id_token_signing_alg_values_supported` は `RS256` だけからなる

### GH-OIDC-DISCOVERY-CLAIMS Discovery の `claims_supported`

- **Evidence**：Published
- **Source**：[GitHub OIDC Discovery](../../research/github/oidc-discovery.md)
- **Statement**：Discovery の応答の `claims_supported` は、`sub`、`aud`、`exp`、`iat`、`iss`、`jti`、`nbf`、`ref`、`sha`、`repository`、`repository_id`、`repository_owner`、`repository_owner_id`、`enterprise`、`enterprise_id`、`run_id`、`run_number`、`run_attempt`、`actor`、`actor_id`、`workflow`、`workflow_ref`、`workflow_sha`、`head_ref`、`base_ref`、`event_name`、`ref_type`、`ref_protected`、`environment`、`environment_node_id`、`job_workflow_ref`、`job_workflow_sha`、`repository_visibility`、`runner_environment`、`issuer_scope`、`check_run_id` の 36 個の Claim 名からなる

### GH-OIDC-DISCOVERY-MEMBERS Discovery の Metadata の構成

- **Evidence**：Published
- **Source**：[GitHub OIDC Discovery](../../research/github/oidc-discovery.md)
- **Statement**：Discovery の応答の Metadata は、`issuer`、`jwks_uri`、`subject_types_supported`（`public`、`pairwise`）、`response_types_supported`（`id_token`）、`claims_supported`、`id_token_signing_alg_values_supported`、`scopes_supported`（`openid`）の 7 個だけで、`authorization_endpoint` と `token_endpoint` はない

## 2. JWKS

### GH-OIDC-JWKS-RSA-MEMBERS 鍵のメンバー

- **Evidence**：Published
- **Source**：[GitHub OIDC JWKS](../../research/github/oidc-jwks.md)
- **Statement**：JWKS のメンバーは `keys` だけで、鍵は 4 個である。すべての鍵に `kty`（`RSA`）、`alg`（`RS256`）、`use`（`sig`）、`kid`、`n`、`e`（`AQAB`）があり、ほかのメンバーは一部の鍵の `x5c` と `x5t`（GH-OIDC-JWKS-X5C）だけで、`key_ops` はない。JWKS と鍵のどのオブジェクトにも、メンバー名の重複はない

### GH-OIDC-JWKS-MODULUS 鍵の法

- **Evidence**：Published
- **Source**：[GitHub OIDC JWKS](../../research/github/oidc-jwks.md)
- **Statement**：JWKS の 4 個の鍵の `n` は、どれも最小のオクテット数で表した Base64urlUInt で、そこから求めた法は 2048 bit の奇数である

### GH-OIDC-JWKS-X5C 鍵の証明書

- **Evidence**：Published
- **Source**：[GitHub OIDC JWKS](../../research/github/oidc-jwks.md)
- **Statement**：JWKS の 4 個の鍵のうち 3 個にだけ `x5c` と `x5t` があり、それらの `x5c` はどれも証明書を 1 個だけ含む

### GH-OIDC-JWKS-X5C-KEY 鍵の証明書の公開鍵

- **Evidence**：Published
- **Source**：[GitHub OIDC JWKS](../../research/github/oidc-jwks.md)
- **Statement**：JWKS の `x5c` を持つ 3 個の鍵では、`x5c` の証明書の公開鍵は、どれも RSA 公開鍵で、その鍵の `n` と `e` に一致する。どの鍵の `x5t` も、その証明書の SHA-1 の指紋に一致する。2 個の証明書は Key Usage に `digitalSignature` と `keyEncipherment` を持ち、1 個の証明書は Key Usage を持たない

### GH-OIDC-JWKS-KID-FORMAT `kid` の形式

- **Evidence**：Published
- **Source**：[GitHub OIDC JWKS](../../research/github/oidc-jwks.md)
- **Statement**：JWKS の 4 個の鍵の `kid` の形式は一様でなく、UUID 形式が 2 個、40 桁の 16 進数字が 2 個である

### GH-OIDC-JWKS-KID-DISTINCT `kid` の一意性

- **Evidence**：Published
- **Source**：[GitHub OIDC JWKS](../../research/github/oidc-jwks.md)
- **Statement**：JWKS の 4 個の鍵の `kid` は、どれも異なる

### GH-OIDC-JWKS-X5C-CHAIN `x5c` に含める証明書

- **Evidence**：Unconfirmed
- **Source**：[GitHub OIDC JWKS](../../research/github/oidc-jwks.md)
- **Statement**：GitHub は、JWKS の鍵の `x5c` に含める証明書の数と、2 個目以降の証明書を含める場合の証明書どうしの関係を定めていない
- **Confirmation**：公式資料の JWKS の `x5c` の記述を確かめるか、2 個以上の証明書を含む `x5c` を公開した JWKS を取得して、証明書どうしの署名の関係を記録する

公式資料は JWKS の内容を説明していません。
記録した JWKS の `x5c` は、どれも証明書が 1 個だけでした（GH-OIDC-JWKS-X5C）。

## 3. JOSE Header

### GH-OIDC-HEADER-DOCUMENTED 公式資料が説明する Header Parameter

- **Evidence**：Documented
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：公式資料は、JOSE Header として `alg`（OIDC Provider が使うアルゴリズム）、`kid`（トークンの鍵の識別子）、`typ`（トークンの種類で、JWT である）を説明する

### GH-OIDC-HEADER-EXAMPLE 公式の JWT の例の Header

- **Evidence**：Documented
- **Source**：[OpenID Connect — Understanding the OIDC token](https://docs.github.com/en/actions/concepts/security/openid-connect#understanding-the-oidc-token)
- **Statement**：公式の JWT の例の Header は、`typ`（`JWT`）、`alg`（`RS256`）、`x5t`、`kid` からなる

### GH-OIDC-HEADER-OBSERVED 記録した JWT の Header

- **Evidence**：Observed
- **Source**：[GitHub OIDC JWT](../../research/github/oidc-jwt.md)、[GitHub OIDC Custom Audience](../../research/github/custom-audience.md)
- **Statement**：個人アカウントの private リポジトリで発行された JWT の Header は、Audience を指定しない 6 件と、URL でない文字列を Audience に指定した 2 件のどれも、`alg`（`RS256`）、`kid`、`typ`、`x5t` だけからなる。Audience を指定しない 6 件では、`typ` は `JWT` で、4 つの値は 6 件とも同じである

### GH-OIDC-HEADER-KID-JWKS 記録した Header の鍵の識別子

- **Evidence**：Observed
- **Source**：[GitHub OIDC JWT](../../research/github/oidc-jwt.md)、[GitHub OIDC JWKS](../../research/github/oidc-jwks.md)
- **Statement**：個人アカウントの private リポジトリで Audience を指定せずに発行された 6 件の JWT の Header の `kid` と `x5t` は、JWKS の同じ 1 個の鍵の `kid` と `x5t` に一致する

### GH-OIDC-HEADER-PARAMETERS ほかの Header Parameter の有無

- **Evidence**：Unconfirmed
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：GitHub は、JOSE Header に `alg`、`kid`、`typ`、`x5t` 以外の Header Parameter（`crit`、`cty`、`b64`、`jku`、`jwk` など）を含めるかどうかを定めていない
- **Confirmation**：公式資料に、JOSE Header の Header Parameter を網羅した記述があるかを確かめる

公式資料が説明する Header Parameter（GH-OIDC-HEADER-DOCUMENTED）は `alg`、`kid`、`typ` だけですが、公式の例と記録した JWT には `x5t` もあります。
公式資料は、説明した Header Parameter がすべてであるとは書いていません。

## 4. Claims

### GH-OIDC-CLAIMS-TOKEN-TABLE 公式資料が説明する Claim

- **Evidence**：Documented
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：公式資料の OIDC token claims の表は、`aud`、`iss`、`sub`、`exp`、`iat`、`jti`、`nbf` と、GitHub が加える `actor`、`actor_id`、`base_ref`、`check_run_id`、`environment`、`event_name`、`head_ref`、`job_workflow_ref`、`job_workflow_sha`、`ref`、`ref_type`、`repository_visibility`、`repository`、`repository_id`、`repository_owner`、`repository_owner_id`、`repo_property_*`、`run_id`、`run_number`、`run_attempt`、`runner_environment`、`workflow`、`workflow_ref`、`workflow_sha` を説明する

表の前の文は「The OIDC token includes the following claims.」ですが、各 Claim がどの条件で JWT に含まれるかは書かれていません（GH-OIDC-CLAIM-PRESENCE）。

### GH-OIDC-REGISTERED-CLAIMS `exp`、`iat`、`nbf`、`jti` の意味

- **Evidence**：Documented
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：`exp` は JWT の有効期限、`iat` は JWT を発行した時刻、`nbf` は JWT をその時刻より前に使えないことを表す時刻、`jti` は OIDC トークンの一意な識別子である

### GH-OIDC-REPOSITORY-ID `repository_id` の意味

- **Evidence**：Documented
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：`repository_id` は、ワークフローを実行するリポジトリの ID である

### GH-OIDC-REPOSITORY-OWNER-ID `repository_owner_id` の意味

- **Evidence**：Documented
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：`repository_owner_id` は、`repository` のリポジトリを格納する Organization の ID である

公式資料の説明は Organization だけを挙げており、User が所有するリポジトリでの値は書かれていません（GH-OIDC-REPOSITORY-OWNER-ID-USER）。

### GH-OIDC-RUN-ID `run_id` の意味

- **Evidence**：Documented
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：`run_id` は、ワークフローを起動したワークフロー実行の ID である

### GH-OIDC-RUN-ATTEMPT `run_attempt` の意味

- **Evidence**：Documented
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：`run_attempt` は、ワークフロー実行が再試行された回数である

### GH-OIDC-ACTOR `actor` と `actor_id` の意味

- **Evidence**：Documented
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：`actor` はワークフロー実行を開始した個人アカウントで、`actor_id` はその個人アカウントの ID である

### GH-OIDC-EVENT-NAME `event_name` の意味

- **Evidence**：Documented
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：`event_name` は、ワークフロー実行を起動したイベントの名前である

### GH-OIDC-EVENT-NAME-DEPENDABOT Dependabot の更新ジョブの `event_name`

- **Evidence**：Documented
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：Dependabot の更新ジョブが要求した JWT の `event_name` は `dynamic` である

### GH-OIDC-REF `ref` の意味

- **Evidence**：Documented
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：`ref` は、ワークフロー実行を起動した Git の参照である

### GH-OIDC-WORKFLOW-REF `workflow_ref` と `job_workflow_ref` の意味

- **Evidence**：Documented
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：`workflow_ref` はワークフローの参照パスで、`job_workflow_ref` は再利用ワークフローを使うジョブでの再利用ワークフローの参照パスである。`workflow_sha` はワークフローファイルのコミット SHA で、`job_workflow_sha` は再利用ワークフローを使うジョブでの再利用ワークフローのファイルのコミット SHA である

### GH-OIDC-ENVIRONMENT `environment` の意味

- **Evidence**：Documented
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：`environment` は、ジョブが使う Environment の名前である

### GH-OIDC-CLAIMS-UNDESCRIBED 公式資料が説明しない Claim

- **Evidence**：Inferred
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)、[GitHub OIDC Discovery](../../research/github/oidc-discovery.md)
- **Statement**：GH-OIDC-DISCOVERY-CLAIMS の `claims_supported` のうち、`sha`、`ref_protected`、`enterprise`、`enterprise_id`、`environment_node_id`、`issuer_scope` の 6 個は、GH-OIDC-CLAIMS-TOKEN-TABLE の表にない

`sha` は表にありませんが、表の `workflow_sha` はワークフローファイルのコミットを指す別の Claim です（GH-OIDC-WORKFLOW-REF）。

### GH-OIDC-CLAIM-MEANING 公式資料が説明しない Claim の意味

- **Evidence**：Unconfirmed
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：GitHub は、OIDC token claims の表にない Claim（`sha`、`ref_protected`、`enterprise`、`enterprise_id`、`environment_node_id`、`issuer_scope`）の意味を定めていない
- **Confirmation**：公式資料にこれらの Claim の説明があるかを確かめる。認可などに使う前に、Claim が含まれる条件と値を、条件を変えた実測で記録する

### GH-OIDC-CLAIM-TYPE-EXAMPLE 公式の JWT の例の Claim の型

- **Evidence**：Documented
- **Source**：[OpenID Connect — Understanding the OIDC token](https://docs.github.com/en/actions/concepts/security/openid-connect#understanding-the-oidc-token)
- **Statement**：公式の JWT の例では、`exp`、`iat`、`nbf` は JSON の数値で、ほかの Claim は JSON の文字列である。`repository_id`、`repository_owner_id`、`actor_id`、`run_number`、`run_attempt` は数字だけの文字列である

公式の例は、`jti`、`sha`、`run_id`、`workflow` と Header の `x5t`、`kid` に、`example-` で始まる置き換えの値（`run_id` は `example-run-id`）を使っています。
これらの Claim について例が示すのは JSON の文字列であることだけで、値の形は示していません。

### GH-OIDC-CLAIM-TYPE-OBSERVED 記録した JWT の Claim の型

- **Evidence**：Observed
- **Source**：[GitHub OIDC JWT](../../research/github/oidc-jwt.md)、[GitHub OIDC Custom Audience](../../research/github/custom-audience.md)
- **Statement**：個人アカウントの private リポジトリで Audience を指定せずに発行された 6 件の JWT では、`exp`、`iat`、`nbf` は JSON の数値で、ほかの Claim はすべて JSON の文字列である。`repository_id`、`repository_owner_id`、`actor_id`、`run_id`、`run_attempt`、`run_number`、`check_run_id` は数字だけの文字列で、`ref_protected` は真偽値ではなく文字列の `"false"` である。URL でない文字列を Audience に指定した 2 件でも、`exp`、`iat`、`nbf` は JSON の数値で、`actor_id`、`repository_owner_id`、`repository_id`、`run_id`、`run_attempt` は JSON の文字列である

Audience を指定した 2 件は、ここに挙げた Claim と `aud` の型だけを記録しています。

### GH-OIDC-CLAIM-TYPE Claim の JSON の型

- **Evidence**：Unconfirmed
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：GitHub は、`repo_property_*` 以外の各 Claim の JSON の型を定めていない
- **Confirmation**：公式資料に Claim の型の記述があるかを確かめる

公式の例（GH-OIDC-CLAIM-TYPE-EXAMPLE）と記録した JWT（GH-OIDC-CLAIM-TYPE-OBSERVED）の型は、どちらもすべての JWT での型を表すものではありません。
`repo_property_*` の値の型は GH-OIDC-REPO-PROPERTY-VALUE が定めます。

### GH-OIDC-CLAIM-ID-FORMAT ID の Claim の値の表記

- **Evidence**：Unconfirmed
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)、[OIDC reference — Immutable subject claims](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims)
- **Statement**：GitHub は、`repository_owner_id`、`repository_id`、`run_id`、`run_attempt` と、不変形式の `sub` の Owner ID と Repository ID の値の表記（数字だけからなるか、先頭に `0` を持つか、桁数の上限）を定めていない
- **Confirmation**：公式資料に ID の値の表記の記述があるかを確かめる

公式の例の `repository_owner_id`、`repository_id`、`run_attempt` と、記録した JWT の ID の値は、どれも先頭に `0` を持たない数字だけの文字列です（GH-OIDC-CLAIM-TYPE-EXAMPLE、GH-OIDC-CLAIM-TYPE-OBSERVED）。
公式の例の `run_id` は置き換えの値で、`sub` は ID を含まない旧形式です。
`run_attempt` の初回の値は GH-OIDC-RUN-ATTEMPT-VALUE が扱います。

### GH-OIDC-REPO-PROPERTY-VALUE `repo_property_*` の値の表し方

- **Evidence**：Documented
- **Source**：[OIDC reference — Including repository custom properties in OIDC tokens](https://docs.github.com/en/actions/reference/security/oidc#including-repository-custom-properties-in-oidc-tokens)
- **Statement**：`repo_property_*` の値は JSON の文字列である。文字列と単一選択のプロパティは値をそのまま、複数選択のプロパティは選んだ値をカンマで区切って連結した文字列、真偽値のプロパティは文字列の `"true"` か `"false"` で表す

### GH-OIDC-REPO-PROPERTY-PRESENCE `repo_property_*` が含まれる条件

- **Evidence**：Documented
- **Source**：[OIDC reference — Including repository custom properties in OIDC tokens](https://docs.github.com/en/actions/reference/security/oidc#including-repository-custom-properties-in-oidc-tokens)
- **Statement**：Organization か Enterprise の管理者がカスタムプロパティを OIDC の設定に加えると、そのプロパティに値を設定したリポジトリの JWT は、プロパティの名前に `repo_property_` を前置した名前の Claim を含む

### GH-OIDC-CLAIM-PRESENCE-OBSERVED 記録した JWT の Claim の有無

- **Evidence**：Observed
- **Source**：[GitHub OIDC JWT](../../research/github/oidc-jwt.md)
- **Statement**：個人アカウントの private リポジトリで Audience を指定せずに発行された 6 件の JWT では、`environment` と `environment_node_id` は Environment を使うジョブの 1 件にだけ含まれる。`base_ref` と `head_ref` は pull_request 以外の起動イベントでも欠落せず、空文字列である。`claims_supported` のうち `enterprise`、`enterprise_id`、`issuer_scope` は 6 件のどれにも含まれない

### GH-OIDC-CLAIM-PRESENCE Claim が含まれる条件

- **Evidence**：Unconfirmed
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：GitHub は、`repo_property_*` 以外の各 Claim が JWT に含まれる条件と欠落する条件を定めていない
- **Confirmation**：公式資料に Claim ごとの出現条件の記述があるかを確かめる

`repo_property_*` が含まれる条件は GH-OIDC-REPO-PROPERTY-PRESENCE が定めます。

`claims_supported`（GH-OIDC-DISCOVERY-CLAIMS）はサポートする Claim の一覧で、すべての JWT がそのすべてを含むことを表すものではありません。

### GH-OIDC-REPOSITORY-OWNER-ID-OBSERVED 記録した JWT の `repository_owner_id`

- **Evidence**：Observed
- **Source**：[GitHub OIDC JWT](../../research/github/oidc-jwt.md)
- **Statement**：個人アカウントが所有する private リポジトリで Audience を指定せずに発行された 6 件の JWT は、どれも `repository_owner_id` を含む。6 件とも `repository_owner` と `actor` は同じアカウントで、`repository_owner_id` の値は `actor_id` と同じである

### GH-OIDC-REPOSITORY-OWNER-ID-USER User が所有するリポジトリの `repository_owner_id`

- **Evidence**：Unconfirmed
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)、[Variables reference](https://docs.github.com/en/actions/reference/workflows-and-actions/variables)
- **Statement**：GitHub は、User が所有するリポジトリで発行される JWT が `repository_owner_id` を含むかどうかと、その値を定めていない
- **Confirmation**：公式資料に User が所有するリポジトリでの `repository_owner_id` の記述があるかを確かめる。リポジトリのオーナーとワークフローを実行するアカウントが異なる条件などで JWT を取得して記録する

公式資料の説明は Organization の ID です（GH-OIDC-REPOSITORY-OWNER-ID）。
環境変数 `GITHUB_REPOSITORY_OWNER_ID` はリポジトリのオーナーのアカウント ID と説明されていますが、この Claim と同じ値になるとは書かれていません。

### GH-OIDC-RUN-ID-RERUN 再実行での `run_id`

- **Evidence**：Unconfirmed
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)、[Variables reference](https://docs.github.com/en/actions/reference/workflows-and-actions/variables)
- **Statement**：GitHub は、ワークフロー実行を再実行したときに `run_id` の値が変わるかどうかを定めていない
- **Confirmation**：初回と再実行の JWT の `run_id` を記録し、環境変数 `GITHUB_RUN_ID` と比べる

環境変数 `GITHUB_RUN_ID` は再実行で変わらないと説明されていますが、この Claim と同じ値になるとは書かれていません。

### GH-OIDC-RUN-ATTEMPT-OBSERVED 記録した JWT の `run_attempt`

- **Evidence**：Observed
- **Source**：[GitHub OIDC JWT](../../research/github/oidc-jwt.md)
- **Statement**：個人アカウントの private リポジトリで Audience を指定せずに発行された 6 件の JWT の `run_attempt` は、どれも `"1"` である

各実行が初回か再実行かは記録していません。

### GH-OIDC-RUN-ATTEMPT-VALUE 初回と再実行の `run_attempt`

- **Evidence**：Unconfirmed
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)、[Variables reference](https://docs.github.com/en/actions/reference/workflows-and-actions/variables)
- **Statement**：GitHub は、ワークフロー実行の初回と再実行での `run_attempt` の値を定めていない
- **Confirmation**：初回と再実行の JWT の `run_attempt` を記録し、環境変数 `GITHUB_RUN_ATTEMPT` と比べる

公式資料の説明は再試行された回数で（GH-OIDC-RUN-ATTEMPT）、初回の値を `0` と読むこともできます。
環境変数 `GITHUB_RUN_ATTEMPT` は初回を `1` とし再実行ごとに増えると説明されていますが、この Claim と同じ値になるとは書かれていません。

### GH-OIDC-WORKFLOW-REF-OBSERVED 記録した JWT の再利用ワークフローの参照

- **Evidence**：Observed
- **Source**：[GitHub OIDC JWT](../../research/github/oidc-jwt.md)
- **Statement**：個人アカウントの private リポジトリで Audience を指定せずに発行された 6 件の JWT では、`job_workflow_ref` は再利用ワークフローを使わないジョブにも含まれ、値は `workflow_ref` と同じである。再利用ワークフローを使うジョブでは、`job_workflow_ref` は再利用ワークフローを指し、`workflow_ref` は呼び出し元のワークフローを指す。再利用ワークフローを別のコミットのコミット SHA で参照した 1 件では、`job_workflow_ref` の `@` の後がそのコミット SHA で、`job_workflow_sha` は `sha` とも `workflow_sha` とも異なる

### GH-OIDC-ORGANIZATION-JWT Organization が所有するリポジトリの JWT

- **Evidence**：Unconfirmed
- **Source**：[GitHub OIDC JWT](../../research/github/oidc-jwt.md)
- **Statement**：Organization が所有するリポジトリで発行される JWT の Claim の有無、値、型は、資料から確定できない
- **Confirmation**：Organization が所有するリポジトリで JWT を取得して記録する

Observed の条項は、どれも個人アカウントが所有するリポジトリの記録です。

## 5. Time Claims

### GH-OIDC-TIME-EXAMPLE 公式の JWT の例の時刻の差

- **Evidence**：Documented
- **Source**：[OpenID Connect — Understanding the OIDC token](https://docs.github.com/en/actions/concepts/security/openid-connect#understanding-the-oidc-token)
- **Statement**：公式の JWT の例では、`nbf` は `iat` の 600 秒前で、`exp` は `iat` の 300 秒後である

### GH-OIDC-TIME-OBSERVED 記録した JWT の時刻の差

- **Evidence**：Observed
- **Source**：[GitHub OIDC JWT](../../research/github/oidc-jwt.md)
- **Statement**：個人アカウントの private リポジトリで Audience を指定せずに発行された 6 件の JWT では、どれも `nbf` は `iat` の 300 秒前で、`exp` は `iat` の 300 秒後である

### GH-OIDC-TIME-INTERVAL `nbf`、`iat`、`exp` の差

- **Evidence**：Unconfirmed
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：GitHub は、`nbf`、`iat`、`exp` の値の差を定めていない
- **Confirmation**：公式資料に JWT の有効期間や時刻の差の記述があるかを確かめる

公式の例（GH-OIDC-TIME-EXAMPLE）と記録した JWT（GH-OIDC-TIME-OBSERVED）では、`nbf` と `iat` の差が異なります。

## 6. Audience

### GH-OIDC-AUD-DEFAULT Audience の既定値

- **Evidence**：Documented
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)
- **Statement**：Audience を指定しない場合、`aud` はリポジトリのオーナーの URL である

### GH-OIDC-AUD-CUSTOM Audience の指定

- **Evidence**：Documented
- **Source**：[OIDC reference — OIDC token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)、[OIDC reference — Customizing the audience value](https://docs.github.com/en/actions/reference/security/oidc#customizing-the-audience-value)
- **Statement**：JWT を要求するときに Audience を指定できる。JWT が発行された場合、指定した値が `aud` になる

公式資料は、Actions Toolkit の `getIDToken(audience)` と、`ACTIONS_ID_TOKEN_REQUEST_URL` に `audience` のクエリパラメーターを付けた要求の例を示しています（[OIDC reference — Methods for requesting the OIDC token](https://docs.github.com/en/actions/reference/security/oidc#methods-for-requesting-the-oidc-token)）。
指定した Audience に対して JWT が発行されないことがあります（GH-OIDC-AUD-CUSTOM-ISSUANCE）。

### GH-OIDC-AUD-DEFAULT-OBSERVED 記録した JWT の既定の `aud`

- **Evidence**：Observed
- **Source**：[GitHub OIDC JWT](../../research/github/oidc-jwt.md)
- **Statement**：個人アカウントの private リポジトリで Audience を指定せずに発行された 6 件の JWT の `aud` は、どれもリポジトリのオーナーの URL（`https://github.com/octocat`）の JSON 文字列である

### GH-OIDC-AUD-CUSTOM-STRING 指定した Audience の `aud` の形

- **Evidence**：Observed
- **Source**：[GitHub OIDC Custom Audience](../../research/github/custom-audience.md)
- **Statement**：個人アカウントの private リポジトリで、URL でない文字列を Audience に指定して発行された 2 件の JWT の `aud` は、どちらも指定した値そのものの JSON 文字列で、配列ではない

### GH-OIDC-AUD-CUSTOM-ISSUANCE 指定した Audience ごとの発行の可否

- **Evidence**：Observed
- **Source**：[GitHub OIDC Custom Audience](../../research/github/custom-audience.md)
- **Statement**：個人アカウントの private リポジトリで workflow_dispatch で起動したワークフローから Audience を指定して JWT を要求すると、次のとおりだった。試した github.com の URL のうち、発行されたのは呼び出し元のオーナーの URL、呼び出し元のリポジトリの URL、`https://github.com` だけで、同じオーナーの別のリポジトリの URL（末尾にスラッシュを付けたもの、ホスト名に大文字を混ぜたものを含む）と別のオーナーの URL は、JWT を発行せず `400` を返した。別のリポジトリを指す api.github.com の URL も `400` を返した。github.io の URL は、呼び出し元と異なるオーナーのサブドメインでも発行された。`example.com` の https の URL、`urn:` と `api://` の URI、URL でない文字列、`sts.amazonaws.com` も発行された

api.github.com の URL は、別のリポジトリを指すものだけを試しています。

### GH-OIDC-AUD-CUSTOM-REFUSAL 指定した Audience の発行を拒否する規則

- **Evidence**：Unconfirmed
- **Source**：[OIDC reference — Customizing the audience value](https://docs.github.com/en/actions/reference/security/oidc#customizing-the-audience-value)、[GitHub OIDC Custom Audience](../../research/github/custom-audience.md)
- **Statement**：GitHub は、指定した Audience に対して JWT を発行しない規則を定めていない
- **Confirmation**：公式資料に Audience の発行の制限の記述があるかを確かめる。試していない形（呼び出し元のリポジトリの下のパス、別のオーナーのリポジトリの URL など）を指定して発行の可否を記録する

GH-OIDC-AUD-CUSTOM-ISSUANCE の拒否は、公式資料には記載がありません。

## 7. Subject

### GH-OIDC-SUB-IMMUTABLE-REASON 不変形式の導入の理由

- **Evidence**：Documented
- **Source**：[OIDC reference — Immutable subject claims](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims)、[GitHub Changelog — Immutable subject claims](https://github.blog/changelog/2026-04-23-immutable-subject-claims-for-github-actions-oidc-tokens/)
- **Statement**：旧形式の既定の `sub` は Organization とリポジトリの名前だけで構成されるため、名前が再利用されると、別のオーナーが同じ値の `sub` を作れる。不変形式は、この問題への対策である

公式資料は、OpenID Connect の仕様が `sub` に局所的に一意で再割当てされないことを求めることを、導入の理由に挙げています。

### GH-OIDC-SUB-IMMUTABLE-FORMAT 不変形式の構文

- **Evidence**：Documented
- **Source**：[OIDC reference — Immutable subject claims](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims)
- **Statement**：不変形式の既定の `sub` は、オーナーの名前と Owner ID、リポジトリの名前と Repository ID を含み、名前と ID を `@` で区切る

```text
構文:      repo:OWNER@OWNER-ID/REPO@REPO-ID:ref:refs/heads/BRANCH
旧形式:    repo:octo-org/octo-repo:ref:refs/heads/main
不変形式:  repo:octo-org@123456/octo-repo@456789:ref:refs/heads/main
```

### GH-OIDC-SUB-EXAMPLE-LEGACY 公式の JWT の例の `sub`

- **Evidence**：Documented
- **Source**：[OpenID Connect — Understanding the OIDC token](https://docs.github.com/en/actions/concepts/security/openid-connect#understanding-the-oidc-token)
- **Statement**：公式の JWT の例の `sub`（`repo:octo-org/octo-repo:environment:prod`）は旧形式である

### GH-OIDC-SUB-AT-SEPARATOR 名前に使えない `@`

- **Evidence**：Documented
- **Source**：[OIDC reference — Immutable subject claims](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims)
- **Statement**：`@` は GitHub のユーザー名とリポジトリ名に使えない

公式資料は、名前と ID の区切りに `@` を使う理由としてこの事実を挙げています。

### GH-OIDC-SUB-IMMUTABLE-DEFAULT 新しいリポジトリの既定の形式

- **Evidence**：Documented
- **Source**：[OIDC reference — Immutable subject claims](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims)
- **Statement**：2026 年 7 月 15 日より後に作成されたリポジトリは、不変形式を既定の `sub` の形式として使う

### GH-OIDC-SUB-LEGACY-RETAINED 古いリポジトリの既定の形式

- **Evidence**：Documented
- **Source**：[OIDC reference — Immutable subject claims](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims)、[REST API — GitHub Actions OIDC](https://docs.github.com/en/rest/actions/oidc?apiVersion=2026-03-10)
- **Statement**：2026 年 7 月 15 日より前に作成されたリポジトリは、Organization かリポジトリの単位で不変形式の利用を選択するまで、旧形式を使い続ける。REST API では、この選択を `use_immutable_subject` で設定する

### GH-OIDC-SUB-RENAME-TRANSFER 名前の変更と移管による形式の変更

- **Evidence**：Documented
- **Source**：[OIDC reference — Immutable subject claims](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims)
- **Statement**：2026 年 7 月 15 日より後にリポジトリの名前を変更するか、リポジトリを移管すると、そのリポジトリの `sub` も不変形式に移る

### GH-OIDC-SUB-CONTEXT 既定の `sub` の末尾

- **Evidence**：Documented
- **Source**：[OIDC reference — Example subject claims](https://docs.github.com/en/actions/reference/security/oidc#example-subject-claims)
- **Statement**：既定の `sub` のリポジトリの後の部分は、ジョブが Environment を参照する場合は `environment:` と Environment の名前、Environment を参照せず pull_request のイベントで起動した場合は `pull_request`、どちらでもない場合は `ref:` と起動したブランチかタグの参照である。値に含まれる `:` は `%3A` に置き換える

### GH-OIDC-SUB-OBSERVED 記録した JWT の `sub`

- **Evidence**：Observed
- **Source**：[GitHub OIDC JWT](../../research/github/oidc-jwt.md)
- **Statement**：個人アカウントの private リポジトリで Audience を指定せずに発行された 6 件の JWT の `sub` は、どれも不変形式である。リポジトリの ID に続く `:` より後の部分は起動条件によって変わり、再利用ワークフローの有無では形式は変わらない

```text
push、workflow_dispatch:  repo:octocat@583231/example-repo@1234567890:ref:refs/heads/<branch>
pull_request:             repo:octocat@583231/example-repo@1234567890:pull_request
Environment を使うジョブ:  repo:octocat@583231/example-repo@1234567890:environment:testing
```

リポジトリの作成時期と `use_immutable_subject` の設定は記録していないため、どの条件で不変形式になったかはこの記録からは分かりません。

### GH-OIDC-SUB-LENGTH `sub` の長さと文字

- **Evidence**：Unconfirmed
- **Source**：[OIDC reference — Example subject claims](https://docs.github.com/en/actions/reference/security/oidc#example-subject-claims)
- **Statement**：GitHub は、`sub` の長さの上限と、`sub` に含まれ得る文字を定めていない
- **Confirmation**：公式資料に `sub` の長さと文字の記述があるかを確かめる。長いブランチ名や Environment の名前、ASCII でない文字を含む名前を使うジョブで JWT を取得し、`sub` の長さと文字を記録する

## 8. Subject Customization

### GH-OIDC-SUB-CUSTOM-REPLACE カスタマイズによる `sub` の置き換え

- **Evidence**：Documented
- **Source**：[OIDC reference — Customizing the subject claims for an organization or repository](https://docs.github.com/en/actions/reference/security/oidc#customizing-the-subject-claims-for-an-organization-or-repository)
- **Statement**：`include_claim_keys` によるカスタマイズは、既定の `sub` の形式の全体を、カスタマイズした形式に置き換える

### GH-OIDC-SUB-CUSTOM-IMMUTABLE-IDS カスタマイズした不変形式の ID

- **Evidence**：Documented
- **Source**：[OIDC reference — Customizing the subject claims for an organization or repository](https://docs.github.com/en/actions/reference/security/oidc#customizing-the-subject-claims-for-an-organization-or-repository)
- **Statement**：不変形式を使うリポジトリでは、`include_claim_keys` でカスタマイズしても、`sub` の `repo` の部分に Owner ID と Repository ID が必ず含まれ、不変形式からこれらの ID を除けない

### GH-OIDC-SUB-CUSTOM-EXAMPLES カスタマイズの例

- **Evidence**：Documented
- **Source**：[OIDC reference — Customizing the subject claims for an organization or repository](https://docs.github.com/en/actions/reference/security/oidc#customizing-the-subject-claims-for-an-organization-or-repository)
- **Statement**：公式資料のカスタマイズの例には、`repo` だけを指定するテンプレートと、`repository_owner:monalisa` のように `repo` の部分を持たない `sub` になるテンプレートがある。例の `sub` は、旧形式と不変形式を区別していない

### GH-OIDC-SUB-CUSTOM-WITHOUT-REPO `repo` を含めないテンプレートの不変形式での `sub`

- **Evidence**：Unconfirmed
- **Source**：[OIDC reference — Customizing the subject claims for an organization or repository](https://docs.github.com/en/actions/reference/security/oidc#customizing-the-subject-claims-for-an-organization-or-repository)
- **Statement**：GitHub は、`repo` を含めないテンプレートを不変形式のリポジトリに適用したときの `sub` を定めていない
- **Confirmation**：公式資料にこの組み合わせの記述があるかを確かめる。不変形式のリポジトリに `repo` を含めないテンプレートを適用して JWT を取得し、`sub` を記録する

GH-OIDC-SUB-CUSTOM-EXAMPLES の `repo` の部分を持たない `sub` が、不変形式のリポジトリにもそのまま当てはまるとは断定できません。

### GH-OIDC-SUB-CUSTOM-REPO-ONLY `repo` だけを指定するテンプレートの `sub` の末尾

- **Evidence**：Unconfirmed
- **Source**：[OIDC reference — Customizing the subject claims for an organization or repository](https://docs.github.com/en/actions/reference/security/oidc#customizing-the-subject-claims-for-an-organization-or-repository)
- **Statement**：GitHub は、`repo` だけを指定するテンプレートを適用したときの `sub` の、`repo` の部分より後の形式（`:` とその後の部分があるかどうかを含む）を定めていない
- **Confirmation**：不変形式のリポジトリに `repo` だけを指定するテンプレートを適用して JWT を取得し、`sub` を記録する

### GH-OIDC-SUB-ORG-TEMPLATE-OPT-IN Organization のテンプレートの適用

- **Evidence**：Documented
- **Source**：[OIDC reference — Customizing the subject claims for an organization or repository](https://docs.github.com/en/actions/reference/security/oidc#customizing-the-subject-claims-for-an-organization-or-repository)
- **Statement**：Organization のカスタムの `sub` のテンプレートは、既存のリポジトリにも新しいリポジトリにも、リポジトリの単位の REST API で `use_default` を `false` にして利用を選択するまで適用されない

### GH-OIDC-SUB-ORG-TEMPLATE-SEPARATE テンプレートの選択と不変形式の選択の区別

- **Evidence**：Documented
- **Source**：[REST API — GitHub Actions OIDC](https://docs.github.com/en/rest/actions/oidc?apiVersion=2026-03-10)
- **Statement**：リポジトリの単位の REST API で、テンプレートの利用の選択（`use_default`）と不変形式の利用の選択（`use_immutable_subject`）は、別のパラメーターである

### GH-OIDC-SUB-FORMAT-PER-REPO Organization の設定と個々のリポジトリの `sub` の形式

- **Evidence**：Inferred
- **Source**：[OIDC reference — Immutable subject claims](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims)、[OIDC reference — Customizing the subject claims for an organization or repository](https://docs.github.com/en/actions/reference/security/oidc#customizing-the-subject-claims-for-an-organization-or-repository)
- **Statement**：テンプレートの適用にはリポジトリの側の選択が要り（GH-OIDC-SUB-ORG-TEMPLATE-OPT-IN）、既定の形式はリポジトリの作成時期、不変形式の選択、名前の変更と移管で決まる（GH-OIDC-SUB-IMMUTABLE-DEFAULT、GH-OIDC-SUB-LEGACY-RETAINED、GH-OIDC-SUB-RENAME-TRANSFER）。そのため、Organization の設定だけでは、個々のリポジトリが発行する `sub` の形式を確定できない
