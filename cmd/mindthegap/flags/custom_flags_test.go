// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package flags

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parsePossibleURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		in              string
		expectedScheme  string
		expectedAddress string
	}{
		{
			name:            "no scheme",
			in:              "0.0.0.0:5000",
			expectedAddress: "0.0.0.0:5000",
		},
		{
			name:            "http scheme",
			in:              "http://0.0.0.0:5000",
			expectedScheme:  "http",
			expectedAddress: "0.0.0.0:5000",
		},
		{
			name:            "https scheme",
			in:              "https://0.0.0.0:5000",
			expectedScheme:  "https",
			expectedAddress: "0.0.0.0:5000",
		},
		{
			name:            "no scheme with path",
			in:              "0.0.0.0:5000/dkp",
			expectedAddress: "0.0.0.0:5000/dkp",
		},
		{
			name:            "http scheme with path",
			in:              "https://0.0.0.0:5000/dkp",
			expectedScheme:  "https",
			expectedAddress: "0.0.0.0:5000/dkp",
		},
		{
			name:            "https scheme with path",
			in:              "https://0.0.0.0:5000/dkp",
			expectedScheme:  "https",
			expectedAddress: "0.0.0.0:5000/dkp",
		},
	}
	for ti := range tests {
		tt := tests[ti]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			scheme, address, _ := parsePossibleURI(tt.in)
			require.Equal(t, tt.expectedScheme, scheme)
			require.Equal(t, tt.expectedAddress, address)
		})
	}
}
