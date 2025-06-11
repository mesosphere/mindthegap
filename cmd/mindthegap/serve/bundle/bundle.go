// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cleanup"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/flags"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/utils"
	"github.com/mesosphere/mindthegap/docker/registry"
)

func NewCommand(
	out output.Output,
	bundleCmdName string,
) (cmd *cobra.Command, stopCh chan struct{}) {
	var (
		bundleFiles        []string
		listenAddress      string
		listenPort         uint16
		tlsCertificate     string
		tlsKey             string
		repositoriesPrefix string
	)

	stopCh = make(chan struct{})

	cmd = &cobra.Command{
		Use:   bundleCmdName,
		Short: "Serve an OCI registry from previously created bundles",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.ValidateRequiredFlags(); err != nil {
				return err
			}

			if err := flags.ValidateFlagsThatRequireValues(cmd, bundleCmdName); err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cleaner := cleanup.NewCleaner()
			defer cleaner.Cleanup()
			out.StartOperation("Creating temporary directory")

			out.EndOperationWithStatus(output.Success())

			bundleFiles, err := utils.FilesWithGlobs(bundleFiles)
			if err != nil {
				return err
			}

			out.StartOperation("Creating Docker registry")
			storage, err := registry.ArchiveStorage(repositoriesPrefix, bundleFiles...)
			if err != nil {
				out.EndOperationWithStatus(output.Failure())
				return fmt.Errorf("failed to create storage for Docker registry from supplied bundles: %w", err)
			}
			reg, err := registry.NewRegistry(registry.Config{
				Storage:  storage,
				ReadOnly: true,
				Host:     listenAddress,
				Port:     listenPort,
				TLS: registry.TLS{
					Certificate: tlsCertificate,
					Key:         tlsKey,
				},
			})
			if err != nil {
				out.EndOperationWithStatus(output.Failure())
				return fmt.Errorf("failed to create local Docker registry: %w", err)
			}
			out.EndOperationWithStatus(output.Success())
			out.Infof("Listening on %s\n", reg.Address())

			go func() {
				if err := reg.ListenAndServe(output.NewOutputLogr(out)); err != nil &&
					!errors.Is(err, http.ErrServerClosed) {
					out.Error(err, "error serving Docker registry")
					os.Exit(2)
				}
			}()
			<-stopCh

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&bundleFiles, bundleCmdName, nil,
		"Bundle to serve. Can also be a glob pattern.")
	_ = cmd.MarkFlagRequired(bundleCmdName)
	cmd.Flags().StringVar(&listenAddress, "listen-address", "127.0.0.1", "Address to listen on")
	cmd.Flags().
		Uint16Var(&listenPort, "listen-port", 0, "Port to listen on (0 means use any free port)")
	cmd.Flags().StringVar(&tlsCertificate, "tls-cert-file", "", "TLS certificate file")
	cmd.Flags().StringVar(&tlsKey, "tls-private-key-file", "", "TLS private key file")
	cmd.Flags().StringVar(&repositoriesPrefix, "repositories-prefix", "",
		"Prefix to prepend to all repositories in the bundle when serving")

	return cmd, stopCh
}
