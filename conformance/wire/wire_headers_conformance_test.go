package wire_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/vormaruntime"
)

func TestWireHeadersConformance(t *testing.T) {
	t.Run("WRC-HDR-001_WIRE-HDR-001_loader_responses_include_build_id_header", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths([]wirePathDef{
				{Pattern: "/hdr/page", SrcPath: "src/hdr/page.tsx", OutPath: "routes/hdr_page.js", ExportKey: "Page"},
				{Pattern: "/hdr/proxy-redirect", SrcPath: "src/hdr/proxy_redirect.tsx", OutPath: "routes/hdr_proxy_redirect.js", ExportKey: "ProxyRedirect"},
				{Pattern: "/hdr/proxy-error", SrcPath: "src/hdr/proxy_error.tsx", OutPath: "routes/hdr_proxy_error.js", ExportKey: "ProxyError"},
			}, nil),
			nil,
		)
		vorma.NewLoader(
			app,
			"/hdr/page",
			func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewLoader(
			app,
			"/hdr/proxy-redirect",
			func(rd *vorma.LoaderReqData) (string, error) {
				_, err := rd.ResponseProxy().Redirect(rd.Request(), "/hdr/target", http.StatusFound)
				return "", err
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewLoader(
			app,
			"/hdr/proxy-error",
			func(rd *vorma.LoaderReqData) (string, error) {
				rd.ResponseProxy().SetStatus(http.StatusTeapot, "proxy-fail")
				return "", nil
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		testCases := []struct {
			name       string
			path       string
			method     string
			wantStatus int
		}{
			{
				name:       "document_success",
				path:       "/hdr/page",
				method:     http.MethodGet,
				wantStatus: http.StatusOK,
			},
			{
				name:       "current_build_json_success",
				path:       "/hdr/page?vorma_json=" + wireBuildID,
				method:     http.MethodGet,
				wantStatus: http.StatusOK,
			},
			{
				name:       "stale_build_json_success",
				path:       "/hdr/page?vorma_json=stale",
				method:     http.MethodGet,
				wantStatus: http.StatusOK,
			},
			{
				name:       "loader_not_found",
				path:       "/hdr/missing?vorma_json=" + wireBuildID,
				method:     http.MethodGet,
				wantStatus: http.StatusNotFound,
			},
			{
				name:       "loader_proxy_redirect",
				path:       "/hdr/proxy-redirect?vorma_json=" + wireBuildID,
				method:     http.MethodGet,
				wantStatus: http.StatusFound,
			},
			{
				name:       "loader_proxy_error",
				path:       "/hdr/proxy-error?vorma_json=" + wireBuildID,
				method:     http.MethodGet,
				wantStatus: http.StatusTeapot,
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(tc.method, tc.path, nil)
				r.ServeHTTP(rec, req)

				if rec.Code != tc.wantStatus {
					t.Fatalf("expected status %d, got %d body=%q", tc.wantStatus, rec.Code, rec.Body.String())
				}
				if got := rec.Header().Get(vormaruntime.VormaBuildIDHeaderKey); got != wireBuildID {
					t.Fatalf("expected %s=%q, got %q", vormaruntime.VormaBuildIDHeaderKey, wireBuildID, got)
				}
			})
		}
	})

	t.Run("WRC-HDR-002_WIRE-HDR-002_action_responses_include_build_id_header", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		type actionInput struct {
			Name string `json:"name"`
		}

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths(nil, nil),
			nil,
		)
		vorma.NewAction[actionInput, actionInput](
			app,
			http.MethodPost,
			"/hdr/action",
			func(rd *vorma.ActionReqData[actionInput]) (actionInput, error) {
				return rd.Input(), nil
			},
			func(rd *vorma.ActionReqData[actionInput]) *vorma.ActionReqData[actionInput] { return rd },
		)
		r := app.InitWithDefaultRouter()

		testCases := []struct {
			name        string
			path        string
			body        string
			contentType string
			wantStatus  int
		}{
			{
				name:        "success",
				path:        "/api/hdr/action",
				body:        `{"name":"alice"}`,
				contentType: "application/json",
				wantStatus:  http.StatusOK,
			},
			{
				name:        "validation_failure",
				path:        "/api/hdr/action",
				body:        `{"name":`,
				contentType: "application/json",
				wantStatus:  http.StatusBadRequest,
			},
			{
				name:        "not_found",
				path:        "/api/hdr/missing",
				body:        `{}`,
				contentType: "application/json",
				wantStatus:  http.StatusNotFound,
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewBufferString(tc.body))
				req.Header.Set("Content-Type", tc.contentType)
				r.ServeHTTP(rec, req)

				if rec.Code != tc.wantStatus {
					t.Fatalf("expected status %d, got %d body=%q", tc.wantStatus, rec.Code, rec.Body.String())
				}
				if got := rec.Header().Get(vormaruntime.VormaBuildIDHeaderKey); got != wireBuildID {
					t.Fatalf("expected %s=%q, got %q", vormaruntime.VormaBuildIDHeaderKey, wireBuildID, got)
				}
			})
		}
	})

	t.Run("WRC-HDR-003_WIRE-HDR-003_stale_json_response_sets_reload_header_without_vorma_json", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths(nil, nil),
			nil,
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/users/1?x=1&vorma_json=stale&y=2", nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("X-Vorma-Reload"); got != "/users/1?x=1&y=2" {
			t.Fatalf("expected X-Vorma-Reload=%q, got %q", "/users/1?x=1&y=2", got)
		}
	})

	t.Run("WRC-HDR-004_WIRE-HDR-004_client_redirect_header_carries_exact_target", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		const redirectTarget = "/hdr/client-redirect-target?from=loader"

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths([]wirePathDef{
				{Pattern: "/hdr/client-redirect", SrcPath: "src/hdr/client_redirect.tsx", OutPath: "routes/hdr_client_redirect.js", ExportKey: "ClientRedirect"},
			}, nil),
			nil,
		)
		vorma.NewLoader(
			app,
			"/hdr/client-redirect",
			func(rd *vorma.LoaderReqData) (string, error) {
				_, err := rd.ResponseProxy().Redirect(rd.Request(), redirectTarget, http.StatusFound)
				return "", err
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/hdr/client-redirect?vorma_json="+wireBuildID, nil)
		req.Header.Set(response.ClientAcceptsRedirectHeader, "1")
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get(response.ClientRedirectHeader); got != redirectTarget {
			t.Fatalf("expected %s=%q, got %q", response.ClientRedirectHeader, redirectTarget, got)
		}
	})

	t.Run("WRC-HDR-005_WRC-HDR-006_WIRE-HDR-005_cache_control_default_and_override", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths([]wirePathDef{
				{Pattern: "/hdr/cache-default", SrcPath: "src/hdr/cache_default.tsx", OutPath: "routes/hdr_cache_default.js", ExportKey: "CacheDefault"},
				{Pattern: "/hdr/cache-explicit", SrcPath: "src/hdr/cache_explicit.tsx", OutPath: "routes/hdr_cache_explicit.js", ExportKey: "CacheExplicit"},
			}, nil),
			nil,
		)
		vorma.NewLoader(
			app,
			"/hdr/cache-default",
			func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewLoader(
			app,
			"/hdr/cache-explicit",
			func(rd *vorma.LoaderReqData) (string, error) {
				rd.ResponseProxy().SetHeader("Cache-Control", "public, max-age=60")
				return "ok", nil
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		defaultRec := httptest.NewRecorder()
		defaultReq := httptest.NewRequest(http.MethodGet, "/hdr/cache-default", nil)
		r.ServeHTTP(defaultRec, defaultReq)
		if defaultRec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, defaultRec.Code, defaultRec.Body.String())
		}
		if got := defaultRec.Header().Get("Cache-Control"); got != "private, max-age=0, must-revalidate, no-cache" {
			t.Fatalf("expected default Cache-Control, got %q", got)
		}

		explicitRec := httptest.NewRecorder()
		explicitReq := httptest.NewRequest(http.MethodGet, "/hdr/cache-explicit", nil)
		r.ServeHTTP(explicitRec, explicitReq)
		if explicitRec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, explicitRec.Code, explicitRec.Body.String())
		}
		if got := explicitRec.Header().Get("Cache-Control"); got != "public, max-age=60" {
			t.Fatalf("expected explicit Cache-Control override, got %q", got)
		}
	})
}
