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

		name := path.Clean(hdr.Name)
		if path.Dir(name) != "." {
			continue
		}
		switch name {
		case "oci-layout":
			return FormatOCILayout, nil
		case "manifest.json":
			return FormatDockerArchive, nil
		}
	}
}
