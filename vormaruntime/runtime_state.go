package vormaruntime

import "strings"

func clearRouteDataCache() {
	gmpdCache.Range(func(key, _ any) bool {
		gmpdCache.Delete(key)
		return true
	})
}

func clearRouteDataCacheForAppIdentity(appIdentity string) {
	if appIdentity == "" {
		return
	}

	cacheKeyPrefix := appIdentity + "|"
	gmpdCache.Range(func(key, _ any) bool {
		cacheKey, isCacheKeyString := key.(string)
		if isCacheKeyString && strings.HasPrefix(cacheKey, cacheKeyPrefix) {
			gmpdCache.Delete(key)
		}
		return true
	})
}

// invalidateRouteDataCacheLocked invalidates route-data cache entries for
// requests that still reference an older runtime snapshot.
//
// Caller must hold v.mu.Lock().
func (v *Vorma) invalidateRouteDataCacheLocked() {
	v._routeDataSnapshotVersion++

	appIdentity := v._routeDataCacheAppIdentity
	if appIdentity == "" {
		appIdentity = computeRouteDataCacheAppIdentity(v)
		v._routeDataCacheAppIdentity = appIdentity
	}
	clearRouteDataCacheForAppIdentity(appIdentity)
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

func (v *Vorma) applyPathsFileMetadataLocked(pathsFile *PathsFile) {
	v._buildID = pathsFile.BuildID
	v._clientEntrySrc = pathsFile.ClientEntrySrc
	v._clientEntryOut = pathsFile.ClientEntryOut
	v._clientEntryDeps = cloneStringSliceOrNil(pathsFile.ClientEntryDeps)
	v._depToCSSBundleMap = cloneDepToCSSBundleMapOrEmpty(pathsFile.DepToCSSBundleMap)
	v._routeManifestFile = pathsFile.RouteManifestFile
}
