# AWS OIDC JWT

本ドキュメントは、AWS 用 OIDC JWT の Claim に GATES が課す条件と、その条件が保証する範囲を定めます。
AWS 用 OIDC JWT は GitHub Actions OIDC Provider が発行し、AWS STS が検証する JWT です。
IAM Role の引き受けに使い、同じジョブで取得する GATES 用 OIDC JWT とは Audience が異なります。

検証そのものは AWS STS が担当し、Lambda はこの JWT を受け取らず再検証しません。
GATES は IAM Role の Trust Policy と許可する操作を設計します。
具体的な JSON と Terraform 設定、`StringLike` の式、ワイルドカードの危険性、名前変更時の更新方法は AWS のリソース設計で定めます。

各条件には、AWS 用 OIDC JWT を表す `AOJ-` で始まる ID を付けます。

## Claim Conditions

| ID | Claim | 外部仕様 | GATES の設計 | 保証しないこと |
|---|---|---|---|---|
| AOJ-ISS | `iss` | GitHub Actions OIDC の固定 Issuer | IAM OIDC Provider の信頼先を `https://token.actions.githubusercontent.com` に固定 | Lambda 内で AWS 用 JWT を検査すること |
| AOJ-AUD | `aud` | GitHub で要求時に指定可能 | IAM で `sts.amazonaws.com` と一致させる | GATES 用 JWT の Audience まで保証すること |
| AOJ-SUB | `sub` | GitHub の旧形式と不変形式が混在 | 許可オーナー名と不変 Owner ID を含む接頭部分を固定し、旧形式を拒否 | Repository ID とコンテキストの完全な構文検証 |
| AOJ-OTHER-CLAIMS | その他 | GitHub が追加 Claim を提供し得る | 現在の Trust Policy の認可条件には使わない | その他の Claim が不正・欠落なら Lambda で検証されること |

## Traceability

| GATES の条件 | 要求上の根拠 |
|---|---|
| AWS STS の入口制御 | [Requirements FR-005、FR-025](../spec/requirements.md#51-authentication) |

## Verification Cases

| 対象 | 受理例・拒否例の焦点 | 期待結果 |
|---|---|---|
| IAM の境界 | 不変 Owner ID 入りの `sub`、旧形式、別オーナーの `sub` | 許可オーナーの接頭部分を制限し、全構文の検証とは区別 |

## Sources

- [GitHub Constraints](../spec/github-constraints.md) — GitHub 公式資料、旧形式と不変形式の `sub`、相違・未確認事項
- [AWS STS Legacy Subject](../research/aws/sts-legacy-subject.md) — 旧形式の `sub` で IAM Role の引き受けを試みた結果
- [GitHub — OpenID Connect reference](https://docs.github.com/en/actions/reference/security/oidc)
