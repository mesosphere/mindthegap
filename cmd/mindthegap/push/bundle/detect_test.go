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
