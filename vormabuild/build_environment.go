package vormabuild

import (
	"fmt"

	"github.com/vormadev/vorma/vormaruntime"
)

func injectFrameworkBuildHooks(v *vormaruntime.Vorma) {
	cfg := v.Wave.GetParsedConfig()
	if cfg.FrameworkDevBuildHook == "" {
		cfg.FrameworkDevBuildHook = fmt.Sprintf("go run ./%s --dev --hook", v.Config.MainBuildEntry)
	}
	if cfg.FrameworkProdBuildHook == "" {
		cfg.FrameworkProdBuildHook = fmt.Sprintf("go run ./%s --hook", v.Config.MainBuildEntry)
	}
}

func configureBuildEnvironment(v *vormaruntime.Vorma) {
	injectDefaultWatchPatterns(v)
	injectFrameworkBuildHooks(v)
}
