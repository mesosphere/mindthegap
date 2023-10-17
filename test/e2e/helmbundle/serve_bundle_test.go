// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package helmbundle_test

import (
	"fmt"
	"path/filepath"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/phayes/freeport"
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	servebundle "github.com/mesosphere/mindthegap/cmd/mindthegap/serve/bundle"
	"github.com/mesosphere/mindthegap/helm"
	"github.com/mesosphere/mindthegap/test/e2e/helmbundle/helpers"
)

var _ = Describe("Serve Bundle", func() {
	var (
		bundleFile string
		cmd        *cobra.Command
		stopCh     chan struct{}
	)

	BeforeEach(func() {
		tmpDir := GinkgoT().TempDir()

		bundleFile = filepath.Join(tmpDir, "helm-bundle.tar")

		cmd = helpers.NewCommand(GinkgoT(), func(out output.Output) *cobra.Command {
			var c *cobra.Command
			c, stopCh = servebundle.NewCommand(out, "helm-bundle")
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
			"--helm-bundle", bundleFile,
			"--listen-port", strconv.Itoa(port),
		})

		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()

			Expect(cmd.Execute()).To(Succeed())

			close(done)
		}()

		helpers.WaitForTCPPort(GinkgoT(), "127.0.0.1", port)

		helpers.ValidateChartIsAvailable(
			GinkgoT(),
			"127.0.0.1",
			port,
			"podinfo",
			"6.2.0",
			helm.InsecureSkipTLSverifyOpt(),
		)

		close(stopCh)

		Eventually(done).Should(BeClosed())
	})

	It("With TLS", func() {
		ipAddr := helpers.GetFirstNonLoopbackIP(GinkgoT())

		tempCertDir := GinkgoT().TempDir()
		_, _, certFile, keyFile := helpers.GenerateCertificateAndKeyWithIPSAN(
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
			"--helm-bundle", bundleFile,
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

		// TODO Reenable once Helm supports custom CA certs and self-signed certs.
		// helpers.ValidateChartIsAvailable(GinkgoT(), "127.0.0.1", port, "podinfo", "6.2.0", helm.CAFileOpt(caCertFile))

		close(stopCh)

		Eventually(done).Should(BeClosed())
	})

	It("Bundle does not exist", func() {
		cmd.SetArgs([]string{
			"--helm-bundle", bundleFile,
		})

		Expect(
			cmd.Execute(),
		).To(MatchError(fmt.Sprintf("did find any matching files for %q", bundleFile)))
	})
})
