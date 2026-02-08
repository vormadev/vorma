package backend_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vormadev/vorma"
)

func TestBackendActionsConformance(t *testing.T) {
	t.Run("BRC-ACT-001_BR-ACT-001_get_actions_parse_query_input", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		type input struct {
			Q    string `json:"q"`
			Page int    `json:"page"`
		}

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFS(true, true))
		vorma.NewAction[input, input](
			app,
			http.MethodGet,
			"/act1/search",
			func(rd *vorma.ActionReqData[input]) (input, error) { return rd.Input(), nil },
			func(rd *vorma.ActionReqData[input]) *vorma.ActionReqData[input] { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/act1/search?q=vorma&page=3", nil)
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		var out input
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("expected JSON action output, got err=%v body=%q", err, rec.Body.String())
		}
		if out.Q != "vorma" || out.Page != 3 {
			t.Fatalf("expected parsed input {Q:%q Page:%d}, got %+v", "vorma", 3, out)
		}
	})

	t.Run("BRC-ACT-002_BR-ACT-002_non_get_actions_parse_json_body", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		type input struct {
			Name string `json:"name"`
		}

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFS(true, true))
		vorma.NewAction[input, input](
			app,
			http.MethodPost,
			"/act2/create",
			func(rd *vorma.ActionReqData[input]) (input, error) { return rd.Input(), nil },
			func(rd *vorma.ActionReqData[input]) *vorma.ActionReqData[input] { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/act2/create", bytes.NewBufferString(`{"name":"alice"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		var out input
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("expected JSON action output, got err=%v body=%q", err, rec.Body.String())
		}
		if out.Name != "alice" {
			t.Fatalf("expected parsed JSON name %q, got %+v", "alice", out)
		}
	})

	t.Run("BRC-ACT-003_BR-ACT-003_form_content_type_does_not_json_decode_input", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		type input struct {
			Name string `json:"name"`
		}

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFS(true, true))
		vorma.NewAction[input, input](
			app,
			http.MethodPost,
			"/act3/form",
			func(rd *vorma.ActionReqData[input]) (input, error) { return rd.Input(), nil },
			func(rd *vorma.ActionReqData[input]) *vorma.ActionReqData[input] { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/act3/form", bytes.NewBufferString("name=bob"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d; body=%q", http.StatusOK, rec.Code, rec.Body.String())
		}

		var out input
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("expected JSON action output, got err=%v body=%q", err, rec.Body.String())
		}
		if out.Name != "" {
			t.Fatalf("expected zero-value input for form content-type default parse path, got %+v", out)
		}
	})

	t.Run("BRC-ACT-004_BR-ACT-004_validation_or_parse_errors_map_to_400", func(t *testing.T) {
		t.Setenv("WAVE_MODE", "")

		type input struct {
			Name string `json:"name"`
		}

		app := makeApp(t, makeWaveConfigJSON(t, nil), makeStaticFS(true, true))
		vorma.NewAction[input, input](
			app,
			http.MethodPost,
			"/act4/validate",
			func(rd *vorma.ActionReqData[input]) (input, error) { return rd.Input(), nil },
			func(rd *vorma.ActionReqData[input]) *vorma.ActionReqData[input] { return rd },
		)
		r := app.InitWithDefaultRouter()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/act4/validate", bytes.NewBufferString(`{"name":`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d for validation-class parse error, got %d; body=%q", http.StatusBadRequest, rec.Code, rec.Body.String())
		}
	})
}
