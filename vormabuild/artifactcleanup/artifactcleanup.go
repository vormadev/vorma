// Package artifactcleanup owns cleanup of generated static build artifacts.
//
// Build orchestration depends on this package to prune generated outputs before
// rewriting artifacts so stale route manifests and prehashed files do not leak
// between builds.
package artifactcleanup

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/internal/vormaruntime"
)

type cleanupDependencies struct {
	walkDirForRemoval            func(string, filepath.WalkFunc) error
	removePathForCleanup         func(string) error
	readDirEntriesForCleanup     func(string) ([]os.DirEntry, error)
	removeTopLevelFileForCleanup func(string) error
	statStaticPublicOutDir       func(string) (fs.FileInfo, error)
}

type cleanupExecutor struct {
	dependencies cleanupDependencies
}

var defaultCleanupExecutor = newCleanupExecutor(cleanupDependencies{})

func defaultCleanupDependencies() cleanupDependencies {
	return cleanupDependencies{
		walkDirForRemoval:            filepath.Walk,
		removePathForCleanup:         os.Remove,
		readDirEntriesForCleanup:     os.ReadDir,
		removeTopLevelFileForCleanup: os.Remove,
		statStaticPublicOutDir:       os.Stat,
	}
}

func normalizeCleanupDependencies(
	dependencies cleanupDependencies,
) cleanupDependencies {
	defaultDependencies := defaultCleanupDependencies()
	if dependencies.walkDirForRemoval == nil {
		dependencies.walkDirForRemoval = defaultDependencies.walkDirForRemoval
	}
	if dependencies.removePathForCleanup == nil {
		dependencies.removePathForCleanup = defaultDependencies.removePathForCleanup
	}
	if dependencies.readDirEntriesForCleanup == nil {
		dependencies.readDirEntriesForCleanup = defaultDependencies.readDirEntriesForCleanup
	}
	if dependencies.removeTopLevelFileForCleanup == nil {
		dependencies.removeTopLevelFileForCleanup = defaultDependencies.removeTopLevelFileForCleanup
	}
	if dependencies.statStaticPublicOutDir == nil {
		dependencies.statStaticPublicOutDir = defaultDependencies.statStaticPublicOutDir
	}
	return dependencies
}

func newCleanupExecutor(dependencies cleanupDependencies) cleanupExecutor {
	return cleanupExecutor{
		dependencies: normalizeCleanupDependencies(dependencies),
	}
}

func CleanStaticPublicOutDir(v *vormaruntime.Vorma) error {
	return defaultCleanupExecutor.cleanStaticPublicOutDir(v)
}

func (executor cleanupExecutor) cleanStaticPublicOutDir(
	v *vormaruntime.Vorma,
) error {
	staticPublicOutDir := v.Wave.StaticPublicOutDir()
	fileInfo, err := executor.dependencies.statStaticPublicOutDir(
		staticPublicOutDir,
	)
	if err != nil {
		if os.IsNotExist(err) {
			v.Log.Warn(
				fmt.Sprintf(
					"static public out dir does not exist: %s",
					staticPublicOutDir,
				),
			)
			return nil
		}
		return err
	}

	if !fileInfo.IsDir() {
		return fmt.Errorf("%s is not a directory", staticPublicOutDir)
	}

	return executor.removeMatchingEntriesRecursively(
		staticPublicOutDir,
		ShouldRemoveGeneratedStaticPublicFile,
	)
}

func ShouldRemoveGeneratedStaticPublicFile(fileBaseName string) bool {
	return strings.HasPrefix(
		fileBaseName,
		vormaruntime.VormaVitePrehashedFilePrefix,
	) || strings.HasPrefix(fileBaseName, vormaruntime.VormaRouteManifestPrefix)
}

func RemoveMatchingEntriesRecursively(
	rootDir string,
	shouldRemove func(string) bool,
) error {
	return defaultCleanupExecutor.removeMatchingEntriesRecursively(
		rootDir,
		shouldRemove,
	)
}

func (executor cleanupExecutor) removeMatchingEntriesRecursively(
	rootDir string,
	shouldRemove func(string) bool,
) error {
	return executor.dependencies.walkDirForRemoval(
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
			return executor.dependencies.removePathForCleanup(path)
		},
	)
}

func RemoveMatchingTopLevelFiles(
	rootDir string,
	shouldRemove func(string) bool,
) error {
	return defaultCleanupExecutor.removeMatchingTopLevelFiles(
		rootDir,
		shouldRemove,
	)
}

func (executor cleanupExecutor) removeMatchingTopLevelFiles(
	rootDir string,
	shouldRemove func(string) bool,
) error {
	entries, err := executor.dependencies.readDirEntriesForCleanup(rootDir)
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
		if err := executor.dependencies.removeTopLevelFileForCleanup(
			filepath.Join(rootDir, name),
		); err != nil {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}

	return nil
}
