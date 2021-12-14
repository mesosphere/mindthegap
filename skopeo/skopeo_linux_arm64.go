package skopeo

import _ "embed"

//go:embed static/skopeo-linux-arm64
var skopeoBinary []byte
