//go:build prod

package dist

import (
	"embed"

	"github.com/vormadev/vorma/kit/fsutil"
)

//go:embed all:.vorma/static
var efs embed.FS

var FS = fsutil.MustSub(efs, ".vorma/static")
