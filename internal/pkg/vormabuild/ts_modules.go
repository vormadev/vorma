package vormabuild

import (
	"fmt"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/set"
)

type ts_route struct {
	Pattern    string
	ImportPath string
	Deps       []string
}

func (cfg vorma_cfg) get_dev_ts_modules(r *vorma.Router) (map[string]ts_route, error) {
	views := r.Views()
	routes := make(map[string]ts_route, len(views))
	seen_mods := set.New[string]()

	for _, view := range views {
		pattern := view.GetPattern()
		mod := view.GetClientModule()
		if pattern == "" || mod == "" {
			continue
		}
		if _, exists := routes[pattern]; exists {
			return nil, fmt.Errorf("duplicate TS route pattern: %s", pattern)
		}
		if seen_mods.Has(mod) {
			return nil, fmt.Errorf("duplicate TS module: %s", mod)
		}
		seen_mods.Add(mod)
		routes[pattern] = ts_route{
			Pattern:    pattern,
			ImportPath: mod,
			Deps:       nil, // nil in dev
		}
	}

	return routes, nil
}
