// Copyright 2021-2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package imagebundle_test

import (
	"context"
	"fmt"
	"net"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/elazarl/goproxy"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/phayes/freeport"
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"
	pushbundle "github.com/mesosphere/mindthegap/cmd/mindthegap/push/bundle"
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/images/httputils"
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

		cmd = helpers.NewCommand(
			GinkgoT(),
			func(out output.Output) *cobra.Command { return pushbundle.NewCommand(out, "image-bundle") },
		)
	})

	runTest := func(
		registryHost string,
		registryScheme string,
		registryInsecure bool,
	) {
		helpers.CreateBundle(
			GinkgoT(),
			bundleFile,
			filepath.Join("testdata", "create-success.yaml"),
		)

		registryCACertFile := ""
		registryCertFile := ""
		registryKeyFile := ""
		if registryHost != "localhost" && registryScheme != "http" {
			tempCertDir := GinkgoT().TempDir()
			registryCACertFile, _, registryCertFile, registryKeyFile = helpers.GenerateCertificateAndKeyWithIPSAN(
				GinkgoT(),
				tempCertDir,
				net.ParseIP(registryHost),
			)
		}

		port, err := freeport.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		reg, err := registry.NewRegistry(registry.Config{
			StorageDirectory: filepath.Join(tmpDir, "registry"),
			Host:             registryHost,
			Port:             uint16(port),
			TLS: registry.TLS{
				Certificate: registryCertFile,
				Key:         registryKeyFile,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()

			Expect(reg.ListenAndServe()).To(Succeed())

			close(done)
		}()

		helpers.WaitForTCPPort(GinkgoT(), registryHost, port)

		registryHostWithOptionalScheme := fmt.Sprintf("%s:%d", registryHost, port)
		if registryScheme != "" {
			registryHostWithOptionalScheme = fmt.Sprintf(
				"%s://%s",
				registryScheme,
				registryHostWithOptionalScheme,
			)
		}

		args := []string{
			"--image-bundle", bundleFile,
			"--to-registry", registryHostWithOptionalScheme,
		}
		if registryInsecure {
			args = append(args, "--to-registry-insecure-skip-tls-verify")
		} else if registryCACertFile != "" {
			args = append(args, "--to-registry-ca-cert-file", registryCACertFile)
		}

		cmd.SetArgs(args)

		Expect(cmd.Execute()).To(Succeed())

		testRoundTripper, err := httputils.TLSConfiguredRoundTripper(
			remote.DefaultTransport,
			net.JoinHostPort(registryHost, strconv.Itoa(port)),
			registryCACertFile != "",
			registryCACertFile,
		)
		Expect(err).NotTo(HaveOccurred())

		helpers.ValidateImageIsAvailable(
			GinkgoT(),
			registryHost,
			port,
			"stefanprodan/podinfo",
			"6.2.0",
			[]*v1.Platform{{
				OS:           "linux",
				Architecture: runtime.GOARCH,
			}},
			remote.WithTransport(testRoundTripper),
		)

		Expect(reg.Shutdown(context.Background())).To((Succeed()))

		Eventually(done).Should(BeClosed())
	}

	DescribeTable("Success",
		runTest,

		Entry("Without TLS", "localhost", "", true),

		Entry("With TLS", helpers.GetFirstNonLoopbackIP(GinkgoT()).String(), "", true),

		Entry("With Insecure TLS", helpers.GetFirstNonLoopbackIP(GinkgoT()).String(), "", true),

		Entry(
			"With http registry",
			helpers.GetFirstNonLoopbackIP(GinkgoT()).String(),
			"http",
			true,
		),

		Entry(
			"With http registry without TLS skip verify flag",
			helpers.GetFirstNonLoopbackIP(GinkgoT()).String(),
			"http",
			false,
		),
	)

	It("Bundle does not exist", func() {
		cmd.SetArgs([]string{
			"--image-bundle", bundleFile,
			"--to-registry", "localhost:0/charts",
			"--to-registry-insecure-skip-tls-verify",
		})

		Expect(
			cmd.Execute(),
		).To(MatchError(fmt.Sprintf("did find any matching files for %q", bundleFile)))
	})

	It("Success using a proxy", Serial, func() {
		proxy := goproxy.NewProxyHttpServer()
		proxy.Verbose = true
		proxy.Logger = GinkgoWriter

		proxyServer := httptest.NewServer(proxy)
		defer proxyServer.Close()

		GinkgoT().Setenv("http_proxy", proxyServer.URL)
		GinkgoT().Setenv("https_proxy", proxyServer.URL)

		runTest(helpers.GetFirstNonLoopbackIP(GinkgoT()).String(), "", false)
	})
})
