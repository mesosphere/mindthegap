// Copyright 2021 D2iQ, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"io"
	"os"

	"github.com/mesosphere/dkp-cli/runtime/cmd/root"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/mesosphere/mindthegap/cmd/create"
	"github.com/mesosphere/mindthegap/cmd/push"
	"github.com/mesosphere/mindthegap/cmd/serve"
)

func NewCommand(in io.Reader, out, errOut io.Writer) *cobra.Command {
	ioStreams := genericclioptions.IOStreams{In: in, Out: out, ErrOut: errOut}

	rootCmd, _ := root.NewCommand(ioStreams)

	rootCmd.AddCommand(create.NewCommand(ioStreams))
	rootCmd.AddCommand(push.NewCommand(ioStreams))
	rootCmd.AddCommand(serve.NewCommand(ioStreams))

	return rootCmd
}

func Execute() {
	rootCmd := NewCommand(os.Stdin, os.Stdout, os.Stderr)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
