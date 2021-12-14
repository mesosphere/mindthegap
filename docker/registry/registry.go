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
	Port             uint16
}

func (c Config) ToRegistryConfiguration() (*configuration.Configuration, error) {
	var configTmpl = `
version: 0.1
storage:
  filesystem:
    rootdirectory: {{ .StorageDirectory }}
  maintenance:
    uploadpurging:
      enabled: false
http:
  net: tcp
  addr: localhost:{{ .Port }}
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

	tmpl := template.New("registryConfig")
	template.Must(tmpl.Parse(configTmpl))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct {
		StorageDirectory string
		Port             uint16
	}{c.StorageDirectory, port}); err != nil {
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
