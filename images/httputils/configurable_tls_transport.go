// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httputils

import (
	"net/http"

	"k8s.io/client-go/transport"
)

type configurableTLSTransport struct {
	cfg               TLSHostsConfig
	delegateTransport http.RoundTripper
}

func (rt *configurableTLSTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}

	if tlsConfig, ok := rt.cfg[host]; ok {
		t, err := transport.New(
			&transport.Config{
				TLS: tlsConfig,
			},
		)
		if err != nil {
			return nil, err
		}
		return t.RoundTrip(req)
	}

	return rt.delegateTransport.RoundTrip(req)
}

type TLSHostsConfig map[string]transport.TLSConfig

func NewConfigurableTLSRoundTripper(
	delegate http.RoundTripper,
	cfg TLSHostsConfig,
) http.RoundTripper {
	return &configurableTLSTransport{
		cfg:               cfg,
		delegateTransport: delegate,
	}
}
