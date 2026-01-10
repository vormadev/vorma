package vorma

import "github.com/vormadev/vorma/kit/matcher"

// getDepsFromSnapshot returns dependencies using an already-acquired paths snapshot.
// Use this when you already have a consistent view of paths to avoid redundant locking.
func (v *Vorma) getDepsFromSnapshot(_matches []*matcher.Match, paths map[string]*Path) []string {
	clientEntryDeps := v.getClientEntryDeps()

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

// getCSSBundles returns CSS bundles for the given dependencies.
// Order matters: client entry CSS first, then downstream deps.
func (v *Vorma) getCSSBundles(deps []string) []string {
	clientEntryOut := v.getClientEntryOut()
	depToCSSBundleMap := v.getDepToCSSBundleMap()

	cssBundles := make([]string, 0, len(deps))
	// first, client entry CSS
	if x, exists := depToCSSBundleMap[clientEntryOut]; exists {
		cssBundles = append(cssBundles, x)
	}
	// then all downstream deps
	for _, dep := range deps {
		if x, exists := depToCSSBundleMap[dep]; exists {
			cssBundles = append(cssBundles, x)
		}
	}
	return cssBundles
}
