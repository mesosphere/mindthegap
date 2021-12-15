// Copyright 2021 Jimmi Dyson <jimmidyson@gmail.com>
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
