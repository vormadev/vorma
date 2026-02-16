package vormabuild

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/wave"
)

func TestDefaultWatchPatternCallbacks_RoutesAndTemplate(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	t.Chdir(fixture.rootDir)

	writeBootstrapStyleRoutesFixtureFiles(t)
	app.SetIsDev(true)
	app.Config.DevReloadRoutesEndpointPath = "/__custom_internal/reload-routes"
	app.Config.DevReloadTemplateEndpointPath = "/__custom_internal/reload-template"

	patterns := getDefaultWatchPatterns(app)
	if len(patterns) != 3 {
		t.Fatalf("expected 3 default watch patterns, got %d", len(patterns))
	}

	routePatternHook := findWatchHookByPattern(t, patterns, app.Config.ClientRouteDefinitionPatterns[0])
	templatePatternHook := findWatchHookByPattern(
		t,
		patterns,
		filepath.Join(app.Wave.GetPrivateStaticDir(), app.Config.HTMLTemplateLocation),
	)
	if routePatternHook == nil || templatePatternHook == nil {
		t.Fatal("expected route and template watch callbacks to be configured")
	}

	var routeResolverCalls int
	var templateResolverCalls int
	var shouldRouteFallback bool
	var shouldTemplateFallback bool
	resolveReloadAction := func(
		_ *vormaruntime.Vorma,
		reloadEndpoint string,
		_ string,
		_ string,
	) *wave.RefreshAction {
		switch reloadEndpoint {
		case app.DevReloadRoutesEndpointPath():
			routeResolverCalls++
			if shouldRouteFallback {
				return newRestartWithoutRecompileAction()
			}
			return newReloadBrowserAndWaitAction()
		case app.DevReloadTemplateEndpointPath():
			templateResolverCalls++
			if shouldTemplateFallback {
				return newRestartWithoutRecompileAction()
			}
			return newReloadBrowserAndWaitAction()
		default:
			t.Fatalf("unexpected reload endpoint %q", reloadEndpoint)
			return nil
		}
	}

	routesHook := routeDefinitionsOnChangeCallbackWithReloadActionResolver(
		app,
		resolveReloadAction,
	)
	templateHook := htmlTemplateOnChangeCallbackWithReloadActionResolver(
		app,
		resolveReloadAction,
	)

	t.Run("routes callback success returns browser reload action", func(t *testing.T) {
		shouldRouteFallback = false

		action, err := routesHook(&wave.HookContext{AppStoppedForBatch: false})
		if err != nil {
			t.Fatalf("routes callback returned error: %v", err)
		}
		if action == nil {
			t.Fatal("expected non-nil action for route reload success")
		}
		if !action.ReloadBrowser || !action.WaitForApp || !action.WaitForVite {
			t.Fatalf("unexpected action on success: %#v", action)
		}
		if action.TriggerRestart || action.RecompileGo {
			t.Fatalf("success action should not restart/recompile: %#v", action)
		}
		if !strings.HasPrefix(app.GetBuildID(), "dev_fast_") {
			t.Fatalf("expected dev_fast build ID after route rebuild, got %q", app.GetBuildID())
		}
		if routeResolverCalls != 1 {
			t.Fatalf("expected exactly one route resolver call, got %d", routeResolverCalls)
		}
	})

	t.Run("routes callback nil context still returns browser reload action", func(t *testing.T) {
		shouldRouteFallback = false

		action, err := routesHook(nil)
		if err != nil {
			t.Fatalf("routes callback returned error: %v", err)
		}
		if action == nil {
			t.Fatal("expected non-nil action for nil route hook context")
		}
		if !action.ReloadBrowser || !action.WaitForApp || !action.WaitForVite {
			t.Fatalf("unexpected action with nil route hook context: %#v", action)
		}
		if routeResolverCalls != 2 {
			t.Fatalf("expected exactly one additional route resolver call, got %d", routeResolverCalls)
		}
	})

	t.Run("routes callback app-stopped returns nil action", func(t *testing.T) {
		action, err := routesHook(&wave.HookContext{AppStoppedForBatch: true})
		if err != nil {
			t.Fatalf("routes callback returned error: %v", err)
		}
		if action != nil {
			t.Fatalf("expected nil action when app stopped for batch, got %#v", action)
		}
		if !strings.HasPrefix(app.GetBuildID(), "dev_fast_") {
			t.Fatalf("expected route rebuild to still run, got build ID %q", app.GetBuildID())
		}
		if routeResolverCalls != 2 {
			t.Fatalf("expected no additional route resolver call when app stopped, got %d", routeResolverCalls)
		}
	})

	t.Run("routes callback endpoint failure falls back to restart", func(t *testing.T) {
		shouldRouteFallback = true

		action, err := routesHook(&wave.HookContext{AppStoppedForBatch: false})
		if err != nil {
			t.Fatalf("routes callback returned error: %v", err)
		}
		if action == nil {
			t.Fatal("expected non-nil action when reload endpoint fails")
		}
		if !action.TriggerRestart || action.RecompileGo {
			t.Fatalf("expected restart without recompilation, got %#v", action)
		}
		if routeResolverCalls != 3 {
			t.Fatalf("expected route resolver call on failure path, got %d", routeResolverCalls)
		}
	})

	t.Run("template callback success returns browser reload action", func(t *testing.T) {
		shouldTemplateFallback = false

		action, err := templateHook(&wave.HookContext{AppStoppedForBatch: false})
		if err != nil {
			t.Fatalf("template callback returned error: %v", err)
		}
		if action == nil {
			t.Fatal("expected non-nil action for template reload success")
		}
		if !action.ReloadBrowser || !action.WaitForApp || !action.WaitForVite {
			t.Fatalf("unexpected action on template success: %#v", action)
		}
		if action.TriggerRestart || action.RecompileGo {
			t.Fatalf("template success should not restart/recompile: %#v", action)
		}
		if templateResolverCalls != 1 {
			t.Fatalf("expected exactly one template resolver call, got %d", templateResolverCalls)
		}
	})

	t.Run("template callback nil context still returns browser reload action", func(t *testing.T) {
		shouldTemplateFallback = false

		action, err := templateHook(nil)
		if err != nil {
			t.Fatalf("template callback returned error: %v", err)
		}
		if action == nil {
			t.Fatal("expected non-nil action for nil template hook context")
		}
		if !action.ReloadBrowser || !action.WaitForApp || !action.WaitForVite {
			t.Fatalf("unexpected action with nil template hook context: %#v", action)
		}
		if templateResolverCalls != 2 {
			t.Fatalf("expected exactly one additional template resolver call, got %d", templateResolverCalls)
		}
	})

	t.Run("template callback app-stopped returns nil action", func(t *testing.T) {
		action, err := templateHook(&wave.HookContext{AppStoppedForBatch: true})
		if err != nil {
			t.Fatalf("template callback returned error: %v", err)
		}
		if action != nil {
			t.Fatalf("expected nil action when app stopped for batch, got %#v", action)
		}
		if templateResolverCalls != 2 {
			t.Fatalf("expected no additional template resolver call when app stopped, got %d", templateResolverCalls)
		}
	})

	t.Run("template callback endpoint failure falls back to restart", func(t *testing.T) {
		shouldTemplateFallback = true

		action, err := templateHook(&wave.HookContext{AppStoppedForBatch: false})
		if err != nil {
			t.Fatalf("template callback returned error: %v", err)
		}
		if action == nil {
			t.Fatal("expected non-nil action when template endpoint fails")
		}
		if !action.TriggerRestart || action.RecompileGo {
			t.Fatalf("expected restart without recompilation, got %#v", action)
		}
		if templateResolverCalls != 3 {
			t.Fatalf("expected template resolver call on failure path, got %d", templateResolverCalls)
		}
	})
}

func writeBootstrapStyleRoutesFixtureFiles(t *testing.T) {
	t.Helper()

	mustWriteFile(t, "frontend/src/components/root.tsx", []byte("export const Root = () => null;"))
	mustWriteFile(t, "frontend/src/components/home.tsx", []byte("export const Home = () => null;"))
	mustWriteFile(t, "frontend/src/components/links.tsx", []byte("export const Links = () => null;"))

	mustWriteFile(t, "frontend/src/vorma.routes.ts", []byte(`
import { route } from "vorma/buildtime";

route("/", import("./components/root.tsx"), "Root");
route("/_index", import("./components/home.tsx"), "Home");
route("/links", import("./components/links.tsx"), "Links");
`))
}

func findWatchHookByPattern(t *testing.T, patterns []wave.WatchedFile, pattern string) func(*wave.HookContext) (*wave.RefreshAction, error) {
	t.Helper()
	for _, watched := range patterns {
		if watched.Pattern == pattern {
			if len(watched.OnChangeHooks) != 1 || watched.OnChangeHooks[0].Callback == nil {
				t.Fatalf("pattern %q did not expose exactly one callback hook", pattern)
			}
			return watched.OnChangeHooks[0].Callback
		}
	}
	t.Fatalf("did not find watched pattern %q", pattern)
	return nil
}
