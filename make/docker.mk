# Copyright 2021 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: dockerauth
dockerauth:
ifdef DOCKER_USERNAME
ifdef DOCKER_PASSWORD
	echo -n $(DOCKER_PASSWORD) | docker login -u $(DOCKER_USERNAME) --password-stdin
endif
endif
