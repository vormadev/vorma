package vormabuild

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/vormaruntime"
)

// writeRouteArtifacts writes all route-related artifacts to disk.
// Includes manifest, paths JSON, and TypeScript generation.
func writeRouteArtifacts(l *vormaruntime.LockedVorma) error {
	if err := writeAndSetRouteManifest(l); err != nil {
		return fmt.Errorf("write route manifest: %w", err)
	}

	if err := writePathsToDiskStageOne(l); err != nil {
		return fmt.Errorf("write paths JSON: %w", err)
	}

	if err := WriteGeneratedTS(l); err != nil {
		return fmt.Errorf("write generated TypeScript: %w", err)
	}

	return nil
}

func writeAndSetRouteManifest(l *vormaruntime.LockedVorma) error {
	v := l.Vorma()
	manifest := generateRouteManifest(l, v.LoadersRouter().NestedRouter)
	manifestFile, err := writeRouteManifestToDisk(v, manifest)
	if err != nil {
		return err
	}
	l.SetRouteManifestFile(manifestFile)
	return nil
}

func writeRouteManifestToDisk(v *vormaruntime.Vorma, manifest map[string]int) (string, error) {
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("marshal route manifest: %w", err)
	}

	filename := routeManifestFilename(manifestJSON)

	outPath := filepath.Join(v.Wave.GetStaticPublicOutDir(), filename)
	if err := os.WriteFile(outPath, manifestJSON, 0644); err != nil {
		return "", fmt.Errorf("write route manifest: %w", err)
	}

	return filename, nil
}

func generateRouteManifest(l *vormaruntime.LockedVorma, nestedRouter *mux.NestedRouter) map[string]int {
	manifest := make(map[string]int)
	paths := l.GetPaths()

	for _, currentPath := range paths {
		manifest[currentPath.OriginalPattern] = routeManifestServerLoaderFlag(nestedRouter, currentPath.OriginalPattern)
	}

	return manifest
}

func routeManifestFilename(manifestJSON []byte) string {
	hash := cryptoutil.Sha256Hash(manifestJSON)
	hashStr := base64.RawURLEncoding.EncodeToString(hash[:8])
	return fmt.Sprintf("%s%s.json", vormaruntime.VormaRouteManifestPrefix, hashStr)
}

func routeManifestServerLoaderFlag(nestedRouter *mux.NestedRouter, pattern string) int {
	if nestedRouter.HasTaskHandler(pattern) {
		return 1
	}
	return 0
}
