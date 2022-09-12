// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helmbundle

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cleanup"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/utils"
	"github.com/mesosphere/mindthegap/config"
	"github.com/mesosphere/mindthegap/docker/ecr"
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/skopeo"
)

func NewCommand(out output.Output) *cobra.Command {
	var (
		helmBundleFiles           []string
		destRepository            string
		destRegistrySkipTLSVerify bool
		destRepositoryUsername    string
		destRepositoryPassword    string
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

			skopeoRunner, skopeoCleanup := skopeo.NewRunner()
			cleaner.AddCleanupFn(func() { _ = skopeoCleanup() })

			skopeoOpts := []skopeo.SkopeoOption{
				skopeo.PreserveDigests(),
			}
			if destRepositoryUsername != "" && destRepositoryPassword != "" {
				skopeoOpts = append(
					skopeoOpts,
					skopeo.DestCredentials(
						destRepositoryUsername,
						destRepositoryPassword,
					),
				)
			} else {
				skopeoStdout, skopeoStderr, err := skopeoRunner.AttemptToLoginToRegistry(
					context.Background(),
					destRepository,
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
			if ecr.IsECRRegistry(destRepository) {
				prePushFuncs = append(
					prePushFuncs,
					ecr.EnsureRepositoryExistsFunc(""),
				)
			}

			return pushOCIArtifacts(
				cfg,
				fmt.Sprintf("%s/charts", reg.Address()),
				destRepository,
				skopeoOpts,
				destRegistrySkipTLSVerify,
				out,
				skopeoRunner,
				prePushFuncs...,
			)
		},
	}

	cmd.Flags().StringSliceVar(&helmBundleFiles, "helm-bundle", nil,
		"Tarball containing list of Helm charts to push. Can also be a glob pattern.")
	_ = cmd.MarkFlagRequired("helm-bundle")
	cmd.Flags().StringVar(&destRepository, "to-repository", "", "Repository to push images to")
	_ = cmd.MarkFlagRequired("to-repository")
	cmd.Flags().BoolVar(&destRegistrySkipTLSVerify, "to-repository-insecure-skip-tls-verify", false,
		"Skip TLS verification of repository to push images to (use for http repositories)")
	cmd.Flags().StringVar(&destRepositoryUsername, "to-repository-username", "",
		"Username to use to log in to destination repository")
	cmd.Flags().StringVar(&destRepositoryPassword, "to-repository-password", "",
		"Password to use to log in to destination repository")

	return cmd
}

type prePushFunc func(destRegistry, imageName string, imageTags ...string) error

func pushOCIArtifacts(
	cfg config.HelmChartsConfig,
	sourceRepository, destRepository string,
	skopeoOpts []skopeo.SkopeoOption,
	destRegistrySkipTLSVerify bool,
	out output.Output,
	skopeoRunner *skopeo.Runner,
	prePushFuncs ...prePushFunc,
) error {
	skopeoOpts = append(skopeoOpts, skopeo.DisableSrcTLSVerify())
	if destRegistrySkipTLSVerify {
		skopeoOpts = append(skopeoOpts, skopeo.DisableDestTLSVerify())
	}

	// Sort repositories for deterministic ordering.
	repoNames := cfg.SortedRepositoryNames()

	for _, repoName := range repoNames {
		repoConfig := cfg.Repositories[repoName]

		// Sort charts for deterministic ordering.
		chartNames := repoConfig.SortedChartNames()

		for _, chartName := range chartNames {
			chartVersions := repoConfig.Charts[chartName]

			for _, prePush := range prePushFuncs {
				if err := prePush("", destRepository); err != nil {
					return fmt.Errorf("pre-push func failed: %w", err)
				}
			}

			for _, chartVersion := range chartVersions {
				out.StartOperation(
					fmt.Sprintf("Copying %s:%s (from bundle) to %s/%s:%s",
						chartName, chartVersion,
						destRepository, chartName, chartVersion,
					),
				)
				skopeoStdout, skopeoStderr, err := skopeoRunner.Copy(context.TODO(),
					fmt.Sprintf("docker://%s/%s:%s", sourceRepository, chartName, chartVersion),
					fmt.Sprintf("docker://%s/%s:%s", destRepository, chartName, chartVersion),
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
