// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helmbundle

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

func NewCommand(out output.Output) *cobra.Command {
	var (
		helmBundleFiles               []string
		destRegistryURI               flags.RegistryURI
		destRegistryCACertificateFile string
		destRegistrySkipTLSVerify     bool
		destRegistryUsername          string
		destRegistryPassword          string
	)

	cmd := &cobra.Command{
		Use:   "helm-bundle",
		Short: "Push images from a Helm chart bundle into an existing OCI registry",
		RunE: func(cmd *cobra.Command, args []string) error {
			cleaner := cleanup.NewCleaner()
			defer cleaner.Cleanup()

			out.StartOperation("Creating temporary directory")
			tempDir, err := os.MkdirTemp("", ".helm-bundle-*")
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to create temporary directory: %w", err)
			}
			cleaner.AddCleanupFn(func() { _ = os.RemoveAll(tempDir) })
			out.EndOperation(true)

			helmBundleFiles, err = utils.FilesWithGlobs(helmBundleFiles)
			if err != nil {
				return err
			}
			_, cfg, err := utils.ExtractBundles(tempDir, out, helmBundleFiles...)
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
			remoteOpts := []remote.Option{remote.WithTransport(destTLSRoundTripper)}

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

			remoteOpts = append(remoteOpts, remote.WithAuthFromKeychain(keychain))

			// Determine type of destination registry.
			var prePushFuncs []prePushFunc
			if ecr.IsECRRegistry(destRegistryURI.Host()) {
				prePushFuncs = append(
					prePushFuncs,
					ecr.EnsureRepositoryExistsFunc(destRegistryURI.Host(), ""),
				)
			}

			return pushOCIArtifacts(
				cfg,
				fmt.Sprintf("%s/charts", reg.Address()),
				destRegistryURI.Address(),
				remoteOpts,
				out,
				prePushFuncs...,
			)
		},
	}

	cmd.Flags().StringSliceVar(&helmBundleFiles, "helm-bundle", nil,
		"Tarball containing list of Helm charts to push. Can also be a glob pattern.")
	_ = cmd.MarkFlagRequired("helm-bundle")
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
		"Username to use to log in to destination repository")
	cmd.Flags().StringVar(&destRegistryPassword, "to-registry-password", "",
		"Password to use to log in to destination registry")

	// TODO Unhide this from DKP CLI once DKP supports OCI registry for Helm charts.
	utils.AddCmdAnnotation(cmd, "exclude-from-dkp-cli", "true")

	return cmd
}

type prePushFunc func(destRegistry, imageName string, imageTags ...string) error

func pushOCIArtifacts(
	cfg config.HelmChartsConfig,
	sourceRegistry, destRegistry string,
	remoteOpts []remote.Option,
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
				src, err := remote.Image(srcChartRef)
				if err != nil {
					out.EndOperation(false)
					return err
				}

				destChart := fmt.Sprintf("%s/%s:%s", destRegistry, chartName, chartVersion)
				fmt.Println(destChart)
				destChartRef, err := name.ParseReference(destChart, name.StrictValidation)
				if err != nil {
					out.EndOperation(false)
					return err
				}

				if err := remote.Write(destChartRef, src, remoteOpts...); err != nil {
					out.EndOperation(false)
					return err
				}

				out.EndOperation(true)
			}
		}
	}

	return nil
}
