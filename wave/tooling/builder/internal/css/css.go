package css

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/internal/shared"
)

// Processor builds and resolves CSS outputs for Wave build and dev workflows.
type Processor struct {
	cfg *wave.ParsedConfig
	log *slog.Logger

	mu sync.RWMutex

	cachedCriticalCSS string
	cachedNormalURL   string

	criticalFreshOutputAvailable bool
	normalFreshOutputAvailable   bool

	criticalImports map[string]struct{}
	normalImports   map[string]struct{}

	resolvePublicURL func(originalPath string) (string, bool, error)
}

// BuildOptions selects which CSS pipelines to execute.
type BuildOptions struct {
	BuildCriticalCSS bool
	BuildNormalCSS   bool
}

// NewProcessor creates a CSS processor for one config.
func NewProcessor(
	cfg *wave.ParsedConfig,
	log *slog.Logger,
	resolvePublicURL func(originalPath string) (string, bool, error),
) *Processor {
	if log == nil {
		log = slog.Default()
	}
	return &Processor{
		cfg:              cfg,
		log:              log,
		criticalImports:  make(map[string]struct{}),
		normalImports:    make(map[string]struct{}),
		resolvePublicURL: resolvePublicURL,
	}
}

// Build executes requested CSS pipelines and updates in-memory cache.
func (processor *Processor) Build(options BuildOptions) error {
	if processor == nil || processor.cfg == nil {
		return errors.New("css processor config is nil")
	}

	if options.BuildCriticalCSS {
		if criticalBuildError := processor.buildCriticalCSS(); criticalBuildError != nil {
			return criticalBuildError
		}
	}
	if options.BuildNormalCSS {
		if normalBuildError := processor.buildNormalCSS(); normalBuildError != nil {
			return normalBuildError
		}
	}
	return nil
}

// IsCriticalCSSFile reports whether path is configured critical CSS entry.
func (processor *Processor) IsCriticalCSSFile(path string) bool {
	if processor == nil {
		return false
	}
	normalizedPath := normalizeCSSFilePathForImportTracking(path)
	if normalizedPath == "" {
		return false
	}

	processor.mu.RLock()
	_, trackedCriticalImport := processor.criticalImports[normalizedPath]
	processor.mu.RUnlock()
	if trackedCriticalImport {
		return true
	}

	if processor.cfg == nil {
		return false
	}
	configuredCriticalCSSEntryPath := processor.cfg.CriticalCSSEntry()
	if configuredCriticalCSSEntryPath == "" {
		return false
	}
	return samePath(path, configuredCriticalCSSEntryPath)
}

// IsNormalCSSFile reports whether path is configured non-critical CSS entry.
func (processor *Processor) IsNormalCSSFile(path string) bool {
	if processor == nil {
		return false
	}
	normalizedPath := normalizeCSSFilePathForImportTracking(path)
	if normalizedPath == "" {
		return false
	}

	processor.mu.RLock()
	_, trackedNormalImport := processor.normalImports[normalizedPath]
	processor.mu.RUnlock()
	if trackedNormalImport {
		return true
	}

	if processor.cfg == nil {
		return false
	}
	configuredNormalCSSEntryPath := processor.cfg.NonCriticalCSSEntry()
	if configuredNormalCSSEntryPath == "" {
		return false
	}
	return samePath(path, configuredNormalCSSEntryPath)
}

// IsCSSFile reports whether path is critical or non-critical CSS input.
func (processor *Processor) IsCSSFile(path string) bool {
	return processor.IsCriticalCSSFile(path) || processor.IsNormalCSSFile(path)
}

// SetTrackedCriticalCSSImportPaths replaces tracked critical CSS import paths.
func (processor *Processor) SetTrackedCriticalCSSImportPaths(importPaths []string) {
	normalizedImportPaths := make(map[string]struct{}, len(importPaths))
	for _, importPath := range importPaths {
		normalizedImportPath := normalizeCSSFilePathForImportTracking(importPath)
		if normalizedImportPath == "" {
			continue
		}
		normalizedImportPaths[normalizedImportPath] = struct{}{}
	}

	processor.mu.Lock()
	processor.criticalImports = normalizedImportPaths
	processor.mu.Unlock()
}

// SetTrackedNormalCSSImportPaths replaces tracked normal CSS import paths.
func (processor *Processor) SetTrackedNormalCSSImportPaths(importPaths []string) {
	normalizedImportPaths := make(map[string]struct{}, len(importPaths))
	for _, importPath := range importPaths {
		normalizedImportPath := normalizeCSSFilePathForImportTracking(importPath)
		if normalizedImportPath == "" {
			continue
		}
		normalizedImportPaths[normalizedImportPath] = struct{}{}
	}

	processor.mu.Lock()
	processor.normalImports = normalizedImportPaths
	processor.mu.Unlock()
}

// CriticalCSS returns cached critical CSS text when available.
func (processor *Processor) CriticalCSS() (string, bool) {
	processor.mu.RLock()
	defer processor.mu.RUnlock()
	if strings.TrimSpace(processor.cachedCriticalCSS) == "" {
		return "", false
	}
	return processor.cachedCriticalCSS, true
}

// NormalCSSURL returns cached normal CSS public URL when available.
func (processor *Processor) NormalCSSURL() (string, bool) {
	processor.mu.RLock()
	defer processor.mu.RUnlock()
	if strings.TrimSpace(processor.cachedNormalURL) == "" {
		return "", false
	}
	return processor.cachedNormalURL, true
}

// CountTrackedCriticalCSSImportPaths returns tracked critical CSS import count.
func (processor *Processor) CountTrackedCriticalCSSImportPaths() int {
	if processor == nil {
		return 0
	}
	processor.mu.RLock()
	defer processor.mu.RUnlock()
	return len(processor.criticalImports)
}

// ListTrackedCriticalCSSImportPaths returns a sorted copy of tracked critical imports.
func (processor *Processor) ListTrackedCriticalCSSImportPaths() []string {
	if processor == nil {
		return nil
	}
	processor.mu.RLock()
	defer processor.mu.RUnlock()

	importPaths := make([]string, 0, len(processor.criticalImports))
	for importPath := range processor.criticalImports {
		importPaths = append(importPaths, importPath)
	}
	sort.Strings(importPaths)
	return importPaths
}

// ReadCriticalCSSHotReloadOutput reads cached critical CSS or falls back to dist.
func (processor *Processor) ReadCriticalCSSHotReloadOutput(
	requireFreshBuildOutput bool,
) (string, error) {
	if processor == nil || processor.cfg == nil {
		return "", errors.New("css processor config is nil")
	}

	processor.mu.RLock()
	cachedCriticalCSS := processor.cachedCriticalCSS
	freshOutputAvailable := processor.criticalFreshOutputAvailable
	processor.mu.RUnlock()

	if requireFreshBuildOutput {
		if !freshOutputAvailable {
			return "", errors.New("critical css fresh build output is unavailable")
		}
		return cachedCriticalCSS, nil
	}

	if strings.TrimSpace(cachedCriticalCSS) != "" {
		return cachedCriticalCSS, nil
	}

	criticalCSSBytes, readCriticalCSSError := os.ReadFile(
		processor.cfg.Dist.CriticalCSS(),
	)
	if readCriticalCSSError != nil {
		return "", readCriticalCSSError
	}
	readCriticalCSS := string(criticalCSSBytes)
	processor.mu.Lock()
	processor.cachedCriticalCSS = readCriticalCSS
	processor.mu.Unlock()
	return readCriticalCSS, nil
}

// ReadNormalCSSHotReloadURL reads cached normal CSS URL or falls back to ref file.
func (processor *Processor) ReadNormalCSSHotReloadURL(
	requireFreshBuildOutput bool,
) (string, error) {
	if processor == nil || processor.cfg == nil {
		return "", errors.New("css processor config is nil")
	}

	processor.mu.RLock()
	cachedNormalURL := processor.cachedNormalURL
	freshOutputAvailable := processor.normalFreshOutputAvailable
	processor.mu.RUnlock()

	if requireFreshBuildOutput {
		if !freshOutputAvailable {
			return "", errors.New("normal css fresh build output is unavailable")
		}
		return cachedNormalURL, nil
	}

	if strings.TrimSpace(cachedNormalURL) != "" {
		return cachedNormalURL, nil
	}

	normalRefBytes, readNormalRefError := os.ReadFile(
		processor.cfg.Dist.NormalCSSRef(),
	)
	if readNormalRefError != nil {
		return "", readNormalRefError
	}

	normalizedRefPath := normalizeNormalCSSRefPath(string(normalRefBytes))
	if normalizedRefPath == "" {
		processor.mu.Lock()
		processor.cachedNormalURL = ""
		processor.mu.Unlock()
		return "", nil
	}

	normalCSSURL := joinPublicURL(
		processor.cfg.PublicPathPrefix(),
		normalizedRefPath,
	)
	processor.mu.Lock()
	processor.cachedNormalURL = normalCSSURL
	processor.mu.Unlock()
	return normalCSSURL, nil
}

// ReloadCachedOutputs loads CSS cache values from output files.
func (processor *Processor) ReloadCachedOutputs() {
	if processor == nil || processor.cfg == nil {
		return
	}

	if criticalCSSBytes, criticalReadError := os.ReadFile(processor.cfg.Dist.CriticalCSS()); criticalReadError == nil {
		processor.mu.Lock()
		processor.cachedCriticalCSS = string(criticalCSSBytes)
		processor.mu.Unlock()
	}

	normalRefBytes, normalRefReadError := os.ReadFile(
		processor.cfg.Dist.NormalCSSRef(),
	)
	if normalRefReadError == nil {
		normalRef := strings.TrimSpace(string(normalRefBytes))
		if normalRef != "" {
			normalURL := joinPublicURL(
				processor.cfg.PublicPathPrefix(),
				normalRef,
			)
			processor.mu.Lock()
			processor.cachedNormalURL = normalURL
			processor.mu.Unlock()
		}
	}
}

// buildCriticalCSS builds the critical CSS pipeline and writes output file.
func (processor *Processor) buildCriticalCSS() error {
	entryPath := processor.cfg.CriticalCSSEntry()
	outputPath := processor.cfg.Dist.CriticalCSS()
	if entryPath == "" {
		processor.mu.Lock()
		processor.cachedCriticalCSS = ""
		processor.criticalFreshOutputAvailable = true
		processor.criticalImports = map[string]struct{}{}
		processor.mu.Unlock()
		_ = os.Remove(outputPath)
		return nil
	}
	normalizedEntryPath := normalizeCSSFilePathForImportTracking(entryPath)

	processor.mu.Lock()
	processor.criticalFreshOutputAvailable = false
	processor.mu.Unlock()
	entrySourceBytes, readEntrySourceError := os.ReadFile(entryPath)
	if readEntrySourceError != nil {
		return fmt.Errorf("read critical css %q: %w", entryPath, readEntrySourceError)
	}
	preparedEntrySource := processor.resolvePublicURLTokensInCSS(
		string(entrySourceBytes),
	)

	buildOutput, buildError := buildSingleCSSEntry(
		entryPath,
		preparedEntrySource,
	)
	if buildError != nil {
		return fmt.Errorf("build critical css %q: %w", entryPath, buildError)
	}

	currentOutputBytes, readCurrentOutputError := os.ReadFile(outputPath)
	if readCurrentOutputError == nil &&
		string(currentOutputBytes) == buildOutput {
		processor.mu.Lock()
		processor.cachedCriticalCSS = buildOutput
		processor.criticalFreshOutputAvailable = true
		processor.criticalImports = map[string]struct{}{
			normalizedEntryPath: {},
		}
		processor.mu.Unlock()
		return nil
	}
	if readCurrentOutputError != nil && !errors.Is(readCurrentOutputError, os.ErrNotExist) {
		return readCurrentOutputError
	}

	if writeError := shared.WriteFileAtomically(outputPath, []byte(buildOutput), 0o644); writeError != nil {
		return writeError
	}

	processor.mu.Lock()
	processor.cachedCriticalCSS = buildOutput
	processor.criticalFreshOutputAvailable = true
	processor.criticalImports = map[string]struct{}{
		normalizedEntryPath: {},
	}
	processor.mu.Unlock()
	processor.log.Info(
		"built critical css",
		"entry",
		entryPath,
		"out",
		outputPath,
	)
	return nil
}

// buildNormalCSS builds non-critical CSS output and writes ref metadata.
func (processor *Processor) buildNormalCSS() error {
	entryPath := processor.cfg.NonCriticalCSSEntry()
	refPath := processor.cfg.Dist.NormalCSSRef()
	outputDirectoryPath := processor.cfg.Dist.StaticPublic()
	if entryPath == "" {
		processor.mu.Lock()
		processor.cachedNormalURL = ""
		processor.normalFreshOutputAvailable = true
		processor.normalImports = map[string]struct{}{}
		processor.mu.Unlock()
		_ = os.Remove(refPath)
		return nil
	}
	normalizedEntryPath := normalizeCSSFilePathForImportTracking(entryPath)

	processor.mu.Lock()
	processor.normalFreshOutputAvailable = false
	processor.mu.Unlock()
	entrySourceBytes, readEntrySourceError := os.ReadFile(entryPath)
	if readEntrySourceError != nil {
		return fmt.Errorf("read normal css %q: %w", entryPath, readEntrySourceError)
	}
	preparedEntrySource := processor.resolvePublicURLTokensInCSS(
		string(entrySourceBytes),
	)

	buildOutput, buildError := buildSingleCSSEntry(
		entryPath,
		preparedEntrySource,
	)
	if buildError != nil {
		return fmt.Errorf("build normal css %q: %w", entryPath, buildError)
	}

	hashSum := sha256.Sum256([]byte(buildOutput))
	hashPrefix := hex.EncodeToString(hashSum[:])[:16]
	fileName := fmt.Sprintf("vorma_internal_normal_%s.css", hashPrefix)
	outputFilePath := filepath.Join(outputDirectoryPath, fileName)

	existingRefPathBytes, existingRefReadError := os.ReadFile(refPath)
	existingRefPath := ""
	if existingRefReadError == nil {
		existingRefPath = strings.TrimSpace(string(existingRefPathBytes))
	}

	if existingRefPath == fileName {
		existingOutputBytes, readExistingOutputError := os.ReadFile(outputFilePath)
		if readExistingOutputError == nil &&
			string(existingOutputBytes) == buildOutput {
			normalURL := joinPublicURL(processor.cfg.PublicPathPrefix(), fileName)
			processor.mu.Lock()
			processor.cachedNormalURL = normalURL
			processor.normalFreshOutputAvailable = true
			processor.normalImports = map[string]struct{}{
				normalizedEntryPath: {},
			}
			processor.mu.Unlock()
			return nil
		}
	}

	if existingRefPath != "" && existingRefPath != fileName {
		_ = os.Remove(filepath.Join(outputDirectoryPath, existingRefPath))
	}

	if writeError := shared.WriteFileAtomically(outputFilePath, []byte(buildOutput), 0o644); writeError != nil {
		return writeError
	}
	if writeRefError := shared.WriteFileAtomically(refPath, []byte(fileName), 0o644); writeRefError != nil {
		return writeRefError
	}

	normalURL := joinPublicURL(processor.cfg.PublicPathPrefix(), fileName)
	processor.mu.Lock()
	processor.cachedNormalURL = normalURL
	processor.normalFreshOutputAvailable = true
	processor.normalImports = map[string]struct{}{
		normalizedEntryPath: {},
	}
	processor.mu.Unlock()
	processor.log.Info(
		"built normal css",
		"entry",
		entryPath,
		"out",
		outputFilePath,
		"url",
		normalURL,
	)
	return nil
}

// BuildCriticalCSSOnly builds only critical CSS pipeline.
func (processor *Processor) BuildCriticalCSSOnly() error {
	return processor.Build(BuildOptions{BuildCriticalCSS: true})
}

// BuildNormalCSSOnly builds only non-critical CSS pipeline.
func (processor *Processor) BuildNormalCSSOnly() error {
	return processor.Build(BuildOptions{BuildNormalCSS: true})
}

// buildSingleCSSEntry executes CSS-only esbuild pipeline and returns output bytes as string.
func buildSingleCSSEntry(
	entryPath string,
	entrySource string,
) (string, error) {
	buildResult := api.Build(api.BuildOptions{
		Bundle:            false,
		Write:             false,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		LogLevel:          api.LogLevelSilent,
		Stdin: &api.StdinOptions{
			Contents:   entrySource,
			ResolveDir: filepath.Dir(entryPath),
			Sourcefile: filepath.Base(entryPath),
			Loader:     api.LoaderCSS,
		},
	})
	if len(buildResult.Errors) > 0 {
		return "", errors.New(buildResult.Errors[0].Text)
	}
	if len(buildResult.OutputFiles) == 0 {
		return "", errors.New("esbuild produced no CSS output files")
	}
	return string(buildResult.OutputFiles[0].Contents), nil
}

// joinPublicURL joins public path prefix and relative asset path.
func joinPublicURL(publicPathPrefix string, relativePath string) string {
	resolvedPrefix := strings.TrimSpace(publicPathPrefix)
	if resolvedPrefix == "" {
		resolvedPrefix = "/"
	}
	if !strings.HasPrefix(resolvedPrefix, "/") {
		resolvedPrefix = "/" + resolvedPrefix
	}
	if resolvedPrefix != "/" && !strings.HasSuffix(resolvedPrefix, "/") {
		resolvedPrefix += "/"
	}

	normalizedRelativePath := strings.TrimPrefix(
		strings.ReplaceAll(relativePath, "\\", "/"),
		"/",
	)
	if resolvedPrefix == "/" {
		return "/" + normalizedRelativePath
	}
	return resolvedPrefix + normalizedRelativePath
}

// samePath reports whether two paths resolve to same cleaned absolute path.
func samePath(leftPath string, rightPath string) bool {
	leftAbsolutePath, leftError := filepath.Abs(leftPath)
	rightAbsolutePath, rightError := filepath.Abs(rightPath)
	if leftError != nil || rightError != nil {
		return filepath.Clean(leftPath) == filepath.Clean(rightPath)
	}
	return filepath.Clean(leftAbsolutePath) == filepath.Clean(rightAbsolutePath)
}

func normalizeCSSFilePathForImportTracking(filePath string) string {
	trimmedFilePath := strings.TrimSpace(filePath)
	if trimmedFilePath == "" {
		return ""
	}

	absoluteFilePath, absolutePathError := filepath.Abs(trimmedFilePath)
	if absolutePathError != nil {
		absoluteFilePath = filepath.Clean(trimmedFilePath)
	}

	resolvedFilePath, resolveError := filepath.EvalSymlinks(absoluteFilePath)
	if resolveError == nil && resolvedFilePath != "" {
		return filepath.Clean(resolvedFilePath)
	}

	return filepath.Clean(absoluteFilePath)
}

var cssURLTokenPattern = regexp.MustCompile(
	`url\(\s*(['"]?)([^'")]+)['"]?\s*\)`,
)

func (processor *Processor) resolvePublicURLTokensInCSS(cssContent string) string {
	if processor == nil || processor.resolvePublicURL == nil {
		return cssContent
	}

	return cssURLTokenPattern.ReplaceAllStringFunc(
		cssContent,
		func(matchedURLToken string) string {
			matchedFields := cssURLTokenPattern.FindStringSubmatch(
				matchedURLToken,
			)
			if len(matchedFields) != 3 {
				return matchedURLToken
			}

			quote := matchedFields[1]
			originalPath := strings.TrimSpace(matchedFields[2])
			if shouldSkipPublicURLResolution(originalPath) {
				return matchedURLToken
			}

			resolvedPublicURL, found, resolveError := processor.resolvePublicURL(
				originalPath,
			)
			if resolveError != nil || !found {
				return matchedURLToken
			}
			return fmt.Sprintf(
				"url(%s%s%s)",
				quote,
				resolvedPublicURL,
				quote,
			)
		},
	)
}

func shouldSkipPublicURLResolution(cssPath string) bool {
	trimmedPath := strings.TrimSpace(cssPath)
	if trimmedPath == "" {
		return true
	}
	if strings.HasPrefix(trimmedPath, "//") {
		return true
	}
	if strings.HasPrefix(trimmedPath, "/") {
		return true
	}
	if strings.HasPrefix(trimmedPath, "http://") {
		return true
	}
	if strings.HasPrefix(trimmedPath, "https://") {
		return true
	}
	if strings.HasPrefix(trimmedPath, "data:") {
		return true
	}
	if strings.HasPrefix(trimmedPath, "#") {
		return true
	}
	return false
}

func normalizeNormalCSSRefPath(normalCSSRef string) string {
	trimmedRef := strings.TrimSpace(normalCSSRef)
	if trimmedRef == "" {
		return ""
	}

	normalizedRef := filepath.Clean(strings.TrimSpace(trimmedRef))
	normalizedRef = strings.ReplaceAll(normalizedRef, "\\", "/")
	for strings.HasPrefix(normalizedRef, "../") {
		normalizedRef = strings.TrimPrefix(normalizedRef, "../")
	}
	normalizedRef = strings.TrimPrefix(normalizedRef, "./")
	normalizedRef = strings.TrimPrefix(normalizedRef, "/")
	if normalizedRef == "." {
		return ""
	}
	return normalizedRef
}

// ValidateCSSConfig validates CSS entry path configuration semantics.
func ValidateCSSConfig(cfg *wave.ParsedConfig) error {
	if cfg == nil || cfg.Core == nil {
		return errors.New("config or core config is nil")
	}

	criticalEntry := cfg.CriticalCSSEntry()
	normalEntry := cfg.NonCriticalCSSEntry()
	if strings.TrimSpace(criticalEntry) == "" &&
		strings.TrimSpace(normalEntry) == "" {
		return nil
	}

	if strings.TrimSpace(criticalEntry) != "" {
		if !strings.HasSuffix(strings.ToLower(criticalEntry), ".css") {
			return fmt.Errorf(
				"critical css entry must end with .css: %q",
				criticalEntry,
			)
		}
	}
	if strings.TrimSpace(normalEntry) != "" {
		if !strings.HasSuffix(strings.ToLower(normalEntry), ".css") {
			return fmt.Errorf(
				"non-critical css entry must end with .css: %q",
				normalEntry,
			)
		}
	}

	if criticalEntry != "" && normalEntry != "" &&
		samePath(criticalEntry, normalEntry) {
		return errors.New(
			"critical and non-critical css entries must not point to same file",
		)
	}

	return nil
}
