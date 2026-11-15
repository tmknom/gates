# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM docker.io/library/debian:trixie-20261104-slim@sha256:a72b4433de1cf7d6bae0c4cee74d0547f58601a643b3375f070c51600d63b6b2

# 実行時はネットワークを持たないため、Z3 はビルド時に入れる。
# 内容は Debian のパッケージの署名で検証し、バージョンを固定する。固定したバージョンがアーカイブから消えるとビルドは失敗する。
# 通信は署名で検証する apt だけのため、ca-certificates は入れない。
# 基準のイメージのパッケージに修正版のある脆弱性が残ることがあるため、ビルド時に上げる。
# ARG はビルド時だけ有効で、実行時の環境変数には残らない
ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
  && apt-get upgrade -y --no-install-recommends \
  && apt-get install -y --no-install-recommends z3=4.13.3-1 \
  && rm -rf /var/lib/apt/lists/*

USER 65534:65534

ENTRYPOINT ["z3"]
CMD ["-version"]
