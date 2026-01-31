package vormabuild

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vormadev/vorma/kit/id"
	"github.com/vormadev/vorma/vormaruntime"
)

// rebuildRoutesOnly is the fast path for rebuilding when only vorma.routes.ts changes.
// Runs in Process A (Dev Server), which has handlers registered for type reflection.
//
// Flow:
//  1. Parse client routes with esbuild
//  2. Generate TypeScript using live reflection (Process A has handlers)
//  3. Write all artifacts to disk
//  4. Wave calls Process B's reload endpoint to sync from disk
//
// Performance: ~50ms vs ~1.5s for full rebuild
func rebuildRoutesOnly(v *vormaruntime.Vorma) error {
	start := time.Now()

	if !v.GetIsDevMode() {
		return errors.New("rebuildRoutesOnly should only be called in dev mode")
	}

	v.Log.Info("START fast route rebuild")

	// 1. Parse client routes (before acquiring lock)
	clientPaths, err := parseClientRoutes(v)
	if err != nil {
		return fmt.Errorf("parse client routes: %w", err)
	}

	// 2. Generate new build ID (before acquiring lock)
	buildID, err := id.New(16)
	if err != nil {
		return fmt.Errorf("generate build ID: %w", err)
	}

	// 3. Acquire lock and update all state atomically
	var writeErr error
	v.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("dev_fast_" + buildID)
		l.Routes().Sync(clientPaths)

		// 4. Clean old route manifests
		if err := cleanRouteManifestsOnly(v); err != nil {
			writeErr = fmt.Errorf("clean route manifests: %w", err)
			return
		}

		// 5. Write all artifacts (manifest, paths JSON, TypeScript)
		if err := writeRouteArtifacts(l); err != nil {
			writeErr = err
			return
		}
	})

	if writeErr != nil {
		return writeErr
	}

	v.Log.Info("DONE fast route rebuild",
		"buildID", v.GetBuildID(),
		"routes", len(v.GetPathsSnapshot()),
		"duration", time.Since(start),
	)

	return nil
}

func cleanRouteManifestsOnly(v *vormaruntime.Vorma) error {
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
		if len(name) > len(vormaruntime.VormaRouteManifestPrefix) &&
			name[:len(vormaruntime.VormaRouteManifestPrefix)] == vormaruntime.VormaRouteManifestPrefix {
			if err := os.Remove(filepath.Join(staticPublicOutDir, name)); err != nil {
				return fmt.Errorf("remove %s: %w", name, err)
			}
		}
	}

	return nil
}
