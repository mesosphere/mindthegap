// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"fmt"

	"github.com/mholt/archiver/v3"
)

func ExtractFileToDirectory(archive, destDir, fileName string) error {
	archiverByExtension, err := archiver.ByExtension(archive)
	if err != nil {
		return fmt.Errorf("failed to identify archive format: %w", err)
	}

	unarc, ok := archiverByExtension.(archiver.Extractor)
	if !ok {
		return fmt.Errorf("not an valid archive extension")
	}

	switch t := unarc.(type) {
	case *archiver.TarGz:
		t.OverwriteExisting = true
	case *archiver.Tar:
		t.OverwriteExisting = true
	}

	if err := unarc.Extract(archive, fileName, destDir); err != nil {
		return fmt.Errorf("failed to extract %s: %w", fileName, err)
	}

	return nil
}
