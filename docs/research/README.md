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
| [GitHub App Permissions](github/app-permissions.md) | GitHub App に設定画面のすべての権限を付与したときに Installation に現れた権限名と水準、権限名と水準を変えた発行の応答、Installation に現れない権限を除いた発行、拒否の本文、`contents:write` のトークンで試した書き込み |
| [GitHub Composite Action Outputs](github/composite-action-outputs.md) | Composite Action の内部でマスクした値を Output で呼び出し元に渡したときの、マスクの範囲、値の受け渡し、ジョブの Output での扱い |
| [GitHub Docker Action Runtime](github/docker-action-runtime.md) | Docker Action のコンテナに届く入力と OIDC の環境変数、マウント、root 以外のユーザーでの書き込み、Output とマスク、post-entrypoint への状態の受け渡し、イメージのビルドの時間、Docker ソケットを通じたホストの操作、後続のステップから読めるファイルコマンドのファイル、`run` のステップに渡る要求用トークン、GATES の Docker Action が Runner に残すファイル、post の順序 |
| [GitHub Docker Action Failure and Cancellation](github/docker-action-cancellation.md) | 失敗したジョブとキャンセルしたジョブでの post-entrypoint の実行、ワークフロー実行のキャンセルがステップに届くまでの時間、`docker run` のプロセスに送った SIGINT への Action の応じ方、Invoke の最中に `docker run` のプロセスを強制終了した後のコンテナ、post-entrypoint の後の post から試した失効済みのトークン |
| [GitHub Workflow Event Contexts](github/workflow-event-contexts.md) | `pull_request`、`pull_request_target`、Environment を使うジョブ、`workflow_run` のジョブが Docker Action でトークンを得たときの、IAM Role の引き受けの成否、`sub` の形、GATES の監査記録に残ったワークフロー起動属性と、起動しなかった `schedule`、fork からの Pull Request の仕様 |
| [GitHub Repository Identity](github/repository-identity.md) | リポジトリの改名と、削除して同じ名前での作り直しでの Repository ID、OIDC JWT と REST API がリポジトリを示す値、改名した Installation の対象を新しい名前と旧名で要求した発行 |
| [GitHub Single File Permission](github/single-file.md) | `single_file:read` だけのトークンで読めたものと読めなかったもの、指定したパスがない、空、ディレクトリ、シンボリックリンク、1 MB を超える場合の Contents API の応答、承認の前の発行、取得の所要時間とレート制限のヘッダー、失効の後に通った要求 |
| [AWS Lambda Invoke Rejection](aws/lambda-invoke-rejection.md) | IAM と Lambda の Invoke API が関数の実行前に拒否した呼び出しと、JSON の解釈が分かれる Payload を関数に渡すか |
| [AWS STS Legacy Subject](aws/sts-legacy-subject.md) | 名前だけの旧形式の `sub` の OIDC JWT で IAM Role の引き受けを試みた結果 |
| [AWS Fault Injection](aws/fault-injection.md) | KMS Key の無効化、関数の環境変数の欠落、Reserved Concurrency 0、関数のタイムアウトの短縮をそれぞれ起こして発行を要求したときの、AWS と GATES の応答、Docker Action の注釈、関数のログ、`terraform plan` の差分 |
| [AWS Lambda Environment Size](aws/lambda-environment-size.md) | Lambda の環境変数の合計の上限の数え方、デプロイした関数の環境変数の現在の合計、YAML の一覧の要素ごとのバイト数 |
