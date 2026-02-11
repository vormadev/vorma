package vormaruntime

import (
	"testing"

	"github.com/vormadev/vorma/kit/mux"
)

func TestRouteRegistrySync_ClearsCacheAndRebuildsPatterns(t *testing.T) {
	stage := defaultPathsFile("route-registry-build", map[string]*Path{
		"/old-client": {
			OriginalPattern: "/old-client",
			SrcPath:         "frontend/src/routes/old-client.tsx",
			OutPath:         "vorma_out/routes/old-client.js",
			ExportKey:       "default",
		},
	})
	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/server-only",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		}),
	)
	app.RegisterPatternIfNeeded("/stale-no-handler")

	clearRouteDataCacheForTest()
	gmpdCache.Store("test-stale", &cachedItemSubset{ImportURLs: []string{"/stale.js"}})

	newPaths := map[string]*Path{
		"/fresh-client": {
			OriginalPattern: "/fresh-client",
			SrcPath:         "frontend/src/routes/fresh-client.tsx",
			OutPath:         "vorma_out/routes/fresh-client.js",
			ExportKey:       "default",
		},
	}

	app.WithLock(func(lv *LockedVorma) {
		lv.Routes().Sync(newPaths)
	})

	if got := routeDataCacheLenForTest(); got != 0 {
		t.Fatalf("route-data cache size = %d, want 0 after Sync", got)
	}

	paths := app.GetPathsSnapshot()
	if _, ok := paths["/fresh-client"]; !ok {
		t.Fatal("expected /fresh-client in synced paths")
	}
	serverOnly, ok := paths["/server-only"]
	if !ok {
		t.Fatal("expected server-only handler route to be merged into paths")
	}
	if serverOnly.SrcPath != "" {
		t.Fatalf("server-only SrcPath = %q, want empty", serverOnly.SrcPath)
	}
	if serverOnly.ExportKey != "default" {
		t.Fatalf("server-only ExportKey = %q, want %q", serverOnly.ExportKey, "default")
	}
	if _, ok := paths["/old-client"]; ok {
		t.Fatal("expected /old-client to be replaced by Sync")
	}

	nr := app.LoadersRouter().NestedRouter
	if !nr.IsRegistered("/fresh-client") {
		t.Fatal("expected /fresh-client to be registered in nested router after Sync")
	}
	if !nr.IsRegistered("/server-only") {
		t.Fatal("expected /server-only handler route to remain registered")
	}
	if nr.IsRegistered("/stale-no-handler") {
		t.Fatal("expected stale no-handler pattern to be removed on Sync rebuild")
	}
}

func TestRouteRegistrySync_NilPathsStillPreservesServerHandlers(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/internal/status",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (string, error) {
			return "ok", nil
		}),
	)

	app.WithLock(func(lv *LockedVorma) {
		lv.Routes().Sync(nil)
	})

	paths := app.GetPathsSnapshot()
	if paths == nil {
		t.Fatal("paths should never be nil after Sync(nil)")
	}
	if _, ok := paths["/internal/status"]; !ok {
		t.Fatal("expected server handler route to be preserved when Sync(nil)")
	}
}

func TestRouteRegistryMergeServerRoutes_SkipsNoHandlerRoutes(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app

	mux.RegisterNestedPatternWithoutHandler(app.LoadersRouter().NestedRouter, "/no-handler-only")
	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/has-handler",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (string, error) {
			return "ok", nil
		}),
	)

	app.WithLock(func(lv *LockedVorma) {
		lv.Routes().Sync(map[string]*Path{})
	})

	paths := app.GetPathsSnapshot()
	if _, ok := paths["/has-handler"]; !ok {
		t.Fatal("expected handler route to be merged into paths")
	}
	if _, ok := paths["/no-handler-only"]; ok {
		t.Fatal("expected no-handler route to be excluded from merged paths")
	}
}

func clearRouteDataCacheForTest() {
	gmpdCache.Range(func(key, _ any) bool {
		gmpdCache.Delete(key)
		return true
	})
}

func routeDataCacheLenForTest() int {
	n := 0
	gmpdCache.Range(func(_, _ any) bool {
		n++
		return true
	})
	return n
}
