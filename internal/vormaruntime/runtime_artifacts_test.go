package vormaruntime

import (
	"github.com/vormadev/vorma/internal/testhelpers/waveoutputtest"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime/routepublic"
)

func TestRuntimeRoutePathConversionHelpers(t *testing.T) {
	pathValue := &Path{
		OriginalPattern: "/products/:id",
		SrcPath:         "frontend/src/routes/products.$id.tsx",
		OutPath:         waveoutputtest.TestWaveOutputPath("routes/products.$id.js"),
		ExportKey:       "default",
		ErrorExportKey:  "ProductErrorBoundary",
		Deps:            []string{waveoutputtest.TestWaveOutputPath("products.js")},
	}

	runtimeCorePath := routepublic.ToRuntimeCoreRoutePath(pathValue)
	if runtimeCorePath == nil {
		t.Fatal("routepublic.ToRuntimeCoreRoutePath returned nil")
	}
	pathValue.Deps[0] = "mutated"
	if got, want := runtimeCorePath.Deps[0], waveoutputtest.TestWaveOutputPath("products.js"); got != want {
		t.Fatalf("runtimecore deps = %q, want %q", got, want)
	}

	convertedBack := routepublic.FromRuntimeCoreRoutePath(runtimeCorePath)
	if convertedBack == nil {
		t.Fatal("routepublic.FromRuntimeCoreRoutePath returned nil")
	}
	runtimeCorePath.Deps[0] = "runtimecore-mutated"
	if got, want := convertedBack.Deps[0], waveoutputtest.TestWaveOutputPath("products.js"); got != want {
		t.Fatalf("converted-back deps = %q, want %q", got, want)
	}
}

func TestRuntimeRoutePathMapConversionHelpers(t *testing.T) {
	paths := map[string]*Path{
		"/a": {
			OriginalPattern: "/a",
			SrcPath:         "frontend/src/routes/a.tsx",
			OutPath:         waveoutputtest.TestWaveOutputPath("routes/a.js"),
			ExportKey:       "default",
			Deps:            []string{waveoutputtest.TestWaveOutputPath("a.js")},
		},
		"/nil": nil,
	}

	runtimeCorePaths := routepublic.ToRuntimeCoreRoutePaths(paths)
	if runtimeCorePaths == nil {
		t.Fatal("routepublic.ToRuntimeCoreRoutePaths returned nil")
	}
	if runtimeCorePaths["/nil"] != nil {
		t.Fatalf(
			"expected nil entry to remain nil, got %#v",
			runtimeCorePaths["/nil"],
		)
	}
	paths["/a"].Deps[0] = "mutated"
	if got, want := runtimeCorePaths["/a"].Deps[0], waveoutputtest.TestWaveOutputPath("a.js"); got != want {
		t.Fatalf("runtimecore deps = %q, want %q", got, want)
	}

	convertedBack := routepublic.FromRuntimeCoreRoutePaths(runtimeCorePaths)
	if convertedBack == nil {
		t.Fatal("routepublic.FromRuntimeCoreRoutePaths returned nil")
	}
	runtimeCorePaths["/a"].Deps[0] = "runtimecore-mutated"
	if got, want := convertedBack["/a"].Deps[0], waveoutputtest.TestWaveOutputPath("a.js"); got != want {
		t.Fatalf("converted-back deps = %q, want %q", got, want)
	}
}

func TestClonePathsMapHelpers(t *testing.T) {
	paths := map[string]*Path{
		"/a": {
			OriginalPattern: "/a",
			SrcPath:         "frontend/src/routes/a.tsx",
			OutPath:         waveoutputtest.TestWaveOutputPath("routes/a.js"),
			ExportKey:       "default",
			Deps:            []string{waveoutputtest.TestWaveOutputPath("a.js")},
		},
	}

	cloned := routepublic.ClonePathsMap(paths)
	if got, want := cloned["/a"].OriginalPattern, "/a"; got != want {
		t.Fatalf("cloned original pattern = %q, want %q", got, want)
	}
	paths["/a"].Deps[0] = "mutated"
	if got, want := cloned["/a"].Deps[0], waveoutputtest.TestWaveOutputPath("a.js"); got != want {
		t.Fatalf("cloned deps = %q, want %q", got, want)
	}

	if got := routepublic.ClonePathsMapOrNil(nil); got != nil {
		t.Fatalf("routepublic.ClonePathsMapOrNil(nil) = %#v, want nil", got)
	}
}
