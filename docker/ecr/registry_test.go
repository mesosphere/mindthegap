// Copyright 2021-2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ecr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	}, {
		name:            "ECR with FIPS protocol",
		registryAddress: "https://123456789.dkr.ecr-fips.us-east-1.amazonaws.com",
		want:            true,
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

func TestParseECRRegistry(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		registryAddress string
		wantError       string
		wantAccountID   string
		wantFips        bool
		wantRegion      string
	}{{
		name:            "Valid ECR with https protocol",
		registryAddress: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
		wantError:       "",
		wantAccountID:   "123456789",
		wantFips:        false,
		wantRegion:      "us-east-1",
	}, {
		name:            "Valid ECR without https protocol",
		registryAddress: "123456789.dkr.ecr.us-east-1.amazonaws.com",
		wantError:       "",
		wantAccountID:   "123456789",
		wantFips:        false,
		wantRegion:      "us-east-1",
	}, {
		name:            "ECR with FIPS",
		registryAddress: "https://123456789.dkr.ecr-fips.us-gov-east-1.amazonaws.com",
		wantError:       "",
		wantAccountID:   "123456789",
		wantFips:        true,
		wantRegion:      "us-gov-east-1",
	}, {
		name:            "public ECR",
		registryAddress: "public.ecr.aws",
		wantError:       "only private Amazon Elastic Container Registry supported",
		wantAccountID:   "",
		wantFips:        false,
		wantRegion:      "",
	}, {
		name:            "non ECR",
		registryAddress: "gcr.io",
		wantError:       "only private Amazon Elastic Container Registry supported",
		wantAccountID:   "",
		wantFips:        false,
		wantRegion:      "",
	}}
	for _, tt := range tests {
		tt := tt // Capture range variable.
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotID, gotFips, gotRegion, gotErr := ParseECRRegistry(tt.registryAddress)

			if tt.wantError != "" {
				require.ErrorContains(t, gotErr, tt.wantError)
			} else {
				require.NoError(t, gotErr)
				assert.Equal(t, tt.wantAccountID, gotID)
				assert.Equal(t, tt.wantFips, gotFips)
				assert.Equal(t, tt.wantRegion, gotRegion)
			}
		})
	}
}
