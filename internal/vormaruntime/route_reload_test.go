package vormaruntime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime/routepipeline"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
)

func TestDevReloadRoutesFromDisk_UpdatesBuildAndPreservesServerRoutes(
	t *testing.T,
) {
	initialPaths := map[string]*Path{
		"/old": {
			OriginalPattern: "/old",
			SrcPath:         "frontend/src/routes/old.tsx",
			OutPath:         "vorma_out/routes/old.js",
			ExportKey:       "default",
		},
	}
	initial := defaultPathsFile("build-old", initialPaths)
	initial.Stage = "stage-one"

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: initial,
		stageTwo: initial,
	})
	app := fixture.app
	app.SetIsDev(true)

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/server-only",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
				return map[string]any{"source": "server"}, nil
			},
		),
	)

	loadersHandler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	reqBefore := httptest.NewRequest(
		http.MethodGet,
		"/new?vorma_json=build-old",
		nil,
	)
	recBefore := httptest.NewRecorder()
	loadersHandler.ServeHTTP(recBefore, reqBefore)
	if recBefore.Code != http.StatusNotFound {
		t.Fatalf(
			"before reload status = %d, want %d",
			recBefore.Code,
			http.StatusNotFound,
		)
	}

	reloadedPaths := map[string]*Path{
		"/new": {
			OriginalPattern: "/new",
			SrcPath:         "frontend/src/routes/new.tsx",
			OutPath:         "vorma_out/routes/new.js",
			ExportKey:       "default",
		},
	}
	reloaded := defaultPathsFile("build-new", reloadedPaths)
	reloaded.Stage = "stage-one"
	mustWriteJSONFile(
		t,
		filepath.Join(
			fixture.privateDir,
			runtimepaths.VormaOutDirname,
			runtimepaths.VormaPathsStageOneJSONFileName,
		),
		reloaded,
	)

	if err := app.devReloadRoutesFromDisk(); err != nil {
		t.Fatalf("devReloadRoutesFromDisk returned error: %v", err)
	}
	if app.BuildID() != "build-new" {
		t.Fatalf("build ID = %q, want %q", app.BuildID(), "build-new")
	}

	reqNew := httptest.NewRequest(
		http.MethodGet,
		"/new?vorma_json=build-new",
		nil,
	)
	recNew := httptest.NewRecorder()
	loadersHandler.ServeHTTP(recNew, reqNew)
	if recNew.Code != http.StatusOK {
		t.Fatalf("new route status = %d, want %d", recNew.Code, http.StatusOK)
	}

	var routeData routepipeline.RouteDataFinal
	if err := json.Unmarshal(recNew.Body.Bytes(), &routeData); err != nil {
		t.Fatalf("decode route data: %v", err)
	}
	if len(routeData.MatchedPatterns) != 1 ||
		routeData.MatchedPatterns[0] != "/new" {
		t.Fatalf(
			"MatchedPatterns = %#v, want %#v",
			routeData.MatchedPatterns,
			[]string{"/new"},
		)
	}

	reqServer := httptest.NewRequest(
		http.MethodGet,
		"/server-only?vorma_json=build-new",
		nil,
	)
	recServer := httptest.NewRecorder()
	loadersHandler.ServeHTTP(recServer, reqServer)
	if recServer.Code != http.StatusOK {
		t.Fatalf(
			"server-only route status = %d, want %d",
			recServer.Code,
			http.StatusOK,
		)
	}

	var serverRouteData routepipeline.RouteDataFinal
	if err := json.Unmarshal(recServer.Body.Bytes(), &serverRouteData); err != nil {
		t.Fatalf("decode server route data: %v", err)
	}
	if len(serverRouteData.MatchedPatterns) != 1 ||
		serverRouteData.MatchedPatterns[0] != "/server-only" {
		t.Fatalf(
			"server MatchedPatterns = %#v",
			serverRouteData.MatchedPatterns,
		)
	}
}

func TestDevReloadRoutesFromDisk_UpdatesClientEntryDepsAndCSSArtifacts(
	t *testing.T,
) {
	oldStage := defaultPathsFile("build-old", map[string]*Path{
		"/page": {
			OriginalPattern: "/page",
			SrcPath:         "frontend/src/routes/page.old.tsx",
			OutPath:         "vorma_out/routes/page.old.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/route-dep-old.js"},
		},
	})
	oldStage.Stage = "stage-one"
	oldStage.ClientEntryOut = "vorma_out/client-entry-old.js"
	oldStage.ClientEntryDeps = []string{"vorma_out/client-dep-old.js"}
	oldStage.DepToCSSBundleMap = map[string][]string{
		"vorma_out/client-entry-old.js": {"vorma_out/client-entry-old.css"},
		"vorma_out/client-dep-old.js":   {"vorma_out/client-dep-old.css"},
		"vorma_out/route-dep-old.js":    {"vorma_out/route-dep-old.css"},
	}

	newStage := defaultPathsFile("build-new", map[string]*Path{
		"/page": {
			OriginalPattern: "/page",
			SrcPath:         "frontend/src/routes/page.new.tsx",
			OutPath:         "vorma_out/routes/page.new.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/route-dep-new.js"},
		},
	})
	newStage.Stage = "stage-one"
	newStage.ClientEntryOut = "vorma_out/client-entry-new.js"
	newStage.ClientEntryDeps = []string{"vorma_out/client-dep-new.js"}
	newStage.DepToCSSBundleMap = map[string][]string{
		"vorma_out/client-entry-new.js": {"vorma_out/client-entry-new.css"},
		"vorma_out/client-dep-new.js":   {"vorma_out/client-dep-new.css"},
		"vorma_out/route-dep-new.js":    {"vorma_out/route-dep-new.css"},
	}

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: oldStage,
		stageTwo: oldStage,
	})
	app := fixture.app
	app.SetIsDev(true)

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/page",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]bool, error) {
				return map[string]bool{"ok": true}, nil
			},
		),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	reqOld := httptest.NewRequest(
		http.MethodGet,
		"/page?vorma_json=build-old",
		nil,
	)
	recOld := httptest.NewRecorder()
	handler.ServeHTTP(recOld, reqOld)
	if recOld.Code != http.StatusOK {
		t.Fatalf("old build status = %d, want %d", recOld.Code, http.StatusOK)
	}

	var oldData routepipeline.RouteDataFinal
	if err := json.Unmarshal(recOld.Body.Bytes(), &oldData); err != nil {
		t.Fatalf("decode old route data: %v", err)
	}
	if !containsString(oldData.Deps, "vorma_out/client-dep-old.js") {
		t.Fatalf("old deps missing old client dep: %#v", oldData.Deps)
	}
	if !containsString(oldData.CSSBundles, "vorma_out/client-entry-old.css") {
		t.Fatalf(
			"old css bundles missing old client-entry css: %#v",
			oldData.CSSBundles,
		)
	}

	mustWriteJSONFile(
		t,
		filepath.Join(
			fixture.privateDir,
			runtimepaths.VormaOutDirname,
			runtimepaths.VormaPathsStageOneJSONFileName,
		),
		newStage,
	)
	if err := app.devReloadRoutesFromDisk(); err != nil {
		t.Fatalf("devReloadRoutesFromDisk returned error: %v", err)
	}

	if got, want := app.BuildID(), "build-new"; got != want {
		t.Fatalf("build ID = %q, want %q", got, want)
	}
	if got, want := app.ClientEntryOut(), "vorma_out/client-entry-new.js"; got != want {
		t.Fatalf("client entry out = %q, want %q", got, want)
	}
	if got := app.ClientEntryDeps(); !containsString(
		got,
		"vorma_out/client-dep-new.js",
	) {
		t.Fatalf("client entry deps = %#v, expected new client dep", got)
	}

	reqNew := httptest.NewRequest(
		http.MethodGet,
		"/page?vorma_json=build-new",
		nil,
	)
	recNew := httptest.NewRecorder()
	handler.ServeHTTP(recNew, reqNew)
	if recNew.Code != http.StatusOK {
		t.Fatalf("new build status = %d, want %d", recNew.Code, http.StatusOK)
	}

	var newData routepipeline.RouteDataFinal
	if err := json.Unmarshal(recNew.Body.Bytes(), &newData); err != nil {
		t.Fatalf("decode new route data: %v", err)
	}

	if !containsString(newData.Deps, "vorma_out/client-dep-new.js") {
		t.Fatalf("new deps missing new client dep: %#v", newData.Deps)
	}
	if containsString(newData.Deps, "vorma_out/client-dep-old.js") {
		t.Fatalf(
			"new deps should not include old client dep: %#v",
			newData.Deps,
		)
	}
	if !containsString(newData.CSSBundles, "vorma_out/client-entry-new.css") {
		t.Fatalf(
			"new css bundles missing new client-entry css: %#v",
			newData.CSSBundles,
		)
	}
	if containsString(newData.CSSBundles, "vorma_out/client-entry-old.css") {
		t.Fatalf(
			"new css bundles should not include old client-entry css: %#v",
			newData.CSSBundles,
		)
	}
}

func TestDevReloadRoutesFromDisk_ClearsOmittedClientEntryDepsAndCSSArtifacts(
	t *testing.T,
) {
	oldStage := defaultPathsFile("build-old", map[string]*Path{
		"/page": {
			OriginalPattern: "/page",
			SrcPath:         "frontend/src/routes/page.old.tsx",
			OutPath:         "vorma_out/routes/page.old.js",
			ExportKey:       "default",
		},
	})
	oldStage.Stage = "stage-one"
	oldStage.ClientEntryOut = "vorma_out/client-entry-old.js"
	oldStage.ClientEntryDeps = []string{"vorma_out/client-dep-old.js"}
	oldStage.DepToCSSBundleMap = map[string][]string{
		"vorma_out/client-entry-old.js": {"vorma_out/client-entry-old.css"},
		"vorma_out/client-dep-old.js":   {"vorma_out/client-dep-old.css"},
	}

	newStage := defaultPathsFile("build-new", map[string]*Path{
		"/page": {
			OriginalPattern: "/page",
			SrcPath:         "frontend/src/routes/page.new.tsx",
			OutPath:         "vorma_out/routes/page.new.js",
			ExportKey:       "default",
		},
	})
	newStage.Stage = "stage-one"
	newStage.ClientEntryOut = "vorma_out/client-entry-new.js"
	newStage.ClientEntryDeps = nil
	newStage.DepToCSSBundleMap = nil

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: oldStage,
		stageTwo: oldStage,
	})
	app := fixture.app
	app.SetIsDev(true)

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/page",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]bool, error) {
				return map[string]bool{"ok": true}, nil
			},
		),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	reqOld := httptest.NewRequest(
		http.MethodGet,
		"/page?vorma_json=build-old",
		nil,
	)
	recOld := httptest.NewRecorder()
	handler.ServeHTTP(recOld, reqOld)
	if recOld.Code != http.StatusOK {
		t.Fatalf("old build status = %d, want %d", recOld.Code, http.StatusOK)
	}

	var oldData routepipeline.RouteDataFinal
	if err := json.Unmarshal(recOld.Body.Bytes(), &oldData); err != nil {
		t.Fatalf("decode old route data: %v", err)
	}
	if !containsString(oldData.Deps, "vorma_out/client-dep-old.js") {
		t.Fatalf("old deps missing old client dep: %#v", oldData.Deps)
	}
	if !containsString(oldData.CSSBundles, "vorma_out/client-entry-old.css") {
		t.Fatalf(
			"old css bundles missing old client-entry css: %#v",
			oldData.CSSBundles,
		)
	}

	mustWriteJSONFile(
		t,
		filepath.Join(
			fixture.privateDir,
			runtimepaths.VormaOutDirname,
			runtimepaths.VormaPathsStageOneJSONFileName,
		),
		newStage,
	)
	if err := app.devReloadRoutesFromDisk(); err != nil {
		t.Fatalf("devReloadRoutesFromDisk returned error: %v", err)
	}

	if got := app.ClientEntryDeps(); got != nil {
		t.Fatalf("client entry deps = %#v, want nil after reload omission", got)
	}
	cssMap := app.DepToCSSBundleMap()
	if cssMap == nil {
		t.Fatal("dep-to-css map should be normalized to empty map, got nil")
	}
	if len(cssMap) != 0 {
		t.Fatalf(
			"dep-to-css map len = %d, want 0 after reload omission",
			len(cssMap),
		)
	}

	reqNew := httptest.NewRequest(
		http.MethodGet,
		"/page?vorma_json=build-new",
		nil,
	)
	recNew := httptest.NewRecorder()
	handler.ServeHTTP(recNew, reqNew)
	if recNew.Code != http.StatusOK {
		t.Fatalf("new build status = %d, want %d", recNew.Code, http.StatusOK)
	}

	var newData routepipeline.RouteDataFinal
	if err := json.Unmarshal(recNew.Body.Bytes(), &newData); err != nil {
		t.Fatalf("decode new route data: %v", err)
	}
	if containsString(newData.Deps, "vorma_out/client-dep-old.js") {
		t.Fatalf(
			"new deps should not include old client dep: %#v",
			newData.Deps,
		)
	}
	if containsString(newData.CSSBundles, "vorma_out/client-entry-old.css") {
		t.Fatalf(
			"new css bundles should not include old client-entry css: %#v",
			newData.CSSBundles,
		)
	}
}

func TestDevReloadRoutesFromDisk_InvalidPathsFileDoesNotMutateRuntimeState(
	t *testing.T,
) {
	initialPaths := map[string]*Path{
		"/old": {
			OriginalPattern: "/old",
			SrcPath:         "frontend/src/routes/old.tsx",
			OutPath:         "vorma_out/routes/old.js",
			ExportKey:       "default",
		},
	}
	initial := defaultPathsFile("build-old", initialPaths)
	initial.Stage = "stage-one"

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: initial,
		stageTwo: initial,
	})
	app := fixture.app
	app.SetIsDev(true)

	loadersHandler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	reqBefore := httptest.NewRequest(
		http.MethodGet,
		"/old?vorma_json=build-old",
		nil,
	)
	recBefore := httptest.NewRecorder()
	loadersHandler.ServeHTTP(recBefore, reqBefore)
	if recBefore.Code != http.StatusOK {
		t.Fatalf(
			"before reload status = %d, want %d",
			recBefore.Code,
			http.StatusOK,
		)
	}

	mustWriteFile(
		t,
		filepath.Join(
			fixture.privateDir,
			runtimepaths.VormaOutDirname,
			runtimepaths.VormaPathsStageOneJSONFileName,
		),
		[]byte(
			`{"stage":"stage-one","buildID":"build-new-invalid","clientEntrySrc":"frontend/src/vorma.entry.tsx","paths":{"/bad":null},"routeManifestFile":"vorma_out/route-manifest.js"}`,
		),
	)

	err := app.devReloadRoutesFromDisk()
	if err == nil {
		t.Fatal(
			"expected devReloadRoutesFromDisk to fail for invalid paths file",
		)
	}
	if !strings.Contains(err.Error(), "cannot be null") {
		t.Fatalf("error = %q, expected null path validation context", err)
	}

	if got, want := app.BuildID(), "build-old"; got != want {
		t.Fatalf("build ID = %q, want %q after failed reload", got, want)
	}

	reqAfter := httptest.NewRequest(
		http.MethodGet,
		"/old?vorma_json=build-old",
		nil,
	)
	recAfter := httptest.NewRecorder()
	loadersHandler.ServeHTTP(recAfter, reqAfter)
	if recAfter.Code != http.StatusOK {
		t.Fatalf(
			"after failed reload status = %d, want %d",
			recAfter.Code,
			http.StatusOK,
		)
	}
}

func TestDevReloadRoutesFromDisk_SemanticValidationFailuresDoNotMutateRuntimeState(
	t *testing.T,
) {
	testCases := []struct {
		name                string
		mutateInvalidStage1 func(*runtimepaths.PathsFile)
	}{
		{
			name: "missing_route_manifest_file",
			mutateInvalidStage1: func(pathsFile *runtimepaths.PathsFile) {
				pathsFile.RouteManifestFile = ""
			},
		},
		{
			name: "missing_export_key_on_client_route",
			mutateInvalidStage1: func(pathsFile *runtimepaths.PathsFile) {
				pathsFile.Paths["/products/:id"].ExportKey = ""
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			initialStage := defaultPathsFile("build-old", map[string]*Path{
				"/products/:id": {
					OriginalPattern: "/products/:id",
					SrcPath:         "frontend/src/routes/products.$id.old.tsx",
					OutPath:         "vorma_out/routes/products.$id.old.js",
					ExportKey:       "default",
					Deps:            []string{"vorma_out/chunk-old.js"},
				},
			})
			initialStage.Stage = "stage-one"

			fixture := newTestFixture(t, testFixtureOptions{
				stageOne: initialStage,
				stageTwo: initialStage,
			})
			app := fixture.app
			app.SetIsDev(true)

			handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

			recBefore := httptest.NewRecorder()
			reqBefore := httptest.NewRequest(
				http.MethodGet,
				"/products/1?vorma_json=build-old",
				nil,
			)
			handler.ServeHTTP(recBefore, reqBefore)
			if recBefore.Code != http.StatusOK {
				t.Fatalf(
					"before failed reload status = %d, want %d",
					recBefore.Code,
					http.StatusOK,
				)
			}
			if !strings.Contains(
				recBefore.Body.String(),
				"/frontend/src/routes/products.$id.old.tsx",
			) {
				t.Fatalf(
					"before failed reload body missing old import URL, body=%q",
					recBefore.Body.String(),
				)
			}

			invalidStage := defaultPathsFile("build-invalid", map[string]*Path{
				"/products/:id": {
					OriginalPattern: "/products/:id",
					SrcPath:         "frontend/src/routes/products.$id.invalid.tsx",
					OutPath:         "vorma_out/routes/products.$id.invalid.js",
					ExportKey:       "default",
					Deps:            []string{"vorma_out/chunk-invalid.js"},
				},
			})
			invalidStage.Stage = "stage-one"
			tc.mutateInvalidStage1(invalidStage)

			mustWriteJSONFile(
				t,
				filepath.Join(
					fixture.privateDir,
					runtimepaths.VormaOutDirname,
					runtimepaths.VormaPathsStageOneJSONFileName,
				),
				invalidStage,
			)

			err := app.devReloadRoutesFromDisk()
			if err == nil {
				t.Fatal(
					"expected devReloadRoutesFromDisk to fail for semantic stage-one artifact validation issue",
				)
			}

			if got, want := app.BuildID(), "build-old"; got != want {
				t.Fatalf(
					"build ID = %q, want %q after failed semantic reload",
					got,
					want,
				)
			}

			recAfter := httptest.NewRecorder()
			reqAfter := httptest.NewRequest(
				http.MethodGet,
				"/products/2?vorma_json=build-old",
				nil,
			)
			handler.ServeHTTP(recAfter, reqAfter)
			if recAfter.Code != http.StatusOK {
				t.Fatalf(
					"after failed reload status = %d, want %d",
					recAfter.Code,
					http.StatusOK,
				)
			}
			if !strings.Contains(
				recAfter.Body.String(),
				"/frontend/src/routes/products.$id.old.tsx",
			) {
				t.Fatalf(
					"after failed reload body missing old import URL, body=%q",
					recAfter.Body.String(),
				)
			}
			if strings.Contains(
				recAfter.Body.String(),
				"/frontend/src/routes/products.$id.invalid.tsx",
			) {
				t.Fatalf(
					"after failed reload body leaked invalid import URL, body=%q",
					recAfter.Body.String(),
				)
			}
		})
	}
}

func TestDevReloadRoutesFromDisk_NilPathsClearsClientRoutesAndPreservesServerHandlers(
	t *testing.T,
) {
	initial := defaultPathsFile("build-old", map[string]*Path{
		"/client-old": {
			OriginalPattern: "/client-old",
			SrcPath:         "frontend/src/routes/client-old.tsx",
			OutPath:         "vorma_out/routes/client-old.js",
			ExportKey:       "default",
		},
	})
	initial.Stage = "stage-one"

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: initial,
		stageTwo: initial,
	})
	app := fixture.app
	app.SetIsDev(true)

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/server-only",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
				return map[string]any{"source": "server"}, nil
			},
		),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	reqClientBefore := httptest.NewRequest(
		http.MethodGet,
		"/client-old?vorma_json=build-old",
		nil,
	)
	recClientBefore := httptest.NewRecorder()
	handler.ServeHTTP(recClientBefore, reqClientBefore)
	if recClientBefore.Code != http.StatusOK {
		t.Fatalf(
			"before reload /client-old status = %d, want %d",
			recClientBefore.Code,
			http.StatusOK,
		)
	}

	nilPathsStage := &runtimepaths.PathsFile{
		Stage:             "stage-one",
		BuildID:           "build-new",
		ClientEntrySrc:    "frontend/src/vorma.entry.tsx",
		ClientEntryOut:    "vorma_out/client-entry.js",
		ClientEntryDeps:   nil,
		Paths:             nil,
		RouteManifestFile: "vorma_out/route-manifest.js",
		DepToCSSBundleMap: nil,
	}
	mustWriteJSONFile(
		t,
		filepath.Join(
			fixture.privateDir,
			runtimepaths.VormaOutDirname,
			runtimepaths.VormaPathsStageOneJSONFileName,
		),
		nilPathsStage,
	)

	if err := app.devReloadRoutesFromDisk(); err != nil {
		t.Fatalf("devReloadRoutesFromDisk returned error: %v", err)
	}

	reqClientAfter := httptest.NewRequest(
		http.MethodGet,
		"/client-old?vorma_json=build-new",
		nil,
	)
	recClientAfter := httptest.NewRecorder()
	handler.ServeHTTP(recClientAfter, reqClientAfter)
	if recClientAfter.Code != http.StatusNotFound {
		t.Fatalf(
			"after reload /client-old status = %d, want %d",
			recClientAfter.Code,
			http.StatusNotFound,
		)
	}

	reqServerAfter := httptest.NewRequest(
		http.MethodGet,
		"/server-only?vorma_json=build-new",
		nil,
	)
	recServerAfter := httptest.NewRecorder()
	handler.ServeHTTP(recServerAfter, reqServerAfter)
	if recServerAfter.Code != http.StatusOK {
		t.Fatalf(
			"after reload /server-only status = %d, want %d",
			recServerAfter.Code,
			http.StatusOK,
		)
	}
	if got := recServerAfter.Header().Get(VormaBuildIDHeaderKey); got != "build-new" {
		t.Fatalf("after reload build header = %q, want %q", got, "build-new")
	}
}

func TestDevReloadTemplateFromDisk_UsesUpdatedTemplate(t *testing.T) {
	stage2 := defaultPathsFile("build-template", map[string]*Path{
		"/template": {
			OriginalPattern: "/template",
			SrcPath:         "frontend/src/routes/template.tsx",
			OutPath:         "vorma_out/routes/template.js",
			ExportKey:       "default",
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage2,
		stageTwo: stage2,
		template: "<!doctype html><html><body>OLD TEMPLATE {{.VormaBodyScripts}}</body></html>",
	})
	app := fixture.app
	app.SetIsDev(true)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	reqOld := httptest.NewRequest(http.MethodGet, "/template", nil)
	recOld := httptest.NewRecorder()
	handler.ServeHTTP(recOld, reqOld)
	if recOld.Code != http.StatusOK {
		t.Fatalf(
			"old template status = %d, want %d",
			recOld.Code,
			http.StatusOK,
		)
	}
	if !strings.Contains(recOld.Body.String(), "OLD TEMPLATE") {
		t.Fatalf("expected old template marker, body=%q", recOld.Body.String())
	}

	mustWriteFile(
		t,
		filepath.Join(fixture.privateDir, "entry.go.html"),
		[]byte(
			"<!doctype html><html><body>NEW TEMPLATE {{.VormaBodyScripts}}</body></html>",
		),
	)
	if err := app.devReloadTemplateFromDisk(); err != nil {
		t.Fatalf("devReloadTemplateFromDisk returned error: %v", err)
	}

	reqNew := httptest.NewRequest(http.MethodGet, "/template", nil)
	recNew := httptest.NewRecorder()
	handler.ServeHTTP(recNew, reqNew)
	if recNew.Code != http.StatusOK {
		t.Fatalf(
			"new template status = %d, want %d",
			recNew.Code,
			http.StatusOK,
		)
	}
	if !strings.Contains(recNew.Body.String(), "NEW TEMPLATE") {
		t.Fatalf("expected new template marker, body=%q", recNew.Body.String())
	}
}

func TestDevReloadTemplateFromDisk_ParseFailureDoesNotMutateTemplate(
	t *testing.T,
) {
	stage2 := defaultPathsFile("build-template-failure", map[string]*Path{
		"/template": {
			OriginalPattern: "/template",
			SrcPath:         "frontend/src/routes/template.tsx",
			OutPath:         "vorma_out/routes/template.js",
			ExportKey:       "default",
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage2,
		stageTwo: stage2,
		template: "<!doctype html><html><body>STABLE TEMPLATE {{.VormaBodyScripts}}</body></html>",
	})
	app := fixture.app
	app.SetIsDev(true)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	reqBefore := httptest.NewRequest(http.MethodGet, "/template", nil)
	recBefore := httptest.NewRecorder()
	handler.ServeHTTP(recBefore, reqBefore)
	if recBefore.Code != http.StatusOK {
		t.Fatalf(
			"pre-failure template status = %d, want %d",
			recBefore.Code,
			http.StatusOK,
		)
	}
	if !strings.Contains(recBefore.Body.String(), "STABLE TEMPLATE") {
		t.Fatalf(
			"expected stable template marker before failure, body=%q",
			recBefore.Body.String(),
		)
	}

	mustWriteFile(
		t,
		filepath.Join(fixture.privateDir, "entry.go.html"),
		[]byte(
			"<!doctype html><html><body>BROKEN TEMPLATE {{.VormaBodyScripts</body></html>",
		),
	)

	err := app.devReloadTemplateFromDisk()
	if err == nil {
		t.Fatal(
			"expected devReloadTemplateFromDisk to fail for malformed template",
		)
	}
	if !strings.Contains(err.Error(), "parse template") {
		t.Fatalf("error = %q, expected parse template context", err)
	}

	reqAfter := httptest.NewRequest(http.MethodGet, "/template", nil)
	recAfter := httptest.NewRecorder()
	handler.ServeHTTP(recAfter, reqAfter)
	if recAfter.Code != http.StatusOK {
		t.Fatalf(
			"post-failure template status = %d, want %d",
			recAfter.Code,
			http.StatusOK,
		)
	}
	if !strings.Contains(recAfter.Body.String(), "STABLE TEMPLATE") {
		t.Fatalf(
			"expected stable template marker after failed reload, body=%q",
			recAfter.Body.String(),
		)
	}
}

func TestDevReloadRoutesFromDisk_RebuildsRouteDataForSamePatternAcrossBuilds(
	t *testing.T,
) {
	oldStage := defaultPathsFile("build-old", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.old.tsx",
			OutPath:         "vorma_out/routes/products.$id.old.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-old.js"},
		},
	})
	newStage := defaultPathsFile("build-new", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.new.tsx",
			OutPath:         "vorma_out/routes/products.$id.new.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-new.js"},
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: oldStage,
		stageTwo: oldStage,
	})
	app := fixture.app
	app.SetIsDev(true)

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/products/:id",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
				return map[string]any{"id": rd.Params()["id"]}, nil
			},
		),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	reqOld := httptest.NewRequest(
		http.MethodGet,
		"/products/1?vorma_json=build-old",
		nil,
	)
	recOld := httptest.NewRecorder()
	handler.ServeHTTP(recOld, reqOld)
	if recOld.Code != http.StatusOK {
		t.Fatalf("old build status = %d, want %d", recOld.Code, http.StatusOK)
	}

	var oldData routepipeline.RouteDataFinal
	if err := json.Unmarshal(recOld.Body.Bytes(), &oldData); err != nil {
		t.Fatalf("decode old route data: %v", err)
	}
	if got, want := oldData.ImportURLs, []string{"/frontend/src/routes/products.$id.old.tsx"}; !slicesEqual(
		got,
		want,
	) {
		t.Fatalf("old ImportURLs = %#v, want %#v", got, want)
	}
	if !containsString(oldData.Deps, "vorma_out/chunk-old.js") {
		t.Fatalf("old Deps missing old chunk: %#v", oldData.Deps)
	}

	mustWriteJSONFile(
		t,
		filepath.Join(
			fixture.privateDir,
			runtimepaths.VormaOutDirname,
			runtimepaths.VormaPathsStageOneJSONFileName,
		),
		newStage,
	)
	if err := app.devReloadRoutesFromDisk(); err != nil {
		t.Fatalf("devReloadRoutesFromDisk returned error: %v", err)
	}

	reqNew := httptest.NewRequest(
		http.MethodGet,
		"/products/2?vorma_json=build-new",
		nil,
	)
	recNew := httptest.NewRecorder()
	handler.ServeHTTP(recNew, reqNew)
	if recNew.Code != http.StatusOK {
		t.Fatalf("new build status = %d, want %d", recNew.Code, http.StatusOK)
	}

	var newData routepipeline.RouteDataFinal
	if err := json.Unmarshal(recNew.Body.Bytes(), &newData); err != nil {
		t.Fatalf("decode new route data: %v", err)
	}
	if got, want := newData.ImportURLs, []string{"/frontend/src/routes/products.$id.new.tsx"}; !slicesEqual(
		got,
		want,
	) {
		t.Fatalf("new ImportURLs = %#v, want %#v", got, want)
	}
	if !containsString(newData.Deps, "vorma_out/chunk-new.js") {
		t.Fatalf("new Deps missing new chunk: %#v", newData.Deps)
	}
	if containsString(newData.Deps, "vorma_out/chunk-old.js") {
		t.Fatalf("new Deps should not include old chunk: %#v", newData.Deps)
	}
}

func TestDevReloadRoutesFromDisk_RebuildsRouteDataWhenBuildIDUnchanged(
	t *testing.T,
) {
	oldStage := defaultPathsFile("build-same", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.old.tsx",
			OutPath:         "vorma_out/routes/products.$id.old.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-old.js"},
		},
	})
	oldStage.Stage = "stage-one"

	newStage := defaultPathsFile("build-same", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.new.tsx",
			OutPath:         "vorma_out/routes/products.$id.new.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-new.js"},
		},
	})
	newStage.Stage = "stage-one"

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: oldStage,
		stageTwo: oldStage,
	})
	app := fixture.app
	app.SetIsDev(true)

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/products/:id",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
				return map[string]any{"id": rd.Params()["id"]}, nil
			},
		),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	reqOld := httptest.NewRequest(
		http.MethodGet,
		"/products/1?vorma_json=build-same",
		nil,
	)
	recOld := httptest.NewRecorder()
	handler.ServeHTTP(recOld, reqOld)
	if recOld.Code != http.StatusOK {
		t.Fatalf(
			"old response status = %d, want %d",
			recOld.Code,
			http.StatusOK,
		)
	}

	var oldData routepipeline.RouteDataFinal
	if err := json.Unmarshal(recOld.Body.Bytes(), &oldData); err != nil {
		t.Fatalf("decode old route data: %v", err)
	}
	if got, want := oldData.ImportURLs, []string{"/frontend/src/routes/products.$id.old.tsx"}; !slicesEqual(
		got,
		want,
	) {
		t.Fatalf("old ImportURLs = %#v, want %#v", got, want)
	}
	if !containsString(oldData.Deps, "vorma_out/chunk-old.js") {
		t.Fatalf("old Deps missing old chunk: %#v", oldData.Deps)
	}

	mustWriteJSONFile(
		t,
		filepath.Join(
			fixture.privateDir,
			runtimepaths.VormaOutDirname,
			runtimepaths.VormaPathsStageOneJSONFileName,
		),
		newStage,
	)
	if err := app.devReloadRoutesFromDisk(); err != nil {
		t.Fatalf("devReloadRoutesFromDisk returned error: %v", err)
	}
	if got, want := app.BuildID(), "build-same"; got != want {
		t.Fatalf("build ID = %q, want %q", got, want)
	}

	reqNew := httptest.NewRequest(
		http.MethodGet,
		"/products/2?vorma_json=build-same",
		nil,
	)
	recNew := httptest.NewRecorder()
	handler.ServeHTTP(recNew, reqNew)
	if recNew.Code != http.StatusOK {
		t.Fatalf(
			"new response status = %d, want %d",
			recNew.Code,
			http.StatusOK,
		)
	}

	var newData routepipeline.RouteDataFinal
	if err := json.Unmarshal(recNew.Body.Bytes(), &newData); err != nil {
		t.Fatalf("decode new route data: %v", err)
	}
	if got, want := newData.ImportURLs, []string{"/frontend/src/routes/products.$id.new.tsx"}; !slicesEqual(
		got,
		want,
	) {
		t.Fatalf("new ImportURLs = %#v, want %#v", got, want)
	}
	if !containsString(newData.Deps, "vorma_out/chunk-new.js") {
		t.Fatalf("new Deps missing new chunk: %#v", newData.Deps)
	}
	if containsString(newData.Deps, "vorma_out/chunk-old.js") {
		t.Fatalf("new Deps should not include old chunk: %#v", newData.Deps)
	}
}

func TestLoadersHandler_ConcurrentReloadAndRequests_OnlyServeCoherentArtifactSets(
	t *testing.T,
) {
	oldStage := defaultPathsFile("build-old", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.old.tsx",
			OutPath:         "vorma_out/routes/products.$id.old.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-old.js"},
		},
	})
	newStage := defaultPathsFile("build-new", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.new.tsx",
			OutPath:         "vorma_out/routes/products.$id.new.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-new.js"},
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: oldStage,
		stageTwo: oldStage,
	})
	app := fixture.app
	app.SetIsDev(true)

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/products/:id",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
				return map[string]any{"id": rd.Params()["id"]}, nil
			},
		),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())
	stageOnePath := filepath.Join(
		fixture.privateDir,
		runtimepaths.VormaOutDirname,
		runtimepaths.VormaPathsStageOneJSONFileName,
	)

	const reloadIterations = 80
	const workers = 8
	const requestsPerWorker = 80

	checkBodyHasCoherentArtifacts := func(body string) (string, error) {
		hasOldImport := strings.Contains(
			body,
			"/frontend/src/routes/products.$id.old.tsx",
		)
		hasNewImport := strings.Contains(
			body,
			"/frontend/src/routes/products.$id.new.tsx",
		)
		hasOldDep := strings.Contains(body, "vorma_out/chunk-old.js")
		hasNewDep := strings.Contains(body, "vorma_out/chunk-new.js")

		if hasOldImport && hasNewImport {
			return "", fmt.Errorf("response contained mixed import URLs")
		}
		if hasOldDep && hasNewDep {
			return "", fmt.Errorf("response contained mixed dependency chunks")
		}
		if !hasOldImport && !hasNewImport {
			return "", fmt.Errorf(
				"response contained neither old nor new import URL",
			)
		}
		if !hasOldDep && !hasNewDep {
			return "", fmt.Errorf(
				"response contained neither old nor new dependency chunk",
			)
		}

		// Import URL and dependency chunk should describe the same build generation.
		if hasOldImport != hasOldDep || hasNewImport != hasNewDep {
			return "", fmt.Errorf(
				"response mixed old/new artifacts across route imports and deps",
			)
		}

		if hasOldImport {
			return "old", nil
		}
		return "new", nil
	}

	errCh := make(chan error, 1)
	reportErr := func(err error) {
		select {
		case errCh <- err:
		default:
		}
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1 + workers)

	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < reloadIterations; i++ {
			next := oldStage
			if i%2 == 1 {
				next = newStage
			}
			if err := writePathsFileNoTB(stageOnePath, next); err != nil {
				reportErr(fmt.Errorf("write stage-one file: %w", err))
				return
			}
			if err := app.devReloadRoutesFromDisk(); err != nil {
				reportErr(fmt.Errorf("reload routes: %w", err))
				return
			}
		}
	}()

	for workerID := 0; workerID < workers; workerID++ {
		go func(workerID int) {
			defer wg.Done()
			<-start
			for i := 0; i < requestsPerWorker; i++ {
				req := httptest.NewRequest(http.MethodGet, "/products/123", nil)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					reportErr(
						fmt.Errorf(
							"worker %d request %d status=%d",
							workerID,
							i,
							rec.Code,
						),
					)
					return
				}
				artifactGeneration, err := checkBodyHasCoherentArtifacts(
					rec.Body.String(),
				)
				if err != nil {
					reportErr(
						fmt.Errorf(
							"worker %d request %d: %w",
							workerID,
							i,
							err,
						),
					)
					return
				}

				buildIDHeader := rec.Header().Get(VormaBuildIDHeaderKey)
				if artifactGeneration == "old" && buildIDHeader != "build-old" {
					reportErr(fmt.Errorf(
						"worker %d request %d: old artifacts with mismatched build header %q",
						workerID,
						i,
						buildIDHeader,
					))
					return
				}
				if artifactGeneration == "new" && buildIDHeader != "build-new" {
					reportErr(fmt.Errorf(
						"worker %d request %d: new artifacts with mismatched build header %q",
						workerID,
						i,
						buildIDHeader,
					))
					return
				}
			}
		}(workerID)
	}

	close(start)
	wg.Wait()

	select {
	case err := <-errCh:
		t.Fatal(err)
	default:
	}
}

func TestLoadersHandler_ReloadDuringRequest_DoesNotMixCSSFromNewBuild(
	t *testing.T,
) {
	oldStage := defaultPathsFile("build-old", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.old.tsx",
			OutPath:         "vorma_out/routes/products.$id.old.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-old.js"},
		},
	})
	oldStage.ClientEntryOut = "vorma_out/client-old.js"
	oldStage.DepToCSSBundleMap = map[string][]string{
		"vorma_out/client-old.js": {"vorma_out/client-old.css"},
		"vorma_out/chunk-old.js":  {"vorma_out/chunk-old.css"},
	}

	newStage := defaultPathsFile("build-new", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.new.tsx",
			OutPath:         "vorma_out/routes/products.$id.new.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-new.js"},
		},
	})
	newStage.ClientEntryOut = "vorma_out/client-new.js"
	newStage.DepToCSSBundleMap = map[string][]string{
		"vorma_out/client-new.js": {"vorma_out/client-new.css"},
		"vorma_out/chunk-new.js":  {"vorma_out/chunk-new.css"},
	}

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: oldStage,
		stageTwo: oldStage,
	})
	app := fixture.app
	app.SetIsDev(true)

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/products/:id",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
				return map[string]any{"id": rd.Params()["id"]}, nil
			},
		),
	)

	stageOnePath := filepath.Join(
		fixture.privateDir,
		runtimepaths.VormaOutDirname,
		runtimepaths.VormaPathsStageOneJSONFileName,
	)
	var didReload atomic.Bool
	app.getDefaultHeadEls = func(r *http.Request, app *Vorma, h *headels.HeadEls) error {
		if !didReload.CompareAndSwap(false, true) {
			return nil
		}
		mustWriteJSONFile(t, stageOnePath, newStage)
		return app.devReloadRoutesFromDisk()
	}

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())
	req := httptest.NewRequest(
		http.MethodGet,
		"/products/123?vorma_json=build-old",
		nil,
	)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Header().Get(VormaBuildIDHeaderKey), "build-old"; got != want {
		t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, want)
	}
	if !didReload.Load() {
		t.Fatal(
			"expected test hook to trigger a dev reload during request handling",
		)
	}

	var routeData routepipeline.RouteDataFinal
	if err := json.Unmarshal(rec.Body.Bytes(), &routeData); err != nil {
		t.Fatalf("decode route data: %v", err)
	}

	if got, want := routeData.ImportURLs, []string{"/frontend/src/routes/products.$id.old.tsx"}; !slicesEqual(
		got,
		want,
	) {
		t.Fatalf("ImportURLs = %#v, want %#v", got, want)
	}
	if got, want := routeData.Deps, []string{"vorma_out/client-shared.js", "vorma_out/chunk-old.js"}; !slicesEqual(
		got,
		want,
	) {
		t.Fatalf("Deps = %#v, want %#v", got, want)
	}

	wantCSSBundles := []string{
		"vorma_out/client-old.css",
		"vorma_out/chunk-old.css",
	}
	if got := routeData.CSSBundles; !slicesEqual(got, wantCSSBundles) {
		t.Fatalf("CSSBundles = %#v, want %#v", got, wantCSSBundles)
	}
	if containsString(routeData.CSSBundles, "vorma_out/client-new.css") {
		t.Fatalf(
			"CSSBundles should not include new-build client CSS: %#v",
			routeData.CSSBundles,
		)
	}
	if containsString(routeData.CSSBundles, "vorma_out/chunk-new.css") {
		t.Fatalf(
			"CSSBundles should not include new-build route CSS: %#v",
			routeData.CSSBundles,
		)
	}
}

func TestLoadersHandler_ReloadDuringHTMLRequest_KeepsBuildHeaderAndSSRPayloadGenerationAligned(
	t *testing.T,
) {
	oldStage := defaultPathsFile("build-old", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.old.tsx",
			OutPath:         "vorma_out/routes/products.$id.old.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-old.js"},
		},
	})
	oldStage.Stage = "stage-one"
	oldStage.RouteManifestFile = "vorma_out/route-manifest-old.js"
	oldStage.ClientEntryOut = "vorma_out/client-old.js"
	oldStage.ClientEntryDeps = []string{"vorma_out/shared-old.js"}
	oldStage.DepToCSSBundleMap = map[string][]string{
		"vorma_out/client-old.js": {"vorma_out/client-old.css"},
		"vorma_out/shared-old.js": {"vorma_out/shared-old.css"},
		"vorma_out/chunk-old.js":  {"vorma_out/chunk-old.css"},
	}

	newStage := defaultPathsFile("build-new", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.new.tsx",
			OutPath:         "vorma_out/routes/products.$id.new.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-new.js"},
		},
	})
	newStage.Stage = "stage-one"
	newStage.RouteManifestFile = "vorma_out/route-manifest-new.js"
	newStage.ClientEntryOut = "vorma_out/client-new.js"
	newStage.ClientEntryDeps = []string{"vorma_out/shared-new.js"}
	newStage.DepToCSSBundleMap = map[string][]string{
		"vorma_out/client-new.js": {"vorma_out/client-new.css"},
		"vorma_out/shared-new.js": {"vorma_out/shared-new.css"},
		"vorma_out/chunk-new.js":  {"vorma_out/chunk-new.css"},
	}

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne:         oldStage,
		stageTwo:         oldStage,
		publicPathPrefix: "/",
	})
	app := fixture.app
	app.SetIsDev(true)

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/products/:id",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
				return map[string]any{"id": rd.Params()["id"]}, nil
			},
		),
	)

	stageOnePath := filepath.Join(
		fixture.privateDir,
		runtimepaths.VormaOutDirname,
		runtimepaths.VormaPathsStageOneJSONFileName,
	)
	var didReload atomic.Bool
	app.getDefaultHeadEls = func(r *http.Request, app *Vorma, h *headels.HeadEls) error {
		if !didReload.CompareAndSwap(false, true) {
			return nil
		}
		mustWriteJSONFile(t, stageOnePath, newStage)
		return app.devReloadRoutesFromDisk()
	}

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())
	req := httptest.NewRequest(http.MethodGet, "/products/123", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Header().Get(VormaBuildIDHeaderKey), "build-old"; got != want {
		t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, want)
	}
	if !didReload.Load() {
		t.Fatal(
			"expected test hook to trigger a dev reload during request handling",
		)
	}

	body := rec.Body.String()
	expectedOldFragments := []string{
		`x.buildID = "build-old";`,
		`x.routeManifestURL = "/vorma_out/route-manifest-old.js";`,
		"/frontend/src/routes/products.$id.old.tsx",
		"vorma_out/shared-old.js",
		"vorma_out/chunk-old.js",
		"vorma_out/client-old.css",
		"vorma_out/shared-old.css",
		"vorma_out/chunk-old.css",
	}
	for _, expectedOldFragment := range expectedOldFragments {
		if !strings.Contains(body, expectedOldFragment) {
			t.Fatalf(
				"body missing old-generation fragment %q, body=%q",
				expectedOldFragment,
				body,
			)
		}
	}

	unexpectedNewFragments := []string{
		`x.buildID = "build-new";`,
		`x.routeManifestURL = "/vorma_out/route-manifest-new.js";`,
		"/frontend/src/routes/products.$id.new.tsx",
		"vorma_out/shared-new.js",
		"vorma_out/chunk-new.js",
		"vorma_out/client-new.css",
		"vorma_out/shared-new.css",
		"vorma_out/chunk-new.css",
	}
	for _, unexpectedNewFragment := range unexpectedNewFragments {
		if strings.Contains(body, unexpectedNewFragment) {
			t.Fatalf(
				"body leaked new-generation fragment %q, body=%q",
				unexpectedNewFragment,
				body,
			)
		}
	}
}

func TestLoadersHandler_ProdHTMLReloadDuringRequest_UsesMatchingClientEntryScriptGeneration(
	t *testing.T,
) {
	oldStage := defaultPathsFile("build-old", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.old.tsx",
			OutPath:         "vorma_out/routes/products.$id.old.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-old.js"},
		},
	})
	oldStage.Stage = "stage-one"
	oldStage.ClientEntryOut = "vorma_out/client-old.js"
	oldStage.ClientEntryDeps = []string{"vorma_out/shared-old.js"}
	oldStage.DepToCSSBundleMap = map[string][]string{
		"vorma_out/client-old.js": {"vorma_out/client-old.css"},
		"vorma_out/shared-old.js": {"vorma_out/shared-old.css"},
		"vorma_out/chunk-old.js":  {"vorma_out/chunk-old.css"},
	}

	newStage := defaultPathsFile("build-new", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.new.tsx",
			OutPath:         "vorma_out/routes/products.$id.new.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-new.js"},
		},
	})
	newStage.Stage = "stage-one"
	newStage.ClientEntryOut = "vorma_out/client-new.js"
	newStage.ClientEntryDeps = []string{"vorma_out/shared-new.js"}
	newStage.DepToCSSBundleMap = map[string][]string{
		"vorma_out/client-new.js": {"vorma_out/client-new.css"},
		"vorma_out/shared-new.js": {"vorma_out/shared-new.css"},
		"vorma_out/chunk-new.js":  {"vorma_out/chunk-new.css"},
	}

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne:         oldStage,
		stageTwo:         oldStage,
		publicPathPrefix: "/",
	})
	app := fixture.app
	app.SetIsDev(false)

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/products/:id",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
				return map[string]any{"id": rd.Params()["id"]}, nil
			},
		),
	)

	stageOnePath := filepath.Join(
		fixture.privateDir,
		runtimepaths.VormaOutDirname,
		runtimepaths.VormaPathsStageOneJSONFileName,
	)
	var didReload atomic.Bool
	app.getDefaultHeadEls = func(r *http.Request, app *Vorma, h *headels.HeadEls) error {
		if !didReload.CompareAndSwap(false, true) {
			return nil
		}
		mustWriteJSONFile(t, stageOnePath, newStage)
		app.SetIsDev(true)
		defer app.SetIsDev(false)
		return app.devReloadRoutesFromDisk()
	}

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())
	req := httptest.NewRequest(http.MethodGet, "/products/123", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Header().Get(VormaBuildIDHeaderKey), "build-old"; got != want {
		t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, want)
	}
	if !didReload.Load() {
		t.Fatal(
			"expected test hook to trigger a dev reload during request handling",
		)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "/vorma_out/routes/products.$id.old.js") {
		t.Fatalf("body missing old route import URL, body=%q", body)
	}
	if strings.Contains(body, "/vorma_out/routes/products.$id.new.js") {
		t.Fatalf("body leaked new route import URL, body=%q", body)
	}
	if !strings.Contains(
		body,
		`<script type="module" src="/vorma_out/client-old.js"></script>`,
	) {
		t.Fatalf(
			"body missing old-generation client entry script, body=%q",
			body,
		)
	}
	if strings.Contains(
		body,
		`<script type="module" src="/vorma_out/client-new.js"></script>`,
	) {
		t.Fatalf(
			"body leaked new-generation client entry script, body=%q",
			body,
		)
	}
}

func TestLoadersHandler_ConcurrentReloadAndStaleJSONRequests_DoNotSilentlyServeNewBuildData(
	t *testing.T,
) {
	oldStage := defaultPathsFile("build-old", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.old.tsx",
			OutPath:         "vorma_out/routes/products.$id.old.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-old.js"},
		},
	})
	newStage := defaultPathsFile("build-new", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.new.tsx",
			OutPath:         "vorma_out/routes/products.$id.new.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-new.js"},
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: oldStage,
		stageTwo: oldStage,
	})
	app := fixture.app
	app.SetIsDev(true)

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/products/:id",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
				return map[string]any{"id": rd.Params()["id"]}, nil
			},
		),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())
	stageOnePath := filepath.Join(
		fixture.privateDir,
		runtimepaths.VormaOutDirname,
		runtimepaths.VormaPathsStageOneJSONFileName,
	)

	const reloadIterations = 80
	const workers = 8
	const requestsPerWorker = 80
	const requestedBuildID = "build-old"

	errCh := make(chan error, 1)
	reportErr := func(err error) {
		select {
		case errCh <- err:
		default:
		}
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1 + workers)

	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < reloadIterations; i++ {
			next := oldStage
			if i%2 == 1 {
				next = newStage
			}
			if err := writePathsFileNoTB(stageOnePath, next); err != nil {
				reportErr(fmt.Errorf("write stage-one file: %w", err))
				return
			}
			if err := app.devReloadRoutesFromDisk(); err != nil {
				reportErr(fmt.Errorf("reload routes: %w", err))
				return
			}
		}
	}()

	for workerID := 0; workerID < workers; workerID++ {
		go func(workerID int) {
			defer wg.Done()
			<-start
			for i := 0; i < requestsPerWorker; i++ {
				req := httptest.NewRequest(
					http.MethodGet,
					"/products/123?vorma_json="+requestedBuildID,
					nil,
				)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					reportErr(
						fmt.Errorf(
							"worker %d request %d status=%d",
							workerID,
							i,
							rec.Code,
						),
					)
					return
				}

				reloadHeader := rec.Header().Get("X-Vorma-Reload")
				if reloadHeader != "" {
					// Stale request fast-path: do not return route-artifact payload.
					body := rec.Body.String()
					hasOldImport := strings.Contains(
						body,
						"/frontend/src/routes/products.$id.old.tsx",
					)
					hasNewImport := strings.Contains(
						body,
						"/frontend/src/routes/products.$id.new.tsx",
					)
					if hasOldImport || hasNewImport {
						reportErr(fmt.Errorf(
							"worker %d request %d: stale reload response leaked route artifacts",
							workerID,
							i,
						))
						return
					}
					continue
				}

				buildIDHeader := rec.Header().Get(VormaBuildIDHeaderKey)
				if buildIDHeader != requestedBuildID {
					reportErr(fmt.Errorf(
						"worker %d request %d: request for build %q received non-reload response for build %q",
						workerID,
						i,
						requestedBuildID,
						buildIDHeader,
					))
					return
				}

				body := rec.Body.String()
				hasOldImport := strings.Contains(
					body,
					"/frontend/src/routes/products.$id.old.tsx",
				)
				hasNewImport := strings.Contains(
					body,
					"/frontend/src/routes/products.$id.new.tsx",
				)
				if !hasOldImport || hasNewImport {
					reportErr(fmt.Errorf(
						"worker %d request %d: non-reload response contained wrong artifacts (old=%t new=%t)",
						workerID,
						i,
						hasOldImport,
						hasNewImport,
					))
					return
				}
			}
		}(workerID)
	}

	close(start)
	wg.Wait()

	select {
	case err := <-errCh:
		t.Fatal(err)
	default:
	}
}

func TestLoadersHandler_ConcurrentReloadAndStaleJSONRequests_WithRedirectingLoaders_DoNotBypassReload(
	t *testing.T,
) {
	oldStage := defaultPathsFile("build-old", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.old.tsx",
			OutPath:         "vorma_out/routes/products.$id.old.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-old.js"},
		},
	})
	newStage := defaultPathsFile("build-new", map[string]*Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.new.tsx",
			OutPath:         "vorma_out/routes/products.$id.new.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-new.js"},
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: oldStage,
		stageTwo: oldStage,
	})
	app := fixture.app
	app.SetIsDev(true)

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/products/:id",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
				if _, err := rd.ResponseProxy().Redirect(rd.Request(), "/login", http.StatusSeeOther); err != nil {
					return nil, err
				}
				return map[string]any{"ignored": true}, nil
			},
		),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())
	stageOnePath := filepath.Join(
		fixture.privateDir,
		runtimepaths.VormaOutDirname,
		runtimepaths.VormaPathsStageOneJSONFileName,
	)

	const reloadIterations = 80
	const workers = 8
	const requestsPerWorker = 80
	const requestedBuildID = "build-old"

	errCh := make(chan error, 1)
	reportErr := func(err error) {
		select {
		case errCh <- err:
		default:
		}
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1 + workers)

	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < reloadIterations; i++ {
			next := oldStage
			if i%2 == 1 {
				next = newStage
			}
			if err := writePathsFileNoTB(stageOnePath, next); err != nil {
				reportErr(fmt.Errorf("write stage-one file: %w", err))
				return
			}
			if err := app.devReloadRoutesFromDisk(); err != nil {
				reportErr(fmt.Errorf("reload routes: %w", err))
				return
			}
		}
	}()

	for workerID := 0; workerID < workers; workerID++ {
		go func(workerID int) {
			defer wg.Done()
			<-start
			for i := 0; i < requestsPerWorker; i++ {
				req := httptest.NewRequest(
					http.MethodGet,
					"/products/123?vorma_json="+requestedBuildID,
					nil,
				)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				buildIDHeader := rec.Header().Get(VormaBuildIDHeaderKey)
				reloadHeader := rec.Header().Get("X-Vorma-Reload")

				if buildIDHeader == "build-new" {
					if reloadHeader == "" {
						reportErr(fmt.Errorf(
							"worker %d request %d: stale request for %q received build-new response without reload header (status=%d)",
							workerID,
							i,
							requestedBuildID,
							rec.Code,
						))
						return
					}
					if rec.Code != http.StatusOK {
						reportErr(fmt.Errorf(
							"worker %d request %d: stale reload response status=%d, want %d",
							workerID,
							i,
							rec.Code,
							http.StatusOK,
						))
						return
					}
					continue
				}

				if buildIDHeader != "build-old" {
					reportErr(fmt.Errorf(
						"worker %d request %d: unexpected build header %q",
						workerID,
						i,
						buildIDHeader,
					))
					return
				}

				if reloadHeader != "" {
					reportErr(fmt.Errorf(
						"worker %d request %d: old-build response unexpectedly carried reload header %q",
						workerID,
						i,
						reloadHeader,
					))
					return
				}

				if rec.Code != http.StatusSeeOther {
					reportErr(fmt.Errorf(
						"worker %d request %d: old-build redirect status=%d, want %d",
						workerID,
						i,
						rec.Code,
						http.StatusSeeOther,
					))
					return
				}
			}
		}(workerID)
	}

	close(start)
	wg.Wait()

	select {
	case err := <-errCh:
		t.Fatal(err)
	default:
	}
}

func TestLoadersHandler_ConcurrentReloadAndRequests_WithRouteShapeChanges(
	t *testing.T,
) {
	oldStage := defaultPathsFile("build-old", map[string]*Path{
		"/alpha": {
			OriginalPattern: "/alpha",
			SrcPath:         "frontend/src/routes/alpha.old.tsx",
			OutPath:         "vorma_out/routes/alpha.old.js",
			ExportKey:       "default",
		},
	})
	newStage := defaultPathsFile("build-new", map[string]*Path{
		"/beta": {
			OriginalPattern: "/beta",
			SrcPath:         "frontend/src/routes/beta.new.tsx",
			OutPath:         "vorma_out/routes/beta.new.js",
			ExportKey:       "default",
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: oldStage,
		stageTwo: oldStage,
	})
	app := fixture.app
	app.SetIsDev(true)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())
	stageOnePath := filepath.Join(
		fixture.privateDir,
		runtimepaths.VormaOutDirname,
		runtimepaths.VormaPathsStageOneJSONFileName,
	)

	const reloadIterations = 80
	const workers = 8
	const requestsPerWorker = 100

	checkResponse := func(path string, status int, body string) error {
		switch path {
		case "/alpha":
			if status == http.StatusNotFound {
				return nil
			}
			if status != http.StatusOK {
				return fmt.Errorf("path %s unexpected status %d", path, status)
			}
			hasAlpha := strings.Contains(
				body,
				"/frontend/src/routes/alpha.old.tsx",
			)
			hasBeta := strings.Contains(
				body,
				"/frontend/src/routes/beta.new.tsx",
			)
			if !hasAlpha || hasBeta {
				return fmt.Errorf(
					"path %s returned incoherent artifacts (hasAlpha=%t hasBeta=%t body=%q)",
					path,
					hasAlpha,
					hasBeta,
					body,
				)
			}
		case "/beta":
			if status == http.StatusNotFound {
				return nil
			}
			if status != http.StatusOK {
				return fmt.Errorf("path %s unexpected status %d", path, status)
			}
			hasAlpha := strings.Contains(
				body,
				"/frontend/src/routes/alpha.old.tsx",
			)
			hasBeta := strings.Contains(
				body,
				"/frontend/src/routes/beta.new.tsx",
			)
			if !hasBeta || hasAlpha {
				return fmt.Errorf(
					"path %s returned incoherent artifacts (hasAlpha=%t hasBeta=%t body=%q)",
					path,
					hasAlpha,
					hasBeta,
					body,
				)
			}
		default:
			return fmt.Errorf("unexpected path %s", path)
		}
		return nil
	}

	errCh := make(chan error, 1)
	reportErr := func(err error) {
		select {
		case errCh <- err:
		default:
		}
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1 + workers)

	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < reloadIterations; i++ {
			next := oldStage
			if i%2 == 1 {
				next = newStage
			}
			if err := writePathsFileNoTB(stageOnePath, next); err != nil {
				reportErr(fmt.Errorf("write stage-one file: %w", err))
				return
			}
			if err := app.devReloadRoutesFromDisk(); err != nil {
				reportErr(fmt.Errorf("reload routes: %w", err))
				return
			}
		}
	}()

	for workerID := 0; workerID < workers; workerID++ {
		go func(workerID int) {
			defer wg.Done()
			<-start
			for i := 0; i < requestsPerWorker; i++ {
				path := "/alpha"
				if (workerID+i)%2 == 1 {
					path = "/beta"
				}

				req := httptest.NewRequest(http.MethodGet, path, nil)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if err := checkResponse(path, rec.Code, rec.Body.String()); err != nil {
					reportErr(
						fmt.Errorf(
							"worker %d request %d: %w",
							workerID,
							i,
							err,
						),
					)
					return
				}
			}
		}(workerID)
	}

	close(start)
	wg.Wait()

	select {
	case err := <-errCh:
		t.Fatal(err)
	default:
	}
}

func TestLoadersHandler_ConcurrentReloadAndRequests_WithNestedParamShapeChanges(
	t *testing.T,
) {
	oldStage := defaultPathsFile("build-old", map[string]*Path{
		"/catalog/:catalogID/items/:itemID": {
			OriginalPattern: "/catalog/:catalogID/items/:itemID",
			SrcPath:         "frontend/src/routes/catalog.$catalogID.items.$itemID.old.tsx",
			OutPath:         "vorma_out/routes/catalog.$catalogID.items.$itemID.old.js",
			ExportKey:       "default",
		},
		"/catalog/:catalogID/items/:itemID/reviews/:reviewID": {
			OriginalPattern: "/catalog/:catalogID/items/:itemID/reviews/:reviewID",
			SrcPath:         "frontend/src/routes/catalog.$catalogID.items.$itemID.reviews.$reviewID.old.tsx",
			OutPath:         "vorma_out/routes/catalog.$catalogID.items.$itemID.reviews.$reviewID.old.js",
			ExportKey:       "default",
		},
	})
	newStage := defaultPathsFile("build-new", map[string]*Path{
		"/orgs/:orgID/repos/:repoID": {
			OriginalPattern: "/orgs/:orgID/repos/:repoID",
			SrcPath:         "frontend/src/routes/orgs.$orgID.repos.$repoID.new.tsx",
			OutPath:         "vorma_out/routes/orgs.$orgID.repos.$repoID.new.js",
			ExportKey:       "default",
		},
		"/orgs/:orgID/repos/:repoID/issues/:issueID": {
			OriginalPattern: "/orgs/:orgID/repos/:repoID/issues/:issueID",
			SrcPath:         "frontend/src/routes/orgs.$orgID.repos.$repoID.issues.$issueID.new.tsx",
			OutPath:         "vorma_out/routes/orgs.$orgID.repos.$repoID.issues.$issueID.new.js",
			ExportKey:       "default",
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: oldStage,
		stageTwo: oldStage,
	})
	app := fixture.app
	app.SetIsDev(true)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())
	stageOnePath := filepath.Join(
		fixture.privateDir,
		runtimepaths.VormaOutDirname,
		runtimepaths.VormaPathsStageOneJSONFileName,
	)

	const reloadIterations = 80
	const workers = 8
	const requestsPerWorker = 80

	const oldLeafPath = "/catalog/books/items/42/reviews/r7"
	const newLeafPath = "/orgs/acme/repos/web/issues/11"

	checkResponse := func(path string, status int, body string) error {
		switch path {
		case oldLeafPath:
			if status == http.StatusNotFound {
				return nil
			}
			if status != http.StatusOK {
				return fmt.Errorf("path %s unexpected status %d", path, status)
			}

			hasOldParent := strings.Contains(
				body,
				"/frontend/src/routes/catalog.$catalogID.items.$itemID.old.tsx",
			)
			hasOldLeaf := strings.Contains(
				body,
				"/frontend/src/routes/catalog.$catalogID.items.$itemID.reviews.$reviewID.old.tsx",
			)
			hasNewParent := strings.Contains(
				body,
				"/frontend/src/routes/orgs.$orgID.repos.$repoID.new.tsx",
			)
			hasNewLeaf := strings.Contains(
				body,
				"/frontend/src/routes/orgs.$orgID.repos.$repoID.issues.$issueID.new.tsx",
			)

			if !hasOldParent || !hasOldLeaf || hasNewParent || hasNewLeaf {
				return fmt.Errorf(
					"path %s returned incoherent nested artifacts (oldParent=%t oldLeaf=%t newParent=%t newLeaf=%t)",
					path,
					hasOldParent,
					hasOldLeaf,
					hasNewParent,
					hasNewLeaf,
				)
			}
		case newLeafPath:
			if status == http.StatusNotFound {
				return nil
			}
			if status != http.StatusOK {
				return fmt.Errorf("path %s unexpected status %d", path, status)
			}

			hasOldParent := strings.Contains(
				body,
				"/frontend/src/routes/catalog.$catalogID.items.$itemID.old.tsx",
			)
			hasOldLeaf := strings.Contains(
				body,
				"/frontend/src/routes/catalog.$catalogID.items.$itemID.reviews.$reviewID.old.tsx",
			)
			hasNewParent := strings.Contains(
				body,
				"/frontend/src/routes/orgs.$orgID.repos.$repoID.new.tsx",
			)
			hasNewLeaf := strings.Contains(
				body,
				"/frontend/src/routes/orgs.$orgID.repos.$repoID.issues.$issueID.new.tsx",
			)

			if !hasNewParent || !hasNewLeaf || hasOldParent || hasOldLeaf {
				return fmt.Errorf(
					"path %s returned incoherent nested artifacts (oldParent=%t oldLeaf=%t newParent=%t newLeaf=%t)",
					path,
					hasOldParent,
					hasOldLeaf,
					hasNewParent,
					hasNewLeaf,
				)
			}
		default:
			return fmt.Errorf("unexpected path %s", path)
		}
		return nil
	}

	errCh := make(chan error, 1)
	reportErr := func(err error) {
		select {
		case errCh <- err:
		default:
		}
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1 + workers)

	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < reloadIterations; i++ {
			next := oldStage
			if i%2 == 1 {
				next = newStage
			}
			if err := writePathsFileNoTB(stageOnePath, next); err != nil {
				reportErr(fmt.Errorf("write stage-one file: %w", err))
				return
			}
			if err := app.devReloadRoutesFromDisk(); err != nil {
				reportErr(fmt.Errorf("reload routes: %w", err))
				return
			}
		}
	}()

	for workerID := 0; workerID < workers; workerID++ {
		go func(workerID int) {
			defer wg.Done()
			<-start
			for i := 0; i < requestsPerWorker; i++ {
				path := oldLeafPath
				if (workerID+i)%2 == 1 {
					path = newLeafPath
				}

				req := httptest.NewRequest(http.MethodGet, path, nil)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if err := checkResponse(path, rec.Code, rec.Body.String()); err != nil {
					reportErr(
						fmt.Errorf(
							"worker %d request %d: %w",
							workerID,
							i,
							err,
						),
					)
					return
				}
			}
		}(workerID)
	}

	close(start)
	wg.Wait()

	select {
	case err := <-errCh:
		t.Fatal(err)
	default:
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestDevReloadMethods_FailOutsideDevMode(t *testing.T) {
	stage := defaultPathsFile("build-prod", map[string]*Path{
		"/page": {
			OriginalPattern: "/page",
			SrcPath:         "frontend/src/routes/page.tsx",
			OutPath:         "vorma_out/routes/page.js",
			ExportKey:       "default",
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app
	app.SetIsDev(false)

	if err := app.devReloadRoutesFromDisk(); err == nil {
		t.Fatal("expected devReloadRoutesFromDisk to fail outside dev mode")
	}
	if err := app.devReloadTemplateFromDisk(); err == nil {
		t.Fatal("expected devReloadTemplateFromDisk to fail outside dev mode")
	}
}

func TestDevReloadMethods_SucceedInDevMode(t *testing.T) {
	oldStage := defaultPathsFile("build-old", map[string]*Path{
		"/old": {
			OriginalPattern: "/old",
			SrcPath:         "frontend/src/routes/old.tsx",
			OutPath:         "vorma_out/routes/old.js",
			ExportKey:       "default",
		},
	})
	newStage := defaultPathsFile("build-new", map[string]*Path{
		"/new": {
			OriginalPattern: "/new",
			SrcPath:         "frontend/src/routes/new.tsx",
			OutPath:         "vorma_out/routes/new.js",
			ExportKey:       "default",
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: oldStage,
		stageTwo: oldStage,
		template: "<!doctype html><html><body>OLD TEMPLATE {{.VormaBodyScripts}}</body></html>",
	})
	app := fixture.app
	app.SetIsDev(true)

	mustWriteJSONFile(
		t,
		filepath.Join(
			fixture.privateDir,
			runtimepaths.VormaOutDirname,
			runtimepaths.VormaPathsStageOneJSONFileName,
		),
		newStage,
	)
	if err := app.devReloadRoutesFromDisk(); err != nil {
		t.Fatalf("devReloadRoutesFromDisk returned error: %v", err)
	}
	if got := app.BuildID(); got != "build-new" {
		t.Fatalf("build ID after dev reload = %q, want %q", got, "build-new")
	}

	mustWriteFile(
		t,
		filepath.Join(fixture.privateDir, "entry.go.html"),
		[]byte(
			"<!doctype html><html><body>NEW TEMPLATE {{.VormaBodyScripts}}</body></html>",
		),
	)
	if err := app.devReloadTemplateFromDisk(); err != nil {
		t.Fatalf("devReloadTemplateFromDisk returned error: %v", err)
	}
}

func writePathsFileNoTB(file string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(file, data, 0o644)
}
