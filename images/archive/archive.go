// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package archive provides readers for image archives: OCI image
// layout tarballs (per the OCI image-spec) and docker-save tarballs
// (the output of `docker save`/`podman save`).
package archive

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// Format identifies the type of an image archive.
type Format int

const (
	// FormatUnknown indicates the file is not a recognised image archive.
	FormatUnknown Format = iota
	// FormatOCILayout is an OCI image layout tarball (contains an
	// "oci-layout" file at the tar root).
	FormatOCILayout
	// FormatDockerArchive is a docker-save tarball (contains a
	// "manifest.json" file at the tar root).
	FormatDockerArchive
)

// String returns a human-readable name for the format.
func (f Format) String() string {
	switch f {
	case FormatOCILayout:
		return "OCI image layout tarball"
	case FormatDockerArchive:
		return "docker-save tarball"
	case FormatUnknown:
		return "unknown"
	default:
		return fmt.Sprintf("Format(%d)", int(f))
	}
}

// Entry represents a single image or image index contained in an
// archive, with an optional embedded reference.
//
// Exactly one of Image or Index is non-nil.
type Entry struct {
	// Ref is the embedded reference from the archive, or nil if the
	// archive did not carry one. Docker archives use the first entry
	// of RepoTags; OCI archives use the
	// org.opencontainers.image.ref.name annotation on the top-level
	// descriptor.
	Ref name.Reference
	// Image is non-nil for single-manifest entries.
	Image v1.Image
	// Index is non-nil for image-index entries (multi-platform).
	Index v1.ImageIndex
}

// Archive iterates image entries in an archive.
type Archive interface {
	// Format returns the classification of the archive.
	Format() Format
	// Entries returns all image entries in the archive. The slice
	// may be empty for an archive that contains no images.
	Entries() ([]Entry, error)
	// Close releases any resources held by the archive.
	Close() error
}

// Open detects the archive format and returns an Archive for reading
// its entries. Returns an error with a friendly message if the file
// is not a recognised image archive.
func Open(archivePath string) (Archive, error) {
	format, err := Detect(archivePath)
	if err != nil {
		return nil, err
	}
	switch format {
	case FormatOCILayout:
		return openOCI(archivePath)
	case FormatDockerArchive:
		return openDocker(archivePath)
	case FormatUnknown:
		return nil, fmt.Errorf(
			"file %s is not a recognised image archive "+
				"(expected OCI image layout tarball or docker-save tarball)",
			archivePath,
		)
	default:
		return nil, fmt.Errorf("unhandled archive format: %v", format)
	}
}

// Detect classifies the tar archive at the given path. Detection is a
// single streaming scan of the tar headers that stops as soon as an
// OCI layout marker ("oci-layout") or docker-save marker
// ("manifest.json") is seen at the tar root, or when the entire
// archive has been walked. Files inside subdirectories are ignored
// because both markers must exist at depth 0 per their respective
// specs.
func Detect(archivePath string) (Format, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return FormatUnknown, fmt.Errorf("opening archive %s: %w", archivePath, err)
	}
	defer f.Close()

	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		switch {
		case errors.Is(err, io.EOF):
			return FormatUnknown, nil
		case err != nil:
			return FormatUnknown, fmt.Errorf("reading tar %s: %w", archivePath, err)
		}

		entryName := path.Clean(hdr.Name)
		if path.Dir(entryName) != "." {
			continue
		}
		switch entryName {
		case "oci-layout":
			return FormatOCILayout, nil
		case "manifest.json":
			return FormatDockerArchive, nil
		}
	}
}
