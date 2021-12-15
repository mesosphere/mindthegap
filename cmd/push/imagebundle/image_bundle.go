package imagebundle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mesosphere/dkp-cli/runtime/cli"
	"github.com/mesosphere/dkp-cli/runtime/cmd/log"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/mesosphere/mindthegap/config"
	"github.com/mesosphere/mindthegap/docker/registry"
	"github.com/mesosphere/mindthegap/skopeo"
)

func NewCommand(ioStreams genericclioptions.IOStreams) *cobra.Command {
	var (
		imageBundleFile           string
		destRegistry              string
		destRegistrySkipTLSVerify bool
	)

	cmd := &cobra.Command{
		Use: "image-bundle",
		RunE: func(cmd *cobra.Command, args []string) error {
			klog.SetOutput(ioStreams.ErrOut)
			logger := log.NewLogger(ioStreams.ErrOut)
			statusLogger := cli.StatusForLogger(logger)

			statusLogger.Start("Creating temporary directory")
			tempDir, err := os.MkdirTemp("", ".image-bundle-*")
			if err != nil {
				statusLogger.End(false)
				return fmt.Errorf("failed to create temporary directory: %w", err)
			}
			defer os.RemoveAll(tempDir)
			statusLogger.End(true)

			statusLogger.Start("Unarchiving image bundle")
			err = archiver.Unarchive(imageBundleFile, tempDir)
			if err != nil {
				statusLogger.End(false)
				return fmt.Errorf("failed to unarchive image bundle: %w", err)
			}
			statusLogger.End(true)

			statusLogger.Start("Parsing image bundle config")
			cfg, err := config.ParseFile(filepath.Join(tempDir, "images.yaml"))
			if err != nil {
				statusLogger.End(false)
				return err
			}
			klog.V(4).Infof("Images config: %+v", cfg)
			statusLogger.End(true)

			statusLogger.Start("Starting temporary Docker registry")
			reg, err := registry.NewRegistry(registry.Config{StorageDirectory: tempDir})
			if err != nil {
				statusLogger.End(false)
				return fmt.Errorf("failed to create local Docker registry: %w", err)
			}
			go func() {
				if err := reg.ListenAndServe(); err != nil {
					fmt.Fprintf(ioStreams.ErrOut, "error serving Docker registry: %v\n", err)
					os.Exit(2)
				}
			}()
			statusLogger.End(true)

			skopeoRunner, skopeoCleanup := skopeo.NewRunner()
			defer func() { _ = skopeoCleanup() }()

			err = skopeoRunner.AttemptToLoginToRegistry(context.TODO(), destRegistry, klog.V(4).Enabled())
			if err != nil {
				return fmt.Errorf("error logging in to target registry: %w", err)
			}

			for _, registryConfig := range cfg {
				skopeoOpts := []skopeo.SkopeoOption{skopeo.DisableSrcTLSVerify()}
				if klog.V(4).Enabled() {
					skopeoOpts = append(skopeoOpts, skopeo.Debug())
				}
				if destRegistrySkipTLSVerify {
					skopeoOpts = append(skopeoOpts, skopeo.DisableDestTLSVerify())
				}
				for imageName, imageTags := range registryConfig.Images {
					for _, imageTag := range imageTags {
						statusLogger.Start(
							fmt.Sprintf("Copying %s/%s:%s to %s/%s:%s",
								reg.Address(), imageName, imageTag,
								destRegistry, imageName, imageTag,
							),
						)
						skopeoOutput, err := skopeoRunner.Copy(context.TODO(),
							fmt.Sprintf("docker://%s/%s:%s", reg.Address(), imageName, imageTag),
							fmt.Sprintf("docker://%s/%s:%s", destRegistry, imageName, imageTag),
							append(
								skopeoOpts, skopeo.All(),
							)...,
						)
						if err != nil {
							klog.Info(string(skopeoOutput))
							statusLogger.End(false)
							return err
						}
						klog.V(4).Info(string(skopeoOutput))
						statusLogger.End(true)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&imageBundleFile, "image-bundle", "", "Tarball containing list of images to push")
	_ = cmd.MarkFlagRequired("image-bundle")
	cmd.Flags().StringVar(&destRegistry, "to-registry", "", "Registry to push images to")
	_ = cmd.MarkFlagRequired("to-registry")
	cmd.Flags().BoolVar(&destRegistrySkipTLSVerify, "to-registry-insecure-skip-tls-verify", false,
		"Skip TLS verification of registry to push images to (use for http registries)")

	return cmd
}
