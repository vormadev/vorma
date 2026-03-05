//go:build prod

package backend

import (
	"embed"

	"github.com/vormadev/vorma/wave"
)

//go:embed all:.wavedist/static wave.config.json
var embedFS embed.FS

var Wave = wave.New(wave.Config{
	FS:         embedFS,
	ConfigPath: "wave.config.json",
})
