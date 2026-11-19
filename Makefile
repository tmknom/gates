SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
.SUFFIXES:
.DEFAULT_GOAL := help

# 特定のディレクトリだけを扱う操作は、そのディレクトリの Makefile に置く。ここには、よく使うものだけを委譲する
TERRAFORM_DIR := terraform
FORMAL_DIR := formal
ACTION_DIR := .github/actions/issue-token

# Variables: Go
MODULE := $(shell go list -m)
DIST_DIR := dist
LAMBDA_BUILD_DIR := $(DIST_DIR)/lambda
LAMBDA_PACKAGE := $(DIST_DIR)/lambda.zip
# main.go は起動だけのため、カバレッジの計測から外す
COVER_EXCLUDE := /main\.go:

# Targets: Go
.PHONY: all
all: fmt goimports tidy vet test cover ## format, tidy, vet, test and cover
	@echo "NEXT STEP: Run 'make vuln' to check known vulnerabilities."
	@echo "NEXT STEP: Run 'make lint' to run golangci-lint in a container."

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

# Docker を使うため all には含めない
.PHONY: lint
lint: container-mountpoints tf-misplaced-values ## static analysis by golangci-lint in an isolated container
	LINT_MODULE=$(CURDIR) LINT_GOMODCACHE="$$(go env GOMODCACHE)" \
		docker compose -f scripts/golangci-lint/compose.yaml run --build --rm -T --interactive=false lint

.PHONY: vuln
vuln: ## check known vulnerabilities by govulncheck
	go tool govulncheck ./...

.PHONY: test
test: ## run test
	go test -count=1 ./...

.PHONY: cover
cover: ## measure test coverage excluding main.go and require 100% under internal/
	mkdir -p $(DIST_DIR)
	go test -count=1 -coverprofile=$(DIST_DIR)/coverage.out ./...
	grep -Ev '$(COVER_EXCLUDE)' $(DIST_DIR)/coverage.out > $(DIST_DIR)/coverage.filtered.out
	go tool cover -func=$(DIST_DIR)/coverage.filtered.out
	@# go tool cover -func は小数第 1 位に丸めて 100% 未満でも 100.0% と表示し得るため、ブロックの実行回数で判定する。
	@# 行末の 2 列はステートメント数と実行回数で、ステートメントを含まないブロック (空の case 等) は実行されなくても未到達に数えない
	@! grep '/internal/' $(DIST_DIR)/coverage.filtered.out | grep -E ' [1-9][0-9]* 0$$' \
		|| (echo "coverage under internal/ must be 100%: the blocks above are not executed" && exit 1)

.PHONY: clean
clean: ## remove build artifacts
	[ ! -e $(DIST_DIR) ] || rm -r $(DIST_DIR)

# リリースの配布物と同じバイト列の ZIP を作り、運用者が Lambda の CodeSha256 を配布物と照合できるようにする。
# そのため、.github/workflows/reusable-build.yml の Build と同じ設定でビルドする。
# Go は go.mod の toolchain の行のバージョンを使い、手元の Go と違えば go コマンドが取得する。
# ビルドの結果を変える設定は、手元の環境変数や go env -w の値を引き継がないよう、環境変数で固定する
LAMBDA_TOOLCHAIN = $(shell awk '$$1 == "toolchain" { print $$2 }' go.mod)
LAMBDA_GOENV = CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GOARM64=v8.0 GOTOOLCHAIN=$(LAMBDA_TOOLCHAIN) \
	GOFLAGS=-mod=readonly GOEXPERIMENT= GOFIPS140=off GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org \
	GOPRIVATE= GONOPROXY= GONOSUMDB= GOINSECURE=

# VCS の情報はバイナリに入れず、ZIP に入る権限と更新時刻を固定する。zip は更新時刻をローカル時刻で記録するため、TZ も固定する。
# 更新時刻は配布物に合わせてコミット日時にするため、コードが同じでもコミットが変われば ZIP は変わり、Plan に関数の更新が出る
# terraform/Makefile の plan が、Plan のたびに呼び出す
.PHONY: lambda-package
lambda-package: ## build the deployment package for Lambda with the same settings as the release package
	[ ! -e $(LAMBDA_BUILD_DIR) ] || rm -r $(LAMBDA_BUILD_DIR)
	[ ! -e $(LAMBDA_PACKAGE) ] || rm $(LAMBDA_PACKAGE)
	[ -n "$(LAMBDA_TOOLCHAIN)" ] || (echo "go.mod must have a toolchain line" && exit 1)
	[ "$$($(LAMBDA_GOENV) go env GOVERSION)" = "$(LAMBDA_TOOLCHAIN)" ] \
		|| (echo "Go $(LAMBDA_TOOLCHAIN) is required, but $$($(LAMBDA_GOENV) go env GOVERSION) is in use" && exit 1)
	mkdir -p $(LAMBDA_BUILD_DIR)
	$(LAMBDA_GOENV) go build -trimpath -buildvcs=false -ldflags="-s -w" -o $(LAMBDA_BUILD_DIR)/bootstrap .
	chmod 755 $(LAMBDA_BUILD_DIR)/bootstrap
	TZ=UTC touch -t "$$(TZ=UTC git log -1 --format=%cd --date=format-local:%Y%m%d%H%M.%S)" $(LAMBDA_BUILD_DIR)/bootstrap
	TZ=UTC zip -X -j $(LAMBDA_PACKAGE) $(LAMBDA_BUILD_DIR)/bootstrap

# Targets: Terraform
.PHONY: tf-plan
tf-plan: ## create a terraform plan by terraform/Makefile
	$(MAKE) -C $(TERRAFORM_DIR) plan

.PHONY: tf-apply
tf-apply: ## apply the saved terraform plan by terraform/Makefile
	$(MAKE) -C $(TERRAFORM_DIR) apply

.PHONY: tf-misplaced-values
tf-misplaced-values:
	$(MAKE) -C $(TERRAFORM_DIR) misplaced-values

# Targets: KMS
# -C で作業ディレクトリを移すと、GITHUB_PRIVATE_KEY_PATH の相対パスが別の場所を指すため、ルートで実行する
# 秘密鍵を扱うため、コンテナに入れず、運用者が手元のシェルで実行する。名前とパスの検査はスクリプトが行う
.PHONY: kms-import-key-material
kms-import-key-material: ## import the GitHub App private key at GITHUB_PRIVATE_KEY_PATH into the PendingImport KMS key KMS_KEY_NAME, then remove the file
	$(MAKE) -C $(TERRAFORM_DIR) aws-credentials
	scripts/kms/import-key-material.sh

.PHONY: kms-test
kms-test: ## test the key material import script with a stubbed aws and throwaway keys
	scripts/kms/import-key-material_test.sh

# Targets: Security
.PHONY: trivy
trivy: container-mountpoints tf-misplaced-values ## scan vulnerabilities, misconfigurations and secrets by trivy in an isolated container
	TRIVY_TARGET=$(CURDIR) docker compose -f scripts/trivy/compose.yaml run --rm -T --interactive=false scan

.PHONY: trivy-update
trivy-update: ## rebuild the trivy image to refresh its vulnerability database
	TRIVY_TARGET=$(CURDIR) docker compose -f scripts/trivy/compose.yaml build --no-cache scan

# 読み取り専用の /src にはマウントポイントを作れないため、compose.yaml が隠すパスをホスト側で用意する。既存の .envrc は変更しない
# Docker Action の Makefile の lint も使う
.PHONY: container-mountpoints
container-mountpoints:
	@mkdir -p .claude $(DIST_DIR) $(TERRAFORM_DIR)/local tmp
	@[ -e .envrc ] || touch .envrc

# Targets: Formal Verification
.PHONY: alloy
alloy: ## verify formal models by Alloy by formal/Makefile
	$(MAKE) -C $(FORMAL_DIR) alloy

.PHONY: z3
z3: ## verify formal specifications by Z3 by formal/Makefile
	$(MAKE) -C $(FORMAL_DIR) z3

.PHONY: trace
trace: ## check the links between the IDs in docs/ and the documents, models, and tests by formal/Makefile
	$(MAKE) -C $(FORMAL_DIR) trace

# 出力をそのまま読めるよう、ディレクトリの移動の表示を止める
.PHONY: trace-map
trace-map: ## print the annotations that formalize and check each clause and GOJ by formal/Makefile
	@$(MAKE) --no-print-directory -C $(FORMAL_DIR) trace-map

# Targets: GitHub Actions
# actionlint は .git からプロジェクトを探すが、コンテナには .github だけを渡すため、検査するファイルを引数で渡す
.PHONY: lint-gha
lint-gha: ## lint workflow files by actionlint in an isolated container
	ACTIONLINT_TARGET=$(CURDIR)/.github docker compose -f scripts/actionlint/compose.yaml run --build --rm -T --interactive=false actionlint \
		$(sort $(wildcard .github/workflows/*.yml .github/workflows/*.yaml))

# Targets: Docker Action
.PHONY: action-all
action-all: ## format, tidy, vet, test and cover the docker action by its Makefile
	$(MAKE) -C $(ACTION_DIR) all

# Targets
.PHONY: help
help: ### Display help
	@grep --no-filename -E '^[a-zA-Z0-9_/-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	| grep -v '###' | sort -u \
	| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
	@echo "More targets: make -C <dir> help, for <dir> in $(TERRAFORM_DIR) $(FORMAL_DIR) $(ACTION_DIR)"
