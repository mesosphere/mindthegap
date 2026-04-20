// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/mholt/archives"
)

const (
	ociLayoutFile   = "oci-layout"
	ociIndexFile    = "index.json"
	ociBlobsPrefix  = "blobs/"
	ociRefNameAnnot = "org.opencontainers.image.ref.name"
)

type ociArchive struct {
	path string
	fsys fs.FS
}

func openOCI(archivePath string) (Archive, error) {
	fsys, err := archives.FileSystem(context.Background(), archivePath, nil)
	if err != nil {
		return nil, fmt.Errorf(
			"opening OCI layout tarball %s as filesystem: %w",
			archivePath, err,
		)
	}
	if _, err := fs.Stat(fsys, ociLayoutFile); err != nil {
		return nil, fmt.Errorf(
			"OCI layout tarball %s is missing %s: %w",
			archivePath, ociLayoutFile, err,
		)
	}
	return &ociArchive{path: archivePath, fsys: fsys}, nil
}

func (o *ociArchive) Format() Format { return FormatOCILayout }

func (o *ociArchive) Close() error { return nil }

func (o *ociArchive) Entries() ([]Entry, error) {
	indexBytes, err := fs.ReadFile(o.fsys, ociIndexFile)
	if err != nil {
		return nil, fmt.Errorf(
			"reading %s from %s: %w", ociIndexFile, o.path, err,
		)
	}
	var idx v1.IndexManifest
	if err := json.Unmarshal(indexBytes, &idx); err != nil {
		return nil, fmt.Errorf(
			"decoding %s from %s: %w", ociIndexFile, o.path, err,
		)
	}

	var entries []Entry
	for i := range idx.Manifests {
		desc := &idx.Manifests[i]
		ref, err := refFromDescriptor(desc)
		if err != nil {
			return nil, fmt.Errorf(
				"parsing embedded ref in %s: %w", o.path, err,
			)
		}
		switch {
		case desc.MediaType.IsIndex():
			ii := &fsIndex{fsys: o.fsys, desc: *desc}
			entries = append(entries, Entry{Ref: ref, Index: ii})
		case desc.MediaType.IsImage():
			img, err := o.imageFromDescriptor(desc)
			if err != nil {
				return nil, err
			}
			entries = append(entries, Entry{Ref: ref, Image: img})
		default:
			return nil, fmt.Errorf(
				"%s: unsupported media type %q in index",
				o.path, desc.MediaType,
			)
		}
	}
	return entries, nil
}

func refFromDescriptor(desc *v1.Descriptor) (name.Reference, error) {
	if desc.Annotations == nil {
		return nil, nil
	}
	raw, ok := desc.Annotations[ociRefNameAnnot]
	if !ok || raw == "" {
		return nil, nil
	}
	return name.ParseReference(raw, name.StrictValidation)
}

func (o *ociArchive) imageFromDescriptor(desc *v1.Descriptor) (v1.Image, error) {
	img := &fsImage{fsys: o.fsys, desc: *desc}
	return partial.CompressedToImage(img)
}

type fsImage struct {
	fsys         fs.FS
	desc         v1.Descriptor
	manifestOnce sync.Once
	manifestBuf  []byte
	manifestErr  error
}

var _ partial.CompressedImageCore = (*fsImage)(nil)

func (i *fsImage) MediaType() (types.MediaType, error) {
	return i.desc.MediaType, nil
}

func (i *fsImage) RawManifest() ([]byte, error) {
	i.manifestOnce.Do(func() {
		i.manifestBuf, i.manifestErr = blobBytes(i.fsys, i.desc.Digest)
	})
	return i.manifestBuf, i.manifestErr
}

func (i *fsImage) RawConfigFile() ([]byte, error) {
	m, err := partial.Manifest(i)
	if err != nil {
		return nil, err
	}
	return blobBytes(i.fsys, m.Config.Digest)
}

func (i *fsImage) LayerByDigest(h v1.Hash) (partial.CompressedLayer, error) {
	m, err := partial.Manifest(i)
	if err != nil {
		return nil, err
	}
	if h == m.Config.Digest {
		return &fsBlob{fsys: i.fsys, desc: m.Config}, nil
	}
	for idx := range m.Layers {
		layer := &m.Layers[idx]
		if h == layer.Digest {
			return &fsBlob{fsys: i.fsys, desc: *layer}, nil
		}
	}
	return nil, fmt.Errorf("blob %s not found in manifest", h)
}

type fsBlob struct {
	fsys fs.FS
	desc v1.Descriptor
}

func (b *fsBlob) Digest() (v1.Hash, error)            { return b.desc.Digest, nil }
func (b *fsBlob) DiffID() (v1.Hash, error)            { return b.desc.Digest, nil }
func (b *fsBlob) Size() (int64, error)                { return b.desc.Size, nil }
func (b *fsBlob) MediaType() (types.MediaType, error) { return b.desc.MediaType, nil }

func (b *fsBlob) Compressed() (io.ReadCloser, error) {
	return openBlob(b.fsys, b.desc.Digest)
}

type fsIndex struct {
	fsys         fs.FS
	desc         v1.Descriptor
	manifestOnce sync.Once
	manifestBuf  []byte
	manifestErr  error
}

func (ii *fsIndex) MediaType() (types.MediaType, error) {
	return ii.desc.MediaType, nil
}

func (ii *fsIndex) Digest() (v1.Hash, error) { return ii.desc.Digest, nil }

func (ii *fsIndex) Size() (int64, error) { return ii.desc.Size, nil }

func (ii *fsIndex) IndexManifest() (*v1.IndexManifest, error) {
	raw, err := ii.RawManifest()
	if err != nil {
		return nil, err
	}
	var m v1.IndexManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (ii *fsIndex) RawManifest() ([]byte, error) {
	ii.manifestOnce.Do(func() {
		ii.manifestBuf, ii.manifestErr = blobBytes(ii.fsys, ii.desc.Digest)
	})
	return ii.manifestBuf, ii.manifestErr
}

func (ii *fsIndex) Image(h v1.Hash) (v1.Image, error) {
	m, err := ii.IndexManifest()
	if err != nil {
		return nil, err
	}
	for idx := range m.Manifests {
		d := &m.Manifests[idx]
		if d.Digest == h && d.MediaType.IsImage() {
			img := &fsImage{fsys: ii.fsys, desc: *d}
			return partial.CompressedToImage(img)
		}
	}
	return nil, fmt.Errorf("image %s not found in index", h)
}

func (ii *fsIndex) ImageIndex(h v1.Hash) (v1.ImageIndex, error) {
	m, err := ii.IndexManifest()
	if err != nil {
		return nil, err
	}
	for idx := range m.Manifests {
		d := &m.Manifests[idx]
		if d.Digest == h && d.MediaType.IsIndex() {
			return &fsIndex{fsys: ii.fsys, desc: *d}, nil
		}
	}
	return nil, fmt.Errorf("index %s not found in index", h)
}

func blobBytes(fsys fs.FS, h v1.Hash) ([]byte, error) {
	rc, err := openBlob(fsys, h)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func openBlob(fsys fs.FS, h v1.Hash) (io.ReadCloser, error) {
	p := path.Join(ociBlobsPrefix+h.Algorithm, h.Hex)
	f, err := fsys.Open(p)
	if err != nil {
		return nil, fmt.Errorf("opening blob %s: %w", p, err)
	}
	rc, ok := f.(io.ReadCloser)
	if !ok {
		return nil, errors.New("fs.File does not implement io.ReadCloser")
	}
	return rc, nil
}
