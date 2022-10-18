# Copyright 2021 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: upx
upx: UPX_REAL_TARGET := $(addsuffix $(if $(filter $(GOOS),windows),.exe),$(basename $(UPX_TARGET)))
ifneq ($(IS_SNAPSHOT),true)
ifeq ($(GOOS)/$(GOARCH),windows/arm64)
upx: ; $(info $(M) skipping packing $(UPX_REAL_TARGET) - $(GOOS)/$(GOARCH) is not yet supported by upx)
else ifeq ($(GOOS),darwin)
upx: ; $(info $(M) skipping packing $(UPX_REAL_TARGET) - upx produces corrupt binaries for $(GOOS))
else
upx: install-tool.upx
upx: ## Pack executable using upx
upx: ; $(info $(M) packing $(UPX_REAL_TARGET))
	(upx -l $(UPX_REAL_TARGET) &>/dev/null && echo $(UPX_REAL_TARGET) is already packed) || upx -9 $(UPX_REAL_TARGET)
# Double check file is successfully compressed - seen errors with macos binaries
	upx -t $(UPX_REAL_TARGET) &>/dev/null || (echo $(UPX_REAL_TARGET) is broken after upx compression && exit 1)
endif
endif
