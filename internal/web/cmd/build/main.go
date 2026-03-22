package main

import (
	"os"
	_ "web/gen/vorma"
	"web/pkg/app"

	"github.com/vormadev/vorma/vorma2/vormabuild"
	"github.com/vormadev/vorma/wave/wavebuild"
)

func main() {
	wavebuild.Build(wavebuild.BuildOpts{
		ConfigPath: "./wave.config.json", // cwd-relative
		IsDev:      len(os.Args) > 1 && os.Args[1] == "--dev",
		Plugins:    []*wavebuild.Plugin{vormabuild.NewPlugin(app.App)},
	})
}
