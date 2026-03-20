package main

import (
	"os"
	"site/backend/src/app"
	_ "site/frontend/src/vorma.gen"

	"github.com/vormadev/vorma/vorma2/vormabuild"
	"github.com/vormadev/vorma/wave/wavebuild"
)

func main() {
	wavebuild.Build(wavebuild.BuildOpts{
		ConfigPath: "./backend/wave.config.json",
		IsDev:      len(os.Args) > 1 && os.Args[1] == "--dev",
		Plugins:    []*wavebuild.Plugin{vormabuild.NewPlugin(app.App)},
	})
}
