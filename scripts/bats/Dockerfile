# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM docker.io/bats/bats:1.14.0@sha256:5322b877351fda0cc435de8c6116de7d0a2ec79d7c680132a0ef329a633bc66f

# ベースイメージの公開後に修正された Alpine のパッケージの脆弱性を取り込むため、パッケージを更新する。
# 内容は Alpine のパッケージの署名で検証し、版は固定しない。
# この層のキャッシュは更新の有無を知らないため、取り込み直すときはキャッシュなしでビルドする。
# 試験を並列に実行しないため、GNU parallel と、それだけが使う perl を外す
RUN apk upgrade --no-cache \
  && apk del --no-cache parallel perl

# 実行時はネットワークを持たないため、jq はビルド時に入れる。
# Alpine のパッケージは版を固定できる期間が短いため、リリースの静的バイナリを sha256sum.txt の値で固定する。
# 値はアーキテクチャごとに異なるため、ADD --checksum ではなく RUN で検証する。版を変えるときは URL と両方の値を一緒に書き換える。
# Runner の jq より新しい版だが、既知の脆弱性を含み得る古い版に合わせるより、手元の環境の安全を優先する。
# 版の差による挙動の違い（@uri が符号化する文字など）は、GitHub Actions 上の試験で確かめる。
# TARGETARCH は BuildKit が設定する
ARG TARGETARCH
RUN set -eu; \
  case "${TARGETARCH}" in \
    amd64) sha256=b1c22172dd303f3be49e935aa56aa48a8b7a46e0bc838b4997d3bb451495870f ;; \
    arm64) sha256=8b85c817833814ddca00a144c33705546355afccf0cf39b188f3cdb48b852309 ;; \
    *) echo "unsupported architecture: ${TARGETARCH}" >&2; exit 1 ;; \
  esac; \
  wget -q -O /usr/local/bin/jq "https://github.com/jqlang/jq/releases/download/jq-1.8.2/jq-linux-${TARGETARCH}"; \
  echo "${sha256}  /usr/local/bin/jq" | sha256sum -c -; \
  chmod 0555 /usr/local/bin/jq

USER 65534:65534

# 公式イメージの entrypoint は tini を経由するが、compose.yaml の init: true が PID 1 の init を置くため、bats を直接起動する
ENTRYPOINT ["/usr/local/bin/bash", "/usr/local/bin/bats"]
