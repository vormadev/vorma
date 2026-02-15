package vormaruntime

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/vormadev/vorma/kit/mux"
)

func TestLoadOrBuildCachedItemSubset_DoesNotStoreWhenSnapshotVersionIsStale(t *testing.T) {
	oldStage := defaultPathsFile("build-same", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.old.tsx",
			OutPath:         "vorma_out/routes/products.$id.old.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-old.js"},
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: oldStage,
		stageTwo: oldStage,
	})
	app := fixture.app
	app.SetIsDev(true)
	app.validateAndDecorateNestedRouter(app.LoadersRouter().NestedRouter)

	req := httptest.NewRequest(http.MethodGet, "/products/1", nil)
	matchResults, found := mux.FindNestedMatches(app.LoadersRouter().NestedRouter, req)
	if !found {
		t.Fatal("expected nested match for /products/1 test setup")
	}
	matches := matchResults.Matches
	cacheKey := app.buildRouteDataCacheKey(matches, true, app.GetBuildID())

	clearRouteDataCacheForTest()
	if got := routeDataCacheLenForTest(); got != 0 {
		t.Fatalf("expected empty route-data cache before test, got %d entries", got)
	}

	var staleSnapshotVersion uint64
	app.WithRLock(func(lv *ReadLockedVorma) {
		staleSnapshotVersion = lv.v._routeDataSnapshotVersion
	})

	oldPathsSnapshot := app.GetPathsSnapshot()
	oldClientEntryDepsSnapshot := app.GetClientEntryDeps()

	app.WithLock(func(lv *LockedVorma) {
		lv.SetPaths(map[string]*Path{
			"/products/:id": {
				OriginalPattern: "/products/:id",
				SrcPath:         "frontend/src/routes/products.$id.new.tsx",
				OutPath:         "vorma_out/routes/products.$id.new.js",
				ExportKey:       "default",
				Deps:            []string{"vorma_out/chunk-new.js"},
			},
		})
	})

	staleCached := loadOrBuildCachedItemSubset(
		app,
		cacheKey,
		matches,
		oldPathsSnapshot,
		oldClientEntryDepsSnapshot,
		true,
		staleSnapshotVersion,
	)
	if staleCached == nil {
		t.Fatal("expected stale snapshot route-data subset build to return data")
	}
	if got, want := staleCached.ImportURLs, []string{"/frontend/src/routes/products.$id.old.tsx"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("stale snapshot ImportURLs = %#v, want %#v", got, want)
	}
	if got := routeDataCacheLenForTest(); got != 0 {
		t.Fatalf("stale snapshot should not repopulate route-data cache, got %d entries", got)
	}

	var currentSnapshotVersion uint64
	app.WithRLock(func(lv *ReadLockedVorma) {
		currentSnapshotVersion = lv.v._routeDataSnapshotVersion
	})

	currentCached := loadOrBuildCachedItemSubset(
		app,
		cacheKey,
		matches,
		app.GetPathsSnapshot(),
		app.GetClientEntryDeps(),
		true,
		currentSnapshotVersion,
	)
	if currentCached == nil {
		t.Fatal("expected current snapshot route-data subset build to return data")
	}
	if got, want := currentCached.ImportURLs, []string{"/frontend/src/routes/products.$id.new.tsx"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("current snapshot ImportURLs = %#v, want %#v", got, want)
	}
	if got := routeDataCacheLenForTest(); got != 1 {
		t.Fatalf("expected cache to contain one current-snapshot entry, got %d", got)
	}
}
