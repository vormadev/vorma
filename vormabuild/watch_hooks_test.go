package vormabuild

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/vormaruntime"
	"github.com/vormadev/vorma/wave"
)

func TestDefaultWatchPatternCallbacks_RoutesAndTemplate(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	t.Chdir(fixture.rootDir)

	writeBootstrapStyleRoutesFixtureFiles(t)
	app.SetIsDev(true)

	patterns := getDefaultWatchPatterns(app)
	if len(patterns) != 3 {
		t.Fatalf("expected 3 default watch patterns, got %d", len(patterns))
	}

	routesHook := findWatchHookByPattern(t, patterns, app.Config.ClientRouteDefinitionPatterns[0])
	templateHook := findWatchHookByPattern(
		t,
		patterns,
		filepath.Join(app.Wave.GetPrivateStaticDir(), app.Config.HTMLTemplateLocation),
	)

	var routeReloadEndpointErr error
	var templateReloadEndpointErr error
	routeReloadEndpointCalls := 0
	templateReloadEndpointCalls := 0

	originalCallReloadEndpointStep := reloadActionDeps.callReloadEndpoint
	reloadActionDeps.callReloadEndpoint = func(_ *vormaruntime.Vorma, endpoint string) error {
		switch endpoint {
		case vormaruntime.Dev_ReloadRoutesPath:
			routeReloadEndpointCalls++
			return routeReloadEndpointErr
		case vormaruntime.Dev_ReloadTemplatePath:
			templateReloadEndpointCalls++
			return templateReloadEndpointErr
		default:
			return errors.New("unexpected endpoint path")
		}
	}
	t.Cleanup(func() {
		reloadActionDeps.callReloadEndpoint = originalCallReloadEndpointStep
	})

	t.Run("routes callback success returns browser reload action", func(t *testing.T) {
		routeReloadEndpointErr = nil

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
		if routeReloadEndpointCalls != 1 {
			t.Fatalf("expected exactly one route endpoint call, got %d", routeReloadEndpointCalls)
		}
	})

	t.Run("routes callback nil context still returns browser reload action", func(t *testing.T) {
		routeReloadEndpointErr = nil

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
		if routeReloadEndpointCalls != 2 {
			t.Fatalf("expected exactly one additional route endpoint call, got %d", routeReloadEndpointCalls)
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
		if routeReloadEndpointCalls != 2 {
			t.Fatalf("expected no additional route endpoint call when app stopped, got %d", routeReloadEndpointCalls)
		}
	})

	t.Run("routes callback endpoint failure falls back to restart", func(t *testing.T) {
		routeReloadEndpointErr = errors.New("route endpoint failed")

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
		if routeReloadEndpointCalls != 3 {
			t.Fatalf("expected route endpoint call on failure path, got %d", routeReloadEndpointCalls)
		}
	})

	t.Run("template callback success returns browser reload action", func(t *testing.T) {
		templateReloadEndpointErr = nil

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
		if templateReloadEndpointCalls != 1 {
			t.Fatalf("expected exactly one template endpoint call, got %d", templateReloadEndpointCalls)
		}
	})

	t.Run("template callback nil context still returns browser reload action", func(t *testing.T) {
		templateReloadEndpointErr = nil

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
		if templateReloadEndpointCalls != 2 {
			t.Fatalf("expected exactly one additional template endpoint call, got %d", templateReloadEndpointCalls)
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
		if templateReloadEndpointCalls != 2 {
			t.Fatalf("expected no additional template endpoint call when app stopped, got %d", templateReloadEndpointCalls)
		}
	})

	t.Run("template callback endpoint failure falls back to restart", func(t *testing.T) {
		templateReloadEndpointErr = errors.New("template endpoint failed")

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
		if templateReloadEndpointCalls != 3 {
			t.Fatalf("expected template endpoint call on failure path, got %d", templateReloadEndpointCalls)
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
