<!--
 Copyright 2021 D2iQ, Inc. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# mindthegap Constitution

## Core Principles

### I. CLI-Centric

The CLI commands (`create`, `push`, `serve`, `import`) are the primary
interface. Internal packages (`archive/`, `config/`, `docker/`, `helm/`,
`images/`, `containerd/`) exist to serve the CLI. The CLI surface
(flags, output, exit codes) is the public API.

### II. Test-Required

All changes must include tests. Unit tests for domain logic, e2e Ginkgo
tests for CLI commands. Writing tests before or after implementation is
flexible -- what matters is that they exist and pass before merge.

### III. SemVer with Deprecation

CLI flags, config file formats (`images.yaml`, `helm-charts.yaml`), and
bundle tarball format are the versioned API surface. Breaking changes
require a deprecation cycle: warn for at least one minor release, then
remove in the next major. Follow SemVer strictly.

### IV. YAGNI and Consistency

No speculative features -- only build what's needed now. Remove dead
code aggressively. New code must follow existing patterns and style in
the codebase. Consistency over novelty.

### V. Simplicity

Prefer straightforward, concrete Go code. Minimize abstraction layers.
The codebase should be approachable to someone reading it for the first
time.

## Development Workflow

- **Ticket tracking**: Features and bugs originate from either GitHub
  issues or JIRA tickets. When starting work via the Spec-Kit process,
  the user must be asked for the ticket source (GitHub issue or JIRA
  ticket) and its identifier.
- **Branch naming**: Branches must reference the ticket source, e.g.
  `gh-123/short-description` for GitHub issues or
  `JIRA-456/short-description` for JIRA tickets.
- **PR descriptions**: Must include a reference to the originating
  ticket (GitHub issue link or JIRA ticket ID).
- **Build system**: Use `task` (Taskfile.yaml) for all build, test,
  and release operations. `devbox` provides the reproducible
  environment.
- **Dependencies**: Managed via Dependabot. Go module updates grouped
  by patch/minor. Keep dependencies current.
- **Code review**: All changes go through GitHub pull requests. No
  direct pushes to `main`.
- **Licensing**: Apache-2.0. All source files must include the SPDX
  license header.
- **Releases**: Automated via goreleaser. Version tags follow SemVer.

## Governance

- This constitution is a living document that evolves with the project.
- Any contributor can propose amendments via the normal GitHub PR
  process.
- The constitution guides decisions but does not override sound
  engineering judgment -- pragmatic exceptions are acceptable when
  justified in the PR description.
- When in doubt, refer to the constitution. When the constitution is
  wrong, update it.

**Version**: 1.0.0 | **Ratified**: 2026-04-08 | **Last Amended**:
2026-04-08
