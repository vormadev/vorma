package build_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vormadev/vorma/vormaruntime"
)

func TestBuildStage2PathsConformance(t *testing.T) {
	t.Run("BDC-STAGE2-001_BUILD-STAGE2-001_stage2_populates_client_entry_output_and_deps", func(t *testing.T) {
		tempRoot := t.TempDir()
		viteCmd := writeFakeViteCmd(t, tempRoot)
		fixture := newBuildFixture(t, &buildFixtureOptions{viteCmdPath: viteCmd})

		out, err := runBuildProbe(t, fixture, "--hook")
		if err != nil {
			t.Fatalf("prod hook failed: err=%v output=%s", err, out)
		}

		stageTwo := mustReadStageTwoPathsFile(t, fixture)
		if got, _ := stageTwo["clientEntryOut"].(string); got != "vorma_out_vite_entry.js" {
			t.Fatalf("expected clientEntryOut=%q, got %#v", "vorma_out_vite_entry.js", stageTwo["clientEntryOut"])
		}
		deps := stageStringSlice(t, stageTwo["clientEntryDeps"], "clientEntryDeps")
		if len(deps) == 0 {
			t.Fatalf("expected non-empty clientEntryDeps in stage-two output")
		}
		if !containsString(deps, "vorma_out_vite_root.js") {
			t.Fatalf("expected clientEntryDeps to include %q, got %v", "vorma_out_vite_root.js", deps)
		}
	})

	t.Run("BDC-STAGE2-002_BUILD-STAGE2-002_stage2_updates_route_entries_with_outPath_and_deps", func(t *testing.T) {
		tempRoot := t.TempDir()
		viteCmd := writeFakeViteCmd(t, tempRoot)
		fixture := newBuildFixture(t, &buildFixtureOptions{viteCmdPath: viteCmd})

		out, err := runBuildProbe(t, fixture, "--hook")
		if err != nil {
			t.Fatalf("prod hook failed: err=%v output=%s", err, out)
		}

		stageTwo := mustReadStageTwoPathsFile(t, fixture)
		paths := mustMapAny(t, stageTwo["paths"], "paths")

		root := mustMapAny(t, paths[""], "paths[\"\"]")
		home := mustMapAny(t, paths["/home"], "paths[\"/home\"]")
		def := mustMapAny(t, paths["/default-key"], "paths[\"/default-key\"]")

		if got, _ := root["outPath"].(string); got != "vorma_out_vite_root.js" {
			t.Fatalf("expected root outPath %q, got %#v", "vorma_out_vite_root.js", root["outPath"])
		}
		if got, _ := home["outPath"].(string); got != "vorma_out_vite_home.js" {
			t.Fatalf("expected /home outPath %q, got %#v", "vorma_out_vite_home.js", home["outPath"])
		}
		if got, _ := def["outPath"].(string); got != "vorma_out_vite_default.js" {
			t.Fatalf("expected /default-key outPath %q, got %#v", "vorma_out_vite_default.js", def["outPath"])
		}

		rootDeps := stageStringSlice(t, root["deps"], "root.deps")
		if !reflect.DeepEqual(rootDeps, []string{"vorma_out_vite_root.js"}) {
			t.Fatalf("expected root deps [vorma_out_vite_root.js], got %v", rootDeps)
		}
	})

	t.Run("BDC-STAGE2-003_BUILD-STAGE2-003_stage2_populates_dep_to_css_bundle_map_with_basenames", func(t *testing.T) {
		tempRoot := t.TempDir()
		viteCmd := writeFakeViteCmd(t, tempRoot)
		fixture := newBuildFixture(t, &buildFixtureOptions{viteCmdPath: viteCmd})

		out, err := runBuildProbe(t, fixture, "--hook")
		if err != nil {
			t.Fatalf("prod hook failed: err=%v output=%s", err, out)
		}

		stageTwo := mustReadStageTwoPathsFile(t, fixture)
		depToCSS := mustMapAny(t, stageTwo["depToCSSBundleMap"], "depToCSSBundleMap")
		raw, exists := depToCSS["vorma_out_vite_entry.js"]
		if !exists {
			t.Fatalf("expected depToCSSBundleMap to include key %q, got keys=%v", "vorma_out_vite_entry.js", mapKeys(depToCSS))
		}
		css := stageStringSlice(t, raw, "depToCSSBundleMap[vorma_out_vite_entry.js]")
		if !reflect.DeepEqual(css, []string{"vorma_out_vite_entry.css"}) {
			t.Fatalf("expected CSS basename mapping [vorma_out_vite_entry.css], got %v", css)
		}
	})

	t.Run("BDC-STAGE2-004_BUILD-STAGE2-004_build_id_is_stable_for_same_inputs_and_changes_when_inputs_change", func(t *testing.T) {
		tempRoot := t.TempDir()
		viteCmd := writeFakeViteCmd(t, tempRoot)
		fixture := newBuildFixture(t, &buildFixtureOptions{viteCmdPath: viteCmd})

		out1, err := runBuildProbe(t, fixture, "--hook")
		if err != nil {
			t.Fatalf("first prod hook failed: err=%v output=%s", err, out1)
		}
		stageTwoA := mustReadStageTwoPathsFile(t, fixture)
		buildIDA, _ := stageTwoA["buildID"].(string)
		if buildIDA == "" {
			t.Fatalf("expected non-empty buildID after first prod hook")
		}

		out2, err := runBuildProbe(t, fixture, "--hook")
		if err != nil {
			t.Fatalf("second prod hook failed: err=%v output=%s", err, out2)
		}
		stageTwoB := mustReadStageTwoPathsFile(t, fixture)
		buildIDB, _ := stageTwoB["buildID"].(string)
		if buildIDA != buildIDB {
			t.Fatalf("expected stable buildID for unchanged inputs, got first=%q second=%q", buildIDA, buildIDB)
		}

		templatePath := filepath.Join(fixture.root, "assets", "private", "index.html")
		mustWriteFile(t, templatePath, `<!doctype html><html><head>{{.VormaHeadEls}}<meta name="changed" content="1"></head><body><div id="{{.VormaRootID}}"></div>{{.VormaBodyScripts}}{{.VormaSSRScript}}</body></html>`)

		out3, err := runBuildProbe(t, fixture, "--hook")
		if err != nil {
			t.Fatalf("third prod hook failed: err=%v output=%s", err, out3)
		}
		stageTwoC := mustReadStageTwoPathsFile(t, fixture)
		buildIDC, _ := stageTwoC["buildID"].(string)
		if buildIDC == buildIDB {
			t.Fatalf("expected buildID to change after template content input changed; before=%q after=%q", buildIDB, buildIDC)
		}
	})
}

func mustReadStageTwoPathsFile(t *testing.T, fixture *buildFixture) map[string]any {
	t.Helper()
	stageTwoPath := filepath.Join(
		fixture.distDir,
		"static",
		"assets",
		"private",
		vormaruntime.VormaOutDirname,
		vormaruntime.VormaPathsStageTwoJSONFileName,
	)
	return mustReadJSONFileMap(t, stageTwoPath)
}

func stageStringSlice(t *testing.T, raw any, field string) []string {
	t.Helper()
	items, ok := raw.([]any)
	if !ok {
		t.Fatalf("expected %s as []any, got %#v", field, raw)
	}
	out := make([]string, 0, len(items))
	for i, item := range items {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("expected %s[%d] as string, got %#v", field, i, item)
		}
		out = append(out, s)
	}
	return out
}
