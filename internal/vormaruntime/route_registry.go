package vormaruntime

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

// SyncFromDevReload updates the route state from parsed client routes for
// dev-time reload, preserving server-only handlers in parsed-path state and
// rebuilding nested-router registrations.
// Caller must hold v.mu.Lock().
func (r *RouteRegistry) SyncFromDevReload(paths map[string]*Path) {
	v := r.vorma
	v._paths = clonePathsMap(paths)
	r.mergeServerRoutes()
	v.invalidateRouteDataCacheLocked()
	r.rebuildNestedRouterFromCurrentPaths()
}

// ReplaceParsedPathsForInit updates route state from parsed client routes for
// init/re-init flows. This does not merge server-only handlers into parsed-path
// state, but can rebuild nested-router registrations when requested.
// Caller must hold v.mu.Lock().
func (r *RouteRegistry) ReplaceParsedPathsForInit(
	paths map[string]*Path,
	rebuildNestedRouter bool,
) {
	v := r.vorma
	v._paths = clonePathsMap(paths)
	v.invalidateRouteDataCacheLocked()
	if rebuildNestedRouter {
		r.rebuildNestedRouterFromCurrentPaths()
	}
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

func (r *RouteRegistry) rebuildNestedRouterFromCurrentPaths() {
	v := r.vorma
	patterns := make([]string, 0, len(v._paths))
	for pattern := range v._paths {
		patterns = append(patterns, pattern)
	}
	v.LoadersRouter().NestedRouter.RebuildPreservingHandlers(patterns)
}

// RegisterPatternIfNeeded registers a pattern if not already registered.
// This method is safe to call without holding the lock as the router handles
// its own synchronization.
func (v *Vorma) RegisterPatternIfNeeded(pattern string) {
	v.LoadersRouter().NestedRouter.AddPatternWithoutHandlerIfMissing(
		pattern,
	)
}
