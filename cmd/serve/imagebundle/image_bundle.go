// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imagebundle

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/archive"
	"github.com/mesosphere/mindthegap/cleanup"
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
		Short: "Serve an image registry from image bundles",
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

			sort.Strings(imageBundleFiles)

			// Just in case users specify the same bundle twice, keep a track of
			// files that have been extracted already so we only extract each of them once.
			extractedBundles := make(map[string]struct{}, len(imageBundleFiles))

			for _, imageBundleFile := range imageBundleFiles {
				if _, ok := extractedBundles[imageBundleFile]; ok {
					continue
				}
				extractedBundles[imageBundleFile] = struct{}{}

				out.StartOperation(fmt.Sprintf("Unarchiving image bundle %q", imageBundleFile))
				err = archive.UnarchiveToDirectory(imageBundleFile, tempDir)
				if err != nil {
					out.EndOperation(false)
					return fmt.Errorf("failed to unarchive image bundle: %w", err)
				}
				out.EndOperation(true)
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

	cmd.Flags().
		StringSliceVar(&imageBundleFiles, "images-bundle", nil, "Tarball of images to serve")
	_ = cmd.MarkFlagRequired("images-bundle")
	cmd.Flags().StringVar(&listenAddress, "listen-address", "localhost", "Address to list on")
	cmd.Flags().
		Uint16Var(&listenPort, "listen-port", 0, "Port to listen on (0 means use any free port)")
	cmd.Flags().StringVar(&tlsCertificate, "tls-cert-file", "", "TLS certificate file")
	cmd.Flags().StringVar(&tlsKey, "tls-private-key-file", "", "TLS private key file")

	return cmd
}
