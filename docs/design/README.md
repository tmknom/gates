# Design

## 概要

本ディレクトリでは、`docs/spec/` の要求と制約を満たすための、GATES の設計文書を管理します。

## ドキュメント一覧

| ファイル | 内容 |
|---|---|
| [System Architecture](architecture.md) | コンポーネントの全体構成と処理の流れ |
| [Domain Model](domain.md) | GATES が扱う概念の名前、意味、ルール、不変条件 |
| [Security Design](security.md) | 信頼境界、認証・認可、資格情報の保護、失敗時の扱い |
| [GATES OIDC JWT Claims](gates-oidc-jwt-claims.md) | GATES 用 OIDC JWT の Claim の受理条件 |
| [GATES OIDC JWT Signature](gates-oidc-jwt-signature.md) | GATES 用 OIDC JWT の直列化、JOSE Header、Discovery、JWK の受理条件 |
| [AWS OIDC JWT](aws-oidc-jwt.md) | AWS 用 OIDC JWT の Claim に課す条件 |
| [GitHub App JWT](github-app-jwt.md) | GitHub App JWT の生成条件 |
| [Lambda Design](lambda.md) | Lambda の入出力契約、コンポーネント、外部連携 |
| [AWS Infrastructure Design](aws-infrastructure.md) | Terraform で管理する AWS リソースの構成 |
| [GitHub Actions Design](github-actions.md) | Composite Action の入出力と処理手順 |
