// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package bundle_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cmd/mindthegap/push/bundle"
	"github.com/mesosphere/mindthegap/images/archive/testutil"
)

// writeBundleLikeTar writes a tar archive at path containing the
// supplied entries. Each entry is the tar member name; an entry name
// ending in "/" is recorded as a directory header (no body).
func writeBundleLikeTar(tb testing.TB, path string, entries ...string) {
	tb.Helper()
	f, err := os.Create(path)
	if err != nil {
		tb.Fatalf("create %s: %v", path, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			tb.Fatalf("close %s: %v", path, cerr)
		}
	}()
	tw := tar.NewWriter(f)
	for _, name := range entries {
		isDir := strings.HasSuffix(name, "/")
		hdr := &tar.Header{Name: name, Mode: 0o644}
		if isDir {
			hdr.Typeflag = tar.TypeDir
			hdr.Mode = 0o755
		} else {
			hdr.Typeflag = tar.TypeReg
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			tb.Fatalf("write header %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		tb.Fatalf("close tar: %v", err)
	}
}

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

// TestPushBundleDoesNotInterceptUnclassifiableFile exercises the
// regression that rejectImageArchives must fall through when the
// classifier cannot parse a file as a tar, so that existing
// downstream error handling (e.g. the "compressed tar archives
// (.tar.gz) are not supported" message surfaced by ArchiveStorage
// for gzipped bundles) is not shadowed by the detection hook.
func TestPushBundleDoesNotInterceptUnclassifiableFile(t *testing.T) {
	tmp := t.TempDir()
	nonBundle := filepath.Join(tmp, "image-bundle.tar.gz")
	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	if _, err := gw.Write([]byte("not-a-bundle")); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	if err := os.WriteFile(nonBundle, gzBuf.Bytes(), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	buf := &bytes.Buffer{}
	out := output.NewNonInteractiveShell(buf, buf, 0)
	cmd := bundle.NewCommand(out, "bundle")
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{
		"--bundle", nonBundle,
		"--to-registry", "registry.invalid:1",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if strings.Contains(err.Error(), "push image-archive") {
		t.Fatalf("detection hook should not intercept unclassifiable files: %v", err)
	}
}

// TestPushBundleAcceptsBundleWithImageArchiveMarkers verifies that
// a tar carrying both the mindthegap bundle markers (registry
// storage tree under docker/registry/v2/ AND at least one of
// images.yaml or charts.yaml at the tar root) is not redirected to
// "push image-archive" even if it also happens to carry an
// oci-layout or manifest.json file at the tar root.
//
// Bundle identity is a positive property; image-archive markers
// alone must not override it. Regression for NCN-114493.
func TestPushBundleAcceptsBundleWithImageArchiveMarkers(t *testing.T) {
	cases := []struct {
		name    string
		entries []string
	}{
		{
			name: "images.yaml plus storage tree plus oci-layout at root",
			entries: []string{
				"oci-layout",
				"images.yaml",
				"docker/registry/v2/repositories/foo/_manifests/tags/v1/current/link",
			},
		},
		{
			name: "charts.yaml plus storage tree plus manifest.json at root",
			entries: []string{
				"manifest.json",
				"charts.yaml",
				"docker/registry/v2/repositories/foo/_manifests/tags/v1/current/link",
			},
		},
		{
			name: "both bundle configs plus storage tree plus both image-archive markers",
			entries: []string{
				"oci-layout",
				"manifest.json",
				"images.yaml",
				"charts.yaml",
				"docker/registry/v2/repositories/foo/_manifests/tags/v1/current/link",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			bundlePath := filepath.Join(tmp, "image-bundle.tar")
			writeBundleLikeTar(t, bundlePath, tc.entries...)

			buf := &bytes.Buffer{}
			out := output.NewNonInteractiveShell(buf, buf, 0)
			cmd := bundle.NewCommand(out, "bundle")
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			cmd.SetArgs([]string{
				"--bundle", bundlePath,
				"--to-registry", "registry.invalid:1",
			})

			err := cmd.Execute()
			if err == nil {
				return
			}
			if strings.Contains(err.Error(), "push image-archive") {
				t.Fatalf("detection hook misclassified bundle as image archive: %v", err)
			}
		})
	}
}

// TestPushBundleRedirectsWhenBundleMarkersIncomplete verifies that
// a tar carrying image-archive markers at the tar root but missing
// either the registry storage tree or both of images.yaml /
// charts.yaml is still redirected to "push image-archive". This
// guards against accidentally widening bundle acceptance: both
// bundle markers must be present together.
func TestPushBundleRedirectsWhenBundleMarkersIncomplete(t *testing.T) {
	cases := []struct {
		name    string
		entries []string
	}{
		{
			name: "oci-layout plus images.yaml but no storage tree",
			entries: []string{
				"oci-layout",
				"images.yaml",
			},
		},
		{
			name: "oci-layout plus storage tree but no config file",
			entries: []string{
				"oci-layout",
				"docker/registry/v2/repositories/foo/_manifests/tags/v1/current/link",
			},
		},
		{
			name: "manifest.json plus charts.yaml but no storage tree",
			entries: []string{
				"manifest.json",
				"charts.yaml",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			archivePath := filepath.Join(tmp, "archive.tar")
			writeBundleLikeTar(t, archivePath, tc.entries...)

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
				t.Fatalf("expected detection hook to redirect to push image-archive, got: %v", err)
			}
		})
	}
}
