SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
.SUFFIXES:
.DEFAULT_GOAL := help

# Variables
MODULE := $(shell go list -m)
ROOT_DIR := ../../..
# ビルドの成果物は、ルートのモジュールと同じくリポジトリのルートの dist/ に置く
DIST_DIR := $(abspath $(ROOT_DIR)/dist)
# main.go は起動だけのため、カバレッジの計測から外す
COVER_EXCLUDE := /main\.go:

# Targets: Go
.PHONY: all
all: fmt goimports tidy vet test cover ## format, tidy, vet, test and cover
	@echo "NEXT STEP: Run 'make vuln' in .github/actions/issue-token to check known vulnerabilities."
	@echo "NEXT STEP: Run 'make lint' in .github/actions/issue-token to run golangci-lint in a container."

.PHONY: fmt
fmt: ## format code
	go fmt ./...

.PHONY: goimports
goimports: ## update import lines
	go tool goimports -w -local $(MODULE) .

.PHONY: tidy
tidy: ## manage modules
	go mod tidy
	go mod verify

.PHONY: vet
vet: ## static analysis by vet
	go vet ./...

# リポジトリのルートをマウントし、隠すパスをルートの lint と同じにしたまま、作業ディレクトリだけをこのモジュールに移す。
# このディレクトリをマウントすると、隠すパスのマウントポイントをこのディレクトリにも作る必要があるため
.PHONY: lint
lint: ## static analysis by golangci-lint in an isolated container
	$(MAKE) -C $(ROOT_DIR) container-mountpoints tf-misplaced-values
	LINT_MODULE=$(abspath $(ROOT_DIR)) LINT_GOMODCACHE="$$(go env GOMODCACHE)" \
		docker compose -f $(ROOT_DIR)/scripts/golangci-lint/compose.yaml run --build --rm -T --interactive=false --workdir /src/.github/actions/issue-token lint

.PHONY: vuln
vuln: ## check known vulnerabilities by govulncheck
	go tool govulncheck ./...

.PHONY: test
test: ## run test
	go test -count=1 ./...

.PHONY: cover
cover: ## measure test coverage excluding main.go and require 100% under internal/
	mkdir -p $(DIST_DIR)
	go test -count=1 -coverprofile=$(DIST_DIR)/action-coverage.out ./...
	grep -Ev '$(COVER_EXCLUDE)' $(DIST_DIR)/action-coverage.out > $(DIST_DIR)/action-coverage.filtered.out
	go tool cover -func=$(DIST_DIR)/action-coverage.filtered.out
	@# go tool cover -func は小数第 1 位に丸めて 100% 未満でも 100.0% と表示し得るため、ブロックの実行回数で判定する。
	@# 行末の 2 列はステートメント数と実行回数で、ステートメントを含まないブロック (空の case 等) は実行されなくても未到達に数えない
	@! grep '/internal/' $(DIST_DIR)/action-coverage.filtered.out | grep -E ' [1-9][0-9]* 0$$' \
		|| (echo "coverage under internal/ must be 100%: the blocks above are not executed" && exit 1)

# Targets: Docker
# Runner がジョブのたびに行うビルドを手元で再現し、通信のないコンテナで起動を確かめる。
# メインは子の前提の確認が環境変数の欠落で失敗し、post-entrypoint は子が STATE_token がないため何もせずに終える。
# どちらも子を起動できなければ注釈が変わるため、親から子を起動できることも確かめられる。
# 2 つを起動するのは、シンボリックリンクのパスで起動した実行ファイルが役割を選べることも確かめるため
IMAGE := localhost/gates/issue-token:local

.PHONY: image
image: ## build the docker image and check that it starts without network
	docker build -t $(IMAGE) .
	@out="$$(docker run --rm --network none $(IMAGE))"; code=$$?; echo "$$out"; \
		[ $$code -eq 1 ] || (echo "issue-token must exit 1 without environment, got $$code" && exit 1); \
		echo "$$out" | grep -q '^::error::ACTIONS_ID_TOKEN_REQUEST_TOKEN is not set' || (echo "issue-token must fail the prerequisite check in the child" && exit 1)
	@out="$$(docker run --rm --network none --entrypoint /revoke-token $(IMAGE))"; code=$$?; echo "$$out"; \
		[ $$code -eq 0 ] || (echo "revoke-token must exit 0 without STATE_token, got $$code" && exit 1); \
		[ -z "$$out" ] || (echo "revoke-token must write nothing without STATE_token" && exit 1)

# Targets
.PHONY: help
help: ### Display help
	@grep --no-filename -E '^[a-zA-Z0-9_/-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	| grep -v '###' | sort -u \
	| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
