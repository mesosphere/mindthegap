// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package archive provides readers for image archives: OCI image
// layout tarballs (per the OCI image-spec) and docker-save tarballs
// (the output of `docker save`/`podman save`).
package archive

import (
	"fmt"
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
