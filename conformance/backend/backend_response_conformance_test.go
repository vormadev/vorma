package backend_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/vormaruntime"
)

func TestBackendResponseConformance(t *testing.T) {
	t.Run("BRC-RESP-001_BR-RESP-001_loader_responses_include_build_id_header", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFS(true, true))

		vorma.NewLoader(
			app,
			"/resp-loader",
			func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)

		r := app.InitWithDefaultRouter()

		testCases := []struct {
			name           string
			path           string
			expectedStatus int
		}{
			{
				name:           "matched_loader_path",
				path:           "/resp-loader?vorma_json=" + testBuildID,
				expectedStatus: http.StatusOK,
			},
			{
				name:           "unmatched_loader_path",
				path:           "/resp-loader-missing?vorma_json=" + testBuildID,
				expectedStatus: http.StatusNotFound,
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, tc.path, nil)
				r.ServeHTTP(rec, req)

				if rec.Code != tc.expectedStatus {
					t.Fatalf("expected status %d, got %d; body=%q", tc.expectedStatus, rec.Code, rec.Body.String())
				}

				if got := rec.Header().Get(vormaruntime.VormaBuildIDHeaderKey); got != testBuildID {
					t.Fatalf("expected %s=%q, got %q", vormaruntime.VormaBuildIDHeaderKey, testBuildID, got)
				}
			})
		}
	})

	t.Run("BRC-RESP-002_BR-RESP-002_action_responses_include_build_id_header", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFS(true, true))

		vorma.NewAction[vorma.None, string](
			app,
			http.MethodPost,
			"/resp-action",
			func(*vorma.ActionReqData[vorma.None]) (string, error) { return "ok", nil },
			func(rd *vorma.ActionReqData[vorma.None]) *vorma.ActionReqData[vorma.None] { return rd },
		)

		r := app.InitWithDefaultRouter()

		testCases := []struct {
			name           string
			path           string
			expectedStatus int
		}{
			{
				name:           "matched_action_path",
				path:           "/api/resp-action",
				expectedStatus: http.StatusOK,
			},
			{
				name:           "unmatched_action_path",
				path:           "/api/resp-action-missing",
				expectedStatus: http.StatusNotFound,
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, tc.path, nil)
				r.ServeHTTP(rec, req)

				if rec.Code != tc.expectedStatus {
					t.Fatalf("expected status %d, got %d; body=%q", tc.expectedStatus, rec.Code, rec.Body.String())
				}

				if got := rec.Header().Get(vormaruntime.VormaBuildIDHeaderKey); got != testBuildID {
					t.Fatalf("expected %s=%q, got %q", vormaruntime.VormaBuildIDHeaderKey, testBuildID, got)
				}
			})
		}
	})
}
