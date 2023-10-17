// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"io"
	"os"
)

func CopyFile(src, dst string) error {
	sStat, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	s, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer s.Close()

	d, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, sStat.Mode().Perm())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer d.Close()

	// Copy the contents of the source file into the destination file
	_, err = io.Copy(d, s)
	if err != nil {
		return fmt.Errorf("failed to copy file contents file: %w", err)
	}

	// Return any errors that result from closing the destination file
	// Will return nil if no errors occurred
	if err := d.Close(); err != nil {
		return fmt.Errorf("failed to close destination file: %w", err)
	}
	return nil
}
