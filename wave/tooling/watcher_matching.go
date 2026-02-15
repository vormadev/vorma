package tooling

import (
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/vormadev/vorma/wave"
)

// MatchPattern checks if a path matches a glob pattern.
// Both pattern and path should already be normalized (absolute + forward slashes).
func (w *watcher) MatchPattern(pattern, path string) bool {
	key := pattern + "\x00" + path

	if cached, found := w.matchCache.Get(key); found {
		return cached
	}

	matches, err := doublestar.Match(pattern, path)
	if err != nil {
		w.log.Error("Pattern match error", "pattern", pattern, "path", path, "error", err)
		return false
	}

	w.matchCache.Set(key, matches, false)
	return matches
}

// IsIgnored checks if a path matches any of the ignored patterns.
// Normalizes the path before matching.
func (w *watcher) IsIgnored(path string, patterns []string) bool {
	normalizedPath := w.norm(path)
	for _, pattern := range patterns {
		if w.MatchPattern(pattern, normalizedPath) {
			return true
		}
	}
	return false
}

// IsIgnoredFile checks if a file path should be ignored
func (w *watcher) IsIgnoredFile(path string) bool {
	return w.IsIgnored(path, w.ignoredFiles)
}

// IsIgnoredDir checks if a directory path should be ignored
func (w *watcher) IsIgnoredDir(path string) bool {
	return w.IsIgnored(path, w.ignoredDirs)
}

// FindWatchedFile finds and merges all matching WatchedFile configs for a path.
// Framework patterns are matched first, then user patterns. Settings are merged
// with "strongest wins" semantics: destructive flags use OR, suppressive flags use AND,
// and hooks are concatenated (framework first, then user).
func (w *watcher) FindWatchedFile(path string) *wave.WatchedFile {
	normalizedPath := w.norm(path)

	var matches []*wave.WatchedFile

	// Collect framework/default matches first (these run first in hook order)
	for i := range w.defaultWatched {
		watchedFile := &w.defaultWatched[i]
		if w.MatchPattern(watchedFile.Pattern, normalizedPath) {
			matches = append(matches, watchedFile)
		}
	}

	// Collect user-defined matches second (these run after framework hooks)
	if w.cfg.Watch != nil {
		for i := range w.cfg.Watch.Include {
			watchedFile := &w.cfg.Watch.Include[i]
			if w.MatchPattern(watchedFile.Pattern, normalizedPath) {
				matches = append(matches, watchedFile)
			}
		}
	}

	if len(matches) == 0 {
		return nil
	}

	if len(matches) == 1 {
		return matches[0]
	}

	return mergeWatchedFiles(matches)
}

// mergeWatchedFiles merges multiple WatchedFile configs into one.
//
// The principle is simple: union of all work. If ANY matching config requests
// an action, we do it. This prevents user config from accidentally disabling
// framework-critical behavior.
//
// For each possible action, we ask: "would ANY config cause this to happen?"
//
//   - Compile Go binary: yes if RecompileGoBinary=true OR (it's a .go file AND TreatAsNonGo=false)
//   - Restart app: yes if RestartApp=true
//   - Run standard build: yes if RunOnChangeOnly=false
//   - Show notification: yes if SkipRebuildingNotification=false
//   - Revalidate only (vs full reload): yes if OnlyRunClientDefinedRevalidateFunc=true (trump flag)
//
// Hooks are concatenated: framework hooks first, then user hooks.
func mergeWatchedFiles(matches []*wave.WatchedFile) *wave.WatchedFile {
	if len(matches) == 0 {
		return nil
	}

	// Start with "no work" defaults
	merged := &wave.WatchedFile{
		Pattern: matches[0].Pattern,

		// These mean "do work" when true - start false, any true wins
		RecompileGoBinary: false,
		RestartApp:        false,

		// These mean "skip work" when true - start true, any false wins
		TreatAsNonGo:               true,
		RunOnChangeOnly:            true,
		SkipRebuildingNotification: true,

		// Trump flag: user explicitly overriding browser behavior - any true wins
		OnlyRunClientDefinedRevalidateFunc: false,
	}

	var allHooks []wave.OnChangeHook

	for _, watchedFile := range matches {
		// "Do X" flags: if any config says do it, we do it
		if watchedFile.RecompileGoBinary {
			merged.RecompileGoBinary = true
		}
		if watchedFile.RestartApp {
			merged.RestartApp = true
		}

		// "Skip X" flags: if any config says DON'T skip, we don't skip
		if !watchedFile.TreatAsNonGo {
			merged.TreatAsNonGo = false
		}
		if !watchedFile.RunOnChangeOnly {
			merged.RunOnChangeOnly = false
		}
		if !watchedFile.SkipRebuildingNotification {
			merged.SkipRebuildingNotification = false
		}

		// Trump flag: user explicitly overriding browser behavior
		if watchedFile.OnlyRunClientDefinedRevalidateFunc {
			merged.OnlyRunClientDefinedRevalidateFunc = true
		}

		allHooks = append(allHooks, watchedFile.OnChangeHooks...)
	}

	merged.OnChangeHooks = allHooks
	merged.Sort()

	return merged
}

// IsPublicStaticFile checks if a path is within the public static directory
func (w *watcher) IsPublicStaticFile(path string) bool {
	if w.absPublicStatic == "" {
		return false
	}
	normalizedPath := w.norm(path)
	return strings.HasPrefix(normalizedPath, w.absPublicStatic+"/")
}

// IsPrivateStaticFile checks if a path is within the private static directory
func (w *watcher) IsPrivateStaticFile(path string) bool {
	if w.absPrivateStatic == "" {
		return false
	}
	normalizedPath := w.norm(path)
	return strings.HasPrefix(normalizedPath, w.absPrivateStatic+"/")
}
