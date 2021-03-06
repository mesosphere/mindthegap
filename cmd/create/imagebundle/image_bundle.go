// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imagebundle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/distribution/distribution/v3/manifest/manifestlist"
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/archive"
	"github.com/mesosphere/mindthegap/cleanup"
	"github.com/mesosphere/mindthegap/config"
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/skopeo"
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
		Short: "Create a tar.gz image bundle",
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

			skopeoRunner, skopeoCleanup := skopeo.NewRunner()
			cleaner.AddCleanupFn(func() { _ = skopeoCleanup() })

			// Sort registries for deterministic ordering.
			regNames := cfg.SortedRegistryNames()

			for _, registryName := range regNames {
				registryConfig := cfg[registryName]
				var skopeoOpts []skopeo.SkopeoOption
				if registryConfig.TLSVerify != nil && !*registryConfig.TLSVerify {
					skopeoOpts = append(skopeoOpts, skopeo.DisableSrcTLSVerify())
				}
				if registryConfig.Credentials != nil && registryConfig.Credentials.Username != "" {
					skopeoOpts = append(
						skopeoOpts,
						skopeo.SrcCredentials(
							registryConfig.Credentials.Username,
							registryConfig.Credentials.Password,
						),
					)
				} else {
					skopeoStdout, skopeoStderr, err := skopeoRunner.AttemptToLoginToRegistry(context.TODO(), registryName)
					if err != nil {
						out.Infof("---skopeo stdout---:\n%s", skopeoStdout)
						out.Infof("---skopeo stderr---:\n%s", skopeoStderr)
						return fmt.Errorf("error logging in to registry: %w", err)
					}
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

						srcSkopeoScheme := "docker://"
						srcImageManifestList, skopeoStdout, skopeoStderr, err := skopeoRunner.InspectManifest(
							context.Background(),
							fmt.Sprintf("%s%s", srcSkopeoScheme, srcImageName),
							skopeo.NoTags(),
						)
						if err != nil {
							srcSkopeoScheme = "docker-daemon:"
							srcDaemonImageManifestList, skopeoDaemonStdout, skopeoDaemonStderr, err := skopeoRunner.InspectManifest(
								context.Background(),
								fmt.Sprintf("%s%s", srcSkopeoScheme, srcImageName),
							)
							if err != nil {
								out.EndOperation(false)
								out.Infof("---skopeo stdout---:\n%s", skopeoStdout)
								out.Infof("---skopeo stderr---:\n%s", skopeoStderr)
								return err
							}
							srcImageManifestList = srcDaemonImageManifestList
							skopeoStdout = append(skopeoStdout, skopeoDaemonStdout...)
							skopeoStderr = append(skopeoStderr, skopeoDaemonStderr...)
						}
						out.V(4).Infof("---skopeo stdout---:\n%s", skopeoStdout)
						out.V(4).Infof("---skopeo stderr---:\n%s", skopeoStderr)
						destImageManifestList := manifestlist.ManifestList{
							Versioned: srcImageManifestList.Versioned,
						}
						platformManifests := make(
							map[string]manifestlist.ManifestDescriptor,
							len(srcImageManifestList.Manifests),
						)
						for i := range srcImageManifestList.Manifests {
							m := srcImageManifestList.Manifests[i]
							srcManifestPlatform := m.Platform.OS + "/" + m.Platform.Architecture
							if m.Platform.Variant != "" {
								srcManifestPlatform += "/" + m.Platform.Variant
							}
							platformManifests[srcManifestPlatform] = m
						}

						for _, p := range platforms {
							platformManifest, ok := platformManifests[p.String()]
							if !ok {
								if p.arch == "arm64" {
									p.variant = "v8"
								}
								platformManifest, ok = platformManifests[p.String()]
								if !ok {
									out.EndOperation(false)
									return fmt.Errorf(
										"could not find platform %s for image %s",
										p,
										srcImageName,
									)
								}
							}

							srcImageToCopy := fmt.Sprintf("%s/%s@%s", registryName,
								imageName,
								platformManifest.Digest)
							if srcSkopeoScheme != "docker://" {
								srcImageToCopy = srcImageName
							}

							skopeoStdout, skopeoStderr, err := skopeoRunner.Copy(
								context.TODO(),
								fmt.Sprintf(
									"%s%s",
									srcSkopeoScheme,
									srcImageToCopy,
								),
								fmt.Sprintf(
									"docker://%s/%s@%s",
									reg.Address(),
									imageName,
									platformManifest.Digest,
								),
								append(
									skopeoOpts,
									skopeo.DisableDestTLSVerify(),
									skopeo.OS(p.os),
									skopeo.Arch(p.arch),
									skopeo.Variant(p.variant),
									skopeo.PreserveDigests(),
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

							destImageManifestList.Manifests = append(
								destImageManifestList.Manifests,
								platformManifest,
							)
						}
						skopeoStdout, skopeoStderr, err = skopeoRunner.CopyManifest(context.TODO(),
							destImageManifestList,
							fmt.Sprintf("docker://%s/%s:%s", reg.Address(), imageName, imageTag),
							append(
								skopeoOpts,
								skopeo.DisableDestTLSVerify(),
							)...,
						)
						if err != nil {
							out.EndOperation(false)
							out.Infof("---skopeo stdout---:\n%s", skopeoStdout)
							out.Infof("---skopeo stderr---:\n%s", skopeoStderr)
							return err
						}
						out.EndOperation(true)
						out.V(4).Infof("---skopeo stdout---:\n%s", skopeoStdout)
						out.V(4).Infof("---skopeo stderr---:\n%s", skopeoStderr)
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
		StringVar(&outputFile, "output-file", "images.tar.gz", "Output file to write image bundle to")
	cmd.Flags().
		BoolVar(&overwrite, "overwrite", false, "Overwrite image bundle file if it already exists")

	return cmd
}
