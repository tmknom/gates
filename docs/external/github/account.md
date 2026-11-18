# GitHub Owner and Repository Constraints

この文書は、GitHub.com のオーナーとリポジトリの識別子と名前の仕様を、GitHub の公式資料を根拠に整理したものです。
GitHub の公式資料そのものではなく、GitHub の仕様が変更された場合は該当する公式資料を確認します。

- **対象**：GitHub.com の個人アカウント（`User`）と Organization、リポジトリの識別子と名前。GitHub Enterprise Server は対象外。

各記述の先頭の太字は、記述の根拠の Evidence を表します。Documented は公式資料の記述、Published は GitHub が公開するエンドポイントから取得した値、Observed は実環境で観測した値、Inferred はそれらから導いた解釈、Unconfirmed は公式資料で確定できない事項です。

## 1. オーナーと識別子

**Documented**：GitHub のリポジトリは個人アカウント（`User`）または Organization に所属します。REST API のリポジトリ表現には `id`、`full_name`、`owner` があり、オーナーの表現には `login`、`id`、`type` が含まれます。`full_name` は `owner/repo` 形式です。OIDC JWT にも `repository_id` と `repository_owner_id` という ID が用意されています（[Creating a new repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/creating-a-new-repository)、[OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)、[Repositories API](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10)）。

**Documented（名前の大文字小文字）**：REST API のパスパラメーターの説明は、`org`、`owner`、`repo` の名前は大文字小文字を区別しないとしています。
`username` の説明にはこの記述がありません（[REST API — GitHub Apps](https://docs.github.com/en/rest/apps/apps?apiVersion=2026-03-10)）。

## 2. 名前の制約

**Documented（リポジトリ名）**：新規リポジトリ名の長さは最大100文字、許容される文字は ASCII 英数字と `.`, `-`, `_` です（[Creating a new repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/creating-a-new-repository)）。

**Documented（Enterprise Managed Users）**：GitHub.com の Enterprise Managed Users のユーザー名には、GitHub が末尾にアンダースコアと企業のショートコード（3〜8文字の英数字）を加えます。アンダースコアとショートコードを含めて最大39文字です。正規化したユーザー名は、ダッシュで始まることも終わることもできず、連続したダッシュも含められません（[Username considerations for external authentication](https://docs.github.com/en/enterprise-cloud@latest/admin/managing-iam/iam-configuration-reference/username-considerations-for-external-authentication)）。このアカウントは、企業のポリシーで許可されていれば private リポジトリに限り所有できます。特権アプリを除き、GitHub App を自身の個人アカウントにはインストールできず、管理者権限を持つリポジトリ単位のインストールは条件付きで可能です。所有するリポジトリでは GitHub-hosted runner を利用できません（[Abilities and restrictions of managed user accounts](https://docs.github.com/en/enterprise-cloud@latest/admin/managing-iam/understanding-iam-for-enterprises/abilities-and-restrictions-of-managed-user-accounts)）。

**Unconfirmed**：リポジトリ名の資料だけからは、名前が `.` または `..` である場合の扱いや、既存リポジトリへの例外的な命名規則を断定できません。
Enterprise Managed Users の資料には、GitHub のユーザー名は英数字とダッシュ（`-`）だけを含められるという記述があります。
しかし同じ資料が Enterprise Managed Users のユーザー名にアンダースコアを加えると定めるため、この記述をすべてのユーザー名に当てはまる規則としては扱いません。
Organization 名の規則もこの資料の対象外であり、`User` と Organization に一様に適用できる文字種・最大長・ハイフンの位置の規則は確定できていません。

## 3. Outstanding Verification

公式資料で確定できていない事項を次にまとめます。根拠と確認範囲は各セクションに記載しています。
Published や Observed の記述で一部の条件を確かめた事項も、GitHub の保証がないためここに残します。

| 項目 | 確定できていない内容 |
|---|---|
| オーナー名 | 一般の `User` / Organization の命名規則と適用上の例外 |
| リポジトリ名 | `.` と `..` の扱い、既存リポジトリへの例外的な命名規則 |
