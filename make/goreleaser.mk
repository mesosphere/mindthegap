GORELEASER_PARALLELISM ?= $(shell nproc --ignore=1)
GORELEASER_DEBUG ?= false

export GORELEASER_CURRENT_TAG=$(GIT_TAG)

.PHONY: build-snapshot
build-snapshot: ## Builds a snapshot with goreleaser
build-snapshot: dockerauth install-tool.goreleaser ; $(info $(M) building snapshot $*)
	goreleaser --debug=$(GORELEASER_DEBUG) \
		build \
		--snapshot \
		--rm-dist \
		--parallelism=$(GORELEASER_PARALLELISM) \
		--single-target \
		--skip-post-hooks

.PHONY: release
release: ## Builds a release with goreleaser
release: dockerauth install-tool.goreleaser ; $(info $(M) building release $*)
	goreleaser --debug=$(GORELEASER_DEBUG) \
		release \
		--rm-dist \
		--parallelism=$(GORELEASER_PARALLELISM)

.PHONY: release-snapshot
release-snapshot: ## Builds a snapshot release with goreleaser
release-snapshot: dockerauth install-tool.goreleaser ; $(info $(M) building snapshot release $*)
	goreleaser --debug=$(GORELEASER_DEBUG) \
		release \
		--snapshot \
		--skip-publish \
		--rm-dist \
		--parallelism=$(GORELEASER_PARALLELISM)
