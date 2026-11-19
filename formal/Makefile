SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
.SUFFIXES:
.DEFAULT_GOAL := help

# Targets: Formal Verification
.PHONY: all
all: alloy z3 trace ## verify formal models by Alloy and Z3, and check the trace links

.PHONY: alloy
alloy: ## verify formal models by Alloy in an isolated container
	ALLOY_TARGET=$(CURDIR)/alloy docker compose -f ../scripts/alloy/compose.yaml run --build --rm -T --interactive=false alloy exec -c '*' -o /tmp/out -t none authorization.als
	ALLOY_TARGET=$(CURDIR)/alloy docker compose -f ../scripts/alloy/compose.yaml run --rm -T --interactive=false alloy exec -c '*' -o /tmp/out -t none issuance-temporal.als

# 検査ごとに Z3 を起動し、期待する判定と比べるスクリプトを実行するため、entrypoint を sh に替える
.PHONY: z3
z3: ## verify formal specifications by Z3 in an isolated container
	Z3_TARGET=$(CURDIR)/z3 docker compose -f ../scripts/z3/compose.yaml run --build --rm -T --interactive=false --entrypoint sh z3 verify.sh

# 検査は docs/ と internal/ も読むため、リポジトリのルートで実行する
.PHONY: trace
trace: ## check the links between the IDs in docs/ and the documents, models, and tests
	go -C .. run ./formal/trace

.PHONY: trace-map
trace-map: ## print the annotations that formalize and check each clause and GOJ
	@go -C .. run ./formal/trace -map

# Targets
.PHONY: help
help: ### Display help
	@grep --no-filename -E '^[a-zA-Z0-9_/-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	| grep -v '###' | sort -u \
	| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
