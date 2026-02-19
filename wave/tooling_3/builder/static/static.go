package static

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling_3/toolingshared"
)

const (
	hashedOutputDirectoryName = "vorma_out"
	publicFileMapSymbolName   = "WAVE_PUBLIC_FILE_MAP"
)

var prehashedFilenamePattern = regexp.MustCompile(
	`(?i)\.[a-f0-9]{8,64}\.[^./]+$`,
)

// fileMapStore defines source/destination roots and metadata paths for one file map.
type fileMapStore struct {
	sourceRootPath string
	outputRootPath string
	gobPath        string
}

// Processor owns static artifact processing and file map persistence.
type Processor struct {
	cfg *wave.ParsedConfig
	log *slog.Logger

	mu sync.Mutex

	publicStore  fileMapStore
	privateStore fileMapStore

	cachedPublicFileMap  wave.FileMap
	cachedPrivateFileMap wave.FileMap
}

// NewProcessor creates static processor for one parsed config.
func NewProcessor(cfg *wave.ParsedConfig, log *slog.Logger) *Processor {
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

// WriteFrameworkPublicFileMapTS writes framework-facing JS export and ref metadata.
func (processor *Processor) WriteFrameworkPublicFileMapTS() error {
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
	fileName := fmt.Sprintf("vorma_internal_public_filemap_%s.js", hashPrefix)
	outputPath := filepath.Join(processor.cfg.Dist.StaticPublic(), fileName)
	refPath := processor.cfg.Dist.PublicFileMapRef()

	jsContent := buildPublicFileMapJSImport(serializedFileMap)
	if writeError := toolingshared.WriteFileAtomically(outputPath, []byte(jsContent), 0o644); writeError != nil {
		return writeError
	}
	if writeRefError := toolingshared.WriteFileAtomically(refPath, []byte(fileName), 0o644); writeRefError != nil {
		return writeRefError
	}

	if frameworkDirectory := strings.TrimSpace(processor.cfg.FrameworkPublicFileMapOutDir); frameworkDirectory != "" {
		targetPath := filepath.Join(frameworkDirectory, fileName)
		if copyError := toolingshared.CopyFileAtomically(outputPath, targetPath); copyError != nil {
			return copyError
		}
	}

	processor.log.Info(
		"wrote framework public filemap",
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
) (wave.FileMap, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("file map path is empty")
	}

	file, openError := os.Open(path)
	if openError != nil {
		return nil, openError
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)
	fileMap := make(wave.FileMap)
	if decodeError := decoder.Decode(&fileMap); decodeError != nil {
		return nil, decodeError
	}
	return fileMap, nil
}

// SaveFileMap writes gob-encoded file map atomically.
func (processor *Processor) SaveFileMap(
	fileMap wave.FileMap,
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
	return toolingshared.WriteFileAtomically(
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

// PublicFileMapSnapshot returns cached public file map clone.
func (processor *Processor) PublicFileMapSnapshot() wave.FileMap {
	processor.mu.Lock()
	defer processor.mu.Unlock()
	return cloneFileMap(processor.cachedPublicFileMap)
}

// PrivateFileMapSnapshot returns cached private file map clone.
func (processor *Processor) PrivateFileMapSnapshot() wave.FileMap {
	processor.mu.Lock()
	defer processor.mu.Unlock()
	return cloneFileMap(processor.cachedPrivateFileMap)
}

// processFullScan rebuilds file map from scratch for one store.
func (processor *Processor) processFullScan(
	store fileMapStore,
	isPublic bool,
) error {
	if processor == nil || processor.cfg == nil {
		return errors.New("static processor config is nil")
	}

	nextFileMap, processError := processor.buildFileMapFromSourceRoot(store)
	if processError != nil {
		return processError
	}

	previousFileMap, _ := processor.LoadFileMapFromPath(store.gobPath)
	if cleanupError := removeStaleDistArtifacts(store.outputRootPath, previousFileMap, nextFileMap); cleanupError != nil {
		return cleanupError
	}

	if saveError := saveFileMap(store.gobPath, nextFileMap); saveError != nil {
		return saveError
	}
	processor.updateCachedFileMap(isPublic, nextFileMap)

	if isPublic {
		if writeFileMapError := processor.WriteFrameworkPublicFileMapTS(); writeFileMapError != nil {
			return writeFileMapError
		}
	}

	processor.log.Info(
		"processed static files (full scan)",
		"public",
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

	currentFileMap, loadError := processor.LoadFileMapFromPath(store.gobPath)
	if loadError != nil {
		if !errors.Is(loadError, os.ErrNotExist) {
			return loadError
		}
		currentFileMap = make(wave.FileMap)
	}

	for _, changedPath := range normalizedChangedPaths {
		relativeSourcePath, withinStore := normalizeSourcePathForStore(
			store.sourceRootPath,
			changedPath,
		)
		if !withinStore {
			continue
		}

		sourcePath := filepath.Join(
			store.sourceRootPath,
			filepath.FromSlash(relativeSourcePath),
		)
		sourceInfo, statError := os.Stat(sourcePath)
		if statError != nil {
			if errors.Is(statError, os.ErrNotExist) {
				if existingVal, found := currentFileMap[relativeSourcePath]; found {
					_ = os.Remove(
						filepath.Join(
							store.outputRootPath,
							existingVal.DistName,
						),
					)
					delete(currentFileMap, relativeSourcePath)
				}
				continue
			}
			return statError
		}
		if sourceInfo.IsDir() {
			continue
		}

		nextValue, computeError := computeFileMapValue(
			sourcePath,
			relativeSourcePath,
		)
		if computeError != nil {
			return computeError
		}

		if existingVal, found := currentFileMap[relativeSourcePath]; found {
			if existingVal.DistName != nextValue.DistName {
				_ = os.Remove(
					filepath.Join(store.outputRootPath, existingVal.DistName),
				)
			}
		}

		if copyError := copySourceFileToDistPath(sourcePath, store.outputRootPath, nextValue.DistName); copyError != nil {
			return copyError
		}
		currentFileMap[relativeSourcePath] = nextValue
	}

	if saveError := saveFileMap(store.gobPath, currentFileMap); saveError != nil {
		return saveError
	}
	processor.updateCachedFileMap(isPublic, currentFileMap)

	if isPublic {
		if writeFileMapError := processor.WriteFrameworkPublicFileMapTS(); writeFileMapError != nil {
			return writeFileMapError
		}
	}

	processor.log.Info(
		"processed static files (changed paths)",
		"public",
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
) (wave.FileMap, error) {
	fileMap := make(wave.FileMap)
	if strings.TrimSpace(store.sourceRootPath) == "" {
		return fileMap, nil
	}

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

			fileValue, computeError := computeFileMapValue(
				path,
				normalizedRelativePath,
			)
			if computeError != nil {
				return computeError
			}
			if copyError := copySourceFileToDistPath(path, store.outputRootPath, fileValue.DistName); copyError != nil {
				return copyError
			}
			fileMap[normalizedRelativePath] = fileValue
			return nil
		},
	)
	if walkError != nil {
		return nil, walkError
	}

	return fileMap, nil
}

// currentPublicFileMap loads or returns cached public file map.
func (processor *Processor) currentPublicFileMap() (wave.FileMap, error) {
	processor.mu.Lock()
	defer processor.mu.Unlock()

	if len(processor.cachedPublicFileMap) > 0 {
		return cloneFileMap(processor.cachedPublicFileMap), nil
	}

	publicFileMap, loadError := processor.LoadFileMapFromPath(
		processor.publicStore.gobPath,
	)
	if loadError != nil {
		return nil, loadError
	}
	processor.cachedPublicFileMap = cloneFileMap(publicFileMap)
	return cloneFileMap(publicFileMap), nil
}

// updateCachedFileMap updates in-memory file map cache.
func (processor *Processor) updateCachedFileMap(
	isPublic bool,
	nextFileMap wave.FileMap,
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
	sourcePath string,
	relativeSourcePath string,
) (wave.FileVal, error) {
	contentHash, hashError := computeFileContentHash(sourcePath)
	if hashError != nil {
		return wave.FileVal{}, hashError
	}

	baseName := filepath.Base(relativeSourcePath)
	directory := filepath.Dir(relativeSourcePath)
	isPrehashed := prehashedFilenamePattern.MatchString(
		strings.ToLower(baseName),
	)

	distName := ""
	if isPrehashed {
		distName = filepath.Join(hashedOutputDirectoryName, directory, baseName)
	} else {
		extension := filepath.Ext(baseName)
		baseWithoutExtension := strings.TrimSuffix(baseName, extension)
		distName = filepath.Join(
			hashedOutputDirectoryName,
			directory,
			fmt.Sprintf("%s.%s%s", baseWithoutExtension, contentHash[:16], extension),
		)
	}
	return wave.FileVal{
		DistName:    normalizeMapKeyPath(distName),
		ContentHash: contentHash,
		IsPrehashed: isPrehashed,
	}, nil
}

// computeFileContentHash computes SHA-256 content hash for a file.
func computeFileContentHash(path string) (string, error) {
	file, openError := os.Open(path)
	if openError != nil {
		return "", openError
	}
	defer file.Close()

	hasher := sha256.New()
	if _, copyError := io.Copy(hasher, file); copyError != nil {
		return "", copyError
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// removeStaleDistArtifacts removes old dist files no longer referenced by next file map.
func removeStaleDistArtifacts(
	distRootPath string,
	previousFileMap wave.FileMap,
	nextFileMap wave.FileMap,
) error {
	if len(previousFileMap) == 0 {
		return nil
	}

	nextDistNames := make(map[string]struct{}, len(nextFileMap))
	for _, value := range nextFileMap {
		nextDistNames[value.DistName] = struct{}{}
	}

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
	if ensureDirectoryError := toolingshared.EnsureDirectoryForFile(destinationPath); ensureDirectoryError != nil {
		return ensureDirectoryError
	}
	return toolingshared.CopyFileAtomically(sourcePath, destinationPath)
}

// saveFileMap writes map gob atomically.
func saveFileMap(path string, fileMap wave.FileMap) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("file map path is empty")
	}
	if ensureDirectoryError := toolingshared.EnsureDirectoryForFile(path); ensureDirectoryError != nil {
		return ensureDirectoryError
	}

	tempPath := path + ".tmp"
	file, createError := os.Create(tempPath)
	if createError != nil {
		return createError
	}

	encoder := gob.NewEncoder(file)
	encodeError := encoder.Encode(fileMap)
	closeError := file.Close()
	if encodeError != nil {
		_ = os.Remove(tempPath)
		return encodeError
	}
	if closeError != nil {
		_ = os.Remove(tempPath)
		return closeError
	}
	if renameError := os.Rename(tempPath, path); renameError != nil {
		_ = os.Remove(tempPath)
		return renameError
	}
	return nil
}

// serializePublicFileMapForFramework serializes file map into stable JSON bytes.
func serializePublicFileMapForFramework(
	publicFileMap wave.FileMap,
) ([]byte, error) {
	sortedKeys := make([]string, 0, len(publicFileMap))
	for key := range publicFileMap {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	resolved := make(map[string]wave.FileVal, len(publicFileMap))
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

// buildPublicFileMapJSImport builds JS module source for framework runtime.
func buildPublicFileMapJSImport(serializedFileMap []byte) string {
	return strings.TrimSpace(fmt.Sprintf(`export const %s = %s;
export default %s;
`, publicFileMapSymbolName, string(serializedFileMap), publicFileMapSymbolName)) + "\n"
}

// normalizeSourcePathForStore returns normalized relative path when path is inside source root.
func normalizeSourcePathForStore(
	sourceRootPath string,
	absoluteCandidatePath string,
) (string, bool) {
	if strings.TrimSpace(sourceRootPath) == "" ||
		strings.TrimSpace(absoluteCandidatePath) == "" {
		return "", false
	}

	absoluteRoot, rootError := filepath.Abs(sourceRootPath)
	absoluteCandidate, candidateError := filepath.Abs(absoluteCandidatePath)
	if rootError != nil || candidateError != nil {
		return "", false
	}

	relativePath, relativeError := filepath.Rel(absoluteRoot, absoluteCandidate)
	if relativeError != nil {
		return "", false
	}
	normalizedRelativePath := normalizeMapKeyPath(relativePath)
	if normalizedRelativePath == "" {
		return "", false
	}
	if strings.HasPrefix(normalizedRelativePath, "../") ||
		normalizedRelativePath == ".." {
		return "", false
	}
	return normalizedRelativePath, true
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
func cloneFileMap(fileMap wave.FileMap) wave.FileMap {
	if len(fileMap) == 0 {
		return make(wave.FileMap)
	}
	cloned := make(wave.FileMap, len(fileMap))
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
