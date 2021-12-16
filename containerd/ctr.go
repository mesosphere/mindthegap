// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"context"
	"fmt"
	"os/exec"
)

type CtrOption func() string

func ImportImage(ctx context.Context, src, tag, containerdNamespace string, debug bool) ([]byte, error) {
	baseArgs := []string{"-n", containerdNamespace}
	if debug {
		baseArgs = append(baseArgs, "--debug")
	}
	//nolint:gosec // Args are fine.
	cmd := exec.CommandContext(ctx, "ctr", append(baseArgs, []string{"images", "pull", "--plain-http", src}...)...)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		return cmdOutput, fmt.Errorf("failed to pull image from temporary docker registry: %w", err)
	}

	if tag != "" {
		//nolint:gosec // Args are fine.
		cmd = exec.CommandContext(ctx, "ctr", append(baseArgs, []string{"images", "tag", "--force", src, tag}...)...)
		tagOutput, err := cmd.CombinedOutput()
		cmdOutput = append(cmdOutput, tagOutput...)
		if err != nil {
			return cmdOutput, fmt.Errorf("failed to tag image: %w", err)
		}

		//nolint:gosec // Args are fine.
		cmd = exec.CommandContext(ctx, "ctr", append(baseArgs, []string{"images", "rm", src}...)...)
		rmOutput, err := cmd.CombinedOutput()
		cmdOutput = append(cmdOutput, rmOutput...)
		if err != nil {
			return cmdOutput, fmt.Errorf("failed to tag image: %w", err)
		}
	}

	return cmdOutput, nil
}
