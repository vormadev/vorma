//go:build !prod

package dist

import "io/fs"

var FS fs.FS
