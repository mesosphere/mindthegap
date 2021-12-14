.PHONY: tag
tag:
ifndef NEW_GIT_TAG
	$(error Please specify git tag to create via NEW_GIT_TAG env var or make variable)
endif
	$(foreach module,\
		$(dir $(GO_SUBMODULES_NO_TOOLS)),\
		git tag -s "$(module)$(NEW_GIT_TAG)" -a -m "$(module)$(NEW_GIT_TAG)";\
	)
	git tag -s "$(NEW_GIT_TAG)" -a -m "$(NEW_GIT_TAG)"
