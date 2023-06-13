# Copyright 2021-2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

SKIP_UPX := true

.PHONY: upx
upx: UPX_REAL_TARGET := $(addsuffix $(if $(filter $(GOOS),windows),.exe),$(basename $(UPX_TARGET)))
ifneq ($(SKIP_UPX),true)
ifeq ($(GOOS)/$(GOARCH),windows/arm64)
upx: ; $(info $(M) skipping packing $(UPX_REAL_TARGET) - $(GOOS)/$(GOARCH) is not yet supported by upx)
else ifeq ($(GOOS),darwin)
upx: ; $(info $(M) skipping packing $(UPX_REAL_TARGET) - $(GOOS) has a bug in packing especially on Ventura - https://github.com/upx/upx/issues/612)
else
upx: ## Pack executable using upx
upx: ; $(info $(M) packing $(UPX_REAL_TARGET))
	(upx -l $(UPX_REAL_TARGET) &>/dev/null && echo $(UPX_REAL_TARGET) is already packed) || upx -9 --lzma $(UPX_REAL_TARGET)
# Double check file is successfully compressed - seen errors with macos binaries
	upx -t $(UPX_REAL_TARGET) &>/dev/null || (echo $(UPX_REAL_TARGET) is broken after upx compression && exit 1)
endif
endif
