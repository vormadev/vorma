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

func GetVormaPathsStageOneJSONPath() string {
	return path.Join(VormaOutDirname, VormaPathsStageOneJSONFileName)
}

func GetVormaPathsStageTwoJSONPath() string {
	return path.Join(VormaOutDirname, VormaPathsStageTwoJSONFileName)
}
