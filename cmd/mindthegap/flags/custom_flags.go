// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package flags

import (
	"strings"
)

const (
	httpScheme  = "http"
	httpsScheme = "https"

	httpSchemePrefix  = "http://"
	httpsSchemePrefix = "https://"
)

type RegistryURI struct {
	raw     string
	scheme  string
	address string
}

func (v RegistryURI) String() string {
	return v.raw
}

func (v *RegistryURI) Set(value string) error {
	v.raw = value
	v.scheme, v.address = parsePossibleURI(value)

	return nil
}

func parsePossibleURI(value string) (string, string) {
	scheme := ""
	address := value
	if strings.HasPrefix(value, httpSchemePrefix) {
		scheme = httpScheme
		address = strings.TrimPrefix(value, httpSchemePrefix)
	} else if strings.HasPrefix(value, httpsSchemePrefix) {
		scheme = httpsScheme
		address = strings.TrimPrefix(value, httpsSchemePrefix)
	}

	return scheme, address
}

func (v RegistryURI) Scheme() string {
	return v.scheme
}

func (v RegistryURI) Address() string {
	return v.address
}

func (*RegistryURI) Type() string {
	return "string"
}
