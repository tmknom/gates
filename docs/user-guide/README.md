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
| [Release Verification](verify-release.md) | GitHub Releases の配布物の来歴をチェックサムと Attestation で検証する手順と、デプロイした Lambda の関数のコードを照合する手順、照合で確かめられる範囲 |
| [Key Material Import](import-key-material.md) | GitHub App Private Key を生成して鍵素材として KMS Key にインポートし、GitHub App の鍵と一致することを確かめる手順と、インポートが確かめることと止まったときの扱い |
| [Key Rotation](rotate-key.md) | GitHub App Private Key を更新する手順。新しい KMS Key を加えて鍵素材をインポートし、使用中の鍵を切り替えて発行を確かめ、旧 KMS Key の無効化、GitHub からの旧 Private Key の削除、旧 KMS Key の State からの除外と削除の予約までを、切り戻しの方法と切り戻せなくなる時点とともに示す |
