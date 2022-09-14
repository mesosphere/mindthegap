// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cmd/mindthegap/push/helmbundle"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/push/imagebundle"
)

func NewCommand(out output.Output) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push images or Helm charts from bundles into an existing OCI registry",
	}

	cmd.AddCommand(imagebundle.NewCommand(out))
	cmd.AddCommand(helmbundle.NewCommand(out))
	return cmd
}
