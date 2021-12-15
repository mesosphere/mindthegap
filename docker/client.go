// Copyright 2021 Mesosphere, Inc. All rights reserved.
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

package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/moby/moby/client"
	"k8s.io/klog/v2"
)

func NewDockerClientFromEnv() (*client.Client, error) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client - is Docker running? Error: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	dockerVersion, err := dockerClient.ServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check Docker version - is Docker running? Error: %w", err)
	}
	klog.V(4).Infof("Docker server version: %v", dockerVersion)
	return dockerClient, nil
}
