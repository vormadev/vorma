package tooling

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestBuilderConfig(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	if builder.Config() != cfg {
		t.Fatal("expected Config() to return original parsed config pointer")
	}
}

func TestBuildGoBuildCommand_DevBuildOmitsProdTags(t *testing.T) {
	cmd := buildGoBuildCommand("dist/main", "./cmd/serve", true, false)

	for _, commandArgument := range cmd.Args {
		if strings.HasPrefix(commandArgument, "-tags=") {
			t.Fatalf("expected dev build command to omit tags, got args %#v", cmd.Args)
		}
	}
}

func TestBuildGoBuildCommand_ProdBuildUsesEmbeddedDistStaticByDefault(t *testing.T) {
	cmd := buildGoBuildCommand("dist/main", "./cmd/serve", false, false)

	if !containsCommandArgument(cmd.Args, "-tags=prod") {
		t.Fatalf("expected prod build tags to be -tags=prod, got args %#v", cmd.Args)
	}
}

func TestBuildGoBuildCommand_ProdBuildCanUseFilesystemDistStatic(t *testing.T) {
	cmd := buildGoBuildCommand("dist/main", "./cmd/serve", false, true)

	if !containsCommandArgument(cmd.Args, "-tags=prod,wave_dist_static_from_disk") {
		t.Fatalf(
			"expected prod build tags to include wave_dist_static_from_disk, got args %#v",
			cmd.Args,
		)
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
