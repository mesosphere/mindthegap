// Copyright 2021-2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mholt/archiver/v3"
)

func ArchiveDirectory(dir, outputFile string) error {
	fi, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}
	filesToArchive := make([]string, 0, len(fi))
	for _, f := range fi {
		filesToArchive = append(filesToArchive, filepath.Join(dir, f.Name()))
	}
	tempTarArchive := filepath.Join(filepath.Dir(outputFile), "."+filepath.Base(outputFile))
	defer os.Remove(tempTarArchive)
	if err = archiver.Archive(filesToArchive, tempTarArchive); err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}
	if err := os.Rename(tempTarArchive, outputFile); err != nil {
		return fmt.Errorf("failed to rename temporary archive to output file: %w", err)
	}
	return nil
}
