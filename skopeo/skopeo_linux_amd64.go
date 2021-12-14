package skopeo

import _ "embed"

//go:embed static/skopeo-linux-amd64
var skopeoBinary []byte
