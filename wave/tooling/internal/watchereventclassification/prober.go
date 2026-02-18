package watchereventclassification

import "os"

// PathProbeSnapshot memoizes all probes for a single path within one batch.
type PathProbeSnapshot struct {
	// HasConfigFileProbe reports whether config-match probe has run.
	HasConfigFileProbe bool
	// IsConfigFile stores the cached config-match probe result.
	IsConfigFile bool
	// HasDirectoryProbe reports whether directory probe has run.
	HasDirectoryProbe bool
	// DirectoryProbeState stores the cached directory probe result.
	DirectoryProbeState DirectoryProbeResult
}

// EventClassificationProber caches config and directory probes so one watcher
// batch does not repeatedly probe the same path.
type EventClassificationProber struct {
	// PathProbeSnapshotByPath memoizes path probe state for one batch.
	PathProbeSnapshotByPath map[string]*PathProbeSnapshot
	// IsConfigFileFn probes whether one path is the active config file.
	IsConfigFileFn func(string) bool
	// StatPathFn probes filesystem metadata for one path.
	StatPathFn func(string) (os.FileInfo, error)
}

// NewEventClassificationProber builds a batch-scoped probe cache.
func NewEventClassificationProber(
	isConfigFileFn func(string) bool,
) *EventClassificationProber {
	return &EventClassificationProber{
		PathProbeSnapshotByPath: make(
			map[string]*PathProbeSnapshot,
		),
		IsConfigFileFn: isConfigFileFn,
		StatPathFn:     os.Stat,
	}
}

// resolvePathProbe returns the shared per-path snapshot used by config and
// directory probes.
func (prober *EventClassificationProber) resolvePathProbe(
	path string,
) *PathProbeSnapshot {
	if prober == nil {
		return nil
	}
	if prober.PathProbeSnapshotByPath == nil {
		prober.PathProbeSnapshotByPath = make(
			map[string]*PathProbeSnapshot,
		)
	}

	pathProbeSnapshot, hasCachedSnapshot := prober.PathProbeSnapshotByPath[path]
	if !hasCachedSnapshot || pathProbeSnapshot == nil {
		pathProbeSnapshot = &PathProbeSnapshot{}
		prober.PathProbeSnapshotByPath[path] = pathProbeSnapshot
	}
	return pathProbeSnapshot
}

// ProbeIsConfigFile memoizes config-path matching for one path.
func (prober *EventClassificationProber) ProbeIsConfigFile(
	path string,
) bool {
	if prober == nil {
		return false
	}
	pathProbeSnapshot := prober.resolvePathProbe(path)
	if pathProbeSnapshot == nil {
		return false
	}
	if pathProbeSnapshot.HasConfigFileProbe {
		return pathProbeSnapshot.IsConfigFile
	}

	resolvedIsConfigFile := false
	if prober.IsConfigFileFn != nil {
		resolvedIsConfigFile = prober.IsConfigFileFn(path)
	}

	pathProbeSnapshot.HasConfigFileProbe = true
	pathProbeSnapshot.IsConfigFile = resolvedIsConfigFile
	return resolvedIsConfigFile
}

// ProbeEventDirectoryStatus memoizes os.Stat-derived directory status for one
// path.
func (prober *EventClassificationProber) ProbeEventDirectoryStatus(
	path string,
) DirectoryProbeResult {
	if prober == nil {
		return DirectoryProbeResult{}
	}
	pathProbeSnapshot := prober.resolvePathProbe(path)
	if pathProbeSnapshot == nil {
		return DirectoryProbeResult{}
	}
	if pathProbeSnapshot.HasDirectoryProbe {
		return pathProbeSnapshot.DirectoryProbeState
	}

	isDirectory := false
	statProbeSucceeded := false
	if prober.StatPathFn != nil {
		pathInfo, pathStatError := prober.StatPathFn(path)
		statProbeSucceeded = pathStatError == nil && pathInfo != nil
		isDirectory = statProbeSucceeded && pathInfo.IsDir()
	}

	directoryProbeResult := DirectoryProbeResult{
		StatProbeSucceeded: statProbeSucceeded,
		IsDirectory:        isDirectory,
	}
	pathProbeSnapshot.HasDirectoryProbe = true
	pathProbeSnapshot.DirectoryProbeState = directoryProbeResult
	return directoryProbeResult
}
