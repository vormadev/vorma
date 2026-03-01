package routepipeline

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime/rendering"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmatcher"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/kit/tasks"
)

func TestPlanRouteResultFromResolvedTaskOutcomes_TerminalMergedProxyShortCircuits(
	t *testing.T,
) {
	redirectProxy := response.NewProxy()
	if _, err := redirectProxy.Redirect(
		httptest.NewRequest(http.MethodGet, "/from", nil),
		"/to",
	); err != nil {
		t.Fatalf("build redirect proxy: %v", err)
	}

	result := PlanRouteResultFromResolvedTaskOutcomes(RouteStageOnePlannerInput{
		RuntimeSnapshot: RuntimeSnapshot{
			BuildID: "build-terminal",
		},
		MergedResponseProxy: redirectProxy,
	})

	if got, want := result.TerminalState, RouteTerminalStateRedirect; got != want {
		t.Fatalf("TerminalState = %v, want %v", got, want)
	}
	if got, want := result.BuildID, "build-terminal"; got != want {
		t.Fatalf("BuildID = %q, want %q", got, want)
	}
	if result.Core != nil {
		t.Fatalf(
			"Core should be nil for terminal planner outputs, got %#v",
			result.Core,
		)
	}
	if result.MergedResponseProxy != redirectProxy {
		t.Fatal("terminal planner output should reuse merged response proxy")
	}
}

func TestPlanRouteResultFromResolvedTaskOutcomes_ErrorAtPrefixRouteTruncatesPayloadAndDeps(
	t *testing.T,
) {
	input := newRouteStageOnePlannerInputFixture(t)
	input.HasRootData = true
	input.LoadersData = []any{
		map[string]any{"route": "parent"},
		map[string]any{"route": "child"},
	}
	input.OutermostLoaderErrorIndex = intPointerForRoutePipelineTest(0)
	input.ClientLoaderErrorMessage = "safe-client-error"

	result := PlanRouteResultFromResolvedTaskOutcomes(input)

	if got, want := result.TerminalState, RouteTerminalStateNone; got != want {
		t.Fatalf("TerminalState = %v, want %v", got, want)
	}
	if result.Core == nil {
		t.Fatal("Core should be present for non-terminal planner outputs")
	}
	if got, want := result.Core.OutermostServerError, "safe-client-error"; got != want {
		t.Fatalf("OutermostServerError = %q, want %q", got, want)
	}
	if result.Core.OutermostServerErrorIdx == nil ||
		*result.Core.OutermostServerErrorIdx != 0 {
		t.Fatalf(
			"OutermostServerErrorIdx = %#v, want 0",
			result.Core.OutermostServerErrorIdx,
		)
	}
	if got, want := result.Core.MatchedPatterns, []string{"/products/:id"}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("MatchedPatterns = %#v, want %#v", got, want)
	}
	if got, want := len(result.Core.LoadersData), 1; got != want {
		t.Fatalf("len(LoadersData) = %d, want %d", got, want)
	}
	if got, want := result.Core.Deps, []string{"vorma_out/client-shared.js", "vorma_out/products.js"}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("Deps = %#v, want %#v", got, want)
	}
	if got, want := len(result.HeadElements), 0; got != want {
		t.Fatalf("len(HeadElements) = %d, want %d", got, want)
	}
	if got, want := result.CSSBundles, []string{
		"vorma_out/client-entry.css",
		"vorma_out/client-shared.css",
		"vorma_out/products.css",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("CSSBundles = %#v, want %#v", got, want)
	}
}

func TestPlanRouteResultFromResolvedTaskOutcomes_SuccessBuildsFullRouteCoreAndAssets(
	t *testing.T,
) {
	input := newRouteStageOnePlannerInputFixture(t)
	input.LoadersData = []any{
		map[string]any{"route": "parent"},
		map[string]any{"route": "child"},
	}

	result := PlanRouteResultFromResolvedTaskOutcomes(input)

	if got, want := result.TerminalState, RouteTerminalStateNone; got != want {
		t.Fatalf("TerminalState = %v, want %v", got, want)
	}
	if result.Core == nil {
		t.Fatal("Core should be present for success planner outputs")
	}
	if result.Core.OutermostServerErrorIdx != nil {
		t.Fatalf(
			"OutermostServerErrorIdx should be nil for success output, got %#v",
			result.Core.OutermostServerErrorIdx,
		)
	}
	if got, want := result.Core.MatchedPatterns, []string{"/products/:id", "/products/:id/details"}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("MatchedPatterns = %#v, want %#v", got, want)
	}
	if got, want := result.Core.Deps, []string{
		"vorma_out/client-shared.js",
		"vorma_out/products.js",
		"vorma_out/product-details.js",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Deps = %#v, want %#v", got, want)
	}
	if got, want := result.CSSBundles, []string{
		"vorma_out/client-entry.css",
		"vorma_out/client-shared.css",
		"vorma_out/products.css",
		"vorma_out/product-details.css",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("CSSBundles = %#v, want %#v", got, want)
	}
	if got, want := len(result.HeadElements), 2; got != want {
		t.Fatalf("len(HeadElements) = %d, want %d", got, want)
	}
	if got, want := result.RouteManifestFileSnapshot, "vorma_out/route-manifest.js"; got != want {
		t.Fatalf("RouteManifestFileSnapshot = %q, want %q", got, want)
	}
	if got, want := result.HTMLRenderSnapshot.ClientEntryOut, "vorma_out/client-entry.js"; got != want {
		t.Fatalf("HTMLRenderSnapshot.ClientEntryOut = %q, want %q", got, want)
	}
	if !result.IsDev {
		t.Fatal("IsDev should be true in success planner output")
	}
}

func TestLoadOrBuildCachedItemSubset_DoesNotStoreWhenSnapshotVersionIsStale(
	t *testing.T,
) {
	matchResults := buildPipelineMatchResults(
		t,
		[]string{"/products/:id"},
		"/products/1",
	)
	matches := matchResults.Matches
	cacheKey := BuildRouteDataCacheKey(matches, true, "build-same", 1)
	cache := &sync.Map{}

	oldPaths := map[string]*PathData{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.old.tsx",
			OutPath:         "vorma_out/routes/products.$id.old.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-old.js"},
		},
	}
	newPaths := map[string]*PathData{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.new.tsx",
			OutPath:         "vorma_out/routes/products.$id.new.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-new.js"},
		},
	}
	clientEntryDeps := []string{"vorma_out/client-shared.js"}

	staleCached := LoadOrBuildCachedItemSubset(
		cacheKey,
		matches,
		oldPaths,
		clientEntryDeps,
		true,
		1,
		cache,
		func(expected uint64) bool {
			if expected != 1 {
				t.Fatalf(
					"expected stale snapshot version check of 1, got %d",
					expected,
				)
			}
			return false
		},
	)
	if staleCached == nil {
		t.Fatal("expected stale snapshot subset build to return data")
	}
	if got, want := staleCached.ImportURLs, []string{"/frontend/src/routes/products.$id.old.tsx"}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("stale snapshot ImportURLs = %#v, want %#v", got, want)
	}
	if got := cacheLen(cache); got != 0 {
		t.Fatalf(
			"stale snapshot should not repopulate cache, got %d entries",
			got,
		)
	}

	currentCached := LoadOrBuildCachedItemSubset(
		cacheKey,
		matches,
		newPaths,
		clientEntryDeps,
		true,
		2,
		cache,
		func(expected uint64) bool {
			if expected != 2 {
				t.Fatalf(
					"expected current snapshot version check of 2, got %d",
					expected,
				)
			}
			return true
		},
	)
	if currentCached == nil {
		t.Fatal("expected current snapshot subset build to return data")
	}
	if got, want := currentCached.ImportURLs, []string{"/frontend/src/routes/products.$id.new.tsx"}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("current snapshot ImportURLs = %#v, want %#v", got, want)
	}
	if got := cacheLen(cache); got != 1 {
		t.Fatalf(
			"expected cache to contain one current-snapshot entry, got %d",
			got,
		)
	}
}

func TestGetDepsFromData_ClientEntryFirstAndDeduped(t *testing.T) {
	paths := map[string]*PathData{
		"": {
			OriginalPattern: "",
			Deps: []string{
				"vorma_out/root.js",
				"vorma_out/shared.js",
			},
		},
		"/items/:id": {
			OriginalPattern: "/items/:id",
			Deps: []string{
				"vorma_out/item.js",
				"vorma_out/shared.js",
			},
		},
	}

	matchResults := buildPipelineMatchResults(
		t,
		[]string{"", "/items/:id"},
		"/items/42",
	)

	deps := GetDepsFromData(
		matchResults.Matches,
		paths,
		[]string{"vorma_out/client.js", "vorma_out/shared.js"},
	)
	want := []string{
		"vorma_out/client.js",
		"vorma_out/shared.js",
		"vorma_out/root.js",
		"vorma_out/item.js",
	}
	if !reflect.DeepEqual(deps, want) {
		t.Fatalf("deps = %#v, want %#v", deps, want)
	}
}

func TestGetCSSBundles_DedupedAndClientEntryFirst(t *testing.T) {
	css := GetCSSBundles(
		[]string{"vorma_out/shared.js", "vorma_out/item.js"},
		"vorma_out/client-entry.js",
		map[string][]string{
			"vorma_out/client-entry.js": {
				"vorma_out/client.css",
				"vorma_out/shared.css",
			},
			"vorma_out/shared.js": {
				"vorma_out/shared.css",
				"vorma_out/layout.css",
			},
			"vorma_out/item.js": {"vorma_out/item.css"},
		},
	)
	want := []string{
		"vorma_out/client.css",
		"vorma_out/shared.css",
		"vorma_out/layout.css",
		"vorma_out/item.css",
	}
	if !reflect.DeepEqual(css, want) {
		t.Fatalf("css bundles = %#v, want %#v", css, want)
	}
}

func TestBuildLoadersReloadURL_RemovesVormaJSONQueryParam(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/products/42?beta=1&vorma_json=1&alpha=2",
		nil,
	)

	reloadURL := BuildLoadersReloadURL(request)
	parsedURL, err := url.Parse(reloadURL)
	if err != nil {
		t.Fatalf("parse reload URL: %v", err)
	}

	if got, want := parsedURL.Path, "/products/42"; got != want {
		t.Fatalf("reload URL path = %q, want %q", got, want)
	}
	if got := parsedURL.Query().Get("vorma_json"); got != "" {
		t.Fatalf("reload URL vorma_json query should be removed, got %q", got)
	}
	if got, want := parsedURL.Query().Get("alpha"), "2"; got != want {
		t.Fatalf("reload URL alpha query = %q, want %q", got, want)
	}
	if got, want := parsedURL.Query().Get("beta"), "1"; got != want {
		t.Fatalf("reload URL beta query = %q, want %q", got, want)
	}
}

func TestWriteTerminalLoadersResponse_HandlesTerminalStates(t *testing.T) {
	t.Run("not_found_commits_404", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		res := response.New(recorder)

		didWrite := WriteTerminalLoadersResponse(res, &RouteResult{
			TerminalState: RouteTerminalStateNotFound,
		})
		if !didWrite {
			t.Fatal("expected not-found terminal state to write response")
		}
		if got, want := recorder.Code, http.StatusNotFound; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}
	})

	t.Run(
		"redirect_and_error_states_short_circuit_without_write",
		func(t *testing.T) {
			for _, terminalState := range []RouteTerminalState{
				RouteTerminalStateRedirect,
				RouteTerminalStateError,
			} {
				recorder := httptest.NewRecorder()
				res := response.New(recorder)

				didWrite := WriteTerminalLoadersResponse(res, &RouteResult{
					TerminalState: terminalState,
				})
				if !didWrite {
					t.Fatalf(
						"expected terminal state %v to short-circuit response flow",
						terminalState,
					)
				}
				if got, want := recorder.Code, http.StatusOK; got != want {
					t.Fatalf(
						"status for terminal state %v = %d, want %d",
						terminalState,
						got,
						want,
					)
				}
			}
		},
	)

	t.Run("non_terminal_state_does_not_short_circuit", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		res := response.New(recorder)
		didWrite := WriteTerminalLoadersResponse(res, &RouteResult{
			TerminalState: RouteTerminalStateNone,
		})
		if didWrite {
			t.Fatal("expected non-terminal state to return false")
		}
	})
}

func TestBuildRouteDataFinal_MapsCoreAndAssets(t *testing.T) {
	core := &RouteDataCore{
		MatchedPatterns: []string{"/docs"},
		LoadersData:     []any{"data"},
		ImportURLs:      []string{"vorma_out/routes/docs.js"},
		ExportKeys:      []string{"default"},
		Deps:            []string{"vorma_out/chunk-layout.js"},
	}
	titleEl := &htmlutil.Element{Tag: "title", TextContent: "Docs"}
	metaEl := &htmlutil.Element{
		Tag: "meta",
		AttributesKnownSafe: map[string]string{
			"name":    "description",
			"content": "Docs page",
		},
	}
	restEl := &htmlutil.Element{
		Tag: "script",
		AttributesKnownSafe: map[string]string{
			"src": "/docs.js",
		},
	}
	assets := &RouteAssets{
		SortedAndPreEscapedHeadEls: &headels.SortedAndPreEscapedHeadEls{
			Title: titleEl,
			Meta:  []*htmlutil.Element{metaEl},
			Rest:  []*htmlutil.Element{restEl},
		},
		Deps:       []string{"/public/vorma_out/chunk-layout.js"},
		CSSBundles: []string{"/public/vorma_out/docs.css"},
	}

	routeData := BuildRouteDataFinal(
		&RouteResult{
			Core:   core,
			Assets: assets,
			IsDev:  false,
		},
		"/public/",
	)

	if routeData.RouteDataCore == core {
		t.Fatal("route data core should be normalized copy, not original pointer")
	}
	if routeData.Title != titleEl {
		t.Fatal("title should be mapped from route assets")
	}
	if got, want := routeData.MetaHeadEls, []*htmlutil.Element{metaEl}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("MetaHeadEls = %#v, want %#v", got, want)
	}
	if got, want := routeData.RestHeadEls, []*htmlutil.Element{restEl}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("RestHeadEls = %#v, want %#v", got, want)
	}
	if got, want := routeData.RouteDataCore.ImportURLs, []string{"/public/vorma_out/routes/docs.js"}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("ImportURLs = %#v, want %#v", got, want)
	}
	if got, want := routeData.RouteDataCore.Deps, []string{"/public/vorma_out/chunk-layout.js"}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("Deps = %#v, want %#v", got, want)
	}
	if got, want := routeData.CSSBundles, []string{"/public/vorma_out/docs.css"}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("CSSBundles = %#v, want %#v", got, want)
	}
}

func TestBuildRouteDataFinal_UsesCoreDepsWhenAssetsAreMissing(t *testing.T) {
	routeData := BuildRouteDataFinal(
		&RouteResult{
			Core: &RouteDataCore{
				MatchedPatterns: []string{"/docs"},
				LoadersData:     []any{"data"},
				ImportURLs:      []string{"vorma_out/routes/docs.js"},
				ExportKeys:      []string{"default"},
				Deps:            []string{"vorma_out/chunk-layout.js"},
			},
			CSSBundles: []string{"vorma_out/docs.css"},
			IsDev:      false,
		},
		"/public/",
	)

	if got, want := routeData.RouteDataCore.ImportURLs, []string{
		"/public/vorma_out/routes/docs.js",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ImportURLs = %#v, want %#v", got, want)
	}
	if got, want := routeData.RouteDataCore.Deps, []string{
		"/public/vorma_out/chunk-layout.js",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Deps = %#v, want %#v", got, want)
	}
	if got, want := routeData.CSSBundles, []string{
		"/public/vorma_out/docs.css",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("CSSBundles = %#v, want %#v", got, want)
	}
	if routeData.Title != nil {
		t.Fatalf("Title = %#v, want nil when assets are missing", routeData.Title)
	}
	if routeData.MetaHeadEls != nil {
		t.Fatalf(
			"MetaHeadEls = %#v, want nil when assets are missing",
			routeData.MetaHeadEls,
		)
	}
	if routeData.RestHeadEls != nil {
		t.Fatalf(
			"RestHeadEls = %#v, want nil when assets are missing",
			routeData.RestHeadEls,
		)
	}
}

func TestEnsureLoadersCacheControlHeader(t *testing.T) {
	t.Run("writes_default_when_header_is_missing", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		res := response.New(recorder)
		EnsureLoadersCacheControlHeader(recorder, res)
		if got, want := recorder.Header().Get("Cache-Control"), "private, max-age=0, must-revalidate, no-cache"; got != want {
			t.Fatalf("Cache-Control = %q, want %q", got, want)
		}
	})

	t.Run("preserves_existing_header", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		recorder.Header().Set("Cache-Control", "public, max-age=3600")
		res := response.New(recorder)
		EnsureLoadersCacheControlHeader(recorder, res)
		if got, want := recorder.Header().Get("Cache-Control"), "public, max-age=3600"; got != want {
			t.Fatalf("Cache-Control = %q, want %q", got, want)
		}
	})
}

func TestWriteLoadersJSONResponse(t *testing.T) {
	t.Run("writes_json_payload", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		res := response.New(recorder)
		payload := &RouteDataFinal{
			RouteDataCore: &RouteDataCore{
				MatchedPatterns: []string{"/docs"},
				LoadersData:     []any{"hello"},
			},
		}

		if err := WriteLoadersJSONResponse(res, payload); err != nil {
			t.Fatalf("WriteLoadersJSONResponse: %v", err)
		}
		if got, want := recorder.Header().Get("Content-Type"), "application/json"; got != want {
			t.Fatalf("Content-Type = %q, want %q", got, want)
		}

		var output map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &output); err != nil {
			t.Fatalf("unmarshal JSON response: %v", err)
		}
		if _, exists := output["matchedPatterns"]; !exists {
			t.Fatalf(
				"expected matchedPatterns in JSON payload, got %#v",
				output,
			)
		}
		if _, exists := output["viteDevURL"]; exists {
			t.Fatalf(
				"expected viteDevURL to be omitted from route-data JSON payload, got %#v",
				output,
			)
		}
	})

	t.Run(
		"returns_error_when_payload_is_not_json_serializable",
		func(t *testing.T) {
			recorder := httptest.NewRecorder()
			res := response.New(recorder)
			payload := &RouteDataFinal{
				RouteDataCore: &RouteDataCore{
					LoadersData: []any{make(chan int)},
				},
			}

			err := WriteLoadersJSONResponse(res, payload)
			if err == nil {
				t.Fatal("expected serialization error for channel loader data")
			}
			if got := recorder.Body.Len(); got != 0 {
				t.Fatalf(
					"expected no response body on serialization error, got %d bytes",
					got,
				)
			}
		},
	)
}

func TestComputeHasRootData(t *testing.T) {
	rootRouter := nestedmux.NewRouter(nil)
	nestedmux.AddTaskHandler(
		rootRouter,
		"",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				return "root-data", nil
			},
		),
	)
	nestedmux.AddTaskHandler(
		rootRouter,
		"/items/:id",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				return "item-data", nil
			},
		),
	)

	requestWithRoot := createRequestWithTasksCtxForRoutePipelineTest(
		http.MethodGet,
		"/items/42",
	)
	matchResultsWithRoot, found := nestedmux.FindMatches(
		rootRouter,
		requestWithRoot,
	)
	if !found {
		t.Fatal("expected nested matches for /items/42 with root route")
	}
	taskResultsWithRoot, found := nestedmux.FindMatchesAndRunTasks(
		rootRouter,
		requestWithRoot,
	)
	if !found {
		t.Fatal("expected task execution matches for /items/42 with root route")
	}

	if !ComputeHasRootData(matchResultsWithRoot, taskResultsWithRoot) {
		t.Fatal(
			"expected root data when root route is matched and runs a handler",
		)
	}

	noRootHandlerRouter := nestedmux.NewRouter(nil)
	nestedmux.AddPatternWithoutHandler(noRootHandlerRouter, "")
	nestedmux.AddTaskHandler(
		noRootHandlerRouter,
		"/items/:id",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				return "item-data", nil
			},
		),
	)

	requestWithoutRootHandler := createRequestWithTasksCtxForRoutePipelineTest(
		http.MethodGet,
		"/items/42",
	)
	matchResultsWithoutRootHandler, found := nestedmux.FindMatches(
		noRootHandlerRouter,
		requestWithoutRootHandler,
	)
	if !found {
		t.Fatal("expected nested matches for /items/42 without root handler")
	}
	taskResultsWithoutRootHandler, found := nestedmux.FindMatchesAndRunTasks(
		noRootHandlerRouter,
		requestWithoutRootHandler,
	)
	if !found {
		t.Fatal(
			"expected task execution matches for /items/42 without root handler",
		)
	}

	if ComputeHasRootData(
		matchResultsWithoutRootHandler,
		taskResultsWithoutRootHandler,
	) {
		t.Fatal("expected no root data when root route has no task handler")
	}
}

func TestCollectLoadersDataAndErrorsAndFindFirstErrorIndex(t *testing.T) {
	router := nestedmux.NewRouter(nil)
	nestedmux.AddTaskHandler(
		router,
		"",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				return "root-data", nil
			},
		),
	)
	nestedmux.AddTaskHandler(
		router,
		"/items/:id",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				return "", errors.New("loader failed")
			},
		),
	)

	request := createRequestWithTasksCtxForRoutePipelineTest(
		http.MethodGet,
		"/items/42",
	)
	taskResults, found := nestedmux.FindMatchesAndRunTasks(router, request)
	if !found {
		t.Fatal("expected nested matches for /items/42")
	}

	loadersData, loadersErrs := CollectLoadersDataAndErrors(taskResults)
	if got, want := len(loadersData), 2; got != want {
		t.Fatalf("len(loadersData) = %d, want %d", got, want)
	}
	if got, want := len(loadersErrs), 2; got != want {
		t.Fatalf("len(loadersErrs) = %d, want %d", got, want)
	}
	if got, want := loadersData[0], "root-data"; got != want {
		t.Fatalf("loadersData[0] = %#v, want %#v", got, want)
	}
	if got, want := loadersData[1], ""; got != want {
		t.Fatalf("loadersData[1] = %#v, want %#v", got, want)
	}
	if loadersErrs[0] != nil {
		t.Fatalf("loadersErrs[0] = %v, want nil", loadersErrs[0])
	}
	if loadersErrs[1] == nil || loadersErrs[1].Error() != "loader failed" {
		t.Fatalf("loadersErrs[1] = %v, want loader failed", loadersErrs[1])
	}

	firstErrIdx := FindFirstErrorIndex(loadersErrs)
	if firstErrIdx == nil || *firstErrIdx != 1 {
		t.Fatalf("first error index = %#v, want 1", firstErrIdx)
	}
	if got := FindFirstErrorIndex([]error{nil, nil}); got != nil {
		t.Fatalf("first error index for no errors = %#v, want nil", got)
	}
}

func TestDetectTerminalStateFromMergedResponseProxy(t *testing.T) {
	if got, want := DetectTerminalStateFromMergedResponseProxy(nil), RouteTerminalStateNone; got != want {
		t.Fatalf("terminal state for nil proxy = %v, want %v", got, want)
	}

	errorProxy := response.NewProxy()
	errorProxy.SetStatus(http.StatusInternalServerError)
	if got, want := DetectTerminalStateFromMergedResponseProxy(errorProxy), RouteTerminalStateError; got != want {
		t.Fatalf("terminal state for error proxy = %v, want %v", got, want)
	}

	redirectProxy := response.NewProxy()
	if _, err := redirectProxy.Redirect(
		httptest.NewRequest(http.MethodGet, "/from", nil),
		"/to",
		http.StatusFound,
	); err != nil {
		t.Fatalf("build redirect proxy: %v", err)
	}
	if got, want := DetectTerminalStateFromMergedResponseProxy(redirectProxy), RouteTerminalStateRedirect; got != want {
		t.Fatalf("terminal state for redirect proxy = %v, want %v", got, want)
	}

	okProxy := response.NewProxy()
	okProxy.SetStatus(http.StatusOK)
	if got, want := DetectTerminalStateFromMergedResponseProxy(okProxy), RouteTerminalStateNone; got != want {
		t.Fatalf("terminal state for OK proxy = %v, want %v", got, want)
	}
}

func TestBuildRouteAssets(t *testing.T) {
	t.Run("validates_required_inputs", func(t *testing.T) {
		_, err := BuildRouteAssets(BuildRouteAssetsInput{})
		if err == nil || !strings.Contains(err.Error(), "route result is nil") {
			t.Fatalf("expected nil route result error, got %v", err)
		}

		_, err = BuildRouteAssets(BuildRouteAssetsInput{
			RouteResult: &RouteResult{},
		})
		if err == nil ||
			!strings.Contains(err.Error(), "route result core is nil") {
			t.Fatalf("expected nil route core error, got %v", err)
		}

		_, err = BuildRouteAssets(BuildRouteAssetsInput{
			RouteResult: &RouteResult{
				Core: &RouteDataCore{},
			},
		})
		if err == nil ||
			!strings.Contains(
				err.Error(),
				"head elements sorting callback is nil",
			) {
			t.Fatalf("expected nil callback error, got %v", err)
		}
	})

	t.Run(
		"production_html_appends_modulepreload_and_css_links",
		func(t *testing.T) {
			defaultHeadEls := []*htmlutil.Element{
				{
					Tag: "meta",
					AttributesKnownSafe: map[string]string{
						"name":    "description",
						"content": "default",
					},
				},
			}
			routeHeadEls := []*htmlutil.Element{
				{Tag: "title", TextContent: "Items"},
			}

			var callbackInput []*htmlutil.Element
			callbackResult := &headels.SortedAndPreEscapedHeadEls{
				Title: &htmlutil.Element{Tag: "title", TextContent: "Sorted"},
			}
			assets, err := BuildRouteAssets(BuildRouteAssetsInput{
				RouteResult: &RouteResult{
					Core: &RouteDataCore{
						Deps: []string{
							"vorma_out/chunk-layout.js",
							"vorma_out/chunk-items.js",
						},
					},
					HeadElements: routeHeadEls,
					CSSBundles:   []string{"vorma_out/chunk-items.css"},
					IsDev:        false,
				},
				DefaultHeadElements: defaultHeadEls,
				IsJSON:              false,
				PublicPathPrefix:    "/public/",
				ToSortedAndPreEscapedHeadElsFn: func(
					input []*htmlutil.Element,
				) *headels.SortedAndPreEscapedHeadEls {
					callbackInput = append([]*htmlutil.Element(nil), input...)
					return callbackResult
				},
			})
			if err != nil {
				t.Fatalf("BuildRouteAssets: %v", err)
			}
			if assets.SortedAndPreEscapedHeadEls != callbackResult {
				t.Fatal("expected route assets to use callback sorting output")
			}
			if got, want := assets.Deps, []string{
				"/public/vorma_out/chunk-layout.js",
				"/public/vorma_out/chunk-items.js",
			}; !reflect.DeepEqual(
				got,
				want,
			) {
				t.Fatalf("Deps = %#v, want %#v", got, want)
			}
			if got, want := assets.CSSBundles, []string{"/public/vorma_out/chunk-items.css"}; !reflect.DeepEqual(
				got,
				want,
			) {
				t.Fatalf("CSSBundles = %#v, want %#v", got, want)
			}
			if got, want := assets.ViteDevURL, ""; got != want {
				t.Fatalf("ViteDevURL = %q, want %q", got, want)
			}

			if got, want := len(callbackInput), 5; got != want {
				t.Fatalf("len(callbackInput) = %d, want %d", got, want)
			}
			if callbackInput[0].Tag != "meta" ||
				callbackInput[1].Tag != "title" {
				t.Fatalf(
					"expected default and route head elements first, got tags %q and %q",
					callbackInput[0].Tag,
					callbackInput[1].Tag,
				)
			}
			if got, want := callbackInput[2].AttributesKnownSafe["rel"], "modulepreload"; got != want {
				t.Fatalf("first preload rel = %q, want %q", got, want)
			}
			if got, want := callbackInput[2].AttributesKnownSafe["href"], "/public/vorma_out/chunk-layout.js"; got != want {
				t.Fatalf("first preload href = %q, want %q", got, want)
			}
			if got, want := callbackInput[3].AttributesKnownSafe["href"], "/public/vorma_out/chunk-items.js"; got != want {
				t.Fatalf("second preload href = %q, want %q", got, want)
			}
			if got, want := callbackInput[4].AttributesKnownSafe["rel"], "stylesheet"; got != want {
				t.Fatalf("stylesheet rel = %q, want %q", got, want)
			}
			if got, want := callbackInput[4].AttributesKnownSafe["href"], "/public/vorma_out/chunk-items.css"; got != want {
				t.Fatalf("stylesheet href = %q, want %q", got, want)
			}
			if got, want := callbackInput[4].Attributes["data-vorma-css-bundle"], "/public/vorma_out/chunk-items.css"; got != want {
				t.Fatalf("data-vorma-css-bundle = %q, want %q", got, want)
			}
		},
	)

	t.Run("dev_or_json_modes_skip_production_links", func(t *testing.T) {
		testCases := []struct {
			name   string
			isDev  bool
			isJSON bool
		}{
			{
				name:   "dev_html",
				isDev:  true,
				isJSON: false,
			},
			{
				name:   "prod_json",
				isDev:  false,
				isJSON: true,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				var callbackInput []*htmlutil.Element
				_, err := BuildRouteAssets(BuildRouteAssetsInput{
					RouteResult: &RouteResult{
						Core: &RouteDataCore{
							Deps: []string{"vorma_out/chunk.js"},
						},
						HeadElements: []*htmlutil.Element{
							{Tag: "title", TextContent: "Route"},
						},
						CSSBundles: []string{"vorma_out/chunk.css"},
						IsDev:      testCase.isDev,
					},
					DefaultHeadElements: []*htmlutil.Element{
						{
							Tag:                 "meta",
							AttributesKnownSafe: map[string]string{"name": "x"},
						},
					},
					IsJSON:           testCase.isJSON,
					PublicPathPrefix: "/public/",
					ToSortedAndPreEscapedHeadElsFn: func(
						input []*htmlutil.Element,
					) *headels.SortedAndPreEscapedHeadEls {
						callbackInput = append(
							[]*htmlutil.Element(nil),
							input...)
						return &headels.SortedAndPreEscapedHeadEls{}
					},
				})
				if err != nil {
					t.Fatalf("BuildRouteAssets: %v", err)
				}
				if got, want := len(callbackInput), 2; got != want {
					t.Fatalf("len(callbackInput) = %d, want %d", got, want)
				}
				for _, element := range callbackInput {
					if element.Tag == "link" {
						t.Fatalf(
							"did not expect production asset link in %s mode",
							testCase.name,
						)
					}
				}
			})
		}
	})

	t.Run(
		"production_html_static_only_head_uses_cached_sorted_output",
		func(t *testing.T) {
			cached := &CachedItemSubset{}
			routeResult := &RouteResult{
				Cached: cached,
				Core: &RouteDataCore{
					Deps: []string{
						"vorma_out/chunk-layout.js",
						"vorma_out/chunk-items.js",
					},
				},
				CSSBundles: []string{"vorma_out/chunk-items.css"},
				IsDev:      false,
			}

			sortCallbackCallCount := 0
			sortCallback := func(
				input []*htmlutil.Element,
			) *headels.SortedAndPreEscapedHeadEls {
				sortCallbackCallCount++
				return &headels.SortedAndPreEscapedHeadEls{
					Rest: append([]*htmlutil.Element(nil), input...),
				}
			}

			assetsFirst, firstError := BuildRouteAssets(BuildRouteAssetsInput{
				RouteResult:                    routeResult,
				IsJSON:                         false,
				ToSortedAndPreEscapedHeadElsFn: sortCallback,
			})
			if firstError != nil {
				t.Fatalf("BuildRouteAssets first call: %v", firstError)
			}
			assetsSecond, secondError := BuildRouteAssets(BuildRouteAssetsInput{
				RouteResult:                    routeResult,
				IsJSON:                         false,
				ToSortedAndPreEscapedHeadElsFn: sortCallback,
			})
			if secondError != nil {
				t.Fatalf("BuildRouteAssets second call: %v", secondError)
			}

			if got, want := sortCallbackCallCount, 1; got != want {
				t.Fatalf("sort callback call count = %d, want %d", got, want)
			}
			if assetsFirst.SortedAndPreEscapedHeadEls != assetsSecond.SortedAndPreEscapedHeadEls {
				t.Fatal("expected cached static sorted head output pointer reuse")
			}
			if got, want := len(assetsFirst.SortedAndPreEscapedHeadEls.Rest), 3; got != want {
				t.Fatalf("len(Rest) = %d, want %d", got, want)
			}
		},
	)

	t.Run(
		"production_html_static_only_head_cache_keys_on_deps_and_css",
		func(t *testing.T) {
			cached := &CachedItemSubset{}
			sortCallbackCallCount := 0
			sortCallback := func(
				input []*htmlutil.Element,
			) *headels.SortedAndPreEscapedHeadEls {
				sortCallbackCallCount++
				return &headels.SortedAndPreEscapedHeadEls{
					Rest: append([]*htmlutil.Element(nil), input...),
				}
			}

			routeResultA := &RouteResult{
				Cached: cached,
				Core: &RouteDataCore{
					Deps: []string{"vorma_out/chunk-a.js"},
				},
				CSSBundles: []string{"vorma_out/chunk-a.css"},
				IsDev:      false,
			}
			routeResultB := &RouteResult{
				Cached: cached,
				Core: &RouteDataCore{
					Deps: []string{"vorma_out/chunk-b.js"},
				},
				CSSBundles: []string{"vorma_out/chunk-b.css"},
				IsDev:      false,
			}

			_, errA1 := BuildRouteAssets(BuildRouteAssetsInput{
				RouteResult:                    routeResultA,
				IsJSON:                         false,
				ToSortedAndPreEscapedHeadElsFn: sortCallback,
			})
			if errA1 != nil {
				t.Fatalf("BuildRouteAssets route A first call: %v", errA1)
			}

			_, errB := BuildRouteAssets(BuildRouteAssetsInput{
				RouteResult:                    routeResultB,
				IsJSON:                         false,
				ToSortedAndPreEscapedHeadElsFn: sortCallback,
			})
			if errB != nil {
				t.Fatalf("BuildRouteAssets route B call: %v", errB)
			}

			_, errA2 := BuildRouteAssets(BuildRouteAssetsInput{
				RouteResult:                    routeResultA,
				IsJSON:                         false,
				ToSortedAndPreEscapedHeadElsFn: sortCallback,
			})
			if errA2 != nil {
				t.Fatalf("BuildRouteAssets route A second call: %v", errA2)
			}

			if got, want := sortCallbackCallCount, 2; got != want {
				t.Fatalf("sort callback call count = %d, want %d", got, want)
			}
		},
	)
}

func TestGetViteDevURLForMode(t *testing.T) {
	if got, want := GetViteDevURLForMode(false), ""; got != want {
		t.Fatalf("GetViteDevURLForMode(false) = %q, want %q", got, want)
	}

	devURL := GetViteDevURLForMode(true)
	if !strings.HasPrefix(devURL, "http://localhost:") {
		t.Fatalf(
			"GetViteDevURLForMode(true) = %q, want localhost URL",
			devURL,
		)
	}
}

func TestBuildCachedItemSubset_UsesOutPathAndHandlesMissingRouteMetadata(
	t *testing.T,
) {
	matchResults := buildPipelineMatchResults(
		t,
		[]string{"/products/:id", "/products/:id/details"},
		"/products/42/details",
	)
	paths := map[string]*PathData{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.tsx",
			OutPath:         "vorma_out/routes/products.$id.js",
			ExportKey:       "ProductRoute",
			ErrorExportKey:  "ProductErrorBoundary",
			Deps:            []string{"vorma_out/chunk-products.js"},
		},
	}

	cached := BuildCachedItemSubset(
		matchResults.Matches,
		paths,
		[]string{"vorma_out/chunk-client.js"},
		false,
	)

	if got, want := cached.ImportURLs, []string{
		"/vorma_out/routes/products.$id.js",
		"",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ImportURLs = %#v, want %#v", got, want)
	}
	if got, want := cached.MatchedPatterns, []string{
		"/products/:id",
		"/products/:id/details",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("MatchedPatterns = %#v, want %#v", got, want)
	}
	if got, want := cached.ExportKeys, []string{"ProductRoute", ""}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("ExportKeys = %#v, want %#v", got, want)
	}
	if got, want := cached.ErrorExportKeys, []string{"ProductErrorBoundary", ""}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("ErrorExportKeys = %#v, want %#v", got, want)
	}
	if got, want := cached.Deps, []string{
		"vorma_out/chunk-client.js",
		"vorma_out/chunk-products.js",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Deps = %#v, want %#v", got, want)
	}
}

func TestLoadOrBuildCachedItemSubset_NoCacheMap(t *testing.T) {
	matchResults := buildPipelineMatchResults(
		t,
		[]string{"/products/:id"},
		"/products/42",
	)

	cached := LoadOrBuildCachedItemSubset(
		"cache-key",
		matchResults.Matches,
		map[string]*PathData{
			"/products/:id": {
				OriginalPattern: "/products/:id",
				SrcPath:         "frontend/src/routes/products.$id.tsx",
				OutPath:         "vorma_out/routes/products.$id.js",
				ExportKey:       "default",
				Deps:            []string{"vorma_out/chunk-products.js"},
			},
		},
		[]string{"vorma_out/chunk-client.js"},
		true,
		1,
		nil,
		nil,
	)

	if cached == nil {
		t.Fatal("expected cached metadata subset when cache map is nil")
	}
	if got, want := cached.ImportURLs, []string{"/frontend/src/routes/products.$id.tsx"}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("ImportURLs = %#v, want %#v", got, want)
	}
}

func TestPlanRouteResultFromResolvedTaskOutcomes_UsesEmptyCachedSubsetWhenMissing(
	t *testing.T,
) {
	input := newRouteStageOnePlannerInputFixture(t)
	input.Cached = nil
	input.LoadersData = []any{
		map[string]any{"route": "parent"},
		map[string]any{"route": "child"},
	}

	result := PlanRouteResultFromResolvedTaskOutcomes(input)
	if result.Core == nil {
		t.Fatal("expected route core for non-terminal result")
	}
	if got, want := result.Core.ImportURLs, []string{"", ""}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("ImportURLs = %#v, want %#v", got, want)
	}
	if got, want := result.Core.ExportKeys, []string{"", ""}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("ExportKeys = %#v, want %#v", got, want)
	}
	if got, want := result.Core.ErrorExportKeys, []string{"", ""}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("ErrorExportKeys = %#v, want %#v", got, want)
	}
}

func TestCollectFlattenedHeadElementsForPrefix_PreservesRouteOrderAndSkipsEmpty(
	t *testing.T,
) {
	parentHead := headels.New()
	parentHead.Title("Parent")
	parentHead.Meta(parentHead.Name("description"), parentHead.Content("p"))

	childHead := headels.New()
	childHead.Link(
		childHead.Rel("stylesheet"),
		childHead.Href("/child.css"),
		childHead.SelfClosing(),
	)

	parentProxy := response.NewProxy()
	parentProxy.AddHeadEls(parentHead)

	noHeadProxy := response.NewProxy()

	childProxy := response.NewProxy()
	childProxy.AddHeadEls(childHead)

	flattened := CollectFlattenedHeadElementsForPrefix(
		[]*response.Proxy{
			parentProxy,
			nil,
			noHeadProxy,
			childProxy,
		},
		4,
	)

	if got, want := len(flattened), 3; got != want {
		t.Fatalf("len(flattened) = %d, want %d", got, want)
	}
	if got, want := flattened[0].Tag, "title"; got != want {
		t.Fatalf("flattened[0].Tag = %q, want %q", got, want)
	}
	if got, want := flattened[1].Tag, "meta"; got != want {
		t.Fatalf("flattened[1].Tag = %q, want %q", got, want)
	}
	if got, want := flattened[2].Tag, "link"; got != want {
		t.Fatalf("flattened[2].Tag = %q, want %q", got, want)
	}
}

func TestCollectFlattenedHeadElementsForPrefix_ReturnsNilWhenNoHeadEls(
	t *testing.T,
) {
	emptyProxy := response.NewProxy()

	flattened := CollectFlattenedHeadElementsForPrefix(
		[]*response.Proxy{nil, emptyProxy},
		2,
	)
	if flattened != nil {
		t.Fatalf("expected nil flattened head elements, got %#v", flattened)
	}
}

func newRouteStageOnePlannerInputFixture(
	t *testing.T,
) RouteStageOnePlannerInput {
	t.Helper()

	matchResults := buildPipelineMatchResults(
		t,
		[]string{"/products/:id", "/products/:id/details"},
		"/products/42/details",
	)
	matches := matchResults.Matches
	if len(matches) != 2 {
		t.Fatalf("expected exactly 2 nested matches, got %d", len(matches))
	}

	paths := map[string]*PathData{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.tsx",
			OutPath:         "vorma_out/routes/products.$id.js",
			ExportKey:       "default",
			ErrorExportKey:  "ProductsErrorBoundary",
			Deps:            []string{"vorma_out/products.js"},
		},
		"/products/:id/details": {
			OriginalPattern: "/products/:id/details",
			SrcPath:         "frontend/src/routes/products.$id.details.tsx",
			OutPath:         "vorma_out/routes/products.$id.details.js",
			ExportKey:       "default",
			ErrorExportKey:  "ProductDetailsErrorBoundary",
			Deps:            []string{"vorma_out/product-details.js"},
		},
	}
	clientEntryDeps := []string{"vorma_out/client-shared.js"}
	cached := BuildCachedItemSubset(matches, paths, clientEntryDeps, true)

	parentHeadEls := headels.New()
	parentHeadEls.Title("Parent")
	childHeadEls := headels.New()
	childHeadEls.Meta(
		childHeadEls.Name("description"),
		childHeadEls.Content("Child"),
	)

	parentProxy := response.NewProxy()
	parentProxy.AddHeadEls(parentHeadEls)
	childProxy := response.NewProxy()
	childProxy.AddHeadEls(childHeadEls)

	return RouteStageOnePlannerInput{
		MatchResults:    matchResults,
		Matches:         matches,
		MatchedPatterns: CollectMatchedPatterns(matches),
		Cached:          cached,
		RuntimeSnapshot: RuntimeSnapshot{
			BuildID:         "planner-build",
			IsDev:           true,
			Paths:           paths,
			ClientEntryDeps: clientEntryDeps,
			ClientEntryOut:  "vorma_out/client-entry.js",
			DepToCSSBundleMap: map[string][]string{
				"vorma_out/client-entry.js":  {"vorma_out/client-entry.css"},
				"vorma_out/client-shared.js": {"vorma_out/client-shared.css"},
				"vorma_out/products.js":      {"vorma_out/products.css"},
				"vorma_out/product-details.js": {
					"vorma_out/product-details.css",
				},
			},
			HTMLRenderSnapshot: rendering.LoadersHTMLRenderSnapshot{
				IsDevMode:      true,
				ClientEntryOut: "vorma_out/client-entry.js",
			},
			RouteManifestFile: "vorma_out/route-manifest.js",
		},
		ResponseProxies: []*response.Proxy{parentProxy, childProxy},
		MergedResponseProxy: response.MergeProxyResponses(
			parentProxy,
			childProxy,
		),
	}
}

func buildPipelineMatchResults(
	t *testing.T,
	patterns []string,
	requestPath string,
) *nestedmatcher.Results {
	t.Helper()

	m := nestedmatcher.New(nil)
	for _, pattern := range patterns {
		m.RegisterPattern(pattern)
	}

	matchResults, found := m.FindMatches(requestPath)
	if !found {
		t.Fatalf("expected nested match for request path %q", requestPath)
	}

	return matchResults
}

func intPointerForRoutePipelineTest(value int) *int {
	out := value
	return &out
}

func createRequestWithTasksCtxForRoutePipelineTest(
	method string,
	url string,
) *http.Request {
	request := httptest.NewRequest(method, url, nil)
	tasksCtx := tasks.NewCtx(request.Context())
	return mux.RequestWithTasksCtx(request, tasksCtx)
}

func cacheLen(cache *sync.Map) int {
	if cache == nil {
		return 0
	}
	length := 0
	cache.Range(func(_, _ any) bool {
		length++
		return true
	})
	return length
}
