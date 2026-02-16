package vormaruntime

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/response"
)

func TestPlanRouteResultFromResolvedTaskOutcomes_TerminalMergedProxyShortCircuits(t *testing.T) {
	redirectProxy := response.NewProxy()
	if _, err := redirectProxy.Redirect(
		httptest.NewRequest(http.MethodGet, "/from", nil),
		"/to",
	); err != nil {
		t.Fatalf("build redirect proxy: %v", err)
	}

	result := planRouteResultFromResolvedTaskOutcomes(routeStageOnePlannerInput{
		runtimeSnapshot: RuntimeSnapshot{
			buildID: "build-terminal",
		},
		mergedResponseProxy: redirectProxy,
	})

	if got, want := result.terminalState, routeTerminalStateRedirect; got != want {
		t.Fatalf("terminalState = %v, want %v", got, want)
	}
	if got, want := result.buildID, "build-terminal"; got != want {
		t.Fatalf("buildID = %q, want %q", got, want)
	}
	if result.core != nil {
		t.Fatalf("core should be nil for terminal planner outputs, got %#v", result.core)
	}
	if result.mergedResponseProxy != redirectProxy {
		t.Fatal("terminal planner output should reuse merged response proxy")
	}
}

func TestPlanRouteResultFromResolvedTaskOutcomes_ErrorAtPrefixRouteTruncatesPayloadAndDeps(t *testing.T) {
	input := newRouteStageOnePlannerInputFixture(t)
	input.hasRootData = true
	input.loadersData = []any{
		map[string]any{"route": "parent"},
		map[string]any{"route": "child"},
	}
	input.outermostLoaderErrorIndex = intPointerForPlannerTest(0)
	input.clientLoaderErrorMessage = "safe-client-error"

	result := planRouteResultFromResolvedTaskOutcomes(input)

	if got, want := result.terminalState, routeTerminalStateNone; got != want {
		t.Fatalf("terminalState = %v, want %v", got, want)
	}
	if result.core == nil {
		t.Fatal("core should be present for non-terminal planner outputs")
	}
	if got, want := result.core.OutermostServerError, "safe-client-error"; got != want {
		t.Fatalf("OutermostServerError = %q, want %q", got, want)
	}
	if result.core.OutermostServerErrorIdx == nil || *result.core.OutermostServerErrorIdx != 0 {
		t.Fatalf("OutermostServerErrorIdx = %#v, want 0", result.core.OutermostServerErrorIdx)
	}
	if got, want := result.core.MatchedPatterns, []string{"/products/:id"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("MatchedPatterns = %#v, want %#v", got, want)
	}
	if got, want := len(result.core.LoadersData), 1; got != want {
		t.Fatalf("len(LoadersData) = %d, want %d", got, want)
	}
	if got, want := result.core.Deps, []string{"vorma_out/client-shared.js", "vorma_out/products.js"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Deps = %#v, want %#v", got, want)
	}
	if got, want := len(result.headElements), 0; got != want {
		t.Fatalf("len(headElements) = %d, want %d", got, want)
	}
	if got, want := result.cssBundles, []string{
		"vorma_out/client-entry.css",
		"vorma_out/client-shared.css",
		"vorma_out/products.css",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cssBundles = %#v, want %#v", got, want)
	}
}

func TestPlanRouteResultFromResolvedTaskOutcomes_SuccessBuildsFullRouteCoreAndAssets(t *testing.T) {
	input := newRouteStageOnePlannerInputFixture(t)
	input.loadersData = []any{
		map[string]any{"route": "parent"},
		map[string]any{"route": "child"},
	}

	result := planRouteResultFromResolvedTaskOutcomes(input)

	if got, want := result.terminalState, routeTerminalStateNone; got != want {
		t.Fatalf("terminalState = %v, want %v", got, want)
	}
	if result.core == nil {
		t.Fatal("core should be present for success planner outputs")
	}
	if result.core.OutermostServerErrorIdx != nil {
		t.Fatalf("OutermostServerErrorIdx should be nil for success output, got %#v", result.core.OutermostServerErrorIdx)
	}
	if got, want := result.core.MatchedPatterns, []string{"/products/:id", "/products/:id/details"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("MatchedPatterns = %#v, want %#v", got, want)
	}
	if got, want := result.core.Deps, []string{
		"vorma_out/client-shared.js",
		"vorma_out/products.js",
		"vorma_out/product-details.js",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Deps = %#v, want %#v", got, want)
	}
	if got, want := result.cssBundles, []string{
		"vorma_out/client-entry.css",
		"vorma_out/client-shared.css",
		"vorma_out/products.css",
		"vorma_out/product-details.css",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cssBundles = %#v, want %#v", got, want)
	}
	if got, want := len(result.headElements), 2; got != want {
		t.Fatalf("len(headElements) = %d, want %d", got, want)
	}
	if got, want := result.routeManifestFileSnapshot, "vorma_out/route-manifest.js"; got != want {
		t.Fatalf("routeManifestFileSnapshot = %q, want %q", got, want)
	}
	if got, want := result.htmlRenderSnapshot.clientEntryOut, "vorma_out/client-entry.js"; got != want {
		t.Fatalf("htmlRenderSnapshot.clientEntryOut = %q, want %q", got, want)
	}
	if !result.isDev {
		t.Fatal("isDev should be true in success planner output")
	}
}

func newRouteStageOnePlannerInputFixture(t *testing.T) routeStageOnePlannerInput {
	t.Helper()

	matchResults := buildPlannerMatchResults(
		t,
		[]string{"/products/:id", "/products/:id/details"},
		"/products/42/details",
	)
	matches := matchResults.Matches
	if len(matches) != 2 {
		t.Fatalf("expected exactly 2 nested matches, got %d", len(matches))
	}

	paths := map[string]*Path{
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
	cached := buildCachedItemSubset(matches, paths, clientEntryDeps, true)

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

	return routeStageOnePlannerInput{
		matchResults:    matchResults,
		matches:         matches,
		matchedPatterns: collectMatchedPatterns(matches),
		cached:          cached,
		runtimeSnapshot: RuntimeSnapshot{
			buildID:         "planner-build",
			isDev:           true,
			paths:           paths,
			clientEntryDeps: clientEntryDeps,
			clientEntryOut:  "vorma_out/client-entry.js",
			depToCSSBundleMap: map[string][]string{
				"vorma_out/client-entry.js":    {"vorma_out/client-entry.css"},
				"vorma_out/client-shared.js":   {"vorma_out/client-shared.css"},
				"vorma_out/products.js":        {"vorma_out/products.css"},
				"vorma_out/product-details.js": {"vorma_out/product-details.css"},
			},
			routeManifestFile: "vorma_out/route-manifest.js",
		},
		responseProxies:     []*response.Proxy{parentProxy, childProxy},
		mergedResponseProxy: response.MergeProxyResponses(parentProxy, childProxy),
	}
}

func buildPlannerMatchResults(
	t *testing.T,
	patterns []string,
	requestPath string,
) *matcher.FindNestedMatchesResults {
	t.Helper()

	m := matcher.New(nil)
	for _, pattern := range patterns {
		m.RegisterPattern(pattern)
	}

	matchResults, found := m.FindNestedMatches(requestPath)
	if !found {
		t.Fatalf("expected nested match for request path %q", requestPath)
	}

	return matchResults
}

func intPointerForPlannerTest(value int) *int {
	out := value
	return &out
}
