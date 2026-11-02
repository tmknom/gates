# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM docker.io/library/alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

# Alpine のリポジトリは古いバージョンを残さないことがあるため、ビルドできなくなったらバージョンを上げる
RUN apk add --no-cache squid=7.6-r0

USER 65534:65534

# 実行時の設定は compose.yaml がマウントする
ENTRYPOINT ["squid", "-N", "-f", "/config/squid.conf"]
