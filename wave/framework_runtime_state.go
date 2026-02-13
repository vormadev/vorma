package wave

func (cfg *ParsedConfig) CopyFrameworkRuntimeFieldsFrom(
	previousParsedConfig *ParsedConfig,
) {
	if cfg == nil || previousParsedConfig == nil {
		return
	}

	cfg.FrameworkWatchPatterns = cloneFrameworkWatchPatterns(
		previousParsedConfig.FrameworkWatchPatterns,
	)
	cfg.FrameworkIgnoredPatterns = append(
		[]string(nil),
		previousParsedConfig.FrameworkIgnoredPatterns...,
	)
	cfg.FrameworkPublicFileMapOutDir = previousParsedConfig.FrameworkPublicFileMapOutDir
	cfg.FrameworkDevBuildHook = previousParsedConfig.FrameworkDevBuildHook
	cfg.FrameworkProdBuildHook = previousParsedConfig.FrameworkProdBuildHook
	cfg.FrameworkBrowserRuntimeNamespace = previousParsedConfig.FrameworkBrowserRuntimeNamespace
	cfg.FrameworkBrowserPublicURLResolverFunctionName = previousParsedConfig.FrameworkBrowserPublicURLResolverFunctionName
	cfg.FrameworkBrowserRevalidateFunctionName = previousParsedConfig.FrameworkBrowserRevalidateFunctionName
	cfg.FrameworkRefreshRebuildingOverlayElementID = previousParsedConfig.FrameworkRefreshRebuildingOverlayElementID
	cfg.FrameworkCriticalCSSStyleElementID = previousParsedConfig.FrameworkCriticalCSSStyleElementID
	cfg.FrameworkNonCriticalCSSLinkElementID = previousParsedConfig.FrameworkNonCriticalCSSLinkElementID
}

func cloneFrameworkWatchPatterns(
	watchedFiles []WatchedFile,
) []WatchedFile {
	if len(watchedFiles) == 0 {
		return nil
	}

	clonedWatchedFiles := make([]WatchedFile, 0, len(watchedFiles))
	for _, watchedFile := range watchedFiles {
		clonedWatchedFiles = append(
			clonedWatchedFiles,
			cloneWatchedFileForFrameworkRuntimeState(watchedFile),
		)
	}

	return clonedWatchedFiles
}

func cloneWatchedFileForFrameworkRuntimeState(
	watchedFile WatchedFile,
) WatchedFile {
	clonedWatchedFile := watchedFile
	clonedWatchedFile.OnChangeHooks = cloneOnChangeHooksForFrameworkRuntimeState(
		watchedFile.OnChangeHooks,
	)
	clonedWatchedFile.SortedHooks = cloneSortedHooksForFrameworkRuntimeState(
		watchedFile.SortedHooks,
	)

	return clonedWatchedFile
}

func cloneSortedHooksForFrameworkRuntimeState(
	sortedHooks *SortedHooks,
) *SortedHooks {
	if sortedHooks == nil {
		return nil
	}

	return &SortedHooks{
		Pre: cloneOnChangeHooksForFrameworkRuntimeState(sortedHooks.Pre),
		Concurrent: cloneOnChangeHooksForFrameworkRuntimeState(
			sortedHooks.Concurrent,
		),
		ConcurrentNoWait: cloneOnChangeHooksForFrameworkRuntimeState(
			sortedHooks.ConcurrentNoWait,
		),
		Post: cloneOnChangeHooksForFrameworkRuntimeState(sortedHooks.Post),
	}
}

func cloneOnChangeHooksForFrameworkRuntimeState(
	onChangeHooks []OnChangeHook,
) []OnChangeHook {
	if len(onChangeHooks) == 0 {
		return nil
	}

	clonedOnChangeHooks := make([]OnChangeHook, 0, len(onChangeHooks))
	for _, onChangeHook := range onChangeHooks {
		clonedOnChangeHook := onChangeHook
		clonedOnChangeHook.Exclude = append(
			[]string(nil),
			onChangeHook.Exclude...,
		)
		clonedOnChangeHooks = append(clonedOnChangeHooks, clonedOnChangeHook)
	}

	return clonedOnChangeHooks
}
