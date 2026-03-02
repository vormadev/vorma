// Package static owns Wave static-asset planning, hashing, and public map
// generation.
//
// This package isolates static processing rules so builder orchestration can
// stay focused on phase control rather than asset transformation details.
package static

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/vormadev/vorma/internal/artifactio"
	"github.com/vormadev/vorma/wave/buildtime/builder/internal/fileops"
	"github.com/vormadev/vorma/wave/internal/wavefilemap"
	"github.com/vormadev/vorma/wave/internal/wavefs"
	"github.com/vormadev/vorma/wave/waveartifacts"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveenv"
)

var staticIgnoreList = map[string]struct{}{
	".DS_Store": {},
}

type fileInfo struct {
	srcPath string
	relPath string
	prehash bool
}

// fileMapStore defines source/destination roots and metadata paths for one file map.
type fileMapStore struct {
	sourceRootPath string
	outputRootPath string
	gobPath        string
}

// Processor owns static artifact processing and file map persistence.
type Processor struct {
	cfg *waveconfig.ParsedConfig
	log *slog.Logger

	mu sync.Mutex

	publicStore  fileMapStore
	privateStore fileMapStore

	cachedPublicFileMap  wavefilemap.FileMap
	cachedPrivateFileMap wavefilemap.FileMap
}

// NewProcessor creates static processor for one parsed config.
func NewProcessor(cfg *waveconfig.ParsedConfig, log *slog.Logger) *Processor {
	if log == nil {
		log = slog.Default()
	}

	processor := &Processor{
		cfg: cfg,
		log: log,
	}
	if cfg != nil {
		processor.publicStore = fileMapStore{
			sourceRootPath: filepath.Clean(cfg.Core.StaticAssetDirs.Public),
			outputRootPath: cfg.Dist.StaticPublic(),
			gobPath:        cfg.Dist.PublicFileMapGob(),
		}
		processor.privateStore = fileMapStore{
			sourceRootPath: filepath.Clean(cfg.Core.StaticAssetDirs.Private),
			outputRootPath: cfg.Dist.StaticPrivate(),
			gobPath:        cfg.Dist.PrivateFileMapGob(),
		}
	}
	return processor
}

// ProcessPublicFilesOnly performs full-scan processing for public static assets.
func (processor *Processor) ProcessPublicFilesOnly() error {
	return processor.processFullScan(processor.publicStore, true)
}

// ProcessPrivateFilesOnly performs full-scan processing for private static assets.
func (processor *Processor) ProcessPrivateFilesOnly() error {
	return processor.processFullScan(processor.privateStore, false)
}

// ProcessPublicFilesOnlyForChangedPaths performs incremental public static processing.
func (processor *Processor) ProcessPublicFilesOnlyForChangedPaths(
	changedPaths []string,
) error {
	return processor.processChangedPaths(
		processor.publicStore,
		true,
		changedPaths,
	)
}

// ProcessPrivateFilesOnlyForChangedPaths performs incremental private static processing.
func (processor *Processor) ProcessPrivateFilesOnlyForChangedPaths(
	changedPaths []string,
) error {
	return processor.processChangedPaths(
		processor.privateStore,
		false,
		changedPaths,
	)
}

// WriteCanonicalPublicFileMapJSONAndRef writes the canonical hashed public
// filemap JSON artifact plus ref metadata.
func (processor *Processor) WriteCanonicalPublicFileMapJSONAndRef() error {
	if processor == nil || processor.cfg == nil {
		return errors.New("static processor config is nil")
	}

	publicFileMap, loadError := processor.currentPublicFileMap()
	if loadError != nil {
		return loadError
	}

	serializedFileMap, marshalError := serializePublicFileMapForFramework(
		publicFileMap,
	)
	if marshalError != nil {
		return marshalError
	}

	hashSum := sha256.Sum256(serializedFileMap)
	hashPrefix := hex.EncodeToString(hashSum[:])[:16]
	fileName := waveartifacts.ApplyWaveFileOutputPrefix(
		waveartifacts.ApplyWaveOwnedFileOutputPrefix(
			fmt.Sprintf("public_filemap_%s.json", hashPrefix),
		),
	)
	outputPath := filepath.Join(processor.cfg.Dist.StaticPublic(), fileName)
	refPath := processor.cfg.Dist.PublicFileMapRef()

	if writeError := artifactio.WriteFileAtomically(outputPath, serializedFileMap, 0o644); writeError != nil {
		return writeError
	}
	if writeRefError := artifactio.WriteFileAtomically(refPath, []byte(fileName), 0o644); writeRefError != nil {
		return writeRefError
	}

	processor.log.Debug(
		"wrote canonical public filemap",
		"path",
		outputPath,
		"entries",
		len(publicFileMap),
	)
	return nil
}

// LoadFileMapFromPath loads a gob-encoded file map from disk.
func (processor *Processor) LoadFileMapFromPath(
	path string,
) (wavefilemap.FileMap, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("file map path is empty")
	}

	file, openError := os.Open(path)
	if openError != nil {
		return nil, openError
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)
	fileMap := make(wavefilemap.FileMap)
	if decodeError := decoder.Decode(&fileMap); decodeError != nil {
		return nil, decodeError
	}
	return fileMap, nil
}

// SaveFileMap writes gob-encoded file map atomically.
func (processor *Processor) SaveFileMap(
	fileMap wavefilemap.FileMap,
	path string,
) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("file map path is empty")
	}

	buffer := &strings.Builder{}
	encoder := gob.NewEncoder(builderWriter{builder: buffer})
	if encodeError := encoder.Encode(fileMap); encodeError != nil {
		return encodeError
	}
	return artifactio.WriteFileAtomically(
		path,
		[]byte(buffer.String()),
		0o644,
	)
}

// PublicURLBuildtime resolves one source path to public URL from current map.
func (processor *Processor) PublicURLBuildtime(
	originalPath string,
) (string, bool, error) {
	publicFileMap, loadError := processor.currentPublicFileMap()
	if loadError != nil {
		return "", false, loadError
	}
	resolvedURL, found := publicFileMap.Lookup(
		originalPath,
		processor.cfg.PublicPathPrefix(),
	)
	if !found {
		return "", false, nil
	}
	return resolvedURL, true, nil
}

// MustPublicURLBuildtime resolves public URL or panics when resolution fails.
func (processor *Processor) MustPublicURLBuildtime(originalPath string) string {
	resolvedURL, found, resolutionError := processor.PublicURLBuildtime(
		originalPath,
	)
	if resolutionError != nil {
		panic(resolutionError)
	}
	if !found {
		panic(fmt.Sprintf("public file map lookup miss for %q", originalPath))
	}
	return resolvedURL
}

func (processor *Processor) loadOrBuildFileMap() (wavefilemap.FileMap, error) {
	if processor == nil || processor.cfg == nil {
		return nil, errors.New("static processor config is nil")
	}

	fileMap, loadError := processor.LoadFileMapFromPath(
		processor.cfg.Dist.PublicFileMapGob(),
	)
	if loadError != nil {
		if !processor.cfg.UsingBrowser() {
			if errors.Is(loadError, os.ErrNotExist) {
				return make(wavefilemap.FileMap), nil
			}
			return nil, loadError
		}
		if processError := processor.ProcessPublicFilesOnly(); processError != nil {
			return nil, fmt.Errorf("build files: %w", processError)
		}
		fileMap, loadError = processor.LoadFileMapFromPath(
			processor.cfg.Dist.PublicFileMapGob(),
		)
	}
	return fileMap, loadError
}

// PublicFileMapKeys returns sorted keys for non-prehashed public assets.
func (processor *Processor) PublicFileMapKeys() ([]string, error) {
	fileMap, loadError := processor.loadOrBuildFileMap()
	if loadError != nil {
		return nil, loadError
	}

	keys := make([]string, 0, len(fileMap))
	for key, value := range fileMap {
		if !value.IsPrehashed {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

// SimplePublicFileMap returns non-prehashed public map as path->distName.
func (processor *Processor) SimplePublicFileMap() (map[string]string, error) {
	fileMap, loadError := processor.loadOrBuildFileMap()
	if loadError != nil {
		return nil, loadError
	}

	simpleMap := make(map[string]string, len(fileMap))
	for key, value := range fileMap {
		if !value.IsPrehashed {
			simpleMap[key] = value.DistName
		}
	}
	return simpleMap, nil
}

// LoadPublicFileMap loads the current public file map from disk.
func (processor *Processor) LoadPublicFileMap() (wavefilemap.FileMap, error) {
	if processor == nil || processor.cfg == nil {
		return nil, errors.New("static processor config is nil")
	}
	return processor.LoadFileMapFromPath(processor.cfg.Dist.PublicFileMapGob())
}

// PublicFileMapSnapshot returns cached public file map clone.
func (processor *Processor) PublicFileMapSnapshot() wavefilemap.FileMap {
	processor.mu.Lock()
	defer processor.mu.Unlock()
	return cloneFileMap(processor.cachedPublicFileMap)
}

// PrivateFileMapSnapshot returns cached private file map clone.
func (processor *Processor) PrivateFileMapSnapshot() wavefilemap.FileMap {
	processor.mu.Lock()
	defer processor.mu.Unlock()
	return cloneFileMap(processor.cachedPrivateFileMap)
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

func removeStaticDistArtifactIfPresent(
	distDirectoryPath string,
	distName string,
) {
	_ = os.Remove(filepath.Join(distDirectoryPath, distName))
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

func removeStaticMapEntriesForChangedRelativePaths(
	staticMap wavefilemap.FileMap,
	distDirectoryPath string,
	changedRelativePaths []string,
) bool {
	if len(changedRelativePaths) == 0 || len(staticMap) == 0 {
		return false
	}

	changedRelativePathSet := make(
		map[string]struct{},
		len(changedRelativePaths),
	)
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
		removeStaticDistArtifactIfPresent(
			distDirectoryPath,
			oldValueForPath.DistName,
		)
	}
	return true
}

// RemoveStaticMapEntriesForChangedRelativePaths removes map entries by changed paths.
func RemoveStaticMapEntriesForChangedRelativePaths(
	staticMap wavefilemap.FileMap,
	distDirectoryPath string,
	changedRelativePaths []string,
) bool {
	return removeStaticMapEntriesForChangedRelativePaths(
		staticMap,
		distDirectoryPath,
		changedRelativePaths,
	)
}

// processFullScan rebuilds file map from scratch for one store.
func (processor *Processor) processFullScan(
	store fileMapStore,
	isPublic bool,
) error {
	if processor == nil || processor.cfg == nil {
		return errors.New("static processor config is nil")
	}
	if _, statError := os.Stat(store.sourceRootPath); os.IsNotExist(statError) {
		previousFileMap, _ := processor.LoadFileMapFromPath(store.gobPath)
		if cleanupError := removeStaleDistArtifacts(
			store.outputRootPath,
			previousFileMap,
			wavefilemap.FileMap{},
		); cleanupError != nil {
			return cleanupError
		}
		emptyFileMap := make(wavefilemap.FileMap)
		if saveError := saveFileMap(store.gobPath, emptyFileMap); saveError != nil {
			return saveError
		}
		processor.updateCachedFileMap(isPublic, emptyFileMap)
		if isPublic {
			if writeFileMapError := processor.WriteCanonicalPublicFileMapJSONAndRef(); writeFileMapError != nil {
				return writeFileMapError
			}
		}
		return nil
	}

	nextFileMap, processError := processor.buildFileMapFromSourceRoot(
		store,
		isPublic,
	)
	if processError != nil {
		return processError
	}

	previousFileMap, _ := processor.LoadFileMapFromPath(store.gobPath)
	if cleanupError := removeStaleDistArtifacts(store.outputRootPath, previousFileMap, nextFileMap); cleanupError != nil {
		return cleanupError
	}
	if reflect.DeepEqual(previousFileMap, nextFileMap) {
		processor.updateCachedFileMap(isPublic, nextFileMap)
		processor.log.Debug(
			"processed static files (full scan)",
			waveartifacts.PublicDirname,
			isPublic,
			"entries",
			len(nextFileMap),
		)
		return nil
	}

	if saveError := saveFileMap(store.gobPath, nextFileMap); saveError != nil {
		return saveError
	}
	processor.updateCachedFileMap(isPublic, nextFileMap)

	if isPublic {
		if writeFileMapError := processor.WriteCanonicalPublicFileMapJSONAndRef(); writeFileMapError != nil {
			return writeFileMapError
		}
	}

	processor.log.Debug(
		"processed static files (full scan)",
		waveartifacts.PublicDirname,
		isPublic,
		"entries",
		len(nextFileMap),
	)
	return nil
}

// processChangedPaths updates file map using changed source paths only.
func (processor *Processor) processChangedPaths(
	store fileMapStore,
	isPublic bool,
	changedPaths []string,
) error {
	if processor == nil || processor.cfg == nil {
		return errors.New("static processor config is nil")
	}

	normalizedChangedPaths := normalizeChangedPaths(changedPaths)
	if len(normalizedChangedPaths) == 0 {
		return processor.processFullScan(store, isPublic)
	}

	changedResolutions, fullBuildRequired, resolutionError := resolveStaticChangedPathResolutions(
		store.sourceRootPath,
		normalizedChangedPaths,
	)
	if resolutionError != nil {
		return resolutionError
	}
	if fullBuildRequired {
		return processor.processFullScan(store, isPublic)
	}
	if len(changedResolutions) == 0 {
		return nil
	}

	currentFileMap, loadError := processor.LoadFileMapFromPath(store.gobPath)
	if loadError != nil {
		return processor.processFullScan(store, isPublic)
	}
	if currentFileMap == nil {
		currentFileMap = make(wavefilemap.FileMap)
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
		oldValueForPath, hadOldValueForPath := currentFileMap[resolvedRelativePath]

		if resolution.sourceExists {
			updatedValue, computeError := computeFileMapValue(
				resolution.fileInfo,
				isPublic,
			)
			if computeError != nil {
				if os.IsNotExist(computeError) {
					relativePathsNeedingRemoval = append(
						relativePathsNeedingRemoval,
						resolvedRelativePath,
					)
					continue
				}
				return computeError
			}

			distPath := filepath.Join(
				store.outputRootPath,
				updatedValue.DistName,
			)
			shouldCopyFile := true
			if hadOldValueForPath &&
				oldValueForPath.ContentHash == updatedValue.ContentHash {
				if _, statError := os.Stat(distPath); statError == nil {
					shouldCopyFile = false
				} else if !os.IsNotExist(statError) {
					return statError
				}
			}
			if shouldCopyFile {
				if copyError := copySourceFileToDistPath(
					resolution.fileInfo.srcPath,
					store.outputRootPath,
					updatedValue.DistName,
				); copyError != nil {
					return copyError
				}
			}

			if !hadOldValueForPath || oldValueForPath != updatedValue {
				mapWasChanged = true
			}
			currentFileMap[resolvedRelativePath] = updatedValue

			if hadOldValueForPath &&
				oldValueForPath.DistName != updatedValue.DistName {
				removeStaticDistArtifactIfPresent(
					store.outputRootPath,
					oldValueForPath.DistName,
				)
			}
			continue
		}

		relativePathsNeedingRemoval = append(
			relativePathsNeedingRemoval,
			resolvedRelativePath,
		)
	}

	if removeStaticMapEntriesForChangedRelativePaths(
		currentFileMap,
		store.outputRootPath,
		relativePathsNeedingRemoval,
	) {
		mapWasChanged = true
	}

	if !mapWasChanged {
		return nil
	}

	if saveError := saveFileMap(store.gobPath, currentFileMap); saveError != nil {
		return saveError
	}
	processor.updateCachedFileMap(isPublic, currentFileMap)

	if isPublic {
		if writeFileMapError := processor.WriteCanonicalPublicFileMapJSONAndRef(); writeFileMapError != nil {
			return writeFileMapError
		}
	}

	processor.log.Debug(
		"processed static files (changed paths)",
		waveartifacts.PublicDirname,
		isPublic,
		"changed_paths",
		len(normalizedChangedPaths),
		"entries",
		len(currentFileMap),
	)
	return nil
}

// buildFileMapFromSourceRoot scans source root and emits fresh file map.
func (processor *Processor) buildFileMapFromSourceRoot(
	store fileMapStore,
	hashOutput bool,
) (wavefilemap.FileMap, error) {
	fileMap := make(wavefilemap.FileMap)
	if strings.TrimSpace(store.sourceRootPath) == "" {
		return fileMap, nil
	}

	discoveredFileInfosByRelativePath := make(map[string]fileInfo)

	walkError := filepath.WalkDir(
		store.sourceRootPath,
		func(path string, directoryEntry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if directoryEntry.IsDir() {
				return nil
			}

			relativePath, relativeError := filepath.Rel(
				store.sourceRootPath,
				path,
			)
			if relativeError != nil {
				return relativeError
			}
			normalizedRelativePath := normalizeMapKeyPath(relativePath)
			if normalizedRelativePath == "" {
				return nil
			}
			staticFileInfo, shouldProcessFile := buildStaticFileInfoFromRelativePath(
				path,
				normalizedRelativePath,
			)
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

			fileValue, computeError := computeFileMapValue(
				staticFileInfo,
				hashOutput,
			)
			if computeError != nil {
				return computeError
			}
			if copyError := copySourceFileToDistPath(path, store.outputRootPath, fileValue.DistName); copyError != nil {
				return copyError
			}
			fileMap[staticFileInfo.relPath] = fileValue
			return nil
		},
	)
	if walkError != nil {
		return nil, walkError
	}

	return fileMap, nil
}

// currentPublicFileMap loads or returns cached public file map.
func (processor *Processor) currentPublicFileMap() (wavefilemap.FileMap, error) {
	processor.mu.Lock()
	defer processor.mu.Unlock()

	if processor.cachedPublicFileMap != nil {
		return cloneFileMap(processor.cachedPublicFileMap), nil
	}

	publicFileMap, loadError := processor.LoadFileMapFromPath(
		processor.publicStore.gobPath,
	)
	if loadError != nil {
		if !processor.cfg.UsingBrowser() &&
			errors.Is(loadError, os.ErrNotExist) {
			processor.cachedPublicFileMap = make(wavefilemap.FileMap)
			return cloneFileMap(processor.cachedPublicFileMap), nil
		}
		return nil, loadError
	}
	processor.cachedPublicFileMap = cloneFileMap(publicFileMap)
	return cloneFileMap(publicFileMap), nil
}

// updateCachedFileMap updates in-memory file map cache.
func (processor *Processor) updateCachedFileMap(
	isPublic bool,
	nextFileMap wavefilemap.FileMap,
) {
	processor.mu.Lock()
	defer processor.mu.Unlock()
	if isPublic {
		processor.cachedPublicFileMap = cloneFileMap(nextFileMap)
		return
	}
	processor.cachedPrivateFileMap = cloneFileMap(nextFileMap)
}

// computeFileMapValue computes file map value for one source file.
func computeFileMapValue(
	staticFileInfo fileInfo,
	hashOutput bool,
) (wavefilemap.FileVal, error) {
	underscorePath := strings.ReplaceAll(staticFileInfo.relPath, "/", "_")
	contentHash, hashError := fileops.HashFile(
		staticFileInfo.srcPath,
		underscorePath,
	)
	if hashError != nil {
		return wavefilemap.FileVal{}, hashError
	}

	distName := deriveStaticDistName(staticFileInfo, hashOutput, contentHash)
	return wavefilemap.FileVal{
		DistName:    distName,
		ContentHash: contentHash,
		IsPrehashed: staticFileInfo.prehash,
	}, nil
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

// removeStaleDistArtifacts removes old dist files no longer referenced by next file map.
func removeStaleDistArtifacts(
	distRootPath string,
	previousFileMap wavefilemap.FileMap,
	nextFileMap wavefilemap.FileMap,
) error {
	nextDistNames := make(map[string]struct{}, len(nextFileMap))
	for _, value := range nextFileMap {
		nextDistNames[value.DistName] = struct{}{}
	}

	if len(previousFileMap) > 0 {
		for _, value := range previousFileMap {
			if _, keep := nextDistNames[value.DistName]; keep {
				continue
			}
			distPath := filepath.Join(
				distRootPath,
				filepath.FromSlash(value.DistName),
			)
			if removeError := os.Remove(distPath); removeError != nil &&
				!errors.Is(removeError, os.ErrNotExist) {
				return removeError
			}
		}
	}

	return nil
}

// copySourceFileToDistPath copies source file into resolved dist relative path.
func copySourceFileToDistPath(
	sourcePath string,
	distRootPath string,
	relativeDistPath string,
) error {
	destinationPath := filepath.Join(
		distRootPath,
		filepath.FromSlash(relativeDistPath),
	)
	if ensureDirectoryError := wavefs.EnsureDirectoryForFile(destinationPath); ensureDirectoryError != nil {
		return ensureDirectoryError
	}
	return artifactio.CopyFileAtomically(sourcePath, destinationPath)
}

// saveFileMap writes map gob atomically.
func saveFileMap(path string, fileMap wavefilemap.FileMap) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("file map path is empty")
	}
	if ensureDirectoryError := wavefs.EnsureDirectoryForFile(path); ensureDirectoryError != nil {
		return ensureDirectoryError
	}
	var serializedFileMap bytes.Buffer
	encoder := gob.NewEncoder(&serializedFileMap)
	if encodeError := encoder.Encode(fileMap); encodeError != nil {
		return encodeError
	}
	return artifactio.WriteFileAtomically(path, serializedFileMap.Bytes(), 0o644)
}

// serializePublicFileMapForFramework serializes file map into stable canonical
// JSON bytes.
func serializePublicFileMapForFramework(
	publicFileMap wavefilemap.FileMap,
) ([]byte, error) {
	sortedKeys := make([]string, 0, len(publicFileMap))
	for key := range publicFileMap {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	resolved := make(map[string]wavefilemap.FileVal, len(publicFileMap))
	for _, key := range sortedKeys {
		resolved[key] = publicFileMap[key]
	}

	jsonBuilder := &strings.Builder{}
	jsonBuilder.WriteString("{")
	for index, key := range sortedKeys {
		value := resolved[key]
		if index > 0 {
			jsonBuilder.WriteString(",")
		}
		jsonBuilder.WriteString(
			fmt.Sprintf(
				"\n  %q: {\"dist\":%q,\"hash\":%q,\"prehashed\":%t}",
				key,
				value.DistName,
				value.ContentHash,
				value.IsPrehashed,
			),
		)
	}
	if len(sortedKeys) > 0 {
		jsonBuilder.WriteString("\n")
	}
	jsonBuilder.WriteString("}")
	return []byte(jsonBuilder.String()), nil
}

func resolveStaticRelativePathFromSourcePath(
	sourceDirectoryPath string,
	sourcePath string,
) (string, bool, error) {
	relativePath, relativePathError := filepath.Rel(
		sourceDirectoryPath,
		sourcePath,
	)
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

	prehashedPrefix := waveartifacts.PrehashedDirname + "/"
	nohashPrefix := waveartifacts.NohashDirname + "/"
	if strings.HasPrefix(normalizedRelativePath, prehashedPrefix) {
		prehash = true
		normalizedRelativePath = strings.TrimPrefix(
			normalizedRelativePath,
			prehashedPrefix,
		)
	} else if strings.HasPrefix(normalizedRelativePath, nohashPrefix) {
		prehash = true
		normalizedRelativePath = strings.TrimPrefix(
			normalizedRelativePath,
			nohashPrefix,
		)
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

	staticFileInfo, shouldProcessFile := buildStaticFileInfoFromRelativePath(
		sourcePath,
		relativePath,
	)
	return staticFileInfo, shouldProcessFile, nil
}

func staticSourceFileExists(sourcePath string) (bool, error) {
	sourceInfo, sourceStatError := os.Stat(sourcePath)
	if sourceStatError != nil {
		if os.IsNotExist(sourceStatError) {
			return false, nil
		}
		return false, fmt.Errorf(
			"stat changed file %s: %w",
			sourcePath,
			sourceStatError,
		)
	}
	return !sourceInfo.IsDir(), nil
}

func staticDirectoryContainsProcessableStaticFile(
	sourceDirectoryPath string,
	directoryPath string,
) (bool, error) {
	directoryInfo, directoryStatError := os.Stat(directoryPath)
	if directoryStatError != nil {
		if os.IsNotExist(directoryStatError) {
			return false, nil
		}
		return false, fmt.Errorf(
			"stat changed directory %s: %w",
			directoryPath,
			directoryStatError,
		)
	}
	if !directoryInfo.IsDir() {
		return false, nil
	}

	stopWalkAfterFirstProcessableFile := errors.New(
		"stop_walk_after_first_processable_static_file",
	)
	hasProcessableStaticFile := false

	walkError := filepath.WalkDir(
		directoryPath,
		func(
			candidatePath string,
			directoryEntry fs.DirEntry,
			directoryWalkError error,
		) error {
			if directoryWalkError != nil {
				return directoryWalkError
			}
			if directoryEntry.IsDir() {
				return nil
			}

			_, shouldProcessFile, resolveError := resolveStaticFileInfoFromSourcePath(
				sourceDirectoryPath,
				candidatePath,
			)
			if resolveError != nil {
				return resolveError
			}
			if !shouldProcessFile {
				return nil
			}

			hasProcessableStaticFile = true
			return stopWalkAfterFirstProcessableFile
		},
	)
	if walkError != nil &&
		!errors.Is(walkError, stopWalkAfterFirstProcessableFile) {
		return false, fmt.Errorf(
			"walk changed directory %s: %w",
			directoryPath,
			walkError,
		)
	}
	return hasProcessableStaticFile, nil
}

func ensureNoStaticLogicalPathCollision(
	existingFileInfo fileInfo,
	candidateFileInfo fileInfo,
) error {
	existingPath := waveenv.Absolute(existingFileInfo.srcPath)
	candidatePath := waveenv.Absolute(candidateFileInfo.srcPath)
	if waveenv.PathsReferToSameLocation(existingPath, candidatePath) {
		return nil
	}
	orderedPaths := []string{existingPath, candidatePath}
	sort.Strings(orderedPaths)
	return fmt.Errorf(
		"static source path collision for logical path %q: multiple source files map to the same output: %s; keep exactly one source file",
		existingFileInfo.relPath,
		strings.Join(orderedPaths, ", "),
	)
}

func ensureNoStaticLogicalPathCollisionWithinSourceDirectory(
	sourceDirectoryPath string,
	relativePath string,
) error {
	relativePathWithNativeSeparators := filepath.FromSlash(relativePath)
	candidateFileInfos := []fileInfo{
		{
			srcPath: filepath.Join(
				sourceDirectoryPath,
				relativePathWithNativeSeparators,
			),
			relPath: relativePath,
			prehash: false,
		},
		{
			srcPath: filepath.Join(
				sourceDirectoryPath,
				waveartifacts.PrehashedDirname,
				relativePathWithNativeSeparators,
			),
			relPath: relativePath,
			prehash: true,
		},
		{
			srcPath: filepath.Join(
				sourceDirectoryPath,
				waveartifacts.NohashDirname,
				relativePathWithNativeSeparators,
			),
			relPath: relativePath,
			prehash: true,
		},
	}

	existingSourcePaths := make([]string, 0, len(candidateFileInfos))
	for _, candidateFileInfo := range candidateFileInfos {
		sourceExists, sourceStatError := staticSourceFileExists(
			candidateFileInfo.srcPath,
		)
		if sourceStatError != nil {
			return sourceStatError
		}
		if sourceExists {
			existingSourcePaths = append(
				existingSourcePaths,
				waveenv.Absolute(candidateFileInfo.srcPath),
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
		convertedChangedResolutions[relativePath] = convertStaticChangedPathResolutionFromInternal(
			resolution,
		)
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
			return convertStaticFileInfoToInternal(
				staticFileInfo,
			), shouldProcessFile, nil
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
			normalizeChangedSourcePath: waveenv.CanonicalizePathForLocationComparison,
			resolveStaticFileInfoFromSourcePath: func(
				sourceDirectoryPath string,
				sourcePath string,
			) (fileInfo, bool, error) {
				return resolveStaticFileInfoFromSourcePath(
					sourceDirectoryPath,
					sourcePath,
				)
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
	return convertStaticChangedPathResolutionMapFromInternal(
		changedResolutions,
	), fullBuildRequired, nil
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
		normalizeChangedSourcePath = waveenv.CanonicalizePathForLocationComparison
	}

	normalizedSourceDirectoryPath := normalizeChangedSourcePath(
		sourceDirectoryPath,
	)
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
		normalizedChangedSourcePath := normalizeChangedSourcePath(
			changedSourcePath,
		)
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
			sourceExistsForPath, sourceStatError := sourceFileExistsProbe(
				normalizedChangedSourcePath,
			)
			if sourceStatError != nil {
				return nil, false, sourceStatError
			}
			sourceExists = sourceExistsForPath
			sourceExistsByNormalizedChangedSourcePath[normalizedChangedSourcePath] = sourceExists
		}
		if !sourceExists {
			directoryContainsProcessableStaticFile, directoryScanError := staticDirectoryContainsProcessableStaticFile(
				sourceDirectoryPath,
				normalizedChangedSourcePath,
			)
			if directoryScanError != nil {
				return nil, false, directoryScanError
			}
			if directoryContainsProcessableStaticFile {
				return nil, true, nil
			}
		}

		if sourceExists {
			relativePath := resolutionProbeResult.staticFileInfo.relPath
			if _, alreadyChecked := collisionCheckedByRelativePath[relativePath]; !alreadyChecked {
				collisionError := collisionProbe(
					sourceDirectoryPath,
					relativePath,
				)
				if collisionError != nil {
					return nil, false, collisionError
				}
				collisionCheckedByRelativePath[relativePath] = struct{}{}
			}
		}

		relativePath := resolutionProbeResult.staticFileInfo.relPath
		existingResolution, alreadyResolved := changedResolutions[relativePath]
		if !alreadyResolved || sourceExists ||
			!existingResolution.sourceExists {
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
		normalizeExportedStaticChangedPathResolutionProbeFunctions(
			probeFunctions,
		),
	)
	if resolutionError != nil {
		return nil, false, resolutionError
	}
	return convertStaticChangedPathResolutionMapFromInternal(
		changedResolutions,
	), fullBuildRequired, nil
}

// normalizeChangedPaths normalizes and deduplicates changed path list.
func normalizeChangedPaths(changedPaths []string) []string {
	seen := make(map[string]struct{}, len(changedPaths))
	normalizedPaths := make([]string, 0, len(changedPaths))
	for _, changedPath := range changedPaths {
		if strings.TrimSpace(changedPath) == "" {
			continue
		}
		normalizedPath := filepath.Clean(changedPath)
		if _, alreadySeen := seen[normalizedPath]; alreadySeen {
			continue
		}
		seen[normalizedPath] = struct{}{}
		normalizedPaths = append(normalizedPaths, normalizedPath)
	}
	return normalizedPaths
}

// normalizeMapKeyPath normalizes map key paths to slash-separated relative style.
func normalizeMapKeyPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	normalizedPath := filepath.Clean(path)
	normalizedPath = strings.ReplaceAll(normalizedPath, "\\", "/")
	normalizedPath = strings.TrimPrefix(normalizedPath, "./")
	if normalizedPath == "." {
		return ""
	}
	return normalizedPath
}

// cloneFileMap clones file map values.
func cloneFileMap(fileMap wavefilemap.FileMap) wavefilemap.FileMap {
	if len(fileMap) == 0 {
		return make(wavefilemap.FileMap)
	}
	cloned := make(wavefilemap.FileMap, len(fileMap))
	for key, value := range fileMap {
		cloned[key] = value
	}
	return cloned
}

// builderWriter adapts strings.Builder to io.Writer for gob encoding.
type builderWriter struct {
	builder *strings.Builder
}

// Write implements io.Writer.
func (writer builderWriter) Write(payload []byte) (int, error) {
	if writer.builder == nil {
		return 0, errors.New("builder writer has nil builder")
	}
	writtenCount, writeError := writer.builder.Write(payload)
	if writeError != nil {
		return writtenCount, writeError
	}
	return writtenCount, nil
}
