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
