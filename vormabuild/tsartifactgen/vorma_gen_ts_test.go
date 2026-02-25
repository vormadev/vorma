package tsartifactgen

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
)

func TestExtractDynamicParamsFromPattern(t *testing.T) {
	params := extractDynamicParamsFromPattern(
		"/teams/:teamID/users/:userID",
		':',
	)
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
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

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
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		content, err := generateRollupOptions(
			l,
			[]string{
				"frontend/src/vorma.entry.tsx",
				"frontend/src/routes/home.tsx",
			},
		)
		if err != nil {
			t.Fatalf("generateRollupOptions returned error: %v", err)
		}

		if !strings.Contains(
			content,
			`buildtimePublicURLFuncName: "waveBuildtimeURL"`,
		) {
			t.Fatalf(
				"rollup options missing buildtime function name:\n%s",
				content,
			)
		}
		if !strings.Contains(
			content,
			`filemapJSONPath: "frontend/src/vorma.gen/filemap.json"`,
		) {
			t.Fatalf("rollup options missing filemap path:\n%s", content)
		}
		if !strings.Contains(content, `"react"`) ||
			!strings.Contains(content, `"react-dom"`) {
			t.Fatalf("rollup options missing react dedupe list:\n%s", content)
		}
		for _, routeDefinitionPattern := range app.Config.ClientRouteDefinitionPatterns {
			if strings.Contains(content, routeDefinitionPattern) {
				continue
			}
			t.Fatalf(
				"rollup options missing client route defs pattern in ignored patterns:\n%s",
				content,
			)
		}
		if !strings.Contains(content, app.Config.TSGenOutDir+"/**/*") {
			t.Fatalf(
				"rollup options missing TS output dir in ignored patterns:\n%s",
				content,
			)
		}
	})
}

func TestGenerateTypeScript_CoversLoadersClientOnlyQueryAndMutation(
	t *testing.T,
) {
	loadersRouter := nestedmux.NewRouter(&nestedmux.Options{
		ExplicitIndexSegmentIdentifier: "_index",
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

	nestedmux.AddTaskHandler(
		loadersRouter,
		"/",
		mux.TaskHandlerFromFunc(
			func(_ *mux.ReqData[mux.None]) (RootData, error) {
				return RootData{}, nil
			},
		),
	)
	nestedmux.AddTaskHandler(
		loadersRouter,
		"/users/:id",
		mux.TaskHandlerFromFunc(
			func(_ *mux.ReqData[mux.None]) (LoaderData, error) {
				return LoaderData{}, nil
			},
		),
	)

	mux.AddTaskHandler(
		actionsRouter,
		http.MethodGet,
		"/users/:id",
		mux.TaskHandlerFromFunc(
			func(_ *mux.ReqData[QueryInput]) (QueryOutput, error) {
				return QueryOutput{}, nil
			},
		),
	)
	mux.AddTaskHandler(
		actionsRouter,
		http.MethodPut,
		"/users/:id",
		mux.TaskHandlerFromFunc(
			func(_ *mux.ReqData[MutationInput]) (MutationOutput, error) {
				return MutationOutput{}, nil
			},
		),
	)
	mux.AddTaskHandler(
		actionsRouter,
		http.MethodOptions,
		"/users/:id",
		mux.TaskHandlerFromFunc(
			func(_ *mux.ReqData[mux.None]) (mux.None, error) {
				return mux.None{}, nil
			},
		),
	)

	content, err := generateTypeScript(tsGenInput{
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
			UIVariant: string(vormaruntime.UIVariantReact),
		},
	})
	if err != nil {
		t.Fatalf("generateTypeScript returned error: %v", err)
	}

	if !strings.Contains(content, `type VormaRootData = Extract`) {
		t.Fatalf(
			"expected root data type extraction in generated output:\n%s",
			content,
		)
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
		t.Fatalf(
			"expected non-POST mutation method to be emitted:\n%s",
			content,
		)
	}
	if strings.Contains(content, `method: "OPTIONS"`) {
		t.Fatalf(
			"did not expect unsupported action methods in output:\n%s",
			content,
		)
	}
	if !strings.Contains(content, `pattern: "/client-only/:slug"`) {
		t.Fatalf("expected client-only route in output:\n%s", content)
	}
	if !strings.Contains(content, `actionsRouterMountRoot: "/api/"`) {
		t.Fatalf("expected actions mount root config in output:\n%s", content)
	}
	if !strings.Contains(
		content,
		`import type { VormaRouteProps } from "vorma/react";`,
	) {
		t.Fatalf("expected UI variant import path in output:\n%s", content)
	}
}

func TestGenerateTypeScript_ClientOnlyLoaderMetadataUsesLoaderRunes(
	t *testing.T,
) {
	loadersRouter := nestedmux.NewRouter(&nestedmux.Options{
		DynamicParamPrefix:     '@',
		SplatSegmentIdentifier: '#',
	})
	actionsRouter := mux.NewRouter(&mux.Options{
		MountRoot:              "/api/",
		DynamicParamPrefix:     ':',
		SplatSegmentIdentifier: '*',
	})

	content, err := generateTypeScript(tsGenInput{
		LoadersRouter: loadersRouter,
		ActionsRouter: actionsRouter,
		Paths: map[string]*vormaruntime.Path{
			"/client-only/@slug": {
				OriginalPattern: "/client-only/@slug",
				SrcPath:         "frontend/src/routes/client-only.tsx",
				ExportKey:       "default",
			},
			"/client-only/#": {
				OriginalPattern: "/client-only/#",
				SrcPath:         "frontend/src/routes/catch-all.tsx",
				ExportKey:       "default",
			},
		},
		Config: &vormaruntime.VormaConfig{
			UIVariant: string(vormaruntime.UIVariantReact),
		},
	})
	if err != nil {
		t.Fatalf("generateTypeScript returned error: %v", err)
	}

	if !strings.Contains(content, `pattern: "/client-only/@slug"`) {
		t.Fatalf(
			"expected client-only dynamic route pattern in output:\n%s",
			content,
		)
	}
	if !strings.Contains(content, `params: ["slug"]`) {
		t.Fatalf(
			"expected loader dynamic rune metadata params for client-only route:\n%s",
			content,
		)
	}
	if !strings.Contains(content, `pattern: "/client-only/#"`) {
		t.Fatalf(
			"expected client-only splat route pattern in output:\n%s",
			content,
		)
	}
	if !strings.Contains(content, `isSplat: true`) {
		t.Fatalf(
			"expected loader splat rune metadata for client-only route:\n%s",
			content,
		)
	}
}

func TestDedupeListForUIVariant(t *testing.T) {
	if got := dedupeListForUIVariant(string(vormaruntime.UIVariantReact)); !slices.Equal(
		got,
		reactDedupeList,
	) {
		t.Fatalf("react dedupe list = %#v, want %#v", got, reactDedupeList)
	}
	if got := dedupeListForUIVariant(string(vormaruntime.UIVariantPreact)); !slices.Equal(
		got,
		preactDedupeList,
	) {
		t.Fatalf("preact dedupe list = %#v, want %#v", got, preactDedupeList)
	}
	if got := dedupeListForUIVariant(string(vormaruntime.UIVariantSolid)); !slices.Equal(
		got,
		solidDedupeList,
	) {
		t.Fatalf("solid dedupe list = %#v, want %#v", got, solidDedupeList)
	}
	if got := dedupeListForUIVariant("unknown"); got != nil {
		t.Fatalf("unknown variant dedupe list = %#v, want nil", got)
	}
}

func TestBuildVitePluginTemplateData(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	entrypoints := []string{
		"frontend/src/vorma.entry.tsx",
		"frontend/src/routes/home.tsx",
	}
	data := buildVitePluginTemplateData(app, entrypoints)

	if !slices.Equal(data.Entrypoints, entrypoints) {
		t.Fatalf("Entrypoints = %#v, want %#v", data.Entrypoints, entrypoints)
	}
	if data.PublicPathPrefix != app.Wave.PublicPathPrefix() {
		t.Fatalf(
			"PublicPathPrefix = %q, want %q",
			data.PublicPathPrefix,
			app.Wave.PublicPathPrefix(),
		)
	}
	if data.FuncName != app.Config.BuildtimePublicURLFuncName {
		t.Fatalf(
			"FuncName = %q, want %q",
			data.FuncName,
			app.Config.BuildtimePublicURLFuncName,
		)
	}
	if data.FilemapJSONPath != "frontend/src/vorma.gen/filemap.json" {
		t.Fatalf(
			"FilemapJSONPath = %q, want %q",
			data.FilemapJSONPath,
			"frontend/src/vorma.gen/filemap.json",
		)
	}
	if !slices.Equal(data.DedupeList, reactDedupeList) {
		t.Fatalf("DedupeList = %#v, want %#v", data.DedupeList, reactDedupeList)
	}
	if len(data.IgnoredPatterns) == 0 {
		t.Fatal("expected non-empty ignored patterns")
	}
}

func TestBuildViteIgnoredPatterns_PanicsForInvalidRouteDefinitionPatterns(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	app.Config.ClientRouteDefinitionPatterns = []string{
		" frontend/src/routes/core.vorma.routes.ts ",
		"frontend/src/routes/core.vorma.routes.ts",
		"",
		"\nfrontend/src/routes/extra.vorma.routes.ts\n",
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal(
				"expected panic for invalid client route definition patterns",
			)
		}
		recoveredMessage := fmt.Sprint(recovered)
		if !strings.Contains(
			recoveredMessage,
			"must not contain surrounding whitespace",
		) {
			t.Fatalf(
				"panic = %q, expected surrounding-whitespace validation message",
				recoveredMessage,
			)
		}
	}()

	_ = buildViteIgnoredPatterns(app)
}

func TestFormatConfigFilePatternForViteIgnore(t *testing.T) {
	absoluteConfigFilePath := filepath.Join(
		t.TempDir(),
		"backend",
		"wave.config.json",
	)

	if got, want := formatConfigFilePatternForViteIgnore("backend/wave.config.json"), filepath.ToSlash(path.Join("**", "backend/wave.config.json")); got != want {
		t.Fatalf(
			"formatConfigFilePatternForViteIgnore(relative) = %q, want %q",
			got,
			want,
		)
	}
	if got, want := formatConfigFilePatternForViteIgnore(absoluteConfigFilePath), filepath.ToSlash(absoluteConfigFilePath); got != want {
		t.Fatalf(
			"formatConfigFilePatternForViteIgnore(absolute) = %q, want %q",
			got,
			want,
		)
	}
	if got := formatConfigFilePatternForViteIgnore("   "); got != "" {
		t.Fatalf(
			"formatConfigFilePatternForViteIgnore(empty) = %q, want empty",
			got,
		)
	}
}

func TestActionCategoryForMethod(t *testing.T) {
	t.Run("query method", func(t *testing.T) {
		category, isMutation, ok := actionCategoryForMethod(http.MethodGet)
		if !ok {
			t.Fatal("expected GET to be recognized")
		}
		if category != "query" {
			t.Fatalf("category = %q, want %q", category, "query")
		}
		if isMutation {
			t.Fatal("expected GET not to be mutation")
		}
	})

	t.Run("mutation method", func(t *testing.T) {
		category, isMutation, ok := actionCategoryForMethod(http.MethodPatch)
		if !ok {
			t.Fatal("expected PATCH to be recognized")
		}
		if category != "mutation" {
			t.Fatalf("category = %q, want %q", category, "mutation")
		}
		if !isMutation {
			t.Fatal("expected PATCH to be mutation")
		}
	})

	t.Run("unsupported method", func(t *testing.T) {
		category, isMutation, ok := actionCategoryForMethod(http.MethodOptions)
		if ok {
			t.Fatal("expected OPTIONS to be unsupported")
		}
		if category != "" {
			t.Fatalf("category = %q, want empty", category)
		}
		if isMutation {
			t.Fatal("expected unsupported method not to be mutation")
		}
	})
}

func TestSortedActionKeys_SortsByPatternThenMethod(t *testing.T) {
	actionsRouter := mux.NewRouter(&mux.Options{MountRoot: "/api/"})

	mux.AddTaskHandler(
		actionsRouter,
		http.MethodPatch,
		"/b",
		mux.TaskHandlerFromFunc(
			func(_ *mux.ReqData[mux.None]) (mux.None, error) {
				return mux.None{}, nil
			},
		),
	)
	mux.AddTaskHandler(
		actionsRouter,
		http.MethodGet,
		"/a",
		mux.TaskHandlerFromFunc(
			func(_ *mux.ReqData[mux.None]) (mux.None, error) {
				return mux.None{}, nil
			},
		),
	)
	mux.AddTaskHandler(
		actionsRouter,
		http.MethodPost,
		"/a",
		mux.TaskHandlerFromFunc(
			func(_ *mux.ReqData[mux.None]) (mux.None, error) {
				return mux.None{}, nil
			},
		),
	)
	mux.AddTaskHandler(
		actionsRouter,
		http.MethodDelete,
		"/b",
		mux.TaskHandlerFromFunc(
			func(_ *mux.ReqData[mux.None]) (mux.None, error) {
				return mux.None{}, nil
			},
		),
	)

	sorted := sortedActionKeys(actionsRouter.AllRoutes())
	if len(sorted) != 4 {
		t.Fatalf("sorted action keys len = %d, want %d", len(sorted), 4)
	}

	got := make([]string, 0, len(sorted))
	for _, actionKey := range sorted {
		got = append(got, actionKey.pattern+" "+actionKey.method)
	}
	want := []string{
		"/a GET",
		"/a POST",
		"/b DELETE",
		"/b PATCH",
	}

	if !slices.Equal(got, want) {
		t.Fatalf("sorted action keys = %#v, want %#v", got, want)
	}
}

func TestGeneratedTSUnchanged(t *testing.T) {
	targetPath := filepath.Join(t.TempDir(), "index.ts")
	content := []byte("export const x = 1;")

	t.Run("missing file", func(t *testing.T) {
		unchanged, err := generatedTSUnchanged(targetPath, content)
		if err != nil {
			t.Fatalf("generatedTSUnchanged returned error: %v", err)
		}
		if unchanged {
			t.Fatal("expected unchanged=false when file does not exist")
		}
	})

	t.Run("existing same content", func(t *testing.T) {
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(targetPath, content, 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
		unchanged, err := generatedTSUnchanged(targetPath, content)
		if err != nil {
			t.Fatalf("generatedTSUnchanged returned error: %v", err)
		}
		if !unchanged {
			t.Fatal("expected unchanged=true when content matches")
		}
	})

	t.Run("existing different content", func(t *testing.T) {
		if err := os.WriteFile(targetPath, []byte("export const x = 2;"), 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
		unchanged, err := generatedTSUnchanged(targetPath, content)
		if err != nil {
			t.Fatalf("generatedTSUnchanged returned error: %v", err)
		}
		if unchanged {
			t.Fatal("expected unchanged=false when content differs")
		}
	})

	t.Run("unexpected read error", func(t *testing.T) {
		badPath := filepath.Join(targetPath, "child.ts")
		_, err := generatedTSUnchanged(badPath, content)
		if err == nil {
			t.Fatal("expected error for unreadable path")
		}
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("expected os.PathError, got %T", err)
		}
	})
}

func TestGenerateAndAssembleTSContent(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/root.tsx",
				ExportKey:       "default",
			},
		})

		contentBytes, err := generateAndAssembleTSContent(app, l)
		if err != nil {
			t.Fatalf("generateAndAssembleTSContent returned error: %v", err)
		}
		content := string(contentBytes)
		if !strings.Contains(content, "const routes = [") {
			t.Fatalf(
				"expected routes collection in generated content:\n%s",
				content,
			)
		}
		if !strings.Contains(content, "Vorma Vite Config:") {
			t.Fatalf(
				"expected rollup config block in generated content:\n%s",
				content,
			)
		}
	})
}

func TestWriteGeneratedTSContentIfChanged(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	targetPath := filepath.Join(t.TempDir(), "index.ts")
	contentBytes := []byte("export const x = 1;\n")

	t.Run("writes new file", func(t *testing.T) {
		if err := writeGeneratedTSContentIfChanged(app, targetPath, contentBytes); err != nil {
			t.Fatalf("writeGeneratedTSContentIfChanged returned error: %v", err)
		}
		written, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("read generated file: %v", err)
		}
		if !bytes.Equal(written, contentBytes) {
			t.Fatalf("written bytes = %q, want %q", written, contentBytes)
		}
	})

	t.Run("skips unchanged content", func(t *testing.T) {
		if err := writeGeneratedTSContentIfChanged(app, targetPath, contentBytes); err != nil {
			t.Fatalf("writeGeneratedTSContentIfChanged returned error: %v", err)
		}
		written, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("read generated file: %v", err)
		}
		if !bytes.Equal(written, contentBytes) {
			t.Fatalf("written bytes = %q, want %q", written, contentBytes)
		}
	})
}
