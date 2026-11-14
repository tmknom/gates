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

.PHONY: tf-output
tf-output: tf-mountpoints ## show the terraform outputs recorded in the state
	$(TF_RUN) offline output

# 退役させる KMS Key を、削除せずに State から外す。removed ブロックは for_each の 1 要素を指せないため、state rm を使う
# 名前は環境変数として読み、variables.tf の kms_keys の名前の形を満たすまでコマンドに展開しない
# 外す前の State は、鍵の名前を付けたバックアップに残す。既定の置き場所は読み取り専用の terraform/ のため、local/ を指定する
.PHONY: tf-state-rm-kms-key
tf-state-rm-kms-key: tf-mountpoints ## remove the retired KMS key KMS_KEY_NAME from the terraform state without deleting the key
	@# grep は行ごとに判定するため、改行などの許さない文字を先に case で拒否する
	@case "$${KMS_KEY_NAME:-}" in "" | *[!a-z0-9-]*) false ;; esac \
		&& printf '%s\n' "$$KMS_KEY_NAME" | grep -Eqx 'key-[1-9][0-9]{0,8}' \
		|| (echo "KMS_KEY_NAME must be the name of the retired key in kms_keys, such as key-1" && exit 1)
	$(TF_RUN) offline state rm -backup=$(TF_LOCAL_NAME)/terraform.tfstate.$(KMS_KEY_NAME).backup 'aws_kms_external_key.signing["$(KMS_KEY_NAME)"]'

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
	@mkdir -p .claude $(DIST_DIR) terraform/$(TF_LOCAL_NAME) tmp
	@[ -e .envrc ] || touch .envrc

# Targets: GitHub Actions
# actionlint は .git からプロジェクトを探すが、コンテナには .github だけを渡すため、検査するファイルを引数で渡す
.PHONY: lint-gha
lint-gha: ## lint workflow files by actionlint in an isolated container
	ACTIONLINT_TARGET=$(CURDIR)/.github docker compose -f scripts/actionlint/compose.yaml run --rm -T --interactive=false actionlint \
		$(sort $(wildcard .github/workflows/*.yml .github/workflows/*.yaml))

# Targets: Docker Action
# Docker Action はルートと別の Go モジュールのため、go -C でそのディレクトリを基準に実行する
ACTION_DIR := .github/actions/issue-token
ACTION_GO := go -C $(ACTION_DIR)
ACTION_MODULE := $(shell $(ACTION_GO) list -m)

.PHONY: action-all
action-all: action-fmt action-goimports action-tidy action-vet action-test action-cover ## format, tidy, vet, test and cover the docker action
	@echo "NEXT STEP: Run 'make action-vuln' to check known vulnerabilities."
	@echo "NEXT STEP: Run 'make action-lint' to run golangci-lint in a container."

.PHONY: action-fmt
action-fmt: ## format the docker action code
	$(ACTION_GO) fmt ./...

.PHONY: action-goimports
action-goimports: ## update import lines of the docker action code
	$(ACTION_GO) tool goimports -w -local $(ACTION_MODULE) .

.PHONY: action-tidy
action-tidy: ## manage modules of the docker action
	$(ACTION_GO) mod tidy
	$(ACTION_GO) mod verify

.PHONY: action-vet
action-vet: ## static analysis of the docker action by vet
	$(ACTION_GO) vet ./...

# リポジトリのルートをマウントし、隠すパスを lint と同じにしたまま、作業ディレクトリだけを Docker Action のモジュールに移す。
# Docker Action のディレクトリをマウントすると、隠すパスのマウントポイントをそのディレクトリにも作る必要があるため
.PHONY: action-lint
action-lint: container-mountpoints tf-misplaced-values ## static analysis of the docker action by golangci-lint in an isolated container
	LINT_MODULE=$(CURDIR) LINT_GOMODCACHE="$$(go env GOMODCACHE)" \
		docker compose -f scripts/golangci-lint/compose.yaml run --rm -T --interactive=false --workdir /src/$(ACTION_DIR) lint

.PHONY: action-vuln
action-vuln: ## check known vulnerabilities of the docker action by govulncheck
	$(ACTION_GO) tool govulncheck ./...

.PHONY: action-test
action-test: ## run test of the docker action
	$(ACTION_GO) test -count=1 ./...

.PHONY: action-cover
action-cover: ## measure test coverage of the docker action excluding main.go and require 100% under internal/
	mkdir -p $(DIST_DIR)
	$(ACTION_GO) test -count=1 -coverprofile=$(CURDIR)/$(DIST_DIR)/action-coverage.out ./...
	grep -Ev '$(COVER_EXCLUDE)' $(DIST_DIR)/action-coverage.out > $(DIST_DIR)/action-coverage.filtered.out
	$(ACTION_GO) tool cover -func=$(CURDIR)/$(DIST_DIR)/action-coverage.filtered.out
	@# 判定はルートの cover と同じく、internal/ 配下のブロックの実行回数で行う
	@! grep '/internal/' $(DIST_DIR)/action-coverage.filtered.out | grep -E ' [1-9][0-9]* 0$$' \
		|| (echo "coverage under internal/ must be 100%: the blocks above are not executed" && exit 1)

# Runner がジョブのたびに行うビルドを手元で再現し、通信のないコンテナで起動を確かめる。
# メインは子の前提の確認が環境変数の欠落で失敗し、post-entrypoint は子が STATE_token がないため何もせずに終える。
# どちらも子を起動できなければ注釈が変わるため、親から子を起動できることも確かめられる。
# 2 つを起動するのは、シンボリックリンクのパスで起動した実行ファイルが役割を選べることも確かめるため
ACTION_IMAGE := localhost/gates/issue-token:local

.PHONY: action-image
action-image: ## build the docker action image and check that it starts without network
	docker build -t $(ACTION_IMAGE) $(ACTION_DIR)
	@out="$$(docker run --rm --network none $(ACTION_IMAGE))"; code=$$?; echo "$$out"; \
		[ $$code -eq 1 ] || (echo "issue-token must exit 1 without environment, got $$code" && exit 1); \
		echo "$$out" | grep -q '^::error::ACTIONS_ID_TOKEN_REQUEST_TOKEN is not set' || (echo "issue-token must fail the prerequisite check in the child" && exit 1)
	@out="$$(docker run --rm --network none --entrypoint /revoke-token $(ACTION_IMAGE))"; code=$$?; echo "$$out"; \
		[ $$code -eq 0 ] || (echo "revoke-token must exit 0 without STATE_token, got $$code" && exit 1); \
		[ -z "$$out" ] || (echo "revoke-token must write nothing without STATE_token" && exit 1)
