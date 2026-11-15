SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
.SUFFIXES:
.DEFAULT_GOAL := help

# 相対パスは terraform/ を基準にする。make -f で別の場所から読むと、意図しないディレクトリを認証情報とともにマウントし得るため、拒否する
ifneq ($(CURDIR),$(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST))))))
$(error run make in terraform/, such as make -C terraform plan)
endif

# Variables
# コンテナにマウントするディレクトリで、接続の確認のときに上書きする
TF_DIR ?= .
# 手元の値の置き場所は TF_DIR から決まる。別の場所を指定したつもりで本物の State を使わないよう、上書きを拒否する
ifneq ($(origin TF_LOCAL_DIR),undefined)
$(error TF_LOCAL_DIR cannot be overridden: override TF_DIR instead)
endif
# 名前を変えるときは backend.tf、.gitignore、ルートの Makefile、scripts/ の terraform、tflint、golangci-lint、trivy の compose.yaml も変更する
TF_LOCAL_NAME := local
TF_LOCAL_DIR := $(TF_DIR)/$(TF_LOCAL_NAME)
DIST_DIR := ../dist
# ルートの Makefile の lambda-package が作る。コンテナでも terraform/ と dist/ の位置関係はホストと同じため、このパスを Terraform にも渡す
LAMBDA_PACKAGE := $(DIST_DIR)/lambda.zip
PACKAGE_CHECKSUM := $(TF_LOCAL_DIR)/lambda.zip.sha256
TF_COMPOSE := TF_HOST_DIR=$(abspath $(TF_DIR)) TF_HOST_LOCAL_DIR=$(abspath $(TF_LOCAL_DIR)) TF_HOST_DIST_DIR=$(abspath $(DIST_DIR)) \
	TF_HOST_UID=$$(id -u) TF_HOST_GID=$$(id -g) docker compose -f ../scripts/terraform/compose.yaml
TF_RUN := $(TF_COMPOSE) run --build --rm -T --interactive=false

# proxy は run の終了後も残るため、run の終了コードを保ったまま止める。止めるとログが消えるため、通信した宛先を先に出力する
define tf_run_with_proxy
	$(TF_RUN) $(1) $(2); status=$$?; $(TF_COMPOSE) logs --no-log-prefix proxy-$(1); $(TF_COMPOSE) down; exit $$status
endef

# Targets: Terraform in containers
.PHONY: lock
lock: mountpoints ## update the provider lock file through the registry allowlist proxy
	@# providers lock は固定中のバージョンから変更できないため、作り直す。ハッシュの変更は Git の差分で確認する
	[ ! -e $(TF_DIR)/.terraform.lock.hcl ] || rm $(TF_DIR)/.terraform.lock.hcl
	$(call tf_run_with_proxy,lock,providers lock -platform=linux_arm64 -platform=linux_amd64)

.PHONY: init
init: mountpoints ## initialize terraform with the providers in the image
	$(TF_RUN) offline init -input=false -lockfile=readonly

.PHONY: fmt
fmt: mountpoints misplaced-values ## format terraform code
	$(TF_RUN) fmt fmt -recursive

.PHONY: check
check: mountpoints misplaced-values ## check format, validate and test terraform code
	$(TF_RUN) fmt-check fmt -check -recursive
	$(TF_RUN) offline validate
	$(TF_RUN) offline test

.PHONY: show
show: mountpoints ## show the saved terraform plan
	$(TF_RUN) offline show $(TF_LOCAL_NAME)/plan.tfplan

.PHONY: output
output: mountpoints ## show the terraform outputs recorded in the state
	$(TF_RUN) offline output

# 退役させる KMS Key を、削除せずに State から外す。removed ブロックは for_each の 1 要素を指せないため、state rm を使う
# 名前は環境変数として読み、variables.tf の kms_keys の名前の形を満たすまでコマンドに展開しない
# 外す前の State は、鍵の名前を付けたバックアップに残す。既定の置き場所は読み取り専用の terraform/ のため、local/ を指定する
.PHONY: state-rm-kms-key
state-rm-kms-key: mountpoints ## remove the retired KMS key KMS_KEY_NAME from the terraform state without deleting the key
	@# grep は行ごとに判定するため、改行などの許さない文字を先に case で拒否する
	@case "$${KMS_KEY_NAME:-}" in "" | *[!a-z0-9-]*) false ;; esac \
		&& printf '%s\n' "$$KMS_KEY_NAME" | grep -Eqx 'key-[1-9][0-9]{0,8}' \
		|| (echo "KMS_KEY_NAME must be the name of the retired key in kms_keys, such as key-1" && exit 1)
	$(TF_RUN) offline state rm -backup=$(TF_LOCAL_NAME)/terraform.tfstate.$(KMS_KEY_NAME).backup 'aws_kms_external_key.signing["$(KMS_KEY_NAME)"]'

# Terraform はエラーで止まった Plan も保存するため、失敗したら捨てる。prevent_destroy で止めた鍵の置換を含む Plan が残るのを防ぐ
.PHONY: plan
plan: aws-credentials mountpoints package-checksum ## create a terraform plan with temporary AWS credentials through the AWS allowlist proxy
	($(call tf_run_with_proxy,aws,plan -var-file=$(TF_LOCAL_NAME)/terraform.tfvars -var=lambda_package_path=$(LAMBDA_PACKAGE) -out=$(TF_LOCAL_NAME)/plan.tfplan)) \
		|| ([ ! -e $(TF_LOCAL_DIR)/plan.tfplan ] || rm $(TF_LOCAL_DIR)/plan.tfplan; exit 1)

# 確認した Plan と違う変更を適用しないよう、保存した Plan の適用だけに絞る。Apply は Plan の後のデプロイパッケージを読むため、Plan のときと同じかも確かめる
.PHONY: apply
apply: aws-credentials mountpoints package-unchanged ## apply the saved terraform plan with temporary AWS credentials through the AWS allowlist proxy
	$(call tf_run_with_proxy,aws,apply $(TF_LOCAL_NAME)/plan.tfplan)

# 古いコードを配布しないよう、Plan のたびにデプロイパッケージを作り直す。
# Plan が失敗すると前回の Plan が残り、新しいチェックサムと組み合わさって Apply できてしまうため、作り直す前に捨てる
.PHONY: package-checksum
package-checksum: local-dir
	[ ! -e $(TF_LOCAL_DIR)/plan.tfplan ] || rm $(TF_LOCAL_DIR)/plan.tfplan
	$(MAKE) -C .. lambda-package
	shasum -a 256 $(LAMBDA_PACKAGE) > $(PACKAGE_CHECKSUM)

.PHONY: package-unchanged
package-unchanged:
	shasum -a 256 -c $(PACKAGE_CHECKSUM)

# AWS_SESSION_TOKEN を必須にして、長期のアクセスキーでは起動できないようにする。ルートの Makefile の KMS の操作も使う
.PHONY: aws-credentials
aws-credentials:
	@: "$${AWS_ACCESS_KEY_ID:?AWS_ACCESS_KEY_ID is required}"
	@: "$${AWS_SECRET_ACCESS_KEY:?AWS_SECRET_ACCESS_KEY is required}"
	@: "$${AWS_SESSION_TOKEN:?AWS_SESSION_TOKEN is required: use temporary credentials}"

.PHONY: mountpoints
mountpoints: local-dir
	@mkdir -p $(DIST_DIR)

.PHONY: local-dir
local-dir:
	@mkdir -p $(TF_LOCAL_DIR)

# Targets: Static Analysis
.PHONY: lint
lint: local-dir misplaced-values ## static analysis by tflint in an isolated container
	TFLINT_TARGET=$(abspath $(TF_DIR)) docker compose -f ../scripts/tflint/compose.yaml run --build --rm -T --interactive=false lint

# compose.yaml が隠すのは手元の値の置き場所だけで、ほかの場所のファイルは静的解析のコンテナから読める。ルートの Makefile の tf-misplaced-values も使う
.PHONY: misplaced-values
misplaced-values:
	@! find $(TF_DIR) -path $(TF_LOCAL_DIR) -prune -o \( -name '*.tfvars' -o -name '*.tfstate' -o -name '*.tfstate.*' -o -name '*.tfplan' \
		-o -name .terraform -o -name .envrc \) -print | grep . \
		|| (echo "move the files above to $(abspath $(TF_LOCAL_DIR))/: containers can read them" && exit 1)

# Targets
.PHONY: help
help: ### Display help
	@grep --no-filename -E '^[a-zA-Z0-9_/-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	| grep -v '###' | sort -u \
	| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
