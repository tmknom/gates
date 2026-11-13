# Contributing

## 概要

本ディレクトリでは、GATES の開発と運用で従う規約と手順を管理します。

## ドキュメント一覧

| ファイル | 内容 |
|---|---|
| [Documentation Policy](docs-policy.md) | `docs/` 配下の各ドキュメントの責務、依存してよい範囲、記述が食い違ったときに正とするドキュメント、定義に付ける ID の扱い、定義と参照の書き方、テストが検査対象とする受け入れ条件の宣言を定める |
| [Testing Environment Setup](setup-environment.md) | GATES の動作の検証に使う GitHub App を個人アカウントに作成してインストールし、GATES の AWS リソースを Terraform で適用し、接続情報を GitHub に登録する手順 |
| [Key Material Import](import-key-material.md) | GitHub App Private Key を生成し、鍵素材として KMS Key にインポートし、GitHub App の鍵と一致することを確かめる手順 |
| [E2E Tests](e2e-tests.md) | デプロイした GATES を Docker Action から呼ぶ E2E テストのワークフローを試験用のリポジトリに写し、発行、失効、失敗時の挙動、キャンセル、資格情報の非出力を確かめる手順 |
| [Release](release.md) | Lambda の配布物を GitHub Releases で公開するワークフローの構成と実行の手順、失敗したときの扱い、利用者が配布物をチェックサムと Attestation で検証する手順 |
