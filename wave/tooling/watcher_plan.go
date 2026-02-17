package tooling

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/wave"
)

type watcherPlan struct {
	ignoredFiles      []string
	ignoredDirs       []string
	defaultWatched    []wave.WatchedFile
	configuredWatched []wave.WatchedFile
}

func (w *watcher) buildWatcherPlan() (watcherPlan, error) {
	watcherPlanForSetup := watcherPlan{
		ignoredFiles: []string{
			w.normalizeLiteralPathForPattern(w.cfg.Dist.Binary()),
		},
		ignoredDirs:       make([]string, 0),
		defaultWatched:    make([]wave.WatchedFile, 0),
		configuredWatched: make([]wave.WatchedFile, 0),
	}

	// Add dist static as absolute path.
	normalizedDistStaticPathPattern := w.normalizeLiteralPathForPattern(w.cfg.Dist.Static())
	watcherPlanForSetup.ignoredDirs = append(
		watcherPlanForSetup.ignoredDirs,
		normalizedDistStaticPathPattern,
	)
	watcherPlanForSetup.ignoredDirs = append(
		watcherPlanForSetup.ignoredDirs,
		normalizedDistStaticPathPattern+"/**",
	)

	// Only add static asset patterns if not in server-only mode.
	if w.cfg.UsingBrowser() {
		publicStatic := filepath.Clean(w.cfg.Core.StaticAssetDirs.Public)
		privateStatic := filepath.Clean(w.cfg.Core.StaticAssetDirs.Private)

		nohashDir := w.normalizeLiteralPathForPattern(filepath.Join(publicStatic, wave.NohashDirname))
		watcherPlanForSetup.ignoredDirs = append(
			watcherPlanForSetup.ignoredDirs,
			nohashDir,
			nohashDir+"/**",
		)

		prehashedDir := w.normalizeLiteralPathForPattern(filepath.Join(publicStatic, wave.PrehashedDirname))
		watcherPlanForSetup.ignoredDirs = append(
			watcherPlanForSetup.ignoredDirs,
			prehashedDir,
			prehashedDir+"/**",
		)

		// Public static files: Wave handles processing and writes filemap.ts directly.
		// No explicit dev build hook command is needed - Vite HMR picks up the TS file change.
		publicStaticPatternPrefix := w.normalizeLiteralPathForPattern(publicStatic)
		watcherPlanForSetup.defaultWatched = append(
			watcherPlanForSetup.defaultWatched,
			wave.WatchedFile{
				Pattern: publicStaticPatternPrefix + "/**/*",
			},
		)

		// Private static files: Wave handles processing and triggers browser reload.
		privateStaticPatternPrefix := w.normalizeLiteralPathForPattern(privateStatic)
		watcherPlanForSetup.defaultWatched = append(
			watcherPlanForSetup.defaultWatched,
			wave.WatchedFile{
				Pattern: privateStaticPatternPrefix + "/**/*",
			},
		)
	}

	// Add framework-injected watch patterns.
	for frameworkWatchPatternIndex, watchedFile := range w.cfg.FrameworkWatchPatterns {
		normalizedWatchedFile, normalizeWatchedFileError := w.normalizeWatchedFileForWatcherPlan(
			normalizeWatchedFileForWatcherPlanArgs{
				watchedFile: watchedFile,
				fieldPath: fmt.Sprintf(
					"FrameworkWatchPatterns[%d]",
					frameworkWatchPatternIndex,
				),
			},
		)
		if normalizeWatchedFileError != nil {
			return watcherPlan{}, normalizeWatchedFileError
		}
		watcherPlanForSetup.defaultWatched = append(
			watcherPlanForSetup.defaultWatched,
			normalizedWatchedFile,
		)
	}

	// Add framework-injected ignored patterns.
	for ignoredPatternIndex, ignoredPattern := range w.cfg.FrameworkIgnoredPatterns {
		normalizedPattern := w.normalizePathOrPatternFromWatchRoot(ignoredPattern)
		if validatePatternError := validateWatcherGlobPatternInputForRuntime(
			fmt.Sprintf("FrameworkIgnoredPatterns[%d]", ignoredPatternIndex),
			ignoredPattern,
		); validatePatternError != nil {
			return watcherPlan{}, validatePatternError
		}

		// Framework ignored patterns apply to both file and directory checks.
		watcherPlanForSetup.ignoredFiles = append(
			watcherPlanForSetup.ignoredFiles,
			normalizedPattern,
		)
		if !strings.HasSuffix(ignoredPattern, "/**") {
			watcherPlanForSetup.ignoredFiles = append(
				watcherPlanForSetup.ignoredFiles,
				normalizedPattern+"/**",
			)
		}
		watcherPlanForSetup.ignoredDirs = append(
			watcherPlanForSetup.ignoredDirs,
			normalizedPattern,
		)
		if !strings.HasSuffix(ignoredPattern, "/**") {
			watcherPlanForSetup.ignoredDirs = append(
				watcherPlanForSetup.ignoredDirs,
				normalizedPattern+"/**",
			)
		}
	}

	// For ** patterns, we need to anchor them to watch root.
	normalizedWatchRootPathPattern := w.normalizeLiteralPathForPattern(w.cfg.WatchRoot())
	watcherPlanForSetup.ignoredDirs = append(
		watcherPlanForSetup.ignoredDirs,
		normalizedWatchRootPathPattern+"/"+globGit,
		normalizedWatchRootPathPattern+"/"+globNodeModules,
	)

	if w.cfg.Watch != nil {
		for watchedFileIndex, watchedFile := range w.cfg.Watch.Include {
			normalizedWatchedFile, normalizeWatchedFileError := w.normalizeWatchedFileForWatcherPlan(
				normalizeWatchedFileForWatcherPlanArgs{
					watchedFile: watchedFile,
					fieldPath: fmt.Sprintf(
						"Watch.Include[%d]",
						watchedFileIndex,
					),
				},
			)
			if normalizeWatchedFileError != nil {
				return watcherPlan{}, normalizeWatchedFileError
			}
			watcherPlanForSetup.configuredWatched = append(
				watcherPlanForSetup.configuredWatched,
				normalizedWatchedFile,
			)
		}

		for excludedDirectoryPatternIndex, excludedDirectoryPattern := range w.cfg.Watch.Exclude.Dirs {
			normalizedDirectoryPattern := w.normalizePathOrPatternFromWatchRoot(
				excludedDirectoryPattern,
			)
			if validatePatternError := validateWatcherGlobPatternInputForRuntime(
				fmt.Sprintf("Watch.Exclude.Dirs[%d]", excludedDirectoryPatternIndex),
				excludedDirectoryPattern,
			); validatePatternError != nil {
				return watcherPlan{}, validatePatternError
			}
			watcherPlanForSetup.ignoredDirs = append(
				watcherPlanForSetup.ignoredDirs,
				normalizedDirectoryPattern,
				normalizedDirectoryPattern+"/**",
			)
		}
		for excludedFilePatternIndex, excludedFilePattern := range w.cfg.Watch.Exclude.Files {
			normalizedExcludedFilePattern := w.normalizePathOrPatternFromWatchRoot(excludedFilePattern)
			if validatePatternError := validateWatcherGlobPatternInputForRuntime(
				fmt.Sprintf("Watch.Exclude.Files[%d]", excludedFilePatternIndex),
				excludedFilePattern,
			); validatePatternError != nil {
				return watcherPlan{}, validatePatternError
			}
			watcherPlanForSetup.ignoredFiles = append(
				watcherPlanForSetup.ignoredFiles,
				normalizedExcludedFilePattern,
			)
		}
	}

	return watcherPlanForSetup, nil
}

func (w *watcher) applyWatcherPlan(
	watcherPlanForSetup watcherPlan,
) {
	w.ignoredFiles = watcherPlanForSetup.ignoredFiles
	w.ignoredDirs = watcherPlanForSetup.ignoredDirs
	w.defaultWatched = watcherPlanForSetup.defaultWatched
	w.configuredWatched = watcherPlanForSetup.configuredWatched
}

type normalizeWatchedFileForWatcherPlanArgs struct {
	watchedFile wave.WatchedFile
	fieldPath   string
}

func (w *watcher) normalizeWatchedFileForWatcherPlan(
	args normalizeWatchedFileForWatcherPlanArgs,
) (wave.WatchedFile, error) {
	watchedFile := args.watchedFile
	normalizedWatchedFile := watchedFile
	normalizedWatchedFile.Pattern = w.normalizePathOrPatternFromWatchRoot(
		watchedFile.Pattern,
	)
	if validatePatternError := validateWatcherGlobPatternInputForRuntime(
		args.fieldPath+".Pattern",
		watchedFile.Pattern,
	); validatePatternError != nil {
		return wave.WatchedFile{}, validatePatternError
	}

	normalizedOnChangeHooks, normalizeOnChangeHookExcludesError := cloneOnChangeHooksForWatcherPlan(
		cloneOnChangeHooksForWatcherPlanArgs{
			onChangeHooks:                    watchedFile.OnChangeHooks,
			normalizePathOrPatternForExclude: w.normalizePathOrPatternFromWatchRoot,
			fieldPath:                        args.fieldPath + ".OnChangeHooks",
		},
	)
	if normalizeOnChangeHookExcludesError != nil {
		return wave.WatchedFile{}, normalizeOnChangeHookExcludesError
	}
	normalizedWatchedFile.OnChangeHooks = normalizedOnChangeHooks
	normalizedWatchedFile.SortedHooks = deriveSortedHooksForWatcherPlan(
		normalizedWatchedFile.OnChangeHooks,
	)

	return normalizedWatchedFile, nil
}

type cloneOnChangeHooksForWatcherPlanArgs struct {
	onChangeHooks                    []wave.OnChangeHook
	normalizePathOrPatternForExclude func(string) string
	fieldPath                        string
}

func cloneOnChangeHooksForWatcherPlan(
	args cloneOnChangeHooksForWatcherPlanArgs,
) ([]wave.OnChangeHook, error) {
	onChangeHooks := args.onChangeHooks
	normalizePathOrPatternForExclude := args.normalizePathOrPatternForExclude

	if len(onChangeHooks) == 0 {
		return nil, nil
	}

	clonedOnChangeHooks := make([]wave.OnChangeHook, 0, len(onChangeHooks))
	for hookIndex, onChangeHook := range onChangeHooks {
		clonedOnChangeHook := onChangeHook
		clonedOnChangeHook.Exclude = append([]string(nil), onChangeHook.Exclude...)
		if normalizePathOrPatternForExclude != nil {
			for excludeIndex, excludePattern := range clonedOnChangeHook.Exclude {
				if validatePatternError := validateWatcherGlobPatternInputForRuntime(
					fmt.Sprintf(
						"%s[%d].Exclude[%d]",
						args.fieldPath,
						hookIndex,
						excludeIndex,
					),
					excludePattern,
				); validatePatternError != nil {
					return nil, validatePatternError
				}
				clonedOnChangeHook.Exclude[excludeIndex] = normalizePathOrPatternForExclude(
					excludePattern,
				)
			}
		}
		clonedOnChangeHooks = append(clonedOnChangeHooks, clonedOnChangeHook)
	}

	return clonedOnChangeHooks, nil
}

func deriveSortedHooksForWatcherPlan(
	onChangeHooks []wave.OnChangeHook,
) *wave.SortedHooks {
	if len(onChangeHooks) == 0 {
		return &wave.SortedHooks{}
	}

	watchedFileForSorting := wave.WatchedFile{
		OnChangeHooks: cloneOnChangeHooksForSortingWithoutNormalization(
			onChangeHooks,
		),
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
		Pre: cloneOnChangeHooksForSortingWithoutNormalization(sortedHooks.Pre),
		Concurrent: cloneOnChangeHooksForSortingWithoutNormalization(
			sortedHooks.Concurrent,
		),
		ConcurrentNoWait: cloneOnChangeHooksForSortingWithoutNormalization(
			sortedHooks.ConcurrentNoWait,
		),
		Post: cloneOnChangeHooksForSortingWithoutNormalization(sortedHooks.Post),
	}
}

func cloneOnChangeHooksForSortingWithoutNormalization(
	onChangeHooks []wave.OnChangeHook,
) []wave.OnChangeHook {
	clonedOnChangeHooks, _ := cloneOnChangeHooksForWatcherPlan(
		cloneOnChangeHooksForWatcherPlanArgs{
			onChangeHooks: onChangeHooks,
		},
	)
	return clonedOnChangeHooks
}

func validateWatcherGlobPatternInputForRuntime(
	fieldPath string,
	pattern string,
) error {
	return validateNamedGlobPatternInput("watcher setup", fieldPath, pattern)
}
