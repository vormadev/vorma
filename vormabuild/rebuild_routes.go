package vormabuild

import (
	"errors"
	"fmt"
	"os"
	"strings"
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

	buildID, err := newFastRebuildID()
	if err != nil {
		return err
	}

	if err := syncRoutesAndWriteFastRebuildArtifacts(v, clientPaths, buildID); err != nil {
		return err
	}

	logFastRouteRebuildCompletion(v, start)
	return nil
}

func newFastRebuildID() (string, error) {
	buildID, err := id.New(16)
	if err != nil {
		return "", fmt.Errorf("generate build ID: %w", err)
	}
	return "dev_fast_" + buildID, nil
}

func syncRoutesAndWriteFastRebuildArtifacts(
	v *vormaruntime.Vorma,
	clientPaths map[string]*vormaruntime.Path,
	buildID string,
) error {
	var writeErr error
	v.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID(buildID)
		l.Routes().Sync(clientPaths)

		if err := cleanRouteManifestsOnly(v); err != nil {
			writeErr = fmt.Errorf("clean route manifests: %w", err)
			return
		}

		if err := writeRouteArtifacts(l); err != nil {
			writeErr = err
		}
	})
	return writeErr
}

func logFastRouteRebuildCompletion(v *vormaruntime.Vorma, start time.Time) {
	v.Log.Info("DONE fast route rebuild",
		"buildID", v.GetBuildID(),
		"routes", len(v.GetPathsSnapshot()),
		"duration", time.Since(start),
	)
}

func cleanRouteManifestsOnly(v *vormaruntime.Vorma) error {
	staticPublicOutDir := v.Wave.GetStaticPublicOutDir()
	err := removeMatchingTopLevelFiles(staticPublicOutDir, isGeneratedRouteManifestFilename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

func isGeneratedRouteManifestFilename(fileName string) bool {
	return len(fileName) > len(vormaruntime.VormaRouteManifestPrefix) &&
		strings.HasPrefix(fileName, vormaruntime.VormaRouteManifestPrefix)
}
