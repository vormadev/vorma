package backend_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/vormaruntime"
)

func TestBackendHTMLConformance(t *testing.T) {
	t.Run("BRC-HTML-001_BR-HTML-001_non_json_loader_request_returns_html", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFSWithPaths([]testPathDef{
			{Pattern: "/html1/page", SrcPath: "src/html1/page.tsx", OutPath: "routes/html1_page.js", ExportKey: "Page"},
		}))
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
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
			t.Fatalf("expected text/html content type, got %q", got)
		}
	})

	t.Run("BRC-HTML-002_BR-HTML-002_template_receives_required_vorma_keys", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		staticFS := makeStaticFSWithPaths([]testPathDef{
			{Pattern: "/html2/page", SrcPath: "src/html2/page.tsx", OutPath: "routes/html2_page.js", ExportKey: "Page"},
		})
		staticFS[path.Join("assets/private", testTemplatePath)] = &fstest.MapFile{
			Data: []byte(`H={{.VormaHeadEls}};S={{.VormaSSRScript}};SH={{.VormaSSRScriptSha256Hash}};R={{.VormaRootID}};B={{.VormaBodyScripts}}`),
		}

		app := makeApp(t, makeWaveConfigJSON(t, nil), staticFS)
		vorma.NewLoader(
			app,
			"/html2/page",
			func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/html2/page", nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		body := rec.Body.String()
		requiredSubstrings := []string{"H=", "S=", "SH=", "R=vorma-root", "B="}
		for _, needle := range requiredSubstrings {
			if !strings.Contains(body, needle) {
				t.Fatalf("expected body to contain %q, got %q", needle, body)
			}
		}
		if strings.Contains(body, "<no value>") {
			t.Fatalf("expected required template keys to be present, got %q", body)
		}
		if strings.Contains(body, "SH=;") {
			t.Fatalf("expected non-empty VormaSSRScriptSha256Hash, got %q", body)
		}
	})

	t.Run("BRC-HTML-003_BR-HTML-003_prod_body_scripts_use_public_prefix_and_client_entry_out", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		cfg := makeWaveConfigJSONWithPublicPathPrefix(t, "/pub")
		app := makeApp(t, cfg, makeStaticFSWithPaths([]testPathDef{
			{Pattern: "/html3/page", SrcPath: "src/html3/page.tsx", OutPath: "routes/html3_page.js", ExportKey: "Page"},
		}))
		vorma.NewLoader(
			app,
			"/html3/page",
			func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/html3/page", nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `<script type="module" src="/pub/vorma_out/client.js"></script>`) {
			t.Fatalf("expected prod body scripts to target prefixed client entry output, body=%q", rec.Body.String())
		}
	})

	t.Run("BRC-HTML-004_BR-HTML-004_dev_body_scripts_include_vite_and_wave_refresh", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "development")
		t.Setenv("__VITE_PORT", "5173")
		t.Setenv("WAVE_REFRESH_SERVER_PORT", "11000")

		distDir := makeDevDistDirWithPaths(t, []testPathDef{
			{Pattern: "/html4/page", SrcPath: "src/html4/page.tsx", OutPath: "routes/html4_page.js", ExportKey: "Page"},
		})
		cfg := makeWaveConfigJSONWithCoreMutator(t, func(core map[string]any) {
			core["DistDir"] = distDir
		})

		app := makeApp(t, cfg, nil)
		vorma.NewLoader(
			app,
			"/html4/page",
			func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/html4/page", nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		body := rec.Body.String()
		requiredSubstrings := []string{
			`http://localhost:5173/@vite/client`,
			`http://localhost:5173/frontend/src/vorma.entry.tsx`,
			`ws://localhost:11000/events`,
		}
		for _, needle := range requiredSubstrings {
			if !strings.Contains(body, needle) {
				t.Fatalf("expected dev HTML body to contain %q, got %q", needle, body)
			}
		}
	})

	t.Run("BRC-HTML-005_BR-HTML-005_default_cache_control_is_applied_when_not_overridden", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFSWithPaths([]testPathDef{
			{Pattern: "/html5/page", SrcPath: "src/html5/page.tsx", OutPath: "routes/html5_page.js", ExportKey: "Page"},
		}))
		vorma.NewLoader(
			app,
			"/html5/page",
			func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/html5/page", nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		const expectedCacheControl = "private, max-age=0, must-revalidate, no-cache"
		if got := rec.Header().Get("Cache-Control"); got != expectedCacheControl {
			t.Fatalf("expected Cache-Control %q, got %q", expectedCacheControl, got)
		}
	})

	t.Run("BRC-HTML-006_BR-HTML-005_explicit_cache_control_override_is_preserved", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFSWithPaths([]testPathDef{
			{Pattern: "/html6/page", SrcPath: "src/html6/page.tsx", OutPath: "routes/html6_page.js", ExportKey: "Page"},
		}))
		vorma.NewLoader(
			app,
			"/html6/page",
			func(rd *vorma.LoaderReqData) (string, error) {
				rd.ResponseProxy().SetHeader("Cache-Control", "public, max-age=60")
				return "ok", nil
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/html6/page", nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Cache-Control"); got != "public, max-age=60" {
			t.Fatalf("expected Cache-Control override to be preserved, got %q", got)
		}
	})
}

func makeWaveConfigJSONWithPublicPathPrefix(t *testing.T, publicPathPrefix string) []byte {
	t.Helper()
	return makeWaveConfigJSONWithCoreMutator(t, func(core map[string]any) {
		core["PublicPathPrefix"] = publicPathPrefix
	})
}

func makeWaveConfigJSONWithCoreMutator(t *testing.T, mutateCore func(core map[string]any)) []byte {
	t.Helper()

	raw := makeWaveConfigJSON(t, nil)
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("failed to decode baseline wave config json: %v", err)
	}

	core, ok := cfg["Core"].(map[string]any)
	if !ok {
		t.Fatalf("expected Core config section, got %#v", cfg["Core"])
	}
	if mutateCore != nil {
		mutateCore(core)
	}

	return mustMarshalJSON(t, cfg)
}

func makeDevDistDirWithPaths(t *testing.T, defs []testPathDef) string {
	t.Helper()

	distDir := filepath.Join(t.TempDir(), "dist")
	privateDir := filepath.Join(distDir, "static", "assets", "private")
	if err := os.MkdirAll(filepath.Join(privateDir, vormaruntime.VormaOutDirname), 0o755); err != nil {
		t.Fatalf("failed to create dev dist private dir: %v", err)
	}

	templateBytes := []byte(`<!doctype html><html><head>{{.VormaHeadEls}}</head><body><div id="{{.VormaRootID}}"></div>{{.VormaBodyScripts}}{{.VormaSSRScript}}</body></html>`)
	if err := os.WriteFile(filepath.Join(privateDir, testTemplatePath), templateBytes, 0o644); err != nil {
		t.Fatalf("failed to write dev template file: %v", err)
	}

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

	stagePayload := map[string]any{
		"buildID":           testBuildID,
		"clientEntrySrc":    "frontend/src/vorma.entry.tsx",
		"paths":             paths,
		"routeManifestFile": "vorma_out/route_manifest.json",
		"clientEntryOut":    "vorma_out/client.js",
		"clientEntryDeps":   []string{},
		"depToCSSBundleMap": map[string][]string{},
	}

	stageOne := make(map[string]any, len(stagePayload)+1)
	stageTwo := make(map[string]any, len(stagePayload)+1)
	for k, v := range stagePayload {
		stageOne[k] = v
		stageTwo[k] = v
	}
	stageOne["stage"] = "1"
	stageTwo["stage"] = "2"

	stageOnePath := filepath.Join(privateDir, vormaruntime.VormaOutDirname, vormaruntime.VormaPathsStageOneJSONFileName)
	stageTwoPath := filepath.Join(privateDir, vormaruntime.VormaOutDirname, vormaruntime.VormaPathsStageTwoJSONFileName)
	if err := os.WriteFile(stageOnePath, mustMarshalJSON(nil, stageOne), 0o644); err != nil {
		t.Fatalf("failed to write dev stage1 paths file: %v", err)
	}
	if err := os.WriteFile(stageTwoPath, mustMarshalJSON(nil, stageTwo), 0o644); err != nil {
		t.Fatalf("failed to write dev stage2 paths file: %v", err)
	}

	return distDir
}
