// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package delete

import (
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cmd/kindthegap/delete/cluster"
)

func NewCommand(out output.Output) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an air-gapped KinD cluster",
	}

	cmd.AddCommand(cluster.NewCommand(out))
	return cmd
}
