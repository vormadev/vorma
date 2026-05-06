//go:build prod

package markdown

import (
	"embed"

	"github.com/vormadev/vorma/kit/fsutil"
)

//go:embed all:content
var efs embed.FS

var fs = fsutil.MustSub(efs, "content")
