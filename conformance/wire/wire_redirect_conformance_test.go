package wire_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/response"
)

func TestWireRedirectConformance(t *testing.T) {
	t.Run("WRC-REDIR-001_WIRE-REDIR-001_accept_header_enables_client_redirect_handshake", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths([]wirePathDef{
				{Pattern: "/redir1/source", SrcPath: "src/redir1/source.tsx", OutPath: "routes/redir1_source.js", ExportKey: "Source"},
			}, nil),
			nil,
		)
		vorma.NewLoader(
			app,
			"/redir1/source",
			func(rd *vorma.LoaderReqData) (string, error) {
				_, err := rd.ResponseProxy().Redirect(rd.Request(), "/redir1/target", http.StatusFound)
				return "", err
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/redir1/source?vorma_json="+wireBuildID, nil)
		req.Header.Set(response.ClientAcceptsRedirectHeader, "1")
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get(response.ClientRedirectHeader); got != "/redir1/target" {
			t.Fatalf("expected %s=%q, got %q", response.ClientRedirectHeader, "/redir1/target", got)
		}
	})

	t.Run("WRC-REDIR-002_WIRE-REDIR-002_stale_reload_signal_wins_before_loader_redirect_logic", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths([]wirePathDef{
				{Pattern: "/redir2/source", SrcPath: "src/redir2/source.tsx", OutPath: "routes/redir2_source.js", ExportKey: "Source"},
			}, nil),
			nil,
		)
		vorma.NewLoader(
			app,
			"/redir2/source",
			func(rd *vorma.LoaderReqData) (string, error) {
				_, err := rd.ResponseProxy().Redirect(rd.Request(), "/redir2/target", http.StatusFound)
				return "", err
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/redir2/source?a=1&vorma_json=stale&b=2", nil)
		req.Header.Set(response.ClientAcceptsRedirectHeader, "1")
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("X-Vorma-Reload"); got != "/redir2/source?a=1&b=2" {
			t.Fatalf("expected stale reload header to remove vorma_json, got %q", got)
		}
		if got := rec.Header().Get("Location"); got != "" {
			t.Fatalf("expected no server redirect Location when stale reload signal is emitted, got %q", got)
		}
		if got := rec.Header().Get(response.ClientRedirectHeader); got != "" {
			t.Fatalf("expected no %s when stale reload short-circuits, got %q", response.ClientRedirectHeader, got)
		}
	})

	t.Run("WRC-REDIR-003_WRC-REDIR-004_WIRE-REDIR-002_server_emits_redirect_indicators_for_client_priority_resolution", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths([]wirePathDef{
				{Pattern: "/redir3/source", SrcPath: "src/redir3/source.tsx", OutPath: "routes/redir3_source.js", ExportKey: "Source"},
				{Pattern: "/redir4/source", SrcPath: "src/redir4/source.tsx", OutPath: "routes/redir4_source.js", ExportKey: "Source"},
			}, nil),
			nil,
		)
		vorma.NewLoader(
			app,
			"/redir3/source",
			func(rd *vorma.LoaderReqData) (string, error) {
				rd.ResponseProxy().SetHeader(response.ClientRedirectHeader, "/redir3/client")
				_, err := rd.ResponseProxy().Redirect(rd.Request(), "/redir3/server", http.StatusFound)
				return "", err
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewLoader(
			app,
			"/redir4/source",
			func(rd *vorma.LoaderReqData) (string, error) {
				_, err := rd.ResponseProxy().Redirect(rd.Request(), "/redir4/client", http.StatusFound)
				return "", err
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		redirectedRec := httptest.NewRecorder()
		redirectedReq := httptest.NewRequest(http.MethodGet, "/redir3/source?vorma_json="+wireBuildID, nil)
		r.ServeHTTP(redirectedRec, redirectedReq)
		if redirectedRec.Code != http.StatusFound {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusFound, redirectedRec.Code, redirectedRec.Body.String())
		}
		if got := redirectedRec.Header().Get("Location"); got != "/redir3/server" {
			t.Fatalf("expected server redirect Location=%q, got %q", "/redir3/server", got)
		}
		if got := redirectedRec.Header().Get(response.ClientRedirectHeader); got != "/redir3/client" {
			t.Fatalf("expected %s to be present as lower-priority signal, got %q", response.ClientRedirectHeader, got)
		}

		clientOnlyRec := httptest.NewRecorder()
		clientOnlyReq := httptest.NewRequest(http.MethodGet, "/redir4/source?vorma_json="+wireBuildID, nil)
		clientOnlyReq.Header.Set(response.ClientAcceptsRedirectHeader, "1")
		r.ServeHTTP(clientOnlyRec, clientOnlyReq)
		if clientOnlyRec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, clientOnlyRec.Code, clientOnlyRec.Body.String())
		}
		if got := clientOnlyRec.Header().Get("Location"); got != "" {
			t.Fatalf("expected no Location when client redirect is used, got %q", got)
		}
		if got := clientOnlyRec.Header().Get(response.ClientRedirectHeader); got != "/redir4/client" {
			t.Fatalf("expected client redirect header %q, got %q", "/redir4/client", got)
		}
	})

	t.Run("WRC-REDIR-005_WIRE-REDIR-003_stale_reload_target_supports_hard_reload_query_compatibility", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths([]wirePathDef{
				{Pattern: "/redir5/page", SrcPath: "src/redir5/page.tsx", OutPath: "routes/redir5_page.js", ExportKey: "Page"},
			}, nil),
			nil,
		)
		vorma.NewLoader(
			app,
			"/redir5/page",
			func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		staleRec := httptest.NewRecorder()
		staleReq := httptest.NewRequest(http.MethodGet, "/redir5/page?keep=1&vorma_json=old", nil)
		r.ServeHTTP(staleRec, staleReq)
		if staleRec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, staleRec.Code, staleRec.Body.String())
		}
		reloadURL := staleRec.Header().Get("X-Vorma-Reload")
		if reloadURL != "/redir5/page?keep=1" {
			t.Fatalf("expected stale reload URL %q, got %q", "/redir5/page?keep=1", reloadURL)
		}

		hardReloadURL := reloadURL + "&vorma_reload=" + wireBuildID
		finalRec := httptest.NewRecorder()
		finalReq := httptest.NewRequest(http.MethodGet, hardReloadURL, nil)
		r.ServeHTTP(finalRec, finalReq)
		if finalRec.Code != http.StatusOK {
			t.Fatalf("expected status %d on hard-reload URL, got %d body=%q", http.StatusOK, finalRec.Code, finalRec.Body.String())
		}
		if !strings.HasPrefix(finalRec.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("expected hard-reload URL to remain a normal document request, got content-type=%q", finalRec.Header().Get("Content-Type"))
		}
	})
}
