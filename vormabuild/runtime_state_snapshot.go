package vormabuild

import "github.com/vormadev/vorma/internal/vormaruntime"

type routeBuildRuntimeStateSnapshot struct {
	paths             map[string]*vormaruntime.Path
	buildID           string
	routeManifestFile string
}

type routeBuildRuntimeStateReader interface {
	GetPaths() map[string]*vormaruntime.Path
	GetBuildID() string
	GetRouteManifestFile() string
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

func restoreRouteBuildRuntimeStateSnapshot(
	l *vormaruntime.LockedVorma,
	snapshot routeBuildRuntimeStateSnapshot,
) {
	shouldRebuildNestedRouter := l.Vorma().LoadersRouter() != nil &&
		l.Vorma().LoadersRouter().NestedRouter != nil

	l.Routes().ReplaceParsedPathsForInit(snapshot.paths, shouldRebuildNestedRouter)
	l.SetBuildID(snapshot.buildID)
	l.SetRouteManifestFile(snapshot.routeManifestFile)
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
