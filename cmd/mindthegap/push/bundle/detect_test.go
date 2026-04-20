// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package bundle_test

import (
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
