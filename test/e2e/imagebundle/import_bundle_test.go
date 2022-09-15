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
	"runtime"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/mesosphere/mindthegap/archive"
	importimagebundle "github.com/mesosphere/mindthegap/cmd/mindthegap/importcmd/imagebundle"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/utils"
	"github.com/mesosphere/mindthegap/test/e2e/imagebundle/helpers"
)

var _ = Describe("Import Bundle", func() {
	var (
		bundleFile string
		cmd        *cobra.Command
	)

	BeforeEach(func() {
		tmpDir := GinkgoT().TempDir()

		bundleFile = filepath.Join(tmpDir, "image-bundle.tar")

		cmd = helpers.NewCommand(GinkgoT(), importimagebundle.NewCommand)
	})

	It("Success", func() {
		helpers.CreateBundle(
			GinkgoT(),
			bundleFile,
			filepath.Join("testdata", "import-bundle.txt"),
		)

		copyFileTempDir := GinkgoT().TempDir()
		Expect(
			os.Rename(bundleFile, filepath.Join(copyFileTempDir, filepath.Base(bundleFile))),
		).To(Succeed())
		Expect(
			utils.CopyFile(
				filepath.Join(
					"..",
					"..",
					"..",
					"dist",
					fmt.Sprintf("mindthegap_linux_%s_v1", runtime.GOARCH),
					"mindthegap",
				),
				filepath.Join(copyFileTempDir, "mindthegap"),
			),
		).To(Succeed())

		tarToCopy := filepath.Join(copyFileTempDir, "copy.tar")
		Expect(archive.ArchiveDirectory(copyFileTempDir, tarToCopy)).To(Succeed())
		f, err := os.Open(tarToCopy)
		Expect(err).NotTo(HaveOccurred())

		dc, err := helpers.NewDockerClient()
		if errors.Is(err, helpers.ErrDockerDaemonNotAccessible) {
			Skip(fmt.Sprintf("Docker daemon is not accessible: %v", err))
		}
		DeferCleanup(dc.Close)

		ctx := context.Background()

		c, err := dc.StartContainer(
			ctx,
			container.Config{
				Image:      "mesosphere/kind-node:v1.25.0",
				Entrypoint: strslice.StrSlice{"containerd"},
			},
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
			"mesosphere/kube-apiserver:v1.24.4_fips.0",
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(exitCode).To(Equal(0))

		Expect(strings.TrimSpace(outBuf.String())).
			To(Equal("sha256:b7c51f9294349f26b8ab08272253c70e94cd1a06ef938875a12f8f7f4753c4ec"))
	})

	It("Bundle does not exist", func() {
		cmd.SetArgs([]string{
			"--image-bundle", bundleFile,
		})

		Expect(
			cmd.Execute(),
		).To(MatchError(fmt.Sprintf("did find any matching files for %q", bundleFile)))
	})
})
