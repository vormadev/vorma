package tooling

import (
	"encoding/json"
	"github.com/vormadev/vorma/wave/tooling/builder"
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
	cfg.FrameworkDevBuildHook = "printf 'framework\\n' >> " + strconv.Quote(
		markerPath,
	)

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	err := builder.Build(toolingbuilder.BuildOpts{
		IsDev:        true,
		CompileGo:    false,
		IsRebuild:    false,
		FileOnlyMode: true,
	})
	if err != nil {
		t.Fatalf("Build(FileOnlyMode=true) returned error: %v", err)
	}

	if _, statErr := os.Stat(markerPath); !os.IsNotExist(statErr) {
		t.Fatalf(
			"expected hooks not to run in file-only mode, stat error: %v",
			statErr,
		)
	}
}

func TestBuild_PropagatesHookFailure(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true
	cfg.Core.DevBuildHook = "false"

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	err := builder.Build(toolingbuilder.BuildOpts{
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

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	err := builder.Build(toolingbuilder.BuildOpts{
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

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	if err := builder.Build(toolingbuilder.BuildOpts{IsDev: false, CompileGo: false, IsRebuild: false}); err != nil {
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

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()
	builder.RegisterSchemaSection(
		"CustomFramework",
		jsonschema.OptionalObject(jsonschema.Def{
			Properties: struct {
				Enabled jsonschema.Entry
			}{
				Enabled: jsonschema.OptionalBoolean(
					jsonschema.Def{Default: true},
				),
			},
		}),
	)

	if err := builder.Build(toolingbuilder.BuildOpts{IsDev: false, CompileGo: false, IsRebuild: false}); err != nil {
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

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	err := builder.Build(toolingbuilder.BuildOpts{
		IsDev:     true,
		CompileGo: true,
		IsRebuild: false,
	})
	if err == nil {
		t.Fatal(
			"expected build to fail when Go compilation target does not exist",
		)
	}
	if !strings.Contains(err.Error(), "go compilation failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessFiles_FullCleanupPreservesOnlyWaveDevLockFile(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	staticDir := cfg.Dist.Static()
	if mkdirErr := os.MkdirAll(staticDir, 0o755); mkdirErr != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", staticDir, mkdirErr)
	}

	lockFilePath := filepath.Join(staticDir, ".wave-dev.lock")
	if writeErr := os.WriteFile(lockFilePath, []byte("123"), 0o644); writeErr != nil {
		t.Fatalf("os.WriteFile(lock) error = %v", writeErr)
	}

	nonLockWavePrefixedFilePath := filepath.Join(
		staticDir,
		".wave-should-be-cleaned",
	)
	if writeErr := os.WriteFile(nonLockWavePrefixedFilePath, []byte("stale"), 0o644); writeErr != nil {
		t.Fatalf("os.WriteFile(non-lock wave-prefixed) error = %v", writeErr)
	}

	regularFilePath := filepath.Join(staticDir, "regular.txt")
	if writeErr := os.WriteFile(regularFilePath, []byte("stale"), 0o644); writeErr != nil {
		t.Fatalf("os.WriteFile(regular) error = %v", writeErr)
	}

	if processErr := builder.ProcessFiles(false, true); processErr != nil {
		t.Fatalf("processFiles(false, true) error = %v", processErr)
	}

	if _, statErr := os.Stat(lockFilePath); statErr != nil {
		t.Fatalf("expected lock file to be preserved, stat error = %v", statErr)
	}
	if _, statErr := os.Stat(nonLockWavePrefixedFilePath); !os.IsNotExist(
		statErr,
	) {
		t.Fatalf(
			"expected non-lock .wave-* file to be removed, stat error = %v",
			statErr,
		)
	}
	if _, statErr := os.Stat(regularFilePath); !os.IsNotExist(statErr) {
		t.Fatalf(
			"expected regular file to be removed, stat error = %v",
			statErr,
		)
	}
}
