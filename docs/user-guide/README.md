# User Guide

本ディレクトリでは、GATES を導入し、運用し、ワークフローから使う人のための文書を管理します。

## 導入を検討する

| ファイル | 内容 |
|---|---|
| [GATES Overview](overview.md) | GATES が解決する問題、仕組み、発行されるトークン、現在の範囲と制約、役割と責任、運用者が用意するものと費用が発生するリソース |
| [Security Considerations](security-considerations.md) | GATES が守るもの、許可オーナーのワークフローが読めるリポジトリ、トークンを受け取るジョブの中のコード、運用者の AWS 権限が届く範囲、トークンの寿命と失効、監査で追えることと追えないこと |

## GATES を運用する

| ファイル | 内容 |
|---|---|
| [Deployment](deployment.md) | GATES を初めてデプロイするチュートリアル。前提の確認、GitHub App の作成とインストール、配布物の検証、`terraform.tfvars` の値の決め方、Terraform の適用とデプロイしたコードの照合、鍵素材のインポートと鍵長が違ったときの扱い、接続情報の登録、最初の発行と監査記録の確認 |
| [Release Verification](verify-release.md) | GitHub Releases の配布物の来歴をチェックサムと Attestation で検証する手順と、デプロイした Lambda の関数のコードを照合する手順、照合で確かめられる範囲 |
| [Key Material Import](import-key-material.md) | GitHub App Private Key を生成して鍵素材として KMS Key にインポートし、GitHub App の鍵と一致することを確かめる手順と、インポートが確かめることと止まったときの扱い |
| [Key Rotation](rotate-key.md) | GitHub App Private Key を更新する手順。新しい KMS Key を加えて鍵素材をインポートし、使用中の鍵を切り替えて発行を確かめ、旧 KMS Key の無効化、GitHub からの旧 Private Key の削除、旧 KMS Key の State からの除外と削除の予約までを、切り戻しの方法と切り戻せなくなる時点とともに示す |
| [Operations](operations.md) | デプロイの後に運用者が行う作業の手順。インストール対象の変更、監査記録の確認とよく使うクエリ、`terraform.tfvars` の値の変更、新しいリリースへの更新と前のリリースへの戻し方、鍵の更新を行う契機、悪用が疑われるときの停止の手段の選び方と、停止と再開の手順 |

## ワークフローから使う

| ファイル | 内容 |
|---|---|
| [Using GATES in Workflows](workflow-usage.md) | トークンを受け取るワークフローの書き方。運用者から受け取るものと Runner の要件、最小のワークフローと Action をコミットの SHA で参照する理由、GitHub CLI、`actions/checkout`、Git でのトークンの使い方とできないこと、要求の書き方、信頼できないコードとトークンを受け取るジョブの分け方、失敗したときの読み方 |

## 失敗を調べる

| ファイル | 内容 |
|---|---|
| [Troubleshooting](troubleshooting.md) | Action の失敗の注釈、`request_id`、ワークフロー実行から、原因と直し方を引くリファレンス。ワークフロー利用者が直せる失敗、運用者に伝える情報、監査記録の探し方、エラーコードごとの原因と対処、`request_id` がない失敗の調べ方、失効の警告 |
