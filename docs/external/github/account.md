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

## 3. Repository ID

**Documented（改名）**：OIDC JWT の `repository_id` を含めるテンプレートの例は、`repository_id` をリポジトリの改名で変わらない識別子と説明しています（[OIDC reference — Example: Using system-generated GUIDs](https://docs.github.com/en/actions/reference/security/oidc#example-using-system-generated-guids)）。

**Documented（不変形式の `sub`）**：不変形式の `sub` を告知した資料は、`sub` に加える Owner ID と Repository ID を「immutable owner and repository IDs」と呼び、この形式で各 `sub` が元のリポジトリに永続的に結び付くとしています。
導入の理由は、名前だけの `sub` では、名前が再利用されると別のオーナーが同じ値の `sub` を作れることです（[GitHub Changelog — Immutable subject claims](https://github.blog/changelog/2026-04-23-immutable-subject-claims-for-github-actions-oidc-tokens/)、[OIDC reference — Immutable subject claims](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims)）。

**Documented（旧名の転送）**：リポジトリを改名すると、Web の参照と `git clone`、`git fetch`、`git push` は新しい名前に転送されます。
旧名で新しいリポジトリを作ると、この転送は効かなくなります（[Renaming a repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/renaming-a-repository)）。
`GET /repos/{owner}/{repo}` の資料は、応答コードに `301` を挙げています（[REST API — Repositories](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10)）。

**Observed**：個人アカウントの private リポジトリで、次のとおりでした（[GitHub Repository Identity](../../research/github/repository-identity.md#改名と作り直しでの-repository-id)）。

- **改名**：改名の前と後で、REST API の `id` は変わらなかった。Installation の対象のリポジトリでも同じだった
- **旧名**：改名の後に旧名を指定した `GET /repos/{owner}/{repo}` は、改名したリポジトリを返した。`301` が返ったかは記録していない
- **作り直し**：削除して同じ名前で作り直したリポジトリの `id` は、削除したリポジトリの `id` と異なった。削除したリポジトリの `id` を指定した `GET /repositories/{id}` は、作り直した後も `404` だった

**Inferred**：資料が Repository ID を「immutable」と呼び、名前の再利用への対策に使うことから、GitHub は Repository ID を、改名で変えず、ほかのリポジトリに割り当てない識別子として扱っていると読めます。

**Unconfirmed**：Repository ID がほかのリポジトリに再び割り当てられないこと、移管で変わらないこと、削除したリポジトリを復元したときに元の ID に戻ることは、参照した資料に明記されていません。
移管の資料（[Transferring a repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/transferring-a-repository)）と、削除と復元の資料（[Deleting a repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/deleting-a-repository)、[Restoring a deleted repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/restoring-a-deleted-repository)）も、ID に触れていません。

## 4. Outstanding Verification

公式資料で確定できていない事項を次にまとめます。根拠と確認範囲は各セクションに記載しています。
Published や Observed の記述で一部の条件を確かめた事項も、GitHub の保証がないためここに残します。

| 項目 | 確定できていない内容 |
|---|---|
| オーナー名 | 一般の `User` / Organization の命名規則と適用上の例外 |
| リポジトリ名 | `.` と `..` の扱い、既存リポジトリへの例外的な命名規則 |
| Repository ID | ほかのリポジトリに再び割り当てられないこと、移管で変わらないこと、復元したリポジトリの ID |
