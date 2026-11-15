# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

# 公式イメージの actionlint は修正版のある Go でビルドされていないことがあるため、同じバージョンを Docker Action のビルドと同じ Go でビルドし直す。
# モジュールは Go のチェックサムデータベースで検証される。実行用のイメージは musl のため、静的にリンクする
FROM docker.io/library/golang:1.27.4-trixie@sha256:b54b2639de8c1bfe538ec5ffc45f29b1932235ac5f6d63c059853646677112d9 AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
RUN go install -trimpath github.com/rhysd/actionlint/cmd/actionlint@v1.7.12

# shellcheck を同梱する公式イメージに、ビルドし直した actionlint を置く
FROM docker.io/rhysd/actionlint:1.7.12@sha256:b1934ee5f1c509618f2508e6eb47ee0d3520686341fec936f3b79331f9315667

# 基準のイメージのパッケージに修正版のある脆弱性が残ることがあるため、ビルド時に上げる。
USER root
RUN apk upgrade --no-cache
COPY --from=build /go/bin/actionlint /usr/local/bin/actionlint

USER 65534:65534
