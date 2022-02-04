// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"github.com/mesosphere/dkp-cli-runtime/core/output"
	"github.com/spf13/cobra"

	"github.com/mesosphere/mindthegap/cmd/push/imagebundle"
)

func NewCommand(out output.Output) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push images from an image bundle into an existing image registry",
	}

	cmd.AddCommand(imagebundle.NewCommand(out))
	return cmd
}
