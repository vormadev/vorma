// Package runtimecore centralizes mutable runtime state transitions used by
// vormaruntime.
//
// The package keeps lifecycle-state transitions, route artifact metadata
// application, route-path cloning/merging, and route-data cache invalidation
// logic together so that runtime mutation semantics are testable as one unit.
package runtimecore

import (
	"fmt"
	"sort"
	"sync"
)

// RoutePath is the route metadata shape used by runtime route-state mutation
// helpers.
type RoutePath struct {
	OriginalPattern string
	SrcPath         string
	OutPath         string
	ExportKey       string
	ErrorExportKey  string
	Deps            []string
}

// RuntimePathsFileSnapshot is the route artifact snapshot read from disk.
type RuntimePathsFileSnapshot struct {
	BuildID           string
	ClientEntrySrc    string
	ClientEntryOut    string
	ClientEntryDeps   []string
	DepToCSSBundleMap map[string][]string
	RouteManifestFile string
	Paths             map[string]*RoutePath
}

// RuntimeRouteArtifacts are parsed runtime artifacts staged for one commit.
type RuntimeRouteArtifacts struct {
	BuildID           string
	ClientEntrySrc    string
	ClientEntryOut    string
	ClientEntryDeps   []string
	DepToCSSBundleMap map[string][]string
	RouteManifestFile string
	ParsedClientPaths map[string]*RoutePath
}

// RouteArtifactCommitMode controls route-state commit behavior.
type RouteArtifactCommitMode uint8

const (
	// RouteArtifactCommitModeInit is used during initial load or full init
	// replacement.
	RouteArtifactCommitModeInit RouteArtifactCommitMode = iota
	// RouteArtifactCommitModeDevReload is used during dev route reload merges.
	RouteArtifactCommitModeDevReload
)

// BuildRuntimeRouteArtifacts normalizes one file snapshot into staged route
// artifacts.
func BuildRuntimeRouteArtifacts(
	pathsFile *RuntimePathsFileSnapshot,
) (*RuntimeRouteArtifacts, error) {
	if pathsFile == nil {
		return nil, fmt.Errorf("paths file is nil")
	}

	return &RuntimeRouteArtifacts{
		BuildID:           pathsFile.BuildID,
		ClientEntrySrc:    pathsFile.ClientEntrySrc,
		ClientEntryOut:    pathsFile.ClientEntryOut,
		ClientEntryDeps:   pathsFile.ClientEntryDeps,
		DepToCSSBundleMap: pathsFile.DepToCSSBundleMap,
		RouteManifestFile: pathsFile.RouteManifestFile,
		ParsedClientPaths: pathsFile.Paths,
	}, nil
}

// RuntimeRouteMetadataState contains mutable runtime metadata fields that are
// updated from staged route artifacts.
type RuntimeRouteMetadataState struct {
	BuildID           string
	ClientEntrySrc    string
	ClientEntryOut    string
	ClientEntryDeps   []string
	DepToCSSBundleMap map[string][]string
	RouteManifestFile string
}

// ApplyRuntimeRouteArtifactsMetadata applies route artifact metadata into
// mutable runtime metadata state.
func ApplyRuntimeRouteArtifactsMetadata(
	state *RuntimeRouteMetadataState,
	artifacts *RuntimeRouteArtifacts,
) {
	if state == nil {
		panic("runtime route metadata state cannot be nil")
	}

	if artifacts == nil {
		state.BuildID = ""
		state.ClientEntrySrc = ""
		state.ClientEntryOut = ""
		state.ClientEntryDeps = nil
		state.DepToCSSBundleMap = make(map[string][]string)
		state.RouteManifestFile = ""
		return
	}

	state.BuildID = artifacts.BuildID
	state.ClientEntrySrc = artifacts.ClientEntrySrc
	state.ClientEntryOut = artifacts.ClientEntryOut
	state.ClientEntryDeps = CloneStringSliceOrNil(artifacts.ClientEntryDeps)
	state.DepToCSSBundleMap = CloneDepToCSSBundleMapOrEmpty(
		artifacts.DepToCSSBundleMap,
	)
	state.RouteManifestFile = artifacts.RouteManifestFile
}

// RouteCacheState captures mutable route-data cache fields on runtime state.
type RouteCacheState struct {
	RouteDataSnapshotVersion uint64
	RouteDataCache           *sync.Map
}

// InvalidateRouteDataCache increments snapshot version and resets cache map.
func InvalidateRouteDataCache(state *RouteCacheState) {
	if state == nil {
		panic("route cache state cannot be nil")
	}
	state.RouteDataSnapshotVersion++
	state.RouteDataCache = &sync.Map{}
}

// RouteMutableState contains the mutable route-state fields affected by
// route-state transitions.
type RouteMutableState struct {
	Paths                    map[string]*RoutePath
	RouteDataSnapshotVersion uint64
	RouteDataCache           *sync.Map
}

// SyncRouteStateFromDevReloadInput defines inputs for dev-reload route-state
// sync.
type SyncRouteStateFromDevReloadInput struct {
	State                               *RouteMutableState
	ParsedClientPaths                   map[string]*RoutePath
	ServerRoutePatternsWithTaskHandlers []string
	RebuildNestedRouterFromCurrentPaths func(
		paths map[string]*RoutePath,
	)
}

// ReplaceRouteStateForInitInput defines inputs for init/re-init route-state
// replacement.
type ReplaceRouteStateForInitInput struct {
	State                               *RouteMutableState
	ParsedClientPaths                   map[string]*RoutePath
	RebuildNestedRouter                 bool
	RebuildNestedRouterFromCurrentPaths func(
		paths map[string]*RoutePath,
	)
}

// SyncRouteStateFromDevReload updates mutable route state for dev-reload
// flows.
func SyncRouteStateFromDevReload(input SyncRouteStateFromDevReloadInput) {
	if input.State == nil {
		panic("route state cannot be nil")
	}

	input.State.Paths = SyncPathsFromDevReload(
		input.ParsedClientPaths,
		input.ServerRoutePatternsWithTaskHandlers,
	)
	cacheState := RouteCacheState{
		RouteDataSnapshotVersion: input.State.RouteDataSnapshotVersion,
		RouteDataCache:           input.State.RouteDataCache,
	}
	InvalidateRouteDataCache(&cacheState)
	input.State.RouteDataSnapshotVersion = cacheState.RouteDataSnapshotVersion
	input.State.RouteDataCache = cacheState.RouteDataCache

	if input.RebuildNestedRouterFromCurrentPaths != nil {
		input.RebuildNestedRouterFromCurrentPaths(input.State.Paths)
	}
}

// ReplaceRouteStateForInit updates mutable route state for init/re-init flows.
func ReplaceRouteStateForInit(input ReplaceRouteStateForInitInput) {
	if input.State == nil {
		panic("route state cannot be nil")
	}

	input.State.Paths = ReplaceParsedPathsForInit(
		input.ParsedClientPaths,
	)
	cacheState := RouteCacheState{
		RouteDataSnapshotVersion: input.State.RouteDataSnapshotVersion,
		RouteDataCache:           input.State.RouteDataCache,
	}
	InvalidateRouteDataCache(&cacheState)
	input.State.RouteDataSnapshotVersion = cacheState.RouteDataSnapshotVersion
	input.State.RouteDataCache = cacheState.RouteDataCache

	if input.RebuildNestedRouter &&
		input.RebuildNestedRouterFromCurrentPaths != nil {
		input.RebuildNestedRouterFromCurrentPaths(input.State.Paths)
	}
}

// CloneRoutePath returns a deep copy of one route path metadata object.
func CloneRoutePath(path *RoutePath) *RoutePath {
	if path == nil {
		return nil
	}
	return &RoutePath{
		OriginalPattern: path.OriginalPattern,
		SrcPath:         path.SrcPath,
		OutPath:         path.OutPath,
		ExportKey:       path.ExportKey,
		ErrorExportKey:  path.ErrorExportKey,
		Deps:            append([]string(nil), path.Deps...),
	}
}

// CloneRoutePaths returns a deep copy of one route path map.
func CloneRoutePaths(paths map[string]*RoutePath) map[string]*RoutePath {
	if paths == nil {
		return make(map[string]*RoutePath)
	}

	cloned := make(map[string]*RoutePath, len(paths))
	for pattern, routePath := range paths {
		cloned[pattern] = CloneRoutePath(routePath)
	}
	return cloned
}

// CloneRoutePathsOrNil returns nil when input is nil, else a deep copy.
func CloneRoutePathsOrNil(paths map[string]*RoutePath) map[string]*RoutePath {
	if paths == nil {
		return nil
	}
	return CloneRoutePaths(paths)
}

// CloneStringSliceOrNil returns a copied slice or nil when input is nil.
func CloneStringSliceOrNil(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string(nil), values...)
}

// CloneDepToCSSBundleMapOrEmpty returns a copied dependency map and never nil.
func CloneDepToCSSBundleMapOrEmpty(
	depToBundles map[string][]string,
) map[string][]string {
	if depToBundles == nil {
		return make(map[string][]string)
	}

	cloned := make(map[string][]string, len(depToBundles))
	for dep, bundles := range depToBundles {
		cloned[dep] = append([]string(nil), bundles...)
	}
	return cloned
}

// CloneDepToCSSBundleMapOrNil returns nil when input is nil, else a copy.
func CloneDepToCSSBundleMapOrNil(
	depToBundles map[string][]string,
) map[string][]string {
	if depToBundles == nil {
		return nil
	}
	return CloneDepToCSSBundleMapOrEmpty(depToBundles)
}

// SyncPathsFromDevReload clones parsed client paths and merges server-only
// handler routes.
func SyncPathsFromDevReload(
	parsedClientPaths map[string]*RoutePath,
	serverRoutePatternsWithTaskHandlers []string,
) map[string]*RoutePath {
	paths := CloneRoutePaths(parsedClientPaths)
	for _, pattern := range serverRoutePatternsWithTaskHandlers {
		if _, hasClientRoute := paths[pattern]; hasClientRoute {
			continue
		}
		paths[pattern] = &RoutePath{
			OriginalPattern: pattern,
			SrcPath:         "",
			ExportKey:       "default",
			ErrorExportKey:  "",
		}
	}
	return paths
}

// ReplaceParsedPathsForInit clones parsed client paths for init/re-init flows.
func ReplaceParsedPathsForInit(
	parsedClientPaths map[string]*RoutePath,
) map[string]*RoutePath {
	return CloneRoutePaths(parsedClientPaths)
}

// BuildNestedRouterPatternList returns sorted path patterns for deterministic
// nested-router rebuilds.
func BuildNestedRouterPatternList(paths map[string]*RoutePath) []string {
	patterns := make([]string, 0, len(paths))
	for pattern := range paths {
		patterns = append(patterns, pattern)
	}
	sort.Strings(patterns)
	return patterns
}

// LifecycleState represents runtime initialization/reload lifecycle state.
type LifecycleState string

const (
	// LifecycleStateUninitialized is the initial pre-init state.
	LifecycleStateUninitialized LifecycleState = "uninitialized"
	// LifecycleStateInitializing is set during init commit.
	LifecycleStateInitializing LifecycleState = "initializing"
	// LifecycleStateReady means runtime state is stable for serving.
	LifecycleStateReady LifecycleState = "ready"
	// LifecycleStateReloadingRoutes is set while route artifacts are reloading.
	LifecycleStateReloadingRoutes LifecycleState = "reloading-routes"
	// LifecycleStateReloadingHTML is set while HTML template is reloading.
	LifecycleStateReloadingHTML LifecycleState = "reloading-html-template"
)

// LifecycleStateTracker contains mutable lifecycle fields.
type LifecycleStateTracker struct {
	CurrentState      LifecycleState
	TransitionSeq     uint64
	LastError         string
	RoutePathsPresent bool
}

// LifecycleTransitionResult captures the transition outcome values.
type LifecycleTransitionResult struct {
	PreviousState  LifecycleState
	CurrentState   LifecycleState
	TransitionSeq  uint64
	CurrentError   string
	TransitionNote string
}

// TransitionLifecycleState advances lifecycle state and transition sequence.
func TransitionLifecycleState(
	tracker *LifecycleStateTracker,
	nextState LifecycleState,
	note string,
	lastError string,
) LifecycleTransitionResult {
	if tracker == nil {
		panic("lifecycle state tracker cannot be nil")
	}

	previousState := tracker.CurrentState
	tracker.TransitionSeq++
	tracker.CurrentState = nextState
	tracker.LastError = lastError

	return LifecycleTransitionResult{
		PreviousState:  previousState,
		CurrentState:   tracker.CurrentState,
		TransitionSeq:  tracker.TransitionSeq,
		CurrentError:   tracker.LastError,
		TransitionNote: note,
	}
}

// LifecycleStateForRouteCommit resolves init vs route-reload lifecycle state.
func LifecycleStateForRouteCommit(paths map[string]*RoutePath) LifecycleState {
	if paths == nil {
		return LifecycleStateInitializing
	}
	return LifecycleStateReloadingRoutes
}
