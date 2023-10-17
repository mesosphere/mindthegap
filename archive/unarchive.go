// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"fmt"

	"github.com/mholt/archiver/v3"
)

func UnarchiveToDirectory(archive, destDir string) error {
	archiverByExtension, err := archiver.ByExtension(archive)
	if err != nil {
		return fmt.Errorf("failed to identify archive format: %w", err)
	}

	unarc, ok := archiverByExtension.(archiver.Unarchiver)
	if !ok {
		return fmt.Errorf("not an valid archive extension")
	}

	switch t := unarc.(type) {
	case *archiver.TarGz:
		t.OverwriteExisting = true
	case *archiver.Tar:
		t.OverwriteExisting = true
	}

	if err := unarc.Unarchive(archive, destDir); err != nil {
		return fmt.Errorf("failed to unarchive bundle: %w", err)
	}

	return nil
}
