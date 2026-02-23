package vormabuild

import (
	"fmt"
	"path/filepath"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/wave"
)

const (
	reloadTriggerRouteDefinitionsWatch = "route-definitions-watch"
	reloadTriggerHTMLTemplateWatch     = "html-template-watch"
)

type reloadActionResolver func(
	v *vormaruntime.Vorma,
	reloadEndpoint string,
	warnMessage string,
	reloadTrigger string,
	hookContext *wave.HookContext,
) *wave.RefreshAction

func injectDefaultWatchPatterns(v *vormaruntime.Vorma) *wave.ParsedConfig {
	cfg := v.Wave.BuildtimeParsedConfig()
	injectDefaultWatchPatternsInConfig(cfg, v)
	return cfg
}

func injectDefaultWatchPatternsInConfig(
	cfg *wave.ParsedConfig,
	v *vormaruntime.Vorma,
) {
	if !shouldInjectDefaultWatchPatterns(v) {
		return
	}
	if cfg == nil {
		return
	}

	patterns := getDefaultWatchPatterns(v)
	appendMissingFrameworkWatchPatterns(cfg, patterns)

	if v.Config.TSGenOutDir != "" {
		injectGeneratedOutputPathsForDefaultWatchPatterns(
			cfg,
			v.Config.TSGenOutDir,
		)
	}
}

func appendMissingFrameworkWatchPatterns(
	cfg *wave.ParsedConfig,
	defaultPatterns []wave.WatchedFile,
) {
	for _, defaultPattern := range defaultPatterns {
		if hasFrameworkWatchPattern(
			cfg.FrameworkWatchPatterns,
			defaultPattern.Pattern,
		) {
			continue
		}
		cfg.FrameworkWatchPatterns = append(
			cfg.FrameworkWatchPatterns,
			defaultPattern,
		)
	}
}

func hasFrameworkWatchPattern(
	existingPatterns []wave.WatchedFile,
	pattern string,
) bool {
	normalizedExpectedPattern := normalizeFrameworkWatchPatternPath(pattern)
	for _, existingPattern := range existingPatterns {
		if existingPattern.Pattern == pattern {
			return true
		}
		if normalizeFrameworkWatchPatternPath(existingPattern.Pattern) ==
			normalizedExpectedPattern {
			return true
		}
	}
	return false
}

func getDefaultWatchPatterns(v *vormaruntime.Vorma) []wave.WatchedFile {
	var patterns []wave.WatchedFile

	patterns = append(patterns, routeDefinitionWatchPatterns(v)...)

	htmlTemplatePattern := htmlTemplateWatchPattern(v)
	if htmlTemplatePattern != nil {
		patterns = append(patterns, *htmlTemplatePattern)
	}

	patterns = append(patterns, goFilesWatchPattern())

	return patterns
}

func routeDefinitionWatchPatterns(v *vormaruntime.Vorma) []wave.WatchedFile {
	normalizedRouteDefinitionPatterns, err := normalizeRouteDefinitionPatternsInInputOrder(
		v.Config.ClientRouteDefinitionPatterns,
	)
	if err != nil {
		panic(
			fmt.Sprintf("normalize client route definition patterns: %v", err),
		)
	}
	if len(normalizedRouteDefinitionPatterns) == 0 {
		return nil
	}

	onChangeCallback := routeDefinitionsOnChangeCallback(v)
	watchPatterns := make(
		[]wave.WatchedFile,
		0,
		len(normalizedRouteDefinitionPatterns),
	)
	for _, routeDefinitionPattern := range normalizedRouteDefinitionPatterns {
		normalizedWatchPattern := normalizeFrameworkWatchPatternPath(
			routeDefinitionPattern,
		)
		watchPatterns = append(
			watchPatterns,
			runOnChangeOnlyWatchPattern(
				normalizedWatchPattern,
				onChangeCallback,
				true,
			),
		)
	}
	return watchPatterns
}

func htmlTemplateWatchPattern(v *vormaruntime.Vorma) *wave.WatchedFile {
	htmlTemplateLocation := v.Config.HTMLTemplateLocation
	privateStaticDir := v.Wave.PrivateStaticDir()
	if htmlTemplateLocation == "" || privateStaticDir == "" {
		return nil
	}

	templatePath := normalizeFrameworkWatchPatternPath(
		filepath.Join(privateStaticDir, htmlTemplateLocation),
	)
	watchPattern := wave.WatchedFile{
		Pattern: templatePath,
		OnChangeHooks: []wave.OnChangeHook{{
			Timing:   wave.OnChangeStrategyPost,
			Callback: htmlTemplateOnChangeCallback(v),
		}},
	}
	return &watchPattern
}

func normalizeFrameworkWatchPatternPath(
	pathPattern string,
) string {
	if pathPattern == "" {
		return ""
	}

	cleanedPathPattern := filepath.Clean(pathPattern)
	if filepath.IsAbs(cleanedPathPattern) {
		return cleanedPathPattern
	}

	absolutePathPattern, absolutePathError := filepath.Abs(cleanedPathPattern)
	if absolutePathError != nil {
		return cleanedPathPattern
	}

	return filepath.Clean(absolutePathPattern)
}

func goFilesWatchPattern() wave.WatchedFile {
	return wave.WatchedFile{
		Pattern: "**/*.go",
		OnChangeHooks: []wave.OnChangeHook{{
			RunCombinedDevBuildHookCommands: true,
			Timing:                          wave.OnChangeStrategyConcurrent,
		}},
	}
}

func routeDefinitionsOnChangeCallback(
	v *vormaruntime.Vorma,
) func(*wave.HookContext) (*wave.RefreshAction, error) {
	return routeDefinitionsOnChangeCallbackWithReloadActionResolver(
		v,
		getReloadActionForEndpointWithFallback,
	)
}

func routeDefinitionsOnChangeCallbackWithReloadActionResolver(
	v *vormaruntime.Vorma,
	resolveReloadAction reloadActionResolver,
) func(*wave.HookContext) (*wave.RefreshAction, error) {
	return watchReloadCallback(
		v,
		v.DevReloadRoutesEndpointPath(),
		"route reload endpoint failed, falling back to restart",
		reloadTriggerRouteDefinitionsWatch,
		rebuildRoutesOnly,
		resolveReloadAction,
	)
}

func htmlTemplateOnChangeCallback(
	v *vormaruntime.Vorma,
) func(*wave.HookContext) (*wave.RefreshAction, error) {
	return htmlTemplateOnChangeCallbackWithReloadActionResolver(
		v,
		getReloadActionForEndpointWithFallback,
	)
}

func htmlTemplateOnChangeCallbackWithReloadActionResolver(
	v *vormaruntime.Vorma,
	resolveReloadAction reloadActionResolver,
) func(*wave.HookContext) (*wave.RefreshAction, error) {
	return watchReloadCallback(
		v,
		v.DevReloadTemplateEndpointPath(),
		"template reload endpoint failed, falling back to restart",
		reloadTriggerHTMLTemplateWatch,
		nil,
		resolveReloadAction,
	)
}

func watchReloadCallback(
	v *vormaruntime.Vorma,
	reloadEndpoint string,
	reloadEndpointFailureWarnMessage string,
	reloadTrigger string,
	preReloadAction func(*vormaruntime.Vorma) error,
	resolveReloadAction reloadActionResolver,
) func(*wave.HookContext) (*wave.RefreshAction, error) {
	if resolveReloadAction == nil {
		resolveReloadAction = getReloadActionForEndpointWithFallback
	}

	return func(ctx *wave.HookContext) (*wave.RefreshAction, error) {
		if preReloadAction != nil {
			if err := preReloadAction(v); err != nil {
				return nil, fmt.Errorf(
					"run pre-reload action for trigger %q: %w",
					reloadTrigger,
					err,
				)
			}
		}

		if ctx != nil && ctx.AppStoppedForBatch {
			if v.Log != nil {
				v.Log.Debug(
					"watch reload callback skipped",
					"reload_endpoint",
					reloadEndpoint,
					"reload_trigger",
					reloadTrigger,
					"skip_reason",
					"app-stopped-for-batch",
				)
			}
			return nil, nil
		}

		return resolveReloadAction(
			v,
			reloadEndpoint,
			reloadEndpointFailureWarnMessage,
			reloadTrigger,
			ctx,
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
		cfg.FrameworkIgnoredPatterns = append(
			cfg.FrameworkIgnoredPatterns,
			defaultIgnoredPattern,
		)
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
