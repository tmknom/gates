# External

## 概要

本ディレクトリでは、GATES が従う外部仕様（JWT と OpenID Connect の標準仕様、GitHub の仕様）を、規定として整理した文書を管理します。
`docs/research/` が外部仕様を調査して取得した値の生の記録であるのに対し、本ディレクトリはそれらと公式資料を根拠に、外部仕様を規定として整理したものです。

## ドキュメント一覧

| ファイル | 内容 |
|---|---|
| [JOSE Standards](standards/jose.md) | JWS / JWK / JWA と JWS の非エンコード Payload オプションの標準仕様から、署名と鍵の検証に関係する規定を整理したもの |
| [JWT Standards](standards/jwt.md) | JSON、JWT、JWT の Best Current Practices の標準仕様から、Claims の検証と JWT の発行・検証の考慮事項に関係する規定を、ID の付いた条項に分けて整理したもの |
| [OpenID Connect Standards](standards/oidc.md) | OpenID Connect Core と Discovery の標準仕様から、ID Token、署名鍵、Discovery の検証に関係する規定を整理したもの |
| [GitHub Actions OIDC Constraints](github/oidc.md) | GitHub Actions OIDC の Issuer、Discovery、JWKS、Claim、`sub`、Audience の仕様と未確認事項 |
| [GitHub Owner and Repository Constraints](github/account.md) | GitHub のオーナーとリポジトリの識別子と名前の仕様と未確認事項 |
| [GitHub App Constraints](github/app.md) | GitHub App、GitHub App JWT、Installation、Installation Access Token、REST API の共通事項の仕様と未確認事項 |
| [GitHub Actions Integration Constraints](github/actions.md) | GitHub Actions の OIDC JWT の取得、Output とマスク、Secrets と Variables、Dependabot、Docker Action、Runner の仕様と未確認事項 |
