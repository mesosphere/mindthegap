# Copyright 2021 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: dockerauth
dockerauth:
ifdef DOCKER_USERNAME
ifdef DOCKER_PASSWORD
	echo -n $(DOCKER_PASSWORD) | docker login -u $(DOCKER_USERNAME) --password-stdin
endif
endif

.PHONY: update-distroless-base-image
update-distroless-base-image: install-tool.gcloud install-tool.gojq install-tool.go.crane; $(info $(M) updating distroless base image)
	LATEST_DISTROLESS_NONROOT_DIGEST="$$(gcloud container images list-tags gcr.io/distroless/static --format=json | gojq -r '.[] | select(.tags | index("nonroot")) | .digest')"; \
	DISTROLESS_AMD64_DIGEST="$$(crane manifest gcr.io/distroless/static@$${LATEST_DISTROLESS_NONROOT_DIGEST} | gojq -r '.manifests[] | select(.platform.os == "linux" and .platform.architecture == "amd64").digest')"; \
	DISTROLESS_ARM64_DIGEST="$$(crane manifest gcr.io/distroless/static@$${LATEST_DISTROLESS_NONROOT_DIGEST} | gojq -r '.manifests[] | select(.platform.os == "linux" and .platform.architecture == "arm64").digest')"; \
	sed -i -e "s|^\(FROM --platform=linux/amd64 gcr.io/distroless/static@\).\+$$|\1$${DISTROLESS_AMD64_DIGEST} as linux-amd64|" \
	       -e "s|^\(FROM --platform=linux/arm64 gcr.io/distroless/static@\).\+$$|\1$${DISTROLESS_ARM64_DIGEST} as linux-arm64|" \
	       Dockerfile
