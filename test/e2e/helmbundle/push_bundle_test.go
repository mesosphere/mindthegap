// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package helmbundle_test

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/go-logr/logr/funcr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/phayes/freeport"
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	pushbundle "github.com/mesosphere/mindthegap/cmd/mindthegap/push/bundle"
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/helm"
	"github.com/mesosphere/mindthegap/test/e2e/helpers"
)

var _ = Describe("Push Bundle", func() {
	var (
		bundleFile string
		cmd        *cobra.Command
		tmpDir     string
	)

	BeforeEach(func() {
		tmpDir = GinkgoT().TempDir()

		bundleFile = filepath.Join(tmpDir, "helm-bundle.tar")

		cmd = helpers.NewCommand(
			GinkgoT(),
			func(out output.Output) *cobra.Command { return pushbundle.NewCommand(out, "bundle") },
		)
	})

	It("Without TLS", func() {
		helpers.CreateBundleHelmCharts(
			GinkgoT(),
			bundleFile,
			filepath.Join("testdata", "create-success.yaml"),
		)

		port, err := freeport.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		reg, err := registry.NewRegistry(registry.Config{
			Storage: registry.FilesystemStorage(filepath.Join(tmpDir, "registry")),
			Port:    uint16(port),
		})
		Expect(err).NotTo(HaveOccurred())

		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()

			Expect(
				reg.ListenAndServe(
					funcr.New(func(prefix, args string) {
						log.Println(prefix, args)
					}, funcr.Options{}),
				),
			).To(Succeed())

			close(done)
		}()

		helpers.WaitForTCPPort(GinkgoT(), "127.0.0.1", port)

		cmd.SetArgs([]string{
			"--bundle", bundleFile,
			"--to-registry", fmt.Sprintf("127.0.0.1:%d/charts", port),
			"--to-registry-insecure-skip-tls-verify",
		})

		Expect(cmd.Execute()).To(Succeed())

		helpers.ValidateChartIsAvailable(
			GinkgoT(),
			Default,
			"127.0.0.1",
			port,
			"podinfo",
			"6.2.0",
			helm.PlainHTTPOpt(),
		)

		helpers.ValidateChartIsAvailable(
			GinkgoT(),
			Default,
			"127.0.0.1",
			port,
			"node-feature-discovery",
			"0.15.2",
			helm.PlainHTTPOpt(),
		)

		helpers.ValidateChartIsAvailable(
			GinkgoT(),
			Default,
			"127.0.0.1",
			port,
			"podinfo",
			"6.3.0",
			helm.PlainHTTPOpt(),
		)

		Expect(reg.Shutdown(context.Background())).To((Succeed()))

		Eventually(done).Should(BeClosed())
	})

	It("With Insecure TLS", func() {
		helpers.CreateBundleHelmCharts(
			GinkgoT(),
			bundleFile,
			filepath.Join("testdata", "create-success.yaml"),
		)

		ipAddr := helpers.GetPreferredOutboundIP(GinkgoT())

		tempCertDir := GinkgoT().TempDir()
		_, _, certFile, keyFile := helpers.GenerateCertificateAndKeyWithIPSAN(
			GinkgoT(),
			tempCertDir,
			ipAddr,
		)

		port, err := freeport.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		reg, err := registry.NewRegistry(registry.Config{
			Storage: registry.FilesystemStorage(filepath.Join(tmpDir, "registry")),
			Host:    ipAddr.String(),
			Port:    uint16(port),
			TLS: registry.TLS{
				Certificate: certFile,
				Key:         keyFile,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()

			Expect(
				reg.ListenAndServe(
					funcr.New(func(prefix, args string) {
						log.Println(prefix, args)
					}, funcr.Options{}),
				),
			).To(Succeed())

			close(done)
		}()

		helpers.WaitForTCPPort(GinkgoT(), ipAddr.String(), port)

		cmd.SetArgs([]string{
			"--bundle", bundleFile,
			"--to-registry", fmt.Sprintf("%s:%d/charts", ipAddr, port),
			"--to-registry-insecure-skip-tls-verify",
		})

		Expect(cmd.Execute()).To(Succeed())

		// TODO Reenable once Helm supports custom CA certs and self-signed certs.
		// helpers.ValidateChartIsAvailable(GinkgoT(), ipAddr.String(), port, "podinfo", "6.2.0", helm.CAFileOpt(caCertFile))

		Expect(reg.Shutdown(context.Background())).To((Succeed()))

		Eventually(done).Should(BeClosed())
	})

	It("With TLS", func() {
		helpers.CreateBundleHelmCharts(
			GinkgoT(),
			bundleFile,
			filepath.Join("testdata", "create-success.yaml"),
		)

		ipAddr := helpers.GetPreferredOutboundIP(GinkgoT())

		tempCertDir := GinkgoT().TempDir()
		caCertFile, _, certFile, keyFile := helpers.GenerateCertificateAndKeyWithIPSAN(
			GinkgoT(),
			tempCertDir,
			ipAddr,
		)

		port, err := freeport.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		reg, err := registry.NewRegistry(registry.Config{
			Storage: registry.FilesystemStorage(filepath.Join(tmpDir, "registry")),
			Host:    ipAddr.String(),
			Port:    uint16(port),
			TLS: registry.TLS{
				Certificate: certFile,
				Key:         keyFile,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()

			Expect(
				reg.ListenAndServe(
					funcr.New(func(prefix, args string) {
						log.Println(prefix, args)
					}, funcr.Options{}),
				),
			).To(Succeed())

			close(done)
		}()

		helpers.WaitForTCPPort(GinkgoT(), ipAddr.String(), port)

		cmd.SetArgs([]string{
			"--bundle", bundleFile,
			"--to-registry", fmt.Sprintf("%s:%d/charts", ipAddr, port),
			"--to-registry-ca-cert-file", caCertFile,
		})

		Expect(cmd.Execute()).To(Succeed())

		// TODO Reenable once Helm supports custom CA certs and self-signed certs.
		// helpers.ValidateChartIsAvailable(GinkgoT(), ipAddr.String(), port, "podinfo", "6.2.0", helm.CAFileOpt(caCertFile))

		Expect(reg.Shutdown(context.Background())).To((Succeed()))

		Eventually(done).Should(BeClosed())
	})

	It("Bundle does not exist", func() {
		cmd.SetArgs([]string{
			"--bundle", bundleFile,
			"--to-registry", "127.0.0.1:0/charts",
			"--to-registry-insecure-skip-tls-verify",
		})

		Expect(
			cmd.Execute(),
		).To(MatchError(fmt.Sprintf("did find any matching files for %q", bundleFile)))
	})
})
