# Testing Environment Setup

GATES の動作を検証するには、トークンの発行元となる GitHub App と、デプロイした GATES が必要です。
検証環境の構築は、運用者向けの [Deployment](../user-guide/deployment.md) の手順に従います。
本ドキュメントには、検証環境に固有の値と手順の差分だけを書きます。

## 1. GitHub App

検証用の GitHub App は、開発者の個人アカウントに作り、そのアカウントを許可オーナーにします。
Deployment の §2.1 のうち、運用者が決める項目は次の値にします。
ほかの項目は Deployment のとおりです。

| 項目 | 設定値 |
|---|---|
| GitHub App name | `Testing GATES` |
| Description | `A GitHub App for testing GATES (GitHub App Token Exchange System), a system that issues GitHub App installation access tokens to GitHub Actions workflows.` |
| Homepage URL | `https://example.com` |

Deployment の Repository permissions は、[E2E Tests](e2e-tests.md) の事例が使う次の水準にします。

| 権限 | 水準 | 使う事例 |
|---|---|---|
| `Contents` | `Read and write` | 書き込みの発行と、`contents:read` の発行 |
| `Pull requests` | `Read-only` | 複数の権限の発行と、付与した水準より高い `pull_requests:write` の要求の拒否 |
| `Commit statuses` | `Read-only` | GitHub App に付与されていて、発行上限に含まれない `statuses:read` の要求の拒否 |
| `Issues` | 付与しない | 付与されていない `issues:read` の要求の拒否 |

`Administration`、`Workflows`、`Secrets`、`Actions` の書き込みは、検証用の GitHub App にも付与しません。
インストール先のリポジトリも、許可オーナーのどのワークフローからでも、発行したトークンで操作されるためです。

## 2. Repositories to Install

Deployment のインストール対象には、[E2E Tests](e2e-tests.md) が読み取りと一時的な ref の作成を試すリポジトリだけを加えます。
E2E テストは、インストール対象の 2 つの非公開リポジトリと、インストール対象に含めない許可オーナーの公開リポジトリを使います。
どのリポジトリをどの Variables に登録するかは、E2E Tests の Setup にあります。

GATES を呼び出す試験用のリポジトリそのものは、インストール対象に含めません。
E2E テストは、試験用のリポジトリを要求に含めたときに発行が拒否されることも確かめるためです。

## 3. Deployment

Deployment の §1 から §7 に従って、AWS リソースを適用し、鍵素材をインポートし、接続情報を試験用のリポジトリに登録します。
`terraform.tfvars` の発行上限（`issuable_permissions`）は、次の値にします。

```hcl
issuable_permissions = [
  "contents:read",
  "contents:write",
  "metadata:read",
  "pull_requests:read",
  "pull_requests:write",
  "issues:read",
]
```

`pull_requests:write` と `issues:read` は GitHub App に付与しませんが、発行上限には含めます。
これらの要求を GATES の認可で止めずに GitHub に届け、GitHub App の権限による拒否を確かめるためです。
GitHub App に付与した `statuses:read` は発行上限に含めず、発行上限による拒否を確かめます。
接続情報のほかに E2E テストが使う Variables は、E2E Tests の Setup にあります。

Deployment の最初の発行の確認には、E2E テストの `_e2e-success.yml` を使えます。

## 4. Key Rotation

[Key Rotation](../user-guide/rotate-key.md) の手順の中で発行を確かめるときは、E2E テストの `_e2e-success.yml` を使います。
