//go:build prod

package waveapp

import (
	"embed"
	"fmt"

	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/wave"
)

func init() { fmt.Println("waveapp: running init() in wave.prod.go") }

//go:embed all:.waveout/static
var embedFS embed.FS

var WaveOpts = wave.Options{
	DistStaticFS: fsutil.MustSub(embedFS, ".waveout/static"),
}
