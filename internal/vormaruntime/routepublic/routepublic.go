// Package routepublic defines the public route-path shape used by
// internal/vormaruntime and bridges it to runtimecore's mutable route model.
package routepublic

import (
	"github.com/vormadev/vorma/internal/vormaruntime/runtimecore"
	"github.com/vormadev/vorma/kit/nestedmux"
)

// Path represents a route path with its associated metadata.
type Path struct {
	NestedRoute nestedmux.AnyRoute `json:"-"`

	// Both stage one and stage two.
	OriginalPattern string `json:"originalPattern"`
	SrcPath         string `json:"srcPath"`
	ExportKey       string `json:"exportKey"`
	ErrorExportKey  string `json:"errorExportKey,omitempty"`

	// Stage two only.
	OutPath string   `json:"outPath,omitempty"`
	Deps    []string `json:"deps,omitempty"`
}

// ToRuntimeCoreRoutePath converts one public Path to the runtimecore shape.
func ToRuntimeCoreRoutePath(pathValue *Path) *runtimecore.RoutePath {
	if pathValue == nil {
		return nil
	}
	return &runtimecore.RoutePath{
		OriginalPattern: pathValue.OriginalPattern,
		SrcPath:         pathValue.SrcPath,
		OutPath:         pathValue.OutPath,
		ExportKey:       pathValue.ExportKey,
		ErrorExportKey:  pathValue.ErrorExportKey,
		Deps:            runtimecore.CloneStringSliceOrNil(pathValue.Deps),
	}
}

// FromRuntimeCoreRoutePath converts one runtimecore RoutePath to the public
// Path shape.
func FromRuntimeCoreRoutePath(pathValue *runtimecore.RoutePath) *Path {
	if pathValue == nil {
		return nil
	}
	return &Path{
		OriginalPattern: pathValue.OriginalPattern,
		SrcPath:         pathValue.SrcPath,
		OutPath:         pathValue.OutPath,
		ExportKey:       pathValue.ExportKey,
		ErrorExportKey:  pathValue.ErrorExportKey,
		Deps:            runtimecore.CloneStringSliceOrNil(pathValue.Deps),
	}
}

// ToRuntimeCoreRoutePaths converts one public path map to runtimecore shape.
func ToRuntimeCoreRoutePaths(
	paths map[string]*Path,
) map[string]*runtimecore.RoutePath {
	if paths == nil {
		return nil
	}

	cloned := make(map[string]*runtimecore.RoutePath, len(paths))
	for pattern, pathValue := range paths {
		cloned[pattern] = ToRuntimeCoreRoutePath(pathValue)
	}
	return cloned
}

// FromRuntimeCoreRoutePaths converts one runtimecore path map to public shape.
func FromRuntimeCoreRoutePaths(
	paths map[string]*runtimecore.RoutePath,
) map[string]*Path {
	if paths == nil {
		return make(map[string]*Path)
	}

	cloned := make(map[string]*Path, len(paths))
	for pattern, pathValue := range paths {
		cloned[pattern] = FromRuntimeCoreRoutePath(pathValue)
	}
	return cloned
}

// ClonePathsMap deep-clones one public path map.
func ClonePathsMap(paths map[string]*Path) map[string]*Path {
	return FromRuntimeCoreRoutePaths(
		runtimecore.CloneRoutePaths(ToRuntimeCoreRoutePaths(paths)),
	)
}

// ClonePathsMapOrNil deep-clones one public path map and preserves nil.
func ClonePathsMapOrNil(paths map[string]*Path) map[string]*Path {
	if paths == nil {
		return nil
	}
	return ClonePathsMap(paths)
}

// CloneRuntimeCorePathsMapAsPublicOrNil deep-clones runtimecore paths and
// converts them to public shape, preserving nil.
func CloneRuntimeCorePathsMapAsPublicOrNil(
	paths map[string]*runtimecore.RoutePath,
) map[string]*Path {
	if paths == nil {
		return nil
	}
	return FromRuntimeCoreRoutePaths(
		runtimecore.CloneRoutePaths(paths),
	)
}
