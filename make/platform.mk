# Copyright 2021 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

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
