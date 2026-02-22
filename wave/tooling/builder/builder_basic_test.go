package builder

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/wave"
)

type staticAssetDirsForTests = struct {
	Private string `json:"Private"`
	Public  string `json:"Public"`
}

type cssEntryFilesForTests = struct {
	Critical    string `json:"Critical,omitempty"`
	NonCritical string `json:"NonCritical,omitempty"`
}

func newDiscardLoggerForBuilderBasicTests() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newParsedConfigForBuilderBasicTestsAtRoot(root string) *wave.ParsedConfig {
	config := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      filepath.Join(root, "dist"),
			StaticAssetDirs: staticAssetDirsForTests{
				Public:  filepath.Join(root, "static", "public"),
				Private: filepath.Join(root, "static", "private"),
			},
		},
		Watch: &wave.WatchConfig{
			WatchRoot: root,
		},
	}
	config.Dist.Root = config.Core.DistDir
	return config
}

func ensureViteConfigForBuilderBasicTests(
	t *testing.T,
	config *wave.ParsedConfig,
) {
	t.Helper()
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if config.Vite != nil {
		return
	}

	configValue := reflect.ValueOf(config).Elem()
	viteField := configValue.FieldByName("Vite")
	if !viteField.IsValid() || !viteField.CanSet() || viteField.IsNil() == false {
		t.Fatal("expected settable Vite field")
	}
	viteField.Set(reflect.New(viteField.Type().Elem()))
}

func TestBuilderConfigReturnsDefensiveCopy(t *testing.T) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	config.FrameworkIgnoredPatterns = []string{"generated/**"}
	config.FrameworkSchemaExtensions = map[string]jsonschema.Entry{
		"Vorma": {
			Type: jsonschema.TypeObject,
		},
	}
	config.FrameworkRunBuildHook = func(context.Context, bool) error { return nil }
	config.FrameworkWatchPatterns = []wave.WatchedFile{
		{
			Pattern: "**/*.go",
			OnChangeHooks: []wave.OnChangeHook{
				{
					Exclude: []string{"vendor/**"},
				},
			},
		},
	}

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	configSnapshot := builderForTest.config()
	if configSnapshot == config {
		t.Fatal("expected config() to return a defensive copy")
	}
	if configSnapshot.Core == config.Core {
		t.Fatal("expected config() to deep-clone Core config")
	}

	configSnapshot.Core.MainAppEntry = "cmd/changed"
	configSnapshot.FrameworkIgnoredPatterns[0] = "changed/**"
	configSnapshot.FrameworkWatchPatterns[0].Pattern = "**/*.changed"
	configSnapshot.FrameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0] = "changed-vendor/**"

	if gotMainAppEntry := config.Core.MainAppEntry; gotMainAppEntry == "cmd/changed" {
		t.Fatalf(
			"config() snapshot mutated core main app entry: %q",
			gotMainAppEntry,
		)
	}
	if gotIgnoredPattern := config.FrameworkIgnoredPatterns[0]; gotIgnoredPattern != "generated/**" {
		t.Fatalf(
			"config() snapshot mutated framework ignored pattern: %q",
			gotIgnoredPattern,
		)
	}
	if gotWatchPattern := config.FrameworkWatchPatterns[0].Pattern; gotWatchPattern != "**/*.go" {
		t.Fatalf(
			"config() snapshot mutated framework watch pattern: %q",
			gotWatchPattern,
		)
	}
	if gotWatchHookExclude := config.FrameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0]; gotWatchHookExclude != "vendor/**" {
		t.Fatalf(
			"config() snapshot mutated framework watch hook exclude: %q",
			gotWatchHookExclude,
		)
	}
	if configSnapshot.FrameworkSchemaExtensions != nil {
		t.Fatal(
			"expected config() snapshot to omit framework schema extensions",
		)
	}
	if configSnapshot.FrameworkRunBuildHook != nil {
		t.Fatal(
			"expected config() snapshot to omit framework run build hook callback",
		)
	}
}

func TestBuilderClose_ReturnsNil(t *testing.T) {
	var nilBuilder *Builder
	if closeError := nilBuilder.Close(); closeError != nil {
		t.Fatalf("nil builder close returned error: %v", closeError)
	}

	builderForTest := NewBuilder(
		newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir()),
		newDiscardLoggerForBuilderBasicTests(),
	)
	if closeError := builderForTest.Close(); closeError != nil {
		t.Fatalf("builder close returned error: %v", closeError)
	}
}

func TestResolveGoBuildEntryPath_PrefixesRelativeExistingDirectoryWithDotSlash(
	t *testing.T,
) {
	originalWorkingDirectory, getWorkingDirectoryError := os.Getwd()
	if getWorkingDirectoryError != nil {
		t.Fatalf("resolve working directory: %v", getWorkingDirectoryError)
	}

	temporaryWorkingDirectory := t.TempDir()
	if makeDirectoryError := os.MkdirAll(
		filepath.Join(temporaryWorkingDirectory, "backend", "cmd", "check"),
		0o755,
	); makeDirectoryError != nil {
		t.Fatalf("create test main package directory: %v", makeDirectoryError)
	}

	if changeDirectoryError := os.Chdir(temporaryWorkingDirectory); changeDirectoryError != nil {
		t.Fatalf("change directory for test: %v", changeDirectoryError)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWorkingDirectory)
	})

	resolvedEntryPath := resolveGoBuildEntryPath("backend/cmd/check")
	if resolvedEntryPath != "./backend/cmd/check" {
		t.Fatalf("resolved go build entry path = %q, expected %q", resolvedEntryPath, "./backend/cmd/check")
	}
}

func TestBuildGoBuildCommand_DevBuildOmitsProdTags(t *testing.T) {
	goBuildArguments := buildGoBuildArguments(
		"/tmp/wave-binary",
		"backend/cmd/serve",
		true,
		"",
	)

	for _, goBuildArgument := range goBuildArguments {
		if goBuildArgument == "-tags=prod" {
			t.Fatalf("did not expect -tags=prod in dev build args: %#v", goBuildArguments)
		}
	}
}

func TestBuildGoBuildCommand_ProdBuildUsesEmbeddedDistStaticByDefault(t *testing.T) {
	goBuildArguments := buildGoBuildArguments(
		"/tmp/wave-binary",
		"backend/cmd/serve",
		false,
		"",
	)

	containsProdTag := false
	for _, goBuildArgument := range goBuildArguments {
		if goBuildArgument == "-tags=prod" {
			containsProdTag = true
			break
		}
	}
	if !containsProdTag {
		t.Fatalf("expected -tags=prod in prod build args: %#v", goBuildArguments)
	}
}

func TestBuildGoBuildCommand_IncludesOverlayArgumentWhenProvided(t *testing.T) {
	goBuildArguments := buildGoBuildArguments(
		"/tmp/wave-binary",
		"backend/cmd/serve",
		true,
		"/tmp/go-build-overlay.json",
	)

	containsOverlayArgument := false
	for _, goBuildArgument := range goBuildArguments {
		if goBuildArgument == "-overlay=/tmp/go-build-overlay.json" {
			containsOverlayArgument = true
			break
		}
	}
	if !containsOverlayArgument {
		t.Fatalf("expected overlay argument in go build args: %#v", goBuildArguments)
	}
}

func TestBuilderViteMethods_NoViteConfigured(t *testing.T) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	if viteProdBuildError := builderForTest.ViteProdBuild(); viteProdBuildError != nil {
		t.Fatalf(
			"ViteProdBuild with nil Vite config returned error: %v",
			viteProdBuildError,
		)
	}

	viteContext, viteContextError := builderForTest.NewViteDevContext()
	if viteContextError != nil {
		t.Fatalf(
			"NewViteDevContext with nil Vite config returned error: %v",
			viteContextError,
		)
	}
	if viteContext != nil {
		t.Fatalf(
			"expected nil Vite dev context when Vite is disabled, got %#v",
			viteContext,
		)
	}
}

func TestBuilderProcessFilesOnly_ServerOnlyMode(t *testing.T) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	config.Core.ServerOnlyMode = true

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	if processFilesError := builderForTest.processFilesOnly(false, false); processFilesError != nil {
		t.Fatalf(
			"processFilesOnly returned error in server-only mode: %v",
			processFilesError,
		)
	}

	requiredPaths := []string{
		config.Dist.Static(),
		config.Dist.Internal(),
	}
	for _, requiredPath := range requiredPaths {
		if _, statError := os.Stat(requiredPath); statError != nil {
			t.Fatalf(
				"expected dist path to exist after processFilesOnly: %s (error: %v)",
				requiredPath,
				statError,
			)
		}
	}
}

func TestBuilderIsCSSFile_ReturnsTrueForTrackedCriticalAndNormalImports(
	t *testing.T,
) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	criticalPath := filepath.Join(t.TempDir(), "critical.css")
	normalPath := filepath.Join(t.TempDir(), "normal.css")

	criticalAbsolutePath := criticalPath
	if resolvedCriticalPath, criticalPathError := filepath.Abs(criticalPath); criticalPathError == nil {
		criticalAbsolutePath = resolvedCriticalPath
	}
	normalAbsolutePath := normalPath
	if resolvedNormalPath, normalPathError := filepath.Abs(normalPath); normalPathError == nil {
		normalAbsolutePath = resolvedNormalPath
	}

	builderForTest.cssProcessor.SetTrackedCriticalCSSImportPaths(
		[]string{criticalAbsolutePath},
	)
	builderForTest.cssProcessor.SetTrackedNormalCSSImportPaths(
		[]string{normalAbsolutePath},
	)

	if !builderForTest.IsCriticalCSSFile(criticalPath) ||
		!builderForTest.isCSSFile(criticalPath) {
		t.Fatalf(
			"expected critical path to be recognized as CSS file: %s",
			criticalPath,
		)
	}
	if !builderForTest.IsNormalCSSFile(normalPath) ||
		!builderForTest.isCSSFile(normalPath) {
		t.Fatalf(
			"expected normal path to be recognized as CSS file: %s",
			normalPath,
		)
	}
}

func TestBuilderListTrackedCriticalCSSImportPaths_ReturnsSortedPaths(t *testing.T) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	firstPath := filepath.Join(t.TempDir(), "b.css")
	secondPath := filepath.Join(t.TempDir(), "a.css")
	builderForTest.cssProcessor.SetTrackedCriticalCSSImportPaths(
		[]string{firstPath, secondPath},
	)

	importPaths := builderForTest.cssProcessor.ListTrackedCriticalCSSImportPaths()
	if len(importPaths) != 2 {
		t.Fatalf("expected 2 tracked imports, got %d", len(importPaths))
	}
	if importPaths[0] > importPaths[1] {
		t.Fatalf("expected sorted import paths, got %#v", importPaths)
	}
}

func TestBuilderCompileGo_PropagatesCompilationError(t *testing.T) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	config.Core.ServerOnlyMode = true
	config.Core.MainAppEntry = "this/package/does/not/exist"
	config.Dist.Root = config.Core.DistDir

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	compileError := builderForTest.CompileGo()
	if compileError == nil {
		t.Fatal("expected CompileGo to fail for missing package")
	}
	if !strings.Contains(compileError.Error(), "compile go binary") {
		t.Fatalf("unexpected compile error: %v", compileError)
	}
}

func TestBuilderCompileGo_UsesFrameworkOverlayPreparationAndCleanup(
	t *testing.T,
) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	config.Core.ServerOnlyMode = true
	config.Core.MainAppEntry = "this/package/does/not/exist"
	config.Dist.Root = config.Core.DistDir

	overlayConfigPath := filepath.Join(t.TempDir(), "go-overlay.json")
	if writeOverlayError := os.WriteFile(
		overlayConfigPath,
		[]byte(`{"Replace":{}}`),
		0o644,
	); writeOverlayError != nil {
		t.Fatalf("write overlay config file: %v", writeOverlayError)
	}

	prepareOverlayCalled := false
	cleanupOverlayCalled := false
	config.FrameworkPrepareGoBuildOverlay = func() (*wave.GoBuildOverlay, error) {
		prepareOverlayCalled = true
		return &wave.GoBuildOverlay{
			OverlayConfigPath: overlayConfigPath,
			Cleanup: func() error {
				cleanupOverlayCalled = true
				return nil
			},
		}, nil
	}

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	compileError := builderForTest.CompileGo()
	if compileError == nil {
		t.Fatal("expected CompileGo to fail for missing package")
	}
	if !prepareOverlayCalled {
		t.Fatal("expected framework go-build overlay preparation to run")
	}
	if !cleanupOverlayCalled {
		t.Fatal("expected framework go-build overlay cleanup to run")
	}
}

func TestBuilderCompileGo_PropagatesOverlayPreparationError(t *testing.T) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	config.Core.ServerOnlyMode = true
	config.Dist.Root = config.Core.DistDir

	expectedOverlayPreparationError := errors.New("prepare overlay boom")
	config.FrameworkPrepareGoBuildOverlay = func() (*wave.GoBuildOverlay, error) {
		return nil, expectedOverlayPreparationError
	}

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	compileError := builderForTest.CompileGo()
	if compileError == nil {
		t.Fatal("expected CompileGo to fail when framework overlay preparation fails")
	}
	if !strings.Contains(
		compileError.Error(),
		expectedOverlayPreparationError.Error(),
	) {
		t.Fatalf("expected overlay preparation error to propagate, got: %v", compileError)
	}
}

func TestBuilderCompileGo_CompileFailureIncludesOverlayCleanupFailure(
	t *testing.T,
) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	config.Core.ServerOnlyMode = true
	config.Core.MainAppEntry = "this/package/does/not/exist"
	config.Dist.Root = config.Core.DistDir

	config.FrameworkPrepareGoBuildOverlay = func() (*wave.GoBuildOverlay, error) {
		return &wave.GoBuildOverlay{
			Cleanup: func() error {
				return errors.New("cleanup overlay boom")
			},
		}, nil
	}

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	compileError := builderForTest.CompileGo()
	if compileError == nil {
		t.Fatal("expected CompileGo to fail for missing package")
	}
	if !strings.Contains(compileError.Error(), "compile go binary") {
		t.Fatalf("expected compile failure text in error, got: %v", compileError)
	}
	if !strings.Contains(
		compileError.Error(),
		"cleanup framework go build overlay failed",
	) {
		t.Fatalf("expected cleanup failure text in compile error, got: %v", compileError)
	}
}

func TestBuilderCompileGo_SuccessfulCompileStillReturnsOverlayCleanupFailure(
	t *testing.T,
) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	config.Core.ServerOnlyMode = true
	config.Core.MainAppEntry = "github.com/vormadev/vorma/internal/cmd/sum"
	config.Dist.Root = config.Core.DistDir

	config.FrameworkPrepareGoBuildOverlay = func() (*wave.GoBuildOverlay, error) {
		return &wave.GoBuildOverlay{
			Cleanup: func() error {
				return errors.New("cleanup overlay boom")
			},
		}, nil
	}

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	compileError := builderForTest.CompileGo()
	if compileError == nil {
		t.Fatal("expected CompileGo to fail when overlay cleanup fails")
	}
	if !strings.Contains(
		compileError.Error(),
		"cleanup framework go build overlay",
	) {
		t.Fatalf("expected cleanup failure error, got: %v", compileError)
	}
	if _, statError := os.Stat(config.Dist.Binary()); statError != nil {
		t.Fatalf(
			"expected go binary to exist when compilation succeeds before cleanup failure, stat error: %v",
			statError,
		)
	}
}

func TestBuilderViteProdBuild_ErrorIsReturned(t *testing.T) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	ensureViteConfigForBuilderBasicTests(t, config)
	config.Vite.JSPackageManagerBaseCmd = "command_that_does_not_exist_for_wave_tests"
	config.Dist.Root = config.Core.DistDir

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	viteProdBuildError := builderForTest.ViteProdBuild()
	if viteProdBuildError == nil {
		t.Fatal(
			"expected ViteProdBuild to fail for invalid package manager command",
		)
	}
}

func TestBuilderNewViteDevContext_WithViteEnabledReturnsContext(t *testing.T) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	ensureViteConfigForBuilderBasicTests(t, config)
	config.Vite.JSPackageManagerBaseCmd = "echo"
	config.Vite.DefaultPort = 5199
	config.Dist.Root = config.Core.DistDir

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	viteContext, viteContextError := builderForTest.NewViteDevContext()
	if viteContextError != nil {
		t.Fatalf("NewViteDevContext returned error: %v", viteContextError)
	}
	if viteContext == nil {
		t.Fatal("expected non-nil Vite dev context when Vite is enabled")
	}

	viteContext.Cleanup()
}
