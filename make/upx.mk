# Copyright 2021 Mesosphere, Inc. All rights reserved.
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
