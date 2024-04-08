// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesWithGlobs(t *testing.T) {
	t.Parallel()

	// filepath.Glob expects the files to exist
	testFiles, createdFiles := tempFiles(t)
	testGlobs, createdGlobFiles := tempGlobs(t)

	//nolint:gocritic // want to a append to a new slice, its fine in tests
	combinedTestFiles := append(testFiles, testGlobs...)
	//nolint:gocritic // want to a append to a new slice, its fine in tests
	combinedCreatedFiles := append(createdFiles, createdGlobFiles...)

	tests := []struct {
		name           string
		in             []string
		expectedOutput []string
		wantErr        error
	}{
		{
			name:    "error: file does not exist",
			in:      []string{"doesnotexist.tar"},
			wantErr: fmt.Errorf("did find any matching files for \"doesnotexist.tar\""),
		},
		{
			name:           "all files",
			in:             testFiles,
			expectedOutput: createdFiles,
		},
		{
			name:           "all globs",
			in:             testGlobs,
			expectedOutput: createdGlobFiles,
		},
		{
			name:           "files and globs",
			in:             combinedTestFiles,
			expectedOutput: combinedCreatedFiles,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, err := FilesWithGlobs(tt.in)
			require.Equal(t, tt.wantErr, err)
			require.Equal(t, tt.expectedOutput, out)
		})
	}
}

//nolint:gocritic // no need for named parameters
func tempGlobs(t *testing.T) ([]string, []string) {
	t.Helper()
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()
	globs := []string{
		fmt.Sprintf("%s/*.tar", tempDir1),
		fmt.Sprintf("%s/*.tar", tempDir2),
	}

	filesToCreate := []string{
		filepath.Join(tempDir1, "images1.tar"),
		filepath.Join(tempDir1, "images2.tar"),
		filepath.Join(tempDir2, "images1.tar"),
	}
	for _, createFile := range filesToCreate {
		_, err := os.Create(createFile)
		require.NoError(t, err)
	}

	return globs, filesToCreate
}

//nolint:gocritic // no need for named parameters
func tempFiles(t *testing.T) ([]string, []string) {
	t.Helper()
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()
	filesToCreate := []string{
		filepath.Join(tempDir1, "images1.tar"),
		filepath.Join(tempDir1, "images2.tar"),
		filepath.Join(tempDir2, "images1.tar"),
	}
	for _, createFile := range filesToCreate {
		_, err := os.Create(createFile)
		require.NoError(t, err)
	}

	return filesToCreate, filesToCreate
}
