# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM docker.io/library/alpine:3.24.4@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf

# Alpine のリポジトリは古いバージョンを残さないことがあるため、ビルドできなくなったらバージョンを上げる。
# 基準のイメージのパッケージに修正版のある脆弱性が残ることがあるため、ビルド時に上げる
RUN apk upgrade --no-cache \
  && apk add --no-cache squid=7.6-r0

USER 65534:65534

# 実行時の設定は compose.yaml がマウントする
ENTRYPOINT ["squid", "-N", "-f", "/config/squid.conf"]
