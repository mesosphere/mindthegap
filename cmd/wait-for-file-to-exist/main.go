// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

const usage = `wait-for-file-to-exist is a simple utility that waits for file(s) to exist and then exits.
An optional timeout can be provided which will cause the program to exit with an error. A timeout of zero (the default)
or less means no timeout.

Usage:

    wait-for-file-to-exist [--timeout duration] file1 file2
`

// Watch for files to exist and then exit.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	timeout := flag.Duration("timeout", 0, "timeout duration")

	flag.Usage = func() {
		fmt.Print(usage)
	}
	flag.Parse()

	// Set up a timeout context if requested.
	if *timeout > 0 {
		timeoutCtx, cancel := context.WithTimeout(ctx, *timeout)
		defer cancel()

		ctx = timeoutCtx
	}

	if flag.NArg() < 1 {
		exit("must specify at-least one file to watch")
	}
	fileToWaitFor := flag.Args()

	// Validate and exit early if any file is invalid.
	for _, f := range fileToWaitFor {
		if err := validateFileToWaitFor(f); err != nil {
			exit("%w", err)
		}
	}

	// Create one watcher per file.
	var wg sync.WaitGroup
	wg.Add(len(fileToWaitFor))
	type fileResult struct {
		file string
		err  error
	}
	resultCh := make(chan fileResult, len(fileToWaitFor))
	for _, f := range fileToWaitFor {
		printOutput("waiting for file %q to exist", f)
		go func(path string) {
			defer wg.Done()
			resultCh <- fileResult{file: path, err: waitForFile(ctx, path)}
		}(f)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for res := range resultCh {
		if res.err != nil {
			exit("error waiting for file %q: %w", res.file, res.err)
		}
		printOutput("file %q exists!", res.file)
	}
}

func validateFileToWaitFor(fileToWaitFor string) error {
	st, err := os.Lstat(fileToWaitFor)

	switch {
	case os.IsNotExist(err):
		// File does not exist, so we need to watch the parent directory for the file creation.
		// Ensure to watch the real path rather than a symlinked path which does not work properly with
		// inotify.
		_, err = filepath.EvalSymlinks(filepath.Dir(fileToWaitFor))
		if err != nil {
			return fmt.Errorf(
				"failed to evaluate any symlinks to read real directory for %q: %w",
				fileToWaitFor,
				err,
			)
		}
	case err != nil:
		return fmt.Errorf("failed to stat %q: %w", fileToWaitFor, err)
	case st.IsDir():
		return fmt.Errorf("%q is a directory, not a file", fileToWaitFor)
	case !st.Mode().IsRegular():
		return fmt.Errorf("%q exists but is not a regular file - type is %s", fileToWaitFor, st.Mode().String())
	}

	// File already exists and is a regular file.
	return nil
}

func waitForFile(ctx context.Context, fileToWaitFor string) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating a new watcher: %w", err)
	}
	defer w.Close()

	errChan := make(chan error)
	go fileLoop(ctx, w, fileToWaitFor, errChan)

	_, err = os.Lstat(fileToWaitFor)
	if os.IsNotExist(err) {
		dirPath, err := filepath.EvalSymlinks(filepath.Dir(fileToWaitFor))
		if err != nil {
			return fmt.Errorf("failed to evaluate any symlinks to read real directory for %q: %w", fileToWaitFor, err)
		}

		if err := w.Add(dirPath); err != nil {
			return fmt.Errorf("failed to add watch %q: %w", fileToWaitFor, err)
		}
	} else {
		// File already exists, no need to wait.
		return nil
	}

	// Wait for the file to exist.
	select {
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("error waiting for file %q: %w", fileToWaitFor, err)
		}
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for file %q to exist: %w", fileToWaitFor, ctx.Err())
	}

	// File now exists, stop waiting.
	return nil
}

func fileLoop(ctx context.Context, w *fsnotify.Watcher, fileToWaitFor string, errChan chan error) {
	for {
		select {
		case err, ok := <-w.Errors:
			if !ok { // Channel was closed (i.e. Watcher.Close() was called).
				errChan <- nil
			}
			errChan <- err
		case e, ok := <-w.Events:
			if !ok { // Channel was closed (i.e. Watcher.Close() was called).
				errChan <- nil
			}

			if filepath.Base(fileToWaitFor) == filepath.Base(e.Name) {
				errChan <- nil
			}
		case <-ctx.Done():
			errChan <- ctx.Err()
		}
	}
}

func printOutput(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

func exit(format string, a ...interface{}) {
	printOutput(format, a...)
	os.Exit(1)
}
