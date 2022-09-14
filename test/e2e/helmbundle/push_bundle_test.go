// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e
// +build e2e

package helmbundle_test

import (
	"context"
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/phayes/freeport"
	"github.com/spf13/cobra"

	pushhelmbundle "github.com/mesosphere/mindthegap/cmd/mindthegap/push/helmbundle"
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/helm"
	"github.com/mesosphere/mindthegap/test/e2e/helmbundle/helpers"
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

		cmd = helpers.NewCommand(GinkgoT(), pushhelmbundle.NewCommand)
	})

	It("Without TLS", func() {
		helpers.CreateBundle(
			GinkgoT(),
			bundleFile,
			filepath.Join("testdata", "create-success.yaml"),
		)

		port, err := freeport.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		reg, err := registry.NewRegistry(registry.Config{
			StorageDirectory: filepath.Join(tmpDir, "registry"),
			Port:             uint16(port),
		})
		Expect(err).NotTo(HaveOccurred())

		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()

			Expect(reg.ListenAndServe()).To(Succeed())

			close(done)
		}()

		helpers.WaitForTCPPort(GinkgoT(), "localhost", port)

		cmd.SetArgs([]string{
			"--helm-bundle", bundleFile,
			"--to-registry", fmt.Sprintf("localhost:%v/charts", port),
			"--to-registry-insecure-skip-tls-verify",
		})

		Expect(cmd.Execute()).To(Succeed())

		helpers.ValidateChartIsAvailable(
			GinkgoT(),
			"localhost",
			port,
			"podinfo",
			"6.2.0",
			helm.InsecureSkipTLSverifyOpt(),
		)

		Expect(reg.Shutdown(context.Background())).To((Succeed()))

		Eventually(done).Should(BeClosed())
	})

	It("With TLS", func() {
		helpers.CreateBundle(
			GinkgoT(),
			bundleFile,
			filepath.Join("testdata", "create-success.yaml"),
		)

		ipAddr := helpers.GetFirstNonLoopbackIP(GinkgoT())

		tempCertDir := GinkgoT().TempDir()
		_, _, certFile, keyFile := helpers.GenerateCertificateAndKeyWithIPSAN(
			GinkgoT(),
			tempCertDir,
			ipAddr,
		)

		port, err := freeport.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		reg, err := registry.NewRegistry(registry.Config{
			StorageDirectory: filepath.Join(tmpDir, "registry"),
			Host:             ipAddr.String(),
			Port:             uint16(port),
			TLS: registry.TLS{
				Certificate: certFile,
				Key:         keyFile,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()

			Expect(reg.ListenAndServe()).To(Succeed())

			close(done)
		}()

		helpers.WaitForTCPPort(GinkgoT(), ipAddr.String(), port)

		cmd.SetArgs([]string{
			"--helm-bundle", bundleFile,
			"--to-registry", fmt.Sprintf("%s:%v/charts", ipAddr, port),
			"--to-registry-insecure-skip-tls-verify",
		})

		Expect(cmd.Execute()).To(Succeed())

		// TODO Reenable once Helm supports custom CA certs and self-signed certs.
		// helpers.ValidateChartIsAvailable(GinkgoT(), ipAddr.String(), port, "podinfo", "6.2.0", helm.CAFileOpt(caCertFile))

		Expect(reg.Shutdown(context.Background())).To((Succeed()))

		Eventually(done).Should(BeClosed())
	})

	It("Bundle does not exist", func() {
		cmd.SetArgs([]string{
			"--helm-bundle", bundleFile,
			"--to-registry", "localhost:unused/charts",
			"--to-registry-insecure-skip-tls-verify",
		})

		Expect(
			cmd.Execute(),
		).To(MatchError(fmt.Sprintf("did find any matching files for %q", bundleFile)))
	})
})
