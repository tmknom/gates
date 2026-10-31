# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM ghcr.io/aquasecurity/trivy:0.76.1@sha256:790fd66d1697395d8693d1acaa1ea702406a20f109b6fc7bac8db623eca6c06c

ENV TRIVY_CACHE_DIR=/opt/trivy/cache

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
