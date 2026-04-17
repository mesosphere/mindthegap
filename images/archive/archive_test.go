// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package archive_test

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mesosphere/mindthegap/images/archive"
)

type tarFile struct {
	Name     string
	Contents []byte
}

// writeTarFile creates a tar archive at path containing the given
// name -> contents mapping. Files are written in the order given.
func writeTarFile(t *testing.T, path string, files []tarFile) {
	t.Helper()

	buf := &bytes.Buffer{}
	tw := tar.NewWriter(buf)
	for _, f := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name:    f.Name,
			Mode:    0o644,
			Size:    int64(len(f.Contents)),
			ModTime: time.Unix(0, 0),
		}); err != nil {
			t.Fatalf("write header %q: %v", f.Name, err)
		}
		if _, err := tw.Write(f.Contents); err != nil {
			t.Fatalf("write body %q: %v", f.Name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write tar %s: %v", path, err)
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name    string
		files   []tarFile
		want    archive.Format
		wantErr bool
	}{
		{
			name: "OCI layout",
			files: []tarFile{
				{Name: "oci-layout", Contents: []byte(`{"imageLayoutVersion":"1.0.0"}`)},
				{Name: "index.json", Contents: []byte(`{"schemaVersion":2}`)},
			},
			want: archive.FormatOCILayout,
		},
		{
			name: "docker-save",
			files: []tarFile{
				{Name: "manifest.json", Contents: []byte(`[]`)},
			},
			want: archive.FormatDockerArchive,
		},
		{
			name: "mindthegap-bundle-like",
			files: []tarFile{
				{Name: "images.yaml", Contents: []byte(`{}`)},
				{
					Name:     "docker/registry/v2/repositories/nginx/_manifests/tags/latest/current/link",
					Contents: []byte(`sha256:deadbeef`),
				},
			},
			want: archive.FormatUnknown,
		},
		{
			name: "unknown",
			files: []tarFile{
				{Name: "random.txt", Contents: []byte(`hi`)},
			},
			want: archive.FormatUnknown,
		},
		{
			name:  "empty",
			files: nil,
			want:  archive.FormatUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input.tar")
			writeTarFile(t, path, tc.files)
			got, err := archive.Detect(path)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got format=%v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
