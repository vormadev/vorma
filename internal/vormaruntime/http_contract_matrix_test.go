package vormaruntime

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/kit/response"
)

type httpContractCase struct {
	name                  string
	handler               func(app *Vorma) http.Handler
	method                string
	target                func(app *Vorma) string
	body                  string
	contentType           string
	requestHeaders        map[string]string
	wantStatus            int
	wantBuildHeader       bool
	wantReloadHeader      string
	wantLocationHeader    string
	wantClientRedirect    string
	wantNoLocationHeader  bool
	wantNoClientRedirect  bool
	wantHeaders           map[string]string
	wantContentTypePrefix string
	wantJSONKeysPresent   []string
	wantJSONKeysAbsent    []string
	wantBodyContains      string
	wantEmptyBody         bool
}

func TestHTTPContractMatrix_LoadersAndActions(t *testing.T) {
	stage := defaultPathsFile("build-contract-matrix", map[string]*Path{
		"/items/:id": {
			OriginalPattern: "/items/:id",
			SrcPath:         "frontend/src/routes/items.$id.tsx",
			OutPath:         testWaveOutPath("routes/items.$id.js"),
			ExportKey:       "default",
			ErrorExportKey:  "ItemsErrorBoundary",
			Deps:            []string{testWaveOutPath("chunk-items.js")},
		},
		"/error": {
			OriginalPattern: "/error",
			SrcPath:         "frontend/src/routes/error.tsx",
			OutPath:         testWaveOutPath("routes/error.js"),
			ExportKey:       "default",
			ErrorExportKey:  "RouteErrorBoundary",
		},
		"/cache": {
			OriginalPattern: "/cache",
			SrcPath:         "frontend/src/routes/cache.tsx",
			OutPath:         testWaveOutPath("routes/cache.js"),
			ExportKey:       "default",
		},
	})
	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/items/:id",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
				return map[string]string{"id": rd.Params()["id"]}, nil
			},
		),
	)
	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/error",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]bool, error) {
				return nil, errors.New("internal loader failure")
			},
		),
	)
	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/cache",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
				rd.ResponseProxy().
					SetHeader("Cache-Control", "public, max-age=120")
				rd.ResponseProxy().SetHeader("X-Loader-Header", "present")
				return map[string]string{"cache": "custom"}, nil
			},
		),
	)

	type actionInput struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	mux.AddTaskHandler(
		app.ActionsRouter().Router,
		http.MethodGet,
		"/echo",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[actionInput]) (actionInput, error) {
				return rd.Input(), nil
			},
		),
	)
	mux.AddTaskHandler(
		app.ActionsRouter().Router,
		http.MethodPost,
		"/echo",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[actionInput]) (actionInput, error) {
				return rd.Input(), nil
			},
		),
	)
	mux.AddTaskHandler(
		app.ActionsRouter().Router,
		http.MethodPut,
		"/echo",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[actionInput]) (actionInput, error) {
				return rd.Input(), nil
			},
		),
	)
	mux.AddTaskHandler(
		app.ActionsRouter().Router,
		http.MethodPatch,
		"/echo",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[actionInput]) (actionInput, error) {
				return rd.Input(), nil
			},
		),
	)
	mux.AddTaskHandler(
		app.ActionsRouter().Router,
		http.MethodDelete,
		"/echo",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[actionInput]) (actionInput, error) {
				return rd.Input(), nil
			},
		),
	)
	mux.AddTaskHandler(
		app.ActionsRouter().Router,
		http.MethodPost,
		"/submit-form",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[FormData]) (map[string]string, error) {
				return map[string]string{
					"name":  rd.Request().FormValue("name"),
					"count": rd.Request().FormValue("count"),
				}, nil
			},
		),
	)
	mux.AddTaskHandler(
		app.ActionsRouter().Router,
		http.MethodPost,
		"/forbid",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[actionInput]) (map[string]string, error) {
				rd.ResponseProxy().SetStatus(http.StatusForbidden, "blocked")
				return map[string]string{"ignored": "true"}, nil
			},
		),
	)
	mux.AddTaskHandler(
		app.ActionsRouter().Router,
		http.MethodPost,
		"/go",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[actionInput]) (map[string]string, error) {
				if _, err := rd.ResponseProxy().Redirect(rd.Request(), "/go-done", http.StatusSeeOther); err != nil {
					return nil, err
				}
				return map[string]string{"ignored": "true"}, nil
			},
		),
	)

	loadersHandler := func(app *Vorma) http.Handler {
		return mux.InjectTasksCtxMiddleware(app.Loaders().Handler())
	}
	actionsHandler := func(app *Vorma) http.Handler {
		return app.Actions().Handler()
	}

	tests := []httpContractCase{
		{
			name:                  "Loaders_JSON_CurrentBuild_SuccessShape",
			handler:               loadersHandler,
			method:                http.MethodGet,
			target:                func(app *Vorma) string { return "/items/42?" + VormaJSONQueryKey + "=" + app.BuildID() },
			wantStatus:            http.StatusOK,
			wantBuildHeader:       true,
			wantContentTypePrefix: "application/json",
			wantJSONKeysPresent: []string{
				"matchedPatterns", "loadersData", "importURLs", "exportKeys", "deps", "cssBundles",
			},
			wantJSONKeysAbsent: []string{
				"outermostServerError",
				"outermostServerErrorIdx",
			},
		},
		{
			name:                  "Loaders_JSON_StaleBuild_ReloadSignalShape",
			handler:               loadersHandler,
			method:                http.MethodGet,
			target:                func(app *Vorma) string { return "/items/42?" + VormaJSONQueryKey + "=stale-build&foo=bar" },
			wantStatus:            http.StatusOK,
			wantBuildHeader:       true,
			wantReloadHeader:      "/items/42?foo=bar",
			wantContentTypePrefix: "application/json",
			wantJSONKeysPresent:   []string{"ok"},
			wantJSONKeysAbsent: []string{
				"matchedPatterns",
				"loadersData",
				"importURLs",
				"exportKeys",
				"deps",
			},
		},
		{
			name:            "Loaders_JSON_NotFound",
			handler:         loadersHandler,
			method:          http.MethodGet,
			target:          func(app *Vorma) string { return "/missing?" + VormaJSONQueryKey + "=" + app.BuildID() },
			wantStatus:      http.StatusNotFound,
			wantBuildHeader: true,
		},
		{
			name:                  "Loaders_HTML_CurrentBuild",
			handler:               loadersHandler,
			method:                http.MethodGet,
			target:                func(app *Vorma) string { return "/items/42" },
			wantStatus:            http.StatusOK,
			wantBuildHeader:       true,
			wantContentTypePrefix: "text/html",
			wantBodyContains:      "vorma-root",
		},
		{
			name:                  "Loaders_JSON_GenericErrorShape",
			handler:               loadersHandler,
			method:                http.MethodGet,
			target:                func(app *Vorma) string { return "/error?" + VormaJSONQueryKey + "=" + app.BuildID() },
			wantStatus:            http.StatusOK,
			wantBuildHeader:       true,
			wantContentTypePrefix: "application/json",
			wantJSONKeysPresent: []string{
				"outermostServerError", "outermostServerErrorIdx", "matchedPatterns", "loadersData", "importURLs",
			},
			wantBodyContains: "An error occurred",
		},
		{
			name:                  "Loaders_JSON_CustomHeadersPreserved",
			handler:               loadersHandler,
			method:                http.MethodGet,
			target:                func(app *Vorma) string { return "/cache?" + VormaJSONQueryKey + "=" + app.BuildID() },
			wantStatus:            http.StatusOK,
			wantBuildHeader:       true,
			wantContentTypePrefix: "application/json",
			wantHeaders: map[string]string{
				"Cache-Control":   "public, max-age=120",
				"X-Loader-Header": "present",
			},
			wantJSONKeysPresent: []string{
				"matchedPatterns", "loadersData",
			},
		},
		{
			name:                  "Actions_GET_QueryInputAndHeader",
			handler:               actionsHandler,
			method:                http.MethodGet,
			target:                func(app *Vorma) string { return "/api/echo?name=ana&count=2" },
			wantStatus:            http.StatusOK,
			wantBuildHeader:       true,
			wantContentTypePrefix: "application/json",
			wantJSONKeysPresent:   []string{"name", "count"},
		},
		{
			name:                  "Actions_HEAD_QueryInputAndHeader_NoBody",
			handler:               actionsHandler,
			method:                http.MethodHead,
			target:                func(app *Vorma) string { return "/api/echo?name=ana&count=2" },
			wantStatus:            http.StatusOK,
			wantBuildHeader:       true,
			wantContentTypePrefix: "application/json",
			wantEmptyBody:         true,
		},
		{
			name:                  "Actions_POST_JSONInputAndHeader",
			handler:               actionsHandler,
			method:                http.MethodPost,
			target:                func(app *Vorma) string { return "/api/echo" },
			body:                  `{"name":"bob","count":7}`,
			contentType:           "application/json",
			wantStatus:            http.StatusOK,
			wantBuildHeader:       true,
			wantContentTypePrefix: "application/json",
			wantJSONKeysPresent:   []string{"name", "count"},
		},
		{
			name:                  "Actions_PUT_JSONInputAndHeader",
			handler:               actionsHandler,
			method:                http.MethodPut,
			target:                func(app *Vorma) string { return "/api/echo" },
			body:                  `{"name":"put-user","count":9}`,
			contentType:           "application/json",
			wantStatus:            http.StatusOK,
			wantBuildHeader:       true,
			wantContentTypePrefix: "application/json",
			wantJSONKeysPresent:   []string{"name", "count"},
		},
		{
			name:                  "Actions_PATCH_JSONInputAndHeader",
			handler:               actionsHandler,
			method:                http.MethodPatch,
			target:                func(app *Vorma) string { return "/api/echo" },
			body:                  `{"name":"patch-user","count":11}`,
			contentType:           "application/json",
			wantStatus:            http.StatusOK,
			wantBuildHeader:       true,
			wantContentTypePrefix: "application/json",
			wantJSONKeysPresent:   []string{"name", "count"},
		},
		{
			name:                  "Actions_DELETE_JSONInputAndHeader",
			handler:               actionsHandler,
			method:                http.MethodDelete,
			target:                func(app *Vorma) string { return "/api/echo" },
			body:                  `{"name":"delete-user","count":13}`,
			contentType:           "application/json",
			wantStatus:            http.StatusOK,
			wantBuildHeader:       true,
			wantContentTypePrefix: "application/json",
			wantJSONKeysPresent:   []string{"name", "count"},
		},
		{
			name:                  "Actions_POST_FormDataRoute",
			handler:               actionsHandler,
			method:                http.MethodPost,
			target:                func(app *Vorma) string { return "/api/submit-form" },
			body:                  "name=alice&count=3",
			contentType:           "application/x-www-form-urlencoded",
			wantStatus:            http.StatusOK,
			wantBuildHeader:       true,
			wantContentTypePrefix: "application/json",
			wantJSONKeysPresent:   []string{"name", "count"},
		},
		{
			name:             "Actions_POST_FormContentTypeRejectedForTypedInput",
			handler:          actionsHandler,
			method:           http.MethodPost,
			target:           func(app *Vorma) string { return "/api/echo" },
			body:             "name=charlie",
			contentType:      "application/x-www-form-urlencoded",
			wantStatus:       http.StatusBadRequest,
			wantBuildHeader:  true,
			wantBodyContains: "form content type requires vormaruntime.FormData input",
		},
		{
			name:            "Actions_POST_InvalidJSONRejected",
			handler:         actionsHandler,
			method:          http.MethodPost,
			target:          func(app *Vorma) string { return "/api/echo" },
			body:            "{",
			contentType:     "application/json",
			wantStatus:      http.StatusBadRequest,
			wantBuildHeader: true,
		},
		{
			name:             "Actions_POST_ResponseProxyErrorShortCircuit",
			handler:          actionsHandler,
			method:           http.MethodPost,
			target:           func(app *Vorma) string { return "/api/forbid" },
			body:             `{"name":"blocked","count":1}`,
			contentType:      "application/json",
			wantStatus:       http.StatusForbidden,
			wantBuildHeader:  true,
			wantBodyContains: "blocked",
		},
		{
			name:                 "Actions_POST_ResponseProxyServerRedirectShortCircuit",
			handler:              actionsHandler,
			method:               http.MethodPost,
			target:               func(app *Vorma) string { return "/api/go" },
			body:                 `{"name":"ana","count":2}`,
			contentType:          "application/json",
			wantStatus:           http.StatusSeeOther,
			wantBuildHeader:      true,
			wantLocationHeader:   "/go-done",
			wantNoClientRedirect: true,
		},
		{
			name:        "Actions_POST_ResponseProxyClientRedirectShortCircuit",
			handler:     actionsHandler,
			method:      http.MethodPost,
			target:      func(app *Vorma) string { return "/api/go" },
			body:        `{"name":"ana","count":2}`,
			contentType: "application/json",
			requestHeaders: map[string]string{
				response.ClientAcceptsRedirectHeader: "true",
			},
			wantStatus:           http.StatusOK,
			wantBuildHeader:      true,
			wantClientRedirect:   "/go-done",
			wantNoLocationHeader: true,
		},
		{
			name:            "Actions_UnknownPath_StillBuildHeader",
			handler:         actionsHandler,
			method:          http.MethodGet,
			target:          func(app *Vorma) string { return "/api/not-found" },
			wantStatus:      http.StatusNotFound,
			wantBuildHeader: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodyReader *strings.Reader
			if tt.body == "" {
				bodyReader = strings.NewReader("")
			} else {
				bodyReader = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, tt.target(app), bodyReader)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			for key, value := range tt.requestHeaders {
				req.Header.Set(key, value)
			}
			rec := httptest.NewRecorder()

			tt.handler(app).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf(
					"status = %d, want %d (body=%q)",
					rec.Code,
					tt.wantStatus,
					rec.Body.String(),
				)
			}

			buildIDHeader := rec.Header().Get(VormaBuildIDHeaderKey)
			if tt.wantBuildHeader {
				if buildIDHeader != app.BuildID() {
					t.Fatalf(
						"%s = %q, want %q",
						VormaBuildIDHeaderKey,
						buildIDHeader,
						app.BuildID(),
					)
				}
			}

			if tt.wantReloadHeader != "" {
				if got := rec.Header().Get("X-Wave-Framework-Reload"); got != tt.wantReloadHeader {
					t.Fatalf(
						"X-Wave-Framework-Reload = %q, want %q",
						got,
						tt.wantReloadHeader,
					)
				}
			} else {
				if got := rec.Header().Get("X-Wave-Framework-Reload"); got != "" {
					t.Fatalf("X-Wave-Framework-Reload = %q, want empty", got)
				}
			}

			if tt.wantLocationHeader != "" {
				if got := rec.Header().Get("Location"); got != tt.wantLocationHeader {
					t.Fatalf(
						"Location = %q, want %q",
						got,
						tt.wantLocationHeader,
					)
				}
			}
			if tt.wantNoLocationHeader {
				if got := rec.Header().Get("Location"); got != "" {
					t.Fatalf("Location = %q, want empty", got)
				}
			}

			if tt.wantClientRedirect != "" {
				if got := rec.Header().Get(response.ClientRedirectHeader); got != tt.wantClientRedirect {
					t.Fatalf(
						"%s = %q, want %q",
						response.ClientRedirectHeader,
						got,
						tt.wantClientRedirect,
					)
				}
			}
			if tt.wantNoClientRedirect {
				if got := rec.Header().Get(response.ClientRedirectHeader); got != "" {
					t.Fatalf(
						"%s = %q, want empty",
						response.ClientRedirectHeader,
						got,
					)
				}
			}

			if tt.wantContentTypePrefix != "" {
				got := rec.Header().Get("Content-Type")
				if !strings.HasPrefix(got, tt.wantContentTypePrefix) {
					t.Fatalf(
						"Content-Type = %q, want prefix %q",
						got,
						tt.wantContentTypePrefix,
					)
				}
			}
			for key, wantValue := range tt.wantHeaders {
				if got := rec.Header().Get(key); got != wantValue {
					t.Fatalf("%s = %q, want %q", key, got, wantValue)
				}
			}

			if tt.wantBodyContains != "" &&
				!strings.Contains(rec.Body.String(), tt.wantBodyContains) {
				t.Fatalf(
					"body %q missing expected substring %q",
					rec.Body.String(),
					tt.wantBodyContains,
				)
			}
			if tt.wantEmptyBody && rec.Body.Len() != 0 {
				t.Fatalf("expected empty body, got %q", rec.Body.String())
			}

			if len(tt.wantJSONKeysPresent) > 0 ||
				len(tt.wantJSONKeysAbsent) > 0 {
				keys := decodeTopLevelJSONKeys(t, rec.Body.Bytes())

				for _, k := range tt.wantJSONKeysPresent {
					if _, ok := keys[k]; !ok {
						t.Fatalf(
							"expected JSON key %q to be present; keys=%v",
							k,
							mapKeys(keys),
						)
					}
				}
				for _, k := range tt.wantJSONKeysAbsent {
					if _, ok := keys[k]; ok {
						t.Fatalf(
							"expected JSON key %q to be absent; keys=%v",
							k,
							mapKeys(keys),
						)
					}
				}
			}
		})
	}
}

func TestHTTPContractMatrix_ActionsUnsupportedMethodValidation(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{
		actionsRouterOpts: ActionsRouterOptions{
			SupportedMethods: []string{http.MethodGet},
		},
	})
	app := fixture.app

	type input struct {
		Name string `json:"name"`
	}
	mux.AddTaskHandler(
		app.ActionsRouter().Router,
		http.MethodPost,
		"/only-get-supported",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[input]) (input, error) {
			return rd.Input(), nil
		}),
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/only-get-supported",
		strings.NewReader(`{"name":"alice"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.Actions().Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := rec.Header().Get(VormaBuildIDHeaderKey); got != app.BuildID() {
		t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, app.BuildID())
	}
	if !strings.Contains(rec.Body.String(), "unsupported method") {
		t.Fatalf(
			"body %q missing expected substring %q",
			rec.Body.String(),
			"unsupported method",
		)
	}
}

func decodeTopLevelJSONKeys(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("decode top-level JSON keys: %v; body=%q", err, string(body))
	}
	return v
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
