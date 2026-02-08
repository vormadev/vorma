package backend_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/vormaruntime"
	"github.com/vormadev/vorma/wave"
)

const (
	testBuildID       = "b123"
	testTemplatePath  = "index.html"
	testLoaderPattern = "/hello"
	testActionPattern = "/ping"
)

func TestBackendInitConformance(t *testing.T) {
	t.Run("BRC-INIT-001_BR-INIT-001_wave_is_required", func(t *testing.T) {
		msg := mustPanic(t, func() {
			_ = vorma.NewVormaApp(vorma.VormaAppConfig{Wave: nil})
		})
		if !strings.Contains(msg, "Wave instance is required") {
			t.Fatalf("expected panic about nil Wave, got: %q", msg)
		}
	})

	t.Run("BRC-INIT-002_BR-INIT-002_required_config_keys", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		requiredKeys := []string{
			"MainBuildEntry",
			"UIVariant",
			"HTMLTemplateLocation",
			"ClientEntry",
			"ClientRouteDefsFile",
			"TSGenOutDir",
		}

		for _, key := range requiredKeys {
			key := key
			t.Run(key, func(t *testing.T) {
				rawCfg := makeWaveConfigJSON(t, func(vormaCfg map[string]any) {
					delete(vormaCfg, key)
				})
				w := makeWave(rawCfg, fstest.MapFS{})
				msg := mustPanic(t, func() {
					_ = vorma.NewVormaApp(vorma.VormaAppConfig{Wave: w})
				})
				expected := "config: Vorma." + key + " is required"
				if !strings.Contains(msg, expected) {
					t.Fatalf("expected panic containing %q, got: %q", expected, msg)
				}
			})
		}
	})

	t.Run("BRC-INIT-003_BR-INIT-003_defaults_apply", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), fstest.MapFS{})

		if got := app.Config.BuildtimePublicURLFuncName; got != "waveBuildtimeURL" {
			t.Fatalf("expected BuildtimePublicURLFuncName default %q, got %q", "waveBuildtimeURL", got)
		}

		if got := app.LoadersRouter().GetExplicitIndexSegment(); got != "_index" {
			t.Fatalf("expected loaders explicit index segment %q, got %q", "_index", got)
		}

		if got := app.ActionsRouter().MountRoot(); got != "/api/" {
			t.Fatalf("expected actions mount root %q, got %q", "/api/", got)
		}
		if got := app.Actions().HandlerMountPattern(); got != "/api/*" {
			t.Fatalf("expected actions handler mount pattern %q, got %q", "/api/*", got)
		}

		requiredMethods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
		supported := app.Actions().SupportedMethods()
		for _, m := range requiredMethods {
			if !supported[m] {
				t.Fatalf("expected default supported methods to include %q", m)
			}
		}
	})

	t.Run("BRC-INIT-004_BR-INIT-004_missing_or_invalid_artifacts_fail_init", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		testCases := []struct {
			name                string
			staticFS            fstest.MapFS
			expectedPanicSubstr string
		}{
			{
				name:                "missing_stage2_paths_file",
				staticFS:            makeStaticFS(false, true),
				expectedPanicSubstr: vormaruntime.VormaPathsStageTwoJSONFileName,
			},
			{
				name:                "missing_template_file",
				staticFS:            makeStaticFS(true, false),
				expectedPanicSubstr: "error parsing root template",
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				app := makeApp(t, makeWaveConfigJSON(t, nil), tc.staticFS)
				msg := mustPanic(t, app.Init)

				if !strings.Contains(msg, "error initializing Vorma") {
					t.Fatalf("expected init panic wrapper, got: %q", msg)
				}
				if !strings.Contains(msg, tc.expectedPanicSubstr) {
					t.Fatalf("expected panic containing %q, got: %q", tc.expectedPanicSubstr, msg)
				}
			})
		}
	})

	t.Run("BRC-INIT-005_BR-INIT-005_init_with_default_router_mounts_loaders_and_actions", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFS(true, true))

		vorma.NewLoader(
			app,
			testLoaderPattern,
			func(*vorma.LoaderReqData) (string, error) { return "loader-ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewAction[vorma.None, string](
			app,
			http.MethodPost,
			testActionPattern,
			func(*vorma.ActionReqData[vorma.None]) (string, error) { return "action-ok", nil },
			func(rd *vorma.ActionReqData[vorma.None]) *vorma.ActionReqData[vorma.None] { return rd },
		)

		r := app.InitWithDefaultRouter()

		loaderRes := httptest.NewRecorder()
		loaderReq := httptest.NewRequest(http.MethodGet, testLoaderPattern+"?vorma_json="+testBuildID, nil)
		r.ServeHTTP(loaderRes, loaderReq)

		if loaderRes.Code != http.StatusOK {
			t.Fatalf("expected loader status %d, got %d; body=%s", http.StatusOK, loaderRes.Code, loaderRes.Body.String())
		}
		if got := loaderRes.Header().Get(vormaruntime.VormaBuildIDHeaderKey); got != testBuildID {
			t.Fatalf("expected %s=%q, got %q", vormaruntime.VormaBuildIDHeaderKey, testBuildID, got)
		}

		var loaderPayload map[string]any
		if err := json.Unmarshal(loaderRes.Body.Bytes(), &loaderPayload); err != nil {
			t.Fatalf("expected JSON loader payload, got err=%v body=%q", err, loaderRes.Body.String())
		}

		loadersData, ok := loaderPayload["loadersData"].([]any)
		if !ok || len(loadersData) == 0 {
			t.Fatalf("expected non-empty loadersData, got: %#v", loaderPayload["loadersData"])
		}
		if got, _ := loadersData[len(loadersData)-1].(string); got != "loader-ok" {
			t.Fatalf("expected last loadersData entry %q, got %#v", "loader-ok", loadersData[len(loadersData)-1])
		}

		actionRes := httptest.NewRecorder()
		actionReq := httptest.NewRequest(http.MethodPost, "/api"+testActionPattern, nil)
		r.ServeHTTP(actionRes, actionReq)

		if actionRes.Code != http.StatusOK {
			t.Fatalf("expected action status %d, got %d; body=%s", http.StatusOK, actionRes.Code, actionRes.Body.String())
		}
		if got := actionRes.Header().Get(vormaruntime.VormaBuildIDHeaderKey); got != testBuildID {
			t.Fatalf("expected %s=%q, got %q", vormaruntime.VormaBuildIDHeaderKey, testBuildID, got)
		}

		var actionPayload string
		if err := json.Unmarshal(actionRes.Body.Bytes(), &actionPayload); err != nil {
			t.Fatalf("expected JSON action payload, got err=%v body=%q", err, actionRes.Body.String())
		}
		if actionPayload != "action-ok" {
			t.Fatalf("expected action payload %q, got %q", "action-ok", actionPayload)
		}
	})
}

func makeApp(t *testing.T, rawCfg []byte, staticFS fstest.MapFS) *vorma.Vorma {
	t.Helper()
	w := makeWave(rawCfg, staticFS)
	return vorma.NewVormaApp(vorma.VormaAppConfig{Wave: w})
}

func makeWave(rawCfg []byte, staticFS fstest.MapFS) *wave.Wave {
	return wave.New(wave.Config{
		WaveConfigJSON: rawCfg,
		DistStaticFS:   staticFS,
	})
}

func makeWaveConfigJSON(t *testing.T, mutateVormaCfg func(map[string]any)) []byte {
	t.Helper()

	vormaCfg := map[string]any{
		"MainBuildEntry":       "cmd/app/main.go",
		"UIVariant":            "react",
		"HTMLTemplateLocation": testTemplatePath,
		"ClientEntry":          "frontend/src/vorma.entry.tsx",
		"ClientRouteDefsFile":  "frontend/src/vorma.routes.ts",
		"TSGenOutDir":          "frontend/src/vorma.gen",
	}
	if mutateVormaCfg != nil {
		mutateVormaCfg(vormaCfg)
	}

	cfg := map[string]any{
		"Core": map[string]any{
			"MainAppEntry": "cmd/app/main.go",
			"DistDir":      "dist",
			"StaticAssetDirs": map[string]any{
				"Private": "private",
				"Public":  "public",
			},
		},
		"Vorma": vormaCfg,
	}

	return mustMarshalJSON(t, cfg)
}

func makeStaticFS(includeStageTwoPaths bool, includeTemplate bool) fstest.MapFS {
	m := fstest.MapFS{}

	if includeTemplate {
		m[path.Join("assets/private", testTemplatePath)] = &fstest.MapFile{
			Data: []byte(`<!doctype html><html><head>{{.VormaHeadEls}}</head><body><div id="{{.VormaRootID}}"></div>{{.VormaBodyScripts}}{{.VormaSSRScript}}</body></html>`),
		}
	}

	if includeStageTwoPaths {
		m[path.Join("assets/private", vormaruntime.VormaOutDirname, vormaruntime.VormaPathsStageTwoJSONFileName)] = &fstest.MapFile{
			Data: mustMarshalJSON(nil, map[string]any{
				"stage":             "2",
				"buildID":           testBuildID,
				"clientEntrySrc":    "frontend/src/vorma.entry.tsx",
				"paths":             map[string]any{},
				"routeManifestFile": "vorma_out/route_manifest.json",
				"clientEntryOut":    "vorma_out/client.js",
				"clientEntryDeps":   []string{},
				"depToCSSBundleMap": map[string][]string{},
			}),
		}
	}

	return m
}

func mustMarshalJSON(t *testing.T, v any) []byte {
	if t != nil {
		t.Helper()
	}
	b, err := json.Marshal(v)
	if err != nil {
		if t == nil {
			panic(fmt.Sprintf("failed to marshal JSON: %v", err))
		}
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	return b
}

func mustPanic(t *testing.T, fn func()) string {
	t.Helper()

	var panicValue any
	func() {
		defer func() {
			panicValue = recover()
		}()
		fn()
	}()

	if panicValue == nil {
		t.Fatalf("expected panic but none occurred")
	}

	switch v := panicValue.(type) {
	case error:
		return v.Error()
	default:
		return fmt.Sprint(v)
	}
}
