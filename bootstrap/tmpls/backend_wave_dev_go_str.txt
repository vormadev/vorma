//go:build !prod

package backend

import (
	"os"

	"github.com/vormadev/vorma/wave"
)

var Wave = wave.New(wave.Config{
	FS:         os.DirFS("backend"),
	ConfigPath: "wave.config.json",
})
