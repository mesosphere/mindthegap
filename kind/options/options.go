// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package options

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
)

func NewClusterNameOptions() ClusterNameOptions {
	return ClusterNameOptions{}
}

func NewClusterOptions(opts ...ClusterOptFn) ClusterOptions {
	o := ClusterOptions{
		controlPlaneReplicas: 1,
		workerReplicas:       0,
	}

	for _, opt := range opts {
		o = opt(o)
	}

	return o
}

type ClusterOptFn func(ClusterOptions) ClusterOptions

func WithClusterName(name string) ClusterOptFn {
	return func(o ClusterOptions) ClusterOptions {
		o.clusterName = name
		return o
	}
}

func WithDockerNetworkName(name string) ClusterOptFn {
	return func(o ClusterOptions) ClusterOptions {
		o.dockerNetworkName = name
		return o
	}
}

func WithKubernetesVersion(v string) ClusterOptFn {
	return func(o ClusterOptions) ClusterOptions {
		o.kubernetesVersion = v
		return o
	}
}

func WithKindNodeImage(img string) ClusterOptFn {
	return func(o ClusterOptions) ClusterOptions {
		o.kindNodeImage = img
		return o
	}
}

func WithControlPlaneReplicas(n uint8) ClusterOptFn {
	return func(o ClusterOptions) ClusterOptions {
		o.controlPlaneReplicas = n
		return o
	}
}

func WithWorkerReplicas(n uint8) ClusterOptFn {
	return func(o ClusterOptions) ClusterOptions {
		o.workerReplicas = n
		return o
	}
}

type ClusterNameOptions struct {
	clusterName       string
	dockerNetworkName string
}

func (o ClusterNameOptions) Validate() error {
	var errs error

	if o.ClusterName() == "" {
		errs = multierror.Append(errs, fmt.Errorf("%w: clusterName", ErrRequiredField))
	}
	if o.DockerNetworkName() == "" {
		errs = multierror.Append(errs, fmt.Errorf("%w: dockerNetworkName", ErrRequiredField))
	}

	return errs
}

func (o *ClusterNameOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.clusterName, "cluster-name", "",
		"Name of cluster to use, if not specified a name will be generated")
	cmd.Flags().
		StringVar(&o.dockerNetworkName, "docker-network-name", "",
			"Docker network name to use, if not specified will be set to the same as cluster name")
}

type ClusterOptions struct {
	ClusterNameOptions
	kubernetesVersion    string
	kindNodeImage        string
	controlPlaneReplicas uint8
	workerReplicas       uint8
}

func (o ClusterNameOptions) ClusterName() string {
	return o.clusterName
}

func (o ClusterNameOptions) DockerNetworkName() string {
	if o.dockerNetworkName == "" {
		return o.clusterName
	}
	return o.dockerNetworkName
}

func (o ClusterOptions) KubernetesVersion() string {
	return o.kubernetesVersion
}

func (o ClusterOptions) KindNodeImage() string {
	return o.kindNodeImage
}

func (o ClusterOptions) ControlPlaneReplicas() uint8 {
	return o.controlPlaneReplicas
}

func (o ClusterOptions) WorkerReplicas() uint8 {
	return o.workerReplicas
}

func (o *ClusterOptions) AddFlags(cmd *cobra.Command) {
	nameOptions := &o.ClusterNameOptions
	nameOptions.AddFlags(cmd)
	cmd.Flags().
		StringVar(&o.kubernetesVersion, "kubernetes-version", "",
			"Kubernetes version to create a cluster for, ignored if --kind-node-image is specified")
	cmd.Flags().
		StringVar(&o.kindNodeImage, "kind-node-image", "",
			"KinD node image to use to create cluster from. If unset, "+
				"will default to mesosphere/kind-node:<kubernetesVersion>")
	cmd.Flags().
		Uint8Var(&o.controlPlaneReplicas, "control-plane-replicas", 1,
			"Number of control plane replicas to create")
	cmd.Flags().
		Uint8Var(&o.workerReplicas, "worker-replicas", 0,
			"Number of worker replicas to create")

	cmd.MarkFlagsMutuallyExclusive("kubernetes-version", "kind-node-image")
}

var ErrRequiredField = errors.New("field is required")

func (o ClusterOptions) Validate() error {
	var errs error

	if err := o.ClusterNameOptions.Validate(); err != nil {
		errs = multierror.Append(errs, err)
	}

	if o.KindNodeImage() == "" && o.KubernetesVersion() == "" {
		errs = multierror.Append(
			errs,
			fmt.Errorf(
				"%w: either kindNodeImage or kubernetesVersion must be set",
				ErrRequiredField,
			),
		)
	}

	return errs
}

func (o ClusterOptions) Complete(opts ...ClusterOptFn) ClusterOptions {
	for _, opt := range opts {
		o = opt(o)
	}

	return o
}
