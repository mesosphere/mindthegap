<!--
 Copyright 2021 D2iQ, Inc. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Push image-archive Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `mindthegap push image-archive` subcommand that pushes OCI
image layout tarballs and docker-save tarballs to an OCI registry
(analogous to `crane push`), and extend `mindthegap push bundle` with
content detection that emits a helpful error when pointed at an image
archive instead of a mindthegap bundle.

**Architecture:** Direct push via `go-containerregistry` with no
intermediate registry. A new `images/archive` package provides format
detection and an iterator over image entries. Docker archives are read
via `tarball.Image` with a file opener. OCI layout archives are read
through an `fs.FS` backed by `archives.FileSystem` (no disk extraction),
using a lightweight `partial.CompressedImageCore` implementation that
reads blobs on demand. The new cobra command in
`cmd/mindthegap/push/imagearchive` wires this up alongside the existing
TLS/auth flag surface. `push bundle` gains a pre-check hook that calls
`archive.Detect` and aborts early when it finds an image archive.

**Tech Stack:** Go 1.25 · `github.com/google/go-containerregistry` (tarball,
layout, partial, remote, name) · `github.com/mholt/archives` (already
vendored third-party for `archives.FileSystem`) · `github.com/spf13/cobra`
for the CLI · Ginkgo/Gomega for e2e tests · standard `testing` for unit
tests · Taskfile targets `task test:unit` and `task test:e2e`.

**Spec:** [`spec.md`](./spec.md)

**Ticket:** [NCN-113655](https://jira.nutanix.com/browse/NCN-113655)

---

## File Structure

**Create:**

- `images/archive/archive.go` — exported `Format`, `Detect(path)`,
  `Open(path)`, `Archive` interface, `Entry` type.
- `images/archive/docker.go` — docker-save tarball reader using
  `tarball.Image` with a file opener.
- `images/archive/oci.go` — OCI image layout tarball reader backed by
  `fs.FS` from `archives.FileSystem`.
- `images/archive/archive_test.go` — unit tests for `Detect`.
- `images/archive/docker_test.go` — unit tests for docker reader.
- `images/archive/oci_test.go` — unit tests for OCI reader.
- `images/archive/testdata/` — generated fixture tarballs (committed).
- `images/archive/testdata/gen_test.go` — helper code that builds
  fixtures (uses `TestMain` to generate on demand; fixtures themselves
  are committed as binary).
- `cmd/mindthegap/push/imagearchive/image_archive.go` — new cobra command.
- `cmd/mindthegap/push/imagearchive/image_archive_test.go` — flag
  validation and reference-resolution unit tests.
- `test/e2e/imagearchive/imagearchive_suite_test.go` — Ginkgo suite.
- `test/e2e/imagearchive/push_image_archive_test.go` — e2e scenarios.
- `test/e2e/imagearchive/detect_bundle_test.go` — e2e for `push bundle`
  detection error.
- `test/e2e/imagearchive/testdata/.gitkeep` — empty placeholder; actual
  test fixtures are built in-test.

**Modify:**

- `cmd/mindthegap/push/push.go` — register the new `image-archive`
  subcommand alongside `bundle`.
- `cmd/mindthegap/push/bundle/bundle.go` — add detection pre-check that
  calls `archive.Detect` on each resolved bundle path and returns the
  suggestion error when it finds an image archive.
- `README.md` — add "Pushing an OCI/docker image archive" section.

**Branch:** `NCN-113655/push-image-archive` (already checked out).

---

## Conventions

- Every Go file starts with the SPDX header:

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0
  ```

- E2E test files start with `//go:build e2e` on the first line after the
  license header.
- Commits use Conventional Commits (e.g. `feat:`, `test:`, `docs:`,
  `refactor:`). Include the JIRA key in the commit body when relevant.
- After each task's commit, run `task test:unit` locally when touching Go
  code, and `task test:e2e -- --focus "..."` for e2e changes. CI runs both.
- Run `golangci-lint run ./images/archive/... ./cmd/mindthegap/push/...`
  before committing code changes; the pre-commit hook does this anyway.
- When adding imports, group them in the existing order: stdlib, third
  party, `github.com/mesosphere/...` last (this is enforced by goimports
  via pre-commit).

---

## Task 1: Skeleton `images/archive` package with `Format` type

**Files:**

- Create: `images/archive/archive.go`

- [ ] **Step 1: Create `images/archive/archive.go` with the `Format` type
  and `Detect` signature**

  Purpose: lay down the public API surface before any behaviour. This
  lets every subsequent task reference concrete types.

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0

  // Package archive provides readers for image archives: OCI image
  // layout tarballs (per the OCI image-spec) and docker-save tarballs
  // (the output of `docker save`/`podman save`).
  package archive

  import (
    "fmt"
  )

  // Format identifies the type of an image archive.
  type Format int

  const (
    // FormatUnknown indicates the file is not a recognised image archive.
    FormatUnknown Format = iota
    // FormatOCILayout is an OCI image layout tarball (contains an
    // "oci-layout" file at the tar root).
    FormatOCILayout
    // FormatDockerArchive is a docker-save tarball (contains a
    // "manifest.json" file at the tar root).
    FormatDockerArchive
  )

  // String returns a human-readable name for the format.
  func (f Format) String() string {
    switch f {
    case FormatOCILayout:
        return "OCI image layout tarball"
    case FormatDockerArchive:
        return "docker-save tarball"
    case FormatUnknown:
        return "unknown"
    default:
        return fmt.Sprintf("Format(%d)", int(f))
    }
  }
  ```

- [ ] **Step 2: Verify the package builds**

  Run: `go build ./images/archive/...`
  Expected: exit 0, no output.

- [ ] **Step 3: Commit**

  ```bash
  git add images/archive/archive.go
  git commit -m "feat(images/archive): add Format type skeleton

  Introduce the images/archive package with the Format enum that will
  drive archive type detection for the upcoming push image-archive
  subcommand.

  Refs: NCN-113655"
  ```

---

## Task 2: `Detect(path string) (Format, error)` — failing test

**Files:**

- Create: `images/archive/testdata/oci-layout-single.tar` (fixture, generated in Task 2 Step 2)
- Create: `images/archive/testdata/docker-archive-single.tar` (fixture)
- Create: `images/archive/testdata/mindthegap-bundle-like.tar` (fixture)
- Create: `images/archive/testdata/unknown.tar` (fixture)
- Create: `images/archive/testdata/empty.tar` (fixture)
- Create: `images/archive/archive_test.go`

- [ ] **Step 1: Add a helper file that builds the fixture tarballs**

  Create `images/archive/testdata_test.go` (package `archive`, so it
  can stay `internal`):

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0

  package archive

  import (
    "archive/tar"
    "bytes"
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
    "time"
  )

  // writeTarFile creates a tar archive at path containing the given
  // name -> contents mapping. Files are written in the order given.
  func writeTarFile(t *testing.T, path string, files []struct {
    Name     string
    Contents []byte
  }) {
    t.Helper()

    buf := &bytes.Buffer{}
    tw := tar.NewWriter(buf)
    for _, f := range files {
        if err := tw.WriteHeader(&tar.Header{
            Name:    f.Name,
            Mode:    0o644,
            Size:    int64(len(f.Contents)),
            ModTime: time.Unix(0, 0),
        }); err != nil {
            t.Fatalf("write header %q: %v", f.Name, err)
        }
        if _, err := tw.Write(f.Contents); err != nil {
            t.Fatalf("write body %q: %v", f.Name, err)
        }
    }
    if err := tw.Close(); err != nil {
        t.Fatalf("close tar: %v", err)
    }
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        t.Fatalf("mkdir: %v", err)
    }
    if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
        t.Fatalf("write tar %s: %v", path, err)
    }
  }

  // randomBlob returns a random byte slice of length n with its sha256
  // hex digest.
  func randomBlob(t *testing.T, n int) (digestHex string, data []byte) {
    t.Helper()
    data = make([]byte, n)
    if _, err := rand.Read(data); err != nil {
        t.Fatalf("rand: %v", err)
    }
    sum := sha256.Sum256(data)
    return hex.EncodeToString(sum[:]), data
  }

  // mustJSON marshals v or fails the test.
  func mustJSON(t *testing.T, v interface{}) []byte {
    t.Helper()
    b, err := json.Marshal(v)
    if err != nil {
        t.Fatalf("marshal: %v", err)
    }
    return b
  }
  ```

- [ ] **Step 2: Write `archive_test.go` with failing `TestDetect`**

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0

  package archive_test

  import (
    "path/filepath"
    "testing"

    "github.com/mesosphere/mindthegap/images/archive"
  )

  func TestDetect(t *testing.T) {
    // The fixtures are built by tests in this package; for Detect we
    // only need minimal tarballs with specific root-level entries.
    tests := []struct {
        name string
        // files lists the tar entries in order of appearance.
        files []struct {
            Name     string
            Contents []byte
        }
        want    archive.Format
        wantErr bool
    }{
        {
            name: "OCI layout",
            files: []struct {
                Name     string
                Contents []byte
            }{
                {Name: "oci-layout", Contents: []byte(`{"imageLayoutVersion":"1.0.0"}`)},
                {Name: "index.json", Contents: []byte(`{"schemaVersion":2}`)},
            },
            want: archive.FormatOCILayout,
        },
        {
            name: "docker-save",
            files: []struct {
                Name     string
                Contents []byte
            }{
                {Name: "manifest.json", Contents: []byte(`[]`)},
            },
            want: archive.FormatDockerArchive,
        },
        {
            name: "mindthegap-bundle-like",
            files: []struct {
                Name     string
                Contents []byte
            }{
                {Name: "images.yaml", Contents: []byte(`{}`)},
                {Name: "docker/registry/v2/repositories/", Contents: nil},
            },
            want: archive.FormatUnknown,
        },
        {
            name: "unknown",
            files: []struct {
                Name     string
                Contents []byte
            }{
                {Name: "random.txt", Contents: []byte(`hi`)},
            },
            want: archive.FormatUnknown,
        },
        {
            name:  "empty",
            files: nil,
            want:  archive.FormatUnknown,
        },
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            path := filepath.Join(t.TempDir(), "input.tar")
            writeTarFileExt(t, path, tc.files)
            got, err := archive.Detect(path)
            if tc.wantErr {
                if err == nil {
                    t.Fatalf("expected error, got format=%v", got)
                }
                return
            }
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if got != tc.want {
                t.Fatalf("got %v, want %v", got, tc.want)
            }
        })
    }
  }
  ```

  Note: `writeTarFileExt` is an external-package shim because
  `archive_test.go` is in `package archive_test`. Add it now in a tiny
  `export_test.go`:

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0

  package archive

  import "testing"

  // WriteTarFileExt is exposed so that tests in other packages can use
  // the same fixture builder.
  func WriteTarFileExt(t *testing.T, path string, files []struct {
    Name     string
    Contents []byte
  }) {
    t.Helper()
    writeTarFile(t, path, files)
  }
  ```

  Then update `archive_test.go` to call
  `archive.WriteTarFileExt(t, path, tc.files)` instead of
  `writeTarFileExt`. (Alternative: move `archive_test.go` into
  `package archive`. Prefer `package archive_test` because it keeps the
  Detect test against the public API.)

  Revise the call:

  ```go
  archive.WriteTarFileExt(t, path, tc.files)
  ```

- [ ] **Step 3: Run test; verify it fails on "undefined: Detect"**

  Run: `go test ./images/archive/... -run TestDetect -v`
  Expected: compile error or test fail — "undefined: archive.Detect".

- [ ] **Step 4: Implement `Detect`**

  Append to `images/archive/archive.go`:

  ```go
  import (
    "archive/tar"
    "errors"
    "fmt"
    "io"
    "os"
    "path"
  )

  // Detect classifies the tar archive at the given path. Detection is a
  // single streaming scan of the tar headers that stops as soon as an
  // OCI layout marker ("oci-layout") or docker-save marker
  // ("manifest.json") is seen at the tar root, or when the entire
  // archive has been walked. Files inside subdirectories are ignored
  // because both markers must exist at depth 0 per their respective
  // specs.
  func Detect(archivePath string) (Format, error) {
    f, err := os.Open(archivePath)
    if err != nil {
        return FormatUnknown, fmt.Errorf("opening archive %s: %w", archivePath, err)
    }
    defer f.Close()

    tr := tar.NewReader(f)
    for {
        hdr, err := tr.Next()
        switch {
        case errors.Is(err, io.EOF):
            return FormatUnknown, nil
        case err != nil:
            return FormatUnknown, fmt.Errorf("reading tar %s: %w", archivePath, err)
        }

        name := path.Clean(hdr.Name)
        // Only consider root-level regular files.
        if path.Dir(name) != "." {
            continue
        }
        switch name {
        case "oci-layout":
            return FormatOCILayout, nil
        case "manifest.json":
            return FormatDockerArchive, nil
        }
    }
  }
  ```

  Note on shadowing: the `archive.go` file currently has one import
  block (from Task 1). Extend that block rather than adding a second
  one.

- [ ] **Step 5: Run test; verify it passes**

  Run: `go test ./images/archive/... -run TestDetect -v`
  Expected: PASS for all five subtests.

- [ ] **Step 6: Commit**

  ```bash
  git add images/archive/archive.go images/archive/archive_test.go \
         images/archive/export_test.go images/archive/testdata_test.go
  git commit -m "feat(images/archive): detect OCI layout and docker-save tarballs

  Detect scans tar headers once at the root level looking for
  oci-layout or manifest.json markers, returning FormatUnknown when
  neither is present. This primitive will be used by push bundle to
  short-circuit with a helpful error when the user points it at an
  image archive.

  Refs: NCN-113655"
  ```

---

## Task 3: `Archive` interface, `Entry` type, and `Open` dispatcher

**Files:**

- Modify: `images/archive/archive.go`

- [ ] **Step 1: Write failing test for `Open` dispatch**

  Append to `images/archive/archive_test.go`:

  ```go
  func TestOpenDispatch(t *testing.T) {
    // OCI layout fixture (minimal; we don't iterate entries here, we
    // just verify dispatch returns the right concrete type signal
    // via Format()).
    ociPath := filepath.Join(t.TempDir(), "oci.tar")
    archive.WriteTarFileExt(t, ociPath, []struct {
        Name     string
        Contents []byte
    }{
        {Name: "oci-layout", Contents: []byte(`{"imageLayoutVersion":"1.0.0"}`)},
        {Name: "index.json", Contents: []byte(`{"schemaVersion":2,"manifests":[]}`)},
    })

    a, err := archive.Open(ociPath)
    if err != nil {
        t.Fatalf("Open: %v", err)
    }
    defer a.Close()
    if a.Format() != archive.FormatOCILayout {
        t.Fatalf("got format %v, want FormatOCILayout", a.Format())
    }

    // Docker archive fixture.
    dockerPath := filepath.Join(t.TempDir(), "docker.tar")
    archive.WriteTarFileExt(t, dockerPath, []struct {
        Name     string
        Contents []byte
    }{
        {Name: "manifest.json", Contents: []byte(`[]`)},
    })

    a2, err := archive.Open(dockerPath)
    if err != nil {
        t.Fatalf("Open: %v", err)
    }
    defer a2.Close()
    if a2.Format() != archive.FormatDockerArchive {
        t.Fatalf("got format %v, want FormatDockerArchive", a2.Format())
    }

    // Unknown fixture should error.
    unkPath := filepath.Join(t.TempDir(), "unknown.tar")
    archive.WriteTarFileExt(t, unkPath, []struct {
        Name     string
        Contents []byte
    }{
        {Name: "random.txt", Contents: []byte(`hi`)},
    })

    if _, err := archive.Open(unkPath); err == nil {
        t.Fatalf("expected error for unknown format, got nil")
    }
  }
  ```

- [ ] **Step 2: Run test to confirm failure**

  Run: `go test ./images/archive/... -run TestOpenDispatch -v`
  Expected: compile error — "undefined: archive.Open", etc.

- [ ] **Step 3: Add `Archive`, `Entry`, and `Open` skeleton**

  Append to `images/archive/archive.go`:

  ```go
  import (
    // existing imports plus:
    "github.com/google/go-containerregistry/pkg/name"
    v1 "github.com/google/go-containerregistry/pkg/v1"
  )

  // Entry represents a single image or image index contained in an
  // archive, with an optional embedded reference.
  //
  // Exactly one of Image or Index is non-nil.
  type Entry struct {
    // Ref is the embedded reference from the archive, or the zero
    // value if the archive did not carry one. Docker archives use
    // the first entry of RepoTags; OCI archives use the
    // org.opencontainers.image.ref.name annotation on the top-level
    // descriptor.
    Ref name.Reference
    // Image is non-nil for single-manifest entries.
    Image v1.Image
    // Index is non-nil for image-index entries (multi-platform).
    Index v1.ImageIndex
  }

  // Archive iterates image entries in an archive.
  type Archive interface {
    // Format returns the classification of the archive.
    Format() Format
    // Entries returns all image entries in the archive. The slice
    // may be empty for an archive that contains no images.
    Entries() ([]Entry, error)
    // Close releases any resources held by the archive.
    Close() error
  }

  // Open detects the archive format and returns an Archive for reading
  // its entries. Returns an error with a friendly message if the file
  // is not a recognised image archive.
  func Open(archivePath string) (Archive, error) {
    format, err := Detect(archivePath)
    if err != nil {
        return nil, err
    }
    switch format {
    case FormatOCILayout:
        return openOCI(archivePath)
    case FormatDockerArchive:
        return openDocker(archivePath)
    case FormatUnknown:
        return nil, fmt.Errorf(
            "file %s is not a recognised image archive "+
                "(expected OCI image layout tarball or docker-save tarball)",
            archivePath,
        )
    default:
        return nil, fmt.Errorf("unhandled archive format: %v", format)
    }
  }
  ```

  Add placeholder bodies in `images/archive/oci.go` and
  `images/archive/docker.go` so the build succeeds (full
  implementations come in Tasks 4 and 5):

  `images/archive/docker.go`:

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0

  package archive

  import "errors"

  type dockerArchive struct{ path string }

  func openDocker(archivePath string) (Archive, error) {
    return &dockerArchive{path: archivePath}, nil
  }

  func (d *dockerArchive) Format() Format              { return FormatDockerArchive }
  func (d *dockerArchive) Entries() ([]Entry, error)  { return nil, errors.New("not implemented") }
  func (d *dockerArchive) Close() error                { return nil }
  ```

  `images/archive/oci.go`:

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0

  package archive

  import "errors"

  type ociArchive struct{ path string }

  func openOCI(archivePath string) (Archive, error) {
    return &ociArchive{path: archivePath}, nil
  }

  func (o *ociArchive) Format() Format               { return FormatOCILayout }
  func (o *ociArchive) Entries() ([]Entry, error)   { return nil, errors.New("not implemented") }
  func (o *ociArchive) Close() error                 { return nil }
  ```

- [ ] **Step 4: Run test to verify it passes**

  Run: `go test ./images/archive/... -run TestOpenDispatch -v`
  Expected: PASS.

- [ ] **Step 5: Commit**

  ```bash
  git add images/archive/archive.go images/archive/docker.go \
         images/archive/oci.go images/archive/archive_test.go
  git commit -m "feat(images/archive): add Archive interface and Open dispatcher

  Introduce the Archive interface and Entry struct that will carry a
  v1.Image or v1.ImageIndex with an optional embedded reference.
  Open() detects the format and routes to docker or OCI readers;
  readers currently return 'not implemented' and are fleshed out in
  follow-up commits.

  Refs: NCN-113655"
  ```

---

## Task 4: Docker-archive reader `Entries()`

**Files:**

- Modify: `images/archive/docker.go`
- Create: `images/archive/docker_test.go`

- [ ] **Step 1: Write the failing test**

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0

  package archive_test

  import (
    "path/filepath"
    "testing"

    "github.com/google/go-containerregistry/pkg/name"
    "github.com/google/go-containerregistry/pkg/v1/empty"
    "github.com/google/go-containerregistry/pkg/v1/mutate"
    "github.com/google/go-containerregistry/pkg/v1/tarball"

    "github.com/mesosphere/mindthegap/images/archive"
  )

  func TestDockerArchiveEntries(t *testing.T) {
    // Build a docker-save tarball with two tagged images using
    // go-containerregistry's tarball.MultiWriteToFile.
    path := filepath.Join(t.TempDir(), "docker.tar")

    img1, err := mutate.Canonical(empty.Image)
    if err != nil {
        t.Fatalf("canonical image: %v", err)
    }
    img2, err := mutate.Canonical(empty.Image)
    if err != nil {
        t.Fatalf("canonical image: %v", err)
    }
    tag1, err := name.NewTag("example.com/foo:v1", name.StrictValidation)
    if err != nil {
        t.Fatalf("tag1: %v", err)
    }
    tag2, err := name.NewTag("example.com/bar:v2", name.StrictValidation)
    if err != nil {
        t.Fatalf("tag2: %v", err)
    }
    if err := tarball.MultiWriteToFile(path, map[name.Tag]v1.Image{
        tag1: img1,
        tag2: img2,
    }); err != nil {
        t.Fatalf("write docker tarball: %v", err)
    }

    a, err := archive.Open(path)
    if err != nil {
        t.Fatalf("Open: %v", err)
    }
    defer a.Close()

    entries, err := a.Entries()
    if err != nil {
        t.Fatalf("Entries: %v", err)
    }
    if len(entries) != 2 {
        t.Fatalf("got %d entries, want 2", len(entries))
    }

    refs := map[string]bool{}
    for _, e := range entries {
        if e.Image == nil {
            t.Fatalf("entry has nil Image for ref %v", e.Ref)
        }
        if e.Index != nil {
            t.Fatalf("docker archive should never produce image indexes; got %v", e.Index)
        }
        if e.Ref == nil {
            t.Fatalf("docker archive entries must carry embedded ref")
        }
        refs[e.Ref.Name()] = true
    }
    if !refs[tag1.Name()] || !refs[tag2.Name()] {
        t.Fatalf("missing expected refs; got %v", refs)
    }
  }
  ```

  Also add the import:

  ```go
  v1 "github.com/google/go-containerregistry/pkg/v1"
  ```

- [ ] **Step 2: Run test; verify it fails**

  Run: `go test ./images/archive/... -run TestDockerArchiveEntries -v`
  Expected: FAIL — "not implemented".

- [ ] **Step 3: Implement `dockerArchive.Entries`**

  Replace the body of `images/archive/docker.go`:

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0

  package archive

  import (
    "encoding/json"
    "fmt"
    "io"
    "os"

    "github.com/google/go-containerregistry/pkg/name"
    "github.com/google/go-containerregistry/pkg/v1/tarball"
  )

  type dockerArchive struct {
    path string
  }

  func openDocker(archivePath string) (Archive, error) {
    return &dockerArchive{path: archivePath}, nil
  }

  func (d *dockerArchive) Format() Format { return FormatDockerArchive }

  func (d *dockerArchive) Close() error { return nil }

  // Entries reads manifest.json from the docker-save tarball and
  // returns one Entry per RepoTags value.
  //
  // Untagged images (RepoTags empty) produce a single Entry with a nil
  // Ref; the command layer decides how to handle that case.
  func (d *dockerArchive) Entries() ([]Entry, error) {
    manifests, err := d.loadManifest()
    if err != nil {
        return nil, err
    }

    opener := func() (io.ReadCloser, error) {
        f, err := os.Open(d.path)
        if err != nil {
            return nil, fmt.Errorf("opening docker archive %s: %w", d.path, err)
        }
        return f, nil
    }

    var entries []Entry
    for _, m := range manifests {
        if len(m.RepoTags) == 0 {
            // Tagless entry — caller is responsible for supplying a
            // destination reference (e.g. via --image-tag).
            img, err := tarball.Image(opener, nil)
            if err != nil {
                return nil, fmt.Errorf("reading untagged image from %s: %w", d.path, err)
            }
            entries = append(entries, Entry{Image: img})
            continue
        }
        for _, rt := range m.RepoTags {
            tag, err := name.NewTag(rt, name.StrictValidation)
            if err != nil {
                return nil, fmt.Errorf(
                    "parsing docker archive tag %q: %w", rt, err,
                )
            }
            img, err := tarball.Image(opener, &tag)
            if err != nil {
                return nil, fmt.Errorf(
                    "reading image %s from %s: %w", rt, d.path, err,
                )
            }
            entries = append(entries, Entry{Ref: tag, Image: img})
        }
    }
    return entries, nil
  }

  // dockerManifestEntry is the subset of the docker-save manifest.json
  // schema that we need.
  type dockerManifestEntry struct {
    Config   string   `json:"Config"`
    RepoTags []string `json:"RepoTags"`
    Layers   []string `json:"Layers"`
  }

  // loadManifest streams the tarball once to extract manifest.json and
  // decode it. We avoid go-containerregistry's LoadManifest here
  // because it requires an Opener and we want a single pass.
  func (d *dockerArchive) loadManifest() ([]dockerManifestEntry, error) {
    f, err := os.Open(d.path)
    if err != nil {
        return nil, fmt.Errorf("opening docker archive %s: %w", d.path, err)
    }
    defer f.Close()

    tr := tarReader(f)
    for {
        hdr, err := tr.Next()
        if err == io.EOF {
            return nil, fmt.Errorf(
                "docker archive %s: manifest.json not found", d.path,
            )
        }
        if err != nil {
            return nil, fmt.Errorf("reading docker archive %s: %w", d.path, err)
        }
        if hdr.Name != "manifest.json" {
            continue
        }
        var entries []dockerManifestEntry
        if err := json.NewDecoder(tr).Decode(&entries); err != nil {
            return nil, fmt.Errorf(
                "decoding manifest.json from %s: %w", d.path, err,
            )
        }
        return entries, nil
    }
  }
  ```

  And add a small `tarReader` helper to `archive.go`, since both
  readers will use one:

  ```go
  import "archive/tar"

  // tarReader wraps a ReadSeeker to avoid repeating the construction
  // at call sites.
  func tarReader(r io.Reader) *tar.Reader { return tar.NewReader(r) }
  ```

  (This is a hair trivial but keeps the call sites identical; if you
  prefer, inline it.)

- [ ] **Step 4: Run test; verify it passes**

  Run: `go test ./images/archive/... -run TestDockerArchiveEntries -v`
  Expected: PASS.

- [ ] **Step 5: Run full package tests**

  Run: `go test ./images/archive/... -v`
  Expected: PASS for `TestDetect`, `TestOpenDispatch`,
  `TestDockerArchiveEntries`.

- [ ] **Step 6: Commit**

  ```bash
  git add images/archive/docker.go images/archive/archive.go \
         images/archive/docker_test.go
  git commit -m "feat(images/archive): implement docker-save tarball reader

  Parse manifest.json from a docker-save tarball and emit one Entry
  per RepoTags value. Untagged entries are also surfaced with a nil
  Ref so the caller can apply a destination tag override.

  Refs: NCN-113655"
  ```

---

## Task 5: OCI image layout reader `Entries()` — failing test

**Files:**

- Create: `images/archive/oci_test.go`

- [ ] **Step 1: Write failing test using an OCI layout fixture built
  with `layout.Write` and then tarred**

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0

  package archive_test

  import (
    "archive/tar"
    "io/fs"
    "os"
    "path/filepath"
    "testing"

    "github.com/google/go-containerregistry/pkg/v1/empty"
    "github.com/google/go-containerregistry/pkg/v1/layout"
    "github.com/google/go-containerregistry/pkg/v1/mutate"

    "github.com/mesosphere/mindthegap/images/archive"
  )

  func buildOCITarball(t *testing.T, withRefName bool) string {
    t.Helper()

    // Build a layout on disk.
    layoutDir := t.TempDir()
    img, err := mutate.Canonical(empty.Image)
    if err != nil {
        t.Fatalf("canonical image: %v", err)
    }
    p, err := layout.Write(layoutDir, empty.Index)
    if err != nil {
        t.Fatalf("layout.Write: %v", err)
    }
    opts := []layout.Option{}
    if withRefName {
        opts = append(opts, layout.WithAnnotations(map[string]string{
            "org.opencontainers.image.ref.name": "example.com/foo:v1",
        }))
    }
    if err := p.AppendImage(img, opts...); err != nil {
        t.Fatalf("AppendImage: %v", err)
    }

    // Tar the layout into a single file.
    tarPath := filepath.Join(t.TempDir(), "oci.tar")
    tarF, err := os.Create(tarPath)
    if err != nil {
        t.Fatalf("create tar: %v", err)
    }
    defer tarF.Close()
    tw := tar.NewWriter(tarF)
    defer tw.Close()

    if err := filepath.WalkDir(layoutDir, func(p string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        if d.IsDir() {
            return nil
        }
        rel, err := filepath.Rel(layoutDir, p)
        if err != nil {
            return err
        }
        info, err := d.Info()
        if err != nil {
            return err
        }
        hdr, err := tar.FileInfoHeader(info, "")
        if err != nil {
            return err
        }
        hdr.Name = filepath.ToSlash(rel)
        if err := tw.WriteHeader(hdr); err != nil {
            return err
        }
        body, err := os.ReadFile(p)
        if err != nil {
            return err
        }
        _, err = tw.Write(body)
        return err
    }); err != nil {
        t.Fatalf("walk: %v", err)
    }

    return tarPath
  }

  func TestOCIArchiveEntries_WithRefName(t *testing.T) {
    tarPath := buildOCITarball(t, true)

    a, err := archive.Open(tarPath)
    if err != nil {
        t.Fatalf("Open: %v", err)
    }
    defer a.Close()

    entries, err := a.Entries()
    if err != nil {
        t.Fatalf("Entries: %v", err)
    }
    if len(entries) != 1 {
        t.Fatalf("got %d entries, want 1", len(entries))
    }
    if entries[0].Image == nil {
        t.Fatalf("entry.Image is nil; want non-nil")
    }
    if entries[0].Ref == nil {
        t.Fatalf("entry.Ref is nil; want example.com/foo:v1")
    }
    if got := entries[0].Ref.Name(); got != "example.com/foo:v1" {
        t.Fatalf("ref=%q want example.com/foo:v1", got)
    }
  }

  func TestOCIArchiveEntries_NoRefName(t *testing.T) {
    tarPath := buildOCITarball(t, false)

    a, err := archive.Open(tarPath)
    if err != nil {
        t.Fatalf("Open: %v", err)
    }
    defer a.Close()

    entries, err := a.Entries()
    if err != nil {
        t.Fatalf("Entries: %v", err)
    }
    if len(entries) != 1 {
        t.Fatalf("got %d entries, want 1", len(entries))
    }
    if entries[0].Image == nil {
        t.Fatalf("entry.Image is nil; want non-nil")
    }
    if entries[0].Ref != nil {
        t.Fatalf("entry.Ref = %v, want nil (no annotation)", entries[0].Ref)
    }
  }
  ```

- [ ] **Step 2: Run tests; verify they fail**

  Run: `go test ./images/archive/... -run TestOCIArchiveEntries -v`
  Expected: FAIL — "not implemented".

---

## Task 6: OCI image layout reader implementation

**Files:**

- Modify: `images/archive/oci.go`

- [ ] **Step 1: Replace the placeholder with the real implementation**

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0

  package archive

  import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "io/fs"
    "path"
    "sync"

    "github.com/google/go-containerregistry/pkg/name"
    v1 "github.com/google/go-containerregistry/pkg/v1"
    "github.com/google/go-containerregistry/pkg/v1/partial"
    "github.com/google/go-containerregistry/pkg/v1/types"
    "github.com/mholt/archives"
  )

  const (
    ociLayoutFile   = "oci-layout"
    ociIndexFile    = "index.json"
    ociBlobsPrefix  = "blobs/"
    ociRefNameAnnot = "org.opencontainers.image.ref.name"
  )

  type ociArchive struct {
    path string
    // fsys is the read-only fs.FS view of the tar.
    fsys fs.FS
  }

  func openOCI(archivePath string) (Archive, error) {
    fsys, err := archives.FileSystem(context.Background(), archivePath, nil)
    if err != nil {
        return nil, fmt.Errorf(
            "opening OCI layout tarball %s as filesystem: %w",
            archivePath, err,
        )
    }
    // Sanity-check the layout version file exists.
    if _, err := fs.Stat(fsys, ociLayoutFile); err != nil {
        return nil, fmt.Errorf(
            "OCI layout tarball %s is missing %s: %w",
            archivePath, ociLayoutFile, err,
        )
    }
    return &ociArchive{path: archivePath, fsys: fsys}, nil
  }

  func (o *ociArchive) Format() Format { return FormatOCILayout }

  func (o *ociArchive) Close() error { return nil }

  func (o *ociArchive) Entries() ([]Entry, error) {
    indexBytes, err := fs.ReadFile(o.fsys, ociIndexFile)
    if err != nil {
        return nil, fmt.Errorf(
            "reading %s from %s: %w", ociIndexFile, o.path, err,
        )
    }
    var idx v1.IndexManifest
    if err := json.Unmarshal(indexBytes, &idx); err != nil {
        return nil, fmt.Errorf(
            "decoding %s from %s: %w", ociIndexFile, o.path, err,
        )
    }

    var entries []Entry
    for i := range idx.Manifests {
        desc := idx.Manifests[i]
        ref, err := refFromDescriptor(desc)
        if err != nil {
            return nil, fmt.Errorf(
                "parsing embedded ref in %s: %w", o.path, err,
            )
        }
        switch {
        case desc.MediaType.IsIndex():
            ii := &fsIndex{fsys: o.fsys, desc: desc}
            entries = append(entries, Entry{Ref: ref, Index: ii})
        case desc.MediaType.IsImage():
            img, err := o.imageFromDescriptor(desc)
            if err != nil {
                return nil, err
            }
            entries = append(entries, Entry{Ref: ref, Image: img})
        default:
            return nil, fmt.Errorf(
                "%s: unsupported media type %q in index",
                o.path, desc.MediaType,
            )
        }
    }
    return entries, nil
  }

  func refFromDescriptor(desc v1.Descriptor) (name.Reference, error) {
    if desc.Annotations == nil {
        return nil, nil
    }
    raw, ok := desc.Annotations[ociRefNameAnnot]
    if !ok || raw == "" {
        return nil, nil
    }
    return name.ParseReference(raw, name.StrictValidation)
  }

  // imageFromDescriptor constructs a v1.Image backed by the fs.FS.
  func (o *ociArchive) imageFromDescriptor(desc v1.Descriptor) (v1.Image, error) {
    img := &fsImage{fsys: o.fsys, desc: desc}
    return partial.CompressedToImage(img)
  }

  // fsImage satisfies partial.CompressedImageCore backed by an fs.FS
  // following the OCI image layout convention.
  type fsImage struct {
    fsys         fs.FS
    desc         v1.Descriptor
    manifestOnce sync.Once
    manifestBuf  []byte
    manifestErr  error
  }

  var _ partial.CompressedImageCore = (*fsImage)(nil)

  func (i *fsImage) MediaType() (types.MediaType, error) {
    return i.desc.MediaType, nil
  }

  func (i *fsImage) RawManifest() ([]byte, error) {
    i.manifestOnce.Do(func() {
        i.manifestBuf, i.manifestErr = blobBytes(i.fsys, i.desc.Digest)
    })
    return i.manifestBuf, i.manifestErr
  }

  func (i *fsImage) RawConfigFile() ([]byte, error) {
    m, err := partial.Manifest(i)
    if err != nil {
        return nil, err
    }
    return blobBytes(i.fsys, m.Config.Digest)
  }

  func (i *fsImage) LayerByDigest(h v1.Hash) (partial.CompressedLayer, error) {
    m, err := partial.Manifest(i)
    if err != nil {
        return nil, err
    }
    if h == m.Config.Digest {
        return &fsBlob{fsys: i.fsys, desc: m.Config}, nil
    }
    for _, layer := range m.Layers {
        if h == layer.Digest {
            return &fsBlob{fsys: i.fsys, desc: layer}, nil
        }
    }
    return nil, fmt.Errorf("blob %s not found in manifest", h)
  }

  // fsBlob satisfies partial.CompressedLayer backed by an fs.FS.
  type fsBlob struct {
    fsys fs.FS
    desc v1.Descriptor
  }

  func (b *fsBlob) Digest() (v1.Hash, error)          { return b.desc.Digest, nil }
  func (b *fsBlob) DiffID() (v1.Hash, error)          { return b.desc.Digest, nil }
  func (b *fsBlob) Size() (int64, error)              { return b.desc.Size, nil }
  func (b *fsBlob) MediaType() (types.MediaType, error) { return b.desc.MediaType, nil }

  func (b *fsBlob) Compressed() (io.ReadCloser, error) {
    return openBlob(b.fsys, b.desc.Digest)
  }

  // fsIndex satisfies v1.ImageIndex backed by an fs.FS.
  type fsIndex struct {
    fsys         fs.FS
    desc         v1.Descriptor
    manifestOnce sync.Once
    manifestBuf  []byte
    manifestErr  error
  }

  func (ii *fsIndex) MediaType() (types.MediaType, error) {
    return ii.desc.MediaType, nil
  }

  func (ii *fsIndex) Digest() (v1.Hash, error) { return ii.desc.Digest, nil }

  func (ii *fsIndex) Size() (int64, error) { return ii.desc.Size, nil }

  func (ii *fsIndex) IndexManifest() (*v1.IndexManifest, error) {
    raw, err := ii.RawManifest()
    if err != nil {
        return nil, err
    }
    var m v1.IndexManifest
    if err := json.Unmarshal(raw, &m); err != nil {
        return nil, err
    }
    return &m, nil
  }

  func (ii *fsIndex) RawManifest() ([]byte, error) {
    ii.manifestOnce.Do(func() {
        ii.manifestBuf, ii.manifestErr = blobBytes(ii.fsys, ii.desc.Digest)
    })
    return ii.manifestBuf, ii.manifestErr
  }

  func (ii *fsIndex) Image(h v1.Hash) (v1.Image, error) {
    m, err := ii.IndexManifest()
    if err != nil {
        return nil, err
    }
    for _, d := range m.Manifests {
        if d.Digest == h && d.MediaType.IsImage() {
            img := &fsImage{fsys: ii.fsys, desc: d}
            return partial.CompressedToImage(img)
        }
    }
    return nil, fmt.Errorf("image %s not found in index", h)
  }

  func (ii *fsIndex) ImageIndex(h v1.Hash) (v1.ImageIndex, error) {
    m, err := ii.IndexManifest()
    if err != nil {
        return nil, err
    }
    for _, d := range m.Manifests {
        if d.Digest == h && d.MediaType.IsIndex() {
            return &fsIndex{fsys: ii.fsys, desc: d}, nil
        }
    }
    return nil, fmt.Errorf("index %s not found in index", h)
  }

  // blobBytes reads an OCI blob by digest from the fs.FS.
  func blobBytes(fsys fs.FS, h v1.Hash) ([]byte, error) {
    rc, err := openBlob(fsys, h)
    if err != nil {
        return nil, err
    }
    defer rc.Close()
    return io.ReadAll(rc)
  }

  // openBlob opens an OCI blob by digest via the fs.FS.
  func openBlob(fsys fs.FS, h v1.Hash) (io.ReadCloser, error) {
    p := path.Join(ociBlobsPrefix+h.Algorithm, h.Hex)
    f, err := fsys.Open(p)
    if err != nil {
        return nil, fmt.Errorf("opening blob %s: %w", p, err)
    }
    rc, ok := f.(io.ReadCloser)
    if !ok {
        return nil, errors.New("fs.File does not implement io.ReadCloser")
    }
    return rc, nil
  }
  ```

  Note: `fsIndex` intentionally does not implement `v1.ImageIndex`
  fully — it only needs the methods used by `remote.WriteIndex`. If
  any method turns out to be missing at test time, add it by reading
  from the same `fs.FS`. `remote.WriteIndex` uses `IndexManifest`,
  `MediaType`, `Image`, and `ImageIndex` — all provided above. It also
  calls `Digest` and `RawManifest`, which are provided.

- [ ] **Step 2: Run tests; verify they pass**

  Run: `go test ./images/archive/... -v`
  Expected: all tests PASS including both new OCI tests.

- [ ] **Step 3: Commit**

  ```bash
  git add images/archive/oci.go
  git commit -m "feat(images/archive): implement OCI layout tarball reader

  Read OCI image layout tarballs through archives.FileSystem with no
  disk extraction. Blobs are served on demand via an fs.FS-backed
  partial.CompressedImageCore; image indexes are returned as a small
  v1.ImageIndex implementation. The reference name (if any) is taken
  from the org.opencontainers.image.ref.name annotation on the
  top-level descriptor.

  Refs: NCN-113655"
  ```

---

## Task 7: Multi-arch OCI and empty-archive coverage

**Files:**

- Modify: `images/archive/oci_test.go`
- Modify: `images/archive/docker_test.go`

- [ ] **Step 1: Add multi-arch test**

  Append:

  ```go
  func TestOCIArchiveEntries_MultiArch(t *testing.T) {
    layoutDir := t.TempDir()
    img1, err := mutate.Canonical(empty.Image)
    if err != nil {
        t.Fatalf("canonical img1: %v", err)
    }
    img2, err := mutate.Canonical(empty.Image)
    if err != nil {
        t.Fatalf("canonical img2: %v", err)
    }
    idx := mutate.AppendManifests(empty.Index,
        mutate.IndexAddendum{
            Add: img1,
            Descriptor: v1.Descriptor{
                Platform: &v1.Platform{OS: "linux", Architecture: "amd64"},
            },
        },
        mutate.IndexAddendum{
            Add: img2,
            Descriptor: v1.Descriptor{
                Platform: &v1.Platform{OS: "linux", Architecture: "arm64"},
            },
        },
    )
    p, err := layout.Write(layoutDir, empty.Index)
    if err != nil {
        t.Fatalf("layout.Write: %v", err)
    }
    if err := p.AppendIndex(idx, layout.WithAnnotations(map[string]string{
        "org.opencontainers.image.ref.name": "example.com/multi:v1",
    })); err != nil {
        t.Fatalf("AppendIndex: %v", err)
    }
    tarPath := tarLayoutDir(t, layoutDir)

    a, err := archive.Open(tarPath)
    if err != nil {
        t.Fatalf("Open: %v", err)
    }
    defer a.Close()
    entries, err := a.Entries()
    if err != nil {
        t.Fatalf("Entries: %v", err)
    }
    if len(entries) != 1 {
        t.Fatalf("got %d entries, want 1", len(entries))
    }
    if entries[0].Index == nil {
        t.Fatalf("entry.Index is nil; want non-nil multi-arch index")
    }
    if entries[0].Image != nil {
        t.Fatalf("entry.Image is non-nil; want only Index set")
    }

    im, err := entries[0].Index.IndexManifest()
    if err != nil {
        t.Fatalf("IndexManifest: %v", err)
    }
    if len(im.Manifests) != 2 {
        t.Fatalf("got %d manifests, want 2", len(im.Manifests))
    }
  }
  ```

  And refactor the tar-layout helper into a reusable func named
  `tarLayoutDir` that wraps the tarring logic from `buildOCITarball`.
  `buildOCITarball` calls `tarLayoutDir`.

- [ ] **Step 2: Add empty-archive tests (FR-012)**

  Verify both readers return zero entries — not an error — when an
  archive contains no images.

  Append to `images/archive/oci_test.go`:

  ```go
  func TestOCIArchiveEntries_Empty(t *testing.T) {
    // A valid OCI layout with no images: write the index+layout
    // marker files without AppendImage.
    layoutDir := t.TempDir()
    if _, err := layout.Write(layoutDir, empty.Index); err != nil {
        t.Fatalf("layout.Write: %v", err)
    }
    tarPath := filepath.Join(t.TempDir(), "empty-oci.tar")
    // tarLayoutDir(t, layoutDir) returns path; or refactor as
    // tarLayoutDirTo(t, layoutDir, tarPath) and use the latter.
    tarLayoutDirTo(t, layoutDir, tarPath)

    a, err := archive.Open(tarPath)
    if err != nil {
        t.Fatalf("Open: %v", err)
    }
    defer a.Close()
    entries, err := a.Entries()
    if err != nil {
        t.Fatalf("Entries: %v", err)
    }
    if len(entries) != 0 {
        t.Fatalf("got %d entries, want 0", len(entries))
    }
  }
  ```

  Append to `images/archive/docker_test.go`:

  ```go
  func TestDockerArchiveEntries_Empty(t *testing.T) {
    path := filepath.Join(t.TempDir(), "empty-docker.tar")
    // Write a manifest.json containing an empty array.
    archive.WriteTarFileExt(t, path, []struct {
        Name     string
        Contents []byte
    }{
        {Name: "manifest.json", Contents: []byte(`[]`)},
    })
    a, err := archive.Open(path)
    if err != nil {
        t.Fatalf("Open: %v", err)
    }
    defer a.Close()
    entries, err := a.Entries()
    if err != nil {
        t.Fatalf("Entries: %v", err)
    }
    if len(entries) != 0 {
        t.Fatalf("got %d entries, want 0", len(entries))
    }
  }
  ```

  Note: you may need to adjust the tar-layout helper function
  signature so both callers work. A shape like
  `tarLayoutDirTo(t, srcDir, dstPath string)` is the most flexible.

- [ ] **Step 3: Run**

  Run: `go test ./images/archive/... -v`
  Expected: all PASS, including both new empty-archive tests.

- [ ] **Step 4: Commit**

  ```bash
  git add images/archive/oci_test.go images/archive/docker_test.go
  git commit -m "test(images/archive): cover multi-arch and empty archives

  Refs: NCN-113655"
  ```

---

## Task 8: `push image-archive` command skeleton (flags only)

**Files:**

- Create: `cmd/mindthegap/push/imagearchive/image_archive.go`
- Modify: `cmd/mindthegap/push/push.go`
- Create: `cmd/mindthegap/push/imagearchive/image_archive_test.go`

- [ ] **Step 1: Failing test for missing required flags**

  `cmd/mindthegap/push/imagearchive/image_archive_test.go`:

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0

  package imagearchive_test

  import (
    "bytes"
    "strings"
    "testing"

    "github.com/mesosphere/dkp-cli-runtime/core/output"

    "github.com/mesosphere/mindthegap/cmd/mindthegap/push/imagearchive"
  )

  func TestMissingRequiredFlags(t *testing.T) {
    buf := &bytes.Buffer{}
    out := output.NewNonInteractiveShell(buf, buf, 0)
    cmd := imagearchive.NewCommand(out)
    cmd.SilenceUsage = true
    cmd.SilenceErrors = true
    cmd.SetArgs([]string{})

    err := cmd.Execute()
    if err == nil {
        t.Fatalf("expected error, got nil")
    }
    if !strings.Contains(err.Error(), "image-archive") {
        t.Fatalf("error does not mention image-archive: %v", err)
    }
  }

  func TestMissingToRegistry(t *testing.T) {
    buf := &bytes.Buffer{}
    out := output.NewNonInteractiveShell(buf, buf, 0)
    cmd := imagearchive.NewCommand(out)
    cmd.SilenceUsage = true
    cmd.SilenceErrors = true
    cmd.SetArgs([]string{"--image-archive", "nonexistent.tar"})

    err := cmd.Execute()
    if err == nil {
        t.Fatalf("expected error, got nil")
    }
    if !strings.Contains(err.Error(), "to-registry") {
        t.Fatalf("error does not mention to-registry: %v", err)
    }
  }
  ```

- [ ] **Step 2: Run; verify fails**

  Run: `go test ./cmd/mindthegap/push/imagearchive/... -v`
  Expected: FAIL — "undefined: imagearchive.NewCommand".

- [ ] **Step 3: Implement command skeleton**

  `cmd/mindthegap/push/imagearchive/image_archive.go`:

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0

  // Package imagearchive implements the `mindthegap push image-archive`
  // subcommand that pushes OCI image layout tarballs and docker-save
  // tarballs to an OCI registry.
  package imagearchive

  import (
    "github.com/spf13/cobra"

    "github.com/mesosphere/dkp-cli-runtime/core/output"

    "github.com/mesosphere/mindthegap/cmd/mindthegap/flags"
  )

  // NewCommand returns the cobra command for `push image-archive`.
  func NewCommand(out output.Output) *cobra.Command {
    var (
        archiveFiles                  []string
        destRegistryURI               flags.RegistryURI
        destRegistryCACertificateFile string
        destRegistrySkipTLSVerify     bool
        destRegistryUsername          string
        destRegistryPassword          string
        imageTagOverride              string
    )

    cmd := &cobra.Command{
        Use:   "image-archive",
        Short: "Push OCI/docker image archive tarballs into an existing OCI registry",
        Long: "Push OCI image layout tarballs (oci-archive) and docker-save " +
            "tarballs (docker-archive) directly to an OCI registry. The " +
            "archive format is auto-detected from the file contents.",
        PreRunE: func(cmd *cobra.Command, args []string) error {
            if err := cmd.ValidateRequiredFlags(); err != nil {
                return err
            }
            return flags.ValidateFlagsThatRequireValues(cmd, "image-archive", "to-registry")
        },
        RunE: func(cmd *cobra.Command, args []string) error {
            return runPushImageArchive(
                out,
                archiveFiles,
                &destRegistryURI,
                destRegistryCACertificateFile,
                destRegistrySkipTLSVerify,
                destRegistryUsername,
                destRegistryPassword,
                imageTagOverride,
            )
        },
    }

    cmd.Flags().StringSliceVar(&archiveFiles, "image-archive", nil,
        "Tarball containing an image archive to push (OCI image layout or "+
            "docker-save format, auto-detected). Can be specified multiple "+
            "times or as a glob pattern.")
    _ = cmd.MarkFlagRequired("image-archive")

    cmd.Flags().Var(&destRegistryURI, "to-registry", "Registry to push images to. "+
        "TLS verification will be skipped when using an http:// registry.")
    _ = cmd.MarkFlagRequired("to-registry")

    cmd.Flags().StringVar(&destRegistryCACertificateFile, "to-registry-ca-cert-file", "",
        "CA certificate file used to verify TLS verification of registry to push images to")
    cmd.Flags().BoolVar(&destRegistrySkipTLSVerify, "to-registry-insecure-skip-tls-verify", false,
        "Skip TLS verification of registry to push images to (also use for non-TLS http registries)")
    cmd.MarkFlagsMutuallyExclusive(
        "to-registry-ca-cert-file",
        "to-registry-insecure-skip-tls-verify",
    )

    cmd.Flags().StringVar(&destRegistryUsername, "to-registry-username", "",
        "Username to use to log in to destination registry")
    cmd.Flags().StringVar(&destRegistryPassword, "to-registry-password", "",
        "Password to use to log in to destination registry")
    cmd.MarkFlagsRequiredTogether(
        "to-registry-username",
        "to-registry-password",
    )

    cmd.Flags().StringVar(&imageTagOverride, "image-tag", "",
        "Destination image reference (repo:tag) to use when the archive "+
            "contains a single image. Overrides any embedded tag; required "+
            "if the archive has no embedded tag. Only valid when exactly "+
            "one archive with one image is provided.")

    return cmd
  }

  // runPushImageArchive is implemented in Task 10.
  func runPushImageArchive(
    out output.Output,
    archiveFiles []string,
    destRegistryURI *flags.RegistryURI,
    destRegistryCACertificateFile string,
    destRegistrySkipTLSVerify bool,
    destRegistryUsername string,
    destRegistryPassword string,
    imageTagOverride string,
  ) error {
    return nil
  }
  ```

- [ ] **Step 4: Wire into `cmd/mindthegap/push/push.go`**

  Modify `NewCommand` to append:

  ```go
  import (
    // existing imports, plus:
    "github.com/mesosphere/mindthegap/cmd/mindthegap/push/imagearchive"
  )

  // Inside NewCommand, after adding bundleCmd:
  cmd.AddCommand(imagearchive.NewCommand(out))
  ```

- [ ] **Step 5: Run; verify tests pass**

  Run: `go test ./cmd/mindthegap/push/imagearchive/... -v`
  Expected: PASS.

  Run: `go build ./...`
  Expected: exit 0.

- [ ] **Step 6: Commit**

  ```bash
  git add cmd/mindthegap/push/imagearchive/ cmd/mindthegap/push/push.go
  git commit -m "feat(cmd/push): add push image-archive subcommand skeleton

  Register the cobra command with its full flag set (image-archive,
  to-registry, TLS/auth, image-tag); the RunE body is a stub that
  returns nil and will be implemented alongside the push logic.

  Refs: NCN-113655"
  ```

---

## Task 9: `--image-tag` validation — single archive, single image

**Files:**

- Modify: `cmd/mindthegap/push/imagearchive/image_archive.go`
- Modify: `cmd/mindthegap/push/imagearchive/image_archive_test.go`

- [ ] **Step 1: Failing tests**

  Add to `image_archive_test.go`:

  ```go
  import (
    "os"
    "path/filepath"
    // existing imports
  )

  // writeDockerTarFile writes a docker-save tarball with the given
  // tags; reused from the archive package's helpers to keep tests
  // independent.
  func writeDockerTarFile(t *testing.T, path string, tags ...string) {
    t.Helper()
    // Minimal docker archive with the given RepoTags entries.
    img, err := mutate.Canonical(empty.Image)
    if err != nil {
        t.Fatalf("canonical: %v", err)
    }
    m := map[name.Tag]v1.Image{}
    for _, tg := range tags {
        nt, err := name.NewTag(tg, name.StrictValidation)
        if err != nil {
            t.Fatalf("tag %q: %v", tg, err)
        }
        m[nt] = img
    }
    if err := tarball.MultiWriteToFile(path, m); err != nil {
        t.Fatalf("write docker tarball: %v", err)
    }
  }

  func TestImageTagValidation_SingleArchiveSingleImage(t *testing.T) {
    tmp := t.TempDir()
    archivePath := filepath.Join(tmp, "one.tar")
    writeDockerTarFile(t, archivePath, "example.com/one:v1")

    buf := &bytes.Buffer{}
    out := output.NewNonInteractiveShell(buf, buf, 0)
    cmd := imagearchive.NewCommand(out)
    cmd.SilenceUsage = true
    cmd.SilenceErrors = true
    cmd.SetArgs([]string{
        "--image-archive", archivePath,
        "--to-registry", "registry.invalid:1/",
        "--image-tag", "example.com/other:v2",
    })

    // Execute will reach the push step which will fail with a
    // networking error, but the --image-tag validation must succeed
    // first. We assert the error does NOT mention image-tag
    // validation.
    err := cmd.Execute()
    if err != nil && strings.Contains(err.Error(), "image-tag") {
        t.Fatalf("unexpected image-tag validation error: %v", err)
    }
  }

  func TestImageTagValidation_MultipleArchives(t *testing.T) {
    tmp := t.TempDir()
    a1 := filepath.Join(tmp, "a1.tar")
    a2 := filepath.Join(tmp, "a2.tar")
    writeDockerTarFile(t, a1, "example.com/one:v1")
    writeDockerTarFile(t, a2, "example.com/two:v1")

    buf := &bytes.Buffer{}
    out := output.NewNonInteractiveShell(buf, buf, 0)
    cmd := imagearchive.NewCommand(out)
    cmd.SilenceUsage = true
    cmd.SilenceErrors = true
    cmd.SetArgs([]string{
        "--image-archive", a1,
        "--image-archive", a2,
        "--to-registry", "registry.invalid:1",
        "--image-tag", "example.com/other:v2",
    })
    err := cmd.Execute()
    if err == nil {
        t.Fatalf("expected error, got nil")
    }
    if !strings.Contains(err.Error(), "single archive") {
        t.Fatalf("unexpected error: %v", err)
    }
  }

  func TestImageTagValidation_MultipleImages(t *testing.T) {
    tmp := t.TempDir()
    archivePath := filepath.Join(tmp, "multi.tar")
    writeDockerTarFile(t, archivePath,
        "example.com/one:v1", "example.com/two:v2")

    buf := &bytes.Buffer{}
    out := output.NewNonInteractiveShell(buf, buf, 0)
    cmd := imagearchive.NewCommand(out)
    cmd.SilenceUsage = true
    cmd.SilenceErrors = true
    cmd.SetArgs([]string{
        "--image-archive", archivePath,
        "--to-registry", "registry.invalid:1",
        "--image-tag", "example.com/other:v2",
    })
    err := cmd.Execute()
    if err == nil {
        t.Fatalf("expected error, got nil")
    }
    if !strings.Contains(err.Error(), "single") {
        t.Fatalf("unexpected error: %v", err)
    }
  }
  ```

  Update imports:

  ```go
  import (
    // existing
    "github.com/google/go-containerregistry/pkg/name"
    "github.com/google/go-containerregistry/pkg/v1/empty"
    "github.com/google/go-containerregistry/pkg/v1/mutate"
    "github.com/google/go-containerregistry/pkg/v1/tarball"
    v1 "github.com/google/go-containerregistry/pkg/v1"
  )
  ```

- [ ] **Step 2: Run; verify failure**

  Run: `go test ./cmd/mindthegap/push/imagearchive/... -run TestImageTag -v`
  Expected: FAIL.

- [ ] **Step 3: Implement `--image-tag` validation in `runPushImageArchive`**

  Replace the stub `runPushImageArchive` body in
  `cmd/mindthegap/push/imagearchive/image_archive.go` with:

  ```go
  import (
    // existing plus:
    "fmt"
    "github.com/google/go-containerregistry/pkg/name"
    "github.com/mesosphere/mindthegap/cmd/mindthegap/utils"
    "github.com/mesosphere/mindthegap/images/archive"
  )

  func runPushImageArchive(
    out output.Output,
    archiveFiles []string,
    destRegistryURI *flags.RegistryURI,
    destRegistryCACertificateFile string,
    destRegistrySkipTLSVerify bool,
    destRegistryUsername string,
    destRegistryPassword string,
    imageTagOverride string,
  ) error {
    paths, err := utils.FilesWithGlobs(archiveFiles)
    if err != nil {
        return err
    }

    archives, err := openArchives(out, paths)
    if err != nil {
        return err
    }
    defer closeArchives(archives)

    if err := validateImageTagOverride(archives, imageTagOverride); err != nil {
        return err
    }

    // Actual push is wired up in Task 10.
    return nil
  }

  type openedArchive struct {
    path    string
    archive archive.Archive
    entries []archive.Entry
  }

  func openArchives(out output.Output, paths []string) ([]openedArchive, error) {
    opened := make([]openedArchive, 0, len(paths))
    for _, p := range paths {
        out.StartOperationf("Opening archive %s", p)
        a, err := archive.Open(p)
        if err != nil {
            out.EndOperationWithStatus(output.Failure())
            return nil, err
        }
        entries, err := a.Entries()
        if err != nil {
            _ = a.Close()
            out.EndOperationWithStatus(output.Failure())
            return nil, fmt.Errorf("reading entries from %s: %w", p, err)
        }
        out.EndOperationWithStatus(output.Success())
        opened = append(opened, openedArchive{path: p, archive: a, entries: entries})
    }
    return opened, nil
  }

  func closeArchives(opened []openedArchive) {
    for _, o := range opened {
        _ = o.archive.Close()
    }
  }

  // validateImageTagOverride enforces the "single archive, single
  // image" precondition when --image-tag is set, and validates the
  // override parses as a valid reference.
  func validateImageTagOverride(opened []openedArchive, imageTagOverride string) error {
    if imageTagOverride == "" {
        return nil
    }
    if len(opened) != 1 {
        return fmt.Errorf(
            "--image-tag can only be used with a single archive containing a single image; got %d archives",
            len(opened),
        )
    }
    if len(opened[0].entries) != 1 {
        return fmt.Errorf(
            "--image-tag can only be used with a single archive containing a single image; archive %s contains %d entries",
            opened[0].path, len(opened[0].entries),
        )
    }
    if _, err := name.ParseReference(imageTagOverride, name.StrictValidation); err != nil {
        return fmt.Errorf("parsing --image-tag %q: %w", imageTagOverride, err)
    }
    return nil
  }
  ```

  Note: `output.Output` does not have `StartOperationf` — check the
  dkp-cli-runtime API. If it doesn't, use `out.StartOperation(fmt.Sprintf(...))`
  — that's the pattern used in `push/bundle/bundle.go`. Use it here too:

  ```go
  out.StartOperation(fmt.Sprintf("Opening archive %s", p))
  ```

- [ ] **Step 4: Run; verify tests pass**

  Run: `go test ./cmd/mindthegap/push/imagearchive/... -v`
  Expected: the validation subtests PASS. The
  `TestImageTagValidation_SingleArchiveSingleImage` test currently
  expects no "image-tag" error — with the stub push still returning
  nil, the command succeeds. That is the intended behaviour for this
  task.

- [ ] **Step 5: Commit**

  ```bash
  git add cmd/mindthegap/push/imagearchive/
  git commit -m "feat(cmd/push): validate --image-tag preconditions

  Open all supplied archives up-front, then enforce that --image-tag
  is only used with exactly one archive containing exactly one image,
  returning a clear error otherwise. The override is also parsed as a
  name.Reference so invalid refs fail fast.

  Refs: NCN-113655"
  ```

---

## Task 10: Push images to destination registry

**Files:**

- Create: `images/archive/testutil/testutil.go`
- Create: `cmd/mindthegap/push/imagearchive/push_test.go`
- Modify: `cmd/mindthegap/push/imagearchive/image_archive.go`
- Modify: `images/archive/oci_test.go` (optional: switch to testutil)

- [ ] **Step 1: Create the shared `testutil` package first**

  This package will be used by both `images/archive/oci_test.go` and
  the new end-to-end unit tests. Creating it up-front avoids
  duplicated helpers.

  `images/archive/testutil/testutil.go`:

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0

  // Package testutil provides helpers for building OCI and docker
  // image archive tarballs in tests across the mindthegap codebase.
  package testutil

  import (
    "archive/tar"
    "io/fs"
    "os"
    "path/filepath"

    "github.com/google/go-containerregistry/pkg/name"
    v1 "github.com/google/go-containerregistry/pkg/v1"
    "github.com/google/go-containerregistry/pkg/v1/empty"
    "github.com/google/go-containerregistry/pkg/v1/layout"
    "github.com/google/go-containerregistry/pkg/v1/mutate"
    "github.com/google/go-containerregistry/pkg/v1/tarball"
  )

  // TB is the minimal subset of testing.TB / ginkgo.GinkgoTInterface
  // used by helpers in this package. Defining it locally avoids
  // pulling testing.TB's unexported methods (which would stop
  // ginkgo.GinkgoTInterface from satisfying the parameter).
  type TB interface {
    Helper()
    Fatalf(format string, args ...any)
    TempDir() string
  }

  // BuildDockerArchive writes a docker-save tarball at path containing
  // one empty image per tag.
  func BuildDockerArchive(tb TB, path string, tags ...string) v1.Image {
    tb.Helper()
    img, err := mutate.Canonical(empty.Image)
    if err != nil {
        tb.Fatalf("canonical: %v", err)
    }
    m := map[name.Tag]v1.Image{}
    for _, tg := range tags {
        nt, err := name.NewTag(tg, name.StrictValidation)
        if err != nil {
            tb.Fatalf("tag %q: %v", tg, err)
        }
        m[nt] = img
    }
    if err := tarball.MultiWriteToFile(path, m); err != nil {
        tb.Fatalf("write docker tarball: %v", err)
    }
    return img
  }

  // BuildOCIArchive writes an OCI image layout tarball at tarPath with
  // a single image annotated with the given ref (empty ref means no
  // annotation). Returns the image so the caller can compare digests.
  func BuildOCIArchive(tb TB, tarPath, ref string) v1.Image {
    tb.Helper()
    layoutDir := tb.TempDir()
    img, err := mutate.Canonical(empty.Image)
    if err != nil {
        tb.Fatalf("canonical: %v", err)
    }
    p, err := layout.Write(layoutDir, empty.Index)
    if err != nil {
        tb.Fatalf("layout.Write: %v", err)
    }
    opts := []layout.Option{}
    if ref != "" {
        opts = append(opts, layout.WithAnnotations(map[string]string{
            "org.opencontainers.image.ref.name": ref,
        }))
    }
    if err := p.AppendImage(img, opts...); err != nil {
        tb.Fatalf("AppendImage: %v", err)
    }
    TarLayoutDir(tb, layoutDir, tarPath)
    return img
  }

  // TarLayoutDir tars the contents of layoutDir into tarPath.
  func TarLayoutDir(tb TB, layoutDir, tarPath string) {
    tb.Helper()
    tarF, err := os.Create(tarPath)
    if err != nil {
        tb.Fatalf("create tar: %v", err)
    }
    defer tarF.Close()
    tw := tar.NewWriter(tarF)
    defer tw.Close()

    if err := filepath.WalkDir(layoutDir, func(p string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        if d.IsDir() {
            return nil
        }
        rel, err := filepath.Rel(layoutDir, p)
        if err != nil {
            return err
        }
        info, err := d.Info()
        if err != nil {
            return err
        }
        hdr, err := tar.FileInfoHeader(info, "")
        if err != nil {
            return err
        }
        hdr.Name = filepath.ToSlash(rel)
        if err := tw.WriteHeader(hdr); err != nil {
            return err
        }
        body, err := os.ReadFile(p)
        if err != nil {
            return err
        }
        _, err = tw.Write(body)
        return err
    }); err != nil {
        tb.Fatalf("walk: %v", err)
    }
  }
  ```

  Build check: `go build ./images/archive/testutil/...`

  (Optional but recommended) In
  `images/archive/oci_test.go` replace the local `buildOCITarball`
  and `tarLayoutDir` helpers from Tasks 5 and 7 with calls to
  `testutil.BuildOCIArchive` / `testutil.TarLayoutDir`. Re-run
  `go test ./images/archive/...` to confirm nothing regresses.

- [ ] **Step 2: Write the failing end-to-end unit tests**

  Create `cmd/mindthegap/push/imagearchive/push_test.go`:

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0

  package imagearchive_test

  import (
    "bytes"
    "fmt"
    "net/http/httptest"
    "path/filepath"
    "testing"

    "github.com/google/go-containerregistry/pkg/crane"
    "github.com/google/go-containerregistry/pkg/registry"

    "github.com/mesosphere/dkp-cli-runtime/core/output"

    "github.com/mesosphere/mindthegap/cmd/mindthegap/push/imagearchive"
    "github.com/mesosphere/mindthegap/images/archive/testutil"
  )

  func TestPushDockerArchive_EndToEnd(t *testing.T) {
    reg := registry.New()
    srv := httptest.NewServer(reg)
    defer srv.Close()
    regHost := srv.Listener.Addr().String()

    tmp := t.TempDir()
    archivePath := filepath.Join(tmp, "src.tar")
    img := testutil.BuildDockerArchive(t, archivePath, "example.com/app:v1")

    buf := &bytes.Buffer{}
    out := output.NewNonInteractiveShell(buf, buf, 0)
    cmd := imagearchive.NewCommand(out)
    cmd.SilenceUsage = true
    cmd.SilenceErrors = true
    cmd.SetArgs([]string{
        "--image-archive", archivePath,
        "--to-registry", fmt.Sprintf("http://%s", regHost),
        "--to-registry-insecure-skip-tls-verify",
    })
    if err := cmd.Execute(); err != nil {
        t.Fatalf("Execute: %v\noutput:\n%s", err, buf.String())
    }

    pulled, err := crane.Pull(fmt.Sprintf("%s/app:v1", regHost))
    if err != nil {
        t.Fatalf("Pull: %v", err)
    }
    gotDigest, err := pulled.Digest()
    if err != nil {
        t.Fatalf("got digest: %v", err)
    }
    wantDigest, err := img.Digest()
    if err != nil {
        t.Fatalf("want digest: %v", err)
    }
    if gotDigest != wantDigest {
        t.Fatalf("digest mismatch: got %s, want %s", gotDigest, wantDigest)
    }
  }

  func TestPushOCIArchive_EndToEnd(t *testing.T) {
    reg := registry.New()
    srv := httptest.NewServer(reg)
    defer srv.Close()
    regHost := srv.Listener.Addr().String()

    tmp := t.TempDir()
    archivePath := filepath.Join(tmp, "src.tar")
    img := testutil.BuildOCIArchive(t, archivePath, "example.com/app:v1")

    buf := &bytes.Buffer{}
    out := output.NewNonInteractiveShell(buf, buf, 0)
    cmd := imagearchive.NewCommand(out)
    cmd.SilenceUsage = true
    cmd.SilenceErrors = true
    cmd.SetArgs([]string{
        "--image-archive", archivePath,
        "--to-registry", fmt.Sprintf("http://%s", regHost),
        "--to-registry-insecure-skip-tls-verify",
    })
    if err := cmd.Execute(); err != nil {
        t.Fatalf("Execute: %v\noutput:\n%s", err, buf.String())
    }

    pulled, err := crane.Pull(fmt.Sprintf("%s/app:v1", regHost))
    if err != nil {
        t.Fatalf("Pull: %v", err)
    }
    gotDigest, err := pulled.Digest()
    if err != nil {
        t.Fatalf("got digest: %v", err)
    }
    wantDigest, err := img.Digest()
    if err != nil {
        t.Fatalf("want digest: %v", err)
    }
    if gotDigest != wantDigest {
        t.Fatalf("digest mismatch: got %s, want %s", gotDigest, wantDigest)
    }
  }
  ```

- [ ] **Step 3: Run; verify tests fail**

  Run: `go test ./cmd/mindthegap/push/imagearchive/... -run EndToEnd -v`
  Expected: FAIL on `crane.Pull` — the stub `runPushImageArchive`
  returns nil without pushing anything, so the destination registry
  has no image.

- [ ] **Step 4: Implement the push loop**

  Replace the `runPushImageArchive` stub return with the actual push
  body. Append to `image_archive.go` (add imports as needed):

  ```go
  import (
    // existing plus:
    "github.com/containers/image/v5/docker/reference"
    "github.com/containers/image/v5/types"
    "github.com/google/go-containerregistry/pkg/authn"
    "github.com/google/go-containerregistry/pkg/v1/remote"
    ggcrname "github.com/google/go-containerregistry/pkg/name"

    "github.com/mesosphere/mindthegap/cmd/mindthegap/utils"
    "github.com/mesosphere/mindthegap/images/authnhelpers"
    "github.com/mesosphere/mindthegap/images/httputils"
  )

  // Replace the `return nil` in runPushImageArchive after
  // validateImageTagOverride with this push block:

    destTLSRoundTripper, err := httputils.TLSConfiguredRoundTripper(
        remote.DefaultTransport,
        destRegistryURI.Host(),
        flags.SkipTLSVerify(destRegistrySkipTLSVerify, destRegistryURI),
        destRegistryCACertificateFile,
    )
    if err != nil {
        return fmt.Errorf("configuring TLS for destination registry: %w", err)
    }
    destRemoteOpts := []remote.Option{
        remote.WithTransport(destTLSRoundTripper),
        remote.WithUserAgent(utils.Useragent()),
    }

    var destNameOpts []ggcrname.Option
    if flags.SkipTLSVerify(destRegistrySkipTLSVerify, destRegistryURI) {
        destNameOpts = append(destNameOpts, ggcrname.Insecure)
    }
    destNameOpts = append(destNameOpts, ggcrname.StrictValidation)

    var keychain authn.Keychain = authn.DefaultKeychain
    if destRegistryUsername != "" && destRegistryPassword != "" {
        keychain = authn.NewMultiKeychain(
            authn.NewKeychainFromHelper(
                authnhelpers.NewStaticHelper(
                    destRegistryURI.Host(),
                    &types.DockerAuthConfig{
                        Username: destRegistryUsername,
                        Password: destRegistryPassword,
                    },
                ),
            ),
            keychain,
        )
    }
    destRemoteOpts = append(destRemoteOpts, remote.WithAuthFromKeychain(keychain))

    destRegistry, err := ggcrname.NewRegistry(destRegistryURI.Host(), destNameOpts...)
    if err != nil {
        return fmt.Errorf("parsing destination registry: %w", err)
    }

    for _, oa := range archives {
        for i := range oa.entries {
            entry := oa.entries[i]
            destRef, err := resolveDestRef(destRegistry, destRegistryURI.Path(), entry, imageTagOverride)
            if err != nil {
                return fmt.Errorf("resolving destination reference for %s: %w", oa.path, err)
            }
            displayName := destRef.Name()
            out.StartOperation(fmt.Sprintf("Pushing %s", displayName))
            switch {
            case entry.Image != nil:
                if err := remote.Write(destRef, entry.Image, destRemoteOpts...); err != nil {
                    out.EndOperationWithStatus(output.Failure())
                    return fmt.Errorf("pushing %s: %w", displayName, err)
                }
            case entry.Index != nil:
                if err := remote.WriteIndex(destRef, entry.Index, destRemoteOpts...); err != nil {
                    out.EndOperationWithStatus(output.Failure())
                    return fmt.Errorf("pushing %s: %w", displayName, err)
                }
            default:
                out.EndOperationWithStatus(output.Failure())
                return fmt.Errorf("archive %s: entry has neither image nor index", oa.path)
            }
            out.EndOperationWithStatus(output.Success())
        }
    }
    return nil
  ```

  And add `resolveDestRef`:

  ```go
  // resolveDestRef decides the destination reference for the given
  // entry: use imageTagOverride when set, otherwise use the embedded
  // reference stripped of its origin registry host. The destination
  // host is always destRegistry's host; destPath (the --to-registry
  // URL path) is prepended as a path prefix.
  func resolveDestRef(
    destRegistry ggcrname.Registry,
    destPath string,
    entry archive.Entry,
    imageTagOverride string,
  ) (ggcrname.Reference, error) {
    input := imageTagOverride
    if input == "" {
        if entry.Ref == nil {
            return nil, fmt.Errorf(
                "entry has no embedded tag; pass --image-tag to specify the destination reference",
            )
        }
        input = entry.Ref.Name()
    }

    norm, err := reference.ParseNormalizedNamed(input)
    if err != nil {
        return nil, fmt.Errorf("parsing %q: %w", input, err)
    }
    repoPath := reference.Path(norm)
    tagPart := "latest"
    if tagged, ok := norm.(reference.Tagged); ok {
        tagPart = tagged.Tag()
    }

    // Use destRegistry.Repo so the destination URL path is appended
    // correctly; this mirrors the approach in push bundle.
    destRepo := destRegistry.Repo(strings.TrimLeft(destPath, "/"), repoPath)
    return destRepo.Tag(tagPart), nil
  }
  ```

  Add `"strings"` to the imports (the file already imports `fmt`,
  `archive`, `ggcrname`, etc.).

- [ ] **Step 5: Run; verify end-to-end tests pass**

  Run: `go test ./cmd/mindthegap/push/imagearchive/... -v`
  Expected: PASS including both `TestPushDockerArchive_EndToEnd` and
  `TestPushOCIArchive_EndToEnd`.

  Run the rest to make sure nothing else regressed:

  Run: `go test ./...`
  Expected: PASS.

- [ ] **Step 6: Commit**

  ```bash
  git add cmd/mindthegap/push/imagearchive/ images/archive/testutil/ \
         images/archive/oci_test.go
  git commit -m "feat(cmd/push): push image archive entries to destination registry

  Wire up the TLS/auth plumbing (mirroring push bundle), resolve each
  archive entry's destination reference using the embedded tag or
  --image-tag override, and push images via remote.Write / image
  indexes via remote.WriteIndex. End-to-end unit tests cover both
  docker-save and OCI layout archives.

  Refs: NCN-113655"
  ```

---

## Task 11: Tagless-archive error message

**Files:**

- Modify: `cmd/mindthegap/push/imagearchive/image_archive_test.go`
- Possibly adjust: `cmd/mindthegap/push/imagearchive/image_archive.go`

- [ ] **Step 1: Failing test**

  Add:

  ```go
  func TestPush_TaglessWithoutOverride(t *testing.T) {
    reg := registry.New()
    srv := httptest.NewServer(reg)
    defer srv.Close()
    regHost := srv.Listener.Addr().String()

    tmp := t.TempDir()
    archivePath := filepath.Join(tmp, "tagless.tar")
    testutil.BuildOCIArchive(t, archivePath, "") // no ref annotation

    buf := &bytes.Buffer{}
    out := output.NewNonInteractiveShell(buf, buf, 0)
    cmd := imagearchive.NewCommand(out)
    cmd.SilenceUsage = true
    cmd.SilenceErrors = true
    cmd.SetArgs([]string{
        "--image-archive", archivePath,
        "--to-registry", fmt.Sprintf("http://%s", regHost),
        "--to-registry-insecure-skip-tls-verify",
    })

    err := cmd.Execute()
    if err == nil {
        t.Fatalf("expected error, got nil")
    }
    if !strings.Contains(err.Error(), "--image-tag") {
        t.Fatalf("error does not mention --image-tag: %v", err)
    }
  }
  ```

- [ ] **Step 2: Run; verify test**

  Run: `go test ./cmd/mindthegap/push/imagearchive/... -run Tagless -v`

  The `resolveDestRef` function already returns the right error when
  `entry.Ref` is nil and no override is given. Verify the error message
  phrasing includes `--image-tag`. If the current message is
  `"entry has no embedded tag; pass --image-tag ..."` the test
  passes.

  Expected: PASS (no code change needed if phrasing already matches).
  If it doesn't, update the error in `resolveDestRef` accordingly.

- [ ] **Step 3: Commit**

  ```bash
  git add cmd/mindthegap/push/imagearchive/image_archive_test.go \
         cmd/mindthegap/push/imagearchive/image_archive.go
  git commit -m "test(cmd/push): cover tagless archive without override

  Refs: NCN-113655"
  ```

---

## Task 12: `push bundle` detection hook

**Files:**

- Modify: `cmd/mindthegap/push/bundle/bundle.go`
- Create: `cmd/mindthegap/push/bundle/detect_test.go`

- [ ] **Step 1: Failing test**

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0

  package bundle_test

  import (
    "bytes"
    "path/filepath"
    "strings"
    "testing"

    "github.com/mesosphere/dkp-cli-runtime/core/output"

    "github.com/mesosphere/mindthegap/cmd/mindthegap/push/bundle"
    "github.com/mesosphere/mindthegap/images/archive/testutil"
  )

  func TestPushBundleRejectsImageArchive(t *testing.T) {
    tmp := t.TempDir()
    archivePath := filepath.Join(tmp, "oci.tar")
    testutil.BuildOCIArchive(t, archivePath, "example.com/foo:v1")

    buf := &bytes.Buffer{}
    out := output.NewNonInteractiveShell(buf, buf, 0)
    cmd := bundle.NewCommand(out, "bundle")
    cmd.SilenceUsage = true
    cmd.SilenceErrors = true
    cmd.SetArgs([]string{
        "--bundle", archivePath,
        "--to-registry", "registry.invalid:1",
    })

    err := cmd.Execute()
    if err == nil {
        t.Fatalf("expected error, got nil")
    }
    want := "push image-archive"
    if !strings.Contains(err.Error(), want) {
        t.Fatalf("error does not mention %q: %v", want, err)
    }
    if !strings.Contains(err.Error(), "image archive") {
        t.Fatalf("error does not mention image archive: %v", err)
    }
  }

  func TestPushBundleRejectsDockerArchive(t *testing.T) {
    tmp := t.TempDir()
    archivePath := filepath.Join(tmp, "docker.tar")
    testutil.BuildDockerArchive(t, archivePath, "example.com/foo:v1")

    buf := &bytes.Buffer{}
    out := output.NewNonInteractiveShell(buf, buf, 0)
    cmd := bundle.NewCommand(out, "bundle")
    cmd.SilenceUsage = true
    cmd.SilenceErrors = true
    cmd.SetArgs([]string{
        "--bundle", archivePath,
        "--to-registry", "registry.invalid:1",
    })

    err := cmd.Execute()
    if err == nil {
        t.Fatalf("expected error, got nil")
    }
    if !strings.Contains(err.Error(), "push image-archive") {
        t.Fatalf("error does not mention push image-archive: %v", err)
    }
  }
  ```

- [ ] **Step 2: Run; verify fails**

  Run: `go test ./cmd/mindthegap/push/bundle/... -run Rejects -v`
  Expected: FAIL — existing behavior raises a different error
  ("no bundle configuration(s) found" or similar).

- [ ] **Step 3: Implement the detection hook in `PushBundles`**

  In `cmd/mindthegap/push/bundle/bundle.go`, add the import (next to
  existing `github.com/mesosphere/mindthegap/...` imports):

  ```go
  "github.com/mesosphere/mindthegap/images/archive"
  ```

  Locate the line (currently around line 288):

  ```go
  bundleFiles, err := utils.FilesWithGlobs(cfg.bundleFiles)
  if err != nil {
    return err
  }
  ```

  Immediately after the `if err != nil { return err }` block, insert:

  ```go
  if err := rejectImageArchives(bundleFiles); err != nil {
    return err
  }
  ```

  And at the bottom of the file, add:

  ```go
  // rejectImageArchives returns an error pointing users to
  // `mindthegap push image-archive` if any of the supplied files are
  // OCI image layout or docker-save tarballs rather than mindthegap
  // bundles.
  func rejectImageArchives(paths []string) error {
    for _, p := range paths {
        format, err := archive.Detect(p)
        if err != nil {
            return fmt.Errorf("inspecting bundle %s: %w", p, err)
        }
        if format == archive.FormatUnknown {
            continue
        }
        return fmt.Errorf(
            "file %s appears to be an OCI/docker image archive, not a "+
                "mindthegap bundle; use 'mindthegap push image-archive' instead",
            p,
        )
    }
    return nil
  }
  ```

- [ ] **Step 4: Run; verify tests pass**

  Run: `go test ./cmd/mindthegap/push/bundle/... -v`
  Expected: existing tests continue to pass AND both rejection tests
  pass.

  Run: `go test ./...`
  Expected: PASS across the board.

- [ ] **Step 5: Commit**

  ```bash
  git add cmd/mindthegap/push/bundle/bundle.go \
         cmd/mindthegap/push/bundle/detect_test.go
  git commit -m "feat(cmd/push): detect image archives in push bundle

  Before extracting bundle configs, inspect each --bundle path with
  archive.Detect and abort with a pointer to push image-archive when
  the file is actually an OCI or docker image archive. This replaces
  the confusing 'bundle config extraction failed' error users
  previously saw.

  Refs: NCN-113655"
  ```

---

## Task 13: E2E tests — Ginkgo suite and basic scenarios

**Files:**

- Create: `test/e2e/imagearchive/imagearchive_suite_test.go`
- Create: `test/e2e/imagearchive/push_image_archive_test.go`

- [ ] **Step 1: Suite file**

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0

  //go:build e2e

  package imagearchive_test

  import (
    "path/filepath"
    "testing"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"

    "github.com/mesosphere/mindthegap/test/e2e/helpers"
  )

  var artifacts helpers.Artifacts

  func TestImageArchive(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Image Archive Suite", Label("image", "imagearchive"))
  }

  var _ = BeforeSuite(func() {
    artifactsFileAbs, err := filepath.Abs(filepath.Join("..", "..", "..",
        "dist", "artifacts.json"))
    Expect(err).NotTo(HaveOccurred())
    relArtifacts, err := helpers.ParseArtifactsFile(artifactsFileAbs)
    Expect(err).NotTo(HaveOccurred())
    artifacts = make(helpers.Artifacts, 0, len(relArtifacts))
    for _, a := range relArtifacts {
        if a.Path != "" {
            a.Path = filepath.Join(filepath.Dir(artifactsFileAbs), "..", a.Path)
        }
        artifacts = append(artifacts, a)
    }
  })
  ```

- [ ] **Step 2: Push test**

  ```go
  // Copyright 2021 D2iQ, Inc. All rights reserved.
  // SPDX-License-Identifier: Apache-2.0

  //go:build e2e

  package imagearchive_test

  import (
    "context"
    "fmt"
    "log"
    "path/filepath"

    "github.com/go-logr/logr/funcr"
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    "github.com/phayes/freeport"
    "github.com/spf13/cobra"

    "github.com/mesosphere/dkp-cli-runtime/core/output"

    "github.com/mesosphere/mindthegap/cmd/mindthegap/push/imagearchive"
    "github.com/mesosphere/mindthegap/docker/registry"
    "github.com/mesosphere/mindthegap/images/archive/testutil"
    "github.com/mesosphere/mindthegap/test/e2e/helpers"
  )

  var _ = Describe("Push Image Archive", func() {
    var (
        cmd *cobra.Command
        tmp string
    )

    BeforeEach(func() {
        tmp = GinkgoT().TempDir()
        cmd = helpers.NewCommand(GinkgoT(),
            func(out output.Output) *cobra.Command {
                return imagearchive.NewCommand(out)
            })
    })

    It("pushes an OCI image layout tarball", func() {
        archivePath := filepath.Join(tmp, "oci.tar")
        testutil.BuildOCIArchive(GinkgoT(), archivePath, "example.com/app:v1")

        port, err := freeport.GetFreePort()
        Expect(err).NotTo(HaveOccurred())
        reg, err := registry.NewRegistry(registry.Config{
            Storage: registry.FilesystemStorage(filepath.Join(tmp, "registry")),
            Host:    "127.0.0.1",
            Port:    uint16(port),
        })
        Expect(err).NotTo(HaveOccurred())

        done := make(chan struct{})
        go func() {
            defer GinkgoRecover()
            Expect(reg.ListenAndServe(
                funcr.New(func(prefix, args string) { log.Println(prefix, args) }, funcr.Options{}),
            )).To(Succeed())
            close(done)
        }()
        helpers.WaitForTCPPort(GinkgoT(), "127.0.0.1", port)

        cmd.SetArgs([]string{
            "--image-archive", archivePath,
            "--to-registry", fmt.Sprintf("http://127.0.0.1:%d", port),
            "--to-registry-insecure-skip-tls-verify",
        })
        Expect(cmd.Execute()).To(Succeed())

        Expect(reg.Shutdown(context.Background())).To(Succeed())
        Eventually(done).Should(BeClosed())
    })

    It("pushes a docker-save tarball", func() {
        archivePath := filepath.Join(tmp, "docker.tar")
        testutil.BuildDockerArchive(GinkgoT(), archivePath, "example.com/app:v1")

        port, err := freeport.GetFreePort()
        Expect(err).NotTo(HaveOccurred())
        reg, err := registry.NewRegistry(registry.Config{
            Storage: registry.FilesystemStorage(filepath.Join(tmp, "registry")),
            Host:    "127.0.0.1",
            Port:    uint16(port),
        })
        Expect(err).NotTo(HaveOccurred())

        done := make(chan struct{})
        go func() {
            defer GinkgoRecover()
            Expect(reg.ListenAndServe(
                funcr.New(func(prefix, args string) { log.Println(prefix, args) }, funcr.Options{}),
            )).To(Succeed())
            close(done)
        }()
        helpers.WaitForTCPPort(GinkgoT(), "127.0.0.1", port)

        cmd.SetArgs([]string{
            "--image-archive", archivePath,
            "--to-registry", fmt.Sprintf("http://127.0.0.1:%d", port),
            "--to-registry-insecure-skip-tls-verify",
        })
        Expect(cmd.Execute()).To(Succeed())

        Expect(reg.Shutdown(context.Background())).To(Succeed())
        Eventually(done).Should(BeClosed())
    })

    It("rejects an image archive passed to push bundle", func() {
        archivePath := filepath.Join(tmp, "oci.tar")
        testutil.BuildOCIArchive(GinkgoT(), archivePath, "example.com/app:v1")

        // Use the push bundle command here.
        // Reuse helpers.NewCommand with pushbundle.NewCommand.
    })
  })
  ```

  Note: `testutil.BuildDockerArchive` etc. take a local `TB`
  interface (defined in Task 10), which both `*testing.T` and
  `ginkgo.GinkgoTInterface` satisfy structurally. No extra work
  needed here.

  Complete the rejection test scenario using `pushbundle.NewCommand`:

  ```go
  It("rejects an image archive passed to push bundle", func() {
    archivePath := filepath.Join(tmp, "oci.tar")
    testutil.BuildOCIArchive(GinkgoT(), archivePath, "example.com/app:v1")

    bundleCmd := helpers.NewCommand(GinkgoT(),
        func(out output.Output) *cobra.Command {
            return pushbundle.NewCommand(out, "bundle")
        })
    bundleCmd.SilenceErrors = true
    bundleCmd.SetArgs([]string{
        "--bundle", archivePath,
        "--to-registry", "registry.invalid:1",
    })

    err := bundleCmd.Execute()
    Expect(err).To(HaveOccurred())
    Expect(err.Error()).To(ContainSubstring("push image-archive"))
  })
  ```

  Add the import:

  ```go
  pushbundle "github.com/mesosphere/mindthegap/cmd/mindthegap/push/bundle"
  ```

- [ ] **Step 3: Run e2e tests**

  Run: `task test:e2e E2E_FOCUS="Push Image Archive"`
  Expected: all three It blocks PASS.

- [ ] **Step 4: Commit**

  ```bash
  git add test/e2e/imagearchive/
  git commit -m "test(e2e): add push image-archive end-to-end suite

  Cover OCI layout push, docker-save push, and the push bundle
  detection error against a live local registry.

  Refs: NCN-113655"
  ```

---

## Task 14: E2E — TLS variants

**Files:**

- Modify: `test/e2e/imagearchive/push_image_archive_test.go`

- [ ] **Step 1: Add DescribeTable with TLS variants**

  Append this DescribeTable after the existing It blocks. It
  parameterises over registry host (loopback vs outbound IP),
  scheme, and whether to skip TLS verification, mirroring the same
  structure used in `test/e2e/imagebundle/push_bundle_test.go`.

  ```go
  DescribeTable(
    "TLS variants",
    func(registryHost, registryScheme string, registryInsecure bool) {
        caCertFile := ""
        certFile := ""
        keyFile := ""
        if registryHost != "127.0.0.1" && registryScheme != "http" {
            certDir := GinkgoT().TempDir()
            caCertFile, _, certFile, keyFile = helpers.GenerateCertificateAndKeyWithIPSAN(
                GinkgoT(), certDir, net.ParseIP(registryHost),
            )
        }

        port, err := freeport.GetFreePort()
        Expect(err).NotTo(HaveOccurred())
        reg, err := registry.NewRegistry(registry.Config{
            Storage: registry.FilesystemStorage(filepath.Join(tmp, "registry")),
            Host:    registryHost,
            Port:    uint16(port),
            TLS: registry.TLS{
                Certificate: certFile,
                Key:         keyFile,
            },
        })
        Expect(err).NotTo(HaveOccurred())

        done := make(chan struct{})
        go func() {
            defer GinkgoRecover()
            Expect(reg.ListenAndServe(
                funcr.New(func(prefix, args string) { log.Println(prefix, args) }, funcr.Options{}),
            )).To(Succeed())
            close(done)
        }()
        helpers.WaitForTCPPort(GinkgoT(), registryHost, port)

        archivePath := filepath.Join(tmp, "tls.tar")
        testutil.BuildDockerArchive(GinkgoT(), archivePath, "example.com/app:v1")

        toURL := fmt.Sprintf("%s:%d", registryHost, port)
        if registryScheme != "" {
            toURL = fmt.Sprintf("%s://%s", registryScheme, toURL)
        }
        args := []string{
            "--image-archive", archivePath,
            "--to-registry", toURL,
        }
        if registryInsecure {
            args = append(args, "--to-registry-insecure-skip-tls-verify")
        } else if caCertFile != "" {
            args = append(args, "--to-registry-ca-cert-file", caCertFile)
        }
        cmd.SetArgs(args)
        Expect(cmd.Execute()).To(Succeed())

        Expect(reg.Shutdown(context.Background())).To(Succeed())
        Eventually(done).Should(BeClosed())
    },
    Entry("Without TLS (loopback)", "127.0.0.1", "", true),
    Entry("With TLS", helpers.GetPreferredOutboundIP(GinkgoT()).String(), "", false),
    Entry("With Insecure TLS", helpers.GetPreferredOutboundIP(GinkgoT()).String(), "", true),
    Entry("With http scheme", helpers.GetPreferredOutboundIP(GinkgoT()).String(), "http", true),
  )
  ```

  Add imports:

  ```go
  "net"
  ```

  (The other imports — `freeport`, `funcr`, `registry`, `log`, etc. —
  are already in scope from Task 13.)

- [ ] **Step 2: Run**

  Run: `task test:e2e E2E_FOCUS="Push Image Archive.*TLS"`
  Expected: all four entries PASS.

- [ ] **Step 3: Commit**

  ```bash
  git add test/e2e/imagearchive/push_image_archive_test.go
  git commit -m "test(e2e): parametrise push image-archive over TLS variants

  Refs: NCN-113655"
  ```

---

## Task 15: Auth — unit-level coverage via `httptest`

The existing e2e test registry does not support basic auth (neither
does the imagebundle e2e suite exercise auth). Rather than adding an
htpasswd-capable test registry, cover the auth flag plumbing at the
unit level using a plain `httptest.Server` wrapping the
`ggcrregistry.New()` handler.

**Files:**

- Modify: `cmd/mindthegap/push/imagearchive/push_test.go`

- [ ] **Step 1: Add auth unit test**

  Append:

  ```go
  import (
    "encoding/base64"
    "net/http"
  )

  func TestPushDockerArchive_BasicAuth(t *testing.T) {
    const user, pass = "u", "p"
    inner := registry.New()
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        gotUser, gotPass, ok := r.BasicAuth()
        if !ok || gotUser != user || gotPass != pass {
            w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
            w.WriteHeader(http.StatusUnauthorized)
            return
        }
        inner.ServeHTTP(w, r)
    })
    srv := httptest.NewServer(handler)
    defer srv.Close()
    regHost := srv.Listener.Addr().String()
    _ = base64.StdEncoding // keep import if needed elsewhere

    tmp := t.TempDir()
    archivePath := filepath.Join(tmp, "auth.tar")
    testutil.BuildDockerArchive(t, archivePath, "example.com/app:v1")

    buf := &bytes.Buffer{}
    out := output.NewNonInteractiveShell(buf, buf, 0)
    cmd := imagearchive.NewCommand(out)
    cmd.SilenceUsage = true
    cmd.SilenceErrors = true
    cmd.SetArgs([]string{
        "--image-archive", archivePath,
        "--to-registry", fmt.Sprintf("http://%s", regHost),
        "--to-registry-insecure-skip-tls-verify",
        "--to-registry-username", user,
        "--to-registry-password", pass,
    })
    if err := cmd.Execute(); err != nil {
        t.Fatalf("Execute: %v\noutput:\n%s", err, buf.String())
    }
  }

  func TestPushDockerArchive_BasicAuthWrongPassword(t *testing.T) {
    const user, pass = "u", "p"
    inner := registry.New()
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        gotUser, gotPass, ok := r.BasicAuth()
        if !ok || gotUser != user || gotPass != pass {
            w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
            w.WriteHeader(http.StatusUnauthorized)
            return
        }
        inner.ServeHTTP(w, r)
    })
    srv := httptest.NewServer(handler)
    defer srv.Close()
    regHost := srv.Listener.Addr().String()

    tmp := t.TempDir()
    archivePath := filepath.Join(tmp, "auth-fail.tar")
    testutil.BuildDockerArchive(t, archivePath, "example.com/app:v1")

    buf := &bytes.Buffer{}
    out := output.NewNonInteractiveShell(buf, buf, 0)
    cmd := imagearchive.NewCommand(out)
    cmd.SilenceUsage = true
    cmd.SilenceErrors = true
    cmd.SetArgs([]string{
        "--image-archive", archivePath,
        "--to-registry", fmt.Sprintf("http://%s", regHost),
        "--to-registry-insecure-skip-tls-verify",
        "--to-registry-username", user,
        "--to-registry-password", "wrong",
    })
    if err := cmd.Execute(); err == nil {
        t.Fatalf("expected authentication error, got nil")
    }
  }
  ```

- [ ] **Step 2: Run**

  Run: `go test ./cmd/mindthegap/push/imagearchive/... -run BasicAuth -v`
  Expected: both tests PASS.

- [ ] **Step 3: Commit**

  ```bash
  git add cmd/mindthegap/push/imagearchive/push_test.go
  git commit -m "test(cmd/push): cover basic auth for push image-archive

  Refs: NCN-113655"
  ```

---

## Task 16: E2E — multi-image archive and --image-tag override

**Files:**

- Modify: `test/e2e/imagearchive/push_image_archive_test.go`

- [ ] **Step 1: Multi-image scenario**

  Append:

  ```go
  It("pushes a docker-save archive with multiple tags", func() {
    archivePath := filepath.Join(tmp, "multi.tar")
    testutil.BuildDockerArchive(
        GinkgoT(), archivePath,
        "example.com/one:v1", "example.com/two:v2",
    )

    port, err := freeport.GetFreePort()
    Expect(err).NotTo(HaveOccurred())
    reg, err := registry.NewRegistry(registry.Config{
        Storage: registry.FilesystemStorage(filepath.Join(tmp, "registry")),
        Host:    "127.0.0.1",
        Port:    uint16(port),
    })
    Expect(err).NotTo(HaveOccurred())
    done := make(chan struct{})
    go func() {
        defer GinkgoRecover()
        Expect(reg.ListenAndServe(
            funcr.New(func(prefix, args string) { log.Println(prefix, args) }, funcr.Options{}),
        )).To(Succeed())
        close(done)
    }()
    helpers.WaitForTCPPort(GinkgoT(), "127.0.0.1", port)

    cmd.SetArgs([]string{
        "--image-archive", archivePath,
        "--to-registry", fmt.Sprintf("http://127.0.0.1:%d", port),
        "--to-registry-insecure-skip-tls-verify",
    })
    Expect(cmd.Execute()).To(Succeed())

    // Both refs should be pullable at the destination.
    for _, refStr := range []string{
        fmt.Sprintf("127.0.0.1:%d/one:v1", port),
        fmt.Sprintf("127.0.0.1:%d/two:v2", port),
    } {
        ref, err := name.ParseReference(refStr, name.StrictValidation)
        Expect(err).NotTo(HaveOccurred())
        _, err = remote.Get(ref)
        Expect(err).NotTo(HaveOccurred(),
            "expected %s to be present on destination", refStr)
    }

    Expect(reg.Shutdown(context.Background())).To(Succeed())
    Eventually(done).Should(BeClosed())
  })
  ```

  Add imports:

  ```go
  "github.com/google/go-containerregistry/pkg/name"
  "github.com/google/go-containerregistry/pkg/v1/remote"
  ```

- [ ] **Step 2: --image-tag override scenario**

  Append:

  ```go
  It("pushes a tagless OCI archive with --image-tag override", func() {
    archivePath := filepath.Join(tmp, "tagless.tar")
    testutil.BuildOCIArchive(GinkgoT(), archivePath, "") // no ref

    port, err := freeport.GetFreePort()
    Expect(err).NotTo(HaveOccurred())
    reg, err := registry.NewRegistry(registry.Config{
        Storage: registry.FilesystemStorage(filepath.Join(tmp, "registry")),
        Host:    "127.0.0.1",
        Port:    uint16(port),
    })
    Expect(err).NotTo(HaveOccurred())
    done := make(chan struct{})
    go func() {
        defer GinkgoRecover()
        Expect(reg.ListenAndServe(
            funcr.New(func(prefix, args string) { log.Println(prefix, args) }, funcr.Options{}),
        )).To(Succeed())
        close(done)
    }()
    helpers.WaitForTCPPort(GinkgoT(), "127.0.0.1", port)

    cmd.SetArgs([]string{
        "--image-archive", archivePath,
        "--to-registry", fmt.Sprintf("http://127.0.0.1:%d", port),
        "--to-registry-insecure-skip-tls-verify",
        "--image-tag", "override:v3",
    })
    Expect(cmd.Execute()).To(Succeed())

    ref, err := name.ParseReference(
        fmt.Sprintf("127.0.0.1:%d/override:v3", port),
        name.StrictValidation,
    )
    Expect(err).NotTo(HaveOccurred())
    _, err = remote.Get(ref)
    Expect(err).NotTo(HaveOccurred())

    Expect(reg.Shutdown(context.Background())).To(Succeed())
    Eventually(done).Should(BeClosed())
  })
  ```

- [ ] **Step 3: Run**

  Run: `task test:e2e E2E_FOCUS="Push Image Archive"`
  Expected: all scenarios PASS.

- [ ] **Step 4: Commit**

  ```bash
  git add test/e2e/imagearchive/push_image_archive_test.go
  git commit -m "test(e2e): cover multi-image and --image-tag override

  Refs: NCN-113655"
  ```

---

## Task 17: README documentation

**Files:**

- Modify: `README.md`

- [ ] **Step 1: Add section after "Pushing a bundle"**

  Insert this block after the end of the "Pushing a bundle" section
  (after the "Existing tag behaviour" subsection) and before "Serving
  a bundle":

  ````markdown
  ### Pushing an OCI/docker image archive

  ```shell
  mindthegap push image-archive \
    --image-archive <path/to/archive.tar> [--image-archive <path> ...] \
    --to-registry <registry.address> \
    [--image-tag <repo:tag>] \
    [--to-registry-insecure-skip-tls-verify]
  ```

  This pushes image archives produced by `docker save`, `podman save`,
  `skopeo copy docker://... oci-archive:out.tar`,
  `crane push --format=oci`, or `buildah push ... oci-archive:out.tar`
  directly to an OCI registry. Archive format (OCI image layout or
  docker-save) is auto-detected from file contents.

  Destination references are taken from the archive's embedded
  metadata — the first entry of `RepoTags` for docker-save archives, or
  the `org.opencontainers.image.ref.name` annotation for OCI layout
  archives. Any registry host in the embedded reference is dropped;
  images are always pushed under `--to-registry`.

  If an archive contains a single image with no embedded tag, or if
  you want to push it under a different reference, supply
  `--image-tag <repo:tag>`. This flag is only valid when exactly one
  archive containing exactly one image is provided.
  ````

- [ ] **Step 2: Verify markdown lints cleanly**

  Run: `pre-commit run markdownlint --files README.md`
  Expected: Passed.

- [ ] **Step 3: Commit**

  ```bash
  git add README.md
  git commit -m "docs(readme): document mindthegap push image-archive

  Refs: NCN-113655"
  ```

---

## Task 18: Final verification sweep

- [ ] **Step 1: Run the full test suite**

  Run: `task test:unit`
  Expected: PASS across all modules.

- [ ] **Step 2: Run the lint suite**

  Run: `pre-commit run --all-files`
  Expected: all hooks pass.

- [ ] **Step 3: Run e2e**

  Run: `task test:e2e E2E_LABEL=imagearchive`
  Expected: all imagearchive scenarios PASS.

- [ ] **Step 4: Manual smoke test (optional but recommended)**

  Build the binary and exercise both real-world paths:

  ```bash
  task build:snapshot
  BIN=./dist/mindthegap_$(go env GOOS)_$(go env GOARCH)/mindthegap
  # Build a docker archive with crane.
  crane pull nginx:1.21.5 /tmp/nginx.tar
  # Run a local registry.
  docker run -d --rm -p 5005:5000 --name mtg-test-reg registry:2
  "$BIN" push image-archive \
    --image-archive /tmp/nginx.tar \
    --to-registry http://127.0.0.1:5005 \
    --to-registry-insecure-skip-tls-verify
  crane ls 127.0.0.1:5005
  docker rm -f mtg-test-reg
  ```

  Expected: `crane ls` shows the image was pushed at its embedded tag.

- [ ] **Step 5: Push the branch and open a PR**

  The git-workflow rule for this repo uses standard GitHub PRs. Do
  NOT push to `refs/for/...`.

  ```bash
  git push -u origin NCN-113655/push-image-archive
  gh pr create --title "feat: push OCI/docker image archive tarballs (NCN-113655)" \
    --body "$(cat <<'EOF'
  ## Summary

  - Adds `mindthegap push image-archive` subcommand for pushing OCI
    image layout tarballs and docker-save tarballs directly to an OCI
    registry, analogous to `crane push`.
  - Extends `mindthegap push bundle` with content-type detection that
    emits a helpful error when pointed at an image archive rather than
    a mindthegap bundle.

  ## Test plan

  - [x] Unit tests covering archive detection, docker reader, OCI
    reader, and reference resolution.
  - [x] End-to-end tests covering OCI-layout push, docker-save push,
    multi-image push, `--image-tag` override, TLS variants, basic
    auth, and `push bundle` rejection.
  - [x] README documents the new subcommand with an example.

  Refs: https://jira.nutanix.com/browse/NCN-113655
  EOF
  )"
  ```

  Expected: PR is created and linked.

---

## Appendix: Spec coverage checklist

- FR-001 → Task 8 (skeleton), Task 9 (validation), Task 10 (run).
- FR-002 → Task 2 (Detect) + Task 3 (Open dispatch).
- FR-003 → Task 10 (remote.Write / remote.WriteIndex).
- FR-004 → Task 10 (resolveDestRef using reference.ParseNormalizedNamed).
- FR-005, FR-006 → Task 9 (validateImageTagOverride) + Task 10
  (override applied in resolveDestRef).
- FR-007 → Task 11 (tagless error).
- FR-008 → Task 12 (rejectImageArchives in push bundle).
- FR-009 → Task 4 (tarball.Image opener) + Task 6 (archives.FileSystem
  OCI reader).
- FR-010 → Task 10 (wrapped errors include destRef).
- FR-011 → Task 10 (out.StartOperation / EndOperation pattern).
- FR-012 → Task 7 Step 2 adds explicit empty-archive tests for both
  OCI and docker readers; Task 10's push loop naturally handles empty
  archives (no iterations = success).
- SC-001..SC-006 → Tasks 13–16 e2e + Task 18 verification.
