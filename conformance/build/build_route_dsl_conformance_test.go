package build_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/vormaruntime"
)

func TestBuildRouteDSLConformance(t *testing.T) {
	t.Run("BDC-ROUTE-001_BUILD-ROUTE-001_route_calls_imported_from_vorma_client_are_discovered", func(t *testing.T) {
		fixture := newBuildFixture(t, &buildFixtureOptions{
			routeDefs: `import { route } from "vorma/client";
route("", "./routes/root.tsx");
route("/account", "./routes/home.tsx");
route("/default-key", "./routes/default_key.tsx");
`,
		})

		out, err := runBuildProbe(t, fixture, "--hook", "--dev")
		if err != nil {
			t.Fatalf("build hook failed: err=%v output=%s", err, out)
		}

		stageOne := mustReadStageOnePathsFile(t, fixture)
		paths := mustMapAny(t, stageOne["paths"], "paths")
		for _, pattern := range []string{"", "/account", "/default-key"} {
			if _, exists := paths[pattern]; !exists {
				t.Fatalf("expected discovered route pattern %q in stage-one paths, got keys=%v", pattern, mapKeys(paths))
			}
		}
	})

	t.Run("BDC-ROUTE-002_BUILD-ROUTE-002_unresolvable_module_routes_are_ignored_with_warning", func(t *testing.T) {
		fixture := newBuildFixture(t, &buildFixtureOptions{
			routeDefs: `import { route } from "vorma/client";
const dynamicModulePath = makeDynamicPath();
function makeDynamicPath(){ return "./routes/home.tsx"; }
route("/resolved", "./routes/root.tsx");
route("/unresolved", dynamicModulePath);
`,
		})

		out, err := runBuildProbe(t, fixture, "--hook", "--dev")
		if err != nil {
			t.Fatalf("build hook failed: err=%v output=%s", err, out)
		}
		if !strings.Contains(out, `cannot be statically resolved`) {
			t.Fatalf("expected warning about unresolved route module expression in output, got %q", out)
		}

		stageOne := mustReadStageOnePathsFile(t, fixture)
		paths := mustMapAny(t, stageOne["paths"], "paths")
		if _, exists := paths["/resolved"]; !exists {
			t.Fatalf("expected resolved route to remain in paths")
		}
		if _, exists := paths["/unresolved"]; exists {
			t.Fatalf("expected unresolved route to be ignored, but it was included")
		}
	})

	t.Run("BDC-ROUTE-003_BUILD-ROUTE-003_missing_module_path_fails_build", func(t *testing.T) {
		fixture := newBuildFixture(t, &buildFixtureOptions{
			routeDefs: `import { route } from "vorma/client";
route("/missing", "./routes/does_not_exist.tsx");
`,
		})

		out, err := runBuildProbe(t, fixture, "--hook", "--dev")
		if err == nil {
			t.Fatalf("expected build hook to fail for missing module path, output=%s", out)
		}
		if !strings.Contains(out, "component module does not exist") {
			t.Fatalf("expected missing-module error message, got %q", out)
		}
	})

	t.Run("BDC-ROUTE-004_BUILD-ROUTE-004_export_key_defaults_to_default_when_omitted", func(t *testing.T) {
		fixture := newBuildFixture(t, &buildFixtureOptions{
			routeDefs: `import { route } from "vorma/client";
route("/no-key", "./routes/root.tsx");
`,
		})

		out, err := runBuildProbe(t, fixture, "--hook", "--dev")
		if err != nil {
			t.Fatalf("build hook failed: err=%v output=%s", err, out)
		}

		stageOne := mustReadStageOnePathsFile(t, fixture)
		paths := mustMapAny(t, stageOne["paths"], "paths")
		entry := mustMapAny(t, paths["/no-key"], "/no-key")
		if got, _ := entry["exportKey"].(string); got != "default" {
			t.Fatalf("expected omitted route export key to default to %q, got %#v", "default", entry["exportKey"])
		}
	})
}

func mustReadStageOnePathsFile(t *testing.T, fixture *buildFixture) map[string]any {
	t.Helper()
	stageOnePath := filepath.Join(
		fixture.distDir,
		"static",
		"assets",
		"private",
		vormaruntime.VormaOutDirname,
		vormaruntime.VormaPathsStageOneJSONFileName,
	)
	return mustReadJSONFileMap(t, stageOnePath)
}

func mustMapAny(t *testing.T, v any, field string) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected %s as map[string]any, got %#v", field, v)
	}
	return m
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func mustUnmarshalJSONMap(t *testing.T, b []byte, field string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal %s json: %v body=%q", field, err, string(b))
	}
	return m
}
