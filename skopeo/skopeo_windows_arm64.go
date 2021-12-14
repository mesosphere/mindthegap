package skopeo

import _ "embed"

//go:embed static/skopeo-windows-arm64.exe
var skopeoBinary []byte
