package tooling

import "os"

// watcherEventDirectoryProbeResult captures directory probe outcomes for one
// watcher event path.
type watcherEventDirectoryProbeResult struct {
	statProbeSucceeded bool
	isDirectory        bool
}

// watcherEventPathProbeSnapshot memoizes all probes for a single path within
// one event batch.
type watcherEventPathProbeSnapshot struct {
	hasConfigFileProbe  bool
	isConfigFile        bool
	hasDirectoryProbe   bool
	directoryProbeState watcherEventDirectoryProbeResult
}

// watcherEventClassificationProber caches path probes so one watcher batch does
// not repeatedly hit config matching or filesystem stats for the same path.
type watcherEventClassificationProber struct {
	pathProbeSnapshotByPath map[string]*watcherEventPathProbeSnapshot
	isConfigFileFn          func(string) bool
	statPathFn              func(string) (os.FileInfo, error)
}

// newWatcherEventClassificationProber builds a batch-scoped prober.
func newWatcherEventClassificationProber(
	isConfigFileFn func(string) bool,
) *watcherEventClassificationProber {
	return &watcherEventClassificationProber{
		pathProbeSnapshotByPath: make(
			map[string]*watcherEventPathProbeSnapshot,
		),
		isConfigFileFn: isConfigFileFn,
		statPathFn:     os.Stat,
	}
}

// resolvePathProbe returns the shared per-path snapshot used by both config and
// directory probes.
func (prober *watcherEventClassificationProber) resolvePathProbe(
	path string,
) *watcherEventPathProbeSnapshot {
	if prober == nil {
		return nil
	}
	if prober.pathProbeSnapshotByPath == nil {
		prober.pathProbeSnapshotByPath = make(
			map[string]*watcherEventPathProbeSnapshot,
		)
	}

	pathProbeSnapshot, hasCachedSnapshot := prober.pathProbeSnapshotByPath[path]
	if !hasCachedSnapshot || pathProbeSnapshot == nil {
		pathProbeSnapshot = &watcherEventPathProbeSnapshot{}
		prober.pathProbeSnapshotByPath[path] = pathProbeSnapshot
	}
	return pathProbeSnapshot
}

// probeIsConfigFile memoizes config-path matching for one path.
func (prober *watcherEventClassificationProber) probeIsConfigFile(
	path string,
) bool {
	if prober == nil {
		return false
	}
	pathProbeSnapshot := prober.resolvePathProbe(path)
	if pathProbeSnapshot == nil {
		return false
	}
	if pathProbeSnapshot.hasConfigFileProbe {
		return pathProbeSnapshot.isConfigFile
	}

	resolvedIsConfigFile := false
	if prober.isConfigFileFn != nil {
		resolvedIsConfigFile = prober.isConfigFileFn(path)
	}

	pathProbeSnapshot.hasConfigFileProbe = true
	pathProbeSnapshot.isConfigFile = resolvedIsConfigFile
	return resolvedIsConfigFile
}

// probeEventDirectoryStatus memoizes os.Stat-derived directory status for one
// path.
func (prober *watcherEventClassificationProber) probeEventDirectoryStatus(
	path string,
) watcherEventDirectoryProbeResult {
	if prober == nil {
		return watcherEventDirectoryProbeResult{}
	}
	pathProbeSnapshot := prober.resolvePathProbe(path)
	if pathProbeSnapshot == nil {
		return watcherEventDirectoryProbeResult{}
	}
	if pathProbeSnapshot.hasDirectoryProbe {
		return pathProbeSnapshot.directoryProbeState
	}

	isDirectory := false
	statProbeSucceeded := false
	if prober.statPathFn != nil {
		pathInfo, pathStatError := prober.statPathFn(path)
		statProbeSucceeded = pathStatError == nil && pathInfo != nil
		isDirectory = statProbeSucceeded && pathInfo.IsDir()
	}

	directoryProbeResult := watcherEventDirectoryProbeResult{
		statProbeSucceeded: statProbeSucceeded,
		isDirectory:        isDirectory,
	}
	pathProbeSnapshot.hasDirectoryProbe = true
	pathProbeSnapshot.directoryProbeState = directoryProbeResult
	return directoryProbeResult
}
