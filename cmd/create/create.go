package create

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/mesosphere/mindthegap/cmd/create/imagebundle"
)

func NewCommand(ioStreams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use: "create",
	}

	cmd.AddCommand(imagebundle.NewCommand(ioStreams))
	return cmd
}
