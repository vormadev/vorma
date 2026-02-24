package routepipeline

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime/rendering"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/nestedmatcher"
	"github.com/vormadev/vorma/kit/response"
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
