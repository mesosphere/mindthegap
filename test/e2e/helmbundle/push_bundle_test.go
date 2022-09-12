// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e
// +build e2e

package helmbundle_test

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/phayes/freeport"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	createhelmbundle "github.com/mesosphere/mindthegap/cmd/mindthegap/create/helmbundle"
	pushhelmbundle "github.com/mesosphere/mindthegap/cmd/mindthegap/push/helmbundle"
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

		cmd = helpers.NewCommand(pushhelmbundle.NewCommand)
	})

	It("Success", func() {
		createBundleCmd := helpers.NewCommand(createhelmbundle.NewCommand)
		createBundleCmd.SetArgs([]string{
			"--output-file", bundleFile,
			"--helm-charts-file", filepath.Join("testdata", "create-success.yaml"),
		})
		Expect(createBundleCmd.Execute()).To(Succeed())

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

		Eventually(func() error {
			conn, err := net.DialTimeout(
				"tcp",
				net.JoinHostPort("localhost", strconv.Itoa(port)),
				1*time.Second,
			)
			DeferCleanup(func() {
				if conn != nil {
					conn.Close()
				}
			})
			return err
		}).ShouldNot(HaveOccurred())

		h, cleanup := helm.NewClient(output.NewNonInteractiveShell(GinkgoWriter, GinkgoWriter, 10))
		DeferCleanup(cleanup)

		helmTmpDir := GinkgoT().TempDir()

		cmd.SetArgs([]string{
			"--helm-bundle", bundleFile,
			"--to-registry", fmt.Sprintf("localhost:%v/charts", port),
			"--to-registry-insecure-skip-tls-verify",
		})

		Expect(cmd.Execute()).To(Succeed())

		d, err := h.GetChartFromRepo(
			helmTmpDir,
			"",
			fmt.Sprintf("%s://localhost:%v/charts/podinfo", helm.OCIScheme, port),
			"6.2.0",
			[]helm.ConfigOpt{helm.RegistryClientConfigOpt()},
			func(p *action.Pull) { p.InsecureSkipTLSverify = true },
		)
		Expect(err).NotTo(HaveOccurred())
		chrt, err := helm.LoadChart(d)
		Expect(err).NotTo(HaveOccurred())
		Expect(chrt.Metadata.Name).To(Equal("podinfo"))
		Expect(chrt.Metadata.Version).To(Equal("6.2.0"))

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
