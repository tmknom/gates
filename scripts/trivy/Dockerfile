# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM ghcr.io/aquasecurity/trivy:0.76.2@sha256:8f6bb947dbc528902e11f2f5be52f7b519f6a39d5d148f41166f2040499768fb

ENV TRIVY_CACHE_DIR=/opt/trivy/cache

# 基準のイメージのパッケージに修正版のある脆弱性が残ることがあるため、ビルド時に上げる
RUN apk upgrade --no-cache

# 実行時はネットワークを持たないため、脆弱性 DB と misconfig のチェックはビルド時に入れる。
# この層のキャッシュは取得の日付を知らないため、更新は Makefile の trivy-update が --no-cache でビルドする。
# チェックのバンドルを取得するサブコマンドはないため、空のディレクトリを検査して取得させる。
# 実行時の非 root ユーザーが読めるよう、権限をそろえる
RUN set -eux; \
  trivy image --download-db-only --no-progress; \
  mkdir /tmp/empty; \
  trivy config /tmp/empty; \
  rmdir /tmp/empty; \
  chmod -R a+rX /opt/trivy/cache

USER 65534:65534
