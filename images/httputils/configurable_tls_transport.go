// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httputils

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/cli/cli/config"
	"github.com/docker/docker/registry"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type configurableTLSTransport struct {
	cfg               TLSHostsConfig
	delegateTransport *http.Transport
}

func (rt *configurableTLSTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tr := rt.delegateTransport.Clone()
	// This function creates new http.Transport for every request.
	// When creating an image bundle this results in ~8x of the number of images.
	// Because this happens relatively fast, the OS may not clean up the TCP connections in time,
	// leading to a "socket: too many open files" error.
	// Because we need to use a new http.Transport based on the request's Host, the http.Transport cannot be reused,
	// therefore we also need to force close the idle connections.
	defer tr.CloseIdleConnections()

	if tr.TLSClientConfig.RootCAs == nil {
		systemPool, err := tlsconfig.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("unable to get system cert pool: %v", err)
		}
		tr.TLSClientConfig.RootCAs = systemPool
	} else {
		tr.TLSClientConfig.RootCAs = tr.TLSClientConfig.RootCAs.Clone()
	}

	host := req.Host
	if host == "" {
		host = req.URL.Host
	}

	tlsHostConfig, tlsHostConfigFound := rt.cfg[host]

	// Always returns nil error...
	hostDockerCertsDir, _ := registry.HostCertsDir(host)
	fs, err := os.ReadDir(hostDockerCertsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read from Docker registry certs: %w", err)
	}

	for _, f := range fs {
		if strings.HasSuffix(f.Name(), ".crt") {
			data, err := os.ReadFile(filepath.Join(hostDockerCertsDir, f.Name()))
			if err != nil {
				return nil, fmt.Errorf("failed tp read CA certificate from Docker certs.d: %w", err)
			}
			_ = tr.TLSClientConfig.RootCAs.AppendCertsFromPEM(data)
		}
	}

	tr.TLSClientConfig.InsecureSkipVerify = tlsHostConfigFound && tlsHostConfig.Insecure

	if tlsHostConfigFound && tlsHostConfig.CAFile != "" {
		b, err := os.ReadFile(tlsHostConfig.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read specified CA file: %w", err)
		}
		_ = tr.TLSClientConfig.RootCAs.AppendCertsFromPEM(b)
	}

	return tr.RoundTrip(req)
}

type TLSHostConfig struct {
	CAFile   string // Path of the PEM-encoded server trusted root certificates.
	Insecure bool   // Server should be accessed without verifying the certificate. For testing only.
}

type TLSHostsConfig map[string]TLSHostConfig

func NewConfigurableTLSRoundTripper(
	cfg TLSHostsConfig,
) http.RoundTripper {
	var tr http.RoundTripper = remote.DefaultTransport.(*http.Transport).Clone()

	// Add any http headers if they are set in the config file.
	cf, err := config.Load(os.Getenv("DOCKER_CONFIG"))
	if err != nil {
		logs.Debug.Printf("failed to read config file: %v", err)
	} else if len(cf.HTTPHeaders) != 0 {
		tr = &headerTransport{
			inner:       tr,
			httpHeaders: cf.HTTPHeaders,
		}
	}

	return &configurableTLSTransport{
		cfg:               cfg,
		delegateTransport: tr.(*http.Transport),
	}
}

// headerTransport sets headers on outgoing requests.
type headerTransport struct {
	httpHeaders map[string]string
	inner       http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (ht *headerTransport) RoundTrip(in *http.Request) (*http.Response, error) {
	for k, v := range ht.httpHeaders {
		if http.CanonicalHeaderKey(k) == "User-Agent" {
			// Docker sets this, which is annoying, since we're not docker.
			// We might want to revisit completely ignoring this.
			continue
		}
		in.Header.Set(k, v)
	}
	return ht.inner.RoundTrip(in)
}
