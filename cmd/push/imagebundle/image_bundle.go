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
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/skopeo"
)

func NewCommand(out output.Output) *cobra.Command {
	var (
		imageBundleFile           string
		destRegistry              string
		destRegistrySkipTLSVerify bool
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

			skopeoRunner, skopeoCleanup := skopeo.NewRunner()
			cleaner.AddCleanupFn(func() { _ = skopeoCleanup() })

			skopeoStdout, skopeoStderr, err := skopeoRunner.AttemptToLoginToRegistry(
				context.TODO(),
				destRegistry,
			)
			if err != nil {
				out.Infof("---skopeo stdout---:\n%s", skopeoStdout)
				out.Infof("---skopeo stderr---:\n%s", skopeoStderr)
				return fmt.Errorf("error logging in to target registry: %w", err)
			}

			for _, registryConfig := range cfg {
				skopeoOpts := []skopeo.SkopeoOption{skopeo.DisableSrcTLSVerify()}
				if destRegistrySkipTLSVerify {
					skopeoOpts = append(skopeoOpts, skopeo.DisableDestTLSVerify())
				}
				for imageName, imageTags := range registryConfig.Images {
					for _, imageTag := range imageTags {
						out.StartOperation(
							fmt.Sprintf("Copying %s/%s:%s to %s/%s:%s",
								reg.Address(), imageName, imageTag,
								destRegistry, imageName, imageTag,
							),
						)
						skopeoStdout, skopeoStderr, err := skopeoRunner.Copy(context.TODO(),
							fmt.Sprintf("docker://%s/%s:%s", reg.Address(), imageName, imageTag),
							fmt.Sprintf("docker://%s/%s:%s", destRegistry, imageName, imageTag),
							append(
								skopeoOpts, skopeo.All(),
							)...,
						)
						if err != nil {
							out.EndOperation(false)
							out.Infof("---skopeo stdout---:\n%s", skopeoStdout)
							out.Infof("---skopeo stderr---:\n%s", skopeoStderr)
							return err
						}
						out.V(4).Infof("---skopeo stdout---:\n%s", skopeoStdout)
						out.V(4).Infof("---skopeo stderr---:\n%s", skopeoStderr)
						out.EndOperation(true)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().
		StringVar(&imageBundleFile, "image-bundle", "", "Tarball of images to push")
	_ = cmd.MarkFlagRequired("image-bundle")
	cmd.Flags().StringVar(&destRegistry, "to-registry", "", "Registry to push images to")
	_ = cmd.MarkFlagRequired("to-registry")
	cmd.Flags().BoolVar(&destRegistrySkipTLSVerify, "to-registry-insecure-skip-tls-verify", false,
		"Skip TLS verification of registry to push images to (use for http registries)")

	return cmd
}
