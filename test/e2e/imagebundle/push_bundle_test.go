// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package imagebundle_test

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/distribution/distribution/v3/manifest/manifestlist"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/phayes/freeport"
	"github.com/spf13/cobra"

	pushimagebundle "github.com/mesosphere/mindthegap/cmd/mindthegap/push/imagebundle"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/utils"
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/skopeo"
	"github.com/mesosphere/mindthegap/test/e2e/imagebundle/helpers"
)

var _ = Describe("Push Bundle", func() {
	var (
		bundleFile string
		cmd        *cobra.Command
		tmpDir     string
	)

	BeforeEach(func() {
		tmpDir = GinkgoT().TempDir()

		bundleFile = filepath.Join(tmpDir, "image-bundle.tar")

		cmd = helpers.NewCommand(GinkgoT(), pushimagebundle.NewCommand)
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
			"--image-bundle", bundleFile,
			"--to-registry", fmt.Sprintf("localhost:%d", port),
			"--to-registry-insecure-skip-tls-verify",
		})

		Expect(cmd.Execute()).To(Succeed())

		helpers.ValidateImageIsAvailable(
			GinkgoT(),
			"localhost",
			port,
			"stefanprodan/podinfo",
			"6.2.0",
			[]manifestlist.PlatformSpec{{
				OS:           "linux",
				Architecture: runtime.GOARCH,
			}},
			skopeo.DisableTLSVerify(),
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
		caCertFile, _, certFile, keyFile := helpers.GenerateCertificateAndKeyWithIPSAN(
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
			"--image-bundle", bundleFile,
			"--to-registry", fmt.Sprintf("%s:%d", ipAddr, port),
			"--to-registry-ca-cert-file", caCertFile,
		})

		Expect(cmd.Execute()).To(Succeed())

		tmpCACertDir := GinkgoT().TempDir()
		err = utils.CopyFile(
			caCertFile,
			filepath.Join(tmpCACertDir, filepath.Base(caCertFile)),
		)
		Expect(err).NotTo(HaveOccurred())

		helpers.ValidateImageIsAvailable(
			GinkgoT(),
			ipAddr.String(),
			port,
			"stefanprodan/podinfo",
			"6.2.0",
			[]manifestlist.PlatformSpec{{
				OS:           "linux",
				Architecture: runtime.GOARCH,
			}},
			skopeo.CertDir(tmpCACertDir),
		)

		Expect(reg.Shutdown(context.Background())).To((Succeed()))

		Eventually(done).Should(BeClosed())
	})

	It("Bundle does not exist", func() {
		cmd.SetArgs([]string{
			"--image-bundle", bundleFile,
			"--to-registry", "localhost:unused/charts",
			"--to-registry-insecure-skip-tls-verify",
		})

		Expect(
			cmd.Execute(),
		).To(MatchError(fmt.Sprintf("did find any matching files for %q", bundleFile)))
	})
})
