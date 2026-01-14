package vormaruntime

import (
	"github.com/vormadev/vorma/kit/mux"
)

// RouteRegistry consolidates route state management.
// IMPORTANT: All methods assume caller holds v.mu.Lock().
type RouteRegistry struct {
	vorma *Vorma
}

func (v *Vorma) Routes() *RouteRegistry {
	return &RouteRegistry{vorma: v}
}

// Sync updates the route state from parsed client routes.
// IMPORTANT: Caller must hold v.mu.Lock().
func (r *RouteRegistry) Sync(paths map[string]*Path) {
	v := r.vorma
	v._paths = paths
	r.mergeServerRoutes()

	patterns := make([]string, 0, len(v._paths))
	for pattern := range v._paths {
		patterns = append(patterns, pattern)
	}
	v.LoadersRouter().NestedRouter.RebuildPreservingHandlers(patterns)
}

// mergeServerRoutes adds server-only routes to paths.
// IMPORTANT: Caller must hold v.mu.Lock().
func (r *RouteRegistry) mergeServerRoutes() {
	v := r.vorma
	allServerRoutes := v.LoadersRouter().NestedRouter.AllRoutes()
	for pattern := range allServerRoutes {
		if !v.LoadersRouter().NestedRouter.HasTaskHandler(pattern) {
			continue
		}
		if _, hasClientRoute := v._paths[pattern]; !hasClientRoute {
			v._paths[pattern] = &Path{
				OriginalPattern: pattern,
				SrcPath:         "",
				ExportKey:       "default",
				ErrorExportKey:  "",
			}
		}
	}
}

// RegisterPatternIfNeeded registers a pattern if not already registered.
func (r *RouteRegistry) RegisterPatternIfNeeded(pattern string) {
	nestedRouter := r.vorma.LoadersRouter().NestedRouter
	if !nestedRouter.IsRegistered(pattern) {
		mux.RegisterNestedPatternWithoutHandler(nestedRouter, pattern)
	}
}
