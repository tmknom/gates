# Testing Environment Setup

GATES の動作を検証するには、トークンの発行元となる GitHub App と、デプロイした GATES が必要です。
検証環境の構築は、運用者向けの [Deployment](../user-guide/deployment.md) の手順に従います。
本ドキュメントには、検証環境に固有の値と手順の差分だけを書きます。

## 1. GitHub App

検証用の GitHub App は、開発者の個人アカウントに作り、そのアカウントを許可オーナーにします。
Deployment の §3.1 のうち、運用者が決める項目は次の値にします。
ほかの項目は Deployment の §3.1 のとおりです。

| 項目 | 設定値 |
|---|---|
| GitHub App name | `Testing GATES` |
| Description | `A GitHub App for testing GATES (GitHub App Token Exchange System), a system that issues GitHub App installation access tokens to GitHub Actions workflows.` |
| Homepage URL | `https://example.com` |

## 2. Repositories to Install

Deployment の §3.2 のインストール対象には、[E2E Tests](e2e-tests.md) が読み取りを試すリポジトリだけを加えます。
E2E テストは、インストール対象の 2 つの非公開リポジトリと、インストール対象に含めない許可オーナーの公開リポジトリを使います。
どのリポジトリをどの Variables に登録するかは、E2E Tests の Setup にあります。

GATES を呼び出す試験用のリポジトリそのものは、インストール対象に含めません。
E2E テストは、試験用のリポジトリを要求に含めたときに発行が拒否されることも確かめるためです。

## 3. Deployment

Deployment の §2 から §8 に従って、AWS リソースを適用し、鍵素材をインポートし、接続情報を試験用のリポジトリに登録します。
接続情報のほかに E2E テストが使う Variables は、E2E Tests の Setup にあります。

Deployment の §9 の最初の発行の確認には、E2E テストの `_e2e-success.yml` を使えます。

## 4. Key Rotation

[Key Rotation](../user-guide/rotate-key.md) の手順の中で発行を確かめるときは、E2E テストの `_e2e-success.yml` を使います。
