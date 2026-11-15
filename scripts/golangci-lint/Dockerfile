# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

# 公式イメージの golangci-lint は修正版のある Go でビルドされていないことがあるため、同じバージョンを Docker Action のビルドと同じ Go でビルドし直す。
# モジュールは Go のチェックサムデータベースで検証される
FROM docker.io/library/golang:1.27.4-trixie@sha256:b54b2639de8c1bfe538ec5ffc45f29b1932235ac5f6d63c059853646677112d9 AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
RUN go install -trimpath github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0

FROM docker.io/golangci/golangci-lint:v2.15.0@sha256:fbfcfd12a6f8d7f1b97b3362ca80fc2a244d46863148c73cb88e7793af96c481

# 基準のイメージのパッケージに修正版のある脆弱性が残ることがあるため、ビルド時に上げる。
# ARG はビルド時だけ有効で、実行時の環境変数には残らない
ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
  && apt-get upgrade -y --no-install-recommends \
  && rm -rf /var/lib/apt/lists/*

# golangci-lint は解析に同梱の Go を使い、go.mod が求めるバージョンを満たす必要がある。
# 同梱の Go も修正版でないことがあるため、ビルドに使った Go に差し替える
RUN rm -rf /usr/local/go
COPY --from=build /usr/local/go /usr/local/go
COPY --from=build /go/bin/golangci-lint /usr/bin/golangci-lint

USER 65534:65534
