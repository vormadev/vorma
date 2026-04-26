package vormarun

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

type handler_test_request_ctx[I any] struct{ *RequestCtx[I] }

func (handler_test_request_ctx[I]) Wrap(c *RequestCtx[I]) *handler_test_request_ctx[I] {
	return &handler_test_request_ctx[I]{RequestCtx: c}
}

type handler_test_loader[I any, O any] = Loader[
	I,
	O,
	*handler_test_request_ctx[I],
	handler_test_request_ctx[I],
]

type handler_test_harness struct {
	t *testing.T
}

func TestLoadersHandlerProdCSSBundlesKeepOrderAfterMainCSS(t *testing.T) {
	h := handler_test_harness{t: t}
	manifest := Manifest{
		VormaVersion:         "test",
		PublicStaticBasePath: "/static/",
		ActionsMountRoot:     "/api/",
		UIVariant:            "react",
		PublicFilemap: map[string]string{
			Main_CSS_Filename: "/static/main.css",
		},
		CriticalCSS: "body { color: black; }",
		ClientEntry: ClientModule{
			URL:           "/static/entry.js",
			DepURLs:       []string{"/static/entry.js", "/static/shared.js"},
			CSSBundleURLs: []string{"/static/entry.css", "/static/shared.css"},
		},
		ClientRoutes: map[string]ClientModule{
			"/": {
				URL:     "/static/root.js",
				DepURLs: []string{"/static/shared.js", "/static/root.js"},
				CSSBundleURLs: []string{
					"/static/shared.css",
					"/static/root.css",
					"/static/entry.css",
				},
			},
		},
	}

	router := h.init_router(manifest)

	html_res := httptest.NewRecorder()
	router.ServeHTTP(html_res, httptest.NewRequest(http.MethodGet, "/", nil))
	if html_res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", html_res.Code)
	}

	body := html_res.Body.String()
	expected_order := []string{
		`id="vorma-main-css"`,
		`data-vorma-css-bundle="/static/entry.css"`,
		`data-vorma-css-bundle="/static/shared.css"`,
		`data-vorma-css-bundle="/static/root.css"`,
	}
	h.assert_substrings_in_order(body, expected_order)

	if count := strings.Count(body, `data-vorma-css-bundle="/static/entry.css"`); count != 1 {
		t.Fatalf("expected entry CSS bundle once, got %d", count)
	}
	if count := strings.Count(body, `data-vorma-css-bundle="/static/shared.css"`); count != 1 {
		t.Fatalf("expected shared CSS bundle once, got %d", count)
	}

	json_res := httptest.NewRecorder()
	router.ServeHTTP(json_res, httptest.NewRequest(
		http.MethodGet,
		"/?"+Query_Key_Vorma_JSON+"=_",
		nil,
	))
	if json_res.Code != http.StatusOK {
		t.Fatalf("expected JSON status 200, got %d", json_res.Code)
	}

	var payload loader_payload
	if err := json.Unmarshal(json_res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("error unmarshaling loader payload: %v", err)
	}

	expected_css_bundles := []string{
		"/static/entry.css",
		"/static/shared.css",
		"/static/root.css",
	}
	if !slices.Equal(payload.CSSBundles, expected_css_bundles) {
		t.Fatalf(
			"expected CSSBundles %v, got %v",
			expected_css_bundles,
			payload.CSSBundles,
		)
	}
}

func TestLoadersHandlerInjectsVercelDeploymentID(t *testing.T) {
	t.Setenv("VERCEL_SKEW_PROTECTION_ENABLED", "1")
	t.Setenv("VERCEL_DEPLOYMENT_ID", "dpl_test_123")

	h := handler_test_harness{t: t}
	manifest := Manifest{
		VormaVersion:         "test",
		PublicStaticBasePath: "/static/",
		ActionsMountRoot:     "/api/",
		UIVariant:            "react",
		PublicFilemap: map[string]string{
			Main_CSS_Filename: "/static/main.css",
		},
		ClientEntry: ClientModule{
			URL: "/static/entry.js",
		},
		ClientRoutes: map[string]ClientModule{
			"/": {
				URL: "/static/root.js",
			},
		},
	}

	router := h.init_router(manifest)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}

	body := res.Body.String()
	id_idx := strings.Index(body, `id="`+Vorma_Data_JSON_Script_El_ID+`"`)
	if id_idx == -1 {
		t.Fatalf("expected body to contain %s script", Vorma_Data_JSON_Script_El_ID)
	}
	open_end := strings.Index(body[id_idx:], ">")
	if open_end == -1 {
		t.Fatalf("expected %s script opening tag to close", Vorma_Data_JSON_Script_El_ID)
	}
	open_end += id_idx
	close_idx := strings.Index(body[open_end:], "</script>")
	if close_idx == -1 {
		t.Fatalf("expected %s script closing tag", Vorma_Data_JSON_Script_El_ID)
	}
	close_idx += open_end

	var payload ssr_payload
	if err := json.Unmarshal([]byte(body[open_end+1:close_idx]), &payload); err != nil {
		t.Fatalf("error unmarshaling SSR payload: %v", err)
	}
	if payload.DeploymentID != "dpl_test_123" {
		t.Fatalf("expected DeploymentID %q, got %q", "dpl_test_123", payload.DeploymentID)
	}
}

func (h handler_test_harness) init_router(manifest Manifest) *Router {
	h.t.Helper()

	manifest_json, err := json.Marshal(manifest)
	if err != nil {
		h.t.Fatalf("error marshaling manifest: %v", err)
	}

	v := &Vorma{}
	router, err := InitRouter(
		v,
		Loaders{
			handler_test_loader[struct{}, struct{}]{
				Pattern:  "/",
				TSModule: "root.tsx",
			},
		},
		nil,
		fstest.MapFS{
			ManifestStaticOutProd: &fstest.MapFile{Data: manifest_json},
		},
	)
	if err != nil {
		h.t.Fatalf("error initializing router: %v", err)
	}
	return router
}

func (h handler_test_harness) assert_substrings_in_order(body string, expected []string) {
	h.t.Helper()

	last_idx := -1
	for _, s := range expected {
		idx := strings.Index(body, s)
		if idx == -1 {
			h.t.Fatalf("expected body to contain %q", s)
		}
		if idx <= last_idx {
			h.t.Fatalf("expected %q after previous substring", s)
		}
		last_idx = idx
	}
}
