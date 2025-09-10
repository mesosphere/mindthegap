// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/jm33-m0/arc/v2"
	"github.com/mholt/archives"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
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
	"github.com/mesosphere/mindthegap/images"
	"github.com/mesosphere/mindthegap/images/authnhelpers"
	"github.com/mesosphere/mindthegap/images/httputils"
)

func NewCommand( //nolint:gocyclo // TODO: Refactor this command to make it more readable.
	out output.Output,
) *cobra.Command {
	var (
		imagesConfigFile       string
		helmChartsConfigFile   string
		ociArtifactsConfigFile string
		platforms              = flags.NewPlatformsValue("linux/amd64")
		allPlatforms           bool
		outputFile             string
		overwrite              bool
		merge                  bool
		imagePullConcurrency   int
	)

	cmd := &cobra.Command{
		Use:   "bundle",
		Short: "Create a bundle containing container images and/or Helm charts",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.ValidateRequiredFlags(); err != nil {
				return err
			}

			archiver, _, err := archives.Identify(context.Background(), outputFile, nil)
			if err != nil {
				return fmt.Errorf(
					"failed to identify archive format for bundle %s: %w", outputFile, err,
				)
			}

			// Disallow tar.gz and tar.bz2 archives as noted in the docs for github.com/mholt/archives that
			// traversing compressed tar archives is extremely slow and inefficient. Benchmarking confirms
			// that this is indeed the case, so we don't support them.
			ext := archiver.Extension()
			if ext == ".tar.gz" || ext == ".tar.bz2" {
				return fmt.Errorf("compressed tar archives (%s) are not supported", ext)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				helmChartsConfig   config.HelmChartsConfig
				imagesConfig       config.ImagesConfig
				ociArtifactsConfig config.ImagesConfig
			)
			if imagesConfigFile != "" {
				out.StartOperation("Parsing image bundle config")
				cfg, err := config.ParseImagesConfigFile(imagesConfigFile)
				if err != nil {
					out.EndOperationWithStatus(output.Failure())
					return err
				}
				out.EndOperationWithStatus(output.Success())
				out.V(4).Infof("Images config: %+v", cfg)
				imagesConfig = cfg
			}

			if helmChartsConfigFile != "" {
				out.StartOperation("Parsing Helm chart bundle config")
				cfg, err := config.ParseHelmChartsConfigFile(helmChartsConfigFile)
				if err != nil {
					out.EndOperationWithStatus(output.Failure())
					return err
				}
				out.EndOperationWithStatus(output.Success())
				out.V(4).Infof("Helm charts config: %+v", cfg)
				helmChartsConfig = cfg
			}

			// for now, we start with re-using the same struct for OCI artifacts as for docker images.
			if ociArtifactsConfigFile != "" {
				out.StartOperation("Parsing OCI artifacts bundle config")
				cfg, err := config.ParseImagesConfigFile(ociArtifactsConfigFile)
				if err != nil {
					out.EndOperationWithStatus(output.Failure())
					return err
				}
				out.EndOperationWithStatus(output.Success())
				out.V(4).Infof("OCI artifacts config: %+v", cfg)
				ociArtifactsConfig = cfg
			}

			if !overwrite && !merge {
				out.StartOperation("Checking if output file already exists")
				_, err := os.Stat(outputFile)
				switch {
				case err == nil:
					out.EndOperationWithStatus(output.Failure())
					return fmt.Errorf(
						"%s already exists: specify --overwrite to overwrite existing file"+
							"                   or --merge to add new images to the existing file",
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

			out.StartOperation("Creating temporary directory")
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

			tempDir, err := os.MkdirTemp(filepath.Dir(outputFileAbs), ".bundle-*")
			if err != nil {
				out.EndOperationWithStatus(output.Failure())
				return fmt.Errorf("failed to create temporary directory: %w", err)
			}
			cleaner.AddCleanupFn(func() { _ = os.RemoveAll(tempDir) })

			out.EndOperationWithStatus(output.Success())

			var (
				existingImagesConfig     config.ImagesConfig
				existingHelmChartsConfig config.HelmChartsConfig
			)

			if merge {
				_, err := os.Stat(outputFile)
				switch {
				case err != nil && !errors.Is(err, os.ErrNotExist):
					return fmt.Errorf(
						"failed to check if output file %s already exists: %w",
						outputFile,
						err,
					)
				case err == nil:
					out.StartOperation("Unpacking existing bundle file to prepare for merge")
					if err := arc.Unarchive(outputFile, tempDir); err != nil {
						out.EndOperationWithStatus(output.Failure())
						return fmt.Errorf("failed to unpack existing bundle file: %w", err)
					}
					existingImagesConfig, err = config.ParseImagesConfigFile(filepath.Join(tempDir, "images.yaml"))
					if err != nil && !errors.Is(err, os.ErrNotExist) {
						out.EndOperationWithStatus(output.Failure())
						return fmt.Errorf("failed to parse existing bundle config: %w", err)
					}
					existingHelmChartsConfig, err = config.ParseHelmChartsConfigFile(
						filepath.Join(tempDir, "charts.yaml"),
					)
					if err != nil && !errors.Is(err, os.ErrNotExist) {
						out.EndOperationWithStatus(output.Failure())
						return fmt.Errorf("failed to parse existing bundle config: %w", err)
					}
					out.EndOperationWithStatus(output.Success())
				default:
					// Do nothing, file doesn't exist.
				}
			}

			out.StartOperation("Starting temporary Docker registry")
			reg, err := registry.NewRegistry(
				registry.Config{Storage: registry.FilesystemStorage(tempDir)},
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

			logs.Debug.SetOutput(out.V(4).InfoWriter())
			logs.Warn.SetOutput(out.V(2).InfoWriter())

			if imagesConfigFile != "" || ociArtifactsConfigFile != "" {
				if allPlatforms {
					platforms = flags.NewPlatformsValue("*/*")
				}

				if err := PullImagesAndOCIArtifacts(
					imagesConfig,
					ociArtifactsConfig,
					existingImagesConfig,
					platforms,
					imagePullConcurrency,
					reg,
					tempDir,
					out,
				); err != nil {
					return err
				}
			}

			if helmChartsConfigFile != "" {
				helmChartsConfigFileAbs, err := filepath.Abs(helmChartsConfigFile)
				if err != nil {
					return err
				}

				if err := pullCharts(
					helmChartsConfig,
					existingHelmChartsConfig,
					helmChartsConfigFileAbs,
					reg,
					tempDir,
					cleaner,
					out,
				); err != nil {
					return err
				}
			}

			out.StartOperation(fmt.Sprintf("Archiving bundle to %s", outputFile))
			if err := archive.ArchiveDirectory(tempDir, outputFile); err != nil {
				out.EndOperationWithStatus(output.Failure())
				return fmt.Errorf("failed to create bundle tarball: %w", err)
			}
			out.EndOperationWithStatus(output.Success())

			return nil
		},
	}

	cmd.Flags().StringVar(&imagesConfigFile, "images-file", "",
		"File containing list of images to create bundle from, either as YAML configuration or a simple list of images")
	cmd.Flags().StringVar(&helmChartsConfigFile, "helm-charts-file", "",
		"YAML file containing configuration of Helm charts to create bundle from")
	cmd.Flags().StringVar(&ociArtifactsConfigFile, "oci-artifacts-file", "",
		"File containing list of oci artifacts to create bundle from, "+
			"either as YAML configuration or a simple list of images")
	cmd.MarkFlagsOneRequired("images-file", "helm-charts-file", "oci-artifacts-file")
	cmd.Flags().
		Var(&platforms, "platform", "platforms to download images for (required format: <os>/<arch>[/<variant>])")
	cmd.Flags().
		BoolVar(&allPlatforms, "all-platforms", false, "Download images for all platforms specified in the image manifests")
	cmd.MarkFlagsMutuallyExclusive("platform", "all-platforms")
	cmd.Flags().
		StringVar(&outputFile, "output-file", "bundle.tar", "Output file to write bundle to")
	cmd.Flags().
		BoolVar(&overwrite, "overwrite", false, "Overwrite bundle file if it already exists")
	cmd.Flags().
		BoolVar(&merge, "merge", false, "Merge new images into existing bundle file if it already exists")
	cmd.MarkFlagsMutuallyExclusive("overwrite", "merge")
	cmd.Flags().
		IntVar(&imagePullConcurrency, "image-pull-concurrency", 1, "Image pull concurrency")

	return cmd
}

func PullImagesAndOCIArtifacts(
	imagesConfig config.ImagesConfig,
	ociArtifactsConfig config.ImagesConfig,
	existingImagesConfig config.ImagesConfig,
	platforms flags.Platforms,
	imagePullConcurrency int,
	reg *registry.Registry,
	outputDir string,
	out output.Output,
) error {
	pullGauge := &output.ProgressGauge{}
	pullGauge.SetCapacity(imagesConfig.TotalImages() + ociArtifactsConfig.TotalImages())
	pullGauge.SetStatus("Pulling requested images")
	out.StartOperationWithProgress(pullGauge)
	progressFn := func() { pullGauge.Inc() }

	if imagesConfig.TotalImages() > 0 {
		if err := pullImages(
			imagesConfig,
			platforms,
			imagePullConcurrency,
			reg,
			progressFn,
			false,
		); err != nil {
			out.EndOperationWithStatus(output.Failure())
			return err
		}
	}

	if ociArtifactsConfig.TotalImages() > 0 {
		if err := pullImages(
			ociArtifactsConfig,
			platforms,
			imagePullConcurrency,
			reg,
			progressFn,
			true,
		); err != nil {
			out.EndOperationWithStatus(output.Failure())
			return err
		}
	}

	if err := config.WriteSanitizedImagesConfigs(
		filepath.Join(outputDir, "images.yaml"),
		imagesConfig,
		ociArtifactsConfig,
		existingImagesConfig,
	); err != nil {
		out.EndOperationWithStatus(output.Failure())
		return err
	}

	out.EndOperationWithStatus(output.Success())
	return nil
}

func pullImages(
	cfg config.ImagesConfig,
	platforms flags.Platforms,
	imagePullConcurrency int,
	reg *registry.Registry,
	progressFn func(),
	isOCIArtifact bool,
) error {
	// Sort registries for deterministic ordering.
	regNames := cfg.SortedRegistryNames()

	eg, egCtx := errgroup.WithContext(context.Background())
	eg.SetLimit(imagePullConcurrency)

	destTLSRoundTripper, err := httputils.InsecureTLSRoundTripper(remote.DefaultTransport)
	if err != nil {
		return fmt.Errorf("error configuring TLS for destination registry: %w", err)
	}
	defer func() {
		if tr, ok := destTLSRoundTripper.(*http.Transport); ok {
			tr.CloseIdleConnections()
		}
	}()
	destRemoteOpts := []remote.Option{
		remote.WithTransport(destTLSRoundTripper),
		remote.WithContext(egCtx),
		remote.WithUserAgent(utils.Useragent()),
	}

	for registryIdx := range regNames {
		registryName := regNames[registryIdx]

		registryConfig := cfg[registryName]

		sourceTLSRoundTripper, err := httputils.TLSConfiguredRoundTripper(
			remote.DefaultTransport,
			registryName,
			registryConfig.TLSVerify != nil && !*registryConfig.TLSVerify,
			"",
		)
		if err != nil {
			return fmt.Errorf("error configuring TLS for source registry: %w", err)
		}

		keychain := authn.NewMultiKeychain(
			authn.NewKeychainFromHelper(
				authnhelpers.NewStaticHelper(registryName, registryConfig.Credentials),
			),
			authn.DefaultKeychain,
		)

		sourceRemoteOpts := []remote.Option{
			remote.WithTransport(sourceTLSRoundTripper),
			remote.WithAuthFromKeychain(keychain),
			remote.WithContext(egCtx),
			remote.WithUserAgent(utils.Useragent()),
		}

		platformsStrings := platforms.GetSlice()

		// Sort images for deterministic ordering.
		imageNames := registryConfig.SortedImageNames()

		wg := new(sync.WaitGroup)

		for imageIdx := range imageNames {
			imageName := imageNames[imageIdx]
			imageTags := registryConfig.Images[imageName]

			wg.Add(len(imageTags))
			for j := range imageTags {
				imageTag := imageTags[j]

				eg.Go(func() error {
					defer wg.Done()

					srcImageName := fmt.Sprintf(
						"%s/%s:%s",
						registryName,
						imageName,
						imageTag,
					)
					destImageName := fmt.Sprintf(
						"%s/%s:%s",
						reg.Address(),
						imageName,
						imageTag,
					)
					ref, err := name.ParseReference(destImageName, name.StrictValidation)
					if err != nil {
						return err
					}

					var image remote.Taggable
					if isOCIArtifact {
						image, err = images.OCIArtifactImage(
							srcImageName,
							sourceRemoteOpts...,
						)
					} else {
						image, err = images.ManifestListForImage(
							srcImageName,
							platformsStrings,
							sourceRemoteOpts...,
						)
					}
					if err != nil {
						return fmt.Errorf("failed to get image %q: %w", srcImageName, err)
					}

					if err := remote.Push(ref, image, destRemoteOpts...); err != nil {
						return err
					}

					progressFn()

					return nil
				})
			}
		}

		go func() {
			wg.Wait()

			if tr, ok := sourceTLSRoundTripper.(*http.Transport); ok {
				tr.CloseIdleConnections()
			}
		}()
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

func pullCharts(
	cfg, existingCfg config.HelmChartsConfig,
	helmChartsConfigFileAbs string,
	reg *registry.Registry,
	outputDir string,
	cleaner cleanup.Cleaner,
	out output.Output,
) error {
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
			outputDir,
			chartURL,
			filepath.Dir(helmChartsConfigFileAbs),
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
			if cfg.Repositories == nil {
				cfg.Repositories = make(map[string]config.HelmRepositorySyncConfig, 1)
			}

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

	if err := config.WriteSanitizedHelmChartsConfig(
		filepath.Join(outputDir, "charts.yaml"), cfg, existingCfg,
	); err != nil {
		return err
	}

	return nil
}
