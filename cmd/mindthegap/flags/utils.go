// Copyright 2021-2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package flags

// SkipTLSVerify returns true if registrySkipTLSVerify is true
// or registryURI URI is http.
func SkipTLSVerify(registrySkipTLSVerify bool, registryURI *RegistryURI) bool {
	return registrySkipTLSVerify || registryURI.Scheme() == httpScheme
}
