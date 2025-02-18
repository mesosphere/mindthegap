// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mholt/archives"
)

func ArchiveDirectory(dir, outputFile string) error {
	fi, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}
	filesToArchive := make(map[string]string, len(fi))
	for _, f := range fi {
		filesToArchive[filepath.Join(dir, f.Name())] = ""
	}
	files, err := archives.FilesFromDisk(context.Background(), nil, filesToArchive)
	if err != nil {
		return err
	}

	tempTarArchive := filepath.Join(filepath.Dir(outputFile), "."+filepath.Base(outputFile))
	// create the output file we'll write to
	tempOutputFile, err := os.Create(tempTarArchive)
	if err != nil {
		return err
	}
	defer os.Remove(tempTarArchive)
	defer tempOutputFile.Close()

	format, _, err := archives.Identify(context.Background(), filepath.Base(outputFile), nil)
	if err != nil {
		return fmt.Errorf("failed to identify output file format: %w", err)
	}
	archiver, ok := format.(archives.Archiver)
	if !ok {
		return fmt.Errorf("output file format is not an archiver")
	}

	err = archiver.Archive(context.Background(), tempOutputFile, files)
	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}
	if err := tempOutputFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary archive file: %w", err)
	}

	if err := os.Rename(tempTarArchive, outputFile); err != nil {
		return fmt.Errorf("failed to rename temporary archive to output file: %w", err)
	}
	return nil
}
