// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package archive_test

import (
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/mesosphere/mindthegap/images/archive"
)

func TestDockerArchiveEntries(t *testing.T) {
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
