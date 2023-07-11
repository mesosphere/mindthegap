// Copyright 2021-2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imagebundle

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/archive"
	"github.com/mesosphere/mindthegap/cleanup"
	"github.com/mesosphere/mindthegap/config"
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/images"
	"github.com/mesosphere/mindthegap/images/authnhelpers"
	"github.com/mesosphere/mindthegap/images/httputils"
)

func NewCommand(out output.Output) *cobra.Command {
	var (
		configFile           string
		platforms            []platform
		outputFile           string
		overwrite            bool
		imagePullConcurrency int
	)

	cmd := &cobra.Command{
		Use:   "image-bundle",
		Short: "Create an image bundle",
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

			out.StartOperation("Parsing image bundle config")
			cfg, err := config.ParseImagesConfigFile(configFile)
			if err != nil {
				out.EndOperationWithStatus(output.Failure())
				return err
			}
			out.EndOperationWithStatus(output.Success())
			out.V(4).Infof("Images config: %+v", cfg)

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

			tempDir, err := os.MkdirTemp(filepath.Dir(outputFileAbs), ".image-bundle-*")
			if err != nil {
				out.EndOperationWithStatus(output.Failure())
				return fmt.Errorf("failed to create temporary directory: %w", err)
			}
			cleaner.AddCleanupFn(func() { _ = os.RemoveAll(tempDir) })

			out.EndOperationWithStatus(output.Success())

			out.StartOperation("Starting temporary Docker registry")
			reg, err := registry.NewRegistry(registry.Config{StorageDirectory: tempDir})
			if err != nil {
				out.EndOperationWithStatus(output.Failure())
				return fmt.Errorf("failed to create local Docker registry: %w", err)
			}
			go func() {
				if err := reg.ListenAndServe(); err != nil {
					out.Error(err, "error serving Docker registry")
					os.Exit(2)
				}
			}()
			out.EndOperationWithStatus(output.Success())

			logs.Debug.SetOutput(out.V(4).InfoWriter())
			logs.Warn.SetOutput(out.InfoWriter())

			destTLSRoundTripper, err := httputils.InsecureTLSRoundTripper(remote.DefaultTransport)
			if err != nil {
				out.Error(err, "error configuring TLS for destination registry")
				os.Exit(2)
			}
			destRemoteOpts := []remote.Option{remote.WithTransport(destTLSRoundTripper)}

			// Sort registries for deterministic ordering.
			regNames := cfg.SortedRegistryNames()

			eg, egCtx := errgroup.WithContext(context.Background())
			eg.SetLimit(imagePullConcurrency)

			pullGauge := &output.ProgressGauge{}
			pullGauge.SetCapacity(cfg.TotalImages())
			pullGauge.SetStatus("Pulling requested images")

			out.StartOperationWithProgress(pullGauge)

			for _, registryName := range regNames {
				registryConfig := cfg[registryName]

				sourceTLSRoundTripper, err := httputils.TLSConfiguredRoundTripper(
					remote.DefaultTransport,
					registryName,
					registryConfig.TLSVerify != nil && !*registryConfig.TLSVerify,
					"",
				)
				if err != nil {
					out.EndOperationWithStatus(output.Failure())
					out.Error(err, "error configuring TLS for source registry")
					os.Exit(2)
				}
				sourceRemoteOpts := []remote.Option{remote.WithTransport(sourceTLSRoundTripper)}

				keychain := authn.NewMultiKeychain(
					authn.NewKeychainFromHelper(
						authnhelpers.NewStaticHelper(registryName, registryConfig.Credentials),
					),
					authn.DefaultKeychain,
				)

				sourceRemoteOpts = append(sourceRemoteOpts, remote.WithAuthFromKeychain(keychain))

				platformsStrings := make([]string, 0, len(platforms))
				for _, p := range platforms {
					platformsStrings = append(platformsStrings, p.String())
				}

				// Sort images for deterministic ordering.
				imageNames := registryConfig.SortedImageNames()

				destRemoteOpts = append(destRemoteOpts, remote.WithContext(egCtx))
				sourceRemoteOpts = append(sourceRemoteOpts, remote.WithContext(egCtx))

				for i := range imageNames {
					imageName := imageNames[i]
					imageTags := registryConfig.Images[imageName]

					for j := range imageTags {
						imageTag := imageTags[j]

						eg.Go(func() error {
							srcImageName := fmt.Sprintf("%s/%s:%s", registryName, imageName, imageTag)

							imageIndex, err := images.ManifestListForImage(
								srcImageName,
								platformsStrings,
								sourceRemoteOpts...,
							)
							if err != nil {
								return err
							}

							destImageName := fmt.Sprintf("%s/%s:%s", reg.Address(), imageName, imageTag)
							ref, err := name.ParseReference(destImageName, name.StrictValidation)
							if err != nil {
								return err
							}

							if err := remote.WriteIndex(ref, imageIndex, destRemoteOpts...); err != nil {
								return err
							}

							pullGauge.Inc()

							return nil
						})
					}
				}

				err = eg.Wait()

				if tr, ok := sourceTLSRoundTripper.(*http.Transport); ok {
					tr.CloseIdleConnections()
				}
				if tr, ok := destTLSRoundTripper.(*http.Transport); ok {
					tr.CloseIdleConnections()
				}

				if err != nil {
					out.EndOperationWithStatus(output.Failure())
					return err
				}
			}

			out.EndOperationWithStatus(output.Success())

			if err := config.WriteSanitizedImagesConfig(cfg, filepath.Join(tempDir, "images.yaml")); err != nil {
				return err
			}

			out.StartOperation(fmt.Sprintf("Archiving images to %s", outputFile))
			if err := archive.ArchiveDirectory(tempDir, outputFile); err != nil {
				out.EndOperationWithStatus(output.Failure())
				return fmt.Errorf("failed to create image bundle tarball: %w", err)
			}
			out.EndOperationWithStatus(output.Success())

			return nil
		},
	}

	cmd.Flags().StringVar(&configFile, "images-file", "",
		"File containing list of images to create bundle from, either as YAML configuration or a simple list of images")
	_ = cmd.MarkFlagRequired("images-file")
	cmd.Flags().
		Var(newPlatformSlicesValue([]platform{{os: "linux", arch: "amd64"}}, &platforms), "platform",
			"platforms to download images (required format: <os>/<arch>[/<variant>])")
	cmd.Flags().
		StringVar(&outputFile, "output-file", "images.tar", "Output file to write image bundle to")
	cmd.Flags().
		BoolVar(&overwrite, "overwrite", false, "Overwrite image bundle file if it already exists")
	cmd.Flags().
		IntVar(&imagePullConcurrency, "image-pull-concurrency", 1, "Image pull concurrency")

	return cmd
}
