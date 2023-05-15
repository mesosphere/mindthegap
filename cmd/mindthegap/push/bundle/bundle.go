// Copyright 2021-2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"fmt"
	"os"

	"github.com/containers/image/v5/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cleanup"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/flags"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/utils"
	"github.com/mesosphere/mindthegap/config"
	"github.com/mesosphere/mindthegap/docker/ecr"
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/images/authnhelpers"
	"github.com/mesosphere/mindthegap/images/httputils"
)

func NewCommand(out output.Output, bundleCmdName string) *cobra.Command {
	var (
		bundleFiles                   []string
		destRegistryURI               flags.RegistryURI
		destRegistryCACertificateFile string
		destRegistrySkipTLSVerify     bool
		destRegistryUsername          string
		destRegistryPassword          string
		ecrLifecyclePolicy            string
	)

	cmd := &cobra.Command{
		Use:   bundleCmdName,
		Short: "Push from bundles into an existing OCI registry",
		RunE: func(cmd *cobra.Command, args []string) error {
			cleaner := cleanup.NewCleaner()
			defer cleaner.Cleanup()

			out.StartOperation("Creating temporary directory")
			tempDir, err := os.MkdirTemp("", ".bundle-*")
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to create temporary directory: %w", err)
			}
			cleaner.AddCleanupFn(func() { _ = os.RemoveAll(tempDir) })
			out.EndOperation(true)

			bundleFiles, err = utils.FilesWithGlobs(bundleFiles)
			if err != nil {
				return err
			}
			imagesCfg, chartsCfg, err := utils.ExtractBundles(tempDir, out, bundleFiles...)
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

			logs.Debug.SetOutput(out.V(4).InfoWriter())
			logs.Warn.SetOutput(out.InfoWriter())

			sourceTLSRoundTripper, err := httputils.InsecureTLSRoundTripper(remote.DefaultTransport)
			if err != nil {
				out.Error(err, "error configuring TLS for source registry")
				os.Exit(2)
			}
			sourceRemoteOpts := []remote.Option{remote.WithTransport(sourceTLSRoundTripper)}

			destTLSRoundTripper, err := httputils.TLSConfiguredRoundTripper(
				remote.DefaultTransport,
				destRegistryURI.Host(),
				flags.SkipTLSVerify(destRegistrySkipTLSVerify, destRegistryURI),
				destRegistryCACertificateFile,
			)
			if err != nil {
				out.Error(err, "error configuring TLS for destination registry")
				os.Exit(2)
			}
			destRemoteOpts := []remote.Option{remote.WithTransport(destTLSRoundTripper)}

			var destNameOpts []name.Option
			if flags.SkipTLSVerify(destRegistrySkipTLSVerify, destRegistryURI) {
				destNameOpts = append(destNameOpts, name.Insecure)
			}

			keychain := authn.DefaultKeychain
			if destRegistryUsername != "" && destRegistryPassword != "" {
				keychain = authn.NewMultiKeychain(
					authn.NewKeychainFromHelper(
						authnhelpers.NewStaticHelper(
							destRegistryURI.Host(),
							&types.DockerAuthConfig{
								Username: destRegistryUsername,
								Password: destRegistryPassword,
							},
						),
					),
					keychain,
				)
			}
			destRemoteOpts = append(destRemoteOpts, remote.WithAuthFromKeychain(keychain))

			// Determine type of destination registry.
			var prePushFuncs []prePushFunc
			if ecr.IsECRRegistry(destRegistryURI.Host()) {
				prePushFuncs = append(
					prePushFuncs,
					ecr.EnsureRepositoryExistsFunc(destRegistryURI.Host(), ecrLifecyclePolicy),
				)
			}

			if imagesCfg != nil {
				err := pushImages(
					*imagesCfg,
					reg.Address(),
					sourceRemoteOpts,
					destRegistryURI.Address(),
					destRemoteOpts,
					destNameOpts,
					out,
					prePushFuncs...,
				)
				if err != nil {
					return err
				}
			}

			if chartsCfg != nil {
				err := pushOCIArtifacts(
					*chartsCfg,
					fmt.Sprintf("%s/charts", reg.Address()),
					sourceRemoteOpts,
					destRegistryURI.Address(),
					destRemoteOpts,
					out,
					prePushFuncs...,
				)
				if err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&bundleFiles, bundleCmdName, nil,
		"Tarball containing list of images to push. Can also be a glob pattern.")
	_ = cmd.MarkFlagRequired(bundleCmdName)
	cmd.Flags().Var(&destRegistryURI, "to-registry", "Registry to push images to. "+
		"TLS verification will be skipped when using an http:// registry.")
	_ = cmd.MarkFlagRequired("to-registry")
	cmd.Flags().StringVar(&destRegistryCACertificateFile, "to-registry-ca-cert-file", "",
		"CA certificate file used to verify TLS verification of registry to push images to")
	cmd.Flags().BoolVar(&destRegistrySkipTLSVerify, "to-registry-insecure-skip-tls-verify", false,
		"Skip TLS verification of registry to push images to (also use for non-TLS http registries)")
	cmd.MarkFlagsMutuallyExclusive(
		"to-registry-ca-cert-file",
		"to-registry-insecure-skip-tls-verify",
	)
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
	sourceRegistry string, sourceRemoteOpts []remote.Option,
	destRegistry string, destRemoteOpts []remote.Option, destNameOpts []name.Option,
	out output.Output,
	prePushFuncs ...prePushFunc,
) error {
	// Sort registries for deterministic ordering.
	regNames := cfg.SortedRegistryNames()

	for _, registryName := range regNames {
		registryConfig := cfg[registryName]

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

				srcImage := fmt.Sprintf("%s/%s:%s", sourceRegistry, imageName, imageTag)
				srcRef, err := name.ParseReference(srcImage, name.Insecure, name.StrictValidation)
				if err != nil {
					out.EndOperation(false)
					return err
				}

				destImage := fmt.Sprintf("%s/%s:%s", destRegistry, imageName, imageTag)
				dstRef, err := name.ParseReference(
					destImage,
					append(destNameOpts, name.StrictValidation)...)
				if err != nil {
					out.EndOperation(false)
					return err
				}

				idx, err := remote.Index(srcRef, sourceRemoteOpts...)
				if err != nil {
					out.EndOperation(false)
					return err
				}

				if err := remote.WriteIndex(dstRef, idx, destRemoteOpts...); err != nil {
					out.EndOperation(false)
					return err
				}

				out.EndOperation(true)
			}
		}
	}

	return nil
}

func pushOCIArtifacts(
	cfg config.HelmChartsConfig,
	sourceRegistry string, sourceRemoteOpts []remote.Option,
	destRegistry string, destRemoteOpts []remote.Option,
	out output.Output,
	prePushFuncs ...prePushFunc,
) error {
	// Sort repositories for deterministic ordering.
	repoNames := cfg.SortedRepositoryNames()

	for _, repoName := range repoNames {
		repoConfig := cfg.Repositories[repoName]

		// Sort charts for deterministic ordering.
		chartNames := repoConfig.SortedChartNames()

		for _, chartName := range chartNames {
			chartVersions := repoConfig.Charts[chartName]

			for _, prePush := range prePushFuncs {
				if err := prePush("", destRegistry); err != nil {
					return fmt.Errorf("pre-push func failed: %w", err)
				}
			}

			for _, chartVersion := range chartVersions {
				out.StartOperation(
					fmt.Sprintf("Copying %s:%s (from bundle) to %s/%s:%s",
						chartName, chartVersion,
						destRegistry, chartName, chartVersion,
					),
				)

				srcChart := fmt.Sprintf("%s/%s:%s", sourceRegistry, chartName, chartVersion)
				srcChartRef, err := name.ParseReference(srcChart, name.StrictValidation)
				if err != nil {
					out.EndOperation(false)
					return err
				}
				src, err := remote.Image(srcChartRef, sourceRemoteOpts...)
				if err != nil {
					out.EndOperation(false)
					return err
				}

				destChart := fmt.Sprintf("%s/%s:%s", destRegistry, chartName, chartVersion)
				destChartRef, err := name.ParseReference(destChart, name.StrictValidation)
				if err != nil {
					out.EndOperation(false)
					return err
				}

				if err := remote.Write(destChartRef, src, destRemoteOpts...); err != nil {
					out.EndOperation(false)
					return err
				}

				out.EndOperation(true)
			}
		}
	}

	return nil
}
