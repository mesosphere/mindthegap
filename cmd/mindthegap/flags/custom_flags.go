// Copyright 2021-2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package flags

import (
	"fmt"
	"net/url"
	"path"
)

const (
	httpScheme = "http"
)

type RegistryURI struct {
	raw     string
	scheme  string
	address string
	host    string
}

func (v RegistryURI) String() string {
	return v.raw
}

func (v *RegistryURI) Set(value string) (err error) {
	v.raw = value
	v.scheme, v.address, v.host, err = parsePossibleURI(value)

	return
}

func parsePossibleURI(raw string) (scheme, address, host string, err error) {
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Host == "" {
		// parse again with a scheme to make it a valid URI
		u, err = url.ParseRequestURI("unused://" + raw)
		if err != nil {
			return "", "", "", fmt.Errorf("could not parse raw url: %q, error: %w", raw, err)
		}
	} else {
		// only set the scheme when set by the caller
		scheme = u.Scheme
	}

	address = path.Join(u.Host, u.Path)

	return scheme, address, u.Host, nil
}

func (v RegistryURI) Scheme() string {
	return v.scheme
}

func (v RegistryURI) Address() string {
	return v.address
}

func (v RegistryURI) Host() string {
	return v.host
}

func (*RegistryURI) Type() string {
	return "string"
}
