// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imagearchive_test

import (
	"bytes"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
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

func TestPush_TaglessWithoutOverride(t *testing.T) {
	reg := registry.New()
	srv := httptest.NewServer(reg)
	defer srv.Close()
	regHost := srv.Listener.Addr().String()

	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "tagless.tar")
	testutil.BuildOCIArchive(t, archivePath, "")

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
