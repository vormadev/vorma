package wire_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vormadev/vorma"
)

func TestWireContentTypeConformance(t *testing.T) {
	t.Setenv("WAVE_MODE", "")

	app := makeWireApp(
		t,
		makeWireConfigJSON(t, nil, nil),
		makeWireStaticFSWithPaths([]wirePathDef{
			{
				Pattern:   "/ct/page",
				SrcPath:   "src/ct/page.tsx",
				OutPath:   "routes/ct_page.js",
				ExportKey: "Page",
			},
		}, nil),
		nil,
	)
	vorma.NewLoader(
		app,
		"/ct/page",
		func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
		func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
	)
	r := app.InitWithDefaultRouter()

	t.Run("WRC-CT-001_WIRE-CT-001_json_responses_use_application_json", func(t *testing.T) {
		current := httptest.NewRecorder()
		currentReq := httptest.NewRequest(http.MethodGet, "/ct/page?vorma_json="+wireBuildID, nil)
		r.ServeHTTP(current, currentReq)
		if current.Code != http.StatusOK {
			t.Fatalf("current JSON status: expected %d got %d body=%q", http.StatusOK, current.Code, current.Body.String())
		}
		if ct := current.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Fatalf("current JSON content-type: expected application/json got %q", ct)
		}

		stale := httptest.NewRecorder()
		staleReq := httptest.NewRequest(http.MethodGet, "/ct/page?vorma_json=stale", nil)
		r.ServeHTTP(stale, staleReq)
		if stale.Code != http.StatusOK {
			t.Fatalf("stale JSON status: expected %d got %d body=%q", http.StatusOK, stale.Code, stale.Body.String())
		}
		if ct := stale.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Fatalf("stale JSON content-type: expected application/json got %q", ct)
		}
	})

	t.Run("WRC-CT-002_WIRE-CT-002_document_mode_uses_text_html", func(t *testing.T) {
		doc := httptest.NewRecorder()
		docReq := httptest.NewRequest(http.MethodGet, "/ct/page", nil)
		r.ServeHTTP(doc, docReq)
		if doc.Code != http.StatusOK {
			t.Fatalf("document status: expected %d got %d body=%q", http.StatusOK, doc.Code, doc.Body.String())
		}
		if ct := doc.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("document content-type: expected text/html got %q", ct)
		}
	})
}
