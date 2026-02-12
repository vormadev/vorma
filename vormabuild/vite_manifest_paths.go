package vormabuild

import (
	"path/filepath"

	"github.com/vormadev/vorma/lab/viteutil"
	"github.com/vormadev/vorma/vormaruntime"
)

type viteManifestApplicationResult struct {
	clientEntryOut    string
	clientEntryDeps   []string
	depToCSSBundleMap map[string][]string
}

func applyViteManifestToPaths(
	viteManifest viteutil.Manifest,
	paths map[string]*vormaruntime.Path,
	cleanClientEntry string,
) viteManifestApplicationResult {
	result := viteManifestApplicationResult{
		clientEntryDeps:   []string{},
		depToCSSBundleMap: make(map[string][]string),
	}
	pathsBySourcePath := indexPathsBySourcePath(paths)

	for key, chunk := range viteManifest {
		cleanChunkOutPath := filepath.Base(chunk.File)

		if len(chunk.CSS) > 0 {
			result.depToCSSBundleMap[cleanChunkOutPath] = collectCSSBundleFileNames(chunk.CSS)
		}

		dependencies := viteutil.FindAllDependencies(viteManifest, key)

		if chunk.IsEntry && cleanClientEntry == chunk.Src {
			result.clientEntryOut = cleanChunkOutPath
			result.clientEntryDeps = removeDependency(dependencies, result.clientEntryOut)
			continue
		}

		updateRoutePathsForChunk(pathsBySourcePath, chunk.Src, cleanChunkOutPath, dependencies)
	}

	return result
}

func collectCSSBundleFileNames(cssFiles []string) []string {
	cssBundleFileNames := make([]string, 0, len(cssFiles))
	for _, cssFile := range cssFiles {
		cssBundleFileNames = append(cssBundleFileNames, filepath.Base(cssFile))
	}
	return cssBundleFileNames
}

func removeDependency(dependencies []string, dependencyToRemove string) []string {
	dependenciesWithoutTarget := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		if dependency == dependencyToRemove {
			continue
		}
		dependenciesWithoutTarget = append(dependenciesWithoutTarget, dependency)
	}
	return dependenciesWithoutTarget
}

func updateRoutePathsForChunk(
	pathsBySourcePath map[string][]*vormaruntime.Path,
	chunkSourcePath string,
	chunkOutPath string,
	chunkDependencies []string,
) {
	for _, currentPath := range pathsBySourcePath[chunkSourcePath] {
		currentPath.OutPath = chunkOutPath
		currentPath.Deps = chunkDependencies
	}
}

func indexPathsBySourcePath(paths map[string]*vormaruntime.Path) map[string][]*vormaruntime.Path {
	pathsBySourcePath := make(map[string][]*vormaruntime.Path)
	for _, currentPath := range paths {
		pathsBySourcePath[currentPath.SrcPath] = append(pathsBySourcePath[currentPath.SrcPath], currentPath)
	}
	return pathsBySourcePath
}
