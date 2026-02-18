package vormaruntime

import (
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/mux"
)

func TestGetSSRInnerHTML_ContainsExpectedRuntimeFields(t *testing.T) {
	stage := defaultPathsFile("build-ssr", map[string]*Path{
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

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne:         stage,
		stageTwo:         stage,
		publicPathPrefix: "/static/",
	})
	app := fixture.app
	app.SetIsDev(true)

	routeData := &RouteDataFinal{
		RouteDataCore: &RouteDataCore{
			OutermostServerError: "",
			ErrorExportKeys:      []string{"ItemErrorBoundary"},
			MatchedPatterns:      []string{"/items/:id"},
			LoadersData:          []any{map[string]any{"id": "42"}},
			ImportURLs:           []string{"/vorma_out/routes/items.$id.js"},
			ExportKeys:           []string{"default"},
			HasRootData:          false,
			Params:               mux.Params{"id": "42"},
			SplatValues:          SplatValues{"detail"},
			Deps:                 []string{"vorma_out/chunk-items.js"},
		},
		CSSBundles: []string{"vorma_out/chunk-items.css"},
		ViteDevURL: "http://localhost:5173",
	}

	out, err := app.getSSRInnerHTML(routeData)
	if err != nil {
		t.Fatalf("getSSRInnerHTML returned error: %v", err)
	}
	if out == nil || out.Script == nil {
		t.Fatal("expected non-nil SSR output and script")
	}
	if out.Sha256Hash == "" {
		t.Fatal("expected non-empty SSR script sha256 hash")
	}

	script := string(*out.Script)
	for _, expected := range []string{
		`<script type="module">`,
		`Symbol.for("__vorma_internal__")`,
		`x.isDev =`,
		`x.buildID = "build-ssr";`,
		`x.rootElementID = "vorma-root";`,
		`x.publicPathPrefix = "\/static\/";`,
		`x.routeManifestURL = "/static/vorma_out/route-manifest.js";`,
		`x.matchedPatterns = ["/items/:id"];`,
		`x.importURLs = ["/vorma_out/routes/items.$id.js"];`,
		`x.cssBundles = ["vorma_out/chunk-items.css"];`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf(
				"SSR script missing expected fragment %q\nscript=%s",
				expected,
				script,
			)
		}
	}
	if !strings.Contains(script, "true") {
		t.Fatalf("SSR script should mark dev mode as true\nscript=%s", script)
	}
}

func TestGetSSRInnerHTML_HashChangesWhenPayloadChanges(t *testing.T) {
	stage := defaultPathsFile("build-ssr-hash", map[string]*Path{
		"/": {
			OriginalPattern: "/",
			SrcPath:         "frontend/src/routes/root.tsx",
			OutPath:         "vorma_out/root.js",
			ExportKey:       "default",
		},
	})
	fixture := newTestFixture(
		t,
		testFixtureOptions{stageOne: stage, stageTwo: stage},
	)
	app := fixture.app

	base := &RouteDataFinal{
		RouteDataCore: &RouteDataCore{
			MatchedPatterns: []string{"/"},
			LoadersData:     []any{map[string]any{"ok": true}},
			ImportURLs:      []string{"/vorma_out/root.js"},
			ExportKeys:      []string{"default"},
		},
		CSSBundles: []string{"vorma_out/root.css"},
	}
	mutated := &RouteDataFinal{
		RouteDataCore: &RouteDataCore{
			MatchedPatterns: []string{"/"},
			LoadersData:     []any{map[string]any{"ok": false}},
			ImportURLs:      []string{"/vorma_out/root.js"},
			ExportKeys:      []string{"default"},
		},
		CSSBundles: []string{"vorma_out/root.css", "vorma_out/extra.css"},
	}

	out1, err := app.getSSRInnerHTML(base)
	if err != nil {
		t.Fatalf("getSSRInnerHTML(base): %v", err)
	}
	out2, err := app.getSSRInnerHTML(mutated)
	if err != nil {
		t.Fatalf("getSSRInnerHTML(mutated): %v", err)
	}
	if out1.Sha256Hash == out2.Sha256Hash {
		t.Fatalf(
			"expected hash to change when SSR payload changes, got same hash %q",
			out1.Sha256Hash,
		)
	}
}

func TestGetSSRInnerHTML_VercelDeploymentIDGate(t *testing.T) {
	stage := defaultPathsFile("build-ssr-env", map[string]*Path{
		"/": {
			OriginalPattern: "/",
			SrcPath:         "frontend/src/routes/root.tsx",
			OutPath:         "vorma_out/root.js",
			ExportKey:       "default",
		},
	})
	fixture := newTestFixture(
		t,
		testFixtureOptions{stageOne: stage, stageTwo: stage},
	)
	app := fixture.app

	routeData := &RouteDataFinal{RouteDataCore: &RouteDataCore{}}

	t.Setenv("VERCEL_SKEW_PROTECTION_ENABLED", "true")
	t.Setenv("VERCEL_DEPLOYMENT_ID", "dep-123")

	out, err := app.getSSRInnerHTML(routeData)
	if err != nil {
		t.Fatalf("getSSRInnerHTML returned error: %v", err)
	}
	script := string(*out.Script)
	if !strings.Contains(script, `x.deploymentID = "dep-123";`) {
		t.Fatalf(
			"expected deployment ID in SSR script when skew protection is enabled\nscript=%s",
			script,
		)
	}
}

func TestGetSSRInnerHTML_NilRouteDataReturnsError(t *testing.T) {
	stage := defaultPathsFile("build-ssr-nil", map[string]*Path{
		"/": {
			OriginalPattern: "/",
			SrcPath:         "frontend/src/routes/root.tsx",
			OutPath:         "vorma_out/root.js",
			ExportKey:       "default",
		},
	})
	fixture := newTestFixture(
		t,
		testFixtureOptions{stageOne: stage, stageTwo: stage},
	)
	app := fixture.app

	out, err := app.getSSRInnerHTML(nil)
	if err == nil {
		t.Fatal("expected error for nil routeData, got nil")
	}
	if out != nil {
		t.Fatalf("expected nil output when routeData is nil, got %#v", out)
	}
}

func TestGetSSRInnerHTML_NilRouteDataCoreReturnsError(t *testing.T) {
	stage := defaultPathsFile("build-ssr-nil-core", map[string]*Path{
		"/": {
			OriginalPattern: "/",
			SrcPath:         "frontend/src/routes/root.tsx",
			OutPath:         "vorma_out/root.js",
			ExportKey:       "default",
		},
	})
	fixture := newTestFixture(
		t,
		testFixtureOptions{stageOne: stage, stageTwo: stage},
	)
	app := fixture.app

	out, err := app.getSSRInnerHTML(&RouteDataFinal{})
	if err == nil {
		t.Fatal("expected error for nil RouteDataCore, got nil")
	}
	if out != nil {
		t.Fatalf("expected nil output when RouteDataCore is nil, got %#v", out)
	}
}
