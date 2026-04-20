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
