// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imagebundle

import (
	"fmt"
	"os"

	"github.com/mesosphere/dkp-cli/runtime/cli"
	"github.com/mesosphere/dkp-cli/runtime/cmd/log"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/mesosphere/mindthegap/docker/registry"
)

func NewCommand(ioStreams genericclioptions.IOStreams) *cobra.Command {
	var (
		imageBundleFile string
		listenAddress   string
		listenPort      uint16
	)

	cmd := &cobra.Command{
		Use: "image-bundle",
		RunE: func(cmd *cobra.Command, args []string) error {
			klog.SetOutput(ioStreams.ErrOut)
			logger := log.NewLogger(ioStreams.ErrOut)
			statusLogger := cli.StatusForLogger(logger)

			statusLogger.Start("Creating temporary directory")
			tempDir, err := os.MkdirTemp("", ".image-bundle-*")
			if err != nil {
				statusLogger.End(false)
				return fmt.Errorf("failed to create temporary directory: %w", err)
			}
			defer os.RemoveAll(tempDir)
			statusLogger.End(true)

			statusLogger.Start("Unarchiving image bundle")
			err = archiver.Unarchive(imageBundleFile, tempDir)
			if err != nil {
				statusLogger.End(false)
				return fmt.Errorf("failed to unarchive image bundle: %w", err)
			}
			statusLogger.End(true)

			statusLogger.Start("Creating temporary Docker registry")
			reg, err := registry.NewRegistry(registry.Config{
				StorageDirectory: tempDir,
				ReadOnly:         true,
				Host:             listenAddress,
				Port:             listenPort,
			})
			if err != nil {
				statusLogger.End(false)
				return fmt.Errorf("failed to create local Docker registry: %w", err)
			}
			statusLogger.End(true)
			fmt.Fprintf(ioStreams.Out, "Listening on %s\n", reg.Address())
			if err := reg.ListenAndServe(); err != nil {
				statusLogger.End(false)
				fmt.Fprintf(ioStreams.ErrOut, "error serving Docker registry: %v\n", err)
				os.Exit(2)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&imageBundleFile, "image-bundle", "", "Tarball containing list of images to push")
	_ = cmd.MarkFlagRequired("image-bundle")
	cmd.Flags().StringVar(&listenAddress, "listen-address", "localhost", "Address to list on")
	cmd.Flags().Uint16Var(&listenPort, "listen-port", 0, "Port to listen on (0 means use any free port)")

	return cmd
}
