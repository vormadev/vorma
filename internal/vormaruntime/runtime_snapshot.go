package vormaruntime

import (
	"html/template"
	"sync"
)

// RuntimeSnapshot captures request-serving runtime state for one coherent
// generation.
type RuntimeSnapshot struct {
	buildID                  string
	isDev                    bool
	paths                    map[string]*Path
	clientEntryDeps          []string
	clientEntryOut           string
	depToCSSBundleMap        map[string][]string
	rootTemplate             *template.Template
	routeManifestFile        string
	routeDataSnapshotVersion uint64
	routeDataCache           *sync.Map
}

func (v *Vorma) captureRuntimeSnapshotLocked() RuntimeSnapshot {
	return RuntimeSnapshot{
		buildID:                  v._buildID,
		isDev:                    v._isDev,
		paths:                    v._paths,
		clientEntryDeps:          v._clientEntryDeps,
		clientEntryOut:           v._clientEntryOut,
		depToCSSBundleMap:        v._depToCSSBundleMap,
		rootTemplate:             v._rootTemplate,
		routeManifestFile:        v._routeManifestFile,
		routeDataSnapshotVersion: v._routeDataSnapshotVersion,
		routeDataCache:           v._routeDataCache,
	}
}

func (snapshot RuntimeSnapshot) toLoadersHTMLRender() loadersHTMLRenderSnapshot {
	return loadersHTMLRenderSnapshot{
		isDevMode:      snapshot.isDev,
		clientEntryOut: snapshot.clientEntryOut,
		rootTemplate:   snapshot.rootTemplate,
	}
}
