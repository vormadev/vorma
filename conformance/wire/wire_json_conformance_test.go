package wire_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/vormadev/vorma"
)

func TestWireJSONConformance(t *testing.T) {
	t.Run("WRC-JSON-001_WIRE-JSON-001_current_build_json_returns_top_level_object", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths([]wirePathDef{
				{Pattern: "/json1/page", SrcPath: "src/json1/page.tsx", OutPath: "routes/json1_page.js", ExportKey: "Page"},
			}, nil),
			nil,
		)
		vorma.NewLoader(
			app,
			"/json1/page",
			func(*vorma.LoaderReqData) (string, error) { return "ok", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/json1/page?vorma_json="+wireBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		payload := mustJSONMap(t, rec.Body.Bytes())
		if len(payload) == 0 {
			t.Fatalf("expected non-empty JSON object payload")
		}
	})

	t.Run("WRC-JSON-002_WIRE-JSON-002_stale_build_returns_ok_sentinel_only", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths(nil, nil),
			nil,
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/json2/page?vorma_json=stale", nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		payload := mustJSONMap(t, rec.Body.Bytes())
		if len(payload) != 1 || payload["ok"] != true {
			t.Fatalf("expected stale sentinel body {\"ok\":true}, got %#v", payload)
		}
		for _, forbiddenKey := range []string{
			"matchedPatterns",
			"loadersData",
			"importURLs",
			"exportKeys",
			"errorExportKeys",
		} {
			if _, exists := payload[forbiddenKey]; exists {
				t.Fatalf("expected stale sentinel to omit %q, got %#v", forbiddenKey, payload)
			}
		}
	})

	t.Run("WRC-JSON-003_WIRE-JSON-003_index_alignment_is_preserved", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths([]wirePathDef{
				{Pattern: "", SrcPath: "src/json3/root.tsx", OutPath: "routes/json3_root.js", ExportKey: "Root"},
				{Pattern: "/json3/users", SrcPath: "src/json3/users.tsx", OutPath: "routes/json3_users.js", ExportKey: "Users"},
				{Pattern: "/json3/users/:id", SrcPath: "src/json3/user_id.tsx", OutPath: "routes/json3_user_id.js", ExportKey: "UserID", ErrorExportKey: "UserError"},
			}, nil),
			nil,
		)
		vorma.NewLoader(
			app,
			"",
			func(*vorma.LoaderReqData) (string, error) { return "root-data", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewLoader(
			app,
			"/json3/users/:id",
			func(*vorma.LoaderReqData) (string, error) { return "leaf-data", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/json3/users/42?vorma_json="+wireBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		payload := mustJSONMap(t, rec.Body.Bytes())
		matchedPatterns := toStringSlice(t, payload["matchedPatterns"], "matchedPatterns")
		loadersData := toAnySlice(t, payload["loadersData"], "loadersData")
		importURLs := toStringSlice(t, payload["importURLs"], "importURLs")
		exportKeys := toStringSlice(t, payload["exportKeys"], "exportKeys")
		errorExportKeys := toStringSlice(t, payload["errorExportKeys"], "errorExportKeys")

		n := len(matchedPatterns)
		if len(loadersData) != n || len(importURLs) != n || len(exportKeys) != n || len(errorExportKeys) != n {
			t.Fatalf(
				"expected aligned lengths: matchedPatterns=%d loadersData=%d importURLs=%d exportKeys=%d errorExportKeys=%d",
				n, len(loadersData), len(importURLs), len(exportKeys), len(errorExportKeys),
			)
		}

		wantPatterns := []string{"", "/json3/users", "/json3/users/:id"}
		if !reflect.DeepEqual(matchedPatterns, wantPatterns) {
			t.Fatalf("expected matchedPatterns=%v, got %v", wantPatterns, matchedPatterns)
		}
	})

	t.Run("WRC-JSON-004_WIRE-JSON-004_outermost_error_truncates_arrays_and_sets_error_fields", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths([]wirePathDef{
				{Pattern: "", SrcPath: "src/json4/root.tsx", OutPath: "routes/json4_root.js", ExportKey: "Root"},
				{Pattern: "/json4/child", SrcPath: "src/json4/child.tsx", OutPath: "routes/json4_child.js", ExportKey: "Child", ErrorExportKey: "ChildErr"},
				{Pattern: "/json4/child/leaf", SrcPath: "src/json4/leaf.tsx", OutPath: "routes/json4_leaf.js", ExportKey: "Leaf", ErrorExportKey: "LeafErr"},
			}, nil),
			nil,
		)
		vorma.NewLoader(
			app,
			"",
			func(*vorma.LoaderReqData) (string, error) { return "root-data", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewLoader(
			app,
			"/json4/child",
			func(*vorma.LoaderReqData) (string, error) { return "", errors.New("child failed") },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		vorma.NewLoader(
			app,
			"/json4/child/leaf",
			func(*vorma.LoaderReqData) (string, error) { return "leaf-data", nil },
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/json4/child/leaf?vorma_json="+wireBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		payload := mustJSONMap(t, rec.Body.Bytes())

		if got, _ := payload["outermostServerError"].(string); got == "" {
			t.Fatalf("expected outermostServerError to be non-empty, got %#v", payload["outermostServerError"])
		}
		if got, ok := payload["outermostServerErrorIdx"].(float64); !ok || int(got) != 1 {
			t.Fatalf("expected outermostServerErrorIdx=1, got %#v", payload["outermostServerErrorIdx"])
		}

		for _, key := range []string{
			"matchedPatterns",
			"loadersData",
			"importURLs",
			"exportKeys",
			"errorExportKeys",
		} {
			v := toAnySlice(t, payload[key], key)
			if len(v) != 2 {
				t.Fatalf("expected %s to be truncated to len=2 at outermost error index, got len=%d val=%#v", key, len(v), v)
			}
		}
	})

	t.Run("WRC-JSON-005_WIRE-JSON-001_head_elements_follow_headel_field_contract", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		app := makeWireApp(
			t,
			makeWireConfigJSON(t, nil, nil),
			makeWireStaticFSWithPaths([]wirePathDef{
				{Pattern: "/json5/head", SrcPath: "src/json5/head.tsx", OutPath: "routes/json5_head.js", ExportKey: "Head"},
			}, nil),
			nil,
		)
		vorma.NewLoader(
			app,
			"/json5/head",
			func(rd *vorma.LoaderReqData) (string, error) {
				h := rd.ResponseProxy().GetHeadEls()
				h.Title("Wire JSON Title")
				h.MetaNameContent("description", "wire-json-desc")
				h.Link(h.Rel("preload"), h.Href("/assets/app.js"), h.As("script"), h.SelfClosing())
				return "ok", nil
			},
			func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/json5/head?vorma_json="+wireBuildID, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}
		payload := mustJSONMap(t, rec.Body.Bytes())

		validateHeadElContractSlice(t, payload, "metaHeadEls")
		validateHeadElContractSlice(t, payload, "restHeadEls")
	})
}

func toAnySlice(t *testing.T, v any, field string) []any {
	t.Helper()
	out, ok := v.([]any)
	if !ok {
		t.Fatalf("expected %s as []any, got %#v", field, v)
	}
	return out
}

func toStringSlice(t *testing.T, v any, field string) []string {
	t.Helper()
	raw := toAnySlice(t, v, field)
	out := make([]string, len(raw))
	for i, item := range raw {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("expected %s[%d] to be string, got %#v", field, i, item)
		}
		out[i] = s
	}
	return out
}

func validateHeadElContractSlice(t *testing.T, payload map[string]any, field string) {
	t.Helper()
	rawSlice, exists := payload[field]
	if !exists {
		return
	}

	slice, ok := rawSlice.([]any)
	if !ok {
		t.Fatalf("expected %s as []any, got %#v", field, rawSlice)
	}

	allowedKeys := map[string]bool{
		"tag":                 true,
		"attributes":          true,
		"attributesKnownSafe": true,
		"booleanAttributes":   true,
		"textContent":         true,
		"dangerousInnerHTML":  true,
	}

	for i, item := range slice {
		el, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("expected %s[%d] as object, got %#v", field, i, item)
		}

		for key := range el {
			if !allowedKeys[key] {
				t.Fatalf("unexpected %s[%d] key %q in HeadEl contract, value=%#v", field, i, key, el[key])
			}
		}

		if attrs, has := el["attributes"]; has {
			if _, ok := attrs.(map[string]any); !ok {
				t.Fatalf("expected %s[%d].attributes as object, got %#v", field, i, attrs)
			}
		}
		if attrs, has := el["attributesKnownSafe"]; has {
			if _, ok := attrs.(map[string]any); !ok {
				t.Fatalf("expected %s[%d].attributesKnownSafe as object, got %#v", field, i, attrs)
			}
		}
		if boolAttrs, has := el["booleanAttributes"]; has {
			if _, ok := boolAttrs.([]any); !ok {
				t.Fatalf("expected %s[%d].booleanAttributes as []any, got %#v", field, i, boolAttrs)
			}
		}
	}
}
