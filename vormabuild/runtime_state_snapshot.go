package vormabuild

import "github.com/vormadev/vorma/internal/vormaruntime"

type buildRuntimeStateSnapshot struct {
	isDev                  bool
	routeBuildRuntimeState routeBuildRuntimeStateSnapshot
}

type routeBuildRuntimeStateSnapshot struct {
	paths             map[string]*vormaruntime.Path
	buildID           string
	routeManifestFile string
}

type buildRuntimeStateReader interface {
	GetIsDev() bool
	GetPaths() map[string]*vormaruntime.Path
	GetBuildID() string
	GetRouteManifestFile() string
}

type routeBuildRuntimeStateReader interface {
	GetPaths() map[string]*vormaruntime.Path
	GetBuildID() string
	GetRouteManifestFile() string
}

func captureBuildRuntimeStateSnapshot(
	l buildRuntimeStateReader,
) buildRuntimeStateSnapshot {
	return buildRuntimeStateSnapshot{
		isDev:                  l.GetIsDev(),
		routeBuildRuntimeState: captureRouteBuildRuntimeStateSnapshot(l),
	}
}

func captureRouteBuildRuntimeStateSnapshot(
	l routeBuildRuntimeStateReader,
) routeBuildRuntimeStateSnapshot {
	return routeBuildRuntimeStateSnapshot{
		paths:             cloneRouteBuildRuntimePathsMap(l.GetPaths()),
		buildID:           l.GetBuildID(),
		routeManifestFile: l.GetRouteManifestFile(),
	}
}

func restoreBuildRuntimeStateSnapshot(
	l *vormaruntime.LockedVorma,
	snapshot buildRuntimeStateSnapshot,
) {
	commitRuntimeStateWithLock(
		l,
		runtimeStateCommitInput{
			shouldCommitIsDev:             true,
			isDev:                         snapshot.isDev,
			shouldCommitBuildID:           true,
			buildID:                       snapshot.routeBuildRuntimeState.buildID,
			shouldCommitRouteManifestFile: true,
			routeManifestFile:             snapshot.routeBuildRuntimeState.routeManifestFile,
			routePaths:                    snapshot.routeBuildRuntimeState.paths,
			routePathsUpdateMode:          runtimeStateRoutePathsUpdateModeReplaceParsedPathsForInit,
			shouldRebuildNestedRouter:     shouldRebuildNestedRouterFromCurrentRuntimeState(l),
		},
	)
}

func restoreRouteBuildRuntimeStateSnapshot(
	l *vormaruntime.LockedVorma,
	snapshot routeBuildRuntimeStateSnapshot,
) {
	commitRuntimeStateWithLock(
		l,
		runtimeStateCommitInput{
			shouldCommitBuildID:           true,
			buildID:                       snapshot.buildID,
			shouldCommitRouteManifestFile: true,
			routeManifestFile:             snapshot.routeManifestFile,
			routePaths:                    snapshot.paths,
			routePathsUpdateMode:          runtimeStateRoutePathsUpdateModeReplaceParsedPathsForInit,
			shouldRebuildNestedRouter:     shouldRebuildNestedRouterFromCurrentRuntimeState(l),
		},
	)
}

func buildRuntimeStateSnapshotMatches(
	l buildRuntimeStateReader,
	snapshot buildRuntimeStateSnapshot,
) bool {
	if l.GetIsDev() != snapshot.isDev {
		return false
	}
	return routeBuildRuntimeStateSnapshotMatches(l, snapshot.routeBuildRuntimeState)
}

func routeBuildRuntimeStateSnapshotMatches(
	l routeBuildRuntimeStateReader,
	snapshot routeBuildRuntimeStateSnapshot,
) bool {
	if l.GetBuildID() != snapshot.buildID {
		return false
	}
	if l.GetRouteManifestFile() != snapshot.routeManifestFile {
		return false
	}
	return routeBuildRuntimePathsMapMatches(l.GetPaths(), snapshot.paths)
}

func routeBuildRuntimePathsMapMatches(
	currentPaths map[string]*vormaruntime.Path,
	expectedPaths map[string]*vormaruntime.Path,
) bool {
	if len(currentPaths) != len(expectedPaths) {
		return false
	}
	for routePattern, currentPath := range currentPaths {
		expectedPath, hasExpectedPath := expectedPaths[routePattern]
		if !hasExpectedPath {
			return false
		}
		if !routeBuildRuntimePathMatches(currentPath, expectedPath) {
			return false
		}
	}
	return true
}

func routeBuildRuntimePathMatches(
	currentPath *vormaruntime.Path,
	expectedPath *vormaruntime.Path,
) bool {
	if currentPath == nil || expectedPath == nil {
		return currentPath == expectedPath
	}
	if currentPath.OriginalPattern != expectedPath.OriginalPattern {
		return false
	}
	if currentPath.SrcPath != expectedPath.SrcPath {
		return false
	}
	if currentPath.ExportKey != expectedPath.ExportKey {
		return false
	}
	if currentPath.ErrorExportKey != expectedPath.ErrorExportKey {
		return false
	}
	if currentPath.OutPath != expectedPath.OutPath {
		return false
	}
	if len(currentPath.Deps) != len(expectedPath.Deps) {
		return false
	}
	for depIndex := range currentPath.Deps {
		if currentPath.Deps[depIndex] != expectedPath.Deps[depIndex] {
			return false
		}
	}
	return true
}

func cloneRouteBuildRuntimePathsMap(
	paths map[string]*vormaruntime.Path,
) map[string]*vormaruntime.Path {
	if paths == nil {
		return nil
	}

	clonedPaths := make(map[string]*vormaruntime.Path, len(paths))
	for pattern, path := range paths {
		clonedPaths[pattern] = cloneRouteBuildRuntimePath(path)
	}
	return clonedPaths
}

func cloneRouteBuildRuntimePath(path *vormaruntime.Path) *vormaruntime.Path {
	if path == nil {
		return nil
	}

	clonedPath := *path
	if path.Deps != nil {
		clonedPath.Deps = append([]string(nil), path.Deps...)
	}
	return &clonedPath
}
