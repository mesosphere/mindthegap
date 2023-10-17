// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package archive_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mesosphere/mindthegap/archive"
)

func TestUnarchiveToDirectorySuccess(t *testing.T) {
	t.Parallel()
	testDataDir := "testdata"

	tmpDir := t.TempDir()

	testDataContents, err := walkDirContentsToMap(testDataDir)
	require.NoError(t, err, "error walking test data directory")

	tarA := filepath.Join(tmpDir, "a.tar")
	require.NoError(t, archive.ArchiveDirectory(testDataDir, tarA),
		"error archiving directory")
	require.FileExists(t, tarA, "archive file should exist")

	untarTmpDir := t.TempDir()

	require.NoError(t, archive.UnarchiveToDirectory(tarA, untarTmpDir))

	unarchivedContents, err := walkDirContentsToMap(untarTmpDir)
	require.NoError(t, err, "error walking unarchived data directory")

	require.Equal(t, testDataContents, unarchivedContents, "incorrect unarchived contents")
}

func TestUnarchiveToDirectoryWithDuplicateContentsSuccess(t *testing.T) {
	t.Parallel()
	testDataDir := filepath.Join("testdata", "unarchivetest")
	testDataDirA := filepath.Join(testDataDir, "dira")
	testDataDirB := filepath.Join(testDataDir, "dirc")

	tmpDir := t.TempDir()

	testDataContentsDirA, err := walkDirContentsToMap(testDataDirA)
	require.NoError(t, err, "error walking test data directory")
	testDataContentsDirB, err := walkDirContentsToMap(testDataDirB)
	require.NoError(t, err, "error walking test data directory")

	testDataContents := testDataContentsDirA
	for k, v := range testDataContentsDirB {
		testDataContents[k] = v
	}

	tarA := filepath.Join(tmpDir, "a.tar")
	require.NoError(t, archive.ArchiveDirectory(testDataDirA, tarA),
		"error archiving directory")
	require.FileExists(t, tarA, "archive file should exist")

	targzB := filepath.Join(tmpDir, "b.tar.gz")
	require.NoError(t, archive.ArchiveDirectory(testDataDirB, targzB),
		"error archiving directory")
	require.FileExists(t, targzB, "archive file should exist")

	untarTmpDir := t.TempDir()

	require.NoError(t, archive.UnarchiveToDirectory(tarA, untarTmpDir))
	require.NoError(t, archive.UnarchiveToDirectory(targzB, untarTmpDir))

	unarchivedContents, err := walkDirContentsToMap(untarTmpDir)
	require.NoError(t, err, "error walking unarchived data directory")

	require.Equal(t, testDataContents, unarchivedContents, "incorrect unarchived contents")
}

func walkDirContentsToMap(dir string) (map[string]string, error) {
	testDataContents := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, _ error) error {
		if d.IsDir() {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading test data file %q: %w", path, err)
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("error getting relative path for test data %q: %w", path, err)
		}
		testDataContents[rel] = string(contents)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return testDataContents, nil
}

func TestUnarchiveDirectoryDestDirNotWritable(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	testDataDir := filepath.Join("testdata", "unarchivetest")

	tarA := filepath.Join(tmpDir, "a.tar")
	require.NoError(t, archive.ArchiveDirectory(testDataDir, tarA),
		"error archiving directory")
	require.FileExists(t, tarA, "archive file should exist")

	notWriteable := filepath.Join(tmpDir, "notwritable")
	require.NoError(t, os.Mkdir(notWriteable, 0o500), "error creating not writable directory")
	require.Error(
		t,
		archive.UnarchiveToDirectory(tarA, notWriteable),
		"expected error unarchiving bundle",
	)
}

func TestUnarchiveDirectoryUnreadableSource(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	testDataDir := filepath.Join("testdata", "unarchivetest")

	tarA := filepath.Join(tmpDir, "a.tar")
	require.NoError(t, archive.ArchiveDirectory(testDataDir, tarA),
		"error archiving directory")
	require.FileExists(t, tarA, "archive file should exist")

	require.NoError(t, os.Chmod(tarA, 0o100), "error changing read permissions")

	require.Error(
		t,
		archive.UnarchiveToDirectory(tarA, tmpDir),
		"expected error unarchiving bundle",
	)
}
