// Copyright 2021 D2iQ, Inc. All rights reserved.
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

func UnarchiveToDirectory(archive, destDir string) error {
	tarArchive, err := os.Open(archive)
	if err != nil {
		return fmt.Errorf("failed to open archive for extraction: %w", err)
	}
	defer tarArchive.Close()

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
