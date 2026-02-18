package vormaruntime

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/vormadev/vorma/kit/mux"
)

func TestGetDepsFromSnapshot_ClientEntryFirstAndDeduped(t *testing.T) {
	stage := defaultPathsFile("deps-build", map[string]*Path{
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
	})
	stage.ClientEntryDeps = []string{
		"vorma_out/client.js",
		"vorma_out/shared.js",
	}
	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app

	nr := mux.NewNestedRouter(nil)
	mux.AddNestedPatternWithoutHandler(nr, "")
	mux.AddNestedPatternWithoutHandler(nr, "/items/:id")

	req := httptest.NewRequest(http.MethodGet, "/items/42", nil)
	findResults, found := mux.FindNestedMatches(nr, req)
	if !found {
		t.Fatal("expected nested matches for /items/42")
	}

	deps := app.getDeps(findResults.Matches, app.Paths())
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
	stage := defaultPathsFile("css-build", map[string]*Path{
		"/items/:id": {
			OriginalPattern: "/items/:id",
			Deps: []string{
				"vorma_out/item.js",
				"vorma_out/shared.js",
			},
		},
	})
	stage.ClientEntryOut = "vorma_out/client-entry.js"
	stage.DepToCSSBundleMap = map[string][]string{
		"vorma_out/client-entry.js": {
			"vorma_out/client.css",
			"vorma_out/shared.css",
		},
		"vorma_out/shared.js": {
			"vorma_out/shared.css",
			"vorma_out/layout.css",
		},
		"vorma_out/item.js": {"vorma_out/item.css"},
	}
	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app

	css := app.getCSSBundles(
		[]string{"vorma_out/shared.js", "vorma_out/item.js"},
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
