package webs

import _ "embed"

//go:embed navapi-web.zip
var staticFile []byte

func Static() []byte {
	return staticFile
}
