// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	"github.com/mesosphere/mindthegap/cmd/mindthegap/push/bundle"
	"github.com/mesosphere/mindthegap/cmd/mindthegap/utils"
)

func NewCommand(out output.Output) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push images or Helm charts from bundles into an existing OCI registry",
	}

	imageBundleCmd := bundle.NewCommand(out, "image-bundle")
	imageBundleCmd.Deprecated = "Please use `push bundle` command instead"
	cmd.AddCommand(imageBundleCmd)

	helmBundleCmd := bundle.NewCommand(out, "helm-bundle")
	helmBundleCmd.Deprecated = "Please use `push bundle` command instead"
	// TODO Unhide this from DKP CLI once DKP supports OCI registry for Helm charts.
	utils.AddCmdAnnotation(helmBundleCmd, "exclude-from-dkp-cli", "true")
	cmd.AddCommand(helmBundleCmd)

	bundleCmd := bundle.NewCommand(out, "bundle")
	cmd.AddCommand(bundleCmd)

	return cmd
}
