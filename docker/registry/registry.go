// Copyright 2021-2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry/handlers"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem"
	"github.com/phayes/freeport"
	"github.com/sirupsen/logrus"
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
	delegate *http.Server
	address  string
}

func NewRegistry(cfg Config) (*Registry, error) {
	registryConfig, err := cfg.ToRegistryConfiguration()
	if err != nil {
		return nil, err
	}

	logrus.SetLevel(logrus.FatalLevel)
	regHandler := handlers.NewApp(context.Background(), registryConfig)

	reg := &http.Server{
		Addr:              registryConfig.HTTP.Addr,
		Handler:           regHandler,
		ReadHeaderTimeout: 1 * time.Second,
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

func (r Registry) Shutdown(ctx context.Context) error {
	return r.delegate.Shutdown(ctx)
}

func (r Registry) ListenAndServe() error {
	var err error
	if r.config.HTTP.TLS.Certificate != "" && r.config.HTTP.TLS.Key != "" {
		err = r.delegate.ListenAndServeTLS(r.config.HTTP.TLS.Certificate, r.config.HTTP.TLS.Key)
	} else {
		err = r.delegate.ListenAndServe()
	}

	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
