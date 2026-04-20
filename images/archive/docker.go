// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"archive/tar"
	"encoding/json"
	"errors"
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

	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
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
