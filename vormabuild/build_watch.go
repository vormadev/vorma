package vormabuild

import (
	"path/filepath"

	"github.com/vormadev/vorma/vormaruntime"
	"github.com/vormadev/vorma/wave"
)

func injectDefaultWatchPatterns(v *vormaruntime.Vorma) {
	if !shouldInjectDefaultWatchPatterns(v) {
		return
	}

	cfg := v.Wave.GetParsedConfig()
	patterns := getDefaultWatchPatterns(v)
	appendMissingFrameworkWatchPatterns(cfg, patterns)

	if v.Config.TSGenOutDir != "" {
		injectGeneratedOutputPathsForDefaultWatchPatterns(cfg, v.Config.TSGenOutDir)
	}
}

func appendMissingFrameworkWatchPatterns(
	cfg *wave.ParsedConfig,
	defaultPatterns []wave.WatchedFile,
) {
	for _, defaultPattern := range defaultPatterns {
		if hasFrameworkWatchPattern(cfg.FrameworkWatchPatterns, defaultPattern.Pattern) {
			continue
		}
		cfg.FrameworkWatchPatterns = append(cfg.FrameworkWatchPatterns, defaultPattern)
	}
}

func hasFrameworkWatchPattern(
	existingPatterns []wave.WatchedFile,
	pattern string,
) bool {
	for _, existingPattern := range existingPatterns {
		if existingPattern.Pattern == pattern {
			return true
		}
	}
	return false
}

func getDefaultWatchPatterns(v *vormaruntime.Vorma) []wave.WatchedFile {
	var patterns []wave.WatchedFile

	routeDefinitionsPattern := routeDefinitionsWatchPattern(v)
	if routeDefinitionsPattern != nil {
		patterns = append(patterns, *routeDefinitionsPattern)
	}

	htmlTemplatePattern := htmlTemplateWatchPattern(v)
	if htmlTemplatePattern != nil {
		patterns = append(patterns, *htmlTemplatePattern)
	}

	patterns = append(patterns, goFilesWatchPattern())

	return patterns
}

func routeDefinitionsWatchPattern(v *vormaruntime.Vorma) *wave.WatchedFile {
	clientRouteDefsFile := v.Config.ClientRouteDefsFile
	if clientRouteDefsFile == "" {
		return nil
	}

	watchPattern := runOnChangeOnlyWatchPattern(
		clientRouteDefsFile,
		routeDefinitionsOnChangeCallback(v),
		true,
	)
	return &watchPattern
}

func htmlTemplateWatchPattern(v *vormaruntime.Vorma) *wave.WatchedFile {
	htmlTemplateLocation := v.Config.HTMLTemplateLocation
	privateStaticDir := v.Wave.GetPrivateStaticDir()
	if htmlTemplateLocation == "" || privateStaticDir == "" {
		return nil
	}

	templatePath := filepath.Join(privateStaticDir, htmlTemplateLocation)
	watchPattern := runOnChangeOnlyWatchPattern(
		templatePath,
		htmlTemplateOnChangeCallback(v),
		false,
	)
	return &watchPattern
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
	return watchReloadCallback(
		v,
		vormaruntime.Dev_ReloadRoutesPath,
		"route reload endpoint failed, falling back to restart",
		rebuildRoutesOnly,
	)
}

func htmlTemplateOnChangeCallback(v *vormaruntime.Vorma) func(*wave.HookContext) (*wave.RefreshAction, error) {
	return watchReloadCallback(
		v,
		vormaruntime.Dev_ReloadTemplatePath,
		"template reload endpoint failed, falling back to restart",
		nil,
	)
}

func watchReloadCallback(
	v *vormaruntime.Vorma,
	reloadEndpoint string,
	reloadEndpointFailureWarnMessage string,
	preReloadAction func(*vormaruntime.Vorma) error,
) func(*wave.HookContext) (*wave.RefreshAction, error) {
	return func(ctx *wave.HookContext) (*wave.RefreshAction, error) {
		if preReloadAction != nil {
			if err := preReloadAction(v); err != nil {
				return nil, err
			}
		}

		if ctx != nil && ctx.AppStoppedForBatch {
			return nil, nil
		}

		return getReloadActionForEndpointWithFallback(
			v,
			reloadEndpoint,
			reloadEndpointFailureWarnMessage,
		), nil
	}
}

func shouldInjectDefaultWatchPatterns(v *vormaruntime.Vorma) bool {
	if v.Config.IncludeDefaults == nil {
		return true
	}
	return *v.Config.IncludeDefaults
}

func injectGeneratedOutputPathsForDefaultWatchPatterns(
	cfg *wave.ParsedConfig,
	tsGenOutDir string,
) {
	if cfg.FrameworkPublicFileMapOutDir == "" {
		cfg.FrameworkPublicFileMapOutDir = tsGenOutDir
	}

	appendMissingFrameworkIgnoredPatterns(cfg,
		filepath.Join(tsGenOutDir, wave.GeneratedTSFileName),
		filepath.Join(tsGenOutDir, wave.PublicFileMapTSName),
		filepath.Join(tsGenOutDir, wave.PublicFileMapJSONName),
	)
}

func appendMissingFrameworkIgnoredPatterns(
	cfg *wave.ParsedConfig,
	defaultIgnoredPatterns ...string,
) {
	for _, defaultIgnoredPattern := range defaultIgnoredPatterns {
		if hasString(cfg.FrameworkIgnoredPatterns, defaultIgnoredPattern) {
			continue
		}
		cfg.FrameworkIgnoredPatterns = append(cfg.FrameworkIgnoredPatterns, defaultIgnoredPattern)
	}
}

func hasString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func runOnChangeOnlyWatchPattern(
	pattern string,
	callback func(*wave.HookContext) (*wave.RefreshAction, error),
	skipRebuildingNotification bool,
) wave.WatchedFile {
	return wave.WatchedFile{
		Pattern:         pattern,
		RunOnChangeOnly: true,
		OnChangeHooks: []wave.OnChangeHook{{
			Callback: callback,
		}},
		SkipRebuildingNotification: skipRebuildingNotification,
	}
}
