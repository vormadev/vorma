package vormaruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/wave"
)

func TestNewVormaApp_RequiredConfigValidation(t *testing.T) {
	rootDir := t.TempDir()
	staticDir := filepath.Join(rootDir, "dist", "static")
	mustMkdirAll(t, staticDir)

	baseCfg := struct {
		Core  wave.CoreConfig `json:"Core"`
		Vorma VormaConfig     `json:"Vorma"`
	}{
		Core: wave.CoreConfig{
			MainAppEntry: "backend/cmd/serve",
			DistDir:      filepath.Join(rootDir, "dist"),
			StaticAssetDirs: wave.StaticAssetDirs{
				Private: "backend/assets",
				Public:  "frontend/assets",
			},
			PublicPathPrefix: "/",
		},
		Vorma: VormaConfig{
			MainBuildEntry:       "backend/cmd/build",
			UIVariant:            string(UIVariants.React),
			HTMLTemplateLocation: "entry.go.html",
			ClientEntry:          "frontend/src/vorma.entry.tsx",
			ClientRouteDefinitionPatterns: []string{
				"frontend/src/**/*vorma.routes.ts",
			},
			TSGenOutDir: "frontend/src/vorma.gen",
		},
	}

	tests := []struct {
		name    string
		mutate  func(*VormaConfig)
		wantMsg string
	}{
		{
			name: "MainBuildEntry",
			mutate: func(c *VormaConfig) {
				c.MainBuildEntry = ""
			},
			wantMsg: "Vorma.MainBuildEntry is required",
		},
		{
			name: "UIVariant",
			mutate: func(c *VormaConfig) {
				c.UIVariant = ""
			},
			wantMsg: "Vorma.UIVariant is required",
		},
		{
			name: "HTMLTemplateLocation",
			mutate: func(c *VormaConfig) {
				c.HTMLTemplateLocation = ""
			},
			wantMsg: "Vorma.HTMLTemplateLocation is required",
		},
		{
			name: "ClientEntry",
			mutate: func(c *VormaConfig) {
				c.ClientEntry = ""
			},
			wantMsg: "Vorma.ClientEntry is required",
		},
		{
			name: "ClientRouteDefinitionPatterns",
			mutate: func(c *VormaConfig) {
				c.ClientRouteDefinitionPatterns = nil
			},
			wantMsg: "Vorma.ClientRouteDefinitionPatterns is required",
		},
		{
			name: "ClientRouteDefinitionPatterns_EmptyEntry",
			mutate: func(c *VormaConfig) {
				c.ClientRouteDefinitionPatterns = []string{""}
			},
			wantMsg: "Vorma.ClientRouteDefinitionPatterns cannot contain empty entries",
		},
		{
			name: "ClientRouteDefinitionPatterns_WhitespaceEntry",
			mutate: func(c *VormaConfig) {
				c.ClientRouteDefinitionPatterns = []string{"   "}
			},
			wantMsg: "Vorma.ClientRouteDefinitionPatterns cannot contain empty entries",
		},
		{
			name: "TSGenOutDir",
			mutate: func(c *VormaConfig) {
				c.TSGenOutDir = ""
			},
			wantMsg: "Vorma.TSGenOutDir is required",
		},
		{
			name: "DevReloadRoutesEndpointPath_MissingLeadingSlash",
			mutate: func(c *VormaConfig) {
				c.DevReloadRoutesEndpointPath = "reload-routes"
			},
			wantMsg: "Vorma.DevReloadRoutesEndpointPath must start with '/'",
		},
		{
			name: "DevReloadTemplateEndpointPath_MissingLeadingSlash",
			mutate: func(c *VormaConfig) {
				c.DevReloadTemplateEndpointPath = "reload-template"
			},
			wantMsg: "Vorma.DevReloadTemplateEndpointPath must start with '/'",
		},
		{
			name: "DevReloadEndpoints_MustDiffer",
			mutate: func(c *VormaConfig) {
				c.DevReloadRoutesEndpointPath = "/__same"
				c.DevReloadTemplateEndpointPath = "/__same"
			},
			wantMsg: "Vorma.DevReloadRoutesEndpointPath and Vorma.DevReloadTemplateEndpointPath must differ",
		},
		{
			name: "TemplateDataKeys_MustBeNonEmpty",
			mutate: func(c *VormaConfig) {
				c.TemplateDataKeyHeadElements = "   "
			},
			wantMsg: "Vorma template data keys must be non-empty",
		},
		{
			name: "TemplateDataKeys_MustBeUnique",
			mutate: func(c *VormaConfig) {
				c.TemplateDataKeyHeadElements = "SharedTemplateKey"
				c.TemplateDataKeyBodyScripts = "SharedTemplateKey"
			},
			wantMsg: "Vorma template data keys must be unique",
		},
		{
			name: "ClientRootElementID_Required",
			mutate: func(c *VormaConfig) {
				c.ClientRootElementID = "   "
			},
			wantMsg: "Vorma.ClientRootElementID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseCfg
			tt.mutate(&cfg.Vorma)

			cfgBytes, err := json.Marshal(cfg)
			if err != nil {
				t.Fatalf("marshal config: %v", err)
			}

			w := wave.New(wave.Config{
				ConfigSource: wave.NewStaticConfigSource(cfgBytes),
				DistStaticFS: os.DirFS(staticDir),
				Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
			})

			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("expected panic containing %q", tt.wantMsg)
				}
				got := fmt.Sprint(r)
				if !strings.Contains(got, tt.wantMsg) {
					t.Fatalf("panic %q does not contain %q", got, tt.wantMsg)
				}
			}()
			_ = NewVormaApp(VormaAppConfig{Wave: w})
		})
	}
}

func TestNewVormaApp_DefaultBuildtimePublicURLFuncName(t *testing.T) {
	rootDir := t.TempDir()
	staticDir := filepath.Join(rootDir, "dist", "static")
	mustMkdirAll(t, staticDir)

	cfg := struct {
		Core  wave.CoreConfig `json:"Core"`
		Vorma VormaConfig     `json:"Vorma"`
	}{
		Core: wave.CoreConfig{
			MainAppEntry: "backend/cmd/serve",
			DistDir:      filepath.Join(rootDir, "dist"),
			StaticAssetDirs: wave.StaticAssetDirs{
				Private: "backend/assets",
				Public:  "frontend/assets",
			},
		},
		Vorma: VormaConfig{
			MainBuildEntry:       "backend/cmd/build",
			UIVariant:            string(UIVariants.React),
			HTMLTemplateLocation: "entry.go.html",
			ClientEntry:          "frontend/src/vorma.entry.tsx",
			ClientRouteDefinitionPatterns: []string{
				"frontend/src/**/*vorma.routes.ts",
			},
			TSGenOutDir: "frontend/src/vorma.gen",
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	w := wave.New(wave.Config{
		ConfigSource: wave.NewStaticConfigSource(cfgBytes),
		DistStaticFS: os.DirFS(staticDir),
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	app := NewVormaApp(VormaAppConfig{Wave: w})
	if app.Config.BuildtimePublicURLFuncName != "waveBuildtimeURL" {
		t.Fatalf("expected default buildtime URL function name, got %q", app.Config.BuildtimePublicURLFuncName)
	}
	if app.Config.DevReloadRoutesEndpointPath != DefaultDevReloadRoutesEndpointPath {
		t.Fatalf(
			"expected default routes reload endpoint path %q, got %q",
			DefaultDevReloadRoutesEndpointPath,
			app.Config.DevReloadRoutesEndpointPath,
		)
	}
	if app.Config.DevReloadTemplateEndpointPath != DefaultDevReloadTemplateEndpointPath {
		t.Fatalf(
			"expected default template reload endpoint path %q, got %q",
			DefaultDevReloadTemplateEndpointPath,
			app.Config.DevReloadTemplateEndpointPath,
		)
	}
	if app.Config.TemplateDataKeyHeadElements != DefaultTemplateDataKeyHeadElements {
		t.Fatalf(
			"expected default head-elements template key %q, got %q",
			DefaultTemplateDataKeyHeadElements,
			app.Config.TemplateDataKeyHeadElements,
		)
	}
	if app.Config.TemplateDataKeyBodyScripts != DefaultTemplateDataKeyBodyScripts {
		t.Fatalf(
			"expected default body-scripts template key %q, got %q",
			DefaultTemplateDataKeyBodyScripts,
			app.Config.TemplateDataKeyBodyScripts,
		)
	}
	if app.Config.TemplateDataKeySSRScript != DefaultTemplateDataKeySSRScript {
		t.Fatalf(
			"expected default SSR-script template key %q, got %q",
			DefaultTemplateDataKeySSRScript,
			app.Config.TemplateDataKeySSRScript,
		)
	}
	if app.Config.TemplateDataKeySSRScriptHash != DefaultTemplateDataKeySSRScriptHash {
		t.Fatalf(
			"expected default SSR-script-hash template key %q, got %q",
			DefaultTemplateDataKeySSRScriptHash,
			app.Config.TemplateDataKeySSRScriptHash,
		)
	}
	if app.Config.TemplateDataKeyRootElementID != DefaultTemplateDataKeyRootElementID {
		t.Fatalf(
			"expected default root-element-id template key %q, got %q",
			DefaultTemplateDataKeyRootElementID,
			app.Config.TemplateDataKeyRootElementID,
		)
	}
	if app.Config.ClientRootElementID != DefaultClientRootElementID {
		t.Fatalf(
			"expected default client root element id %q, got %q",
			DefaultClientRootElementID,
			app.Config.ClientRootElementID,
		)
	}
}

func TestNewVormaApp_RequiresWaveInstance(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when Wave is nil")
		}
		if got := fmt.Sprint(r); !strings.Contains(got, "Wave instance is required") {
			t.Fatalf("panic = %q, expected to mention missing Wave instance", got)
		}
	}()

	_ = NewVormaApp(VormaAppConfig{})
}

func TestNewVormaApp_MissingVormaSectionStillTriggersRequiredValidation(t *testing.T) {
	rootDir := t.TempDir()
	staticDir := filepath.Join(rootDir, "dist", "static")
	mustMkdirAll(t, staticDir)

	cfg := struct {
		Core wave.CoreConfig `json:"Core"`
	}{
		Core: wave.CoreConfig{
			MainAppEntry: "backend/cmd/serve",
			DistDir:      filepath.Join(rootDir, "dist"),
			StaticAssetDirs: wave.StaticAssetDirs{
				Private: "backend/assets",
				Public:  "frontend/assets",
			},
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	w := wave.New(wave.Config{
		ConfigSource: wave.NewStaticConfigSource(cfgBytes),
		DistStaticFS: os.DirFS(staticDir),
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when Vorma section is omitted")
		}
		if got := fmt.Sprint(r); !strings.Contains(got, "Vorma.MainBuildEntry is required") {
			t.Fatalf("panic = %q, expected required Vorma config validation error", got)
		}
	}()

	_ = NewVormaApp(VormaAppConfig{Wave: w})
}

func TestVorma_DefaultRouterContracts(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app

	if got := app.Loaders().HandlerMountPattern(); got != "/*" {
		t.Fatalf("loaders mount pattern = %q, want %q", got, "/*")
	}
	if got := app.Actions().HandlerMountPattern(); got != "/api/*" {
		t.Fatalf("actions mount pattern = %q, want %q", got, "/api/*")
	}

	methods := app.Actions().SupportedMethods()
	for _, method := range []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	} {
		if !methods[method] {
			t.Fatalf("expected method %q to be supported", method)
		}
	}
	if len(methods) != 5 {
		t.Fatalf("expected exactly 5 default supported methods, got %d", len(methods))
	}
}

func TestActionsSupportedMethods_NormalizesConfiguredMethodCasing(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{
		actionsRouterOpts: ActionsRouterOptions{
			SupportedMethods: []string{" get ", "PoSt", "PATCH", " ", "", "patch"},
		},
	})
	app := fixture.app

	methods := app.Actions().SupportedMethods()
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPatch} {
		if !methods[method] {
			t.Fatalf("expected normalized method %q to be supported, methods=%v", method, methods)
		}
	}
	for _, method := range []string{" get ", "PoSt"} {
		if methods[method] {
			t.Fatalf("unexpected non-normalized method key %q in supported methods map", method)
		}
	}
	if methods[""] {
		t.Fatal("unexpected empty-string method key in supported methods map")
	}
	if len(methods) != 3 {
		t.Fatalf("supported methods length = %d, want %d", len(methods), 3)
	}
}

func TestVormaServeStatic_PublicAssetAndPassthroughContracts(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{
		publicPathPrefix: "/static/",
	})
	app := fixture.app

	mustWriteFile(
		t,
		filepath.Join(fixture.staticDir, "assets", "public", "hello.txt"),
		[]byte("hello-static"),
	)

	t.Run("existing_public_asset_is_served_and_immutable", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusTeapot)
			w.Write([]byte("next-handler"))
		})
		handler := app.ServeStatic()(next)

		req := httptest.NewRequest(http.MethodGet, "/static/hello.txt", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Body.String(); got != "hello-static" {
			t.Fatalf("body = %q, want %q", got, "hello-static")
		}
		if !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
			t.Fatalf("Cache-Control = %q, expected immutable caching", rec.Header().Get("Cache-Control"))
		}
		if nextCalled {
			t.Fatal("next handler should not be called for existing public asset")
		}
	})

	t.Run("non_static_path_passthroughs_to_next", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusTeapot)
			w.Write([]byte("next-handler"))
		})
		handler := app.ServeStatic()(next)

		req := httptest.NewRequest(http.MethodGet, "/dynamic-route", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusTeapot {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusTeapot)
		}
		if got := rec.Body.String(); got != "next-handler" {
			t.Fatalf("body = %q, want %q", got, "next-handler")
		}
		if !nextCalled {
			t.Fatal("next handler should be called for non-static paths")
		}
	})

	t.Run("missing_static_namespace_path_returns_404_without_next", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusTeapot)
			w.Write([]byte("next-handler"))
		})
		handler := app.ServeStatic()(next)

		req := httptest.NewRequest(http.MethodGet, "/static/missing.txt", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
		if nextCalled {
			t.Fatal("next handler should not be called for static namespace paths")
		}
	})
}

func TestVormaServeStatic_RootPublicPathPrefixContracts(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{
		publicPathPrefix: "/",
	})
	app := fixture.app

	mustWriteFile(
		t,
		filepath.Join(fixture.staticDir, "assets", "public", "asset.txt"),
		[]byte("root-prefix-asset"),
	)

	t.Run("existing_root_level_asset_is_served", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusTeapot)
			w.Write([]byte("next-handler"))
		})
		handler := app.ServeStatic()(next)

		req := httptest.NewRequest(http.MethodGet, "/asset.txt", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Body.String(); got != "root-prefix-asset" {
			t.Fatalf("body = %q, want %q", got, "root-prefix-asset")
		}
		if nextCalled {
			t.Fatal("next handler should not be called for existing root-level asset")
		}
	})

	t.Run("missing_root_level_path_passthroughs_to_next", func(t *testing.T) {
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusTeapot)
			w.Write([]byte("next-handler"))
		})
		handler := app.ServeStatic()(next)

		req := httptest.NewRequest(http.MethodGet, "/dynamic-page", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusTeapot {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusTeapot)
		}
		if got := rec.Body.String(); got != "next-handler" {
			t.Fatalf("body = %q, want %q", got, "next-handler")
		}
		if !nextCalled {
			t.Fatal("next handler should be called when root-level asset is missing")
		}
	})
}

func TestActionsHandler_ParsesInputAndSetsBuildHeader(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app

	type Input struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	mux.RegisterTaskHandler(
		app.ActionsRouter().Router,
		http.MethodGet,
		"/echo",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[Input]) (Input, error) {
			return rd.Input(), nil
		}),
	)
	mux.RegisterTaskHandler(
		app.ActionsRouter().Router,
		http.MethodPost,
		"/echo",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[Input]) (Input, error) {
			return rd.Input(), nil
		}),
	)

	handler := app.Actions().Handler()

	t.Run("GET_UsesQueryParams", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/echo?name=alice&count=3", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get(VormaBuildIDHeaderKey); got != app.GetBuildID() {
			t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, app.GetBuildID())
		}

		var got Input
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if got.Name != "alice" || got.Count != 3 {
			t.Fatalf("got %+v, want {Name:alice Count:3}", got)
		}
	})

	t.Run("POST_UsesJSONBody", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/echo", strings.NewReader(`{"name":"bob","count":7}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var got Input
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if got.Name != "bob" || got.Count != 7 {
			t.Fatalf("got %+v, want {Name:bob Count:7}", got)
		}
	})

	t.Run("POST_InvalidJSON_IsBadRequest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/echo", strings.NewReader("{"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestActionsHandler_ResponseProxyRedirectContracts(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app

	mux.RegisterTaskHandler(
		app.ActionsRouter().Router,
		http.MethodPost,
		"/go",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[struct{}]) (map[string]bool, error) {
			if _, err := rd.ResponseProxy().Redirect(rd.Request(), "/done", http.StatusSeeOther); err != nil {
				return nil, err
			}
			return map[string]bool{"ok": true}, nil
		}),
	)

	handler := app.Actions().Handler()

	t.Run("server_redirect_when_client_header_absent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/go", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
		}
		if got := rec.Header().Get("Location"); got != "/done" {
			t.Fatalf("Location = %q, want %q", got, "/done")
		}
		if strings.Contains(rec.Body.String(), `"ok"`) {
			t.Fatalf("redirect response unexpectedly contained normal action payload: %q", rec.Body.String())
		}
		if got := rec.Header().Get(VormaBuildIDHeaderKey); got != app.GetBuildID() {
			t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, app.GetBuildID())
		}
	})

	t.Run("client_redirect_when_header_present", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/go", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(response.ClientAcceptsRedirectHeader, "true")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get(response.ClientRedirectHeader); got != "/done" {
			t.Fatalf("%s = %q, want %q", response.ClientRedirectHeader, got, "/done")
		}
		if got := rec.Header().Get("Location"); got != "" {
			t.Fatalf("Location = %q, want empty when using client redirect", got)
		}
		if strings.Contains(rec.Body.String(), `"ok"`) {
			t.Fatalf("client redirect response unexpectedly contained normal action payload: %q", rec.Body.String())
		}
		if got := rec.Header().Get(VormaBuildIDHeaderKey); got != app.GetBuildID() {
			t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, app.GetBuildID())
		}
	})
}

func TestActionsHandler_FormContentTypesAndNonJSONFallback(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app

	mux.RegisterTaskHandler(
		app.ActionsRouter().Router,
		http.MethodPost,
		"/submit-form",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[FormData]) (map[string]string, error) {
			return map[string]string{
				"name":  rd.Request().FormValue("name"),
				"count": rd.Request().FormValue("count"),
			}, nil
		}),
	)

	type JSONInput struct {
		Name string `json:"name"`
	}
	mux.RegisterTaskHandler(
		app.ActionsRouter().Router,
		http.MethodPost,
		"/submit-json",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[JSONInput]) (JSONInput, error) {
			return rd.Input(), nil
		}),
	)

	handler := app.Actions().Handler()

	t.Run("POST_URLEncodedForm_ExposedViaRequest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/submit-form", strings.NewReader("name=alice&count=3"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var got map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if got["name"] != "alice" || got["count"] != "3" {
			t.Fatalf("got %#v, want name=alice,count=3", got)
		}
	})

	t.Run("POST_MultipartForm_ExposedViaRequest", func(t *testing.T) {
		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		if err := w.WriteField("name", "bob"); err != nil {
			t.Fatalf("WriteField(name): %v", err)
		}
		if err := w.WriteField("count", "9"); err != nil {
			t.Fatalf("WriteField(count): %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("multipart close: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/api/submit-form", &body)
		req.Header.Set("Content-Type", w.FormDataContentType())
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var got map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if got["name"] != "bob" || got["count"] != "9" {
			t.Fatalf("got %#v, want name=bob,count=9", got)
		}
	})

	t.Run("POST_URLEncodedForm_RejectsTypedJSONInput", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/submit-json", strings.NewReader("name=charlie"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("POST_NonJSONContentType_RejectsTypedJSONInput", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/submit-json", strings.NewReader("name=charlie"))
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestActionsHandler_UnsupportedMethodIsBadRequest(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{
		actionsRouterOpts: ActionsRouterOptions{
			SupportedMethods: []string{http.MethodGet},
		},
	})
	app := fixture.app

	type Input struct {
		Name string `json:"name"`
	}

	mux.RegisterTaskHandler(
		app.ActionsRouter().Router,
		http.MethodPost,
		"/only-get-supported",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[Input]) (Input, error) {
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
	if !strings.Contains(rec.Body.String(), "unsupported method") {
		t.Fatalf("response body = %q, expected to mention unsupported method", rec.Body.String())
	}
}

func TestActionsSupportedMethods_ReturnsDefensiveCopy(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app

	first := app.Actions().SupportedMethods()
	first[http.MethodGet] = false
	first["CUSTOM"] = true

	second := app.Actions().SupportedMethods()
	if !second[http.MethodGet] {
		t.Fatal("mutation of first map leaked into app SupportedMethods state")
	}
	if second["CUSTOM"] {
		t.Fatal("caller-added key leaked into app SupportedMethods state")
	}
}

func TestInitWithDefaultRouter_Integration(t *testing.T) {
	stage := defaultPathsFile("build-router", map[string]*Path{
		"/hello": {
			OriginalPattern: "/hello",
			SrcPath:         "frontend/src/routes/hello.tsx",
			OutPath:         "vorma_out/routes/hello.js",
			ExportKey:       "default",
		},
	})
	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/hello",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
			return map[string]string{"message": "hi"}, nil
		}),
	)
	mux.RegisterTaskHandler(
		app.ActionsRouter().Router,
		http.MethodPost,
		"/echo",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[struct {
			Name string `json:"name"`
		}]) (map[string]string, error) {
			return map[string]string{"name": rd.Input().Name}, nil
		}),
	)

	router := app.InitWithDefaultRouter()

	reqLoader := httptest.NewRequest(http.MethodGet, "/hello?vorma_json="+app.GetBuildID(), nil)
	recLoader := httptest.NewRecorder()
	router.ServeHTTP(recLoader, reqLoader)
	if recLoader.Code != http.StatusOK {
		t.Fatalf("loader status = %d, want %d", recLoader.Code, http.StatusOK)
	}
	var loaderData RouteDataFinal
	if err := json.Unmarshal(recLoader.Body.Bytes(), &loaderData); err != nil {
		t.Fatalf("decode loader response: %v", err)
	}
	if !reflect.DeepEqual(loaderData.MatchedPatterns, []string{"/hello"}) {
		t.Fatalf("loader MatchedPatterns = %#v", loaderData.MatchedPatterns)
	}

	reqAction := httptest.NewRequest(http.MethodPost, "/api/echo", strings.NewReader(`{"name":"sam"}`))
	reqAction.Header.Set("Content-Type", "application/json")
	recAction := httptest.NewRecorder()
	router.ServeHTTP(recAction, reqAction)
	if recAction.Code != http.StatusOK {
		t.Fatalf("action status = %d, want %d", recAction.Code, http.StatusOK)
	}
	var actionData map[string]string
	if err := json.Unmarshal(recAction.Body.Bytes(), &actionData); err != nil {
		t.Fatalf("decode action response: %v", err)
	}
	if actionData["name"] != "sam" {
		t.Fatalf(`actionData["name"] = %q, want %q`, actionData["name"], "sam")
	}
}

func TestInitWithDefaultRouter_RespectsSupportedMethods(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{
		actionsRouterOpts: ActionsRouterOptions{
			SupportedMethods: []string{http.MethodGet},
		},
	})
	app := fixture.app

	type Input struct {
		Name string `json:"name"`
	}

	mux.RegisterTaskHandler(
		app.ActionsRouter().Router,
		http.MethodGet,
		"/echo",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[Input]) (Input, error) {
			return rd.Input(), nil
		}),
	)
	mux.RegisterTaskHandler(
		app.ActionsRouter().Router,
		http.MethodPost,
		"/echo",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[Input]) (Input, error) {
			return rd.Input(), nil
		}),
	)

	router := app.InitWithDefaultRouter()

	reqGet := httptest.NewRequest(http.MethodGet, "/api/echo?name=ok", nil)
	recGet := httptest.NewRecorder()
	router.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", recGet.Code, http.StatusOK)
	}

	reqHead := httptest.NewRequest(http.MethodHead, "/api/echo?name=head-ok", nil)
	recHead := httptest.NewRecorder()
	router.ServeHTTP(recHead, reqHead)
	if recHead.Code != http.StatusOK {
		t.Fatalf("HEAD status = %d, want %d when GET is mounted", recHead.Code, http.StatusOK)
	}
	if recHead.Body.Len() != 0 {
		t.Fatalf("HEAD response should not include body, got %q", recHead.Body.String())
	}

	reqPost := httptest.NewRequest(http.MethodPost, "/api/echo", strings.NewReader(`{"name":"nope"}`))
	reqPost.Header.Set("Content-Type", "application/json")
	recPost := httptest.NewRecorder()
	router.ServeHTTP(recPost, reqPost)
	if recPost.Code != http.StatusNotFound {
		t.Fatalf("POST status = %d, want %d when method not mounted", recPost.Code, http.StatusNotFound)
	}
}

func TestInitWithDefaultRouter_HeadRequiresMountedGetActionHandler(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{
		actionsRouterOpts: ActionsRouterOptions{
			SupportedMethods: []string{http.MethodPost},
		},
	})
	app := fixture.app

	type Input struct {
		Name string `json:"name"`
	}

	mux.RegisterTaskHandler(
		app.ActionsRouter().Router,
		http.MethodPost,
		"/echo",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[Input]) (Input, error) {
			return rd.Input(), nil
		}),
	)

	router := app.InitWithDefaultRouter()

	reqHead := httptest.NewRequest(http.MethodHead, "/api/echo?name=missing", nil)
	recHead := httptest.NewRecorder()
	router.ServeHTTP(recHead, reqHead)

	if recHead.Code != http.StatusNotFound {
		t.Fatalf("HEAD status = %d, want %d when GET is not mounted", recHead.Code, http.StatusNotFound)
	}
}

func TestInitWithDefaultRouter_RespectsCustomActionsMountRoot(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{
		actionsRouterOpts: ActionsRouterOptions{
			MountRoot: "rpc",
		},
	})
	app := fixture.app

	type Input struct {
		Name string `json:"name"`
	}

	mux.RegisterTaskHandler(
		app.ActionsRouter().Router,
		http.MethodGet,
		"/echo",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[Input]) (Input, error) {
			return rd.Input(), nil
		}),
	)

	if got, want := app.Actions().HandlerMountPattern(), "/rpc/*"; got != want {
		t.Fatalf("HandlerMountPattern() = %q, want %q", got, want)
	}

	router := app.InitWithDefaultRouter()

	reqRPC := httptest.NewRequest(http.MethodGet, "/rpc/echo?name=ok", nil)
	recRPC := httptest.NewRecorder()
	router.ServeHTTP(recRPC, reqRPC)
	if recRPC.Code != http.StatusOK {
		t.Fatalf("GET /rpc/echo status = %d, want %d", recRPC.Code, http.StatusOK)
	}

	reqAPI := httptest.NewRequest(http.MethodGet, "/api/echo?name=ok", nil)
	recAPI := httptest.NewRecorder()
	router.ServeHTTP(recAPI, reqAPI)
	if recAPI.Code != http.StatusNotFound {
		t.Fatalf("GET /api/echo status = %d, want %d with custom mount root", recAPI.Code, http.StatusNotFound)
	}
}

func TestInitWithDefaultRouter_SupportedMethodsCasingIsNormalized(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{
		actionsRouterOpts: ActionsRouterOptions{
			SupportedMethods: []string{"get"},
		},
	})
	app := fixture.app

	type Input struct {
		Name string `json:"name"`
	}

	mux.RegisterTaskHandler(
		app.ActionsRouter().Router,
		http.MethodGet,
		"/echo",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[Input]) (Input, error) {
			return rd.Input(), nil
		}),
	)

	router := app.InitWithDefaultRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/echo?name=ok", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d with lowercase configured method", rec.Code, http.StatusOK)
	}
}

func TestInitWithDefaultRouter_ActionsHeadFallsBackToGetWithoutBody(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app

	type Input struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	mux.RegisterTaskHandler(
		app.ActionsRouter().Router,
		http.MethodGet,
		"/echo",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[Input]) (Input, error) {
			return rd.Input(), nil
		}),
	)

	router := app.InitWithDefaultRouter()

	req := httptest.NewRequest(http.MethodHead, "/api/echo?name=head&count=5", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HEAD status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(VormaBuildIDHeaderKey); got != app.GetBuildID() {
		t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, app.GetBuildID())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json prefix", got)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("HEAD response should not include body, got %q", rec.Body.String())
	}
}

func TestInitWithDefaultRouter_LoadersHeadContracts(t *testing.T) {
	stage := defaultPathsFile("build-loaders-head", map[string]*Path{
		"/hello": {
			OriginalPattern: "/hello",
			SrcPath:         "frontend/src/routes/hello.tsx",
			OutPath:         "vorma_out/routes/hello.js",
			ExportKey:       "default",
		},
	})
	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/hello",
		mux.TaskHandlerFromFunc(func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
			return map[string]string{"message": "hi"}, nil
		}),
	)

	router := app.InitWithDefaultRouter()

	t.Run("current_build_json_head_has_headers_and_no_body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/hello?vorma_json="+app.GetBuildID(), nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("HEAD status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get(VormaBuildIDHeaderKey); got != app.GetBuildID() {
			t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, app.GetBuildID())
		}
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
			t.Fatalf("Content-Type = %q, want application/json prefix", got)
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("HEAD response should not include body, got %q", rec.Body.String())
		}
	})

	t.Run("html_head_has_headers_and_no_body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/hello", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("HEAD status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get(VormaBuildIDHeaderKey); got != app.GetBuildID() {
			t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, app.GetBuildID())
		}
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
			t.Fatalf("Content-Type = %q, want text/html prefix", got)
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("HEAD response should not include body, got %q", rec.Body.String())
		}
	})

	t.Run("stale_build_json_head_sets_reload_header_and_no_body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/hello?vorma_json=stale-build&x=1", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("HEAD status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get(VormaBuildIDHeaderKey); got != app.GetBuildID() {
			t.Fatalf("%s = %q, want %q", VormaBuildIDHeaderKey, got, app.GetBuildID())
		}
		if got, want := rec.Header().Get("X-Vorma-Reload"), "/hello?x=1"; got != want {
			t.Fatalf("X-Vorma-Reload = %q, want %q", got, want)
		}
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
			t.Fatalf("Content-Type = %q, want application/json prefix", got)
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("HEAD response should not include body, got %q", rec.Body.String())
		}
	})
}

func TestFormDataTSTypeRaw(t *testing.T) {
	var fd FormData
	if got, want := fd.TSTypeRaw(), "FormData"; got != want {
		t.Fatalf("TSTypeRaw() = %q, want %q", got, want)
	}
}
