package main

import (
	"os"
	_ "site/__wave/vorma.gen"
	"site/backend/src/app"

	"github.com/vormadev/vorma/vorma2/vormabuild"
	"github.com/vormadev/vorma/wave/wavebuild"
)

func main() {
	wavebuild.Build(wavebuild.BuildOpts{
		ConfigPath: "./__wave/wave.config.json",
		IsDev:      len(os.Args) > 1 && os.Args[1] == "--dev",
		Plugins:    []*wavebuild.Plugin{vormabuild.NewPlugin(app.App)},
	})
}
