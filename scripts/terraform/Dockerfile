# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

# Terraform のバージョンを変えるときは terraform/versions.tf の required_version も書き換える
FROM docker.io/hashicorp/terraform:1.16.7@sha256:c9c77f5b9820832cbc7b88d16df406ac6e75fee5a9077e5437db0c54e5ff33e3 AS base

USER 65534:65534

# lock ファイルを作るサービスは base を使う。lock ファイルを COPY するステージに依存すると、最初の lock ファイルを作れない
FROM base AS provider

ARG TARGETARCH

USER root

COPY versions.tf .terraform.lock.hcl /build/
COPY --from=config terraformrc /opt/terraform/terraformrc

# 実行時は認証情報を持つプロセスに Provider を取得させないため、ビルド時に入れる
RUN set -eux; \
  terraform -chdir=/build providers mirror -platform="linux_${TARGETARCH}" /opt/terraform/providers; \
  chmod -R a+rX /opt/terraform

ENV TF_CLI_CONFIG_FILE=/opt/terraform/terraformrc

USER 65534:65534
