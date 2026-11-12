# golangci-lint

Go のコードを静的解析する golangci-lint を、隔離したコンテナで実行します。

## ラップしているもの

| 対象 | 取得元 | バージョンの固定先 |
|---|---|---|
| golangci-lint | `docker.io/golangci/golangci-lint` | `compose.yaml` の `image` のタグと digest |

追加するものがないため Dockerfile を持たず、公式イメージを直接使います。
バージョンを変えるときは、タグと digest を一緒に書き換えます。

## 使い方

ホストの Go が必要です。
依存モジュールはホストのキャッシュから読むため、`go mod download` などで取得しておきます。

リポジトリのルートで、`compose.yaml` が隠すパスのマウントポイントを用意してから実行します。

```sh
mkdir -p .claude dist terraform/local tmp
[ -e .envrc ] || touch .envrc
LINT_MODULE="$PWD" LINT_GOMODCACHE="$(go env GOMODCACHE)" \
  docker compose -f scripts/golangci-lint/compose.yaml run --rm -T --interactive=false lint
```

対象のパッケージを絞るときは、サービス `lint` の後に golangci-lint のコマンドを続けます。
指定すると `compose.yaml` の `command` を置き換えるため、`--config` も指定します。

```sh
LINT_MODULE="$PWD" LINT_GOMODCACHE="$(go env GOMODCACHE)" \
  docker compose -f scripts/golangci-lint/compose.yaml run --rm -T --interactive=false \
  lint golangci-lint run --config=/config/.golangci.yml ./internal/...
```

Docker Action のモジュール（`.github/actions/issue-token`）は、リポジトリのルートをマウントしたまま、作業ディレクトリをそのモジュールに移して lint します（`make action-lint`）。
隠すパスはルートを渡すときと同じで、モジュールのディレクトリにマウントポイントを用意する必要がありません。

```sh
LINT_MODULE="$PWD" LINT_GOMODCACHE="$(go env GOMODCACHE)" \
  docker compose -f scripts/golangci-lint/compose.yaml run --rm -T --interactive=false \
  --workdir /src/.github/actions/issue-token lint
```

## 環境変数

次の変数をすべて指定します。

| 変数 | 内容 |
|---|---|
| `LINT_MODULE` | `/src` にマウントするディレクトリの絶対パス。lint する Go モジュールか、それを含むリポジトリのルート |
| `LINT_GOMODCACHE` | ホストのモジュールキャッシュの絶対パス。`go env GOMODCACHE` の値を指定する |

## 設計と実装

全コンテナに共通する方針は [コンテナのセキュリティ設計](../container-design.md) に定めています。

**モジュールの解決**：ネットワークを遮断したまま解析できるよう、ホストのモジュールキャッシュを読み取り専用でマウントします。
`GOPROXY=off` と `GOFLAGS=-mod=readonly` で、取得と `go.mod` の更新を止めます。
`GOTOOLCHAIN=local` で別のツールチェーンの取得も止めるため、`go.mod` が求める Go のバージョンはイメージの Go で満たす必要があります。

**不要なパスの隠蔽**：モジュールのルート全体を渡すため、機密ファイルの置き場所など、lint に不要なパスを隠します。
ディレクトリは空の tmpfs で覆い、`.envrc` には `/dev/null` を重ねます。
読み取り専用の `/src` にはマウントポイントを作れないため、これらのパスはホスト側に用意します。
別のディレクトリを `LINT_MODULE` に指定するときは、`compose.yaml` が隠すパスをそのディレクトリにも用意します。

**キャッシュ**：ビルドと解析のキャッシュは `/tmp` の tmpfs に置きます。
コンテナの終了とともに消えるため、実行のたびにすべてを解析します。

**設定ファイル**：`.golangci.yml` は検査対象の外の `/config` にマウントします。
既定ではパスが `/config` を基準にして `../src/` で始まるため、`run.relative-path-mode` を `gomod` にしています。
`linters.default` が `none` のため、有効な linter は `linters.enable` に列挙したものだけです。

**2 つのモジュールの規則**：1 つの `.golangci.yml` で、ルートのモジュールと Docker Action のモジュールの両方を lint します。
depguard の `files` は絶対パスと照合するため、`.github/` の有無で規則を分けます。
除外の `path` はモジュールのルートを基準にしたパスと照合し、2 つのモジュールを区別できないため、ルートのモジュールと重ならないメッセージで限ります。
