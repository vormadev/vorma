package vormaruntime

import (
	"reflect"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime/runtimecore"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
)

func TestBuildRuntimePathsFileSnapshot(t *testing.T) {
	t.Run("nil_paths_file_returns_nil_snapshot", func(t *testing.T) {
		if buildRuntimePathsFileSnapshot(nil) != nil {
			t.Fatal("expected nil snapshot for nil paths file input")
		}
	})

	t.Run("maps_paths_file_fields_to_runtimecore_snapshot", func(t *testing.T) {
		pathsFile := defaultPathsFile("build-artifacts", map[string]*Path{
			"/products/:id": {
				OriginalPattern: "/products/:id",
				SrcPath:         "frontend/src/routes/products.$id.tsx",
				OutPath:         "vorma_out/routes/products.$id.js",
				ExportKey:       "default",
				Deps:            []string{"vorma_out/products.js"},
			},
		})

		snapshot := buildRuntimePathsFileSnapshot(pathsFile)
		if snapshot == nil {
			t.Fatal("expected non-nil runtime paths snapshot")
		}
		if got, want := snapshot.BuildID, pathsFile.BuildID; got != want {
			t.Fatalf("BuildID = %q, want %q", got, want)
		}
		if got, want := snapshot.ClientEntrySrc, pathsFile.ClientEntrySrc; got != want {
			t.Fatalf("ClientEntrySrc = %q, want %q", got, want)
		}
		if got, want := snapshot.ClientEntryOut, pathsFile.ClientEntryOut; got != want {
			t.Fatalf("ClientEntryOut = %q, want %q", got, want)
		}
		if got, want := snapshot.RouteManifestFile, pathsFile.RouteManifestFile; got != want {
			t.Fatalf("RouteManifestFile = %q, want %q", got, want)
		}
		if got, want := snapshot.ClientEntryDeps, pathsFile.ClientEntryDeps; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("ClientEntryDeps = %#v, want %#v", got, want)
		}
		if got, want := snapshot.DepToCSSBundleMap, pathsFile.DepToCSSBundleMap; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("DepToCSSBundleMap = %#v, want %#v", got, want)
		}

		wantPaths := map[string]*runtimecore.RoutePath{
			"/products/:id": {
				OriginalPattern: "/products/:id",
				SrcPath:         "frontend/src/routes/products.$id.tsx",
				OutPath:         "vorma_out/routes/products.$id.js",
				ExportKey:       "default",
				Deps:            []string{"vorma_out/products.js"},
			},
		}
		if got := snapshot.Paths; !reflect.DeepEqual(got, wantPaths) {
			t.Fatalf("Paths = %#v, want %#v", got, wantPaths)
		}
	})
}

func TestRuntimeRoutePathConversionHelpers(t *testing.T) {
	pathValue := &Path{
		OriginalPattern: "/products/:id",
		SrcPath:         "frontend/src/routes/products.$id.tsx",
		OutPath:         "vorma_out/routes/products.$id.js",
		ExportKey:       "default",
		ErrorExportKey:  "ProductErrorBoundary",
		Deps:            []string{"vorma_out/products.js"},
	}

	runtimeCorePath := toRuntimeCoreRoutePath(pathValue)
	if runtimeCorePath == nil {
		t.Fatal("toRuntimeCoreRoutePath returned nil")
	}
	pathValue.Deps[0] = "mutated"
	if got, want := runtimeCorePath.Deps[0], "vorma_out/products.js"; got != want {
		t.Fatalf("runtimecore deps = %q, want %q", got, want)
	}

	convertedBack := fromRuntimeCoreRoutePath(runtimeCorePath)
	if convertedBack == nil {
		t.Fatal("fromRuntimeCoreRoutePath returned nil")
	}
	runtimeCorePath.Deps[0] = "runtimecore-mutated"
	if got, want := convertedBack.Deps[0], "vorma_out/products.js"; got != want {
		t.Fatalf("converted-back deps = %q, want %q", got, want)
	}
}

func TestRuntimeRoutePathMapConversionHelpers(t *testing.T) {
	paths := map[string]*Path{
		"/a": {
			OriginalPattern: "/a",
			SrcPath:         "frontend/src/routes/a.tsx",
			OutPath:         "vorma_out/routes/a.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/a.js"},
		},
		"/nil": nil,
	}

	runtimeCorePaths := toRuntimeCoreRoutePaths(paths)
	if runtimeCorePaths == nil {
		t.Fatal("toRuntimeCoreRoutePaths returned nil")
	}
	if runtimeCorePaths["/nil"] != nil {
		t.Fatalf(
			"expected nil entry to remain nil, got %#v",
			runtimeCorePaths["/nil"],
		)
	}
	paths["/a"].Deps[0] = "mutated"
	if got, want := runtimeCorePaths["/a"].Deps[0], "vorma_out/a.js"; got != want {
		t.Fatalf("runtimecore deps = %q, want %q", got, want)
	}

	convertedBack := fromRuntimeCoreRoutePaths(runtimeCorePaths)
	if convertedBack == nil {
		t.Fatal("fromRuntimeCoreRoutePaths returned nil")
	}
	runtimeCorePaths["/a"].Deps[0] = "runtimecore-mutated"
	if got, want := convertedBack["/a"].Deps[0], "vorma_out/a.js"; got != want {
		t.Fatalf("converted-back deps = %q, want %q", got, want)
	}
}

func TestToRuntimeCoreRoutePathsFromRuntimePaths_ConvertsAndClones(
	t *testing.T,
) {
	runtimePaths := map[string]*runtimepaths.RoutePath{
		"/a": {
			OriginalPattern: "/a",
			SrcPath:         "frontend/src/routes/a.tsx",
			OutPath:         "vorma_out/routes/a.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/a.js"},
		},
		"/nil": nil,
	}

	runtimeCorePaths := toRuntimeCoreRoutePathsFromRuntimePaths(runtimePaths)
	if runtimeCorePaths["/nil"] != nil {
		t.Fatalf(
			"expected nil entry to remain nil, got %#v",
			runtimeCorePaths["/nil"],
		)
	}
	runtimePaths["/a"].Deps[0] = "mutated"
	if got, want := runtimeCorePaths["/a"].Deps[0], "vorma_out/a.js"; got != want {
		t.Fatalf("runtimecore deps = %q, want %q", got, want)
	}
}

func TestClonePathsMapHelpers(t *testing.T) {
	paths := map[string]*Path{
		"/a": {
			OriginalPattern: "/a",
			SrcPath:         "frontend/src/routes/a.tsx",
			OutPath:         "vorma_out/routes/a.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/a.js"},
		},
	}

	cloned := clonePathsMap(paths)
	if got, want := cloned["/a"].OriginalPattern, "/a"; got != want {
		t.Fatalf("cloned original pattern = %q, want %q", got, want)
	}
	paths["/a"].Deps[0] = "mutated"
	if got, want := cloned["/a"].Deps[0], "vorma_out/a.js"; got != want {
		t.Fatalf("cloned deps = %q, want %q", got, want)
	}

	if got := clonePathsMapOrNil(nil); got != nil {
		t.Fatalf("clonePathsMapOrNil(nil) = %#v, want nil", got)
	}
}
