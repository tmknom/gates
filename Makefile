# Include: minimum
-include .makefiles/minimum/Makefile
.makefiles/minimum/Makefile:
	@git clone https://github.com/tmknom/makefiles.git .makefiles >/dev/null 2>&1

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
	[ ! -e $(DIST_DIR) ] || rm -r $(DIST_DIR)

# コードが同じなら ZIP のハッシュを同じにし、変更のない関数の更新を Plan に出さない。
# そのため、コミットごとに変わる VCS の情報をバイナリに入れず、ZIP に入る権限と更新時刻を固定する。zip は更新時刻をローカル時刻で記録するため、TZ も固定する
.PHONY: lambda-package
lambda-package: ## build the deployment package for Lambda
	[ ! -e $(LAMBDA_BUILD_DIR) ] || rm -r $(LAMBDA_BUILD_DIR)
	[ ! -e $(LAMBDA_PACKAGE) ] || rm $(LAMBDA_PACKAGE)
	mkdir -p $(LAMBDA_BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false -o $(LAMBDA_BUILD_DIR)/bootstrap .
	chmod 755 $(LAMBDA_BUILD_DIR)/bootstrap
	TZ=UTC touch -t 200001010000 $(LAMBDA_BUILD_DIR)/bootstrap
	TZ=UTC zip -X -j $(LAMBDA_PACKAGE) $(LAMBDA_BUILD_DIR)/bootstrap

# Targets: Terraform
.PHONY: tflint
tflint: tf-local-dir tf-misplaced-values ## static analysis by tflint in an isolated container
	TFLINT_TARGET=$(abspath $(TF_DIR)) docker compose -f scripts/tflint/compose.yaml run --build --rm -T --interactive=false lint

# Targets: Terraform in containers
# コンテナにマウントするディレクトリで、接続の確認のときに上書きする
TF_DIR ?= terraform
# 手元の値の置き場所は TF_DIR から決まる。別の場所を指定したつもりで本物の State を使わないよう、上書きを拒否する
ifneq ($(origin TF_LOCAL_DIR),undefined)
$(error TF_LOCAL_DIR cannot be overridden: override TF_DIR instead)
endif
# 名前を変えるときは terraform/backend.tf、.gitignore、scripts/ の terraform、tflint、golangci-lint、trivy の compose.yaml も変更する
TF_LOCAL_NAME := local
TF_LOCAL_DIR := $(TF_DIR)/$(TF_LOCAL_NAME)
TF_PACKAGE_CHECKSUM := $(TF_LOCAL_DIR)/lambda.zip.sha256
TF_COMPOSE := TF_HOST_DIR=$(abspath $(TF_DIR)) TF_HOST_LOCAL_DIR=$(abspath $(TF_LOCAL_DIR)) TF_HOST_DIST_DIR=$(abspath $(DIST_DIR)) \
	TF_HOST_UID=$$(id -u) TF_HOST_GID=$$(id -g) docker compose -f scripts/terraform/compose.yaml
TF_RUN := $(TF_COMPOSE) run --build --rm -T --interactive=false

# proxy は run の終了後も残るため、run の終了コードを保ったまま止める。止めるとログが消えるため、通信した宛先を先に出力する
define tf_run_with_proxy
	$(TF_RUN) $(1) $(2); status=$$?; $(TF_COMPOSE) logs --no-log-prefix proxy-$(1); $(TF_COMPOSE) down; exit $$status
endef

.PHONY: tf-lock
tf-lock: tf-mountpoints ## update the provider lock file through the registry allowlist proxy
	@# providers lock は固定中のバージョンから変更できないため、作り直す。ハッシュの変更は Git の差分で確認する
	[ ! -e $(TF_DIR)/.terraform.lock.hcl ] || rm $(TF_DIR)/.terraform.lock.hcl
	$(call tf_run_with_proxy,lock,providers lock -platform=linux_arm64 -platform=linux_amd64)

.PHONY: tf-init
tf-init: tf-mountpoints ## initialize terraform with the providers in the image
	$(TF_RUN) offline init -input=false -lockfile=readonly

.PHONY: tf-fmt
tf-fmt: tf-mountpoints tf-misplaced-values ## format terraform code
	$(TF_RUN) fmt fmt -recursive

.PHONY: tf-check
tf-check: tf-mountpoints tf-misplaced-values ## check format, validate and test terraform code
	$(TF_RUN) fmt-check fmt -check -recursive
	$(TF_RUN) offline validate
	$(TF_RUN) offline test

.PHONY: tf-show
tf-show: tf-mountpoints ## show the saved terraform plan
	$(TF_RUN) offline show $(TF_LOCAL_NAME)/plan.tfplan

# 古いコードを配布しないよう、Plan のたびにデプロイパッケージを作り直す
# Terraform はエラーで止まった Plan も保存するため、失敗したら捨てる。prevent_destroy で止めた鍵の置換を含む Plan が残るのを防ぐ
.PHONY: tf-plan
tf-plan: tf-aws-credentials tf-mountpoints tf-package-checksum ## create a terraform plan with temporary AWS credentials through the AWS allowlist proxy
	($(call tf_run_with_proxy,aws,plan -var-file=$(TF_LOCAL_NAME)/terraform.tfvars -var=lambda_package_path=../$(LAMBDA_PACKAGE) -out=$(TF_LOCAL_NAME)/plan.tfplan)) \
		|| ([ ! -e $(TF_LOCAL_DIR)/plan.tfplan ] || rm $(TF_LOCAL_DIR)/plan.tfplan; exit 1)

# 確認した Plan と違う変更を適用しないよう、保存した Plan の適用だけに絞る。Apply は Plan の後のデプロイパッケージを読むため、Plan のときと同じかも確かめる
.PHONY: tf-apply
tf-apply: tf-aws-credentials tf-mountpoints tf-package-unchanged ## apply the saved terraform plan with temporary AWS credentials through the AWS allowlist proxy
	$(call tf_run_with_proxy,aws,apply $(TF_LOCAL_NAME)/plan.tfplan)

.PHONY: tf-package-checksum
tf-package-checksum: tf-plan-discard lambda-package tf-local-dir
	shasum -a 256 $(LAMBDA_PACKAGE) > $(TF_PACKAGE_CHECKSUM)

# Plan が失敗すると前回の Plan が残り、新しいチェックサムと組み合わさって Apply できてしまうため、先に捨てる
.PHONY: tf-plan-discard
tf-plan-discard:
	[ ! -e $(TF_LOCAL_DIR)/plan.tfplan ] || rm $(TF_LOCAL_DIR)/plan.tfplan

.PHONY: tf-package-unchanged
tf-package-unchanged:
	shasum -a 256 -c $(TF_PACKAGE_CHECKSUM)

# AWS_SESSION_TOKEN を必須にして、長期のアクセスキーでは起動できないようにする
.PHONY: tf-aws-credentials
tf-aws-credentials:
	@: "$${AWS_ACCESS_KEY_ID:?AWS_ACCESS_KEY_ID is required}"
	@: "$${AWS_SECRET_ACCESS_KEY:?AWS_SECRET_ACCESS_KEY is required}"
	@: "$${AWS_SESSION_TOKEN:?AWS_SESSION_TOKEN is required: use temporary credentials}"

.PHONY: tf-mountpoints
tf-mountpoints: tf-local-dir
	@mkdir -p $(DIST_DIR)

.PHONY: tf-local-dir
tf-local-dir:
	@mkdir -p $(TF_LOCAL_DIR)

# compose.yaml が隠すのは手元の値の置き場所だけで、ほかの場所のファイルは静的解析のコンテナから読める
.PHONY: tf-misplaced-values
tf-misplaced-values:
	@! find $(TF_DIR) -path $(TF_LOCAL_DIR) -prune -o \( -name '*.tfvars' -o -name '*.tfstate' -o -name '*.tfstate.*' -o -name '*.tfplan' \
		-o -name .terraform -o -name .envrc \) -print | grep . \
		|| (echo "move the files above to $(TF_LOCAL_DIR)/: containers can read them" && exit 1)

# Targets: Security
.PHONY: trivy
trivy: container-mountpoints tf-misplaced-values ## scan vulnerabilities, misconfigurations and secrets by trivy in an isolated container
	TRIVY_TARGET=$(CURDIR) docker compose -f scripts/trivy/compose.yaml run --rm -T --interactive=false scan

.PHONY: trivy-update
trivy-update: ## rebuild the trivy image to refresh its vulnerability database
	TRIVY_TARGET=$(CURDIR) docker compose -f scripts/trivy/compose.yaml build --no-cache scan

# Targets: Formal Verification
.PHONY: alloy
alloy: ## verify formal models by Alloy in an isolated container
	ALLOY_TARGET=$(CURDIR)/formal/alloy docker compose -f scripts/alloy/compose.yaml run --build --rm -T --interactive=false alloy exec -c '*' -o /tmp/out -t none authorization.als
	ALLOY_TARGET=$(CURDIR)/formal/alloy docker compose -f scripts/alloy/compose.yaml run --rm -T --interactive=false alloy exec -c '*' -o /tmp/out -t none issuance-temporal.als

# 検査ごとに Z3 を起動し、期待する判定と比べるスクリプトを実行するため、entrypoint を sh に替える
.PHONY: z3
z3: ## verify formal specifications by Z3 in an isolated container
	Z3_TARGET=$(CURDIR)/formal/z3 docker compose -f scripts/z3/compose.yaml run --build --rm -T --interactive=false --entrypoint sh z3 verify.sh

# 読み取り専用の /src にはマウントポイントを作れないため、compose.yaml が隠すパスをホスト側で用意する。既存の .envrc は変更しない
.PHONY: container-mountpoints
container-mountpoints:
	@mkdir -p .claude .makefiles $(DIST_DIR) terraform/$(TF_LOCAL_NAME) tmp
	@[ -e .envrc ] || touch .envrc

# Targets: GitHub Actions
.PHONY: lint-gha
lint-gha: lint/workflow lint/yaml ## Lint workflow files and YAML files

.PHONY: fmt-gha
fmt-gha: fmt/yaml ## Format YAML files
