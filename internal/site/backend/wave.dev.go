//go:build !prod

package backend

import (
	"os"

	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/wave"
)

var dirFS = os.DirFS("backend")

var Wave = wave.New(wave.Config{
	WaveConfigJSON: fsutil.MustReadFile(dirFS, "wave.config.json"),
	DistStaticFS:   fsutil.MustSub(dirFS, "dist", "static"),
})
