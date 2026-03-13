package main

import (
	"wave/internal/pkg/wavebuild"
)

func main() {
	wavebuild.Build(wavebuild.BuildOpts{
		ConfigPath: "./internal/tester_proj/wave.config.json",
		IsDev:      true,
	})
}
