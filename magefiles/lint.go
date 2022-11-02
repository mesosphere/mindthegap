// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build mage

package main

import (
	"context"

	"github.com/magefile/mage/mg"

	"github.com/mesosphere/daggers/dagger/options"
	precommitdagger "github.com/mesosphere/daggers/dagger/precommit"
	"github.com/mesosphere/daggers/mage/precommit"
)

type Lint mg.Namespace

// Precommit runs precommit checks.
func (Lint) Precommit(ctx context.Context) error {
	return precommit.PrecommitWithOptions(ctx,
		precommitdagger.CustomizeContainer(
			options.DownloadExecutableFile(
				ctx,
				"https://github.com/mvdan/sh/releases/download/v3.5.1/shfmt_v3.5.1_linux_amd64",
				"/usr/local/bin/shfmt",
			),
			options.InstallGo(ctx),
		),
	)
}
