// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/mesosphere/mindthegap/cmd/push/imagebundle"
)

func NewCommand(ioStreams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use: "push",
	}

	cmd.AddCommand(imagebundle.NewCommand(ioStreams))
	return cmd
}
