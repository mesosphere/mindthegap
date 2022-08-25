// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kind

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/log"

	"github.com/mesosphere/mindthegap/docker/client"
	"github.com/mesosphere/mindthegap/kind/options"
)

func IsExistingCluster(name string, logger log.Logger) (bool, error) {
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
		cluster.ProviderWithDocker(),
	)
	clusters, err := provider.List()
	if err != nil {
		return false, err
	}
	for _, cluster := range clusters {
		if cluster == name {
			return true, nil
		}
	}
	return false, nil
}

var createLock sync.Mutex

func CreateAirGappedCluster(opts options.ClusterOptions, logger log.Logger) error {
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
		cluster.ProviderWithDocker(),
	)

	var cpNodes []v1alpha4.Node
	for i := 0; i < int(opts.ControlPlaneReplicas()); i++ {
		cpNodes = append(cpNodes, v1alpha4.Node{Role: v1alpha4.ControlPlaneRole})
	}
	var workerNodes []v1alpha4.Node
	for i := 0; i < int(opts.WorkerReplicas()); i++ {
		workerNodes = append(workerNodes, v1alpha4.Node{Role: v1alpha4.WorkerRole})
	}

	tempKubeconfig, err := os.CreateTemp("", ".kubeconfig-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file for kubeconfig: %w", err)
	}
	tempKubeconfig.Close()
	defer os.Remove(tempKubeconfig.Name())

	nodeImage := opts.KindNodeImage()
	if nodeImage == "" {
		nodeImage = fmt.Sprintf("mesosphere/kind-node:%s", opts.KubernetesVersion())
	}

	_, err = client.CreateDockerNetwork(context.Background(), opts.DockerNetworkName(), true)
	if err != nil {
		return err
	}

	createLock.Lock()
	defer createLock.Unlock()

	os.Setenv("KIND_EXPERIMENTAL_DOCKER_NETWORK", opts.DockerNetworkName())
	defer os.Unsetenv("KIND_EXPERIMENTAL_DOCKER_NETWORK")

	err = provider.Create(
		opts.ClusterName(),
		cluster.CreateWithKubeconfigPath(tempKubeconfig.Name()),
		cluster.CreateWithNodeImage(nodeImage),
		cluster.CreateWithV1Alpha4Config(&v1alpha4.Cluster{
			Name:  opts.ClusterName(),
			Nodes: append(cpNodes, workerNodes...),
		}),
	)
	if err != nil {
		if !strings.Contains(err.Error(), "failed to get api server port") {
			return fmt.Errorf("failed to create KinD cluster: %w", err)
		}
	}

	return nil
}

func DeleteAirGappedCluster(name string, logger log.Logger) error {
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
		cluster.ProviderWithDocker(),
	)

	nwID, err := client.ContainerNetworkID(
		context.Background(),
		fmt.Sprintf("%s-control-plane", name),
	)
	if err != nil {
		return err
	}

	if err := provider.Delete(name, ""); err != nil {
		return fmt.Errorf("failed to delete KinD cluster: %w", err)
	}

	if err := client.DeleteDockerNetwork(context.Background(), nwID); err != nil {
		return err
	}

	return nil
}
