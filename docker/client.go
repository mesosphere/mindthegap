package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/moby/moby/client"
	"k8s.io/klog/v2"
)

func NewDockerClientFromEnv() (*client.Client, error) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client - is Docker running? Error: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	dockerVersion, err := dockerClient.ServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check Docker version - is Docker running? Error: %w", err)
	}
	klog.V(4).Infof("Docker server version: %v", dockerVersion)
	return dockerClient, nil
}
