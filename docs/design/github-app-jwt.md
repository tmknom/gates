# GitHub App JWT

本ドキュメントは、GATES が GitHub App JWT を生成するときの条件を定めます。
GitHub App JWT は、GitHub API に GitHub App として認証するために Lambda が構築し、KMS が署名する JWS です。
GATES はこの JWT で、Installation の取得と Installation Access Token の発行を要求します。
OpenID Connect ID Token ではないため、OIDC の `sub`、`aud`、`nonce` に関する Claim 要求を機械的に追加しません（[GitHub Constraints §4.2](../spec/github-constraints.md#42-github-app-jwt)、[GitHub App JWT 公式資料](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app)）。

各条件には、GitHub App JWT を表す `GAJ-` で始まる ID を付けます。

## Claim Summary

| ID | Claim | GitHub 公式仕様 | GATES の生成条件 |
|---|---|---|---|
| GAJ-ISS | `iss` | GitHub App の App ID または Client ID。Client ID 推奨 | 固定設定の GitHub App Client ID |
| GAJ-IAT | `iat` | 発行時刻。60秒過去への設定を推奨 | 生成時刻から60秒前の NumericDate |
| GAJ-EXP | `exp` | 失効時刻。現在から最大10分先 | 生成時刻の5分後の NumericDate |

## GAJ-ISS `iss`

**GitHub 公式仕様**：GitHub App の App ID または Client ID を指定できます。GitHub は Client ID を推奨しています。

**GATES の生成条件**：デプロイ時の固定設定にある GitHub App Client ID を設定します。Installation ID、Owner ID、ワークフローの値を `iss` に使いません。GitHub API はこの発行者情報と署名鍵から App を検証します。

## GAJ-IAT `iat`

**標準仕様**：RFC 7519 §4.1.6 の NumericDate。

**GitHub 公式仕様**：60秒過去に設定することは時計差対策の推奨で、MUST ではありません。

**GATES の生成条件**：生成時刻の60秒前を JSON 数値の整数として設定します。UTC の秒単位の値を用います。

## GAJ-EXP `exp`

**標準仕様**：RFC 7519 §4.1.4 の NumericDate。

**GitHub 公式仕様**：現在時刻から最大10分先まで。

**GATES の生成条件**：生成時刻の5分後を JSON 数値の整数として設定します。GitHub に渡すときの有効期限は GitHub が検証するもので、GATES が利用者に返す Installation Access Token の有効期限とは別です。

5分は GitHub の上限の10分より短い値です。Lambda の時計が GitHub の時計より進んでいると、GitHub からは `exp` が5分より先に見えて10分の上限に近づき、遅れていると期限が早く来ます。5分はどちらの向きにも数分のずれを許します。生成から使い終えるまでの時間がこれより短いことは、GitHub App JWT を使うコンポーネントの設計が確かめます。

## GAJ-HEADER Header と署名

JOSE Header は `alg` と `typ` の2つだけとし、`alg` は `RS256`、`typ` は `JWT` に固定します。Base64URL（paddingなし）の JWS Compact Serialization として生成します。`RS256` は SHA-256 と RSA PKCS#1 v1.5 署名の組です。

`typ` は RFC 7519 §5.1 で任意ですが、置きます。GitHub が Installation の取得を受け付けた調査記録の GitHub App JWT がこの Header だったためです（[GitHub App Installation](../research/github/app-installation.md)）。`typ` のない GitHub App JWT を GitHub が受け付けるかは確かめていません。

## Traceability

| GATES の生成条件 | 要求上の根拠 |
|---|---|
| GitHub App JWT の生成、秘密鍵の非抽出、KMS 署名 | [Requirements FR-013](../spec/requirements.md#53-token-issuance) |

## Verification Cases

| 対象 | 確認の焦点 | 期待結果 |
|---|---|---|
| GitHub App JWT | Client ID、`iat=now-60`、`exp=now+300`、`alg=RS256` と `typ=JWT` だけの Header、RS256/KMS | GitHub 公式上限を満たす JWT を生成 |

## Sources

- [GitHub Constraints](../spec/github-constraints.md) — GitHub App JWT の GitHub 公式仕様
- [GitHub App Installation](../research/github/app-installation.md) — GitHub が受け付けた GitHub App JWT の Header
- [RFC 7519 — JSON Web Token](https://www.rfc-editor.org/rfc/rfc7519.html)
- [GitHub — Generating a JWT for a GitHub App](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app)
