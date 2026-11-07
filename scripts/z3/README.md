# Z3

SMT ソルバーの Z3 を、隔離したコンテナで実行します。

## ラップしているもの

| 対象 | 取得元 | バージョンの固定先 |
|---|---|---|
| Z3 | Debian の `z3` パッケージ | `Dockerfile` の `apt-get install` のバージョン |
| Debian | `docker.io/library/debian` | `Dockerfile` の `FROM` のタグと digest |

パッケージの内容は、Debian のアーカイブの署名で検証します。
`apt-get update` はベースイメージの日付によらず最新のパッケージ一覧を読むため、固定したバージョンがアーカイブから消えると、ベースイメージが同じでもビルドは失敗します。
その場合は、`Dockerfile` のバージョンを Debian の現行のものに更新します。

## 使い方

リポジトリのルートで、サービス `z3` の後に Z3 の引数を続けて実行します。
次の例は、`Z3_TARGET` に置いた SMT-LIB のファイルを解きます。

```sh
Z3_TARGET="$PWD/path/to/smt" docker compose -f scripts/z3/compose.yaml run --build --rm -T --interactive=false \
  z3 example.smt2
```

引数を省略すると、Z3 のバージョンを表示します。

`make z3` は、`--entrypoint sh` で entrypoint を替え、`formal/z3/verify.sh` を実行します。
このスクリプトは検査ごとに Z3 を起動し、判定を期待するものと比べます。
結果の読み方は [Z3 Models](../../formal/z3/README.md) に記述しています。

## 環境変数

次の変数を指定します。

| 変数 | 内容 |
|---|---|
| `Z3_TARGET` | SMT-LIB のファイルを置いたディレクトリの絶対パス。リポジトリのルートではなく、そのディレクトリに絞る |

## 設計と実装

全コンテナに共通する方針は [コンテナのセキュリティ設計](../container-design.md) に定めています。

**終了コード**：`check-sat` の結果が `sat`、`unsat`、`unknown` のいずれでも、終了コードは 0 です。
結果は標準出力から読み取り、期待する結果と比べます。
入力に誤りがあると、Z3 は標準出力に `(error ...)` を出し、終了コードは 1 になります。
Z3 は処理を続けるため、標準出力には後続の `check-sat` の結果も出ます。

**実行時間**：`cpus` は CPU の使用率を抑えるだけで、実行時間に上限はありません。
停止しない探索を避けるため、`-T:<秒>` で全体の時間の上限を指定します。
上限に達すると、Z3 は標準出力に `timeout` を出して終了し、終了コードは 0 です。
