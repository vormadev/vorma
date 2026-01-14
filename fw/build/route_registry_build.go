package build

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vormadev/vorma/fw/runtime"
	"github.com/vormadev/vorma/fw/types"
	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/mux"
)

// writeRouteArtifacts writes all route-related artifacts to disk.
// Includes manifest, paths JSON, and TypeScript generation.
// IMPORTANT: Caller must hold v.mu.Lock().
func writeRouteArtifacts(v *runtime.Vorma) error {
	// 1. Generate & Write Manifest
	manifest := generateRouteManifest(v, v.LoadersRouter().NestedRouter)
	manifestFile, err := writeRouteManifestToDisk(v, manifest)
	if err != nil {
		return fmt.Errorf("write route manifest: %w", err)
	}
	v.UnsafeSetRouteManifestFile(manifestFile)

	// 2. Write Paths JSON (Stage One)
	if err := writePathsToDisk_StageOne(v); err != nil {
		return fmt.Errorf("write paths JSON: %w", err)
	}

	// 3. Generate TypeScript
	if err := WriteGeneratedTS(v); err != nil {
		return fmt.Errorf("write generated TypeScript: %w", err)
	}

	return nil
}

func writeRouteManifestToDisk(v *runtime.Vorma, manifest map[string]int) (string, error) {
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("marshal route manifest: %w", err)
	}

	hash := cryptoutil.Sha256Hash(manifestJSON)
	hashStr := base64.RawURLEncoding.EncodeToString(hash[:8])
	filename := fmt.Sprintf("%s%s.json", types.VormaRouteManifestPrefix, hashStr)

	outPath := filepath.Join(v.Wave.GetStaticPublicOutDir(), filename)
	if err := os.WriteFile(outPath, manifestJSON, 0644); err != nil {
		return "", fmt.Errorf("write route manifest: %w", err)
	}

	return filename, nil
}

func generateRouteManifest(v *runtime.Vorma, nestedRouter *mux.NestedRouter) map[string]int {
	manifest := make(map[string]int)
	paths := v.UnsafeGetPaths()

	for _, p := range paths {
		hasServerLoader := 0
		if nestedRouter.HasTaskHandler(p.OriginalPattern) {
			hasServerLoader = 1
		}
		manifest[p.OriginalPattern] = hasServerLoader
	}

	return manifest
}
