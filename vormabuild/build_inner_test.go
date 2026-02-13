package vormabuild

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vormadev/vorma/vormaruntime"
)

type fakeBuildInnerPublicFileMapWriter struct {
	writeErr       error
	closeErr       error
	writeCalled    bool
	closeCalled    bool
	receivedOutDir string
}

func (writer *fakeBuildInnerPublicFileMapWriter) WritePublicFileMapTS(outDir string) error {
	writer.writeCalled = true
	writer.receivedOutDir = outDir
	return writer.writeErr
}

func (writer *fakeBuildInnerPublicFileMapWriter) Close() error {
	writer.closeCalled = true
	return writer.closeErr
}

func TestBuildInner(t *testing.T) {
	restoreBuildInnerSteps := func(t *testing.T) {
		t.Helper()
		originalCaptureBuildInnerRuntimeStateStep := buildInnerDeps.captureBuildInnerRuntimeState
		originalRestoreBuildInnerRuntimeStateStep := buildInnerDeps.restoreBuildInnerRuntimeState
		originalInitializeBuildInnerStateStep := buildInnerDeps.initializeBuildInnerState
		originalParseAndSyncClientRoutesStep := buildInnerDeps.parseAndSyncClientRoutes
		originalCleanStaticPublicOutDirStep := buildInnerDeps.cleanStaticPublicOutDir
		originalWritePublicFileMapTypeScriptStep := buildInnerDeps.writePublicFileMapTypeScript
		originalWriteRouteArtifactsWithLockStep := buildInnerDeps.writeRouteArtifactsWithLock
		originalLogBuildInnerCompletionStep := buildInnerDeps.logBuildInnerCompletion
		originalParseClientRoutesForSync := buildInnerRouteSyncDeps.parseClientRoutes
		originalParseBackendLoaderPatternsForSync := buildInnerRouteSyncDeps.parseBackendLoaderPatterns
		originalMergeBackendLoaderPatternsInPathForSync := buildInnerRouteSyncDeps.mergeBackendLoaderPatternsInPath
		originalRunRouteSyncExecution := buildInnerRouteSyncDeps.runRouteSyncExecution
		originalGenerateDevBuildIDSuffixStep := buildInnerBuildIDDeps.generateDevBuildIDSuffix
		originalNewPublicFileMapWriterStep := buildInnerPublicFileMapDeps.newPublicFileMapWriter
		t.Cleanup(func() {
			buildInnerDeps.captureBuildInnerRuntimeState = originalCaptureBuildInnerRuntimeStateStep
			buildInnerDeps.restoreBuildInnerRuntimeState = originalRestoreBuildInnerRuntimeStateStep
			buildInnerDeps.initializeBuildInnerState = originalInitializeBuildInnerStateStep
			buildInnerDeps.parseAndSyncClientRoutes = originalParseAndSyncClientRoutesStep
			buildInnerDeps.cleanStaticPublicOutDir = originalCleanStaticPublicOutDirStep
			buildInnerDeps.writePublicFileMapTypeScript = originalWritePublicFileMapTypeScriptStep
			buildInnerDeps.writeRouteArtifactsWithLock = originalWriteRouteArtifactsWithLockStep
			buildInnerDeps.logBuildInnerCompletion = originalLogBuildInnerCompletionStep
			buildInnerRouteSyncDeps.parseClientRoutes = originalParseClientRoutesForSync
			buildInnerRouteSyncDeps.parseBackendLoaderPatterns = originalParseBackendLoaderPatternsForSync
			buildInnerRouteSyncDeps.mergeBackendLoaderPatternsInPath = originalMergeBackendLoaderPatternsInPathForSync
			buildInnerRouteSyncDeps.runRouteSyncExecution = originalRunRouteSyncExecution
			buildInnerBuildIDDeps.generateDevBuildIDSuffix = originalGenerateDevBuildIDSuffixStep
			buildInnerPublicFileMapDeps.newPublicFileMapWriter = originalNewPublicFileMapWriterStep
		})
	}

	t.Run("runs all build steps in order", func(t *testing.T) {
		restoreBuildInnerSteps(t)

		var observedStepOrder []string
		buildInnerDeps.initializeBuildInnerState = func(_ *vormaruntime.Vorma, options *buildInnerOptions) error {
			if !options.isDev {
				t.Fatal("expected buildInner options to preserve isDev value")
			}
			observedStepOrder = append(observedStepOrder, "initialize")
			return nil
		}
		buildInnerDeps.parseAndSyncClientRoutes = func(*vormaruntime.Vorma) error {
			observedStepOrder = append(observedStepOrder, "parse-and-sync")
			return nil
		}
		buildInnerDeps.cleanStaticPublicOutDir = func(*vormaruntime.Vorma) error {
			observedStepOrder = append(observedStepOrder, "clean-public")
			return nil
		}
		buildInnerDeps.writePublicFileMapTypeScript = func(*vormaruntime.Vorma) error {
			observedStepOrder = append(observedStepOrder, "write-file-map-ts")
			return nil
		}
		buildInnerDeps.writeRouteArtifactsWithLock = func(*vormaruntime.Vorma) error {
			observedStepOrder = append(observedStepOrder, "write-route-artifacts")
			return nil
		}
		buildInnerDeps.logBuildInnerCompletion = func(*vormaruntime.Vorma, time.Time) {
			observedStepOrder = append(observedStepOrder, "log-completion")
		}

		err := buildInner(&vormaruntime.Vorma{}, &buildInnerOptions{isDev: true})
		if err != nil {
			t.Fatalf("buildInner returned error: %v", err)
		}

		expectedStepOrder := []string{
			"initialize",
			"parse-and-sync",
			"clean-public",
			"write-file-map-ts",
			"write-route-artifacts",
			"log-completion",
		}
		if len(observedStepOrder) != len(expectedStepOrder) {
			t.Fatalf("observed step order length = %d, want %d (%#v)", len(observedStepOrder), len(expectedStepOrder), observedStepOrder)
		}
		for i := range expectedStepOrder {
			if observedStepOrder[i] != expectedStepOrder[i] {
				t.Fatalf("step %d = %q, want %q", i, observedStepOrder[i], expectedStepOrder[i])
			}
		}
	})

	t.Run("defaults to non-dev mode when options are nil", func(t *testing.T) {
		restoreBuildInnerSteps(t)

		initializeCalled := false
		buildInnerDeps.initializeBuildInnerState = func(_ *vormaruntime.Vorma, options *buildInnerOptions) error {
			initializeCalled = true
			if options == nil {
				t.Fatal("expected buildInner to normalize nil options")
			}
			if options.isDev {
				t.Fatal("expected nil options to default to non-dev mode")
			}
			return nil
		}
		buildInnerDeps.parseAndSyncClientRoutes = func(*vormaruntime.Vorma) error { return nil }
		buildInnerDeps.cleanStaticPublicOutDir = func(*vormaruntime.Vorma) error { return nil }
		buildInnerDeps.writePublicFileMapTypeScript = func(*vormaruntime.Vorma) error { return nil }
		buildInnerDeps.writeRouteArtifactsWithLock = func(*vormaruntime.Vorma) error { return nil }
		buildInnerDeps.logBuildInnerCompletion = func(*vormaruntime.Vorma, time.Time) {}

		if err := buildInner(&vormaruntime.Vorma{}, nil); err != nil {
			t.Fatalf("buildInner returned error with nil options: %v", err)
		}
		if !initializeCalled {
			t.Fatal("expected initializeBuildInnerState to be called")
		}
	})

	t.Run("restores runtime state snapshot when a step fails", func(t *testing.T) {
		restoreBuildInnerSteps(t)

		type buildStepFailureCase struct {
			name              string
			failingStep       string
			expectedErrorText string
		}

		failureCases := []buildStepFailureCase{
			{
				name:              "initialize fails",
				failingStep:       "initialize",
				expectedErrorText: "",
			},
			{
				name:              "parse-and-sync fails",
				failingStep:       "parse",
				expectedErrorText: "parse client routes",
			},
			{
				name:              "clean fails",
				failingStep:       "clean",
				expectedErrorText: "clean static public out dir",
			},
			{
				name:              "write public file map fails",
				failingStep:       "write-public-file-map",
				expectedErrorText: "write public file map TS",
			},
			{
				name:              "write route artifacts fails",
				failingStep:       "write-route-artifacts",
				expectedErrorText: "write route artifacts",
			},
		}

		for _, failureCase := range failureCases {
			failureCase := failureCase

			t.Run(failureCase.name, func(t *testing.T) {
				restoreBuildInnerSteps(t)

				fixture := newBuildTestFixture(t, nil)
				app := fixture.app

				const baselineBuildID = "build-before-inner-failure"
				const baselineRouteManifestFile = "route-manifest-before-inner-failure.json"
				baselinePaths := map[string]*vormaruntime.Path{
					"/existing": {
						OriginalPattern: "/existing",
						SrcPath:         "frontend/src/routes/existing.tsx",
						ExportKey:       "default",
					},
				}

				app.SetIsDev(false)
				app.WithLock(func(l *vormaruntime.LockedVorma) {
					l.SetBuildID(baselineBuildID)
					l.SetRouteManifestFile(baselineRouteManifestFile)
					l.SetPaths(baselinePaths)
				})

				mutateRuntimeStateForBuildInnerFailure := func(
					v *vormaruntime.Vorma,
					stateID string,
				) {
					v.SetIsDev(true)
					v.WithLock(func(l *vormaruntime.LockedVorma) {
						l.SetBuildID("build-after-" + stateID)
						l.SetRouteManifestFile("route-manifest-after-" + stateID + ".json")
						l.SetPaths(map[string]*vormaruntime.Path{
							"/mutated-" + stateID: {
								OriginalPattern: "/mutated-" + stateID,
								SrcPath:         "frontend/src/routes/mutated_" + stateID + ".tsx",
								ExportKey:       "default",
							},
						})
					})
				}

				expectedErr := errors.New("step failed")
				buildInnerDeps.initializeBuildInnerState = func(v *vormaruntime.Vorma, _ *buildInnerOptions) error {
					mutateRuntimeStateForBuildInnerFailure(v, "initialize")
					if failureCase.failingStep == "initialize" {
						return expectedErr
					}
					return nil
				}
				buildInnerDeps.parseAndSyncClientRoutes = func(v *vormaruntime.Vorma) error {
					mutateRuntimeStateForBuildInnerFailure(v, "parse")
					if failureCase.failingStep == "parse" {
						return expectedErr
					}
					return nil
				}
				buildInnerDeps.cleanStaticPublicOutDir = func(v *vormaruntime.Vorma) error {
					mutateRuntimeStateForBuildInnerFailure(v, "clean")
					if failureCase.failingStep == "clean" {
						return expectedErr
					}
					return nil
				}
				buildInnerDeps.writePublicFileMapTypeScript = func(v *vormaruntime.Vorma) error {
					mutateRuntimeStateForBuildInnerFailure(v, "write-public-file-map")
					if failureCase.failingStep == "write-public-file-map" {
						return expectedErr
					}
					return nil
				}
				buildInnerDeps.writeRouteArtifactsWithLock = func(v *vormaruntime.Vorma) error {
					mutateRuntimeStateForBuildInnerFailure(v, "write-route-artifacts")
					if failureCase.failingStep == "write-route-artifacts" {
						return expectedErr
					}
					return nil
				}
				buildInnerDeps.logBuildInnerCompletion = func(*vormaruntime.Vorma, time.Time) {
					t.Fatal("did not expect completion log when buildInner step fails")
				}

				err := buildInner(app, &buildInnerOptions{isDev: true})
				if err == nil {
					t.Fatal("expected buildInner to return error")
				}
				if failureCase.expectedErrorText != "" && !strings.Contains(err.Error(), failureCase.expectedErrorText) {
					t.Fatalf("error = %q, expected %q context", err, failureCase.expectedErrorText)
				}
				if !errors.Is(err, expectedErr) {
					t.Fatalf("error = %v, expected wrapped step error", err)
				}

				if app.GetIsDevMode() {
					t.Fatal("expected runtime state rollback to restore isDev=false")
				}
				if got := app.GetBuildID(); got != baselineBuildID {
					t.Fatalf("build ID after rollback = %q, want %q", got, baselineBuildID)
				}
				if got := app.GetRouteManifestFile(); got != baselineRouteManifestFile {
					t.Fatalf(
						"route manifest after rollback = %q, want %q",
						got,
						baselineRouteManifestFile,
					)
				}
				paths := app.GetPathsSnapshot()
				if len(paths) != 1 {
					t.Fatalf("paths after rollback length = %d, want 1 (%#v)", len(paths), paths)
				}
				if paths["/existing"] == nil {
					t.Fatalf("expected /existing path after rollback, got %#v", paths)
				}
			})
		}
	})

	t.Run("restores runtime state snapshot then re-panics when a step panics", func(t *testing.T) {
		restoreBuildInnerSteps(t)

		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		const baselineBuildID = "build-before-inner-panic"
		const baselineRouteManifestFile = "route-manifest-before-inner-panic.json"
		baselinePaths := map[string]*vormaruntime.Path{
			"/existing": {
				OriginalPattern: "/existing",
				SrcPath:         "frontend/src/routes/existing.tsx",
				ExportKey:       "default",
			},
		}

		app.SetIsDev(false)
		app.WithLock(func(l *vormaruntime.LockedVorma) {
			l.SetBuildID(baselineBuildID)
			l.SetRouteManifestFile(baselineRouteManifestFile)
			l.SetPaths(baselinePaths)
		})

		mutateRuntimeStateForBuildInnerPanic := func(
			v *vormaruntime.Vorma,
			stateID string,
		) {
			v.SetIsDev(true)
			v.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetBuildID("build-after-" + stateID)
				l.SetRouteManifestFile("route-manifest-after-" + stateID + ".json")
				l.SetPaths(map[string]*vormaruntime.Path{
					"/mutated-" + stateID: {
						OriginalPattern: "/mutated-" + stateID,
						SrcPath:         "frontend/src/routes/mutated_" + stateID + ".tsx",
						ExportKey:       "default",
					},
				})
			})
		}

		expectedPanic := errors.New("parse panic")
		buildInnerDeps.initializeBuildInnerState = func(v *vormaruntime.Vorma, _ *buildInnerOptions) error {
			mutateRuntimeStateForBuildInnerPanic(v, "initialize")
			return nil
		}
		buildInnerDeps.parseAndSyncClientRoutes = func(v *vormaruntime.Vorma) error {
			mutateRuntimeStateForBuildInnerPanic(v, "parse")
			panic(expectedPanic)
		}
		buildInnerDeps.cleanStaticPublicOutDir = func(*vormaruntime.Vorma) error {
			t.Fatal("did not expect clean step after parse panic")
			return nil
		}
		buildInnerDeps.logBuildInnerCompletion = func(*vormaruntime.Vorma, time.Time) {
			t.Fatal("did not expect completion log when buildInner panics")
		}

		defer func() {
			recoveredPanicValue := recover()
			if recoveredPanicValue == nil {
				t.Fatal("expected buildInner to panic")
			}
			recoveredPanicErr, ok := recoveredPanicValue.(error)
			if !ok {
				t.Fatalf("recovered panic type = %T, want error", recoveredPanicValue)
			}
			if !errors.Is(recoveredPanicErr, expectedPanic) {
				t.Fatalf("recovered panic = %v, want %v", recoveredPanicErr, expectedPanic)
			}

			if app.GetIsDevMode() {
				t.Fatal("expected panic rollback to restore isDev=false")
			}
			if got := app.GetBuildID(); got != baselineBuildID {
				t.Fatalf("build ID after panic rollback = %q, want %q", got, baselineBuildID)
			}
			if got := app.GetRouteManifestFile(); got != baselineRouteManifestFile {
				t.Fatalf(
					"route manifest after panic rollback = %q, want %q",
					got,
					baselineRouteManifestFile,
				)
			}
			paths := app.GetPathsSnapshot()
			if len(paths) != 1 {
				t.Fatalf("paths after panic rollback length = %d, want 1 (%#v)", len(paths), paths)
			}
			if paths["/existing"] == nil {
				t.Fatalf("expected /existing path after panic rollback, got %#v", paths)
			}
		}()

		_ = buildInner(app, &buildInnerOptions{isDev: true})
	})

	t.Run("returns initialization error directly", func(t *testing.T) {
		restoreBuildInnerSteps(t)

		expectedErr := errors.New("initialize failed")
		buildInnerDeps.initializeBuildInnerState = func(*vormaruntime.Vorma, *buildInnerOptions) error {
			return expectedErr
		}
		buildInnerDeps.parseAndSyncClientRoutes = func(*vormaruntime.Vorma) error {
			t.Fatal("did not expect buildInnerDeps.parseAndSyncClientRoutes after initialization error")
			return nil
		}

		err := buildInner(&vormaruntime.Vorma{}, &buildInnerOptions{})
		if err == nil {
			t.Fatal("expected initialization error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped initialization error", err)
		}
	})

	t.Run("wraps parse-and-sync errors", func(t *testing.T) {
		restoreBuildInnerSteps(t)

		buildInnerDeps.initializeBuildInnerState = func(*vormaruntime.Vorma, *buildInnerOptions) error {
			return nil
		}
		expectedErr := errors.New("parse failed")
		buildInnerDeps.parseAndSyncClientRoutes = func(*vormaruntime.Vorma) error {
			return expectedErr
		}

		err := buildInner(&vormaruntime.Vorma{}, &buildInnerOptions{})
		if err == nil {
			t.Fatal("expected parse-and-sync error")
		}
		if !strings.Contains(err.Error(), "parse client routes") {
			t.Fatalf("error = %q, expected parse context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped parse error", err)
		}
	})

	t.Run("wraps clean static public out dir errors", func(t *testing.T) {
		restoreBuildInnerSteps(t)

		buildInnerDeps.initializeBuildInnerState = func(*vormaruntime.Vorma, *buildInnerOptions) error {
			return nil
		}
		buildInnerDeps.parseAndSyncClientRoutes = func(*vormaruntime.Vorma) error {
			return nil
		}
		expectedErr := errors.New("clean failed")
		buildInnerDeps.cleanStaticPublicOutDir = func(*vormaruntime.Vorma) error {
			return expectedErr
		}

		err := buildInner(&vormaruntime.Vorma{}, &buildInnerOptions{})
		if err == nil {
			t.Fatal("expected clean-static-public error")
		}
		if !strings.Contains(err.Error(), "clean static public out dir") {
			t.Fatalf("error = %q, expected clean-static-public context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped clean-static-public error", err)
		}
	})

	t.Run("wraps write public file map TS errors", func(t *testing.T) {
		restoreBuildInnerSteps(t)

		buildInnerDeps.initializeBuildInnerState = func(*vormaruntime.Vorma, *buildInnerOptions) error {
			return nil
		}
		buildInnerDeps.parseAndSyncClientRoutes = func(*vormaruntime.Vorma) error {
			return nil
		}
		buildInnerDeps.cleanStaticPublicOutDir = func(*vormaruntime.Vorma) error {
			return nil
		}
		expectedErr := errors.New("write file map failed")
		buildInnerDeps.writePublicFileMapTypeScript = func(*vormaruntime.Vorma) error {
			return expectedErr
		}

		err := buildInner(&vormaruntime.Vorma{}, &buildInnerOptions{})
		if err == nil {
			t.Fatal("expected write-public-file-map-ts error")
		}
		if !strings.Contains(err.Error(), "write public file map TS") {
			t.Fatalf("error = %q, expected write-public-file-map-ts context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped write-public-file-map-ts error", err)
		}
	})

	t.Run("wraps write route artifacts errors", func(t *testing.T) {
		restoreBuildInnerSteps(t)

		buildInnerDeps.initializeBuildInnerState = func(*vormaruntime.Vorma, *buildInnerOptions) error {
			return nil
		}
		buildInnerDeps.parseAndSyncClientRoutes = func(*vormaruntime.Vorma) error {
			return nil
		}
		buildInnerDeps.cleanStaticPublicOutDir = func(*vormaruntime.Vorma) error {
			return nil
		}
		buildInnerDeps.writePublicFileMapTypeScript = func(*vormaruntime.Vorma) error {
			return nil
		}
		expectedErr := errors.New("write route artifacts failed")
		buildInnerDeps.writeRouteArtifactsWithLock = func(*vormaruntime.Vorma) error {
			return expectedErr
		}

		err := buildInner(&vormaruntime.Vorma{}, &buildInnerOptions{})
		if err == nil {
			t.Fatal("expected write-route-artifacts error")
		}
		if !strings.Contains(err.Error(), "write route artifacts") {
			t.Fatalf("error = %q, expected write-route-artifacts context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped write-route-artifacts error", err)
		}
	})
}

func TestWritePublicFileMapTypeScript(t *testing.T) {
	originalNewPublicFileMapWriterStep := buildInnerPublicFileMapDeps.newPublicFileMapWriter
	t.Cleanup(func() {
		buildInnerPublicFileMapDeps.newPublicFileMapWriter = originalNewPublicFileMapWriterStep
	})

	t.Run("writes TS into configured output directory", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		writer := &fakeBuildInnerPublicFileMapWriter{}
		buildInnerPublicFileMapDeps.newPublicFileMapWriter = func(v *vormaruntime.Vorma) buildInnerPublicFileMapWriter {
			if v != app {
				t.Fatalf("writer received app %p, want %p", v, app)
			}
			return writer
		}

		if err := writePublicFileMapTypeScript(app); err != nil {
			t.Fatalf("writePublicFileMapTypeScript returned error: %v", err)
		}
		if !writer.writeCalled {
			t.Fatal("expected WritePublicFileMapTS to be called")
		}
		if writer.receivedOutDir != app.Config.TSGenOutDir {
			t.Fatalf("out dir = %q, want %q", writer.receivedOutDir, app.Config.TSGenOutDir)
		}
		if !writer.closeCalled {
			t.Fatal("expected writer.Close to be called")
		}
	})

	t.Run("returns write error and still closes writer", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedErr := errors.New("write failed")
		writer := &fakeBuildInnerPublicFileMapWriter{
			writeErr: expectedErr,
		}
		buildInnerPublicFileMapDeps.newPublicFileMapWriter = func(*vormaruntime.Vorma) buildInnerPublicFileMapWriter {
			return writer
		}

		err := writePublicFileMapTypeScript(app)
		if err == nil {
			t.Fatal("expected writePublicFileMapTypeScript to return write error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped write error", err)
		}
		if !writer.closeCalled {
			t.Fatal("expected writer.Close to be called after write error")
		}
	})

	t.Run("returns close error when write succeeds", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedErr := errors.New("close failed")
		writer := &fakeBuildInnerPublicFileMapWriter{
			closeErr: expectedErr,
		}
		buildInnerPublicFileMapDeps.newPublicFileMapWriter = func(*vormaruntime.Vorma) buildInnerPublicFileMapWriter {
			return writer
		}

		err := writePublicFileMapTypeScript(app)
		if err == nil {
			t.Fatal("expected writePublicFileMapTypeScript to return close error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped close error", err)
		}
		if !strings.Contains(err.Error(), "close wave builder") {
			t.Fatalf("error = %q, expected close context", err)
		}
	})

	t.Run("joins close error when write fails", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedWriteErr := errors.New("write failed")
		expectedCloseErr := errors.New("close failed")
		writer := &fakeBuildInnerPublicFileMapWriter{
			writeErr: expectedWriteErr,
			closeErr: expectedCloseErr,
		}
		buildInnerPublicFileMapDeps.newPublicFileMapWriter = func(*vormaruntime.Vorma) buildInnerPublicFileMapWriter {
			return writer
		}

		err := writePublicFileMapTypeScript(app)
		if err == nil {
			t.Fatal("expected writePublicFileMapTypeScript to return joined write+close error")
		}
		if !errors.Is(err, expectedWriteErr) {
			t.Fatalf("error = %v, expected write error in joined chain", err)
		}
		if !errors.Is(err, expectedCloseErr) {
			t.Fatalf("error = %v, expected close error in joined chain", err)
		}
		if !strings.Contains(err.Error(), "close wave builder") {
			t.Fatalf("error = %q, expected close context", err)
		}
	})
}

func TestParseAndSyncClientRoutes(t *testing.T) {
	originalParseClientRoutesForSync := buildInnerRouteSyncDeps.parseClientRoutes
	originalRunRouteSyncExecution := buildInnerRouteSyncDeps.runRouteSyncExecution
	t.Cleanup(func() {
		buildInnerRouteSyncDeps.parseClientRoutes = originalParseClientRoutesForSync
		buildInnerRouteSyncDeps.runRouteSyncExecution = originalRunRouteSyncExecution
	})

	t.Run("returns parse error", func(t *testing.T) {
		expectedErr := errors.New("parse failed")
		buildInnerRouteSyncDeps.parseClientRoutes = func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
			return nil, expectedErr
		}

		err := parseAndSyncClientRoutes(&vormaruntime.Vorma{})
		if err == nil {
			t.Fatal("expected parseAndSyncClientRoutes to return error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped parse error", err)
		}
	})

	t.Run("syncs parsed paths into route registry", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		buildInnerRouteSyncDeps.parseClientRoutes = func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
			return map[string]*vormaruntime.Path{
				"/synced": {
					OriginalPattern: "/synced",
					SrcPath:         "frontend/src/routes/synced.tsx",
					ExportKey:       "default",
				},
			}, nil
		}

		if err := parseAndSyncClientRoutes(app); err != nil {
			t.Fatalf("parseAndSyncClientRoutes returned error: %v", err)
		}
		if app.GetPathsSnapshot()["/synced"] == nil {
			t.Fatal("expected parsed route to be synced into app paths")
		}
	})
}

func TestInitializeBuildInnerState(t *testing.T) {
	originalGenerateDevBuildIDSuffixStep := buildInnerBuildIDDeps.generateDevBuildIDSuffix
	t.Cleanup(func() {
		buildInnerBuildIDDeps.generateDevBuildIDSuffix = originalGenerateDevBuildIDSuffixStep
	})

	t.Run("production mode marks app as non-dev and skips build ID generation", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		app.WithLock(func(l *vormaruntime.LockedVorma) {
			l.SetBuildID("existing-build-id")
		})
		buildInnerBuildIDDeps.generateDevBuildIDSuffix = func() (string, error) {
			t.Fatal("did not expect dev build ID generation in production mode")
			return "", nil
		}

		if err := initializeBuildInnerState(app, &buildInnerOptions{isDev: false}); err != nil {
			t.Fatalf("initializeBuildInnerState returned error: %v", err)
		}
		if app.GetIsDevMode() {
			t.Fatal("expected app not to be in dev mode after production initialization")
		}
		if app.GetBuildID() != "existing-build-id" {
			t.Fatalf("build ID = %q, want %q", app.GetBuildID(), "existing-build-id")
		}
	})

	t.Run("development mode sets prefixed build ID from generated suffix", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		buildInnerBuildIDDeps.generateDevBuildIDSuffix = func() (string, error) {
			return "stubid", nil
		}

		if err := initializeBuildInnerState(app, &buildInnerOptions{isDev: true}); err != nil {
			t.Fatalf("initializeBuildInnerState returned error: %v", err)
		}
		if !app.GetIsDevMode() {
			t.Fatal("expected app to be in dev mode after development initialization")
		}
		if app.GetBuildID() != "dev_stubid" {
			t.Fatalf("build ID = %q, want %q", app.GetBuildID(), "dev_stubid")
		}
	})

	t.Run("development mode wraps build ID generation error", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedErr := errors.New("id generation failed")
		buildInnerBuildIDDeps.generateDevBuildIDSuffix = func() (string, error) {
			return "", expectedErr
		}

		err := initializeBuildInnerState(app, &buildInnerOptions{isDev: true})
		if err == nil {
			t.Fatal("expected initializeBuildInnerState to return build ID generation error")
		}
		if !strings.Contains(err.Error(), "generate build ID") {
			t.Fatalf("error = %q, expected generate-build-id context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped build ID generation error", err)
		}
	})
}
