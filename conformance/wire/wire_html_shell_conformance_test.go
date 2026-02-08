package wire_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vormadev/vorma"
)

func TestWireHTMLShellConformance(t *testing.T) {
	t.Run("WRC-HTML-001_WIRE-HTML-001_document_html_initializes_vorma_bootstrap_symbol", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths([]wirePathDef{
				{Pattern: "/html1/page", SrcPath: "src/html1/page.tsx", OutPath: "routes/html1_page.js", ExportKey: "Page"},
			}, nil),
			nil,
		)
		vorma.NewLoader(
			app,
			"/html1/page",
			func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/html1/page", nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, `globalThis[Symbol.for("__vorma_internal__")] = {};`) {
			t.Fatalf("expected SSR bootstrap symbol initialization in HTML body, got %q", body)
		}
	})

	t.Run("WRC-HTML-002_WIRE-HTML-002_bootstrap_includes_required_runtime_keys", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths([]wirePathDef{
				{Pattern: "", SrcPath: "src/html2/root.tsx", OutPath: "routes/html2_root.js", ExportKey: "Root"},
				{Pattern: "/html2/page", SrcPath: "src/html2/page.tsx", OutPath: "routes/html2_page.js", ExportKey: "Page", Deps: []string{"assets/chunk-a.js"}},
			}, &wireStaticOpts{
				IncludeManifestFile: true,
				RouteManifestJSON:   mustJSON(nil, map[string]int{"": 1, "/html2/page": 1}),
			}),
			nil,
		)
		vorma.NewLoader(
			app,
			"",
			func(*vorma.LoaderReqData) (map[string]any, error) {
				return map[string]any{"root": true}, nil
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewLoader(
			app,
			"/html2/page",
			func(rd *vorma.LoaderReqData) (map[string]any, error) {
				h := rd.ResponseProxy().GetHeadEls()
				h.Title("Wire HTML")
				return map[string]any{"page": true}, nil
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/html2/page", nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()

		requiredAssignments := []string{
			"x.patternToWaitFnMap =",
			"x.clientLoadersData =",
			"x.isDev =",
			"x.viteDevURL =",
			"x.buildID =",
			"x.publicPathPrefix =",
			"x.outermostServerError =",
			"x.outermostServerErrorIdx =",
			"x.errorExportKeys =",
			"x.matchedPatterns =",
			"x.loadersData =",
			"x.importURLs =",
			"x.exportKeys =",
			"x.hasRootData =",
			"x.params =",
			"x.splatValues =",
			"x.deps =",
			"x.cssBundles =",
			"x.deploymentID =",
			"x.routeManifestURL =",
		}
		for _, needle := range requiredAssignments {
			if !strings.Contains(body, needle) {
				t.Fatalf("expected bootstrap assignment %q in HTML body, got %q", needle, body)
			}
		}
	})

	t.Run("WRC-HTML-003_WIRE-HTML-003_head_contains_meta_and_rest_marker_comments", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths([]wirePathDef{
				{Pattern: "/html3/page", SrcPath: "src/html3/page.tsx", OutPath: "routes/html3_page.js", ExportKey: "Page"},
			}, nil),
			nil,
		)
		vorma.NewLoader(
			app,
			"/html3/page",
			func(rd *vorma.LoaderReqData) (string, error) {
				h := rd.ResponseProxy().GetHeadEls()
				h.MetaNameContent("description", "html3")
				h.Link(h.Rel("stylesheet"), h.Href("/x.css"), h.SelfClosing())
				return "ok", nil
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/html3/page", nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()

		requiredMarkers := []string{
			`<!-- data-vorma="meta-start" -->`,
			`<!-- data-vorma="meta-end" -->`,
			`<!-- data-vorma="rest-start" -->`,
			`<!-- data-vorma="rest-end" -->`,
		}
		for _, marker := range requiredMarkers {
			if !strings.Contains(body, marker) {
				t.Fatalf("expected head marker %q in HTML body, got %q", marker, body)
			}
		}
	})
}
