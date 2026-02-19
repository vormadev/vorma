package static

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/waveshared"
)

var staticIgnoreList = map[string]struct{}{
	".DS_Store": {},
}

// Processor owns static asset processing and file-map persistence.
type Processor struct {
	cfg                   *wave.ParsedConfig
	log                   *slog.Logger
	cachedPublicFileMap   wave.FileMap
	cachedPublicFileMapMu sync.Mutex
}

// NewProcessor constructs a static processor for one parsed config.
func NewProcessor(cfg *wave.ParsedConfig, log *slog.Logger) *Processor {
	return &Processor{
		cfg: cfg,
		log: log,
	}
}

// ProcessPublicFiles processes public static files.
func (b *Processor) ProcessPublicFiles(granular bool) error {
	b.ResetPublicFileMapCache()
	return b.processStaticFiles(staticOpts{
		srcDir:     waveshared.Absolute(b.cfg.Core.StaticAssetDirs.Public),
		distDir:    b.cfg.Dist.StaticPublic(),
		gobPath:    b.cfg.Dist.PublicFileMapGob(),
		granular:   granular,
		isPublic:   true,
		hashOutput: true,
	})
}

// ProcessPrivateFiles processes private static files.
func (b *Processor) ProcessPrivateFiles(granular bool) error {
	return b.processStaticFiles(staticOpts{
		srcDir:     waveshared.Absolute(b.cfg.Core.StaticAssetDirs.Private),
		distDir:    b.cfg.Dist.StaticPrivate(),
		gobPath:    b.cfg.Dist.PrivateFileMapGob(),
		granular:   granular,
		isPublic:   false,
		hashOutput: false,
	})
}

// ProcessPublicFilesForChangedPaths processes changed public static paths only.
func (b *Processor) ProcessPublicFilesForChangedPaths(changedSourcePaths []string) error {
	b.ResetPublicFileMapCache()
	return b.processStaticFilesForChangedPaths(
		staticOpts{
			srcDir:     waveshared.Absolute(b.cfg.Core.StaticAssetDirs.Public),
			distDir:    b.cfg.Dist.StaticPublic(),
			gobPath:    b.cfg.Dist.PublicFileMapGob(),
			granular:   true,
			isPublic:   true,
			hashOutput: true,
		},
		changedSourcePaths,
	)
}

// ProcessPrivateFilesForChangedPaths processes changed private static paths only.
func (b *Processor) ProcessPrivateFilesForChangedPaths(changedSourcePaths []string) error {
	return b.processStaticFilesForChangedPaths(
		staticOpts{
			srcDir:     waveshared.Absolute(b.cfg.Core.StaticAssetDirs.Private),
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

// DetermineStaticProcessingWorkerCount derives worker count from GOMAXPROCS.
func DetermineStaticProcessingWorkerCount(gomaxprocs int) int {
	return determineStaticProcessingWorkerCount(gomaxprocs)
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

// RemoveStaticMapEntriesForChangedRelativePaths removes map entries by changed paths.
func RemoveStaticMapEntriesForChangedRelativePaths(
	staticMap wave.FileMap,
	distDirectoryPath string,
	changedRelativePaths []string,
) bool {
	return removeStaticMapEntriesForChangedRelativePaths(
		staticMap,
		distDirectoryPath,
		changedRelativePaths,
	)
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

func (b *Processor) processStaticFileAndReturnValue(
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

func (b *Processor) processFile(
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

func (b *Processor) processStaticFilesForChangedPaths(
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

	oldMap, loadError := b.LoadFileMapFromPath(opts.gobPath)
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

	if err := b.SaveFileMap(oldMap, opts.gobPath); err != nil {
		return err
	}

	if opts.isPublic {
		return b.SavePublicFileMapJS(oldMap)
	}

	return nil
}

func (b *Processor) processChangedStaticFile(
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

func (b *Processor) processStaticFiles(opts staticOpts) error {
	if _, err := os.Stat(opts.srcDir); os.IsNotExist(err) {
		return b.clearStaticOutputsWhenSourceDirectoryMissing(opts)
	}

	newMap := &sync.Map{}
	var oldMap wave.FileMap
	oldMapLoaded := false

	if opts.granular {
		old, err := b.LoadFileMapFromPath(opts.gobPath)
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
		if err := b.SaveFileMap(finalMap, opts.gobPath); err != nil {
			return err
		}
	}

	if opts.isPublic {
		return b.SavePublicFileMapJS(finalMap)
	}

	return nil
}

func (b *Processor) clearStaticOutputsWhenSourceDirectoryMissing(
	opts staticOpts,
) error {
	emptyMap := wave.FileMap{}

	previousMap, loadError := b.LoadFileMapFromPath(opts.gobPath)
	if loadError == nil {
		cleanupStaleStaticDistFiles(opts.distDir, previousMap, emptyMap)
	}

	if err := b.SaveFileMap(emptyMap, opts.gobPath); err != nil {
		return err
	}
	if opts.isPublic {
		return b.SavePublicFileMapJS(emptyMap)
	}
	return nil
}

func (b *Processor) LoadFileMapFromPath(gobPath string) (wave.FileMap, error) {
	f, err := os.Open(gobPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return fsutil.FromGob[wave.FileMap](f)
}

func (b *Processor) SaveFileMap(fm wave.FileMap, gobPath string) error {
	return writeFileAtomic(gobPath, func(f *os.File) error {
		return gob.NewEncoder(f).Encode(fm)
	})
}

func (b *Processor) SavePublicFileMapJS(fm wave.FileMap) error {
	simpleMap := make(map[string]string, len(fm))
	for k, v := range fm {
		simpleMap[k] = v.DistName
	}

	jsonBytes, err := json.Marshal(simpleMap)
	if err != nil {
		return err
	}

	content := fmt.Sprintf(
		"export const wavePublicFileMap = %s;",
		string(jsonBytes),
	)
	hashedName := hashBytes(
		[]byte(content),
		wave.RelPaths.PublicFileMapJSName(),
	)
	_, publishError := publishHashedArtifactWithRef(
		hashedArtifactPublishOptions{
			log:                   b.log,
			outputDirectoryPath:   b.cfg.Dist.StaticPublic(),
			refFilePath:           b.cfg.Dist.PublicFileMapRef(),
			desiredHashedFileName: hashedName,
			content:               []byte(content),
			globPattern:           wave.FileMapJSGlobPattern,
		},
	)
	return publishError
}

// WritePublicFileMapTS writes the public file map as a TypeScript file to the specified directory.
// This enables Vite HMR to pick up public static file changes without running Go.
func (b *Processor) WritePublicFileMapTS(outDir string) error {
	fm, err := b.loadOrBuildFileMap()
	if err != nil {
		return fmt.Errorf("load file map: %w", err)
	}

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

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	outPath := filepath.Join(outDir, wave.RelPaths.PublicFileMapTSName())
	if _, err := writeFileAtomicBytesIfChanged(outPath, []byte(sb.String())); err != nil {
		return fmt.Errorf("write TS file: %w", err)
	}

	if err := b.writePublicFileMapJSON(outDir, fm); err != nil {
		return fmt.Errorf("write JSON file: %w", err)
	}

	return nil
}

// writePublicFileMapJSON writes the public file map as a JSON file for the Vite plugin
// to read in dev mode. This allows the plugin to dynamically reload the filemap
// without restarting Vite.
func (b *Processor) writePublicFileMapJSON(outDir string, fm wave.FileMap) error {
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

type atomicFileWriteDependencies struct {
	renameTempFile       func(string, string) error
	removeExistingTarget func(string) error
	statTarget           func(string) (os.FileInfo, error)
}

// AtomicFileWriteDependencies customizes filesystem operations for atomic writes.
type AtomicFileWriteDependencies struct {
	RenameTempFile       func(string, string) error
	RemoveExistingTarget func(string) error
	StatTarget           func(string) (os.FileInfo, error)
}

func normalizeExportedAtomicFileWriteDependencies(
	dependencies AtomicFileWriteDependencies,
) atomicFileWriteDependencies {
	return atomicFileWriteDependencies{
		renameTempFile:       dependencies.RenameTempFile,
		removeExistingTarget: dependencies.RemoveExistingTarget,
		statTarget:           dependencies.StatTarget,
	}
}

func defaultAtomicFileWriteDependencies() atomicFileWriteDependencies {
	return atomicFileWriteDependencies{
		renameTempFile:       os.Rename,
		removeExistingTarget: os.Remove,
		statTarget:           os.Stat,
	}
}

func normalizeAtomicFileWriteDependencies(
	dependencies atomicFileWriteDependencies,
) atomicFileWriteDependencies {
	defaultDependencies := defaultAtomicFileWriteDependencies()
	if dependencies.renameTempFile == nil {
		dependencies.renameTempFile = defaultDependencies.renameTempFile
	}
	if dependencies.removeExistingTarget == nil {
		dependencies.removeExistingTarget = defaultDependencies.removeExistingTarget
	}
	if dependencies.statTarget == nil {
		dependencies.statTarget = defaultDependencies.statTarget
	}
	return dependencies
}

func shouldRetryRenameByReplacingTarget(
	renameError error,
	targetPath string,
	dependencies atomicFileWriteDependencies,
) bool {
	if errors.Is(renameError, fs.ErrExist) ||
		errors.Is(renameError, os.ErrExist) {
		return true
	}

	if errors.Is(renameError, fs.ErrPermission) ||
		errors.Is(renameError, os.ErrPermission) {
		_, targetStatError := dependencies.statTarget(targetPath)
		return targetStatError == nil
	}

	return false
}

// writeFileAtomic writes data to a file atomically using a randomized temp file
// and rename. The write function is called with the temp file to write content.
func writeFileAtomic(path string, write func(*os.File) error) error {
	return writeFileAtomicWithDependencies(
		path,
		write,
		atomicFileWriteDependencies{},
	)
}

// WriteFileAtomic writes a file atomically with a temp file + rename.
func WriteFileAtomic(path string, write func(*os.File) error) error {
	return writeFileAtomic(path, write)
}

func writeFileAtomicWithDependencies(
	path string,
	write func(*os.File) error,
	dependencies atomicFileWriteDependencies,
) error {
	dependencies = normalizeAtomicFileWriteDependencies(dependencies)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	if err := write(tmpFile); err != nil {
		tmpFile.Close()
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	renameError := dependencies.renameTempFile(tmpPath, path)
	if renameError != nil {
		if !shouldRetryRenameByReplacingTarget(
			renameError,
			path,
			dependencies,
		) {
			return fmt.Errorf("rename temp file: %w", renameError)
		}

		removeExistingTargetError := dependencies.removeExistingTarget(path)
		if removeExistingTargetError != nil &&
			!os.IsNotExist(removeExistingTargetError) {
			return fmt.Errorf(
				"remove existing target file before rename: %w",
				removeExistingTargetError,
			)
		}

		retryRenameError := dependencies.renameTempFile(tmpPath, path)
		if retryRenameError != nil {
			return fmt.Errorf(
				"rename temp file after replacing existing target: %w",
				retryRenameError,
			)
		}
	}

	success = true
	return nil
}

// WriteFileAtomicWithDependencies writes a file atomically using custom fs ops.
func WriteFileAtomicWithDependencies(
	path string,
	write func(*os.File) error,
	dependencies AtomicFileWriteDependencies,
) error {
	return writeFileAtomicWithDependencies(
		path,
		write,
		normalizeExportedAtomicFileWriteDependencies(dependencies),
	)
}

func writeFileAtomicBytes(path string, data []byte) error {
	return writeFileAtomic(path, func(f *os.File) error {
		_, err := f.Write(data)
		return err
	})
}

// WriteFileAtomicBytes writes bytes to a file atomically.
func WriteFileAtomicBytes(path string, data []byte) error {
	return writeFileAtomicBytes(path, data)
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

// WriteFileAtomicBytesIfChanged writes bytes atomically when content changed.
func WriteFileAtomicBytesIfChanged(path string, data []byte) (bool, error) {
	return writeFileAtomicBytesIfChanged(path, data)
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

func ensureNoStaticLogicalPathCollision(
	existingFileInfo fileInfo,
	candidateFileInfo fileInfo,
) error {
	if existingFileInfo.relPath != candidateFileInfo.relPath {
		return nil
	}

	if waveshared.PathsReferToSameLocation(existingFileInfo.srcPath, candidateFileInfo.srcPath) {
		return nil
	}

	conflictingSourcePaths := []string{
		waveshared.Absolute(existingFileInfo.srcPath),
		waveshared.Absolute(candidateFileInfo.srcPath),
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
				waveshared.Absolute(candidateFileInfo.srcPath),
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

type staticChangedPathResolution struct {
	fileInfo     fileInfo
	sourceExists bool
}

// StaticFileInfo describes one static source file resolution.
type StaticFileInfo struct {
	SourcePath   string
	RelativePath string
	IsPrehashed  bool
}

func convertStaticFileInfoToInternal(staticFileInfo StaticFileInfo) fileInfo {
	return fileInfo{
		srcPath: staticFileInfo.SourcePath,
		relPath: staticFileInfo.RelativePath,
		prehash: staticFileInfo.IsPrehashed,
	}
}

func convertStaticFileInfoFromInternal(staticFileInfo fileInfo) StaticFileInfo {
	return StaticFileInfo{
		SourcePath:   staticFileInfo.srcPath,
		RelativePath: staticFileInfo.relPath,
		IsPrehashed:  staticFileInfo.prehash,
	}
}

// StaticChangedPathResolution describes processing state for one relative path.
type StaticChangedPathResolution struct {
	FileInfo     StaticFileInfo
	SourceExists bool
}

func convertStaticChangedPathResolutionFromInternal(
	resolution staticChangedPathResolution,
) StaticChangedPathResolution {
	return StaticChangedPathResolution{
		FileInfo:     convertStaticFileInfoFromInternal(resolution.fileInfo),
		SourceExists: resolution.sourceExists,
	}
}

func convertStaticChangedPathResolutionMapFromInternal(
	changedResolutions map[string]staticChangedPathResolution,
) map[string]StaticChangedPathResolution {
	convertedChangedResolutions := make(
		map[string]StaticChangedPathResolution,
		len(changedResolutions),
	)
	for relativePath, resolution := range changedResolutions {
		convertedChangedResolutions[relativePath] = convertStaticChangedPathResolutionFromInternal(resolution)
	}
	return convertedChangedResolutions
}

type staticChangedPathResolutionProbeFunctions struct {
	normalizeChangedSourcePath                                        func(string) string
	resolveStaticFileInfoFromSourcePath                               func(string, string) (fileInfo, bool, error)
	sourceFileExists                                                  func(string) (bool, error)
	ensureNoStaticLogicalPathCollisionWithinSourceDirectoryForRelPath func(string, string) error
}

// StaticChangedPathResolutionProbeFunctions customizes changed-path resolution.
type StaticChangedPathResolutionProbeFunctions struct {
	NormalizeChangedSourcePath          func(string) string
	ResolveStaticFileInfoFromSourcePath func(
		string,
		string,
	) (StaticFileInfo, bool, error)
	SourceFileExists                                                       func(string) (bool, error)
	EnsureNoStaticLogicalPathCollisionWithinSourceDirectoryForRelativePath func(
		string,
		string,
	) error
}

func normalizeExportedStaticChangedPathResolutionProbeFunctions(
	probeFunctions StaticChangedPathResolutionProbeFunctions,
) staticChangedPathResolutionProbeFunctions {
	var resolveStaticFileInfoFromSourcePathProbe func(
		string,
		string,
	) (fileInfo, bool, error)
	if probeFunctions.ResolveStaticFileInfoFromSourcePath != nil {
		resolveStaticFileInfoFromSourcePathProbe = func(
			sourceDirectoryPath string,
			sourcePath string,
		) (fileInfo, bool, error) {
			staticFileInfo, shouldProcessFile, resolveError := probeFunctions.ResolveStaticFileInfoFromSourcePath(
				sourceDirectoryPath,
				sourcePath,
			)
			if resolveError != nil {
				return fileInfo{}, false, resolveError
			}
			return convertStaticFileInfoToInternal(staticFileInfo), shouldProcessFile, nil
		}
	}

	return staticChangedPathResolutionProbeFunctions{
		normalizeChangedSourcePath:          probeFunctions.NormalizeChangedSourcePath,
		resolveStaticFileInfoFromSourcePath: resolveStaticFileInfoFromSourcePathProbe,
		sourceFileExists:                    probeFunctions.SourceFileExists,
		ensureNoStaticLogicalPathCollisionWithinSourceDirectoryForRelPath: probeFunctions.EnsureNoStaticLogicalPathCollisionWithinSourceDirectoryForRelativePath,
	}
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
			normalizeChangedSourcePath: waveshared.CanonicalizePathForLocationComparison,
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

// ResolveStaticChangedPathResolutions resolves changed source paths for static processing.
func ResolveStaticChangedPathResolutions(
	sourceDirectoryPath string,
	changedSourcePaths []string,
) (map[string]StaticChangedPathResolution, bool, error) {
	changedResolutions, fullBuildRequired, resolutionError := resolveStaticChangedPathResolutions(
		sourceDirectoryPath,
		changedSourcePaths,
	)
	if resolutionError != nil {
		return nil, false, resolutionError
	}
	return convertStaticChangedPathResolutionMapFromInternal(changedResolutions), fullBuildRequired, nil
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
		normalizeChangedSourcePath = waveshared.CanonicalizePathForLocationComparison
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

// ResolveStaticChangedPathResolutionsWithProbeFunctions resolves changed paths with probes.
func ResolveStaticChangedPathResolutionsWithProbeFunctions(
	sourceDirectoryPath string,
	changedSourcePaths []string,
	probeFunctions StaticChangedPathResolutionProbeFunctions,
) (map[string]StaticChangedPathResolution, bool, error) {
	changedResolutions, fullBuildRequired, resolutionError := resolveStaticChangedPathResolutionsWithProbeFunctions(
		sourceDirectoryPath,
		changedSourcePaths,
		normalizeExportedStaticChangedPathResolutionProbeFunctions(probeFunctions),
	)
	if resolutionError != nil {
		return nil, false, resolutionError
	}
	return convertStaticChangedPathResolutionMapFromInternal(changedResolutions), fullBuildRequired, nil
}

// hashFile computes a content-addressed filename.
// Includes originalName in the hash to prevent collisions when two files
// have identical content but different normalized names.
func hashFile(filePath, originalName string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()

	// Include original name to prevent collision edge case where:
	// 1. Two files have the same content
	// 2. Their underscore-normalized names would collide
	h.Write([]byte(originalName))

	buf := make([]byte, 32*1024)

	for {
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	return formatHashedName(h, originalName), nil
}

// hashBytes computes a content-addressed filename for bytes
func hashBytes(content []byte, originalName string) string {
	h := sha256.New()
	h.Write([]byte(originalName)) // Include name to prevent collisions
	h.Write(content)
	return formatHashedName(h, originalName)
}

// HashFile computes a content-addressed filename for a file.
func HashFile(filePath string, originalName string) (string, error) {
	return hashFile(filePath, originalName)
}

// HashBytes computes a content-addressed filename for bytes.
func HashBytes(content []byte, originalName string) string {
	return hashBytes(content, originalName)
}

func formatHashedName(h hash.Hash, originalName string) string {
	hashStr := fmt.Sprintf("%x", h.Sum(nil))[:12]
	ext := filepath.Ext(originalName)
	base := strings.TrimSuffix(originalName, ext)
	return fmt.Sprintf("%s%s_%s%s", wave.HashedOutputPrefix, base, hashStr, ext)
}

type hashedArtifactPublishOptions struct {
	log                   *slog.Logger
	outputDirectoryPath   string
	refFilePath           string
	desiredHashedFileName string
	content               []byte
	globPattern           string
}

// HashedArtifactPublishOptions defines how a hashed artifact publish should run.
type HashedArtifactPublishOptions struct {
	Log                   *slog.Logger
	OutputDirectoryPath   string
	RefFilePath           string
	DesiredHashedFileName string
	Content               []byte
	GlobPattern           string
}

func publishHashedArtifactWithRef(
	opts hashedArtifactPublishOptions,
) (string, error) {
	if err := os.MkdirAll(opts.outputDirectoryPath, 0o755); err != nil {
		return "", fmt.Errorf("mkdir output directory: %w", err)
	}

	desiredOutputPath := filepath.Join(
		opts.outputDirectoryPath,
		opts.desiredHashedFileName,
	)
	previousHashedFileName, hasPreviousRefFile, readRefError := readHashedArtifactRefFileName(
		opts.refFilePath,
	)
	if readRefError != nil {
		return "", readRefError
	}
	hasValidPreviousRefFile := hasPreviousRefFile &&
		previousHashedFileName != ""

	if hasValidPreviousRefFile &&
		previousHashedFileName == opts.desiredHashedFileName {
		if _, statError := os.Stat(desiredOutputPath); statError == nil {
			return opts.desiredHashedFileName, nil
		} else if !os.IsNotExist(statError) {
			return "", statError
		}
	}

	if hasValidPreviousRefFile &&
		previousHashedFileName != opts.desiredHashedFileName {
		removeHashedArtifactIfPresent(
			opts.log,
			filepath.Join(opts.outputDirectoryPath, previousHashedFileName),
		)
	}
	if !hasValidPreviousRefFile {
		cleanupOldHashedArtifactsWhenRefFileMissing(
			opts.log,
			opts.outputDirectoryPath,
			opts.globPattern,
			opts.desiredHashedFileName,
		)
	}

	if _, writeError := writeFileAtomicBytesIfChanged(desiredOutputPath, opts.content); writeError != nil {
		return "", writeError
	}

	if _, writeError := writeFileAtomicBytesIfChanged(opts.refFilePath, []byte(opts.desiredHashedFileName)); writeError != nil {
		return "", fmt.Errorf("write hashed artifact ref: %w", writeError)
	}

	return opts.desiredHashedFileName, nil
}

// PublishHashedArtifactWithRef publishes a hashed artifact and updates its ref.
func PublishHashedArtifactWithRef(
	opts HashedArtifactPublishOptions,
) (string, error) {
	return publishHashedArtifactWithRef(
		hashedArtifactPublishOptions{
			log:                   opts.Log,
			outputDirectoryPath:   opts.OutputDirectoryPath,
			refFilePath:           opts.RefFilePath,
			desiredHashedFileName: opts.DesiredHashedFileName,
			content:               opts.Content,
			globPattern:           opts.GlobPattern,
		},
	)
}

func readHashedArtifactRefFileName(
	refFilePath string,
) (string, bool, error) {
	existingRefData, readError := os.ReadFile(refFilePath)
	if readError != nil {
		if os.IsNotExist(readError) {
			return "", false, nil
		}
		return "", false, readError
	}

	return strings.TrimSpace(string(existingRefData)), true, nil
}

func removeHashedArtifactIfPresent(
	logger *slog.Logger,
	artifactPath string,
) {
	if removeError := os.Remove(artifactPath); removeError != nil &&
		!os.IsNotExist(removeError) {
		if logger != nil {
			logger.Warn(
				"failed to remove old hashed artifact",
				"file",
				artifactPath,
				"error",
				removeError,
			)
		}
	}
}

func cleanupOldHashedArtifactsWhenRefFileMissing(
	logger *slog.Logger,
	outputDirectoryPath string,
	globPattern string,
	desiredHashedFileName string,
) {
	oldFiles, globError := filepath.Glob(
		filepath.Join(outputDirectoryPath, globPattern),
	)
	if globError != nil {
		if logger != nil {
			logger.Warn(
				"failed to glob old hashed artifacts",
				"error",
				globError,
			)
		}
		return
	}

	for _, oldFilePath := range oldFiles {
		if filepath.Base(oldFilePath) == desiredHashedFileName {
			continue
		}
		if removeError := os.Remove(oldFilePath); removeError != nil {
			if logger != nil {
				logger.Warn(
					"failed to remove old hashed artifact",
					"file",
					oldFilePath,
					"error",
					removeError,
				)
			}
		}
	}
}

// ResetPublicFileMapCache clears cached build-time public file map state.
func (b *Processor) ResetPublicFileMapCache() {
	b.cachedPublicFileMapMu.Lock()
	b.cachedPublicFileMap = nil
	b.cachedPublicFileMapMu.Unlock()
}

// GetPublicURLBuildtimeCached resolves a public URL using cached file map.
// Panics when lookup or map load fails.
func (b *Processor) GetPublicURLBuildtimeCached(original string) string {
	b.cachedPublicFileMapMu.Lock()
	defer b.cachedPublicFileMapMu.Unlock()

	if b.cachedPublicFileMap == nil {
		fileMap, loadError := b.LoadFileMapFromPath(b.cfg.Dist.PublicFileMapGob())
		if loadError != nil {
			panic(fmt.Errorf("load file map for CSS URL resolution: %w", loadError))
		}
		b.cachedPublicFileMap = fileMap
	}

	url, found := b.cachedPublicFileMap.Lookup(original, b.cfg.PublicPathPrefix())
	if !found {
		panic(fmt.Errorf("no hashed URL found for %q", original))
	}
	return url
}

// MustPublicURLBuildtime resolves a public URL at build time.
// This function reads the file map from disk on each call and panics on error.
// For hot paths (like CSS URL resolution), use GetPublicURLBuildtimeCached instead.
func (b *Processor) MustPublicURLBuildtime(original string) string {
	fm, err := b.LoadFileMapFromPath(b.cfg.Dist.PublicFileMapGob())
	if err != nil {
		b.log.Error("failed to load file map", "error", err)
		panic(err)
	}

	url, found := fm.Lookup(original, b.cfg.PublicPathPrefix())
	if !found {
		resolveError := fmt.Errorf("no hashed URL found for %q", original)
		b.log.Error("no hashed URL found", "url", original)
		panic(resolveError)
	}
	return url
}

// PublicURLBuildtime resolves a public URL at build time without panicking.
// Returns the resolved URL and any error that occurred.
func (b *Processor) PublicURLBuildtime(original string) (string, error) {
	fm, err := b.LoadFileMapFromPath(b.cfg.Dist.PublicFileMapGob())
	if err != nil {
		return "", err
	}

	url, found := fm.Lookup(original, b.cfg.PublicPathPrefix())
	if !found {
		return "", fmt.Errorf("no hashed URL found for %q", original)
	}
	return url, nil
}

// PublicFileMapKeys returns sorted keys of non-prehashed public files
func (b *Processor) PublicFileMapKeys() ([]string, error) {
	fm, err := b.loadOrBuildFileMap()
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(fm))
	for k, v := range fm {
		if !v.IsPrehashed {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

// SimplePublicFileMap returns a simple path->distName map
func (b *Processor) SimplePublicFileMap() (map[string]string, error) {
	fm, err := b.loadOrBuildFileMap()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(fm))
	for k, v := range fm {
		if !v.IsPrehashed {
			result[k] = v.DistName
		}
	}
	return result, nil
}

func (b *Processor) loadOrBuildFileMap() (wave.FileMap, error) {
	fm, err := b.LoadFileMapFromPath(b.cfg.Dist.PublicFileMapGob())
	if err != nil {
		if !b.cfg.UsingBrowser() {
			return nil, err
		}
		if buildErr := b.ProcessPublicFiles(false); buildErr != nil {
			return nil, fmt.Errorf("build files: %w", buildErr)
		}
		fm, err = b.LoadFileMapFromPath(b.cfg.Dist.PublicFileMapGob())
	}
	return fm, err
}

// LoadPublicFileMap loads the public file map from disk (for build-time use)
func (b *Processor) LoadPublicFileMap() (wave.FileMap, error) {
	return b.LoadFileMapFromPath(b.cfg.Dist.PublicFileMapGob())
}

// AddPublicAssetKeys adds public asset keys for TypeScript generation.
func (b *Processor) AddPublicAssetKeys(
	statements *tsgen.Statements,
) (*tsgen.Statements, error) {
	resolvedStatements := statements
	if resolvedStatements == nil {
		resolvedStatements = &tsgen.Statements{}
	}

	keys, err := b.PublicFileMapKeys()
	if err != nil {
		return nil, fmt.Errorf("public file map keys: %w", err)
	}

	resolvedStatements.MustSerialize("const WAVE_PUBLIC_ASSETS", keys)
	resolvedStatements.Raw(
		"export type WavePublicAsset",
		"`${\"/\" | \"\"}${(typeof WAVE_PUBLIC_ASSETS)[number]}`",
	)

	return resolvedStatements, nil
}
