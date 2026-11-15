# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

# 公式イメージの tflint は修正版のある Go でビルドされていないことがあるため、同じバージョンを Docker Action のビルドと同じ Go でビルドし直す。
# モジュールは Go のチェックサムデータベースで検証される。実行用のイメージは musl のため、静的にリンクする
FROM docker.io/library/golang:1.27.4-trixie@sha256:b54b2639de8c1bfe538ec5ffc45f29b1932235ac5f6d63c059853646677112d9 AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
RUN go install -trimpath github.com/terraform-linters/tflint@v0.66.0

FROM ghcr.io/terraform-linters/tflint:v0.66.0@sha256:65b9944a9274c4989492c1d79998c2801664bf8a55fddb4bd3a9e6d73a006ddc

COPY --from=build /go/bin/tflint /usr/local/bin/tflint

ARG TARGETARCH

# 実行時はネットワークを持たないため、プラグインはビルド時に入れる。
# ハッシュはリリースの checksums.txt の zip の値で、URL のバージョンを変えるときは一緒に書き換える。
# 基準のイメージのパッケージに修正版のある脆弱性が残ることがあるため、ビルド時に上げる
RUN set -eux; \
  apk upgrade --no-cache; \
  case "$TARGETARCH" in \
    amd64) sha256=cdd433fdbc0929e7b9f9709dd9e72237fdfe1fc235674445c284b1e4b94a5c26 ;; \
    arm64) sha256=9be425f8a8e32b118b03a6f368d381aa8b9bebece5e4f9bb7d028e96ca9e0ee2 ;; \
    *) echo "unsupported architecture: $TARGETARCH" >&2; exit 1 ;; \
  esac; \
  mkdir -p /opt/tflint/plugins; \
  wget -O /opt/tflint/ruleset.zip "https://github.com/terraform-linters/tflint-ruleset-aws/releases/download/v0.50.0/tflint-ruleset-aws_linux_${TARGETARCH}.zip"; \
  echo "${sha256}  /opt/tflint/ruleset.zip" | sha256sum -c -; \
  unzip /opt/tflint/ruleset.zip tflint-ruleset-aws -d /opt/tflint/plugins; \
  chmod 0555 /opt/tflint/plugins/tflint-ruleset-aws; \
  rm /opt/tflint/ruleset.zip

ENV TFLINT_PLUGIN_DIR=/opt/tflint/plugins

USER 65534:65534
