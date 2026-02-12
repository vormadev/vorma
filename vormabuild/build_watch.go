package vormabuild

import (
	"path/filepath"

	"github.com/vormadev/vorma/vormaruntime"
	"github.com/vormadev/vorma/wave"
)

func injectDefaultWatchPatterns(v *vormaruntime.Vorma) {
	includeDefaults := true
	if v.Config.IncludeDefaults != nil {
		includeDefaults = *v.Config.IncludeDefaults
	}
	if !includeDefaults {
		return
	}

	cfg := v.Wave.GetParsedConfig()
	patterns := getDefaultWatchPatterns(v)
	cfg.FrameworkWatchPatterns = append(cfg.FrameworkWatchPatterns, patterns...)

	if v.Config.TSGenOutDir != "" {
		cfg.FrameworkPublicFileMapOutDir = v.Config.TSGenOutDir
		cfg.FrameworkIgnoredPatterns = append(cfg.FrameworkIgnoredPatterns,
			filepath.Join(v.Config.TSGenOutDir, wave.GeneratedTSFileName),
			filepath.Join(v.Config.TSGenOutDir, wave.PublicFileMapTSName),
			filepath.Join(v.Config.TSGenOutDir, wave.PublicFileMapJSONName),
		)
	}
}

func getDefaultWatchPatterns(v *vormaruntime.Vorma) []wave.WatchedFile {
	var patterns []wave.WatchedFile

	if pattern, ok := routeDefinitionsWatchPattern(v); ok {
		patterns = append(patterns, pattern)
	}

	if pattern, ok := htmlTemplateWatchPattern(v); ok {
		patterns = append(patterns, pattern)
	}

	patterns = append(patterns, goFilesWatchPattern())

	return patterns
}

func routeDefinitionsWatchPattern(v *vormaruntime.Vorma) (wave.WatchedFile, bool) {
	clientRouteDefsFile := v.Config.ClientRouteDefsFile
	if clientRouteDefsFile == "" {
		return wave.WatchedFile{}, false
	}

	return wave.WatchedFile{
		Pattern:         clientRouteDefsFile,
		RunOnChangeOnly: true, // Skip standard build - callback handles everything
		OnChangeHooks: []wave.OnChangeHook{{
			Callback: routeDefinitionsOnChangeCallback(v),
		}},
		SkipRebuildingNotification: true,
	}, true
}

func htmlTemplateWatchPattern(v *vormaruntime.Vorma) (wave.WatchedFile, bool) {
	htmlTemplateLocation := v.Config.HTMLTemplateLocation
	privateStaticDir := v.Wave.GetPrivateStaticDir()
	if htmlTemplateLocation == "" || privateStaticDir == "" {
		return wave.WatchedFile{}, false
	}

	templatePath := filepath.Join(privateStaticDir, htmlTemplateLocation)
	return wave.WatchedFile{
		Pattern:         templatePath,
		RunOnChangeOnly: true, // Skip standard build - callback handles everything
		OnChangeHooks: []wave.OnChangeHook{{
			Callback: htmlTemplateOnChangeCallback(v),
		}},
	}, true
}

func goFilesWatchPattern() wave.WatchedFile {
	return wave.WatchedFile{
		Pattern: "**/*.go",
		OnChangeHooks: []wave.OnChangeHook{{
			Cmd:    "DevBuildHook",
			Timing: wave.OnChangeStrategyConcurrent,
		}},
	}
}

func routeDefinitionsOnChangeCallback(v *vormaruntime.Vorma) func(*wave.HookContext) (*wave.RefreshAction, error) {
	return func(ctx *wave.HookContext) (*wave.RefreshAction, error) {
		if err := rebuildRoutesOnly(v); err != nil {
			return nil, err
		}

		if ctx.AppStoppedForBatch {
			return nil, nil
		}

		return getReloadActionForEndpointWithFallback(
			v,
			vormaruntime.Dev_ReloadRoutesPath,
			"route reload endpoint failed, falling back to restart",
		), nil
	}
}

func htmlTemplateOnChangeCallback(v *vormaruntime.Vorma) func(*wave.HookContext) (*wave.RefreshAction, error) {
	return func(ctx *wave.HookContext) (*wave.RefreshAction, error) {
		if ctx.AppStoppedForBatch {
			return nil, nil
		}

		return getReloadActionForEndpointWithFallback(
			v,
			vormaruntime.Dev_ReloadTemplatePath,
			"template reload endpoint failed, falling back to restart",
		), nil
	}
}
