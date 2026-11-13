# GitHub Constraints

## 1. Scope and Evidence

この文書は、GitHub.com の GitHub Actions、GitHub App、GitHub REST API の仕様を、GitHub の公式資料と、GitHub から取得した値の調査記録を根拠に整理したものです。
GitHub の公式資料そのものではなく、GitHub の仕様が変更された場合は該当する公式資料を確認します。
[Boundaries with JWT and OpenID Connect Standards](#8-boundaries-with-jwt-and-openid-connect-standards) は、この整理を標準仕様と比較します。

- **対象**：GitHub.com。GitHub Enterprise Server は対象外。
- **REST API の確認対象版**：`2026-03-10`。以下の REST API 資料と OpenAPI 定義（REST API Description）はこの版を参照する。
- **バージョン指定の仕様**：REST API は `X-GitHub-Api-Version` で版を指定する。ヘッダーを省略すると `2022-11-28` が使われる。各版は新しい版の公開から24か月サポートされ、`2022-11-28` のサポートは2028年3月10日に終わる。サポート終了後、その版を指定した要求は `410 Gone` になり、ヘッダーを省略した要求の既定の版は変わる。版に依存する記述は、別の版では照合が必要である（[API Versions](https://docs.github.com/en/rest/about-the-rest-api/api-versions)）。
- **`User-Agent` の仕様**：REST API のすべての要求は、有効な `User-Agent` ヘッダーを含む必要がある。ヘッダーのない要求は拒否され、無効な値には `403 Forbidden` が返る。値には、GitHub のユーザー名かアプリケーションの名前が推奨される（[Getting started with the REST API](https://docs.github.com/en/rest/using-the-rest-api/getting-started-with-the-rest-api)）。
- **取得した値の調査記録**：[GitHub OIDC Discovery](../research/github/oidc-discovery.md)、[GitHub OIDC JWKS](../research/github/oidc-jwks.md)、[GitHub OIDC JWT](../research/github/oidc-jwt.md)、[GitHub App Installation](../research/github/app-installation.md)、[GitHub Installation Access Token](../research/github/installation-access-token.md)、[GitHub App Installation Changes](../research/github/installation-changes.md)、[GitHub Composite Action Outputs](../research/github/composite-action-outputs.md)。取得日、取得方法、記録した応答はこれらの記録にある。

根拠の種類は次のように区別します。

| 区分 | 意味 |
|---|---|
| 公式仕様 | リンクした GitHub の公式資料に記述された内容。資料の条件・適用範囲を含めて読む |
| 公開値 | GitHub が公開するエンドポイントから取得し、調査記録に残した値。取得時点の値であり、将来も同じ値であることの保証ではない |
| 実測値 | GitHub Actions の実環境で発行された JWT、GitHub API の実際の応答、GitHub Actions の実行ログから取得し、調査記録に残した値。記録した条件での観測であり、ほかの条件や将来も同じ値であることの保証ではない |
| 推論 | 公式仕様、公開値、実測値から導いた解釈。GitHub が明示的に保証した事実とは区別する |
| 未確認 | 公式資料で確定できない動作。公開値や実測値があっても、それだけでは保証とみなさない |

## 2. Owner and Repository

### 2.1 オーナーと識別子

**公式仕様**：GitHub のリポジトリは個人アカウント（`User`）または Organization に所属します。REST API のリポジトリ表現には `id`、`full_name`、`owner` があり、オーナーの表現には `login`、`id`、`type` が含まれます。`full_name` は `owner/repo` 形式です。OIDC JWT にも `repository_id` と `repository_owner_id` という ID が用意されています（[Creating a new repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/creating-a-new-repository)、[OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)、[Repositories API](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10)）。

**公式仕様（名前の大文字小文字）**：REST API のパスパラメーターの説明は、`org`、`owner`、`repo` の名前は大文字小文字を区別しないとしています。
`username` の説明にはこの記述がありません（[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)）。

### 2.2 名前の制約

**公式仕様（リポジトリ名）**：新規リポジトリ名の長さは最大100文字、許容される文字は ASCII 英数字と `.`, `-`, `_` です（[Creating a new repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/creating-a-new-repository)）。

**公式仕様（Enterprise Managed Users）**：GitHub.com の Enterprise Managed Users のユーザー名には、GitHub が末尾にアンダースコアと企業のショートコード（3〜8文字の英数字）を加えます。アンダースコアとショートコードを含めて最大39文字です。正規化したユーザー名は、ダッシュで始まることも終わることもできず、連続したダッシュも含められません（[Username considerations for external authentication](https://docs.github.com/en/enterprise-cloud@latest/admin/managing-iam/iam-configuration-reference/username-considerations-for-external-authentication)）。このアカウントは、企業のポリシーで許可されていれば private リポジトリに限り所有できます。特権アプリを除き、GitHub App を自身の個人アカウントにはインストールできず、管理者権限を持つリポジトリ単位のインストールは条件付きで可能です。所有するリポジトリでは GitHub-hosted runner を利用できません（[Abilities and restrictions of managed user accounts](https://docs.github.com/en/enterprise-cloud@latest/admin/managing-iam/understanding-iam-for-enterprises/abilities-and-restrictions-of-managed-user-accounts)）。

**未確認**：リポジトリ名の資料だけからは、名前が `.` または `..` である場合の扱いや、既存リポジトリへの例外的な命名規則を断定できません。
Enterprise Managed Users の資料には、GitHub のユーザー名は英数字とダッシュ（`-`）だけを含められるという記述があります。
しかし同じ資料が Enterprise Managed Users のユーザー名にアンダースコアを加えると定めるため、この記述をすべてのユーザー名に当てはまる規則としては扱いません。
Organization 名の規則もこの資料の対象外であり、`User` と Organization に一様に適用できる文字種・最大長・ハイフンの位置の規則は確定できていません。

## 3. GitHub Actions OIDC

### 3.1 Issuer、Audience、Claim

**公式仕様**：GitHub Actions の OIDC Issuer は `https://token.actions.githubusercontent.com` です。OIDC Discovery は同 Issuer の `/.well-known/openid-configuration` で提供され、対応する Claim は Discovery の `claims_supported` から確認できます。Audience (`aud`) は要求時に指定でき、既定値はリポジトリのオーナーの URL です。JOSE ヘッダーとして `alg`、`kid`、`typ` が説明されています（[OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)）。

**公開値（Discovery）**：応答の Metadata は次の7個で、`authorization_endpoint` と `token_endpoint` はありません（[GitHub OIDC Discovery](../research/github/oidc-discovery.md)）。

| Metadata | 値 |
|---|---|
| `issuer` | `https://token.actions.githubusercontent.com` |
| `jwks_uri` | `https://token.actions.githubusercontent.com/.well-known/jwks` |
| `subject_types_supported` | `public`、`pairwise` |
| `response_types_supported` | `id_token` |
| `claims_supported` | 36個の Claim 名 |
| `id_token_signing_alg_values_supported` | `RS256` |
| `scopes_supported` | `openid` |

**公開値（JWKS）**：鍵は4個です。
すべての鍵に `kty`（`RSA`）、`alg`（`RS256`）、`use`（`sig`）、`kid`、`n`、`e`（`AQAB`）があり、`n` から求めた法の長さは2048 bit で、`key_ops` はありません。
`x5c`（証明書1個）と `x5t` は、4個のうち3個の鍵にだけあります。
`kid` の形式は一様でなく、UUID 形式と40桁の16進数字が2個ずつあります（[GitHub OIDC JWKS](../research/github/oidc-jwks.md)）。

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

**公式仕様（JWT の例）**：公式の JWT の例では、Header に `typ`、`alg`、`x5t`、`kid` があります。
`exp`、`iat`、`nbf` は JSON の数値で、`repository_id`、`repository_owner_id`、`run_attempt` は数字の JSON 文字列です。
この例の `sub` は旧形式で、`repository_owner_id` も含まれています。
`nbf` は `iat` の10分前、`exp` は `iat` の5分後です（[OpenID Connect](https://docs.github.com/en/actions/concepts/security/openid-connect)）。

**実測値（JWT）**：個人アカウントの private リポジトリで Audience を指定せずに発行された JWT を、起動イベント、再利用ワークフロー、Environment の条件を変えて6件記録しています。
次の内容は、この6件から確かめたものです（[GitHub OIDC JWT](../research/github/oidc-jwt.md)）。

- **Header**：6件とも `alg`（`RS256`）、`kid`、`typ`（`JWT`）、`x5t` だけからなり、値も同じである。`kid` と `x5t` は [GitHub OIDC JWKS](../research/github/oidc-jwks.md) の同じ鍵の値と一致する
- **Claim の型**：`exp`、`iat`、`nbf` は JSON の数値で、ほかの Claim はすべて JSON の文字列である。`repository_id`、`repository_owner_id`、`actor_id`、`run_id`、`run_attempt`、`run_number`、`check_run_id` は数字だけの文字列で、`ref_protected` も真偽値ではなく文字列の `"false"` である
- **時刻の差**：6件とも `nbf` は `iat` の5分前、`exp` は `iat` の5分後である
- **`aud`**：リポジトリのオーナーの URL（`https://github.com/octocat`）である
- **`repository_owner_id`**：個人アカウントが所有するリポジトリの JWT にも含まれる。6件とも `repository_owner` と `actor` は同じアカウントで、値は `actor_id` と同じである
- **`run_attempt`**：6件とも `"1"` である。各実行が初回かどうかは記録していない
- **再利用ワークフロー**：`job_workflow_ref` は再利用ワークフローを使わないジョブにも含まれ、値は `workflow_ref` と同じである。再利用ワークフローを使うと、`job_workflow_ref` は再利用ワークフローを指し、`workflow_ref` は呼び出し元のワークフローを指す。別のコミットをコミット SHA で参照した記録では、`job_workflow_ref` の `@` の後がコミット SHA になり、`job_workflow_sha` は `sha`、`workflow_sha` と異なる
- **Claim の有無**：`environment` と `environment_node_id` は Environment を使う1件にだけ含まれる。`base_ref` と `head_ref` は pull_request 以外でも欠落せず、空文字列である。`claims_supported` のうち `enterprise`、`enterprise_id`、`issuer_scope` は6件のどれにも含まれない

**実測値（Audience の指定）**：Audience を指定して取得した JWT と、指定した値ごとの発行の可否を記録しています（[GitHub OIDC Custom Audience](../research/github/custom-audience.md)）。

- **`aud`**：URL でない文字列を指定した2件とも、指定した値そのものの JSON 文字列で、配列ではない。時刻と ID の Claim の型、Header のキーと `alg` は、Audience を指定しない場合と同じである。ほかの Claim は記録していない
- **発行の可否**：試した github.com の URL のうち、発行されたのは呼び出し元のオーナーの URL、呼び出し元のリポジトリの URL、`https://github.com` だけである。同じオーナーの別のリポジトリの URL（末尾にスラッシュを付けたもの、ホスト名に大文字を混ぜたものを含む）と、別のオーナーの URL は、JWT を発行せず `400` を返した。api.github.com の URL は、別のリポジトリを指すものだけを試し、`400` を返した。github.io の URL は、呼び出し元と異なるオーナーのサブドメインでも発行された。`example.com` の https URL、`urn:` と `api://` の URI、URL でない文字列、`sts.amazonaws.com` も発行された。試していない形（呼び出し元のリポジトリの下のパス、別のオーナーのリポジトリの URL など）の扱いと拒否の規則は、公式資料に記載がない

**推論**：公式の例も実測値も、すべての JWT の型や時刻の差を保証するものとは読めません。
実測値の `nbf` は公式の例と異なり `iat` の5分前ですが、JWT の標準仕様は `nbf`、`iat`、`exp` の差を定めていないため、この違いだけでは仕様違反になりません。

### 3.2 Immutable subject とカスタマイズ

**公式仕様（導入の理由）**：OpenID Connect の仕様は `sub` に、局所的に一意で再割当てされないことを求めています。
旧形式の既定の `sub` は名前だけで構成されるため、Organization やリポジトリの名前が再利用されると、別のオーナーが同じ `sub` の JWT を GitHub に発行させることができます。
不変形式は、この問題への対策です（[OIDC reference — Immutable subject claims](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims)、[GitHub Changelog — Immutable subject claims](https://github.blog/changelog/2026-04-23-immutable-subject-claims-for-github-actions-oidc-tokens/)）。

**公式仕様（既定の `sub`）**：2026年7月15日より後に作成されたリポジトリは、不変 Owner ID と Repository ID を含む形式を既定で利用します。同日より前に作成されたリポジトリは、Organization またはリポジトリ単位の設定（REST API では `use_immutable_subject`）で切り替えない限り、旧形式を維持します。同日より後にリポジトリ名を変更した場合や、リポジトリを移管した場合も、不変形式へ移行します。名前と ID は `@` で区切られます。この文字は GitHub のユーザー名とリポジトリ名には使えません（[OIDC reference — Immutable subject claims](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims)、[GitHub Changelog — Immutable subject claims](https://github.blog/changelog/2026-04-23-immutable-subject-claims-for-github-actions-oidc-tokens/)）。

```text
旧形式:    repo:octo-org/octo-repo:ref:refs/heads/main
不変形式:  repo:octo-org@123456/octo-repo@456789:ref:refs/heads/main
```

**実測値（`sub`）**：記録した6件はすべて不変形式で、リポジトリ ID に続く `:` より後の部分は起動条件によって変わります（[GitHub OIDC JWT](../research/github/oidc-jwt.md)）。
再利用ワークフローの有無では形式は変わりません。
リポジトリの作成時期と `use_immutable_subject` の設定は記録していないため、どの条件で不変形式になったかはこの記録からは分かりません。

```text
push、workflow_dispatch:  repo:octocat@583231/example-repo@1234567890:ref:refs/heads/<branch>
pull_request:             repo:octocat@583231/example-repo@1234567890:pull_request
Environment を使うジョブ:  repo:octocat@583231/example-repo@1234567890:environment:testing
```

**公式仕様（`sub` のカスタマイズ）**：`include_claim_keys` によるカスタマイズは、既定の `sub` 全体をカスタム形式に置き換えます。
不変形式を使用するリポジトリでは、`include_claim_keys` でカスタマイズしても `repo` 部分に Owner ID と Repository ID が必ず含まれ、不変形式からこれらの ID を除けません。
カスタマイズの例には、`repo` だけを指定するテンプレートや、`repository_owner:monalisa` のように `repo` 部分のない `sub` もあります（[OIDC reference — Customizing the token claims](https://docs.github.com/en/actions/reference/security/oidc#customizing-the-token-claims)）。

**未確認**：`repo` を含めないテンプレートを不変形式のリポジトリに適用したときの `sub` は明記されていません。上記の例は不変形式を区別しておらず、そのまま不変形式のリポジトリに当てはまるとは断定しません。

**公式仕様（Organization テンプレート）**：Organization のカスタム `sub` テンプレートは、既存・新規を問わず、リポジトリがリポジトリ単位の REST API で `use_default` を `false` にして利用を選択するまで適用されません。**このテンプレートの利用選択は、不変形式への切り替え（`use_immutable_subject`）とは別の設定です**（[OIDC reference — Customizing the subject claims for an organization or repository](https://docs.github.com/en/actions/reference/security/oidc#customizing-the-subject-claims-for-an-organization-or-repository)、[REST API — GitHub Actions OIDC](https://docs.github.com/en/rest/actions/oidc?apiVersion=2026-03-10)）。

**推論**：テンプレートの適用にはリポジトリ側の選択が要り、作成時期による既定の形式の違いもあるため、Organization の設定だけでは個々のリポジトリが発行する `sub` の形式を確定できません。

## 4. GitHub App and App JWT

### 4.1 GitHub App の識別と Private Key

**公式仕様**：GitHub App には App ID と Client ID があり、両者は異なります。GitHub App JWT の `iss` にはいずれも使用できますが、Client ID が推奨されています（[Generating a JWT for a GitHub App](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app)、[Generating an installation access token](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app)）。

**公式仕様**：GitHub から取得する Private Key は PEM 形式で、PKCS#1 `RSAPrivateKey` です。1 App につき最大25個を保持でき、Private Key は自動失効せず手動で失効させます。GitHub が保存するのは鍵の公開部分だけです。Fingerprint は Private Key から導出した公開鍵 DER の SHA-256 の Base64 表現で照合できます（[Managing private keys](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps)）。

**公式仕様**：Private Key を削除するとき、GitHub App に鍵が一つしかなければ、**新しい鍵を生成してから** 既存の鍵を削除する必要があります（同資料の「Deleting private keys」）。

**推論**：GitHub が保存するのは公開部分だけであるため、ダウンロード済みの Private Key の内容は GitHub から再取得できません。

**未確認**：GitHub が新規生成する RSA 鍵のビット長を常に特定の値とする保証は、参照した公式資料から確認できません。

### 4.2 GitHub App JWT

**公式仕様**：GitHub App として認証する JWT は `RS256` で署名し、`iat`、`exp`、`iss` を含めます。`exp` は現在時刻から最大10分先までです。`iat` を60秒過去にすることは GitHub の推奨であり、必須ではありません。JWT を GitHub API に渡すときは `Authorization: Bearer` を使用します（[Generating a JWT for a GitHub App](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app)）。

## 5. Installation

**公式仕様**：GitHub App は個人アカウントまたは Organization にインストールできます。同じ App を複数のアカウントにインストールすることもできます。App のインストール時、リポジトリ権限を要求する App では `All repositories` または `Only select repositories` を指定できます（[Installing a GitHub App](https://docs.github.com/en/apps/using-github-apps/installing-a-github-app-from-a-third-party)）。

**推論**：同じ App を複数のアカウントにインストールできるため、**App 自体の登録所有者と、Installation のインストール先アカウントは別の概念** です。

**公式仕様（対象リポジトリの変更）**：Organization オーナーがインストール済みの App について、リポジトリ管理者は、App の権限にかかわらず、自身が管理するリポジトリを Installation の対象に追加できます。
Organization オーナーがリポジトリ管理者によるインストールを制限すると、この追加はできません（[Installing a GitHub App](https://docs.github.com/en/apps/using-github-apps/installing-a-github-app-from-a-third-party)）。
また、リポジトリに対する `installation_repositories:write` を持つ呼び出し元は、そのリポジトリを Organization 内の任意の Installation に追加または削除できます。呼び出し元がユーザーの場合は、Organization オーナーまたはそのリポジトリの管理者である必要があります（[GitHub Changelog — GitHub App improvements](https://github.blog/changelog/2026-10-13-github-app-improvements-for-least-privilege-and-management/)）。

**公式仕様（公開リポジトリとトークンの対象範囲）**：GitHub のインストール手順は、GitHub App が GitHub 上のすべての公開リポジトリに少なくとも読み取りアクセスを持つと説明しています（[Installing a GitHub App](https://docs.github.com/en/apps/using-github-apps/installing-a-github-app-from-a-third-party#installing-a-github-app)）。一方、Installation Access Token の発行 API は、Installation に許可されていないリポジトリへのアクセスをトークンに付与できないと規定しています（[Generating an installation access token](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app)）。

**推論**：**App が公開リポジトリを読み取れることと、特定の Installation Access Token の対象に指定できることは別です。**

**実測値（選択対象外の公開リポジトリ）**：インストール先のアカウントが所有し、Installation で選択していない公開リポジトリを含む発行の要求は、`422` で拒否されました（[GitHub Installation Access Token](../research/github/installation-access-token.md#発行が拒否された要求)）。
一方、対象を選択リポジトリに限って発行したトークンでは、その公開リポジトリの内容を読めました。
同じトークンで、要求していない選択リポジトリと選択対象外の private リポジトリの内容は `404` でした（[GitHub Installation Access Token](../research/github/installation-access-token.md#発行したトークンで呼んだ-api)）。

**公式仕様**：GitHub App JWT で認証すると、次の API から Installation 情報を取得できます（[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)）。資料に示された応答コードは、Organization と User の API が `200` だけ、Repository の API が `200`、`301`、`404` です。

| 対象 | API |
|---|---|
| Organization | `GET /orgs/{org}/installation` |
| User | `GET /users/{username}/installation` |
| Repository | `GET /repos/{owner}/{repo}/installation` |

資料の応答例には `id`（Installation ID）、`account.id`、`account.type`、`target_type`、`repository_selection`（`all` / `selected`）、`permissions`、`suspended_at` などが含まれます。OpenAPI 定義では、`account` は `null`、`User` の表現、Enterprise の表現のいずれかで、Enterprise の表現には `type` がありません。`suspended_at` も `null` を取り得ます（[REST API Description](https://github.com/github/rest-api-description)）。

**実測値（Installation の取得）**：User の API の記録では、`id` と `account.id` は JSON の数値で、停止していない Installation の `suspended_at` は欠落せず `null` で現れました。
署名の不正な App JWT には `401` が返りました（[GitHub App Installation](../research/github/app-installation.md)）。

**実測値（Installation の変更）**：個人アカウントの Installation を画面で変えた記録では、次のとおりでした（[GitHub App Installation Changes](../research/github/installation-changes.md)）。

- **停止**：停止中の Installation の User の API は `200` で、`suspended_at` は時刻だった。値そのものは記録していない
- **アンインストール**：アンインストールした後の User の API は `404` だった
- **再インストール**：同じ App を再インストールすると Installation ID が変わり、`account.id` は変わらなかった。User の API は新しい Installation を返した
- **対象の追加と削除**：保存から約 1 分後の発行の要求に反映されていた。外したリポジトリを含む要求は `422` で拒否され、対象を縮小した応答はなかった
- **App の権限の追加**：Installation の側で承認するまで、Installation の権限は変わらなかった。承認の後も、`{"contents":"read"}` を指定した発行応答の `permissions` は `contents:read` と `metadata:read` だけだった

**推論**：オーナーを指定して単一の Installation を返す API があることから、GitHub App とオーナーの組で Installation は単一に決まると読めます。ただし、この一意性を明記した公式の記述はありません。

## 6. Installation Access Token

### 6.1 発行 API の呼び出しと有効期限

**公式仕様**：GitHub App JWT で `POST /app/installations/{installation_id}/access_tokens` を呼び出すと、Installation Access Token を発行できます（[Generating an installation access token](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app)、[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)、[REST API Description](https://github.com/github/rest-api-description)）。

| 要求の要素 | GitHub が定める仕様 |
|---|---|
| `repositories` | リポジトリ**名**の配列。資料の例は `Hello-World` で、オーナー名を含まない |
| `repository_ids` | Repository ID の配列。`repositories` と並ぶ対象の指定方法 |
| 対象件数 | 一度に指定できる対象は最大500リポジトリ |
| `permissions` | 権限名から権限水準へのオブジェクト。例：`{"contents":"read"}`。スキーマ「App Permissions」では、権限名と水準（`read`、`write`、`admin`）はどれも ASCII 小文字とアンダースコアで表される。選べる水準は権限ごとに異なる（`workflows` は `write` だけ、`organization_events` は `read` だけなど） |
| 対象指定の省略 | Installation に許可された全リポジトリへのアクセスを持つ |
| Permissions 指定の省略 | App に付与された全権限を持つ |
| 付与できる範囲 | Installation に許可されていないリポジトリへのアクセスと、App に付与されていない権限は付与できない |
| 有効期間 | 発行から1時間。期限切れトークンは再発行が必要 |

**公式仕様（応答コード）**：Installation Access Token の発行 API の資料は `201`（作成）、`401`（認証が必要）、`403`（禁止）、`404`（見つからない）、`422`（検証失敗またはスパム判定）を示しています（[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)）。特に **`422` は「対象リポジトリが Installation の範囲外」だけを意味するコードではありません。**

**未確認**：個々の失敗原因と応答コードの対応は資料に示されていません。

**公式仕様（レート制限）**：REST API は、プライマリとセカンダリのどちらのレート制限でも `403` または `429` を返します。プライマリでは `x-ratelimit-remaining` が `0` になります。セカンダリでは、制限の超過を示すエラーメッセージが返り、`retry-after` が付く場合があり、`x-ratelimit-remaining` が `0` の場合の扱いも説明されています（[Rate limits for the REST API](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)）。発行 API の応答コードの一覧に `429` はありませんが、レート制限の資料は REST API 全般の応答として `429` を挙げています。

**推論**：`x-ratelimit-remaining` が `0` になる場合はプライマリとセカンダリの両方にあるため、このヘッダーだけでは両者を区別できません。

**実測値（レート制限のヘッダー）**：レート制限に達していない記録では、GitHub App JWT で認証した Installation の取得と発行 API の応答に `x-ratelimit-*` のヘッダーがありませんでした（[GitHub App Installation](../research/github/app-installation.md#installation-の取得)、[GitHub Installation Access Token](../research/github/installation-access-token.md#レート制限のヘッダー)）。

### 6.2 発行応答・Permission

**公式仕様**：発行 API の応答には `token`、`expires_at`、`permissions`、`repositories`（適用される場合）、`repository_selection` などが示されます。`repository_selection` は `all` / `selected` の区別です。応答例には `repositories[].id`、`repositories[].full_name`、`repositories[].owner.id` といったリポジトリ情報が含まれます（[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)）。

**未確認**：すべての発行で `metadata:read` が無条件に付与されるという保証は、参照した公式資料からは確定できません。

**公式仕様（トークンの形式）**：2026年4月27日に始まった Installation Access Token の新形式（`ghs_APPID_JWT`）への移行は完了しており、新規発行のトークンは既定でこの形式です。
長さは40文字から約520文字に増え、トークンに格納するデータによって変わります。
GitHub は、トークンを、中身を解釈しない文字列として扱うよう求めており、40文字固定の検証、旧形式だけに合うパターン、短い固定長の保管先、長い `Authorization` ヘッダーを切り詰めるか拒否する中継、旧形式だけに合うログの秘匿ルールを点検の対象に挙げています。
トークン内の JWT はクライアントが検証するものではなく、その内容に依存することも禁じられています。
形式を切り替える `X-GitHub-Stateless-S2S-Token` ヘッダーは2026年11月30日に廃止されます。
権限、対象リポジトリ、1時間の有効期間、発行 API は変わりません（[GitHub Changelog — Stateless installation tokens](https://github.blog/changelog/2026-10-02-stateless-github-app-installation-tokens-rolled-out/)、[GitHub Changelog — Notice about the new format](https://github.blog/changelog/2026-04-24-notice-about-upcoming-new-format-for-github-app-installation-tokens/)）。

**実測値（発行応答）**：`Only select repositories` の Installation で、対象と `{"contents":"read"}` を明示して発行した記録では、次のとおりでした（[GitHub Installation Access Token](../research/github/installation-access-token.md)）。

- **応答のフィールド**：`token`、`expires_at`、`permissions`、`repository_selection`（`selected`）、`repositories` がすべてあった
- **`permissions`**：要求していない `metadata:read` が加わった
- **`repositories`**：要求した件数のリポジトリが返った。対象 2 件の要求では、要求の並びを入れ替えても応答の並びは同じだった。`full_name` は要求の表記ではなく GitHub 上の表記だった
- **本文の大きさ**：対象1件ごとにリポジトリの完全な表現が入り、1件あたり数 KB だった
- **拒否**：Installation で選択していない同じアカウントの公開リポジトリ、存在しない名前、`.`、`..` のどれかを含む要求は、同じ本文の `422` で、どのリポジトリが原因かを示さなかった。Installation で選択していない同じアカウントの private リポジトリを含む要求も `422` だった（本文は記録していない）。対象を縮小した応答はなかった

**未確認**：次の振る舞いは、参照した公式資料に記述がありません。

- Installation 範囲外のリポジトリを要求したとき、発行を拒否するか、対象を縮小した応答を返すか。実測したのは Installation で選択していない同じアカウントの公開リポジトリと private リポジトリだけで、別のアカウントのもの、archived のものは試していない
- 500件を超えて指定したときの応答
- 停止中の Installation に対する発行 API の応答。App 停止の API の説明は、停止するとそのアカウントに対する App の API アクセスが遮断されるとするが、発行 API の応答コードは示していない（[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)）
- 発行応答の各フィールドの存在条件。OpenAPI 定義で必須なのは `token` と `expires_at` だけで、`permissions`、`repositories`、`repository_selection` は必須ではない（[REST API Description](https://github.com/github/rest-api-description)）
- 発行応答の `repositories` の並び

### 6.3 失効

**公式仕様**：`DELETE /installation/token` は、その呼び出しを認証した Installation Access Token 自身を失効させます。GitHub App JWT で呼び出す API ではありません。成功の応答は `204` です（[REST API — App installations](https://docs.github.com/en/rest/apps/installations?apiVersion=2026-03-10)）。この資料も新形式への移行の案内も、新形式のトークンでの失効の動作を個別には説明していません。

**実測値（失効）**：発行したトークンの失効は `204` を返し、失効したトークンによる要求は `401` で拒否されました（[GitHub Installation Access Token](../research/github/installation-access-token.md#失効の前後のトークン)）。

## 7. GitHub Actions Integration

**公式仕様**：GitHub Actions が OIDC JWT を取得するには、ワークフローまたはジョブで `permissions: id-token: write` を付与します。ランナーでは `ACTIONS_ID_TOKEN_REQUEST_URL` と `ACTIONS_ID_TOKEN_REQUEST_TOKEN` による取得方法も提供され、要求時に Audience を指定できます（[OIDC reference](https://docs.github.com/en/actions/reference/security/oidc#workflow-permissions-for-the-requesting-the-oidc-token)、[OIDC reference — Methods for requesting the OIDC token](https://docs.github.com/en/actions/reference/security/oidc#methods-for-requesting-the-oidc-token)）。

**公式仕様**：Composite Action では、`GITHUB_OUTPUT` へ書き込んだ値を、`outputs.<output_id>.value` で Action の Output として後続ステップへ公開できます。ログに表示させたくない値は `add-mask` でマスク対象に登録できます（[Workflow commands](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands)、[Metadata syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/metadata-syntax)）。

**公式仕様（マスクと Output）**：マスクは、ジョブごと、値ごとに登録が必要です。
同じ資料は、マスクした値は Output に設定できないと説明する一方で、同一ジョブ内なら生成、`add-mask`、`GITHUB_OUTPUT` の順で後続ステップへ渡す例を示しています。
ジョブやワークフローをまたいでマスクした値を渡す場合は、シークレットストアを経由するよう案内しています（[Workflow commands](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands)）。

**実測値（マスクと Output）**：リポジトリ内の Composite Action の内部ステップで `add-mask` した値を `GITHUB_OUTPUT` に書き、`action.yml` の `outputs` で公開した記録では、呼び出し元の同じジョブの後続ステップが値を欠けずに受け取り、その値はログで `***` にマスクされました。
呼び出し元は `add-mask` を実行していません。
`name=value` の形式と区切り文字の形式の両方で、`+`、`/`、`=` を含む64文字の値と、500文字の値が、どちらも同じ値で届きました。
同じ値をジョブの `outputs` に置くと、GitHub は「Skip output ... since it may contain secret」の警告を出して設定せず、次のジョブでは空文字列でした（[GitHub Composite Action Outputs](../research/github/composite-action-outputs.md)）。

**推論**：公式資料の「マスクした値は Output に設定できない」は、この観測と照らすと、ジョブの Output を指すと読めます。

**公式仕様**：GitHub Actions の Secrets / Variables はリポジトリ、Environment、Organization の単位で設定できます。Organization の Secrets / Variables はポリシーで利用できるリポジトリを制限でき、GitHub Free の private リポジトリからは Organization の Secrets / Variables を利用できません（[Using secrets](https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-secrets)、[Variables reference](https://docs.github.com/en/actions/reference/workflows-and-actions/variables)）。

**推論**：個人アカウントが所有するリポジトリでは Organization 単位の設定を使えないため、値はリポジトリまたは Environment の単位で設定します。

**公式仕様（Dependabot が起動したワークフロー）**：Dependabot が `push`、`pull_request`、`pull_request_review`、`pull_request_review_comment` で起動したワークフローは、既定ではフォークから開いたものと同じに扱われ、`GITHUB_TOKEN` は読み取り専用になります。`permissions` キーで権限を引き上げられます。
参照できる Secrets は Dependabot の Secrets だけで、GitHub Actions の Secrets は参照できません。
Dependabot の Secrets は `secrets` コンテキストに同じ書き方で入ります（[Troubleshooting Dependabot on GitHub Actions](https://docs.github.com/en/code-security/reference/supply-chain-security/troubleshoot-dependabot/dependabot-on-actions)）。

**推論**：Dependabot が起動したワークフローでも、Variables は参照できます。上記の資料が参照を制限するのは Secrets と `GITHUB_TOKEN` の権限で、Variables は制限の対象に挙げていないためです。

**公式仕様（Dependabot の `github-actions` の対象）**：`package-ecosystem: github-actions` の `directory` に `/` を指定すると、Dependabot は `/.github/workflows` と、ルートの `action.yml` / `action.yaml` を探します。
`directories` はグロブとワイルドカード `*` を受け付け、`directory` は受け付けません（[Dependabot options reference](https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference)）。

**公式仕様（Docker Action）**：Docker Action は Linux の Runner だけで動き、セルフホストの Runner には Linux と Docker が要ります（[About custom actions](https://docs.github.com/en/actions/concepts/workflows-and-actions/custom-actions)）。
`runs.image` に `Dockerfile` を指定すると、Runner がそのファイルからイメージをビルドします。パスは `action.yml` からの相対パスで、ファイル名は `Dockerfile` でなければなりません。
入力は `INPUT_<大文字の入力名>` の環境変数で渡され、空白だけが `_` に変わります。
`runs.args` はコンテナの `ENTRYPOINT` に渡す引数で、`Dockerfile` の `CMD` を置き換え、`runs.entrypoint` は `ENTRYPOINT` を置き換えます。
`runs.post-entrypoint` は、`runs.entrypoint` の後に同じイメージの新しいコンテナで動き、状態はワークスペース、`HOME`、`STATE_` の環境変数で受け渡します。`runs.post-if` の既定は `always()` です（[Metadata syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/metadata-syntax)）。
`GITHUB_STATE` に書いた値は、同じ Action の `pre:` または `post:` で `STATE_<名前>` の環境変数として読めます（[Workflow commands](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands)）。
Docker Action は既定の Docker ユーザー（root）で動かす必要があり、`USER` を使うと `GITHUB_WORKSPACE` にアクセスできなくなります。
`WORKDIR` は使わず、`ENTRYPOINT` は exec 形式と絶対パスで書くよう推奨しています。exec 形式では `args` がシェルを通らず、環境変数が展開されません（[Dockerfile support for GitHub Actions](https://docs.github.com/en/actions/reference/workflows-and-actions/dockerfile-support)）。

**実測値（Docker Action）**：リポジトリ内の Docker Action を `args` なしで動かした記録では、`role-arn` の入力は `INPUT_ROLE-ARN` の環境変数で届き、ハイフンは残りました。
`id-token: write` のあるジョブでは `ACTIONS_ID_TOKEN_REQUEST_URL` と `ACTIONS_ID_TOKEN_REQUEST_TOKEN` がコンテナに渡り、コンテナの中から `sts.amazonaws.com` を Audience とする OIDC JWT を取得できました。ないジョブでは両方とも渡りませんでした。
`ACTIONS_RUNTIME_TOKEN` はどちらのジョブでも渡りました。
コンテナには `/var/run/docker.sock`、`RUNNER_TEMP`、`HOME`、ファイルコマンドのディレクトリ、ワークスペースがマウントされ、`docker run` に `--user` は付きませんでした。
`USER 65534:65534` のイメージでは、`GITHUB_OUTPUT`、`GITHUB_STATE`、`GITHUB_ENV` に書き込めず（所有者は Runner のユーザーで、ほかのユーザーに書き込み権限がない）、標準出力のワークフローコマンドだけが効きました。
root では、メインで `add-mask` した値が `GITHUB_OUTPUT` を経て呼び出し元の後続ステップに欠けずに届いてマスクされ、`GITHUB_STATE` を経て post-entrypoint に `STATE_` の環境変数として届いてマスクされました。
メインが終了コード 1 で終えたステップ（`continue-on-error: true`）でも、post-entrypoint は動きました（[GitHub Docker Action Runtime](../research/github/docker-action-runtime.md)）。

**実測値（失敗とキャンセル）**：`continue-on-error` を付けないメインの失敗でジョブが失敗した場合、後続のステップの失敗でジョブが失敗した場合、後続のステップの実行中にワークフロー実行をキャンセルした場合のどれでも、post-entrypoint は動きました。
ワークフロー実行のキャンセルの API を呼んでから Runner がステップを終えるまでに、12〜13 秒かかりました。
`docker run` のプロセスに送った SIGINT は、コンテナの中のプロセスに届きました（[GitHub Docker Action Failure and Cancellation](../research/github/docker-action-cancellation.md)）。

**推論（Docker Action の実行ユーザー）**：公式資料が `USER` を禁じる理由に挙げるのは `GITHUB_WORKSPACE` だけですが、観測ではファイルコマンドにも書き込めませんでした。ファイルコマンドの所有者は Runner を動かすユーザーで、その uid は Runner の環境ごとに決まるため、Action のイメージが固定の uid で書き込める保証はありません。Output と `STATE_` を使う Docker Action は、root で動かす前提になります。

**公式仕様（Dependabot の `docker`）**：`package-ecosystem: docker` は `Dockerfile` のイメージのタグを更新します（[Dependabot options reference](https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference)、[Dependabot supported ecosystems and repositories](https://docs.github.com/en/code-security/reference/supply-chain-security/supported-ecosystems-and-repositories)）。
docs.github.com の資料は、`FROM` の digest を更新するとは明記していません。

## 8. Boundaries with JWT and OpenID Connect Standards

GitHub が発行または受理する JWT には、[JWT Constraints](jwt-constraints.md) の規定がすべて同じように適用されるわけではありません。
GitHub Actions OIDC JWT は通常の OpenID Connect の認証フローとは異なる方法で発行され、GitHub App JWT は GitHub API の認証に使うためです。
この節の評価はすべて、公式資料、公開値、実測値からの推論です。

### 8.1 GitHub Actions OIDC と OpenID Connect の相違

| 項目 | 標準仕様 | GitHub の仕様・公開値 | 評価 |
|---|---|---|---|
| `authorization_endpoint` | Discovery で **REQUIRED** | Discovery の公開値にない | Discovery が定める Metadata の要件を満たさない。通常の認証フローを実装するクライアントをそのまま適用できる保証はない |
| `token_endpoint` | Implicit Flow だけを使う場合を除き **REQUIRED** | Discovery の公開値にない。`response_types_supported` は `id_token` だけ | この欠落だけでは標準仕様への違反と断定できない |
| `sub` の再割当て | OpenID Connect Core は再割当てを禁止 | 旧形式では、名前の再利用で別のオーナーが同じ `sub` の JWT を GitHub に発行させることができる。不変形式は Owner ID と Repository ID を含む | 旧形式は再割当ての禁止を満たせない。不変形式は対策だが、旧形式のリポジトリも残る |
| `aud` と認証フロー | ID Token の `aud` は登録済みの `client_id` を含む。`nonce` は認証要求で送った場合に照合する | ワークフローが認証要求を介さずに JWT を取得し、Audience を指定する。既定値はオーナーの URL | `client_id` と `nonce` の検証を機械的に適用することは適切でない |

これらの相違は、署名の検証、Issuer と検証鍵の結び付け、`aud` の照合を省略する根拠にはなりません。
これらの検証は ID Token に限らない JWT の規則に基づくためです（[JWT Constraints — OpenID Connect and Discovery](jwt-constraints.md#5-openid-connect-and-discovery)、[JWT Constraints — JWT Security Considerations](jwt-constraints.md#6-jwt-security-considerations)）。

JWT / JWS / JWK の基本規則については、参照した公式資料、公開値、実測値の範囲では、GitHub との矛盾は見つかっていません。
実測値は記録した条件の JWT に限られ、確定していない事項は [Outstanding Verification](#9-outstanding-verification) に記録します。

### 8.2 GitHub App JWT と ID Token の相違

GitHub App JWT は、GitHub API の認証のために GitHub App 側が署名する JWT であり、OpenID Connect の ID Token として扱いません。
したがって、OpenID Connect Core が ID Token に求める `sub` と `aud` の必須条件や、`iss` を HTTPS URL とする条件は適用されません。
Client ID または App ID を `iss` にすることは、`iss` を文字列とする JWT 一般の規定と矛盾しません。
`RS256` の指定と `exp` の上限10分は、JWT の標準仕様ではなく GitHub が追加する条件です（[GitHub App JWT](#42-github-app-jwt)）。

## 9. Outstanding Verification

公式資料で確定できていない事項を次にまとめます。根拠と確認範囲は各セクションに記載しています。
公開値や実測値で一部の条件を確かめた事項も、GitHub の保証がないためここに残します。

| 項目 | 確定できていない内容 |
|---|---|
| オーナー名 | 一般の `User` / Organization の命名規則と適用上の例外 |
| リポジトリ名 | `.` と `..` の扱い、既存リポジトリへの例外的な命名規則 |
| OIDC | カスタマイズ後の `sub` の形式（`repo` を含めないテンプレートを不変形式のリポジトリに適用した場合を含む）、`sub` が OpenID Connect Core の上限255 ASCII 文字に常に収まるか、Claim の JSON の型、Claim が欠落する条件、`nbf`、`iat`、`exp` の時刻の差、JOSE Header に `crit`、`jku`、`jwk` などが現れるか、`User` 所有のリポジトリでの `repository_owner_id`、`run_attempt` の初回の値、Organization が所有するリポジトリで発行された JWT の実測値、指定した Audience の発行を拒否する規則（実測した値の外での振る舞い） |
| GitHub App の鍵 | 新規生成される RSA 鍵のビット長 |
| Installation の取得 API | Organization の API の `200` 以外の応答（インストールされていない場合を含む）、User の API のアンインストール以外の `200` 以外の応答、Organization の API の応答の形、停止中の Installation の `suspended_at` の値の形、GitHub App とオーナーの組での一意性 |
| トークン発行 API | 範囲外リポジトリへの要求、500件超の要求、停止中の Installation への要求、`422` の原因、`metadata:read` の付与条件、応答フィールドの有無、`repositories` の並び |
| トークン失効 API | 新形式のトークンでの失効の動作 |
| Docker Action | ジョブが失敗した状態とキャンセルした状態での post-entrypoint の実行、複数の Action の post が動く順序、別のリポジトリを `uses:` で参照したときのビルドの時間と post-entrypoint でのビルドのし直し、Dependabot の `docker` が `FROM` のタグと並べた digest を更新するか |
