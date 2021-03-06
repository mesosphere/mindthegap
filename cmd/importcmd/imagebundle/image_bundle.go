// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imagebundle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cleanup"
	"github.com/mesosphere/mindthegap/config"
	"github.com/mesosphere/mindthegap/containerd"
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/skopeo"
)

func NewCommand(out output.Output) *cobra.Command {
	var (
		imageBundleFile     string
		containerdNamespace string
	)

	cmd := &cobra.Command{
		Use:   "image-bundle",
		Short: "Import images from an image bundle into Containerd",
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

			skopeoRunner, skopeoCleanup := skopeo.NewRunner()
			cleaner.AddCleanupFn(func() { _ = skopeoCleanup() })

			ociExportsTempDir, err := os.MkdirTemp("", ".oci-exports-*")
			if err != nil {
				return fmt.Errorf("failed to create temporary directory for OCI exports: %w", err)
			}
			cleaner.AddCleanupFn(func() { _ = os.RemoveAll(ociExportsTempDir) })

			for registryName, registryConfig := range cfg {
				for imageName, imageTags := range registryConfig.Images {
					for _, imageTag := range imageTags {
						srcImageName := fmt.Sprintf("%s/%s:%s", reg.Address(), imageName, imageTag)
						destImageName := fmt.Sprintf("%s/%s:%s", registryName, imageName, imageTag)

						out.StartOperation(fmt.Sprintf("Importing %s", destImageName))

						exportTarball := filepath.Join(ociExportsTempDir, "oci-export.tar")

						skopeoStdout, skopeoStderr, err := skopeoRunner.Copy(context.TODO(),
							fmt.Sprintf("docker://%s", srcImageName),
							fmt.Sprintf("oci-archive:%s:%s", exportTarball, destImageName),
							skopeo.All(),
							skopeo.DisableSrcTLSVerify(),
						)
						if err != nil {
							out.EndOperation(false)
							out.Infof("---skopeo stdout---:\n%s", skopeoStdout)
							out.Infof("---skopeo stderr---:\n%s", skopeoStderr)
							return err
						}
						out.V(4).Infof("---skopeo stdout---:\n%s", skopeoStdout)
						out.V(4).Infof("---skopeo stderr---:\n%s", skopeoStderr)

						ctrOutput, err := containerd.ImportImageArchive(
							context.TODO(), exportTarball, containerdNamespace,
						)
						if err != nil {
							out.Info(string(ctrOutput))
							out.EndOperation(false)
							return err
						}
						out.V(4).Info(string(ctrOutput))

						_ = os.Remove(exportTarball)

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
