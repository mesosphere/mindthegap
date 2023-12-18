// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package imagebundle_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/mesosphere/mindthegap/archive"
	importimagebundle "github.com/mesosphere/mindthegap/cmd/mindthegap/importcmd/imagebundle"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/utils"
	"github.com/mesosphere/mindthegap/test/e2e/imagebundle/helpers"
)

var _ = Describe("Import Bundle", Label("import"), Serial, func() {
	DescribeTable(
		"Success",
		func(image, opsys, arch, expectedDigest string) {
			tmpDir := GinkgoT().TempDir()

			bundleFile := filepath.Join(tmpDir, "image-bundle.tar")

			imagesFile := filepath.Join(tmpDir, "image-bundle.txt")
			Expect(os.WriteFile(imagesFile, []byte(image), 0o600)).To(Succeed())

			helpers.CreateBundle(
				GinkgoT(),
				bundleFile,
				imagesFile,
				opsys+"/"+arch,
			)

			bin, found := artifacts.SelectBinary("mindthegap", opsys, arch)
			Expect(found).To(BeTrue())
			Expect(
				utils.CopyFile(
					bin.Path,
					filepath.Join(tmpDir, "mindthegap"),
				),
			).To(Succeed())

			tarToCopy := filepath.Join(tmpDir, "copy.tar")
			Expect(archive.ArchiveDirectory(tmpDir, tarToCopy)).To(Succeed())
			f, err := os.Open(tarToCopy)
			Expect(err).NotTo(HaveOccurred())

			dc, err := helpers.NewDockerClient()
			if errors.Is(err, helpers.ErrDockerDaemonNotAccessible) {
				Skip(fmt.Sprintf("Docker daemon is not accessible: %v", err))
			}
			DeferCleanup(dc.Close)

			ctx := context.Background()

			c, err := dc.StartContainerWithPlatform(
				ctx,
				container.Config{
					Image:      "ghcr.io/mesosphere/kind-node:v1.28.2",
					Entrypoint: strslice.StrSlice{"containerd"},
				},
				&specs.Platform{OS: opsys, Architecture: arch},
			)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(c.Stop, ctx)

			Expect(c.CopyTo(ctx, "/tmp", f)).To(Succeed())

			exitCode, err := c.Exec(
				ctx,
				GinkgoWriter,
				GinkgoWriter,
				"/tmp/mindthegap",
				"import",
				"image-bundle",
				"--image-bundle",
				filepath.Join("/tmp", filepath.Base(bundleFile)),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))

			var outBuf, errBuf bytes.Buffer

			exitCode, err = c.Exec(
				ctx,
				&outBuf,
				&errBuf,
				"crictl",
				"inspecti",
				"-q",
				"-o", "go-template",
				"--template", "{{.status.id}}",
				image,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(0))

			Expect(strings.TrimSpace(outBuf.String())).
				To(Equal("sha256:" + expectedDigest))
		},

		Entry(
			"FIPS linux/amd64",
			"mesosphere/kube-apiserver:v1.24.4_fips.0",
			"linux",
			"amd64",
			"c4c4421f5e2ee9c34b4c85b0d00d17fea04d148f42f10a2ff9c2ff64784a098b",
		),
		Entry(
			"CAPA linux/amd64",
			"registry.k8s.io/cluster-api-aws/cluster-api-aws-controller:v1.5.1",
			"linux",
			"amd64",
			"da0be714be8c1a615bfcb0bb31184c4c2448db0ca0e58bb00be50aeec70bcf41",
		),
		Entry(
			"CAPA linux/arm64",
			"registry.k8s.io/cluster-api-aws/cluster-api-aws-controller:v1.5.1",
			"linux",
			"arm64",
			"fad2d7abb2da9b011072bed909fac04a1dc7f6dd29d2699c3d29a6b139e53559",
		),
	)

	It("Bundle does not exist", func() {
		cmd := helpers.NewCommand(GinkgoT(), importimagebundle.NewCommand)

		cmd.SetArgs([]string{
			"--image-bundle", "non-existent-file",
		})

		Expect(
			cmd.Execute(),
		).To(MatchError(fmt.Sprintf("did find any matching files for %q", "non-existent-file")))
	})

	It("Bundle file exists, is a tarball, but not an image bundle", func() {
		tmpDir := GinkgoT().TempDir()
		nonBundleFile := filepath.Join(tmpDir, "image-bundle.tar")

		Expect(archive.ArchiveDirectory("testdata", nonBundleFile)).To(Succeed())

		cmd := helpers.NewCommand(GinkgoT(), importimagebundle.NewCommand)

		cmd.SetArgs([]string{
			"--image-bundle", nonBundleFile,
		})

		Expect(
			cmd.Execute(),
		).To(MatchError("no bundle configuration(s) found: please check that you have specified valid air-gapped bundle(s)"))
	})
})
