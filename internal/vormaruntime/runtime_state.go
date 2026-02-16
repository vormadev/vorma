package vormaruntime

import "sync"

// invalidateRouteDataCacheLocked invalidates route-data cache entries for
// requests that still reference an older runtime snapshot.
//
// Caller must hold v.mu.Lock().
func (v *Vorma) invalidateRouteDataCacheLocked() {
	v._routeDataSnapshotVersion++
	v._routeDataCache = &sync.Map{}
}

func clonePathsMap(paths map[string]*Path) map[string]*Path {
	if paths == nil {
		return make(map[string]*Path)
	}

	cloned := make(map[string]*Path, len(paths))
	for pattern, p := range paths {
		cloned[pattern] = clonePath(p)
	}
	return cloned
}

func clonePathsMapOrNil(paths map[string]*Path) map[string]*Path {
	if paths == nil {
		return nil
	}
	return clonePathsMap(paths)
}

func cloneStringSliceOrNil(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string(nil), values...)
}

func cloneDepToCSSBundleMapOrEmpty(depToBundles map[string][]string) map[string][]string {
	if depToBundles == nil {
		return make(map[string][]string)
	}

	cloned := make(map[string][]string, len(depToBundles))
	for dep, bundles := range depToBundles {
		cloned[dep] = append([]string(nil), bundles...)
	}
	return cloned
}

func cloneDepToCSSBundleMapOrNil(depToBundles map[string][]string) map[string][]string {
	if depToBundles == nil {
		return nil
	}
	return cloneDepToCSSBundleMapOrEmpty(depToBundles)
}

func (v *Vorma) applyRuntimeRouteArtifactsMetadataLocked(
	artifacts *runtimeRouteArtifacts,
) {
	if artifacts == nil {
		v._buildID = ""
		v._clientEntrySrc = ""
		v._clientEntryOut = ""
		v._clientEntryDeps = nil
		v._depToCSSBundleMap = make(map[string][]string)
		v._routeManifestFile = ""
		return
	}

	v._buildID = artifacts.buildID
	v._clientEntrySrc = artifacts.clientEntrySrc
	v._clientEntryOut = artifacts.clientEntryOut
	v._clientEntryDeps = cloneStringSliceOrNil(artifacts.clientEntryDeps)
	v._depToCSSBundleMap = cloneDepToCSSBundleMapOrEmpty(artifacts.depToCSSBundleMap)
	v._routeManifestFile = artifacts.routeManifestFile
}
