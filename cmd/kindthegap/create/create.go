// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package create

import (
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cmd/kindthegap/create/cluster"
)

func NewCommand(out output.Output) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an air-gapped KinD cluster",
	}

	cmd.AddCommand(cluster.NewCommand(out))
	return cmd
}
