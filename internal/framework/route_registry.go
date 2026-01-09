package vorma

import (
	"fmt"

	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/typed"
)

// routeRegistry consolidates route state management to ensure buildInner
// and rebuildRoutesOnly stay in sync. Both methods must go through this
// registry to modify route state.
//
// Owned state:
//   - v._paths (the canonical route definitions)
//   - v.gmpdCache (match results cache)
//   - router registration state
type routeRegistry struct {
	vorma *Vorma
}

func (v *Vorma) routes() *routeRegistry {
	return &routeRegistry{vorma: v}
}

// Sync updates the route state from parsed client routes.
// This is the single entry point for route modifications.
//
// It performs:
//  1. Stores paths
//  2. Merges server-only routes
//  3. Rebuilds the router atomically
//  4. Clears the gmpd cache
//
// IMPORTANT: Caller must hold v.mu.Lock() before calling this method.
func (r *routeRegistry) Sync(paths map[string]*Path) {
	v := r.vorma

	// Store paths
	v._paths = paths

	// Merge server-only routes (routes with loaders but no client component)
	r.mergeServerRoutes()

	// Rebuild router atomically
	patterns := make([]string, 0, len(v._paths))
	for pattern := range v._paths {
		patterns = append(patterns, pattern)
	}
	v.LoadersRouter().NestedRouter.RebuildWithClientRoutes(patterns)

	// Clear cache
	r.clearCache()
}

// WriteArtifacts writes all route-related artifacts to disk.
// Call this after Sync() to persist changes.
//
// It writes:
//  1. Route manifest (JSON for client)
//  2. Paths stage one JSON
//  3. Generated TypeScript
//
// IMPORTANT: Caller must hold v.mu.Lock() before calling this method.
func (r *routeRegistry) WriteArtifacts() error {
	v := r.vorma

	manifest := v.generateRouteManifest(v.LoadersRouter().NestedRouter)
	manifestFile, err := v.writeRouteManifestToDisk(manifest)
	if err != nil {
		return fmt.Errorf("write route manifest: %w", err)
	}
	v._routeManifestFile = manifestFile

	if err := v.writePathsToDisk_StageOne(); err != nil {
		return fmt.Errorf("write paths JSON: %w", err)
	}

	if err := v.writeGeneratedTS(); err != nil {
		return fmt.Errorf("write generated TypeScript: %w", err)
	}

	return nil
}

// mergeServerRoutes adds server-only routes to paths.
// Server-only routes have Go loaders but no client component.
//
// IMPORTANT: Caller must hold v.mu.Lock() before calling this method.
func (r *routeRegistry) mergeServerRoutes() {
	v := r.vorma
	allServerRoutes := v.LoadersRouter().NestedRouter.AllRoutes()
	for pattern := range allServerRoutes {
		// IMPORTANT: Only merge if it actually has a server handler.
		// Otherwise we risk resurrecting deleted client-only routes as ghosts.
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

// clearCache clears the gmpd cache so new routes are picked up.
// Uses atomic swap internally so in-flight requests keep their reference to the old map.
//
// IMPORTANT: Caller must hold v.mu.Lock() before calling this method.
func (r *routeRegistry) clearCache() {
	v := r.vorma
	v.gmpdCache = typed.NewSyncMap[string, *cachedItemSubset]()
}

// RegisterPatternIfNeeded registers a pattern in the router if not already registered.
// Used during initialization when routes are registered incrementally.
func (r *routeRegistry) RegisterPatternIfNeeded(pattern string) {
	nestedRouter := r.vorma.LoadersRouter().NestedRouter
	if !nestedRouter.IsRegistered(pattern) {
		mux.RegisterNestedPatternWithoutHandler(nestedRouter, pattern)
	}
}
