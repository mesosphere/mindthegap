// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package importcmd

import (
	"github.com/mesosphere/dkp-cli-runtime/core/output"
	"github.com/spf13/cobra"

	"github.com/mesosphere/mindthegap/cmd/importcmd/imagebundle"
)

func NewCommand(out output.Output) *cobra.Command {
	cmd := &cobra.Command{
		Use: "import",
	}

	cmd.AddCommand(imagebundle.NewCommand(out))
	return cmd
}
