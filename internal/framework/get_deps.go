package vorma

import "github.com/vormadev/vorma/kit/matcher"

func (v *Vorma) getDeps(_matches []*matcher.Match) []string {
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
	if v._clientEntryDeps != nil {
		handleDeps(v._clientEntryDeps)
	}
	for _, match := range _matches {
		path := v._paths[match.OriginalPattern()]
		if path == nil {
			continue
		}
		handleDeps(path.Deps)
	}
	return deps
}

// order matters
func (v *Vorma) getCSSBundles(deps []string) []string {
	cssBundles := make([]string, 0, len(deps))
	// first, client entry CSS
	if x, exists := v._depToCSSBundleMap[v._clientEntryOut]; exists {
		cssBundles = append(cssBundles, x)
	}
	// then all downstream deps
	for _, dep := range deps {
		if x, exists := v._depToCSSBundleMap[dep]; exists {
			cssBundles = append(cssBundles, x)
		}
	}
	return cssBundles
}
