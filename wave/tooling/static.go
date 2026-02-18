package tooling

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/pathnorm"
)

var staticIgnoreList = map[string]struct{}{
	".DS_Store": {},
}

func (b *Builder) processPublicFiles(granular bool) error {
	return b.processStaticFiles(staticOpts{
		srcDir:     pathnorm.Absolute(b.cfg.Core.StaticAssetDirs.Public),
		distDir:    b.cfg.Dist.StaticPublic(),
		gobPath:    b.cfg.Dist.PublicFileMapGob(),
		granular:   granular,
		isPublic:   true,
		hashOutput: true,
	})
}

func (b *Builder) processPrivateFiles(granular bool) error {
	return b.processStaticFiles(staticOpts{
		srcDir:     pathnorm.Absolute(b.cfg.Core.StaticAssetDirs.Private),
		distDir:    b.cfg.Dist.StaticPrivate(),
		gobPath:    b.cfg.Dist.PrivateFileMapGob(),
		granular:   granular,
		isPublic:   false,
		hashOutput: false,
	})
}

func (b *Builder) processPublicFilesForChangedPaths(changedSourcePaths []string) error {
	return b.processStaticFilesForChangedPaths(
		staticOpts{
			srcDir:     pathnorm.Absolute(b.cfg.Core.StaticAssetDirs.Public),
			distDir:    b.cfg.Dist.StaticPublic(),
			gobPath:    b.cfg.Dist.PublicFileMapGob(),
			granular:   true,
			isPublic:   true,
			hashOutput: true,
		},
		changedSourcePaths,
	)
}

func (b *Builder) processPrivateFilesForChangedPaths(changedSourcePaths []string) error {
	return b.processStaticFilesForChangedPaths(
		staticOpts{
			srcDir:     pathnorm.Absolute(b.cfg.Core.StaticAssetDirs.Private),
			distDir:    b.cfg.Dist.StaticPrivate(),
			gobPath:    b.cfg.Dist.PrivateFileMapGob(),
			granular:   true,
			isPublic:   false,
			hashOutput: false,
		},
		changedSourcePaths,
	)
}

type staticOpts struct {
	srcDir     string
	distDir    string
	gobPath    string
	granular   bool
	isPublic   bool
	hashOutput bool
}

type fileInfo struct {
	srcPath string
	relPath string
	prehash bool
}

func determineStaticProcessingWorkerCount(gomaxprocs int) int {
	if gomaxprocs <= 0 {
		return 1
	}

	workerCount := gomaxprocs * 2
	if workerCount < 1 {
		return 1
	}
	if workerCount > 32 {
		return 32
	}

	return workerCount
}

func determineStaticProcessingWorkerCountFromRuntime() int {
	return determineStaticProcessingWorkerCount(runtime.GOMAXPROCS(0))
}

func cleanupStaleStaticDistFiles(
	distDirectoryPath string,
	oldMap wave.FileMap,
	newMap wave.FileMap,
) {
	for key, oldVal := range oldMap {
		newVal, exists := newMap[key]
		if !exists || newVal.DistName != oldVal.DistName {
			removeStaticDistArtifactIfPresent(distDirectoryPath, oldVal.DistName)
		}
	}
}

func removeStaticDistArtifactIfPresent(
	distDirectoryPath string,
	distName string,
) {
	_ = os.Remove(filepath.Join(distDirectoryPath, distName))
}

func removeStaticMapEntriesForChangedRelativePaths(
	staticMap wave.FileMap,
	distDirectoryPath string,
	changedRelativePaths []string,
) bool {
	if len(changedRelativePaths) == 0 || len(staticMap) == 0 {
		return false
	}

	changedRelativePathSet := make(map[string]struct{}, len(changedRelativePaths))
	for _, changedRelativePath := range changedRelativePaths {
		if changedRelativePath == "" {
			continue
		}
		changedRelativePathSet[changedRelativePath] = struct{}{}
	}
	if len(changedRelativePathSet) == 0 {
		return false
	}

	pathsToDelete := make([]string, 0)
	for existingRelativePath := range staticMap {
		if shouldRemoveStaticMapEntryForChangedRelativePathSet(
			existingRelativePath,
			changedRelativePathSet,
		) {
			pathsToDelete = append(pathsToDelete, existingRelativePath)
		}
	}

	if len(pathsToDelete) == 0 {
		return false
	}

	sort.Strings(pathsToDelete)

	for _, relativePathToDelete := range pathsToDelete {
		oldValueForPath := staticMap[relativePathToDelete]
		delete(staticMap, relativePathToDelete)
		removeStaticDistArtifactIfPresent(distDirectoryPath, oldValueForPath.DistName)
	}

	return true
}

func shouldRemoveStaticMapEntryForChangedRelativePathSet(
	existingRelativePath string,
	changedRelativePathSet map[string]struct{},
) bool {
	if _, hasExactMatch := changedRelativePathSet[existingRelativePath]; hasExactMatch {
		return true
	}

	for index, character := range existingRelativePath {
		if character != '/' {
			continue
		}

		if _, hasParentDirectoryMatch := changedRelativePathSet[existingRelativePath[:index]]; hasParentDirectoryMatch {
			return true
		}
	}

	return false
}

func (b *Builder) processStaticFileAndReturnValue(
	staticFileInfo fileInfo,
	opts staticOpts,
	oldMap wave.FileMap,
) (wave.FileVal, error) {
	underscorePath := strings.ReplaceAll(staticFileInfo.relPath, "/", "_")
	contentHash, hashError := hashFile(staticFileInfo.srcPath, underscorePath)
	if hashError != nil {
		return wave.FileVal{}, fmt.Errorf("hash %s: %w", staticFileInfo.srcPath, hashError)
	}

	distName := deriveStaticDistName(staticFileInfo, opts.hashOutput, contentHash)
	processedValue := wave.FileVal{
		DistName:    distName,
		ContentHash: contentHash,
		IsPrehashed: staticFileInfo.prehash,
	}
	distPath := deriveStaticDistPath(staticFileInfo, opts, distName)

	if oldMap != nil {
		if oldVal, hasOldValue := oldMap[staticFileInfo.relPath]; hasOldValue && oldVal.ContentHash == contentHash {
			if _, statErr := os.Stat(distPath); statErr == nil {
				return processedValue, nil
			} else if !os.IsNotExist(statErr) {
				return wave.FileVal{}, statErr
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(distPath), 0o755); err != nil {
		return wave.FileVal{}, err
	}

	if err := fsutil.CopyFile(staticFileInfo.srcPath, distPath); err != nil {
		return wave.FileVal{}, err
	}

	return processedValue, nil
}

func deriveStaticDistName(
	staticFileInfo fileInfo,
	hashOutput bool,
	contentHash string,
) string {
	if staticFileInfo.prehash {
		return staticFileInfo.relPath
	}
	if !hashOutput {
		return staticFileInfo.relPath
	}
	return contentHash
}

func deriveStaticDistPath(
	staticFileInfo fileInfo,
	opts staticOpts,
	distName string,
) string {
	if opts.hashOutput {
		return filepath.Join(opts.distDir, distName)
	}
	return filepath.Join(opts.distDir, staticFileInfo.relPath)
}

func (b *Builder) processFile(
	fi fileInfo,
	opts staticOpts,
	newMap *sync.Map,
	oldMap wave.FileMap,
) error {
	processedValue, processError := b.processStaticFileAndReturnValue(
		fi,
		opts,
		oldMap,
	)
	if processError != nil {
		return processError
	}

	newMap.Store(fi.relPath, processedValue)
	return nil
}

func (b *Builder) processStaticFilesForChangedPaths(
	opts staticOpts,
	changedSourcePaths []string,
) error {
	if len(changedSourcePaths) == 0 {
		return b.processStaticFiles(opts)
	}

	changedResolutions, fullBuildRequired, resolutionError := resolveStaticChangedPathResolutions(
		opts.srcDir,
		changedSourcePaths,
	)
	if resolutionError != nil {
		return resolutionError
	}
	if fullBuildRequired {
		return b.processStaticFiles(opts)
	}
	if len(changedResolutions) == 0 {
		return nil
	}

	oldMap, loadError := b.loadFileMapFromPath(opts.gobPath)
	if loadError != nil {
		return b.processStaticFiles(opts)
	}
	if oldMap == nil {
		oldMap = make(wave.FileMap)
	}

	resolvedRelativePaths := make([]string, 0, len(changedResolutions))
	for relativePath := range changedResolutions {
		resolvedRelativePaths = append(resolvedRelativePaths, relativePath)
	}
	sort.Strings(resolvedRelativePaths)

	mapWasChanged := false
	relativePathsNeedingRemoval := make([]string, 0, len(resolvedRelativePaths))
	for _, resolvedRelativePath := range resolvedRelativePaths {
		resolution := changedResolutions[resolvedRelativePath]
		oldValueForPath, hadOldValueForPath := oldMap[resolvedRelativePath]

		if resolution.sourceExists {
			updatedValue, hasUpdatedValue, processError := b.processChangedStaticFile(
				resolution.fileInfo,
				opts,
				oldMap,
			)
			if processError != nil {
				return processError
			}
			if hasUpdatedValue {
				if !hadOldValueForPath || oldValueForPath != updatedValue {
					mapWasChanged = true
				}
				oldMap[resolvedRelativePath] = updatedValue

				if hadOldValueForPath && oldValueForPath.DistName != updatedValue.DistName {
					removeStaticDistArtifactIfPresent(opts.distDir, oldValueForPath.DistName)
				}
				continue
			}
		}
		relativePathsNeedingRemoval = append(relativePathsNeedingRemoval, resolvedRelativePath)
	}

	if removeStaticMapEntriesForChangedRelativePaths(
		oldMap,
		opts.distDir,
		relativePathsNeedingRemoval,
	) {
		mapWasChanged = true
	}

	if !mapWasChanged {
		return nil
	}

	if err := b.saveFileMap(oldMap, opts.gobPath); err != nil {
		return err
	}

	if opts.isPublic {
		return b.savePublicFileMapJS(oldMap)
	}

	return nil
}

func (b *Builder) processChangedStaticFile(
	staticFileInfo fileInfo,
	opts staticOpts,
	oldMap wave.FileMap,
) (wave.FileVal, bool, error) {
	processedValue, processError := b.processStaticFileAndReturnValue(
		staticFileInfo,
		opts,
		oldMap,
	)
	if processError != nil {
		if os.IsNotExist(processError) {
			return wave.FileVal{}, false, nil
		}
		return wave.FileVal{}, false, processError
	}

	return processedValue, true, nil
}

func (b *Builder) processStaticFiles(opts staticOpts) error {
	if _, err := os.Stat(opts.srcDir); os.IsNotExist(err) {
		return b.clearStaticOutputsWhenSourceDirectoryMissing(opts)
	}

	newMap := &sync.Map{}
	var oldMap wave.FileMap
	oldMapLoaded := false

	if opts.granular {
		old, err := b.loadFileMapFromPath(opts.gobPath)
		if err == nil {
			oldMap = old
			oldMapLoaded = true
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	files := make(chan fileInfo, 100)
	discoveredFileInfosByRelativePath := make(map[string]fileInfo)

	var walkErr error
	var walkErrOnce sync.Once

	go func() {
		defer close(files)
		err := filepath.WalkDir(opts.srcDir, func(path string, directoryEntry fs.DirEntry, walkEntryErr error) error {
			if walkEntryErr != nil {
				return walkEntryErr
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if directoryEntry.IsDir() {
				return nil
			}

			staticFileInfo, shouldProcessFile, resolveError := resolveStaticFileInfoFromSourcePath(
				opts.srcDir,
				path,
			)
			if resolveError != nil {
				return fmt.Errorf("resolve static file info for %s: %w", path, resolveError)
			}
			if !shouldProcessFile {
				return nil
			}

			existingFileInfo, alreadyDiscovered := discoveredFileInfosByRelativePath[staticFileInfo.relPath]
			if alreadyDiscovered {
				if collisionError := ensureNoStaticLogicalPathCollision(existingFileInfo, staticFileInfo); collisionError != nil {
					return collisionError
				}
				return nil
			}
			discoveredFileInfosByRelativePath[staticFileInfo.relPath] = staticFileInfo

			select {
			case files <- staticFileInfo:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
		if err != nil {
			walkErrOnce.Do(func() {
				if !errors.Is(err, context.Canceled) {
					walkErr = fmt.Errorf("walk %s: %w", opts.srcDir, err)
				}
				cancel()
			})
		}
	}()

	numWorkers := determineStaticProcessingWorkerCountFromRuntime()
	var waitGroup sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for range numWorkers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for staticFileInfo := range files {
				select {
				case <-ctx.Done():
					return
				default:
				}

				processError := b.processFile(staticFileInfo, opts, newMap, oldMap)
				if processError != nil {
					errOnce.Do(func() {
						firstErr = processError
						cancel()
					})
					return
				}
			}
		}()
	}

	waitGroup.Wait()

	if firstErr != nil {
		return firstErr
	}
	if walkErr != nil {
		return walkErr
	}

	finalMap := make(wave.FileMap)
	newMap.Range(func(key, val any) bool {
		finalMap[key.(string)] = val.(wave.FileVal)
		return true
	})

	if opts.granular && oldMapLoaded {
		cleanupStaleStaticDistFiles(opts.distDir, oldMap, finalMap)
	}

	shouldSaveFileMap := !oldMapLoaded || !maps.Equal(oldMap, finalMap)
	if shouldSaveFileMap {
		if err := b.saveFileMap(finalMap, opts.gobPath); err != nil {
			return err
		}
	}

	if opts.isPublic {
		return b.savePublicFileMapJS(finalMap)
	}

	return nil
}

func (b *Builder) clearStaticOutputsWhenSourceDirectoryMissing(
	opts staticOpts,
) error {
	emptyMap := wave.FileMap{}

	previousMap, loadError := b.loadFileMapFromPath(opts.gobPath)
	if loadError == nil {
		cleanupStaleStaticDistFiles(opts.distDir, previousMap, emptyMap)
	}

	if err := b.saveFileMap(emptyMap, opts.gobPath); err != nil {
		return err
	}
	if opts.isPublic {
		return b.savePublicFileMapJS(emptyMap)
	}
	return nil
}
