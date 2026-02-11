package vormaruntime

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/mux"
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
			ClientRouteDefsFile:  "frontend/src/vorma.routes.ts",
			TSGenOutDir:          "frontend/src/vorma.gen",
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
			name: "ClientRouteDefsFile",
			mutate: func(c *VormaConfig) {
				c.ClientRouteDefsFile = ""
			},
			wantMsg: "Vorma.ClientRouteDefsFile is required",
		},
		{
			name: "TSGenOutDir",
			mutate: func(c *VormaConfig) {
				c.TSGenOutDir = ""
			},
			wantMsg: "Vorma.TSGenOutDir is required",
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
				WaveConfigJSON: cfgBytes,
				DistStaticFS:   os.DirFS(staticDir),
				Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
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
			ClientRouteDefsFile:  "frontend/src/vorma.routes.ts",
			TSGenOutDir:          "frontend/src/vorma.gen",
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	w := wave.New(wave.Config{
		WaveConfigJSON: cfgBytes,
		DistStaticFS:   os.DirFS(staticDir),
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	app := NewVormaApp(VormaAppConfig{Wave: w})
	if app.Config.BuildtimePublicURLFuncName != "waveBuildtimeURL" {
		t.Fatalf("expected default buildtime URL function name, got %q", app.Config.BuildtimePublicURLFuncName)
	}
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
