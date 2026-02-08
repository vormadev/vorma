package backend_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vormadev/vorma"
)

func TestBackendResponseProxyConformance(t *testing.T) {
	t.Run("BRC-PROXY-001_BR-PROXY-001_merged_proxy_mutations_affect_final_response", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFSWithPaths([]testPathDef{
			{Pattern: "", SrcPath: "src/proxy1/root.tsx", OutPath: "routes/proxy1_root.js", ExportKey: "Root"},
			{Pattern: "/proxy1/page", SrcPath: "src/proxy1/page.tsx", OutPath: "routes/proxy1_page.js", ExportKey: "Page"},
		}))
		vorma.NewLoader(
			app,
			"",
			func(rd *vorma.LoaderReqData) (string, error) {
				rd.ResponseProxy().SetHeader("X-Proxy-Root", "root")
				return "root-ok", nil
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewLoader(
			app,
			"/proxy1/page",
			func(rd *vorma.LoaderReqData) (string, error) {
				rd.ResponseProxy().SetHeader("X-Proxy-Child", "child")
				return "page-ok", nil
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/proxy1/page?vorma_json="+testBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("X-Proxy-Root"); got != "root" {
			t.Fatalf("expected X-Proxy-Root=%q, got %q", "root", got)
		}
		if got := rec.Header().Get("X-Proxy-Child"); got != "child" {
			t.Fatalf("expected X-Proxy-Child=%q, got %q", "child", got)
		}
	})

	t.Run("BRC-PROXY-002_BR-PROXY-002_redirect_short_circuits_normal_payload", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFSWithPaths([]testPathDef{
			{Pattern: "/proxy2/source", SrcPath: "src/proxy2/source.tsx", OutPath: "routes/proxy2_source.js", ExportKey: "Source"},
		}))
		vorma.NewLoader(
			app,
			"/proxy2/source",
			func(rd *vorma.LoaderReqData) (string, error) {
				_, err := rd.ResponseProxy().Redirect(rd.Request(), "/proxy2/target", http.StatusFound)
				return "ignored-after-redirect", err
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/proxy2/source?vorma_json="+testBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusFound {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusFound, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Location"); got != "/proxy2/target" {
			t.Fatalf("expected Location=%q, got %q", "/proxy2/target", got)
		}
		if strings.Contains(rec.Body.String(), "matchedPatterns") || strings.Contains(rec.Body.String(), "loadersData") {
			t.Fatalf("expected redirect response to skip normal route-data payload, got body=%q", rec.Body.String())
		}
	})

	t.Run("BRC-PROXY-003_BR-PROXY-003_error_short_circuits_normal_payload", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFSWithPaths([]testPathDef{
			{Pattern: "/proxy3/fail", SrcPath: "src/proxy3/fail.tsx", OutPath: "routes/proxy3_fail.js", ExportKey: "Fail"},
		}))
		vorma.NewLoader(
			app,
			"/proxy3/fail",
			func(rd *vorma.LoaderReqData) (string, error) {
				rd.ResponseProxy().SetStatus(http.StatusTeapot, "proxy error")
				return "ignored-after-error", nil
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/proxy3/fail?vorma_json="+testBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusTeapot {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusTeapot, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "proxy error") {
			t.Fatalf("expected proxy error text in body, got %q", rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "matchedPatterns") || strings.Contains(rec.Body.String(), "loadersData") {
			t.Fatalf("expected error short-circuit to skip normal route-data payload, got body=%q", rec.Body.String())
		}
	})
}
