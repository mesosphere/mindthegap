// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/moby/moby/client"
)

type Docker interface {
	IsExistingNetwork(ctx context.Context, name string) (existing bool, err error)
	CreateDockerNetwork(
		ctx context.Context,
		name string,
		internal bool,
	) (networkID string, err error)
	DeleteDockerNetwork(ctx context.Context, networkID string) error
	ContainerNetworkID(ctx context.Context, containerID string) (string, error)
}

type dockerClient struct {
	cl *client.Client
}

func New() (Docker, error) {
	cl, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	return &dockerClient{cl: cl}, nil
}

func (c dockerClient) IsExistingNetwork(ctx context.Context, name string) (bool, error) {
	_, err := c.cl.NetworkInspect(ctx, name, types.NetworkInspectOptions{})
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to inspect Docker network: %w", err)
	}

	return true, nil
}

func IsExistingNetwork(ctx context.Context, name string) (bool, error) {
	cl, err := New()
	if err != nil {
		return false, err
	}

	return cl.IsExistingNetwork(ctx, name)
}

func (c dockerClient) CreateDockerNetwork(
	ctx context.Context,
	name string,
	internal bool,
) (networkID string, err error) {
	nw, err := c.cl.NetworkCreate(
		ctx,
		name,
		types.NetworkCreate{Internal: internal, CheckDuplicate: true},
	)
	if err != nil {
		return "", fmt.Errorf("failed to create Docker network: %w", err)
	}

	return nw.ID, nil
}

func CreateDockerNetwork(
	ctx context.Context,
	name string,
	internal bool,
) (networkID string, err error) {
	cl, err := New()
	if err != nil {
		return "", err
	}

	return cl.CreateDockerNetwork(ctx, name, internal)
}

func (c dockerClient) DeleteDockerNetwork(ctx context.Context, networkID string) error {
	err := c.cl.NetworkRemove(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to delete Docker network: %w", err)
	}

	return nil
}

func DeleteDockerNetwork(ctx context.Context, networkID string) error {
	cl, err := New()
	if err != nil {
		return err
	}

	return cl.DeleteDockerNetwork(ctx, networkID)
}

func (c dockerClient) ContainerNetworkID(ctx context.Context, containerID string) (string, error) {
	container, err := c.cl.ContainerInspect(ctx, containerID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return "", nil
		}

		return "", fmt.Errorf("failed to inspect Docker container: %w", err)
	}

	for _, v := range container.NetworkSettings.Networks {
		return v.NetworkID, nil
	}

	return "", nil
}

func ContainerNetworkID(ctx context.Context, containerName string) (string, error) {
	cl, err := New()
	if err != nil {
		return "", err
	}

	return cl.ContainerNetworkID(ctx, containerName)
}
