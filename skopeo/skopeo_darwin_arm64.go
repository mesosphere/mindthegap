package skopeo

import _ "embed"

//go:embed static/skopeo-darwin-arm64
var skopeoBinary []byte
