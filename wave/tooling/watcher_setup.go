package tooling

import (
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/wave"
)

func (w *watcher) setupPatterns() {
	w.ignoredFiles = []string{
		w.norm(w.cfg.Dist.Binary()),
	}

	// Add dist static as absolute path
	w.ignoredDirs = append(w.ignoredDirs, w.norm(w.cfg.Dist.Static()))
	w.ignoredDirs = append(w.ignoredDirs, w.norm(w.cfg.Dist.Static())+"/**")

	// Only add static asset patterns if not in server-only mode
	if w.cfg.UsingBrowser() {
		publicStatic := filepath.Clean(w.cfg.Core.StaticAssetDirs.Public)
		privateStatic := filepath.Clean(w.cfg.Core.StaticAssetDirs.Private)

		nohashDir := w.norm(filepath.Join(publicStatic, wave.NohashDirname))
		w.ignoredDirs = append(w.ignoredDirs, nohashDir)
		w.ignoredDirs = append(w.ignoredDirs, nohashDir+"/**")

		prehashedDir := w.norm(filepath.Join(publicStatic, wave.PrehashedDirname))
		w.ignoredDirs = append(w.ignoredDirs, prehashedDir)
		w.ignoredDirs = append(w.ignoredDirs, prehashedDir+"/**")

		// Public static files: Wave handles processing and writes filemap.ts directly.
		// No explicit dev build hook command is needed - Vite HMR picks up the TS file change.
		w.defaultWatched = []wave.WatchedFile{
			{
				Pattern: w.norm(publicStatic) + "/**/*",
			},
		}

		// Private static files: Wave handles processing and triggers browser reload.
		w.defaultWatched = append(w.defaultWatched, wave.WatchedFile{
			Pattern: w.norm(privateStatic) + "/**/*",
		})
	}

	// Add framework-injected watch patterns
	w.addFrameworkWatchPatterns()

	// Add framework-injected ignored patterns
	for _, p := range w.cfg.FrameworkIgnoredPatterns {
		normalizedPattern := w.normalizePathOrPatternFromWatchRoot(p)

		// Heuristic: if it ends in /** or looks like a dir, treat as ignored dir
		if strings.HasSuffix(p, "/**") {
			w.ignoredDirs = append(w.ignoredDirs, normalizedPattern)
		} else {
			// It might be a file or a pattern
			w.ignoredFiles = append(w.ignoredFiles, normalizedPattern)
		}
	}

	// For ** patterns, we need to anchor them to watch root
	w.ignoredDirs = append(w.ignoredDirs, w.absWatchRoot+"/"+globGit)
	w.ignoredDirs = append(w.ignoredDirs, w.absWatchRoot+"/"+globNodeModules)

	if w.cfg.Watch != nil {
		for _, p := range w.cfg.Watch.Exclude.Dirs {
			normalizedDirectoryPattern := w.normalizePathOrPatternFromWatchRoot(p)
			w.ignoredDirs = append(w.ignoredDirs, normalizedDirectoryPattern)
			w.ignoredDirs = append(w.ignoredDirs, normalizedDirectoryPattern+"/**")
		}
		for _, p := range w.cfg.Watch.Exclude.Files {
			w.ignoredFiles = append(w.ignoredFiles, w.normalizePathOrPatternFromWatchRoot(p))
		}
	}

	w.joinPatternsWithRoot()
	w.preSortHooks()
}

// addFrameworkWatchPatterns adds patterns injected by frameworks (e.g., Vorma)
func (w *watcher) addFrameworkWatchPatterns() {
	for _, wf := range w.cfg.FrameworkWatchPatterns {
		// Create a copy with normalized pattern
		normalizedWatchedFile := wf
		normalizedWatchedFile.Pattern = w.normalizePathOrPatternFromWatchRoot(wf.Pattern)

		// Normalize exclude patterns in hooks
		for i, hook := range normalizedWatchedFile.OnChangeHooks {
			for j, excludePattern := range hook.Exclude {
				normalizedWatchedFile.OnChangeHooks[i].Exclude[j] = w.normalizePathOrPatternFromWatchRoot(excludePattern)
			}
		}

		w.defaultWatched = append(w.defaultWatched, normalizedWatchedFile)
	}
}

func (w *watcher) joinPatternsWithRoot() {
	if w.cfg.Watch == nil {
		return
	}

	for i, watchedFile := range w.cfg.Watch.Include {
		w.cfg.Watch.Include[i].Pattern = w.normalizePathOrPatternFromWatchRoot(watchedFile.Pattern)

		for j, hook := range watchedFile.OnChangeHooks {
			for k, excludePattern := range hook.Exclude {
				w.cfg.Watch.Include[i].OnChangeHooks[j].Exclude[k] = w.normalizePathOrPatternFromWatchRoot(excludePattern)
			}
		}
	}
}

// preSortHooks pre-sorts hooks for all watched files to avoid repeated sorting during event handling
func (w *watcher) preSortHooks() {
	if w.cfg.Watch != nil {
		for i := range w.cfg.Watch.Include {
			w.cfg.Watch.Include[i].Sort()
		}
	}

	for i := range w.defaultWatched {
		w.defaultWatched[i].Sort()
	}
}
