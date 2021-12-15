# This is compatible with Darwin, see https://itnext.io/upgrading-bash-on-macos-7138bd1066ba
SHELL := /usr/bin/env bash
.SHELLFLAGS = -euo pipefail -c

# We need to explicitly get the bash version via shell command here because the user could be
# running a different shell and hence BASH_VERSION var will not be set in the Make environment.
BASH_VERSION := $(shell echo $${BASH_VERSION})
ifneq (5, $(word 1, $(sort 5 $(BASH_VERSION))))
  $(error Only bash >= 5 is supported (current version: $(BASH_VERSION)). Please upgrade your version of bash. If on macOS, see https://formulae.brew.sh/formula/bash)
endif
