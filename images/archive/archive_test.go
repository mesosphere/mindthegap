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

func TestOpenDispatch(t *testing.T) {
	ociPath := filepath.Join(t.TempDir(), "oci.tar")
	writeTarFile(t, ociPath, []tarFile{
		{Name: "oci-layout", Contents: []byte(`{"imageLayoutVersion":"1.0.0"}`)},
		{Name: "index.json", Contents: []byte(`{"schemaVersion":2,"manifests":[]}`)},
	})

	a, err := archive.Open(ociPath)
	if err != nil {
		t.Fatalf("Open OCI: %v", err)
	}
	defer a.Close()
	if a.Format() != archive.FormatOCILayout {
		t.Fatalf("OCI: got format %v, want FormatOCILayout", a.Format())
	}

	dockerPath := filepath.Join(t.TempDir(), "docker.tar")
	writeTarFile(t, dockerPath, []tarFile{
		{Name: "manifest.json", Contents: []byte(`[]`)},
	})

	a2, err := archive.Open(dockerPath)
	if err != nil {
		t.Fatalf("Open docker: %v", err)
	}
	defer a2.Close()
	if a2.Format() != archive.FormatDockerArchive {
		t.Fatalf("docker: got format %v, want FormatDockerArchive", a2.Format())
	}

	unkPath := filepath.Join(t.TempDir(), "unknown.tar")
	writeTarFile(t, unkPath, []tarFile{
		{Name: "random.txt", Contents: []byte(`hi`)},
	})

	if _, err := archive.Open(unkPath); err == nil {
		t.Fatalf("expected error for unknown format, got nil")
	}
}
