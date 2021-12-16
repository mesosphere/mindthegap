// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package skopeo

import _ "embed"

//go:embed static/skopeo-windows-arm64.exe
var skopeoBinary []byte
