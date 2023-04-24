// Copyright 2021-2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	configWithoutTLS = `
version: 0.1
storage:
  filesystem:
    rootdirectory: /tmp
  maintenance:
    uploadpurging:
      enabled: false
    readonly:
      enabled: true
http:
  net: tcp
  addr: 0.0.0.0:5000
log:
  accesslog:
    disabled: true
  level: error
`

	configWithTLS = `
version: 0.1
storage:
  filesystem:
    rootdirectory: /tmp
  maintenance:
    uploadpurging:
      enabled: false
    readonly:
      enabled: true
http:
  net: tcp
  addr: 0.0.0.0:5000
  tls:
    certificate: /tmp/tls.cert
    key: /tmp/tls.key
log:
  accesslog:
    disabled: true
  level: error
`
)

func Test_registryConfiguration_withoutTLS(t *testing.T) {
	t.Parallel()
	c := Config{
		StorageDirectory: "/tmp",
		Host:             "0.0.0.0",
		Port:             5000,
		ReadOnly:         true,
	}

	config, err := registryConfiguration(c)
	require.NoError(t, err)
	require.Equal(t, configWithoutTLS, config)
}

func Test_registryConfiguration_withTLS(t *testing.T) {
	t.Parallel()
	c := Config{
		StorageDirectory: "/tmp",
		Host:             "0.0.0.0",
		Port:             5000,
		ReadOnly:         true,
		TLS: TLS{
			Certificate: "/tmp/tls.cert",
			Key:         "/tmp/tls.key",
		},
	}

	config, err := registryConfiguration(c)
	require.NoError(t, err)
	require.Equal(t, configWithTLS, config)
}
