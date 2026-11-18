# GitHub Actions OIDC Constraints

この文書は、GitHub.com の GitHub Actions OIDC の仕様を、GitHub の公式資料と、GitHub から取得した値の調査記録を根拠に整理したものです。
GitHub の公式資料そのものではなく、GitHub の仕様が変更された場合は該当する公式資料を確認します。

- **対象**：GitHub.com の GitHub Actions OIDC の Issuer、Discovery、JWKS、Claim、`sub`、Audience。GitHub Enterprise Server は対象外。
- **取得した値の調査記録**：[GitHub OIDC Discovery](../../research/github/oidc-discovery.md)、[GitHub OIDC JWKS](../../research/github/oidc-jwks.md)、[GitHub OIDC JWT](../../research/github/oidc-jwt.md)、[GitHub OIDC Custom Audience](../../research/github/custom-audience.md)。取得日、取得方法、記録した応答はこれらの記録にある。

各記述の先頭の太字は、記述の根拠の Evidence を表します。Documented は公式資料の記述、Published は GitHub が公開するエンドポイントから取得した値、Observed は実環境で観測した値、Inferred はそれらから導いた解釈、Unconfirmed は公式資料で確定できない事項です。

## 1. Issuer、Audience、Claim

**Documented**：GitHub Actions の OIDC Issuer は `https://token.actions.githubusercontent.com` です。OIDC Discovery は同 Issuer の `/.well-known/openid-configuration` で提供され、対応する Claim は Discovery の `claims_supported` から確認できます。Audience (`aud`) は要求時に指定でき、既定値はリポジトリのオーナーの URL です。JOSE ヘッダーとして `alg`、`kid`、`typ` が説明されています（[OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)）。

**Published（Discovery）**：応答の Metadata は次の7個で、`authorization_endpoint` と `token_endpoint` はありません（[GitHub OIDC Discovery](../../research/github/oidc-discovery.md)）。

| Metadata | 値 |
|---|---|
| `issuer` | `https://token.actions.githubusercontent.com` |
| `jwks_uri` | `https://token.actions.githubusercontent.com/.well-known/jwks` |
| `subject_types_supported` | `public`、`pairwise` |
| `response_types_supported` | `id_token` |
| `claims_supported` | 36個の Claim 名 |
| `id_token_signing_alg_values_supported` | `RS256` |
| `scopes_supported` | `openid` |

**Published（JWKS）**：鍵は4個です。
すべての鍵に `kty`（`RSA`）、`alg`（`RS256`）、`use`（`sig`）、`kid`、`n`、`e`（`AQAB`）があり、`n` から求めた法の長さは2048 bit で、`key_ops` はありません。
`x5c`（証明書1個）と `x5t` は、4個のうち3個の鍵にだけあります。
`kid` の形式は一様でなく、UUID 形式と40桁の16進数字が2個ずつあります（[GitHub OIDC JWKS](../../research/github/oidc-jwks.md)）。

主な Claim は次のとおりです。

| Claim | GitHub の説明に基づく意味 |
|---|---|
| `iss`, `aud`, `sub` | 発行者、受信者、主体 |
| `exp`, `iat`, `nbf` | 有効期限、発行時刻、利用開始可能時刻 |
| `repository_id` | ワークフロー実行元のリポジトリの ID |
| `repository_owner_id` | 公式の説明は、リポジトリを格納する Organization の ID である。環境変数 `GITHUB_REPOSITORY_OWNER_ID` はリポジトリ所有者のアカウント ID とされるが、User 所有のリポジトリでのこの Claim の値は明記されていない |
| `run_id` | ワークフロー実行の ID。環境変数 `GITHUB_RUN_ID` は再実行で変わらないとされるが、この Claim との一致は明記されていない |
| `run_attempt` | 公式の説明は、ワークフロー実行が再試行された回数である。環境変数 `GITHUB_RUN_ATTEMPT` は初回を `1` とし再実行ごとに増やすが、この Claim との一致は明記されていない |
| `actor`, `actor_id`, `event_name`, `ref` | 実行を開始した個人アカウントとその ID、起動イベント、起動した Git の参照 |
| `sha` | コミット SHA。Token claims の表にはなく、Discovery の `claims_supported` に含まれる。表にある `workflow_sha` はワークフローファイルのコミットを指す別の Claim である |
| `workflow_ref`, `job_workflow_ref`, `environment` | ワークフローの参照パス、再利用ワークフローを使うジョブの参照パス、ジョブが使う Environment の名前 |

出典：[OIDC reference — Token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)、[Variables reference](https://docs.github.com/en/actions/reference/workflows-and-actions/variables)。各 Claim が欠落する条件と、Claim の JSON の型は明記されていません。ここに列挙した Claim がすべてのジョブで必ず含まれるという意味ではありません。

**Documented（JWT の例）**：公式の JWT の例では、Header に `typ`、`alg`、`x5t`、`kid` があります。
`exp`、`iat`、`nbf` は JSON の数値で、`repository_id`、`repository_owner_id`、`run_attempt` は数字の JSON 文字列です。
この例の `sub` は旧形式で、`repository_owner_id` も含まれています。
`nbf` は `iat` の10分前、`exp` は `iat` の5分後です（[OpenID Connect](https://docs.github.com/en/actions/concepts/security/openid-connect)）。

**Observed（JWT）**：個人アカウントの private リポジトリで Audience を指定せずに発行された JWT を、起動イベント、再利用ワークフロー、Environment の条件を変えて6件記録しています。
次の内容は、この6件から確かめたものです（[GitHub OIDC JWT](../../research/github/oidc-jwt.md)）。

- **Header**：6件とも `alg`（`RS256`）、`kid`、`typ`（`JWT`）、`x5t` だけからなり、値も同じである。`kid` と `x5t` は [GitHub OIDC JWKS](../../research/github/oidc-jwks.md) の同じ鍵の値と一致する
- **Claim の型**：`exp`、`iat`、`nbf` は JSON の数値で、ほかの Claim はすべて JSON の文字列である。`repository_id`、`repository_owner_id`、`actor_id`、`run_id`、`run_attempt`、`run_number`、`check_run_id` は数字だけの文字列で、`ref_protected` も真偽値ではなく文字列の `"false"` である
- **時刻の差**：6件とも `nbf` は `iat` の5分前、`exp` は `iat` の5分後である
- **`aud`**：リポジトリのオーナーの URL（`https://github.com/octocat`）である
- **`repository_owner_id`**：個人アカウントが所有するリポジトリの JWT にも含まれる。6件とも `repository_owner` と `actor` は同じアカウントで、値は `actor_id` と同じである
- **`run_attempt`**：6件とも `"1"` である。各実行が初回かどうかは記録していない
- **再利用ワークフロー**：`job_workflow_ref` は再利用ワークフローを使わないジョブにも含まれ、値は `workflow_ref` と同じである。再利用ワークフローを使うと、`job_workflow_ref` は再利用ワークフローを指し、`workflow_ref` は呼び出し元のワークフローを指す。別のコミットをコミット SHA で参照した記録では、`job_workflow_ref` の `@` の後がコミット SHA になり、`job_workflow_sha` は `sha`、`workflow_sha` と異なる
- **Claim の有無**：`environment` と `environment_node_id` は Environment を使う1件にだけ含まれる。`base_ref` と `head_ref` は pull_request 以外でも欠落せず、空文字列である。`claims_supported` のうち `enterprise`、`enterprise_id`、`issuer_scope` は6件のどれにも含まれない

**Observed（Audience の指定）**：Audience を指定して取得した JWT と、指定した値ごとの発行の可否を記録しています（[GitHub OIDC Custom Audience](../../research/github/custom-audience.md)）。

- **`aud`**：URL でない文字列を指定した2件とも、指定した値そのものの JSON 文字列で、配列ではない。時刻と ID の Claim の型、Header のキーと `alg` は、Audience を指定しない場合と同じである。ほかの Claim は記録していない
- **発行の可否**：試した github.com の URL のうち、発行されたのは呼び出し元のオーナーの URL、呼び出し元のリポジトリの URL、`https://github.com` だけである。同じオーナーの別のリポジトリの URL（末尾にスラッシュを付けたもの、ホスト名に大文字を混ぜたものを含む）と、別のオーナーの URL は、JWT を発行せず `400` を返した。api.github.com の URL は、別のリポジトリを指すものだけを試し、`400` を返した。github.io の URL は、呼び出し元と異なるオーナーのサブドメインでも発行された。`example.com` の https URL、`urn:` と `api://` の URI、URL でない文字列、`sts.amazonaws.com` も発行された。試していない形（呼び出し元のリポジトリの下のパス、別のオーナーのリポジトリの URL など）の扱いと拒否の規則は、公式資料に記載がない

**Inferred**：公式の例も実測値も、すべての JWT の型や時刻の差を保証するものとは読めません。
実測値の `nbf` は公式の例と異なり `iat` の5分前ですが、JWT の標準仕様は `nbf`、`iat`、`exp` の差を定めていないため、この違いだけでは仕様違反になりません。

## 2. Immutable subject とカスタマイズ

**Documented（導入の理由）**：OpenID Connect の仕様は `sub` に、局所的に一意で再割当てされないことを求めています。
旧形式の既定の `sub` は名前だけで構成されるため、Organization やリポジトリの名前が再利用されると、別のオーナーが同じ `sub` の JWT を GitHub に発行させることができます。
不変形式は、この問題への対策です（[OIDC reference — Immutable subject claims](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims)、[GitHub Changelog — Immutable subject claims](https://github.blog/changelog/2026-04-23-immutable-subject-claims-for-github-actions-oidc-tokens/)）。

**Documented（既定の `sub`）**：2026年7月15日より後に作成されたリポジトリは、不変 Owner ID と Repository ID を含む形式を既定で利用します。同日より前に作成されたリポジトリは、Organization またはリポジトリ単位の設定（REST API では `use_immutable_subject`）で切り替えない限り、旧形式を維持します。同日より後にリポジトリ名を変更した場合や、リポジトリを移管した場合も、不変形式へ移行します。名前と ID は `@` で区切られます。この文字は GitHub のユーザー名とリポジトリ名には使えません（[OIDC reference — Immutable subject claims](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims)、[GitHub Changelog — Immutable subject claims](https://github.blog/changelog/2026-04-23-immutable-subject-claims-for-github-actions-oidc-tokens/)）。

```text
旧形式:    repo:octo-org/octo-repo:ref:refs/heads/main
不変形式:  repo:octo-org@123456/octo-repo@456789:ref:refs/heads/main
```

**Observed（`sub`）**：記録した6件はすべて不変形式で、リポジトリ ID に続く `:` より後の部分は起動条件によって変わります（[GitHub OIDC JWT](../../research/github/oidc-jwt.md)）。
再利用ワークフローの有無では形式は変わりません。
リポジトリの作成時期と `use_immutable_subject` の設定は記録していないため、どの条件で不変形式になったかはこの記録からは分かりません。

```text
push、workflow_dispatch:  repo:octocat@583231/example-repo@1234567890:ref:refs/heads/<branch>
pull_request:             repo:octocat@583231/example-repo@1234567890:pull_request
Environment を使うジョブ:  repo:octocat@583231/example-repo@1234567890:environment:testing
```

**Documented（`sub` のカスタマイズ）**：`include_claim_keys` によるカスタマイズは、既定の `sub` 全体をカスタム形式に置き換えます。
不変形式を使用するリポジトリでは、`include_claim_keys` でカスタマイズしても `repo` 部分に Owner ID と Repository ID が必ず含まれ、不変形式からこれらの ID を除けません。
カスタマイズの例には、`repo` だけを指定するテンプレートや、`repository_owner:monalisa` のように `repo` 部分のない `sub` もあります（[OIDC reference — Customizing the token claims](https://docs.github.com/en/actions/reference/security/oidc#customizing-the-token-claims)）。

**Unconfirmed**：`repo` を含めないテンプレートを不変形式のリポジトリに適用したときの `sub` は明記されていません。上記の例は不変形式を区別しておらず、そのまま不変形式のリポジトリに当てはまるとは断定しません。

**Documented（Organization テンプレート）**：Organization のカスタム `sub` テンプレートは、既存・新規を問わず、リポジトリがリポジトリ単位の REST API で `use_default` を `false` にして利用を選択するまで適用されません。**このテンプレートの利用選択は、不変形式への切り替え（`use_immutable_subject`）とは別の設定です**（[OIDC reference — Customizing the subject claims for an organization or repository](https://docs.github.com/en/actions/reference/security/oidc#customizing-the-subject-claims-for-an-organization-or-repository)、[REST API — GitHub Actions OIDC](https://docs.github.com/en/rest/actions/oidc?apiVersion=2026-03-10)）。

**Inferred**：テンプレートの適用にはリポジトリ側の選択が要り、作成時期による既定の形式の違いもあるため、Organization の設定だけでは個々のリポジトリが発行する `sub` の形式を確定できません。

## 3. Outstanding Verification

公式資料で確定できていない事項を次にまとめます。根拠と確認範囲は各セクションに記載しています。
Published や Observed の記述で一部の条件を確かめた事項も、GitHub の保証がないためここに残します。

| 項目 | 確定できていない内容 |
|---|---|
| OIDC | カスタマイズ後の `sub` の形式（`repo` を含めないテンプレートを不変形式のリポジトリに適用した場合を含む）、`sub` が OpenID Connect Core の上限255 ASCII 文字に常に収まるか、Claim の JSON の型、Claim が欠落する条件、`nbf`、`iat`、`exp` の時刻の差、JOSE Header に `crit`、`jku`、`jwk` などが現れるか、`User` 所有のリポジトリでの `repository_owner_id`、`run_attempt` の初回の値、Organization が所有するリポジトリで発行された JWT の実測値、指定した Audience の発行を拒否する規則（実測した値の外での振る舞い） |
