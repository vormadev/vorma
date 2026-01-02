package vorma

import "github.com/vormadev/vorma/kit/matcher"

func (h *Vorma) getDeps(_matches []*matcher.Match) []string {
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
	if h._clientEntryDeps != nil {
		handleDeps(h._clientEntryDeps)
	}
	for _, match := range _matches {
		path := h._paths[match.OriginalPattern()]
		if path == nil {
			continue
		}
		handleDeps(path.Deps)
	}
	return deps
}

// order matters
func (h *Vorma) getCSSBundles(deps []string) []string {
	cssBundles := make([]string, 0, len(deps))
	// first, client entry CSS
	if x, exists := h._depToCSSBundleMap[h._clientEntryOut]; exists {
		cssBundles = append(cssBundles, x)
	}
	// then all downstream deps
	for _, dep := range deps {
		if x, exists := h._depToCSSBundleMap[dep]; exists {
			cssBundles = append(cssBundles, x)
		}
	}
	return cssBundles
}
