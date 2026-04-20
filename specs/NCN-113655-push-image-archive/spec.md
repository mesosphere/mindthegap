<!--
 Copyright 2021 D2iQ, Inc. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Feature Specification: Push OCI/docker image archive tarballs

**Feature Branch**: `NCN-113655/push-image-archive`
**Created**: 2026-04-17
**Status**: Draft
**Ticket**: [NCN-113655](https://jira.nutanix.com/browse/NCN-113655)
**Input**: User description: "enhance mindthegap to be able to push OCI image export tarballs"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Push an OCI image layout tarball (Priority: P1)

A platform engineer has an OCI image layout tarball produced by their build
pipeline (for example via `skopeo copy docker://… oci-archive:out.tar`,
`crane push --format=oci …`, or `buildah push … oci-archive:out.tar`). In an
air-gapped environment, they want to push that tarball's contents into their
internal OCI registry using `mindthegap` — the same tool they already use for
image bundles — rather than installing additional tooling.

**Why this priority**: This is the core value of the feature. OCI image
layout tarballs are the standards-based artifact produced by most modern
build tooling, and mindthegap users already expect mindthegap to be the
single tool for air-gapped image workflows.

**Independent Test**: Given an OCI image layout tarball containing a single
image with an `org.opencontainers.image.ref.name` annotation, running
`mindthegap push image-archive --image-archive out.tar --to-registry
registry.example.com` must result in that image being pullable from
`registry.example.com` at the embedded reference, with the same digest as in
the source tarball.

**Acceptance Scenarios**:

1. **Given** an OCI image layout tarball containing one single-platform
   image tagged `my/app:v1`, **When** the user runs `mindthegap push
   image-archive --image-archive out.tar --to-registry registry.example.com`,
   **Then** `registry.example.com/my/app:v1` is pullable and has the same
   digest as the image in the tarball.
2. **Given** an OCI image layout tarball containing a multi-arch image index
   tagged `my/app:v1`, **When** the user runs the same command, **Then**
   `registry.example.com/my/app:v1` resolves to an image index covering the
   same platforms and digests as the source.
3. **Given** an OCI image layout tarball containing multiple tagged images,
   **When** the user runs the same command, **Then** every contained image is
   pushed to its respective `<to-registry>/<repo>:<tag>`.

---

### User Story 2 - Push a docker-save tarball (Priority: P1)

A platform engineer has a tarball produced by `docker save` or `podman save`
and wants to push its contents to an OCI registry without restoring it to a
local Docker daemon first. This is the same need as User Story 1, but for
the older docker-archive format still common in CI pipelines.

**Why this priority**: Equivalent practical value to Story 1; the two
formats together cover essentially all image tarballs users will have.
Treated as P1 because the feature is incomplete without both.

**Independent Test**: Given a tarball produced by `docker save nginx:1.21.5
-o out.tar`, running `mindthegap push image-archive --image-archive out.tar
--to-registry registry.example.com` must result in `registry.example.com/
library/nginx:1.21.5` (or equivalent) being pullable with a matching digest.

**Acceptance Scenarios**:

1. **Given** a docker-save tarball with one `RepoTags` entry, **When** the
   user runs `mindthegap push image-archive --image-archive out.tar
   --to-registry registry.example.com`, **Then** the image is pushed to the
   destination at the repo+tag from the archive's `RepoTags` entry.
2. **Given** a docker-save tarball with multiple images (multiple
   `RepoTags`), **When** the user runs the same command, **Then** every
   image is pushed to its respective destination.

---

### User Story 3 - Override destination tag for a tagless archive (Priority: P2)

An engineer has an OCI image layout tarball that contains one image with no
tag annotation (for example produced by `crane push` without an explicit
tag). They need to specify the destination reference on the command line.

**Why this priority**: Needed for tagless archives to be usable at all, but
affects a narrower audience than Stories 1 and 2.

**Independent Test**: Given an OCI image layout tarball with exactly one
image and no `org.opencontainers.image.ref.name` annotation, running
`mindthegap push image-archive --image-archive out.tar --to-registry
registry.example.com --image-tag my/app:v1` must result in the image being
pushed to `registry.example.com/my/app:v1`.

**Acceptance Scenarios**:

1. **Given** a single-image tagless OCI layout tarball, **When** the user
   provides `--image-tag my/app:v1`, **Then** the image is pushed to
   `registry.example.com/my/app:v1`.
2. **Given** multiple archives OR an archive with multiple images, **When**
   the user also passes `--image-tag`, **Then** the command returns a
   validation error before making any network call.
3. **Given** a single-image archive that already has an embedded tag,
   **When** the user passes `--image-tag` with a different reference,
   **Then** the override takes precedence.

---

### User Story 4 - Helpful error when pointing `push bundle` at an image archive (Priority: P2)

A user who is familiar with `mindthegap push bundle` mistakenly points it at
an OCI or docker image archive they produced elsewhere. They need a clear,
actionable error message instead of a cryptic failure deep in bundle-config
extraction.

**Why this priority**: Discovery and usability. Without this, users will
have a confusing first encounter with the new subcommand.

**Independent Test**: Given any OCI or docker image archive, running
`mindthegap push bundle --bundle out.tar --to-registry registry.example.com`
must return a clearly worded error suggesting `mindthegap push image-archive`
instead, before any upload attempt.

**Acceptance Scenarios**:

1. **Given** an OCI image layout tarball, **When** the user runs `mindthegap
   push bundle --bundle out.tar --to-registry …`, **Then** the command exits
   non-zero with the message `file <path> appears to be an OCI/docker image
   archive, not a mindthegap bundle; use 'mindthegap push image-archive'
   instead` and makes no network calls.
2. **Given** a docker-save tarball, **When** the user runs the same command,
   **Then** the same error is emitted.
3. **Given** a real mindthegap bundle tarball, **When** the user runs the
   same command, **Then** existing behaviour is unchanged.

---

### Edge Cases

- **Empty archive**: An OCI layout tarball with an empty `index.json`, or a
  docker-save tarball with an empty `manifest.json`. Behaviour: succeed with
  no work performed, log a clear "no images found in archive" notice.
- **Unrecognised tarball**: A tarball that is neither a mindthegap bundle,
  OCI layout, nor docker-save. Behaviour: `push image-archive` errors with
  *"file `<path>` is not a recognised image archive (expected OCI image
  layout tarball or docker-save tarball)"*. `push bundle` continues to its
  existing "no bundle configuration(s) found" error because that signal is
  already well-defined.
- **Corrupt tar**: The tarball cannot be opened as a tar stream. Behaviour:
  wrap the underlying tar error with the file path.
- **Embedded tag includes registry host** (e.g. `quay.io/foo/bar:v1`): The
  host is dropped; only repo+tag are preserved and pushed under
  `--to-registry`, matching `push bundle`'s handling of origin registries.
- **Destination registry requires auth / TLS**: Same flag surface and
  behaviour as `push bundle` (CA file, insecure skip, basic auth).
- **Glob expansion yields zero files**: Behaviour matches `push bundle`'s
  existing `utils.FilesWithGlobs` handling — error out with a clear message.
- **Mixed archives across `--image-archive` flags**: All combinations of
  OCI-layout and docker-save archives in a single invocation are supported;
  each archive is detected and read independently.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The CLI MUST expose a new subcommand `mindthegap push
  image-archive` with flags: `--image-archive <path>` (repeatable,
  glob-enabled, required, at least one value), `--to-registry <host>`
  (required), `--image-tag <repo:tag>` (optional), and the same TLS/auth
  flag set as `mindthegap push bundle` (`--to-registry-ca-cert-file`,
  `--to-registry-insecure-skip-tls-verify`, `--to-registry-username`,
  `--to-registry-password`).
- **FR-002**: The subcommand MUST auto-detect the archive format from
  content (OCI image layout tarball vs docker-save tarball) without relying
  on file extensions.
- **FR-003**: The subcommand MUST push every image and image index contained
  in each supplied archive to the destination registry. Single-platform
  images use `remote.Write`; image indexes use `remote.WriteIndex`. Media
  types are preserved as-is from the source archive.
- **FR-004**: The subcommand MUST determine the destination reference from
  the archive's embedded metadata by default: docker `RepoTags[0]` for
  docker-save archives; the `org.opencontainers.image.ref.name` annotation
  for OCI layout archives. Only the repo+tag portion is preserved — any
  embedded registry host is dropped and replaced with `--to-registry`'s
  host, plus the registry URI's optional path prefix. For example, an
  embedded tag of `docker.io/library/nginx:1.21.5` pushed to `--to-registry
  registry.example.com/mirror` becomes `registry.example.com/mirror/library/
  nginx:1.21.5`.
- **FR-005**: When `--image-tag` is provided, the subcommand MUST validate
  — before making any network call — that exactly one archive with exactly
  one contained image is present. If that precondition is not met, the
  command MUST return a clear validation error.
- **FR-006**: When `--image-tag` is provided and valid, the subcommand MUST
  use that reference as the destination, overriding any embedded tag.
- **FR-007**: When an archive contains an image with no embedded tag and
  `--image-tag` is not provided, the subcommand MUST return a clear error
  identifying the offending archive and suggesting `--image-tag`.
- **FR-008**: `mindthegap push bundle` MUST, before attempting to extract
  bundle configuration, detect whether each supplied `--bundle` file is an
  OCI image layout tarball or docker-save tarball. If so, it MUST return a
  clear error naming the file and suggesting `mindthegap push image-archive`.
  Existing behaviour for real mindthegap bundles MUST be unchanged.
- **FR-009**: The archive-reader implementation MUST NOT fully extract
  archive contents to disk. OCI layout archives MUST be read via the
  existing `archives.FileSystem` pattern used by
  `docker/registry/storage/driver/archive`, with blobs read on demand.
  Docker-save archives MUST be read via `tarball.Image` with a file opener,
  matching the existing pattern in `cmd/mindthegap/importcmd/imagebundle`.
- **FR-010**: On any push failure, the subcommand MUST abort with a wrapped
  error including the offending `<repo:tag>`. Partial uploads of a given
  image MAY occur (they do in `push bundle` today as well); no new
  guarantees are introduced in v1.
- **FR-012**: When an archive contains zero images (empty `index.json` or
  empty `manifest.json`), the subcommand MUST log an informational
  "no images found in archive" notice and treat the archive as successfully
  processed. The overall command MUST still succeed if at least one
  archive was provided.
- **FR-011**: Progress output MUST use the same `output.Output` helpers as
  `push bundle` — TTY progress gauge when attached to a terminal,
  line-per-image otherwise.

### Out of Scope (v1)

- `--on-existing-tag` (overwrite / error / skip / merge-with-retain /
  merge-with-overwrite) behaviour on the destination.
- `--image-push-concurrency` for parallel pushes.
- ECR-specific behaviour: repository auto-creation, lifecycle policy files,
  credential helper integration.
- `--force-oci-media-types` media-type normalization.
- Helm chart archives and arbitrary non-image OCI artifacts.
- Recovery / resume of partial pushes.

### Key Entities

- **Image archive**: A tarball classified as either an OCI image layout
  tarball (contains `oci-layout` at the tar root) or a docker-save tarball
  (contains `manifest.json` at the tar root). Carries zero or more image
  entries.
- **Image entry**: A single image or image index within an archive, with an
  optional embedded reference (tag) and the image content itself. In the
  internal model: `{Ref name.Reference (optional), Image v1.Image, Index
  v1.ImageIndex}` with exactly one of `Image`/`Index` set.
- **Destination reference**: The fully-qualified `<registry-host>[/<path-
  prefix>]/<repo>:<tag>` where the image entry will be pushed, derived from
  the archive's embedded metadata and the `--to-registry` / `--image-tag`
  flags.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can push an OCI image layout tarball to an OCI registry
  with a single `mindthegap push image-archive` invocation, without
  pre-extracting the archive or running a helper registry.
- **SC-002**: Users can push a docker-save tarball to an OCI registry with
  the same single invocation; the feature treats the two formats
  interchangeably.
- **SC-003**: A user who runs `mindthegap push bundle` against an image
  archive receives an error message naming the file and pointing to
  `mindthegap push image-archive` within 100% of such invocations, with no
  bytes sent to the destination registry.
- **SC-004**: 100% of successfully pushed images have an identical manifest
  digest at the destination as in the source archive (no repackaging).
- **SC-005**: End-to-end tests covering OCI-layout push, docker-save push,
  multi-image push, `--image-tag` override, TLS with CA file, TLS
  skip-verify, basic auth, and `push bundle` detection all pass on CI for
  every commit.
- **SC-006**: No regression in `push bundle` behaviour for real mindthegap
  bundles. Detection overhead is bounded by a single tar header walk that
  stops at the first classifying marker (`oci-layout` or root-level
  `manifest.json`) or the end of the archive; mindthegap bundle tarballs
  contain neither marker, so the walk's cost is at most reading the tar
  headers of a real bundle.

## Assumptions

- Users know which tarball format their build tooling produces, or are
  willing to rely on auto-detection.
- The destination OCI registry supports the media types present in the
  source archive (the feature does not translate media types in v1).
- `archives.FileSystem` already used by the registry storage driver is a
  suitable abstraction for reading OCI layout tarballs — it is already a
  trusted dependency of the project.
- The existing TLS/auth flag surface of `push bundle` is the right shape to
  mirror for the new subcommand; users will expect it to behave identically.
- Basic auth and token-based auth via `authnhelpers` + the default keychain
  cover all v1 auth needs; ECR-specific flows are deferred.
- Progress UX expectations match `push bundle` — users accept sequential
  pushes in v1; parallelism is a nice-to-have, not a requirement.
