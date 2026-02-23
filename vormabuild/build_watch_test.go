package vormabuild

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/wave"
)

func TestRouteDefinitionWatchPatterns_ReturnNilWhenRouteDefsPatternsMissing(
	t *testing.T,
) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	app.Config.ClientRouteDefinitionPatterns = nil

	patterns := routeDefinitionWatchPatterns(app)
	if len(patterns) != 0 {
		t.Fatalf(
			"expected no watch patterns when route definitions patterns are empty, got %#v",
			patterns,
		)
	}
}

func TestRouteDefinitionWatchPatterns_PreservesInputOrderForValidPatterns(
	t *testing.T,
) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	app.Config.ClientRouteDefinitionPatterns = []string{
		"frontend/src/routes/alpha.vorma.routes.ts",
		"frontend/src/routes/beta.vorma.routes.ts",
	}

	patterns := routeDefinitionWatchPatterns(app)
	if len(patterns) != 2 {
		t.Fatalf(
			"len(patterns) = %d, want %d (%#v)",
			len(patterns),
			2,
			patterns,
		)
	}
	expectedAlphaPattern := normalizeFrameworkWatchPatternPath(
		"frontend/src/routes/alpha.vorma.routes.ts",
	)
	if patterns[0].Pattern != expectedAlphaPattern {
		t.Fatalf(
			"patterns[0].Pattern = %q, want %q",
			patterns[0].Pattern,
			expectedAlphaPattern,
		)
	}
	expectedBetaPattern := normalizeFrameworkWatchPatternPath(
		"frontend/src/routes/beta.vorma.routes.ts",
	)
	if patterns[1].Pattern != expectedBetaPattern {
		t.Fatalf(
			"patterns[1].Pattern = %q, want %q",
			patterns[1].Pattern,
			expectedBetaPattern,
		)
	}
}

func TestHTMLTemplateWatchPattern_ReturnsNilWhenTemplateLocationMissing(
	t *testing.T,
) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	app.Config.HTMLTemplateLocation = ""

	pattern := htmlTemplateWatchPattern(app)
	if pattern != nil {
		t.Fatalf(
			"expected no watch pattern when html template location is empty, got %#v",
			pattern,
		)
	}
}

func TestRouteDefinitionsOnChangeCallback_ReturnsErrorWhenNotInDevMode(
	t *testing.T,
) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	callback := routeDefinitionsOnChangeCallback(app)
	action, err := callback(&wave.HookContext{AppStoppedForBatch: false})
	if err == nil {
		t.Fatal("expected callback to return error when app is not in dev mode")
	}
	if !strings.Contains(err.Error(), "only be called in dev mode") {
		t.Fatalf("error = %q, expected non-dev-mode context", err)
	}
	if action != nil {
		t.Fatalf("expected nil action when callback errors, got %#v", action)
	}
}

func TestWatchReloadCallback_WrapsPreReloadActionErrorWithTriggerContext(
	t *testing.T,
) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	expectedErr := errors.New("pre-reload action failed")
	callback := watchReloadCallback(
		app,
		"/reload-routes",
		"reload endpoint failed",
		"test-trigger",
		func(*vormaruntime.Vorma) error {
			return expectedErr
		},
		nil,
	)

	action, err := callback(&wave.HookContext{AppStoppedForBatch: false})
	if err == nil {
		t.Fatal("expected callback to return wrapped pre-reload error")
	}
	if action != nil {
		t.Fatalf("expected nil action on pre-reload failure, got %#v", action)
	}
	if !strings.Contains(
		err.Error(),
		`run pre-reload action for trigger "test-trigger"`,
	) {
		t.Fatalf("error = %q, expected trigger context", err)
	}
	if !errors.Is(err, expectedErr) {
		t.Fatalf("error = %v, expected wrapped pre-reload error", err)
	}
}

func TestGetDefaultWatchPatterns_SkipsMissingOptionalPatterns(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	app.Config.ClientRouteDefinitionPatterns = nil
	app.Config.HTMLTemplateLocation = ""

	patterns := getDefaultWatchPatterns(app)
	if len(patterns) != 1 {
		t.Fatalf(
			"len(patterns) = %d, want %d when optional patterns are missing",
			len(patterns),
			1,
		)
	}
	if patterns[0].Pattern != "**/*.go" {
		t.Fatalf("pattern[0] = %q, want %q", patterns[0].Pattern, "**/*.go")
	}
}

func TestInjectDefaultWatchPatterns_IsIdempotent(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	parsedCfg := app.Wave.BuildtimeParsedConfig()
	injectDefaultWatchPatternsInConfig(parsedCfg, app)
	injectDefaultWatchPatternsInConfig(parsedCfg, app)

	if len(parsedCfg.FrameworkWatchPatterns) != 3 {
		t.Fatalf(
			"len(FrameworkWatchPatterns) = %d, want %d",
			len(parsedCfg.FrameworkWatchPatterns),
			3,
		)
	}

	templatePath := filepath.Join(
		app.Wave.PrivateStaticDir(),
		app.Config.HTMLTemplateLocation,
	)
	expectedPatternCounts := map[string]int{
		normalizeFrameworkWatchPatternPath(templatePath): 1,
		"**/*.go": 1,
	}
	for _, routeDefinitionPattern := range app.Config.ClientRouteDefinitionPatterns {
		expectedPatternCounts[normalizeFrameworkWatchPatternPath(routeDefinitionPattern)] = 1
	}
	assertFrameworkWatchPatternCounts(
		t,
		parsedCfg.FrameworkWatchPatterns,
		expectedPatternCounts,
	)

	expectedIgnoredPaths := []string{
		filepath.Join(app.Config.TSGenOutDir, wave.GeneratedTSFileName),
		filepath.Join(app.Config.TSGenOutDir, wave.PublicFileMapTSName),
		filepath.Join(app.Config.TSGenOutDir, wave.PublicFileMapJSONName),
	}
	assertIgnoredPatternCounts(
		t,
		parsedCfg.FrameworkIgnoredPatterns,
		expectedIgnoredPaths,
		1,
	)

	if parsedCfg.FrameworkPublicFileMapOutDir != app.Config.TSGenOutDir {
		t.Fatalf(
			"FrameworkPublicFileMapOutDir = %q, want %q",
			parsedCfg.FrameworkPublicFileMapOutDir,
			app.Config.TSGenOutDir,
		)
	}
}

func TestInjectDefaultWatchPatterns_PreservesExistingUserConfiguration(
	t *testing.T,
) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	parsedCfg := app.Wave.BuildtimeParsedConfig()

	customGoWatchPattern := wave.WatchedFile{
		Pattern: "**/*.go",
		OnChangeHooks: []wave.OnChangeHook{{
			Cmd:    "CustomGoHook",
			Timing: wave.OnChangeStrategyConcurrent,
		}},
	}
	parsedCfg.FrameworkWatchPatterns = append(
		parsedCfg.FrameworkWatchPatterns,
		customGoWatchPattern,
	)

	existingOutDir := "custom/framework/public-map"
	parsedCfg.FrameworkPublicFileMapOutDir = existingOutDir

	existingIgnoredPattern := filepath.Join(
		app.Config.TSGenOutDir,
		wave.GeneratedTSFileName,
	)
	parsedCfg.FrameworkIgnoredPatterns = append(
		parsedCfg.FrameworkIgnoredPatterns,
		existingIgnoredPattern,
	)

	injectDefaultWatchPatternsInConfig(parsedCfg, app)

	if parsedCfg.FrameworkPublicFileMapOutDir != existingOutDir {
		t.Fatalf(
			"FrameworkPublicFileMapOutDir = %q, want %q",
			parsedCfg.FrameworkPublicFileMapOutDir,
			existingOutDir,
		)
	}

	if len(parsedCfg.FrameworkWatchPatterns) != 3 {
		t.Fatalf(
			"len(FrameworkWatchPatterns) = %d, want %d",
			len(parsedCfg.FrameworkWatchPatterns),
			3,
		)
	}

	var observedGoPattern *wave.WatchedFile
	for idx := range parsedCfg.FrameworkWatchPatterns {
		currentPattern := &parsedCfg.FrameworkWatchPatterns[idx]
		if currentPattern.Pattern == "**/*.go" {
			if observedGoPattern != nil {
				t.Fatal("expected exactly one go watch pattern")
			}
			observedGoPattern = currentPattern
		}
	}
	if observedGoPattern == nil {
		t.Fatal("missing go watch pattern")
	}
	if len(observedGoPattern.OnChangeHooks) != 1 ||
		observedGoPattern.OnChangeHooks[0].Cmd != "CustomGoHook" {
		t.Fatalf(
			"go watch pattern hooks = %#v, expected existing user-defined hooks to be preserved",
			observedGoPattern.OnChangeHooks,
		)
	}

	expectedIgnoredPaths := []string{
		filepath.Join(app.Config.TSGenOutDir, wave.GeneratedTSFileName),
		filepath.Join(app.Config.TSGenOutDir, wave.PublicFileMapTSName),
		filepath.Join(app.Config.TSGenOutDir, wave.PublicFileMapJSONName),
	}
	assertIgnoredPatternCounts(
		t,
		parsedCfg.FrameworkIgnoredPatterns,
		expectedIgnoredPaths,
		1,
	)
}

func TestInjectDefaultWatchPatterns_DoesNotDuplicateSemanticallyEquivalentRoutePattern(
	t *testing.T,
) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	parsedCfg := app.Wave.BuildtimeParsedConfig()

	existingRoutePattern := app.Config.ClientRouteDefinitionPatterns[0]
	parsedCfg.FrameworkWatchPatterns = append(
		parsedCfg.FrameworkWatchPatterns,
		wave.WatchedFile{
			Pattern: existingRoutePattern,
			OnChangeHooks: []wave.OnChangeHook{{
				Cmd:    "ExistingRouteHook",
				Timing: wave.OnChangeStrategyConcurrent,
			}},
		},
	)

	injectDefaultWatchPatternsInConfig(parsedCfg, app)

	if len(parsedCfg.FrameworkWatchPatterns) != 3 {
		t.Fatalf(
			"len(FrameworkWatchPatterns) = %d, want %d",
			len(parsedCfg.FrameworkWatchPatterns),
			3,
		)
	}

	normalizedRoutePattern := normalizeFrameworkWatchPatternPath(
		existingRoutePattern,
	)
	normalizedRoutePatternCount := 0
	for _, currentPattern := range parsedCfg.FrameworkWatchPatterns {
		if normalizeFrameworkWatchPatternPath(
			currentPattern.Pattern,
		) == normalizedRoutePattern {
			normalizedRoutePatternCount++
		}
	}
	if normalizedRoutePatternCount != 1 {
		t.Fatalf(
			"normalized route pattern count = %d, want %d",
			normalizedRoutePatternCount,
			1,
		)
	}

	var observedRoutePattern *wave.WatchedFile
	for idx := range parsedCfg.FrameworkWatchPatterns {
		if parsedCfg.FrameworkWatchPatterns[idx].Pattern == existingRoutePattern {
			observedRoutePattern = &parsedCfg.FrameworkWatchPatterns[idx]
			break
		}
	}
	if observedRoutePattern == nil {
		t.Fatalf(
			"expected existing route pattern %q to be preserved",
			existingRoutePattern,
		)
	}
	if len(observedRoutePattern.OnChangeHooks) != 1 ||
		observedRoutePattern.OnChangeHooks[0].Cmd != "ExistingRouteHook" {
		t.Fatalf(
			"existing route pattern hooks = %#v, expected existing hooks to be preserved",
			observedRoutePattern.OnChangeHooks,
		)
	}
}

func TestInjectDefaultWatchPatterns_MatchesRouteAndTemplateWhenWatchRootIsAncestor(
	t *testing.T,
) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	originalWorkingDirectory, workingDirectoryError := os.Getwd()
	if workingDirectoryError != nil {
		t.Fatalf("read current working directory: %v", workingDirectoryError)
	}
	if chdirError := os.Chdir(fixture.rootDir); chdirError != nil {
		t.Fatalf("chdir into fixture root %q: %v", fixture.rootDir, chdirError)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWorkingDirectory)
	})

	parsedCfg := app.Wave.BuildtimeParsedConfig()
	if parsedCfg.Watch == nil {
		parsedCfg.Watch = &wave.WatchConfig{}
	}
	parsedCfg.Watch.WatchRoot = filepath.Dir(fixture.rootDir)
	injectDefaultWatchPatternsInConfig(parsedCfg, app)

	templatePattern := normalizeFrameworkWatchPatternPath(
		filepath.Join(
			app.Wave.PrivateStaticDir(),
			app.Config.HTMLTemplateLocation,
		),
	)
	routePattern := normalizeFrameworkWatchPatternPath(
		app.Config.ClientRouteDefinitionPatterns[0],
	)
	assertFrameworkWatchPatternCounts(
		t,
		parsedCfg.FrameworkWatchPatterns,
		map[string]int{
			routePattern:    1,
			templatePattern: 1,
			"**/*.go":       1,
		},
	)

	var routeWatchPattern *wave.WatchedFile
	var templateWatchPattern *wave.WatchedFile
	for index := range parsedCfg.FrameworkWatchPatterns {
		pattern := &parsedCfg.FrameworkWatchPatterns[index]
		if pattern.Pattern == routePattern {
			routeWatchPattern = pattern
		}
		if pattern.Pattern == templatePattern {
			templateWatchPattern = pattern
		}
	}
	if routeWatchPattern == nil {
		t.Fatalf("expected route watch pattern %q", routePattern)
	}
	if !routeWatchPattern.RunOnChangeOnly {
		t.Fatalf(
			"expected route watch pattern RunOnChangeOnly=true, got %#v",
			routeWatchPattern,
		)
	}
	if templateWatchPattern == nil {
		t.Fatalf("expected template watch pattern %q", templatePattern)
	}
	if templateWatchPattern.RunOnChangeOnly {
		t.Fatalf(
			"expected template watch pattern RunOnChangeOnly=false, got %#v",
			templateWatchPattern,
		)
	}
}

func assertFrameworkWatchPatternCounts(
	t *testing.T,
	patterns []wave.WatchedFile,
	expectedCounts map[string]int,
) {
	t.Helper()

	actualCounts := make(map[string]int, len(patterns))
	for _, pattern := range patterns {
		actualCounts[pattern.Pattern]++
	}

	for expectedPattern, expectedCount := range expectedCounts {
		actualCount := actualCounts[expectedPattern]
		if actualCount != expectedCount {
			t.Fatalf(
				"watch pattern count for %q = %d, want %d",
				expectedPattern,
				actualCount,
				expectedCount,
			)
		}
	}
}

func assertIgnoredPatternCounts(
	t *testing.T,
	ignoredPatterns []string,
	targetPatterns []string,
	expectedCount int,
) {
	t.Helper()

	for _, targetPattern := range targetPatterns {
		actualCount := 0
		for _, ignoredPattern := range ignoredPatterns {
			if ignoredPattern == targetPattern {
				actualCount++
			}
		}
		if actualCount != expectedCount {
			t.Fatalf(
				"ignored pattern count for %q = %d, want %d",
				targetPattern,
				actualCount,
				expectedCount,
			)
		}
	}
}

func TestInjectGeneratedOutputPathsForDefaultWatchPatterns_DoesNotOverrideExistingOutputDir(
	t *testing.T,
) {
	cfg := &wave.ParsedConfig{
		FrameworkPublicFileMapOutDir: "existing/filemap/out",
	}

	injectGeneratedOutputPathsForDefaultWatchPatterns(
		cfg,
		"frontend/src/vorma.gen",
	)

	if cfg.FrameworkPublicFileMapOutDir != "existing/filemap/out" {
		t.Fatalf(
			"FrameworkPublicFileMapOutDir = %q, want %q",
			cfg.FrameworkPublicFileMapOutDir,
			"existing/filemap/out",
		)
	}
}

func TestRunOnChangeOnlyWatchPattern_UsesCallbackOnly(t *testing.T) {
	callbackInvoked := false
	callback := func(ctx *wave.HookContext) (*wave.RefreshAction, error) {
		callbackInvoked = true
		if ctx == nil {
			t.Fatal("expected hook context to be non-nil")
		}
		return &wave.RefreshAction{ReloadBrowser: true}, nil
	}

	pattern := runOnChangeOnlyWatchPattern(
		"frontend/src/vorma.routes.ts",
		callback,
		true,
	)
	if !pattern.RunOnChangeOnly {
		t.Fatal("expected RunOnChangeOnly to be true")
	}
	if !pattern.SkipRebuildingNotification {
		t.Fatal("expected SkipRebuildingNotification to be true")
	}
	if len(pattern.OnChangeHooks) != 1 {
		t.Fatalf("len(OnChangeHooks) = %d, want 1", len(pattern.OnChangeHooks))
	}
	if pattern.OnChangeHooks[0].Callback == nil {
		t.Fatal("expected callback hook to be set")
	}
	if pattern.OnChangeHooks[0].Cmd != "" {
		t.Fatalf(
			"expected callback-only hook cmd to be empty, got %q",
			pattern.OnChangeHooks[0].Cmd,
		)
	}

	_, err := pattern.OnChangeHooks[0].Callback(
		&wave.HookContext{AppStoppedForBatch: false},
	)
	if err != nil {
		t.Fatalf("callback returned error: %v", err)
	}
	if !callbackInvoked {
		t.Fatal("expected callback to be invoked")
	}
}

func TestGoFilesWatchPattern_UsesCombinedDevBuildHookCommands(t *testing.T) {
	pattern := goFilesWatchPattern()

	if pattern.Pattern != "**/*.go" {
		t.Fatalf("Pattern = %q, want %q", pattern.Pattern, "**/*.go")
	}
	if pattern.RunOnChangeOnly {
		t.Fatal("expected go watch pattern to allow rebuild notifications")
	}
	if pattern.SkipRebuildingNotification {
		t.Fatal("expected go watch pattern to keep rebuild notifications")
	}
	if len(pattern.OnChangeHooks) != 1 {
		t.Fatalf("len(OnChangeHooks) = %d, want 1", len(pattern.OnChangeHooks))
	}

	hook := pattern.OnChangeHooks[0]
	if hook.Cmd != "" {
		t.Fatalf("Cmd = %q, want empty", hook.Cmd)
	}
	if !hook.RunCombinedDevBuildHookCommands {
		t.Fatal("expected RunCombinedDevBuildHookCommands=true")
	}
	if hook.Timing != wave.OnChangeStrategyConcurrent {
		t.Fatalf(
			"Timing = %q, want %q",
			hook.Timing,
			wave.OnChangeStrategyConcurrent,
		)
	}
	if hook.Callback != nil {
		t.Fatal("expected go watch pattern hook callback to be nil")
	}
}

func TestShouldInjectDefaultWatchPatterns(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	if !shouldInjectDefaultWatchPatterns(app) {
		t.Fatal("expected defaults to be injected when IncludeDefaults is nil")
	}

	includeDefaults := false
	app.Config.IncludeDefaults = &includeDefaults
	if shouldInjectDefaultWatchPatterns(app) {
		t.Fatal("expected defaults to be skipped when IncludeDefaults is false")
	}

	includeDefaults = true
	if !shouldInjectDefaultWatchPatterns(app) {
		t.Fatal("expected defaults to be injected when IncludeDefaults is true")
	}
}

func TestRouteDefinitionWatchPatterns_UseConfiguredRoutePattern(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	app.Config.ClientRouteDefinitionPatterns = []string{
		"frontend/src/custom.routes.ts",
	}

	patterns := routeDefinitionWatchPatterns(app)
	if len(patterns) != 1 {
		t.Fatalf("len(patterns) = %d, want 1", len(patterns))
	}

	pattern := patterns[0]
	expectedPattern := normalizeFrameworkWatchPatternPath(
		"frontend/src/custom.routes.ts",
	)
	if pattern.Pattern != expectedPattern {
		t.Fatalf(
			"Pattern = %q, want %q",
			pattern.Pattern,
			expectedPattern,
		)
	}
	if !pattern.RunOnChangeOnly {
		t.Fatal(
			"expected route definitions watch pattern to be RunOnChangeOnly",
		)
	}
	if !pattern.SkipRebuildingNotification {
		t.Fatal(
			"expected route definitions watch pattern to skip rebuild notifications",
		)
	}
	if len(pattern.OnChangeHooks) != 1 ||
		pattern.OnChangeHooks[0].Callback == nil {
		t.Fatalf(
			"OnChangeHooks = %#v, expected one callback hook",
			pattern.OnChangeHooks,
		)
	}
}

func TestHTMLTemplateWatchPattern_UsesPrivateStaticDirPrefix(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	app.Config.HTMLTemplateLocation = "templates/custom.entry.go.html"

	pattern := htmlTemplateWatchPattern(app)
	if pattern == nil {
		t.Fatal("expected html template watch pattern")
	}

	expectedPattern := filepath.Join(
		app.Wave.PrivateStaticDir(),
		"templates/custom.entry.go.html",
	)
	expectedPattern = normalizeFrameworkWatchPatternPath(expectedPattern)
	if pattern.Pattern != expectedPattern {
		t.Fatalf("Pattern = %q, want %q", pattern.Pattern, expectedPattern)
	}
	if !filepath.IsAbs(pattern.Pattern) {
		t.Fatalf(
			"expected absolute template watch pattern, got %q",
			pattern.Pattern,
		)
	}
	if pattern.RunOnChangeOnly {
		t.Fatal(
			"expected html template watch pattern not to be RunOnChangeOnly",
		)
	}
	if pattern.SkipRebuildingNotification {
		t.Fatal(
			"expected html template watch pattern not to skip rebuild notifications",
		)
	}
	if len(pattern.OnChangeHooks) != 1 ||
		pattern.OnChangeHooks[0].Callback == nil {
		t.Fatalf(
			"OnChangeHooks = %#v, expected one callback hook",
			pattern.OnChangeHooks,
		)
	}
	if pattern.OnChangeHooks[0].Timing != wave.OnChangeStrategyPost {
		t.Fatalf(
			"OnChangeHooks[0].Timing = %q, want %q",
			pattern.OnChangeHooks[0].Timing,
			wave.OnChangeStrategyPost,
		)
	}
}

func TestIsGeneratedRouteManifestFilename_PrefixOnlyIsNotManifest(
	t *testing.T,
) {
	if isGeneratedRouteManifestFilename(vormaruntime.VormaRouteManifestPrefix) {
		t.Fatalf(
			"isGeneratedRouteManifestFilename(%q) = true, want false",
			vormaruntime.VormaRouteManifestPrefix,
		)
	}
}
