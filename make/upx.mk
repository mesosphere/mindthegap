.PHONY: upx
upx: UPX_REAL_TARGET := $(addsuffix $(if $(filter $(GOOS),windows),.exe),$(basename $(UPX_TARGET)))
ifneq ($(IS_SNAPSHOT),true)
ifeq ($(GOOS)/$(GOARCH),windows/arm64)
upx: ; $(info $(M) skipping packing $(UPX_REAL_TARGET) - $(GOOS)/$(GOARCH) is not yet supported by upx)
else
upx: install-tool.upx
upx: ## Pack executable using upx
upx: ; $(info $(M) packing $(UPX_REAL_TARGET))
	(upx -l $(UPX_REAL_TARGET) &>/dev/null && echo $(UPX_REAL_TARGET) is already packed) || upx -9 $(UPX_REAL_TARGET)
endif
endif
