# Use distroless/static:nonroot image for a base.
FROM gcr.io/distroless/static@sha256:b89b98ea1f5bc6e0b48c8be6803a155b2a3532ac6f1e9508a8bcbf99885a9152

# Run as nonroot user using numeric ID for compatibllity.
USER 65532

COPY golang-repository-template /golang-repository-template

ENTRYPOINT ["/golang-repository-template"]
