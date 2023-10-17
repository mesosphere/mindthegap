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
)

func TLSConfiguredRoundTripper(
	rt http.RoundTripper,
	host string,
	insecureTLSSkipVerify bool,
	caCertificateFile string,
) (http.RoundTripper, error) {
	tr := rt.(*http.Transport).Clone()

	if insecureTLSSkipVerify {
		tr.TLSClientConfig.InsecureSkipVerify = insecureTLSSkipVerify
		return tr, nil
	}

	if tr.TLSClientConfig.RootCAs == nil {
		systemPool, err := tlsconfig.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("unable to get system cert pool: %v", err)
		}
		tr.TLSClientConfig.RootCAs = systemPool
	}

	hostDockerCertsDir := registry.HostCertsDir(host)
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

	if caCertificateFile != "" {
		b, err := os.ReadFile(caCertificateFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read specified CA file: %w", err)
		}
		_ = tr.TLSClientConfig.RootCAs.AppendCertsFromPEM(b)
	}

	rt = tr

	// Add any http headers if they are set in the config file.
	cf, err := config.Load(os.Getenv("DOCKER_CONFIG"))
	if err != nil {
		logs.Debug.Printf("failed to read config file: %v", err)
	} else if len(cf.HTTPHeaders) != 0 {
		rt = &headerTransport{
			inner:       tr,
			httpHeaders: cf.HTTPHeaders,
		}
	}

	return rt, nil
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
