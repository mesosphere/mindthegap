package tar

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Tar(dir, outputFile string) error {
	tmpBundleFile, err := os.CreateTemp(filepath.Dir(outputFile), fmt.Sprintf(".%s-*", filepath.Base(outputFile)))
	if err != nil {
		return fmt.Errorf("failed to create temp bundle file: %w", err)
	}
	defer func() {
		tmpBundleFile.Close()
		os.Remove(tmpBundleFile.Name())
	}()

	tw := tar.NewWriter(tmpBundleFile)
	defer tw.Close()

	err = filepath.Walk(dir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		// create a new dir/file header
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		// update the name to correctly reflect the desired destination when untaring
		header.Name = strings.TrimPrefix(strings.ReplaceAll(file, dir, ""), string(filepath.Separator))

		// write the header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// open files for taring
		f, err := os.Open(file)
		if err != nil {
			return err
		}

		// copy file data into tar writer
		if _, err := io.Copy(tw, f); err != nil {
			return err
		}

		// manually close here after each file operation; defering would cause each file close
		// to wait until all operations have completed.
		f.Close()

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to tar up bundle directory: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	if err := tmpBundleFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary bundle file %w", err)
	}

	if err := os.Rename(tmpBundleFile.Name(), outputFile); err != nil {
		return fmt.Errorf("failed to move temporary bundle file to output file: %w", err)
	}

	return nil
}
