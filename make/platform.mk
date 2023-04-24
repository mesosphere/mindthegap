# Copyright 2021-2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ifeq ($(OS), darwin)
  BREW_PREFIX := $(shell brew --prefix 2>/dev/null)
  ifeq ($(BREW_PREFIX),)
    $(error Unable to discover brew prefix - do you have brew installed? See https://brew.sh/ for details of how to install)
  endif

  COREUTILS_GNUBIN_PATH := $(BREW_PREFIX)/opt/coreutils/libexec/gnubin
  ifeq ($(wildcard $(COREUTILS_GNUBIN_PATH)/*),)
    $(error Cannot find GNU coreutils - have you installed them via brew? See https://formulae.brew.sh/formula/coreutils for details)
  endif
  export PATH := $(COREUTILS_GNUBIN_PATH):$(PATH)

  FINDUTILS_GNUBIN_PATH := $(BREW_PREFIX)/opt/findutils/libexec/gnubin
  ifeq ($(wildcard $(FINDUTILS_GNUBIN_PATH)/*),)
    $(error Cannot find GNU findutils - have you installed them via brew? See https://formulae.brew.sh/formula/findutils for details)
  endif
  export PATH := $(FINDUTILS_GNUBIN_PATH):$(PATH)
endif
