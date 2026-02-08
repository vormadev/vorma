package backend_test

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

func TestBackendDevReloadConformance(t *testing.T) {
	t.Run("BRC-DEV-001_BR-DEV-001_reload_routes_endpoint_reloads_stage_one_metadata", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "development")

		defs := []testPathDef{
			{Pattern: "/dev1/page", SrcPath: "src/dev1/page.tsx", OutPath: "routes/dev1_page.js", ExportKey: "Page"},
		}
		distDir := makeDevDistDirWithPaths(t, defs)
		cfg := makeWaveConfigJSONWithCoreMutator(t, func(core map[string]any) {
			core["DistDir"] = distDir
		})

		app := makeApp(t, cfg, nil)
		vorma.NewLoader(
			app,
			"/dev1/page",
			func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		before := httptest.NewRecorder()
		beforeReq := httptest.NewRequest(http.MethodGet, "/dev1/page?vorma_json="+testBuildID, nil)
		r.ServeHTTP(before, beforeReq)
		if before.Code != http.StatusOK {
			t.Fatalf("expected status %d before reload, got %d; body=%q", http.StatusOK, before.Code, before.Body.String())
		}
		if got := before.Header().Get(vormaruntime.VormaBuildIDHeaderKey); got != testBuildID {
			t.Fatalf("expected pre-reload build id %q, got %q", testBuildID, got)
		}

		const reloadedBuildID = "b456"
		writeDevStageOnePathsFile(t, distDir, reloadedBuildID, defs)

		reloadRes := httptest.NewRecorder()
		reloadReq := httptest.NewRequest(http.MethodGet, vormaruntime.DevReloadRoutesPath, nil)
		r.ServeHTTP(reloadRes, reloadReq)
		if reloadRes.Code != http.StatusOK || strings.TrimSpace(reloadRes.Body.String()) != "ok" {
			t.Fatalf("expected reload-routes success (200 + ok), got status=%d body=%q", reloadRes.Code, reloadRes.Body.String())
		}

		after := httptest.NewRecorder()
		afterReq := httptest.NewRequest(http.MethodGet, "/dev1/page?vorma_json="+reloadedBuildID, nil)
		r.ServeHTTP(after, afterReq)
		if after.Code != http.StatusOK {
			t.Fatalf("expected status %d after reload, got %d; body=%q", http.StatusOK, after.Code, after.Body.String())
		}
		if got := after.Header().Get(vormaruntime.VormaBuildIDHeaderKey); got != reloadedBuildID {
			t.Fatalf("expected post-reload build id %q, got %q", reloadedBuildID, got)
		}
	})

	t.Run("BRC-DEV-002_BR-DEV-002_reload_template_endpoint_reloads_template_from_disk", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "development")

		defs := []testPathDef{
			{Pattern: "/dev2/page", SrcPath: "src/dev2/page.tsx", OutPath: "routes/dev2_page.js", ExportKey: "Page"},
		}
		distDir := makeDevDistDirWithPaths(t, defs)
		privateDir := filepath.Join(distDir, "static", "assets", "private")
		templatePath := filepath.Join(privateDir, testTemplatePath)

		if err := os.WriteFile(templatePath, []byte(`<html><body>before-template {{.VormaRootID}}</body></html>`), 0o644); err != nil {
			t.Fatalf("failed to write initial template file: %v", err)
		}

		cfg := makeWaveConfigJSONWithCoreMutator(t, func(core map[string]any) {
			core["DistDir"] = distDir
			staticDirs, ok := core["StaticAssetDirs"].(map[string]any)
			if !ok {
				t.Fatalf("expected Core.StaticAssetDirs map, got %#v", core["StaticAssetDirs"])
			}
			staticDirs["Private"] = privateDir
		})

		app := makeApp(t, cfg, nil)
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
			t.Fatalf("expected status %d before template reload, got %d; body=%q", http.StatusOK, before.Code, before.Body.String())
		}
		if !strings.Contains(before.Body.String(), "before-template") {
			t.Fatalf("expected initial template output before reload, body=%q", before.Body.String())
		}

		if err := os.WriteFile(templatePath, []byte(`<html><body>after-template {{.VormaRootID}}</body></html>`), 0o644); err != nil {
			t.Fatalf("failed to write updated template file: %v", err)
		}

		reloadRes := httptest.NewRecorder()
		reloadReq := httptest.NewRequest(http.MethodGet, vormaruntime.DevReloadTemplatePath, nil)
		r.ServeHTTP(reloadRes, reloadReq)
		if reloadRes.Code != http.StatusOK || strings.TrimSpace(reloadRes.Body.String()) != "ok" {
			t.Fatalf("expected reload-template success (200 + ok), got status=%d body=%q", reloadRes.Code, reloadRes.Body.String())
		}

		after := httptest.NewRecorder()
		afterReq := httptest.NewRequest(http.MethodGet, "/dev2/page", nil)
		r.ServeHTTP(after, afterReq)
		if after.Code != http.StatusOK {
			t.Fatalf("expected status %d after template reload, got %d; body=%q", http.StatusOK, after.Code, after.Body.String())
		}
		if !strings.Contains(after.Body.String(), "after-template") {
			t.Fatalf("expected updated template output after reload, body=%q", after.Body.String())
		}
	})

	t.Run("BRC-DEV-003_BR-DEV-003_reload_endpoint_failure_returns_500_with_error_text", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "development")

		defs := []testPathDef{
			{Pattern: "/dev3/page", SrcPath: "src/dev3/page.tsx", OutPath: "routes/dev3_page.js", ExportKey: "Page"},
		}
		distDir := makeDevDistDirWithPaths(t, defs)
		cfg := makeWaveConfigJSONWithCoreMutator(t, func(core map[string]any) {
			core["DistDir"] = distDir
		})

		app := makeApp(t, cfg, nil)
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
			t.Fatalf("failed to remove stage-one paths file for failure scenario: %v", err)
		}

		reloadRes := httptest.NewRecorder()
		reloadReq := httptest.NewRequest(http.MethodGet, vormaruntime.DevReloadRoutesPath, nil)
		r.ServeHTTP(reloadRes, reloadReq)

		if reloadRes.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d on reload failure, got %d; body=%q", http.StatusInternalServerError, reloadRes.Code, reloadRes.Body.String())
		}
		if !strings.Contains(reloadRes.Body.String(), vormaruntime.VormaPathsStageOneJSONFileName) {
			t.Fatalf("expected failure body to include missing source detail, got %q", reloadRes.Body.String())
		}
	})
}

func writeDevStageOnePathsFile(t *testing.T, distDir string, buildID string, defs []testPathDef) {
	t.Helper()

	paths := make(map[string]any, len(defs))
	for _, def := range defs {
		exportKey := def.ExportKey
		if exportKey == "" {
			exportKey = "default"
		}
		paths[def.Pattern] = map[string]any{
			"originalPattern": def.Pattern,
			"srcPath":         def.SrcPath,
			"exportKey":       exportKey,
			"errorExportKey":  def.ErrorExportKey,
			"outPath":         def.OutPath,
			"deps":            []string{},
		}
	}

	payload := map[string]any{
		"stage":             "1",
		"buildID":           buildID,
		"clientEntrySrc":    "frontend/src/vorma.entry.tsx",
		"paths":             paths,
		"routeManifestFile": "vorma_out/route_manifest.json",
		"clientEntryOut":    "vorma_out/client.js",
		"clientEntryDeps":   []string{},
		"depToCSSBundleMap": map[string][]string{},
	}

	stageOnePath := filepath.Join(
		distDir,
		"static",
		"assets",
		"private",
		vormaruntime.VormaOutDirname,
		vormaruntime.VormaPathsStageOneJSONFileName,
	)
	if err := os.WriteFile(stageOnePath, mustMarshalJSON(nil, payload), 0o644); err != nil {
		t.Fatalf("failed to write stage-one paths file: %v", err)
	}
}
