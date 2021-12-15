// Copyright 2021 D2iQ, Inc. All rights reserved.
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
}

func (c Config) ToRegistryConfiguration() (*configuration.Configuration, error) {
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
log:
  accesslog:
    disabled: true
  level: error
`
	port := c.Port
	if port == 0 {
		freePort, err := freeport.GetFreePort()
		if err != nil {
			return nil, fmt.Errorf("failed to get free port: %w", err)
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
	}{c.StorageDirectory, host, port, c.ReadOnly}); err != nil {
		return nil, fmt.Errorf("failed to render registry configuration: %w", err)
	}

	registryConfig, err := configuration.Parse(strings.NewReader(buf.String()))
	if err != nil {
		return nil, fmt.Errorf("failed to parse registry configuration: %w", err)
	}
	return registryConfig, nil
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
