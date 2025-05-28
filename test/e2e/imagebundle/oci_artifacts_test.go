// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package imagebundle_test

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/go-logr/logr/funcr"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/phayes/freeport"
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	createbundle "github.com/mesosphere/mindthegap/cmd/mindthegap/create/bundle"
	pushbundle "github.com/mesosphere/mindthegap/cmd/mindthegap/push/bundle"
	servebundle "github.com/mesosphere/mindthegap/cmd/mindthegap/serve/bundle"
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/images/httputils"
	"github.com/mesosphere/mindthegap/test/e2e/helpers"
)

var _ = Describe("OCI artifacts support", func() {
	var (
		bundleFile           string
		cmd                  *cobra.Command
		stopCh               chan struct{}
		expectedOCIArtifacts = []string{
			"stefanprodan/manifests/podinfo:6.8.0",
			"stefanprodan/charts/podinfo:6.8.0",
			"mesosphere/kommander-applications:v2.14.0",
		}
	)

	BeforeEach(func() {
		tmpDir := GinkgoT().TempDir()

		bundleFile = filepath.Join(tmpDir, "image-bundle.tar")

		cmd = helpers.NewCommand(GinkgoT(), func(out output.Output) *cobra.Command {
			var c *cobra.Command
			c, stopCh = servebundle.NewCommand(out, "bundle")
			return c
		})
	})

	It("supports various oci artifacts media types", func() {
		helpers.CreateBundleOCI(
			GinkgoT(),
			bundleFile,
			filepath.Join("testdata", "create-success-oci.yaml"),
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

		for _, imageRef := range expectedOCIArtifacts {
			mindthegapRef := fmt.Sprintf("127.0.0.1:%d/%s", port, imageRef)
			ref, err := name.ParseReference(mindthegapRef)
			Expect(err).NotTo(HaveOccurred())
			_, err = remote.Image(ref)
			Expect(err).NotTo(HaveOccurred())
		}

		close(stopCh)

		Eventually(done).Should(BeClosed())
	})

	It("bundles OCI artifacts with repositories prefix", func() {
		ipAddr := helpers.GetPreferredOutboundIP(GinkgoT())

		tempCertDir := GinkgoT().TempDir()
		caCertFile, _, certFile, keyFile := helpers.GenerateCertificateAndKeyWithIPSAN(
			GinkgoT(),
			tempCertDir,
			ipAddr,
		)

		helpers.CreateBundleOCI(
			GinkgoT(),
			bundleFile,
			filepath.Join("testdata", "create-success-oci.yaml"),
		)

		port, err := freeport.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		cmd.SetArgs([]string{
			"--bundle", bundleFile,
			"--listen-address", ipAddr.String(),
			"--listen-port", strconv.Itoa(port),
			"--tls-cert-file", certFile,
			"--tls-private-key-file", keyFile,
			"--repositories-prefix", "/some/test/prefix",
		})

		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()

			Expect(cmd.Execute()).To(Succeed())

			close(done)
		}()

		helpers.WaitForTCPPort(GinkgoT(), ipAddr.String(), port)

		testRoundTripper, err := httputils.TLSConfiguredRoundTripper(
			remote.DefaultTransport,
			net.JoinHostPort(ipAddr.String(), strconv.Itoa(port)),
			false,
			caCertFile,
		)
		Expect(err).NotTo(HaveOccurred())

		for _, imageRef := range expectedOCIArtifacts {
			mindthegapRef := fmt.Sprintf("%s:%d%s/%s", ipAddr, port, "/some/test/prefix", imageRef)
			ref, err := name.ParseReference(mindthegapRef)
			Expect(err).NotTo(HaveOccurred())
			_, err = remote.Image(ref, remote.WithTransport(testRoundTripper))
			Expect(err).NotTo(HaveOccurred())
		}

		close(stopCh)

		Eventually(done).Should(BeClosed())
	})

	It("failes to create bundle with OCI image in OCI artifacts list", func() {
		imagesTxt := filepath.Join(GinkgoT().TempDir(), "oic-images.txt")
		Expect(os.WriteFile(imagesTxt, []byte("stefanprodan/podinfo:6.8.0"), 0o644)).To(Succeed())
		createBundleCmd := helpers.NewCommand(GinkgoT(), createbundle.NewCommand)
		createBundleCmd.SetArgs([]string{
			"--output-file", bundleFile,
			"--oci-artifacts-file", imagesTxt,
		})
		Expect(
			createBundleCmd.Execute(),
		).To(MatchError(ContainSubstring("unexpected media type in descriptor for OCI artifact")))
	})

	It("pushes OCI artifacts from OCI artfacts only bundle to the registry", func() {
		helpers.CreateBundleOCI(
			GinkgoT(),
			bundleFile,
			filepath.Join("testdata", "create-success-oci.yaml"),
		)

		port, err := freeport.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		reg, err := registry.NewRegistry(registry.Config{
			Storage: registry.FilesystemStorage(filepath.Join(GinkgoT().TempDir(), "registry")),
			Host:    "127.0.0.1",
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

		cmd := helpers.NewCommand(
			GinkgoT(),
			func(out output.Output) *cobra.Command { return pushbundle.NewCommand(out, "bundle") },
		)
		args := []string{
			"--bundle", bundleFile,
			"--to-registry", fmt.Sprintf("127.0.0.1:%d", port),
		}
		cmd.SetArgs(args)
		Expect(cmd.Execute()).To(Succeed())

		for _, imageRef := range expectedOCIArtifacts {
			mindthegapRef := fmt.Sprintf("127.0.0.1:%d/%s", port, imageRef)
			ref, err := name.ParseReference(mindthegapRef)
			Expect(err).NotTo(HaveOccurred())
			_, err = remote.Image(ref)
			Expect(err).NotTo(HaveOccurred())
		}

		Expect(reg.Shutdown(context.Background())).To((Succeed()))
		Eventually(done).Should(BeClosed())
	})

	It("pushes images from mixes bundle with OCI images and OCI artfacts to the registry", func() {
		helpers.CreateBundleOCIAndImages(
			GinkgoT(),
			bundleFile,
			filepath.Join("testdata", "create-success-oci.yaml"),
			filepath.Join("testdata", "create-success.yaml"),
			"linux/"+runtime.GOARCH,
		)

		port, err := freeport.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		reg, err := registry.NewRegistry(registry.Config{
			Storage: registry.FilesystemStorage(filepath.Join(GinkgoT().TempDir(), "registry")),
			Host:    "127.0.0.1",
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

		cmd := helpers.NewCommand(
			GinkgoT(),
			func(out output.Output) *cobra.Command { return pushbundle.NewCommand(out, "bundle") },
		)
		args := []string{
			"--bundle", bundleFile,
			"--to-registry", fmt.Sprintf("127.0.0.1:%d", port),
		}
		cmd.SetArgs(args)
		Expect(cmd.Execute()).To(Succeed())

		for _, imageRef := range expectedOCIArtifacts {
			mindthegapRef := fmt.Sprintf("127.0.0.1:%d/%s", port, imageRef)
			ref, err := name.ParseReference(mindthegapRef)
			Expect(err).NotTo(HaveOccurred())
			_, err = remote.Image(ref)
			Expect(err).NotTo(HaveOccurred())
		}

		helpers.ValidateImageIsAvailable(
			GinkgoT(),
			"127.0.0.1",
			port,
			"",
			"stefanprodan/podinfo",
			"6.2.0",
			[]*v1.Platform{{
				OS:           "linux",
				Architecture: runtime.GOARCH,
			}},
			false,
		)

		Expect(reg.Shutdown(context.Background())).To((Succeed()))
		Eventually(done).Should(BeClosed())
	})
})
