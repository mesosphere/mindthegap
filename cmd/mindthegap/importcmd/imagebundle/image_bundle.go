// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imagebundle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cleanup"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/utils"
	"github.com/mesosphere/mindthegap/containerd"
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/images/httputils"
)

func NewCommand(out output.Output) *cobra.Command {
	var (
		imageBundleFiles    []string
		containerdNamespace string
	)

	cmd := &cobra.Command{
		Use:   "image-bundle",
		Short: "Import images from image bundles into Containerd",
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

			imageBundleFiles, err = utils.FilesWithGlobs(imageBundleFiles)
			if err != nil {
				return err
			}
			cfg, _, err := utils.ExtractBundles(tempDir, out, imageBundleFiles...)
			if err != nil {
				return err
			}

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

			ociExportsTempDir, err := os.MkdirTemp("", ".oci-exports-*")
			if err != nil {
				return fmt.Errorf("failed to create temporary directory for OCI exports: %w", err)
			}
			cleaner.AddCleanupFn(func() { _ = os.RemoveAll(ociExportsTempDir) })

			// Import the images from the merged bundle config.
			for registryName, registryConfig := range cfg {
				for imageName, imageTags := range registryConfig.Images {
					for _, imageTag := range imageTags {
						srcImageName := fmt.Sprintf("%s/%s:%s", reg.Address(), imageName, imageTag)
						destImageName := fmt.Sprintf("%s/%s:%s", registryName, imageName, imageTag)

						out.StartOperation(fmt.Sprintf("Importing %s", destImageName))

						ref, err := name.ParseReference(srcImageName, name.StrictValidation)
						if err != nil {
							out.EndOperation(false)
							return err
						}

						v1Image, err := remote.Image(
							ref,
							remote.WithTransport(
								httputils.NewConfigurableTLSRoundTripper(
									httputils.TLSHostsConfig{
										reg.Address(): httputils.TLSHostConfig{Insecure: true},
									},
								),
							),
							remote.WithPlatform(
								v1.Platform{OS: runtime.GOOS, Architecture: runtime.GOARCH},
							),
						)
						if err != nil {
							out.EndOperation(false)
							return err
						}

						tag, err := name.NewTag(destImageName, name.StrictValidation)
						if err != nil {
							out.EndOperation(false)
							return err
						}

						exportTarball := filepath.Join(ociExportsTempDir, "docker-archive.tar")

						if err := tarball.MultiWriteToFile(exportTarball, map[name.Tag]v1.Image{tag: v1Image}); err != nil {
							out.EndOperation(false)
							return err
						}

						ctrOutput, err := containerd.ImportImageArchive(
							context.TODO(), exportTarball, containerdNamespace,
						)
						if err != nil {
							out.Warn(string(ctrOutput))
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

	cmd.Flags().StringSliceVar(&imageBundleFiles, "image-bundle", nil,
		"Tarball containing list of images to import. Can also be a glob pattern.")
	_ = cmd.MarkFlagRequired("image-bundle")
	cmd.Flags().StringVar(&containerdNamespace, "containerd-namespace", "k8s.io",
		"Containerd namespace to import images into")

	return cmd
}
