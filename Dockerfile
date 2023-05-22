# Copyright 2022 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# syntax=docker/dockerfile:1

# Use distroless/static:nonroot image for a base.
FROM --platform=linux/amd64 gcr.io/distroless/static@sha256:1b4dbd7d38a0fd4bbaf5216a21a615d07b56747a96d3c650689cbb7fdc412b49 as linux-amd64
FROM --platform=linux/arm64 gcr.io/distroless/static@sha256:05810557ec4b4bf01f4df548c06cc915bb29d81cb339495fe1ad2e668434bf8e as linux-arm64

FROM --platform=linux/${TARGETARCH} linux-${TARGETARCH}

# Run as nonroot user using numeric ID for compatibllity.
USER 65532

COPY mindthegap /usr/local/bin/mindthegap

ENTRYPOINT ["/usr/local/bin/mindthegap"]
