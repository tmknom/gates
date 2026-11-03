# Contributing

## 概要

本ディレクトリでは、GATES の開発と運用で従う規約と手順を管理します。

## ドキュメント一覧

| ファイル | 内容 |
|---|---|
| [Documentation Policy](docs-policy.md) | `docs/` 配下の各ドキュメントの責務、依存してよい範囲、記述が食い違ったときに正とするドキュメントを定める |
| [Testing Environment Setup](setup-environment.md) | GATES の動作の検証に使う GitHub App を個人アカウントに作成してインストールし、GATES の AWS リソースを Terraform で適用する手順 |
| [Key Material Import](import-key-material.md) | GitHub App Private Key を生成し、鍵素材として KMS Key にインポートし、GitHub App の鍵と一致することを確かめる手順 |
