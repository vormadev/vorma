package devreload

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimeconfig"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/internal/wavetest"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
	"github.com/vormadev/vorma/wave/waveframework"
	"github.com/vormadev/vorma/wave/wavewatch"
)

func parseMutatedAppVormaConfigForBuildWatchTests(
	tb testing.TB,
	app *vormaruntime.Vorma,
	mutate func(*vormaruntime.VormaConfigJSON),
) (vormaruntime.VormaConfig, error) {
	tb.Helper()
	if app == nil || app.Wave == nil {
		return nil, errors.New("app with Wave runtime is required")
	}
	rawConfig := struct {
		Vorma vormaruntime.VormaConfigJSON `json:"Vorma"`
	}{}
	if unmarshalError := json.Unmarshal(
		app.Wave.RawConfigJSON(),
		&rawConfig,
	); unmarshalError != nil {
		return nil, unmarshalError
	}
	if mutate != nil {
		mutate(&rawConfig.Vorma)
	}
	mutatedPayload, marshalError := json.Marshal(rawConfig)
	if marshalError != nil {
		return nil, marshalError
	}
	return runtimeconfig.ParseVormaConfigJSON(mutatedPayload, app.Wave.ParsedConfig())
}

func TestRouteDefinitionWatchPatterns_ReturnsParseErrorWhenRouteDefsPatternsMissing(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	_, parseError := parseMutatedAppVormaConfigForBuildWatchTests(
		t,
		app,
		func(config *vormaruntime.VormaConfigJSON) {
			config.ClientRouteDefinitionPatterns = nil
		},
	)
	if parseError == nil {
		t.Fatal("expected parse error when route definition patterns are missing")
	}
	if !strings.Contains(
		parseError.Error(),
		"Vorma.ClientRouteDefinitionPatterns is required",
	) {
		t.Fatalf("error = %q, expected required-pattern parse error", parseError)
	}
}

func TestRouteDefinitionWatchPatterns_PreservesInputOrderForValidPatterns(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	testkit.MustMutateAppVormaConfig(
		t,
		app,
		func(config *vormaruntime.VormaConfigJSON) {
			config.ClientRouteDefinitionPatterns = []string{
				"frontend/src/routes/alpha.vorma.routes.ts",
				"frontend/src/routes/beta.vorma.routes.ts",
			}
		},
	)

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

func TestHTMLTemplateWatchPattern_ReturnsParseErrorWhenTemplateLocationMissing(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	_, parseError := parseMutatedAppVormaConfigForBuildWatchTests(
		t,
		app,
		func(config *vormaruntime.VormaConfigJSON) {
			config.HTMLTemplateLocation = ""
		},
	)
	if parseError == nil {
		t.Fatal("expected parse error when html template location is empty")
	}
	if !strings.Contains(
		parseError.Error(),
		"Vorma.HTMLTemplateLocation is required",
	) {
		t.Fatalf("error = %q, expected required-template parse error", parseError)
	}
}

func TestRouteDefinitionsOnChangeCallback_ReturnsErrorWhenNotInDevMode(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	callback := routeDefinitionsOnChangeCallback(app)
	action, err := callback(&wavewatch.HookContext{AppStoppedForBatch: false})
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
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

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

	action, err := callback(&wavewatch.HookContext{AppStoppedForBatch: false})
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

func TestGetDefaultWatchPatterns_IncludesRequiredRouteTemplateAndGoPatterns(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	patterns := getDefaultWatchPatterns(app)
	expectedPatternCount := len(app.Config.ClientRouteDefinitionPatterns()) + 2
	if len(patterns) != expectedPatternCount {
		t.Fatalf(
			"len(patterns) = %d, want %d",
			len(patterns),
			expectedPatternCount,
		)
	}
	hasGoPattern := false
	for _, pattern := range patterns {
		if pattern.Pattern == "**/*.go" {
			hasGoPattern = true
			break
		}
	}
	if !hasGoPattern {
		t.Fatal("expected default watch patterns to include **/*.go")
	}
}

func TestInjectDefaultWatchPatterns_IsIdempotent(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	parsedCfg := app.Wave.ParsedConfig()
	InjectDefaultWatchPatternsInConfig(parsedCfg, app)
	InjectDefaultWatchPatternsInConfig(parsedCfg, app)

	if len(waveframework.StateForConfig(parsedCfg).WatchPatterns) != 3 {
		t.Fatalf(
			"len(FrameworkWatchPatterns) = %d, want %d",
			len(waveframework.StateForConfig(parsedCfg).WatchPatterns),
			3,
		)
	}

	templatePath := filepath.Join(
		app.Wave.PrivateStaticDir(),
		app.Config.HTMLTemplateLocation(),
	)
	expectedPatternCounts := map[string]int{
		normalizeFrameworkWatchPatternPath(templatePath): 1,
		"**/*.go": 1,
	}
	for _, routeDefinitionPattern := range app.Config.ClientRouteDefinitionPatterns() {
		expectedPatternCounts[normalizeFrameworkWatchPatternPath(routeDefinitionPattern)] = 1
	}
	assertFrameworkWatchPatternCounts(
		t,
		waveframework.StateForConfig(parsedCfg).WatchPatterns,
		expectedPatternCounts,
	)

	expectedIgnoredPaths := []string{
		filepath.Join(
			app.Config.TSGenOutDir(),
			runtimepaths.GeneratedTypeScriptIndexFileName,
		),
		filepath.Join(
			app.Config.TSGenOutDir(),
			runtimepaths.GeneratedTypeScriptPublicFileMapFileName,
		),
	}
	assertIgnoredPatternCounts(
		t,
		waveframework.StateForConfig(parsedCfg).IgnoredPatterns,
		expectedIgnoredPaths,
		1,
	)
}

func TestInjectDefaultWatchPatterns_PreservesExistingUserConfiguration(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	parsedCfg := app.Wave.ParsedConfig()

	customGoWatchPattern := wavewatch.WatchedFile{
		Pattern: "**/*.go",
		OnChangeHooks: []wavewatch.OnChangeHook{{
			Cmd:    "CustomGoHook",
			Timing: wavewatch.OnChangeStrategyConcurrent,
		}},
	}
	waveframework.StateForConfig(parsedCfg).WatchPatterns = append(
		waveframework.StateForConfig(parsedCfg).WatchPatterns,
		customGoWatchPattern,
	)

	existingIgnoredPattern := filepath.Join(
		app.Config.TSGenOutDir(),
		runtimepaths.GeneratedTypeScriptIndexFileName,
	)
	waveframework.StateForConfig(parsedCfg).IgnoredPatterns = append(
		waveframework.StateForConfig(parsedCfg).IgnoredPatterns,
		existingIgnoredPattern,
	)

	InjectDefaultWatchPatternsInConfig(parsedCfg, app)

	if len(waveframework.StateForConfig(parsedCfg).WatchPatterns) != 3 {
		t.Fatalf(
			"len(FrameworkWatchPatterns) = %d, want %d",
			len(waveframework.StateForConfig(parsedCfg).WatchPatterns),
			3,
		)
	}

	var observedGoPattern *wavewatch.WatchedFile
	for idx := range waveframework.StateForConfig(parsedCfg).WatchPatterns {
		currentPattern := &waveframework.StateForConfig(parsedCfg).WatchPatterns[idx]
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
		filepath.Join(
			app.Config.TSGenOutDir(),
			runtimepaths.GeneratedTypeScriptIndexFileName,
		),
		filepath.Join(
			app.Config.TSGenOutDir(),
			runtimepaths.GeneratedTypeScriptPublicFileMapFileName,
		),
	}
	assertIgnoredPatternCounts(
		t,
		waveframework.StateForConfig(parsedCfg).IgnoredPatterns,
		expectedIgnoredPaths,
		1,
	)
}

func TestInjectDefaultWatchPatterns_DoesNotDuplicateSemanticallyEquivalentRoutePattern(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	parsedCfg := app.Wave.ParsedConfig()

	existingRoutePattern := app.Config.ClientRouteDefinitionPatterns()[0]
	waveframework.StateForConfig(parsedCfg).WatchPatterns = append(
		waveframework.StateForConfig(parsedCfg).WatchPatterns,
		wavewatch.WatchedFile{
			Pattern: existingRoutePattern,
			OnChangeHooks: []wavewatch.OnChangeHook{{
				Cmd:    "ExistingRouteHook",
				Timing: wavewatch.OnChangeStrategyConcurrent,
			}},
		},
	)

	InjectDefaultWatchPatternsInConfig(parsedCfg, app)

	if len(waveframework.StateForConfig(parsedCfg).WatchPatterns) != 3 {
		t.Fatalf(
			"len(FrameworkWatchPatterns) = %d, want %d",
			len(waveframework.StateForConfig(parsedCfg).WatchPatterns),
			3,
		)
	}

	normalizedRoutePattern := normalizeFrameworkWatchPatternPath(
		existingRoutePattern,
	)
	normalizedRoutePatternCount := 0
	for _, currentPattern := range waveframework.StateForConfig(parsedCfg).WatchPatterns {
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

	var observedRoutePattern *wavewatch.WatchedFile
	for idx := range waveframework.StateForConfig(parsedCfg).WatchPatterns {
		if waveframework.StateForConfig(parsedCfg).WatchPatterns[idx].Pattern == existingRoutePattern {
			observedRoutePattern = &waveframework.StateForConfig(parsedCfg).WatchPatterns[idx]
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

func TestInjectDefaultWatchPatterns_MatchesRouteAndTemplateWhenResolveRootIsAncestor(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	originalWorkingDirectory, workingDirectoryError := os.Getwd()
	if workingDirectoryError != nil {
		t.Fatalf("read current working directory: %v", workingDirectoryError)
	}
	if chdirError := os.Chdir(fixture.RootDir); chdirError != nil {
		t.Fatalf("chdir into fixture root %q: %v", fixture.RootDir, chdirError)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWorkingDirectory)
	})

	parsedCfg := app.Wave.ParsedConfig()
	InjectDefaultWatchPatternsInConfig(parsedCfg, app)

	templatePattern := normalizeFrameworkWatchPatternPath(
		filepath.Join(
			app.Wave.PrivateStaticDir(),
			app.Config.HTMLTemplateLocation(),
		),
	)
	routePattern := normalizeFrameworkWatchPatternPath(
		app.Config.ClientRouteDefinitionPatterns()[0],
	)
	assertFrameworkWatchPatternCounts(
		t,
		waveframework.StateForConfig(parsedCfg).WatchPatterns,
		map[string]int{
			routePattern:    1,
			templatePattern: 1,
			"**/*.go":       1,
		},
	)

	var routeWatchPattern *wavewatch.WatchedFile
	var templateWatchPattern *wavewatch.WatchedFile
	for index := range waveframework.StateForConfig(parsedCfg).WatchPatterns {
		pattern := &waveframework.StateForConfig(parsedCfg).WatchPatterns[index]
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
	patterns []wavewatch.WatchedFile,
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

func TestInjectGeneratedOutputPathsForDefaultWatchPatterns_AppendsGeneratedOutputsToIgnoreList(
	t *testing.T,
) {
	cfg := wavetest.NewParsedConfigAtRoot(t, t.TempDir())

	injectGeneratedOutputPathsForDefaultWatchPatterns(
		cfg,
		"frontend/src/vorma.gen",
	)

	expectedIgnoredPatterns := []string{
		filepath.Join(
			"frontend/src/vorma.gen",
			runtimepaths.GeneratedTypeScriptIndexFileName,
		),
		filepath.Join(
			"frontend/src/vorma.gen",
			runtimepaths.GeneratedTypeScriptPublicFileMapFileName,
		),
	}
	assertIgnoredPatternCounts(
		t,
		waveframework.StateForConfig(cfg).IgnoredPatterns,
		expectedIgnoredPatterns,
		1,
	)
	if len(waveframework.StateForConfig(cfg).IgnoredPatterns) != len(expectedIgnoredPatterns) {
		t.Fatalf(
			"len(FrameworkIgnoredPatterns) = %d, want %d",
			len(waveframework.StateForConfig(cfg).IgnoredPatterns),
			len(expectedIgnoredPatterns),
		)
	}
}

func TestRunOnChangeOnlyWatchPattern_UsesCallbackOnly(t *testing.T) {
	callbackInvoked := false
	callback := func(ctx *wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
		callbackInvoked = true
		if ctx == nil {
			t.Fatal("expected hook context to be non-nil")
		}
		return &wavewatch.RefreshAction{ReloadBrowser: true}, nil
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
		&wavewatch.HookContext{AppStoppedForBatch: false},
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
	if hook.Timing != wavewatch.OnChangeStrategyConcurrent {
		t.Fatalf(
			"Timing = %q, want %q",
			hook.Timing,
			wavewatch.OnChangeStrategyConcurrent,
		)
	}
	if hook.Callback != nil {
		t.Fatal("expected go watch pattern hook callback to be nil")
	}
}

func TestShouldInjectDefaultWatchPatterns(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	if !shouldInjectDefaultWatchPatterns(app) {
		t.Fatal("expected defaults to be injected when IncludeDefaults is nil")
	}

	includeDefaults := false
	testkit.MustMutateAppVormaConfig(
		t,
		app,
		func(config *vormaruntime.VormaConfigJSON) {
			config.IncludeDefaults = &includeDefaults
		},
	)
	if shouldInjectDefaultWatchPatterns(app) {
		t.Fatal("expected defaults to be skipped when IncludeDefaults is false")
	}

	includeDefaults = true
	testkit.MustMutateAppVormaConfig(
		t,
		app,
		func(config *vormaruntime.VormaConfigJSON) {
			config.IncludeDefaults = &includeDefaults
		},
	)
	if !shouldInjectDefaultWatchPatterns(app) {
		t.Fatal("expected defaults to be injected when IncludeDefaults is true")
	}
}

func TestRouteDefinitionWatchPatterns_UseConfiguredRoutePattern(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	testkit.MustMutateAppVormaConfig(
		t,
		app,
		func(config *vormaruntime.VormaConfigJSON) {
			config.ClientRouteDefinitionPatterns = []string{
				"frontend/src/custom.routes.ts",
			}
		},
	)

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
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	testkit.MustMutateAppVormaConfig(
		t,
		app,
		func(config *vormaruntime.VormaConfigJSON) {
			config.HTMLTemplateLocation = "templates/custom.entry.go.html"
		},
	)

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
	if pattern.OnChangeHooks[0].Timing != wavewatch.OnChangeStrategyPost {
		t.Fatalf(
			"OnChangeHooks[0].Timing = %q, want %q",
			pattern.OnChangeHooks[0].Timing,
			wavewatch.OnChangeStrategyPost,
		)
	}
}
