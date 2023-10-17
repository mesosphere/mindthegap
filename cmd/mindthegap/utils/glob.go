// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"path/filepath"
)

// FilesWithGlobs expects a list of files and/or globs, and returns a new list of files.
// Returns an error if in does not match any files on the disk.
func FilesWithGlobs(in []string) ([]string, error) {
	var out []string
	for _, file := range in {
		matches, err := filepath.Glob(file)
		if err != nil {
			return nil, fmt.Errorf("error finding matching files for %q: %w", file, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("did find any matching files for %q", file)
		}
		out = append(out, matches...)
	}

	return out, nil
}
