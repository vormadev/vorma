package tooling

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/wave"
)

func TestBuilderConfigReturnsDefensiveCopy(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.FrameworkIgnoredPatterns = []string{"generated/**"}
	cfg.FrameworkSchemaExtensions = map[string]jsonschema.Entry{
		"Vorma": {
			Type: jsonschema.TypeObject,
		},
	}
	cfg.FrameworkRunBuildHook = func(context.Context, bool) error { return nil }
	cfg.FrameworkWatchPatterns = []wave.WatchedFile{
		{
			Pattern: "**/*.go",
			OnChangeHooks: []wave.OnChangeHook{
				{
					Exclude: []string{"vendor/**"},
				},
			},
		},
	}
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	configSnapshot := builder.Config()
	if configSnapshot == cfg {
		t.Fatal("expected Config() to return a defensive copy")
	}
	if configSnapshot.Core == cfg.Core {
		t.Fatal("expected Config() to deep-clone Core config")
	}

	configSnapshot.Core.MainAppEntry = "cmd/changed"
	configSnapshot.FrameworkIgnoredPatterns[0] = "changed/**"
	configSnapshot.FrameworkWatchPatterns[0].Pattern = "**/*.changed"
	configSnapshot.FrameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0] = "changed-vendor/**"

	if got := cfg.Core.MainAppEntry; got == "cmd/changed" {
		t.Fatalf("Config() snapshot mutated core main app entry: %q", got)
	}
	if got := cfg.FrameworkIgnoredPatterns[0]; got != "generated/**" {
		t.Fatalf("Config() snapshot mutated framework ignored pattern: %q", got)
	}
	if got := cfg.FrameworkWatchPatterns[0].Pattern; got != "**/*.go" {
		t.Fatalf("Config() snapshot mutated framework watch pattern: %q", got)
	}
	if got := cfg.FrameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0]; got != "vendor/**" {
		t.Fatalf("Config() snapshot mutated framework watch hook exclude: %q", got)
	}
	if configSnapshot.FrameworkSchemaExtensions != nil {
		t.Fatal("expected Config() snapshot to omit framework schema extensions")
	}
	if configSnapshot.FrameworkRunBuildHook != nil {
		t.Fatal("expected Config() snapshot to omit framework run build hook callback")
	}
}

func TestBuildGoBuildCommand_DevBuildOmitsProdTags(t *testing.T) {
	cmd := buildGoBuildCommand("dist/main", "./cmd/serve", true, "")

	for _, commandArgument := range cmd.Args {
		if strings.HasPrefix(commandArgument, "-tags=") {
			t.Fatalf("expected dev build command to omit tags, got args %#v", cmd.Args)
		}
	}
}

func TestBuildGoBuildCommand_ProdBuildUsesEmbeddedDistStaticByDefault(t *testing.T) {
	cmd := buildGoBuildCommand("dist/main", "./cmd/serve", false, "")

	if !containsCommandArgument(cmd.Args, "-tags=prod") {
		t.Fatalf("expected prod build tags to be -tags=prod, got args %#v", cmd.Args)
	}
}

func TestBuildGoBuildCommand_IncludesOverlayArgumentWhenProvided(t *testing.T) {
	cmd := buildGoBuildCommand(
		"dist/main",
		"./cmd/serve",
		true,
		"/tmp/vorma-overlay.json",
	)

	if !containsCommandArgument(cmd.Args, "-overlay=/tmp/vorma-overlay.json") {
		t.Fatalf("expected overlay argument, got args %#v", cmd.Args)
	}
}

func containsCommandArgument(commandArguments []string, expectedArgument string) bool {
	for _, commandArgument := range commandArguments {
		if commandArgument == expectedArgument {
			return true
		}
	}

	return false
}

func TestBuilderViteMethods_NoViteConfigured(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	if err := builder.ViteProdBuild(); err != nil {
		t.Fatalf("ViteProdBuild with nil Vite config returned error: %v", err)
	}

	ctx, err := builder.NewViteDevContext()
	if err != nil {
		t.Fatalf("NewViteDevContext with nil Vite config returned error: %v", err)
	}
	if ctx != nil {
		t.Fatalf("expected nil Vite dev context when Vite is disabled, got %#v", ctx)
	}
}

func TestBuilderProcessFilesOnly_ServerOnlyMode(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	if err := builder.ProcessFilesOnly(false, false); err != nil {
		t.Fatalf("ProcessFilesOnly returned error in server-only mode: %v", err)
	}

	requiredPaths := []string{
		cfg.Dist.Static(),
		cfg.Dist.Internal(),
	}
	for _, path := range requiredPaths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected dist path to exist after ProcessFilesOnly: %s (error: %v)", path, err)
		}
	}
}

func TestBuilderIsCSSFile_ReturnsTrueForTrackedCriticalAndNormalImports(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	criticalPath := filepath.Join(t.TempDir(), "critical.css")
	normalPath := filepath.Join(t.TempDir(), "normal.css")

	criticalAbs := criticalPath
	if resolved, err := filepath.Abs(criticalPath); err == nil {
		criticalAbs = resolved
	}
	normalAbs := normalPath
	if resolved, err := filepath.Abs(normalPath); err == nil {
		normalAbs = resolved
	}

	builder.css.criticalImports[criticalAbs] = struct{}{}
	builder.css.normalImports[normalAbs] = struct{}{}

	if !builder.IsCriticalCSSFile(criticalPath) || !builder.IsCSSFile(criticalPath) {
		t.Fatalf("expected critical path to be recognized as CSS file: %s", criticalPath)
	}
	if !builder.IsNormalCSSFile(normalPath) || !builder.IsCSSFile(normalPath) {
		t.Fatalf("expected normal path to be recognized as CSS file: %s", normalPath)
	}
}

func TestBuilderCompileGoOnly_PropagatesCompilationError(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true
	cfg.Core.MainAppEntry = "this/package/does/not/exist"
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	err := builder.CompileGoOnly(true)
	if err == nil {
		t.Fatal("expected CompileGoOnly to fail for missing package")
	}
	if !strings.Contains(err.Error(), "go build") {
		t.Fatalf("unexpected compile error: %v", err)
	}
}

func TestBuilderCompileGoOnly_UsesFrameworkOverlayPreparationAndCleanup(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true
	cfg.Core.MainAppEntry = "this/package/does/not/exist"
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	overlayConfigPath := filepath.Join(t.TempDir(), "go-overlay.json")
	if err := os.WriteFile(overlayConfigPath, []byte(`{"Replace":{}}`), 0o644); err != nil {
		t.Fatalf("write overlay config file: %v", err)
	}

	prepareOverlayCalled := false
	cleanupOverlayCalled := false
	cfg.FrameworkPrepareGoBuildOverlay = func() (*wave.GoBuildOverlay, error) {
		prepareOverlayCalled = true
		return &wave.GoBuildOverlay{
			OverlayConfigPath: overlayConfigPath,
			Cleanup: func() error {
				cleanupOverlayCalled = true
				return nil
			},
		}, nil
	}

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	err := builder.CompileGoOnly(true)
	if err == nil {
		t.Fatal("expected CompileGoOnly to fail for missing package")
	}
	if !prepareOverlayCalled {
		t.Fatal("expected framework go-build overlay preparation to run")
	}
	if !cleanupOverlayCalled {
		t.Fatal("expected framework go-build overlay cleanup to run")
	}
}

func TestBuilderViteProdBuild_ErrorIsReturned(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Vite = &wave.ViteConfig{
		JSPackageManagerBaseCmd: "command_that_does_not_exist_for_wave_tests",
	}
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	err := builder.ViteProdBuild()
	if err == nil {
		t.Fatal("expected ViteProdBuild to fail for invalid package manager command")
	}
}

func TestBuilderNewViteDevContext_WithViteEnabledReturnsContext(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Vite = &wave.ViteConfig{
		JSPackageManagerBaseCmd: "echo",
		DefaultPort:             5199,
	}
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	ctx, err := builder.NewViteDevContext()
	if err != nil {
		t.Fatalf("NewViteDevContext returned error: %v", err)
	}
	if ctx == nil {
		t.Fatal("expected non-nil Vite dev context when Vite is enabled")
	}

	ctx.Cleanup()
}
