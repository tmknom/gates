# User Guide

本ディレクトリでは、GATES を導入し、運用し、ワークフローから使う人のための文書を管理します。
読者の役割ごとに表を分け、各表は読む順に並べています。

## 導入を検討する

| ファイル | 内容 |
|---|---|
| [GATES Overview](overview.md) | GATES で得られるもの、仕組み、導入の前に知っておく制約、役割、用語、運用者が用意するものと費用 |
| [Security Considerations](security-considerations.md) | 利用者が守ること、GATES が守るもの、残るリスク、監査で追えること |

## トークンで操作されるリポジトリを管理する

| ファイル | 内容 |
|---|---|
| [Repository Policy](repository-policy.md) | リポジトリポリシーを適用するデプロイで、どの Caller にどの権限を許すかを決めるファイルの書き方と、許す前に知っておくこと |

## ワークフローから使う

| ファイル | 内容 |
|---|---|
| [Using GATES in Workflows](workflow-usage.md) | 最小のワークフロー、運用者から受け取るもの、守ること、トークンの使い方、書き込みと複数の権限 |
| [Action Reference](action-reference.md) | Action の入力の書式と検証、発行の条件、要求できる権限、出力、トークンの失効の時機 |

## GATES を運用する

| ファイル | 内容 |
|---|---|
| [Deployment](deployment.md) | GATES を初めてデプロイし、最初の発行を確かめるまでの手順 |
| [Release Verification](verify-release.md) | 配布物と、デプロイした関数のコードを検証する手順 |
| [Key Material Import](import-key-material.md) | GitHub App Private Key を KMS Key にインポートする手順 |
| [Operations](operations.md) | インストール対象、権限、保護対象の一覧の変更、リポジトリポリシーの適用の切り替え、監査記録の確認、設定の変更、リリースの更新 |
| [Key Rotation](rotate-key.md) | GitHub App Private Key を新しい鍵に替える手順 |
| [Stopping GATES](stop-issuance.md) | 悪用が疑われるときに、発行や署名を止める手順 |

## 失敗を調べる

| ファイル | 内容 |
|---|---|
| [Troubleshooting](troubleshooting.md) | Action の失敗の注釈から、原因と直し方を引くリファレンス |
