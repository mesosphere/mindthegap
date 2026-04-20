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
