// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package archive

import "errors"

type ociArchive struct {
	path string
}

func openOCI(archivePath string) (Archive, error) {
	return &ociArchive{path: archivePath}, nil
}

func (o *ociArchive) Format() Format { return FormatOCILayout }

func (o *ociArchive) Entries() ([]Entry, error) {
	return nil, errors.New("not implemented")
}

func (o *ociArchive) Close() error { return nil }
