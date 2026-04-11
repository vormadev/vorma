//go:build !prod

package md

import "os"

var fs = os.DirFS("app/md/content")
