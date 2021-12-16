# Copyright 2021 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

REPO_ROOT := $(CURDIR)

include make/all.mk

ASDF_VERSION=v0.8.1

CI_DOCKER_BUILD_ARGS=ASDF_VERSION=$(ASDF_VERSION)
