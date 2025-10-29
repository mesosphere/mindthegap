// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	mediatypes "github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag/v2"
	"golang.org/x/sync/errgroup"

	"github.com/mesosphere/dkp-cli-runtime/core/output"
	"github.com/mesosphere/dkp-cli-runtime/core/term"

	"github.com/mesosphere/mindthegap/cleanup"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/flags"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/utils"
	"github.com/mesosphere/mindthegap/config"
	"github.com/mesosphere/mindthegap/docker/ecr"
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/images"
	"github.com/mesosphere/mindthegap/images/authnhelpers"
	"github.com/mesosphere/mindthegap/images/httputils"
)

type onExistingTagMode enumflag.Flag

const (
	Overwrite onExistingTagMode = iota
	Error
	Skip
	MergeWithRetain
	MergeWithOverwrite
)

var onExistingTagModes = map[onExistingTagMode][]string{
	Overwrite:          {"overwrite"},
	Error:              {"error"},
	Skip:               {"skip"},
	MergeWithRetain:    {"merge-with-retain"},
	MergeWithOverwrite: {"merge-with-overwrite"},
}

func NewCommand(out output.Output, bundleCmdName string) *cobra.Command {
	var (
		bundleFiles                   []string
		destRegistryURI               flags.RegistryURI
		destRegistryCACertificateFile string
		destRegistrySkipTLSVerify     bool
		destRegistryUsername          string
		destRegistryPassword          string
		ecrLifecyclePolicy            string
		onExistingTag                 = Overwrite
		imagePushConcurrency          int
		forceOCIMediaTypes            bool
	)

	cmd := &cobra.Command{
		Use:   bundleCmdName,
		Short: "Push from bundles into an existing OCI registry",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.ValidateRequiredFlags(); err != nil {
				return err
			}

			if err := flags.ValidateFlagsThatRequireValues(cmd, bundleCmdName, "to-registry"); err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create configuration from command flags
			cfg, err := NewPushBundleOpts(bundleFiles, &destRegistryURI)
			if err != nil {
				return fmt.Errorf("invalid configuration: %w", err)
			}
			// Set TLS configuration (mutually exclusive options)
			if err := cfg.WithRegistryCACertificateFile(destRegistryCACertificateFile); err != nil {
				return fmt.Errorf("invalid TLS configuration: %w", err)
			}
			if err := cfg.WithRegistrySkipTLSVerify(destRegistrySkipTLSVerify); err != nil {
				return fmt.Errorf("invalid TLS configuration: %w", err)
			}
			// Set credentials (both must be provided together)
			if err := cfg.WithRegistryCredentials(destRegistryUsername, destRegistryPassword); err != nil {
				return fmt.Errorf("invalid credentials: %w", err)
			}
			// Set optional configuration
			cfg.WithECRLifecyclePolicy(ecrLifecyclePolicy).
				WithOnExistingTag(onExistingTag).
				WithImagePushConcurrency(imagePushConcurrency).
				WithForceOCIMediaTypes(forceOCIMediaTypes)

			return PushBundles(cfg, out)
		},
	}

	cmd.Flags().StringSliceVar(&bundleFiles, bundleCmdName, nil,
		"Tarball containing list of images to push. Can also be a glob pattern.")
	_ = cmd.MarkFlagRequired(bundleCmdName)
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
	cmd.Flags().StringVar(&ecrLifecyclePolicy, "ecr-lifecycle-policy-file", "",
		"File containing ECR lifecycle policy for newly created repositories "+
			"(only applies if target registry is hosted on ECR, ignored otherwise)")

	cmd.Flags().Var(
		enumflag.New(&onExistingTag, "string", onExistingTagModes, enumflag.EnumCaseSensitive),
		"on-existing-tag",
		`how to handle existing tags: one of "overwrite", "error", or "skip"`,
	)
	cmd.Flags().
		IntVar(&imagePushConcurrency, "image-push-concurrency", 1, "Image push concurrency")

	cmd.Flags().
		BoolVar(&forceOCIMediaTypes, "force-oci-media-types", false, "force OCI media types")

	return cmd
}

type prePushFunc func(destRepositoryName name.Repository, imageTags ...string) error

type pushConfig struct {
	forceOCIMediaTypes bool
	onExistingTag      onExistingTagMode
}

type pushOpt func(*pushConfig)

func withForceOCIMediaTypes(force bool) pushOpt {
	return func(cfg *pushConfig) {
		cfg.forceOCIMediaTypes = force
	}
}

func withOnExistingTagMode(mode onExistingTagMode) pushOpt {
	return func(cfg *pushConfig) {
		cfg.onExistingTag = mode
	}
}

// pushBundleOpts holds all configuration needed for pushing bundles.
// Use NewPushBundleOpts to create a properly validated instance.
type pushBundleOpts struct {
	// Bundle files to process
	bundleFiles []string

	// Destination registry configuration
	registryURI               *flags.RegistryURI
	registryCACertificateFile string
	registrySkipTLSVerify     bool
	registryUsername          string
	registryPassword          string

	// ECR specific configuration
	ecrLifecyclePolicy string

	// Push behavior configuration
	onExistingTag        onExistingTagMode
	imagePushConcurrency int
	forceOCIMediaTypes   bool
}

// NewPushBundleOpts creates a new pushBundleOpts with required fields.
// Returns an error if required fields are invalid.
func NewPushBundleOpts(bundleFiles []string, registryURI *flags.RegistryURI) (*pushBundleOpts, error) {
	if len(bundleFiles) == 0 {
		return nil, fmt.Errorf("at least one bundle file is required")
	}

	if registryURI.Host() == "" {
		return nil, fmt.Errorf("registry URI is required")
	}

	return &pushBundleOpts{
		bundleFiles:          bundleFiles,
		registryURI:          registryURI,
		onExistingTag:        Overwrite, // default
		imagePushConcurrency: 1,         // default
	}, nil
}

// WithRegistryCredentials sets the registry credentials.
// Both username and password must be provided together.
func (c *pushBundleOpts) WithRegistryCredentials(username, password string) error {
	if (username == "") != (password == "") {
		return fmt.Errorf("both username and password must be provided together")
	}
	c.registryUsername = username
	c.registryPassword = password
	return nil
}

// WithRegistryCACertificateFile sets the CA certificate file for TLS verification.
// This option is mutually exclusive with WithRegistrySkipTLSVerify(true).
func (c *pushBundleOpts) WithRegistryCACertificateFile(caCertFile string) error {
	if caCertFile != "" && c.registrySkipTLSVerify {
		return fmt.Errorf("cannot specify both CA certificate and skip TLS verify")
	}
	c.registryCACertificateFile = caCertFile
	return nil
}

// WithRegistrySkipTLSVerify sets whether to skip TLS verification.
// This option is mutually exclusive with WithRegistryCACertificateFile.
func (c *pushBundleOpts) WithRegistrySkipTLSVerify(skip bool) error {
	if skip && c.registryCACertificateFile != "" {
		return fmt.Errorf("cannot specify both CA certificate and skip TLS verify")
	}
	c.registrySkipTLSVerify = skip
	return nil
}

// WithECRLifecyclePolicy sets the ECR lifecycle policy file.
func (c *pushBundleOpts) WithECRLifecyclePolicy(policyFile string) *pushBundleOpts {
	c.ecrLifecyclePolicy = policyFile
	return c
}

// WithOnExistingTag sets the behavior for handling existing tags.
func (c *pushBundleOpts) WithOnExistingTag(mode onExistingTagMode) *pushBundleOpts {
	c.onExistingTag = mode
	return c
}

// WithImagePushConcurrency sets the image push concurrency.
func (c *pushBundleOpts) WithImagePushConcurrency(concurrency int) *pushBundleOpts {
	c.imagePushConcurrency = concurrency
	return c
}

// WithForceOCIMediaTypes sets whether to force OCI media types.
func (c *pushBundleOpts) WithForceOCIMediaTypes(force bool) *pushBundleOpts {
	c.forceOCIMediaTypes = force
	return c
}

// PushBundles pushes both images and charts from bundle files to the destination registry.
func PushBundles(cfg *pushBundleOpts, out output.Output) error {
	cleaner := cleanup.NewCleaner()
	defer cleaner.Cleanup()

	out.StartOperation("Creating temporary directory")
	tempDir, err := os.MkdirTemp("", ".bundle-*")
	if err != nil {
		out.EndOperationWithStatus(output.Failure())
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	cleaner.AddCleanupFn(func() { _ = os.RemoveAll(tempDir) })
	out.EndOperationWithStatus(output.Success())

	bundleFiles, err := utils.FilesWithGlobs(cfg.bundleFiles)
	if err != nil {
		return err
	}
	imagesCfg, chartsCfg, err := utils.ExtractConfigs(tempDir, out, bundleFiles...)
	if err != nil {
		return err
	}

	out.StartOperation("Starting temporary Docker registry")
	storage, err := registry.ArchiveStorage("", bundleFiles...)
	if err != nil {
		out.EndOperationWithStatus(output.Failure())
		return fmt.Errorf("failed to create storage for Docker registry from supplied bundles: %w", err)
	}
	reg, err := registry.NewRegistry(
		registry.Config{Storage: storage},
	)
	if err != nil {
		out.EndOperationWithStatus(output.Failure())
		return fmt.Errorf("failed to create local Docker registry: %w", err)
	}
	registryErrCh := make(chan error, 1)
	go func() {
		if err := reg.ListenAndServe(output.NewOutputLogr(out)); err != nil {
			registryErrCh <- fmt.Errorf("error serving Docker registry: %w", err)
		}
	}()
	out.EndOperationWithStatus(output.Success())

	logs.Debug.SetOutput(out.V(4).InfoWriter())
	logs.Warn.SetOutput(out.V(2).InfoWriter())

	sourceTLSRoundTripper, err := httputils.InsecureTLSRoundTripper(remote.DefaultTransport)
	if err != nil {
		return fmt.Errorf("error configuring TLS for source registry: %w", err)
	}
	sourceRemoteOpts := []remote.Option{
		remote.WithTransport(sourceTLSRoundTripper),
		remote.WithUserAgent(utils.Useragent()),
	}

	destTLSRoundTripper, err := httputils.TLSConfiguredRoundTripper(
		remote.DefaultTransport,
		cfg.registryURI.Host(),
		flags.SkipTLSVerify(cfg.registrySkipTLSVerify, cfg.registryURI),
		cfg.registryCACertificateFile,
	)
	if err != nil {
		return fmt.Errorf("error configuring TLS for destination registry: %w", err)
	}
	destRemoteOpts := []remote.Option{
		remote.WithTransport(destTLSRoundTripper),
		remote.WithUserAgent(utils.Useragent()),
	}

	var destNameOpts []name.Option
	if flags.SkipTLSVerify(cfg.registrySkipTLSVerify, cfg.registryURI) {
		destNameOpts = append(destNameOpts, name.Insecure)
	}

	// Determine type of destination registry.
	var prePushFuncs []prePushFunc
	if ecr.IsECRRegistry(cfg.registryURI.Host()) {
		ecrClient, err := ecr.ClientForRegistry(cfg.registryURI.Host())
		if err != nil {
			return err
		}

		prePushFuncs = append(
			prePushFuncs,
			ecr.EnsureRepositoryExistsFunc(ecrClient, cfg.ecrLifecyclePolicy),
		)

		// If a password hasn't been specified, then try to retrieve a token.
		if cfg.registryPassword == "" {
			out.StartOperation("Retrieving ECR credentials")
			cfg.registryUsername, cfg.registryPassword, err = ecr.RetrieveUsernameAndToken(ecrClient)
			if err != nil {
				out.EndOperationWithStatus(output.Failure())
				return fmt.Errorf(
					"failed to retrieve ECR credentials: %w\n\nPlease ensure you have authenticated to AWS and try again",
					err,
				)
			}
			out.EndOperationWithStatus(output.Success())
		}
	}

	var keychain authn.Keychain = authn.DefaultKeychain
	if cfg.registryUsername != "" && cfg.registryPassword != "" {
		keychain = authn.NewMultiKeychain(
			authn.NewKeychainFromHelper(
				authnhelpers.NewStaticHelper(
					cfg.registryURI.Host(),
					&types.DockerAuthConfig{
						Username: cfg.registryUsername,
						Password: cfg.registryPassword,
					},
				),
			),
			keychain,
		)
	}
	destRemoteOpts = append(destRemoteOpts, remote.WithAuthFromKeychain(keychain))

	srcRegistry, err := name.NewRegistry(
		reg.Address(),
		name.Insecure,
		name.StrictValidation,
	)
	if err != nil {
		return err
	}
	destRegistry, err := name.NewRegistry(
		cfg.registryURI.Host(),
		append(destNameOpts, name.StrictValidation)...)
	if err != nil {
		return err
	}

	if imagesCfg != nil {
		err = pushImages(
			*imagesCfg,
			srcRegistry,
			sourceRemoteOpts,
			destRegistry,
			cfg.registryURI.Path(),
			destRemoteOpts,
			cfg.onExistingTag,
			cfg.imagePushConcurrency,
			out,
			cfg.forceOCIMediaTypes,
			prePushFuncs...,
		)
		if err != nil {
			return err
		}
	}

	chartsSrcRegistry, err := name.NewRegistry(
		reg.Address(),
		name.Insecure,
	)
	if err != nil {
		return err
	}

	if chartsCfg != nil {
		err = pushOCIArtifacts(
			*chartsCfg,
			chartsSrcRegistry,
			"/charts",
			sourceRemoteOpts,
			destRegistry,
			cfg.registryURI.Path(),
			destRemoteOpts,
			out,
			prePushFuncs...,
		)
		if err != nil {
			return err
		}
	}

	// Check if the registry goroutine encountered any errors
	select {
	case err := <-registryErrCh:
		return err
	default:
		// No error from registry
	}

	return nil
}

type pushFunc func(
	srcImage name.Reference,
	sourceRemoteOpts []remote.Option,
	destImage name.Reference,
	destRemoteOpts []remote.Option,
	pushOpts ...pushOpt,
) error

func pushImages(
	cfg config.ImagesConfig,
	sourceRegistry name.Registry, sourceRemoteOpts []remote.Option,
	destRegistry name.Registry, destRegistryPath string, destRemoteOpts []remote.Option,
	onExistingTag onExistingTagMode,
	imagePushConcurrency int,
	out output.Output,
	forceOCIMediaTypes bool,
	prePushFuncs ...prePushFunc,
) error {
	puller, err := remote.NewPuller(destRemoteOpts...)
	if err != nil {
		return err
	}

	// Sort registries for deterministic ordering.
	regNames := cfg.SortedRegistryNames()

	eg, egCtx := errgroup.WithContext(context.Background())
	eg.SetLimit(imagePushConcurrency)

	sourceRemoteOpts = append(sourceRemoteOpts, remote.WithContext(egCtx))
	destRemoteOpts = append(destRemoteOpts, remote.WithContext(egCtx))

	// Either use a gauge for interactive TTY or line per image for non-TTY.
	isTTY := term.IsSmartTerminal(os.Stderr)
	type completePushFunc func(image, tag string) error
	var completePush completePushFunc
	if isTTY {
		pushGauge := &output.ProgressGauge{}
		pushGauge.SetCapacity(cfg.TotalImages())
		pushGauge.SetStatus("Pushing bundled images")
		completePush = func(_, _ string) error {
			pushGauge.Inc()

			return nil
		}
		out.StartOperationWithProgress(pushGauge)
	} else {
		// Use an output writer mutex to ensure the output is not interleaved.
		var outputWriterMutex sync.RWMutex
		currentImageIdx := 0
		completePush = func(image, tag string) error {
			outputWriterMutex.Lock()
			defer outputWriterMutex.Unlock()
			currentImageIdx++
			out.StartOperation(fmt.Sprintf("[%d/%d] Pushing %s:%s", currentImageIdx, cfg.TotalImages(), image, tag))
			// Use the deprecated EndOperation instead of EndOperationWithStatus to ensure the correct INF prefix
			// is printed in the output. This needs to be fixed upstream, but this is ok for now.
			out.EndOperation(true) //nolint:staticcheck // Needs to be fixed upstream.
			return nil
		}
	}

	for _, registryName := range regNames {
		registryConfig := cfg[registryName]

		// Sort images for deterministic ordering.
		imageNames := registryConfig.SortedImageNames()

		for _, imageName := range imageNames {
			// Output the origin image name and tag, normalized to the canonical format including
			// docker.io prefix if necessary.
			originImage, err := reference.ParseNormalizedNamed(imageName)
			if err != nil {
				return fmt.Errorf("failed to parse image name: %w", err)
			}

			srcRepository := sourceRegistry.Repo(imageName)
			destRepository := destRegistry.Repo(strings.TrimLeft(destRegistryPath, "/"), imageName)

			imageTags := registryConfig.Images[imageName]

			var (
				imageTagPrePushSync sync.Once
				imageTagPrePushErr  error
				existingImageTags   map[string]struct{}
			)

			for _, imageTag := range imageTags {
				eg.Go(func() error {
					imageTagPrePushSync.Do(func() {
						for _, prePush := range prePushFuncs {
							if err := prePush(destRepository, imageTags...); err != nil {
								imageTagPrePushErr = fmt.Errorf("pre-push func failed: %w", err)
							}
						}

						existingImageTags, imageTagPrePushErr = getExistingImages(
							context.Background(),
							onExistingTag,
							puller,
							destRepository,
						)
					})

					if imageTagPrePushErr != nil {
						return imageTagPrePushErr
					}

					srcImage := srcRepository.Tag(imageTag)
					destImage := destRepository.Tag(imageTag)

					var pushFn pushFunc = pushTag

					switch onExistingTag {
					case Overwrite, MergeWithRetain, MergeWithOverwrite:
						// Nothing to do here, pushFn is already set to pushTag above.
					case Skip:
						// If tag exists already then do nothing.
						if _, exists := existingImageTags[imageTag]; exists {
							pushFn = func(
								_ name.Reference, _ []remote.Option, _ name.Reference, _ []remote.Option, _ ...pushOpt,
							) error {
								return nil
							}
						}
					case Error:
						if _, exists := existingImageTags[imageTag]; exists {
							pushFn = func(
								_ name.Reference, _ []remote.Option, _ name.Reference, _ []remote.Option, _ ...pushOpt,
							) error {
								return fmt.Errorf(
									"failed to push image %s:%s to %s: image tag already exists in destination registry",
									originImage,
									imageTag,
									destRepository,
								)
							}
						}
					}

					opts := []pushOpt{withOnExistingTagMode(onExistingTag)}
					if forceOCIMediaTypes {
						opts = append(opts, withForceOCIMediaTypes(forceOCIMediaTypes))
					}

					if err := pushFn(srcImage, sourceRemoteOpts, destImage, destRemoteOpts, opts...); err != nil {
						return fmt.Errorf(
							"failed to push image %s:%s to %s: %w",
							originImage,
							imageTag,
							destRepository,
							err,
						)
					}

					if err := completePush(originImage.Name(), imageTag); err != nil {
						return err
					}

					return nil
				})
			}
		}
	}

	if err := eg.Wait(); err != nil {
		out.EndOperationWithStatus(output.Failure())
		return err
	}

	out.EndOperationWithStatus(output.Success())

	return nil
}

func pushTag(
	srcImage name.Reference,
	sourceRemoteOpts []remote.Option,
	destImage name.Reference,
	destRemoteOpts []remote.Option,
	pushOpts ...pushOpt,
) error {
	var pushCfg pushConfig
	for _, opt := range pushOpts {
		opt(&pushCfg)
	}

	desc, err := remote.Get(srcImage, sourceRemoteOpts...)
	if err != nil {
		return err
	}

	if !desc.MediaType.IsIndex() {
		image, err := desc.Image()
		if err != nil {
			return err
		}
		return remote.Write(destImage, image, destRemoteOpts...)
	}

	idx, err := desc.ImageIndex()
	if err != nil {
		return err
	}

	// Get the existing index from the destination registry if merging is enabled.
	if pushCfg.onExistingTag == MergeWithOverwrite || pushCfg.onExistingTag == MergeWithRetain {
		existingIdx, err := fetchExistingIndex(
			destImage,
			destRemoteOpts,
		)
		if err != nil {
			return fmt.Errorf("failed to fetch existing index: %w", err)
		}

		mergeFromIndex, mergeToIndex := idx, existingIdx
		if pushCfg.onExistingTag == MergeWithRetain {
			mergeFromIndex, mergeToIndex = existingIdx, idx
		}

		idx, err = mergeIndexesOverwriteExisting(mergeFromIndex, mergeToIndex)
		if err != nil {
			return fmt.Errorf("failed to merge indexes: %w", err)
		}
	}

	if pushCfg.forceOCIMediaTypes {
		idx, err = convertToOCIIndex(idx, srcImage, sourceRemoteOpts)
		if err != nil {
			return fmt.Errorf("failed to convert index to OCI format: %w", err)
		}
	}

	return remote.WriteIndex(destImage, idx, destRemoteOpts...)
}

func pushOCIArtifacts(
	cfg config.HelmChartsConfig,
	sourceRegistry name.Registry, sourceRegistryPath string, sourceRemoteOpts []remote.Option,
	destRegistry name.Registry, destRegistryPath string, destRemoteOpts []remote.Option,
	out output.Output,
	prePushFuncs ...prePushFunc,
) error {
	// Sort repositories for deterministic ordering.
	repoNames := cfg.SortedRepositoryNames()

	for _, repoName := range repoNames {
		repoConfig := cfg.Repositories[repoName]

		// Sort charts for deterministic ordering.
		chartNames := repoConfig.SortedChartNames()

		for _, chartName := range chartNames {
			srcRepository := sourceRegistry.Repo(
				strings.TrimLeft(sourceRegistryPath, "/"),
				chartName,
			)
			destRepository := destRegistry.Repo(strings.TrimLeft(destRegistryPath, "/"), chartName)

			chartVersions := repoConfig.Charts[chartName]

			for _, prePush := range prePushFuncs {
				if err := prePush(destRepository, chartVersions...); err != nil {
					return fmt.Errorf("pre-push func failed: %w", err)
				}
			}

			for _, chartVersion := range chartVersions {
				destChart := destRepository.Tag(chartVersion)

				out.StartOperation(
					fmt.Sprintf("Copying %s:%s (from bundle) to %s",
						chartName, chartVersion,
						destChart.Name(),
					),
				)

				srcChart := srcRepository.Tag(chartVersion)
				src, err := remote.Image(srcChart, sourceRemoteOpts...)
				if err != nil {
					out.EndOperationWithStatus(output.Failure())
					return err
				}

				if err := remote.Write(destChart, src, destRemoteOpts...); err != nil {
					out.EndOperationWithStatus(output.Failure())
					return err
				}

				out.EndOperationWithStatus(output.Success())
			}
		}
	}

	return nil
}

func getExistingImages(
	ctx context.Context,
	onExistingTag onExistingTagMode,
	puller *remote.Puller,
	repo name.Repository,
) (map[string]struct{}, error) {
	if onExistingTag == Overwrite {
		return nil, nil
	}

	tags, err := puller.List(ctx, repo)
	if err != nil {
		var terr *transport.Error
		if errors.As(err, &terr) {
			// Some registries create repository on first push, so listing tags will fail.
			// If we see 404 or 403, assume we failed because the repository hasn't been created yet.
			if terr.StatusCode == http.StatusNotFound || terr.StatusCode == http.StatusForbidden {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("failed to list existing tags: %w", err)
	}

	existingTags := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		existingTags[t] = struct{}{}
	}

	return existingTags, nil
}

func convertToOCIIndex(
	originalIndex v1.ImageIndex,
	srcImage name.Reference,
	sourceRemoteOpts []remote.Option,
) (v1.ImageIndex, error) {
	originalMediaType, err := originalIndex.MediaType()
	if err != nil {
		return nil, fmt.Errorf("failed to get media type of image index: %w", err)
	}

	if originalMediaType == mediatypes.OCIImageIndex {
		return originalIndex, nil
	}

	var ociIdx v1.ImageIndex = empty.Index
	ociIdx = mutate.IndexMediaType(ociIdx, mediatypes.OCIImageIndex)

	originalIdx, err := originalIndex.IndexManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to read original image index manifest: %w", err)
	}

	for i := range originalIdx.Manifests {
		manifest := originalIdx.Manifests[i]
		manifest.MediaType = mediatypes.OCIManifestSchema1

		digestRef, err := name.NewDigest(
			fmt.Sprintf("%s@%s", srcImage.Context().Name(), manifest.Digest.String()),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create digest reference: %w", err)
		}

		imgDesc, err := remote.Get(digestRef, sourceRemoteOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to get image %q: %w", digestRef, err)
		}

		img, err := imgDesc.Image()
		if err != nil {
			return nil, fmt.Errorf(
				"failed to convert image descriptor for %q to image: %w",
				digestRef,
				err,
			)
		}

		ociImg := empty.Image
		ociImg = mutate.MediaType(ociImg, mediatypes.OCIManifestSchema1)
		ociImg = mutate.ConfigMediaType(ociImg, mediatypes.OCIConfigJSON)
		layers, err := img.Layers()
		if err != nil {
			return nil, fmt.Errorf("failed to get layers for image %q: %w", digestRef, err)
		}

		for _, layer := range layers {
			ociImg, err = mutate.Append(ociImg, mutate.Addendum{
				Layer:     layer,
				MediaType: mediatypes.OCILayer,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to append layer to image %q: %w", digestRef, err)
			}
		}

		ociImgDigest, err := ociImg.Digest()
		if err != nil {
			return nil, fmt.Errorf("failed to get digest for image %q: %w", digestRef, err)
		}

		manifest.Digest = ociImgDigest

		ociIdx = mutate.AppendManifests(ociIdx, mutate.IndexAddendum{
			Add:        ociImg,
			Descriptor: manifest,
		})
	}

	return ociIdx, nil
}

func fetchExistingIndex(destImage name.Reference, destRemoteOpts []remote.Option) (v1.ImageIndex, error) {
	existingDesc, err := remote.Get(destImage, destRemoteOpts...)
	if err != nil {
		var terr *transport.Error
		if errors.As(err, &terr) {
			if terr.StatusCode == http.StatusNotFound {
				return empty.Index, nil
			}
		}
		return nil, fmt.Errorf("failed to fetch existing descriptor: %w", err)
	}

	switch {
	case existingDesc.MediaType.IsIndex():
		index, err := existingDesc.ImageIndex()
		if err != nil {
			return nil, fmt.Errorf("failed to read image index for %q: %w", destImage, err)
		}
		return index, nil
	case existingDesc.MediaType.IsImage():
		image, err := existingDesc.Image()
		if err != nil {
			return nil, fmt.Errorf("failed to read image for %q: %w", destImage, err)
		}
		return images.IndexForSinglePlatformImage(destImage, image)
	default:
		return nil, fmt.Errorf(
			"unexpected media type in descriptor for image %q: %v",
			destImage,
			existingDesc.MediaType,
		)
	}
}

func mergeIndexesOverwriteExisting(
	mergeFromIndex v1.ImageIndex,
	mergeToIndex v1.ImageIndex,
) (v1.ImageIndex, error) {
	mergeFromIndexManifest, err := mergeFromIndex.IndexManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to read index manifest: %w", err)
	}

	// Collect all platforms from the mergeFromIndex.
	var fromPlatforms []v1.Platform
	for manifestIdx := range mergeFromIndexManifest.Manifests {
		child := mergeFromIndexManifest.Manifests[manifestIdx]
		if child.Platform != nil {
			fromPlatforms = append(fromPlatforms, *child.Platform)
		}
	}

	// Filter out all the platforms in mergeToIndex that were already in mergeFromIndex.
	mergeToIndex = mutate.RemoveManifests(mergeToIndex, func(manifest v1.Descriptor) bool {
		if manifest.Platform == nil {
			return false
		}

		for _, fromPlatform := range fromPlatforms {
			if manifest.Platform.Satisfies(fromPlatform) {
				return true
			}
		}

		return false
	})

	var adds []mutate.IndexAddendum
	for manifestIdx := range mergeFromIndexManifest.Manifests {
		child := mergeFromIndexManifest.Manifests[manifestIdx]
		switch {
		case child.MediaType.IsImage():
			img, err := mergeFromIndex.Image(child.Digest)
			if err != nil {
				return nil, err
			}
			adds = append(adds, mutate.IndexAddendum{
				Add:        img,
				Descriptor: child,
			})
		case child.MediaType.IsIndex():
			idx, err := mergeFromIndex.ImageIndex(child.Digest)
			if err != nil {
				return nil, err
			}
			adds = append(adds, mutate.IndexAddendum{
				Add:        idx,
				Descriptor: child,
			})
		default:
			return nil, fmt.Errorf("unexpected child %q with media type %q", child.Digest, child.MediaType)
		}
	}

	mergedIndex := mutate.AppendManifests(mergeToIndex, adds...)

	return mergedIndex, nil
}
