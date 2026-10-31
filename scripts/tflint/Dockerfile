# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM ghcr.io/terraform-linters/tflint:v0.64.0@sha256:1c595f42d794c32c45a6ea8b58655fd66433d4ca3b1bc631c574a48d120bd19f

ARG TARGETARCH

# 実行時はネットワークを持たないため、プラグインはビルド時に入れる。
# ハッシュはリリースの checksums.txt の zip の値で、URL のバージョンを変えるときは一緒に書き換える
RUN set -eux; \
  case "$TARGETARCH" in \
    amd64) sha256=98a530130d33efd49408f3d0ab9198072192ede413cc2a2ce38295d0922eb1c6 ;; \
    arm64) sha256=94f4f965a959d1699ae59687ffccc25031f8aaeae6f49756e5d8ca6e36d9f95b ;; \
    *) echo "unsupported architecture: $TARGETARCH" >&2; exit 1 ;; \
  esac; \
  wget -O /tmp/ruleset.zip "https://github.com/terraform-linters/tflint-ruleset-aws/releases/download/v0.49.0/tflint-ruleset-aws_linux_${TARGETARCH}.zip"; \
  echo "${sha256}  /tmp/ruleset.zip" | sha256sum -c -; \
  mkdir -p /opt/tflint/plugins; \
  unzip /tmp/ruleset.zip tflint-ruleset-aws -d /opt/tflint/plugins; \
  chmod 0555 /opt/tflint/plugins/tflint-ruleset-aws; \
  rm /tmp/ruleset.zip

ENV TFLINT_PLUGIN_DIR=/opt/tflint/plugins
