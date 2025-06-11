// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mholt/archives"
)

func ExtractFileToDirectory(archive, destDir, fileName string) error {
	archiveFile, err := os.Open(archive)
	if err != nil {
		return fmt.Errorf("failed to open archive %s: %w", archive, err)
	}
	defer archiveFile.Close()

	archiver, archiveStream, err := archives.Identify(context.Background(), fileName, archiveFile)
	if err != nil {
		return fmt.Errorf("failed to identify archive format: %w", err)
	}

	// Disallow tar.gz and tar.bz2 archives as noted in the docs for github.com/mholt/archives that
	// traversing compressed tar archives is extremely slow and inefficient. Benchmarking confirms
	// that this is indeed the case, so we don't support them.
	ext := archiver.Extension()
	if ext == ".tar.gz" || ext == ".tar.bz2" {
		return fmt.Errorf("compressed tar archives (%s) are not supported", ext)
	}

	unarc, ok := archiver.(archives.Extractor)
	if !ok {
		return fmt.Errorf("not an valid archive extension")
	}

	err = unarc.Extract(
		context.Background(),
		archiveStream,
		func(ctx context.Context, f archives.FileInfo) error {
			if f.NameInArchive != fileName {
				return nil
			}

			fi, err := f.Open()
			if err != nil {
				return fmt.Errorf("failed to open file %s: %w", f.NameInArchive, err)
			}
			defer fi.Close()

			destFilePath := filepath.Join(destDir, fileName)
			destFile, err := os.Create(destFilePath)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", destFilePath, err)
			}
			defer destFile.Close()

			_, err = io.Copy(destFile, fi)
			if err != nil {
				return fmt.Errorf("failed to copy %s: %w", fileName, err)
			}

			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to extract %s: %w", fileName, err)
	}

	return nil
}
