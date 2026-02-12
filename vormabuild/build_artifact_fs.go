package vormabuild

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/vormaruntime"
)

func cleanStaticPublicOutDir(v *vormaruntime.Vorma) error {
	staticPublicOutDir := v.Wave.GetStaticPublicOutDir()

	fileInfo, err := os.Stat(staticPublicOutDir)
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
	return filepath.Walk(rootDir, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !shouldRemove(filepath.Base(path)) {
			return nil
		}
		return os.Remove(path)
	})
}

func removeMatchingTopLevelFiles(
	rootDir string,
	shouldRemove func(string) bool,
) error {
	entries, err := os.ReadDir(rootDir)
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
		if err := os.Remove(filepath.Join(rootDir, name)); err != nil {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}

	return nil
}

func writePathsToDiskStageOne(l *vormaruntime.LockedVorma) error {
	v := l.Vorma()
	pathsJSONOut := stageOnePathsOutputPath(v)
	pathsAsJSON, err := marshalIndentedPathsFile(stageOnePathsFile(l))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(pathsJSONOut), os.ModePerm); err != nil {
		return err
	}

	return os.WriteFile(pathsJSONOut, pathsAsJSON, os.ModePerm)
}

func stageOnePathsOutputPath(v *vormaruntime.Vorma) string {
	return pathsOutputPath(v, vormaruntime.VormaPathsStageOneJSONFileName)
}

func stageOnePathsFile(l *vormaruntime.LockedVorma) *vormaruntime.PathsFile {
	v := l.Vorma()
	return &vormaruntime.PathsFile{
		Stage:             "one",
		Paths:             l.GetPaths(),
		ClientEntrySrc:    v.Config.ClientEntry,
		BuildID:           l.GetBuildID(),
		RouteManifestFile: l.GetRouteManifestFile(),
	}
}
