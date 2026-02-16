package tooling

import (
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/wave"
)

type watcherPlan struct {
	ignoredFiles     []string
	ignoredDirs      []string
	defaultWatched   []wave.WatchedFile
	configuredWatched []wave.WatchedFile
}

func (w *watcher) buildWatcherPlan() watcherPlan {
	watcherPlanForSetup := watcherPlan{
		ignoredFiles: []string{
			w.norm(w.cfg.Dist.Binary()),
		},
		ignoredDirs:      make([]string, 0),
		defaultWatched:   make([]wave.WatchedFile, 0),
		configuredWatched: make([]wave.WatchedFile, 0),
	}

	// Add dist static as absolute path.
	watcherPlanForSetup.ignoredDirs = append(
		watcherPlanForSetup.ignoredDirs,
		w.norm(w.cfg.Dist.Static()),
	)
	watcherPlanForSetup.ignoredDirs = append(
		watcherPlanForSetup.ignoredDirs,
		w.norm(w.cfg.Dist.Static())+"/**",
	)

	// Only add static asset patterns if not in server-only mode.
	if w.cfg.UsingBrowser() {
		publicStatic := filepath.Clean(w.cfg.Core.StaticAssetDirs.Public)
		privateStatic := filepath.Clean(w.cfg.Core.StaticAssetDirs.Private)

		nohashDir := w.norm(filepath.Join(publicStatic, wave.NohashDirname))
		watcherPlanForSetup.ignoredDirs = append(
			watcherPlanForSetup.ignoredDirs,
			nohashDir,
			nohashDir+"/**",
		)

		prehashedDir := w.norm(filepath.Join(publicStatic, wave.PrehashedDirname))
		watcherPlanForSetup.ignoredDirs = append(
			watcherPlanForSetup.ignoredDirs,
			prehashedDir,
			prehashedDir+"/**",
		)

		// Public static files: Wave handles processing and writes filemap.ts directly.
		// No explicit dev build hook command is needed - Vite HMR picks up the TS file change.
		watcherPlanForSetup.defaultWatched = append(
			watcherPlanForSetup.defaultWatched,
			wave.WatchedFile{
				Pattern: w.norm(publicStatic) + "/**/*",
			},
		)

		// Private static files: Wave handles processing and triggers browser reload.
		watcherPlanForSetup.defaultWatched = append(
			watcherPlanForSetup.defaultWatched,
			wave.WatchedFile{
				Pattern: w.norm(privateStatic) + "/**/*",
			},
		)
	}

	// Add framework-injected watch patterns.
	for _, watchedFile := range w.cfg.FrameworkWatchPatterns {
		watcherPlanForSetup.defaultWatched = append(
			watcherPlanForSetup.defaultWatched,
			w.normalizeWatchedFileForWatcherPlan(watchedFile),
		)
	}

	// Add framework-injected ignored patterns.
	for _, ignoredPattern := range w.cfg.FrameworkIgnoredPatterns {
		normalizedPattern := w.normalizePathOrPatternFromWatchRoot(ignoredPattern)

		// Heuristic: if it ends in /** or looks like a dir, treat as ignored dir.
		if strings.HasSuffix(ignoredPattern, "/**") {
			watcherPlanForSetup.ignoredDirs = append(
				watcherPlanForSetup.ignoredDirs,
				normalizedPattern,
			)
		} else {
			// It might be a file or a pattern.
			watcherPlanForSetup.ignoredFiles = append(
				watcherPlanForSetup.ignoredFiles,
				normalizedPattern,
			)
		}
	}

	// For ** patterns, we need to anchor them to watch root.
	watcherPlanForSetup.ignoredDirs = append(
		watcherPlanForSetup.ignoredDirs,
		w.absWatchRoot+"/"+globGit,
		w.absWatchRoot+"/"+globNodeModules,
	)

	if w.cfg.Watch != nil {
		for _, watchedFile := range w.cfg.Watch.Include {
			watcherPlanForSetup.configuredWatched = append(
				watcherPlanForSetup.configuredWatched,
				w.normalizeWatchedFileForWatcherPlan(watchedFile),
			)
		}

		for _, excludedDirectoryPattern := range w.cfg.Watch.Exclude.Dirs {
			normalizedDirectoryPattern := w.normalizePathOrPatternFromWatchRoot(
				excludedDirectoryPattern,
			)
			watcherPlanForSetup.ignoredDirs = append(
				watcherPlanForSetup.ignoredDirs,
				normalizedDirectoryPattern,
				normalizedDirectoryPattern+"/**",
			)
		}
		for _, excludedFilePattern := range w.cfg.Watch.Exclude.Files {
			watcherPlanForSetup.ignoredFiles = append(
				watcherPlanForSetup.ignoredFiles,
				w.normalizePathOrPatternFromWatchRoot(excludedFilePattern),
			)
		}
	}

	return watcherPlanForSetup
}

func (w *watcher) applyWatcherPlan(
	watcherPlanForSetup watcherPlan,
) {
	w.ignoredFiles = watcherPlanForSetup.ignoredFiles
	w.ignoredDirs = watcherPlanForSetup.ignoredDirs
	w.defaultWatched = watcherPlanForSetup.defaultWatched
	w.configuredWatched = watcherPlanForSetup.configuredWatched
}

func (w *watcher) normalizeWatchedFileForWatcherPlan(
	watchedFile wave.WatchedFile,
) wave.WatchedFile {
	normalizedWatchedFile := watchedFile
	normalizedWatchedFile.Pattern = w.normalizePathOrPatternFromWatchRoot(
		watchedFile.Pattern,
	)
	normalizedWatchedFile.OnChangeHooks = cloneOnChangeHooksForWatcherPlan(
		watchedFile.OnChangeHooks,
		w.normalizePathOrPatternFromWatchRoot,
	)
	normalizedWatchedFile.SortedHooks = deriveSortedHooksForWatcherPlan(
		normalizedWatchedFile.OnChangeHooks,
	)
	return normalizedWatchedFile
}

func cloneOnChangeHooksForWatcherPlan(
	onChangeHooks []wave.OnChangeHook,
	normalizePathOrPatternForExclude func(string) string,
) []wave.OnChangeHook {
	if len(onChangeHooks) == 0 {
		return nil
	}

	clonedOnChangeHooks := make([]wave.OnChangeHook, 0, len(onChangeHooks))
	for _, onChangeHook := range onChangeHooks {
		clonedOnChangeHook := onChangeHook
		clonedOnChangeHook.Exclude = append([]string(nil), onChangeHook.Exclude...)
		if normalizePathOrPatternForExclude != nil {
			for excludeIndex, excludePattern := range clonedOnChangeHook.Exclude {
				clonedOnChangeHook.Exclude[excludeIndex] = normalizePathOrPatternForExclude(
					excludePattern,
				)
			}
		}
		clonedOnChangeHooks = append(clonedOnChangeHooks, clonedOnChangeHook)
	}

	return clonedOnChangeHooks
}

func deriveSortedHooksForWatcherPlan(
	onChangeHooks []wave.OnChangeHook,
) *wave.SortedHooks {
	if len(onChangeHooks) == 0 {
		return &wave.SortedHooks{}
	}

	watchedFileForSorting := wave.WatchedFile{
		OnChangeHooks: cloneOnChangeHooksForWatcherPlan(onChangeHooks, nil),
	}
	watchedFileForSorting.Sort()
	return cloneSortedHooksForWatcherPlan(watchedFileForSorting.SortedHooks)
}

func cloneSortedHooksForWatcherPlan(
	sortedHooks *wave.SortedHooks,
) *wave.SortedHooks {
	if sortedHooks == nil {
		return &wave.SortedHooks{}
	}

	return &wave.SortedHooks{
		Pre: cloneOnChangeHooksForWatcherPlan(sortedHooks.Pre, nil),
		Concurrent: cloneOnChangeHooksForWatcherPlan(
			sortedHooks.Concurrent,
			nil,
		),
		ConcurrentNoWait: cloneOnChangeHooksForWatcherPlan(
			sortedHooks.ConcurrentNoWait,
			nil,
		),
		Post: cloneOnChangeHooksForWatcherPlan(sortedHooks.Post, nil),
	}
}

