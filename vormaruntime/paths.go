package vormaruntime

import "path"

// Directory name constants
const (
	VormaOutDirname = "vorma_out"
)

// File name constants
const (
	VormaPathsStageOneJSONFileName = "vorma_paths_stage_1.json"
	VormaPathsStageTwoJSONFileName = "vorma_paths_stage_2.json"
)

// Output prefix constants
const (
	VormaOutPrefix               = "vorma_out_"
	VormaVitePrehashedFilePrefix = VormaOutPrefix + "vite_"
	VormaRouteManifestPrefix     = VormaOutPrefix + "vorma_internal_route_manifest_"
)

// VormaPaths provides path construction helpers for Vorma-specific paths.
var VormaPaths = vormaPaths{}

type vormaPaths struct{}

func (vormaPaths) StageOneJSON() string {
	return path.Join(VormaOutDirname, VormaPathsStageOneJSONFileName)
}

func (vormaPaths) StageTwoJSON() string {
	return path.Join(VormaOutDirname, VormaPathsStageTwoJSONFileName)
}
