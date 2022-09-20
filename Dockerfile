# Copyright 2022 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# syntax=docker/dockerfile:1

# Use distroless/static:nonroot image for a base.
FROM --platform=linux/amd64 gcr.io/distroless/static@sha256:d11899d4ea81ceaab422afb0af9b446bb560411b9cec4664af4f913ef85f0b62 as linux-amd64
FROM --platform=linux/arm64 gcr.io/distroless/static@sha256:690d14f648e06c53ec52a477de11f37028b764cffd18123f08bcd4179ca75a9f as linux-arm64

FROM --platform=linux/${TARGETARCH} linux-${TARGETARCH}

# Run as nonroot user using numeric ID for compatibllity.
USER 65532

COPY mindthegap /usr/local/bin/mindthegap

ENTRYPOINT ["/usr/local/bin/mindthegap"]
