// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package imagebundle_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/docker/cli/cli/config"
	"github.com/elazarl/goproxy"
	"github.com/go-logr/logr/funcr"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/phayes/freeport"
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/archive"
	createbundle "github.com/mesosphere/mindthegap/cmd/mindthegap/create/bundle"
	pushbundle "github.com/mesosphere/mindthegap/cmd/mindthegap/push/bundle"
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/images/httputils"
	"github.com/mesosphere/mindthegap/test/e2e/helpers"
)

var _ = Describe("Push Bundle", func() {
	var (
		bundleFile string
		cmd        *cobra.Command
		tmpDir     string
	)

	Context("Success", func() {
		BeforeEach(func() {
			tmpDir = GinkgoT().TempDir()

			bundleFile = filepath.Join(tmpDir, "image-bundle.tar")

			cmd = helpers.NewCommand(
				GinkgoT(),
				func(out output.Output) *cobra.Command { return pushbundle.NewCommand(out, "bundle") },
			)
		})

		runTest := func(
			registryHost string,
			registryScheme string,
			registryPath string,
			registryInsecure bool,
			forceOCIMediaTypes bool,
		) {
			registryCACertFile := ""
			registryCertFile := ""
			registryKeyFile := ""
			if registryHost != "127.0.0.1" && registryScheme != "http" {
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
				Storage: registry.FilesystemStorage(filepath.Join(tmpDir, "registry")),
				Host:    registryHost,
				Port:    uint16(port),
				TLS: registry.TLS{
					Certificate: registryCertFile,
					Key:         registryKeyFile,
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

			helpers.WaitForTCPPort(GinkgoT(), registryHost, port)

			registryHostWithOptionalScheme := fmt.Sprintf(
				"%s:%d/%s",
				registryHost,
				port,
				strings.TrimLeft(registryPath, "/"),
			)
			if registryScheme != "" {
				registryHostWithOptionalScheme = fmt.Sprintf(
					"%s://%s",
					registryScheme,
					registryHostWithOptionalScheme,
				)
			}

			args := []string{
				"--bundle", bundleFile,
				"--to-registry", registryHostWithOptionalScheme,
			}
			if registryInsecure {
				args = append(args, "--to-registry-insecure-skip-tls-verify")
			} else if registryCACertFile != "" {
				args = append(args, "--to-registry-ca-cert-file", registryCACertFile)
			}

			if forceOCIMediaTypes {
				args = append(args, "--force-oci-media-types")
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
				registryPath,
				"stefanprodan/podinfo",
				"6.2.0",
				[]*v1.Platform{{
					OS:           "linux",
					Architecture: runtime.GOARCH,
				}},
				forceOCIMediaTypes,
				remote.WithTransport(testRoundTripper),
			)

			Expect(reg.Shutdown(context.Background())).To((Succeed()))

			Eventually(done).Should(BeClosed())
		}

		DescribeTable(
			"Success",
			func(
				registryHost string,
				registryScheme string,
				registryPath string,
				registryInsecure bool,
				forceOCIMediaTypes bool,
			) {
				helpers.CreateBundleImages(
					GinkgoT(),
					bundleFile,
					filepath.Join("testdata", "create-success.yaml"),
					"linux/"+runtime.GOARCH,
				)

				runTest(
					registryHost,
					registryScheme,
					registryPath,
					registryInsecure,
					forceOCIMediaTypes,
				)
			},

			Entry("Without TLS", "127.0.0.1", "", "", true, false),

			Entry(
				"With TLS",
				helpers.GetPreferredOutboundIP(GinkgoT()).String(),
				"",
				"",
				false,
				false,
			),

			Entry(
				"With Insecure TLS",
				helpers.GetPreferredOutboundIP(GinkgoT()).String(),
				"",
				"",
				true,
				false,
			),

			Entry(
				"With http registry",
				helpers.GetPreferredOutboundIP(GinkgoT()).String(),
				"http",
				"",
				true,
				false,
			),

			Entry(
				"With http registry without TLS skip verify flag",
				helpers.GetPreferredOutboundIP(GinkgoT()).String(),
				"http",
				"",
				false,
				false,
			),

			Entry(
				"With Subpath",
				helpers.GetPreferredOutboundIP(GinkgoT()).String(),
				"",
				"/nested/path/for/registry",
				false,
				false,
			),

			Entry(
				"With force OCI media types",
				helpers.GetPreferredOutboundIP(GinkgoT()).String(),
				"",
				"",
				false,
				true,
			),
		)

		Context("With multiple image bundles", Ordered, func() {
			var (
				bundleFile2     string
				registryAddress string
				registryPort    int
				outputBuf       *bytes.Buffer
			)

			BeforeAll(func() {
				port, err := freeport.GetFreePort()
				Expect(err).NotTo(HaveOccurred())
				registryPort = port
				reg, err := registry.NewRegistry(registry.Config{
					Storage: registry.FilesystemStorage(filepath.Join(tmpDir, "registry")),
					Host:    "127.0.0.1",
					Port:    uint16(registryPort),
				})
				Expect(err).NotTo(HaveOccurred())
				registryAddress = fmt.Sprintf("http://127.0.0.1:%d", registryPort)

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

				DeferCleanup(func() {
					Expect(reg.Shutdown(context.Background())).To((Succeed()))

					Eventually(done).Should(BeClosed())
				})

				helpers.WaitForTCPPort(GinkgoT(), "127.0.0.1", port)
			})

			BeforeEach(func() {
				helpers.CreateBundleImages(
					GinkgoT(),
					bundleFile,
					filepath.Join("testdata", "create-success.yaml"),
					"linux/"+runtime.GOARCH,
				)

				bundleFile2 = filepath.Join(tmpDir, "busybox-image-bundle.tar")
				helpers.CreateBundleImages(
					GinkgoT(),
					bundleFile2,
					filepath.Join("testdata", "create-success-busybox.yaml"),
					"linux/"+runtime.GOARCH,
				)

				DeferCleanup(GinkgoWriter.ClearTeeWriters)
				outputBuf = bytes.NewBuffer(nil)
				GinkgoWriter.TeeTo(outputBuf)
			})

			It("Success with multiple image bundles", func() {
				args := []string{
					"--bundle", bundleFile,
					"--bundle", bundleFile2,
					"--to-registry", registryAddress,
					"--to-registry-insecure-skip-tls-verify",
				}

				cmd.SetArgs(args)

				Expect(cmd.Execute()).To(Succeed())

				Expect(outputBuf.String()).NotTo(ContainSubstring("✗"))
			})
		})

		Context("Success for all-platforms", func() {
			var (
				registryAddress string
				registryPort    int
			)

			busyboxAllPlatformsManifest := map[*v1.Platform]string{
				{
					OS:           "linux",
					Architecture: "amd64",
				}: "sha256:a365663cd8c362ff782b6c42ca36a7868443098283442694adb0d348d40d7327",
				{
					OS:           "linux",
					Architecture: "arm",
					Variant:      "v6",
				}: "sha256:359c15b7fec7eb5759c08d0d93f389b442b51954975b3c165a7f8ab4a8815df3",
				{
					OS:           "linux",
					Architecture: "arm",
					Variant:      "v7",
				}: "sha256:a8188a0498d60905f5aafe19199fe05bc026018c419075cd54f3170252e804e2",
				{
					OS:           "linux",
					Architecture: "arm64",
					Variant:      "v8",
				}: "sha256:fcbf1c0057103af4d375c8fc4f863821edc2223124adbb924c586790c53d009d",
				{
					OS:           "linux",
					Architecture: "386",
				}: "sha256:e6030835df7fb6514e08929009cef5175557237805dd62cdb67db94201f8af09",
				{
					OS:           "linux",
					Architecture: "ppc64le",
				}: "sha256:310db88bf4598613680c4f923fdbad11ec1bdc6a592479517f8aac0a05212db3",
				{
					OS:           "linux",
					Architecture: "riscv64",
				}: "sha256:a0bd2930f338a6edf221649c0dff1a1d35bff4b9035a2c50097d8b63599e9cb0",
				{
					OS:           "linux",
					Architecture: "s390x",
				}: "sha256:510295b91981d4672e016a7eec849806c26ef46399efab85b0c5adf175cf93c1",
			}

			BeforeEach(func() {
				port, err := freeport.GetFreePort()
				Expect(err).NotTo(HaveOccurred())
				registryPort = port
				reg, err := registry.NewRegistry(registry.Config{
					Storage: registry.FilesystemStorage(filepath.Join(tmpDir, "registry")),
					Host:    "127.0.0.1",
					Port:    uint16(registryPort),
				})
				Expect(err).NotTo(HaveOccurred())
				registryAddress = fmt.Sprintf("http://127.0.0.1:%d", registryPort)

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

				DeferCleanup(func() {
					Expect(reg.Shutdown(context.Background())).To((Succeed()))

					Eventually(done).Should(BeClosed())
				})

				helpers.WaitForTCPPort(GinkgoT(), "127.0.0.1", port)
			})

			It("Successful push with all-platforms specified via */*", func() {
				allPlatformsBundleFile := filepath.Join(tmpDir, "all-platforms-image-bundle.tar")
				helpers.CreateBundleImages(
					GinkgoT(),
					allPlatformsBundleFile,
					filepath.Join("testdata", "create-success-busybox.yaml"),
					"*/*",
				)
				args := []string{
					"--bundle", allPlatformsBundleFile,
					"--to-registry", registryAddress,
					"--to-registry-insecure-skip-tls-verify",
				}

				cmd.SetArgs(args)
				Expect(cmd.Execute()).To(Succeed())

				helpers.ValidatePlatformDigestsInIndex(
					GinkgoT(),
					"127.0.0.1",
					registryPort,
					"",
					"library/busybox",
					"1.37.0-musl",
					busyboxAllPlatformsManifest,
				)
			})

			It("Successful push with all-platforms flag", func() {
				allPlatformsBundleFile := filepath.Join(tmpDir, "all-platforms-image-bundle.tar")
				createBundleCmd := helpers.NewCommand(GinkgoT(), createbundle.NewCommand)
				createBundleCmd.SetArgs([]string{
					"--output-file", allPlatformsBundleFile,
					"--images-file", filepath.Join("testdata", "create-success-busybox.yaml"),
					"--all-platforms",
				})
				Expect(createBundleCmd.Execute()).To(Succeed())

				args := []string{
					"--bundle", allPlatformsBundleFile,
					"--to-registry", registryAddress,
					"--to-registry-insecure-skip-tls-verify",
				}

				cmd.SetArgs(args)
				Expect(cmd.Execute()).To(Succeed())

				helpers.ValidatePlatformDigestsInIndex(
					GinkgoT(),
					"127.0.0.1",
					registryPort,
					"",
					"library/busybox",
					"1.37.0-musl",
					busyboxAllPlatformsManifest,
				)
			})
		})

		Context("Success with merged bundle files", func() {
			var (
				registryAddress string
				registryPort    int
			)

			BeforeEach(func() {
				port, err := freeport.GetFreePort()
				Expect(err).NotTo(HaveOccurred())
				registryPort = port
				reg, err := registry.NewRegistry(registry.Config{
					Storage: registry.FilesystemStorage(filepath.Join(tmpDir, "registry")),
					Host:    "127.0.0.1",
					Port:    uint16(registryPort),
				})
				Expect(err).NotTo(HaveOccurred())
				registryAddress = fmt.Sprintf("http://127.0.0.1:%d", registryPort)

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

				DeferCleanup(func() {
					Expect(reg.Shutdown(context.Background())).To((Succeed()))

					Eventually(done).Should(BeClosed())
				})

				helpers.WaitForTCPPort(GinkgoT(), "127.0.0.1", port)
			})

			It("Successful push", func() {
				mergedBundleFile := filepath.Join(tmpDir, "merged-bundle.tar")

				helpers.CreateBundleImages(
					GinkgoT(),
					mergedBundleFile,
					filepath.Join("testdata", "create-success.yaml"),
					"linux/amd64",
				)

				createMergedBundleCmd := helpers.NewCommand(GinkgoT(), createbundle.NewCommand)
				createMergedBundleCmd.SetArgs([]string{
					"--output-file", mergedBundleFile,
					"--images-file", filepath.Join("testdata", "create-success-busybox.yaml"),
					"--platform", "linux/amd64",
					"--merge",
				})
				Expect(createMergedBundleCmd.Execute()).To(Succeed())

				createMergedBundleCmd.SetArgs([]string{
					"--output-file", mergedBundleFile,
					"--images-file", filepath.Join("testdata", "create-success-extra-podinfo-tag.yaml"),
					"--platform", "linux/amd64",
					"--merge",
				})
				Expect(createMergedBundleCmd.Execute()).To(Succeed())

				args := []string{
					"--bundle", mergedBundleFile,
					"--to-registry", registryAddress,
					"--to-registry-insecure-skip-tls-verify",
				}

				cmd.SetArgs(args)
				Expect(cmd.Execute()).To(Succeed())

				helpers.ValidatePlatformDigestsInIndex(
					GinkgoT(),
					"127.0.0.1",
					registryPort,
					"",
					"library/busybox",
					"1.37.0-musl",
					map[*v1.Platform]string{
						{
							OS:           "linux",
							Architecture: "amd64",
						}: "sha256:a365663cd8c362ff782b6c42ca36a7868443098283442694adb0d348d40d7327",
					},
				)

				helpers.ValidatePlatformDigestsInIndex(
					GinkgoT(),
					"127.0.0.1",
					registryPort,
					"",
					"stefanprodan/podinfo",
					"6.2.0",
					map[*v1.Platform]string{
						{
							OS:           "linux",
							Architecture: "amd64",
						}: "sha256:f60e14b08375a64528113dd8808b16030c771f626e66961dfaf511b74d6f68dc",
					},
				)

				helpers.ValidatePlatformDigestsInIndex(
					GinkgoT(),
					"127.0.0.1",
					registryPort,
					"",
					"stefanprodan/podinfo",
					"6.3.0",
					map[*v1.Platform]string{
						{
							OS:           "linux",
							Architecture: "amd64",
						}: "sha256:cbf799da7ce644b32b8bbd9440abe9a47378f8018850b314b3561230bcdc128c",
					},
				)
			})
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

		It("Bundle file exists but is a compressed tarball", func() {
			tmpDir := GinkgoT().TempDir()
			nonBundleFile := filepath.Join(tmpDir, "image-bundle.tar.gz")
			Expect(archive.ArchiveDirectory("testdata", nonBundleFile)).To(Succeed())

			cmd.SetArgs([]string{
				"--bundle", nonBundleFile,
				"--to-registry", "127.0.0.1:0/charts",
				"--to-registry-insecure-skip-tls-verify",
			})

			Expect(
				cmd.Execute(),
			).To(MatchError(ContainSubstring("compressed tar archives (.tar.gz) are not supported")))
		})

		Context("With proxy", Serial, func() {
			BeforeEach(func() {
				helpers.CreateBundleImages(
					GinkgoT(),
					bundleFile,
					filepath.Join("testdata", "create-success.yaml"),
					"linux/"+runtime.GOARCH,
				)
			})

			JustBeforeEach(func() {
				proxy := goproxy.NewProxyHttpServer()
				proxy.Verbose = true
				proxy.Logger = GinkgoWriter

				proxyServer := httptest.NewServer(proxy)
				DeferCleanup(proxyServer.Close)

				GinkgoT().Setenv("http_proxy", proxyServer.URL)
				GinkgoT().Setenv("https_proxy", proxyServer.URL)
			})

			It("Success", func() {
				runTest(helpers.GetPreferredOutboundIP(GinkgoT()).String(), "", "", false, false)
			})

			Context("With headers from Docker config", func() {
				BeforeEach(func() {
					dockerConfigDir := GinkgoT().TempDir()
					DeferCleanup(os.Setenv, "DOCKER_CONFIG", os.Getenv("DOCKER_CONFIG"))
					Expect(os.Setenv("DOCKER_CONFIG", dockerConfigDir)).To(Succeed())
					err := os.WriteFile(
						filepath.Join(dockerConfigDir, config.ConfigFileName),
						[]byte(`{
			"HttpHeaders": {
				"MyHeader": "MyValue"
			}
		}`),
						0o644,
					)
					Expect(err).NotTo(HaveOccurred())
				})

				It("Success", func() {
					runTest(helpers.GetPreferredOutboundIP(GinkgoT()).String(), "", "", false, false)
				})
			})
		})

		Context("On existing tag", Ordered, func() {
			var (
				registryAddress string
				outputBuf       *bytes.Buffer
			)

			BeforeAll(func() {
				port, err := freeport.GetFreePort()
				Expect(err).NotTo(HaveOccurred())
				reg, err := registry.NewRegistry(registry.Config{
					Storage: registry.FilesystemStorage(filepath.Join(tmpDir, "registry")),
					Host:    "127.0.0.1",
					Port:    uint16(port),
				})
				Expect(err).NotTo(HaveOccurred())
				registryAddress = fmt.Sprintf("http://127.0.0.1:%d", port)

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

				DeferCleanup(func() {
					Expect(reg.Shutdown(context.Background())).To((Succeed()))

					Eventually(done).Should(BeClosed())
				})

				helpers.WaitForTCPPort(GinkgoT(), "127.0.0.1", port)
			})

			BeforeEach(func() {
				helpers.CreateBundleImages(
					GinkgoT(),
					bundleFile,
					filepath.Join("testdata", "create-success.yaml"),
					"linux/"+runtime.GOARCH,
				)

				DeferCleanup(GinkgoWriter.ClearTeeWriters)
				outputBuf = bytes.NewBuffer(nil)
				GinkgoWriter.TeeTo(outputBuf)
			})

			It(
				"Successful push with explicit --on-existing-tag=skip flag even though doesn't exist yet",
				func() {
					args := []string{
						"--bundle", bundleFile,
						"--to-registry", registryAddress,
						"--to-registry-insecure-skip-tls-verify",
						"--on-existing-tag=skip",
						"--image-push-concurrency=4",
					}

					cmd.SetArgs(args)

					Expect(cmd.Execute()).To(Succeed())

					Expect(outputBuf.String()).NotTo(ContainSubstring("✗"))
				},
			)

			It("Successful push without on-existing-tag flag (default to overwrite)", func() {
				args := []string{
					"--bundle", bundleFile,
					"--to-registry", registryAddress,
					"--to-registry-insecure-skip-tls-verify",
					"--image-push-concurrency=4",
				}

				cmd.SetArgs(args)

				Expect(cmd.Execute()).To(Succeed())

				Expect(outputBuf.String()).NotTo(ContainSubstring("✗"))
			})

			It("Successful push with explicit --on-existing-tag=overwrite", func() {
				args := []string{
					"--bundle", bundleFile,
					"--to-registry", registryAddress,
					"--to-registry-insecure-skip-tls-verify",
					"--on-existing-tag=overwrite",
					"--image-push-concurrency=4",
				}

				cmd.SetArgs(args)

				Expect(cmd.Execute()).To(Succeed())

				Expect(outputBuf.String()).NotTo(ContainSubstring("✗"))
			})

			It("Successful push with explicit --on-existing-tag=skip", func() {
				args := []string{
					"--bundle", bundleFile,
					"--to-registry", registryAddress,
					"--to-registry-insecure-skip-tls-verify",
					"--on-existing-tag=skip",
					"--image-push-concurrency=4",
				}

				cmd.SetArgs(args)

				Expect(cmd.Execute()).To(Succeed())

				Expect(outputBuf.String()).NotTo(ContainSubstring("✗"))
			})

			It("Failed push with explicit --on-existing-tag=error", func() {
				args := []string{
					"--bundle", bundleFile,
					"--to-registry", registryAddress,
					"--to-registry-insecure-skip-tls-verify",
					"--on-existing-tag=error",
					"--image-push-concurrency=4",
				}

				cmd.SetArgs(args)

				Expect(
					cmd.Execute(),
				).To(MatchError(ContainSubstring("image tag already exists in destination registry")))
			})
		})

		Context("Merge existing", Ordered, func() {
			var (
				registryAddress                  string
				outputBuf                        *bytes.Buffer
				arm64BundleFile, amd64BundleFile string
				registryHost                     string
				registryPort                     int
			)

			BeforeAll(func() {
				port, err := freeport.GetFreePort()
				Expect(err).NotTo(HaveOccurred())
				reg, err := registry.NewRegistry(registry.Config{
					Storage: registry.FilesystemStorage(filepath.Join(tmpDir, "registry")),
					Host:    "127.0.0.1",
					Port:    uint16(port),
				})
				registryHost = "127.0.0.1"
				registryPort = port
				registryAddress = fmt.Sprintf("http://127.0.0.1:%d", port)

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

				DeferCleanup(func() {
					Expect(reg.Shutdown(context.Background())).To((Succeed()))

					Eventually(done).Should(BeClosed())
				})

				helpers.WaitForTCPPort(GinkgoT(), "127.0.0.1", port)
			})

			BeforeEach(func() {
				// Deliberately copy an older tag to the test registry so we can test overwrite and retain.
				craneCopyOutput, err := exec.Command(
					"crane",
					"copy",
					"ghcr.io/stefanprodan/podinfo:6.1.0",
					fmt.Sprintf("%s:%d/stefanprodan/podinfo:6.2.0", registryHost, registryPort),
				).CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), string(craneCopyOutput))

				amd64BundleFile = filepath.Join(tmpDir, "amd64-image-bundle.tar")
				helpers.CreateBundleImages(
					GinkgoT(),
					amd64BundleFile,
					filepath.Join("testdata", "create-success.yaml"),
					"linux/amd64",
				)

				arm64BundleFile = filepath.Join(tmpDir, "arm64-image-bundle.tar")
				helpers.CreateBundleImages(
					GinkgoT(),
					arm64BundleFile,
					filepath.Join("testdata", "create-success.yaml"),
					"linux/arm64",
				)

				DeferCleanup(GinkgoWriter.ClearTeeWriters)
				outputBuf = bytes.NewBuffer(nil)
				GinkgoWriter.TeeTo(outputBuf)
			})

			It("Successful push with explicit --on-existing-tag=merge-with-overwrite", func() {
				args := []string{
					"--bundle", arm64BundleFile,
					"--to-registry", registryAddress,
					"--to-registry-insecure-skip-tls-verify",
					"--on-existing-tag=merge-with-overwrite",
					"--image-push-concurrency=4",
				}
				cmd.SetArgs(args)
				Expect(cmd.Execute()).To(Succeed())

				args = []string{
					"--bundle", amd64BundleFile,
					"--to-registry", registryAddress,
					"--to-registry-insecure-skip-tls-verify",
					"--on-existing-tag=merge-with-overwrite",
					"--image-push-concurrency=4",
				}
				cmd.SetArgs(args)
				Expect(cmd.Execute()).To(Succeed())

				helpers.ValidatePlatformDigestsInIndex(
					GinkgoT(),
					registryHost,
					registryPort,
					"",
					"stefanprodan/podinfo",
					"6.2.0",
					map[*v1.Platform]string{
						// Two new digests overwritten by the pushes above.
						{
							OS:           "linux",
							Architecture: "amd64",
						}: "sha256:f60e14b08375a64528113dd8808b16030c771f626e66961dfaf511b74d6f68dc",
						{
							OS:           "linux",
							Architecture: "arm64",
						}: "sha256:87e43935515a74fcb742d66ee23f5229bd8ac5782f2810787b23c47325cb963e",
						// And another existing digest that was retained.
						{
							OS:           "linux",
							Architecture: "arm",
							Variant:      "v7",
						}: "sha256:26e9410e14d2090953bc1773b4b80beaeeb9171701eda64309a02bc8e87a3f64",
					},
				)
			})

			It("Successful push with explicit --on-existing-tag=merge-with-retain", func() {
				args := []string{
					"--bundle", arm64BundleFile,
					"--to-registry", registryAddress,
					"--to-registry-insecure-skip-tls-verify",
					"--on-existing-tag=merge-with-retain",
					"--image-push-concurrency=4",
				}
				cmd.SetArgs(args)
				Expect(cmd.Execute()).To(Succeed())

				args = []string{
					"--bundle", amd64BundleFile,
					"--to-registry", registryAddress,
					"--to-registry-insecure-skip-tls-verify",
					"--on-existing-tag=merge-with-retain",
					"--image-push-concurrency=4",
				}
				cmd.SetArgs(args)
				Expect(cmd.Execute()).To(Succeed())

				helpers.ValidatePlatformDigestsInIndex(
					GinkgoT(),
					registryHost,
					registryPort,
					"",
					"stefanprodan/podinfo",
					"6.2.0",
					map[*v1.Platform]string{
						// All digests should be retained.
						{
							OS:           "linux",
							Architecture: "amd64",
						}: "sha256:6c84106ca01450e29f2fe21a93d9e93554bcde3ed1ce2c8da49d572b30f932f0",
						{
							OS:           "linux",
							Architecture: "arm64",
						}: "sha256:76f835bf06880d0ec867ba008a3ae099651f17720cab39af12149ab725e34efd",
						{
							OS:           "linux",
							Architecture: "arm",
							Variant:      "v7",
						}: "sha256:26e9410e14d2090953bc1773b4b80beaeeb9171701eda64309a02bc8e87a3f64",
					},
				)
			})
		})

		Context("Checking memory limit", Ordered, func() {
			var (
				registryAddress string
				outputBuf       *bytes.Buffer
			)

			BeforeAll(func() {
				port, err := freeport.GetFreePort()
				Expect(err).NotTo(HaveOccurred())
				reg, err := registry.NewRegistry(registry.Config{
					Storage: registry.FilesystemStorage(filepath.Join(tmpDir, "registry")),
					Host:    "127.0.0.1",
					Port:    uint16(port),
				})
				Expect(err).NotTo(HaveOccurred())
				registryAddress = fmt.Sprintf("http://127.0.0.1:%d", port)

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

				DeferCleanup(func() {
					Expect(reg.Shutdown(context.Background())).To((Succeed()))

					Eventually(done).Should(BeClosed())
				})

				helpers.WaitForTCPPort(GinkgoT(), "127.0.0.1", port)
			})

			BeforeEach(func() {
				helpers.CreateBundleImages(
					GinkgoT(),
					bundleFile,
					filepath.Join("testdata", "create-success-large-images.yaml"),
					"linux/"+runtime.GOARCH,
				)

				// Check bundle file is large enough for GOMEMLIMIT to actually be effective.
				bundleFileInfo, err := os.Stat(bundleFile)
				Expect(err).NotTo(HaveOccurred())
				Expect(bundleFileInfo.Size()).To(BeNumerically(">", 500*1024*1024))

				DeferCleanup(GinkgoWriter.ClearTeeWriters)
				outputBuf = bytes.NewBuffer(nil)
				GinkgoWriter.TeeTo(outputBuf)
			})

			It("Successful push with GOMEMLIMIT set", func() {
				bin, found := artifacts.SelectBinary("mindthegap", runtime.GOOS, runtime.GOARCH)
				Expect(found).To(BeTrue())

				cmd := exec.Command(
					bin.Path,
					"push",
					"bundle",
					"--bundle", bundleFile,
					"--to-registry", registryAddress,
					"--to-registry-insecure-skip-tls-verify",
				)

				cmd.Env = append(cmd.Env, "GOMEMLIMIT=100MiB")

				output, err := cmd.CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), string(output))
				Expect(output).NotTo(ContainSubstring("✗"))
			})
		})
	})
})
