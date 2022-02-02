// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package archive_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mesosphere/mindthegap/archive"
)

func TestArchiveDirectorySuccess(t *testing.T) {
	testDataDir := "testdata"
	testDataContents := map[string]string{}
	err := filepath.WalkDir(testDataDir, func(path string, d fs.DirEntry, _ error) error {
		if d.IsDir() {
			return nil
		}
		contents, err := os.ReadFile(path)
		require.NoErrorf(t, err, "error reading test data file %s", path)
		rel, err := filepath.Rel(testDataDir, path)
		require.NoError(t, err, "error getting relative path for test data")
		testDataContents[rel] = string(contents)
		return nil
	})
	require.NoError(t, err, "error walking test data directory")

	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "out.tar.gz")
	require.NoError(t, archive.ArchiveDirectory(testDataDir, outputFile),
		"error archiving directory")
	require.FileExists(t, outputFile, "archive file should exist")
	f, err := os.Open(outputFile)
	require.NoError(t, err, "error opening tarball for reading")
	defer f.Close()

	archivedContents := make(map[string]string, len(testDataContents))

	gzr, err := gzip.NewReader(f)
	require.NoError(t, err, "error creating gzip reader for tarball")
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		require.NoError(t, err, "error reading listing from tarball")
		if hdr.FileInfo().IsDir() {
			continue
		}
		var buf bytes.Buffer
		_, err = io.CopyN(&buf, tr, 1024)
		require.Condition(
			t,
			func() (success bool) { return err == nil || err == io.EOF },
			"error reading content from tarball",
		)
		archivedContents[hdr.Name] = buf.String()
	}

	require.Equal(t, testDataContents, archivedContents, "incorrect tarball contents")
}

func TestArchiveDirectoryDestDirNotWritable(t *testing.T) {
	tmpDir := t.TempDir()
	notWriteable := filepath.Join(tmpDir, "notwritable")
	require.NoError(t, os.Mkdir(notWriteable, 0o500), "error creating not writable directory")
	outputFile := filepath.Join(notWriteable, "out.tar.gz")
	require.Error(
		t,
		archive.ArchiveDirectory("testdata", outputFile),
		"expected error archiving directory",
	)
}

func TestArchiveDirectoryDestFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "out.tar.gz")
	f, err := os.OpenFile(outputFile, os.O_CREATE, 0o400)
	require.NoError(t, err, "error creating dummy file")
	require.NoError(t, f.Close(), "error closing dummy file")
	require.NoError(
		t,
		archive.ArchiveDirectory("testdata", outputFile),
		"unexpected error archiving directory",
	)
}

func TestArchiveDirectoryUnreadableSource(t *testing.T) {
	tmpDir := t.TempDir()
	unreadable := filepath.Join(tmpDir, "unreadable")
	require.NoError(t, os.Mkdir(unreadable, 0o100), "error creating unreadable directory")
	outputFile := filepath.Join(tmpDir, "out.tar.gz")
	require.Error(
		t,
		archive.ArchiveDirectory(unreadable, outputFile),
		"expected error archiving directory",
	)
}
