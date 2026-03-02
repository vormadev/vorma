package buildlifecycle

import (
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
)

func TestCommitRuntimeStateWithLock_CommitsAllCoreBuildFields(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		CommitRuntimeStateWithLock(
			l,
			RuntimeStateCommitInput{
				ShouldCommitIsDev:             true,
				IsDev:                         true,
				ShouldCommitBuildID:           true,
				BuildID:                       "committed-build-id",
				ShouldCommitRouteManifestFile: true,
				RouteManifestFile:             "committed-manifest.json",
				RoutePaths: map[string]*vormaruntime.Path{
					"/committed": {
						OriginalPattern: "/committed",
						SrcPath:         "frontend/src/routes/committed.tsx",
						ExportKey:       "default",
					},
				},
				RoutePathsUpdateMode: RuntimeStateRoutePathsUpdateModeReplaceParsedPathsForInit,
			},
		)
	})

	if !app.IsDevMode() {
		t.Fatal("expected isDev mode to be committed to true")
	}
	if got := app.BuildID(); got != "committed-build-id" {
		t.Fatalf("build ID = %q, want %q", got, "committed-build-id")
	}
	if got := app.RouteManifestFile(); got != "committed-manifest.json" {
		t.Fatalf(
			"route manifest file = %q, want %q",
			got,
			"committed-manifest.json",
		)
	}

	paths := app.Paths()
	if _, hasCommittedPath := paths["/committed"]; !hasCommittedPath {
		t.Fatalf("expected committed path in runtime state, got %#v", paths)
	}
}

func TestCommitRuntimeStateWithLock_SyncFromDevReloadMergesServerOnlyHandlers(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/server-only",
		mux.TaskHandlerFromFunc(
			func(*mux.ReqData[mux.None]) (map[string]any, error) {
				return map[string]any{"ok": true}, nil
			},
		),
	)

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		CommitRuntimeStateWithLock(
			l,
			RuntimeStateCommitInput{
				RoutePaths: map[string]*vormaruntime.Path{
					"/client": {
						OriginalPattern: "/client",
						SrcPath:         "frontend/src/routes/client.tsx",
						ExportKey:       "default",
					},
				},
				RoutePathsUpdateMode: RuntimeStateRoutePathsUpdateModeSyncFromDevReload,
			},
		)
	})

	paths := app.Paths()
	if _, hasClientPath := paths["/client"]; !hasClientPath {
		t.Fatalf(
			"expected committed client path in runtime state, got %#v",
			paths,
		)
	}
	serverOnlyPath, hasServerOnlyPath := paths["/server-only"]
	if !hasServerOnlyPath {
		t.Fatalf(
			"expected server-only handler path to be merged in runtime state, got %#v",
			paths,
		)
	}
	if serverOnlyPath == nil {
		t.Fatalf(
			"expected non-nil server-only handler path, got %#v",
			serverOnlyPath,
		)
	}
	if serverOnlyPath.SrcPath != "" {
		t.Fatalf("server-only SrcPath = %q, want empty", serverOnlyPath.SrcPath)
	}
}

func TestCommitRuntimeState_AppliesMutationViaInternalLockingPath(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	CommitRuntimeState(
		app,
		RuntimeStateCommitInput{
			ShouldCommitBuildID: true,
			BuildID:             "lock-wrapper-build-id",
		},
	)

	if got := app.BuildID(); got != "lock-wrapper-build-id" {
		t.Fatalf("build ID = %q, want %q", got, "lock-wrapper-build-id")
	}
}

func TestCommitRuntimeStateWithLock_PanicsForUnsupportedRoutePathUpdateMode(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	defer func() {
		panicValue := recover()
		if panicValue == nil {
			t.Fatal(
				"expected CommitRuntimeStateWithLock to panic for unsupported route path mode",
			)
		}

		panicText := ""
		switch typedPanicValue := panicValue.(type) {
		case string:
			panicText = typedPanicValue
		case error:
			panicText = typedPanicValue.Error()
		default:
			panicText = "<non-string panic>"
		}
		if !strings.Contains(
			panicText,
			"unsupported runtime state route paths update mode",
		) {
			t.Fatalf(
				"panic text = %q, expected unsupported-route-mode context",
				panicText,
			)
		}
	}()

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		CommitRuntimeStateWithLock(
			l,
			RuntimeStateCommitInput{
				RoutePathsUpdateMode: RuntimeStateRoutePathsUpdateMode(
					255,
				),
			},
		)
	})
}

func TestShouldRestoreRuntimeStateSnapshotForAttemptBuildID(t *testing.T) {
	if !ShouldRestoreRuntimeStateSnapshotForAttemptBuildID(
		"build-id",
		"",
	) {
		t.Fatal(
			"expected runtime snapshot restore when no attempt build ID token is captured",
		)
	}
	if !ShouldRestoreRuntimeStateSnapshotForAttemptBuildID(
		"build-id",
		"build-id",
	) {
		t.Fatal(
			"expected runtime snapshot restore when current build ID matches attempt build ID token",
		)
	}
	if ShouldRestoreRuntimeStateSnapshotForAttemptBuildID(
		"build-current",
		"build-attempt",
	) {
		t.Fatal(
			"expected runtime snapshot restore to skip when build IDs differ",
		)
	}
}
