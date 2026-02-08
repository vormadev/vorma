package build_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/vormaruntime"
)

func TestBuildCleanConformance(t *testing.T) {
	t.Run("BDC-CLEAN-001_BUILD-CLEAN-001_cleanup_removes_vorma_generated_public_files", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)

		publicOutDir := filepath.Join(fixture.distDir, "static", "assets", "public")
		mustMkdirAll(t, publicOutDir)

		oldVite := filepath.Join(publicOutDir, vormaruntime.VormaVitePrehashedFilePrefix+"old.js")
		oldManifest := filepath.Join(publicOutDir, vormaruntime.VormaRouteManifestPrefix+"old.json")
		mustWriteFile(t, oldVite, "old")
		mustWriteFile(t, oldManifest, "{}")

		out, err := runBuildProbe(t, fixture, "--hook", "--dev")
		if err != nil {
			t.Fatalf("build hook failed: err=%v output=%s", err, out)
		}

		if _, err := os.Stat(oldVite); !os.IsNotExist(err) {
			t.Fatalf("expected old Vorma vite-prefixed file to be removed, stat err=%v", err)
		}
		if _, err := os.Stat(oldManifest); !os.IsNotExist(err) {
			t.Fatalf("expected old Vorma route-manifest-prefixed file to be removed, stat err=%v", err)
		}

		entries, err := os.ReadDir(publicOutDir)
		if err != nil {
			t.Fatalf("read public out dir: %v", err)
		}
		hasNewManifest := false
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), vormaruntime.VormaRouteManifestPrefix) && strings.HasSuffix(entry.Name(), ".json") {
				hasNewManifest = true
				break
			}
		}
		if !hasNewManifest {
			t.Fatalf("expected new route manifest file to be generated after cleanup")
		}
	})

	t.Run("BDC-CLEAN-002_BUILD-CLEAN-002_cleanup_tolerates_missing_static_public_dir", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)

		out, err := runBuildProbe(t, fixture, "--hook", "--dev")
		if err != nil {
			t.Fatalf("expected build hook to succeed with initially missing public dir: err=%v output=%s", err, out)
		}
	})
}
