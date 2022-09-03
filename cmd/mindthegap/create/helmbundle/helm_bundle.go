// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helmbundle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/utils/pointer"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/archive"
	"github.com/mesosphere/mindthegap/cleanup"
	"github.com/mesosphere/mindthegap/config"
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/helm"
)

func NewCommand(out output.Output) *cobra.Command {
	var (
		configFile string
		outputFile string
		overwrite  bool
	)

	cmd := &cobra.Command{
		Use:   "helm-bundle",
		Short: "Create a helm bundle",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !overwrite {
				out.StartOperation("Checking if output file already exists")
				_, err := os.Stat(outputFile)
				switch {
				case err == nil:
					out.EndOperation(false)
					return fmt.Errorf(
						"%s already exists: specify --overwrite to overwrite existing file",
						outputFile,
					)
				case !errors.Is(err, os.ErrNotExist):
					out.EndOperation(false)
					return fmt.Errorf(
						"failed to check if output file %s already exists: %w",
						outputFile,
						err,
					)
				default:
					out.EndOperation(true)
				}
			}

			out.StartOperation("Parsing helm bundle config")
			cfg, err := config.ParseHelmChartsConfigFile(configFile)
			if err != nil {
				out.EndOperation(false)
				return err
			}
			out.EndOperation(true)
			out.V(4).Infof("Helm charts config: %+v", cfg)

			configFileAbs, err := filepath.Abs(configFile)
			if err != nil {
				return err
			}

			out.StartOperation("Creating temporary directory")
			outputFileAbs, err := filepath.Abs(outputFile)
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf(
					"failed to determine where to create temporary directory: %w",
					err,
				)
			}

			cleaner := cleanup.NewCleaner()
			defer cleaner.Cleanup()

			tempDir, err := os.MkdirTemp(filepath.Dir(outputFileAbs), ".helm-bundle-*")
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to create temporary directory: %w", err)
			}
			cleaner.AddCleanupFn(func() { _ = os.RemoveAll(tempDir) })
			out.EndOperation(true)

			out.StartOperation("Starting temporary OCI registry")
			reg, err := registry.NewRegistry(registry.Config{StorageDirectory: tempDir})
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to create local OCI registry: %w", err)
			}
			go func() {
				if err := reg.ListenAndServe(); err != nil {
					out.Error(err, "error serving OCI registry")
					os.Exit(2)
				}
			}()
			out.EndOperation(true)

			helmClient, helmCleanup := helm.NewClient(out)
			cleaner.AddCleanupFn(func() { _ = helmCleanup() })

			for repoName, repoConfig := range cfg.Repositories {
				for chartName, chartVersions := range repoConfig.Charts {
					out.StartOperation(
						fmt.Sprintf(
							"Fetching Helm chart %s (versions %v) from %s (%s)",
							chartName,
							chartVersions,
							repoName,
							repoConfig.RepoURL,
						),
					)
					var opts []action.PullOpt
					if repoConfig.Username != "" {
						opts = append(
							opts,
							helm.UsernamePasswordOpt(repoConfig.Username, repoConfig.Password),
						)
					}
					if !pointer.BoolDeref(repoConfig.TLSVerify, true) {
						opts = append(opts, helm.InsecureSkipTLSverifyOpt())
					}
					for _, chartVersion := range chartVersions {
						if err := helmClient.GetChartFromRepo(tempDir, repoConfig.RepoURL, chartName, chartVersion, opts...); err != nil {
							out.EndOperation(false)
							return fmt.Errorf("failed to create Helm chart bundle: %v", err)
						}
					}
					out.EndOperation(true)
				}
			}
			for _, chartURL := range cfg.ChartURLs {
				out.StartOperation(fmt.Sprintf("Fetching Helm chart from URL %s", chartURL))
				if err := helmClient.GetChartFromURL(tempDir, chartURL, filepath.Dir(configFileAbs)); err != nil {
					out.EndOperation(false)
					return fmt.Errorf("failed to create Helm chart bundle: %v", err)
				}
				out.EndOperation(true)
			}

			out.StartOperation("Creating Helm repository index")
			if err := helmClient.CreateHelmRepoIndex(tempDir); err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to create Helm chart bundle: %v", err)
			}
			out.EndOperation(true)

			if err := config.WriteHelmChartsConfig(cfg, filepath.Join(tempDir, "charts.yaml")); err != nil {
				return err
			}

			out.StartOperation(fmt.Sprintf("Archiving Helm charts to %s", outputFile))
			if err := archive.ArchiveDirectory(tempDir, outputFile); err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to create Helm charts bundle tarball: %w", err)
			}
			out.EndOperation(true)

			return nil
		},
	}

	cmd.Flags().StringVar(&configFile, "helm-charts-file", "",
		"YAML file containing configuration of Helm charts to create bundle from")
	_ = cmd.MarkFlagRequired("helm-charts-file")
	cmd.Flags().
		StringVar(&outputFile, "output-file", "helm-charts.tar", "Output file to write Helm charts bundle to")
	cmd.Flags().
		BoolVar(&overwrite, "overwrite", false, "Overwrite Helm charts bundle file if it already exists")

	return cmd
}
