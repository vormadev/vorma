package vorma

import "path"

// Directory name constants
const (
	// VormaOutDirname is the directory name for Vorma-generated output files
	// within the private static directory
	VormaOutDirname = "vorma_out"
)

// File name constants
const (
	VormaPathsStageOneJSONFileName = "vorma_paths_stage_1.json"
	VormaPathsStageTwoJSONFileName = "vorma_paths_stage_2.json"
)

// Output prefix constants
const (
	// vormaOutPrefix is the prefix for all Vorma-generated hashed output files
	vormaOutPrefix = "vorma_out_"

	// vormaVitePrehashedFilePrefix is the prefix for Vite-generated files
	// that Vorma should clean up during builds
	vormaVitePrehashedFilePrefix = vormaOutPrefix + "vite_"

	// vormaRouteManifestPrefix is the prefix for route manifest files
	vormaRouteManifestPrefix = vormaOutPrefix + "vorma_internal_route_manifest_"
)

// VormaPaths provides path construction helpers for Vorma-specific paths.
// These use forward slashes for fs.FS compatibility.
var VormaPaths = vormaPaths{}

type vormaPaths struct{}

// StageOneJSON returns the relative path to the stage one paths JSON file
func (vormaPaths) StageOneJSON() string {
	return path.Join(VormaOutDirname, VormaPathsStageOneJSONFileName)
}

// StageTwoJSON returns the relative path to the stage two paths JSON file
func (vormaPaths) StageTwoJSON() string {
	return path.Join(VormaOutDirname, VormaPathsStageTwoJSONFileName)
}
