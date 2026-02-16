package vormaruntime

import "fmt"

type runtimeRouteArtifacts struct {
	buildID           string
	clientEntrySrc    string
	clientEntryOut    string
	clientEntryDeps   []string
	depToCSSBundleMap map[string][]string
	routeManifestFile string
	parsedClientPaths map[string]*Path
}

func buildRuntimeRouteArtifacts(pathsFile *PathsFile) (*runtimeRouteArtifacts, error) {
	if pathsFile == nil {
		return nil, fmt.Errorf("paths file is nil")
	}

	return &runtimeRouteArtifacts{
		buildID:           pathsFile.BuildID,
		clientEntrySrc:    pathsFile.ClientEntrySrc,
		clientEntryOut:    pathsFile.ClientEntryOut,
		clientEntryDeps:   pathsFile.ClientEntryDeps,
		depToCSSBundleMap: pathsFile.DepToCSSBundleMap,
		routeManifestFile: pathsFile.RouteManifestFile,
		parsedClientPaths: pathsFile.Paths,
	}, nil
}
