package vormabuild

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/internal/vormaruntime"
)

type buildArtifactCleanupDependencies struct {
	walkDirForRemoval            func(string, filepath.WalkFunc) error
	removePathForCleanup         func(string) error
	readDirEntriesForCleanup     func(string) ([]os.DirEntry, error)
	removeTopLevelFileForCleanup func(string) error
	statStaticPublicOutDir       func(string) (fs.FileInfo, error)
}

type stageOnePathsWriteDependencies struct {
	marshalStageOnePathsFile         func(*vormaruntime.PathsFile) ([]byte, error)
	makeStageOnePathsOutputDirectory func(string, fs.FileMode) error
	writeStageOnePathsJSON           func(string, []byte, fs.FileMode) error
}

type buildArtifactFileSystemExecutorDependencies struct {
	buildArtifactCleanupDependencies buildArtifactCleanupDependencies
	stageOnePathsWriteDependencies   stageOnePathsWriteDependencies
}

type buildArtifactFileSystemExecutor struct {
	dependencies buildArtifactFileSystemExecutorDependencies
}

var defaultBuildArtifactFileSystemExecutor = newBuildArtifactFileSystemExecutor(
	buildArtifactFileSystemExecutorDependencies{},
)

func defaultBuildArtifactFileSystemExecutorDependencies() buildArtifactFileSystemExecutorDependencies {
	return buildArtifactFileSystemExecutorDependencies{
		buildArtifactCleanupDependencies: buildArtifactCleanupDependencies{
			walkDirForRemoval:            filepath.Walk,
			removePathForCleanup:         os.Remove,
			readDirEntriesForCleanup:     os.ReadDir,
			removeTopLevelFileForCleanup: os.Remove,
			statStaticPublicOutDir:       os.Stat,
		},
		stageOnePathsWriteDependencies: stageOnePathsWriteDependencies{
			marshalStageOnePathsFile: func(pathsFile *vormaruntime.PathsFile) ([]byte, error) {
				return json.MarshalIndent(pathsFile, "", "\t")
			},
			makeStageOnePathsOutputDirectory: os.MkdirAll,
			writeStageOnePathsJSON:           writeFileAtomically,
		},
	}
}

func normalizeBuildArtifactFileSystemExecutorDependencies(
	dependencies buildArtifactFileSystemExecutorDependencies,
) buildArtifactFileSystemExecutorDependencies {
	defaultDependencies := defaultBuildArtifactFileSystemExecutorDependencies()

	if dependencies.buildArtifactCleanupDependencies.walkDirForRemoval == nil {
		dependencies.buildArtifactCleanupDependencies.walkDirForRemoval = defaultDependencies.buildArtifactCleanupDependencies.walkDirForRemoval
	}
	if dependencies.buildArtifactCleanupDependencies.removePathForCleanup == nil {
		dependencies.buildArtifactCleanupDependencies.removePathForCleanup = defaultDependencies.buildArtifactCleanupDependencies.removePathForCleanup
	}
	if dependencies.buildArtifactCleanupDependencies.readDirEntriesForCleanup == nil {
		dependencies.buildArtifactCleanupDependencies.readDirEntriesForCleanup = defaultDependencies.buildArtifactCleanupDependencies.readDirEntriesForCleanup
	}
	if dependencies.buildArtifactCleanupDependencies.removeTopLevelFileForCleanup == nil {
		dependencies.buildArtifactCleanupDependencies.removeTopLevelFileForCleanup = defaultDependencies.buildArtifactCleanupDependencies.removeTopLevelFileForCleanup
	}
	if dependencies.buildArtifactCleanupDependencies.statStaticPublicOutDir == nil {
		dependencies.buildArtifactCleanupDependencies.statStaticPublicOutDir = defaultDependencies.buildArtifactCleanupDependencies.statStaticPublicOutDir
	}

	if dependencies.stageOnePathsWriteDependencies.marshalStageOnePathsFile == nil {
		dependencies.stageOnePathsWriteDependencies.marshalStageOnePathsFile = defaultDependencies.stageOnePathsWriteDependencies.marshalStageOnePathsFile
	}
	if dependencies.stageOnePathsWriteDependencies.makeStageOnePathsOutputDirectory == nil {
		dependencies.stageOnePathsWriteDependencies.makeStageOnePathsOutputDirectory = defaultDependencies.stageOnePathsWriteDependencies.makeStageOnePathsOutputDirectory
	}
	if dependencies.stageOnePathsWriteDependencies.writeStageOnePathsJSON == nil {
		dependencies.stageOnePathsWriteDependencies.writeStageOnePathsJSON = defaultDependencies.stageOnePathsWriteDependencies.writeStageOnePathsJSON
	}

	return dependencies
}

func newBuildArtifactFileSystemExecutor(
	dependencies buildArtifactFileSystemExecutorDependencies,
) buildArtifactFileSystemExecutor {
	return buildArtifactFileSystemExecutor{
		dependencies: normalizeBuildArtifactFileSystemExecutorDependencies(dependencies),
	}
}

func cleanStaticPublicOutDir(v *vormaruntime.Vorma) error {
	return defaultBuildArtifactFileSystemExecutor.cleanStaticPublicOutDir(v)
}

func (executor buildArtifactFileSystemExecutor) cleanStaticPublicOutDir(
	v *vormaruntime.Vorma,
) error {
	staticPublicOutDir := v.Wave.StaticPublicOutDir()

	fileInfo, err := executor.dependencies.buildArtifactCleanupDependencies.statStaticPublicOutDir(
		staticPublicOutDir,
	)
	if err != nil {
		if os.IsNotExist(err) {
			v.Log.Warn(fmt.Sprintf("static public out dir does not exist: %s", staticPublicOutDir))
			return nil
		}
		return err
	}

	if !fileInfo.IsDir() {
		return fmt.Errorf("%s is not a directory", staticPublicOutDir)
	}

	return executor.removeMatchingEntriesRecursively(
		staticPublicOutDir,
		shouldRemoveGeneratedStaticPublicFile,
	)
}

func shouldRemoveGeneratedStaticPublicFile(fileBaseName string) bool {
	return strings.HasPrefix(fileBaseName, vormaruntime.VormaVitePrehashedFilePrefix) ||
		strings.HasPrefix(fileBaseName, vormaruntime.VormaRouteManifestPrefix)
}

func removeMatchingEntriesRecursively(
	rootDir string,
	shouldRemove func(string) bool,
) error {
	return defaultBuildArtifactFileSystemExecutor.removeMatchingEntriesRecursively(rootDir, shouldRemove)
}

func (executor buildArtifactFileSystemExecutor) removeMatchingEntriesRecursively(
	rootDir string,
	shouldRemove func(string) bool,
) error {
	return executor.dependencies.buildArtifactCleanupDependencies.walkDirForRemoval(
		rootDir,
		func(path string, info fs.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info != nil && info.IsDir() {
				return nil
			}
			if !shouldRemove(filepath.Base(path)) {
				return nil
			}
			return executor.dependencies.buildArtifactCleanupDependencies.removePathForCleanup(path)
		},
	)
}

func removeMatchingTopLevelFiles(
	rootDir string,
	shouldRemove func(string) bool,
) error {
	return defaultBuildArtifactFileSystemExecutor.removeMatchingTopLevelFiles(rootDir, shouldRemove)
}

func (executor buildArtifactFileSystemExecutor) removeMatchingTopLevelFiles(
	rootDir string,
	shouldRemove func(string) bool,
) error {
	entries, err := executor.dependencies.buildArtifactCleanupDependencies.readDirEntriesForCleanup(rootDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !shouldRemove(name) {
			continue
		}
		if err := executor.dependencies.buildArtifactCleanupDependencies.removeTopLevelFileForCleanup(
			filepath.Join(rootDir, name),
		); err != nil {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}

	return nil
}

func writePathsToDiskStageOne(l *vormaruntime.LockedVorma) error {
	return defaultBuildArtifactFileSystemExecutor.writePathsToDiskStageOne(l)
}

func (executor buildArtifactFileSystemExecutor) writePathsToDiskStageOne(
	l *vormaruntime.LockedVorma,
) error {
	return executor.writePathsToDiskStageOneWithRouteManifest(l, l.RouteManifestFile())
}

func writePathsToDiskStageOneWithRouteManifest(
	l *vormaruntime.LockedVorma,
	routeManifestFile string,
) error {
	return defaultBuildArtifactFileSystemExecutor.writePathsToDiskStageOneWithRouteManifest(
		l,
		routeManifestFile,
	)
}

func (executor buildArtifactFileSystemExecutor) writePathsToDiskStageOneWithRouteManifest(
	l *vormaruntime.LockedVorma,
	routeManifestFile string,
) error {
	return executor.writeStageOnePathsFileToDisk(
		l.Vorma(),
		stageOnePathsFile(l, routeManifestFile),
	)
}

func writePathsToDiskStageOneFromRuntimeState(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
	routeManifestFile string,
) error {
	return defaultBuildArtifactFileSystemExecutor.writePathsToDiskStageOneFromRuntimeState(
		v,
		runtimeStateSnapshot,
		routeManifestFile,
	)
}

func (executor buildArtifactFileSystemExecutor) writePathsToDiskStageOneFromRuntimeState(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
	routeManifestFile string,
) error {
	return executor.writeStageOnePathsFileToDisk(
		v,
		stageOnePathsFileFromRuntimeState(
			v,
			runtimeStateSnapshot,
			routeManifestFile,
		),
	)
}

func (executor buildArtifactFileSystemExecutor) writeStageOnePathsFileToDisk(
	v *vormaruntime.Vorma,
	stageOnePathsFileData *vormaruntime.PathsFile,
) error {
	pathsJSONOut := pathsOutputPath(v, vormaruntime.VormaPathsStageOneJSONFileName)
	pathsAsJSON, err := executor.dependencies.stageOnePathsWriteDependencies.marshalStageOnePathsFile(
		stageOnePathsFileData,
	)
	if err != nil {
		return fmt.Errorf("marshal stage-one paths file: %w", err)
	}
	return writePathsJSONBytesToOutputPath(
		pathsJSONOut,
		pathsAsJSON,
		pathsJSONWriteDependencies{
			makePathsOutputDirectory: executor.dependencies.stageOnePathsWriteDependencies.makeStageOnePathsOutputDirectory,
			writePathsJSON:           executor.dependencies.stageOnePathsWriteDependencies.writeStageOnePathsJSON,
		},
		"create stage-one paths output directory",
		"write stage-one paths JSON",
	)
}

func stageOnePathsFile(
	l *vormaruntime.LockedVorma,
	routeManifestFile string,
) *vormaruntime.PathsFile {
	v := l.Vorma()
	return &vormaruntime.PathsFile{
		Stage:             "one",
		Paths:             l.Paths(),
		ClientEntrySrc:    v.Config.ClientEntry,
		BuildID:           l.BuildID(),
		RouteManifestFile: routeManifestFile,
	}
}

func stageOnePathsFileFromRuntimeState(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
	routeManifestFile string,
) *vormaruntime.PathsFile {
	return &vormaruntime.PathsFile{
		Stage:             "one",
		Paths:             runtimeStateSnapshot.paths,
		ClientEntrySrc:    v.Config.ClientEntry,
		BuildID:           runtimeStateSnapshot.buildID,
		RouteManifestFile: routeManifestFile,
	}
}
