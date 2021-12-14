package skopeo

import _ "embed"

//go:embed static/skopeo-darwin-amd64
var skopeoBinary []byte
