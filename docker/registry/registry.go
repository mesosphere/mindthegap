// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem"
	"github.com/phayes/freeport"
)

type Config struct {
	StorageDirectory string
	Host             string
	Port             uint16
	ReadOnly         bool
	TLS              TLS
}

type TLS struct {
	Certificate string
	Key         string
}

func (c Config) ToRegistryConfiguration() (*configuration.Configuration, error) {
	registryConfigString, err := registryConfiguration(c)
	if err != nil {
		return nil, err
	}

	registryConfig, err := configuration.Parse(strings.NewReader(registryConfigString))
	if err != nil {
		return nil, fmt.Errorf("failed to parse registry configuration: %w", err)
	}
	return registryConfig, nil
}

func registryConfiguration(c Config) (string, error) {
	configTmpl := `
version: 0.1
storage:
  filesystem:
    rootdirectory: {{ .StorageDirectory }}
  maintenance:
    uploadpurging:
      enabled: false
    readonly:
      enabled: {{ .ReadOnly }}
http:
  net: tcp
  addr: {{ .Host }}:{{ .Port }}
  {{- if .TLSCertificate }}
  tls:
    certificate: {{ .TLSCertificate }}
    key: {{ .TLSKey }}
  {{- end }}
log:
  accesslog:
    disabled: true
  level: error
`
	port := c.Port
	if port == 0 {
		freePort, err := freeport.GetFreePort()
		if err != nil {
			return "", fmt.Errorf("failed to get free port: %w", err)
		}
		port = uint16(freePort)
	}

	host := "localhost"
	if c.Host != "" {
		host = c.Host
	}

	tmpl := template.New("registryConfig")
	template.Must(tmpl.Parse(configTmpl))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct {
		StorageDirectory string
		Host             string
		Port             uint16
		ReadOnly         bool
		TLSCertificate   string
		TLSKey           string
	}{c.StorageDirectory, host, port, c.ReadOnly, c.TLS.Certificate, c.TLS.Key}); err != nil {
		return "", fmt.Errorf("failed to render registry configuration: %w", err)
	}

	return buf.String(), nil
}

type Registry struct {
	config   *configuration.Configuration
	delegate *registry.Registry
	address  string
}

func NewRegistry(cfg Config) (*Registry, error) {
	registryConfig, err := cfg.ToRegistryConfiguration()
	if err != nil {
		return nil, err
	}

	reg, err := registry.NewRegistry(context.TODO(), registryConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry: %w", err)
	}

	return &Registry{
		config:   registryConfig,
		delegate: reg,
		address:  registryConfig.HTTP.Addr,
	}, nil
}

func (r Registry) Address() string {
	return r.address
}

func (r *Registry) ListenAndServe() error {
	return r.delegate.ListenAndServe()
}
