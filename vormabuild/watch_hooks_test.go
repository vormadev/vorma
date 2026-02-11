package vormabuild

import (
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
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

	routesHook := findWatchHookByPattern(t, patterns, app.Config.ClientRouteDefsFile)
	templateHook := findWatchHookByPattern(
		t,
		patterns,
		filepath.Join(app.Wave.GetPrivateStaticDir(), app.Config.HTMLTemplateLocation),
	)

	routeStatusCode := http.StatusOK
	templateStatusCode := http.StatusOK
	var statusMu sync.Mutex

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate listener failed: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			statusMu.Lock()
			defer statusMu.Unlock()
			switch r.URL.Path {
			case vormaruntime.Dev_ReloadRoutesPath:
				w.WriteHeader(routeStatusCode)
			case vormaruntime.Dev_ReloadTemplatePath:
				w.WriteHeader(templateStatusCode)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}),
	}

	defer ln.Close()
	go func() {
		_ = server.Serve(ln)
	}()
	defer server.Close()

	t.Setenv("WAVE_MODE", "")
	t.Setenv("PORT", fmt.Sprintf("%d", port))
	t.Setenv("WAVE_PORT_HAS_BEEN_SET", "true")

	t.Run("routes callback success returns browser reload action", func(t *testing.T) {
		statusMu.Lock()
		routeStatusCode = http.StatusOK
		statusMu.Unlock()

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
	})

	t.Run("routes callback endpoint failure falls back to restart", func(t *testing.T) {
		statusMu.Lock()
		routeStatusCode = http.StatusInternalServerError
		statusMu.Unlock()

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
	})

	t.Run("template callback success returns browser reload action", func(t *testing.T) {
		statusMu.Lock()
		templateStatusCode = http.StatusOK
		statusMu.Unlock()

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
	})

	t.Run("template callback app-stopped returns nil action", func(t *testing.T) {
		action, err := templateHook(&wave.HookContext{AppStoppedForBatch: true})
		if err != nil {
			t.Fatalf("template callback returned error: %v", err)
		}
		if action != nil {
			t.Fatalf("expected nil action when app stopped for batch, got %#v", action)
		}
	})

	t.Run("template callback endpoint failure falls back to restart", func(t *testing.T) {
		statusMu.Lock()
		templateStatusCode = http.StatusInternalServerError
		statusMu.Unlock()

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
