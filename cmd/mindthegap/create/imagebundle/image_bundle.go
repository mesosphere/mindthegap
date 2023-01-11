// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imagebundle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
	"k8s.io/client-go/transport"

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
		configFile string
		platforms  []platform
		outputFile string
		overwrite  bool
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

			out.StartOperation("Parsing image bundle config")
			cfg, err := config.ParseImagesConfigFile(configFile)
			if err != nil {
				out.EndOperation(false)
				return err
			}
			out.EndOperation(true)
			out.V(4).Infof("Images config: %+v", cfg)

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

			tempDir, err := os.MkdirTemp(filepath.Dir(outputFileAbs), ".image-bundle-*")
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to create temporary directory: %w", err)
			}
			cleaner.AddCleanupFn(func() { _ = os.RemoveAll(tempDir) })

			out.EndOperation(true)

			out.StartOperation("Starting temporary Docker registry")
			reg, err := registry.NewRegistry(registry.Config{StorageDirectory: tempDir})
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

			// Sort registries for deterministic ordering.
			regNames := cfg.SortedRegistryNames()

			for _, registryName := range regNames {
				registryConfig := cfg[registryName]

				var remoteOpts []remote.Option
				if registryConfig.TLSVerify != nil && !*registryConfig.TLSVerify {
					transport := httputils.NewConfigurableTLSRoundTripper(
						remote.DefaultTransport,
						httputils.TLSHostsConfig{registryName: transport.TLSConfig{Insecure: true}},
					)

					remoteOpts = append(remoteOpts, remote.WithTransport(transport))
				}

				keychain := authn.NewMultiKeychain(
					authn.NewKeychainFromHelper(
						authnhelpers.NewStaticHelper(registryName, registryConfig.Credentials),
					),
					authn.DefaultKeychain,
				)

				remoteOpts = append(remoteOpts, remote.WithAuthFromKeychain(keychain))

				platformsStrings := make([]string, 0, len(platforms))
				for _, p := range platforms {
					platformsStrings = append(platformsStrings, p.String())
				}

				// Sort images for deterministic ordering.
				imageNames := registryConfig.SortedImageNames()

				for _, imageName := range imageNames {
					imageTags := registryConfig.Images[imageName]
					for _, imageTag := range imageTags {
						srcImageName := fmt.Sprintf("%s/%s:%s", registryName, imageName, imageTag)
						out.StartOperation(
							fmt.Sprintf("Copying %s (platforms: %v)",
								srcImageName, platforms,
							),
						)

						imageIndex, err := images.ManifestListForImage(
							srcImageName,
							platformsStrings,
							remoteOpts...)
						if err != nil {
							out.EndOperation(false)
							return err
						}

						destImageName := fmt.Sprintf("%s/%s:%s", reg.Address(), imageName, imageTag)
						ref, err := name.ParseReference(destImageName, name.StrictValidation)
						if err != nil {
							out.EndOperation(false)
							return err
						}

						if err := remote.WriteIndex(ref, imageIndex, remoteOpts...); err != nil {
							out.EndOperation(false)
							return err
						}

						out.EndOperation(true)
					}
				}
			}

			if err := config.WriteSanitizedImagesConfig(cfg, filepath.Join(tempDir, "images.yaml")); err != nil {
				return err
			}

			out.StartOperation(fmt.Sprintf("Archiving images to %s", outputFile))
			if err := archive.ArchiveDirectory(tempDir, outputFile); err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to create image bundle tarball: %w", err)
			}
			out.EndOperation(true)

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

	return cmd
}
