// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helmbundle

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/phayes/freeport"
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/archive"
	"github.com/mesosphere/mindthegap/cleanup"
	"github.com/mesosphere/mindthegap/httpfs"
)

func NewCommand(out output.Output) *cobra.Command {
	var (
		helmBundleFile string
		listenAddress  string
		listenPort     uint16
		tlsCertificate string
		tlsKey         string
	)

	cmd := &cobra.Command{
		Use:   "helm-bundle",
		Short: "Serve a helm chart repository",
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

			out.StartOperation("Unarchiving Helm chart bundle")
			err = archive.UnarchiveToDirectory(helmBundleFile, tempDir)
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to unarchive Helm chart bundle: %w", err)
			}
			out.EndOperation(true)

			out.StartOperation("Creating http server")
			if listenPort == 0 {
				freePort, err := freeport.GetFreePort()
				if err != nil {
					out.EndOperation(false)
					return fmt.Errorf("failed to find a free port: %w", err)
				}
				listenPort = uint16(freePort)
			}
			addr := net.JoinHostPort(listenAddress, strconv.Itoa(int(listenPort)))
			srv := &http.Server{
				Addr:              addr,
				Handler:           http.FileServer(httpfs.DisableDirListingFS(tempDir)),
				ReadHeaderTimeout: 1 * time.Second,
			}
			out.EndOperation(true)
			scheme := "http"
			if tlsCertificate != "" && tlsKey != "" {
				scheme = "https"
			}

			out.Infof("Serving Helm charts at %s://%s", scheme, addr)
			if tlsCertificate != "" && tlsKey != "" {
				err = srv.ListenAndServeTLS(tlsCertificate, tlsKey)
			} else {
				err = srv.ListenAndServe()
			}
			if err != nil {
				return fmt.Errorf("failed to start http server: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().
		StringVar(&helmBundleFile, "helm-charts-bundle", "", "Tarball of Helm charts to serve")
	_ = cmd.MarkFlagRequired("helm-charts-bundle")
	cmd.Flags().StringVar(&listenAddress, "listen-address", "localhost", "Address to list on")
	cmd.Flags().
		Uint16Var(&listenPort, "listen-port", 0, "Port to listen on (0 means use any free port)")
	cmd.Flags().StringVar(&tlsCertificate, "tls-cert-file", "", "TLS certificate file")
	cmd.Flags().StringVar(&tlsKey, "tls-private-key-file", "", "TLS private key file")

	return cmd
}
