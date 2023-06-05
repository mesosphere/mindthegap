# Copyright 2021-2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# Override this in your own top-level Makefile if this is in a different path in your repo.
GO_TOOLS_FILE ?= $(REPO_ROOT)/.go-tools

# Explicitly override GOBIN so it does not inherit from the environment - this allows for a truly
# self-contained build environment for the project.
override GOBIN := $(REPO_ROOT)/.local/bin
export GOBIN
export PATH := $(GOBIN):$(PATH)

ifneq ($(wildcard $(GO_TOOLS_FILE)),)
define install_go_tool
	mkdir -p $(GOBIN)
	CGO_ENABLED=0 go install -v $$(grep -Eo '^.+$1[^ ]+' $(GO_TOOLS_FILE))
endef

.PHONY:
install-tool.go.%: ## Installs go tools
install-tool.go.%: ; $(info $(M) installing go tool $*)
	$(call install_go_tool,$*)
endif

.PHONY: upgrade-go-tools
upgrade-go-tools: ## Upgrades all go tools to latest available versions
upgrade-go-tools: ; $(info $(M) upgrading all go tools to latest available versions)
	grep -v '# FREEZE' .go-tools | \
		grep -Eo '^[^#][^@]+' | \
			xargs -I {} bash -ec ' \
				original_module_path={}; \
				module_path={}; \
				while [ "$${module_path}" != "." ]; do \
					LATEST_VERSION=$$(go list -m $${module_path}@latest 2>/dev/null || echo ""); \
					if [ -n "$${LATEST_VERSION}" ]; then \
						sed -i "s|$${original_module_path}@.\+$$|$${original_module_path}@$${LATEST_VERSION#* }|" .go-tools; \
						exit; \
					else \
						module_path=$$(dirname $${module_path}); \
					fi; \
				done; \
				echo "Failed to find latest module version for $${original_module_path}"; \
				exit 1'
