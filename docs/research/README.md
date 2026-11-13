# Research

## 概要

本ディレクトリでは、外部仕様を調査した結果を、調査対象ごとのディレクトリで管理します。
各ファイルは、取得した値と、その取得日や取得方法を記録します。

## ドキュメント一覧

| ファイル | 内容 |
|---|---|
| [GitHub OIDC Discovery](github/oidc-discovery.md) | GitHub OIDC の Discovery エンドポイントから取得した応答 |
| [GitHub OIDC JWKS](github/oidc-jwks.md) | GitHub OIDC の JWKS エンドポイントから取得した応答 |
| [GitHub OIDC JWT](github/oidc-jwt.md) | GitHub Actions の実環境で発行された OIDC JWT の Claim を、起動条件ごとに取得した値 |
| [GitHub OIDC Custom Audience](github/custom-audience.md) | Audience を指定して取得した OIDC JWT の形と、GitHub が発行を拒否した Audience |
| [GitHub App Installation](github/app-installation.md) | GitHub App JWT でユーザーアカウントの Installation を取得した応答と、署名を壊した GitHub App JWT への `401` の応答 |
| [GitHub Installation Access Token](github/installation-access-token.md) | 対象と権限を明示した Installation Access Token の発行と失効の応答 |
| [GitHub App Installation Changes](github/installation-changes.md) | Installation の対象の追加と削除、停止、アンインストールと再インストール、GitHub App の権限の追加と承認と削除のたびに、Installation の取得と発行が返した結果 |
| [GitHub Composite Action Outputs](github/composite-action-outputs.md) | Composite Action の内部でマスクした値を Output で呼び出し元に渡したときの、マスクの範囲、値の受け渡し、ジョブの Output での扱い |
| [GitHub Docker Action Runtime](github/docker-action-runtime.md) | Docker Action のコンテナに届く入力と OIDC の環境変数、マウント、root 以外のユーザーでの書き込み、Output とマスク、post-entrypoint への状態の受け渡し、イメージのビルドの時間 |
| [GitHub Docker Action Failure and Cancellation](github/docker-action-cancellation.md) | 失敗したジョブとキャンセルしたジョブでの post-entrypoint の実行、ワークフロー実行のキャンセルがステップに届くまでの時間、`docker run` のプロセスに送った SIGINT への Action の応じ方 |
| [AWS Lambda Invoke Rejection](aws/lambda-invoke-rejection.md) | IAM と Lambda の Invoke API が関数の実行前に拒否した呼び出しと、JSON の解釈が分かれる Payload を関数に渡すか |
| [AWS STS Legacy Subject](aws/sts-legacy-subject.md) | 名前だけの旧形式の `sub` の OIDC JWT で IAM Role の引き受けを試みた結果 |
