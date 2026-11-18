# GitHub App Constraints

この文書は、GitHub.com の GitHub App、Installation、Installation Access Token と、それらを扱う GitHub REST API の仕様を、GitHub の公式資料と、GitHub から取得した値の調査記録を根拠に整理したものです。
GitHub の公式資料そのものではなく、GitHub の仕様が変更された場合は該当する公式資料を確認します。

- **対象**：GitHub.com の GitHub App、GitHub App JWT、Installation、Installation Access Token、REST API の共通事項。GitHub Enterprise Server は対象外。
- **取得した値の調査記録**：[GitHub App Installation](../../research/github/app-installation.md)、[GitHub Installation Access Token](../../research/github/installation-access-token.md)、[GitHub App Installation Changes](../../research/github/installation-changes.md)、[GitHub Docker Action Failure and Cancellation](../../research/github/docker-action-cancellation.md)。取得日、取得方法、記録した応答はこれらの記録にある。

各記述の先頭の太字は、記述の根拠の Evidence を表します。Documented は公式資料の記述、Published は GitHub が公開するエンドポイントから取得した値、Observed は実環境で観測した値、Inferred はそれらから導いた解釈、Unconfirmed は公式資料で確定できない事項です。

## 1. REST API の共通事項

- **REST API の確認対象版**：`2026-03-10`。以下の REST API 資料と OpenAPI 定義（REST API Description）はこの版を参照する。
- **バージョン指定の仕様**：REST API は `X-GitHub-Api-Version` で版を指定する。ヘッダーを省略すると `2022-11-28` が使われる。各版は新しい版の公開から24か月サポートされ、`2022-11-28` のサポートは2028年3月10日に終わる。サポート終了後、その版を指定した要求は `410 Gone` になり、ヘッダーを省略した要求の既定の版は変わる。版に依存する記述は、別の版では照合が必要である（[API Versions](https://docs.github.com/en/rest/about-the-rest-api/api-versions)）。
- **`User-Agent` の仕様**：REST API のすべての要求は、有効な `User-Agent` ヘッダーを含む必要がある。ヘッダーのない要求は拒否され、無効な値には `403 Forbidden` が返る。値には、GitHub のユーザー名かアプリケーションの名前が推奨される（[Getting started with the REST API](https://docs.github.com/en/rest/using-the-rest-api/getting-started-with-the-rest-api)）。

## 2. GitHub App and App JWT

### 2.1 GitHub App の識別と Private Key

**Documented**：GitHub App には App ID と Client ID があり、両者は異なります。GitHub App JWT の `iss` にはいずれも使用できますが、Client ID が推奨されています（[Generating a JWT for a GitHub App](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app)、[Generating an installation access token](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app)）。

**Documented**：GitHub から取得する Private Key は PEM 形式で、PKCS#1 `RSAPrivateKey` です。1 App につき最大25個を保持でき、Private Key は自動失効せず手動で失効させます。GitHub が保存するのは鍵の公開部分だけです。Fingerprint は Private Key から導出した公開鍵 DER の SHA-256 の Base64 表現で照合できます（[Managing private keys](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps)）。

**Documented**：Private Key を削除するとき、GitHub App に鍵が一つしかなければ、**新しい鍵を生成してから** 既存の鍵を削除する必要があります（同資料の「Deleting private keys」）。

**Inferred**：GitHub が保存するのは公開部分だけであるため、ダウンロード済みの Private Key の内容は GitHub から再取得できません。

**Unconfirmed**：GitHub が新規生成する RSA 鍵のビット長を常に特定の値とする保証は、参照した公式資料から確認できません。

### 2.2 GitHub App JWT

**Documented**：GitHub App として認証する JWT は `RS256` で署名し、`iat`、`exp`、`iss` を含めます。`exp` は現在時刻から最大10分先までです。`iat` を60秒過去にすることは GitHub の推奨であり、必須ではありません。JWT を GitHub API に渡すときは `Authorization: Bearer` を使用します（[Generating a JWT for a GitHub App](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app)）。

## 3. Installation

**Documented**：GitHub App は個人アカウントまたは Organization にインストールできます。同じ App を複数のアカウントにインストールすることもできます。App のインストール時、リポジトリ権限を要求する App では `All repositories` または `Only select repositories` を指定できます（[Installing a GitHub App](https://docs.github.com/en/apps/using-github-apps/installing-a-github-app-from-a-third-party)）。

**Inferred**：同じ App を複数のアカウントにインストールできるため、**App 自体の登録所有者と、Installation のインストール先アカウントは別の概念** です。

**Documented（対象リポジトリの変更）**：Organization オーナーがインストール済みの App について、リポジトリ管理者は、App の権限にかかわらず、自身が管理するリポジトリを Installation の対象に追加できます。
Organization オーナーがリポジトリ管理者によるインストールを制限すると、この追加はできません（[Installing a GitHub App](https://docs.github.com/en/apps/using-github-apps/installing-a-github-app-from-a-third-party)）。
また、リポジトリに対する `installation_repositories:write` を持つ呼び出し元は、そのリポジトリを Organization 内の任意の Installation に追加または削除できます。呼び出し元がユーザーの場合は、Organization オーナーまたはそのリポジトリの管理者である必要があります（[GitHub Changelog — GitHub App improvements](https://github.blog/changelog/2026-10-13-github-app-improvements-for-least-privilege-and-management/)）。

**Documented（公開リポジトリとトークンの対象範囲）**：GitHub のインストール手順は、GitHub App が GitHub 上のすべての公開リポジトリに少なくとも読み取りアクセスを持つと説明しています（[Installing a GitHub App](https://docs.github.com/en/apps/using-github-apps/installing-a-github-app-from-a-third-party#installing-a-github-app)）。一方、Installation Access Token の発行 API は、Installation に許可されていないリポジトリへのアクセスをトークンに付与できないと規定しています（[Generating an installation access token](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app)）。

**Inferred**：**App が公開リポジトリを読み取れることと、特定の Installation Access Token の対象に指定できることは別です。**

**Observed（選択対象外の公開リポジトリ）**：インストール先のアカウントが所有し、Installation で選択していない公開リポジトリを含む発行の要求は、`422` で拒否されました（[GitHub Installation Access Token](../../research/github/installation-access-token.md#発行が拒否された要求)）。
一方、対象を選択リポジトリに限って発行したトークンでは、その公開リポジトリの内容を読めました。
同じトークンで、要求していない選択リポジトリと選択対象外の private リポジトリの内容は `404` でした（[GitHub Installation Access Token](../../research/github/installation-access-token.md#発行したトークンで呼んだ-api)）。

**Documented**：GitHub App JWT で認証すると、次の API から Installation 情報を取得できます（[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)）。資料に示された応答コードは、Organization と User の API が `200` だけ、Repository の API が `200`、`301`、`404` です。

| 対象 | API |
|---|---|
| Organization | `GET /orgs/{org}/installation` |
| User | `GET /users/{username}/installation` |
| Repository | `GET /repos/{owner}/{repo}/installation` |

資料の応答例には `id`（Installation ID）、`account.id`、`account.type`、`target_type`、`repository_selection`（`all` / `selected`）、`permissions`、`suspended_at` などが含まれます。OpenAPI 定義では、`account` は `null`、`User` の表現、Enterprise の表現のいずれかで、Enterprise の表現には `type` がありません。`suspended_at` も `null` を取り得ます（[REST API Description](https://github.com/github/rest-api-description)）。

**Observed（Installation の取得）**：User の API の記録では、`id` と `account.id` は JSON の数値で、停止していない Installation の `suspended_at` は欠落せず `null` で現れました。
署名の不正な App JWT には `401` が返りました（[GitHub App Installation](../../research/github/app-installation.md)）。

**Observed（Installation の変更）**：個人アカウントの Installation を画面で変えた記録では、次のとおりでした（[GitHub App Installation Changes](../../research/github/installation-changes.md)）。

- **停止**：停止中の Installation の User の API は `200` で、`suspended_at` は時刻だった。値そのものは記録していない
- **アンインストール**：アンインストールした後の User の API は `404` だった
- **再インストール**：同じ App を再インストールすると Installation ID が変わり、`account.id` は変わらなかった。User の API は新しい Installation を返した
- **対象の追加と削除**：保存から約 1 分後の発行の要求に反映されていた。外したリポジトリを含む要求は `422` で拒否され、対象を縮小した応答はなかった
- **App の権限の追加**：Installation の側で承認するまで、Installation の権限は変わらなかった。承認の後も、`{"contents":"read"}` を指定した発行応答の `permissions` は `contents:read` と `metadata:read` だけだった

**Inferred**：オーナーを指定して単一の Installation を返す API があることから、GitHub App とオーナーの組で Installation は単一に決まると読めます。ただし、この一意性を明記した公式の記述はありません。

## 4. Installation Access Token

### 4.1 発行 API の呼び出しと有効期限

**Documented**：GitHub App JWT で `POST /app/installations/{installation_id}/access_tokens` を呼び出すと、Installation Access Token を発行できます（[Generating an installation access token](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app)、[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)、[REST API Description](https://github.com/github/rest-api-description)）。

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

**Documented（応答コード）**：Installation Access Token の発行 API の資料は `201`（作成）、`401`（認証が必要）、`403`（禁止）、`404`（見つからない）、`422`（検証失敗またはスパム判定）を示しています（[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)）。特に **`422` は「対象リポジトリが Installation の範囲外」だけを意味するコードではありません。**

**Unconfirmed**：個々の失敗原因と応答コードの対応は資料に示されていません。

**Documented（レート制限）**：REST API は、プライマリとセカンダリのどちらのレート制限でも `403` または `429` を返します。プライマリでは `x-ratelimit-remaining` が `0` になります。セカンダリでは、制限の超過を示すエラーメッセージが返り、`retry-after` が付く場合があり、`x-ratelimit-remaining` が `0` の場合の扱いも説明されています（[Rate limits for the REST API](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)）。発行 API の応答コードの一覧に `429` はありませんが、レート制限の資料は REST API 全般の応答として `429` を挙げています。

**Inferred**：`x-ratelimit-remaining` が `0` になる場合はプライマリとセカンダリの両方にあるため、このヘッダーだけでは両者を区別できません。

**Observed（レート制限のヘッダー）**：レート制限に達していない記録では、GitHub App JWT で認証した Installation の取得と発行 API の応答に `x-ratelimit-*` のヘッダーがありませんでした（[GitHub App Installation](../../research/github/app-installation.md#installation-の取得)、[GitHub Installation Access Token](../../research/github/installation-access-token.md#レート制限のヘッダー)）。

### 4.2 発行応答・Permission

**Documented**：発行 API の応答には `token`、`expires_at`、`permissions`、`repositories`（適用される場合）、`repository_selection` などが示されます。`repository_selection` は `all` / `selected` の区別です。応答例には `repositories[].id`、`repositories[].full_name`、`repositories[].owner.id` といったリポジトリ情報が含まれます（[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)）。

**Unconfirmed**：すべての発行で `metadata:read` が無条件に付与されるという保証は、参照した公式資料からは確定できません。

**Documented（トークンの形式）**：2026年4月27日に始まった Installation Access Token の新形式（`ghs_APPID_JWT`）への移行は完了しており、新規発行のトークンは既定でこの形式です。
長さは40文字から約520文字に増え、トークンに格納するデータによって変わります。
GitHub は、トークンを、中身を解釈しない文字列として扱うよう求めており、40文字固定の検証、旧形式だけに合うパターン、短い固定長の保管先、長い `Authorization` ヘッダーを切り詰めるか拒否する中継、旧形式だけに合うログの秘匿ルールを点検の対象に挙げています。
トークン内の JWT はクライアントが検証するものではなく、その内容に依存することも禁じられています。
形式を切り替える `X-GitHub-Stateless-S2S-Token` ヘッダーは2026年11月30日に廃止されます。
権限、対象リポジトリ、1時間の有効期間、発行 API は変わりません（[GitHub Changelog — Stateless installation tokens](https://github.blog/changelog/2026-10-02-stateless-github-app-installation-tokens-rolled-out/)、[GitHub Changelog — Notice about the new format](https://github.blog/changelog/2026-04-24-notice-about-upcoming-new-format-for-github-app-installation-tokens/)）。

**Observed（発行応答）**：`Only select repositories` の Installation で、対象と `{"contents":"read"}` を明示して発行した記録では、次のとおりでした（[GitHub Installation Access Token](../../research/github/installation-access-token.md)）。

- **応答のフィールド**：`token`、`expires_at`、`permissions`、`repository_selection`（`selected`）、`repositories` がすべてあった
- **`permissions`**：要求していない `metadata:read` が加わった
- **`repositories`**：要求した件数のリポジトリが返った。対象 2 件の要求では、要求の並びを入れ替えても応答の並びは同じだった。`full_name` は要求の表記ではなく GitHub 上の表記だった
- **本文の大きさ**：対象1件ごとにリポジトリの完全な表現が入り、1件あたり数 KB だった
- **拒否**：Installation で選択していない同じアカウントの公開リポジトリ、存在しない名前、`.`、`..` のどれかを含む要求は、同じ本文の `422` で、どのリポジトリが原因かを示さなかった。Installation で選択していない同じアカウントの private リポジトリを含む要求も `422` だった（本文は記録していない）。対象を縮小した応答はなかった

**Unconfirmed**：次の振る舞いは、参照した公式資料に記述がありません。

- Installation 範囲外のリポジトリを要求したとき、発行を拒否するか、対象を縮小した応答を返すか。実測したのは Installation で選択していない同じアカウントの公開リポジトリと private リポジトリだけで、別のアカウントのもの、archived のものは試していない
- 500件を超えて指定したときの応答
- 停止中の Installation に対する発行 API の応答。App 停止の API の説明は、停止するとそのアカウントに対する App の API アクセスが遮断されるとするが、発行 API の応答コードは示していない（[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)）
- 発行応答の各フィールドの存在条件。OpenAPI 定義で必須なのは `token` と `expires_at` だけで、`permissions`、`repositories`、`repository_selection` は必須ではない（[REST API Description](https://github.com/github/rest-api-description)）
- 発行応答の `repositories` の並び

### 4.3 失効

**Documented**：`DELETE /installation/token` は、その呼び出しを認証した Installation Access Token 自身を失効させます。GitHub App JWT で呼び出す API ではありません。成功の応答は `204` です（[REST API — App installations](https://docs.github.com/en/rest/apps/installations?apiVersion=2026-03-10)）。この資料も新形式への移行の案内も、新形式のトークンでの失効の動作を個別には説明していません。

**Observed（失効）**：発行したトークンの失効は `204` を返し、失効したトークンによる要求は `401` で拒否されました（[GitHub Installation Access Token](../../research/github/installation-access-token.md#失効の前後のトークン)）。ただし、`204` の約 0.4 秒後の要求が `200` で通った事例が 1 回あり、失効が直ちにすべての要求に効くとは限りません。公式資料は、失効が効くまでの時間を記載していません（[GitHub Docker Action Failure and Cancellation](../../research/github/docker-action-cancellation.md#revocation-not-yet-in-effect)）。

## 5. Outstanding Verification

公式資料で確定できていない事項を次にまとめます。根拠と確認範囲は各セクションに記載しています。
Published や Observed の記述で一部の条件を確かめた事項も、GitHub の保証がないためここに残します。

| 項目 | 確定できていない内容 |
|---|---|
| GitHub App の鍵 | 新規生成される RSA 鍵のビット長 |
| Installation の取得 API | Organization の API の `200` 以外の応答（インストールされていない場合を含む）、User の API のアンインストール以外の `200` 以外の応答、Organization の API の応答の形、停止中の Installation の `suspended_at` の値の形、GitHub App とオーナーの組での一意性 |
| トークン発行 API | 範囲外リポジトリへの要求、500件超の要求、停止中の Installation への要求、`422` の原因、`metadata:read` の付与条件、応答フィールドの有無、`repositories` の並び |
| トークン失効 API | 新形式のトークンでの失効の動作、`204` の後に失効が効くまでの時間 |
