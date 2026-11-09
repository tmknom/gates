# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM docker.io/library/golang:1.27.3-trixie@sha256:af1bfc2614ebb1dbd31af514b78740fd678850e76498762b425ecee4c5c6ea80 AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-mod=readonly GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# scratch には ln がないため、シンボリックリンクはこのステージで作り、ディレクトリごと複製してリンクのまま移す
RUN mkdir /out \
    && go build -trimpath -buildvcs=false -o /out/issue-token . \
    && ln -s /issue-token /out/revoke-token

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/ /
# USER を置かず root で動かす（GitHub Actions Design の Image）
ENTRYPOINT ["/issue-token"]
