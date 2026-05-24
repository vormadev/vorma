package vormarun_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/vormadev/vorma/internal/pkg/vormarun"
	"golang.org/x/net/html"
)

type test_request_ctx[I any] struct{ *vormarun.RequestCtx[I] }

func (test_request_ctx[I]) Wrap(c *vormarun.RequestCtx[I]) *test_request_ctx[I] {
	return &test_request_ctx[I]{RequestCtx: c}
}

type public_test_harness struct {
	t *testing.T
}

const (
	emitted_public_asset_url     = "/vorma_out_vite_dynamic.js"
	emitted_public_asset_fs_path = "public/vorma_out_vite_dynamic.js"
)

type page_output struct {
	ID     string
	Filter string
}

type api_route_input struct {
	Message string
}

type api_route_output struct {
	ID      string
	Message string
}

func TestPublicRouterServesHTMLLoaderPayload(t *testing.T) {
	h := public_test_harness{t: t}
	router := h.init_router(
		h.default_manifest(),
		vormarun.Views{
			vormarun.View[
				struct{},
				page_output,
				*test_request_ctx[struct{}],
				test_request_ctx[struct{}],
			]{
				Pattern:    "/items/:id",
				ClientFile: "items.tsx",
				Loader: func(ctx *test_request_ctx[struct{}]) (page_output, error) {
					return page_output{
						ID:     ctx.Param("id"),
						Filter: ctx.Request().URL.Query().Get("filter"),
					}, nil
				},
			},
		},
		nil,
	)

	res := httptest.NewRecorder()
	router.ServeHTTP(
		res,
		httptest.NewRequest(http.MethodGet, "/items/42?filter=active", nil),
	)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if got := res.Header().Get(vormarun.X_Vorma_Client_Build_Id); got == "" {
		t.Fatalf("expected %s header", vormarun.X_Vorma_Client_Build_Id)
	}
	if got := res.Header().Get("Cache-Control"); got == "" {
		t.Fatalf("expected Cache-Control header")
	}

	payload := h.html_payload(res.Body.Bytes())
	h.assert_payload_string_slice(payload, "MatchedPatterns", []string{"/items/:id"})
	h.assert_payload_string(
		payload,
		"ClientBuildID",
		res.Header().Get(vormarun.X_Vorma_Client_Build_Id),
	)
	if got, ok := payload["IsDev"].(bool); ok && got {
		t.Fatalf("expected IsDev to be false or omitted, got %t", got)
	}
	h.assert_payload_string_map(payload, "Params", map[string]string{"id": "42"})
	h.assert_payload_string_slice(payload, "ImportURLs", []string{"/static/items.js"})
	h.assert_payload_string_slice(
		payload,
		"Deps",
		[]string{"/static/entry.js", "/static/shared.js", "/static/items.js"},
	)
	h.assert_payload_string_slice(
		payload,
		"CSSBundles",
		[]string{"/static/entry.css", "/static/items.css"},
	)

	loaders_data, ok := payload["LoadersData"].([]any)
	if !ok || len(loaders_data) != 1 {
		t.Fatalf("expected one loader payload, got %#v", payload["LoadersData"])
	}
	page_data, ok := loaders_data[0].(map[string]any)
	if !ok {
		t.Fatalf("expected object loader payload, got %#v", loaders_data[0])
	}
	h.assert_payload_string(page_data, "ID", "42")
	h.assert_payload_string(page_data, "Filter", "active")
}

func TestPublicRouterServesJSONAndReportsBuildSkew(t *testing.T) {
	h := public_test_harness{t: t}
	router := h.init_router(
		h.default_manifest(),
		vormarun.Views{
			vormarun.View[
				struct{},
				page_output,
				*test_request_ctx[struct{}],
				test_request_ctx[struct{}],
			]{
				Pattern:    "/items/:id",
				ClientFile: "items.tsx",
				Loader: func(ctx *test_request_ctx[struct{}]) (page_output, error) {
					return page_output{ID: ctx.Param("id")}, nil
				},
			},
		},
		nil,
	)

	html_res := httptest.NewRecorder()
	router.ServeHTTP(html_res, httptest.NewRequest(http.MethodGet, "/items/42", nil))
	client_build_id := html_res.Header().Get(vormarun.X_Vorma_Client_Build_Id)
	if client_build_id == "" {
		t.Fatalf("expected client build id header")
	}

	json_res := httptest.NewRecorder()
	router.ServeHTTP(
		json_res,
		httptest.NewRequest(
			http.MethodGet,
			"/items/42?"+vormarun.Query_Key_Vorma_JSON+"="+client_build_id,
			nil,
		),
	)
	if json_res.Code != http.StatusOK {
		t.Fatalf("expected JSON status 200, got %d", json_res.Code)
	}
	payload := h.json_payload(json_res.Body.Bytes())
	h.assert_payload_string_slice(payload, "MatchedPatterns", []string{"/items/:id"})
	h.assert_payload_string_map(payload, "Params", map[string]string{"id": "42"})

	skew_res := httptest.NewRecorder()
	router.ServeHTTP(
		skew_res,
		httptest.NewRequest(
			http.MethodGet,
			"/items/42?"+vormarun.Query_Key_Vorma_JSON+"=stale-build",
			nil,
		),
	)
	if skew_res.Code != http.StatusOK {
		t.Fatalf("expected skew status 200, got %d", skew_res.Code)
	}
	if got := skew_res.Header().Get(vormarun.X_Vorma_Build_Skew); got != "1" {
		t.Fatalf("expected skew header %q, got %q", "1", got)
	}
	skew_payload := h.json_payload(skew_res.Body.Bytes())
	skew_ok, ok := skew_payload["ok"].(bool)
	if !ok || !skew_ok {
		t.Fatalf("expected skew OK payload, got %#v", skew_payload)
	}
}

func TestPublicRouterServesAPIRoutesFromConfiguredAPIMount(t *testing.T) {
	h := public_test_harness{t: t}
	router := h.init_router(
		h.default_manifest(),
		vormarun.Views{
			vormarun.View[
				struct{},
				struct{},
				*test_request_ctx[struct{}],
				test_request_ctx[struct{}],
			]{
				Pattern:    "/",
				ClientFile: "root.tsx",
			},
		},
		vormarun.APIRoutes{
			vormarun.APIRoute[
				api_route_input,
				api_route_output,
				*test_request_ctx[api_route_input],
				test_request_ctx[api_route_input],
			]{
				Method:  http.MethodPost,
				Pattern: "/echo/:id",
				Kind:    vormarun.APIRouteKindMutation,
				Handler: func(ctx *test_request_ctx[api_route_input]) (api_route_output, error) {
					return api_route_output{
						ID:      ctx.Param("id"),
						Message: ctx.Input().Message,
					}, nil
				},
			},
		},
	)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/echo/abc",
		bytes.NewBufferString(`{"Message":"hello"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if got := res.Header().Get(vormarun.X_Vorma_Client_Build_Id); got == "" {
		t.Fatalf("expected %s header", vormarun.X_Vorma_Client_Build_Id)
	}
	payload := h.json_payload(res.Body.Bytes())
	h.assert_payload_string(payload, "ID", "abc")
	h.assert_payload_string(payload, "Message", "hello")
}

func TestPublicStaticMiddlewareServesManifestAssetsOnly(t *testing.T) {
	h := public_test_harness{t: t}
	v := vormarun.New(&vormarun.Config{})
	v.MustSetStaticFS(h.static_fs(h.default_manifest()))
	router, err := v.Router()
	if err != nil {
		t.Fatalf("error getting router: %v", err)
	}
	err = router.UsePublicFileServerMiddleware()
	if err != nil {
		t.Fatalf("error adding public file server middleware: %v", err)
	}
	router.AddHTTPHandlerFunc("GET", "/*", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	asset_res := httptest.NewRecorder()
	router.Router.ServeHTTP(
		asset_res,
		httptest.NewRequest(http.MethodGet, "/static/favicon.ico", nil),
	)
	if asset_res.Code != http.StatusOK {
		t.Fatalf("expected asset status 200, got %d", asset_res.Code)
	}
	if got := asset_res.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("expected immutable cache header, got %q", got)
	}
	if got := asset_res.Body.String(); got != "ico" {
		t.Fatalf("expected static asset body, got %q", got)
	}

	pass_res := httptest.NewRecorder()
	router.Router.ServeHTTP(
		pass_res,
		httptest.NewRequest(http.MethodGet, "/not-public", nil),
	)
	if pass_res.Code != http.StatusAccepted {
		t.Fatalf("expected request to pass through, got %d", pass_res.Code)
	}
}

func TestPublicStaticMiddlewareServesEmittedFilesWhenBaseIsRoot(t *testing.T) {
	h := public_test_harness{t: t}
	manifest := h.default_manifest()
	manifest.PublicStaticBasePath = "/"
	manifest.PublicFilepaths = []string{emitted_public_asset_url}

	v := vormarun.New(&vormarun.Config{})
	v.MustSetStaticFS(h.static_fs(manifest))
	router, err := v.Router()
	if err != nil {
		t.Fatalf("error getting router: %v", err)
	}
	err = router.UsePublicFileServerMiddleware()
	if err != nil {
		t.Fatalf("error adding public file server middleware: %v", err)
	}
	router.AddHTTPHandlerFunc("GET", "/*", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	asset_res := httptest.NewRecorder()
	router.Router.ServeHTTP(
		asset_res,
		httptest.NewRequest(http.MethodGet, emitted_public_asset_url, nil),
	)
	if asset_res.Code != http.StatusOK {
		t.Fatalf("expected emitted asset status 200, got %d", asset_res.Code)
	}
	if got := asset_res.Body.String(); got != "dynamic" {
		t.Fatalf("expected emitted asset body, got %q", got)
	}

	pass_res := httptest.NewRecorder()
	router.Router.ServeHTTP(
		pass_res,
		httptest.NewRequest(http.MethodGet, "/not-public", nil),
	)
	if pass_res.Code != http.StatusAccepted {
		t.Fatalf("expected request to pass through, got %d", pass_res.Code)
	}
}

func (h public_test_harness) default_manifest() vormarun.Manifest {
	return vormarun.Manifest{
		VormaVersion:         "test",
		PublicStaticBasePath: "/static/",
		APIMountRoot:         "/api/",
		UIVariant:            "react",
		PublicFilepaths: []string{
			"/static/favicon.ico",
			"/static/entry.css",
			"/static/entry.js",
			"/static/items.css",
			"/static/items.js",
			"/static/shared.js",
		},
		PublicFilemap: map[string]string{
			"favicon.ico": "/static/favicon.ico",
		},
		CriticalCSS: "html{color-scheme:light}",
		ClientEntry: vormarun.ClientModule{
			URL:           "/static/entry.js",
			DepURLs:       []string{"/static/entry.js", "/static/shared.js"},
			CSSBundleURLs: []string{"/static/entry.css"},
		},
		ClientRoutes: map[string]vormarun.ClientModule{
			"/": {
				URL: "/static/root.js",
			},
			"/items/:id": {
				URL:           "/static/items.js",
				DepURLs:       []string{"/static/shared.js", "/static/items.js"},
				CSSBundleURLs: []string{"/static/items.css"},
			},
		},
	}
}

func (h public_test_harness) init_router(
	manifest vormarun.Manifest,
	views vormarun.Views,
	api_routes vormarun.APIRoutes,
) *vormarun.Router {
	h.t.Helper()

	v := vormarun.New(&vormarun.Config{})
	v.MustSetStaticFS(h.static_fs(manifest))
	router, err := v.Router()
	if err != nil {
		h.t.Fatalf("error getting router: %v", err)
	}
	for _, view := range views {
		router.View(view)
	}
	for _, api_route := range api_routes {
		router.APIRoute(api_route)
	}
	return router
}

func (h public_test_harness) static_fs(manifest vormarun.Manifest) fstest.MapFS {
	manifest_json, err := json.Marshal(manifest)
	if err != nil {
		h.t.Fatalf("error marshaling manifest: %v", err)
	}
	return fstest.MapFS{
		vormarun.ManifestStaticOutProd: &fstest.MapFile{Data: manifest_json},
		"public/favicon.ico":           &fstest.MapFile{Data: []byte("ico")},
		emitted_public_asset_fs_path:   &fstest.MapFile{Data: []byte("dynamic")},
	}
}

func (h public_test_harness) html_payload(body []byte) map[string]any {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		h.t.Fatalf("error parsing HTML: %v", err)
	}
	text, ok := h.data_script_text(doc)
	if !ok {
		h.t.Fatalf("expected data script %s", vormarun.Vorma_Data_JSON_Script_El_ID)
	}
	return h.json_payload([]byte(text))
}

func (h public_test_harness) data_script_text(node *html.Node) (string, bool) {
	if node.Type == html.ElementNode && node.Data == "script" {
		for _, attr := range node.Attr {
			if attr.Key == "id" && attr.Val == vormarun.Vorma_Data_JSON_Script_El_ID {
				if node.FirstChild == nil {
					return "", true
				}
				return node.FirstChild.Data, true
			}
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		text, ok := h.data_script_text(child)
		if ok {
			return text, true
		}
	}
	return "", false
}

func (h public_test_harness) json_payload(body []byte) map[string]any {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		h.t.Fatalf("error unmarshaling JSON payload: %v", err)
	}
	return payload
}

func (h public_test_harness) assert_payload_string(
	payload map[string]any,
	key string,
	expected string,
) {
	h.t.Helper()

	got, ok := payload[key].(string)
	if !ok {
		h.t.Fatalf("expected %s to be string, got %#v", key, payload[key])
	}
	if got != expected {
		h.t.Fatalf("expected %s %q, got %q", key, expected, got)
	}
}

func (h public_test_harness) assert_payload_string_slice(
	payload map[string]any,
	key string,
	expected []string,
) {
	h.t.Helper()

	raw, ok := payload[key].([]any)
	if !ok {
		h.t.Fatalf("expected %s to be array, got %#v", key, payload[key])
	}
	if len(raw) != len(expected) {
		h.t.Fatalf("expected %s %v, got %#v", key, expected, raw)
	}
	for i, item := range raw {
		got, ok := item.(string)
		if !ok {
			h.t.Fatalf("expected %s[%d] to be string, got %#v", key, i, item)
		}
		if got != expected[i] {
			h.t.Fatalf("expected %s[%d] %q, got %q", key, i, expected[i], got)
		}
	}
}

func (h public_test_harness) assert_payload_string_map(
	payload map[string]any,
	key string,
	expected map[string]string,
) {
	h.t.Helper()

	raw, ok := payload[key].(map[string]any)
	if !ok {
		h.t.Fatalf("expected %s to be object, got %#v", key, payload[key])
	}
	if len(raw) != len(expected) {
		h.t.Fatalf("expected %s %v, got %#v", key, expected, raw)
	}
	for k, expected_v := range expected {
		got, ok := raw[k].(string)
		if !ok {
			h.t.Fatalf("expected %s.%s to be string, got %#v", key, k, raw[k])
		}
		if got != expected_v {
			h.t.Fatalf("expected %s.%s %q, got %q", key, k, expected_v, got)
		}
	}
}
