// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helmbundle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/utils/ptr"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/archive"
	"github.com/mesosphere/mindthegap/cleanup"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/flags"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/utils"
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
		Short: "Create a Helm chart bundle",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.ValidateRequiredFlags(); err != nil {
				return err
			}

			if err := flags.ValidateFlagsThatRequireValues(cmd, "helm-charts-file"); err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !overwrite {
				out.StartOperation("Checking if output file already exists")
				_, err := os.Stat(outputFile)
				switch {
				case err == nil:
					out.EndOperationWithStatus(output.Failure())
					return fmt.Errorf(
						"%s already exists: specify --overwrite to overwrite existing file",
						outputFile,
					)
				case !errors.Is(err, os.ErrNotExist):
					out.EndOperationWithStatus(output.Failure())
					return fmt.Errorf(
						"failed to check if output file %s already exists: %w",
						outputFile,
						err,
					)
				default:
					out.EndOperationWithStatus(output.Success())
				}
			}

			out.StartOperation("Parsing Helm chart bundle config")
			cfg, err := config.ParseHelmChartsConfigFile(configFile)
			if err != nil {
				out.EndOperationWithStatus(output.Failure())
				return err
			}
			out.EndOperationWithStatus(output.Success())
			out.V(4).Infof("Helm charts config: %+v", cfg)

			configFileAbs, err := filepath.Abs(configFile)
			if err != nil {
				return err
			}

			out.StartOperation("Creating temporary OCI registry directory")
			outputFileAbs, err := filepath.Abs(outputFile)
			if err != nil {
				out.EndOperationWithStatus(output.Failure())
				return fmt.Errorf(
					"failed to determine where to create temporary directory: %w",
					err,
				)
			}

			cleaner := cleanup.NewCleaner()
			defer cleaner.Cleanup()

			tempRegistryDir, err := os.MkdirTemp(filepath.Dir(outputFileAbs), ".helm-bundle-*")
			if err != nil {
				out.EndOperationWithStatus(output.Failure())
				return fmt.Errorf("failed to create temporary directory for OCI registry: %w", err)
			}
			cleaner.AddCleanupFn(func() { _ = os.RemoveAll(tempRegistryDir) })
			out.EndOperationWithStatus(output.Success())

			out.StartOperation("Starting temporary OCI registry")
			reg, err := registry.NewRegistry(registry.Config{StorageDirectory: tempRegistryDir})
			if err != nil {
				out.EndOperationWithStatus(output.Failure())
				return fmt.Errorf("failed to create local OCI registry: %w", err)
			}
			go func() {
				if err := reg.ListenAndServe(); err != nil {
					out.Error(err, "error serving OCI registry")
					os.Exit(2)
				}
			}()
			out.EndOperationWithStatus(output.Success())

			out.StartOperation("Creating temporary chart storage directory")

			tempHelmChartStorageDir, err := os.MkdirTemp("", ".helm-bundle-temp-storage-*")
			if err != nil {
				out.EndOperationWithStatus(output.Failure())
				return fmt.Errorf(
					"failed to create temporary directory for Helm chart storage: %w",
					err,
				)
			}
			cleaner.AddCleanupFn(func() { _ = os.RemoveAll(tempHelmChartStorageDir) })
			out.EndOperationWithStatus(output.Success())

			helmClient, helmCleanup := helm.NewClient(out)
			cleaner.AddCleanupFn(func() { _ = helmCleanup() })

			ociAddress := fmt.Sprintf("%s://%s/charts", helm.OCIScheme, reg.Address())

			for repoName, repoConfig := range cfg.Repositories {
				for chartName, chartVersions := range repoConfig.Charts {
					sort.Strings(chartVersions)

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
					if !ptr.Deref(repoConfig.TLSVerify, true) {
						opts = append(opts, helm.InsecureSkipTLSverifyOpt())
					}
					for _, chartVersion := range chartVersions {
						downloaded, err := helmClient.GetChartFromRepo(
							tempHelmChartStorageDir,
							repoConfig.RepoURL,
							chartName,
							chartVersion,
							[]helm.ConfigOpt{helm.RegistryClientConfigOpt()},
							opts...,
						)
						if err != nil {
							out.EndOperationWithStatus(output.Failure())
							return fmt.Errorf("failed to create Helm chart bundle: %w", err)
						}

						if err := helmClient.PushHelmChartToOCIRegistry(
							downloaded, ociAddress,
						); err != nil {
							out.EndOperationWithStatus(output.Failure())
							return fmt.Errorf(
								"failed to push Helm chart to temporary registry: %w",
								err,
							)
						}

						// Best effort cleanup of downloaded chart, will be cleaned up when the cleaner deletes the temporary
						// directory anyway.
						_ = os.Remove(downloaded)
					}
					out.EndOperationWithStatus(output.Success())
				}
			}
			for _, chartURL := range cfg.ChartURLs {
				out.StartOperation(fmt.Sprintf("Fetching Helm chart from URL %s", chartURL))
				downloaded, err := helmClient.GetChartFromURL(
					tempRegistryDir,
					chartURL,
					filepath.Dir(configFileAbs),
				)
				if err != nil {
					out.EndOperationWithStatus(output.Failure())
					return fmt.Errorf("failed to create Helm chart bundle: %w", err)
				}

				chrt, err := helm.LoadChart(downloaded)
				if err != nil {
					out.EndOperationWithStatus(output.Failure())
					return fmt.Errorf(
						"failed to extract Helm chart details from local chart: %w",
						err,
					)
				}

				_, ok := cfg.Repositories["local"]
				if !ok {
					cfg.Repositories["local"] = config.HelmRepositorySyncConfig{
						Charts: make(map[string][]string, 1),
					}
				}
				_, ok = cfg.Repositories["local"].Charts[chrt.Name()]
				if !ok {
					cfg.Repositories["local"].Charts[chrt.Name()] = make([]string, 0, 1)
				}
				cfg.Repositories["local"].Charts[chrt.Name()] = append(
					cfg.Repositories["local"].Charts[chrt.Name()],
					chrt.Metadata.Version,
				)

				if err := helmClient.PushHelmChartToOCIRegistry(
					downloaded, ociAddress,
				); err != nil {
					out.EndOperationWithStatus(output.Failure())
					return fmt.Errorf("failed to push Helm chart to temporary registry: %w", err)
				}

				// Best effort cleanup of downloaded chart, will be cleaned up when the cleaner deletes the temporary
				// directory anyway.
				_ = os.Remove(downloaded)

				out.EndOperationWithStatus(output.Success())
			}

			if err := config.WriteSanitizedHelmChartsConfig(cfg, filepath.Join(tempRegistryDir, "charts.yaml")); err != nil {
				return err
			}

			out.StartOperation(fmt.Sprintf("Archiving Helm charts to %s", outputFile))
			if err := archive.ArchiveDirectory(tempRegistryDir, outputFile); err != nil {
				out.EndOperationWithStatus(output.Failure())
				return fmt.Errorf("failed to create Helm charts bundle tarball: %w", err)
			}
			out.EndOperationWithStatus(output.Success())

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

	// TODO Unhide this from DKP CLI once DKP supports OCI registry for Helm charts.
	utils.AddCmdAnnotation(cmd, "exclude-from-dkp-cli", "true")

	return cmd
}
