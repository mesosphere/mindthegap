package skopeo

import _ "embed"

//go:embed static/skopeo-windows-amd64.exe
var skopeoBinary []byte
