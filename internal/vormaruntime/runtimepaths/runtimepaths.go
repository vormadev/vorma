// Package runtimepaths defines and loads disk-backed route artifacts consumed
// by vormaruntime.
//
// The package owns file naming, decoding, and validation for stage-one/stage-two
// paths files, plus bootstrap conversion into runtimecore artifacts, so runtime
// path artifact rules are enforced consistently from one place.
package runtimepaths

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"strings"

	"github.com/vormadev/vorma/internal/vormaruntime/runtimecore"
)

// RoutePath aliases runtimecore.RoutePath so path shape ownership lives in one
// place.
type RoutePath = runtimecore.RoutePath

// PathsFile represents the serialized route artifact payload written to disk.
type PathsFile struct {
	Stage             string                `json:"stage"`
	BuildID           string                `json:"buildID,omitempty"`
	ClientEntrySrc    string                `json:"clientEntrySrc"`
	Paths             map[string]*RoutePath `json:"paths"`
	RouteManifestFile string                `json:"routeManifestFile"`
	ClientEntryOut    string                `json:"clientEntryOut,omitempty"`
	ClientEntryDeps   []string              `json:"clientEntryDeps,omitempty"`
	DepToCSSBundleMap map[string][]string   `json:"depToCSSBundleMap,omitempty"`
}

const (
	// VormaInternalDirname is the private output directory for Vorma runtime
	// internal artifacts.
	VormaInternalDirname = "vorma_owned"
)

const (
	// VormaPathsStageOneJSONFileName is the stage-one route artifacts filename.
	VormaPathsStageOneJSONFileName = "vorma_paths_stage_1.json"
	// VormaPathsStageTwoJSONFileName is the stage-two route artifacts filename.
	VormaPathsStageTwoJSONFileName = "vorma_paths_stage_2.json"
	// GeneratedTypeScriptIndexFileName is the main generated TypeScript runtime
	// module filename.
	GeneratedTypeScriptIndexFileName = "index.ts"
	// GeneratedTypeScriptPublicFileMapFileName is the generated TypeScript
	// static-public filemap module filename.
	GeneratedTypeScriptPublicFileMapFileName = "filemap.ts"
)

// GetVormaPathsStageOneJSONPath returns the stage-one artifacts file path.
func GetVormaPathsStageOneJSONPath() string {
	return path.Join(VormaInternalDirname, VormaPathsStageOneJSONFileName)
}

// GetVormaPathsStageTwoJSONPath returns the stage-two artifacts file path.
func GetVormaPathsStageTwoJSONPath() string {
	return path.Join(VormaInternalDirname, VormaPathsStageTwoJSONFileName)
}

// LoadPathsFileFromFS reads, decodes, and validates stage-one/stage-two paths
// artifacts from the provided filesystem.
func LoadPathsFileFromFS(
	privateFS fs.FS,
	isDev bool,
) (*PathsFile, error) {
	if privateFS == nil {
		return nil, fmt.Errorf("private fs is nil")
	}

	fileToUse := VormaPathsStageOneJSONFileName
	if !isDev {
		fileToUse = VormaPathsStageTwoJSONFileName
	}

	file, err := privateFS.Open(path.Join(VormaInternalDirname, fileToUse))
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %w", fileToUse, err)
	}
	defer file.Close()

	var pathsFile PathsFile
	if err := json.NewDecoder(file).Decode(&pathsFile); err != nil {
		return nil, fmt.Errorf("could not decode %s: %w", fileToUse, err)
	}
	if err := ValidatePathsFileStructuralIntegrity(&pathsFile); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", fileToUse, err)
	}
	if err := ValidatePathsFileSemanticIntegrity(&pathsFile, isDev); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", fileToUse, err)
	}

	return &pathsFile, nil
}

// ValidatePathsFileStructuralIntegrity validates structural shape constraints.
func ValidatePathsFileStructuralIntegrity(pathsFile *PathsFile) error {
	if pathsFile == nil {
		return fmt.Errorf("paths file is nil")
	}
	if pathsFile.Paths == nil {
		return nil
	}
	for mapKeyPattern, pathEntry := range pathsFile.Paths {
		if pathEntry == nil {
			return fmt.Errorf("paths[%q] cannot be null", mapKeyPattern)
		}
		if mapKeyPattern != "" && pathEntry.OriginalPattern == "" {
			return fmt.Errorf(
				"paths[%q].originalPattern is required",
				mapKeyPattern,
			)
		}
		if pathEntry.OriginalPattern != mapKeyPattern {
			return fmt.Errorf(
				"paths[%q].originalPattern=%q does not match key",
				mapKeyPattern,
				pathEntry.OriginalPattern,
			)
		}
	}
	return nil
}

// ValidatePathsFileSemanticIntegrity validates semantic artifact constraints.
func ValidatePathsFileSemanticIntegrity(
	pathsFile *PathsFile,
	isDev bool,
) error {
	if pathsFile == nil {
		return fmt.Errorf("paths file is nil")
	}
	if strings.TrimSpace(pathsFile.RouteManifestFile) == "" {
		return fmt.Errorf("routeManifestFile is required")
	}
	if !isDev && strings.TrimSpace(pathsFile.ClientEntryOut) == "" {
		return fmt.Errorf("clientEntryOut is required")
	}

	for mapKeyPattern, pathEntry := range pathsFile.Paths {
		if pathEntry == nil {
			continue
		}
		if pathEntry.SrcPath != "" &&
			strings.TrimSpace(pathEntry.ExportKey) == "" {
			return fmt.Errorf(
				"paths[%q].exportKey is required when srcPath is set",
				mapKeyPattern,
			)
		}
		if !isDev && pathEntry.SrcPath != "" &&
			strings.TrimSpace(pathEntry.OutPath) == "" {
			return fmt.Errorf(
				"paths[%q].outPath is required in production mode",
				mapKeyPattern,
			)
		}
	}

	return nil
}

// PrettyPrintFS is a debug utility for fs.FS instances.
func PrettyPrintFS(fsys fs.FS) error {
	return fs.WalkDir(
		fsys,
		".",
		func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				fmt.Println(p)
			} else {
				fmt.Printf("%s (%s)\n", p, d.Type())
			}
			return nil
		},
	)
}

// RouteArtifactsLoadOutput contains loaded path artifacts and normalized route
// artifacts.
type RouteArtifactsLoadOutput struct {
	PathsFile        *PathsFile
	RuntimeArtifacts *runtimecore.RuntimeRouteArtifacts
}

// LoadRouteArtifactsFromFS reads route artifacts from disk and normalizes them
// into runtimecore artifacts for one runtime mode.
func LoadRouteArtifactsFromFS(
	privateFS fs.FS,
	isDev bool,
) (*RouteArtifactsLoadOutput, error) {
	pathsFile, err := LoadPathsFileFromFS(privateFS, isDev)
	if err != nil {
		return nil, err
	}
	runtimeArtifacts, err := runtimecore.BuildRuntimeRouteArtifacts(
		BuildRuntimePathsFileSnapshot(pathsFile),
	)
	if err != nil {
		return nil, fmt.Errorf("build runtime route artifacts: %w", err)
	}
	return &RouteArtifactsLoadOutput{
		PathsFile:        pathsFile,
		RuntimeArtifacts: runtimeArtifacts,
	}, nil
}

// ParseRootTemplateFromFS parses the configured root template from disk.
func ParseRootTemplateFromFS(
	privateFS fs.FS,
	rootTemplateLocation string,
) (*template.Template, error) {
	if privateFS == nil {
		return nil, fmt.Errorf("private fs is nil")
	}
	rootTemplate, err := template.ParseFS(privateFS, rootTemplateLocation)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	return rootTemplate, nil
}

// BuildRuntimePathsFileSnapshot maps one runtimepaths file payload to the
// runtimecore snapshot shape.
func BuildRuntimePathsFileSnapshot(
	pathsFile *PathsFile,
) *runtimecore.RuntimePathsFileSnapshot {
	if pathsFile == nil {
		return nil
	}

	return &runtimecore.RuntimePathsFileSnapshot{
		BuildID:        pathsFile.BuildID,
		ClientEntrySrc: pathsFile.ClientEntrySrc,
		ClientEntryOut: pathsFile.ClientEntryOut,
		ClientEntryDeps: runtimecore.CloneStringSliceOrNil(
			pathsFile.ClientEntryDeps,
		),
		DepToCSSBundleMap: runtimecore.CloneDepToCSSBundleMapOrNil(
			pathsFile.DepToCSSBundleMap,
		),
		RouteManifestFile: pathsFile.RouteManifestFile,
		Paths:             runtimecore.CloneRoutePathsOrNil(pathsFile.Paths),
	}
}
