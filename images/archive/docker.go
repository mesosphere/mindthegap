// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package archive

import "errors"

type dockerArchive struct {
	path string
}

func openDocker(archivePath string) (Archive, error) {
	return &dockerArchive{path: archivePath}, nil
}

func (d *dockerArchive) Format() Format { return FormatDockerArchive }

func (d *dockerArchive) Entries() ([]Entry, error) {
	return nil, errors.New("not implemented")
}

func (d *dockerArchive) Close() error { return nil }
