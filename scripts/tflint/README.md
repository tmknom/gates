# tflint

Terraform のコードを静的解析する tflint を、隔離したコンテナで実行します。

## ラップしているもの

| 対象 | 取得元 | バージョンの固定先 |
|---|---|---|
| tflint | `ghcr.io/terraform-linters/tflint` | `Dockerfile` の `FROM` のタグと digest |
| AWS ruleset | `terraform-linters/tflint-ruleset-aws` の GitHub リリース | `Dockerfile` の URL と、アーキテクチャごとの SHA-256 |

terraform ruleset は tflint に同梱されています。
AWS ruleset のバージョンを変えるときは、URL と、`amd64` と `arm64` のハッシュを一緒に書き換えます。
ハッシュには、リリースの `checksums.txt` にある zip の値を使います。

## 使い方

リポジトリのルートで実行すると、`terraform/` を再帰的に lint します。
読み取り専用の `/src` にはマウントポイントを作れないため、対象のディレクトリに `local/` を用意してから実行します。

```sh
mkdir -p terraform/local
TFLINT_TARGET="$PWD/terraform" docker compose -f scripts/tflint/compose.yaml run --build --rm -T --interactive=false lint
```

## 環境変数

次の変数を指定します。

| 変数 | 内容 |
|---|---|
| `TFLINT_TARGET` | lint するディレクトリの絶対パス |

## 設計と実装

全コンテナに共通する方針は [コンテナのセキュリティ設計](../container-design.md) に定めています。

**プラグインの配置**：実行時はネットワークを遮断するため、AWS ruleset はビルド時にイメージへ入れ、`TFLINT_PLUGIN_DIR` で参照します。
そのため `.tflint.hcl` に `source` を書かず、`tflint --init` による取得も使いません。

**deep check の無効化**：deep check は AWS の認証情報とネットワークを必要とするため、`deep_check = false` にしています。
実際の AWS リソースと照合するルールは動きません。

**手元の値の隠蔽**：State や `.tfvars` などの手元の値を置く `local/` を、空の tmpfs で覆います。
`local/` の外に置かれた手元の値はコンテナから読めます。

**設定ファイル**：`.tflint.hcl` は検査対象の外の `/config` にマウントします。
`--recursive` と一緒に `--config` で指定し、すべてのディレクトリに同じ設定を適用します。
