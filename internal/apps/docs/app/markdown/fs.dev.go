//go:build !prod

package markdown

import "os"

var fs = os.DirFS("app/markdown/content")
