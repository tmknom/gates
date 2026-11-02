# Alloy

形式モデルを検証する Alloy を、隔離したコンテナで実行します。

## ラップしているもの

| 対象 | 取得元 | バージョンの固定先 |
|---|---|---|
| Alloy の JAR | `AlloyTools/org.alloytools.alloy` の GitHub リリース | `Dockerfile` の URL と SHA-256 |
| JRE | `docker.io/library/eclipse-temurin` | `Dockerfile` の `FROM` のタグと digest |

リリースにハッシュの公開がないため、SHA-256 は取得した JAR から計算した値です。
Alloy のバージョンを変えるときは、URL とハッシュを一緒に書き換えます。

## 使い方

リポジトリのルートで、サービス `alloy` の後に Alloy の引数を続けて実行します。
次の例は、モデルにある `run` と `check` をすべて実行します。
`-c` には、実行する `run` や `check` の名前を指定できます。

```sh
ALLOY_TARGET="$PWD/formal" docker compose -f scripts/alloy/compose.yaml run --build --rm -T --interactive=false \
  alloy exec -c '*' -o /tmp/out -t none authorization.als
```

結果の読み方は [形式モデルの README](../../formal/README.md) に記述しています。

## 環境変数

次の変数を指定します。

| 変数 | 内容 |
|---|---|
| `ALLOY_TARGET` | モデルを置いたディレクトリの絶対パス。リポジトリのルートではなく、モデルのディレクトリに絞る |

## 設計と実装

全コンテナに共通する方針は [コンテナのセキュリティ設計](../container-design.md) に定めています。

**出力先**：`exec` の出力先は既定でモデルと同じディレクトリですが、`/src` は読み取り専用です。
そのため `-o` で `/tmp` の tmpfs に向けます。
`-o -` を使わないのは、解を標準出力に出す代わりに、`run` や `check` ごとの判定の表を出さなくなるためです。

**終了コード**：`check` で反例が見つかっても、終了コードは 0 です。
終了コードで判定できるよう、モデルの各 `run` と `check` に `expect` を書きます。
期待と異なる `run` や `check` があると、終了コードは 1 になります。

**JVM の設定**：コンテナに画面がないため `-Djava.awt.headless=true` を指定します。
ヒープの上限は `-XX:MaxRAMPercentage=75` で、`mem_limit` に対する割合として決まります。
