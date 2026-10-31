# Include: minimum
-include .makefiles/minimum/Makefile
.makefiles/minimum/Makefile:
	@git clone https://github.com/tmknom/makefiles.git .makefiles >/dev/null 2>&1

# Variables: Go
MODULE := $(shell go list -m)
DIST_DIR := dist
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
lint: container-mountpoints ## static analysis by golangci-lint in an isolated container
	LINT_MODULE=$(CURDIR) LINT_GOMODCACHE="$$(go env GOMODCACHE)" \
		docker compose -f scripts/golangci-lint/compose.yaml run --rm -T --interactive=false lint

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
	rm -rf $(DIST_DIR)

# Targets: Terraform
.PHONY: tflint
tflint: container-mountpoints ## static analysis by tflint in an isolated container
	TFLINT_TARGET=$(CURDIR) docker compose -f scripts/tflint/compose.yaml run --build --rm -T --interactive=false lint

# Targets: Security
.PHONY: trivy
trivy: container-mountpoints ## scan vulnerabilities, misconfigurations and secrets by trivy in an isolated container
	TRIVY_TARGET=$(CURDIR) docker compose -f scripts/trivy/compose.yaml run --rm -T --interactive=false scan

.PHONY: trivy-update
trivy-update: ## rebuild the trivy image to refresh its vulnerability database
	TRIVY_TARGET=$(CURDIR) docker compose -f scripts/trivy/compose.yaml build --no-cache scan

# 読み取り専用の /src にはマウントポイントを作れないため、compose.yaml が隠すパスをホスト側で用意する。既存の .envrc と .env は変更しない
.PHONY: container-mountpoints
container-mountpoints:
	@mkdir -p .claude .makefiles $(DIST_DIR) tmp
	@[ -e .envrc ] || touch .envrc
	@[ -e .env ] || touch .env

# Targets: GitHub Actions
.PHONY: lint-gha
lint-gha: lint/workflow lint/yaml ## Lint workflow files and YAML files

.PHONY: fmt-gha
fmt-gha: fmt/yaml ## Format YAML files
