// Copyright 2021 D2iQ, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"github.com/mesosphere/mindthegap/containerd"
	"github.com/mesosphere/mindthegap/docker/registry"
)

func NewCommand(ioStreams genericclioptions.IOStreams) *cobra.Command {
	var (
		imageBundleFile     string
		containerdNamespace string
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
			reg, err := registry.NewRegistry(registry.Config{StorageDirectory: tempDir, ReadOnly: true})
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

			for registryName, registryConfig := range cfg {
				for imageName, imageTags := range registryConfig.Images {
					for _, imageTag := range imageTags {
						srcImageName := fmt.Sprintf("%s/%s:%s", reg.Address(), imageName, imageTag)
						destImageName := fmt.Sprintf("%s/%s:%s", registryName, imageName, imageTag)
						statusLogger.Start(fmt.Sprintf("Importing %s", destImageName))
						ctrOutput, err := containerd.ImportImage(
							context.TODO(), srcImageName, destImageName, containerdNamespace, klog.V(4).Enabled(),
						)
						if err != nil {
							klog.Info(string(ctrOutput))
							statusLogger.End(false)
							return err
						}
						klog.V(4).Info(string(ctrOutput))
						statusLogger.End(true)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&imageBundleFile, "image-bundle", "", "Tarball containing list of images to push")
	_ = cmd.MarkFlagRequired("image-bundle")
	cmd.Flags().StringVar(&containerdNamespace, "containerd-namespace", "k8s.io",
		"Containerd namespace to import images into")

	return cmd
}
