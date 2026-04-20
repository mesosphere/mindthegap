// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package imagearchive implements the `mindthegap push image-archive`
// subcommand that pushes OCI image layout tarballs and docker-save
// tarballs to an OCI registry.
package imagearchive

import (
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cmd/mindthegap/flags"
)

// NewCommand returns the cobra command for `push image-archive`.
func NewCommand(out output.Output) *cobra.Command {
	var (
		archiveFiles                  []string
		destRegistryURI               flags.RegistryURI
		destRegistryCACertificateFile string
		destRegistrySkipTLSVerify     bool
		destRegistryUsername          string
		destRegistryPassword          string
		imageTagOverride              string
	)

	cmd := &cobra.Command{
		Use:   "image-archive",
		Short: "Push OCI/docker image archive tarballs into an existing OCI registry",
		Long: "Push OCI image layout tarballs (oci-archive) and docker-save " +
			"tarballs (docker-archive) directly to an OCI registry. The " +
			"archive format is auto-detected from the file contents.",
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := cmd.ValidateRequiredFlags(); err != nil {
				return err
			}
			return flags.ValidateFlagsThatRequireValues(cmd, "image-archive", "to-registry")
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return runPushImageArchive(
				out,
				archiveFiles,
				&destRegistryURI,
				destRegistryCACertificateFile,
				destRegistrySkipTLSVerify,
				destRegistryUsername,
				destRegistryPassword,
				imageTagOverride,
			)
		},
	}

	cmd.Flags().StringSliceVar(&archiveFiles, "image-archive", nil,
		"Tarball containing an image archive to push (OCI image layout or "+
			"docker-save format, auto-detected). Can be specified multiple "+
			"times or as a glob pattern.")
	_ = cmd.MarkFlagRequired("image-archive")

	cmd.Flags().Var(&destRegistryURI, "to-registry", "Registry to push images to. "+
		"TLS verification will be skipped when using an http:// registry.")
	_ = cmd.MarkFlagRequired("to-registry")

	cmd.Flags().StringVar(&destRegistryCACertificateFile, "to-registry-ca-cert-file", "",
		"CA certificate file used to verify TLS verification of registry to push images to")
	cmd.Flags().BoolVar(&destRegistrySkipTLSVerify, "to-registry-insecure-skip-tls-verify", false,
		"Skip TLS verification of registry to push images to (also use for non-TLS http registries)")
	cmd.MarkFlagsMutuallyExclusive(
		"to-registry-ca-cert-file",
		"to-registry-insecure-skip-tls-verify",
	)

	cmd.Flags().StringVar(&destRegistryUsername, "to-registry-username", "",
		"Username to use to log in to destination registry")
	cmd.Flags().StringVar(&destRegistryPassword, "to-registry-password", "",
		"Password to use to log in to destination registry")
	cmd.MarkFlagsRequiredTogether(
		"to-registry-username",
		"to-registry-password",
	)

	cmd.Flags().StringVar(&imageTagOverride, "image-tag", "",
		"Destination image reference (repo:tag) to use when the archive "+
			"contains a single image. Overrides any embedded tag; required "+
			"if the archive has no embedded tag. Only valid when exactly "+
			"one archive with one image is provided.")

	return cmd
}

// runPushImageArchive is the RunE body. The real push logic is
// implemented in a follow-up commit; for now the skeleton returns nil
// so tests focused on flag handling can run.
func runPushImageArchive(
	_ output.Output,
	_ []string,
	_ *flags.RegistryURI,
	_ string,
	_ bool,
	_ string,
	_ string,
	_ string,
) error {
	return nil
}
