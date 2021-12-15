.PHONY: upx
ifeq ($(IS_SNAPSHOT),false)
ifneq ($(GOOS)/$(GOARCH),windows/arm64)
upx: install-tool.upx
endif
endif
upx: UPX_REAL_TARGET := $(addsuffix $(if $(filter $(GOOS),windows),.exe),$(basename $(UPX_TARGET)))
upx: ## Pack executable using upx
upx: ; $(info $(M) packing $(UPX_REAL_TARGET))
ifeq ($(IS_SNAPSHOT),false)
ifneq ($(GOOS)/$(GOARCH),windows/arm64)
	(upx -l $(UPX_REAL_TARGET) &>/dev/null && echo $(UPX_REAL_TARGET) is already packed) || upx -9 $(UPX_REAL_TARGET)
endif
endif
