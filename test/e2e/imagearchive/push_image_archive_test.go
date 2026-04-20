// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package imagearchive_test

import (
	"context"
	"fmt"
	"log"
	"net"
	"path/filepath"

	"github.com/go-logr/logr/funcr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/phayes/freeport"
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	pushbundle "github.com/mesosphere/mindthegap/cmd/mindthegap/push/bundle"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/push/imagearchive"
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/images/archive/testutil"
	"github.com/mesosphere/mindthegap/test/e2e/helpers"
)

var _ = Describe("Push Image Archive", func() {
	var (
		cmd *cobra.Command
		tmp string
	)

	BeforeEach(func() {
		tmp = GinkgoT().TempDir()
		cmd = helpers.NewCommand(GinkgoT(), func(out output.Output) *cobra.Command {
			return imagearchive.NewCommand(out)
		})
	})

	startRegistry := func(host string, port int, tls registry.TLS) (*registry.Registry, chan struct{}) {
		reg, err := registry.NewRegistry(registry.Config{
			Storage: registry.FilesystemStorage(filepath.Join(tmp, "registry")),
			Host:    host,
			Port:    uint16(port),
			TLS:     tls,
		})
		Expect(err).NotTo(HaveOccurred())

		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()
			Expect(reg.ListenAndServe(
				funcr.New(func(prefix, args string) { log.Println(prefix, args) }, funcr.Options{}),
			)).To(Succeed())
			close(done)
		}()
		helpers.WaitForTCPPort(GinkgoT(), host, port)
		return reg, done
	}

	It("pushes an OCI image layout tarball", func() {
		archivePath := filepath.Join(tmp, "oci.tar")
		testutil.BuildOCIArchive(GinkgoT(), archivePath, "example.com/app:v1")

		port, err := freeport.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		reg, done := startRegistry("127.0.0.1", port, registry.TLS{})

		cmd.SetArgs([]string{
			"--image-archive", archivePath,
			"--to-registry", fmt.Sprintf("http://127.0.0.1:%d", port),
			"--to-registry-insecure-skip-tls-verify",
		})
		Expect(cmd.Execute()).To(Succeed())

		Expect(reg.Shutdown(context.Background())).To(Succeed())
		Eventually(done).Should(BeClosed())
	})

	It("pushes a docker-save tarball", func() {
		archivePath := filepath.Join(tmp, "docker.tar")
		testutil.BuildDockerArchive(GinkgoT(), archivePath, "example.com/app:v1")

		port, err := freeport.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		reg, done := startRegistry("127.0.0.1", port, registry.TLS{})

		cmd.SetArgs([]string{
			"--image-archive", archivePath,
			"--to-registry", fmt.Sprintf("http://127.0.0.1:%d", port),
			"--to-registry-insecure-skip-tls-verify",
		})
		Expect(cmd.Execute()).To(Succeed())

		Expect(reg.Shutdown(context.Background())).To(Succeed())
		Eventually(done).Should(BeClosed())
	})

	It("rejects an image archive passed to push bundle", func() {
		archivePath := filepath.Join(tmp, "oci.tar")
		testutil.BuildOCIArchive(GinkgoT(), archivePath, "example.com/app:v1")

		bundleCmd := helpers.NewCommand(GinkgoT(), func(out output.Output) *cobra.Command {
			return pushbundle.NewCommand(out, "bundle")
		})
		bundleCmd.SilenceErrors = true
		bundleCmd.SetArgs([]string{
			"--bundle", archivePath,
			"--to-registry", "registry.invalid:1",
		})

		err := bundleCmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("push image-archive"))
	})

	It("pushes a docker-save archive with multiple tags", func() {
		archivePath := filepath.Join(tmp, "multi.tar")
		testutil.BuildDockerArchive(
			GinkgoT(), archivePath,
			"example.com/one:v1", "example.com/two:v2",
		)

		port, err := freeport.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		reg, done := startRegistry("127.0.0.1", port, registry.TLS{})

		cmd.SetArgs([]string{
			"--image-archive", archivePath,
			"--to-registry", fmt.Sprintf("http://127.0.0.1:%d", port),
			"--to-registry-insecure-skip-tls-verify",
		})
		Expect(cmd.Execute()).To(Succeed())

		for _, refStr := range []string{
			fmt.Sprintf("127.0.0.1:%d/one:v1", port),
			fmt.Sprintf("127.0.0.1:%d/two:v2", port),
		} {
			ref, err := name.ParseReference(refStr, name.StrictValidation)
			Expect(err).NotTo(HaveOccurred())
			_, err = remote.Get(ref)
			Expect(err).NotTo(HaveOccurred(),
				"expected %s to be present on destination", refStr)
		}

		Expect(reg.Shutdown(context.Background())).To(Succeed())
		Eventually(done).Should(BeClosed())
	})

	It("pushes a tagless OCI archive with --image-tag override", func() {
		archivePath := filepath.Join(tmp, "tagless.tar")
		testutil.BuildOCIArchive(GinkgoT(), archivePath, "")

		port, err := freeport.GetFreePort()
		Expect(err).NotTo(HaveOccurred())
		reg, done := startRegistry("127.0.0.1", port, registry.TLS{})

		cmd.SetArgs([]string{
			"--image-archive", archivePath,
			"--to-registry", fmt.Sprintf("http://127.0.0.1:%d", port),
			"--to-registry-insecure-skip-tls-verify",
			"--image-tag", "override:v3",
		})
		Expect(cmd.Execute()).To(Succeed())

		ref, err := name.ParseReference(
			fmt.Sprintf("127.0.0.1:%d/override:v3", port),
			name.StrictValidation,
		)
		Expect(err).NotTo(HaveOccurred())
		_, err = remote.Get(ref)
		Expect(err).NotTo(HaveOccurred())

		Expect(reg.Shutdown(context.Background())).To(Succeed())
		Eventually(done).Should(BeClosed())
	})

	DescribeTable(
		"TLS variants",
		func(registryHost, registryScheme string, registryInsecure bool) {
			caCertFile := ""
			certFile := ""
			keyFile := ""
			if registryHost != "127.0.0.1" && registryScheme != "http" {
				certDir := GinkgoT().TempDir()
				caCertFile, _, certFile, keyFile = helpers.GenerateCertificateAndKeyWithIPSAN(
					GinkgoT(), certDir, net.ParseIP(registryHost),
				)
			}

			port, err := freeport.GetFreePort()
			Expect(err).NotTo(HaveOccurred())
			reg, done := startRegistry(registryHost, port, registry.TLS{
				Certificate: certFile,
				Key:         keyFile,
			})

			archivePath := filepath.Join(tmp, "tls.tar")
			testutil.BuildDockerArchive(GinkgoT(), archivePath, "example.com/app:v1")

			toURL := fmt.Sprintf("%s:%d", registryHost, port)
			if registryScheme != "" {
				toURL = fmt.Sprintf("%s://%s", registryScheme, toURL)
			}
			args := []string{
				"--image-archive", archivePath,
				"--to-registry", toURL,
			}
			if registryInsecure {
				args = append(args, "--to-registry-insecure-skip-tls-verify")
			} else if caCertFile != "" {
				args = append(args, "--to-registry-ca-cert-file", caCertFile)
			}
			cmd.SetArgs(args)
			Expect(cmd.Execute()).To(Succeed())

			Expect(reg.Shutdown(context.Background())).To(Succeed())
			Eventually(done).Should(BeClosed())
		},
		Entry("Without TLS (loopback)", "127.0.0.1", "", true),
		Entry("With TLS", helpers.GetPreferredOutboundIP(GinkgoT()).String(), "", false),
		Entry("With Insecure TLS", helpers.GetPreferredOutboundIP(GinkgoT()).String(), "", true),
		Entry("With http scheme", helpers.GetPreferredOutboundIP(GinkgoT()).String(), "http", true),
	)
})
