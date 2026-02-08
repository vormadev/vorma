package build_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/vormaruntime"
)

func TestBuildFastRebuildConformance(t *testing.T) {
	t.Run("BDC-FAST-001_BUILD-FAST-001_fast_rebuild_fails_outside_dev_mode", func(t *testing.T) {
		tempRoot := t.TempDir()
		viteCmd := writeFakeViteCmd(t, tempRoot)
		fixture := newBuildFixture(t, &buildFixtureOptions{viteCmdPath: viteCmd})
		callbackDump := filepath.Join(fixture.root, "callback_dump.json")

		out, err := runBuildProbeWithEnv(t, fixture, map[string]string{
			"BUILDPROBE_RUN_CALLBACK_PATTERN": "frontend/src/vorma.routes.ts",
			"BUILDPROBE_CALLBACK_DUMP_JSON":   callbackDump,
		}, "--hook")
		if err != nil {
			t.Fatalf("prod hook build failed: err=%v output=%s", err, out)
		}

		payload := mustReadJSONFileMap(t, callbackDump)
		errMsg, _ := payload["error"].(string)
		if !strings.Contains(errMsg, "rebuildRoutesOnly should only be called in dev mode") {
			t.Fatalf("expected fast rebuild guard error outside dev mode, got %#v", payload["error"])
		}
	})

	t.Run("BDC-FAST-002_BUILD-FAST-002_fast_rebuild_updates_build_id_and_rewrites_route_artifacts", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)
		callbackDump := filepath.Join(fixture.root, "callback_dump.json")

		out, err := runBuildProbeWithEnv(t, fixture, map[string]string{
			"BUILDPROBE_RUN_CALLBACK_PATTERN": "frontend/src/vorma.routes.ts",
			"BUILDPROBE_CALLBACK_DUMP_JSON":   callbackDump,
			"PORT":                            "18080",
			"WAVE_PORT_HAS_BEEN_SET":          "true",
		}, "--hook", "--dev")
		if err != nil {
			t.Fatalf("dev hook build failed: err=%v output=%s", err, out)
		}

		stageOne := mustReadStageOnePathsFile(t, fixture)
		buildID, _ := stageOne["buildID"].(string)
		if !strings.HasPrefix(buildID, "dev_fast_") {
			t.Fatalf("expected fast rebuild to assign dev_fast_* build id, got %q", buildID)
		}

		publicOutDir := filepath.Join(fixture.distDir, "static", "assets", "public")
		manifestPath := findSingleFileWithPrefix(t, publicOutDir, vormaruntime.VormaRouteManifestPrefix)
		if _, statErr := os.Stat(manifestPath); statErr != nil {
			t.Fatalf("expected route manifest to be rewritten after fast rebuild: %v", statErr)
		}

		if _, statErr := os.Stat(filepath.Join(fixture.tsGenDir, "index.ts")); statErr != nil {
			t.Fatalf("expected generated TS index.ts to be rewritten during fast rebuild: %v", statErr)
		}

		payload := mustReadJSONFileMap(t, callbackDump)
		if payload["action"] == nil {
			t.Fatalf("expected callback to return a refresh action payload, got %#v", payload)
		}
		action := mustAnyMap(t, payload["action"], "action")
		if got, _ := action["triggerRestart"].(bool); !got {
			t.Fatalf("expected fallback refresh action to request restart when reload endpoint is unreachable, got %#v", action)
		}
		if got, _ := action["recompileGo"].(bool); got {
			t.Fatalf("expected fallback refresh action RecompileGo=false, got %#v", action)
		}
	})
}
