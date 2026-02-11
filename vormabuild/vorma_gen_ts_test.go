package vormabuild

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/vormaruntime"
)

func TestExtractDynamicParamsFromPattern(t *testing.T) {
	params := extractDynamicParamsFromPattern("/teams/:teamID/users/:userID", ':')
	if !slices.Equal(params, []string{"teamID", "userID"}) {
		t.Fatalf("params = %#v, want %#v", params, []string{"teamID", "userID"})
	}

	none := extractDynamicParamsFromPattern("/static/path", ':')
	if len(none) != 0 {
		t.Fatalf("expected no params, got %#v", none)
	}
}

func TestIsSplat(t *testing.T) {
	if !isSplat("/files/*", '*') {
		t.Fatal("expected /files/* to be splat")
	}
	if isSplat("/files/:id", '*') {
		t.Fatal("expected /files/:id not to be splat")
	}
}

func TestGetEntrypoints_ReturnsStableSortedUniqueList(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetPaths(map[string]*vormaruntime.Path{
			"/a": {
				OriginalPattern: "/a",
				SrcPath:         "frontend/src/routes/z.tsx",
				ExportKey:       "default",
			},
			"/b": {
				OriginalPattern: "/b",
				SrcPath:         "frontend/src/routes/a.tsx",
				ExportKey:       "default",
			},
			"/c": {
				OriginalPattern: "/c",
				SrcPath:         "frontend/src/routes/a.tsx",
				ExportKey:       "default",
			},
			"/server-only": {
				OriginalPattern: "/server-only",
				SrcPath:         "",
				ExportKey:       "default",
			},
		})

		got := getEntrypoints(l)
		want := []string{
			"frontend/src/routes/a.tsx",
			"frontend/src/routes/z.tsx",
			"frontend/src/vorma.entry.tsx",
		}
		if !slices.Equal(got, want) {
			t.Fatalf("getEntrypoints() = %#v, want %#v", got, want)
		}
	})
}

func TestGenerateRollupOptions_ContainsExpectedConfig(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		content, err := generateRollupOptions(
			l,
			[]string{"frontend/src/vorma.entry.tsx", "frontend/src/routes/home.tsx"},
		)
		if err != nil {
			t.Fatalf("generateRollupOptions returned error: %v", err)
		}

		if !strings.Contains(content, `buildtimePublicURLFuncName: "waveBuildtimeURL"`) {
			t.Fatalf("rollup options missing buildtime function name:\n%s", content)
		}
		if !strings.Contains(content, `filemapJSONPath: "frontend/src/vorma.gen/filemap.json"`) {
			t.Fatalf("rollup options missing filemap path:\n%s", content)
		}
		if !strings.Contains(content, `"react"`) || !strings.Contains(content, `"react-dom"`) {
			t.Fatalf("rollup options missing react dedupe list:\n%s", content)
		}
		if !strings.Contains(content, app.Config.ClientRouteDefsFile) {
			t.Fatalf("rollup options missing client route defs file in ignored patterns:\n%s", content)
		}
		if !strings.Contains(content, app.Config.TSGenOutDir+"/**/*") {
			t.Fatalf("rollup options missing TS output dir in ignored patterns:\n%s", content)
		}
	})
}

func TestGenerateTypeScript_CoversLoadersClientOnlyQueryAndMutation(t *testing.T) {
	loadersRouter := mux.NewNestedRouter(&mux.NestedOptions{
		ExplicitIndexSegment: "_index",
	})
	actionsRouter := mux.NewRouter(&mux.Options{MountRoot: "/api/"})

	type RootData struct {
		Message string `json:"message"`
	}
	type LoaderData struct {
		ID string `json:"id"`
	}
	type QueryInput struct {
		ID string `json:"id"`
	}
	type QueryOutput struct {
		Name string `json:"name"`
	}
	type MutationInput struct {
		Name string `json:"name"`
	}
	type MutationOutput struct {
		OK bool `json:"ok"`
	}

	mux.RegisterNestedTaskHandler(
		loadersRouter,
		"/",
		mux.TaskHandlerFromFunc(func(_ *mux.ReqData[mux.None]) (RootData, error) {
			return RootData{}, nil
		}),
	)
	mux.RegisterNestedTaskHandler(
		loadersRouter,
		"/users/:id",
		mux.TaskHandlerFromFunc(func(_ *mux.ReqData[mux.None]) (LoaderData, error) {
			return LoaderData{}, nil
		}),
	)

	mux.RegisterTaskHandler(
		actionsRouter,
		http.MethodGet,
		"/users/:id",
		mux.TaskHandlerFromFunc(func(_ *mux.ReqData[QueryInput]) (QueryOutput, error) {
			return QueryOutput{}, nil
		}),
	)
	mux.RegisterTaskHandler(
		actionsRouter,
		http.MethodPut,
		"/users/:id",
		mux.TaskHandlerFromFunc(func(_ *mux.ReqData[MutationInput]) (MutationOutput, error) {
			return MutationOutput{}, nil
		}),
	)
	mux.RegisterTaskHandler(
		actionsRouter,
		http.MethodOptions,
		"/users/:id",
		mux.TaskHandlerFromFunc(func(_ *mux.ReqData[mux.None]) (mux.None, error) {
			return mux.None{}, nil
		}),
	)

	content, err := generateTypeScript(TSGenInput{
		LoadersRouter: loadersRouter,
		ActionsRouter: actionsRouter,
		Paths: map[string]*vormaruntime.Path{
			"/client-only/:slug": {
				OriginalPattern: "/client-only/:slug",
				SrcPath:         "frontend/src/routes/client-only.tsx",
				ExportKey:       "default",
			},
		},
		Config: &vormaruntime.VormaConfig{
			UIVariant: string(vormaruntime.UIVariants.React),
		},
	})
	if err != nil {
		t.Fatalf("generateTypeScript returned error: %v", err)
	}

	if !strings.Contains(content, `type VormaRootData = Extract`) {
		t.Fatalf("expected root data type extraction in generated output:\n%s", content)
	}
	if !strings.Contains(content, `isRootData: true`) {
		t.Fatalf("expected root loader to be marked as root data:\n%s", content)
	}
	if !strings.Contains(content, `pattern: "/users/:id"`) {
		t.Fatalf("expected users route pattern in output:\n%s", content)
	}
	if !strings.Contains(content, `_type: "query"`) {
		t.Fatalf("expected query category in output:\n%s", content)
	}
	if !strings.Contains(content, `_type: "mutation"`) {
		t.Fatalf("expected mutation category in output:\n%s", content)
	}
	if !strings.Contains(content, `method: "PUT"`) {
		t.Fatalf("expected non-POST mutation method to be emitted:\n%s", content)
	}
	if strings.Contains(content, `method: "OPTIONS"`) {
		t.Fatalf("did not expect unsupported action methods in output:\n%s", content)
	}
	if !strings.Contains(content, `pattern: "/client-only/:slug"`) {
		t.Fatalf("expected client-only route in output:\n%s", content)
	}
	if !strings.Contains(content, `actionsRouterMountRoot: "/api/"`) {
		t.Fatalf("expected actions mount root config in output:\n%s", content)
	}
	if !strings.Contains(content, `import type { VormaRouteProps } from "vorma/react";`) {
		t.Fatalf("expected UI variant import path in output:\n%s", content)
	}
}
