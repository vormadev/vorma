package vormaruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/response"
)

type testCustomJSONMarshaler struct {
	RawNonSerializable chan int
}

func (m testCustomJSONMarshaler) MarshalJSON() ([]byte, error) {
	return []byte(`{"ok":true}`), nil
}

func TestIsJSONRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/products?vorma_json=abc123", nil)
	if !IsJSONRequest(req) {
		t.Fatal("expected request to be treated as JSON request")
	}

	req = httptest.NewRequest(http.MethodGet, "/products", nil)
	if IsJSONRequest(req) {
		t.Fatal("expected request without vorma_json to be non-JSON request")
	}
}

func TestBuildIDHelpers(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app

	t.Run("IsCurrentBuildJSONRequest", func(t *testing.T) {
		reqCurrent := httptest.NewRequest(http.MethodGet, "/x?vorma_json="+app.GetBuildID(), nil)
		if !app.IsCurrentBuildJSONRequest(reqCurrent) {
			t.Fatal("expected current build request to match")
		}

		reqStale := httptest.NewRequest(http.MethodGet, "/x?vorma_json=stale-build", nil)
		if app.IsCurrentBuildJSONRequest(reqStale) {
			t.Fatal("expected stale build request not to match")
		}
	})
}

func TestDevReloadEndpointPaths_DefaultAndCustom(t *testing.T) {
	defaultFixture := newTestFixture(t, testFixtureOptions{})
	defaultApp := defaultFixture.app
	if got, want := defaultApp.DevReloadRoutesEndpointPath(), DefaultDevReloadRoutesEndpointPath; got != want {
		t.Fatalf("default routes endpoint path = %q, want %q", got, want)
	}
	if got, want := defaultApp.DevReloadTemplateEndpointPath(), DefaultDevReloadTemplateEndpointPath; got != want {
		t.Fatalf("default template endpoint path = %q, want %q", got, want)
	}

	customFixture := newTestFixture(t, testFixtureOptions{})
	customApp := customFixture.app
	customApp.Config.DevReloadRoutesEndpointPath = "/__custom_internal/reload-routes"
	customApp.Config.DevReloadTemplateEndpointPath = "/__custom_internal/reload-template"
	if got, want := customApp.DevReloadRoutesEndpointPath(), "/__custom_internal/reload-routes"; got != want {
		t.Fatalf("custom routes endpoint path = %q, want %q", got, want)
	}
	if got, want := customApp.DevReloadTemplateEndpointPath(), "/__custom_internal/reload-template"; got != want {
		t.Fatalf("custom template endpoint path = %q, want %q", got, want)
	}
}

func TestLoadersHandler_JSONBuildAndRouteDataBehavior(t *testing.T) {
	paths := map[string]*Path{
		"/items/:id": {
			OriginalPattern: "/items/:id",
			SrcPath:         "frontend/src/routes/items.$id.tsx",
			OutPath:         "vorma_out/routes/items.$id.js",
			ExportKey:       "default",
			ErrorExportKey:  "ErrorBoundary",
			Deps: []string{
				"vorma_out/chunk-items.js",
				"vorma_out/chunk-shared.js",
				"vorma_out/chunk-items.js",
			},
		},
	}

	stage2 := &PathsFile{
		Stage:             "stage-two",
		BuildID:           "build-new",
		ClientEntrySrc:    "frontend/src/vorma.entry.tsx",
		ClientEntryOut:    "vorma_out/client-entry.js",
		ClientEntryDeps:   []string{"vorma_out/chunk-shared.js"},
		Paths:             paths,
		RouteManifestFile: "vorma_out/route-manifest.js",
		DepToCSSBundleMap: map[string][]string{
			"vorma_out/client-entry.js":  {"vorma_out/client-entry.css"},
			"vorma_out/chunk-shared.js":  {"vorma_out/chunk-shared.css"},
			"vorma_out/chunk-items.js":   {"vorma_out/chunk-items.css", "vorma_out/chunk-shared.css"},
			"vorma_out/unused-chunk.js":  {"vorma_out/unused.css"},
			"vorma_out/unused-client.js": {"vorma_out/unused-client.css"},
		},
	}

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage2,
		stageTwo: stage2,
	})
	app := fixture.app
	var loaderRunCount atomic.Int64

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/items/:id",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
			loaderRunCount.Add(1)
			return map[string]any{"itemID": rd.Params()["id"]}, nil
		}),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	t.Run("stale_build_json_request_returns_reload_header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/items/42?vorma_json=old-build&foo=bar", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get(VormaBuildIDHeaderKey); got != "build-new" {
			t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, "build-new")
		}
		if got := rec.Header().Get("X-Vorma-Reload"); got != "/items/42?foo=bar" {
			t.Fatalf("X-Vorma-Reload = %q, want %q", got, "/items/42?foo=bar")
		}
		if got, want := rec.Header().Get("Cache-Control"), "private, max-age=0, must-revalidate, no-cache"; got != want {
			t.Fatalf("Cache-Control = %q, want %q", got, want)
		}
		if body := strings.TrimSpace(rec.Body.String()); body != `{"ok":true}` {
			t.Fatalf("body = %q, want %q", body, `{"ok":true}`)
		}
		if got := loaderRunCount.Load(); got != 0 {
			t.Fatalf("stale build request should not run loaders, ran %d loaders", got)
		}
	})

	t.Run("stale_build_reload_header_preserves_non_vorma_query_params", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodGet,
			"/items/42?vorma_json=old-build&foo=a&foo=b&zap=1",
			nil,
		)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		raw := rec.Header().Get("X-Vorma-Reload")
		if raw == "" {
			t.Fatal("X-Vorma-Reload header should be set for stale build request")
		}

		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse X-Vorma-Reload header %q: %v", raw, err)
		}
		if got, want := u.Path, "/items/42"; got != want {
			t.Fatalf("reload path = %q, want %q", got, want)
		}

		q := u.Query()
		if got := q.Get(VormaJSONQueryKey); got != "" {
			t.Fatalf("%s should be removed in reload header, got %q", VormaJSONQueryKey, got)
		}
		if got := q["foo"]; !reflect.DeepEqual(got, []string{"a", "b"}) {
			t.Fatalf(`foo query values = %#v, want %#v`, got, []string{"a", "b"})
		}
		if got := q.Get("zap"); got != "1" {
			t.Fatalf(`zap query value = %q, want "1"`, got)
		}
	})

	t.Run("current_build_json_request_returns_route_data", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/items/42?vorma_json=build-new", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get(VormaBuildIDHeaderKey); got != "build-new" {
			t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, "build-new")
		}

		var routeData RouteDataFinal
		if err := json.Unmarshal(rec.Body.Bytes(), &routeData); err != nil {
			t.Fatalf("decode route data: %v", err)
		}

		wantPatterns := []string{"/items/:id"}
		if !reflect.DeepEqual(routeData.MatchedPatterns, wantPatterns) {
			t.Fatalf("MatchedPatterns = %#v, want %#v", routeData.MatchedPatterns, wantPatterns)
		}
		if routeData.Params["id"] != "42" {
			t.Fatalf(`Params["id"] = %q, want %q`, routeData.Params["id"], "42")
		}

		if len(routeData.LoadersData) != 1 {
			t.Fatalf("expected 1 loader data item, got %d", len(routeData.LoadersData))
		}
		gotLoaderData, ok := routeData.LoadersData[0].(map[string]any)
		if !ok {
			t.Fatalf("loader data has unexpected type %T", routeData.LoadersData[0])
		}
		if gotLoaderData["itemID"] != "42" {
			t.Fatalf(`loader itemID = %#v, want %q`, gotLoaderData["itemID"], "42")
		}

		wantDeps := []string{"vorma_out/chunk-shared.js", "vorma_out/chunk-items.js"}
		if !reflect.DeepEqual(routeData.Deps, wantDeps) {
			t.Fatalf("Deps = %#v, want %#v", routeData.Deps, wantDeps)
		}

		wantCSSBundles := []string{
			"vorma_out/client-entry.css",
			"vorma_out/chunk-shared.css",
			"vorma_out/chunk-items.css",
		}
		if !reflect.DeepEqual(routeData.CSSBundles, wantCSSBundles) {
			t.Fatalf("CSSBundles = %#v, want %#v", routeData.CSSBundles, wantCSSBundles)
		}

		wantImportURLs := []string{"/vorma_out/routes/items.$id.js"}
		if !reflect.DeepEqual(routeData.ImportURLs, wantImportURLs) {
			t.Fatalf("ImportURLs = %#v, want %#v", routeData.ImportURLs, wantImportURLs)
		}
	})
}

func TestLoadersHandler_MissingTasksCtxDoesNotPanicAndReturns500(t *testing.T) {
	stage := defaultPathsFile("build-missing-tasks-ctx", map[string]*Path{
		"/items/:id": {
			OriginalPattern: "/items/:id",
			SrcPath:         "frontend/src/routes/items.$id.tsx",
			OutPath:         "vorma_out/routes/items.$id.js",
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
		"/items/:id",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
			return map[string]any{"itemID": rd.Params()["id"]}, nil
		}),
	)

	handler := app.Loaders().Handler() // intentionally not wrapped with InjectTasksCtxMiddleware
	req := httptest.NewRequest(http.MethodGet, "/items/42?vorma_json="+app.GetBuildID(), nil)
	rec := httptest.NewRecorder()

	didPanic := false
	func() {
		defer func() {
			if recover() != nil {
				didPanic = true
			}
		}()
		handler.ServeHTTP(rec, req)
	}()

	if didPanic {
		t.Fatal("loaders handler panicked without TasksCtx; should return an HTTP error instead")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestLoadersHandler_HasRootDataAndSplatValuesContracts(t *testing.T) {
	stage := defaultPathsFile("build-root-splat", map[string]*Path{
		"/": {
			OriginalPattern: "/",
			SrcPath:         "frontend/src/routes/root.tsx",
			OutPath:         "vorma_out/routes/root.js",
			ExportKey:       "default",
		},
		"/files/*": {
			OriginalPattern: "/files/*",
			SrcPath:         "frontend/src/routes/files.splat.tsx",
			OutPath:         "vorma_out/routes/files.splat.js",
			ExportKey:       "default",
		},
	})

	runCase := func(t *testing.T, rootHasHandler bool) {
		t.Helper()

		fixture := newTestFixture(t, testFixtureOptions{stageOne: stage, stageTwo: stage})
		app := fixture.app

		if rootHasHandler {
			mux.RegisterNestedTaskHandler(
				app.LoadersRouter().NestedRouter,
				"/",
				mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
					return map[string]string{"layout": "ok"}, nil
				}),
			)
		} else {
			mux.RegisterNestedPatternWithoutHandler(app.LoadersRouter().NestedRouter, "/")
		}

		mux.RegisterNestedTaskHandler(
			app.LoadersRouter().NestedRouter,
			"/files/*",
			mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
				return map[string]string{"joinedSplat": strings.Join(rd.SplatValues(), "/")}, nil
			}),
		)

		req := httptest.NewRequest(
			http.MethodGet,
			"/files/docs/readme.txt?vorma_json="+app.GetBuildID(),
			nil,
		)
		rec := httptest.NewRecorder()
		mux.InjectTasksCtxMiddleware(app.Loaders().Handler()).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var routeData RouteDataFinal
		if err := json.Unmarshal(rec.Body.Bytes(), &routeData); err != nil {
			t.Fatalf("decode route data: %v", err)
		}

		if got, want := routeData.MatchedPatterns, []string{"/", "/files/*"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("MatchedPatterns = %#v, want %#v", got, want)
		}
		if got, want := []string(routeData.SplatValues), []string{"docs", "readme.txt"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("SplatValues = %#v, want %#v", got, want)
		}
		if len(routeData.LoadersData) != 2 {
			t.Fatalf("LoadersData length = %d, want 2", len(routeData.LoadersData))
		}

		childData, ok := routeData.LoadersData[1].(map[string]any)
		if !ok {
			t.Fatalf("child loader data has unexpected type %T", routeData.LoadersData[1])
		}
		if got, want := childData["joinedSplat"], "docs/readme.txt"; got != want {
			t.Fatalf("child joinedSplat = %#v, want %q", got, want)
		}

		if rootHasHandler {
			if !routeData.HasRootData {
				t.Fatal("HasRootData = false, want true when root loader handler runs")
			}
			rootData, ok := routeData.LoadersData[0].(map[string]any)
			if !ok {
				t.Fatalf("root loader data has unexpected type %T", routeData.LoadersData[0])
			}
			if got, want := rootData["layout"], "ok"; got != want {
				t.Fatalf("root layout = %#v, want %q", got, want)
			}
		} else {
			if routeData.HasRootData {
				t.Fatal("HasRootData = true, want false when root has no handler")
			}
			if routeData.LoadersData[0] != nil {
				t.Fatalf("root loader data = %#v, want nil when root has no handler", routeData.LoadersData[0])
			}
		}
	}

	t.Run("RootHandlerPresent", func(t *testing.T) {
		runCase(t, true)
	})
	t.Run("RootPatternWithoutHandler", func(t *testing.T) {
		runCase(t, false)
	})
}

func TestLoadersHandler_NotFoundAndHTMLResponseHeaders(t *testing.T) {
	paths := map[string]*Path{
		"/known": {
			OriginalPattern: "/known",
			SrcPath:         "frontend/src/routes/known.tsx",
			OutPath:         "vorma_out/routes/known.js",
			ExportKey:       "default",
		},
	}
	stage2 := defaultPathsFile("build-known", paths)

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage2,
		stageTwo: stage2,
		template: "<!doctype html><html><body>KNOWN {{.VormaBodyScripts}}</body></html>",
	})
	app := fixture.app

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/known",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		}),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	t.Run("unknown_route_is_not_found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/missing", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
		if got := rec.Header().Get(VormaBuildIDHeaderKey); got != "build-known" {
			t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, "build-known")
		}
	})

	t.Run("html_route_sets_default_cache_control", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/known", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Cache-Control"); got != "private, max-age=0, must-revalidate, no-cache" {
			t.Fatalf("Cache-Control = %q", got)
		}
		if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
			t.Fatalf("Content-Type = %q, want HTML", got)
		}
		if !strings.Contains(rec.Body.String(), "KNOWN") {
			t.Fatalf("HTML body did not include template marker; body=%q", rec.Body.String())
		}
	})
}

func TestLoadersHandler_ProxyRedirectAndErrorShortCircuit(t *testing.T) {
	stage := defaultPathsFile("build-proxy", map[string]*Path{
		"/redirect": {
			OriginalPattern: "/redirect",
			SrcPath:         "frontend/src/routes/redirect.tsx",
			OutPath:         "vorma_out/routes/redirect.js",
			ExportKey:       "default",
		},
		"/blocked": {
			OriginalPattern: "/blocked",
			SrcPath:         "frontend/src/routes/blocked.tsx",
			OutPath:         "vorma_out/routes/blocked.js",
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
		"/redirect",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
			if _, err := rd.ResponseProxy().Redirect(rd.Request(), "/login", http.StatusSeeOther); err != nil {
				return nil, err
			}
			return map[string]string{"ignored": "true"}, nil
		}),
	)
	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/blocked",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
			rd.ResponseProxy().SetStatus(http.StatusForbidden, "blocked by policy")
			return map[string]string{"ignored": "true"}, nil
		}),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	t.Run("redirect_response_proxy_short_circuits_json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/redirect?vorma_json=build-proxy", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
		}
		if got := rec.Header().Get("Location"); got != "/login" {
			t.Fatalf("Location = %q, want %q", got, "/login")
		}
		if strings.TrimSpace(rec.Body.String()) == "" {
			return
		}
		// http.Redirect writes a small HTML body; ensure no JSON payload was emitted.
		if strings.Contains(rec.Body.String(), `"matchedPatterns"`) {
			t.Fatalf("redirect response unexpectedly contained route JSON payload: %q", rec.Body.String())
		}
		if got := rec.Header().Get(VormaBuildIDHeaderKey); got != "build-proxy" {
			t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, "build-proxy")
		}
	})

	t.Run("redirect_response_proxy_short_circuits_html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/redirect", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
		}
		if got := rec.Header().Get("Location"); got != "/login" {
			t.Fatalf("Location = %q, want %q", got, "/login")
		}
		if strings.Contains(rec.Body.String(), "vorma-root") {
			t.Fatalf("redirect response unexpectedly contained root HTML payload: %q", rec.Body.String())
		}
		if got := rec.Header().Get(VormaBuildIDHeaderKey); got != "build-proxy" {
			t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, "build-proxy")
		}
	})

	t.Run("redirect_response_proxy_honors_client_redirect_header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/redirect?vorma_json=build-proxy", nil)
		req.Header.Set(response.ClientAcceptsRedirectHeader, "true")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get(response.ClientRedirectHeader); got != "/login" {
			t.Fatalf("%s = %q, want %q", response.ClientRedirectHeader, got, "/login")
		}
		if got := rec.Header().Get("Location"); got != "" {
			t.Fatalf("Location = %q, want empty when using client redirect header", got)
		}
		if strings.Contains(rec.Body.String(), `"matchedPatterns"`) {
			t.Fatalf("client redirect response unexpectedly contained route JSON payload: %q", rec.Body.String())
		}
		if got := rec.Header().Get(VormaBuildIDHeaderKey); got != "build-proxy" {
			t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, "build-proxy")
		}
	})

	t.Run("error_status_proxy_short_circuits_json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/blocked?vorma_json=build-proxy", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
		if !strings.Contains(rec.Body.String(), "blocked by policy") {
			t.Fatalf("body = %q, expected custom policy message", rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), `"matchedPatterns"`) {
			t.Fatalf("error response unexpectedly contained route JSON payload: %q", rec.Body.String())
		}
		if got := rec.Header().Get(VormaBuildIDHeaderKey); got != "build-proxy" {
			t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, "build-proxy")
		}
	})
}

func TestLoadersHandler_DefaultHeadErrorsDoNotOverrideShortCircuitResponses(t *testing.T) {
	stage := defaultPathsFile("build-short-circuit", map[string]*Path{
		"/redirect": {
			OriginalPattern: "/redirect",
			SrcPath:         "frontend/src/routes/redirect.tsx",
			OutPath:         "vorma_out/routes/redirect.js",
			ExportKey:       "default",
		},
		"/blocked": {
			OriginalPattern: "/blocked",
			SrcPath:         "frontend/src/routes/blocked.tsx",
			OutPath:         "vorma_out/routes/blocked.js",
			ExportKey:       "default",
		},
	})

	var defaultHeadCalls atomic.Int32
	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
		getDefaultHeadEls: func(r *http.Request, app *Vorma, h *headels.HeadEls) error {
			defaultHeadCalls.Add(1)
			return errors.New("default head should not run for short-circuit responses")
		},
	})
	app := fixture.app

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/redirect",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]bool, error) {
			if _, err := rd.ResponseProxy().Redirect(rd.Request(), "/login", http.StatusSeeOther); err != nil {
				return nil, err
			}
			return map[string]bool{"ignored": true}, nil
		}),
	)
	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/blocked",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]bool, error) {
			rd.ResponseProxy().SetStatus(http.StatusForbidden, "blocked by policy")
			return map[string]bool{"ignored": true}, nil
		}),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	t.Run("redirect_remains_redirect", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/redirect", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
		}
		if got := rec.Header().Get("Location"); got != "/login" {
			t.Fatalf("Location = %q, want %q", got, "/login")
		}
	})

	t.Run("proxy_error_remains_proxy_error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/blocked?vorma_json=build-short-circuit", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
		if !strings.Contains(rec.Body.String(), "blocked by policy") {
			t.Fatalf("body = %q, expected custom policy message", rec.Body.String())
		}
	})

	t.Run("not_found_remains_not_found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/missing", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	if got := defaultHeadCalls.Load(); got != 0 {
		t.Fatalf("GetDefaultHeadEls calls = %d, want 0 for short-circuit responses", got)
	}
}

func TestLoadersHandler_LoaderErrorContract(t *testing.T) {
	stage := defaultPathsFile("build-errors", map[string]*Path{
		"/items": {
			OriginalPattern: "/items",
			SrcPath:         "frontend/src/routes/items.tsx",
			OutPath:         "vorma_out/routes/items.js",
			ExportKey:       "default",
			ErrorExportKey:  "ItemsErrorBoundary",
		},
		"/items/:id": {
			OriginalPattern: "/items/:id",
			SrcPath:         "frontend/src/routes/items.$id.tsx",
			OutPath:         "vorma_out/routes/items.$id.js",
			ExportKey:       "default",
			ErrorExportKey:  "ItemErrorBoundary",
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
		getDefaultHeadEls: func(r *http.Request, app *Vorma, h *headels.HeadEls) error {
			h.Meta(h.Name("app"), h.Content("ok"))
			return nil
		},
	})
	app := fixture.app

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/items",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
			return map[string]string{"scope": "items"}, nil
		}),
	)
	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/items/:id",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
			return nil, &LoaderError{
				Client: "Could not load item",
				Server: errors.New("db timeout"),
			}
		}),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())
	req := httptest.NewRequest(http.MethodGet, "/items/42?vorma_json=build-errors", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var routeData RouteDataFinal
	if err := json.Unmarshal(rec.Body.Bytes(), &routeData); err != nil {
		t.Fatalf("decode route data: %v", err)
	}

	if got, want := routeData.OutermostServerError, "Could not load item"; got != want {
		t.Fatalf("OutermostServerError = %q, want %q", got, want)
	}
	if routeData.OutermostServerErrorIdx == nil || *routeData.OutermostServerErrorIdx != 1 {
		t.Fatalf("OutermostServerErrorIdx = %#v, want pointer to 1", routeData.OutermostServerErrorIdx)
	}
	if !reflect.DeepEqual(routeData.MatchedPatterns, []string{"/items", "/items/:id"}) {
		t.Fatalf("MatchedPatterns = %#v", routeData.MatchedPatterns)
	}
	if !reflect.DeepEqual(routeData.ErrorExportKeys, []string{"ItemsErrorBoundary", "ItemErrorBoundary"}) {
		t.Fatalf("ErrorExportKeys = %#v", routeData.ErrorExportKeys)
	}
	if len(routeData.LoadersData) != 2 {
		t.Fatalf("LoadersData length = %d, want 2", len(routeData.LoadersData))
	}
	if got, ok := routeData.LoadersData[0].(map[string]any); !ok || got["scope"] != "items" {
		t.Fatalf("unexpected first loader data: %#v", routeData.LoadersData[0])
	}
	if routeData.LoadersData[1] != nil {
		t.Fatalf("second loader data should be nil on error, got %#v", routeData.LoadersData[1])
	}
}

func TestLoadersHandler_LoaderErrorDepsAreTrimmedToOutermostBoundary(t *testing.T) {
	stage := defaultPathsFile("build-error-deps", map[string]*Path{
		"/items": {
			OriginalPattern: "/items",
			SrcPath:         "frontend/src/routes/items.tsx",
			OutPath:         "vorma_out/routes/items.js",
			ExportKey:       "default",
			ErrorExportKey:  "ItemsErrorBoundary",
			Deps:            []string{"vorma_out/items.js"},
		},
		"/items/:id": {
			OriginalPattern: "/items/:id",
			SrcPath:         "frontend/src/routes/items.$id.tsx",
			OutPath:         "vorma_out/routes/items.$id.js",
			ExportKey:       "default",
			ErrorExportKey:  "ItemErrorBoundary",
			Deps:            []string{"vorma_out/item-detail.js"},
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/items",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
			return nil, &LoaderError{
				Client: "Could not load items",
				Server: errors.New("items upstream failure"),
			}
		}),
	)
	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/items/:id",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
			return map[string]any{"id": rd.Params()["id"]}, nil
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/items/42?vorma_json=build-error-deps", nil)
	rec := httptest.NewRecorder()
	mux.InjectTasksCtxMiddleware(app.Loaders().Handler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var routeData RouteDataFinal
	if err := json.Unmarshal(rec.Body.Bytes(), &routeData); err != nil {
		t.Fatalf("decode route data: %v", err)
	}

	if routeData.OutermostServerErrorIdx == nil || *routeData.OutermostServerErrorIdx != 0 {
		t.Fatalf("OutermostServerErrorIdx = %#v, want pointer to 0", routeData.OutermostServerErrorIdx)
	}
	if !reflect.DeepEqual(routeData.MatchedPatterns, []string{"/items"}) {
		t.Fatalf("MatchedPatterns = %#v, want only outermost failing boundary", routeData.MatchedPatterns)
	}

	wantDeps := []string{"vorma_out/client-shared.js", "vorma_out/items.js"}
	if !reflect.DeepEqual(routeData.Deps, wantDeps) {
		t.Fatalf("Deps = %#v, want %#v", routeData.Deps, wantDeps)
	}
}

func TestLoadersHandler_WrappedLoaderErrorPreservesClientMessage(t *testing.T) {
	stage := defaultPathsFile("build-wrapped-errors", map[string]*Path{
		"/items/:id": {
			OriginalPattern: "/items/:id",
			SrcPath:         "frontend/src/routes/items.$id.tsx",
			OutPath:         "vorma_out/routes/items.$id.js",
			ExportKey:       "default",
			ErrorExportKey:  "ItemErrorBoundary",
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/items/:id",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
			return nil, fmt.Errorf(
				"wrapped loader error: %w",
				&LoaderError{Client: "Could not load item", Server: errors.New("upstream timeout")},
			)
		}),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())
	req := httptest.NewRequest(http.MethodGet, "/items/42?vorma_json=build-wrapped-errors", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var routeData RouteDataFinal
	if err := json.Unmarshal(rec.Body.Bytes(), &routeData); err != nil {
		t.Fatalf("decode route data: %v", err)
	}

	if got, want := routeData.OutermostServerError, "Could not load item"; got != want {
		t.Fatalf("OutermostServerError = %q, want %q", got, want)
	}
	if routeData.OutermostServerErrorIdx == nil || *routeData.OutermostServerErrorIdx != 0 {
		t.Fatalf("OutermostServerErrorIdx = %#v, want pointer to 0", routeData.OutermostServerErrorIdx)
	}
}

func TestLoadersHandler_EmptyLoaderErrorClientMessageFallsBackToGeneric(t *testing.T) {
	stage := defaultPathsFile("build-empty-client-loader-error", map[string]*Path{
		"/items/:id": {
			OriginalPattern: "/items/:id",
			SrcPath:         "frontend/src/routes/items.$id.tsx",
			OutPath:         "vorma_out/routes/items.$id.js",
			ExportKey:       "default",
			ErrorExportKey:  "ItemErrorBoundary",
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/items/:id",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
			return nil, &LoaderError{
				Client: "",
				Server: errors.New("upstream timeout"),
			}
		}),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())
	req := httptest.NewRequest(http.MethodGet, "/items/42?vorma_json=build-empty-client-loader-error", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var routeData RouteDataFinal
	if err := json.Unmarshal(rec.Body.Bytes(), &routeData); err != nil {
		t.Fatalf("decode route data: %v", err)
	}

	if got, want := routeData.OutermostServerError, "An error occurred"; got != want {
		t.Fatalf("OutermostServerError = %q, want %q", got, want)
	}
	if routeData.OutermostServerErrorIdx == nil || *routeData.OutermostServerErrorIdx != 0 {
		t.Fatalf("OutermostServerErrorIdx = %#v, want pointer to 0", routeData.OutermostServerErrorIdx)
	}
}

func TestLoadersHandler_GenericLoaderErrorDoesNotLeakInternalMessage(t *testing.T) {
	stage := defaultPathsFile("build-generic-errors", map[string]*Path{
		"/items": {
			OriginalPattern: "/items",
			SrcPath:         "frontend/src/routes/items.tsx",
			OutPath:         "vorma_out/routes/items.js",
			ExportKey:       "default",
			ErrorExportKey:  "ItemsErrorBoundary",
		},
		"/items/:id": {
			OriginalPattern: "/items/:id",
			SrcPath:         "frontend/src/routes/items.$id.tsx",
			OutPath:         "vorma_out/routes/items.$id.js",
			ExportKey:       "default",
			ErrorExportKey:  "ItemErrorBoundary",
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/items",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
			return nil, errors.New("sensitive upstream database failure")
		}),
	)
	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/items/:id",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
			return map[string]string{"id": rd.Params()["id"]}, nil
		}),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())
	req := httptest.NewRequest(http.MethodGet, "/items/42?vorma_json=build-generic-errors", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var routeData RouteDataFinal
	if err := json.Unmarshal(rec.Body.Bytes(), &routeData); err != nil {
		t.Fatalf("decode route data: %v", err)
	}

	if got, want := routeData.OutermostServerError, "An error occurred"; got != want {
		t.Fatalf("OutermostServerError = %q, want %q", got, want)
	}
	if routeData.OutermostServerErrorIdx == nil || *routeData.OutermostServerErrorIdx != 0 {
		t.Fatalf("OutermostServerErrorIdx = %#v, want pointer to 0", routeData.OutermostServerErrorIdx)
	}

	if !reflect.DeepEqual(routeData.MatchedPatterns, []string{"/items"}) {
		t.Fatalf("MatchedPatterns = %#v, want only failing parent route", routeData.MatchedPatterns)
	}
	if !reflect.DeepEqual(routeData.ImportURLs, []string{"/vorma_out/routes/items.js"}) {
		t.Fatalf("ImportURLs = %#v, want only parent import URL", routeData.ImportURLs)
	}
	if !reflect.DeepEqual(routeData.ErrorExportKeys, []string{"ItemsErrorBoundary"}) {
		t.Fatalf("ErrorExportKeys = %#v, want only parent error boundary", routeData.ErrorExportKeys)
	}

	if strings.Contains(rec.Body.String(), "sensitive upstream database failure") {
		t.Fatalf("response body leaked internal error detail: %s", rec.Body.String())
	}
}

func TestLoadersHandler_HTMLExcludesFailingRouteHeadElementsOnLoaderError(t *testing.T) {
	stage := defaultPathsFile("build-error-head", map[string]*Path{
		"/items": {
			OriginalPattern: "/items",
			SrcPath:         "frontend/src/routes/items.tsx",
			OutPath:         "vorma_out/routes/items.js",
			ExportKey:       "default",
			ErrorExportKey:  "ItemsErrorBoundary",
		},
		"/items/:id": {
			OriginalPattern: "/items/:id",
			SrcPath:         "frontend/src/routes/items.$id.tsx",
			OutPath:         "vorma_out/routes/items.$id.js",
			ExportKey:       "default",
			ErrorExportKey:  "ItemErrorBoundary",
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/items",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]bool, error) {
			head := rd.ResponseProxy().GetHeadEls()
			head.Meta(head.Name("parent-head-ok"), head.Content("1"))
			return map[string]bool{"ok": true}, nil
		}),
	)
	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/items/:id",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]bool, error) {
			head := rd.ResponseProxy().GetHeadEls()
			head.Meta(head.Name("child-head-should-not-appear"), head.Content("1"))
			return nil, &LoaderError{
				Client: "Could not load item",
				Server: errors.New("upstream timeout"),
			}
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/items/42", nil)
	rec := httptest.NewRecorder()
	mux.InjectTasksCtxMiddleware(app.Loaders().Handler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `name="parent-head-ok"`) {
		t.Fatalf("missing parent head marker in HTML body=%q", body)
	}
	if strings.Contains(body, `name="child-head-should-not-appear"`) {
		t.Fatalf("failing child route head marker leaked into HTML body=%q", body)
	}
}

func TestLoadersHandler_DefaultHeadAndRootTemplateDataErrorsReturn500(t *testing.T) {
	stage := defaultPathsFile("build-hooks", map[string]*Path{
		"/hooks": {
			OriginalPattern: "/hooks",
			SrcPath:         "frontend/src/routes/hooks.tsx",
			OutPath:         "vorma_out/routes/hooks.js",
			ExportKey:       "default",
		},
	})

	t.Run("default_head_error", func(t *testing.T) {
		fixture := newTestFixture(t, testFixtureOptions{
			stageOne: stage,
			stageTwo: stage,
			getDefaultHeadEls: func(r *http.Request, app *Vorma, h *headels.HeadEls) error {
				return errors.New("default head failure")
			},
		})
		app := fixture.app
		mux.RegisterNestedTaskHandler(
			app.LoadersRouter().NestedRouter,
			"/hooks",
			mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]bool, error) {
				return map[string]bool{"ok": true}, nil
			}),
		)

		req := httptest.NewRequest(http.MethodGet, "/hooks?vorma_json=build-hooks", nil)
		rec := httptest.NewRecorder()
		mux.InjectTasksCtxMiddleware(app.Loaders().Handler()).ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})

	t.Run("root_template_data_error", func(t *testing.T) {
		fixture := newTestFixture(t, testFixtureOptions{
			stageOne: stage,
			stageTwo: stage,
			getRootTemplateData: func(r *http.Request) (map[string]any, error) {
				return nil, errors.New("template data failure")
			},
		})
		app := fixture.app
		mux.RegisterNestedTaskHandler(
			app.LoadersRouter().NestedRouter,
			"/hooks",
			mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]bool, error) {
				return map[string]bool{"ok": true}, nil
			}),
		)

		req := httptest.NewRequest(http.MethodGet, "/hooks", nil)
		rec := httptest.NewRecorder()
		mux.InjectTasksCtxMiddleware(app.Loaders().Handler()).ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})
}

func TestLoadersHandler_HeadRenderingAndTemplateExecutionFailuresReturn500(t *testing.T) {
	stage := defaultPathsFile("build-render-errors", map[string]*Path{
		"/hooks": {
			OriginalPattern: "/hooks",
			SrcPath:         "frontend/src/routes/hooks.tsx",
			OutPath:         "vorma_out/routes/hooks.js",
			ExportKey:       "default",
		},
	})

	t.Run("head_render_error", func(t *testing.T) {
		fixture := newTestFixture(t, testFixtureOptions{
			stageOne: stage,
			stageTwo: stage,
			getDefaultHeadEls: func(r *http.Request, app *Vorma, h *headels.HeadEls) error {
				h.AddElements(headels.FromRaw([]*htmlutil.Element{
					{Tag: ""},
				}))
				return nil
			},
		})
		app := fixture.app
		mux.RegisterNestedTaskHandler(
			app.LoadersRouter().NestedRouter,
			"/hooks",
			mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]bool, error) {
				return map[string]bool{"ok": true}, nil
			}),
		)

		req := httptest.NewRequest(http.MethodGet, "/hooks", nil)
		rec := httptest.NewRecorder()
		mux.InjectTasksCtxMiddleware(app.Loaders().Handler()).ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})

	t.Run("root_template_execute_error", func(t *testing.T) {
		fixture := newTestFixture(t, testFixtureOptions{
			stageOne: stage,
			stageTwo: stage,
			template: "<!doctype html><html><body>{{call .VormaHeadEls}}</body></html>",
		})
		app := fixture.app
		mux.RegisterNestedTaskHandler(
			app.LoadersRouter().NestedRouter,
			"/hooks",
			mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]bool, error) {
				return map[string]bool{"ok": true}, nil
			}),
		)

		req := httptest.NewRequest(http.MethodGet, "/hooks", nil)
		rec := httptest.NewRecorder()
		mux.InjectTasksCtxMiddleware(app.Loaders().Handler()).ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})
}

func TestLoadersHandler_NilRootTemplateDataMapDoesNotPanic(t *testing.T) {
	stage := defaultPathsFile("build-nil-root-template-data", map[string]*Path{
		"/hooks": {
			OriginalPattern: "/hooks",
			SrcPath:         "frontend/src/routes/hooks.tsx",
			OutPath:         "vorma_out/routes/hooks.js",
			ExportKey:       "default",
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
		getRootTemplateData: func(r *http.Request) (map[string]any, error) {
			return nil, nil
		},
	})
	app := fixture.app
	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/hooks",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]bool, error) {
			return map[string]bool{"ok": true}, nil
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/hooks", nil)
	rec := httptest.NewRecorder()
	mux.InjectTasksCtxMiddleware(app.Loaders().Handler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "vorma-root") {
		t.Fatalf("expected HTML body to include root marker, body=%q", rec.Body.String())
	}
}

func TestLoadersHandler_RuntimeDoesNotMutateAppTemplateDataMap(t *testing.T) {
	stage := defaultPathsFile("build-template-data-clone", map[string]*Path{
		"/hooks": {
			OriginalPattern: "/hooks",
			SrcPath:         "frontend/src/routes/hooks.tsx",
			OutPath:         "vorma_out/routes/hooks.js",
			ExportKey:       "default",
		},
	})

	sharedTemplateData := map[string]any{
		"AppKey": "stable",
	}

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
		getRootTemplateData: func(r *http.Request) (map[string]any, error) {
			return sharedTemplateData, nil
		},
	})
	app := fixture.app
	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/hooks",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]bool, error) {
			return map[string]bool{"ok": true}, nil
		}),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/hooks", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	if got, want := len(sharedTemplateData), 1; got != want {
		t.Fatalf("sharedTemplateData len = %d, want %d", got, want)
	}
	if got, want := sharedTemplateData["AppKey"], "stable"; got != want {
		t.Fatalf("sharedTemplateData[AppKey] = %#v, want %#v", got, want)
	}

	runtimeInjectedKeys := []string{
		app.TemplateDataKeyHeadElements(),
		app.TemplateDataKeySSRScript(),
		app.TemplateDataKeySSRScriptHash(),
		app.TemplateDataKeyRootElementID(),
		app.TemplateDataKeyBodyScripts(),
	}
	for _, key := range runtimeInjectedKeys {
		if _, exists := sharedTemplateData[key]; exists {
			t.Fatalf("sharedTemplateData unexpectedly mutated with runtime key %q", key)
		}
	}
}

func TestLoadersHandler_UsesConfiguredTemplateDataKeysAndRootElementID(t *testing.T) {
	stage := defaultPathsFile("build-custom-template-keys", map[string]*Path{
		"/keys": {
			OriginalPattern: "/keys",
			SrcPath:         "frontend/src/routes/keys.tsx",
			OutPath:         "vorma_out/routes/keys.js",
			ExportKey:       "default",
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
		template: "<!doctype html><html><head>{{.AppHeadElements}}</head><body><div id=\"{{.AppRootElementID}}\"></div>{{.AppSSRScript}}{{.AppBodyScripts}}</body></html>",
		configureVormaConfig: func(config *VormaConfig) {
			config.TemplateDataKeyHeadElements = "AppHeadElements"
			config.TemplateDataKeyBodyScripts = "AppBodyScripts"
			config.TemplateDataKeySSRScript = "AppSSRScript"
			config.TemplateDataKeySSRScriptHash = "AppSSRHash"
			config.TemplateDataKeyRootElementID = "AppRootElementID"
			config.ClientRootElementID = "app-root-custom"
		},
	})
	app := fixture.app

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/keys",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]bool, error) {
			return map[string]bool{"ok": true}, nil
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/keys", nil)
	rec := httptest.NewRecorder()
	mux.InjectTasksCtxMiddleware(app.Loaders().Handler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `id="app-root-custom"`) {
		t.Fatalf("expected custom root id to render, body=%q", body)
	}
	if !strings.Contains(body, "<script") {
		t.Fatalf("expected configured body script key to render scripts, body=%q", body)
	}
}

func TestLoadersHandler_NonSerializableLoaderDataReturns500(t *testing.T) {
	stage := defaultPathsFile("build-bad-loader-data", map[string]*Path{
		"/bad": {
			OriginalPattern: "/bad",
			SrcPath:         "frontend/src/routes/bad.tsx",
			OutPath:         "vorma_out/routes/bad.js",
			ExportKey:       "default",
		},
	})
	fixture := newTestFixture(t, testFixtureOptions{stageOne: stage, stageTwo: stage})
	app := fixture.app

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/bad",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
			return map[string]any{"nonSerializable": make(chan int)}, nil
		}),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	t.Run("JSONRequest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/bad?vorma_json="+app.GetBuildID(), nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})

	t.Run("HTMLRequest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/bad", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})
}

func TestLoadersHandler_CustomJSONMarshalerLoaderDataIsAccepted(t *testing.T) {
	stage := defaultPathsFile("build-custom-marshaler", map[string]*Path{
		"/custom": {
			OriginalPattern: "/custom",
			SrcPath:         "frontend/src/routes/custom.tsx",
			OutPath:         "vorma_out/routes/custom.js",
			ExportKey:       "default",
		},
	})
	fixture := newTestFixture(t, testFixtureOptions{stageOne: stage, stageTwo: stage})
	app := fixture.app

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/custom",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (testCustomJSONMarshaler, error) {
			return testCustomJSONMarshaler{RawNonSerializable: make(chan int)}, nil
		}),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	t.Run("JSONRequest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/custom?vorma_json="+app.GetBuildID(), nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var routeData RouteDataFinal
		if err := json.Unmarshal(rec.Body.Bytes(), &routeData); err != nil {
			t.Fatalf("decode route data: %v", err)
		}
		if len(routeData.LoadersData) != 1 {
			t.Fatalf("LoadersData length = %d, want 1", len(routeData.LoadersData))
		}
		got, ok := routeData.LoadersData[0].(map[string]any)
		if !ok {
			t.Fatalf("LoadersData[0] type = %T, want map[string]any", routeData.LoadersData[0])
		}
		if got["ok"] != true {
			t.Fatalf(`LoadersData[0]["ok"] = %#v, want true`, got["ok"])
		}
	})

	t.Run("HTMLRequest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/custom", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}

func TestLoadersHandler_CacheIsolatedAcrossAppsAndDevMode(t *testing.T) {
	makeStage := func(buildID, outPath, srcPath, dep string) *PathsFile {
		return defaultPathsFile(buildID, map[string]*Path{
			"/shared/:id": {
				OriginalPattern: "/shared/:id",
				SrcPath:         srcPath,
				OutPath:         outPath,
				ExportKey:       "default",
				Deps:            []string{dep},
			},
		})
	}

	stageA := makeStage("shared-build", "vorma_out/routes/shared-a.js", "frontend/src/routes/shared-a.tsx", "vorma_out/chunk-a.js")
	stageB := makeStage("shared-build", "vorma_out/routes/shared-b.js", "frontend/src/routes/shared-b.tsx", "vorma_out/chunk-b.js")

	fixtureA := newTestFixture(t, testFixtureOptions{stageOne: stageA, stageTwo: stageA})
	fixtureB := newTestFixture(t, testFixtureOptions{stageOne: stageB, stageTwo: stageB})
	appA := fixtureA.app
	appB := fixtureB.app

	registerLoader := func(app *Vorma) {
		mux.RegisterNestedTaskHandler(
			app.LoadersRouter().NestedRouter,
			"/shared/:id",
			mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
				return map[string]string{"id": rd.Params()["id"]}, nil
			}),
		)
	}
	registerLoader(appA)
	registerLoader(appB)

	handlerA := mux.InjectTasksCtxMiddleware(appA.Loaders().Handler())
	handlerB := mux.InjectTasksCtxMiddleware(appB.Loaders().Handler())

	getJSON := func(t *testing.T, handler http.Handler, path string) RouteDataFinal {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		var out RouteDataFinal
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode route data: %v", err)
		}
		return out
	}

	resultA := getJSON(t, handlerA, "/shared/1?vorma_json=shared-build")
	resultB := getJSON(t, handlerB, "/shared/2?vorma_json=shared-build")

	if !reflect.DeepEqual(resultA.ImportURLs, []string{"/vorma_out/routes/shared-a.js"}) {
		t.Fatalf("appA ImportURLs = %#v", resultA.ImportURLs)
	}
	if !reflect.DeepEqual(resultB.ImportURLs, []string{"/vorma_out/routes/shared-b.js"}) {
		t.Fatalf("appB ImportURLs = %#v", resultB.ImportURLs)
	}
	if !reflect.DeepEqual(resultA.Deps, []string{"vorma_out/client-shared.js", "vorma_out/chunk-a.js"}) {
		t.Fatalf("appA Deps = %#v", resultA.Deps)
	}
	if !reflect.DeepEqual(resultB.Deps, []string{"vorma_out/client-shared.js", "vorma_out/chunk-b.js"}) {
		t.Fatalf("appB Deps = %#v", resultB.Deps)
	}

	appA.SetIsDev(true)
	devResult := getJSON(t, handlerA, "/shared/3?vorma_json=shared-build")
	if !reflect.DeepEqual(devResult.ImportURLs, []string{"/frontend/src/routes/shared-a.tsx"}) {
		t.Fatalf("dev ImportURLs = %#v, expected src-path import URL", devResult.ImportURLs)
	}
}

func TestLoadersHandler_ReloadIsolationAcrossApps(t *testing.T) {
	stageOld := &PathsFile{
		Stage:           "stage-two",
		BuildID:         "shared-old-build",
		ClientEntrySrc:  "frontend/src/vorma.entry.tsx",
		ClientEntryOut:  "vorma_out/client-entry.js",
		ClientEntryDeps: []string{"vorma_out/shared.js"},
		Paths: map[string]*Path{
			"/items/:id": {
				OriginalPattern: "/items/:id",
				SrcPath:         "frontend/src/routes/items_old.$id.tsx",
				OutPath:         "vorma_out/routes/items_old.$id.js",
				ExportKey:       "default",
				Deps:            []string{"vorma_out/shared.js"},
			},
		},
		RouteManifestFile: "vorma_out/route-manifest.js",
		DepToCSSBundleMap: map[string][]string{
			"vorma_out/client-entry.js": {"vorma_out/client.css"},
			"vorma_out/shared.js":       {"vorma_out/shared.css"},
		},
	}
	stageNewA := &PathsFile{
		Stage:           "stage-one",
		BuildID:         "app-a-new-build",
		ClientEntrySrc:  "frontend/src/vorma.entry.tsx",
		ClientEntryOut:  "vorma_out/client-entry.js",
		ClientEntryDeps: []string{"vorma_out/shared.js"},
		Paths: map[string]*Path{
			"/items/:id": {
				OriginalPattern: "/items/:id",
				SrcPath:         "frontend/src/routes/items_new.$id.tsx",
				OutPath:         "vorma_out/routes/items_new.$id.js",
				ExportKey:       "default",
				Deps:            []string{"vorma_out/shared.js"},
			},
		},
		RouteManifestFile: "vorma_out/route-manifest.js",
		DepToCSSBundleMap: map[string][]string{
			"vorma_out/client-entry.js": {"vorma_out/client.css"},
			"vorma_out/shared.js":       {"vorma_out/shared.css"},
		},
	}

	fixtureA := newTestFixture(t, testFixtureOptions{stageOne: stageOld, stageTwo: stageOld})
	fixtureB := newTestFixture(t, testFixtureOptions{stageOne: stageOld, stageTwo: stageOld})
	appA := fixtureA.app
	appB := fixtureB.app
	appA.SetIsDev(false)
	appB.SetIsDev(false)

	registerLoader := func(app *Vorma) {
		mux.RegisterNestedTaskHandler(
			app.LoadersRouter().NestedRouter,
			"/items/:id",
			mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
				return map[string]any{"id": rd.Params()["id"]}, nil
			}),
		)
	}
	registerLoader(appA)
	registerLoader(appB)

	handlerA := mux.InjectTasksCtxMiddleware(appA.Loaders().Handler())
	handlerB := mux.InjectTasksCtxMiddleware(appB.Loaders().Handler())

	getJSON := func(t *testing.T, handler http.Handler, path string) RouteDataFinal {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		var out RouteDataFinal
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode route data: %v", err)
		}
		return out
	}

	// Warm both apps on the shared old build.
	oldA := getJSON(t, handlerA, "/items/1?vorma_json="+appA.GetBuildID())
	oldB := getJSON(t, handlerB, "/items/2?vorma_json="+appB.GetBuildID())
	if !reflect.DeepEqual(oldA.ImportURLs, []string{"/vorma_out/routes/items_old.$id.js"}) {
		t.Fatalf("appA old importURLs = %#v", oldA.ImportURLs)
	}
	if !reflect.DeepEqual(oldB.ImportURLs, []string{"/vorma_out/routes/items_old.$id.js"}) {
		t.Fatalf("appB old importURLs = %#v", oldB.ImportURLs)
	}

	// Reload only appA to a new build.
	mustWriteJSONFile(
		t,
		filepath.Join(fixtureA.privateDir, VormaOutDirname, VormaPathsStageOneJSONFileName),
		stageNewA,
	)
	appA.SetIsDev(true)
	if err := appA.devReloadRoutesFromDisk(); err != nil {
		t.Fatalf("appA devReloadRoutesFromDisk() error: %v", err)
	}
	appA.SetIsDev(false)

	newA := getJSON(t, handlerA, "/items/3?vorma_json="+appA.GetBuildID())
	stillB := getJSON(t, handlerB, "/items/4?vorma_json="+appB.GetBuildID())

	if got, want := appA.GetBuildID(), "app-a-new-build"; got != want {
		t.Fatalf("appA buildID = %q, want %q", got, want)
	}
	if got, want := appB.GetBuildID(), "shared-old-build"; got != want {
		t.Fatalf("appB buildID = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(newA.ImportURLs, []string{"/vorma_out/routes/items_new.$id.js"}) {
		t.Fatalf("appA new importURLs = %#v, want new route output", newA.ImportURLs)
	}
	if !reflect.DeepEqual(stillB.ImportURLs, []string{"/vorma_out/routes/items_old.$id.js"}) {
		t.Fatalf("appB importURLs leaked reload from appA: %#v", stillB.ImportURLs)
	}
}

func TestLoadersHandler_HeadDedupeRulesAreScopedPerApp(t *testing.T) {
	stage := defaultPathsFile("build-head-scope", map[string]*Path{
		"/page": {
			OriginalPattern: "/page",
			SrcPath:         "frontend/src/routes/page.tsx",
			OutPath:         "vorma_out/routes/page.js",
			ExportKey:       "default",
		},
	})

	newApp := func(withViewportDedupe bool) *Vorma {
		opts := testFixtureOptions{
			stageOne: stage,
			stageTwo: stage,
			getDefaultHeadEls: func(r *http.Request, app *Vorma, h *headels.HeadEls) error {
				h.Meta(h.Name("viewport"), h.Content("width=device-width"))
				return nil
			},
		}
		if withViewportDedupe {
			opts.getHeadDedupeKeys = func(h *headels.HeadEls) {
				h.Meta(h.Name("viewport"))
			}
		}
		fixture := newTestFixture(t, opts)
		app := fixture.app
		mux.RegisterNestedTaskHandler(
			app.LoadersRouter().NestedRouter,
			"/page",
			mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]bool, error) {
				h := rd.ResponseProxy().GetHeadEls()
				h.Meta(h.Name("viewport"), h.Content("width=device-width,initial-scale=1"))
				return map[string]bool{"ok": true}, nil
			}),
		)
		return app
	}

	// App A initializes first without custom viewport dedupe rules.
	appA := newApp(false)
	reqA := httptest.NewRequest(http.MethodGet, "/page", nil)
	recA := httptest.NewRecorder()
	mux.InjectTasksCtxMiddleware(appA.Loaders().Handler()).ServeHTTP(recA, reqA)
	if recA.Code != http.StatusOK {
		t.Fatalf("appA status = %d, want %d", recA.Code, http.StatusOK)
	}
	if got := strings.Count(recA.Body.String(), `name="viewport"`); got != 2 {
		t.Fatalf("appA viewport count = %d, want %d", got, 2)
	}

	// App B should still get its own dedupe rules even though appA initialized first.
	appB := newApp(true)
	reqB := httptest.NewRequest(http.MethodGet, "/page", nil)
	recB := httptest.NewRecorder()
	mux.InjectTasksCtxMiddleware(appB.Loaders().Handler()).ServeHTTP(recB, reqB)
	if recB.Code != http.StatusOK {
		t.Fatalf("appB status = %d, want %d", recB.Code, http.StatusOK)
	}

	bodyB := recB.Body.String()
	if got := strings.Count(bodyB, `name="viewport"`); got != 1 {
		t.Fatalf("appB viewport count = %d, want %d; body=%q", got, 1, bodyB)
	}
	if !strings.Contains(bodyB, `content="width=device-width,initial-scale=1"`) {
		t.Fatalf("appB should keep route viewport value after dedupe; body=%q", bodyB)
	}
	if strings.Contains(bodyB, `content="width=device-width"`) {
		t.Fatalf("appB should dedupe default viewport value; body=%q", bodyB)
	}
}

func TestLoadersHandler_HTMLHeadDedupeAndAssetLinks(t *testing.T) {
	stage := defaultPathsFile("build-head", map[string]*Path{
		"/items/:id": {
			OriginalPattern: "/items/:id",
			SrcPath:         "frontend/src/routes/items.$id.tsx",
			OutPath:         "vorma_out/routes/items.$id.js",
			ExportKey:       "default",
			Deps: []string{
				"vorma_out/chunk-items.js",
				"vorma_out/chunk-shared.js",
			},
		},
	})
	stage.ClientEntryDeps = []string{"vorma_out/chunk-shared.js"}
	stage.DepToCSSBundleMap = map[string][]string{
		"vorma_out/client-entry.js": {"vorma_out/client-entry.css"},
		"vorma_out/chunk-shared.js": {"vorma_out/chunk-shared.css"},
		"vorma_out/chunk-items.js":  {"vorma_out/chunk-items.css"},
	}

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne:         stage,
		stageTwo:         stage,
		publicPathPrefix: "/static/",
		getDefaultHeadEls: func(r *http.Request, app *Vorma, h *headels.HeadEls) error {
			h.Title("Default App Title")
			h.Description("default description")
			return nil
		},
	})
	app := fixture.app

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/items/:id",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]any, error) {
			h := rd.ResponseProxy().GetHeadEls()
			h.Title("Item Detail Title")
			h.Description("item description")
			return map[string]any{"id": rd.Params()["id"]}, nil
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/items/42", nil)
	rec := httptest.NewRecorder()
	mux.InjectTasksCtxMiddleware(app.Loaders().Handler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()

	if strings.Count(body, "<title>") != 1 {
		t.Fatalf("expected exactly one <title>, body=%q", body)
	}
	if !strings.Contains(body, "<title>Item Detail Title</title>") {
		t.Fatalf("expected route title to win dedupe, body=%q", body)
	}
	if strings.Contains(body, "Default App Title") {
		t.Fatalf("default title should have been deduped out, body=%q", body)
	}

	if strings.Count(body, `name="description"`) != 1 {
		t.Fatalf("expected exactly one meta description, body=%q", body)
	}
	if !strings.Contains(body, `content="item description"`) {
		t.Fatalf("expected route description to win dedupe, body=%q", body)
	}
	if strings.Contains(body, `content="default description"`) {
		t.Fatalf("default description should have been deduped out, body=%q", body)
	}

	for _, dep := range []string{
		"vorma_out/chunk-shared.js",
		"vorma_out/chunk-items.js",
	} {
		if !strings.Contains(body, `rel="modulepreload"`) || !strings.Contains(body, `href="/static/`+dep+`"`) {
			t.Fatalf("expected modulepreload for %q, body=%q", dep, body)
		}
	}

	for _, bundle := range []string{
		"vorma_out/client-entry.css",
		"vorma_out/chunk-shared.css",
		"vorma_out/chunk-items.css",
	} {
		if !strings.Contains(body, `data-vorma-css-bundle="`+bundle+`"`) {
			t.Fatalf("expected css bundle marker for %q, body=%q", bundle, body)
		}
		if !strings.Contains(body, `href="/static/`+bundle+`"`) {
			t.Fatalf("expected css bundle href for %q, body=%q", bundle, body)
		}
	}

	if !strings.Contains(body, `<script type="module" src="/static/vorma_out/client-entry.js"></script>`) {
		t.Fatalf("missing production client entry script tag, body=%q", body)
	}
}

func TestLoadersHandler_RespectsExistingCacheControlHeader(t *testing.T) {
	stage := defaultPathsFile("build-cache", map[string]*Path{
		"/cache": {
			OriginalPattern: "/cache",
			SrcPath:         "frontend/src/routes/cache.tsx",
			OutPath:         "vorma_out/routes/cache.js",
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
		"/cache",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]bool, error) {
			rd.ResponseProxy().SetHeader("Cache-Control", "public, max-age=120")
			return map[string]bool{"ok": true}, nil
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/cache", nil)
	rec := httptest.NewRecorder()
	mux.InjectTasksCtxMiddleware(app.Loaders().Handler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Header().Get("Cache-Control"), "public, max-age=120"; got != want {
		t.Fatalf("Cache-Control = %q, want %q", got, want)
	}
}

func TestLoadersHandler_DevReloadEndpoints(t *testing.T) {
	stageOld := defaultPathsFile("build-old", map[string]*Path{
		"/hello": {
			OriginalPattern: "/hello",
			SrcPath:         "frontend/src/routes/hello.tsx",
			OutPath:         "vorma_out/routes/hello.js",
			ExportKey:       "default",
		},
	})
	stageNew := defaultPathsFile("build-new", map[string]*Path{
		"/hello": {
			OriginalPattern: "/hello",
			SrcPath:         "frontend/src/routes/hello.tsx",
			OutPath:         "vorma_out/routes/hello.js",
			ExportKey:       "default",
		},
		"/new": {
			OriginalPattern: "/new",
			SrcPath:         "frontend/src/routes/new.tsx",
			OutPath:         "vorma_out/routes/new.js",
			ExportKey:       "default",
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne:         stageOld,
		stageTwo:         stageOld,
		template:         "<!doctype html><html><body>OLD TEMPLATE {{.VormaBodyScripts}}</body></html>",
		publicPathPrefix: "/",
	})
	app := fixture.app
	app.SetIsDev(true)
	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	t.Run("reload_routes_success", func(t *testing.T) {
		mustWriteJSONFile(
			t,
			filepath.Join(fixture.privateDir, VormaOutDirname, VormaPathsStageOneJSONFileName),
			stageNew,
		)

		req := httptest.NewRequest(http.MethodGet, app.DevReloadRoutesEndpointPath(), nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if body := strings.TrimSpace(rec.Body.String()); body != "ok" {
			t.Fatalf("body = %q, want %q", body, "ok")
		}
		if got := app.GetBuildID(); got != "build-new" {
			t.Fatalf("build ID after reload = %q, want %q", got, "build-new")
		}

		routeReq := httptest.NewRequest(http.MethodGet, "/new?vorma_json=build-new", nil)
		routeRec := httptest.NewRecorder()
		handler.ServeHTTP(routeRec, routeReq)
		if routeRec.Code != http.StatusOK {
			t.Fatalf("new route status = %d, want %d", routeRec.Code, http.StatusOK)
		}
		var routeData RouteDataFinal
		if err := json.Unmarshal(routeRec.Body.Bytes(), &routeData); err != nil {
			t.Fatalf("decode route data: %v", err)
		}
		if !reflect.DeepEqual(routeData.MatchedPatterns, []string{"/new"}) {
			t.Fatalf("MatchedPatterns after route reload = %#v", routeData.MatchedPatterns)
		}
	})

	t.Run("reload_template_success", func(t *testing.T) {
		beforeReq := httptest.NewRequest(http.MethodGet, "/hello", nil)
		beforeRec := httptest.NewRecorder()
		handler.ServeHTTP(beforeRec, beforeReq)
		if beforeRec.Code != http.StatusOK {
			t.Fatalf("status before template reload = %d, want %d", beforeRec.Code, http.StatusOK)
		}
		if !strings.Contains(beforeRec.Body.String(), "OLD TEMPLATE") {
			t.Fatalf("expected OLD TEMPLATE marker, body=%q", beforeRec.Body.String())
		}

		mustWriteFile(
			t,
			filepath.Join(fixture.privateDir, "entry.go.html"),
			[]byte("<!doctype html><html><body>NEW TEMPLATE {{.VormaBodyScripts}}</body></html>"),
		)
		req := httptest.NewRequest(http.MethodGet, app.DevReloadTemplateEndpointPath(), nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if body := strings.TrimSpace(rec.Body.String()); body != "ok" {
			t.Fatalf("body = %q, want %q", body, "ok")
		}

		afterReq := httptest.NewRequest(http.MethodGet, "/hello", nil)
		afterRec := httptest.NewRecorder()
		handler.ServeHTTP(afterRec, afterReq)
		if afterRec.Code != http.StatusOK {
			t.Fatalf("status after template reload = %d, want %d", afterRec.Code, http.StatusOK)
		}
		if !strings.Contains(afterRec.Body.String(), "NEW TEMPLATE") {
			t.Fatalf("expected NEW TEMPLATE marker, body=%q", afterRec.Body.String())
		}
	})

	t.Run("reload_routes_error", func(t *testing.T) {
		stageOnePath := filepath.Join(fixture.privateDir, VormaOutDirname, VormaPathsStageOneJSONFileName)
		if err := os.Remove(stageOnePath); err != nil {
			t.Fatalf("remove stage one file: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, app.DevReloadRoutesEndpointPath(), nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})

	t.Run("reload_template_error", func(t *testing.T) {
		app.Config.HTMLTemplateLocation = "missing-template.go.html"

		req := httptest.NewRequest(http.MethodGet, app.DevReloadTemplateEndpointPath(), nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})
}

func TestLoadersHandler_DevReloadEndpointsNotExposedInProd(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app
	app.SetIsDev(false)
	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	for _, path := range []string{
		app.DevReloadRoutesEndpointPath(),
		app.DevReloadTemplateEndpointPath(),
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("path %q status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
		if strings.TrimSpace(rec.Body.String()) == "ok" {
			t.Fatalf("path %q unexpectedly executed dev endpoint in prod mode", path)
		}
	}
}
