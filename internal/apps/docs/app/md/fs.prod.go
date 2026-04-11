//go:build prod

package md

import (
	"embed"

	"github.com/vormadev/vorma/kit/fsutil"
)

//go:embed all:content
var efs embed.FS

var fs = fsutil.MustSub(efs, "content")
