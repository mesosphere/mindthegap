// Copyright 2021-2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"context"
	"fmt"
	"os/exec"
)

type CtrOption func() string

func ImportImageArchive(
	ctx context.Context,
	archivePath, containerdNamespace string,
) ([]byte, error) {
	baseArgs := []string{"-n", containerdNamespace}
	//nolint:gosec // Args are fine.
	cmd := exec.CommandContext(
		ctx,
		"ctr",
		append(
			baseArgs,
			[]string{
				"images",
				"import",
				"--no-unpack",
				"--all-platforms",
				"--digests",
				archivePath,
			}...)...)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		return cmdOutput, fmt.Errorf("failed to import image(s) from image archive: %w", err)
	}

	return cmdOutput, nil
}
