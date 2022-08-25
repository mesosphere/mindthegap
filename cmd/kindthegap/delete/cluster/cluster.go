// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"fmt"

	"github.com/spf13/cobra"
	"sigs.k8s.io/kind/pkg/log"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/kind"
	"github.com/mesosphere/mindthegap/kind/options"
)

func NewCommand(out output.Output) *cobra.Command {
	opts := options.NewClusterNameOptions()

	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Delete an air-gapped KinD cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			ok, err := kind.IsExistingCluster(opts.ClusterName(), log.NoopLogger{})
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to check if KinD cluster already exists: %w", err)
			}
			if !ok {
				out.EndOperation(false)
				return fmt.Errorf("KinD cluster %q does not exist", opts.ClusterName())
			}
			out.EndOperation(true)

			out.StartOperation(fmt.Sprintf("Deleting KinD cluster %q", opts.ClusterName()))
			err = kind.DeleteAirGappedCluster(opts.ClusterName(), log.NoopLogger{})
			if err != nil {
				out.EndOperation(false)
				return err
			}
			out.EndOperation(true)

			return nil
		},
	}

	opts.AddFlags(cmd)

	return cmd
}
