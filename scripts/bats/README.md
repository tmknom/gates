# bats

Composite Action のスクリプトの試験を、bats-core で隔離したコンテナで実行します。

## ラップしているもの

| 対象 | 取得元 | バージョンの固定先 |
|---|---|---|
| bats-core と Alpine | `docker.io/bats/bats` | `Dockerfile` の `FROM` のタグと digest |
| jq | `jqlang/jq` の GitHub リリースの静的バイナリ | `Dockerfile` の URL と、アーキテクチャごとの SHA-256 |

jq の SHA-256 は、リリースの `sha256sum.txt` の値です。
版を変えるときは、URL と amd64、arm64 の両方の値を一緒に書き換えます。

ベースイメージの Alpine のパッケージは、ビルドのときに `apk upgrade` で更新します。
ベースイメージの公開後に修正された脆弱性を取り込むためで、内容は Alpine のパッケージの署名で検証し、版は固定しません。
層のキャッシュがあると更新しないため、取り込み直すときはキャッシュなしでビルドします。

```sh
BATS_TARGET="$PWD/.github/actions/issue-token/scripts" docker compose -f scripts/bats/compose.yaml build --no-cache bats
```

## 使い方

リポジトリのルートで、サービス `bats` の後に bats の引数を続けて実行します。
`make test-action` は、Composite Action の `scripts/tests/` をすべて実行します。

```sh
BATS_TARGET="$PWD/.github/actions/issue-token/scripts" docker compose -f scripts/bats/compose.yaml run --build --rm -T --interactive=false \
  bats tests
```

GitHub と AWS には接続しません。

## 環境変数

次の変数を指定します。

| 変数 | 内容 |
|---|---|
| `BATS_TARGET` | Composite Action のスクリプトのディレクトリ（`scripts/`）の絶対パス。試験もその中に置く。`action.yml` やリポジトリのルートではなく、そのディレクトリに絞る |

## 設計と実装

全コンテナに共通する方針は [コンテナのセキュリティ設計](../container-design.md) に定めています。
ローカルの試験で確かめる事項は [GitHub Actions Design](../../docs/design/github-actions.md#61-local-tests) に記述しています。

**一時ファイル**：bats は試験の一時ファイルを `/tmp` の tmpfs に作ります。
`/tmp` は `noexec` のため、試験が実行するスクリプトとスタブは、読み取り専用でマウントしたスクリプトのディレクトリに置きます。

**entrypoint**：公式イメージの entrypoint は tini を経由しますが、`compose.yaml` の `init: true` が PID 1 の init を置くため、bats を直接起動します。

**取り除くもの**：試験を並列に実行しないため、公式イメージの GNU parallel と、それだけが使う perl を取り除きます。
