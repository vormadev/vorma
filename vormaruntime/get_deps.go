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

	cssBundles := make([]string, 0, len(deps))
	if x, exists := depToCSSBundleMap[clientEntryOut]; exists {
		cssBundles = append(cssBundles, x)
	}
	for _, dep := range deps {
		if x, exists := depToCSSBundleMap[dep]; exists {
			cssBundles = append(cssBundles, x)
		}
	}
	return cssBundles
}
