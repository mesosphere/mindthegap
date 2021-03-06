// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package serve

import (
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cmd/serve/helmbundle"
	"github.com/mesosphere/mindthegap/cmd/serve/imagebundle"
)

func NewCommand(out output.Output) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve an image registry or a helm chart repository",
	}

	cmd.AddCommand(imagebundle.NewCommand(out))
	cmd.AddCommand(helmbundle.NewCommand(out))
	return cmd
}
