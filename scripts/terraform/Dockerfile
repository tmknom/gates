# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

# Terraform のバージョンを変えるときは terraform/versions.tf の required_version も書き換える
FROM docker.io/hashicorp/terraform:1.17.2@sha256:6e9c1e647c31a960dc9b59e13272f9111d90010c10b717f77ffb324767660475 AS base

# 基準のイメージの Alpine のパッケージに修正版のある脆弱性が残るため、ビルド時に上げる
USER root
RUN apk upgrade --no-cache

USER 65534:65534

# lock ファイルを作るサービスは base を使う。lock ファイルを COPY するステージに依存すると、最初の lock ファイルを作れない
FROM base AS provider

ARG TARGETARCH

USER root

COPY versions.tf .terraform.lock.hcl /build/
COPY --from=config terraformrc /opt/terraform/terraformrc

# 実行時は認証情報を持つプロセスに Provider を取得させないため、ビルド時に入れる。
# zip のままだと init が .terraform/ へ展開してコピーを作るため、展開しておき、シンボリックリンクで参照させる。
# リンク先はイメージの中のパスなので、ホストからはリンク切れに見えるが、異常ではない
RUN set -eux; \
  terraform -chdir=/build providers mirror -platform="linux_${TARGETARCH}" /opt/terraform/providers; \
  cd /opt/terraform/providers; \
  for package in */*/*/*.zip; do \
    file="${package##*/}"; \
    version="${file#*_}"; \
    version="${version%%_*}"; \
    target="${package%/*}/${version}/linux_${TARGETARCH}"; \
    mkdir -p "${target}"; \
    unzip "${package}" -d "${target}"; \
    chmod 0555 "${target}"/terraform-provider-*; \
    rm "${package}"; \
  done; \
  rm ./*/*/*/*.json; \
  chmod -R a+rX /opt/terraform

ENV TF_CLI_CONFIG_FILE=/opt/terraform/terraformrc

USER 65534:65534
