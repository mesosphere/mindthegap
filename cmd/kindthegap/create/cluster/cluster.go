// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/moby/moby/pkg/namesgenerator"
	"github.com/spf13/cobra"
	"sigs.k8s.io/kind/pkg/log"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/docker/client"
	"github.com/mesosphere/mindthegap/kind"
	"github.com/mesosphere/mindthegap/kind/options"
)

func NewCommand(out output.Output) *cobra.Command {
	opts := options.NewClusterOptions()

	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Create an air-gapped KinD cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts = opts.Complete(
				func(co options.ClusterOptions) options.ClusterOptions {
					if co.ClusterName() == "" {
						name := namesgenerator.GetRandomName(0)
						name = strings.ReplaceAll(name, "_", "-")
						return options.WithClusterName(name)(co)
					}
					return co
				},
			)

			if err := opts.Validate(); err != nil {
				return err
			}

			out.StartOperation(
				fmt.Sprintf(
					"Checking that KinD cluster %q doesn't already exist",
					opts.ClusterName(),
				),
			)
			ok, err := kind.IsExistingCluster(opts.ClusterName(), log.NoopLogger{})
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to check if KinD cluster already exists: %w", err)
			}
			if ok {
				out.EndOperation(false)
				return fmt.Errorf("KinD cluster %q already exists", opts.ClusterName())
			}
			out.EndOperation(true)

			out.StartOperation(
				fmt.Sprintf(
					"Checking that Docker network %q doesn't already exist",
					opts.DockerNetworkName(),
				),
			)
			ok, err = client.IsExistingNetwork(context.Background(), opts.DockerNetworkName())
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to check if Docker network already exists: %w", err)
			}
			if ok {
				out.EndOperation(false)
				return fmt.Errorf("docker network %q already exists", opts.DockerNetworkName())
			}
			out.EndOperation(true)

			out.StartOperation(fmt.Sprintf("Creating KinD cluster %q", opts.ClusterName()))
			if err := kind.CreateAirGappedCluster(opts, log.NoopLogger{}); err != nil {
				out.EndOperation(false)
				return err
			}
			out.EndOperation(true)

			out.Infof(`
To connect to your air-gapped KinD cluster, please docker exec into acontrol plane node directly:

docker exec -it %s-control-plane bash
`, opts.ClusterName())

			return nil
		},
	}

	opts.AddFlags(cmd)

	return cmd
}
