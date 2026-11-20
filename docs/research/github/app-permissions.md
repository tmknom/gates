# GitHub App Permissions

本ドキュメントでは、個人アカウントの GitHub App に権限を付与し、権限名と水準を変えて Installation Access Token の発行を要求した応答と、`contents:write` で発行したトークンで試した書き込みの応答を記述します。

- 調査日: **2026/11/20**
- 要求: GitHub App JWT で `GET https://api.github.com/app`、`GET https://api.github.com/users/{username}/installation`、`POST https://api.github.com/app/installations/{installation_id}/access_tokens` を呼び、発行したトークンで `DELETE https://api.github.com/installation/token` と、リポジトリの API を呼んだ
- API バージョン: `X-GitHub-Api-Version: 2026-03-10`
- 検証環境: 個人アカウントの検証用 GitHub App。Installation は `Only select repositories` で、対象は private リポジトリ 2 件（以下 `example-repo` と `example-repo-2`）。発行の要求の `repositories` は、どれも `["example-repo"]` だけにした

GitHub App の Private Key、GitHub App JWT、Installation Access Token は記録していません。
トークンは長さと `ghs_` で始まるかだけを記録しました。
発行したトークンは、どれも長さが 390 文字で、`ghs_` で始まりました。
オーナー名とリポジトリ名は、`octocat` と上の名前に置き換えています。

## 取得方法

調査は 2 つの状態で行いました。

| 状態 | GitHub App の権限 | Installation の権限 |
|---|---|---|
| 変更の前 | `contents:read`、`metadata:read` | `contents:read`、`metadata:read` |
| 付与の後 | 設定画面の Repository permissions、Organization permissions、Account permissions、Enterprise permissions のすべてを、選べる最も高い水準にした（137 の権限名） | Installation の側で承認した後の 43 の権限名（[設定画面の項目と Installation の権限名](#設定画面の項目と-installation-の権限名)） |

要求の送信、応答の記録、トークンの失効は、一つの Python のプロセスで行いました。
GitHub App JWT は、ローカルの Private Key のファイルを使って OpenSSL で署名し、`iat` を 60 秒前、`exp` を 9 分後にしました。
発行の要求が `201` を返したら、そのトークンですぐに `DELETE /installation/token` を呼びました。
[`contents:write` のトークンで試した書き込み](#contentswrite-のトークンで試した書き込み)のトークンだけは、書き込みを試した後に失効させました。
失効は、変更の前の状態で 5 件、付与の後の状態で 188 件で、すべて `204` でした。

付与の後の状態では、発行の要求を送る前に、Installation の権限から要求の一覧を作り、その一覧を承認してから送りました。
スクリプトは、GitHub App、Installation、発行、失効、検証用のリポジトリのほかのパスを呼べないようにしました。
Organization と Enterprise の権限でトークンが発行されても、そのトークンで呼んだのは失効だけです。

## 設定画面の項目と Installation の権限名

GitHub App の設定画面の Repository permissions には 43 の項目がありました。
すべてを最も高い水準にして承認した後、Installation の `permissions` には 43 の権限名が現れました。
GitHub App の `permissions` には、Organization permissions、Account permissions、Enterprise permissions を含めて 137 の権限名がありましたが、Installation に現れたのは、このうちの 43 でした。

次の表の列は、それぞれ次のとおりです。

- **設定画面の項目**：設定画面の Repository permissions の項目名
- **権限名**：Installation の `permissions` のキー
- **付与の水準**：Installation の `permissions` の値。設定画面で選べた最も高い水準である
- **スキーマの水準**：REST API の OpenAPI 定義のスキーマ「App Permissions」（`app-permissions`）が、その権限名に定める水準。スキーマに権限名がなければ「なし」
- **資料の表示名**：[Permissions required for GitHub Apps](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps) のページに埋め込まれたデータが、その権限名に付けた Repository permissions の表示名。データに権限名がなければ「なし」
- **要求どおりに発行された水準**：その権限名だけを、その水準で要求した発行が `201` で、応答の `permissions` が、要求した権限と `metadata:read` だけだった水準

| 設定画面の項目 | 権限名 | 付与の水準 | スキーマの水準 | 資料の表示名 | 要求どおりに発行された水準 |
|---|---|---|---|---|---|
| Actions | `actions` | `write` | `read`, `write` | `Actions` | `read`, `write` |
| Administration | `administration` | `write` | `read`, `write` | `Administration` | `read`, `write` |
| Agent secrets | `agent_secrets` | `write` | なし | `Agent secrets` | `read`, `write` |
| Agent tasks | `agent_tasks` | `write` | なし | なし | `read`, `write` |
| Agent variables | `agent_variables` | `write` | なし | `Agent variables` | `read`, `write` |
| Artifact metadata | `artifact_metadata` | `write` | `read`, `write` | `Artifact metadata` | `read`, `write` |
| Attestations | `attestations` | `write` | `read`, `write` | `Attestations` | `read`, `write` |
| Checks | `checks` | `write` | `read`, `write` | `Checks` | `read`, `write` |
| Code quality | `code_quality` | `write` | `read`, `write` | `Code quality` | `read`, `write` |
| Code scanning alerts | `security_events` | `write` | `read`, `write` | `Code scanning alerts` | `read`, `write` |
| Codespaces | `codespaces` | `write` | `read`, `write` | `Codespaces` | `read`, `write` |
| Codespaces lifecycle admin | `codespaces_lifecycle_admin` | `write` | なし | `Codespaces lifecycle admin` | `read`, `write` |
| Codespaces metadata | `codespaces_metadata` | `read` | なし | `Codespaces metadata` | `read` |
| Codespaces secrets | `codespaces_secrets` | `write` | なし | `Codespaces secrets` | `read`, `write` |
| Commit statuses | `statuses` | `write` | `read`, `write` | `Commit statuses` | `read`, `write` |
| Contents | `contents` | `write` | `read`, `write` | `Contents` | `read`, `write` |
| Copilot agent settings | `copilot_agent_settings` | `write` | なし | `Copilot agent settings` | `read`, `write` |
| Custom properties | `repository_custom_properties` | `write` | `read`, `write` | `Custom properties` | `read`, `write` |
| Dependabot alerts | `vulnerability_alerts` | `write` | `read`, `write` | `Dependabot alerts` | `read`, `write` |
| Dependabot secrets | `dependabot_secrets` | `write` | `read`, `write` | `Dependabot secrets` | `read`, `write` |
| Deployments | `deployments` | `write` | `read`, `write` | `Deployments` | `read`, `write` |
| Discussions | `discussions` | `write` | `read`, `write` | なし | `read`, `write` |
| Environments | `environments` | `write` | `read`, `write` | `Environments` | `read`, `write` |
| GitHub App installation repository access | `installation_repositories` | `write` | なし | `GitHub App installation repository access` | `read`, `write` |
| Issues | `issues` | `write` | `read`, `write` | `Issues` | `read`, `write` |
| License compliance alerts | `license_compliance_alerts` | `write` | なし | なし | `read`, `write` |
| Merge queues | `merge_queues` | `write` | `read`, `write` | なし | `read`, `write` |
| Metadata | `metadata` | `read` | `read`, `write` | `Metadata` | `read` |
| Packages | `packages` | `write` | `read`, `write` | なし | `read`, `write` |
| Pages | `pages` | `write` | `read`, `write` | `Pages` | `read`, `write` |
| Projects | `repository_projects` | `admin` | `read`, `write`, `admin` | なし | `read`, `write`, `admin` |
| Pull requests | `pull_requests` | `write` | `read`, `write` | `Pull requests` | `read`, `write` |
| Repository creation | `repository_creation` | `write` | なし | `Repository creation` | `read`, `write` |
| Repository license policy | `repository_license_policy` | `read` | なし | なし | `read` |
| Repository security advisories | `repository_advisories` | `write` | なし | `Repository security advisories` | `read`, `write` |
| Secret scanning alert dismissal requests | `repo_secret_scanning_dismissal_requests` | `write` | なし | なし | `read`, `write` |
| Secret scanning alerts | `secret_scanning_alerts` | `write` | `read`, `write` | `Secret scanning alerts` | `read`, `write` |
| Secret scanning push protection bypass requests | `secret_scanning_bypass_requests` | `write` | なし | なし | `read`, `write` |
| Secrets | `secrets` | `write` | `read`, `write` | `Secrets` | `read`, `write` |
| Single file | `single_file` | `write` | `read`, `write` | なし | `read`, `write` |
| Variables | `actions_variables` | `write` | なし | `Variables` | `read`, `write` |
| Webhooks | `repository_hooks` | `write` | `read`, `write` | `Webhooks` | `read`, `write` |
| Workflows | `workflows` | `write` | `write` | `Workflows` | `read`, `write` |

権限を一つずつ付与して確かめてはいないため、設定画面の項目と権限名の組は、資料の表示名か、項目名と権限名の似かたで対応させたものです。
`Projects` と `repository_projects` の組は、付与の水準が `admin` だった権限名が `repository_projects` だけだったことでも決まります。
`Single file` は、設定画面の項目のうち、ファイルのパスを指定する欄があった唯一の項目です。
欄の説明は `The content paths to single files your app can access.` で、この記録では `README.md` を指定しました。
Installation の設定画面には `Write access to files located at README.md` と表示されました。
`single_file` を含む要求の発行応答にだけ、次の 3 つのキーが加わりました（`single_file` の `write` と `read` の 2 件）。

```json
{
  "single_file": "README.md",
  "has_multiple_single_files": false,
  "single_file_paths": ["README.md"]
}
```

ほかの要求の発行応答には、これらのキーはありませんでした。
Installation の取得の応答に、パスに当たるキーがあるかは記録していません。

スキーマの 57 の権限名のうち、Installation の 43 の権限名に入っていたのは 28 でした。
スキーマの残りの 29 の権限名は、Installation に現れませんでした。
説明文がリポジトリに触れる `interaction_limits` も、GitHub App には付与されましたが、Installation には現れませんでした。
Installation の 43 の権限名のうち、15 はスキーマに権限名がありません。
その 15 のうち 10 は、資料のデータに Repository permissions の権限名として載っています。

## GitHub App だけに付与された権限

GitHub App の 137 の権限名のうち、Installation に現れなかった 94 は、設定画面の Organization permissions、Account permissions、Enterprise permissions の項目に当たります。
権限名は次のとおりです。

`blocking`、`codespaces_user_secrets`、`copilot_editor_context`、`copilot_messages`、`custom_properties_for_organizations`、`emails`、`enterprise_access_restrictions`、`enterprise_ai_controls`、`enterprise_billing`、`enterprise_bypass_requests`、`enterprise_copilot_licenses`、`enterprise_copilot_metrics`、`enterprise_copilot_usage`、`enterprise_credentials`、`enterprise_custom_enterprise_roles`、`enterprise_custom_org_roles`、`enterprise_custom_properties`、`enterprise_custom_properties_for_organizations`、`enterprise_delegate_policies`、`enterprise_innersource_vulnerabilities`、`enterprise_ip_allow_list`、`enterprise_license_policy`、`enterprise_organization_installation_repositories`、`enterprise_organization_installations`、`enterprise_organizations`、`enterprise_people`、`enterprise_ref_rules`、`enterprise_scim`、`enterprise_security_products`、`enterprise_settings`、`enterprise_sso`、`enterprise_teams`、`enterprise_two_factor_authentication`、`enterprise_verifiable_domains`、`followers`、`gists`、`git_signing_ssh_public_keys`、`gpg_keys`、`interaction_limits`、`issue_fields`、`issue_types`、`keys`、`members`、`org_copilot_content_exclusion`、`organization_actions_variables`、`organization_administration`、`organization_agent_secrets`、`organization_agent_variables`、`organization_announcement_banners`、`organization_api_insights`、`organization_campaigns`、`organization_code_scanning_dismissal_requests`、`organization_codespaces`、`organization_codespaces_secrets`、`organization_codespaces_settings`、`organization_copilot_agent_settings`、`organization_copilot_metrics`、`organization_copilot_requests`、`organization_copilot_seat_management`、`organization_copilot_spaces`、`organization_credentials`、`organization_custom_org_roles`、`organization_custom_properties`、`organization_custom_roles`、`organization_dependabot_dismissal_requests`、`organization_dependabot_secrets`、`organization_events`、`organization_external_properties_for_repos`、`organization_hooks`、`organization_innersource_vulnerabilities`、`organization_ip_allow_list`、`organization_models`、`organization_network_configurations`、`organization_personal_access_token_requests`、`organization_personal_access_tokens`、`organization_plan`、`organization_private_registries`、`organization_projects`、`organization_runner_custom_images`、`organization_secret_scanning_bypass_requests`、`organization_secrets`、`organization_self_hosted_runners`、`organization_user_blocking`、`organization_verifiable_domains`、`plan`、`plugin_gateway_connections`、`profile`、`secret_scanning_dismissal_requests`、`starring`、`user_events`、`user_issue_fields`、`user_issue_types`、`user_models`、`watching`

[Choosing permissions for a GitHub App](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/choosing-permissions-for-a-github-app) は、Organization permissions は Organization にインストールした場合、Enterprise permissions は Enterprise にインストールした場合に効き、Account permissions はユーザーが GitHub App を認可した場合にそのユーザーの資源に効くと説明しています。
また、個人アカウントか Organization にインストールするとき、利用者は Repository permissions と Organization permissions を確認して付与し、Account permissions はユーザーが GitHub App を認可するときに付与すると説明しています。
この記録の個人アカウントの Installation の `permissions` には、Organization permissions は現れませんでした。
設定画面も、Account permissions を「These permissions are granted on an individual user basis as part of the User authorization flow.」と説明していました。
この 94 の権限名を一つずつ、GitHub App に付与した水準で要求すると、どれも `201` で、応答の `permissions` は `{}` で、`repositories` のキーがありませんでした。
`repository_selection` は `selected` でした。

```json
{
  "token": "<omitted>",
  "expires_at": "2026-11-20T04:36:32Z",
  "permissions": {},
  "repository_selection": "selected"
}
```

上は `{"members":"write"}` を要求した応答です。
`{"contents":"read","members":"read"}` を要求した応答は `201` で、`permissions` は `contents:read` と `metadata:read`、`repositories` は `example-repo` の 1 件でした。
要求した `members` は、拒否されずに除かれました。

変更の前の状態（GitHub App に `members` を付与していない）では、同じ `{"members":"read"}` と `{"contents":"read","members":"read"}` は `422` でした（[拒否された要求](#拒否された要求)）。

スキーマの権限名 `email_addresses` は、付与の後の状態でも `422` でした。
GitHub App の `permissions` にあったのは `email_addresses` ではなく `emails` で、`{"emails":"write"}` は上と同じ `201` と空の `permissions` でした。

## 付与した権限の発行

付与の後の状態で要求した `permissions` と応答の `permissions` は、次のとおりでした。
どの `201` の応答も、`repositories` は `example-repo` の 1 件で、`repository_selection` は `selected` でした。

| 要求の `permissions` | 応答の `permissions` |
|---|---|
| 43 の権限名のどれか一つを、付与の水準で | 要求した権限と `metadata:read`。`metadata` だけの要求は `metadata:read` だけ |
| `write` か `admin` で付与した権限名のどれか一つを、それより低い水準で（41 件） | 要求した権限と `metadata:read` |
| `{"contents":"write"}` | `contents:write`、`metadata:read` |
| `{"workflows":"write"}` | `workflows:write`、`metadata:read` |
| `{"workflows":"read"}` | `workflows:read`、`metadata:read` |
| `{"administration":"write"}` | `administration:write`、`metadata:read` |
| `{"contents":"read","pull_requests":"write"}` | 要求した 2 つと `metadata:read` |
| `{"contents":"write","workflows":"write"}` | 要求した 2 つと `metadata:read` |
| `{"issues":"write","pull_requests":"read"}` | 要求した 2 つと `metadata:read` |
| `{"contents":"read","issues":"read","pull_requests":"read"}` | 要求した 3 つと `metadata:read` |
| `{"contents":"read","metadata":"read"}` | 要求と同じ |
| `{"contents":"write","metadata":"read"}` | 要求と同じ |

`metadata:read` のほかに、要求していない権限が加わった応答はありませんでした。
要求した水準より高い水準か低い水準で返った権限もありません。
`workflows:read` はスキーマに定められていない水準ですが、要求どおりに発行されました。

変更の前の状態では、`{"metadata":"read"}` の応答は `metadata:read` だけで、`{"contents":"read"}` の応答は `contents:read` と `metadata:read` でした。
`{}` と `permissions` を省いた要求は、どちらも `201` で、応答の `permissions` は `contents:read` と `metadata:read` でした。
付与の後の状態では、この 2 つの要求を送っていません。

## 拒否された要求

拒否はすべて `422` で、本文の `message` は次の 3 種類でした。

| `message` | 要求の `permissions` |
|---|---|
| `The permissions requested are not granted to this installation.` | 変更の前の状態で、GitHub App に付与していない権限（スキーマの 28 の権限名のうち `contents` と `metadata` を除くもの、`members`、`organization_administration`、`email_addresses`）を、単独か `contents:read` とともに要求した。付与の後の状態で、`nonexistent_permission`、`Contents`（大文字）、`email_addresses` を要求した |
| `The level of access for permissions requested are not granted to this installation.` | 付与の水準より高い水準を要求した（変更の前の `contents:write`、`metadata:write`、`{"contents":"write","metadata":"read"}`、付与の後の `metadata:write`）。`contents:admin` を要求した（どちらの状態でも） |
| `There is at least one permission action that is not supported. It should be one of: "read", "write" or "admin".` | 水準に `READ` か `none` を指定した |

`documentation_url` は、どれも `https://docs.github.com/rest/reference/apps#create-an-installation-access-token-for-an-app` でした。
権限を除くか水準を下げて `201` を返した応答は、GitHub App に付与していない権限と、付与の水準より高い水準の要求にはありませんでした。

変更の前の状態の `{"workflows":"read"}` は 1 つ目の `message` でした。
GitHub App に `workflows` を付与していなかったため、水準の問題か付与の問題かは、この応答から区別できません。

## `contents:write` のトークンで試した書き込み

付与の後の状態で、`{"contents":"write"}` と `["example-repo"]` で発行したトークンで、次の API を順に呼びました。

| 要求 | 対象 | 応答 | `X-Accepted-GitHub-Permissions` |
|---|---|---|---|
| `GET /repos/{owner}/{repo}` | 要求した対象 | `200` | `metadata=read` |
| `GET /repos/{owner}/{repo}/git/ref/heads/main` | 要求した対象 | `200` | `contents=read` |
| `POST /repos/{owner}/{repo}/git/refs`（新しいブランチ） | 要求した対象 | `201` | `contents=write; contents=write,workflows=write` |
| `PUT /repos/{owner}/{repo}/contents/{path}`（新しいブランチに新しいファイル） | 要求した対象 | `201` | `contents=write; contents=write,workflows=write` |
| `DELETE /repos/{owner}/{repo}/git/refs/heads/{branch}` | 要求した対象 | `204` | `contents=write` |
| `PUT /repos/{owner}/{repo}/contents/{path}`（既定のブランチに新しいファイル） | Installation の対象で、要求していない private リポジトリ | `404` | `contents=write; contents=write,workflows=write` |
| `PUT /repos/{owner}/{repo}/contents/{path}`（既定のブランチに新しいファイル） | Installation の対象外の、同じアカウントの公開リポジトリ | `403` | `contents=write; contents=write,workflows=write` |
| `PUT /repos/{owner}/{repo}/contents/{path}`（既定のブランチに新しいファイル） | Installation の対象外の、同じアカウントの private リポジトリ | `404` | `contents=write; contents=write,workflows=write` |
| `DELETE /installation/token` | なし | `204` | `allows_permissionless_access=true` |

`404` の本文の `message` は `Not Found`、`403` の本文の `message` は `Resource not accessible by integration` でした。
要求した対象に作ったブランチは、同じトークンで削除しました。

## 確かめなかったこと

- ブランチ保護と ruleset が、GitHub App のトークンによる変更に適用されるか
- Installation の側で承認する前の Repository permissions を要求したときの応答。[GitHub App だけに付与された権限](#github-app-だけに付与された権限)と同じく、権限を除いて発行するかは確かめていない
- Organization の Installation での応答。Organization permissions が Installation に現れる場合の振る舞いは、この記録の対象外である
- 権限を一つずつ付与したときの、設定画面の項目と権限名の対応

## 参照した公式文書

- [Permissions required for GitHub Apps](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps)：権限ごとの表示名と、使える API。ページに埋め込まれたデータが、Repository permissions の 33 の権限名と表示名の組を持つ
- [REST API endpoints for GitHub Apps](https://docs.github.com/en/rest/apps/apps)：Installation Access Token の発行 API
- [REST API Description](https://github.com/github/rest-api-description)：OpenAPI 定義のスキーマ「App Permissions」
- [Choosing permissions for a GitHub App](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/choosing-permissions-for-a-github-app)：Repository、Organization、Enterprise、Account の各分類の権限が効く条件
- [Modifying a GitHub App registration](https://docs.github.com/en/apps/maintaining-github-apps/modifying-a-github-app-registration)：権限の追加は Installation の側の承認まで効かないこと
