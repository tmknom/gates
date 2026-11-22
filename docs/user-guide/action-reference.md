# Action Reference

ワークフロー利用者と運用者に向けて、Action の入力、要求できる権限、出力、トークンを失効させる時機を示すリファレンスです。

## 1. Inputs

すべて必須です。

| 入力 | 意味 |
|---|---|
| `repositories` | トークンで操作するリポジトリ。カンマ区切りの `owner/repo` |
| `permissions` | トークンの権限。カンマ区切りの `権限名:水準`（例：`contents:read,pull_requests:write`）。書ける組は「要求できる権限」にある |
| `role-arn` | 引き受ける IAM Role の ARN |
| `function-name` | GATES の Lambda の関数名 |
| `aws-region` | GATES の Lambda の AWS リージョン |
| `audience` | GATES に送る OIDC JWT の Audience |

### 1.1 List Syntax

`repositories` と `permissions` の要素の前後の空白は除かれ、大文字小文字はそのまま GATES に送られます。
次のリストは、GATES に送る前に `invalid inputs` の注釈で失敗します。

- 空の要素がある
- `repositories` の要素が `owner/repo` の形でない
- `permissions` の要素が `権限名:水準` の形でない
- 同じリポジトリ（大文字小文字を区別しない）か、同じ権限名が 2 つ以上ある
- UTF-8 として正しくない

### 1.2 Conditions for Issuance

GATES がトークンを発行するのは、要素がどれも次の条件を満たすときだけです。
**一つでも満たさない要素があると要求全体が失敗し、許可される要素だけのトークンも返りません。**

- **`repositories`**：許可オーナーのリポジトリで、インストール対象に含まれる
- **`permissions`**：「要求できる権限」の組で、GitHub App に付与され、発行上限が許す。トークンは要求した水準で発行される
- **保護対象を含むとき**：ワークフローを置くリポジトリがその保護対象の許可 Caller に当たり、要求の権限がどれも権限の上限にある。保護対象かどうかと権限の上限は、運用者に確かめる

## 2. Requestable Permissions

GitHub App の Repository permissions に当たる、次の権限名と水準の組です。
Organization permissions（`members` など）と Account permissions（`emails` など）は、GitHub App に付与されていても要求できません。

| 水準 | 権限名 |
|---|---|
| `read`、`write` | `actions`、`actions_variables`、`administration`、`agent_secrets`、`agent_tasks`、`agent_variables`、`artifact_metadata`、`attestations`、`checks`、`code_quality`、`codespaces`、`codespaces_lifecycle_admin`、`codespaces_secrets`、`contents`、`copilot_agent_settings`、`dependabot_secrets`、`deployments`、`discussions`、`environments`、`installation_repositories`、`issues`、`license_compliance_alerts`、`merge_queues`、`packages`、`pages`、`pull_requests`、`repo_secret_scanning_dismissal_requests`、`repository_advisories`、`repository_creation`、`repository_custom_properties`、`repository_hooks`、`secret_scanning_alerts`、`secret_scanning_bypass_requests`、`secrets`、`security_events`、`single_file`、`statuses`、`vulnerability_alerts`、`workflows` |
| `read`、`write`、`admin` | `repository_projects` |
| `read` | `codespaces_metadata`、`metadata`、`repository_license_policy` |

- **権限名は表の名前で書く**：GitHub App の設定画面の表示名（`Contents`、`Pull requests`、`Commit statuses` など）ではなく、`contents`、`pull_requests`、`statuses` のように書く。表示名と権限名は、`Commit statuses` と `statuses` のように一致しないことがある
- **水準は付与された水準以下にする**：GitHub App に `Read-only` で付与された権限の `write` は、GitHub に拒否される
- **`single_file` のファイルは選べない**：読み書きできるファイルのパスは GitHub App の設定で決まり、要求では指定できない
- **`metadata` は書かなくてよい**：GitHub が `metadata:read` を加えることがある。明示して要求するときは、ほかの権限と同じく発行上限が許している必要がある

## 3. Outputs

| 出力 | 意味 |
|---|---|
| `token` | トークン。ログではマスクされる |
| `expires-at` | トークンの有効期限（RFC 3339）。発行から 1 時間 |
| `repositories` | トークンで操作できるリポジトリ。カンマ区切りの `owner/repo` |
| `permissions` | トークンの権限。カンマ区切りの `権限名:水準`。要求した権限に加えて、GitHub が加える `metadata:read` を含むことがある |

## 4. Token Revocation

ジョブの終了時に、Runner は `Post <ステップ名>` のステップを動かし、Action はトークンの失効を試みます。
後続のステップが失敗した場合や、ジョブがキャンセルされた場合も試みます。
Action がトークンを受け取った後に失敗かキャンセルで終えるときは、その場で失効を試みます。
失効に失敗しても、Action は `::warning::` の注釈を書くだけで、ジョブを失敗させません。

## 5. References

- [GitHub — Permissions required for GitHub Apps](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps)
