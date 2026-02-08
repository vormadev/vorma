package wire_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vormadev/vorma"
)

func TestWireQueryConformance(t *testing.T) {
	t.Run("WRC-Q-001_WIRE-Q-001_only_non_empty_vorma_json_activates_json_mode", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths([]wirePathDef{
				{Pattern: "/q1/page", SrcPath: "src/q1/page.tsx", OutPath: "routes/q1_page.js", ExportKey: "Page"},
			}, nil),
			nil,
		)
		vorma.NewLoader(
			app,
			"/q1/page",
			func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		testCases := []struct {
			name       string
			path       string
			wantJSON   bool
			wantReload bool
			wantStatus int
		}{
			{
				name:       "no_vorma_json",
				path:       "/q1/page",
				wantJSON:   false,
				wantReload: false,
				wantStatus: http.StatusOK,
			},
			{
				name:       "empty_vorma_json",
				path:       "/q1/page?vorma_json=",
				wantJSON:   false,
				wantReload: false,
				wantStatus: http.StatusOK,
			},
			{
				name:       "non_empty_vorma_json",
				path:       "/q1/page?vorma_json=abc",
				wantJSON:   true,
				wantReload: true,
				wantStatus: http.StatusOK,
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, tc.path, nil)
				r.ServeHTTP(rec, req)

				if rec.Code != tc.wantStatus {
					t.Fatalf("expected status %d, got %d body=%q", tc.wantStatus, rec.Code, rec.Body.String())
				}

				isJSON := strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json")
				if isJSON != tc.wantJSON {
					t.Fatalf("expected JSON mode=%v, got content-type %q", tc.wantJSON, rec.Header().Get("Content-Type"))
				}

				hasReload := rec.Header().Get("X-Vorma-Reload") != ""
				if hasReload != tc.wantReload {
					t.Fatalf("expected reload header present=%v, got %q", tc.wantReload, rec.Header().Get("X-Vorma-Reload"))
				}
			})
		}
	})

	t.Run("WRC-Q-002_WIRE-Q-002_current_build_returns_route_data_stale_returns_reload_sentinel", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths([]wirePathDef{
				{Pattern: "/q2/page", SrcPath: "src/q2/page.tsx", OutPath: "routes/q2_page.js", ExportKey: "Page"},
			}, nil),
			nil,
		)
		vorma.NewLoader(
			app,
			"/q2/page",
			func(*vorma.LoaderReqData) (map[string]any, error) { return map[string]any{"ok": true}, nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		currentRec := httptest.NewRecorder()
		currentReq := httptest.NewRequest(http.MethodGet, "/q2/page?vorma_json="+wireBuildID, nil)
		r.ServeHTTP(currentRec, currentReq)
		if currentRec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, currentRec.Code, currentRec.Body.String())
		}
		currentBody := mustJSONMap(t, currentRec.Body.Bytes())
		if _, has := currentBody["matchedPatterns"]; !has {
			t.Fatalf("expected current-build route-data payload, got %#v", currentBody)
		}
		if _, has := currentBody["loadersData"]; !has {
			t.Fatalf("expected current-build payload to include loadersData, got %#v", currentBody)
		}
		if got := currentRec.Header().Get("X-Vorma-Reload"); got != "" {
			t.Fatalf("expected no stale reload header for current build, got %q", got)
		}

		staleRec := httptest.NewRecorder()
		staleReq := httptest.NewRequest(http.MethodGet, "/q2/page?vorma_json=other", nil)
		r.ServeHTTP(staleRec, staleReq)
		if staleRec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, staleRec.Code, staleRec.Body.String())
		}
		if got := staleRec.Header().Get("X-Vorma-Reload"); got != "/q2/page" {
			t.Fatalf("expected X-Vorma-Reload=%q, got %q", "/q2/page", got)
		}
		staleBody := mustJSONMap(t, staleRec.Body.Bytes())
		if len(staleBody) != 1 || staleBody["ok"] != true {
			t.Fatalf("expected stale sentinel body {\"ok\":true}, got %#v", staleBody)
		}
	})

	t.Run("WRC-Q-003_WIRE-Q-003_server_tolerates_vorma_reload_query_key", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths([]wirePathDef{
				{Pattern: "/q3/page", SrcPath: "src/q3/page.tsx", OutPath: "routes/q3_page.js", ExportKey: "Page"},
			}, nil),
			nil,
		)
		vorma.NewLoader(
			app,
			"/q3/page",
			func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		docRec := httptest.NewRecorder()
		docReq := httptest.NewRequest(http.MethodGet, "/q3/page?vorma_reload=token123", nil)
		r.ServeHTTP(docRec, docReq)
		if docRec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, docRec.Code, docRec.Body.String())
		}
		if got := docRec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
			t.Fatalf("expected document-mode content-type text/html, got %q", got)
		}

		jsonRec := httptest.NewRecorder()
		jsonReq := httptest.NewRequest(http.MethodGet, "/q3/page?vorma_json="+wireBuildID+"&vorma_reload=token123", nil)
		r.ServeHTTP(jsonRec, jsonReq)
		if jsonRec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, jsonRec.Code, jsonRec.Body.String())
		}
		if got := jsonRec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
			t.Fatalf("expected JSON-mode content-type application/json, got %q", got)
		}
		jsonBody := mustJSONMap(t, jsonRec.Body.Bytes())
		if _, has := jsonBody["matchedPatterns"]; !has {
			t.Fatalf("expected normal JSON route-data payload with vorma_reload present, got %#v", jsonBody)
		}
	})
}
