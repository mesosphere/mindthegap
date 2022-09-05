// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ecr

import (
	"net/url"
	"regexp"
)

var ecrRegistryRegexp = regexp.MustCompile(`\.dkr\.ecr\.[^.]+\.amazonaws\.com$`)

func IsECRRegistry(registryAddress string) bool {
	u, err := url.Parse(registryAddress)
	if err != nil {
		return false
	}
	return ecrRegistryRegexp.MatchString(u.Hostname())
}
