package backend_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vormadev/vorma"
)

func TestBackendJSONConformance(t *testing.T) {
	t.Run("BRC-JSON-001_BR-JSON-001_non_empty_vorma_json_enables_json_mode", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFS(true, true))
		vorma.NewLoader(
			app,
			"/page",
			func(*vorma.LoaderReqData) (string, error) { return "page-data", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		testCases := []struct {
			name                    string
			path                    string
			expectJSON              bool
			expectReloadHeaderValue string
		}{
			{
				name:       "no_vorma_json_query",
				path:       "/page",
				expectJSON: false,
			},
			{
				name:       "empty_vorma_json_query",
				path:       "/page?vorma_json=",
				expectJSON: false,
			},
			{
				name:                    "non_empty_vorma_json_query",
				path:                    "/page?vorma_json=abc",
				expectJSON:              true,
				expectReloadHeaderValue: "/page",
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, tc.path, nil)
				r.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
				}

				contentType := rec.Header().Get("Content-Type")
				isJSON := strings.HasPrefix(contentType, "application/json")
				if isJSON != tc.expectJSON {
					t.Fatalf("expected JSON=%v for path %q, got content-type %q", tc.expectJSON, tc.path, contentType)
				}

				reloadHeader := rec.Header().Get("X-Vorma-Reload")
				if tc.expectReloadHeaderValue == "" {
					if reloadHeader != "" {
						t.Fatalf("expected empty X-Vorma-Reload for path %q, got %q", tc.path, reloadHeader)
					}
					return
				}
				if reloadHeader != tc.expectReloadHeaderValue {
					t.Fatalf("expected X-Vorma-Reload=%q, got %q", tc.expectReloadHeaderValue, reloadHeader)
				}
			})
		}
	})

	t.Run("BRC-JSON-002_BR-JSON-002_stale_build_returns_reload_signal", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFS(true, true))
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/page?x=1&vorma_json=stale&y=2", nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		contentType := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Fatalf("expected application/json content-type, got %q", contentType)
		}

		if got := rec.Header().Get("X-Vorma-Reload"); got != "/page?x=1&y=2" {
			t.Fatalf("expected X-Vorma-Reload=%q, got %q", "/page?x=1&y=2", got)
		}

		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("expected JSON body, got err=%v body=%q", err, rec.Body.String())
		}
		okRaw, exists := payload["ok"]
		okVal, isBool := okRaw.(bool)
		if !exists || !isBool || !okVal {
			t.Fatalf("expected stale-build body to include {\"ok\":true}, got %#v", payload)
		}
	})

	t.Run("BRC-JSON-003_BR-JSON-003_current_build_json_returns_route_data_payload", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFS(true, true))
		vorma.NewLoader(
			app,
			"/users/:id",
			func(*vorma.LoaderReqData) (map[string]any, error) {
				return map[string]any{"ok": true}, nil
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/users/42?vorma_json="+testBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		contentType := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Fatalf("expected application/json content-type, got %q", contentType)
		}

		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("expected JSON body, got err=%v body=%q", err, rec.Body.String())
		}

		requiredKeys := []string{"matchedPatterns", "loadersData", "importURLs", "exportKeys", "params"}
		for _, key := range requiredKeys {
			if _, ok := payload[key]; !ok {
				t.Fatalf("expected route-data payload key %q, got payload=%#v", key, payload)
			}
		}

		matchedPatterns, ok := payload["matchedPatterns"].([]any)
		if !ok || len(matchedPatterns) == 0 {
			t.Fatalf("expected non-empty matchedPatterns, got %#v", payload["matchedPatterns"])
		}
		lastPattern, _ := matchedPatterns[len(matchedPatterns)-1].(string)
		if lastPattern != "/users/:id" {
			t.Fatalf("expected last matched pattern %q, got %#v", "/users/:id", matchedPatterns[len(matchedPatterns)-1])
		}

		params, ok := payload["params"].(map[string]any)
		if !ok {
			t.Fatalf("expected params map, got %#v", payload["params"])
		}
		if got, _ := params["id"].(string); got != "42" {
			t.Fatalf("expected params.id=%q, got %#v", "42", params["id"])
		}

		loadersData, ok := payload["loadersData"].([]any)
		if !ok || len(loadersData) == 0 {
			t.Fatalf("expected non-empty loadersData, got %#v", payload["loadersData"])
		}
		lastLoaderData, ok := loadersData[len(loadersData)-1].(map[string]any)
		if !ok {
			t.Fatalf("expected last loadersData entry map, got %#v", loadersData[len(loadersData)-1])
		}
		if got, _ := lastLoaderData["ok"].(bool); !got {
			t.Fatalf("expected last loadersData entry to include ok=true, got %#v", lastLoaderData)
		}
	})
}
