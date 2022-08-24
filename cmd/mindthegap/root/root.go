// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/cmd/root"
	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cmd/mindthegap/create"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/importcmd"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/push"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/serve"
)

func NewCommand(in io.Reader, out, errOut io.Writer) (*cobra.Command, output.Output) {
	rootCmd, rootOpts := root.NewCommand(out, errOut)

	rootCmd.AddCommand(create.NewCommand(rootOpts.Output))
	rootCmd.AddCommand(push.NewCommand(rootOpts.Output))
	rootCmd.AddCommand(serve.NewCommand(rootOpts.Output))
	rootCmd.AddCommand(importcmd.NewCommand(rootOpts.Output))

	return rootCmd, rootOpts.Output
}

func Execute() {
	rootCmd, out := NewCommand(os.Stdin, os.Stdout, os.Stderr)
	// disable cobra built-in error printing, we output the error with formatting.
	rootCmd.SilenceErrors = true

	if err := rootCmd.Execute(); err != nil {
		out.Error(err, "")
		os.Exit(1)
	}
}
