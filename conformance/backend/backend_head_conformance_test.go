package backend_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/headels"
)

func TestBackendHeadConformance(t *testing.T) {
	t.Run("BRC-HEAD-001_BR-HEAD-001_default_and_loader_head_elements_are_merged_with_dedupe", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		cfg := makeWaveConfigJSON(t, nil)
		staticFS := makeStaticFSWithPaths([]testPathDef{
			{Pattern: "/head1/page", SrcPath: "src/head1/page.tsx", OutPath: "routes/head1_page.js", ExportKey: "Page"},
		})

		app := vorma.NewVormaApp(vorma.VormaAppConfig{
			Wave: makeWave(cfg, staticFS),
			GetDefaultHeadEls: func(r *http.Request, app *vorma.Vorma, h *headels.HeadEls) error {
				h.Title("Default Title")
				h.Description("Default Description")
				h.MetaPropertyContent("og:site_name", "Vorma")
				return nil
			},
		})

		vorma.NewLoader(
			app,
			"/head1/page",
			func(rd *vorma.LoaderReqData) (string, error) {
				h := rd.ResponseProxy().GetHeadEls()
				h.Title("Loader Title")
				h.Description("Loader Description")
				return "ok", nil
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)

		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/head1/page", nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		body := rec.Body.String()
		if !strings.Contains(body, "<title>Loader Title</title>") {
			t.Fatalf("expected loader title in final head output, body=%q", body)
		}
		if strings.Contains(body, "<title>Default Title</title>") {
			t.Fatalf("expected default title to be deduped out, body=%q", body)
		}
		if !strings.Contains(body, "Loader Description") {
			t.Fatalf("expected loader description in final head output, body=%q", body)
		}
		if strings.Contains(body, "Default Description") {
			t.Fatalf("expected default description meta to be deduped out, body=%q", body)
		}
		if !strings.Contains(body, "og:site_name") || !strings.Contains(body, "Vorma") {
			t.Fatalf("expected default non-deduped head element to remain, body=%q", body)
		}
	})

	t.Run("BRC-HEAD-002_BR-HEAD-002_html_includes_wave_critical_css_and_stylesheet_link", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		cfg := makeWaveConfigJSONWithCoreMutator(t, func(core map[string]any) {
			core["CSSEntryFiles"] = map[string]any{
				"Critical":    "frontend/src/critical.css",
				"NonCritical": "frontend/src/non_critical.css",
			}
		})
		staticFS := makeStaticFSWithPaths([]testPathDef{
			{Pattern: "/head2/page", SrcPath: "src/head2/page.tsx", OutPath: "routes/head2_page.js", ExportKey: "Page"},
		})
		staticFS["internal/critical.css"] = &fstest.MapFile{
			Data: []byte("body{background:#fff;}"),
		}
		staticFS["internal/normal_css_file_ref.txt"] = &fstest.MapFile{
			Data: []byte("vorma_out_normal.css"),
		}

		app := makeApp(t, cfg, staticFS)
		vorma.NewLoader(
			app,
			"/head2/page",
			func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/head2/page", nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		body := rec.Body.String()
		if !strings.Contains(body, `id="wave-critical-css"`) {
			t.Fatalf("expected critical css style element in head output, body=%q", body)
		}
		if !strings.Contains(body, `id="wave-normal-css"`) {
			t.Fatalf("expected normal stylesheet link element in head output, body=%q", body)
		}
	})
}
