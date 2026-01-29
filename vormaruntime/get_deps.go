package vormaruntime

import (
	"github.com/vormadev/vorma/kit/matcher"
)

func (v *Vorma) getDepsFromSnapshot(_matches []*matcher.Match, paths map[string]*Path) []string {
	clientEntryDeps := v.GetClientEntryDeps()

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
	clientEntryOut := v.GetClientEntryOut()
	depToCSSBundleMap := v.GetDepToCSSBundleMap()

	// Use a map to deduplicate CSS bundles
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
	if bundles, exists := depToCSSBundleMap[clientEntryOut]; exists {
		addBundles(bundles)
	}

	// Add CSS bundles from dependencies
	for _, dep := range deps {
		if bundles, exists := depToCSSBundleMap[dep]; exists {
			addBundles(bundles)
		}
	}

	return cssBundles
}
