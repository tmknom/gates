# actionlint

GitHub Actions のワークフローを静的解析する actionlint を、隔離したコンテナで実行します。

## ラップしているもの

| 対象 | 取得元 | バージョンの固定先 |
|---|---|---|
| actionlint と shellcheck | `docker.io/rhysd/actionlint` | `compose.yaml` の `image` のタグと digest |

追加するものがないため Dockerfile を持たず、公式イメージを直接使います。
`run` の中のスクリプトは、イメージに入っている shellcheck で検査します。
バージョンを変えるときは、タグと digest を一緒に書き換えます。

## 使い方

リポジトリのルートで、サービス `actionlint` の後に検査するワークフローのファイルを続けて実行します。
検査で指摘があれば、終了コードは 1 になります。

```sh
ACTIONLINT_TARGET="$PWD/.github" docker compose -f scripts/actionlint/compose.yaml run --rm -T --interactive=false \
  actionlint .github/workflows/*.yml
```

## 環境変数

次の変数を指定します。

| 変数 | 内容 |
|---|---|
| `ACTIONLINT_TARGET` | リポジトリの `.github` ディレクトリの絶対パス。リポジトリのルートではなく、検査する `.github` に絞る |

## 設計と実装

全コンテナに共通する方針は [コンテナのセキュリティ設計](../container-design.md) に定めています。

**ファイルを引数で渡す理由**：actionlint は、引数がないと作業ディレクトリから `.git` をたどってプロジェクトを探します。
コンテナには `.github` だけを渡し `.git` を渡さないため、検査するファイルを引数で渡します。
