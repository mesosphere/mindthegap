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

.PHONY: skopeo.build
skopeo.build:  ## Builds the skopeo static binary
skopeo.build: skopeo/static/skopeo-$(GOOS)-$(GOARCH)$(if $(filter $(GOOS),windows),.exe)
	$(MAKE) upx UPX_TARGET=skopeo/static/skopeo-$(GOOS)-$(GOARCH)$(if $(filter $(GOOS),windows),.exe)

.PHONY: skopeo.build.all
skopeo.build.all:
	$(MAKE) --no-print-directory GOOS=linux GOARCH=amd64 skopeo.build
	$(MAKE) --no-print-directory GOOS=linux GOARCH=arm64 skopeo.build
	$(MAKE) --no-print-directory GOOS=darwin GOARCH=amd64 skopeo.build
	$(MAKE) --no-print-directory GOOS=darwin GOARCH=arm64 skopeo.build
	$(MAKE) --no-print-directory GOOS=windows GOARCH=amd64 skopeo.build
	$(MAKE) --no-print-directory GOOS=windows GOARCH=arm64 skopeo.build

ifeq ($(IS_SNAPSHOT),false)
.PHONY: skopeo/static/skopeo-$(GOOS)-$(GOARCH)$(if $(filter $(GOOS),windows),.exe)
endif
skopeo/static/skopeo-$(GOOS)-$(GOARCH)$(if $(filter $(GOOS),windows),.exe): ; $(info $(M) Building skopeo for $(GOOS)/$(GOARCH))
	mkdir -p $(dir $@)
	rm -f $(REPO_ROOT)/$@
	cd skopeo-static && \
		CGO_ENABLED=0 go build -o $(REPO_ROOT)/$@ \
			-trimpath -ldflags='-s -w' \
			-tags containers_image_openpgp \
			github.com/containers/skopeo/cmd/skopeo
