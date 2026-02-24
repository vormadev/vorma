package vormaruntime

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime/routepipeline"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
)

var (
	benchBodyLenSink int
	benchDepsSink    []string
	benchCSSSink     []string
	benchStringSink  string
)

type benchActionInput struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func BenchmarkLoadersHandler_JSONCurrentBuild(b *testing.B) {
	paths := map[string]*Path{
		"/items/:id": {
			OriginalPattern: "/items/:id",
			SrcPath:         "frontend/src/routes/items.$id.tsx",
			OutPath:         "vorma_out/routes/items.$id.js",
			ExportKey:       "default",
			Deps: []string{
				"vorma_out/chunk-items.js",
				"vorma_out/chunk-shared.js",
				"vorma_out/chunk-details.js",
			},
		},
	}
	stage := defaultPathsFile("bench-build", paths)
	stage.ClientEntryDeps = []string{"vorma_out/chunk-shared.js"}
	stage.DepToCSSBundleMap = map[string][]string{
		"vorma_out/client-entry.js": {"vorma_out/client-entry.css"},
		"vorma_out/chunk-shared.js": {"vorma_out/chunk-shared.css"},
		"vorma_out/chunk-items.js":  {"vorma_out/chunk-items.css"},
		"vorma_out/chunk-details.js": {
			"vorma_out/chunk-details.css",
			"vorma_out/chunk-shared.css",
		},
	}

	fixture := newTestFixture(b, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app
	app.SetIsDev(false)

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/items/:id",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
				return map[string]any{"id": rd.Params()["id"]}, nil
			},
		),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())
	url := "/items/42?vorma_json=" + app.BuildID()

	warmReq := httptest.NewRequest(http.MethodGet, url, nil)
	warmRec := httptest.NewRecorder()
	handler.ServeHTTP(warmRec, warmReq)
	if warmRec.Code != http.StatusOK {
		b.Fatalf("warmup status = %d, want %d", warmRec.Code, http.StatusOK)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		benchBodyLenSink = rec.Body.Len()
	}
}

func BenchmarkRouteDepsAndCSSResolution(b *testing.B) {
	paths := map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			Deps: []string{
				"vorma_out/chunk-a.js",
				"vorma_out/chunk-b.js",
				"vorma_out/chunk-c.js",
				"vorma_out/chunk-a.js",
			},
		},
	}
	stage := defaultPathsFile("bench-deps", paths)
	stage.ClientEntryDeps = []string{
		"vorma_out/chunk-root.js",
		"vorma_out/chunk-b.js",
	}
	stage.DepToCSSBundleMap = map[string][]string{
		"vorma_out/client-entry.js": {"vorma_out/client.css"},
		"vorma_out/chunk-root.js":   {"vorma_out/root.css"},
		"vorma_out/chunk-a.js":      {"vorma_out/a.css"},
		"vorma_out/chunk-b.js": {
			"vorma_out/b.css",
			"vorma_out/shared.css",
		},
		"vorma_out/chunk-c.js": {
			"vorma_out/c.css",
			"vorma_out/shared.css",
		},
	}

	fixture := newTestFixture(b, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app
	app.SetIsDev(false)

	nestedmux.AddPatternWithoutHandler(
		app.LoadersRouter().NestedRouter,
		"/products/:id",
	)
	req := httptest.NewRequest(http.MethodGet, "/products/123", nil)
	findResults, found := nestedmux.FindMatches(
		app.LoadersRouter().NestedRouter,
		req,
	)
	if !found {
		b.Fatal("failed to find nested match for benchmark setup")
	}

	pathsSnapshot := app.Paths()
	routePipelineSnapshot := captureRuntimeServingSnapshot(
		runtimeServingSnapshotInput{
			Paths: toRuntimeCoreRoutePaths(pathsSnapshot),
		},
	).ToRoutePipelineSnapshot()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		deps := routepipeline.GetDepsFromData(
			findResults.Matches,
			routePipelineSnapshot.Paths,
			app.ClientEntryDeps(),
		)
		css := routepipeline.GetCSSBundles(
			deps,
			app.ClientEntryOut(),
			app.DepToCSSBundleMap(),
		)
		benchDepsSink = deps
		benchCSSSink = css
	}
}

func BenchmarkLoadersHandler_HTMLCurrentBuild(b *testing.B) {
	paths := map[string]*Path{
		"/items/:id": {
			OriginalPattern: "/items/:id",
			SrcPath:         "frontend/src/routes/items.$id.tsx",
			OutPath:         "vorma_out/routes/items.$id.js",
			ExportKey:       "default",
			Deps: []string{
				"vorma_out/chunk-items.js",
				"vorma_out/chunk-shared.js",
				"vorma_out/chunk-details.js",
			},
		},
	}
	stage := defaultPathsFile("bench-html-build", paths)
	stage.ClientEntryDeps = []string{"vorma_out/chunk-shared.js"}
	stage.DepToCSSBundleMap = map[string][]string{
		"vorma_out/client-entry.js": {"vorma_out/client-entry.css"},
		"vorma_out/chunk-shared.js": {"vorma_out/chunk-shared.css"},
		"vorma_out/chunk-items.js":  {"vorma_out/chunk-items.css"},
		"vorma_out/chunk-details.js": {
			"vorma_out/chunk-details.css",
			"vorma_out/chunk-shared.css",
		},
	}

	fixture := newTestFixture(b, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app
	app.SetIsDev(false)

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/items/:id",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
				return map[string]any{"id": rd.Params()["id"]}, nil
			},
		),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())
	url := "/items/42"

	warmReq := httptest.NewRequest(http.MethodGet, url, nil)
	warmRec := httptest.NewRecorder()
	handler.ServeHTTP(warmRec, warmReq)
	if warmRec.Code != http.StatusOK {
		b.Fatalf("warmup status = %d, want %d", warmRec.Code, http.StatusOK)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		benchBodyLenSink = rec.Body.Len()
	}
}

func BenchmarkLoadersHandler_JSONCurrentBuild_ColdCache(b *testing.B) {
	paths := map[string]*Path{
		"/items/:id": {
			OriginalPattern: "/items/:id",
			SrcPath:         "frontend/src/routes/items.$id.tsx",
			OutPath:         "vorma_out/routes/items.$id.js",
			ExportKey:       "default",
			Deps: []string{
				"vorma_out/chunk-items.js",
				"vorma_out/chunk-shared.js",
				"vorma_out/chunk-details.js",
			},
		},
	}
	stage := defaultPathsFile("bench-cold-build", paths)
	stage.ClientEntryDeps = []string{"vorma_out/chunk-shared.js"}
	stage.DepToCSSBundleMap = map[string][]string{
		"vorma_out/client-entry.js": {"vorma_out/client-entry.css"},
		"vorma_out/chunk-shared.js": {"vorma_out/chunk-shared.css"},
		"vorma_out/chunk-items.js":  {"vorma_out/chunk-items.css"},
		"vorma_out/chunk-details.js": {
			"vorma_out/chunk-details.css",
			"vorma_out/chunk-shared.css",
		},
	}

	fixture := newTestFixture(b, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app
	app.SetIsDev(false)

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/items/:id",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
				return map[string]any{"id": rd.Params()["id"]}, nil
			},
		),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())
	url := "/items/42?vorma_json=" + app.BuildID()

	warmReq := httptest.NewRequest(http.MethodGet, url, nil)
	warmRec := httptest.NewRecorder()
	handler.ServeHTTP(warmRec, warmReq)
	if warmRec.Code != http.StatusOK {
		b.Fatalf("warmup status = %d, want %d", warmRec.Code, http.StatusOK)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		clearGMPDCacheForBenchmark(app)
		b.StartTimer()

		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		benchBodyLenSink = rec.Body.Len()
	}
}

func BenchmarkSSRInnerHTMLGeneration(b *testing.B) {
	stage := defaultPathsFile("bench-ssr", map[string]*Path{
		"/items/:id": {
			OriginalPattern: "/items/:id",
			SrcPath:         "frontend/src/routes/items.$id.tsx",
			OutPath:         "vorma_out/routes/items.$id.js",
			ExportKey:       "default",
			ErrorExportKey:  "ItemErrorBoundary",
			Deps:            []string{"vorma_out/chunk-items.js"},
		},
	})
	stage.RouteManifestFile = "vorma_out/route-manifest.js"

	fixture := newTestFixture(b, testFixtureOptions{
		stageOne:         stage,
		stageTwo:         stage,
		publicPathPrefix: "/static/",
	})
	app := fixture.app
	app.SetIsDev(false)

	routeData := &routepipeline.RouteDataFinal{
		RouteDataCore: &routepipeline.RouteDataCore{
			OutermostServerError: "",
			ErrorExportKeys:      []string{"ItemErrorBoundary"},
			MatchedPatterns:      []string{"/items/:id"},
			LoadersData:          []any{map[string]any{"id": "42"}},
			ImportURLs:           []string{"/vorma_out/routes/items.$id.js"},
			ExportKeys:           []string{"default"},
			HasRootData:          false,
			Params:               mux.Params{"id": "42"},
			SplatValues:          []string{"detail"},
			Deps:                 []string{"vorma_out/chunk-items.js"},
		},
		CSSBundles: []string{"vorma_out/chunk-items.css"},
		ViteDevURL: "",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		out, err := app.getSSRInnerHTML(routeData)
		if err != nil {
			b.Fatalf("getSSRInnerHTML: %v", err)
		}
		benchStringSink = out.Sha256Hash
	}
}

func BenchmarkActionsHandler_POSTJSONCurrentBuild(b *testing.B) {
	fixture := newTestFixture(b, testFixtureOptions{})
	app := fixture.app
	app.SetIsDev(false)

	mux.AddTaskHandler(
		app.ActionsRouter().Router,
		http.MethodPost,
		"/echo",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[benchActionInput]) (benchActionInput, error) {
				return rd.Input(), nil
			},
		),
	)

	handler := app.Actions().Handler()
	body := `{"name":"bench","count":7}`

	warmReq := httptest.NewRequest(
		http.MethodPost,
		"/api/echo",
		strings.NewReader(body),
	)
	warmReq.Header.Set("Content-Type", "application/json")
	warmRec := httptest.NewRecorder()
	handler.ServeHTTP(warmRec, warmReq)
	if warmRec.Code != http.StatusOK {
		b.Fatalf("warmup status = %d, want %d", warmRec.Code, http.StatusOK)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/echo",
			strings.NewReader(body),
		)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		benchBodyLenSink = rec.Body.Len()
	}
}

func BenchmarkActionsHandler_GETQueryCurrentBuild(b *testing.B) {
	fixture := newTestFixture(b, testFixtureOptions{})
	app := fixture.app
	app.SetIsDev(false)

	mux.AddTaskHandler(
		app.ActionsRouter().Router,
		http.MethodGet,
		"/echo",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[benchActionInput]) (benchActionInput, error) {
				return rd.Input(), nil
			},
		),
	)

	handler := app.Actions().Handler()
	url := "/api/echo?name=bench&count=7"

	warmReq := httptest.NewRequest(http.MethodGet, url, nil)
	warmRec := httptest.NewRecorder()
	handler.ServeHTTP(warmRec, warmReq)
	if warmRec.Code != http.StatusOK {
		b.Fatalf("warmup status = %d, want %d", warmRec.Code, http.StatusOK)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		benchBodyLenSink = rec.Body.Len()
	}
}

func clearGMPDCacheForBenchmark(app *Vorma) {
	if app == nil {
		return
	}
	app.WithLock(func(lv *LockedVorma) {
		lv.v._routeDataCache = &sync.Map{}
	})
}
