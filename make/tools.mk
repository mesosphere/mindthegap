# Copyright 2021 D2iQ, Inc. All rights reserved.
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
	CGO_ENABLED=0 go install -v $$(grep $1 $(GO_TOOLS_FILE))
endef

.PHONY:
install-tool.go.%: ## Installs go tools
install-tool.go.%: install-tool.golang; $(info $(M) installing go tool $*)
	$(call install_go_tool,$*)
endif

ifndef SKIP_ASDF_CHECK
ifeq ($(shell command -v asdf),)
  $(error "This repo requires asdf - see https://asdf-vm.com/guide/getting-started.html for instructions to install")
endif
endif

define install_tool
	$(if $(1), \
		(asdf plugin list 2>/dev/null | grep -E '^$(1)$$' &>/dev/null) || asdf plugin add $(1), \
		grep -Eo '^[^#]\S+' $(REPO_ROOT)/.tool-versions | \
			xargs -I{} bash -ec '(asdf plugin list 2>/dev/null | grep -E "^{}$$" &>/dev/null) || \
														asdf plugin add {}' \
	)
	asdf install $1
endef

.PHONY: install-tools
install-tools: ## Install all tools
install-tools: ; $(info $(M) installing all tools)
	$(call install_tool,)
ifneq ($(wildcard $(GO_TOOLS_FILE)),)
	cat $(GO_TOOLS_FILE) | xargs -L1 go install -v
endif

.PHONY: install-tool.%
install-tool.%: ## Install specific tool
install-tool.%: ; $(info $(M) installing $*)
	$(call install_tool,$*)

.PHONY: upgrade-tools
# ASDF plugins use different env vars for GitHub authentication when querying releases. Try to
# handle this nicely by specifying some of the known env vars to prevent rate limiting.
ifdef GITHUB_USER_TOKEN
upgrade-tools: export GITHUB_API_TOKEN=$(GITHUB_USER_TOKEN)
else
ifdef GITHUB_TOKEN
upgrade-tools: export GITHUB_API_TOKEN=$(GITHUB_TOKEN)
endif
endif
upgrade-tools: export OAUTH_TOKEN=$(GITHUB_API_TOKEN)
upgrade-tools: ## Upgrades all tools to latest available versions
upgrade-tools: upgrade-go-tools; $(info $(M) upgrading all tools to latest available versions)
	grep -Eo '^[^#]\S+' $(REPO_ROOT)/.tool-versions | \
						xargs -I{} bash -ec '(asdf plugin list 2>/dev/null | grep -E "^{}$$" &>/dev/null) || \
																 asdf plugin add {}'
	grep -v '# FREEZE' $(REPO_ROOT)/.tool-versions | \
		grep -Eo '^[^#]\S+' | \
		xargs -I{} bash -ec '\
			export VERSION="$$( \
				asdf list all {} | \
				grep -vE "(^Available versions:|-src|-dev|-latest|-stm|[-\\.]rc|-alpha|-beta|[-\\.]pre|-next|(a|b|c)[0-9]+|snapshot|master)" | \
				tail -1 \
			)" && asdf install {} $${VERSION} && asdf local {} $${VERSION}'

.PHONY: upgrade-go-tools
upgrade-go-tools: ## Upgrades all go tools to latest available versions
upgrade-go-tools: install-tool.golang; $(info $(M) upgrading all go tools to latest available versions)
	grep -v '# FREEZE' .go-tools | \
		grep -Eo '^[^#]\S+' | \
		sed 's/@.\+$$/@latest/' | \
			xargs -I {} bash -ec '\
				export LATEST_VERSION=$$(go list -m {}) && \
				sed -i "s|$${LATEST_VERSION%% *}@.\+$$|$${LATEST_VERSION/ /@}|" .go-tools'
