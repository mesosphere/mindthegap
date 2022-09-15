// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package helpers

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

var ErrDockerDaemonNotAccessible = errors.New("Docker daemon is not accessible")

type Docker struct {
	cl *client.Client
}

func NewDockerClient() (*Docker, error) {
	cl, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	_, err = cl.Info(context.Background())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDockerDaemonNotAccessible, err)
	}

	return &Docker{cl: cl}, nil
}

func (d *Docker) PullImage(ctx context.Context, image string) error {
	_, _, err := d.cl.ImageInspectWithRaw(ctx, image)
	if err == nil {
		return nil
	}
	if !client.IsErrNotFound(err) {
		return fmt.Errorf("failed to inspect image %q: %w", image, err)
	}

	r, err := d.cl.ImagePull(ctx, image, types.ImagePullOptions{})
	defer r.Close()
	if err != nil {
		return fmt.Errorf("failed to pull image %q: %w", image, err)
	}
	_, err = io.Copy(io.Discard, r)
	if err != nil {
		return fmt.Errorf("failed to swallow pull image output: %w", err)
	}

	return nil
}

func (d *Docker) StartContainer(ctx context.Context, cfg container.Config) (*Container, error) {
	if err := d.PullImage(ctx, cfg.Image); err != nil {
		return nil, err
	}

	container, err := d.cl.ContainerCreate(
		ctx,
		&cfg,
		&container.HostConfig{},
		&network.NetworkingConfig{},
		&specs.Platform{},
		"",
	)
	if err != nil {
		return nil, err
	}

	if err := d.cl.ContainerStart(ctx, container.ID, types.ContainerStartOptions{}); err != nil {
		_ = d.cl.ContainerRemove(
			ctx,
			container.ID,
			types.ContainerRemoveOptions{Force: true, RemoveVolumes: true},
		)

		return nil, err
	}

	return &Container{
		id: container.ID,
		d:  d,
	}, nil
}

func (d *Docker) Close() error {
	return d.cl.Close()
}

type Container struct {
	id string
	d  *Docker
}

func (c *Container) Stop(ctx context.Context) error {
	return c.d.cl.ContainerRemove(
		ctx,
		c.id,
		types.ContainerRemoveOptions{Force: true, RemoveVolumes: true},
	)
}

func (c *Container) CopyTo(ctx context.Context, dest string, src io.Reader) error {
	return c.d.cl.CopyToContainer(ctx, c.id, dest, src, types.CopyToContainerOptions{})
}

func (c *Container) Exec(
	ctx context.Context,
	stdout, stderr io.Writer,
	cmd ...string,
) (int, error) {
	exec, err := c.d.cl.ContainerExecCreate(
		ctx,
		c.id,
		types.ExecConfig{
			Cmd:          cmd,
			AttachStdout: true,
			AttachStderr: true,
		},
	)
	if err != nil {
		return -1, fmt.Errorf("failed to create exec in container: %w", err)
	}
	resp, err := c.d.cl.ContainerExecAttach(ctx, exec.ID, types.ExecStartCheck{})
	if err != nil {
		return -1, fmt.Errorf("failed to attach exec in container: %w", err)
	}
	defer resp.Close()
	errCh := make(chan error)
	go func() {
		_, err := stdcopy.StdCopy(stdout, stderr, resp.Reader)
		if err != nil {
			errCh <- fmt.Errorf("failed to copy exec streams: %w", err)
		}
		close(errCh)
	}()
	if err := c.d.cl.ContainerExecStart(ctx, exec.ID, types.ExecStartCheck{}); err != nil {
		return -1, fmt.Errorf("failed to start exec in container: %w", err)
	}
	if err := <-errCh; err != nil {
		return -1, fmt.Errorf("failed to read exec streams: %w", err)
	}
	execInspect, err := c.d.cl.ContainerExecInspect(ctx, exec.ID)
	if err != nil {
		return -1, fmt.Errorf("failed to inspect exec in container: %w", err)
	}
	return execInspect.ExitCode, nil
}
