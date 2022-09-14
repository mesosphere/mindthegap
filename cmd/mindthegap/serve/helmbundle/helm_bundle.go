// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helmbundle

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cleanup"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/utils"
	"github.com/mesosphere/mindthegap/config"
	"github.com/mesosphere/mindthegap/docker/registry"
)

func NewCommand(out output.Output) (cmd *cobra.Command, stopCh chan struct{}) {
	var (
		helmBundleFiles []string
		listenAddress   string
		listenPort      uint16
		tlsCertificate  string
		tlsKey          string
	)

	stopCh = make(chan struct{})

	cmd = &cobra.Command{
		Use:   "helm-bundle",
		Short: "Serve a Helm chart repository in an OCI registry from Helm chart bundles",
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

			// Write out the merged image bundle config to the target directory for completeness.
			if err := config.WriteSanitizedHelmChartsConfig(cfg, filepath.Join(tempDir, "charts.yaml")); err != nil {
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

			go func() {
				if err := reg.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					out.Error(err, "error serving Docker registry")
					os.Exit(2)
				}
			}()
			<-stopCh

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&helmBundleFiles, "helm-bundle", nil,
		"Tarball of Helm charts to serve. Can also be a glob pattern.")
	_ = cmd.MarkFlagRequired("helm-bundle")
	cmd.Flags().StringVar(&listenAddress, "listen-address", "localhost", "Address to listen on")
	cmd.Flags().
		Uint16Var(&listenPort, "listen-port", 0, "Port to listen on (0 means use any free port)")
	cmd.Flags().StringVar(&tlsCertificate, "tls-cert-file", "", "TLS certificate file")
	cmd.Flags().StringVar(&tlsKey, "tls-private-key-file", "", "TLS private key file")

	// TODO Unhide this from DKP CLI once DKP supports OCI registry for Helm charts.
	utils.AddCmdAnnotation(cmd, "exclude-from-dkp-cli", "true")

	return cmd, stopCh
}
