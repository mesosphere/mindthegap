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

	"github.com/fsnotify/fsnotify"
)

const usage = `wait-for-file-to-exist is a simple utility that waits for a file to exist and then exits.
An optional timeout can be provided which will cause the program to exit with an error. A timeout of zero (the default)
or less means no timeout.

Usage:

    wait-for-file-to-exist [--timeout duration] file
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

	if flag.NArg() < 1 {
		exit("must specify a file to watch")
	}
	if flag.NArg() > 1 {
		exit("only one file can be specified")
	}
	fileToWaitFor := flag.Arg(0)

	if *timeout > 0 {
		timeoutCtx, cancel := context.WithTimeout(ctx, *timeout)
		defer cancel()

		ctx = timeoutCtx
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		exit("creating a new watcher: %s", err)
	}
	defer w.Close()

	// Start listening for events.
	errChan := make(chan error)
	go fileLoop(ctx, w, fileToWaitFor, errChan)

	st, err := os.Lstat(fileToWaitFor)

	switch {
	case os.IsNotExist(err):
		// File does not exist, so we need to watch the parent directory for the file creation.
		// Ensure to watch the real path rather than a symlinked path which does not work properly with
		// inotify.
		dirPath, err := filepath.EvalSymlinks(filepath.Dir(fileToWaitFor))
		if err != nil {
			exit(
				"failed to evaluate any symlinks to read real directory for %q: %v",
				fileToWaitFor,
				err,
			)
		}

		if dirPath != filepath.Dir(fileToWaitFor) {
			printOutput(
				"watching for file %q (real path %q) to exist",
				fileToWaitFor,
				filepath.Join(dirPath, filepath.Base(fileToWaitFor)),
			)
		} else {
			printOutput(
				"watching for file %q to exist",
				fileToWaitFor,
			)
		}

		if err := w.Add(dirPath); err != nil {
			exit("failed to add watch %q: %v", fileToWaitFor, err)
		}
	case err != nil:
		exit("failed to stat %q: %v", fileToWaitFor, err)
	case st.IsDir():
		exit("%q is a directory, not a file", fileToWaitFor)
	case !st.Mode().IsRegular():
		exit("%q exists but is not a regular file - type is %s", fileToWaitFor, st.Mode().String())
	default:
		// File already exists and is a regular file - we are done.
		printOutput("file already exists!")
		return
	}

	// Wait for the file to exist.
	select {
	case err := <-errChan:
		if err != nil {
			exit("error waiting for file %q: %v", fileToWaitFor, err)
		}
		printOutput("file now exists!")
	case <-ctx.Done():
		exit("timeout waiting for file %q: %v", fileToWaitFor, ctx.Err())
	}
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
