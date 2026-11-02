# Scripts

## 概要

本ディレクトリでは、外部のツールを隔離して実行するコンテナの定義を、ツールごとのディレクトリで管理します。
全コンテナに共通する方針は [コンテナのセキュリティ設計](container-design.md) に定めています。

## コンテナ一覧

| コンテナ | 内容 |
|---|---|
| [Alloy](alloy/README.md) | 形式モデルを検証する |
| [golangci-lint](golangci-lint/README.md) | Go のコードを静的解析する |
| [Terraform](terraform/README.md) | AWS のインフラを管理する |
| [tflint](tflint/README.md) | Terraform のコードを静的解析する |
| [trivy](trivy/README.md) | リポジトリの脆弱性、設定の不備、シークレットを検査する |

各コンテナの README は、`docker compose` での実行を記述しています。

## make での実行

リポジトリのルートで実行します。
環境変数は、下記の AWS の認証情報を除いて Makefile が設定します。

| コマンド | コンテナ | 処理 | Docker のほかに必要なもの |
|---|---|---|---|
| `make alloy` | Alloy | `formal/` の `authorization.als` と `issuance-temporal.als` にある `run` と `check` をすべて実行する | |
| `make lint` | golangci-lint | リポジトリのルートのモジュールを lint する | Go と、取得済みの依存モジュール |
| `make tflint` | tflint | `terraform/` を再帰的に lint する | |
| `make trivy` | trivy | リポジトリのルートを検査する | |
| `make trivy-update` | trivy | イメージをキャッシュなしでビルドし直し、脆弱性 DB とチェックを更新する | |
| `make tf-lock` | Terraform | lock ファイルを作り直す | |
| `make tf-init` | Terraform | イメージの Provider で初期化する | |
| `make tf-fmt` | Terraform | コードをフォーマットする | |
| `make tf-check` | Terraform | フォーマットの確認、`validate`、`test` を実行する | |
| `make tf-plan` | Terraform | デプロイパッケージをビルドし、Plan を `terraform/local/plan.tfplan` に保存する | AWS の認証情報、Go、`zip` |
| `make tf-show` | Terraform | 保存した Plan を表示する | |
| `make tf-apply` | Terraform | 保存した Plan を適用する | AWS の認証情報 |

Alloy のモデルを追加するときは、Makefile の `alloy` に実行の行を追加します。

### Terraform の変更の進め方

通常の変更は次の順に進めます。
Plan の前に、`terraform/terraform.tfvars.example` を参考に `terraform/local/terraform.tfvars` を作ります。

```sh
make tf-init
make tf-fmt
make tf-check
make tf-plan
make tf-show
make tf-apply
```

`make tf-plan` と `make tf-apply` は、次の AWS の一時的な認証情報をシェルから引き継ぎます。
すべて必須で、`AWS_SESSION_TOKEN` がない長期のアクセスキーでは実行できません。

- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_SESSION_TOKEN`

`make tf-lock`、`make tf-plan`、`make tf-apply` は、実行後に proxy のログを出力してから proxy を止めます。
通信した宛先はこのログで確認できます。

`make tf-plan` は古い Plan を先に捨ててからデプロイパッケージを作り直し、そのチェックサムを `terraform/local/` に保存します。
`make tf-apply` はチェックサムの一致を確かめてから、保存した Plan を適用します。

### 対象のディレクトリの変更

Make の変数 `TF_DIR` は、Terraform と tflint のコンテナにマウントするディレクトリを上書きします。
既定は `terraform` です。
State や `.tfvars` などの手元の値の置き場所 `TF_LOCAL_DIR` は、`$(TF_DIR)/local` に決まります。
別の場所を指定したつもりで本物の State を使わないよう、`TF_LOCAL_DIR` の上書きは拒否します。

```sh
make tflint TF_DIR=path/to/terraform
```

### 実行前の確認と準備

Makefile は、コンテナを起動する前に次の処理を行います。

- `make lint`、`make trivy`、`make tflint` は、読み取り専用の `/src` にはマウントポイントを作れないため、`compose.yaml` が隠すパスをホスト側に用意する
- `make lint`、`make trivy`、`make tflint`、`make tf-fmt`、`make tf-check` は、`tf-misplaced-values` で `terraform/local/` の外に置かれた `.tfvars` や State などを検出して失敗する
