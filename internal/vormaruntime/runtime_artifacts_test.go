package vormaruntime

import (
	"testing"
)

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
