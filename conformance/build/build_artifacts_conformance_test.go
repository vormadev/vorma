package build_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/vormaruntime"
)

func TestBuildArtifactsConformance(t *testing.T) {
	t.Run("BDC-ART-001_BUILD-ART-001_stage1_paths_file_exists_with_stage_one", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)

		out, err := runBuildProbe(t, fixture, "--hook", "--dev")
		if err != nil {
			t.Fatalf("build hook failed: err=%v output=%s", err, out)
		}

		stageOnePath := filepath.Join(
			fixture.distDir,
			"static",
			"assets",
			"private",
			vormaruntime.VormaOutDirname,
			vormaruntime.VormaPathsStageOneJSONFileName,
		)
		stageOne := mustReadJSONFileMap(t, stageOnePath)
		if got, _ := stageOne["stage"].(string); got != "one" {
			t.Fatalf("expected stage one file to have stage=%q, got %#v", "one", stageOne["stage"])
		}
	})

	t.Run("BDC-ART-002_BUILD-ART-002_stage2_paths_file_exists_with_stage_two", func(t *testing.T) {
		tempRoot := t.TempDir()
		viteCmd := writeFakeViteCmd(t, tempRoot)
		fixture := newBuildFixture(t, &buildFixtureOptions{viteCmdPath: viteCmd})

		out, err := runBuildProbe(t, fixture, "--hook")
		if err != nil {
			t.Fatalf("prod hook failed: err=%v output=%s", err, out)
		}

		stageTwoPath := filepath.Join(
			fixture.distDir,
			"static",
			"assets",
			"private",
			vormaruntime.VormaOutDirname,
			vormaruntime.VormaPathsStageTwoJSONFileName,
		)
		stageTwo := mustReadJSONFileMap(t, stageTwoPath)
		if got, _ := stageTwo["stage"].(string); got != "two" {
			t.Fatalf("expected stage two file to have stage=%q, got %#v", "two", stageTwo["stage"])
		}
	})

	t.Run("BDC-ART-003_BUILD-ART-003_route_manifest_filename_and_shape", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)

		out, err := runBuildProbe(t, fixture, "--hook", "--dev")
		if err != nil {
			t.Fatalf("build hook failed: err=%v output=%s", err, out)
		}

		publicOutDir := filepath.Join(fixture.distDir, "static", "assets", "public")
		manifestPath := findSingleFileWithPrefix(t, publicOutDir, vormaruntime.VormaRouteManifestPrefix)
		base := filepath.Base(manifestPath)
		if !strings.HasSuffix(base, ".json") {
			t.Fatalf("expected route manifest suffix .json, got %q", base)
		}

		data, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("read route manifest: %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatalf("unmarshal route manifest: %v body=%q", err, string(data))
		}
		if len(payload) == 0 {
			t.Fatalf("expected non-empty route manifest payload")
		}
		for pattern, raw := range payload {
			num, ok := raw.(float64)
			if !ok {
				t.Fatalf("expected numeric manifest value for pattern %q, got %#v", pattern, raw)
			}
			if int(num) != 0 && int(num) != 1 {
				t.Fatalf("expected manifest value 0|1 for pattern %q, got %v", pattern, raw)
			}
		}
	})

	t.Run("BDC-ART-004_BUILD-ART-004_generated_ts_index_exists", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)

		out, err := runBuildProbe(t, fixture, "--hook", "--dev")
		if err != nil {
			t.Fatalf("build hook failed: err=%v output=%s", err, out)
		}

		indexPath := filepath.Join(fixture.tsGenDir, "index.ts")
		if _, err := os.Stat(indexPath); err != nil {
			t.Fatalf("expected generated TS index file at %s: %v", indexPath, err)
		}
	})

	t.Run("BDC-ART-005_BUILD-ART-005_generated_filemap_outputs_exist", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)

		out, err := runBuildProbe(t, fixture, "--hook", "--dev")
		if err != nil {
			t.Fatalf("build hook failed: err=%v output=%s", err, out)
		}

		for _, name := range []string{"filemap.ts", "filemap.json"} {
			target := filepath.Join(fixture.tsGenDir, name)
			if _, err := os.Stat(target); err != nil {
				t.Fatalf("expected generated file %s: %v", target, err)
			}
		}
	})

	t.Run("BDC-ART-006_BUILD-ART-006_generated_ts_write_is_skipped_when_content_unchanged", func(t *testing.T) {
		fixture := newBuildFixture(t, nil)

		firstOut, err := runBuildProbe(t, fixture, "--hook", "--dev")
		if err != nil {
			t.Fatalf("first build hook failed: err=%v output=%s", err, firstOut)
		}

		indexPath := filepath.Join(fixture.tsGenDir, "index.ts")
		before := mustStatModTime(t, indexPath)

		secondOut, err := runBuildProbe(t, fixture, "--hook", "--dev")
		if err != nil {
			t.Fatalf("second build hook failed: err=%v output=%s", err, secondOut)
		}

		after := mustStatModTime(t, indexPath)
		if !before.Equal(after) {
			t.Fatalf("expected generated TS index mtime to remain unchanged when content is unchanged, before=%s after=%s", before, after)
		}
	})
}
