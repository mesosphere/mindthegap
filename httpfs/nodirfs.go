// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpfs

import (
	"net/http"
	"os"
)

func DisableDirListingFS(dir string) http.FileSystem {
	return noListFileSystem{base: http.Dir(dir)}
}

type noListFile struct {
	http.File
}

func (f noListFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, nil
}

type noListFileSystem struct {
	base http.FileSystem
}

func (fs noListFileSystem) Open(name string) (http.File, error) {
	f, err := fs.base.Open(name)
	if err != nil {
		return nil, err
	}

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, os.ErrNotExist
	}

	return noListFile{f}, nil
}
