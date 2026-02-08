package wire_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/vormaruntime"
)

func TestWireDevReloadConformance(t *testing.T) {
	t.Run("WRC-DEV-001_WIRE-DEV-001_reload_routes_endpoint_returns_200_ok_and_applies_new_build_id", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "development")

		defs := []wirePathDef{
			{Pattern: "/dev1/page", SrcPath: "src/dev1/page.tsx", OutPath: "routes/dev1_page.js", ExportKey: "Page"},
		}
		distDir := makeWireDevDistDirWithPaths(t, defs, nil)
		cfg := makeWireConfigJSON(t, func(core map[string]any) {
			core["DistDir"] = distDir
		}, nil)

		app := makeWireApp(t, cfg, nil, nil)
		vorma.NewLoader(
			app,
			"/dev1/page",
			func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		before := httptest.NewRecorder()
		beforeReq := httptest.NewRequest(http.MethodGet, "/dev1/page?vorma_json="+wireBuildID, nil)
		r.ServeHTTP(before, beforeReq)
		if before.Code != http.StatusOK {
			t.Fatalf("expected status %d before reload, got %d body=%q", http.StatusOK, before.Code, before.Body.String())
		}
		if got := before.Header().Get(vormaruntime.VormaBuildIDHeaderKey); got != wireBuildID {
			t.Fatalf("expected pre-reload build id %q, got %q", wireBuildID, got)
		}

		const reloadedBuildID = "b999"
		writeWireDevStagePathsFile(t, distDir, "1", reloadedBuildID, defs, "vorma_out_vorma_internal_route_manifest_test.json")

		reloadRes := httptest.NewRecorder()
		reloadReq := httptest.NewRequest(http.MethodGet, vormaruntime.DevReloadRoutesPath, nil)
		r.ServeHTTP(reloadRes, reloadReq)
		if reloadRes.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, reloadRes.Code, reloadRes.Body.String())
		}
		if strings.TrimSpace(reloadRes.Body.String()) != "ok" {
			t.Fatalf("expected reload-routes body %q, got %q", "ok", reloadRes.Body.String())
		}

		after := httptest.NewRecorder()
		afterReq := httptest.NewRequest(http.MethodGet, "/dev1/page?vorma_json="+reloadedBuildID, nil)
		r.ServeHTTP(after, afterReq)
		if after.Code != http.StatusOK {
			t.Fatalf("expected status %d after reload, got %d body=%q", http.StatusOK, after.Code, after.Body.String())
		}
		if got := after.Header().Get(vormaruntime.VormaBuildIDHeaderKey); got != reloadedBuildID {
			t.Fatalf("expected post-reload build id %q, got %q", reloadedBuildID, got)
		}
	})

	t.Run("WRC-DEV-002_WIRE-DEV-002_reload_template_endpoint_returns_200_ok_and_uses_reloaded_template", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "development")

		defs := []wirePathDef{
			{Pattern: "/dev2/page", SrcPath: "src/dev2/page.tsx", OutPath: "routes/dev2_page.js", ExportKey: "Page"},
		}
		distDir := makeWireDevDistDirWithPaths(t, defs, nil)
		privateDir := filepath.Join(distDir, "static", "assets", "private")
		templatePath := filepath.Join(privateDir, wireTemplatePath)
		if err := os.WriteFile(templatePath, []byte(`<html><body>before {{.VormaRootID}}</body></html>`), 0o644); err != nil {
			t.Fatalf("write initial template: %v", err)
		}

		cfg := makeWireConfigJSON(t, func(core map[string]any) {
			core["DistDir"] = distDir
			staticDirs, ok := core["StaticAssetDirs"].(map[string]any)
			if !ok {
				t.Fatalf("expected Core.StaticAssetDirs map, got %#v", core["StaticAssetDirs"])
			}
			staticDirs["Private"] = privateDir
		}, nil)

		app := makeWireApp(t, cfg, nil, nil)
		vorma.NewLoader(
			app,
			"/dev2/page",
			func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		before := httptest.NewRecorder()
		beforeReq := httptest.NewRequest(http.MethodGet, "/dev2/page", nil)
		r.ServeHTTP(before, beforeReq)
		if before.Code != http.StatusOK {
			t.Fatalf("expected status %d before template reload, got %d body=%q", http.StatusOK, before.Code, before.Body.String())
		}
		if !strings.Contains(before.Body.String(), "before") {
			t.Fatalf("expected HTML from initial template, got body=%q", before.Body.String())
		}

		if err := os.WriteFile(templatePath, []byte(`<html><body>after {{.VormaRootID}}</body></html>`), 0o644); err != nil {
			t.Fatalf("write updated template: %v", err)
		}

		reloadRes := httptest.NewRecorder()
		reloadReq := httptest.NewRequest(http.MethodGet, vormaruntime.DevReloadTemplatePath, nil)
		r.ServeHTTP(reloadRes, reloadReq)
		if reloadRes.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, reloadRes.Code, reloadRes.Body.String())
		}
		if strings.TrimSpace(reloadRes.Body.String()) != "ok" {
			t.Fatalf("expected reload-template body %q, got %q", "ok", reloadRes.Body.String())
		}

		after := httptest.NewRecorder()
		afterReq := httptest.NewRequest(http.MethodGet, "/dev2/page", nil)
		r.ServeHTTP(after, afterReq)
		if after.Code != http.StatusOK {
			t.Fatalf("expected status %d after template reload, got %d body=%q", http.StatusOK, after.Code, after.Body.String())
		}
		if !strings.Contains(after.Body.String(), "after") {
			t.Fatalf("expected HTML from reloaded template, got body=%q", after.Body.String())
		}
	})

	t.Run("WRC-DEV-003_WIRE-DEV-003_reload_endpoint_failures_return_500_with_error_text", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "development")

		defs := []wirePathDef{
			{Pattern: "/dev3/page", SrcPath: "src/dev3/page.tsx", OutPath: "routes/dev3_page.js", ExportKey: "Page"},
		}
		distDir := makeWireDevDistDirWithPaths(t, defs, nil)
		cfg := makeWireConfigJSON(t, func(core map[string]any) {
			core["DistDir"] = distDir
		}, nil)

		app := makeWireApp(t, cfg, nil, nil)
		r := app.InitWithDefaultRouter()

		stageOnePath := filepath.Join(
			distDir,
			"static",
			"assets",
			"private",
			vormaruntime.VormaOutDirname,
			vormaruntime.VormaPathsStageOneJSONFileName,
		)
		if err := os.Remove(stageOnePath); err != nil {
			t.Fatalf("remove stage-one paths file: %v", err)
		}

		reloadRes := httptest.NewRecorder()
		reloadReq := httptest.NewRequest(http.MethodGet, vormaruntime.DevReloadRoutesPath, nil)
		r.ServeHTTP(reloadRes, reloadReq)
		if reloadRes.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusInternalServerError, reloadRes.Code, reloadRes.Body.String())
		}
		if !strings.Contains(reloadRes.Body.String(), vormaruntime.VormaPathsStageOneJSONFileName) {
			t.Fatalf("expected failure body to mention missing stage-one file, got %q", reloadRes.Body.String())
		}
	})
}
