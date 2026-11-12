# trivy

リポジトリの脆弱性、設定の不備、シークレットを検査する trivy を、隔離したコンテナで実行します。

## ラップしているもの

| 対象 | 取得元 | バージョンの固定先 |
|---|---|---|
| trivy | `ghcr.io/aquasecurity/trivy` | `Dockerfile` の `FROM` のタグと digest |
| 脆弱性 DB、misconfig のチェック | trivy の配布元 | 固定しない。ビルド時点の内容をイメージに持つ |

## 使い方

リポジトリのルートで、`compose.yaml` が隠すパスのマウントポイントを用意してから検査します。
`trivy.yaml` の `exit-code: 1` により、検出があれば失敗します。

```sh
mkdir -p .claude dist terraform/local tmp
[ -e .envrc ] || touch .envrc
TRIVY_TARGET="$PWD" docker compose -f scripts/trivy/compose.yaml run --rm -T --interactive=false scan
```

`run` はイメージがないときだけビルドします。
DB の更新や `Dockerfile` の変更を反映するには、キャッシュなしでビルドし直します。

```sh
TRIVY_TARGET="$PWD" docker compose -f scripts/trivy/compose.yaml build --no-cache scan
```

`trivy.yaml` と `.trivyignore.yaml` は実行時にマウントするため、ビルドせずに反映されます。

検出を無視するときは、ID と理由（`statement`）を `.trivyignore.yaml` に追加します。
特定のファイルの検出だけを無視するときは、`paths` でファイルを限ります。

検査の範囲を絞るときは、サービス `scan` の後に trivy のコマンドを続けます。
指定すると `compose.yaml` の `command` を置き換えるため、`--config` も指定します。

```sh
TRIVY_TARGET="$PWD" docker compose -f scripts/trivy/compose.yaml run --rm -T --interactive=false \
  scan fs --config=/config/trivy.yaml ./terraform
```

## 環境変数

ビルドと実行のどちらでも、次の変数を指定します。

| 変数 | 内容 |
|---|---|
| `TRIVY_TARGET` | 検査するディレクトリの絶対パス |

## 設計と実装

全コンテナに共通する方針は [コンテナのセキュリティ設計](../container-design.md) に定めています。

**データの配置**：実行時はネットワークを遮断するため、脆弱性 DB と misconfig のチェックはビルド時にイメージへ入れます。
チェックを取得するサブコマンドがないため、Dockerfile では空のディレクトリを検査して取得させます。
Docker のレイヤーキャッシュは配布元の更新を検知しないため、更新には `--no-cache` が要ります。

**通信の停止**：`trivy.yaml` で DB、Java の DB、チェック、VEX リポジトリの更新と、バージョンの確認、テレメトリを止めます。
イメージの DB が古くなっても自動では更新されないため、更新の時期は運用者が決めます。

**不要なパスの隠蔽**：リポジトリのルート全体を渡すため、機密ファイルの置き場所など、検査に不要なパスをコンテナから隠し、検査対象からも外します。
ディレクトリは空の tmpfs で覆い、`.envrc` には `/dev/null` を重ねます。
読み取り専用の `/src` にはマウントポイントを作れないため、これらのパスはホスト側に用意します。
別のディレクトリを `TRIVY_TARGET` に指定するときは、`compose.yaml` が隠すパスをそのディレクトリにも用意します。
