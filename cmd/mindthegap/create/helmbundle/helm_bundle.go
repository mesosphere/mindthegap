// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helmbundle

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cmd/mindthegap/create/bundle"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/flags"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/utils"
)

func NewCommand(out output.Output) *cobra.Command {
	var (
		configFile string
		outputFile string
		overwrite  bool
	)

	cmd := &cobra.Command{
		Use:   "helm-bundle",
		Short: "Create a Helm chart bundle",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.ValidateRequiredFlags(); err != nil {
				return err
			}

			if err := flags.ValidateFlagsThatRequireValues(cmd, "helm-charts-file"); err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			createBundleCmd := bundle.NewCommand(out)
			createBundleCmd.SetArgs([]string{
				"--helm-charts-file", configFile,
				"--output-file", outputFile,
				"--overwrite", strconv.FormatBool(overwrite),
			})
			return createBundleCmd.Execute()
		},
	}

	cmd.Flags().StringVar(&configFile, "helm-charts-file", "",
		"YAML file containing configuration of Helm charts to create bundle from")
	_ = cmd.MarkFlagRequired("helm-charts-file")
	cmd.Flags().
		StringVar(&outputFile, "output-file", "helm-charts.tar", "Output file to write Helm charts bundle to")
	cmd.Flags().
		BoolVar(&overwrite, "overwrite", false, "Overwrite Helm charts bundle file if it already exists")

	// TODO Unhide this from DKP CLI once DKP supports OCI registry for Helm charts.
	utils.AddCmdAnnotation(cmd, "exclude-from-dkp-cli", "true")

	cmd.Deprecated = `"mindthegap create helm-bundle" is deprecated, please use "mindthegap create bundle" instead`

	return cmd
}
