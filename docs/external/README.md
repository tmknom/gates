# External

## 概要

本ディレクトリでは、GATES が従う外部仕様（JWT と OpenID Connect の標準仕様、GitHub の仕様）を、規定として整理した文書を管理します。
`docs/research/` が外部仕様を調査して取得した値の生の記録であるのに対し、本ディレクトリはそれらと公式資料を根拠に、外部仕様を規定として整理したものです。

## ドキュメント一覧

| ファイル | 内容 |
|---|---|
| [RFC 7159 JSON](standards/rfc7159.md) | JSON の文字列の比較の条項 |
| [RFC 7515 JSON Web Signature](standards/rfc7515.md) | Base64url、JWS Compact Serialization、厳密な JSON、JOSE Header（`alg`、`kid`、`typ`、`cty`、`crit` など）、署名の検証、文字列の比較の条項 |
| [RFC 7517 JSON Web Key](standards/rfc7517.md) | JWK のメンバー（`kty`、`use`、`key_ops`、`alg`、`kid`、証明書）、JWK Set、非公開の鍵の情報の保護の条項 |
| [RFC 7518 JSON Web Algorithms](standards/rfc7518.md) | Base64urlUInt、RSASSA-PKCS1-v1_5 と RSASSA-PSS の鍵の長さ、`none`、RSA の鍵のメンバー、秘密の情報を表す鍵のメンバーの条項 |
| [RFC 7519 JSON Web Token](standards/rfc7519.md) | JWT の直列化、StringOrURI と NumericDate、登録済みの Claim、JWT の `typ` と `cty`、Unsecured JWT、JWT の検証、文字列の比較の条項 |
| [RFC 7797 JWS Unencoded Payload Option](standards/rfc7797.md) | `b64` と、JWT で `b64` を `false` にしないことの条項 |
| [RFC 8725 JWT Best Current Practices](standards/rfc8725.md) | アルゴリズム、暗号操作、UTF-8、Issuer と鍵、Audience、JWT の種類の区別の Best Current Practices の条項 |
| [RFC 9964 ML-DSA for JOSE and COSE](standards/rfc9964.md) | JWK で表す AKP 鍵の条項 |
| [OpenID Connect Core 1.0](standards/oidc-core.md) | ID Token の Claim、認証フローごとの ID Token の検証、署名、文字列の操作の条項。ID Token と認証フローに限る範囲を Condition に書く |
| [OpenID Connect Discovery 1.0](standards/oidc-discovery.md) | OpenID Provider Metadata、設定の取得と検証、文字列の操作、TLS と成りすましへの対策の条項 |
| [GitHub Actions OIDC Constraints](github/oidc.md) | GitHub Actions OIDC の Issuer、Discovery、JWKS、Claim、`sub`、Audience の仕様と未確認事項 |
| [GitHub Owner and Repository Constraints](github/account.md) | GitHub のオーナーとリポジトリの識別子と名前の仕様と未確認事項 |
| [GitHub App Constraints](github/app.md) | GitHub App、GitHub App JWT、Installation、Installation Access Token、REST API の共通事項の仕様と未確認事項 |
| [GitHub Actions Integration Constraints](github/actions.md) | GitHub Actions の OIDC JWT の取得、Output とマスク、Secrets と Variables、Dependabot、Docker Action、Runner の仕様と未確認事項 |
