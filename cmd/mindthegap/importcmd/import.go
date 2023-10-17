// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package importcmd

import (
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cmd/mindthegap/importcmd/imagebundle"
)

func NewCommand(out output.Output) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import images from an image bundle into Containerd",
	}

	cmd.AddCommand(imagebundle.NewCommand(out))
	return cmd
}
