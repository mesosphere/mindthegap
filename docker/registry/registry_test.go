// Copyright 2021 D2iQ, Inc. All rights reserved.
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

	configWithArchive = `
version: 0.1
storage:
  archive:
    archives: ["/tmp/1.tar","/some/other/path/2.tar"]
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
)

func Test_registryConfiguration_withoutTLS(t *testing.T) {
	t.Parallel()
	c := Config{
		Storage:  FilesystemStorage("/tmp"),
		Host:     "0.0.0.0",
		Port:     5000,
		ReadOnly: true,
	}

	config, err := registryConfiguration(c)
	require.NoError(t, err)
	require.Equal(t, configWithoutTLS, config)
}

func Test_registryConfiguration_withTLS(t *testing.T) {
	t.Parallel()
	c := Config{
		Storage:  FilesystemStorage("/tmp"),
		Host:     "0.0.0.0",
		Port:     5000,
		ReadOnly: true,
		TLS: TLS{
			Certificate: "/tmp/tls.cert",
			Key:         "/tmp/tls.key",
		},
	}

	config, err := registryConfiguration(c)
	require.NoError(t, err)
	require.Equal(t, configWithTLS, config)
}

func Test_registryConfiguration_archive(t *testing.T) {
	t.Parallel()
	storage, err := ArchiveStorage("", "/tmp/1.tar", "/some/other/path/2.tar")
	require.NoError(t, err)
	c := Config{
		Storage:  storage,
		Host:     "0.0.0.0",
		Port:     5000,
		ReadOnly: true,
	}

	config, err := registryConfiguration(c)
	require.NoError(t, err)
	require.Equal(t, configWithArchive, config)
}

func Test_registryConfiguration_archiveDisallowsCompressedArchives(t *testing.T) {
	t.Parallel()
	_, err := ArchiveStorage("", "/tmp/1.tar", "/some/other/path/2.tar.gz")
	require.ErrorContains(t, err, "compressed tar archives (.tar.gz) are not supported")
}
