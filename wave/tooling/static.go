package tooling

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
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

type staticChangedPathResolution struct {
	fileInfo     fileInfo
	sourceExists bool
}

type staticChangedPathResolutionProbeFunctions struct {
	normalizeChangedSourcePath                                        func(string) string
	resolveStaticFileInfoFromSourcePath                               func(string, string) (fileInfo, bool, error)
	sourceFileExists                                                  func(string) (bool, error)
	ensureNoStaticLogicalPathCollisionWithinSourceDirectoryForRelPath func(string, string) error
}

type staticChangedPathResolutionProbeResult struct {
	staticFileInfo    fileInfo
	shouldProcessFile bool
}

func resolveStaticChangedPathResolutions(
	sourceDirectoryPath string,
	changedSourcePaths []string,
) (map[string]staticChangedPathResolution, bool, error) {
	return resolveStaticChangedPathResolutionsWithProbeFunctions(
		sourceDirectoryPath,
		changedSourcePaths,
		staticChangedPathResolutionProbeFunctions{
			normalizeChangedSourcePath: pathnorm.CanonicalizePathForLocationComparison,
			resolveStaticFileInfoFromSourcePath: func(
				sourceDirectoryPath string,
				sourcePath string,
			) (fileInfo, bool, error) {
				return resolveStaticFileInfoFromSourcePath(sourceDirectoryPath, sourcePath)
			},
			sourceFileExists: staticSourceFileExists,
			ensureNoStaticLogicalPathCollisionWithinSourceDirectoryForRelPath: func(
				sourceDirectoryPath string,
				relativePath string,
			) error {
				return ensureNoStaticLogicalPathCollisionWithinSourceDirectory(
					sourceDirectoryPath,
					relativePath,
				)
			},
		},
	)
}

func resolveStaticChangedPathResolutionsWithProbeFunctions(
	sourceDirectoryPath string,
	changedSourcePaths []string,
	probeFunctions staticChangedPathResolutionProbeFunctions,
) (map[string]staticChangedPathResolution, bool, error) {
	changedResolutions := make(map[string]staticChangedPathResolution)
	resolveProbeResultByNormalizedChangedSourcePath := make(
		map[string]staticChangedPathResolutionProbeResult,
	)
	sourceExistsByNormalizedChangedSourcePath := make(map[string]bool)
	collisionCheckedByRelativePath := make(map[string]struct{})

	normalizeChangedSourcePath := probeFunctions.normalizeChangedSourcePath
	if normalizeChangedSourcePath == nil {
		normalizeChangedSourcePath = pathnorm.CanonicalizePathForLocationComparison
	}

	normalizedSourceDirectoryPath := normalizeChangedSourcePath(sourceDirectoryPath)
	if normalizedSourceDirectoryPath != "" {
		sourceDirectoryPath = normalizedSourceDirectoryPath
	}
	resolveStaticFileInfoFromSourcePathProbe := probeFunctions.resolveStaticFileInfoFromSourcePath
	if resolveStaticFileInfoFromSourcePathProbe == nil {
		resolveStaticFileInfoFromSourcePathProbe = resolveStaticFileInfoFromSourcePath
	}
	sourceFileExistsProbe := probeFunctions.sourceFileExists
	if sourceFileExistsProbe == nil {
		sourceFileExistsProbe = staticSourceFileExists
	}
	collisionProbe := probeFunctions.ensureNoStaticLogicalPathCollisionWithinSourceDirectoryForRelPath
	if collisionProbe == nil {
		collisionProbe = ensureNoStaticLogicalPathCollisionWithinSourceDirectory
	}

	for _, changedSourcePath := range changedSourcePaths {
		normalizedChangedSourcePath := normalizeChangedSourcePath(changedSourcePath)
		if normalizedChangedSourcePath == "" {
			continue
		}
		if normalizedChangedSourcePath == sourceDirectoryPath {
			return nil, true, nil
		}

		resolutionProbeResult, hasResolutionProbeResult := resolveProbeResultByNormalizedChangedSourcePath[normalizedChangedSourcePath]
		if !hasResolutionProbeResult {
			staticFileInfo, shouldProcessFile, resolveError := resolveStaticFileInfoFromSourcePathProbe(
				sourceDirectoryPath,
				normalizedChangedSourcePath,
			)
			if resolveError != nil {
				return nil, false, fmt.Errorf(
					"resolve static file info for changed path %s: %w",
					changedSourcePath,
					resolveError,
				)
			}

			resolutionProbeResult = staticChangedPathResolutionProbeResult{
				staticFileInfo:    staticFileInfo,
				shouldProcessFile: shouldProcessFile,
			}
			resolveProbeResultByNormalizedChangedSourcePath[normalizedChangedSourcePath] = resolutionProbeResult
		}
		if !resolutionProbeResult.shouldProcessFile {
			continue
		}

		sourceExists, hasSourceExists := sourceExistsByNormalizedChangedSourcePath[normalizedChangedSourcePath]
		if !hasSourceExists {
			sourceExistsForPath, sourceStatError := sourceFileExistsProbe(normalizedChangedSourcePath)
			if sourceStatError != nil {
				return nil, false, sourceStatError
			}

			sourceExists = sourceExistsForPath
			sourceExistsByNormalizedChangedSourcePath[normalizedChangedSourcePath] = sourceExists
		}

		if sourceExists {
			relativePath := resolutionProbeResult.staticFileInfo.relPath
			if _, alreadyChecked := collisionCheckedByRelativePath[relativePath]; !alreadyChecked {
				collisionError := collisionProbe(sourceDirectoryPath, relativePath)
				if collisionError != nil {
					return nil, false, collisionError
				}
				collisionCheckedByRelativePath[relativePath] = struct{}{}
			}
		}

		relativePath := resolutionProbeResult.staticFileInfo.relPath
		existingResolution, alreadyResolved := changedResolutions[relativePath]
		if !alreadyResolved || sourceExists || !existingResolution.sourceExists {
			changedResolutions[relativePath] = staticChangedPathResolution{
				fileInfo:     resolutionProbeResult.staticFileInfo,
				sourceExists: sourceExists,
			}
		}
	}

	return changedResolutions, false, nil
}

func ensureNoStaticLogicalPathCollision(
	existingFileInfo fileInfo,
	candidateFileInfo fileInfo,
) error {
	if existingFileInfo.relPath != candidateFileInfo.relPath {
		return nil
	}

	if pathnorm.PathsReferToSameLocation(existingFileInfo.srcPath, candidateFileInfo.srcPath) {
		return nil
	}

	conflictingSourcePaths := []string{
		pathnorm.Absolute(existingFileInfo.srcPath),
		pathnorm.Absolute(candidateFileInfo.srcPath),
	}
	sort.Strings(conflictingSourcePaths)

	return fmt.Errorf(
		"static source path collision for logical path %q: %q and %q both map to the same output; keep exactly one source file",
		existingFileInfo.relPath,
		conflictingSourcePaths[0],
		conflictingSourcePaths[1],
	)
}

func ensureNoStaticLogicalPathCollisionWithinSourceDirectory(
	sourceDirectoryPath string,
	relativePath string,
) error {
	relativePathWithNativeSeparators := filepath.FromSlash(relativePath)
	candidateFileInfos := []fileInfo{
		{
			srcPath: filepath.Join(sourceDirectoryPath, relativePathWithNativeSeparators),
			relPath: relativePath,
			prehash: false,
		},
		{
			srcPath: filepath.Join(
				sourceDirectoryPath,
				wave.PrehashedDirname,
				relativePathWithNativeSeparators,
			),
			relPath: relativePath,
			prehash: true,
		},
		{
			srcPath: filepath.Join(
				sourceDirectoryPath,
				wave.NohashDirname,
				relativePathWithNativeSeparators,
			),
			relPath: relativePath,
			prehash: true,
		},
	}

	existingSourcePaths := make([]string, 0, len(candidateFileInfos))
	for _, candidateFileInfo := range candidateFileInfos {
		sourceExists, sourceStatError := staticSourceFileExists(candidateFileInfo.srcPath)
		if sourceStatError != nil {
			return sourceStatError
		}
		if sourceExists {
			existingSourcePaths = append(
				existingSourcePaths,
				pathnorm.Absolute(candidateFileInfo.srcPath),
			)
		}
	}

	if len(existingSourcePaths) <= 1 {
		return nil
	}

	sort.Strings(existingSourcePaths)
	return fmt.Errorf(
		"static source path collision for logical path %q: multiple source files map to the same output: %s; keep exactly one source file",
		relativePath,
		strings.Join(existingSourcePaths, ", "),
	)
}

func resolveStaticRelativePathFromSourcePath(
	sourceDirectoryPath string,
	sourcePath string,
) (string, bool, error) {
	relativePath, relativePathError := filepath.Rel(sourceDirectoryPath, sourcePath)
	if relativePathError != nil {
		return "", false, relativePathError
	}
	normalizedRelativePath := filepath.ToSlash(relativePath)
	if normalizedRelativePath == "." ||
		normalizedRelativePath == ".." ||
		strings.HasPrefix(normalizedRelativePath, "../") {
		return "", false, nil
	}
	return normalizedRelativePath, true, nil
}

func buildStaticFileInfoFromRelativePath(
	sourcePath string,
	relativePath string,
) (fileInfo, bool) {
	normalizedRelativePath := relativePath
	prehash := false
	prehashedPrefix := wave.PrehashedDirname + "/"
	nohashPrefix := wave.NohashDirname + "/"
	if strings.HasPrefix(normalizedRelativePath, prehashedPrefix) {
		prehash = true
		normalizedRelativePath = strings.TrimPrefix(normalizedRelativePath, prehashedPrefix)
	} else if strings.HasPrefix(normalizedRelativePath, nohashPrefix) {
		prehash = true
		normalizedRelativePath = strings.TrimPrefix(normalizedRelativePath, nohashPrefix)
	}

	if normalizedRelativePath == "" {
		return fileInfo{}, false
	}

	if _, ignore := staticIgnoreList[filepath.Base(normalizedRelativePath)]; ignore {
		return fileInfo{}, false
	}

	return fileInfo{
		srcPath: sourcePath,
		relPath: normalizedRelativePath,
		prehash: prehash,
	}, true
}

func resolveStaticFileInfoFromSourcePath(
	sourceDirectoryPath string,
	sourcePath string,
) (fileInfo, bool, error) {
	relativePath, isWithinSourceDirectory, relativePathError := resolveStaticRelativePathFromSourcePath(
		sourceDirectoryPath,
		sourcePath,
	)
	if relativePathError != nil {
		return fileInfo{}, false, relativePathError
	}
	if !isWithinSourceDirectory {
		return fileInfo{}, false, nil
	}

	staticFileInfo, shouldProcessFile := buildStaticFileInfoFromRelativePath(sourcePath, relativePath)
	return staticFileInfo, shouldProcessFile, nil
}

func (b *Builder) processStaticFiles(opts staticOpts) error {
	if _, err := os.Stat(opts.srcDir); os.IsNotExist(err) {
		if err := b.saveFileMap(wave.FileMap{}, opts.gobPath); err != nil {
			return err
		}
		if opts.isPublic {
			return b.savePublicFileMapJS(wave.FileMap{})
		}
		return nil
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

	// Use context for cancellation on error
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Discover files
	files := make(chan fileInfo, 100)
	discoveredFileInfosByRelativePath := make(map[string]fileInfo)

	// Track walk errors
	var walkErr error
	var walkErrOnce sync.Once

	go func() {
		defer close(files)
		err := filepath.WalkDir(opts.srcDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Check for cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if d.IsDir() {
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
				// Only record walk error if it's not just a cancellation
				// caused by a worker error (which is stored in firstErr)
				if !errors.Is(err, context.Canceled) {
					walkErr = fmt.Errorf("walk %s: %w", opts.srcDir, err)
				}
				cancel()
			})
		}
	}()

	// Process files with worker pool
	numWorkers := determineStaticProcessingWorkerCount(runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fi := range files {
				select {
				case <-ctx.Done():
					return
				default:
				}

				err := b.processFile(fi, opts, newMap, oldMap)
				if err != nil {
					errOnce.Do(func() {
						firstErr = err
						cancel() // Cancel other workers
					})
					return
				}
			}
		}()
	}

	wg.Wait()

	// Check for errors - prioritize worker errors over walk errors
	// since walk errors may just be "context canceled" from our cancellation
	if firstErr != nil {
		return firstErr
	}
	if walkErr != nil {
		return walkErr
	}

	// Convert to regular map
	finalMap := make(wave.FileMap)
	newMap.Range(func(k, v any) bool {
		finalMap[k.(string)] = v.(wave.FileVal)
		return true
	})

	// Cleanup old files
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

func staticSourceFileExists(sourcePath string) (bool, error) {
	sourceInfo, sourceStatError := os.Stat(sourcePath)
	if sourceStatError != nil {
		if os.IsNotExist(sourceStatError) {
			return false, nil
		}
		return false, fmt.Errorf("stat changed file %s: %w", sourcePath, sourceStatError)
	}

	return !sourceInfo.IsDir(), nil
}

func (b *Builder) processChangedStaticFile(
	staticFileInfo fileInfo,
	opts staticOpts,
	oldMap wave.FileMap,
) (wave.FileVal, bool, error) {
	processedFileValues := &sync.Map{}
	if err := b.processFile(staticFileInfo, opts, processedFileValues, oldMap); err != nil {
		if os.IsNotExist(err) {
			return wave.FileVal{}, false, nil
		}
		return wave.FileVal{}, false, err
	}

	processedValueAny, processedValueExists := processedFileValues.Load(staticFileInfo.relPath)
	if !processedValueExists {
		return wave.FileVal{}, false, fmt.Errorf("processed static file value missing for %s", staticFileInfo.relPath)
	}

	processedValue, valueTypeOK := processedValueAny.(wave.FileVal)
	if !valueTypeOK {
		return wave.FileVal{}, false, fmt.Errorf("processed static file value has unexpected type for %s", staticFileInfo.relPath)
	}

	return processedValue, true, nil
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

func (b *Builder) processFile(
	fi fileInfo,
	opts staticOpts,
	newMap *sync.Map,
	oldMap wave.FileMap,
) error {
	underscorePath := strings.ReplaceAll(fi.relPath, "/", "_")
	contentHash, err := hashFile(fi.srcPath, underscorePath)
	if err != nil {
		return fmt.Errorf("hash %s: %w", fi.srcPath, err)
	}

	var distName string
	if fi.prehash {
		distName = fi.relPath
	} else if !opts.hashOutput {
		distName = fi.relPath
	} else {
		distName = contentHash
	}

	val := wave.FileVal{
		DistName:    distName,
		ContentHash: contentHash,
		IsPrehashed: fi.prehash,
	}
	newMap.Store(fi.relPath, val)

	var distPath string
	if opts.hashOutput {
		distPath = filepath.Join(opts.distDir, distName)
	} else {
		distPath = filepath.Join(opts.distDir, fi.relPath)
	}

	// Skip if unchanged
	if oldMap != nil {
		if oldVal, ok := oldMap[fi.relPath]; ok {
			if oldVal.ContentHash == contentHash {
				if _, statErr := os.Stat(distPath); statErr == nil {
					return nil
				} else if !os.IsNotExist(statErr) {
					return statErr
				}
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(distPath), 0755); err != nil {
		return err
	}

	return fsutil.CopyFile(fi.srcPath, distPath)
}
func (b *Builder) loadFileMapFromPath(gobPath string) (wave.FileMap, error) {
	f, err := os.Open(gobPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return fsutil.FromGob[wave.FileMap](f)
}

func (b *Builder) saveFileMap(fm wave.FileMap, gobPath string) error {
	return writeFileAtomic(gobPath, func(f *os.File) error {
		return gob.NewEncoder(f).Encode(fm)
	})
}

func (b *Builder) savePublicFileMapJS(fm wave.FileMap) error {
	simpleMap := make(map[string]string, len(fm))
	for k, v := range fm {
		simpleMap[k] = v.DistName
	}

	jsonBytes, err := json.Marshal(simpleMap)
	if err != nil {
		return err
	}

	content := fmt.Sprintf("export const wavePublicFileMap = %s;", string(jsonBytes))
	hashedName := hashBytes([]byte(content), wave.RelPaths.PublicFileMapJSName())
	publicDir := b.cfg.Dist.StaticPublic()
	refPath := b.cfg.Dist.PublicFileMapRef()

	if existingRefBytes, readErr := os.ReadFile(refPath); readErr == nil {
		existingHashedName := strings.TrimSpace(string(existingRefBytes))
		if existingHashedName == hashedName {
			existingHashedPath := filepath.Join(publicDir, existingHashedName)
			if _, statErr := os.Stat(existingHashedPath); statErr == nil {
				return nil
			}
		}
	}

	// Cleanup old files
	oldFiles, err := filepath.Glob(filepath.Join(publicDir, wave.FileMapJSGlobPattern))
	if err != nil {
		b.log.Warn("failed to glob old filemap files", "error", err)
	}
	for _, old := range oldFiles {
		if err := os.Remove(old); err != nil {
			b.log.Warn("failed to remove old filemap file", "file", old, "error", err)
		}
	}

	// Write ref file atomically
	if err := writeFileAtomicBytes(refPath, []byte(hashedName)); err != nil {
		return err
	}

	// Write JS file atomically
	return writeFileAtomicBytes(filepath.Join(publicDir, hashedName), []byte(content))
}

// WritePublicFileMapTS writes the public file map as a TypeScript file to the specified directory.
// This enables Vite HMR to pick up public static file changes without running Go.
func (b *Builder) WritePublicFileMapTS(outDir string) error {
	fm, err := b.loadOrBuildFileMap()
	if err != nil {
		return fmt.Errorf("load file map: %w", err)
	}

	// Collect and sort keys for deterministic output
	keys := make([]string, 0, len(fm))
	for k := range fm {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString("/////// Auto-generated by Wave. Do not edit.\n\n")
	sb.WriteString("export const staticPublicAssetMap = {\n")

	for _, k := range keys {
		v := fm[k]
		sb.WriteString(fmt.Sprintf("\t%q: %q,\n", k, v.DistName))
	}

	sb.WriteString("} as const;\n")

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	outPath := filepath.Join(outDir, wave.RelPaths.PublicFileMapTSName())
	if _, err := writeFileAtomicBytesIfChanged(outPath, []byte(sb.String())); err != nil {
		return fmt.Errorf("write TS file: %w", err)
	}

	// Also write JSON version for Vite plugin dev mode cache invalidation
	if err := b.writePublicFileMapJSON(outDir, fm); err != nil {
		return fmt.Errorf("write JSON file: %w", err)
	}

	return nil
}

// writePublicFileMapJSON writes the public file map as a JSON file for the Vite plugin
// to read in dev mode. This allows the plugin to dynamically reload the filemap
// without restarting Vite.
func (b *Builder) writePublicFileMapJSON(outDir string, fm wave.FileMap) error {
	simpleMap := make(map[string]string, len(fm))
	for k, v := range fm {
		simpleMap[k] = v.DistName
	}

	jsonBytes, err := json.MarshalIndent(simpleMap, "", "\t")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	outPath := filepath.Join(outDir, wave.RelPaths.PublicFileMapJSONName())
	_, err = writeFileAtomicBytesIfChanged(outPath, jsonBytes)
	return err
}

// writeFileAtomic writes data to a file atomically using a randomized temp file
// and rename. The write function is called with the temp file to write content.
func writeFileAtomic(path string, write func(*os.File) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Create temp file with randomized name to avoid race conditions
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file on any error
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	// Write content
	if err := write(tmpFile); err != nil {
		tmpFile.Close()
		return err
	}

	// Close before rename
	if err := tmpFile.Close(); err != nil {
		return err
	}

	// Rename to final destination
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	success = true
	return nil
}

// writeFileAtomicBytes is a convenience wrapper for writing byte slices atomically
func writeFileAtomicBytes(path string, data []byte) error {
	return writeFileAtomic(path, func(f *os.File) error {
		_, err := f.Write(data)
		return err
	})
}

func writeFileAtomicBytesIfChanged(path string, data []byte) (bool, error) {
	existingData, readErr := os.ReadFile(path)
	if readErr == nil {
		if bytes.Equal(existingData, data) {
			return false, nil
		}
	} else if !os.IsNotExist(readErr) {
		return false, readErr
	}

	if err := writeFileAtomicBytes(path, data); err != nil {
		return false, err
	}

	return true, nil
}
