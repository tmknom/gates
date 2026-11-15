# Scripts

## 概要

本ディレクトリでは、外部のツールを隔離して実行するコンテナの定義を、ツールごとのディレクトリで管理します。
全コンテナに共通する方針は [コンテナのセキュリティ設計](container-design.md) に定めています。

`kms/` だけはコンテナを使わず、運用者が手元のシェルで実行するスクリプトを置きます。
GitHub App Private Key を KMS Key にインポートするスクリプトで、秘密鍵を扱うため、Coding Agent は実行しません。
使い方は [Key Material Import](../docs/contributing/import-key-material.md) にあります。

## コンテナ一覧

| コンテナ | 内容 |
|---|---|
| [actionlint](actionlint/README.md) | GitHub Actions のワークフローを静的解析する |
| [Alloy](alloy/README.md) | 形式モデルを検証する |
| [golangci-lint](golangci-lint/README.md) | Go のコードを静的解析する |
| [Terraform](terraform/README.md) | AWS のインフラを管理する |
| [tflint](tflint/README.md) | Terraform のコードを静的解析する |
| [trivy](trivy/README.md) | リポジトリの脆弱性、設定の不備、シークレットを検査する |
| [Z3](z3/README.md) | SMT-LIB の制約を解く |

各コンテナの README は、`docker compose` での実行を記述しています。

## make での実行

特定のディレクトリだけを扱う操作は、そのディレクトリの Makefile に置きます。
ルートの Makefile には、リポジトリ全体を扱う操作と、よく使う操作への委譲だけを置きます。
委譲していない操作は、`make -C <ディレクトリ> <ターゲット>` で実行します。
環境変数は、下記の AWS の認証情報を除いて Makefile が設定します。

| コマンド | コンテナ | 処理 | Docker のほかに必要なもの |
|---|---|---|---|
| `make lint` | golangci-lint | リポジトリのルートのモジュールを lint する | Go と、取得済みの依存モジュール |
| `make trivy` | trivy | リポジトリのルートを検査する | |
| `make trivy-update` | trivy | イメージをキャッシュなしでビルドし直し、脆弱性 DB とチェックを更新する | |
| `make lint-gha` | actionlint | `.github/workflows/` のワークフローを lint する | |
| `make kms-import-key-material` | なし（手元のシェル） | `KMS_KEY_NAME` の KMS Key に、`GITHUB_PRIVATE_KEY_PATH` の GitHub App Private Key をインポートし、そのファイルを削除する | AWS の認証情報、AWS CLI、OpenSSL 3 以降、`jq`、`xxd` |
| `make kms-test` | なし（手元のシェル） | `aws` をスタブに差し替え、使い捨ての鍵でインポートのスクリプトを試験する | OpenSSL 3 以降、`jq`、`xxd` |
| `make -C formal alloy` | Alloy | `formal/alloy/` の `authorization.als` と `issuance-temporal.als` にある `run` と `check` をすべて実行する。ルートの `make alloy` も同じ | |
| `make -C formal z3` | Z3 | `formal/z3/` の検査をすべて実行し、期待する判定と比べる。ルートの `make z3` も同じ | |
| `make -C .github/actions/issue-token lint` | golangci-lint | Docker Action のモジュールを lint する | Go と、取得済みの依存モジュール |
| `make -C terraform lint` | tflint | `terraform/` を再帰的に lint する | |
| `make -C terraform lock` | Terraform | lock ファイルを作り直す | |
| `make -C terraform init` | Terraform | イメージの Provider で初期化する | |
| `make -C terraform fmt` | Terraform | コードをフォーマットする | |
| `make -C terraform check` | Terraform | フォーマットの確認、`validate`、`test` を実行する | |
| `make -C terraform plan` | Terraform | デプロイパッケージをビルドし、Plan を `terraform/local/plan.tfplan` に保存する。ルートの `make tf-plan` も同じ | AWS の認証情報、Go、`zip` |
| `make -C terraform show` | Terraform | 保存した Plan を表示する | |
| `make -C terraform apply` | Terraform | 保存した Plan を適用する。ルートの `make tf-apply` も同じ | AWS の認証情報 |

KMS のインポートはルートで実行します。
`-C` で作業ディレクトリを移すと、`GITHUB_PRIVATE_KEY_PATH` の相対パスが別の場所を指すためです。

Alloy のモデルを追加するときは、`formal/Makefile` の `alloy` に実行の行を追加します。
Z3 の検査を追加するときは、Makefile ではなく `formal/z3/checks/` にファイルを追加します。

### Terraform の変更の進め方

通常の変更は次の順に進めます。
Plan の前に、`terraform/terraform.tfvars.example` を参考に `terraform/local/terraform.tfvars` を作ります。

```sh
make -C terraform init
make -C terraform fmt
make -C terraform check
make tf-plan
make -C terraform show
make tf-apply
```

`plan` と `apply` は、次の AWS の一時的な認証情報をシェルから引き継ぎます。
すべて必須で、`AWS_SESSION_TOKEN` がない長期のアクセスキーでは実行できません。

- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_SESSION_TOKEN`

`lock`、`plan`、`apply` は、実行後に proxy のログを出力してから proxy を止めます。
通信した宛先はこのログで確認できます。

`plan` は古い Plan を先に捨ててから、ルートの Makefile の `lambda-package` でデプロイパッケージを作り直し、そのチェックサムを `terraform/local/` に保存します。
`apply` はチェックサムの一致を確かめてから、保存した Plan を適用します。

`terraform/Makefile` は `terraform/` で実行したときだけ動きます。
`make -f terraform/Makefile` のように別の場所から読むと、相対パスが別のディレクトリを指し、そのディレクトリを AWS の認証情報とともにコンテナにマウントし得るためです。

### 対象のディレクトリの変更

`terraform/Makefile` の変数 `TF_DIR` は、Terraform と tflint のコンテナにマウントするディレクトリを上書きします。
既定は `terraform/` 自身で、相対パスは `terraform/` を基準にします。
State や `.tfvars` などの手元の値の置き場所 `TF_LOCAL_DIR` は、`$(TF_DIR)/local` に決まります。
別の場所を指定したつもりで本物の State を使わないよう、`TF_LOCAL_DIR` の上書きは拒否します。

```sh
make -C terraform lint TF_DIR=/path/to/terraform
```

### 実行前の確認と準備

Makefile は、コンテナを起動する前に次の処理を行います。

- `make lint`、`make -C .github/actions/issue-token lint`、`make trivy` は、読み取り専用の `/src` にはマウントポイントを作れないため、ルートの Makefile の `container-mountpoints` で `compose.yaml` が隠すパスをホスト側に用意する
- `make lint`、`make -C .github/actions/issue-token lint`、`make trivy`、`make -C terraform lint`、`make -C terraform fmt`、`make -C terraform check` は、`terraform/Makefile` の `misplaced-values` で `terraform/local/` の外に置かれた `.tfvars` や State などを検出して失敗する
