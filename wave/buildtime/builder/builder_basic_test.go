package builder

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave/waveartifacts"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveframework"

	"github.com/vormadev/vorma/internal/wavetest"
)

func newDiscardLoggerForBuilderBasicTests() *slog.Logger {
	return wavetest.NewDiscardLogger()
}

func newParsedConfigForBuilderBasicTestsAtRoot(
	root string,
) *waveconfig.ParsedConfig {
	return wavetest.NewParsedConfigAtRoot(root)
}

func ensureViteConfigForBuilderBasicTests(
	t *testing.T,
	config *waveconfig.ParsedConfig,
) {
	wavetest.EnsureViteConfig(t, config)
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
		t.Fatalf(
			"resolved go build entry path = %q, expected %q",
			resolvedEntryPath,
			"./backend/cmd/check",
		)
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
			t.Fatalf(
				"did not expect -tags=prod in dev build args: %#v",
				goBuildArguments,
			)
		}
	}
}

func TestBuildGoBuildCommand_ProdBuildUsesEmbeddedDistStaticByDefault(
	t *testing.T,
) {
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
		t.Fatalf(
			"expected -tags=prod in prod build args: %#v",
			goBuildArguments,
		)
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
		t.Fatalf(
			"expected overlay argument in go build args: %#v",
			goBuildArguments,
		)
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

func TestSetupDistDir_PreservesExistingPrivateOwnedOutputDirectories(
	t *testing.T,
) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())

	nonLegacyWaveOutDirectoryPath := filepath.Join(
		config.Dist.StaticPrivate(),
		waveartifacts.HashedOutputDirname,
	)
	legacyVormaOutDirectoryPath := filepath.Join(
		config.Dist.StaticPrivate(),
		"vorma_out",
	)
	currentWaveInternalDirectoryPath := filepath.Join(
		config.Dist.StaticPrivate(),
		waveartifacts.WaveInternalDirname,
	)
	currentVormaInternalDirectoryPath := filepath.Join(
		config.Dist.StaticPrivate(),
		"vorma_owned",
	)

	if makeDirectoryError := os.MkdirAll(
		nonLegacyWaveOutDirectoryPath,
		0o755,
	); makeDirectoryError != nil {
		t.Fatalf("create non-legacy wave_out directory: %v", makeDirectoryError)
	}
	if writeLegacyWaveOutFileError := os.WriteFile(
		filepath.Join(nonLegacyWaveOutDirectoryPath, "manifest.json"),
		[]byte("legacy-wave-out"),
		0o644,
	); writeLegacyWaveOutFileError != nil {
		t.Fatalf(
			"write non-legacy wave_out file: %v",
			writeLegacyWaveOutFileError,
		)
	}
	if makeDirectoryError := os.MkdirAll(
		legacyVormaOutDirectoryPath,
		0o755,
	); makeDirectoryError != nil {
		t.Fatalf("create legacy vorma_out directory: %v", makeDirectoryError)
	}
	if writeLegacyVormaOutFileError := os.WriteFile(
		filepath.Join(legacyVormaOutDirectoryPath, "paths.json"),
		[]byte("legacy-vorma-out"),
		0o644,
	); writeLegacyVormaOutFileError != nil {
		t.Fatalf(
			"write legacy vorma_out file: %v",
			writeLegacyVormaOutFileError,
		)
	}

	if makeDirectoryError := os.MkdirAll(
		currentWaveInternalDirectoryPath,
		0o755,
	); makeDirectoryError != nil {
		t.Fatalf(
			"create %q directory: %v",
			waveartifacts.WaveInternalDirname,
			makeDirectoryError,
		)
	}
	currentWaveInternalFilePath := filepath.Join(
		currentWaveInternalDirectoryPath,
		waveartifacts.ViteManifestFileName,
	)
	if writeCurrentWaveInternalFileError := os.WriteFile(
		currentWaveInternalFilePath,
		[]byte("current-wave-internal"),
		0o644,
	); writeCurrentWaveInternalFileError != nil {
		t.Fatalf(
			"write %q file: %v",
			waveartifacts.WaveInternalDirname,
			writeCurrentWaveInternalFileError,
		)
	}
	if makeDirectoryError := os.MkdirAll(
		currentVormaInternalDirectoryPath,
		0o755,
	); makeDirectoryError != nil {
		t.Fatalf("create vorma_owned directory: %v", makeDirectoryError)
	}
	currentVormaInternalFilePath := filepath.Join(
		currentVormaInternalDirectoryPath,
		"vorma_paths_stage_1.json",
	)
	if writeCurrentVormaInternalFileError := os.WriteFile(
		currentVormaInternalFilePath,
		[]byte("current-vorma-internal"),
		0o644,
	); writeCurrentVormaInternalFileError != nil {
		t.Fatalf(
			"write vorma_owned file: %v",
			writeCurrentVormaInternalFileError,
		)
	}

	if setupError := SetupDistDir(config); setupError != nil {
		t.Fatalf("SetupDistDir returned error: %v", setupError)
	}

	if _, statError := os.Stat(nonLegacyWaveOutDirectoryPath); statError != nil {
		t.Fatalf(
			"expected non-legacy wave_out directory to remain, stat error: %v",
			statError,
		)
	}
	if _, statError := os.Stat(legacyVormaOutDirectoryPath); statError != nil {
		t.Fatalf(
			"expected existing vorma_out directory to remain, stat error: %v",
			statError,
		)
	}

	if _, statError := os.Stat(currentWaveInternalFilePath); statError != nil {
		t.Fatalf(
			"expected current %q file to remain, stat error: %v",
			waveartifacts.WaveInternalDirname,
			statError,
		)
	}
	if _, statError := os.Stat(currentVormaInternalFilePath); statError != nil {
		t.Fatalf(
			"expected current vorma_owned file to remain, stat error: %v",
			statError,
		)
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

	criticalPath := filepath.Join(
		t.TempDir(),
		waveartifacts.CriticalCSSFileName,
	)
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

func TestBuilderListTrackedCriticalCSSImportPaths_ReturnsSortedPaths(
	t *testing.T,
) {
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

func TestBuilderCompileGo_LogsInfoStartAndDoneOnSuccess(t *testing.T) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	config.Core.ServerOnlyMode = true
	config.Core.MainAppEntry = "./testdata/compile_success_main"
	config.Dist.Root = config.Core.DistDir

	var compileLogBuffer bytes.Buffer
	builderForTest := NewBuilder(
		config,
		slog.New(
			slog.NewTextHandler(
				&compileLogBuffer,
				&slog.HandlerOptions{Level: slog.LevelInfo},
			),
		),
	)
	defer builderForTest.Close()

	if compileError := builderForTest.CompileGo(); compileError != nil {
		t.Fatalf("CompileGo returned error: %v", compileError)
	}

	compileLogOutput := compileLogBuffer.String()
	if !strings.Contains(compileLogOutput, "START compiling Go binary") {
		t.Fatalf(
			"expected CompileGo log output to include compile start, got: %q",
			compileLogOutput,
		)
	}
	if !strings.Contains(compileLogOutput, "DONE compiling Go binary") {
		t.Fatalf(
			"expected CompileGo log output to include compile completion, got: %q",
			compileLogOutput,
		)
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
	waveframework.StateForConfig(config).PrepareGoBuildOverlay = func() (*waveframework.GoBuildOverlay, error) {
		prepareOverlayCalled = true
		return &waveframework.GoBuildOverlay{
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
	waveframework.StateForConfig(config).PrepareGoBuildOverlay = func() (*waveframework.GoBuildOverlay, error) {
		return nil, expectedOverlayPreparationError
	}

	builderForTest := NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	compileError := builderForTest.CompileGo()
	if compileError == nil {
		t.Fatal(
			"expected CompileGo to fail when framework overlay preparation fails",
		)
	}
	if !strings.Contains(
		compileError.Error(),
		expectedOverlayPreparationError.Error(),
	) {
		t.Fatalf(
			"expected overlay preparation error to propagate, got: %v",
			compileError,
		)
	}
}

func TestBuilderCompileGo_CompileFailureIncludesOverlayCleanupFailure(
	t *testing.T,
) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	config.Core.ServerOnlyMode = true
	config.Core.MainAppEntry = "this/package/does/not/exist"
	config.Dist.Root = config.Core.DistDir

	waveframework.StateForConfig(config).PrepareGoBuildOverlay = func() (*waveframework.GoBuildOverlay, error) {
		return &waveframework.GoBuildOverlay{
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
		t.Fatalf(
			"expected compile failure text in error, got: %v",
			compileError,
		)
	}
	if !strings.Contains(
		compileError.Error(),
		"cleanup framework go build overlay failed",
	) {
		t.Fatalf(
			"expected cleanup failure text in compile error, got: %v",
			compileError,
		)
	}
}

func TestBuilderCompileGo_SuccessfulCompileStillReturnsOverlayCleanupFailure(
	t *testing.T,
) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	config.Core.ServerOnlyMode = true
	config.Core.MainAppEntry = "github.com/vormadev/vorma/internal/cmd/sum"
	config.Dist.Root = config.Core.DistDir

	waveframework.StateForConfig(config).PrepareGoBuildOverlay = func() (*waveframework.GoBuildOverlay, error) {
		return &waveframework.GoBuildOverlay{
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
