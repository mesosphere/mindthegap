// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imagebundle

import (
	"context"
	"errors"
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
	"github.com/mesosphere/mindthegap/cmd/mindthegap/flags"
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
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.ValidateRequiredFlags(); err != nil {
				return err
			}

			if err := flags.ValidateFlagsThatRequireValues(cmd, "image-bundle"); err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cleaner := cleanup.NewCleaner()
			defer cleaner.Cleanup()

			out.StartOperation("Creating temporary directory")
			tempDir, err := os.MkdirTemp("", ".image-bundle-*")
			if err != nil {
				out.EndOperationWithStatus(output.Failure())
				return fmt.Errorf("failed to create temporary directory: %w", err)
			}
			cleaner.AddCleanupFn(func() { _ = os.RemoveAll(tempDir) })
			out.EndOperationWithStatus(output.Success())

			imageBundleFiles, err = utils.FilesWithGlobs(imageBundleFiles)
			if err != nil {
				return err
			}
			cfg, _, err := utils.ExtractConfigs(tempDir, out, imageBundleFiles...)
			if err != nil {
				return err
			}
			if cfg == nil {
				return errors.New(
					"no bundle configuration(s) found: please check that you have specified valid air-gapped bundle(s)",
				)
			}

			out.StartOperation("Starting temporary Docker registry")
			storage, err := registry.ArchiveStorage("", imageBundleFiles...)
			if err != nil {
				out.EndOperationWithStatus(output.Failure())
				return fmt.Errorf("failed to create storage for Docker registry from supplied bundles: %w", err)
			}

			reg, err := registry.NewRegistry(
				registry.Config{Storage: storage},
			)
			if err != nil {
				out.EndOperationWithStatus(output.Failure())
				return fmt.Errorf("failed to create local Docker registry: %w", err)
			}
			go func() {
				if err := reg.ListenAndServe(output.NewOutputLogr(out)); err != nil {
					out.Error(err, "error serving Docker registry")
					os.Exit(2)
				}
			}()
			out.EndOperationWithStatus(output.Success())

			ociExportsTempDir, err := os.MkdirTemp("", ".oci-exports-*")
			if err != nil {
				return fmt.Errorf("failed to create temporary directory for OCI exports: %w", err)
			}
			cleaner.AddCleanupFn(func() { _ = os.RemoveAll(ociExportsTempDir) })

			sourceTLSRoundTripper, err := httputils.InsecureTLSRoundTripper(remote.DefaultTransport)
			if err != nil {
				out.Error(err, "error configuring TLS for source registry")
				os.Exit(2)
			}

			// Import the images from the merged bundle config.
			for registryName, registryConfig := range *cfg {
				for imageName, imageTags := range registryConfig.Images {
					for _, imageTag := range imageTags {
						srcImageName := fmt.Sprintf("%s/%s:%s", reg.Address(), imageName, imageTag)
						destImageName := fmt.Sprintf("%s/%s:%s", registryName, imageName, imageTag)

						out.StartOperation(fmt.Sprintf("Importing %s", destImageName))

						ref, err := name.ParseReference(srcImageName, name.StrictValidation)
						if err != nil {
							out.EndOperationWithStatus(output.Failure())
							return err
						}

						v1Image, err := remote.Image(
							ref,
							remote.WithTransport(sourceTLSRoundTripper),
							remote.WithPlatform(
								v1.Platform{OS: runtime.GOOS, Architecture: runtime.GOARCH},
							),
						)
						if err != nil {
							out.EndOperationWithStatus(output.Failure())
							return err
						}

						tag, err := name.NewTag(destImageName, name.StrictValidation)
						if err != nil {
							out.EndOperationWithStatus(output.Failure())
							return err
						}

						exportTarball := filepath.Join(ociExportsTempDir, "docker-archive.tar")

						if err := tarball.MultiWriteToFile(exportTarball, map[name.Tag]v1.Image{tag: v1Image}); err != nil {
							out.EndOperationWithStatus(output.Failure())
							return err
						}

						ctrOutput, err := containerd.ImportImageArchive(
							context.TODO(), exportTarball, containerdNamespace,
						)
						if err != nil {
							out.Warn(string(ctrOutput))
							out.EndOperationWithStatus(output.Failure())
							return err
						}

						out.V(4).Info(string(ctrOutput))

						_ = os.Remove(exportTarball)

						out.EndOperationWithStatus(output.Success())
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
