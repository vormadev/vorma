package build_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSchemaConformance(t *testing.T) {
	schemaPath := filepath.Join(repoRoot(t), "wave", "tooling", "schema.go")
	schemaSrc, schemaSet, schemaAST := mustParseGoSourceFile(t, schemaPath)

	t.Run("BDC-SCHEMA-001_BUILD-SCHEMA-001_core_required_and_static_asset_dirs_conditionally_required", func(t *testing.T) {
		writeBody := mustFunctionBodySource(t, schemaSrc, schemaSet, schemaAST, "writeConfigSchema")
		if !strings.Contains(writeBody, "Required:    []string{\"Core\"}") {
			t.Fatalf("expected top-level schema to require Core")
		}

		if !strings.Contains(schemaSrc, "\"ServerOnlyMode\": map[string]any{\"const\": true}") {
			t.Fatalf("expected server-only conditional guard for static assets")
		}
		if !strings.Contains(schemaSrc, "\"required\": []string{\"StaticAssetDirs\"}") {
			t.Fatalf("expected conditional requirement for Core.StaticAssetDirs")
		}
	})

	t.Run("BDC-SCHEMA-002_BUILD-SCHEMA-002_framework_schema_extensions_merge_into_top_level_properties", func(t *testing.T) {
		writeBody := mustFunctionBodySource(t, schemaSrc, schemaSet, schemaAST, "writeConfigSchema")
		for _, expected := range []string{
			"if len(b.cfg.FrameworkSchemaExtensions) > 0 {",
			"props := schema.Properties.(map[string]jsonschema.Entry)",
			"maps.Copy(props, b.cfg.FrameworkSchemaExtensions)",
		} {
			if !strings.Contains(writeBody, expected) {
				t.Fatalf("expected schema extension merge behavior to include %q", expected)
			}
		}
	})

	t.Run("BDC-SCHEMA-003_BUILD-SCHEMA-003_schema_output_path_and_pretty_json_formatting_are_stable", func(t *testing.T) {
		writeBody := mustFunctionBodySource(t, schemaSrc, schemaSet, schemaAST, "writeConfigSchema")
		for _, expected := range []string{
			"jsonBytes, err := json.MarshalIndent(schema, \"\", \"\\t\")",
			"jsonBytes = append(jsonBytes, '\\n')",
			"target := filepath.Join(b.cfg.Dist.Internal(), \"schema.json\")",
			"if err = os.WriteFile(target, jsonBytes, 0644); err != nil {",
		} {
			if !strings.Contains(writeBody, expected) {
				t.Fatalf("expected schema write contract to include %q", expected)
			}
		}
	})
}
