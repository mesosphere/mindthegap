// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imagebundle

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cleanup"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/utils"
	"github.com/mesosphere/mindthegap/config"
	"github.com/mesosphere/mindthegap/docker/registry"
)

func NewCommand(out output.Output) *cobra.Command {
	var (
		imageBundleFiles []string
		listenAddress    string
		listenPort       uint16
		tlsCertificate   string
		tlsKey           string
	)

	cmd := &cobra.Command{
		Use:   "image-bundle",
		Short: "Serve an OCI registry from image bundles",
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

			imageBundleFiles, err = utils.FilesWithGlobs(imageBundleFiles)
			if err != nil {
				return err
			}
			cfg, _, err := utils.ExtractBundles(tempDir, out, imageBundleFiles...)
			if err != nil {
				return err
			}

			// Write out the merged image bundle config to the target directory for completeness.
			if err := config.WriteSanitizedImagesConfig(cfg, filepath.Join(tempDir, "images.yaml")); err != nil {
				return err
			}

			out.StartOperation("Creating Docker registry")
			reg, err := registry.NewRegistry(registry.Config{
				StorageDirectory: tempDir,
				ReadOnly:         true,
				Host:             listenAddress,
				Port:             listenPort,
				TLS: registry.TLS{
					Certificate: tlsCertificate,
					Key:         tlsKey,
				},
			})
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to create local Docker registry: %w", err)
			}
			out.EndOperation(true)
			out.Infof("Listening on %s\n", reg.Address())
			if err := reg.ListenAndServe(); err != nil {
				out.Error(err, "error serving Docker registry")
				os.Exit(2)
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&imageBundleFiles, "images-bundle", nil,
		"Tarball of images to serve. Can also be a glob pattern.")
	_ = cmd.MarkFlagRequired("images-bundle")
	cmd.Flags().StringVar(&listenAddress, "listen-address", "localhost", "Address to listen on")
	cmd.Flags().
		Uint16Var(&listenPort, "listen-port", 0, "Port to listen on (0 means use any free port)")
	cmd.Flags().StringVar(&tlsCertificate, "tls-cert-file", "", "TLS certificate file")
	cmd.Flags().StringVar(&tlsKey, "tls-private-key-file", "", "TLS private key file")

	return cmd
}
