# GitHub Constraints

## 1. Scope and Evidence

この文書は、GitHub.com の GitHub Actions、GitHub App、GitHub REST API の仕様を、GitHub の公式資料だけを根拠に整理したものです。GitHub の公式資料そのものではなく、GitHub の仕様が変更された場合は該当する公式資料を確認します。
[Boundaries with JWT and OpenID Connect Standards](#8-boundaries-with-jwt-and-openid-connect-standards) は、この整理を標準仕様と比較します。

- **対象**：GitHub.com。GitHub Enterprise Server は対象外。
- **REST API の確認対象版**：`2026-03-10`。以下の REST API 資料と OpenAPI 定義（REST API Description）はこの版を参照します。
- **バージョン指定の仕様**：REST API は `X-GitHub-Api-Version` で版を指定します。ヘッダーを省略すると `2022-11-28` が使われます。各版は新しい版の公開から24か月サポートされ、`2022-11-28` のサポートは2028年3月10日に終わります。サポート終了後、その版を指定した要求は `410 Gone` になり、ヘッダーを省略した要求の既定の版は変わります。版に依存する記述は、別の版では照合が必要です（[API Versions](https://docs.github.com/en/rest/about-the-rest-api/api-versions)）。

根拠の種類は次のように区別します。

| 区分 | 意味 |
|---|---|
| 公式仕様 | リンクした GitHub の公式資料に記述された内容。資料の条件・適用範囲を含めて読む |
| 公開値 | GitHub が公開するエンドポイントから取得した値。取得時点の値であり、将来も同じ値であることの保証ではない |
| 推論 | 公式仕様や公開値から導いた解釈。GitHub が明示的に保証した事実とは区別する |
| 未確認 | 公式資料で確定できない動作 |

## 2. Owner and Repository

### 2.1 オーナーと識別子

**公式仕様**：GitHub のリポジトリは個人アカウント（`User`）または Organization に所属します。REST API のリポジトリ表現には `id`、`full_name`、`owner` があり、オーナーの表現には `login`、`id`、`type` が含まれます。`full_name` は `owner/repo` 形式です。OIDC JWT にも `repository_id` と `repository_owner_id` という ID が用意されています（[Creating a new repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/creating-a-new-repository)、[OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)、[Repositories API](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10)）。

**公式仕様（名前の大文字小文字）**：REST API のパスパラメーターの説明は、`org`、`owner`、`repo` の名前を大文字小文字を区別しないものとしています。
`username` の説明にはこの記述がありません（[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)）。

### 2.2 名前の制約

**公式仕様（リポジトリ名）**：新規リポジトリ名の長さは最大100文字、許容される文字は ASCII 英数字と `.`, `-`, `_` です（[Creating a new repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/creating-a-new-repository)）。

**公式仕様（Enterprise Managed Users）**：GitHub.com の Enterprise Managed Users のユーザー名には、GitHub が末尾にアンダースコアと企業のショートコード（3〜8文字の英数字）を加えます。アンダースコアとショートコードを含めて最大39文字です（[Username considerations for external authentication](https://docs.github.com/en/enterprise-cloud@latest/admin/managing-iam/iam-configuration-reference/username-considerations-for-external-authentication)）。このアカウントは、企業のポリシーで許可されていれば private リポジトリに限り所有できます。特権アプリを除き、GitHub App を自身の個人アカウントにはインストールできず、管理者権限を持つリポジトリ単位のインストールは条件付きで可能です。所有するリポジトリでは GitHub-hosted runner を利用できません（[Abilities and restrictions of managed user accounts](https://docs.github.com/en/enterprise-cloud@latest/admin/managing-iam/understanding-iam-for-enterprises/abilities-and-restrictions-of-managed-user-accounts)）。

**未確認**：リポジトリ名の資料だけからは、名前が `.` または `..` である場合や、既存リポジトリへの例外的な命名規則を断定できません。
Enterprise Managed Users の資料には、GitHub のユーザー名は英数字とダッシュ（`-`）だけを含められるという記述があります。
しかし同じ資料が Enterprise Managed Users のユーザー名にアンダースコアを加えると定めるため、この記述をすべてのユーザー名に当てはまる規則としては扱いません。
Organization 名の規則もこの資料の対象外であり、`User` と Organization に一様に適用できる文字種・最大長・ハイフンの位置の規則は確定できていません。

## 3. GitHub Actions OIDC

### 3.1 Issuer、Audience、Claim

**公式仕様**：GitHub Actions の OIDC Issuer は `https://token.actions.githubusercontent.com` です。OIDC Discovery は同 Issuer の `/.well-known/openid-configuration` で提供され、対応する Claim は Discovery の `claims_supported` から確認できます。Audience (`aud`) は要求時に指定でき、既定値はリポジトリのオーナーの URL です。JOSE ヘッダーとして `alg`、`kid`、`typ` が説明されています（[OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)、[OIDC Discovery](https://token.actions.githubusercontent.com/.well-known/openid-configuration)）。

**公開値（Discovery）**：`issuer`、`jwks_uri`、`subject_types_supported`、`response_types_supported`（`id_token`）、`claims_supported`、`id_token_signing_alg_values_supported`（`RS256`）、`scopes_supported` があり、`authorization_endpoint` と `token_endpoint` はありません（[OIDC Discovery](https://token.actions.githubusercontent.com/.well-known/openid-configuration)）。

**公開値（JWKS）**：各鍵に `kty`（`RSA`）、`n`、`e`、`use`（`sig`）、`alg`（`RS256`）、`kid` があり、`key_ops` はありません。
`x5c` と `x5t` は、一部の鍵にだけあります（[JWKS](https://token.actions.githubusercontent.com/.well-known/jwks)）。

主な Claim は次のとおりです。

| Claim | GitHub の説明に基づく意味 |
|---|---|
| `iss`, `aud`, `sub` | 発行者、受信者、主体 |
| `exp`, `iat`, `nbf` | 有効期限、発行時刻、利用開始可能時刻 |
| `repository_id` | ワークフロー実行元のリポジトリの ID |
| `repository_owner_id` | 公式の説明は、リポジトリを格納する Organization の ID である。環境変数 `GITHUB_REPOSITORY_OWNER_ID` はリポジトリ所有者のアカウント ID とされるが、User 所有のリポジトリでのこの Claim の値は明記されていない |
| `run_id` | ワークフロー実行の ID |
| `run_attempt` | 公式の説明は、ワークフロー実行が再試行された回数である。環境変数 `GITHUB_RUN_ATTEMPT` は初回を `1` とするが、この Claim との一致は明記されていない |
| `actor`, `actor_id`, `event_name`, `ref` | 実行を開始した個人アカウントとその ID、起動イベント、起動した Git の参照 |
| `sha` | コミット SHA。Token claims の表にはなく、Discovery の `claims_supported` に含まれる。表にある `workflow_sha` はワークフローファイルのコミットを指す別の Claim である |
| `workflow_ref`, `job_workflow_ref`, `environment` | ワークフローの参照パス、再利用ワークフローを使うジョブの参照パス、ジョブが使う Environment の名前 |

出典：[OIDC reference — Token claims](https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims)、[Variables reference](https://docs.github.com/en/actions/reference/workflows-and-actions/variables)。各 Claim が欠落する条件と、Claim の JSON の型は明記されていません。ここに列挙した Claim がすべてのジョブで必ず含まれるという意味ではありません。

**公式仕様（JWT の例）**：公式の JWT の例では、Header に `typ`、`alg`、`x5t`、`kid` があります。
`exp`、`iat`、`nbf` は JSON の数値で、`repository_id`、`repository_owner_id`、`run_attempt` は数字の JSON 文字列です。
`nbf` は `iat` の10分前、`exp` は `iat` の5分後です（[OpenID Connect](https://docs.github.com/en/actions/concepts/security/openid-connect)）。

**推論**：これらは例の値であり、すべての JWT の型や時刻の差を保証するものとは読めません。
JWT の標準仕様は `nbf`、`iat`、`exp` の差を定めていないため、実 JWT の差が例と異なっても（確認結果は [Lambda Design](../design/lambda.md)）、それだけでは仕様違反になりません。

### 3.2 Immutable subject とカスタマイズ

**公式仕様（導入の理由）**：OpenID Connect の仕様は `sub` に、局所的に一意で再割当てされないことを求めています。
旧形式の既定の `sub` は名前だけで構成されるため、Organization やリポジトリの名前が再利用されると、別のオーナーが同じ `sub` の JWT を発行させられます。
不変形式は、この問題への対策です（[OIDC reference — Immutable subject claims](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims)、[GitHub Changelog — Immutable subject claims](https://github.blog/changelog/2026-04-23-immutable-subject-claims-for-github-actions-oidc-tokens/)）。

**公式仕様（既定の `sub`）**：2026年7月15日より後に作成されたリポジトリは、不変 Owner ID と Repository ID を含む形式を既定で利用します。同日より前に作成されたリポジトリは、Organization またはリポジトリ単位の設定（REST API では `use_immutable_subject`）で切り替えない限り、旧形式を維持します。同日より後にリポジトリ名を変更した場合や、リポジトリを移管した場合も、不変形式へ移行します。名前と ID は `@` で区切られます。この文字は GitHub のユーザー名とリポジトリ名には使えません（[OIDC reference — Immutable subject claims](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims)、[GitHub Changelog — Immutable subject claims](https://github.blog/changelog/2026-04-23-immutable-subject-claims-for-github-actions-oidc-tokens/)）。

```text
旧形式:    repo:octo-org/octo-repo:ref:refs/heads/main
不変形式:  repo:octo-org@123456/octo-repo@456789:ref:refs/heads/main
```

**公式仕様（`sub` のカスタマイズ）**：`include_claim_keys` によるカスタマイズは、既定の `sub` 全体をカスタム形式に置き換えます。
不変形式を使用するリポジトリでは、`include_claim_keys` でカスタマイズしても `repo` 部分に Owner ID と Repository ID が必ず含まれ、不変形式からこれらの ID を除けません。
カスタマイズの例には、`repo` だけを指定するテンプレートや、`repository_owner:monalisa` のように `repo` 部分のない `sub` もあります（[OIDC reference — Customizing the token claims](https://docs.github.com/en/actions/reference/security/oidc#customizing-the-token-claims)）。

**未確認**：`repo` を含めないテンプレートを不変形式のリポジトリに適用したときの `sub` は明記されていません。上記の例は不変形式を区別しておらず、そのまま不変形式のリポジトリに当てはまるとは断定しません。

**公式仕様（Organization テンプレート）**：Organization のカスタム `sub` テンプレートは、既存・新規を問わず、リポジトリがリポジトリ単位の REST API で `use_default` を `false` にして利用を選択するまで適用されません。**このテンプレートの利用選択は、不変形式への切り替え（`use_immutable_subject`）とは別の設定です**（[OIDC reference — Customizing the subject claims for an organization or repository](https://docs.github.com/en/actions/reference/security/oidc#customizing-the-subject-claims-for-an-organization-or-repository)、[REST API — GitHub Actions OIDC](https://docs.github.com/en/rest/actions/oidc?apiVersion=2026-03-10)）。

**推論**：テンプレートの適用にはリポジトリ側の選択が要り、作成時期による既定の形式の違いもあるため、Organization の設定だけでは個々のリポジトリが発行する `sub` の形式を確定できません。

## 4. GitHub App and App JWT

### 4.1 GitHub App の識別と Private Key

**公式仕様**：GitHub App には App ID と Client ID があり、両者は異なります。GitHub App JWT の `iss` にはいずれも使用できますが、Client ID が推奨されています（[Generating a JWT for a GitHub App](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app)）。

**公式仕様**：GitHub から取得する Private Key は PEM 形式で、PKCS#1 `RSAPrivateKey` です。1 App につき最大25個を保持でき、Private Key は自動失効せず手動で失効させます。GitHub が保存するのは鍵の公開部分だけです。Fingerprint は Private Key から導出した公開鍵 DER の SHA-256 の Base64 表現で照合できます（[Managing private keys](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps)）。

**公式仕様**：Private Key を削除するとき、GitHub App に鍵が一つしかなければ、**新しい鍵を生成してから** 既存の鍵を削除する必要があります（同資料の「Deleting private keys」）。

**推論**：GitHub が保存するのは公開部分だけであるため、ダウンロード済みの Private Key の内容は GitHub から再取得できません。

**未確認**：GitHub が新規生成する RSA 鍵のビット長を常に特定の値とする保証は、参照した公式資料から確認できません。

### 4.2 GitHub App JWT

**公式仕様**：GitHub App として認証する JWT は `RS256` で署名し、`iat`、`exp`、`iss` を含めます。`exp` は現在時刻から最大10分先までです。`iat` を60秒過去にすることは GitHub の推奨であり、必須値ではありません。JWT を GitHub API に渡すときは `Authorization: Bearer` を使用します（[Generating a JWT for a GitHub App](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app)）。

## 5. Installation

**公式仕様**：GitHub App は個人アカウントまたは Organization にインストールできます。同じ App を複数のアカウントにインストールすることもできます。App のインストール時、リポジトリ権限を要求する App では `All repositories` または `Only select repositories` を指定できます（[Installing a GitHub App](https://docs.github.com/en/apps/using-github-apps/installing-a-github-app-from-a-third-party)）。

**推論**：同じ App を複数のアカウントにインストールできるため、**App 自体の登録所有者と、Installation のインストール先アカウントは別の概念** です。

**公式仕様（対象リポジトリの変更）**：Organization オーナーがインストール済みの App について、リポジトリ管理者は、App の権限にかかわらず、自身が管理するリポジトリを Installation の対象に追加できます。
Organization オーナーがリポジトリ管理者によるインストールを制限すると、この追加はできません（[Installing a GitHub App](https://docs.github.com/en/apps/using-github-apps/installing-a-github-app-from-a-third-party)）。
また、リポジトリに対する `installation_repositories:write` を持つ呼び出し元は、そのリポジトリを Organization 内の任意の Installation に追加または削除できます。呼び出し元がユーザーの場合は、Organization オーナーまたはそのリポジトリの管理者である必要があります（[GitHub Changelog — GitHub App improvements](https://github.blog/changelog/2026-10-13-github-app-improvements-for-least-privilege-and-management/)）。

**公式仕様（公開リポジトリとトークンの対象範囲）**：GitHub のインストール手順は、GitHub App が GitHub 上のすべての公開リポジトリに少なくとも読み取りアクセスを持つと説明しています（[Installing a GitHub App](https://docs.github.com/en/apps/using-github-apps/installing-a-github-app-from-a-third-party#installing-a-github-app)）。一方、Installation Access Token の発行 API は、Installation に許可されていないリポジトリへのアクセスをトークンに付与できないと規定しています（[Generating an installation access token](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app)）。

**推論**：**App が公開リポジトリを読み取れることと、特定の Installation Access Token の対象に指定できることは別です。**

**未確認**：選択対象外の公開リポジトリに対するトークンの振る舞いは、これらの記述だけでは確定できません。

**公式仕様**：GitHub App JWT で認証すると、次の API から Installation 情報を取得できます（[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)）。公開されている応答コードは、Organization と User の API が `200` だけ、Repository の API が `200`、`301`、`404` です。

| 対象 | API |
|---|---|
| Organization | `GET /orgs/{org}/installation` |
| User | `GET /users/{username}/installation` |
| Repository | `GET /repos/{owner}/{repo}/installation` |

応答の公開例には `id`（Installation ID）、`account.id`、`account.type`、`target_type`、`repository_selection`（`all` / `selected`）、`permissions`、`suspended_at` などが含まれます。OpenAPI 定義では、`account` は `null`、`User` の表現、Enterprise の表現のいずれかで、Enterprise の表現には `type` がありません。`suspended_at` も `null` を取り得ます（[REST API Description](https://github.com/github/rest-api-description)）。

**推論**：オーナーを指定して単一の Installation を返す API があることから、GitHub App とオーナーの組で Installation は単一に決まると読めます。ただし、この一意性を明記した公式の記述はありません。

## 6. Installation Access Token

### 6.1 発行要求と有効期限

**公式仕様**：GitHub App JWT で `POST /app/installations/{installation_id}/access_tokens` を呼び出すと、Installation Access Token を発行できます（[Generating an installation access token](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app)、[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)、[REST API Description](https://github.com/github/rest-api-description)）。

| 要求の要素 | GitHub が定める仕様 |
|---|---|
| `repositories` | リポジトリ**名**の配列。公開例は `Hello-World` で、オーナー名を含まない |
| `repository_ids` | Repository ID の配列。`repositories` と並ぶ対象の指定方法 |
| 対象件数 | 一度に指定できる対象は最大500リポジトリ |
| `permissions` | 権限名から権限水準へのオブジェクト。例：`{"contents":"read"}`。スキーマ「App Permissions」では、権限名と水準（`read`、`write`、`admin`）はどれも ASCII 小文字とアンダースコアで表される。選べる水準は権限ごとに異なる（`workflows` は `write` だけ、`organization_events` は `read` だけなど） |
| 対象指定の省略 | Installation に許可された全リポジトリへのアクセスを持つ |
| Permissions 指定の省略 | App に付与された全権限を持つ |
| 発行上限 | Installation に許可されていないリポジトリへのアクセスと、App に付与されていない権限は付与できない |
| 有効期間 | 発行から1時間。期限切れトークンは再発行が必要 |

**公式仕様（応答コード）**：Installation Access Token の発行 API は `201`（作成）、`401`（認証が必要）、`403`（禁止）、`404`（見つからない）、`422`（検証失敗またはスパム判定）を公開しています（[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)）。特に **`422` は「対象リポジトリが Installation の範囲外」だけを意味するコードではありません。**

**未確認**：個々の失敗原因と応答コードの対応は公開されていません。

**公式仕様（レート制限）**：REST API は、プライマリとセカンダリのどちらのレート制限でも `403` または `429` を返します。プライマリでは `x-ratelimit-remaining` が `0` になります。セカンダリでは、制限の超過を示すエラーメッセージが返り、`retry-after` が付く場合があり、`x-ratelimit-remaining` が `0` の場合の扱いも説明されています（[Rate limits for the REST API](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)）。発行 API の応答コードの一覧に `429` はありませんが、レート制限の資料は REST API 全般の応答として `429` を挙げています。

**推論**：`x-ratelimit-remaining` が `0` になる場合はプライマリとセカンダリの両方にあるため、このヘッダーだけでは両者を区別できません。

### 6.2 発行応答・Permission

**公式仕様**：発行 API の応答には `token`、`expires_at`、`permissions`、`repositories`（適用される場合）、`repository_selection` などが示されます。`repository_selection` は `all` / `selected` の区別です。応答例には `repositories[].id`、`repositories[].full_name`、`repositories[].owner.id` といったリポジトリ情報が含まれます（[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)）。

**公式仕様（Permission の表現）**：`permissions` の表現には `contents: read` や `metadata: read` のような権限名と水準があります。

**未確認**：すべての発行で `metadata:read` が無条件に付与されるという保証は、参照した公式資料からは確定できません。

**公式仕様（トークンの形式）**：2026年4月27日に始まった Installation Access Token の新形式（`ghs_APPID_JWT`）への移行は完了しており、新規発行のトークンは既定でこの形式です。
長さは40文字から約520文字に増え、トークンに格納するデータによって変わります。
GitHub は、トークンを中身を解釈しない文字列として扱うよう求めており、40文字固定の検証、旧形式だけに合うパターン、短い固定長の保管先、長い `Authorization` ヘッダーを切り詰める中継、旧形式だけに合うログの秘匿ルールを点検の対象に挙げています。
トークン内の JWT はクライアントが検証するものではなく、その内容に依存することも禁じられています。
形式を切り替える `X-GitHub-Stateless-S2S-Token` ヘッダーは2026年11月30日に廃止されます。
権限、対象リポジトリ、1時間の有効期間、発行 API は変わりません（[GitHub Changelog — Stateless installation tokens](https://github.blog/changelog/2026-10-02-stateless-github-app-installation-tokens-rolled-out/)、[GitHub Changelog — Notice about the new format](https://github.blog/changelog/2026-04-24-notice-about-upcoming-new-format-for-github-app-installation-tokens/)）。

**未確認**：次の振る舞いは、参照した公式資料に記述がありません。

- Installation 範囲外のリポジトリを要求したとき、発行を拒否するか、対象を縮小した応答を返すか
- 500件を超えて指定したときの応答
- 停止中の Installation に対する発行要求の応答。App 停止の API の説明は、停止するとそのアカウントに対する App の API アクセスが遮断されるとするが、発行 API の応答コードは示していない（[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)）
- 発行応答の各フィールドの存在条件。OpenAPI 定義で必須なのは `token` と `expires_at` だけで、`permissions`、`repositories`、`repository_selection` は必須ではない（[REST API Description](https://github.com/github/rest-api-description)）

### 6.3 失効

**公式仕様**：`DELETE /installation/token` は、その呼び出しを認証した Installation Access Token 自身を失効させます。GitHub App JWT で呼び出す API ではありません。成功の応答は `204` で、失効するとそのトークンは利用できなくなります（[REST API — App installations](https://docs.github.com/en/rest/apps/installations?apiVersion=2026-03-10)）。この資料も新形式の移行の案内も、新形式のトークンでの失効の動作を個別には説明していません。

## 7. GitHub Actions Integration

**公式仕様**：GitHub Actions が OIDC JWT を取得するには、ワークフローまたはジョブで `permissions: id-token: write` を付与します。ランナーでは `ACTIONS_ID_TOKEN_REQUEST_URL` と `ACTIONS_ID_TOKEN_REQUEST_TOKEN` による取得方法も提供され、要求時に Audience を指定できます（[OIDC reference](https://docs.github.com/en/actions/reference/security/oidc#workflow-permissions-for-the-requesting-the-oidc-token)、[OIDC reference — Methods for requesting the OIDC token](https://docs.github.com/en/actions/reference/security/oidc#methods-for-requesting-the-oidc-token)）。

**公式仕様**：Composite Action では、`GITHUB_OUTPUT` へ書き込んだ値を、`outputs.<output_id>.value` で Action の Output として後続ステップへ公開できます。ログに表示させたくない値は `add-mask` でマスク対象に登録できます（[Workflow commands](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands)、[Metadata syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/metadata-syntax)）。

**公式仕様（マスクと Output）**：マスクはジョブごとに値ごとの登録が必要です。
同じ資料は、マスクした値は Output に設定できないと説明する一方で、同一ジョブ内なら生成、`add-mask`、`GITHUB_OUTPUT` の順で後続ステップへ渡す例を示しています。
ジョブやワークフローをまたいでマスクした値を渡す場合は、シークレットストアを経由するよう案内しています（[Workflow commands](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands)）。

**公式仕様**：GitHub Actions の Secrets / Variables はリポジトリ、Environment、Organization の単位で設定できます。Organization の Secrets / Variables はポリシーで利用できるリポジトリを制限でき、GitHub Free の private リポジトリからは Organization の Secrets / Variables を利用できません（[Using secrets](https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-secrets)、[Variables reference](https://docs.github.com/en/actions/reference/workflows-and-actions/variables)）。

**推論**：個人アカウントが所有するリポジトリでは Organization 単位の設定を使えないため、値はリポジトリまたは Environment の単位で設定します。

## 8. Boundaries with JWT and OpenID Connect Standards

GitHub が発行または受理する JWT には、[JWT Constraints](jwt-constraints.md) の規定がすべて同じように適用されるわけではありません。
GitHub Actions OIDC JWT は通常の OpenID Connect の認証フローとは異なる方法で発行され、GitHub App JWT は GitHub API の認証に使うためです。
この節の評価はすべて、公式資料と公開値からの推論です。

### 8.1 GitHub Actions OIDC と OpenID Connect の相違

| 項目 | 標準仕様 | GitHub の仕様・公開値 | 評価 |
|---|---|---|---|
| `authorization_endpoint` | Discovery で **REQUIRED** | Discovery の公開値にない | Discovery が定める Metadata の要件を満たさない。通常の認証フローを実装するクライアントをそのまま適用できる保証はない |
| `token_endpoint` | Implicit Flow だけを使う場合を除き **REQUIRED** | Discovery の公開値にない。`response_types_supported` は `id_token` だけ | この欠落だけでは標準仕様への違反と断定できない |
| `sub` の再割当て | OpenID Connect Core は再割当てを禁止 | 旧形式では、名前の再利用で別のオーナーが同じ `sub` を発行させられる。不変形式は Owner ID と Repository ID を含む | 旧形式は再割当ての禁止を満たせない。不変形式は対策だが、旧形式のリポジトリも残る |
| `aud` と認証フロー | ID Token の `aud` は登録済みの `client_id` を含む。`nonce` は認証要求で送った場合に照合する | ワークフローが認証要求を介さずに JWT を取得し、Audience を指定する。既定値はオーナーの URL | `client_id` と `nonce` の検証を機械的に適用することは適切でない |

これらの相違は、署名の検証、Issuer と検証鍵の結び付け、`aud` の照合を省略する根拠にはなりません。
これらは ID Token に限らない JWT の規則に基づくためです（[JWT Constraints — OpenID Connect and Discovery](jwt-constraints.md#5-openid-connect-and-discovery)、[JWT Constraints — JWT Security Considerations](jwt-constraints.md#6-jwt-security-considerations)）。

JWT / JWS / JWK の基本規則については、参照した公式資料と公開値の範囲では、GitHub との矛盾は見つかっていません。
これは実 JWT を網羅して確かめた結果ではなく、確定していない事項は [Outstanding Verification](#9-outstanding-verification) に記録します。

### 8.2 GitHub App JWT と ID Token の相違

GitHub App JWT は、GitHub API の認証のために GitHub App 側が署名する JWT であり、OpenID Connect の ID Token として扱いません。
したがって、OpenID Connect Core が ID Token に求める `sub` と `aud` の必須条件や、`iss` を HTTPS URL とする条件は適用されません。
Client ID または App ID を `iss` にすることは、`iss` を文字列とする JWT 一般の規定と矛盾しません。
`RS256` の指定と `exp` の上限10分は、JWT の標準仕様ではなく GitHub が追加する条件です（[GitHub App JWT](#42-github-app-jwt)）。

## 9. Outstanding Verification

公式資料で確定できていない事項を次にまとめます。根拠と確認範囲は各セクションに記載しています。

| 項目 | 確定できていない内容 |
|---|---|
| オーナー名 | 一般の `User` / Organization の命名規則と適用上の例外 |
| リポジトリ名 | `.` と `..` の扱い、既存リポジトリへの例外的な命名規則 |
| OIDC | カスタマイズ後の `sub` の形式（`repo` を含めないテンプレートを不変形式のリポジトリに適用した場合を含む）、`sub` が OpenID Connect Core の上限255 ASCII 文字に常に収まるか、Claim の JSON の型、Claim が欠落する条件、`nbf`、`iat`、`exp` の時刻の差、JOSE Header に `crit`、`jku`、`jwk` などが現れるか、`User` 所有のリポジトリでの `repository_owner_id`、`run_attempt` の初回の値 |
| GitHub App の鍵 | 新規生成される RSA 鍵のビット長 |
| Installation の取得 API | Organization と User の API の `200` 以外の応答（インストールされていない場合を含む）、GitHub App とオーナーの組での一意性 |
| トークン発行 API | 範囲外リポジトリへの要求、選択対象外の公開リポジトリに対するトークンの振る舞い、500件超の要求、停止中の Installation への要求、`422` の原因、`metadata:read` の付与条件、応答フィールドの有無 |
| トークン失効 API | 新形式のトークンでの失効の動作 |
