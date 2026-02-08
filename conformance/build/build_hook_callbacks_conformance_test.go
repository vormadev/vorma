package build_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildHookCallbacksConformance(t *testing.T) {
	t.Run("BDC-HOOK-001_BUILD-HOOK-001_framework_hook_commands_match_expected_shape", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)
		dumpPath := filepath.Join(fixture.root, "probe_dump.json")

		out, err := runBuildProbeWithEnv(t, fixture, map[string]string{
			"BUILDPROBE_DUMP_CONFIG_JSON": dumpPath,
		}, "--hook", "--dev")
		if err != nil {
			t.Fatalf("build hook failed: err=%v output=%s", err, out)
		}

		dump := mustReadJSONFileMap(t, dumpPath)
		if got, _ := dump["frameworkDevBuildHook"].(string); got != "go run ./cmd/build/main.go --dev --hook" {
			t.Fatalf("expected framework dev hook command shape, got %q", got)
		}
		if got, _ := dump["frameworkProdBuildHook"].(string); got != "go run ./cmd/build/main.go --hook" {
			t.Fatalf("expected framework prod hook command shape, got %q", got)
		}
	})

	t.Run("BDC-HOOK-002_BUILD-HOOK-002_user_hook_ordering_runs_before_framework_hook", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)
		addFixtureCLIGoCommands(t, fixture)

		markerPath := filepath.Join(fixture.root, "hook_order.txt")
		mutateFixtureConfig(t, fixture, func(cfg map[string]any) {
			core, ok := cfg["Core"].(map[string]any)
			if !ok {
				t.Fatalf("expected Core config map, got %#v", cfg["Core"])
			}
			core["ProdBuildHook"] = `printf 'user\n' >> "$CLI_HOOK_MARKER"`
		})

		out, err := runBuildProbeWithEnv(
			t,
			fixture,
			map[string]string{"CLI_HOOK_MARKER": markerPath},
		)
		if err != nil {
			t.Fatalf("prod build failed: err=%v output=%s", err, out)
		}

		markerBytes, readErr := os.ReadFile(markerPath)
		if readErr != nil {
			t.Fatalf("read hook marker file: %v", readErr)
		}
		lines := strings.Split(strings.TrimSpace(string(markerBytes)), "\n")
		if len(lines) < 2 {
			t.Fatalf("expected at least user + framework marker lines, got %q", string(markerBytes))
		}
		if strings.TrimSpace(lines[0]) != "user" {
			t.Fatalf("expected user hook line first, got %q", lines[0])
		}
		if !strings.Contains(lines[1], "--hook") {
			t.Fatalf("expected framework hook line second with --hook marker, got %q", lines[1])
		}
	})
}
