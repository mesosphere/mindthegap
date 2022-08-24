// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imagebundle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/archive"
	"github.com/mesosphere/mindthegap/cleanup"
	"github.com/mesosphere/mindthegap/config"
	"github.com/mesosphere/mindthegap/docker/ecr"
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/skopeo"
)

func NewCommand(out output.Output) *cobra.Command {
	var (
		imageBundleFiles          []string
		destRegistry              string
		destRegistrySkipTLSVerify bool
		destRegistryUsername      string
		destRegistryPassword      string
		ecrLifecyclePolicy        string
	)

	cmd := &cobra.Command{
		Use:   "image-bundle",
		Short: "Push images from an image bundle into an existing image registry",
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

			sort.Strings(imageBundleFiles)

			var cfg config.ImagesConfig

			// Just in case users specify the same bundle twice, keep a track of
			// files that have been extracted already so we only extract each of them once.
			extractedBundles := make(map[string]struct{}, len(imageBundleFiles))

			for _, imageBundleFile := range imageBundleFiles {
				if _, ok := extractedBundles[imageBundleFile]; ok {
					continue
				}
				extractedBundles[imageBundleFile] = struct{}{}

				out.StartOperation(fmt.Sprintf("Unarchiving image bundle %q", imageBundleFile))
				err = archive.UnarchiveToDirectory(imageBundleFile, tempDir)
				if err != nil {
					out.EndOperation(false)
					return fmt.Errorf("failed to unarchive image bundle: %w", err)
				}
				out.EndOperation(true)

				out.StartOperation("Parsing image bundle config")
				bundleCfg, err := config.ParseImagesConfigFile(
					filepath.Join(tempDir, "images.yaml"),
				)
				if err != nil {
					out.EndOperation(false)
					return err
				}
				out.V(4).Infof("Images config: %+v", bundleCfg)
				out.EndOperation(true)

				cfg = cfg.Merge(bundleCfg)
			}

			out.V(4).Infof("Merged images config: %+v", cfg)

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

			skopeoOpts := []skopeo.SkopeoOption{
				skopeo.PreserveDigests(),
			}
			if destRegistryUsername != "" && destRegistryPassword != "" {
				skopeoOpts = append(
					skopeoOpts,
					skopeo.DestCredentials(
						destRegistryUsername,
						destRegistryPassword,
					),
				)
			} else {
				skopeoStdout, skopeoStderr, err := skopeoRunner.AttemptToLoginToRegistry(
					context.Background(),
					destRegistry,
				)
				if err != nil {
					out.Infof("---skopeo stdout---:\n%s", skopeoStdout)
					out.Infof("---skopeo stderr---:\n%s", skopeoStderr)
					return fmt.Errorf("error logging in to target registry: %w", err)
				}
				out.V(4).Infof("---skopeo stdout---:\n%s", skopeoStdout)
				out.V(4).Infof("---skopeo stderr---:\n%s", skopeoStderr)
			}

			// Determine type of destination registry.
			var prePushFuncs []prePushFunc
			if ecr.IsECRRegistry(destRegistry) {
				prePushFuncs = append(
					prePushFuncs,
					ecr.EnsureRepositoryExistsFunc(ecrLifecyclePolicy),
				)
			}

			return pushImages(
				cfg,
				reg.Address(),
				destRegistry,
				skopeoOpts,
				destRegistrySkipTLSVerify,
				out,
				skopeoRunner,
				prePushFuncs...,
			)
		},
	}

	cmd.Flags().
		StringSliceVar(&imageBundleFiles, "image-bundle", nil, "Tarball containing list of images to push")
	_ = cmd.MarkFlagRequired("image-bundle")
	cmd.Flags().StringVar(&destRegistry, "to-registry", "", "Registry to push images to")
	_ = cmd.MarkFlagRequired("to-registry")
	cmd.Flags().BoolVar(&destRegistrySkipTLSVerify, "to-registry-insecure-skip-tls-verify", false,
		"Skip TLS verification of registry to push images to (use for http registries)")
	cmd.Flags().StringVar(&destRegistryUsername, "to-registry-username", "",
		"Username to use to log in to destination registry")
	cmd.Flags().StringVar(&destRegistryPassword, "to-registry-password", "",
		"Password to use to log in to destination registry")
	cmd.Flags().StringVar(&ecrLifecyclePolicy, "ecr-lifecycle-policy-file", "",
		"File containing ECR lifecycle policy for newly created repositories "+
			"(only applies if target registry is hosted on ECR, ignored otherwise)")

	return cmd
}

type prePushFunc func(destRegistry, imageName string, imageTags ...string) error

func pushImages(
	cfg config.ImagesConfig,
	sourceRegistry, destRegistry string,
	skopeoOpts []skopeo.SkopeoOption,
	destRegistrySkipTLSVerify bool,
	out output.Output,
	skopeoRunner *skopeo.Runner,
	prePushFuncs ...prePushFunc,
) error {
	// Sort registries for deterministic ordering.
	regNames := cfg.SortedRegistryNames()

	for _, registryName := range regNames {
		registryConfig := cfg[registryName]
		skopeoOpts = append(skopeoOpts, skopeo.DisableSrcTLSVerify())
		if destRegistrySkipTLSVerify {
			skopeoOpts = append(skopeoOpts, skopeo.DisableDestTLSVerify())
		}

		// Sort images for deterministic ordering.
		imageNames := registryConfig.SortedImageNames()

		for _, imageName := range imageNames {
			imageTags := registryConfig.Images[imageName]

			for _, prePush := range prePushFuncs {
				if err := prePush(destRegistry, imageName, imageTags...); err != nil {
					return fmt.Errorf("pre-push func failed: %w", err)
				}
			}

			for _, imageTag := range imageTags {
				out.StartOperation(
					fmt.Sprintf("Copying %s/%s:%s (from bundle) to %s/%s:%s",
						registryName, imageName, imageTag,
						destRegistry, imageName, imageTag,
					),
				)
				skopeoStdout, skopeoStderr, err := skopeoRunner.Copy(context.TODO(),
					fmt.Sprintf("docker://%s/%s:%s", sourceRegistry, imageName, imageTag),
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
}
