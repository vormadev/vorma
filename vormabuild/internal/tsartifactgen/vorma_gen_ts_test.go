package tsartifactgen

import (
	"bytes"
	"encoding/json"
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
	"github.com/vormadev/vorma/internal/vormaruntime/runtimeconfig"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
	"github.com/vormadev/vorma/wave"
)

func mustParsedVormaConfigForTSArtifactGenerationTests(
	tb testing.TB,
	overrides *vormaruntime.VormaConfigJSON,
) vormaruntime.VormaConfig {
	tb.Helper()

	rawConfig := vormaruntime.VormaConfigJSON{
		MainBuildEntry:       "backend/cmd/build",
		UIVariant:            string(vormaruntime.UIVariantReact),
		HTMLTemplateLocation: "entry.go.html",
		ClientEntry:          "frontend/src/vorma.entry.tsx",
		ClientRouteDefinitionPatterns: []string{
			"frontend/src/**/*vorma.routes.ts",
		},
		TSGenOutDir: "frontend/src/vorma.gen",
	}
	if overrides != nil {
		if overrides.MainBuildEntry != "" {
			rawConfig.MainBuildEntry = overrides.MainBuildEntry
		}
		if overrides.UIVariant != "" {
			rawConfig.UIVariant = overrides.UIVariant
		}
		if overrides.HTMLTemplateLocation != "" {
			rawConfig.HTMLTemplateLocation = overrides.HTMLTemplateLocation
		}
		if overrides.ClientEntry != "" {
			rawConfig.ClientEntry = overrides.ClientEntry
		}
		if overrides.ClientRouteDefinitionPatterns != nil {
			rawConfig.ClientRouteDefinitionPatterns = append(
				[]string(nil),
				overrides.ClientRouteDefinitionPatterns...,
			)
		}
		if overrides.TSGenOutDir != "" {
			rawConfig.TSGenOutDir = overrides.TSGenOutDir
		}
		if overrides.BuildtimePublicURLFuncName != "" {
			rawConfig.BuildtimePublicURLFuncName = overrides.BuildtimePublicURLFuncName
		}
	}

	return testkit.MustParseVormaConfigJSONForTest(tb, rawConfig)
}

func parseMutatedAppVormaConfigForTSArtifactGenerationTests(
	tb testing.TB,
	app *vormaruntime.Vorma,
	mutate func(*vormaruntime.VormaConfigJSON),
) (vormaruntime.VormaConfig, error) {
	tb.Helper()
	if app == nil || app.Wave == nil {
		return nil, errors.New("app with Wave runtime is required")
	}
	rawConfig := struct {
		Vorma vormaruntime.VormaConfigJSON `json:"Vorma"`
	}{}
	if unmarshalError := json.Unmarshal(
		app.Wave.RawConfigJSON(),
		&rawConfig,
	); unmarshalError != nil {
		return nil, unmarshalError
	}
	if mutate != nil {
		mutate(&rawConfig.Vorma)
	}
	mutatedPayload, marshalError := json.Marshal(rawConfig)
	if marshalError != nil {
		return nil, marshalError
	}
	return runtimeconfig.ParseVormaConfigJSON(mutatedPayload, app.Wave.ParsedConfig())
}

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

		got, err := getEntrypoints(l)
		if err != nil {
			t.Fatalf("getEntrypoints returned error: %v", err)
		}
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
		currentWorkingDirectory, currentWorkingDirectoryError := os.Getwd()
		if currentWorkingDirectoryError != nil {
			t.Fatalf(
				"resolve current working directory: %v",
				currentWorkingDirectoryError,
			)
		}
		currentWorkingDirectoryRelativeDistDirForGeneratedTypeScript, distDirError := pathRelativeToCurrentWorkingDirectoryForGeneratedTypeScript(
			currentWorkingDirectory,
			distRootPathForGeneratedTypeScript(app),
		)
		if distDirError != nil {
			t.Fatalf(
				"normalize dist dir for generated TypeScript assertion: %v",
				distDirError,
			)
		}
		if !strings.Contains(
			content,
			fmt.Sprintf(
				`distDir: "%s"`,
				currentWorkingDirectoryRelativeDistDirForGeneratedTypeScript,
			),
		) {
			t.Fatalf("rollup options missing dist dir:\n%s", content)
		}
		if strings.Contains(
			content,
			filepath.ToSlash(filepath.Clean(currentWorkingDirectory)),
		) {
			t.Fatalf(
				"rollup options leaked current working directory absolute path:\n%s",
				content,
			)
		}
		if !strings.Contains(content, `"react"`) ||
			!strings.Contains(content, `"react-dom"`) {
			t.Fatalf("rollup options missing react dedupe list:\n%s", content)
		}
		for _, routeDefinitionPattern := range app.Config.ClientRouteDefinitionPatterns() {
			resolveRootRelativeRouteDefinitionPattern, routeDefinitionPatternError := runtimeconfig.NormalizePathOrPatternToResolveRootRelative(
				app.Wave.ParsedConfig().ResolveRoot(),
				routeDefinitionPattern,
			)
			if routeDefinitionPatternError != nil {
				t.Fatalf(
					"normalize route definition pattern %q for assertion: %v",
					routeDefinitionPattern,
					routeDefinitionPatternError,
				)
			}
			if strings.Contains(content, resolveRootRelativeRouteDefinitionPattern) {
				continue
			}
			t.Fatalf(
				"rollup options missing client route defs pattern in ignored patterns:\n%s",
				content,
			)
		}
		currentWorkingDirectoryRelativeTSGenOutDir, tsGenOutDirError := pathRelativeToCurrentWorkingDirectoryForGeneratedTypeScript(
			currentWorkingDirectory,
			app.Config.TSGenOutDir(),
		)
		if tsGenOutDirError != nil {
			t.Fatalf(
				"normalize TSGenOutDir for generated TypeScript assertion: %v",
				tsGenOutDirError,
			)
		}
		if !strings.Contains(content, currentWorkingDirectoryRelativeTSGenOutDir+"/**/*") {
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
		Config: mustParsedVormaConfigForTSArtifactGenerationTests(
			t,
			&vormaruntime.VormaConfigJSON{
				UIVariant: string(vormaruntime.UIVariantReact),
			},
		),
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
		Config: mustParsedVormaConfigForTSArtifactGenerationTests(
			t,
			&vormaruntime.VormaConfigJSON{
				UIVariant: string(vormaruntime.UIVariantReact),
			},
		),
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
	data, buildVitePluginTemplateDataError := buildVitePluginTemplateData(
		app,
		entrypoints,
	)
	if buildVitePluginTemplateDataError != nil {
		t.Fatalf(
			"buildVitePluginTemplateData returned error: %v",
			buildVitePluginTemplateDataError,
		)
	}

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
	if data.FuncName != app.Config.BuildtimePublicURLFuncName() {
		t.Fatalf(
			"FuncName = %q, want %q",
			data.FuncName,
			app.Config.BuildtimePublicURLFuncName(),
		)
	}
	currentWorkingDirectory, currentWorkingDirectoryError := os.Getwd()
	if currentWorkingDirectoryError != nil {
		t.Fatalf(
			"resolve current working directory: %v",
			currentWorkingDirectoryError,
		)
	}
	currentWorkingDirectoryRelativeDistDirForGeneratedTypeScript, distDirError := pathRelativeToCurrentWorkingDirectoryForGeneratedTypeScript(
		currentWorkingDirectory,
		distRootPathForGeneratedTypeScript(app),
	)
	if distDirError != nil {
		t.Fatalf(
			"normalize dist dir for generated TypeScript assertion: %v",
			distDirError,
		)
	}
	if data.DistDir != currentWorkingDirectoryRelativeDistDirForGeneratedTypeScript {
		t.Fatalf(
			"DistDir = %q, want %q",
			data.DistDir,
			currentWorkingDirectoryRelativeDistDirForGeneratedTypeScript,
		)
	}
	if !slices.Equal(data.DedupeList, reactDedupeList) {
		t.Fatalf("DedupeList = %#v, want %#v", data.DedupeList, reactDedupeList)
	}
	if len(data.IgnoredPatterns) == 0 {
		t.Fatal("expected non-empty ignored patterns")
	}
}

func TestBuildVitePluginTemplateData_UsesConfigFSRelativeDistWhenConfigIsNested(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	t.Chdir(projectRoot)

	testkit.MustWriteFile(
		t,
		filepath.Join("backend", "wave.config.json"),
		[]byte(`{
  "Core": {
    "ProjectID": "tsartifactgen-test",
    "ResolveRoot": ".",
    "MainAppEntry": "cmd/serve",
    "StaticAssetDirs": {
      "Private": ".wavedist/static/assets/private",
      "Public": ".wavedist/static/assets/public"
    },
    "PublicPathPrefix": "/"
  },
  "Vorma": {
    "MainBuildEntry": "cmd/build",
    "UIVariant": "react",
    "HTMLTemplateLocation": "entry.go.html",
    "ClientEntry": "frontend/src/vorma.entry.tsx",
    "ClientRouteDefinitionPatterns": ["frontend/src/**/*vorma.routes.ts"],
    "TSGenOutDir": "frontend/src/vorma.gen",
    "BuildtimePublicURLFuncName": "waveBuildtimeURL"
  }
}`),
	)
	if makeDistStaticDirectoryError := os.MkdirAll(
		filepath.Join(projectRoot, "backend", ".wavedist", "static"),
		0o755,
	); makeDistStaticDirectoryError != nil {
		t.Fatalf(
			"create nested backend dist static directory: %v",
			makeDistStaticDirectoryError,
		)
	}

	waveRuntime := wave.New(wave.Config{
		FS:         os.DirFS(projectRoot),
		ConfigPath: filepath.Join("backend", "wave.config.json"),
		Logger:     testkit.TestLogger(),
	})
	app := vormaruntime.NewVormaApp(vormaruntime.VormaAppConfig{
		Wave:   waveRuntime,
		Logger: testkit.TestLogger(),
	})

	templateData, buildTemplateDataError := buildVitePluginTemplateData(
		app,
		[]string{"frontend/src/vorma.entry.tsx"},
	)
	if buildTemplateDataError != nil {
		t.Fatalf(
			"buildVitePluginTemplateData returned error: %v",
			buildTemplateDataError,
		)
	}

	expectedDistDir := filepath.ToSlash(
		filepath.Clean(filepath.Join("backend", ".wavedist")),
	)
	if templateData.DistDir != expectedDistDir {
		t.Fatalf(
			"DistDir = %q, want %q",
			templateData.DistDir,
			expectedDistDir,
		)
	}
	if filepath.IsAbs(templateData.DistDir) {
		t.Fatalf("DistDir must not be machine-absolute: %q", templateData.DistDir)
	}
}

func TestBuildViteIgnoredPatterns_ReturnsParseErrorForInvalidRouteDefinitionPatterns(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	_, parseError := parseMutatedAppVormaConfigForTSArtifactGenerationTests(
		t,
		app,
		func(config *vormaruntime.VormaConfigJSON) {
			config.ClientRouteDefinitionPatterns = []string{
				" frontend/src/routes/core.vorma.routes.ts ",
				"frontend/src/routes/core.vorma.routes.ts",
				"",
				"\nfrontend/src/routes/extra.vorma.routes.ts\n",
			}
		},
	)
	if parseError == nil {
		t.Fatal(
			"expected parse error for invalid client route definition patterns",
		)
	}
	if !strings.Contains(
		parseError.Error(),
		"must not contain surrounding whitespace",
	) {
		t.Fatalf(
			"error = %q, expected surrounding-whitespace parse validation message",
			parseError.Error(),
		)
	}
}

func TestFormatConfigFilePatternForViteIgnore(t *testing.T) {
	currentWorkingDirectory, currentWorkingDirectoryError := os.Getwd()
	if currentWorkingDirectoryError != nil {
		t.Fatalf(
			"resolve current working directory: %v",
			currentWorkingDirectoryError,
		)
	}

	absoluteConfigFilePath := filepath.Join(
		currentWorkingDirectory,
		"tmp",
		"config-test",
		"wave.config.json",
	)

	gotFromRelativePattern, relativePatternError := formatConfigFilePatternForViteIgnore(
		currentWorkingDirectory,
		"backend/wave.config.json",
	)
	if relativePatternError != nil {
		t.Fatalf(
			"formatConfigFilePatternForViteIgnore(relative) returned error: %v",
			relativePatternError,
		)
	}
	if got, want := gotFromRelativePattern, filepath.ToSlash(path.Join("**", "backend/wave.config.json")); got != want {
		t.Fatalf(
			"formatConfigFilePatternForViteIgnore(relative) = %q, want %q",
			got,
			want,
		)
	}

	gotFromAbsolutePattern, absolutePatternError := formatConfigFilePatternForViteIgnore(
		currentWorkingDirectory,
		absoluteConfigFilePath,
	)
	if absolutePatternError != nil {
		t.Fatalf(
			"formatConfigFilePatternForViteIgnore(absolute) returned error: %v",
			absolutePatternError,
		)
	}
	if got, want := gotFromAbsolutePattern, filepath.ToSlash(path.Join("**", "tmp/config-test/wave.config.json")); got != want {
		t.Fatalf(
			"formatConfigFilePatternForViteIgnore(absolute) = %q, want %q",
			got,
			want,
		)
	}

	gotFromEmptyPattern, emptyPatternError := formatConfigFilePatternForViteIgnore(
		currentWorkingDirectory,
		"   ",
	)
	if emptyPatternError != nil {
		t.Fatalf(
			"formatConfigFilePatternForViteIgnore(empty) returned error: %v",
			emptyPatternError,
		)
	}
	if got := gotFromEmptyPattern; got != "" {
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
