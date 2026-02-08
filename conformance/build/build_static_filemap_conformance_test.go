package build_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildStaticFilemapConformance(t *testing.T) {
	staticPath := filepath.Join(repoRoot(t), "wave", "tooling", "static.go")
	staticSrc, staticSet, staticAST := mustParseGoSourceFile(t, staticPath)

	t.Run("BDC-STATIC-001_BUILD-STATIC-001_missing_source_emits_empty_filemap_outputs", func(t *testing.T) {
		processBody := mustFunctionBodySource(t, staticSrc, staticSet, staticAST, "processStaticFiles")
		for _, expected := range []string{
			"if _, err := os.Stat(opts.srcDir); os.IsNotExist(err) {",
			"if err := b.saveFileMap(wave.FileMap{}, opts.gobPath); err != nil {",
			"if opts.isPublic {",
			"return b.savePublicFileMapJS(wave.FileMap{})",
		} {
			if !strings.Contains(processBody, expected) {
				t.Fatalf("expected missing-source behavior to include %q", expected)
			}
		}
	})

	t.Run("BDC-STATIC-002_BUILD-STATIC-002_prehashed_and_nohash_paths_preserve_dist_names", func(t *testing.T) {
		processBody := mustFunctionBodySource(t, staticSrc, staticSet, staticAST, "processStaticFiles")
		requireOrderedSubstrings(
			t,
			processBody,
			[]string{
				"prehashedPrefix := wave.PrehashedDirname + \"/\"",
				"nohashPrefix := wave.NohashDirname + \"/\"",
				"if strings.HasPrefix(relPath, prehashedPrefix) {",
				"relPath = strings.TrimPrefix(relPath, prehashedPrefix)",
				"} else if strings.HasPrefix(relPath, nohashPrefix) {",
				"relPath = strings.TrimPrefix(relPath, nohashPrefix)",
			},
		)
	})

	t.Run("BDC-STATIC-003_BUILD-STATIC-003_dist_name_strategy_matches_public_private_and_prehash_modes", func(t *testing.T) {
		processFileBody := mustFunctionBodySource(t, staticSrc, staticSet, staticAST, "processFile")
		requireOrderedSubstrings(
			t,
			processFileBody,
			[]string{
				"if fi.prehash {",
				"distName = fi.relPath",
				"} else if !opts.hashOutput {",
				"distName = fi.relPath",
				"} else {",
				"distName = contentHash",
			},
		)
	})

	t.Run("BDC-STATIC-004_BUILD-STATIC-004_granular_delta_skips_unchanged_and_removes_stale_outputs", func(t *testing.T) {
		processFileBody := mustFunctionBodySource(t, staticSrc, staticSet, staticAST, "processFile")
		for _, expected := range []string{
			"if oldMap != nil {",
			"if oldVal.(wave.FileVal).ContentHash == contentHash {",
			"return nil",
		} {
			if !strings.Contains(processFileBody, expected) {
				t.Fatalf("expected unchanged-content skip behavior to include %q", expected)
			}
		}

		processBody := mustFunctionBodySource(t, staticSrc, staticSet, staticAST, "processStaticFiles")
		for _, expected := range []string{
			"if opts.granular && oldMap != nil {",
			"if newVal, exists := newMap.Load(key); !exists || newVal.(wave.FileVal).DistName != oldVal.DistName {",
			"os.Remove(filepath.Join(opts.distDir, oldVal.DistName))",
		} {
			if !strings.Contains(processBody, expected) {
				t.Fatalf("expected stale-output cleanup behavior to include %q", expected)
			}
		}
	})

	t.Run("BDC-STATIC-005_BUILD-STATIC-005_public_filemap_js_ref_outputs_are_rotated_and_written_atomically", func(t *testing.T) {
		saveBody := mustFunctionBodySource(t, staticSrc, staticSet, staticAST, "savePublicFileMapJS")
		for _, expected := range []string{
			"content := fmt.Sprintf(\"export const wavePublicFileMap = %s;\", string(jsonBytes))",
			"oldFiles, err := filepath.Glob(filepath.Join(publicDir, wave.FileMapJSGlobPattern))",
			"if err := writeFileAtomicBytes(refPath, []byte(hashedName)); err != nil {",
			"return writeFileAtomicBytes(filepath.Join(publicDir, hashedName), []byte(content))",
		} {
			if !strings.Contains(saveBody, expected) {
				t.Fatalf("expected public filemap output contract to include %q", expected)
			}
		}
	})

	t.Run("BDC-STATIC-006_BUILD-STATIC-006_public_filemap_ts_json_outputs_are_deterministic_and_dual_written", func(t *testing.T) {
		tsBody := mustFunctionBodySource(t, staticSrc, staticSet, staticAST, "WritePublicFileMapTS")
		for _, expected := range []string{
			"sort.Strings(keys)",
			`sb.WriteString("export const staticPublicAssetMap = {\n")`,
			"if err := b.writePublicFileMapJSON(outDir, fm); err != nil {",
		} {
			if !strings.Contains(tsBody, expected) {
				t.Fatalf("expected TS/JSON filemap contract to include %q", expected)
			}
		}
	})

	t.Run("BDC-STATIC-007_BUILD-STATIC-007_atomic_write_uses_temp_file_and_rename_with_failure_cleanup", func(t *testing.T) {
		atomicBody := mustFunctionBodySource(t, staticSrc, staticSet, staticAST, "writeFileAtomic")
		for _, expected := range []string{
			"tmpFile, err := os.CreateTemp(dir, \".tmp-*\")",
			"defer func() {",
			"if !success {",
			"os.Remove(tmpPath)",
			"if err := os.Rename(tmpPath, path); err != nil {",
		} {
			if !strings.Contains(atomicBody, expected) {
				t.Fatalf("expected atomic write contract to include %q", expected)
			}
		}
	})
}
