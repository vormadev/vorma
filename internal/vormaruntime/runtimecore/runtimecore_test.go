package runtimecore

import (
	"reflect"
	"sync"
	"testing"
)

func TestBuildRuntimeRouteArtifacts(t *testing.T) {
	t.Run("nil_paths_file", func(t *testing.T) {
		_, err := BuildRuntimeRouteArtifacts(nil)
		if err == nil {
			t.Fatal(
				"expected error when building artifacts from nil paths file",
			)
		}
	})

	t.Run("maps_paths_file_fields_to_runtime_artifacts", func(t *testing.T) {
		pathsFile := &RuntimePathsFileSnapshot{
			BuildID:        "build-artifacts",
			ClientEntrySrc: "frontend/src/main.tsx",
			ClientEntryOut: "vorma_out/main.js",
			ClientEntryDeps: []string{
				"vorma_out/chunk-shared.js",
			},
			DepToCSSBundleMap: map[string][]string{
				"vorma_out/main.js": {"vorma_out/main.css"},
			},
			RouteManifestFile: "vorma_out/route-manifest.js",
			Paths: map[string]*RoutePath{
				"/products/:id": {
					OriginalPattern: "/products/:id",
					SrcPath:         "frontend/src/routes/products.$id.tsx",
					OutPath:         "vorma_out/routes/products.$id.js",
					ExportKey:       "default",
					Deps:            []string{"vorma_out/products.js"},
				},
			},
		}

		artifacts, err := BuildRuntimeRouteArtifacts(pathsFile)
		if err != nil {
			t.Fatalf("BuildRuntimeRouteArtifacts: %v", err)
		}
		if got, want := artifacts.BuildID, pathsFile.BuildID; got != want {
			t.Fatalf("BuildID = %q, want %q", got, want)
		}
		if got, want := artifacts.ClientEntrySrc, pathsFile.ClientEntrySrc; got != want {
			t.Fatalf("ClientEntrySrc = %q, want %q", got, want)
		}
		if got, want := artifacts.ClientEntryOut, pathsFile.ClientEntryOut; got != want {
			t.Fatalf("ClientEntryOut = %q, want %q", got, want)
		}
		if got, want := artifacts.RouteManifestFile, pathsFile.RouteManifestFile; got != want {
			t.Fatalf("RouteManifestFile = %q, want %q", got, want)
		}
		if got, want := artifacts.ClientEntryDeps, pathsFile.ClientEntryDeps; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("ClientEntryDeps = %#v, want %#v", got, want)
		}
		if got, want := artifacts.DepToCSSBundleMap, pathsFile.DepToCSSBundleMap; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("DepToCSSBundleMap = %#v, want %#v", got, want)
		}
		if got, want := artifacts.ParsedClientPaths, pathsFile.Paths; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("ParsedClientPaths = %#v, want %#v", got, want)
		}
	})
}

func TestApplyRuntimeRouteArtifactsMetadata(t *testing.T) {
	state := &RuntimeRouteMetadataState{}
	ApplyRuntimeRouteArtifactsMetadata(state, &RuntimeRouteArtifacts{
		BuildID:         "build-new",
		ClientEntrySrc:  "frontend/src/main.tsx",
		ClientEntryOut:  "vorma_out/main.js",
		ClientEntryDeps: []string{"vorma_out/chunk-shared.js"},
		DepToCSSBundleMap: map[string][]string{
			"vorma_out/main.js": {"vorma_out/main.css"},
		},
		RouteManifestFile: "vorma_out/route-manifest.js",
	})

	if got, want := state.BuildID, "build-new"; got != want {
		t.Fatalf("BuildID = %q, want %q", got, want)
	}
	if got := state.ClientEntryDeps; !reflect.DeepEqual(
		got,
		[]string{"vorma_out/chunk-shared.js"},
	) {
		t.Fatalf("ClientEntryDeps = %#v", got)
	}

	ApplyRuntimeRouteArtifactsMetadata(state, nil)
	if got, want := state.BuildID, ""; got != want {
		t.Fatalf("BuildID after nil artifacts = %q, want %q", got, want)
	}
	if state.DepToCSSBundleMap == nil {
		t.Fatal("DepToCSSBundleMap should be non-nil after nil artifact reset")
	}
}

func TestInvalidateRouteDataCache(t *testing.T) {
	state := &RouteCacheState{
		RouteDataSnapshotVersion: 4,
		RouteDataCache:           &sync.Map{},
	}
	state.RouteDataCache.Store("old", "value")

	InvalidateRouteDataCache(state)

	if got, want := state.RouteDataSnapshotVersion, uint64(5); got != want {
		t.Fatalf("RouteDataSnapshotVersion = %d, want %d", got, want)
	}
	found := false
	state.RouteDataCache.Range(func(_, _ any) bool {
		found = true
		return false
	})
	if found {
		t.Fatal("expected route-data cache to be empty after invalidation")
	}
}

func TestSyncPathsFromDevReload_MergesServerRoutesAndClones(t *testing.T) {
	input := map[string]*RoutePath{
		"/client": {
			OriginalPattern: "/client",
			SrcPath:         "frontend/src/routes/client.tsx",
			OutPath:         "vorma_out/routes/client.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/chunk-client.js"},
		},
	}

	merged := SyncPathsFromDevReload(input, []string{"/server-only"})
	if _, ok := merged["/server-only"]; !ok {
		t.Fatal("expected /server-only route to be merged")
	}
	if got := merged["/server-only"].ExportKey; got != "default" {
		t.Fatalf("server-only ExportKey = %q, want %q", got, "default")
	}

	input["/client"].SrcPath = "MUTATED"
	input["/client"].Deps[0] = "MUTATED_DEP"
	if got := merged["/client"].SrcPath; got != "frontend/src/routes/client.tsx" {
		t.Fatalf("SrcPath = %q, want original value", got)
	}
	if got := merged["/client"].Deps[0]; got != "vorma_out/chunk-client.js" {
		t.Fatalf("Deps[0] = %q, want original value", got)
	}
}

func TestReplaceParsedPathsForInit_Clones(t *testing.T) {
	input := map[string]*RoutePath{
		"/docs": {
			OriginalPattern: "/docs",
			OutPath:         "vorma_out/routes/docs.js",
			Deps:            []string{"vorma_out/chunk-docs.js"},
		},
	}
	cloned := ReplaceParsedPathsForInit(input)
	input["/docs"].OutPath = "MUTATED"
	input["/docs"].Deps[0] = "MUTATED_DEP"

	if got := cloned["/docs"].OutPath; got != "vorma_out/routes/docs.js" {
		t.Fatalf("OutPath = %q, want original value", got)
	}
	if got := cloned["/docs"].Deps[0]; got != "vorma_out/chunk-docs.js" {
		t.Fatalf("Deps[0] = %q, want original value", got)
	}
}

func TestCloneRoutePathAndRouteMaps_NilAndDeepCopySemantics(t *testing.T) {
	if got := CloneRoutePath(nil); got != nil {
		t.Fatalf("CloneRoutePath(nil) = %#v, want nil", got)
	}

	originalPaths := map[string]*RoutePath{
		"/pricing": {
			OriginalPattern: "/pricing",
			SrcPath:         "frontend/src/routes/pricing.tsx",
			OutPath:         "vorma_out/routes/pricing.js",
			ExportKey:       "Pricing",
			ErrorExportKey:  "PricingError",
			Deps:            []string{"vorma_out/chunk-pricing.js"},
		},
	}

	clonedPaths := CloneRoutePaths(originalPaths)
	if got := CloneRoutePaths(nil); got == nil {
		t.Fatal("CloneRoutePaths(nil) should return a non-nil empty map")
	}

	originalPaths["/pricing"].SrcPath = "MUTATED_SRC"
	originalPaths["/pricing"].Deps[0] = "MUTATED_DEP"
	if got, want := clonedPaths["/pricing"].SrcPath, "frontend/src/routes/pricing.tsx"; got != want {
		t.Fatalf("cloned SrcPath = %q, want %q", got, want)
	}
	if got, want := clonedPaths["/pricing"].Deps[0], "vorma_out/chunk-pricing.js"; got != want {
		t.Fatalf("cloned Deps[0] = %q, want %q", got, want)
	}

	if got := CloneRoutePathsOrNil(nil); got != nil {
		t.Fatalf("CloneRoutePathsOrNil(nil) = %#v, want nil", got)
	}
}

func TestCloneStringAndBundleMaps_NilAndDeepCopySemantics(t *testing.T) {
	if got := CloneStringSliceOrNil(nil); got != nil {
		t.Fatalf("CloneStringSliceOrNil(nil) = %#v, want nil", got)
	}

	originalStrings := []string{"one", "two"}
	clonedStrings := CloneStringSliceOrNil(originalStrings)
	originalStrings[0] = "MUTATED"
	if got, want := clonedStrings[0], "one"; got != want {
		t.Fatalf("cloned string slice first value = %q, want %q", got, want)
	}

	if got := CloneDepToCSSBundleMapOrEmpty(nil); got == nil {
		t.Fatal("CloneDepToCSSBundleMapOrEmpty(nil) should return empty map")
	}
	if got := CloneDepToCSSBundleMapOrNil(nil); got != nil {
		t.Fatalf("CloneDepToCSSBundleMapOrNil(nil) = %#v, want nil", got)
	}

	originalBundleMap := map[string][]string{
		"vorma_out/main.js": {"vorma_out/main.css"},
	}
	clonedBundleMap := CloneDepToCSSBundleMapOrNil(originalBundleMap)
	originalBundleMap["vorma_out/main.js"][0] = "MUTATED"
	if got, want := clonedBundleMap["vorma_out/main.js"][0], "vorma_out/main.css"; got != want {
		t.Fatalf("cloned bundle map first value = %q, want %q", got, want)
	}
}

func TestBuildNestedRouterPatternList_Sorted(t *testing.T) {
	paths := map[string]*RoutePath{
		"/z": nil,
		"/a": nil,
		"/m": nil,
	}
	got := BuildNestedRouterPatternList(paths)
	want := []string{"/a", "/m", "/z"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("patterns = %#v, want %#v", got, want)
	}
}

func TestLifecycleStateFunctions(t *testing.T) {
	tracker := &LifecycleStateTracker{CurrentState: LifecycleStateUninitialized}
	result := TransitionLifecycleState(
		tracker,
		LifecycleStateReloadingRoutes,
		"note",
		"",
	)
	if got, want := result.PreviousState, LifecycleStateUninitialized; got != want {
		t.Fatalf("PreviousState = %q, want %q", got, want)
	}
	if got, want := result.CurrentState, LifecycleStateReloadingRoutes; got != want {
		t.Fatalf("CurrentState = %q, want %q", got, want)
	}
	if got, want := result.TransitionSeq, uint64(1); got != want {
		t.Fatalf("TransitionSeq = %d, want %d", got, want)
	}

	if got, want := LifecycleStateForRouteCommit(nil), LifecycleStateInitializing; got != want {
		t.Fatalf("LifecycleStateForRouteCommit(nil) = %q, want %q", got, want)
	}
	if got, want := LifecycleStateForRouteCommit(map[string]*RoutePath{}), LifecycleStateReloadingRoutes; got != want {
		t.Fatalf(
			"LifecycleStateForRouteCommit(non-nil) = %q, want %q",
			got,
			want,
		)
	}
}
