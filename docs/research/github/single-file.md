# GitHub Single File Permission

本ドキュメントでは、GitHub App に `Single file` を付与し、`single_file:read` だけを要求して発行した Installation Access Token で、指定したパスのファイルとそのほかのリソースを読んだ応答と、そのトークンを失効させた後の応答を記述します。

- 調査日: **2026/11/23**
- 要求: デプロイした GATES を GitHub Actions のワークフローから Docker Action で呼んで発行し、発行したトークンで `GET https://api.github.com/repos/{owner}/{repo}/contents/{path}` などのリポジトリの API と `DELETE https://api.github.com/installation/token` を呼んだ
- API バージョン: `X-GitHub-Api-Version: 2026-03-10`（応答の `x-github-api-version-selected` も `2026-03-10`）
- 検証環境: 個人アカウントの検証用 GitHub App。Repository permissions は `Contents` の `Read-only`、`Issues` の `Read and write`、`Commit statuses` の `Read-only` に、`Single file` の `Read-only`（パス `.github/gates-policy.yml`）を加えた。Installation は `Only select repositories` で、対象は private リポジトリ 2 件（以下 `example-repo` と `example-repo-2`）

GitHub App の Private Key と Installation Access Token は記録していません。
応答の `download_url` と、tarball と zipball の `location` は一時的な資格情報を含み得るため、値を出さずに有無やホストだけを記録しました。
オーナー名とリポジトリ名は `octocat` と上の名前に置き換えています。

## 取得方法

発行の要求は、どれも `permissions` を `single_file:read` だけにしました。
対象は `example-repo` だけの要求と、`example-repo` と `example-repo-2` の要求の 2 つです。
発行応答の本文は GATES の外に出ないため、[発行応答](#発行応答)の値は Docker Action の出力と、発行したトークンで呼んだ API の応答から読んでいます。

発行したトークンで、次の内容を置いた `example-repo` のファイルを読みました。
`example-repo-2` には `README.md` だけがあり、`.github/gates-policy.yml` はありません。

| ブランチ | `.github/gates-policy.yml` |
|---|---|
| `main`（既定ブランチ） | 58 バイトの YAML のファイル |
| `variant` | 内容の異なる 60 バイトのファイル |
| `empty` | 0 バイトのファイル |
| `directory` | ディレクトリ（中に `inner.yml` を置いた） |
| `symlink` | 同じディレクトリの `not-policy.yml` を指すシンボリックリンク |
| `symlink-dangling` | 存在しない `nonexistent.yml` を指すシンボリックリンク |
| `large` | 1,152,048 バイトのファイル |
| `missing` | なし |

どのブランチにも `README.md` と `.github/not-policy.yml` があります。
既定ブランチ以外の状態は、`ref` にブランチ名を付けた要求で読みました。

要求はワークフローの `curl` で送り、接続を要求ごとに張り直しました。
`Accept` は、指定のない限り `application/vnd.github+json` です。
`raw` は `application/vnd.github.raw+json`、`object` は `application/vnd.github.object+json` を指します。

## 発行応答

| 要求の対象 | 応答 | Docker Action の出力の `permissions` | トークンの `GET /installation/repositories` |
|---|---|---|---|
| `example-repo` | `201` | `metadata:read,single_file:read` | `total_count` が `1`、`example-repo` だけ |
| `example-repo`、`example-repo-2` | `201` | `metadata:read,single_file:read` | `total_count` が `2`、要求した 2 件だけ |

GATES は、発行応答の `permissions` が要求した権限と `metadata:read` に収まり、`repositories` が要求した対象と一致し、`repository_selection` が `selected` のときだけトークンを返します。
どちらの発行もトークンが返ったため、発行応答はこれらを満たしていました。
発行応答の `single_file`、`has_multiple_single_files`、`single_file_paths` の値は、GATES の外から見えないため記録していません。
これらのキーは、パスに `README.md` を指定した記録で、`single_file` を含む要求の発行応答にだけ現れています（[GitHub App Permissions](app-permissions.md)）。

## Installation で承認する前の発行

GitHub App に `Single file` を加えて保存し、Installation の側で承認する前に、`example-repo` と `single_file:read` で発行を要求しました。
GATES は `github_issuance_rejected` を返し、監査記録の理由は `status 422` でした。
GATES は発行 API の応答本文を記録しないため、`message` は記録していません。
承認の後の同じ要求は `201` でした。

## 指定したパスの取得

`example-repo` だけを対象にしたトークンの応答です。

| 要求 | 応答 | `X-Accepted-GitHub-Permissions` | 本文 |
|---|---|---|---|
| `GET /repos/{owner}/example-repo/contents/.github/gates-policy.yml` | `200` | `contents=read` | `type` が `file`、`size` が `58`、`encoding` が `base64`、`content` は 82 文字、`download_url` あり。復号した内容は既定ブランチのファイルと一致した |
| 同じ（`raw`） | `200` | `contents=read` | ファイルの内容そのもの（58 バイト）。`content-type` は `application/vnd.github.raw+json; charset=utf-8` |
| 同じ、`ref=main` | `200` | `contents=read` | `ref` のない要求と同じ `sha` と内容 |
| 同じ、`ref=variant` | `200` | `contents=read` | `variant` のファイル（60 バイト、別の `sha`） |

`ref` を付けない要求は、既定ブランチのファイルを返しました。
`X-Accepted-GitHub-Permissions` は `contents=read` だけを示し、`single_file` を含みませんでした。
対象 2 件のトークンでも、`example-repo` の応答は同じでした。

## 指定したパスのほかの取得

`example-repo` だけを対象にしたトークンの応答です。
本文はどれも、`message` が `Resource not accessible by integration`、`status` が `"403"` でした。

| 要求 | 応答 | `X-Accepted-GitHub-Permissions` |
|---|---|---|
| `GET .../contents/README.md`（既定と `raw`） | `403` | `contents=read` |
| `GET .../contents/.github/not-policy.yml` | `403` | `contents=read` |
| `GET .../contents/.github/nonexistent.yml`（存在しないファイル） | `403` | `contents=read` |
| `GET .../contents/.GITHUB/GATES-POLICY.YML`（大文字にしたパス） | `403` | `contents=read` |
| `GET .../contents/.github/gates-policy.yml/../../README.md`（`..` を正規化せずに送った） | `403` | `contents=read` |
| `GET .../contents/.github`（親のディレクトリ） | `403` | `contents=read` |
| `GET .../contents/`（既定と `object`） | `403` | `contents=read` |
| `GET .../git/trees/main`（`recursive=1` の有無） | `403` | `contents=read` |
| `GET .../git/blobs/{sha}`（指定したパスのファイルの blob） | `403` | `contents=read` |
| `GET .../readme` | `403` | `contents=read` |
| `GET .../tarball`、`GET .../zipball` | `403` | `contents=read` |
| `GET .../commits?per_page=1` | `403` | `contents=read` |
| `GET .../branches` | `403` | `contents=read` |
| `GET /repos/{owner}/example-repo` | `200` | `metadata=read` |

存在しないファイルも、存在するファイルと同じ `403` でした。
指定したパスにファイルがない場合の `404`（[指定したパスに有効なファイルがない場合](#指定したパスに有効なファイルがない場合)）とは応答コードで分かれます。
`GET /repos/{owner}/{repo}` は、付随する `metadata:read` で読めました。

対象 2 件のトークンでは、`example-repo-2` の `README.md` も `403` でした。
`example-repo` だけを対象にしたトークンでは、`example-repo-2` の `.github/gates-policy.yml` は `404`（`Not Found`）でした。

## 指定したパスに有効なファイルがない場合

| 状態 | 応答（既定） | 本文（既定） | 応答（`raw`） |
|---|---|---|---|
| 既定ブランチにファイルがない（`example-repo-2`、対象 2 件のトークン） | `404` | `message` が `Not Found` | `404`、本文は既定と同じ |
| ファイルがない（`missing`） | `404` | `message` が `Not Found` | `404`、本文は既定と同じ |
| 0 バイトのファイル（`empty`） | `200` | `type` が `file`、`size` が `0`、`encoding` が `base64`、`content` が空文字列 | `200`、本文は 0 バイト |
| ディレクトリ（`directory`） | `200` | 配列。要素は `inner.yml`（`type` が `file`、`path` が `.github/gates-policy.yml/inner.yml`） | `200`、本文は既定と同じ配列 |
| 指定したパスの外のファイルを指すシンボリックリンク（`symlink`） | `200` | `type` が `file`、`name` と `path` は指定したパス、`size` が `48`、内容はリンク先の `not-policy.yml` | `200`、本文はリンク先の内容 |
| 存在しないファイルを指すシンボリックリンク（`symlink-dangling`） | `200` | `type` が `symlink`、`target` が `nonexistent.yml`、`size` が `15`、`content` のキーはない | `200`、本文はリンク先の名前（15 バイト） |
| 1 MB を超えるファイル（`large`） | `200` | `type` が `file`、`size` が `1152048`、`encoding` が `none`、`content` が空文字列 | `200`、本文はファイルの内容そのもの（1,152,048 バイト） |

`object` で読んだ 1 MB を超えるファイルの応答は、既定の応答と同じでした。
シンボリックリンクがリンク先のファイルを指すとき、既定の応答の `type` は `file` で、指定したパスにあるファイルと見分けられませんでした。
指定したパスの外にある `not-policy.yml` は、直接読むと `403` でしたが、シンボリックリンクを通すと読めました。
どの状態でも、`X-Accepted-GitHub-Permissions` は `contents=read` でした。

## Installation の対象外の公開リポジトリ

`example-repo` だけを対象にしたトークンで、Installation の対象外の、同じアカウントの公開リポジトリ（`public-repo`）を読みました。
`GET .../contents/README.md` は `200` で、内容を読めました。
`GET .../contents/.github/gates-policy.yml` は、ファイルがないため `404` でした。

## 所要時間とレート制限

`example-repo` の `.github/gates-policy.yml` の取得を、接続を要求ごとに張り直して 20 回続けて行いました。
Runner は Azure の `centralus` にありました。
20 回とも `200` でした。

| 区間 | 最小 | 中央値 | 90 パーセンタイル | 最大 |
|---|---|---|---|---|
| TCP の接続まで | 0.028 秒 | 0.035 秒 | 0.040 秒 | 0.041 秒 |
| TLS の確立まで | 0.054 秒 | 0.065 秒 | 0.073 秒 | 0.076 秒 |
| 応答の最初のバイトまで | 0.187 秒 | 0.238 秒 | 0.274 秒 | 0.286 秒 |
| 全体 | 0.187 秒 | 0.238 秒 | 0.274 秒 | 0.286 秒 |

この表のほかの、2 つのジョブの Contents API の要求の全体の時間は 0.07 秒から 0.40 秒でした（Runner は `centralus` と `eastus2`）。
0.40 秒は、1 MB を超えるファイルを `raw` で読んだ要求です。

どの応答にも `x-ratelimit-resource: core` と `x-ratelimit-limit: 5000` があり、取得 1 回ごとに `x-ratelimit-used` が 1 増えました。
`DELETE /installation/token` の応答にも、`x-ratelimit-resource: core` がありました。
`x-ratelimit-remaining` は、一続きの要求の途中で `4986` から `4999` に戻るなど、増える場合がありました。

同じ時間帯に GATES の Lambda（`ap-northeast-1`）が発行を処理した時間は、Lambda の `REPORT` の `Duration` で 663 ms から 825 ms でした（4 回）。
この時間には、OIDC JWT の検証、KMS の署名、Installation の取得、発行 API の呼び出しが含まれます。
Lambda から Contents API を呼んだ時間は測っていません。

## 失効の後の取得

`example-repo` だけを対象にしたトークンで、取得を続けた後に `DELETE /installation/token` を呼びました。
応答は `204` で、本文はありませんでした。
その直後の取得は `401`（`message` が `Bad credentials`）でしたが、5 秒待った後の取得は `200` で、ファイルの内容を読めました。
同じトークンでの Docker Action の post の失効は `401` でした。

この振る舞いを確かめるため、別の実行で、`single_file:read` と `contents:read` のトークンをそれぞれ発行し、`204` の後に約 2 秒ごとに約 3 分間、ファイルの取得と `GET /installation/repositories` を呼びました。
どちらのトークンでも、180 回の要求はすべて `401` で、失効の後に通った要求はありませんでした。
最後に呼んだ `DELETE /installation/token` は、どちらも `401` でした。

## 確かめていないこと

- 空のリポジトリ（コミットのないリポジトリ）での取得の応答
- 発行応答の `single_file_paths` の値と、Installation の取得の応答に現れるパス
- GitHub App の `Single file` のパスが `.github/gates-policy.yml` と異なるときの取得の応答。指定していないパスの取得（`403`）から推測できるが、パスを変えて試してはいない
- `single_file:write` のトークンでの書き込み
- Organization の Installation での振る舞い
- 失効の後に要求が通る現象の起きる条件と、続く時間
