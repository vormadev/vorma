package vorma

import (
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"

	"github.com/vormadev/vorma/kit/id"
)

// rebuildRoutesOnly is the fast path for rebuilding when only vorma.routes.ts changes.
// It reuses the in-memory router state instead of spawning a Go subprocess.
//
// Performance: ~50ms vs ~1.5s for full rebuild
//
// Safety: This is safe because when this function is called, we know Go code
// has not changed (Go file changes trigger a different code path that does
// a full rebuild). Therefore the in-memory LoadersRouter and ActionsRouter
// are guaranteed to be current.
func (v *Vorma) rebuildRoutesOnly() error {
	start := time.Now()

	// Parse client routes BEFORE acquiring the lock to minimize lock duration.
	// This is safe because parseClientRoutes only reads from disk and doesn't
	// access any mutable Vorma state.
	paths, err := v.parseClientRoutes()
	if err != nil {
		return fmt.Errorf("parse client routes: %w", err)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if !v._isDev {
		return errors.New("rebuildRoutesOnly should only be called in dev mode")
	}

	Log.Info("START fast route rebuild")

	// Generate new build ID for cache busting
	buildID, err := id.New(16)
	if err != nil {
		return fmt.Errorf("generate build ID: %w", err)
	}
	v._buildID = "dev_fast_" + buildID

	// Sync routes via registry (merges server routes, rebuilds router, clears cache)
	v.routes().Sync(paths)

	// Clean old route manifests only (preserve Vite files)
	if err := v.cleanRouteManifestsOnly(); err != nil {
		return fmt.Errorf("clean route manifests: %w", err)
	}

	// Write all route artifacts
	if err := v.routes().WriteArtifacts(); err != nil {
		return err
	}

	Log.Info("DONE fast route rebuild",
		"buildID", v._buildID,
		"routes", len(v._paths),
		"duration", time.Since(start),
	)

	return nil
}

// reloadTemplate re-parses the HTML template from disk without restarting.
// This is the fast path for when only the HTML template file changes.
func (v *Vorma) reloadTemplate() error {
	// Parse template BEFORE acquiring lock to minimize lock duration
	srcPath := filepath.Join(v.Wave.GetPrivateStaticDir(), v.config.HTMLTemplateLocation)
	tmpl, err := template.ParseFiles(srcPath)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if !v._isDev {
		return errors.New("reloadTemplate should only be called in dev mode")
	}

	Log.Info("Reloading HTML template")

	v._rootTemplate = tmpl

	Log.Info("HTML template reloaded successfully")
	return nil
}

// cleanRouteManifestsOnly removes only route manifest files from the output dir.
// Unlike cleanStaticPublicOutDir, this preserves Vite output files since they
// don't change when only routes change.
func (v *Vorma) cleanRouteManifestsOnly() error {
	staticPublicOutDir := v.Wave.GetStaticPublicOutDir()

	entries, err := os.ReadDir(staticPublicOutDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) > len(vormaRouteManifestPrefix) &&
			name[:len(vormaRouteManifestPrefix)] == vormaRouteManifestPrefix {
			if err := os.Remove(filepath.Join(staticPublicOutDir, name)); err != nil {
				return fmt.Errorf("remove %s: %w", name, err)
			}
		}
	}

	return nil
}
