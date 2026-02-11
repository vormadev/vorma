package vormaruntime

import (
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
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
		t.Fatalf("stage one build ID = %q, want %q", gotStageOne.BuildID, "stage-one-build")
	}

	gotStageTwo, err := app.getBasePaths_StageOneOrTwo(false)
	if err != nil {
		t.Fatalf("getBasePaths_StageOneOrTwo(false) error = %v", err)
	}
	if gotStageTwo.BuildID != "stage-two-build" {
		t.Fatalf("stage two build ID = %q, want %q", gotStageTwo.BuildID, "stage-two-build")
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

	t.Run("NullPathEntry", func(t *testing.T) {
		app.WithLock(func(lv *LockedVorma) {
			lv.v._privateFS = fstest.MapFS{
				"vorma_out/" + VormaPathsStageOneJSONFileName: &fstest.MapFile{
					Data: []byte(`{"stage":"stage-one","buildID":"b","clientEntrySrc":"frontend/src/vorma.entry.tsx","paths":{"/bad":null},"routeManifestFile":"vorma_out/route-manifest.js"}`),
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
					Data: []byte(`{"stage":"stage-one","buildID":"b","clientEntrySrc":"frontend/src/vorma.entry.tsx","paths":{"/a":{"originalPattern":"/b","srcPath":"frontend/src/routes/a.tsx","exportKey":"default"}},"routeManifestFile":"vorma_out/route-manifest.js"}`),
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
					Data: []byte(`{"stage":"stage-one","buildID":"b","clientEntrySrc":"frontend/src/vorma.entry.tsx","paths":{"/a":{"srcPath":"frontend/src/routes/a.tsx","exportKey":"default"}},"routeManifestFile":"vorma_out/route-manifest.js"}`),
				},
			}
		})

		_, err := app.getBasePaths_StageOneOrTwo(true)
		if err == nil {
			t.Fatal("expected error for missing originalPattern")
		}
		if !strings.Contains(err.Error(), "originalPattern is required") {
			t.Fatalf("error = %q, expected missing originalPattern validation error", err)
		}
	})
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
		mux.RegisterNestedPatternWithoutHandler(nr, "/")
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
			t.Fatalf("panic message = %q, expected underlying parse error context", msg)
		}
	}()

	app.Init()
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

	paths := app.GetPathsSnapshot()
	if paths == nil {
		t.Fatal("GetPathsSnapshot() returned nil; expected empty map after init normalization")
	}
	if len(paths) != 0 {
		t.Fatalf("GetPathsSnapshot() len = %d, want 0", len(paths))
	}

	cssMap := app.GetDepToCSSBundleMap()
	if cssMap == nil {
		t.Fatal("GetDepToCSSBundleMap() returned nil; expected empty map after init normalization")
	}
	if len(cssMap) != 0 {
		t.Fatalf("GetDepToCSSBundleMap() len = %d, want 0", len(cssMap))
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

	app.Init()
}
