package buildlifecycle

import (
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
)

func TestRouteBuildRuntimeStateSnapshotIsCurrent(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("build-snapshot")
	})

	if !RouteBuildRuntimeStateSnapshotIsCurrent(
		app,
		RouteBuildRuntimeStateSnapshot{
			BuildID: "build-snapshot",
		},
	) {
		t.Fatal("expected runtime snapshot to be current when build IDs match")
	}
	if RouteBuildRuntimeStateSnapshotIsCurrent(
		app,
		RouteBuildRuntimeStateSnapshot{
			BuildID: "different-build",
		},
	) {
		t.Fatal("expected runtime snapshot to be stale when build IDs differ")
	}
}

func TestShouldCommitRouteManifestFileForRuntimeState(t *testing.T) {
	if !ShouldCommitRouteManifestFileForRuntimeState(
		"build-id",
		"build-id",
	) {
		t.Fatal("expected manifest commit when build IDs match")
	}
	if ShouldCommitRouteManifestFileForRuntimeState(
		"build-id-current",
		"build-id-expected",
	) {
		t.Fatal("expected manifest commit to be rejected when build IDs differ")
	}
}
