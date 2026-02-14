package vormabuild

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/vormaruntime"
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

var buildArtifactCleanupDeps = buildArtifactCleanupDependencies{
	walkDirForRemoval:            filepath.Walk,
	removePathForCleanup:         os.Remove,
	readDirEntriesForCleanup:     os.ReadDir,
	removeTopLevelFileForCleanup: os.Remove,
	statStaticPublicOutDir:       os.Stat,
}

var stageOnePathsWriteDeps = stageOnePathsWriteDependencies{
	marshalStageOnePathsFile: func(pathsFile *vormaruntime.PathsFile) ([]byte, error) {
		return json.MarshalIndent(pathsFile, "", "\t")
	},
	makeStageOnePathsOutputDirectory: os.MkdirAll,
	writeStageOnePathsJSON:           writeFileAtomically,
}

func cleanStaticPublicOutDir(v *vormaruntime.Vorma) error {
	staticPublicOutDir := v.Wave.GetStaticPublicOutDir()

	fileInfo, err := buildArtifactCleanupDeps.statStaticPublicOutDir(staticPublicOutDir)
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

	return removeMatchingEntriesRecursively(staticPublicOutDir, shouldRemoveGeneratedStaticPublicFile)
}

func shouldRemoveGeneratedStaticPublicFile(fileBaseName string) bool {
	return strings.HasPrefix(fileBaseName, vormaruntime.VormaVitePrehashedFilePrefix) ||
		strings.HasPrefix(fileBaseName, vormaruntime.VormaRouteManifestPrefix)
}

func removeMatchingEntriesRecursively(
	rootDir string,
	shouldRemove func(string) bool,
) error {
	return buildArtifactCleanupDeps.walkDirForRemoval(
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
			return buildArtifactCleanupDeps.removePathForCleanup(path)
		},
	)
}

func removeMatchingTopLevelFiles(
	rootDir string,
	shouldRemove func(string) bool,
) error {
	entries, err := buildArtifactCleanupDeps.readDirEntriesForCleanup(rootDir)
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
		if err := buildArtifactCleanupDeps.removeTopLevelFileForCleanup(filepath.Join(rootDir, name)); err != nil {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}

	return nil
}

func writePathsToDiskStageOne(l *vormaruntime.LockedVorma) error {
	return writePathsToDiskStageOneWithRouteManifest(l, l.GetRouteManifestFile())
}

func writePathsToDiskStageOneWithRouteManifest(
	l *vormaruntime.LockedVorma,
	routeManifestFile string,
) error {
	v := l.Vorma()
	pathsJSONOut := pathsOutputPath(v, vormaruntime.VormaPathsStageOneJSONFileName)
	pathsAsJSON, err := stageOnePathsWriteDeps.marshalStageOnePathsFile(
		stageOnePathsFile(l, routeManifestFile),
	)
	if err != nil {
		return fmt.Errorf("marshal stage-one paths file: %w", err)
	}
	return writePathsJSONBytesToOutputPath(
		pathsJSONOut,
		pathsAsJSON,
		pathsJSONWriteDependencies{
			makePathsOutputDirectory: stageOnePathsWriteDeps.makeStageOnePathsOutputDirectory,
			writePathsJSON:           stageOnePathsWriteDeps.writeStageOnePathsJSON,
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
		Paths:             l.GetPaths(),
		ClientEntrySrc:    v.Config.ClientEntry,
		BuildID:           l.GetBuildID(),
		RouteManifestFile: routeManifestFile,
	}
}
