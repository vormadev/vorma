package tooling

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/vormadev/vorma/lab/jsonschema"
)

func TestBuild_FileOnlyModeSkipsHooks(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true

	markerPath := filepath.Join(root, "hook-marker.txt")
	cfg.Core.DevBuildHook = "printf 'user\\n' >> " + strconv.Quote(markerPath)
	cfg.FrameworkDevBuildHook = "printf 'framework\\n' >> " + strconv.Quote(markerPath)

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	err := builder.Build(BuildOpts{
		IsDev:        true,
		CompileGo:    false,
		IsRebuild:    false,
		FileOnlyMode: true,
	})
	if err != nil {
		t.Fatalf("Build(FileOnlyMode=true) returned error: %v", err)
	}

	if _, statErr := os.Stat(markerPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected hooks not to run in file-only mode, stat error: %v", statErr)
	}
}

func TestBuild_PropagatesHookFailure(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true
	cfg.Core.DevBuildHook = "false"

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	err := builder.Build(BuildOpts{
		IsDev:     true,
		CompileGo: false,
		IsRebuild: false,
	})
	if err == nil {
		t.Fatal("expected build to fail when dev build hook fails")
	}
	if !strings.Contains(err.Error(), "build hook failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuild_Success(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	err := builder.Build(BuildOpts{
		IsDev:     false,
		CompileGo: false,
		IsRebuild: false,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
}

func TestBuild_WritesConfigSchema(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	if err := builder.Build(BuildOpts{IsDev: false, CompileGo: false, IsRebuild: false}); err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	schemaPath := filepath.Join(cfg.Dist.Internal(), "schema.json")
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	for _, requiredSection := range []string{"Core", "Vite", "Watch"} {
		if _, ok := schema.Properties[requiredSection]; !ok {
			t.Fatalf("schema missing %q section", requiredSection)
		}
	}
}

func TestBuild_IncludesRegisteredSchemaSection(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()
	builder.RegisterSchemaSection(
		"CustomFramework",
		jsonschema.OptionalObject(jsonschema.Def{
			Properties: struct {
				Enabled jsonschema.Entry
			}{
				Enabled: jsonschema.OptionalBoolean(jsonschema.Def{Default: true}),
			},
		}),
	)

	if err := builder.Build(BuildOpts{IsDev: false, CompileGo: false, IsRebuild: false}); err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	schemaPath := filepath.Join(cfg.Dist.Internal(), "schema.json")
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	if _, ok := schema.Properties["CustomFramework"]; !ok {
		t.Fatal("schema missing CustomFramework section")
	}
}

func TestBuild_CompileGoFailureIsReported(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true
	cfg.Core.MainAppEntry = "this/package/does/not/exist"

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	err := builder.Build(BuildOpts{
		IsDev:     true,
		CompileGo: true,
		IsRebuild: false,
	})
	if err == nil {
		t.Fatal("expected build to fail when Go compilation target does not exist")
	}
	if !strings.Contains(err.Error(), "go compilation failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
