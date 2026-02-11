package vormaruntime

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/vormadev/vorma/kit/mux"
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
