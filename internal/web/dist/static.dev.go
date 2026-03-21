//go:build !prod

package dist

import "io/fs"

var DistStaticFS fs.FS
