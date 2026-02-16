package vormabuild

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vormadev/vorma/internal/vormaruntime"
)

func TestSyncClientRoutesFromParsedPathsWithLock(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	t.Run("syncs paths and updates build ID when provided", func(t *testing.T) {
		err := syncClientRoutesFromParsedPathsWithLock(
			app,
			map[string]*vormaruntime.Path{
				"/home": {
					OriginalPattern: "/home",
					SrcPath:         "frontend/src/routes/home.tsx",
					ExportKey:       "default",
				},
			},
			"dev_fast_build",
			nil,
		)
		if err != nil {
			t.Fatalf("syncClientRoutesFromParsedPathsWithLock returned error: %v", err)
		}

		if got := app.GetBuildID(); got != "dev_fast_build" {
			t.Fatalf("build ID = %q, want %q", got, "dev_fast_build")
		}

		paths := app.GetPathsSnapshot()
		if _, ok := paths["/home"]; !ok {
			t.Fatalf("expected /home path after sync, got %#v", paths)
		}
	})

	t.Run("post-sync hook runs without holding runtime write lock", func(t *testing.T) {
		err := syncClientRoutesFromParsedPathsWithLock(
			app,
			map[string]*vormaruntime.Path{
				"/lock-check": {
					OriginalPattern: "/lock-check",
					SrcPath:         "frontend/src/routes/lock-check.tsx",
					ExportKey:       "default",
				},
			},
			"dev_fast_lock_check",
			func(*vormaruntime.Vorma) error {
				readLockAcquired := make(chan struct{})
				go func() {
					app.WithRLock(func(*vormaruntime.ReadLockedVorma) {})
					close(readLockAcquired)
				}()

				select {
				case <-readLockAcquired:
					return nil
				case <-time.After(100 * time.Millisecond):
					return errors.New("post-sync hook executed while runtime write lock was held")
				}
			},
		)
		if err != nil {
			t.Fatalf(
				"syncClientRoutesFromParsedPathsWithLock returned error: %v",
				err,
			)
		}
	})

	t.Run("propagates post-sync hook error", func(t *testing.T) {
		expectedErr := errors.New("post-sync failed")
		wasCalled := false
		app.WithLock(func(l *vormaruntime.LockedVorma) {
			l.SetBuildID("build-before-failed-post-sync")
			l.SetRouteManifestFile("manifest-before-failed-post-sync.json")
			l.SetPaths(map[string]*vormaruntime.Path{
				"/existing": {
					OriginalPattern: "/existing",
					SrcPath:         "frontend/src/routes/existing.tsx",
					ExportKey:       "default",
				},
			})
		})

		err := syncClientRoutesFromParsedPathsWithLock(
			app,
			map[string]*vormaruntime.Path{
				"/about": {
					OriginalPattern: "/about",
					SrcPath:         "frontend/src/routes/about.tsx",
					ExportKey:       "default",
				},
			},
			"build-after-sync-but-before-post-sync",
			func(v *vormaruntime.Vorma) error {
				wasCalled = true
				v.WithLock(func(l *vormaruntime.LockedVorma) {
					l.SetRouteManifestFile("manifest-set-in-failing-post-sync-hook.json")
					if l.GetBuildID() != "build-after-sync-but-before-post-sync" {
						t.Fatalf(
							"build ID before post-sync failure = %q, want %q",
							l.GetBuildID(),
							"build-after-sync-but-before-post-sync",
						)
					}
					if _, ok := l.GetPaths()["/about"]; !ok {
						t.Fatalf("expected /about path to be synced before post-sync hook, got %#v", l.GetPaths())
					}
				})
				return expectedErr
			},
		)
		if err == nil {
			t.Fatal("expected syncClientRoutesFromParsedPathsWithLock to return hook error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped hook error", err)
		}
		if !wasCalled {
			t.Fatal("expected post-sync hook to be called")
		}

		if got := app.GetBuildID(); got != "build-before-failed-post-sync" {
			t.Fatalf("build ID after failed post-sync = %q, want %q", got, "build-before-failed-post-sync")
		}
		if got := app.GetRouteManifestFile(); got != "manifest-before-failed-post-sync.json" {
			t.Fatalf(
				"route manifest after failed post-sync = %q, want %q",
				got,
				"manifest-before-failed-post-sync.json",
			)
		}
		paths := app.GetPathsSnapshot()
		if paths["/existing"] == nil {
			t.Fatalf("expected rollback to restore /existing path, got %#v", paths)
		}
		if paths["/about"] != nil {
			t.Fatalf("expected rollback to remove failed /about sync path, got %#v", paths["/about"])
		}
	})

	t.Run("restores runtime state then re-panics when post-sync hook panics", func(t *testing.T) {
		app.WithLock(func(l *vormaruntime.LockedVorma) {
			l.SetBuildID("build-before-panic-post-sync")
			l.SetRouteManifestFile("manifest-before-panic-post-sync.json")
			l.SetPaths(map[string]*vormaruntime.Path{
				"/existing": {
					OriginalPattern: "/existing",
					SrcPath:         "frontend/src/routes/existing.tsx",
					ExportKey:       "default",
				},
			})
		})

		expectedPanic := errors.New("post-sync panic")
		defer func() {
			recoveredPanicValue := recover()
			if recoveredPanicValue == nil {
				t.Fatal("expected syncClientRoutesFromParsedPathsWithLock to panic")
			}
			recoveredPanicErr, ok := recoveredPanicValue.(error)
			if !ok {
				t.Fatalf("recovered panic type = %T, want error", recoveredPanicValue)
			}
			if !errors.Is(recoveredPanicErr, expectedPanic) {
				t.Fatalf("recovered panic = %v, want %v", recoveredPanicErr, expectedPanic)
			}

			if got := app.GetBuildID(); got != "build-before-panic-post-sync" {
				t.Fatalf("build ID after panic rollback = %q, want %q", got, "build-before-panic-post-sync")
			}
			if got := app.GetRouteManifestFile(); got != "manifest-before-panic-post-sync.json" {
				t.Fatalf(
					"route manifest after panic rollback = %q, want %q",
					got,
					"manifest-before-panic-post-sync.json",
				)
			}
			paths := app.GetPathsSnapshot()
			if paths["/existing"] == nil {
				t.Fatalf("expected /existing path after panic rollback, got %#v", paths)
			}
			if paths["/about"] != nil {
				t.Fatalf("expected /about path to be removed after panic rollback, got %#v", paths["/about"])
			}
		}()

		_ = syncClientRoutesFromParsedPathsWithLock(
			app,
			map[string]*vormaruntime.Path{
				"/about": {
					OriginalPattern: "/about",
					SrcPath:         "frontend/src/routes/about.tsx",
					ExportKey:       "default",
				},
			},
			"build-after-sync-before-panic-post-sync",
			func(v *vormaruntime.Vorma) error {
				v.WithLock(func(l *vormaruntime.LockedVorma) {
					l.SetRouteManifestFile("manifest-set-in-panicking-post-sync-hook.json")
				})
				panic(expectedPanic)
			},
		)
	})
}

func TestPrepareParsedRouteSyncInput(t *testing.T) {
	t.Run("returns parse error when parse context is empty", func(t *testing.T) {
		expectedErr := errors.New("parse failed")
		buildIDCalled := false

		_, _, err := prepareParsedRouteSyncInput(
			&vormaruntime.Vorma{},
			func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
				return nil, expectedErr
			},
			func() (string, error) {
				buildIDCalled = true
				return "", nil
			},
			"",
		)
		if err == nil {
			t.Fatal("expected parse error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped parse error", err)
		}
		if buildIDCalled {
			t.Fatal("did not expect build ID generation after parse error")
		}
	})

	t.Run("wraps parse error when parse context is provided", func(t *testing.T) {
		expectedErr := errors.New("parse failed")

		_, _, err := prepareParsedRouteSyncInput(
			&vormaruntime.Vorma{},
			func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
				return nil, expectedErr
			},
			nil,
			"parse client routes",
		)
		if err == nil {
			t.Fatal("expected parse error")
		}
		if !strings.Contains(err.Error(), "parse client routes") {
			t.Fatalf("error = %q, expected parse context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped parse error", err)
		}
	})

	t.Run("returns build ID generation error", func(t *testing.T) {
		expectedErr := errors.New("build ID failed")

		_, _, err := prepareParsedRouteSyncInput(
			&vormaruntime.Vorma{},
			func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
				return map[string]*vormaruntime.Path{}, nil
			},
			func() (string, error) {
				return "", expectedErr
			},
			"",
		)
		if err == nil {
			t.Fatal("expected build ID generation error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped build ID generation error", err)
		}
	})

	t.Run("returns parsed paths with generated build ID", func(t *testing.T) {
		expectedPaths := map[string]*vormaruntime.Path{
			"/ok": {
				OriginalPattern: "/ok",
				SrcPath:         "frontend/src/routes/ok.tsx",
				ExportKey:       "default",
			},
		}

		preparedPaths, preparedBuildID, err := prepareParsedRouteSyncInput(
			&vormaruntime.Vorma{},
			func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
				return expectedPaths, nil
			},
			func() (string, error) {
				return "dev_fast_stub", nil
			},
			"",
		)
		if err != nil {
			t.Fatalf("prepareParsedRouteSyncInput returned error: %v", err)
		}
		if preparedPaths["/ok"] == nil {
			t.Fatalf("prepared paths = %#v, want /ok path", preparedPaths)
		}
		if preparedBuildID != "dev_fast_stub" {
			t.Fatalf("prepared build ID = %q, want %q", preparedBuildID, "dev_fast_stub")
		}
	})

	t.Run("returns parsed paths with empty build ID when generator is nil", func(t *testing.T) {
		preparedPaths, preparedBuildID, err := prepareParsedRouteSyncInput(
			&vormaruntime.Vorma{},
			func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
				return map[string]*vormaruntime.Path{
					"/ok": {
						OriginalPattern: "/ok",
						SrcPath:         "frontend/src/routes/ok.tsx",
						ExportKey:       "default",
					},
				}, nil
			},
			nil,
			"",
		)
		if err != nil {
			t.Fatalf("prepareParsedRouteSyncInput returned error: %v", err)
		}
		if preparedPaths["/ok"] == nil {
			t.Fatalf("prepared paths = %#v, want /ok path", preparedPaths)
		}
		if preparedBuildID != "" {
			t.Fatalf("prepared build ID = %q, want empty string", preparedBuildID)
		}
	})
}

func TestRunRouteSyncExecution(t *testing.T) {
	t.Run("returns error when parse function is missing", func(t *testing.T) {
		err := runRouteSyncExecution(&vormaruntime.Vorma{}, routeSyncExecutionOptions{})
		if err == nil {
			t.Fatal("expected runRouteSyncExecution to return error when parse function is missing")
		}
		if !strings.Contains(err.Error(), "route sync parse function is required") {
			t.Fatalf("error = %q, expected missing-parse-function context", err)
		}
	})

	t.Run("wraps parse error using provided parse context", func(t *testing.T) {
		expectedErr := errors.New("parse failed")
		err := runRouteSyncExecution(
			&vormaruntime.Vorma{},
			routeSyncExecutionOptions{
				parseClientRoutes: func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
					return nil, expectedErr
				},
				parseClientRoutesErrorText: "parse client routes",
			},
		)
		if err == nil {
			t.Fatal("expected runRouteSyncExecution to return parse error")
		}
		if !strings.Contains(err.Error(), "parse client routes") {
			t.Fatalf("error = %q, expected parse context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped parse error", err)
		}
	})

	t.Run("parses, generates build ID, syncs routes, then runs post-sync hook", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app
		var observedStepOrder []string

		err := runRouteSyncExecution(
			app,
			routeSyncExecutionOptions{
				parseClientRoutes: func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
					observedStepOrder = append(observedStepOrder, "parse")
					return map[string]*vormaruntime.Path{
						"/ok": {
							OriginalPattern: "/ok",
							SrcPath:         "frontend/src/routes/ok.tsx",
							ExportKey:       "default",
						},
					}, nil
				},
				generateBuildID: func() (string, error) {
					observedStepOrder = append(observedStepOrder, "build-id")
					return "dev_fast_stub", nil
				},
				postSyncHook: func(v *vormaruntime.Vorma) error {
					observedStepOrder = append(observedStepOrder, "post-sync")
					v.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
						if l.GetBuildID() != "dev_fast_stub" {
							t.Fatalf("build ID = %q, want %q", l.GetBuildID(), "dev_fast_stub")
						}
						if l.GetPaths()["/ok"] == nil {
							t.Fatalf("expected synced /ok route, got %#v", l.GetPaths())
						}
					})
					return nil
				},
			},
		)
		if err != nil {
			t.Fatalf("runRouteSyncExecution returned error: %v", err)
		}
		if got := strings.Join(observedStepOrder, ","); got != "parse,build-id,post-sync" {
			t.Fatalf("observed step order = %q, want %q", got, "parse,build-id,post-sync")
		}
	})
}
