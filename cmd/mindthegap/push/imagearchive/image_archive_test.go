// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imagearchive_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cmd/mindthegap/push/imagearchive"
)

func writeDockerTarFile(t *testing.T, path string, tags ...string) {
	t.Helper()
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
