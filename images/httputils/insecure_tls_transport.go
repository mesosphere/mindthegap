// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httputils

import "net/http"

func InsecureTLSRoundTripper(rt http.RoundTripper) (http.RoundTripper, error) {
	return TLSConfiguredRoundTripper(rt, "", true, "")
}
