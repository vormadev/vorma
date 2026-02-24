// Package runtimepaths defines the on-disk route artifact schema consumed by
// vormaruntime.
//
// The package owns file naming, decoding, and validation for stage-one/stage-two
// paths files so that runtime path artifact rules are enforced consistently from
// one place.
package runtimepaths

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// RoutePath is one serialized route entry from stage-one/stage-two paths files.
type RoutePath struct {
	OriginalPattern string   `json:"originalPattern"`
	SrcPath         string   `json:"srcPath"`
	ExportKey       string   `json:"exportKey"`
	ErrorExportKey  string   `json:"errorExportKey,omitempty"`
	OutPath         string   `json:"outPath,omitempty"`
	Deps            []string `json:"deps,omitempty"`
}

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
	// VormaOutDirname is the output directory for Vorma build artifacts.
	VormaOutDirname = "vorma_out"
)

const (
	// VormaPathsStageOneJSONFileName is the stage-one route artifacts filename.
	VormaPathsStageOneJSONFileName = "vorma_paths_stage_1.json"
	// VormaPathsStageTwoJSONFileName is the stage-two route artifacts filename.
	VormaPathsStageTwoJSONFileName = "vorma_paths_stage_2.json"
)

// GetVormaPathsStageOneJSONPath returns the stage-one artifacts file path.
func GetVormaPathsStageOneJSONPath() string {
	return path.Join(VormaOutDirname, VormaPathsStageOneJSONFileName)
}

// GetVormaPathsStageTwoJSONPath returns the stage-two artifacts file path.
func GetVormaPathsStageTwoJSONPath() string {
	return path.Join(VormaOutDirname, VormaPathsStageTwoJSONFileName)
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

	file, err := privateFS.Open(path.Join(VormaOutDirname, fileToUse))
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
