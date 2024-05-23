// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imagebundle

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cmd/mindthegap/create/bundle"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/flags"
)

func NewCommand(out output.Output) *cobra.Command {
	var (
		configFile           string
		platforms            = flags.NewPlatformsValue("linux/amd64")
		outputFile           string
		overwrite            bool
		imagePullConcurrency int
	)

	cmd := &cobra.Command{
		Use:   "image-bundle",
		Short: "Create an image bundle",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.ValidateRequiredFlags(); err != nil {
				return err
			}

			if err := flags.ValidateFlagsThatRequireValues(cmd, "images-file"); err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			createBundleCmd := bundle.NewCommand(out)
			createBundleCmd.SetArgs([]string{
				"--images-file", configFile,
				"--platform", strings.Join(platforms.GetSlice(), ","),
				"--output-file", outputFile,
				"--overwrite", strconv.FormatBool(overwrite),
				"--image-pull-concurrency", strconv.Itoa(imagePullConcurrency),
			})
			return createBundleCmd.Execute()
		},
	}

	cmd.Flags().StringVar(&configFile, "images-file", "",
		"File containing list of images to create bundle from, either as YAML configuration or a simple list of images")
	_ = cmd.MarkFlagRequired("images-file")
	cmd.Flags().Var(&platforms, "platform", "platforms to download images for (required format: <os>/<arch>[/<variant>])")
	cmd.Flags().
		StringVar(&outputFile, "output-file", "images.tar", "Output file to write image bundle to")
	cmd.Flags().
		BoolVar(&overwrite, "overwrite", false, "Overwrite image bundle file if it already exists")
	cmd.Flags().
		IntVar(&imagePullConcurrency, "image-pull-concurrency", 1, "Image pull concurrency")

	cmd.Deprecated = `"mindthegap create image-bundle" is deprecated, please use "mindthegap create bundle" instead`

	return cmd
}
