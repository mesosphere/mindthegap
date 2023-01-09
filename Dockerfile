# Copyright 2022 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# syntax=docker/dockerfile:1

# Use distroless/static:nonroot image for a base.
FROM --platform=linux/amd64 gcr.io/distroless/static@sha256:39e460e64a929bb6d08a7b899eb76c78c17a487b84f7cfe5191415473423471f as linux-amd64
FROM --platform=linux/arm64 gcr.io/distroless/static@sha256:b5e90ec08ae3e1e72b28a92caf75e9e9f6eae54624e34486155349843d420126 as linux-arm64

FROM --platform=linux/${TARGETARCH} linux-${TARGETARCH}

# Run as nonroot user using numeric ID for compatibllity.
USER 65532

COPY mindthegap /usr/local/bin/mindthegap

ENTRYPOINT ["/usr/local/bin/mindthegap"]
