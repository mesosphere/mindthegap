package cmd

import (
	"io"
	"os"

	"github.com/mesosphere/dkp-cli/runtime/cmd/root"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/mesosphere/mindthegap/cmd/create"
	"github.com/mesosphere/mindthegap/cmd/push"
)

func NewCommand(in io.Reader, out, errOut io.Writer) *cobra.Command {
	ioStreams := genericclioptions.IOStreams{In: in, Out: out, ErrOut: errOut}

	rootCmd, _ := root.NewCommand(ioStreams)

	rootCmd.AddCommand(create.NewCommand(ioStreams))
	rootCmd.AddCommand(push.NewCommand(ioStreams))

	return rootCmd
}

func Execute() {
	rootCmd := NewCommand(os.Stdin, os.Stdout, os.Stderr)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
