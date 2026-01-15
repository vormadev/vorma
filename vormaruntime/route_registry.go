package vormaruntime

import (
	"github.com/vormadev/vorma/kit/mux"
)

// RouteRegistry consolidates route state management.
// All methods require the Vorma mutex to be held.
type RouteRegistry struct {
	vorma *Vorma
}

// routes returns a RouteRegistry for route management operations.
// This is unexported to enforce that external packages use
// LockedVorma.Routes() via WithLock for compile-time safety.
func (v *Vorma) routes() *RouteRegistry {
	return &RouteRegistry{vorma: v}
}

// Sync updates the route state from parsed client routes.
// Caller must hold v.mu.Lock().
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
// Caller must hold v.mu.Lock().
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
// This method is safe to call without holding the lock as the router handles
// its own synchronization.
func (v *Vorma) RegisterPatternIfNeeded(pattern string) {
	nestedRouter := v.LoadersRouter().NestedRouter
	if !nestedRouter.IsRegistered(pattern) {
		mux.RegisterNestedPatternWithoutHandler(nestedRouter, pattern)
	}
}
