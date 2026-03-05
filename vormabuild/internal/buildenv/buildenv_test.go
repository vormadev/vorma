package buildenv

import (
	"context"
	"fmt"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimeconfig"
	"github.com/vormadev/vorma/vormabuild/internal/backendroutes/registraroverlay"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
	"github.com/vormadev/vorma/wave/waveframework"
)

func TestConfigure_WiresHooksAndDefaults(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	parsedCfg := Configure(app)

	if _, hasVormaSchema := waveframework.StateForConfig(parsedCfg).SchemaExtensions["Vorma"]; !hasVormaSchema {
		t.Fatal("expected Configure to register Vorma schema extension")
	}

	mainBuildEntryRelativeToResolveRoot, mainBuildEntryError := runtimeconfig.NormalizePathOrPatternToResolveRootRelative(
		app.Wave.ParsedConfig().ResolveRoot(),
		app.Config.MainBuildEntry(),
	)
	if mainBuildEntryError != nil {
		t.Fatalf("normalize MainBuildEntry for expectation: %v", mainBuildEntryError)
	}

	expectedDevHook := fmt.Sprintf(
		"go run ./%s --dev --hook",
		mainBuildEntryRelativeToResolveRoot,
	)
	if waveframework.StateForConfig(parsedCfg).DevBuildHook != expectedDevHook {
		t.Fatalf(
			"FrameworkDevBuildHook = %q, want %q",
			waveframework.StateForConfig(parsedCfg).DevBuildHook,
			expectedDevHook,
		)
	}

	expectedProdHook := fmt.Sprintf(
		"go run ./%s --hook",
		mainBuildEntryRelativeToResolveRoot,
	)
	if waveframework.StateForConfig(parsedCfg).ProdBuildHook != expectedProdHook {
		t.Fatalf(
			"FrameworkProdBuildHook = %q, want %q",
			waveframework.StateForConfig(parsedCfg).ProdBuildHook,
			expectedProdHook,
		)
	}

	if len(waveframework.StateForConfig(parsedCfg).WatchPatterns) != 3 {
		t.Fatalf(
			"expected 3 default framework watch patterns, got %d",
			len(waveframework.StateForConfig(parsedCfg).WatchPatterns),
		)
	}
	if got, want := waveframework.StateForConfig(parsedCfg).PublicFileMapReloadEndpointPath, runtimeconfig.DefaultDevReloadPublicFileMapEndpointPath; got != want {
		t.Fatalf(
			"PublicFileMapReloadEndpointPath = %q, want %q",
			got,
			want,
		)
	}

	if waveframework.StateForConfig(parsedCfg).RunBuildHook == nil {
		t.Fatal("expected Configure to wire framework build hook runner")
	}

	if waveframework.StateForConfig(parsedCfg).PrepareGoBuildOverlay == nil {
		t.Fatal(
			"expected Configure to wire framework go-build overlay preparation",
		)
	}
}

func TestConfigureInConfig_PreservesExistingFrameworkBuildHooks(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	parsedCfg := app.Wave.ParsedConfig()

	waveframework.StateForConfig(parsedCfg).DevBuildHook = "go run ./custom/devhook"
	waveframework.StateForConfig(parsedCfg).ProdBuildHook = "go run ./custom/prodhook"

	ConfigureInConfig(app, parsedCfg)

	if waveframework.StateForConfig(parsedCfg).DevBuildHook != "go run ./custom/devhook" {
		t.Fatalf(
			"FrameworkDevBuildHook = %q, want preserved custom hook",
			waveframework.StateForConfig(parsedCfg).DevBuildHook,
		)
	}
	if waveframework.StateForConfig(parsedCfg).ProdBuildHook != "go run ./custom/prodhook" {
		t.Fatalf(
			"FrameworkProdBuildHook = %q, want preserved custom hook",
			waveframework.StateForConfig(parsedCfg).ProdBuildHook,
		)
	}
}

func TestConfigureInConfig_PreservesExistingFrameworkBuildHookRunner(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	parsedCfg := app.Wave.ParsedConfig()

	frameworkBuildHookRunnerCalled := false
	waveframework.StateForConfig(parsedCfg).RunBuildHook = func(context.Context, bool) error {
		frameworkBuildHookRunnerCalled = true
		return nil
	}

	ConfigureInConfig(app, parsedCfg)

	if waveframework.StateForConfig(parsedCfg).RunBuildHook == nil {
		t.Fatal(
			"expected existing framework build hook runner to remain configured",
		)
	}
	if err := waveframework.StateForConfig(parsedCfg).RunBuildHook(context.Background(), true); err != nil {
		t.Fatalf("existing framework build hook runner returned error: %v", err)
	}
	if !frameworkBuildHookRunnerCalled {
		t.Fatal(
			"expected ConfigureInConfig to preserve existing framework build hook runner",
		)
	}
}

func TestConfigureInConfig_PreservesExistingFrameworkGoBuildOverlayPreparation(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	parsedCfg := app.Wave.ParsedConfig()

	overlayPreparationCalled := false
	waveframework.StateForConfig(parsedCfg).PrepareGoBuildOverlay = func() (*waveframework.GoBuildOverlay, error) {
		overlayPreparationCalled = true
		return nil, nil
	}

	ConfigureInConfig(app, parsedCfg)

	if waveframework.StateForConfig(parsedCfg).PrepareGoBuildOverlay == nil {
		t.Fatal(
			"expected existing framework go-build overlay preparation to remain configured",
		)
	}
	if _, err := waveframework.StateForConfig(parsedCfg).PrepareGoBuildOverlay(); err != nil {
		t.Fatalf(
			"existing framework go-build overlay preparation returned error: %v",
			err,
		)
	}
	if !overlayPreparationCalled {
		t.Fatal(
			"expected ConfigureInConfig to preserve existing framework overlay callback",
		)
	}
}

func TestConfigure_FrameworkBuildHookRunner_ExecutesHookCommand(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	parsedCfg := app.Wave.ParsedConfig()

	overlayCleanupCalled := false
	var capturedGoRunArgs []string
	frameworkBuildHookExecutor := newFrameworkBuildHookExecutor(
		frameworkBuildHookExecutionDependencies{
			prepareDiscoveredRouteRegistrarOverlayWithArtifactCache: func(
				*vormaruntime.Vorma,
				*registraroverlay.DiscoveredRouteRegistrarArtifactCache,
			) (*registraroverlay.DiscoveredRouteRegistrarOverlay, error) {
				return registraroverlay.NewDiscoveredRouteRegistrarOverlay(
					"/tmp/vorma-test-overlay.json",
					func() error {
						overlayCleanupCalled = true
						return nil
					},
				), nil
			},
			runGoCommandWithContext: func(
				commandExecutionContext context.Context,
				goRunArgs []string,
			) error {
				if commandExecutionContext == nil {
					t.Fatal("expected non-nil command execution context")
				}
				capturedGoRunArgs = append([]string{}, goRunArgs...)
				return nil
			},
		},
	)

	configureInConfigWithFrameworkBuildHookExecutor(
		app,
		parsedCfg,
		frameworkBuildHookExecutor,
	)
	if waveframework.StateForConfig(parsedCfg).RunBuildHook == nil {
		t.Fatal("expected framework build hook runner to be configured")
	}

	if err := waveframework.StateForConfig(parsedCfg).RunBuildHook(context.Background(), true); err != nil {
		t.Fatalf("framework build hook runner returned error: %v", err)
	}

	expectedGoRunArgs := []string{
		"run",
		"-overlay=/tmp/vorma-test-overlay.json",
		"./backend/cmd/build",
		"--dev",
		"--hook-inner",
	}
	if len(capturedGoRunArgs) != len(expectedGoRunArgs) {
		t.Fatalf(
			"go run args len = %d, want %d; got %#v",
			len(capturedGoRunArgs),
			len(expectedGoRunArgs),
			capturedGoRunArgs,
		)
	}
	for argumentIndex, expectedArgument := range expectedGoRunArgs {
		if capturedGoRunArgs[argumentIndex] != expectedArgument {
			t.Fatalf(
				"go run arg[%d] = %q, want %q (args=%#v)",
				argumentIndex,
				capturedGoRunArgs[argumentIndex],
				expectedArgument,
				capturedGoRunArgs,
			)
		}
	}
	if !overlayCleanupCalled {
		t.Fatal("expected framework build hook runner to cleanup overlay")
	}
}

func TestConfigure_UsesLifecycleOwnedDiscoveredRegistrarCache(
	t *testing.T,
) {
	var capturedCaches []*registraroverlay.DiscoveredRouteRegistrarArtifactCache
	frameworkBuildHookExecutor := newFrameworkBuildHookExecutor(
		frameworkBuildHookExecutionDependencies{
			prepareDiscoveredRouteRegistrarOverlayWithArtifactCache: func(
				_ *vormaruntime.Vorma,
				discoveredRegistrarArtifactsCache *registraroverlay.DiscoveredRouteRegistrarArtifactCache,
			) (*registraroverlay.DiscoveredRouteRegistrarOverlay, error) {
				if discoveredRegistrarArtifactsCache == nil {
					t.Fatal(
						"expected non-nil discovered registrar artifacts cache",
					)
				}
				capturedCaches = append(
					capturedCaches,
					discoveredRegistrarArtifactsCache,
				)
				return nil, nil
			},
			runGoCommandWithContext: func(
				commandExecutionContext context.Context,
				_ []string,
			) error {
				if commandExecutionContext == nil {
					t.Fatal("expected non-nil command execution context")
				}
				return nil
			},
		},
	)

	fixtureOne := testkit.NewBuildTestFixture(t, nil)
	parsedCfgOne := fixtureOne.App.Wave.ParsedConfig()
	configureInConfigWithFrameworkBuildHookExecutor(
		fixtureOne.App,
		parsedCfgOne,
		frameworkBuildHookExecutor,
	)

	if err := waveframework.StateForConfig(parsedCfgOne).RunBuildHook(context.Background(), true); err != nil {
		t.Fatalf("first framework build hook run returned error: %v", err)
	}
	if _, err := waveframework.StateForConfig(parsedCfgOne).PrepareGoBuildOverlay(); err != nil {
		t.Fatalf(
			"first framework go build overlay preparation returned error: %v",
			err,
		)
	}
	if err := waveframework.StateForConfig(parsedCfgOne).RunBuildHook(context.Background(), false); err != nil {
		t.Fatalf("second framework build hook run returned error: %v", err)
	}

	if len(capturedCaches) != 3 {
		t.Fatalf("captured cache count = %d, want 3", len(capturedCaches))
	}
	firstLifecycleCache := capturedCaches[0]
	if capturedCaches[1] != firstLifecycleCache ||
		capturedCaches[2] != firstLifecycleCache {
		t.Fatalf(
			"expected same cache instance within lifecycle, got %#v",
			capturedCaches,
		)
	}

	fixtureTwo := testkit.NewBuildTestFixture(t, nil)
	parsedCfgTwo := fixtureTwo.App.Wave.ParsedConfig()
	configureInConfigWithFrameworkBuildHookExecutor(
		fixtureTwo.App,
		parsedCfgTwo,
		frameworkBuildHookExecutor,
	)

	if err := waveframework.StateForConfig(parsedCfgTwo).RunBuildHook(context.Background(), true); err != nil {
		t.Fatalf("third framework build hook run returned error: %v", err)
	}
	if len(capturedCaches) != 4 {
		t.Fatalf("captured cache count = %d, want 4", len(capturedCaches))
	}
	secondLifecycleCache := capturedCaches[3]
	if secondLifecycleCache == firstLifecycleCache {
		t.Fatal(
			"expected distinct lifecycle cache for second configured runtime",
		)
	}
}
