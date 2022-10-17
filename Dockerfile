# Copyright 2022 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# syntax=docker/dockerfile:1

# Use distroless/static:nonroot image for a base.
FROM --platform=linux/amd64 gcr.io/distroless/static@sha256:6e5f8857479b83d032a14a17f8e0731634c6b8b5e225f53a039085ec1f7698c6 as linux-amd64
FROM --platform=linux/arm64 gcr.io/distroless/static@sha256:d79a4342bd72644f30436ae22e55ab68a7c3a125e91d76936bcb2be66aa2af57 as linux-arm64

FROM --platform=linux/${TARGETARCH} linux-${TARGETARCH}

# Run as nonroot user using numeric ID for compatibllity.
USER 65532

COPY mindthegap /usr/local/bin/mindthegap

ENTRYPOINT ["/usr/local/bin/mindthegap"]
