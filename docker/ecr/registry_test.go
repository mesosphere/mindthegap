// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ecr

import "testing"

func TestIsECRRegistry(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		registryAddress string
		want            bool
	}{{
		name:            "ECR",
		registryAddress: "123456789.dkr.ecr.us-east-1.amazonaws.com",
		want:            true,
	}, {
		name:            "ECR with https protocol",
		registryAddress: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
		want:            true,
	}, {
		name:            "ECR with http protocol",
		registryAddress: "http://123456789.dkr.ecr.us-east-1.amazonaws.com",
		want:            false,
	}, {
		name:            "non-ECR",
		registryAddress: "gcr.io",
		want:            false,
	}, {
		name:            "non-ECR with https protocol",
		registryAddress: "https://gcr.io",
		want:            false,
	}, {
		name:            "non-ECR with http protocol",
		registryAddress: "http://gcr.io",
		want:            false,
	}}
	for _, tt := range tests {
		tt := tt // Capture range variable.
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsECRRegistry(tt.registryAddress); got != tt.want {
				t.Errorf("IsECRRegistry() = %v, want %v", got, tt.want)
			}
		})
	}
}
