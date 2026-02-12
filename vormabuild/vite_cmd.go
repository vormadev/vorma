package vormabuild

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/lab/viteutil"
	"github.com/vormadev/vorma/vormaruntime"
)

func postViteProdBuild(v *vormaruntime.Vorma) error {
	pathsFile, err := toPathsFileStageTwo(v)
	if err != nil {
		return fmt.Errorf("convert paths to stage two: %w", err)
	}

	pathsAsJSON, err := marshalIndentedPathsFile(pathsFile)
	if err != nil {
		return fmt.Errorf("marshal paths: %w", err)
	}

	if err := writeStageTwoPathsJSON(v, pathsAsJSON); err != nil {
		return fmt.Errorf("write paths: %w", err)
	}

	return nil
}

func toPathsFileStageTwo(v *vormaruntime.Vorma) (*vormaruntime.PathsFile, error) {
	viteManifest, err := viteutil.ReadManifest(v.Wave.GetViteManifestLocation())
	if err != nil {
		return nil, fmt.Errorf("read vite manifest: %w", err)
	}

	paths := v.GetPathsSnapshot()
	cleanClientEntry := filepath.Clean(v.Config.ClientEntry)
	clientEntryOut, clientEntryDeps, depToCSSBundleMap := applyViteManifestToPaths(
		viteManifest,
		paths,
		cleanClientEntry,
	)

	pathsFile := buildStageTwoPathsFile(
		v,
		paths,
		clientEntryOut,
		clientEntryDeps,
		depToCSSBundleMap,
	)

	buildID, err := computeStageTwoBuildID(v, pathsFile)
	if err != nil {
		return nil, err
	}

	applyBuildIDToVormaAndPathsFile(v, pathsFile, buildID)
	return pathsFile, nil
}

func marshalIndentedPathsFile(pathsFile *vormaruntime.PathsFile) ([]byte, error) {
	return json.MarshalIndent(pathsFile, "", "\t")
}

func writeStageTwoPathsJSON(v *vormaruntime.Vorma, pathsAsJSON []byte) error {
	return os.WriteFile(stageTwoPathsOutputPath(v), pathsAsJSON, os.ModePerm)
}

func stageTwoPathsOutputPath(v *vormaruntime.Vorma) string {
	return pathsOutputPath(v, vormaruntime.VormaPathsStageTwoJSONFileName)
}

func pathsOutputPath(v *vormaruntime.Vorma, fileName string) string {
	return filepath.Join(v.Wave.GetStaticPrivateOutDir(), vormaruntime.VormaOutDirname, fileName)
}

func applyBuildIDToVormaAndPathsFile(v *vormaruntime.Vorma, pathsFile *vormaruntime.PathsFile, buildID string) {
	v.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID(buildID)
	})
	pathsFile.BuildID = buildID
}

func applyViteManifestToPaths(
	viteManifest viteutil.Manifest,
	paths map[string]*vormaruntime.Path,
	cleanClientEntry string,
) (string, []string, map[string][]string) {
	clientEntryOut := ""
	clientEntryDeps := []string{}
	depToCSSBundleMap := make(map[string][]string)
	pathsBySourcePath := indexPathsBySourcePath(paths)

	for key, chunk := range viteManifest {
		cleanChunkOutPath := filepath.Base(chunk.File)

		if len(chunk.CSS) > 0 {
			depToCSSBundleMap[cleanChunkOutPath] = collectCSSBundleFileNames(chunk.CSS)
		}

		dependencies := viteutil.FindAllDependencies(viteManifest, key)

		if chunk.IsEntry && cleanClientEntry == chunk.Src {
			clientEntryOut = cleanChunkOutPath
			clientEntryDeps = removeDependency(dependencies, clientEntryOut)
			continue
		}

		updateRoutePathsForChunk(pathsBySourcePath, chunk.Src, cleanChunkOutPath, dependencies)
	}

	return clientEntryOut, clientEntryDeps, depToCSSBundleMap
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

func buildStageTwoPathsFile(
	v *vormaruntime.Vorma,
	paths map[string]*vormaruntime.Path,
	clientEntryOut string,
	clientEntryDeps []string,
	depToCSSBundleMap map[string][]string,
) *vormaruntime.PathsFile {
	return &vormaruntime.PathsFile{
		Stage:             "two",
		DepToCSSBundleMap: depToCSSBundleMap,
		Paths:             paths,
		ClientEntrySrc:    v.Config.ClientEntry,
		ClientEntryOut:    clientEntryOut,
		ClientEntryDeps:   clientEntryDeps,
		RouteManifestFile: v.GetRouteManifestFile(),
	}
}

func computeStageTwoBuildID(v *vormaruntime.Vorma, pathsFile *vormaruntime.PathsFile) (string, error) {
	htmlTemplateContent, err := os.ReadFile(path.Join(v.Wave.GetPrivateStaticDir(), v.Config.HTMLTemplateLocation))
	if err != nil {
		return "", fmt.Errorf("read HTML template: %w", err)
	}
	htmlContentHash := cryptoutil.Sha256Hash(htmlTemplateContent)

	asJSON, err := json.Marshal(pathsFile)
	if err != nil {
		return "", fmt.Errorf("marshal paths file: %w", err)
	}
	pfJSONHash := cryptoutil.Sha256Hash(asJSON)

	publicFSSummaryHash, err := getFSSummaryHash(os.DirFS(v.Wave.GetStaticPublicOutDir()))
	if err != nil {
		return "", fmt.Errorf("get FS summary hash: %w", err)
	}

	fullHash := sha256.New()
	fullHash.Write(htmlContentHash)
	fullHash.Write(pfJSONHash)
	fullHash.Write(publicFSSummaryHash)
	return base64.RawURLEncoding.EncodeToString(fullHash.Sum(nil)[:16]), nil
}
