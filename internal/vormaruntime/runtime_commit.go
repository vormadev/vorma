package vormaruntime

import "html/template"

type routeArtifactCommitMode uint8

const (
	routeArtifactCommitModeInit routeArtifactCommitMode = iota
	routeArtifactCommitModeDevReload
)

func (v *Vorma) commitRouteArtifactsLocked(
	artifacts *runtimeRouteArtifacts,
	rebuildNestedRouter bool,
	commitMode routeArtifactCommitMode,
) {
	if artifacts == nil {
		panic("runtimeRouteArtifacts cannot be nil")
	}

	v.applyRuntimeRouteArtifactsMetadataLocked(artifacts)

	switch commitMode {
	case routeArtifactCommitModeDevReload:
		v.routes().SyncFromDevReload(artifacts.parsedClientPaths)
	default:
		v.routes().ReplaceParsedPathsForInit(artifacts.parsedClientPaths, rebuildNestedRouter)
	}
}

func (v *Vorma) commitRootTemplateLocked(rootTemplate *template.Template) {
	v._rootTemplate = rootTemplate
}
