// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package imagebundle_test

import (
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"strconv"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/phayes/freeport"
	"github.com/spf13/cobra"
	"k8s.io/client-go/transport"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	serveimagebundle "github.com/mesosphere/mindthegap/cmd/mindthegap/serve/imagebundle"
	"github.com/mesosphere/mindthegap/images/httputils"
	"github.com/mesosphere/mindthegap/test/e2e/imagebundle/helpers"
)

var _ = Describe("Serve Bundle", func() {
	var (
		bundleFile string
		cmd        *cobra.Command
		stopCh     chan struct{}
	)

	BeforeEach(func() {
		tmpDir := GinkgoT().TempDir()

		bundleFile = filepath.Join(tmpDir, "image-bundle.tar")

		cmd = helpers.NewCommand(GinkgoT(), func(out output.Output) *cobra.Command {
			var c *cobra.Command
			c, stopCh = serveimagebundle.NewCommand(out)
			return c
		})
	})

	It("Without TLS", func() {
		helpers.CreateBundle(
			GinkgoT(),
			bundleFile,
			filepath.Join("testdata", "create-success.yaml"),
		)

		port, err := freeport.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		cmd.SetArgs([]string{
			"--image-bundle", bundleFile,
			"--listen-port", strconv.Itoa(port),
		})

		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()

			Expect(cmd.Execute()).To(Succeed())

			close(done)
		}()

		helpers.WaitForTCPPort(GinkgoT(), "localhost", port)

		helpers.ValidateImageIsAvailable(
			GinkgoT(),
			"localhost",
			port,
			"stefanprodan/podinfo",
			"6.2.0",
			[]*v1.Platform{{
				OS:           "linux",
				Architecture: runtime.GOARCH,
			}},
		)

		close(stopCh)

		Eventually(done).Should(BeClosed())
	})

	It("With TLS", func() {
		ipAddr := helpers.GetFirstNonLoopbackIP(GinkgoT())

		tempCertDir := GinkgoT().TempDir()
		caCertFile, _, certFile, keyFile := helpers.GenerateCertificateAndKeyWithIPSAN(
			GinkgoT(),
			tempCertDir,
			ipAddr,
		)

		helpers.CreateBundle(
			GinkgoT(),
			bundleFile,
			filepath.Join("testdata", "create-success.yaml"),
		)

		port, err := freeport.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		cmd.SetArgs([]string{
			"--image-bundle", bundleFile,
			"--listen-address", ipAddr.String(),
			"--listen-port", strconv.Itoa(port),
			"--tls-cert-file", certFile,
			"--tls-private-key-file", keyFile,
		})

		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()

			Expect(cmd.Execute()).To(Succeed())

			close(done)
		}()

		helpers.WaitForTCPPort(GinkgoT(), ipAddr.String(), port)

		helpers.ValidateImageIsAvailable(
			GinkgoT(),
			ipAddr.String(),
			port,
			"stefanprodan/podinfo",
			"6.2.0",
			[]*v1.Platform{{
				OS:           "linux",
				Architecture: runtime.GOARCH,
			}},
			remote.WithTransport(
				httputils.NewConfigurableTLSRoundTripper(
					remote.DefaultTransport, httputils.TLSHostsConfig{
						net.JoinHostPort(ipAddr.String(), strconv.Itoa(port)): transport.TLSConfig{
							CAFile: caCertFile,
						},
					},
				),
			),
		)

		close(stopCh)

		Eventually(done).Should(BeClosed())
	})

	It("Bundle does not exist", func() {
		cmd.SetArgs([]string{
			"--image-bundle", bundleFile,
		})

		Expect(
			cmd.Execute(),
		).To(MatchError(fmt.Sprintf("did find any matching files for %q", bundleFile)))
	})
})
