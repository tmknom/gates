# GitHub App Installation Changes

本ドキュメントでは、個人アカウントの GitHub App について、Installation の対象、Installation の停止、アンインストールと再インストール、GitHub App の権限を画面で変え、変えるたびに Installation Access Token の発行を要求した結果を記述します。

- 調査日: **2026/11/13**
- 要求: GitHub Actions のワークフローから GATES に発行を要求した。GATES は GitHub App JWT で `GET https://api.github.com/users/{username}/installation` を呼び、Installation が停止していなければ `POST https://api.github.com/app/installations/{installation_id}/access_tokens` に対象のリポジトリ名と `{"contents":"read"}` を指定する
- API バージョン: `X-GitHub-Api-Version: 2026-03-10`
- 検証環境: 個人アカウントの検証用 GitHub App。変更の前は、Installation が `Only select repositories` で対象は private リポジトリ 2 件（以下 A と B）、Repository permissions は `Contents` の `Read-only` だけ。Installation の対象外だった同じアカウントのリポジトリ 1 件（以下 C）を追加と削除に使った。C が公開か非公開かは記録していない

GitHub App JWT、Installation Access Token、Installation の取得と発行の応答本文は記録していません。
GitHub の応答は、GATES が記録した値（Installation の取得と発行の応答コード、Installation ID、エラーの種類）と、発行したトークンで呼んだ API の応答コードから読みました。
Installation ID は値を記録せず、変わったかどうかだけを記録しています。

## 取得方法

変更のたびに、ワークフローの `env` の要求の対象を書き換えて push し、GATES の Docker Action で発行を要求しました。
発行に成功したら、トークンで次の API を呼び、ステータスと `X-Accepted-GitHub-Permissions` のヘッダーだけを出しました。

- `GET /installation/repositories`：トークンから見える対象の件数と `full_name`
- `GET /repos/{owner}/{repo}/contents/README.md`：要求した各対象の内容の読み取り
- `GET /repos/{owner}/{repo}/issues`：要求した各対象の Issues の一覧

トークンはどの要求でも、ジョブの終わりに `DELETE /installation/token` で失効させ、`204` が返りました。

## 変更ごとの結果

| 順 | 画面での変更 | 要求の対象 | Installation の取得 | 発行 | トークンから見える対象 |
|---|---|---|---|---|---|
| 0 | なし | A | `200` | `201` | A |
| 1 | C を Installation の対象に加える | A、B、C | `200` | `201` | A、B、C |
| 2 | C を Installation の対象から外す | A、B、C | `200` | `422` | なし（発行されない） |
| 3 | なし | A、B | `200` | `201` | A、B |
| 4 | Installation を停止する（Suspend） | A | `200`。`suspended_at` は時刻 | 呼ばない | なし |
| 5 | 停止を解く（Unsuspend） | A | `200` | `201` | A |
| 6 | GitHub App をアンインストールする | A | `404` | 呼ばない | なし |
| 7 | A と B を選んで再インストールする | A / A、B | `200`。Installation ID が変わった | `201` | A / A、B |
| 8 | GitHub App の権限に `Issues` の `Read-only` を加える | A | `200` | `201` | A |
| 9 | Installation の側で新しい権限を承認する | A | `200` | `201` | A |
| 10 | GitHub App の権限から `Issues` を外す | A | `200` | `201` | A |

発行に成功したすべての要求で、発行応答の `permissions` は `contents:read` と `metadata:read` で、要求した各対象の `README.md` は `200` でした。

## Installation の対象の追加と削除

C を対象に加えた変更と外した変更は、どちらも保存から約 1 分後の要求で反映されていました。
これより短い間隔は試していません。

C を加えた後の発行応答の `repositories` は、要求の並び（A、B、C）ではなく、C、A、B の並びでした。

C を外した後の A、B、C の要求は `422` で拒否され、対象を A と B に縮小した応答はありませんでした。
外した後の A と B だけの要求は `201` で、トークンから見える対象は A と B の 2 件でした。

## 停止中の Installation

停止中の Installation の取得は `200` でした。
GATES は応答の `suspended_at` を、`null` なら停止していない、ゼロ値でない RFC 3339 の時刻なら停止している、キーの欠落やそれ以外の値なら読み取れない、と判定します。
この要求では停止していると判定したため、`suspended_at` は時刻で返ったことになります。
値そのものは記録していません。

GATES は停止と判定した時点で要求を終えるため、停止中の Installation に対する発行 API は呼んでおらず、その応答は観測していません。

`suspended_at` の値を GitHub App JWT を使わずに見るため、停止中に gh の OAuth トークンで `GET /user/installations` を呼びましたが、`403` で拒否されました。

```json
{
  "message": "You must authenticate with an access token authorized to a GitHub App in order to list installations",
  "documentation_url": "https://docs.github.com/rest/apps/installations#list-app-installations-accessible-to-the-user-access-token",
  "status": "403"
}
```

## アンインストールと再インストール

アンインストールした後の `GET /users/{username}/installation` は `404` でした。

同じ GitHub App を、同じ A と B を選んで再インストールした後の取得は `200` で、Installation ID は再インストールの前と異なる値になりました。
`account.id` は変わりませんでした。
GATES の設定を変えずに、A だけの要求と A と B の要求がどちらも `201` で発行されました。

## GitHub App の権限の変更

GitHub App の設定で `Issues` の `Read-only` を加えて保存すると、Installation の設定画面に `Permission updates requested. Review request.` が表示されました。
承認の前も後も、`{"contents":"read"}` を指定した発行応答の `permissions` は `contents:read` と `metadata:read` だけでした。

承認すると、Installation の設定画面の Permissions に Issues の行が加わりました。
承認の後に発行したトークンで A の Issues の一覧を呼ぶと、`403` で、`X-Accepted-GitHub-Permissions` は `issues=read` でした。

GitHub App の設定で `Issues` を `No access` に戻した後も、発行は `201` で、`permissions` は `contents:read` と `metadata:read` でした。
戻した後の GitHub App の Repository permissions は、`Contents` と `Metadata` の `Read-only` だけでした。

## Issues が無効なリポジトリの Issues の一覧

A と B は private で、Issues が無効です。
Issues の権限を持たないトークンで Issues の一覧を呼ぶと、Issues が無効な A と B でも `410` ではなく `403` が返り、`X-Accepted-GitHub-Permissions` は `issues=read` でした。
Issues の有効・無効を記録していない C も、同じく `403` と `issues=read` でした。

## 参照した公式文書

- [Reviewing and modifying installed GitHub Apps](https://docs.github.com/en/apps/using-github-apps/reviewing-and-modifying-installed-github-apps)：Installation の対象の変更、停止、アンインストールの画面での手順
- [Suspending a GitHub App installation](https://docs.github.com/en/apps/maintaining-github-apps/suspending-a-github-app-installation)：停止を解けるのは停止した側だけであること
- [Modifying a GitHub App registration](https://docs.github.com/en/apps/maintaining-github-apps/modifying-a-github-app-registration)：権限の追加は Installation の側の承認まで効かず、権限の削除はすぐに効くこと
- [Approving updated permissions for a GitHub App](https://docs.github.com/en/apps/using-github-apps/approving-updated-permissions-for-a-github-app)：承認しなければ GitHub App は現在の権限のままであること
- [REST API endpoints for GitHub App installations](https://docs.github.com/en/rest/apps/installations)：`GET /user/installations` が GitHub App のユーザーアクセストークンを要すること
