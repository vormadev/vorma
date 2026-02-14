package wave

// Clone returns a defensive public snapshot of parsed config values.
// Internal framework-only mutable fields are intentionally omitted.
func (parsedConfig *ParsedConfig) Clone() *ParsedConfig {
	if parsedConfig == nil {
		return nil
	}

	return &ParsedConfig{
		Core:  cloneCoreConfig(parsedConfig.Core),
		Vite:  cloneViteConfig(parsedConfig.Vite),
		Watch: cloneWatchConfig(parsedConfig.Watch),

		Dist: parsedConfig.Dist,

		FrameworkWatchPatterns: cloneFrameworkWatchPatterns(
			parsedConfig.FrameworkWatchPatterns,
		),
		FrameworkIgnoredPatterns: append(
			[]string(nil),
			parsedConfig.FrameworkIgnoredPatterns...,
		),
		FrameworkPublicFileMapOutDir:                  parsedConfig.FrameworkPublicFileMapOutDir,
		FrameworkSchemaExtensions:                     nil,
		FrameworkDevBuildHook:                         parsedConfig.FrameworkDevBuildHook,
		FrameworkProdBuildHook:                        parsedConfig.FrameworkProdBuildHook,
		FrameworkRunBuildHook:                         nil,
		FrameworkPrepareGoBuildOverlay:                nil,
		FrameworkBrowserRuntimeNamespace:              parsedConfig.FrameworkBrowserRuntimeNamespace,
		FrameworkBrowserPublicURLResolverFunctionName: parsedConfig.FrameworkBrowserPublicURLResolverFunctionName,
		FrameworkBrowserRevalidateFunctionName:        parsedConfig.FrameworkBrowserRevalidateFunctionName,
		FrameworkRefreshRebuildingOverlayElementID:    parsedConfig.FrameworkRefreshRebuildingOverlayElementID,
		FrameworkCriticalCSSStyleElementID:            parsedConfig.FrameworkCriticalCSSStyleElementID,
		FrameworkNonCriticalCSSLinkElementID:          parsedConfig.FrameworkNonCriticalCSSLinkElementID,
	}
}

func cloneCoreConfig(
	coreConfig *CoreConfig,
) *CoreConfig {
	if coreConfig == nil {
		return nil
	}

	clonedCoreConfig := *coreConfig
	return &clonedCoreConfig
}

func cloneViteConfig(
	viteConfig *ViteConfig,
) *ViteConfig {
	if viteConfig == nil {
		return nil
	}

	clonedViteConfig := *viteConfig
	return &clonedViteConfig
}

func cloneWatchConfig(
	watchConfig *WatchConfig,
) *WatchConfig {
	if watchConfig == nil {
		return nil
	}

	clonedWatchConfig := *watchConfig
	clonedWatchConfig.Include = cloneFrameworkWatchPatterns(watchConfig.Include)
	clonedWatchConfig.Exclude.Dirs = append([]string(nil), watchConfig.Exclude.Dirs...)
	clonedWatchConfig.Exclude.Files = append([]string(nil), watchConfig.Exclude.Files...)

	return &clonedWatchConfig
}
