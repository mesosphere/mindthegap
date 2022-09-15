// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package helmbundle_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	createhelmbundle "github.com/mesosphere/mindthegap/cmd/mindthegap/create/helmbundle"
	"github.com/mesosphere/mindthegap/test/e2e/helmbundle/helpers"
)

var _ = Describe("Create Bundle", func() {
	var (
		bundleFile string
		cmd        *cobra.Command
	)

	BeforeEach(func() {
		tmpDir := GinkgoT().TempDir()

		bundleFile = filepath.Join(tmpDir, "helm-bundle.tar")

		cmd = helpers.NewCommand(GinkgoT(), createhelmbundle.NewCommand)
		cmd.SilenceUsage = true
	})

	It("Success", func() {
		cmd.SetArgs([]string{
			"--output-file", bundleFile,
			"--helm-charts-file", filepath.Join("testdata", "create-success.yaml"),
		})

		Expect(cmd.Execute()).To(Succeed())
	})

	It("Fail with unresolvable repository name", func() {
		cmd.SetArgs([]string{
			"--output-file", bundleFile,
			"--helm-charts-file", filepath.Join("testdata", "create-failure-unresolvable-repository-name.yaml"),
		})

		err := cmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(
			MatchRegexp(
				"dial tcp: lookup unknownchartrepository(?: on .+)?: (?:no such host|Temporary failure in name resolution)",
			),
		)
	})

	It("Fail with unknown chart version", func() {
		cmd.SetArgs([]string{
			"--output-file", bundleFile,
			"--helm-charts-file", filepath.Join("testdata", "create-failure-unknown-chart-version.yaml"),
		})

		err := cmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(
			ContainSubstring(
				`chart "podinfo" version "unknown" not found in https://stefanprodan.github.io/podinfo repository`,
			),
		)
	})
})
