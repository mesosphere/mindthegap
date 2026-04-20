// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package imagearchive implements the `mindthegap push image-archive`
// subcommand that pushes OCI image layout tarballs and docker-save
// tarballs to an OCI registry.
package imagearchive

import (
	"fmt"
	"strings"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/types"
	"github.com/google/go-containerregistry/pkg/authn"
	ggcrname "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cmd/mindthegap/flags"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/utils"
	"github.com/mesosphere/mindthegap/images/archive"
	"github.com/mesosphere/mindthegap/images/authnhelpers"
	"github.com/mesosphere/mindthegap/images/httputils"
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

// runPushImageArchive is the RunE body.
func runPushImageArchive(
	out output.Output,
	archiveFiles []string,
	destRegistryURI *flags.RegistryURI,
	destRegistryCACertificateFile string,
	destRegistrySkipTLSVerify bool,
	destRegistryUsername string,
	destRegistryPassword string,
	imageTagOverride string,
) error {
	paths, err := utils.FilesWithGlobs(archiveFiles)
	if err != nil {
		return err
	}

	opened, err := openArchives(out, paths)
	if err != nil {
		return err
	}
	defer closeArchives(opened)

	if err := validateImageTagOverride(opened, imageTagOverride); err != nil {
		return err
	}

	destTLSRoundTripper, err := httputils.TLSConfiguredRoundTripper(
		remote.DefaultTransport,
		destRegistryURI.Host(),
		flags.SkipTLSVerify(destRegistrySkipTLSVerify, destRegistryURI),
		destRegistryCACertificateFile,
	)
	if err != nil {
		return fmt.Errorf("configuring TLS for destination registry: %w", err)
	}
	destRemoteOpts := []remote.Option{
		remote.WithTransport(destTLSRoundTripper),
		remote.WithUserAgent(utils.Useragent()),
	}

	var destNameOpts []ggcrname.Option
	if flags.SkipTLSVerify(destRegistrySkipTLSVerify, destRegistryURI) {
		destNameOpts = append(destNameOpts, ggcrname.Insecure)
	}
	destNameOpts = append(destNameOpts, ggcrname.StrictValidation)

	var keychain authn.Keychain = authn.DefaultKeychain
	if destRegistryUsername != "" && destRegistryPassword != "" {
		keychain = authn.NewMultiKeychain(
			authn.NewKeychainFromHelper(
				authnhelpers.NewStaticHelper(
					destRegistryURI.Host(),
					&types.DockerAuthConfig{
						Username: destRegistryUsername,
						Password: destRegistryPassword,
					},
				),
			),
			keychain,
		)
	}
	destRemoteOpts = append(destRemoteOpts, remote.WithAuthFromKeychain(keychain))

	destRegistry, err := ggcrname.NewRegistry(destRegistryURI.Host(), destNameOpts...)
	if err != nil {
		return fmt.Errorf("parsing destination registry: %w", err)
	}

	for _, oa := range opened {
		for i := range oa.entries {
			entry := oa.entries[i]
			destRef, err := resolveDestRef(destRegistry, destRegistryURI.Path(), entry, imageTagOverride)
			if err != nil {
				return fmt.Errorf("resolving destination reference for %s: %w", oa.path, err)
			}
			displayName := destRef.Name()
			out.StartOperation(fmt.Sprintf("Pushing %s", displayName))
			switch {
			case entry.Image != nil:
				if err := remote.Write(destRef, entry.Image, destRemoteOpts...); err != nil {
					out.EndOperationWithStatus(output.Failure())
					return fmt.Errorf("pushing %s: %w", displayName, err)
				}
			case entry.Index != nil:
				if err := remote.WriteIndex(destRef, entry.Index, destRemoteOpts...); err != nil {
					out.EndOperationWithStatus(output.Failure())
					return fmt.Errorf("pushing %s: %w", displayName, err)
				}
			default:
				out.EndOperationWithStatus(output.Failure())
				return fmt.Errorf("archive %s: entry has neither image nor index", oa.path)
			}
			out.EndOperationWithStatus(output.Success())
		}
	}
	return nil
}

// resolveDestRef decides the destination reference for the given
// entry: use imageTagOverride when set, otherwise use the embedded
// reference stripped of its origin registry host. The destination
// host is always destRegistry's host; destPath (the --to-registry
// URL path) is prepended as a path prefix.
func resolveDestRef(
	destRegistry ggcrname.Registry,
	destPath string,
	entry archive.Entry,
	imageTagOverride string,
) (ggcrname.Reference, error) {
	input := imageTagOverride
	if input == "" {
		if entry.Ref == nil {
			return nil, fmt.Errorf(
				"entry has no embedded tag; pass --image-tag to specify the destination reference",
			)
		}
		input = entry.Ref.Name()
	}

	norm, err := reference.ParseNormalizedNamed(input)
	if err != nil {
		return nil, fmt.Errorf("parsing %q: %w", input, err)
	}
	repoPath := reference.Path(norm)
	tagPart := "latest"
	if tagged, ok := norm.(reference.Tagged); ok {
		tagPart = tagged.Tag()
	}

	destRepo := destRegistry.Repo(strings.TrimLeft(destPath, "/"), repoPath)
	return destRepo.Tag(tagPart), nil
}

type openedArchive struct {
	path    string
	archive archive.Archive
	entries []archive.Entry
}

func openArchives(out output.Output, paths []string) ([]openedArchive, error) {
	opened := make([]openedArchive, 0, len(paths))
	for _, p := range paths {
		out.StartOperation(fmt.Sprintf("Opening archive %s", p))
		a, err := archive.Open(p)
		if err != nil {
			out.EndOperationWithStatus(output.Failure())
			return nil, err
		}
		entries, err := a.Entries()
		if err != nil {
			_ = a.Close()
			out.EndOperationWithStatus(output.Failure())
			return nil, fmt.Errorf("reading entries from %s: %w", p, err)
		}
		out.EndOperationWithStatus(output.Success())
		opened = append(opened, openedArchive{path: p, archive: a, entries: entries})
	}
	return opened, nil
}

func closeArchives(opened []openedArchive) {
	for _, o := range opened {
		_ = o.archive.Close()
	}
}

// validateImageTagOverride enforces the "single archive, single
// image" precondition when --image-tag is set, and validates the
// override parses as a valid reference.
func validateImageTagOverride(opened []openedArchive, imageTagOverride string) error {
	if imageTagOverride == "" {
		return nil
	}
	if len(opened) != 1 {
		return fmt.Errorf(
			"--image-tag can only be used with a single archive containing a single image; got %d archives",
			len(opened),
		)
	}
	if len(opened[0].entries) != 1 {
		return fmt.Errorf(
			"--image-tag can only be used with a single archive containing a single image; archive %s contains %d entries",
			opened[0].path,
			len(opened[0].entries),
		)
	}
	if _, err := ggcrname.ParseReference(imageTagOverride, ggcrname.StrictValidation); err != nil {
		return fmt.Errorf("parsing --image-tag %q: %w", imageTagOverride, err)
	}
	return nil
}
