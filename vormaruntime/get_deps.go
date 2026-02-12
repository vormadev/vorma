package vormaruntime

import (
	"github.com/vormadev/vorma/kit/matcher"
)

func (v *Vorma) getDepsFromSnapshot(_matches []*matcher.Match, paths map[string]*Path) []string {
	v.mu.RLock()
	clientEntryDeps := v._clientEntryDeps
	v.mu.RUnlock()

	return getDepsFromData(_matches, paths, clientEntryDeps)
}

func getDepsFromData(_matches []*matcher.Match, paths map[string]*Path, clientEntryDeps []string) []string {
	var deps []string
	seen := make(map[string]struct{}, len(_matches))
	handleDeps := func(src []string) {
		for _, d := range src {
			if _, ok := seen[d]; !ok {
				deps = append(deps, d)
				seen[d] = struct{}{}
			}
		}
	}
	if clientEntryDeps != nil {
		handleDeps(clientEntryDeps)
	}
	for _, match := range _matches {
		path := paths[match.OriginalPattern()]
		if path == nil {
			continue
		}
		handleDeps(path.Deps)
	}
	return deps
}

func (v *Vorma) getCSSBundles(deps []string) []string {
	v.mu.RLock()
	clientEntryOut := v._clientEntryOut
	depToCSSBundleMap := v._depToCSSBundleMap
	v.mu.RUnlock()

	return getCSSBundlesFromSnapshot(deps, clientEntryOut, depToCSSBundleMap)
}

func getCSSBundlesFromSnapshot(
	deps []string,
	clientEntryOut string,
	depToCSSBundleMap map[string][]string,
) []string {
	// Use a map to deduplicate CSS bundles
	clientEntryBundles := depToCSSBundleMap[clientEntryOut]
	seen := make(map[string]struct{})
	cssBundles := make([]string, 0, len(deps))

	addBundles := func(bundles []string) {
		for _, bundle := range bundles {
			if _, exists := seen[bundle]; !exists {
				seen[bundle] = struct{}{}
				cssBundles = append(cssBundles, bundle)
			}
		}
	}

	// Add CSS bundles from client entry first
	if len(clientEntryBundles) > 0 {
		addBundles(clientEntryBundles)
	}

	// Add CSS bundles from dependencies
	for _, dep := range deps {
		if bundles, exists := depToCSSBundleMap[dep]; exists {
			addBundles(bundles)
		}
	}

	return cssBundles
}
