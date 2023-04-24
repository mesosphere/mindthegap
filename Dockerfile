# Copyright 2022 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# syntax=docker/dockerfile:1

# Use distroless/static:nonroot image for a base.
FROM --platform=linux/amd64 gcr.io/distroless/static@sha256:81c9a17d330510c4c068d2570c2796cae06dc822014ddb79476ea136ca95ee71 as linux-amd64
FROM --platform=linux/arm64 gcr.io/distroless/static@sha256:42bf7118eb11d6e471f2e0740b8289452e5925c209da33447b00dda8f051a9ea as linux-arm64

FROM --platform=linux/${TARGETARCH} linux-${TARGETARCH}

# Run as nonroot user using numeric ID for compatibllity.
USER 65532

COPY mindthegap /usr/local/bin/mindthegap

ENTRYPOINT ["/usr/local/bin/mindthegap"]
