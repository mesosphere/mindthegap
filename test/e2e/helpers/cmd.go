// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/spf13/cobra"

	"github.com/mesosphere/dkp-cli-runtime/core/output"
)

func NewCommand(newFn func(out output.Output) *cobra.Command) *cobra.Command {
	cmd := newFn(output.NewNonInteractiveShell(ginkgo.GinkgoWriter, ginkgo.GinkgoWriter, 10))
	cmd.SilenceUsage = true
	return cmd
}
