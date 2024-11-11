// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry/handlers"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem"
	"github.com/go-logr/logr"
	"github.com/phayes/freeport"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
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

		if freePort <= 0 || freePort > math.MaxUint16 {
			return "", fmt.Errorf(
				"invalid free port - must be between 1 and %d inclusive: %d",
				freePort,
				math.MaxUint16,
			)
		}
		port = uint16(freePort)
	}

	host := "127.0.0.1"
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

	return &Registry{
		config: registryConfig,
		delegate: &http.Server{
			Addr:              registryConfig.HTTP.Addr,
			Handler:           regHandler,
			ReadHeaderTimeout: 1 * time.Second,
		},
		address: registryConfig.HTTP.Addr,
	}, nil
}

func (r Registry) Address() string {
	return r.address
}

func (r Registry) Shutdown(ctx context.Context) error {
	return r.delegate.Shutdown(ctx)
}

func (r Registry) ListenAndServe(log logr.Logger) error {
	var err error
	if r.config.HTTP.TLS.Certificate != "" && r.config.HTTP.TLS.Key != "" {
		watcher, cwErr := certwatcher.New(r.config.HTTP.TLS.Certificate, r.config.HTTP.TLS.Key)
		if cwErr != nil {
			return fmt.Errorf("failed to read TLS certificate or key: %w", cwErr)
		}
		r.delegate.TLSConfig = &tls.Config{
			GetCertificate: watcher.GetCertificate,
			MinVersion:     tls.VersionTLS12,
		}
		go func() {
			if startErr := watcher.Start(context.TODO()); startErr != nil {
				panic(fmt.Sprintf("certwatcher Start failed: %v", startErr))
			}
		}()

		// Certificate and key are not passed to ListenAndServeTLS, because they
		// are read from r.delegate.TLSConfig.GetCertificate().
		err = r.delegate.ListenAndServeTLS("", "")
	} else {
		err = r.delegate.ListenAndServe()
	}

	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
