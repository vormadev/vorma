package backend

import (
	"embed"

	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/wave"
)

//go:embed all:dist/static wave.config.json
var embeddedFS embed.FS

var Wave = wave.New(wave.Config{
	WaveConfigJSON:     fsutil.MustReadFile(embeddedFS, "wave.config.json"),
	WaveConfigFilePath: "backend/wave.config.json",
	DistStaticFS:       fsutil.MustSub(embeddedFS, "dist", "static"),
})
