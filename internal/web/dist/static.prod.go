//go:build prod

package dist

import (
	"embed"

	"github.com/vormadev/vorma/kit/fsutil"
)

//go:embed all:.wavedist/static
var embedFS embed.FS

var DistStaticFS = fsutil.MustSub(embedFS, ".waveout/static")
