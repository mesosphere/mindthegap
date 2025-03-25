// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package helmbundle_test

import (
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/phayes/freeport"
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	servebundle "github.com/mesosphere/mindthegap/cmd/mindthegap/serve/bundle"
	"github.com/mesosphere/mindthegap/helm"
	"github.com/mesosphere/mindthegap/test/e2e/helmbundle/helpers"
)

var _ = Describe("Serve Helm Bundle", func() {
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
			c, stopCh = servebundle.NewCommand(out, "bundle")
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
			"--bundle", bundleFile,
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
			Default,
			"127.0.0.1",
			port,
			"podinfo",
			"6.2.0",
			helm.InsecureSkipTLSverifyOpt(),
		)

		helpers.ValidateChartIsAvailable(
			GinkgoT(),
			Default,
			"127.0.0.1",
			port,
			"node-feature-discovery",
			"0.15.2",
			helm.InsecureSkipTLSverifyOpt(),
		)

		close(stopCh)

		Eventually(done).Should(BeClosed())
	})

	It("With TLS", func() {
		ipAddr := helpers.GetFirstNonLoopbackIP(GinkgoT())

		tempCertDir := GinkgoT().TempDir()
		originalCACertFile, _, certFile, keyFile := helpers.GenerateCertificateAndKeyWithIPSAN(
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
			"--bundle", bundleFile,
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

		// First check that the helm chart is accessible with the old certificate.
		helpers.ValidateChartIsAvailable(
			GinkgoT(),
			Default,
			ipAddr.String(),
			port,
			"podinfo",
			"6.2.0",
			helm.CAFileOpt(originalCACertFile),
		)

		helpers.ValidateChartIsAvailable(
			GinkgoT(),
			Default,
			ipAddr.String(),
			port,
			"node-feature-discovery",
			"0.15.2",
			helm.CAFileOpt(originalCACertFile),
		)

		// Backup the original CA file to be used after checking the new CA file works.
		// This is to ensure that the server is definitely using the new certificate.
		backupDir := GinkgoT().TempDir()
		caCertFileName := filepath.Base(originalCACertFile)
		Expect(
			os.Rename(originalCACertFile, filepath.Join(backupDir, caCertFileName)),
		).To(Succeed())
		originalCACertFile = filepath.Join(backupDir, caCertFileName)

		// Create a new certificate. This can happen at any time the server is running,
		// and the server is expected to eventually use the new certificate.
		// This also generates a new CA file which is even better because we can check
		// that the server is using the certificate issued by the new CA.
		newCACertFile, _, _, _ := helpers.GenerateCertificateAndKeyWithIPSAN(
			GinkgoT(),
			tempCertDir,
			ipAddr,
		)

		Eventually(func(g Gomega) {
			helpers.ValidateChartIsAvailable(
				GinkgoT(),
				g,
				ipAddr.String(),
				port,
				"podinfo",
				"6.2.0",
				helm.CAFileOpt(newCACertFile),
			)

			helpers.ValidateChartIsAvailable(
				GinkgoT(),
				g,
				ipAddr.String(),
				port,
				"node-feature-discovery",
				"0.15.2",
				helm.CAFileOpt(newCACertFile),
			)
		}).WithTimeout(time.Second * 5).WithPolling(time.Second * 1).Should(Succeed())

		// Now check that the original CA file is now no longer valid, ensuring that the new
		// certificate is being used by mindthegap serve.
		h, cleanup := helm.NewClient(
			output.NewNonInteractiveShell(GinkgoWriter, GinkgoWriter, 10),
		)
		DeferCleanup(cleanup)
		helmTmpDir := GinkgoT().TempDir()

		_, err = h.GetChartFromRepo(
			helmTmpDir,
			"",
			fmt.Sprintf("%s://%s:%d/charts/%s", helm.OCIScheme, ipAddr.String(), port, "podinfo"),
			"6.2.0",
			helm.CAFileOpt(originalCACertFile),
		)
		Expect(errors.As(err, &x509.UnknownAuthorityError{})).To(BeTrue())

		close(stopCh)

		Eventually(done).Should(BeClosed())
	})

	It("Bundle does not exist", func() {
		cmd.SetArgs([]string{
			"--bundle", bundleFile,
		})

		Expect(
			cmd.Execute(),
		).To(MatchError(fmt.Sprintf("did find any matching files for %q", bundleFile)))
	})
})
