.DEFAULT_GOAL := help

SHELL := /bin/bash -euo pipefail

INTERACTIVE := $(shell [ -t 0 ] && echo 1)

export DOCKER_REPOSITORY ?= mesosphere

export GOMODULENAME := $(shell go list -m)

ifneq ($(shell git status --porcelain 2>/dev/null; echo $$?), 0)
	export GIT_TREE_STATE := dirty
else
	export GIT_TREE_STATE :=
endif

.PHONY: dev
dev: ## dev build
dev: clean install-tools generate build lint test mod-tidy build-snapshot

.PHONY: ci
ci: ## CI build
ci: dev diff

.PHONY: clean
clean: ## remove files created during build
	$(call print-target)
	rm -rf dist
	rm -f coverage.*

.PHONY: install-tools
install-tools: ## go install tools
	$(call print-target)
	cd tools && go install -v $(shell cd tools && go list -f '{{ join .Imports " " }}' -tags=tools)

.PHONY: generate
generate: ## go generate
generate: install-tools
	$(call print-target)
	go generate ./...

.PHONY: build
build: ## go build
	$(call print-target)
	go build -o /dev/null ./...

.PHONY: lint
lint: ## golangci-lint
lint: install-tools
	$(call print-target)
	golangci-lint run -c .golangci.yml --fix

.PHONY: test
test: ## go test with race detector and code coverage
test: install-tools
	$(call print-target)
	go-acc --covermode=atomic --output=coverage.out ./... -- -race -short -v
	go tool cover -html=coverage.out -o coverage.html

.PHONY: integration-test
integration-test: ## go test with race detector for integration tests
	$(call print-target)
	go test -race -run Integration -v ./...

.PHONY: mod-tidy
mod-tidy: ## go mod tidy
	$(call print-target)
	go mod tidy
	cd tools && go mod tidy

.PHONY: build-snapshot
build-snapshot: ## goreleaser build --snapshot --rm-dist
build-snapshot: install-tools
	$(call print-target)
	goreleaser build --snapshot --rm-dist

.PHONY: diff
diff: ## git diff
	$(call print-target)
	git diff --exit-code
	RES=$$(git status --porcelain) ; if [ -n "$$RES" ]; then echo $$RES && exit 1 ; fi

.PHONY: release
release: ## goreleaser --rm-dist
release: install-tools
	$(call print-target)
	goreleaser --rm-dist

.PHONY: release-snapshot
release-snapshot: ## goreleaser --snapshot --rm-dist
release-snapshot: install-tools
	$(call print-target)
	goreleaser release --snapshot --skip-publish --rm-dist

.PHONY: go-clean
go-clean: ## go clean build, test and modules caches
	$(call print-target)
	go clean -r -i -cache -testcache -modcache

.PHONY: docker
docker: ## run in golang container, example: make docker run="make ci"
	docker run --rm $(if $(INTERACTIVE),-it) \
		-v $(CURDIR):/repo $(args) \
		-w /repo \
		golang:1.16 $(run)

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

define print-target
    @printf "Executing target: \033[36m$@\033[0m\n"
endef
