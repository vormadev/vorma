package vormaruntime

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/wave"
)

func TestGetBasePathsStageOneOrTwo_SelectsExpectedStageFile(t *testing.T) {
	stageOne := defaultPathsFile("stage-one-build", map[string]*Path{
		"/": {OriginalPattern: "/"},
	})
	stageTwo := defaultPathsFile("stage-two-build", map[string]*Path{
		"/": {OriginalPattern: "/"},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stageOne,
		stageTwo: stageTwo,
	})
	app := fixture.app

	gotStageOne, err := app.getBasePaths_StageOneOrTwo(true)
	if err != nil {
		t.Fatalf("getBasePaths_StageOneOrTwo(true) error = %v", err)
	}
	if gotStageOne.BuildID != "stage-one-build" {
		t.Fatalf(
			"stage one build ID = %q, want %q",
			gotStageOne.BuildID,
			"stage-one-build",
		)
	}

	gotStageTwo, err := app.getBasePaths_StageOneOrTwo(false)
	if err != nil {
		t.Fatalf("getBasePaths_StageOneOrTwo(false) error = %v", err)
	}
	if gotStageTwo.BuildID != "stage-two-build" {
		t.Fatalf(
			"stage two build ID = %q, want %q",
			gotStageTwo.BuildID,
			"stage-two-build",
		)
	}
}

func TestGetBasePathsStageOneOrTwo_ErrorPaths(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app

	t.Run("MissingFile", func(t *testing.T) {
		app.WithLock(func(lv *LockedVorma) {
			lv.v._privateFS = fstest.MapFS{}
		})

		_, err := app.getBasePaths_StageOneOrTwo(true)
		if err == nil {
			t.Fatal("expected error when stage file is missing")
		}
		if !strings.Contains(err.Error(), "could not open") {
			t.Fatalf("error = %q, expected to mention open failure", err)
		}
	})

	t.Run("MalformedJSON", func(t *testing.T) {
		app.WithLock(func(lv *LockedVorma) {
			lv.v._privateFS = fstest.MapFS{
				"vorma_out/" + VormaPathsStageOneJSONFileName: &fstest.MapFile{
					Data: []byte("{"),
				},
			}
		})

		_, err := app.getBasePaths_StageOneOrTwo(true)
		if err == nil {
			t.Fatal("expected error for malformed stage JSON")
		}
		if !strings.Contains(err.Error(), "could not decode") {
			t.Fatalf("error = %q, expected to mention decode failure", err)
		}
	})

	t.Run("DevStageOneAllowsMissingClientEntryOut", func(t *testing.T) {
		app.WithLock(func(lv *LockedVorma) {
			lv.v._privateFS = fstest.MapFS{
				"vorma_out/" + VormaPathsStageOneJSONFileName: &fstest.MapFile{
					Data: []byte(
						`{"stage":"stage-one","buildID":"b","clientEntrySrc":"frontend/src/vorma.entry.tsx","paths":{"/":{"originalPattern":"/","srcPath":"frontend/src/routes/root.tsx","exportKey":"default"}},"routeManifestFile":"vorma_out/route-manifest.js"}`,
					),
				},
			}
		})

		got, err := app.getBasePaths_StageOneOrTwo(true)
		if err != nil {
			t.Fatalf(
				"expected stage one in dev mode to allow missing clientEntryOut, got error: %v",
				err,
			)
		}
		if got == nil {
			t.Fatal("expected stage one result, got nil")
		}
		if got.ClientEntryOut != "" {
			t.Fatalf(
				"stage one clientEntryOut = %q, want empty value",
				got.ClientEntryOut,
			)
		}
	})

	t.Run("ProdStageTwoRequiresClientEntryOut", func(t *testing.T) {
		app.WithLock(func(lv *LockedVorma) {
			lv.v._privateFS = fstest.MapFS{
				"vorma_out/" + VormaPathsStageTwoJSONFileName: &fstest.MapFile{
					Data: []byte(
						`{"stage":"stage-two","buildID":"b","clientEntrySrc":"frontend/src/vorma.entry.tsx","paths":{"/":{"originalPattern":"/","srcPath":"frontend/src/routes/root.tsx","outPath":"vorma_out/root.js","exportKey":"default"}},"routeManifestFile":"vorma_out/route-manifest.js"}`,
					),
				},
			}
		})

		_, err := app.getBasePaths_StageOneOrTwo(false)
		if err == nil {
			t.Fatal(
				"expected stage two in production mode to require clientEntryOut",
			)
		}
		if !strings.Contains(err.Error(), "clientEntryOut is required") {
			t.Fatalf(
				"error = %q, expected missing clientEntryOut validation error",
				err,
			)
		}
	})

	t.Run("NullPathEntry", func(t *testing.T) {
		app.WithLock(func(lv *LockedVorma) {
			lv.v._privateFS = fstest.MapFS{
				"vorma_out/" + VormaPathsStageOneJSONFileName: &fstest.MapFile{
					Data: []byte(
						`{"stage":"stage-one","buildID":"b","clientEntrySrc":"frontend/src/vorma.entry.tsx","paths":{"/bad":null},"routeManifestFile":"vorma_out/route-manifest.js"}`,
					),
				},
			}
		})

		_, err := app.getBasePaths_StageOneOrTwo(true)
		if err == nil {
			t.Fatal("expected error for null path entry")
		}
		if !strings.Contains(err.Error(), "cannot be null") {
			t.Fatalf("error = %q, expected null path validation error", err)
		}
	})

	t.Run("PathEntryPatternMismatch", func(t *testing.T) {
		app.WithLock(func(lv *LockedVorma) {
			lv.v._privateFS = fstest.MapFS{
				"vorma_out/" + VormaPathsStageOneJSONFileName: &fstest.MapFile{
					Data: []byte(
						`{"stage":"stage-one","buildID":"b","clientEntrySrc":"frontend/src/vorma.entry.tsx","paths":{"/a":{"originalPattern":"/b","srcPath":"frontend/src/routes/a.tsx","exportKey":"default"}},"routeManifestFile":"vorma_out/route-manifest.js"}`,
					),
				},
			}
		})

		_, err := app.getBasePaths_StageOneOrTwo(true)
		if err == nil {
			t.Fatal("expected error for path/originalPattern mismatch")
		}
		if !strings.Contains(err.Error(), "does not match key") {
			t.Fatalf("error = %q, expected path mismatch validation error", err)
		}
	})

	t.Run("PathEntryMissingOriginalPattern", func(t *testing.T) {
		app.WithLock(func(lv *LockedVorma) {
			lv.v._privateFS = fstest.MapFS{
				"vorma_out/" + VormaPathsStageOneJSONFileName: &fstest.MapFile{
					Data: []byte(
						`{"stage":"stage-one","buildID":"b","clientEntrySrc":"frontend/src/vorma.entry.tsx","paths":{"/a":{"srcPath":"frontend/src/routes/a.tsx","exportKey":"default"}},"routeManifestFile":"vorma_out/route-manifest.js"}`,
					),
				},
			}
		})

		_, err := app.getBasePaths_StageOneOrTwo(true)
		if err == nil {
			t.Fatal("expected error for missing originalPattern")
		}
		if !strings.Contains(err.Error(), "originalPattern is required") {
			t.Fatalf(
				"error = %q, expected missing originalPattern validation error",
				err,
			)
		}
	})
}

func TestInit_ReinitSemanticArtifactValidationFailuresDoNotMutateRuntimeState(
	t *testing.T,
) {
	testCases := []struct {
		name                string
		mutateInvalidStage2 func(*PathsFile)
	}{
		{
			name: "missing_route_manifest_file",
			mutateInvalidStage2: func(pathsFile *PathsFile) {
				pathsFile.RouteManifestFile = ""
			},
		},
		{
			name: "missing_stage_two_client_entry_out",
			mutateInvalidStage2: func(pathsFile *PathsFile) {
				pathsFile.ClientEntryOut = ""
			},
		},
		{
			name: "missing_stage_two_route_out_path",
			mutateInvalidStage2: func(pathsFile *PathsFile) {
				pathsFile.Paths["/products/:id"].OutPath = ""
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			initialStage := defaultPathsFile(
				"semantic-old-build",
				map[string]*Path{
					"/products/:id": {
						OriginalPattern: "/products/:id",
						SrcPath:         "frontend/src/routes/products.$id.old.tsx",
						OutPath:         "vorma_out/routes/products.$id.old.js",
						ExportKey:       "default",
					},
				},
			)

			fixture := newTestFixture(t, testFixtureOptions{
				stageOne: initialStage,
				stageTwo: initialStage,
			})
			app := fixture.app
			handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

			recBefore := httptest.NewRecorder()
			reqBefore := httptest.NewRequest(
				http.MethodGet,
				"/products/1?vorma_json=semantic-old-build",
				nil,
			)
			handler.ServeHTTP(recBefore, reqBefore)
			if recBefore.Code != http.StatusOK {
				t.Fatalf(
					"before failed init status = %d, want %d",
					recBefore.Code,
					http.StatusOK,
				)
			}
			if !strings.Contains(
				recBefore.Body.String(),
				"/vorma_out/routes/products.$id.old.js",
			) {
				t.Fatalf(
					"before failed init body missing old import URL, body=%q",
					recBefore.Body.String(),
				)
			}

			invalidStage := defaultPathsFile(
				"semantic-invalid-build",
				map[string]*Path{
					"/products/:id": {
						OriginalPattern: "/products/:id",
						SrcPath:         "frontend/src/routes/products.$id.invalid.tsx",
						OutPath:         "vorma_out/routes/products.$id.invalid.js",
						ExportKey:       "default",
					},
				},
			)
			tc.mutateInvalidStage2(invalidStage)

			mustWriteJSONFile(
				t,
				filepath.Join(
					fixture.privateDir,
					VormaOutDirname,
					VormaPathsStageTwoJSONFileName,
				),
				invalidStage,
			)

			didPanic := false
			func() {
				defer func() {
					if recover() != nil {
						didPanic = true
					}
				}()
				app.MustInit()
			}()
			if !didPanic {
				t.Fatal(
					"expected Init() to panic for semantic stage-two artifact validation issue",
				)
			}

			if got, want := app.BuildID(), "semantic-old-build"; got != want {
				t.Fatalf(
					"build ID = %q, want %q after failed semantic re-init",
					got,
					want,
				)
			}

			recAfter := httptest.NewRecorder()
			reqAfter := httptest.NewRequest(
				http.MethodGet,
				"/products/2?vorma_json=semantic-old-build",
				nil,
			)
			handler.ServeHTTP(recAfter, reqAfter)
			if recAfter.Code != http.StatusOK {
				t.Fatalf(
					"after failed init status = %d, want %d",
					recAfter.Code,
					http.StatusOK,
				)
			}
			if !strings.Contains(
				recAfter.Body.String(),
				"/vorma_out/routes/products.$id.old.js",
			) {
				t.Fatalf(
					"after failed init body missing old import URL, body=%q",
					recAfter.Body.String(),
				)
			}
			if strings.Contains(
				recAfter.Body.String(),
				"/vorma_out/routes/products.$id.invalid.js",
			) {
				t.Fatalf(
					"after failed init body leaked invalid import URL, body=%q",
					recAfter.Body.String(),
				)
			}
		})
	}
}

func TestValidateAndDecorateNestedRouter(t *testing.T) {
	stage := defaultPathsFile("build", map[string]*Path{
		"/": {
			OriginalPattern: "/",
			SrcPath:         "frontend/src/routes/root.tsx",
			OutPath:         "vorma_out/root.js",
			ExportKey:       "default",
		},
		"/items/:id": {
			OriginalPattern: "/items/:id",
			SrcPath:         "frontend/src/routes/items.$id.tsx",
			OutPath:         "vorma_out/items.$id.js",
			ExportKey:       "default",
		},
	})

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app

	t.Run("RegistersMissingPatterns", func(t *testing.T) {
		nr := mux.NewNestedRouter(nil)
		mux.AddNestedPatternWithoutHandler(nr, "/")
		if nr.IsRegistered("/items/:id") {
			t.Fatal("setup failure: /items/:id should not be registered yet")
		}

		app.validateAndDecorateNestedRouter(nr)

		if !nr.IsRegistered("/") {
			t.Fatal("expected root pattern to stay registered")
		}
		if !nr.IsRegistered("/items/:id") {
			t.Fatal("expected missing pattern to be registered")
		}
	})

	t.Run("PanicsOnNilRouter", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic when nestedRouter is nil")
			}
		}()
		app.validateAndDecorateNestedRouter(nil)
	})
}

func TestRegisterPatternIfNeeded_IsIdempotent(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app

	const pattern = "/server-only"
	if app.LoadersRouter().NestedRouter.IsRegistered(pattern) {
		t.Fatalf("setup failure: %q unexpectedly registered", pattern)
	}

	app.RegisterPatternIfNeeded(pattern)
	if !app.LoadersRouter().NestedRouter.IsRegistered(pattern) {
		t.Fatalf("expected %q to be registered", pattern)
	}

	// Calling again should be a no-op and must not panic.
	app.RegisterPatternIfNeeded(pattern)
	if !app.LoadersRouter().NestedRouter.IsRegistered(pattern) {
		t.Fatalf("expected %q to remain registered", pattern)
	}
}

func TestRegisterPatternIfNeeded_IsConcurrentSafe(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app

	panicValues := make(chan any, 1)
	const goroutinesPerPattern = 8
	const patternCount = 128

	for patternIndex := range patternCount {
		pattern := fmt.Sprintf("/concurrent-%d", patternIndex)
		startGate := make(chan struct{})
		var waitGroup sync.WaitGroup
		waitGroup.Add(goroutinesPerPattern)

		for range goroutinesPerPattern {
			go func() {
				defer waitGroup.Done()
				defer func() {
					if recoveredPanicValue := recover(); recoveredPanicValue != nil {
						select {
						case panicValues <- recoveredPanicValue:
						default:
						}
					}
				}()

				<-startGate
				app.RegisterPatternIfNeeded(pattern)
			}()
		}

		close(startGate)
		waitGroup.Wait()

		select {
		case recoveredPanicValue := <-panicValues:
			t.Fatalf(
				"RegisterPatternIfNeeded should not panic under concurrency; panic=%v",
				recoveredPanicValue,
			)
		default:
		}

		if !app.LoadersRouter().NestedRouter.IsRegistered(pattern) {
			t.Fatalf(
				"expected %q to be registered after concurrent calls",
				pattern,
			)
		}
	}
}

func TestInit_PanicsWithWrappedInitError(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app
	app.Config.HTMLTemplateLocation = "missing-template.go.html"

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from Init() when template is missing")
		}
		msg := r.(error).Error()
		if !strings.Contains(msg, "error initializing Vorma") {
			t.Fatalf("panic message = %q, expected wrapped init prefix", msg)
		}
		if !strings.Contains(msg, "error parsing root template") {
			t.Fatalf(
				"panic message = %q, expected underlying parse error context",
				msg,
			)
		}
	}()

	app.MustInit()
}

func TestPrettyPrintFS_NoErrorOnBasicFS(t *testing.T) {
	fsys := fstest.MapFS{
		"root.txt":  {Data: []byte("ok")},
		"dir/a.txt": {Data: []byte("a")},
	}
	if err := PrettyPrintFS(fsys); err != nil {
		t.Fatalf("PrettyPrintFS returned error: %v", err)
	}
}

func TestInitInner_NormalizesNilStageCollections(t *testing.T) {
	stage := &PathsFile{
		Stage:             "stage-two",
		BuildID:           "nil-collections-build",
		ClientEntrySrc:    "frontend/src/vorma.entry.tsx",
		ClientEntryOut:    "vorma_out/client-entry.js",
		ClientEntryDeps:   nil,
		Paths:             nil,
		RouteManifestFile: "vorma_out/route-manifest.js",
		DepToCSSBundleMap: nil,
	}

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app

	paths := app.Paths()
	if paths == nil {
		t.Fatal(
			"Paths() returned nil; expected empty map after init normalization",
		)
	}
	if len(paths) != 0 {
		t.Fatalf("Paths() len = %d, want 0", len(paths))
	}

	cssMap := app.DepToCSSBundleMap()
	if cssMap == nil {
		t.Fatal(
			"DepToCSSBundleMap() returned nil; expected empty map after init normalization",
		)
	}
	if len(cssMap) != 0 {
		t.Fatalf("DepToCSSBundleMap() len = %d, want 0", len(cssMap))
	}
}

func TestInit_PanicsWhenPrivateFSUnavailable(t *testing.T) {
	rootDir := t.TempDir()

	cfg := struct {
		Core  wave.CoreConfig `json:"Core"`
		Vorma VormaConfig     `json:"Vorma"`
	}{
		Core: wave.CoreConfig{
			MainAppEntry: "backend/cmd/serve",
			DistDir:      filepath.Join(rootDir, "dist"),
			StaticAssetDirs: wave.StaticAssetDirs{
				Private: "assets/private",
				Public:  "assets/public",
			},
			PublicPathPrefix: "/",
		},
		Vorma: VormaConfig{
			MainBuildEntry:       "backend/cmd/build",
			UIVariant:            string(UIVariantReact),
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
		WaveConfigJSON: cfgBytes,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	app := NewVormaApp(VormaAppConfig{
		Wave:   w,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when private FS is unavailable")
		}
		msg := r.(error).Error()
		if !strings.Contains(msg, "could not get private fs") {
			t.Fatalf("panic message = %q, expected private fs context", msg)
		}
	}()

	app.MustInit()
}

func TestInit_ReinitReplacesRemovedClientRoutes(t *testing.T) {
	const buildID = "reinit-routes-build"

	initialStage := defaultPathsFile(buildID, map[string]*Path{
		"/old": {
			OriginalPattern: "/old",
			SrcPath:         "frontend/src/routes/old.tsx",
			OutPath:         "vorma_out/routes/old.js",
			ExportKey:       "default",
		},
	})
	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: initialStage,
		stageTwo: initialStage,
	})
	app := fixture.app
	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	firstReq := httptest.NewRequest(
		http.MethodGet,
		"/old?vorma_json="+buildID,
		nil,
	)
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf(
			"first /old status = %d, want %d",
			firstRec.Code,
			http.StatusOK,
		)
	}

	updatedStage := defaultPathsFile(buildID, map[string]*Path{
		"/new": {
			OriginalPattern: "/new",
			SrcPath:         "frontend/src/routes/new.tsx",
			OutPath:         "vorma_out/routes/new.js",
			ExportKey:       "default",
		},
	})
	stageOneFile := filepath.Join(
		fixture.privateDir,
		VormaOutDirname,
		VormaPathsStageOneJSONFileName,
	)
	stageTwoFile := filepath.Join(
		fixture.privateDir,
		VormaOutDirname,
		VormaPathsStageTwoJSONFileName,
	)
	mustWriteJSONFile(t, stageOneFile, updatedStage)
	mustWriteJSONFile(t, stageTwoFile, updatedStage)

	app.MustInit()
	pathsAfterReinit := app.Paths()
	if _, ok := pathsAfterReinit["/new"]; !ok {
		t.Fatalf(
			"expected /new to exist in paths after reinit, got %#v",
			pathsAfterReinit,
		)
	}
	if _, ok := pathsAfterReinit["/old"]; ok {
		t.Fatalf(
			"expected /old to be removed from paths after reinit, got %#v",
			pathsAfterReinit,
		)
	}

	oldReq := httptest.NewRequest(
		http.MethodGet,
		"/old?vorma_json="+buildID,
		nil,
	)
	oldRec := httptest.NewRecorder()
	handler.ServeHTTP(oldRec, oldReq)
	if oldRec.Code != http.StatusNotFound {
		t.Fatalf(
			"post-reinit /old status = %d, want %d",
			oldRec.Code,
			http.StatusNotFound,
		)
	}

	newReq := httptest.NewRequest(
		http.MethodGet,
		"/new?vorma_json="+buildID,
		nil,
	)
	newRec := httptest.NewRecorder()
	handler.ServeHTTP(newRec, newReq)
	if newRec.Code != http.StatusOK {
		t.Fatalf(
			"post-reinit /new status = %d, want %d",
			newRec.Code,
			http.StatusOK,
		)
	}
}

func TestInit_ReinitInvalidatesRouteDataCacheWhenBuildIDUnchanged(
	t *testing.T,
) {
	const buildID = "reinit-cache-build"

	stageV1 := defaultPathsFile(buildID, map[string]*Path{
		"/items": {
			OriginalPattern: "/items",
			SrcPath:         "frontend/src/routes/items.v1.tsx",
			OutPath:         "vorma_out/routes/items.v1.js",
			ExportKey:       "default",
		},
	})
	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stageV1,
		stageTwo: stageV1,
	})
	app := fixture.app
	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	clearRouteDataCacheForTest(app)

	v1Req := httptest.NewRequest(
		http.MethodGet,
		"/items?vorma_json="+buildID,
		nil,
	)
	v1Rec := httptest.NewRecorder()
	handler.ServeHTTP(v1Rec, v1Req)
	if v1Rec.Code != http.StatusOK {
		t.Fatalf("v1 /items status = %d, want %d", v1Rec.Code, http.StatusOK)
	}
	if !strings.Contains(v1Rec.Body.String(), "/vorma_out/routes/items.v1.js") {
		t.Fatalf(
			"v1 body missing expected import URL, body=%q",
			v1Rec.Body.String(),
		)
	}
	if got := routeDataCacheLenForTest(app); got == 0 {
		t.Fatal(
			"expected route-data cache to contain entries after first request",
		)
	}

	stageV2 := defaultPathsFile(buildID, map[string]*Path{
		"/items": {
			OriginalPattern: "/items",
			SrcPath:         "frontend/src/routes/items.v2.tsx",
			OutPath:         "vorma_out/routes/items.v2.js",
			ExportKey:       "default",
		},
	})
	stageOneFile := filepath.Join(
		fixture.privateDir,
		VormaOutDirname,
		VormaPathsStageOneJSONFileName,
	)
	stageTwoFile := filepath.Join(
		fixture.privateDir,
		VormaOutDirname,
		VormaPathsStageTwoJSONFileName,
	)
	mustWriteJSONFile(t, stageOneFile, stageV2)
	mustWriteJSONFile(t, stageTwoFile, stageV2)

	app.MustInit()
	pathsAfterReinit := app.Paths()
	itemsPath, ok := pathsAfterReinit["/items"]
	if !ok || itemsPath == nil {
		t.Fatalf(
			"expected /items to exist after reinit, got %#v",
			pathsAfterReinit,
		)
	}
	if itemsPath.OutPath != "vorma_out/routes/items.v2.js" {
		t.Fatalf(
			"post-reinit /items outPath = %q, want %q",
			itemsPath.OutPath,
			"vorma_out/routes/items.v2.js",
		)
	}

	v2Req := httptest.NewRequest(
		http.MethodGet,
		"/items?vorma_json="+buildID,
		nil,
	)
	v2Rec := httptest.NewRecorder()
	handler.ServeHTTP(v2Rec, v2Req)
	if v2Rec.Code != http.StatusOK {
		t.Fatalf("v2 /items status = %d, want %d", v2Rec.Code, http.StatusOK)
	}
	if strings.Contains(v2Rec.Body.String(), "/vorma_out/routes/items.v1.js") {
		t.Fatalf(
			"v2 body leaked stale import URL, body=%q",
			v2Rec.Body.String(),
		)
	}
	if !strings.Contains(v2Rec.Body.String(), "/vorma_out/routes/items.v2.js") {
		t.Fatalf(
			"v2 body missing updated import URL, body=%q",
			v2Rec.Body.String(),
		)
	}
}

func TestInit_ReinitPreservesServerOnlyHandlerRoutes(t *testing.T) {
	const buildID = "reinit-server-only-build"

	initialStage := defaultPathsFile(buildID, map[string]*Path{
		"/client-old": {
			OriginalPattern: "/client-old",
			SrcPath:         "frontend/src/routes/client-old.tsx",
			OutPath:         "vorma_out/routes/client-old.js",
			ExportKey:       "default",
		},
	})
	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: initialStage,
		stageTwo: initialStage,
	})
	app := fixture.app

	mux.AddNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/server-only",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]bool, error) {
				return map[string]bool{"ok": true}, nil
			},
		),
	)

	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	serverOnlyBeforeReq := httptest.NewRequest(
		http.MethodGet,
		"/server-only?vorma_json="+buildID,
		nil,
	)
	serverOnlyBeforeRec := httptest.NewRecorder()
	handler.ServeHTTP(serverOnlyBeforeRec, serverOnlyBeforeReq)
	if serverOnlyBeforeRec.Code != http.StatusOK {
		t.Fatalf(
			"pre-reinit /server-only status = %d, want %d",
			serverOnlyBeforeRec.Code,
			http.StatusOK,
		)
	}

	updatedStage := defaultPathsFile(buildID, map[string]*Path{
		"/client-new": {
			OriginalPattern: "/client-new",
			SrcPath:         "frontend/src/routes/client-new.tsx",
			OutPath:         "vorma_out/routes/client-new.js",
			ExportKey:       "default",
		},
	})
	stageOneFile := filepath.Join(
		fixture.privateDir,
		VormaOutDirname,
		VormaPathsStageOneJSONFileName,
	)
	stageTwoFile := filepath.Join(
		fixture.privateDir,
		VormaOutDirname,
		VormaPathsStageTwoJSONFileName,
	)
	mustWriteJSONFile(t, stageOneFile, updatedStage)
	mustWriteJSONFile(t, stageTwoFile, updatedStage)

	app.MustInit()

	serverOnlyAfterReq := httptest.NewRequest(
		http.MethodGet,
		"/server-only?vorma_json="+buildID,
		nil,
	)
	serverOnlyAfterRec := httptest.NewRecorder()
	handler.ServeHTTP(serverOnlyAfterRec, serverOnlyAfterReq)
	if serverOnlyAfterRec.Code != http.StatusOK {
		t.Fatalf(
			"post-reinit /server-only status = %d, want %d",
			serverOnlyAfterRec.Code,
			http.StatusOK,
		)
	}
	if !strings.Contains(serverOnlyAfterRec.Body.String(), `"/server-only"`) {
		t.Fatalf(
			"post-reinit /server-only body missing matched pattern, body=%q",
			serverOnlyAfterRec.Body.String(),
		)
	}

	clientOldReq := httptest.NewRequest(
		http.MethodGet,
		"/client-old?vorma_json="+buildID,
		nil,
	)
	clientOldRec := httptest.NewRecorder()
	handler.ServeHTTP(clientOldRec, clientOldReq)
	if clientOldRec.Code != http.StatusNotFound {
		t.Fatalf(
			"post-reinit /client-old status = %d, want %d",
			clientOldRec.Code,
			http.StatusNotFound,
		)
	}

	clientNewReq := httptest.NewRequest(
		http.MethodGet,
		"/client-new?vorma_json="+buildID,
		nil,
	)
	clientNewRec := httptest.NewRecorder()
	handler.ServeHTTP(clientNewRec, clientNewReq)
	if clientNewRec.Code != http.StatusOK {
		t.Fatalf(
			"post-reinit /client-new status = %d, want %d",
			clientNewRec.Code,
			http.StatusOK,
		)
	}
}

func TestInit_ReinitFailureDoesNotPartiallyMutateRuntimeState(t *testing.T) {
	initialStage := defaultPathsFile("atomic-old-build", map[string]*Path{
		"/old": {
			OriginalPattern: "/old",
			SrcPath:         "frontend/src/routes/old.tsx",
			OutPath:         "vorma_out/routes/old.js",
			ExportKey:       "default",
		},
	})
	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: initialStage,
		stageTwo: initialStage,
	})
	app := fixture.app
	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	beforeReq := httptest.NewRequest(http.MethodGet, "/old", nil)
	beforeRec := httptest.NewRecorder()
	handler.ServeHTTP(beforeRec, beforeReq)
	if beforeRec.Code != http.StatusOK {
		t.Fatalf(
			"pre-failure /old status = %d, want %d",
			beforeRec.Code,
			http.StatusOK,
		)
	}
	if got := beforeRec.Header().Get(VormaBuildIDHeaderKey); got != "atomic-old-build" {
		t.Fatalf(
			"pre-failure build header = %q, want %q",
			got,
			"atomic-old-build",
		)
	}

	updatedStage := defaultPathsFile("atomic-new-build", map[string]*Path{
		"/new": {
			OriginalPattern: "/new",
			SrcPath:         "frontend/src/routes/new.tsx",
			OutPath:         "vorma_out/routes/new.js",
			ExportKey:       "default",
		},
	})
	stageOneFile := filepath.Join(
		fixture.privateDir,
		VormaOutDirname,
		VormaPathsStageOneJSONFileName,
	)
	stageTwoFile := filepath.Join(
		fixture.privateDir,
		VormaOutDirname,
		VormaPathsStageTwoJSONFileName,
	)
	mustWriteJSONFile(t, stageOneFile, updatedStage)
	mustWriteJSONFile(t, stageTwoFile, updatedStage)

	previousTemplateLocation := app.Config.HTMLTemplateLocation
	app.Config.HTMLTemplateLocation = "missing-template.go.html"
	defer func() {
		app.Config.HTMLTemplateLocation = previousTemplateLocation
	}()

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected Init() to panic when template is missing")
			}
		}()
		app.MustInit()
	}()

	afterOldReq := httptest.NewRequest(http.MethodGet, "/old", nil)
	afterOldRec := httptest.NewRecorder()
	handler.ServeHTTP(afterOldRec, afterOldReq)
	if afterOldRec.Code != http.StatusOK {
		t.Fatalf(
			"post-failure /old status = %d, want %d",
			afterOldRec.Code,
			http.StatusOK,
		)
	}
	if got := afterOldRec.Header().Get(VormaBuildIDHeaderKey); got != "atomic-old-build" {
		t.Fatalf(
			"post-failure build header = %q, want %q",
			got,
			"atomic-old-build",
		)
	}

	afterNewReq := httptest.NewRequest(http.MethodGet, "/new", nil)
	afterNewRec := httptest.NewRecorder()
	handler.ServeHTTP(afterNewRec, afterNewReq)
	if afterNewRec.Code != http.StatusNotFound {
		t.Fatalf(
			"post-failure /new status = %d, want %d",
			afterNewRec.Code,
			http.StatusNotFound,
		)
	}
}

func TestInit_ReinitMalformedStageFileDoesNotPartiallyMutateRuntimeState(
	t *testing.T,
) {
	initialStage := defaultPathsFile("malformed-old-build", map[string]*Path{
		"/old": {
			OriginalPattern: "/old",
			SrcPath:         "frontend/src/routes/old.tsx",
			OutPath:         "vorma_out/routes/old.js",
			ExportKey:       "default",
		},
	})
	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: initialStage,
		stageTwo: initialStage,
	})
	app := fixture.app
	handler := mux.InjectTasksCtxMiddleware(app.Loaders().Handler())

	beforeReq := httptest.NewRequest(http.MethodGet, "/old", nil)
	beforeRec := httptest.NewRecorder()
	handler.ServeHTTP(beforeRec, beforeReq)
	if beforeRec.Code != http.StatusOK {
		t.Fatalf(
			"pre-failure /old status = %d, want %d",
			beforeRec.Code,
			http.StatusOK,
		)
	}
	if got := beforeRec.Header().Get(VormaBuildIDHeaderKey); got != "malformed-old-build" {
		t.Fatalf(
			"pre-failure build header = %q, want %q",
			got,
			"malformed-old-build",
		)
	}

	stageTwoFile := filepath.Join(
		fixture.privateDir,
		VormaOutDirname,
		VormaPathsStageTwoJSONFileName,
	)
	mustWriteFile(t, stageTwoFile, []byte("{"))

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected Init() to panic on malformed stage file")
			}
			msg := r.(error).Error()
			if !strings.Contains(msg, "could not decode") {
				t.Fatalf(
					"panic message = %q, expected decode failure context",
					msg,
				)
			}
		}()
		app.MustInit()
	}()

	afterOldReq := httptest.NewRequest(http.MethodGet, "/old", nil)
	afterOldRec := httptest.NewRecorder()
	handler.ServeHTTP(afterOldRec, afterOldReq)
	if afterOldRec.Code != http.StatusOK {
		t.Fatalf(
			"post-failure /old status = %d, want %d",
			afterOldRec.Code,
			http.StatusOK,
		)
	}
	if got := afterOldRec.Header().Get(VormaBuildIDHeaderKey); got != "malformed-old-build" {
		t.Fatalf(
			"post-failure build header = %q, want %q",
			got,
			"malformed-old-build",
		)
	}
}
