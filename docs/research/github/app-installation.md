# GitHub App Installation

本ドキュメントでは、GitHub App JWT でユーザーアカウントの Installation を取得した応答を記述します。

- 調査日: **2026/11/03**
- 要求: `GET https://api.github.com/users/{username}/installation`
- API バージョン: `X-GitHub-Api-Version: 2026-03-10`（応答の `x-github-api-version-selected` も `2026-03-10`）
- 検証環境: 個人アカウントの検証用 GitHub App。Installation は `Only select repositories` で、Repository permissions は `Contents` の `Read-only` だけ

GitHub App の Private Key と GitHub App JWT は記録していません。
Private Key は調査のためだけに検証用 App に追加した一時的な鍵で、調査の後に GitHub の `Key pairs` から削除しました。

## 取得コマンド

GitHub App JWT は OpenSSL で署名し、`iat` を 60 秒前、`exp` を 9 分後にしました。

```bash
b64url() { openssl base64 -A | tr '+/' '-_' | tr -d '='; }

now="$(date +%s)"
header="$(printf '{"alg":"RS256","typ":"JWT"}' | b64url)"
payload="$(printf '{"iat":%d,"exp":%d,"iss":"%s"}' "$((now - 60))" "$((now + 540))" "${CLIENT_ID}" | b64url)"
signature="$(printf '%s.%s' "${header}" "${payload}" | openssl dgst -sha256 -sign "${PRIVATE_KEY_PATH}" | b64url)"
APP_JWT="${header}.${payload}.${signature}"

curl --silent --show-error --include \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  -H "Authorization: Bearer ${APP_JWT}" \
  "https://api.github.com/users/${USERNAME}/installation"
```

## Installation の取得

応答コードは `200` で、本文は約 1.6 KB でした。
`x-ratelimit-*` と `retry-after` のヘッダーはありません。

```json
{
  "id": 12345678,
  "client_id": "Iv23liEXAMPLE0000000",
  "account": {
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
  "repository_selection": "selected",
  "access_tokens_url": "https://api.github.com/app/installations/12345678/access_tokens",
  "repositories_url": "https://api.github.com/installation/repositories",
  "html_url": "https://github.com/settings/installations/12345678",
  "app_id": 123456,
  "app_slug": "testing-gates-for-octocat",
  "target_id": 583231,
  "target_type": "User",
  "permissions": {
    "contents": "read",
    "metadata": "read"
  },
  "events": [],
  "created_at": "2026-10-28T04:32:35.000Z",
  "updated_at": "2026-11-03T11:17:03.000Z",
  "single_file_name": null,
  "has_multiple_single_files": false,
  "single_file_paths": [],
  "suspended_by": null,
  "suspended_at": null
}
```

停止していない Installation では、`suspended_at` と `suspended_by` のキーは欠落せず、`null` で現れます。
`id` と `account.id` は JSON の数値です。
`permissions` には、App の設定で選んだ `contents` に加えて `metadata` の `read` があります。

## 署名を壊した GitHub App JWT

同じ要求を、署名の先頭のバイトを 1 ビット反転させた GitHub App JWT で送りました。
応答コードは `401` で、本文は約 120 バイトでした。
`x-ratelimit-*` と `retry-after` のヘッダーはありません。

```json
{
  "message": "A JSON web token could not be decoded",
  "documentation_url": "https://docs.github.com/rest",
  "status": "401"
}
```

`status` は JSON の文字列です。
