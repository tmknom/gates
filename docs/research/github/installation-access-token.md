# GitHub Installation Access Token

本ドキュメントでは、対象リポジトリと権限を明示して Installation Access Token を発行した応答と、そのトークンを失効させた応答を記述します。

- 調査日: **2026/11/03**
- 要求: `POST https://api.github.com/app/installations/{installation_id}/access_tokens`（GitHub App JWT で認証）、`DELETE https://api.github.com/installation/token`（発行したトークンで認証）
- API バージョン: `X-GitHub-Api-Version: 2026-03-10`（応答の `x-github-api-version-selected` も `2026-03-10`）
- 検証環境: 個人アカウントの検証用 GitHub App。Installation は `Only select repositories` で、対象は private リポジトリ 2 件。Repository permissions は `Contents` の `Read-only` だけ

GitHub App の Private Key、GitHub App JWT、Installation Access Token は記録していません。
トークンは長さと `ghs_` で始まるかだけを記録し、応答の `token` の値は `<omitted>` に置き換えています。

## 取得方法

GitHub App JWT は、ローカルの Private Key のファイルを使って OpenSSL で署名し、`iat` を 60 秒前、`exp` を 9 分後にしました。
発行と失効は、トークンをファイルにも端末にも出力しないよう、要求の送信、応答の記録、失効を一つの Python のプロセスで行いました。
送った要求は次の `curl` と同じです。
この `curl` は要求の内容を示すもので、そのまま実行すると応答の `token` が端末に出力されます。

```bash
curl --silent --show-error --include -X POST \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  -H "Authorization: Bearer ${APP_JWT}" \
  -d '{"repositories":["example-repo"],"permissions":{"contents":"read"}}' \
  "https://api.github.com/app/installations/${INSTALLATION_ID}/access_tokens"

curl --silent --show-error --include -X DELETE \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  -H "Authorization: Bearer ${INSTALLATION_ACCESS_TOKEN}" \
  "https://api.github.com/installation/token"
```

次の表の発行の要求を、同じ GitHub App JWT で続けて送りました。
すべての要求の `permissions` は `{"contents":"read"}` です。

| 要求の `repositories` | 応答コード | 本文の大きさ |
|---|---|---|
| `["example-repo"]` | `201` | 約 6 KB |
| `["example-repo", "example-repo-2"]` | `201` | 約 12 KB |
| `["example-repo-2", "example-repo"]` | `201` | 約 12 KB |
| `["Example-Repo"]` | `201` | 約 6 KB |
| `["example-repo", "public-repo"]` | `422` | 約 240 バイト |
| `["public-repo"]` | `422` | 約 240 バイト |
| `["example-repo", "nonexistent-repo"]` | `422` | 約 240 バイト |
| `["example-repo", "."]` | `422` | 約 240 バイト |
| `["example-repo", ".."]` | `422` | 約 240 バイト |

## 対象 1 件の発行

`["example-repo"]` を要求した応答です。
本文のキーは `token`、`expires_at`、`permissions`、`repository_selection`、`repositories` の 5 個でした。

```json
{
  "token": "<omitted>",
  "expires_at": "2026-11-03T12:18:48Z",
  "permissions": {
    "contents": "read",
    "metadata": "read"
  },
  "repository_selection": "selected",
  "repositories": [
    {
      "id": 1234567890,
      "node_id": "R_kgDOSZYC0g",
      "name": "example-repo",
      "full_name": "octocat/example-repo",
      "private": true,
      "owner": {
        "login": "octocat",
        "id": 583231,
        "node_id": "MDQ6VXNlcjU4MzIzMQ==",
        "avatar_url": "https://avatars.githubusercontent.com/u/583231?v=4",
        "gravatar_id": "",
        "url": "https://api.github.com/users/octocat",
        "html_url": "https://github.com/octocat",
        "followers_url": "https://api.github.com/users/octocat/followers",
        "following_url": "https://api.github.com/users/octocat/following{/other_user}",
        "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
        "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
        "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
        "organizations_url": "https://api.github.com/users/octocat/orgs",
        "repos_url": "https://api.github.com/users/octocat/repos",
        "events_url": "https://api.github.com/users/octocat/events{/privacy}",
        "received_events_url": "https://api.github.com/users/octocat/received_events",
        "type": "User",
        "user_view_type": "public",
        "site_admin": false
      },
      "html_url": "https://github.com/octocat/example-repo",
      "description": null,
      "fork": false,
      "url": "https://api.github.com/repos/octocat/example-repo",
      "forks_url": "https://api.github.com/repos/octocat/example-repo/forks",
      "keys_url": "https://api.github.com/repos/octocat/example-repo/keys{/key_id}",
      "collaborators_url": "https://api.github.com/repos/octocat/example-repo/collaborators{/collaborator}",
      "teams_url": "https://api.github.com/repos/octocat/example-repo/teams",
      "hooks_url": "https://api.github.com/repos/octocat/example-repo/hooks",
      "issue_events_url": "https://api.github.com/repos/octocat/example-repo/issues/events{/number}",
      "events_url": "https://api.github.com/repos/octocat/example-repo/events",
      "assignees_url": "https://api.github.com/repos/octocat/example-repo/assignees{/user}",
      "branches_url": "https://api.github.com/repos/octocat/example-repo/branches{/branch}",
      "tags_url": "https://api.github.com/repos/octocat/example-repo/tags",
      "blobs_url": "https://api.github.com/repos/octocat/example-repo/git/blobs{/sha}",
      "git_tags_url": "https://api.github.com/repos/octocat/example-repo/git/tags{/sha}",
      "git_refs_url": "https://api.github.com/repos/octocat/example-repo/git/refs{/sha}",
      "trees_url": "https://api.github.com/repos/octocat/example-repo/git/trees{/sha}",
      "statuses_url": "https://api.github.com/repos/octocat/example-repo/statuses/{sha}",
      "languages_url": "https://api.github.com/repos/octocat/example-repo/languages",
      "stargazers_url": "https://api.github.com/repos/octocat/example-repo/stargazers",
      "contributors_url": "https://api.github.com/repos/octocat/example-repo/contributors",
      "subscribers_url": "https://api.github.com/repos/octocat/example-repo/subscribers",
      "subscription_url": "https://api.github.com/repos/octocat/example-repo/subscription",
      "commits_url": "https://api.github.com/repos/octocat/example-repo/commits{/sha}",
      "git_commits_url": "https://api.github.com/repos/octocat/example-repo/git/commits{/sha}",
      "comments_url": "https://api.github.com/repos/octocat/example-repo/comments{/number}",
      "issue_comment_url": "https://api.github.com/repos/octocat/example-repo/issues/comments{/number}",
      "contents_url": "https://api.github.com/repos/octocat/example-repo/contents/{+path}",
      "compare_url": "https://api.github.com/repos/octocat/example-repo/compare/{base}...{head}",
      "merges_url": "https://api.github.com/repos/octocat/example-repo/merges",
      "archive_url": "https://api.github.com/repos/octocat/example-repo/{archive_format}{/ref}",
      "downloads_url": "https://api.github.com/repos/octocat/example-repo/downloads",
      "issues_url": "https://api.github.com/repos/octocat/example-repo/issues{/number}",
      "pulls_url": "https://api.github.com/repos/octocat/example-repo/pulls{/number}",
      "milestones_url": "https://api.github.com/repos/octocat/example-repo/milestones{/number}",
      "notifications_url": "https://api.github.com/repos/octocat/example-repo/notifications{?since,all,participating}",
      "labels_url": "https://api.github.com/repos/octocat/example-repo/labels{/name}",
      "releases_url": "https://api.github.com/repos/octocat/example-repo/releases{/id}",
      "deployments_url": "https://api.github.com/repos/octocat/example-repo/deployments",
      "created_at": "2026-10-28T04:09:23Z",
      "updated_at": "2026-10-28T04:10:06Z",
      "pushed_at": "2026-10-28T04:09:25Z",
      "git_url": "git://github.com/octocat/example-repo.git",
      "ssh_url": "git@github.com:octocat/example-repo.git",
      "clone_url": "https://github.com/octocat/example-repo.git",
      "svn_url": "https://github.com/octocat/example-repo",
      "homepage": null,
      "size": 0,
      "stargazers_count": 0,
      "watchers_count": 0,
      "language": null,
      "has_issues": false,
      "has_projects": false,
      "has_wiki": false,
      "has_pages": false,
      "has_discussions": false,
      "forks_count": 0,
      "mirror_url": null,
      "archived": false,
      "disabled": false,
      "open_issues_count": 0,
      "license": null,
      "allow_forking": true,
      "is_template": false,
      "web_commit_signoff_required": false,
      "has_pull_requests": false,
      "pull_request_creation_policy": "all",
      "topics": [],
      "visibility": "private",
      "forks": 0,
      "open_issues": 0,
      "watchers": 0,
      "default_branch": "main"
    }
  ]
}
```

- **`token`**：390 文字で、`ghs_` で始まる。対象 2 件と名前の大文字小文字を変えた発行のトークンも 390 文字だった。旧形式の 40 文字と異なるため新形式のトークンだと読めるが、トークンの構造は確かめていない
- **`expires_at`**：秒までの UTC で、末尾が `Z` の RFC 3339 形式。小数秒はない
- **`permissions`**：要求していない `metadata` の `read` が加わる
- **`repositories[]`**：リポジトリの完全な表現（80 個のキー）で、`id` と `owner.id` は JSON の数値。`permissions` のキーはない

## 対象 2 件の発行

`["example-repo", "example-repo-2"]` と `["example-repo-2", "example-repo"]` の応答は、どちらも `repositories` が 2 件で、`example-repo`、`example-repo-2` の順でした。
要求の並びは応答の並びに現れません。
この 2 件では、応答の並びは Repository ID の昇順とも名前の昇順とも一致します。
`token`、`expires_at`、`permissions`、`repository_selection` の形は対象 1 件の発行と同じです。

```json
{
  "repositories": [
    {
      "id": 1234567890,
      "name": "example-repo",
      "full_name": "octocat/example-repo",
      "owner": { "id": 583231 }
    },
    {
      "id": 1234567891,
      "name": "example-repo-2",
      "full_name": "octocat/example-repo-2",
      "owner": { "id": 583231 }
    }
  ]
}
```

上は `repositories[]` の要素から `id`、`name`、`full_name`、`owner.id` だけを抜き出したものです。
本文は対象 1 件で約 6 KB、2 件で約 12 KB でした。
リポジトリの表現は名前を URL の中で繰り返すため、1 件あたりの大きさは名前の長さで変わります。

## 名前の大文字小文字を変えた発行

`["Example-Repo"]` を要求した応答は `201` で、`repositories[].name` は `example-repo`、`full_name` は `octocat/example-repo` でした。
応答の名前は要求の表記ではなく、GitHub 上の表記です。

## 発行が拒否された要求

Installation で選択していない同じアカウントの公開リポジトリ（`public-repo`）を含む要求、そのリポジトリだけの要求、存在しないリポジトリを含む要求、`.` を含む要求、`..` を含む要求は、どれも `422` で、本文も同じでした。
対象を Installation の範囲に縮小した `201` の応答はありません。

```json
{
  "message": "There is at least one repository that does not exist or is not accessible to the parent installation.",
  "documentation_url": "https://docs.github.com/rest/reference/apps#create-an-installation-access-token-for-an-app",
  "status": "422"
}
```

本文は、どのリポジトリが原因かを示しません。

## 失効の前後のトークン

対象 1 件の発行で得たトークンで、失効の前に `GET /installation/repositories` を呼ぶと `200` で、`total_count` は `1`、`repositories` は `example-repo` だけでした。
その後の `DELETE /installation/token` は `204` で、本文はありません。
失効の後に同じトークンで `GET /installation/repositories` を呼ぶと `401` でした。

```json
{
  "message": "Bad credentials",
  "documentation_url": "https://docs.github.com/rest",
  "status": "401"
}
```

対象 2 件の 2 回の発行と、名前の大文字小文字を変えた発行のトークンも、`DELETE /installation/token` がすべて `204` を返しました。
発行した 4 件のトークンのうち、失効に失敗したものはありません。

## レート制限のヘッダー

レート制限は起こしていません。
`retry-after` のヘッダーは、この記録のどの応答にもありません。
`x-ratelimit-*` のヘッダーの有無は、要求の認証で分かれました。

| 要求 | 認証 | `x-ratelimit-*` |
|---|---|---|
| `POST /app/installations/{installation_id}/access_tokens`（`201`、`422`） | GitHub App JWT | なし |
| `GET /installation/repositories`（失効の前、`200`） | Installation Access Token | `x-ratelimit-limit` が `5000`、`x-ratelimit-remaining` が `4999`、`x-ratelimit-resource` が `core` |
| `DELETE /installation/token`（`204`） | Installation Access Token | `x-ratelimit-limit` が `5000`、`x-ratelimit-remaining` が `5000`、`x-ratelimit-resource` が `core` |
| `GET /installation/repositories`（失効の後、`401`） | 失効したトークン | なし |
