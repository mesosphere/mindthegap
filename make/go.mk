# Copyright 2021 D2iQ, Inc. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# The GOPRIVATE environment variable controls which modules the go command considers
# to be private (not available publicly) and should therefore not use the proxy or checksum database
export GOPRIVATE ?= github.com/mesosphere

ALL_GO_SUBMODULES := $(shell find -mindepth 2 -maxdepth 2 -name go.mod -printf '%P\n' | sort)
GO_SUBMODULES_NO_TOOLS := $(filter-out $(addsuffix /go.mod,skopeo-static tools),$(ALL_GO_SUBMODULES))

ifndef GOOS
export GOOS := $(OS)
endif
ifndef GOARCH
export GOARCH := $(shell go env GOARCH)
endif

define go_test
	gotestsum \
		--junitfile junit-report.xml \
		--junitfile-testsuite-name=relative \
		--junitfile-testcase-classname=short \
		-- \
		-covermode=atomic \
		-coverprofile=coverage.out \
		-race \
		-short \
		-v \
		$(if $(GOTEST_RUN),-run "$(GOTEST_RUN)") \
		./... && \
	go tool cover \
		-html=coverage.out \
		-o coverage.html
endef

.PHONY: test
test: ## Runs go tests for all modules in repository
test: install-tool.go.gotestsum
ifneq ($(wildcard $(REPO_ROOT)/go.mod),)
	$(info $(M) running tests$(if $(GOTEST_RUN), matching "$(GOTEST_RUN)") for root module)
	$(call go_test)
endif
ifneq ($(words $(GO_SUBMODULES_NO_TOOLS)),0)
	$(MAKE) $(addprefix test.,$(GO_SUBMODULES_NO_TOOLS:/go.mod=))
endif

.PHONY: test.%
test.%: ## Runs go tests for a specific module
test.%: ; $(info $(M) running tests$(if $(GOTEST_RUN), matching "$(GOTEST_RUN)") for module $*)
	cd $* && $(call go_test)

.PHONY: integration-test
integration-test: ## Runs integration tests for all modules in repository
	$(MAKE) GOTEST_RUN=Integration test

.PHONY: integration-test.%
integration-test.%: ## Runs integration tests for a specific module
	$(MAKE) GOTEST_RUN=Integration test.$*

.PHONY: bench
bench: ## Runs go benchmarks for all modules in repository
ifneq ($(wildcard $(REPO_ROOT)/go.mod),)
	$(info $(M) running benchmarks$(if $(GOTEST_RUN), matching "$(GOTEST_RUN)") for root module)
	go test $(if $(GOTEST_RUN),-run "$(GOTEST_RUN)") -race -cover -bench=. -v ./...
endif
ifneq ($(words $(GO_SUBMODULES_NO_TOOLS)),0)
	$(MAKE) $(addprefix bench.,$(GO_SUBMODULES_NO_TOOLS:/go.mod=))
endif

.PHONY: bench.%
bench.%: ## Runs go benchmarks for a specific module
bench.%: ; $(info $(M) running benchmarks$(if $(GOTEST_RUN), matching "$(GOTEST_RUN)") for module $*)
	cd $* && go test $(if $(GOTEST_RUN),-run "$(GOTEST_RUN)") -race -cover -v ./...

GOLANGCI_CONFIG_FILE ?= $(wildcard $(REPO_ROOT)/.golangci.y*ml)

.PHONY: lint
lint: ## Runs golangci-lint for all modules in repository
lint: install-tool.go.golangci-lint
ifneq ($(wildcard $(REPO_ROOT)/go.mod),)
lint: lint.root
endif
ifneq ($(words $(GO_SUBMODULES_NO_TOOLS)),0)
lint: $(addprefix lint.,$(GO_SUBMODULES_NO_TOOLS:/go.mod=))
endif

.PHONY: lint.%
lint.%: ## Runs golangci-lint for a specific module
lint.%: install-tool.go.golangci-lint; $(info $(M) running golangci-lint for $* module)
	$(if $(filter-out root,$*),cd $* && )golangci-lint run --fix --config=$(GOLANGCI_CONFIG_FILE)
	$(if $(filter-out root,$*),cd $* && )go fmt ./...
	$(if $(filter-out root,$*),cd $* && )go fix ./...

.PHONY: mod-tidy
mod-tidy:  ## Run go mod tidy for all modules
ifneq ($(wildcard $(REPO_ROOT)/go.mod),)
	$(info $(M) running go mod tidy for root module)
	go mod tidy -v -compat=1.17
	go mod verify
endif
ifneq ($(words $(ALL_GO_SUBMODULES)),0)
	$(MAKE) $(addprefix mod-tidy.,$(ALL_GO_SUBMODULES:/go.mod=))
endif

.PHONY: mod-tidy.%
mod-tidy.%: ## Runs go mod tidy for a specific module
mod-tidy.%: ; $(info $(M) running go mod tidy for module $*)
	cd $* && go mod tidy -v -compat=1.17
	cd $* && go mod verify

.PHONY: go-clean
go-clean: ## Cleans go build, test and modules caches for all modules
ifneq ($(wildcard $(REPO_ROOT)/go.mod),)
	$(info $(M) running go clean for root module)
	go clean -r -i -cache -testcache -modcache
endif
ifneq ($(words $(ALL_GO_SUBMODULES)),0)
	$(MAKE) $(addprefix go-clean.,$(ALL_GO_SUBMODULES:/go.mod=))
endif

.PHONY: go-clean.%
go-clean.%: ## Cleans go build, test and modules caches for a specific module
go-clean.%: ; $(info $(M) running go clean for module $*)
	cd $* && go clean -r -i -cache -testcache -modcache

.PHONY: go-generate
go-generate: ## Runs go generate
go-generate: install-tool.mockery ; $(info $(M) running go generate)
	go generate ./...

.PHONY: go-mod-upgrade
go-mod-upgrade: ## Interactive check for direct module dependency upgrades
go-mod-upgrade: install-tools.go ; $(info $(M) checking for direct module dependency upgrades)
	go-mod-upgrade
