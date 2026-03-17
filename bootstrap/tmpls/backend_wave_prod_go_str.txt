//go:build prod

package backend

import (
	"embed"

	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/wave"
)

//go:embed all:.waveout/static
var embedFS embed.FS

var Wave = wave.New(wave.Options{
	DistStaticFS: fsutil.MustSub(embedFS, ".waveout/static"),
})
