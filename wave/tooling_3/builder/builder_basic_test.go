package builder_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling_3/builder"
)

func newDiscardLoggerForBuilderBasicTests() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newParsedConfigForBuilderBasicTestsAtRoot(root string) *wave.ParsedConfig {
	config := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      filepath.Join(root, "dist"),
			StaticAssetDirs: wave.StaticAssetDirs{
				Public:  filepath.Join(root, "static", "public"),
				Private: filepath.Join(root, "static", "private"),
			},
		},
		Watch: &wave.WatchConfig{
			WatchRoot: root,
		},
	}
	config.Dist = wave.DistLayout{Root: config.Core.DistDir}
	return config
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

	builderForTest := builder.NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	configSnapshot := builderForTest.Config()
	if configSnapshot == config {
		t.Fatal("expected Config() to return a defensive copy")
	}
	if configSnapshot.Core == config.Core {
		t.Fatal("expected Config() to deep-clone Core config")
	}

	configSnapshot.Core.MainAppEntry = "cmd/changed"
	configSnapshot.FrameworkIgnoredPatterns[0] = "changed/**"
	configSnapshot.FrameworkWatchPatterns[0].Pattern = "**/*.changed"
	configSnapshot.FrameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0] = "changed-vendor/**"

	if gotMainAppEntry := config.Core.MainAppEntry; gotMainAppEntry == "cmd/changed" {
		t.Fatalf(
			"Config() snapshot mutated core main app entry: %q",
			gotMainAppEntry,
		)
	}
	if gotIgnoredPattern := config.FrameworkIgnoredPatterns[0]; gotIgnoredPattern != "generated/**" {
		t.Fatalf(
			"Config() snapshot mutated framework ignored pattern: %q",
			gotIgnoredPattern,
		)
	}
	if gotWatchPattern := config.FrameworkWatchPatterns[0].Pattern; gotWatchPattern != "**/*.go" {
		t.Fatalf(
			"Config() snapshot mutated framework watch pattern: %q",
			gotWatchPattern,
		)
	}
	if gotWatchHookExclude := config.FrameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0]; gotWatchHookExclude != "vendor/**" {
		t.Fatalf(
			"Config() snapshot mutated framework watch hook exclude: %q",
			gotWatchHookExclude,
		)
	}
	if configSnapshot.FrameworkSchemaExtensions != nil {
		t.Fatal(
			"expected Config() snapshot to omit framework schema extensions",
		)
	}
	if configSnapshot.FrameworkRunBuildHook != nil {
		t.Fatal(
			"expected Config() snapshot to omit framework run build hook callback",
		)
	}
}

func TestBuildGoBuildCommand_DevBuildOmitsProdTags(t *testing.T) {
	command := builder.BuildGoBuildCommand(
		"dist/main",
		"./cmd/serve",
		true,
		"",
	)

	for _, commandArgument := range command.Args {
		if strings.HasPrefix(commandArgument, "-tags=") {
			t.Fatalf(
				"expected dev build command to omit tags, got args %#v",
				command.Args,
			)
		}
	}
}

func TestBuildGoBuildCommand_ProdBuildUsesEmbeddedDistStaticByDefault(
	t *testing.T,
) {
	command := builder.BuildGoBuildCommand(
		"dist/main",
		"./cmd/serve",
		false,
		"",
	)

	if !containsCommandArgument(command.Args, "-tags=prod") {
		t.Fatalf(
			"expected prod build tags to be -tags=prod, got args %#v",
			command.Args,
		)
	}
}

func TestBuildGoBuildCommand_IncludesOverlayArgumentWhenProvided(t *testing.T) {
	command := builder.BuildGoBuildCommand(
		"dist/main",
		"./cmd/serve",
		true,
		"/tmp/vorma-overlay.json",
	)

	if !containsCommandArgument(
		command.Args,
		"-overlay=/tmp/vorma-overlay.json",
	) {
		t.Fatalf("expected overlay argument, got args %#v", command.Args)
	}
}

func TestBuilderViteMethods_NoViteConfigured(t *testing.T) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	builderForTest := builder.NewBuilder(
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

	builderForTest := builder.NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	if processFilesError := builderForTest.ProcessFilesOnly(false, false); processFilesError != nil {
		t.Fatalf(
			"ProcessFilesOnly returned error in server-only mode: %v",
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
				"expected dist path to exist after ProcessFilesOnly: %s (error: %v)",
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
	builderForTest := builder.NewBuilder(
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

	builderForTest.SetTrackedCriticalCSSImportPaths([]string{criticalAbsolutePath})
	builderForTest.SetTrackedNormalCSSImportPaths([]string{normalAbsolutePath})

	if !builderForTest.IsCriticalCSSFile(criticalPath) ||
		!builderForTest.IsCSSFile(criticalPath) {
		t.Fatalf("expected critical path to be recognized as CSS file: %s", criticalPath)
	}
	if !builderForTest.IsNormalCSSFile(normalPath) ||
		!builderForTest.IsCSSFile(normalPath) {
		t.Fatalf("expected normal path to be recognized as CSS file: %s", normalPath)
	}
}

func TestBuilderCompileGoOnly_PropagatesCompilationError(t *testing.T) {
	config := newParsedConfigForBuilderBasicTestsAtRoot(t.TempDir())
	config.Core.ServerOnlyMode = true
	config.Core.MainAppEntry = "this/package/does/not/exist"
	config.Dist = wave.DistLayout{Root: config.Core.DistDir}

	builderForTest := builder.NewBuilder(
		config,
		newDiscardLoggerForBuilderBasicTests(),
	)
	defer builderForTest.Close()

	compileError := builderForTest.CompileGoOnly(true)
	if compileError == nil {
		t.Fatal("expected CompileGoOnly to fail for missing package")
	}
	if !strings.Contains(compileError.Error(), "go build") {
		t.Fatalf("unexpected compile error: %v", compileError)
	}
}

func containsCommandArgument(
	commandArguments []string,
	expectedArgument string,
) bool {
	for _, commandArgument := range commandArguments {
		if commandArgument == expectedArgument {
			return true
		}
	}

	return false
}
