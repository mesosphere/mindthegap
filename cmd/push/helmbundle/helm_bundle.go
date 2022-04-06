// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helmbundle

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/chartmuseum/helm-push/pkg/chartmuseum"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cleanup"
)

func NewCommand(out output.Output) *cobra.Command {
	var (
		helmBundleFile              string
		destRepository              string
		destRepositorySkipTLSVerify bool
		destRepositoryCAFile        string
		destRepositoryUsername      string
		destRepositoryPassword      string
	)

	cmd := &cobra.Command{
		Use:   "helm-bundle",
		Short: "Push Helm charts from a Helm chart bundle into an existing repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			cleaner := cleanup.NewCleaner()
			defer cleaner.Cleanup()

			out.StartOperation("Creating temporary directory")
			tempDir, err := os.MkdirTemp("", ".chart-bundle-*")
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to create temporary directory: %w", err)
			}
			cleaner.AddCleanupFn(func() { _ = os.RemoveAll(tempDir) })
			out.EndOperation(true)

			out.StartOperation("Unarchiving Helm chart bundle")
			err = archiver.Unarchive(helmBundleFile, tempDir)
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to unarchive Helm chart bundle: %w", err)
			}
			out.EndOperation(true)

			out.StartOperation("Creating ChartMuseum client")
			cmOpts := []chartmuseum.Option{chartmuseum.URL(destRepository)}
			if destRepositoryUsername != "" && destRepositoryPassword != "" {
				cmOpts = append(
					cmOpts,
					chartmuseum.Username(
						destRepositoryUsername,
					),
					chartmuseum.Password(destRepositoryPassword),
				)
			}
			if destRepositoryCAFile != "" {
				cmOpts = append(cmOpts, chartmuseum.CAFile(destRepositoryCAFile))
			}
			cmClient, err := chartmuseum.NewClient(cmOpts...)
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to create ChartMuseum client: %v", err)
			}
			out.EndOperation(true)

			err = filepath.WalkDir(tempDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				if d.IsDir() {
					return nil
				}

				if !strings.HasSuffix(path, ".tgz") {
					return nil
				}

				out.StartOperation(
					fmt.Sprintf("Pushing %s to %s", filepath.Base(path), destRepository),
				)
				resp, err := cmClient.UploadChartPackage(path, true)
				if err != nil {
					return fmt.Errorf("failed to push Helm chart %q to ChartMuseum: %v", path, err)
				}
				defer resp.Body.Close()

				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					var respBody bytes.Buffer
					_, _ = io.Copy(&respBody, resp.Body)
					return fmt.Errorf(
						"failed to push %s: %s",
						filepath.Base(path),
						respBody.String(),
					)
				}
				_, _ = io.Copy(out.V(4).InfoWriter(), resp.Body)
				out.EndOperation(true)

				return nil
			})
			if err != nil {
				out.EndOperation(false)
				return fmt.Errorf("failed to push Helm charts: %v", err)
			}

			return nil
		},
	}

	cmd.Flags().
		StringVar(&helmBundleFile, "helm-charts-bundle", "", "Tarball of Helm charts to push")
	_ = cmd.MarkFlagRequired("helm-charts-bundle")
	cmd.Flags().StringVar(&destRepository, "to-repository", "", "Repository to push Helm charts to")
	_ = cmd.MarkFlagRequired("to-registry")
	cmd.Flags().
		BoolVar(&destRepositorySkipTLSVerify, "to-repository-insecure-skip-tls-verify", false,
			"Skip TLS verification of repository to push Helm charts to")
	cmd.Flags().StringVar(&destRepositoryUsername, "to-repository-username", "",
		"Username to use to log in to destination repository")
	cmd.Flags().StringVar(&destRepositoryPassword, "to-repository-password", "",
		"Password to use to log in to destination repository")
	cmd.Flags().StringVar(&destRepositoryCAFile, "to-repository-ca-file", "",
		"File containing CA to validate TLS certificate of destination repository")

	return cmd
}
