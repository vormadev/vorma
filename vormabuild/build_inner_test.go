package vormabuild

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vormadev/vorma/internal/vormaruntime"
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
	defaultDependencies := defaultBuildInnerDependencies()
	newDependencies := func() buildInnerDependencies {
		return defaultDependencies
	}

	t.Run("runs all build steps in order", func(t *testing.T) {
		var observedStepOrder []string
		dependencies := newDependencies()
		dependencies.initializeBuildInnerState = func(_ *vormaruntime.Vorma, options *buildInnerOptions) error {
			if !options.isDev {
				t.Fatal("expected buildInner options to preserve isDev value")
			}
			observedStepOrder = append(observedStepOrder, "initialize")
			return nil
		}
		dependencies.parseAndSyncClientRoutes = func(*vormaruntime.Vorma) error {
			observedStepOrder = append(observedStepOrder, "parse-and-sync")
			return nil
		}
		dependencies.cleanStaticPublicOutDir = func(*vormaruntime.Vorma) error {
			observedStepOrder = append(observedStepOrder, "clean-public")
			return nil
		}
		dependencies.writePublicFileMapTypeScript = func(*vormaruntime.Vorma) error {
			observedStepOrder = append(observedStepOrder, "write-file-map-ts")
			return nil
		}
		dependencies.writeRouteArtifacts = func(*vormaruntime.Vorma) error {
			observedStepOrder = append(observedStepOrder, "write-route-artifacts")
			return nil
		}
		dependencies.logBuildInnerCompletion = func(*vormaruntime.Vorma, time.Time) {
			observedStepOrder = append(observedStepOrder, "log-completion")
		}

		err := buildInnerWithDependencies(
			&vormaruntime.Vorma{},
			&buildInnerOptions{isDev: true},
			dependencies,
		)
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

	t.Run("runs non-commit build steps without holding runtime write lock", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		dependencies := newDependencies()
		dependencies.initializeBuildInnerState = func(v *vormaruntime.Vorma, _ *buildInnerOptions) error {
			assertRuntimeWriteLockCanBeAcquiredPromptly(
				t,
				v,
				"initializeBuildInnerState",
			)
			return nil
		}
		dependencies.parseAndSyncClientRoutes = func(v *vormaruntime.Vorma) error {
			assertRuntimeWriteLockCanBeAcquiredPromptly(
				t,
				v,
				"parseAndSyncClientRoutes",
			)
			return nil
		}
		dependencies.cleanStaticPublicOutDir = func(v *vormaruntime.Vorma) error {
			assertRuntimeWriteLockCanBeAcquiredPromptly(
				t,
				v,
				"cleanStaticPublicOutDir",
			)
			return nil
		}
		dependencies.writePublicFileMapTypeScript = func(v *vormaruntime.Vorma) error {
			assertRuntimeWriteLockCanBeAcquiredPromptly(
				t,
				v,
				"writePublicFileMapTypeScript",
			)
			return nil
		}
		dependencies.writeRouteArtifacts = func(v *vormaruntime.Vorma) error {
			assertRuntimeWriteLockCanBeAcquiredPromptly(
				t,
				v,
				"writeRouteArtifacts",
			)
			return nil
		}
		dependencies.logBuildInnerCompletion = func(*vormaruntime.Vorma, time.Time) {}

		if err := buildInnerWithDependencies(
			app,
			&buildInnerOptions{isDev: true},
			dependencies,
		); err != nil {
			t.Fatalf("buildInner returned error: %v", err)
		}
	})

	t.Run("defaults to non-dev mode when options are nil", func(t *testing.T) {
		initializeCalled := false
		dependencies := newDependencies()
		dependencies.initializeBuildInnerState = func(_ *vormaruntime.Vorma, options *buildInnerOptions) error {
			initializeCalled = true
			if options == nil {
				t.Fatal("expected buildInner to normalize nil options")
			}
			if options.isDev {
				t.Fatal("expected nil options to default to non-dev mode")
			}
			return nil
		}
		dependencies.parseAndSyncClientRoutes = func(*vormaruntime.Vorma) error { return nil }
		dependencies.cleanStaticPublicOutDir = func(*vormaruntime.Vorma) error { return nil }
		dependencies.writePublicFileMapTypeScript = func(*vormaruntime.Vorma) error { return nil }
		dependencies.writeRouteArtifacts = func(*vormaruntime.Vorma) error { return nil }
		dependencies.logBuildInnerCompletion = func(*vormaruntime.Vorma, time.Time) {}

		if err := buildInnerWithDependencies(&vormaruntime.Vorma{}, nil, dependencies); err != nil {
			t.Fatalf("buildInner returned error with nil options: %v", err)
		}
		if !initializeCalled {
			t.Fatal("expected initializeBuildInnerState to be called")
		}
	})

	t.Run("restores runtime state snapshot when a step fails", func(t *testing.T) {
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
				fixture := newBuildTestFixture(t, nil)
				app := fixture.app

				const baselineBuildID = "build-before-inner-failure"
				const baselineRouteManifestFile = "route-manifest-before-inner-failure.json"
				const currentAttemptBuildID = "build-current-attempt"
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
						l.SetBuildID(currentAttemptBuildID)
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
				dependencies := newDependencies()
				dependencies.initializeBuildInnerState = func(v *vormaruntime.Vorma, _ *buildInnerOptions) error {
					if failureCase.failingStep == "initialize" {
						return expectedErr
					}
					mutateRuntimeStateForBuildInnerFailure(v, "initialize")
					return nil
				}
				dependencies.parseAndSyncClientRoutes = func(v *vormaruntime.Vorma) error {
					mutateRuntimeStateForBuildInnerFailure(v, "parse")
					if failureCase.failingStep == "parse" {
						return expectedErr
					}
					return nil
				}
				dependencies.cleanStaticPublicOutDir = func(v *vormaruntime.Vorma) error {
					mutateRuntimeStateForBuildInnerFailure(v, "clean")
					if failureCase.failingStep == "clean" {
						return expectedErr
					}
					return nil
				}
				dependencies.writePublicFileMapTypeScript = func(v *vormaruntime.Vorma) error {
					mutateRuntimeStateForBuildInnerFailure(v, "write-public-file-map")
					if failureCase.failingStep == "write-public-file-map" {
						return expectedErr
					}
					return nil
				}
				dependencies.writeRouteArtifacts = func(v *vormaruntime.Vorma) error {
					mutateRuntimeStateForBuildInnerFailure(v, "write-route-artifacts")
					if failureCase.failingStep == "write-route-artifacts" {
						return expectedErr
					}
					return nil
				}
				dependencies.logBuildInnerCompletion = func(*vormaruntime.Vorma, time.Time) {
					t.Fatal("did not expect completion log when buildInner step fails")
				}

				err := buildInnerWithDependencies(
					app,
					&buildInnerOptions{isDev: true},
					dependencies,
				)
				if err == nil {
					t.Fatal("expected buildInner to return error")
				}
				if failureCase.expectedErrorText != "" && !strings.Contains(err.Error(), failureCase.expectedErrorText) {
					t.Fatalf("error = %q, expected %q context", err, failureCase.expectedErrorText)
				}
				if !errors.Is(err, expectedErr) {
					t.Fatalf("error = %v, expected wrapped step error", err)
				}

				if app.IsDevMode() {
					t.Fatal("expected runtime state rollback to restore isDev=false")
				}
				if got := app.BuildID(); got != baselineBuildID {
					t.Fatalf("build ID after rollback = %q, want %q", got, baselineBuildID)
				}
				if got := app.RouteManifestFile(); got != baselineRouteManifestFile {
					t.Fatalf(
						"route manifest after rollback = %q, want %q",
						got,
						baselineRouteManifestFile,
					)
				}
				paths := app.Paths()
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
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		const baselineBuildID = "build-before-inner-panic"
		const baselineRouteManifestFile = "route-manifest-before-inner-panic.json"
		const currentAttemptBuildID = "build-current-panic-attempt"
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
				l.SetBuildID(currentAttemptBuildID)
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
		dependencies := newDependencies()
		dependencies.initializeBuildInnerState = func(v *vormaruntime.Vorma, _ *buildInnerOptions) error {
			mutateRuntimeStateForBuildInnerPanic(v, "initialize")
			return nil
		}
		dependencies.parseAndSyncClientRoutes = func(v *vormaruntime.Vorma) error {
			mutateRuntimeStateForBuildInnerPanic(v, "parse")
			panic(expectedPanic)
		}
		dependencies.cleanStaticPublicOutDir = func(*vormaruntime.Vorma) error {
			t.Fatal("did not expect clean step after parse panic")
			return nil
		}
		dependencies.logBuildInnerCompletion = func(*vormaruntime.Vorma, time.Time) {
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

			if app.IsDevMode() {
				t.Fatal("expected panic rollback to restore isDev=false")
			}
			if got := app.BuildID(); got != baselineBuildID {
				t.Fatalf("build ID after panic rollback = %q, want %q", got, baselineBuildID)
			}
			if got := app.RouteManifestFile(); got != baselineRouteManifestFile {
				t.Fatalf(
					"route manifest after panic rollback = %q, want %q",
					got,
					baselineRouteManifestFile,
				)
			}
			paths := app.Paths()
			if len(paths) != 1 {
				t.Fatalf("paths after panic rollback length = %d, want 1 (%#v)", len(paths), paths)
			}
			if paths["/existing"] == nil {
				t.Fatalf("expected /existing path after panic rollback, got %#v", paths)
			}
		}()

		_ = buildInnerWithDependencies(
			app,
			&buildInnerOptions{isDev: true},
			dependencies,
		)
	})

	t.Run("skips rollback and re-panics when build ID token is superseded before panic rollback", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		app.WithLock(func(l *vormaruntime.LockedVorma) {
			l.SetIsDev(false)
			l.SetBuildID("build-before")
			l.SetRouteManifestFile("route-manifest-before.json")
			l.SetPaths(map[string]*vormaruntime.Path{
				"/before": {
					OriginalPattern: "/before",
					SrcPath:         "frontend/src/routes/before.tsx",
					ExportKey:       "default",
				},
			})
		})

		expectedPanic := errors.New("parse panic after newer build committed")
		dependencies := newDependencies()
		dependencies.getCurrentBuildIDWithReadLock = func(*vormaruntime.Vorma) string {
			return "build-attempt"
		}
		dependencies.initializeBuildInnerState = func(v *vormaruntime.Vorma, _ *buildInnerOptions) error {
			v.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetIsDev(true)
				l.SetBuildID("build-attempt")
				l.SetRouteManifestFile("route-manifest-attempt.json")
				l.SetPaths(map[string]*vormaruntime.Path{
					"/attempt": {
						OriginalPattern: "/attempt",
						SrcPath:         "frontend/src/routes/attempt.tsx",
						ExportKey:       "default",
					},
				})
			})
			return nil
		}
		dependencies.parseAndSyncClientRoutes = func(v *vormaruntime.Vorma) error {
			v.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetIsDev(true)
				l.SetBuildID("build-after-newer-sync")
				l.SetRouteManifestFile("route-manifest-after-newer-sync.json")
				l.SetPaths(map[string]*vormaruntime.Path{
					"/newer": {
						OriginalPattern: "/newer",
						SrcPath:         "frontend/src/routes/newer.tsx",
						ExportKey:       "default",
					},
				})
			})
			panic(expectedPanic)
		}
		dependencies.cleanStaticPublicOutDir = func(*vormaruntime.Vorma) error {
			t.Fatal("did not expect clean step after parse panic")
			return nil
		}
		dependencies.logBuildInnerCompletion = func(*vormaruntime.Vorma, time.Time) {
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

			if !app.IsDevMode() {
				t.Fatal("expected newer runtime state to remain in dev mode after stale panic rollback skip")
			}
			if got := app.BuildID(); got != "build-after-newer-sync" {
				t.Fatalf("build ID after stale panic rollback skip = %q, want %q", got, "build-after-newer-sync")
			}
			if got := app.RouteManifestFile(); got != "route-manifest-after-newer-sync.json" {
				t.Fatalf(
					"route manifest after stale panic rollback skip = %q, want %q",
					got,
					"route-manifest-after-newer-sync.json",
				)
			}
			paths := app.Paths()
			if len(paths) != 1 {
				t.Fatalf("paths after stale panic rollback skip length = %d, want 1 (%#v)", len(paths), paths)
			}
			if paths["/newer"] == nil {
				t.Fatalf("expected newer path to remain after stale panic rollback skip, got %#v", paths)
			}
		}()

		_ = buildInnerWithDependencies(
			app,
			&buildInnerOptions{isDev: true},
			dependencies,
		)
	})

	t.Run("skips rollback when build ID token is superseded before failure rollback", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		app.WithLock(func(l *vormaruntime.LockedVorma) {
			l.SetIsDev(false)
			l.SetBuildID("build-before")
			l.SetRouteManifestFile("route-manifest-before.json")
			l.SetPaths(map[string]*vormaruntime.Path{
				"/before": {
					OriginalPattern: "/before",
					SrcPath:         "frontend/src/routes/before.tsx",
					ExportKey:       "default",
				},
			})
		})

		expectedErr := errors.New("parse failed after newer build committed")
		dependencies := newDependencies()
		dependencies.getCurrentBuildIDWithReadLock = func(*vormaruntime.Vorma) string {
			return "build-attempt"
		}
		dependencies.initializeBuildInnerState = func(v *vormaruntime.Vorma, _ *buildInnerOptions) error {
			v.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetIsDev(true)
				l.SetBuildID("build-attempt")
				l.SetRouteManifestFile("route-manifest-attempt.json")
				l.SetPaths(map[string]*vormaruntime.Path{
					"/attempt": {
						OriginalPattern: "/attempt",
						SrcPath:         "frontend/src/routes/attempt.tsx",
						ExportKey:       "default",
					},
				})
			})
			return nil
		}
		dependencies.parseAndSyncClientRoutes = func(v *vormaruntime.Vorma) error {
			v.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetIsDev(true)
				l.SetBuildID("build-after-newer-sync")
				l.SetRouteManifestFile("route-manifest-after-newer-sync.json")
				l.SetPaths(map[string]*vormaruntime.Path{
					"/newer": {
						OriginalPattern: "/newer",
						SrcPath:         "frontend/src/routes/newer.tsx",
						ExportKey:       "default",
					},
				})
			})
			return expectedErr
		}
		dependencies.cleanStaticPublicOutDir = func(*vormaruntime.Vorma) error {
			t.Fatal("did not expect clean step after parse failure")
			return nil
		}
		dependencies.writePublicFileMapTypeScript = func(*vormaruntime.Vorma) error {
			t.Fatal("did not expect public file map write after parse failure")
			return nil
		}
		dependencies.writeRouteArtifacts = func(*vormaruntime.Vorma) error {
			t.Fatal("did not expect route artifact write after parse failure")
			return nil
		}
		dependencies.logBuildInnerCompletion = func(*vormaruntime.Vorma, time.Time) {
			t.Fatal("did not expect completion log when buildInner fails")
		}

		err := buildInnerWithDependencies(
			app,
			&buildInnerOptions{isDev: true},
			dependencies,
		)
		if err == nil {
			t.Fatal("expected buildInner to return parse error")
		}
		if !strings.Contains(err.Error(), "parse client routes") {
			t.Fatalf("error = %q, expected parse context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped parse error", err)
		}

		if !app.IsDevMode() {
			t.Fatal("expected newer runtime state to remain in dev mode after stale rollback skip")
		}
		if got := app.BuildID(); got != "build-after-newer-sync" {
			t.Fatalf("build ID after stale rollback skip = %q, want %q", got, "build-after-newer-sync")
		}
		if got := app.RouteManifestFile(); got != "route-manifest-after-newer-sync.json" {
			t.Fatalf(
				"route manifest after stale rollback skip = %q, want %q",
				got,
				"route-manifest-after-newer-sync.json",
			)
		}
		paths := app.Paths()
		if len(paths) != 1 {
			t.Fatalf("paths after stale rollback skip length = %d, want 1 (%#v)", len(paths), paths)
		}
		if paths["/newer"] == nil {
			t.Fatalf("expected newer path to remain after stale rollback skip, got %#v", paths)
		}
	})

	t.Run("preserves newer runtime state when overlapping build attempts race", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		app.WithLock(func(l *vormaruntime.LockedVorma) {
			l.SetIsDev(false)
			l.SetBuildID("build-before")
			l.SetRouteManifestFile("manifest-before.json")
			l.SetPaths(map[string]*vormaruntime.Path{
				"/before": {
					OriginalPattern: "/before",
					SrcPath:         "frontend/src/routes/before.tsx",
					ExportKey:       "default",
				},
			})
		})

		firstBuildParseStarted := make(chan struct{})
		allowFirstBuildParseFailure := make(chan struct{})
		firstBuildErrCh := make(chan error, 1)
		firstBuildExpectedErr := errors.New("first build parse failed")

		firstBuildDependencies := newDependencies()
		firstBuildDependencies.initializeBuildInnerState = func(v *vormaruntime.Vorma, _ *buildInnerOptions) error {
			v.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetIsDev(true)
				l.SetBuildID("build-first-attempt")
				l.SetRouteManifestFile("manifest-first-attempt.json")
				l.SetPaths(map[string]*vormaruntime.Path{
					"/first": {
						OriginalPattern: "/first",
						SrcPath:         "frontend/src/routes/first.tsx",
						ExportKey:       "default",
					},
				})
			})
			return nil
		}
		firstBuildDependencies.parseAndSyncClientRoutes = func(*vormaruntime.Vorma) error {
			close(firstBuildParseStarted)
			<-allowFirstBuildParseFailure
			return firstBuildExpectedErr
		}
		firstBuildDependencies.cleanStaticPublicOutDir = func(*vormaruntime.Vorma) error {
			t.Fatal("did not expect clean step in first build after parse failure")
			return nil
		}
		firstBuildDependencies.writePublicFileMapTypeScript = func(*vormaruntime.Vorma) error {
			t.Fatal("did not expect write public file map step in first build after parse failure")
			return nil
		}
		firstBuildDependencies.writeRouteArtifacts = func(*vormaruntime.Vorma) error {
			t.Fatal("did not expect write route artifacts step in first build after parse failure")
			return nil
		}
		firstBuildDependencies.logBuildInnerCompletion = func(*vormaruntime.Vorma, time.Time) {
			t.Fatal("did not expect completion log for failed first build")
		}

		secondBuildDependencies := newDependencies()
		secondBuildDependencies.initializeBuildInnerState = func(v *vormaruntime.Vorma, _ *buildInnerOptions) error {
			v.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetIsDev(true)
				l.SetBuildID("build-second-attempt")
				l.SetRouteManifestFile("manifest-second-attempt.json")
				l.SetPaths(map[string]*vormaruntime.Path{
					"/second": {
						OriginalPattern: "/second",
						SrcPath:         "frontend/src/routes/second.tsx",
						ExportKey:       "default",
					},
				})
			})
			return nil
		}
		secondBuildDependencies.parseAndSyncClientRoutes = func(*vormaruntime.Vorma) error {
			return nil
		}
		secondBuildDependencies.cleanStaticPublicOutDir = func(*vormaruntime.Vorma) error {
			return nil
		}
		secondBuildDependencies.writePublicFileMapTypeScript = func(*vormaruntime.Vorma) error {
			return nil
		}
		secondBuildDependencies.writeRouteArtifacts = func(*vormaruntime.Vorma) error {
			return nil
		}
		secondBuildDependencies.logBuildInnerCompletion = func(*vormaruntime.Vorma, time.Time) {}

		go func() {
			firstBuildErrCh <- buildInnerWithDependencies(
				app,
				&buildInnerOptions{isDev: true},
				firstBuildDependencies,
			)
		}()

		select {
		case <-firstBuildParseStarted:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for first build parse step to start")
		}

		if err := buildInnerWithDependencies(
			app,
			&buildInnerOptions{isDev: true},
			secondBuildDependencies,
		); err != nil {
			t.Fatalf("second concurrent buildInner returned error: %v", err)
		}

		close(allowFirstBuildParseFailure)

		select {
		case err := <-firstBuildErrCh:
			if err == nil {
				t.Fatal("expected first concurrent buildInner to return parse error")
			}
			if !strings.Contains(err.Error(), "parse client routes") {
				t.Fatalf("first concurrent build error = %q, expected parse context", err)
			}
			if !errors.Is(err, firstBuildExpectedErr) {
				t.Fatalf("first concurrent build error = %v, expected wrapped parse error", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for first concurrent build to return")
		}

		if !app.IsDevMode() {
			t.Fatal("expected newer runtime state from second build to remain in dev mode")
		}
		if got := app.BuildID(); got != "build-second-attempt" {
			t.Fatalf("build ID after overlapping builds = %q, want %q", got, "build-second-attempt")
		}
		if got := app.RouteManifestFile(); got != "manifest-second-attempt.json" {
			t.Fatalf(
				"route manifest after overlapping builds = %q, want %q",
				got,
				"manifest-second-attempt.json",
			)
		}
		paths := app.Paths()
		if len(paths) != 1 {
			t.Fatalf("paths after overlapping builds length = %d, want 1 (%#v)", len(paths), paths)
		}
		if paths["/second"] == nil {
			t.Fatalf("expected second-build path to remain after overlapping builds, got %#v", paths)
		}
	})

	t.Run("returns initialization error directly", func(t *testing.T) {
		expectedErr := errors.New("initialize failed")
		dependencies := newDependencies()
		dependencies.initializeBuildInnerState = func(*vormaruntime.Vorma, *buildInnerOptions) error {
			return expectedErr
		}
		dependencies.parseAndSyncClientRoutes = func(*vormaruntime.Vorma) error {
			t.Fatal("did not expect dependencies.parseAndSyncClientRoutes after initialization error")
			return nil
		}

		err := buildInnerWithDependencies(
			&vormaruntime.Vorma{},
			&buildInnerOptions{},
			dependencies,
		)
		if err == nil {
			t.Fatal("expected initialization error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped initialization error", err)
		}
	})

	t.Run("wraps parse-and-sync errors", func(t *testing.T) {
		dependencies := newDependencies()
		dependencies.initializeBuildInnerState = func(*vormaruntime.Vorma, *buildInnerOptions) error {
			return nil
		}
		expectedErr := errors.New("parse failed")
		dependencies.parseAndSyncClientRoutes = func(*vormaruntime.Vorma) error {
			return expectedErr
		}

		err := buildInnerWithDependencies(
			&vormaruntime.Vorma{},
			&buildInnerOptions{},
			dependencies,
		)
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
		dependencies := newDependencies()
		dependencies.initializeBuildInnerState = func(*vormaruntime.Vorma, *buildInnerOptions) error {
			return nil
		}
		dependencies.parseAndSyncClientRoutes = func(*vormaruntime.Vorma) error {
			return nil
		}
		expectedErr := errors.New("clean failed")
		dependencies.cleanStaticPublicOutDir = func(*vormaruntime.Vorma) error {
			return expectedErr
		}

		err := buildInnerWithDependencies(
			&vormaruntime.Vorma{},
			&buildInnerOptions{},
			dependencies,
		)
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
		dependencies := newDependencies()
		dependencies.initializeBuildInnerState = func(*vormaruntime.Vorma, *buildInnerOptions) error {
			return nil
		}
		dependencies.parseAndSyncClientRoutes = func(*vormaruntime.Vorma) error {
			return nil
		}
		dependencies.cleanStaticPublicOutDir = func(*vormaruntime.Vorma) error {
			return nil
		}
		expectedErr := errors.New("write file map failed")
		dependencies.writePublicFileMapTypeScript = func(*vormaruntime.Vorma) error {
			return expectedErr
		}

		err := buildInnerWithDependencies(
			&vormaruntime.Vorma{},
			&buildInnerOptions{},
			dependencies,
		)
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
		dependencies := newDependencies()
		dependencies.initializeBuildInnerState = func(*vormaruntime.Vorma, *buildInnerOptions) error {
			return nil
		}
		dependencies.parseAndSyncClientRoutes = func(*vormaruntime.Vorma) error {
			return nil
		}
		dependencies.cleanStaticPublicOutDir = func(*vormaruntime.Vorma) error {
			return nil
		}
		dependencies.writePublicFileMapTypeScript = func(*vormaruntime.Vorma) error {
			return nil
		}
		expectedErr := errors.New("write route artifacts failed")
		dependencies.writeRouteArtifacts = func(*vormaruntime.Vorma) error {
			return expectedErr
		}

		err := buildInnerWithDependencies(
			&vormaruntime.Vorma{},
			&buildInnerOptions{},
			dependencies,
		)
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

func TestShouldRollbackBuildInnerRuntimeStateAfterFailure(t *testing.T) {
	if !shouldRollbackBuildInnerRuntimeStateAfterFailure("any-build", "") {
		t.Fatal("expected rollback when no attempt build ID token was captured")
	}
	if !shouldRollbackBuildInnerRuntimeStateAfterFailure("build-id", "build-id") {
		t.Fatal("expected rollback when current build ID matches attempt build ID token")
	}
	if shouldRollbackBuildInnerRuntimeStateAfterFailure("build-current", "build-attempt") {
		t.Fatal("expected rollback to skip when current build ID differs from attempt build ID token")
	}
}

func TestWritePublicFileMapTypeScript(t *testing.T) {
	t.Run("writes TS into configured output directory", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		writer := &fakeBuildInnerPublicFileMapWriter{}
		dependencies := buildInnerPublicFileMapDependencies{
			newPublicFileMapWriter: func(v *vormaruntime.Vorma) buildInnerPublicFileMapWriter {
				if v != app {
					t.Fatalf("writer received app %p, want %p", v, app)
				}
				return writer
			},
		}

		if err := writePublicFileMapTypeScriptWithDependencies(app, dependencies); err != nil {
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
		dependencies := buildInnerPublicFileMapDependencies{
			newPublicFileMapWriter: func(*vormaruntime.Vorma) buildInnerPublicFileMapWriter {
				return writer
			},
		}

		err := writePublicFileMapTypeScriptWithDependencies(app, dependencies)
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
		dependencies := buildInnerPublicFileMapDependencies{
			newPublicFileMapWriter: func(*vormaruntime.Vorma) buildInnerPublicFileMapWriter {
				return writer
			},
		}

		err := writePublicFileMapTypeScriptWithDependencies(app, dependencies)
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
		dependencies := buildInnerPublicFileMapDependencies{
			newPublicFileMapWriter: func(*vormaruntime.Vorma) buildInnerPublicFileMapWriter {
				return writer
			},
		}

		err := writePublicFileMapTypeScriptWithDependencies(app, dependencies)
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
	t.Run("returns parse error", func(t *testing.T) {
		expectedErr := errors.New("parse failed")
		dependencies := buildInnerRouteSyncDependencies{
			parseClientRoutes: func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
				return nil, expectedErr
			},
		}

		err := parseAndSyncClientRoutesWithDependencies(&vormaruntime.Vorma{}, dependencies)
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

		dependencies := buildInnerRouteSyncDependencies{
			parseClientRoutes: func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
				return map[string]*vormaruntime.Path{
					"/synced": {
						OriginalPattern: "/synced",
						SrcPath:         "frontend/src/routes/synced.tsx",
						ExportKey:       "default",
					},
				}, nil
			},
		}

		if err := parseAndSyncClientRoutesWithDependencies(app, dependencies); err != nil {
			t.Fatalf("parseAndSyncClientRoutes returned error: %v", err)
		}
		if app.Paths()["/synced"] == nil {
			t.Fatal("expected parsed route to be synced into app paths")
		}
	})
}

func TestInitializeBuildInnerState(t *testing.T) {
	t.Run("production mode marks app as non-dev and skips build ID generation", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		app.WithLock(func(l *vormaruntime.LockedVorma) {
			l.SetBuildID("existing-build-id")
		})
		dependencies := buildInnerBuildIDDependencies{
			generateDevBuildIDSuffix: func() (string, error) {
				t.Fatal("did not expect dev build ID generation in production mode")
				return "", nil
			},
		}

		if err := initializeBuildInnerStateWithBuildIDDependencies(
			app,
			&buildInnerOptions{isDev: false},
			dependencies,
		); err != nil {
			t.Fatalf("initializeBuildInnerState returned error: %v", err)
		}
		if app.IsDevMode() {
			t.Fatal("expected app not to be in dev mode after production initialization")
		}
		if app.BuildID() != "existing-build-id" {
			t.Fatalf("build ID = %q, want %q", app.BuildID(), "existing-build-id")
		}
	})

	t.Run("development mode sets prefixed build ID from generated suffix", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		dependencies := buildInnerBuildIDDependencies{
			generateDevBuildIDSuffix: func() (string, error) {
				return "stubid", nil
			},
		}

		if err := initializeBuildInnerStateWithBuildIDDependencies(
			app,
			&buildInnerOptions{isDev: true},
			dependencies,
		); err != nil {
			t.Fatalf("initializeBuildInnerState returned error: %v", err)
		}
		if !app.IsDevMode() {
			t.Fatal("expected app to be in dev mode after development initialization")
		}
		if app.BuildID() != "dev_stubid" {
			t.Fatalf("build ID = %q, want %q", app.BuildID(), "dev_stubid")
		}
	})

	t.Run("development mode does not expose mixed runtime state while build ID is pending", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		app.SetIsDev(false)
		app.WithLock(func(l *vormaruntime.LockedVorma) {
			l.SetBuildID("build-before-dev-init")
		})

		buildIDGenerationStarted := make(chan struct{})
		continueBuildIDGeneration := make(chan struct{})
		releaseBuildIDGeneration := func() {
			select {
			case <-continueBuildIDGeneration:
			default:
				close(continueBuildIDGeneration)
			}
		}
		defer releaseBuildIDGeneration()

		dependencies := buildInnerBuildIDDependencies{
			generateDevBuildIDSuffix: func() (string, error) {
				close(buildIDGenerationStarted)
				<-continueBuildIDGeneration
				return "stubid", nil
			},
		}

		initializeErrCh := make(chan error, 1)
		go func() {
			initializeErrCh <- initializeBuildInnerStateWithBuildIDDependencies(
				app,
				&buildInnerOptions{isDev: true},
				dependencies,
			)
		}()

		<-buildIDGenerationStarted

		if app.IsDevMode() {
			t.Fatal("expected runtime to keep previous isDev value until build state commits atomically")
		}
		if app.BuildID() != "build-before-dev-init" {
			t.Fatalf(
				"build ID during pending dev initialization = %q, want %q",
				app.BuildID(),
				"build-before-dev-init",
			)
		}

		releaseBuildIDGeneration()
		select {
		case err := <-initializeErrCh:
			if err != nil {
				t.Fatalf("initializeBuildInnerState returned error: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for initializeBuildInnerState to return")
		}
	})

	t.Run("development mode wraps build ID generation error", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedErr := errors.New("id generation failed")
		dependencies := buildInnerBuildIDDependencies{
			generateDevBuildIDSuffix: func() (string, error) {
				return "", expectedErr
			},
		}

		err := initializeBuildInnerStateWithBuildIDDependencies(
			app,
			&buildInnerOptions{isDev: true},
			dependencies,
		)
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
