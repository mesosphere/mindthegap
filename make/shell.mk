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

# This is compatible with Darwin, see https://itnext.io/upgrading-bash-on-macos-7138bd1066ba
SHELL := /usr/bin/env bash
.SHELLFLAGS = -euo pipefail -c

# We need to explicitly get the bash version via shell command here because the user could be
# running a different shell and hence BASH_VERSION var will not be set in the Make environment.
BASH_VERSION := $(shell echo $${BASH_VERSION})
ifneq (5, $(word 1, $(sort 5 $(BASH_VERSION))))
  $(error Only bash >= 5 is supported (current version: $(BASH_VERSION)). Please upgrade your version of bash. If on macOS, see https://formulae.brew.sh/formula/bash)
endif
