// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imagebundle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mesosphere/dkp-cli-runtime/core/output"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"

	"github.com/mesosphere/mindthegap/cleanup"
	"github.com/mesosphere/mindthegap/config"
	"github.com/mesosphere/mindthegap/containerd"
	"github.com/mesosphere/mindthegap/docker/registry"
)

func NewCommand(out output.Output) *cobra.Command {
	var (
		imageBundleFile     string
		containerdNamespace string
	)

	cmd := &cobra.Command{
		Use: "image-bundle",
		RunE: func(cmd *cobra.Command, args []string) error {
			cleaner := cleanup.NewCleaner()
			defer cleaner.Cleanup()

			out.StartOperation("Creating temporary directory")
			tempDir, err := os.MkdirTemp("", ".image-bundle-*")
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to create temporary directory: %w", err)
			}
			cleaner.AddCleanupFn(func() { _ = os.RemoveAll(tempDir) })
			out.EndOperation(true)

			out.StartOperation("Unarchiving image bundle")
			err = archiver.Unarchive(imageBundleFile, tempDir)
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to unarchive image bundle: %w", err)
			}
			out.EndOperation(true)

			out.StartOperation("Parsing image bundle config")
			cfg, err := config.ParseImagesConfigFile(filepath.Join(tempDir, "images.yaml"))
			if err != nil {
				out.EndOperation(false)
				return err
			}
			out.V(4).Infof("Images config: %+v", cfg)
			out.EndOperation(true)

			out.StartOperation("Starting temporary Docker registry")
			reg, err := registry.NewRegistry(
				registry.Config{StorageDirectory: tempDir, ReadOnly: true},
			)
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to create local Docker registry: %w", err)
			}
			go func() {
				if err := reg.ListenAndServe(); err != nil {
					out.Error(err, "error serving Docker registry")
					os.Exit(2)
				}
			}()
			out.EndOperation(true)

			for registryName, registryConfig := range cfg {
				for imageName, imageTags := range registryConfig.Images {
					for _, imageTag := range imageTags {
						srcImageName := fmt.Sprintf("%s/%s:%s", reg.Address(), imageName, imageTag)
						destImageName := fmt.Sprintf("%s/%s:%s", registryName, imageName, imageTag)
						out.StartOperation(fmt.Sprintf("Importing %s", destImageName))
						ctrOutput, err := containerd.ImportImage(
							context.TODO(), srcImageName, destImageName, containerdNamespace,
						)
						if err != nil {
							out.Info(string(ctrOutput))
							out.EndOperation(false)
							return err
						}
						out.V(4).Info(string(ctrOutput))
						out.EndOperation(true)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().
		StringVar(&imageBundleFile, "image-bundle", "", "Tarball containing list of images to push")
	_ = cmd.MarkFlagRequired("image-bundle")
	cmd.Flags().StringVar(&containerdNamespace, "containerd-namespace", "k8s.io",
		"Containerd namespace to import images into")

	return cmd
}
