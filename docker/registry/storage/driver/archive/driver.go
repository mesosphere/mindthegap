// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	storagedriver "github.com/distribution/distribution/v3/registry/storage/driver"
	"github.com/distribution/distribution/v3/registry/storage/driver/base"
	"github.com/distribution/distribution/v3/registry/storage/driver/factory"
	"github.com/mholt/archives"
)

const (
	driverName        = "archive"
	defaultMaxThreads = uint64(100)

	// minThreads is the minimum value for the maxthreads configuration
	// parameter. If the driver's parameters are less than this we set
	// the parameters to minThreads.
	minThreads = uint64(25)

	dockerReposPath = "/docker/registry/v2/repositories"
)

// DriverParameters represents all configuration options available for the
// archive driver.
type DriverParameters struct {
	Archives           []string
	RepositoriesPrefix string
	MaxThreads         uint64
}

//nolint:gochecknoinits // This is the standard pattern for drivers.
func init() {
	factory.Register(driverName, &archiveDriverFactory{})
}

// archiveDriverFactory implements the factory.StorageDriverFactory interface.
type archiveDriverFactory struct{}

func (factory *archiveDriverFactory) Create(
	ctx context.Context, parameters map[string]interface{},
) (storagedriver.StorageDriver, error) {
	return FromParameters(ctx, parameters)
}

// driver is a storagedriver.StorageDriver implementation backed by a
// number of archives.
type driver struct {
	archiveFileSystems []fs.FS
	repositoriesPrefix string
}

type baseEmbed struct {
	base.Base
}

// Driver is a storagedriver.StorageDriver implementation backed by a
// number of archives. All provided paths will be entries in the archive.
type Driver struct {
	baseEmbed
}

// FromParameters constructs a new Driver with a given parameters map
// Optional Parameters:
// - archives
// - maxthreads.
func FromParameters(ctx context.Context, parameters map[string]interface{}) (*Driver, error) {
	params, err := fromParametersImpl(parameters)
	if err != nil || params == nil {
		return nil, err
	}
	return New(ctx, *params)
}

func fromParametersImpl(parameters map[string]interface{}) (*DriverParameters, error) {
	archivesParam, ok := parameters["archives"]
	if !ok {
		return nil, errors.New("archive config is required")
	}

	archivesIface, ok := archivesParam.([]interface{})
	if !ok {
		return nil, errors.New("archives  config must be a string array")
	}

	archiveBundles := make([]string, 0, len(archivesIface))
	for _, archive := range archivesIface {
		archiveStr, ok := archive.(string)
		if !ok {
			return nil, errors.New("archives config must be a string array")
		}
		archiveBundles = append(archiveBundles, archiveStr)
	}
	if len(archiveBundles) == 0 {
		return nil, errors.New("archives config is required")
	}

	maxThreads, err := base.GetLimitFromParameter(parameters["maxthreads"], minThreads, defaultMaxThreads)
	if err != nil {
		return nil, fmt.Errorf("maxthreads config error: %s", err.Error())
	}

	repositoriesPrefix := ""
	repositoriesPrefixConfig, ok := parameters["repositoriesPrefix"]
	if ok && repositoriesPrefixConfig != nil {
		repositoriesPrefix = fmt.Sprint(repositoriesPrefixConfig)
	}

	params := &DriverParameters{
		Archives:           archiveBundles,
		MaxThreads:         maxThreads,
		RepositoriesPrefix: repositoriesPrefix,
	}
	return params, nil
}

// New constructs a new Driver with a given archives.
func New(ctx context.Context, params DriverParameters) (*Driver, error) {
	archiveFileSystems := make([]fs.FS, 0, len(params.Archives))

	for _, archive := range params.Archives {
		fsys, err := archives.FileSystem(ctx, archive, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to open archive %s as filesystem: %w", archive, err)
		}

		archiveFileSystems = append(archiveFileSystems, fsys)
	}

	archivesDriver := &driver{
		archiveFileSystems: archiveFileSystems,
		repositoriesPrefix: params.RepositoriesPrefix,
	}

	return &Driver{
		baseEmbed: baseEmbed{
			Base: base.Base{
				StorageDriver: base.NewRegulator(archivesDriver, params.MaxThreads),
			},
		},
	}, nil
}

// Implement the storagedriver.StorageDriver interface

func (d *driver) Name() string {
	return driverName
}

// GetContent retrieves the content stored at "path" as a []byte.
func (d *driver) GetContent(ctx context.Context, fPath string) ([]byte, error) {
	fPath = strings.Replace(
		fPath, path.Join(dockerReposPath, d.repositoriesPrefix), dockerReposPath, 1,
	)
	fPath = strings.TrimPrefix(fPath, "/")

	rc, err := d.Reader(ctx, fPath, 0)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	p, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// PutContent stores the []byte content at a location designated by "path".
func (d *driver) PutContent(ctx context.Context, subPath string, contents []byte) error {
	return fmt.Errorf("archive driver is read-only")
}

// Reader retrieves an io.ReadCloser for the content stored at "path" with a
// given byte offset.
func (d *driver) Reader(ctx context.Context, fPath string, offset int64) (io.ReadCloser, error) {
	if offset != 0 {
		return nil, fmt.Errorf("archive driver does not support reading from an offset")
	}

	fPath = strings.Replace(
		fPath, path.Join(dockerReposPath, d.repositoriesPrefix), dockerReposPath, 1,
	)
	fPath = strings.TrimPrefix(fPath, "/")

	var err error
	for _, tfs := range d.archiveFileSystems {
		var file fs.File
		file, err = tfs.Open(fPath)
		if err == nil {
			return file, nil
		}

		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	if os.IsNotExist(err) {
		return nil, storagedriver.PathNotFoundError{Path: fPath}
	}

	return nil, err
}

func (d *driver) Writer(
	ctx context.Context, subPath string, appendTo bool,
) (storagedriver.FileWriter, error) {
	return nil, fmt.Errorf("archive driver is read-only")
}

// Stat retrieves the FileInfo for the given path, including the current size
// in bytes and the creation time.
func (d *driver) Stat(ctx context.Context, subPath string) (storagedriver.FileInfo, error) {
	archiveSubpath := strings.Replace(
		subPath, path.Join(dockerReposPath, d.repositoriesPrefix), dockerReposPath, 1,
	)
	archiveSubpath = strings.TrimPrefix(archiveSubpath, "/")

	var err error
	for _, tfs := range d.archiveFileSystems {
		var fi fs.FileInfo
		fi, err = fs.Stat(tfs, archiveSubpath)
		if err == nil {
			return fileInfo{
				path:     subPath,
				FileInfo: fi,
			}, nil
		}

		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	if !os.IsNotExist(err) {
		return nil, storagedriver.PathNotFoundError{Path: subPath}
	}

	return nil, err
}

// List returns a list of the objects that are direct descendants of the given
// path.
func (d *driver) List(ctx context.Context, subPath string) ([]string, error) {
	var keys []string

	archiveSubpath := strings.Replace(
		subPath, path.Join(dockerReposPath, d.repositoriesPrefix), dockerReposPath, 1,
	)
	archiveSubpath = strings.TrimPrefix(archiveSubpath, "/")

	for _, tfs := range d.archiveFileSystems {
		dirEntries, err := fs.ReadDir(tfs, archiveSubpath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, storagedriver.PathNotFoundError{Path: subPath}
			}
			return nil, err
		}

		for _, dirEntry := range dirEntries {
			keys = append(
				keys,
				path.Join(
					strings.Replace("/"+archiveSubpath, dockerReposPath, path.Join(dockerReposPath, d.repositoriesPrefix), 1),
					dirEntry.Name(),
				),
			)
		}
	}

	return keys, nil
}

// Move moves an object stored at sourcePath to destPath, removing the original
// object.
func (d *driver) Move(ctx context.Context, sourcePath, destPath string) error {
	return fmt.Errorf("archive driver is read-only")
}

// Delete recursively deletes all objects stored at "path" and its subpaths.
func (d *driver) Delete(ctx context.Context, subPath string) error {
	return fmt.Errorf("archive driver is read-only")
}

// RedirectURL returns a URL which may be used to retrieve the content stored at the given path.
func (d *driver) RedirectURL(*http.Request, string) (string, error) {
	return "", nil
}

// Walk traverses a filesystem defined within driver, starting
// from the given path, calling f on each file and directory.
func (d *driver) Walk(
	ctx context.Context, path string, f storagedriver.WalkFn, options ...func(*storagedriver.WalkOptions),
) error {
	return storagedriver.WalkFallback(ctx, d, path, f, options...)
}

type fileInfo struct {
	os.FileInfo
	path string
}

var _ storagedriver.FileInfo = fileInfo{}

// Path provides the full path of the target of this file info.
func (fi fileInfo) Path() string {
	return fi.path
}

// Size returns current length in bytes of the file. The return value can
// be used to write to the end of the file at path. The value is
// meaningless if IsDir returns true.
func (fi fileInfo) Size() int64 {
	if fi.IsDir() {
		return 0
	}

	return fi.FileInfo.Size()
}

// ModTime returns the modification time for the file. For backends that
// don't have a modification time, the creation time should be returned.
func (fi fileInfo) ModTime() time.Time {
	return fi.FileInfo.ModTime()
}

// IsDir returns true if the path is a directory.
func (fi fileInfo) IsDir() bool {
	return fi.FileInfo.IsDir()
}
