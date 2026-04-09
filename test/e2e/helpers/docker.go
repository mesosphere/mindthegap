// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package helpers

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

var ErrDockerDaemonNotAccessible = errors.New("Docker daemon is not accessible")

type Docker struct {
	cl *client.Client
}

func NewDockerClient() (*Docker, error) {
	cl, err := client.New(client.FromEnv)
	if err != nil {
		return nil, err
	}

	_, err = cl.Ping(context.Background(), client.PingOptions{})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDockerDaemonNotAccessible, err)
	}

	return &Docker{cl: cl}, nil
}

func (d *Docker) PullImage(ctx context.Context, image string) error {
	return d.PullImageWithPlatform(ctx, image, nil)
}

func (d *Docker) PullImageWithPlatform(
	ctx context.Context,
	img string,
	platform *specs.Platform,
) error {
	opts := client.ImagePullOptions{}
	if platform != nil && platform.OS != "" {
		opts.Platforms = []specs.Platform{*platform}
	}
	resp, err := d.cl.ImagePull(ctx, img, opts)
	if err != nil {
		return fmt.Errorf("failed to pull image %q: %w", img, err)
	}
	defer resp.Close()
	if err := resp.Wait(ctx); err != nil {
		return fmt.Errorf("failed to pull image %q: %w", img, err)
	}

	return nil
}

func (d *Docker) StartContainer(ctx context.Context, cfg container.Config) (*Container, error) {
	return d.StartContainerWithPlatform(ctx, cfg, &specs.Platform{})
}

func (d *Docker) StartContainerWithPlatform(
	ctx context.Context,
	cfg container.Config,
	platform *specs.Platform,
) (*Container, error) {
	if err := d.PullImageWithPlatform(ctx, cfg.Image, platform); err != nil {
		return nil, err
	}

	ctr, err := d.cl.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:           &cfg,
		HostConfig:       &container.HostConfig{},
		NetworkingConfig: &network.NetworkingConfig{},
		Platform:         platform,
	})
	if err != nil {
		return nil, err
	}

	if _, err := d.cl.ContainerStart(ctx, ctr.ID, client.ContainerStartOptions{}); err != nil {
		_, _ = d.cl.ContainerRemove(
			ctx,
			ctr.ID,
			client.ContainerRemoveOptions{Force: true, RemoveVolumes: true},
		)

		return nil, err
	}

	return &Container{
		id: ctr.ID,
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
	_, err := c.d.cl.ContainerRemove(
		ctx,
		c.id,
		client.ContainerRemoveOptions{Force: true, RemoveVolumes: true},
	)
	return err
}

func (c *Container) CopyTo(ctx context.Context, dest string, src io.Reader) error {
	_, err := c.d.cl.CopyToContainer(ctx, c.id, client.CopyToContainerOptions{
		DestinationPath: dest,
		Content:         src,
	})
	return err
}

func (c *Container) Exec(
	ctx context.Context,
	stdout, stderr io.Writer,
	cmd ...string,
) (int, error) {
	exec, err := c.d.cl.ExecCreate(
		ctx,
		c.id,
		client.ExecCreateOptions{
			Cmd:          cmd,
			AttachStdout: true,
			AttachStderr: true,
		},
	)
	if err != nil {
		return -1, fmt.Errorf("failed to create exec in container: %w", err)
	}
	resp, err := c.d.cl.ExecAttach(ctx, exec.ID, client.ExecAttachOptions{})
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
	if _, err := c.d.cl.ExecStart(ctx, exec.ID, client.ExecStartOptions{}); err != nil {
		return -1, fmt.Errorf("failed to start exec in container: %w", err)
	}
	if err := <-errCh; err != nil {
		return -1, fmt.Errorf("failed to read exec streams: %w", err)
	}
	execInspect, err := c.d.cl.ExecInspect(ctx, exec.ID, client.ExecInspectOptions{})
	if err != nil {
		return -1, fmt.Errorf("failed to inspect exec in container: %w", err)
	}
	return execInspect.ExitCode, nil
}
