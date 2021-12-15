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

OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ifeq ($(OS), darwin)
  BREW_PREFIX := $(shell brew --prefix 2>/dev/null)
  ifeq ($(BREW_PREFIX),)
    $(error Unable to discover brew prefix - do you have brew installed? See https://brew.sh/ for details of how to install)
  endif

  GNUBIN_PATH := $(BREW_PREFIX)/opt/coreutils/libexec/gnubin
  ifeq ($(wildcard $(GNUBIN_PATH)/*),)
    $(error Cannot find GNU coreutils - have you installed them via brew? See https://formulae.brew.sh/formula/coreutils for details)
  endif
  export PATH := $(GNUBIN_PATH):$(PATH)
endif
