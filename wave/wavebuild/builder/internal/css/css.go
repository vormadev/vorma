package css

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/shared"
	"github.com/vormadev/vorma/wave/internal/wavecore"
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
func (processor *Processor) SetTrackedCriticalCSSImportPaths(
	importPaths []string,
) {
	normalizedImportPaths := make(map[string]struct{}, len(importPaths))
	for _, importPath := range importPaths {
		normalizedImportPath := normalizeCSSFilePathForImportTracking(
			importPath,
		)
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
func (processor *Processor) SetTrackedNormalCSSImportPaths(
	importPaths []string,
) {
	normalizedImportPaths := make(map[string]struct{}, len(importPaths))
	for _, importPath := range importPaths {
		normalizedImportPath := normalizeCSSFilePathForImportTracking(
			importPath,
		)
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
			return "", errors.New(
				"critical css fresh build output is unavailable",
			)
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
			return "", errors.New(
				"normal css fresh build output is unavailable",
			)
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

	processor.mu.Lock()
	processor.criticalFreshOutputAvailable = false
	processor.mu.Unlock()
	entrySourceBytes, readEntrySourceError := os.ReadFile(entryPath)
	if readEntrySourceError != nil {
		return fmt.Errorf(
			"read critical css %q: %w",
			entryPath,
			readEntrySourceError,
		)
	}
	buildOutput, buildError := buildSingleCSSEntry(
		entryPath,
		string(entrySourceBytes),
		processor.resolvePublicURL,
	)
	if buildError != nil {
		return fmt.Errorf("build critical css %q: %w", entryPath, buildError)
	}
	trackedCriticalImports := resolveTrackedCSSImportPaths(
		entryPath,
		buildOutput.InputPaths,
	)

	currentOutputBytes, readCurrentOutputError := os.ReadFile(outputPath)
	if readCurrentOutputError == nil &&
		string(currentOutputBytes) == buildOutput.CSS {
		processor.mu.Lock()
		processor.cachedCriticalCSS = buildOutput.CSS
		processor.criticalFreshOutputAvailable = true
		processor.criticalImports = trackedCriticalImports
		processor.mu.Unlock()
		return nil
	}
	if readCurrentOutputError != nil &&
		!errors.Is(readCurrentOutputError, os.ErrNotExist) {
		return readCurrentOutputError
	}

	if writeError := shared.WriteFileAtomically(
		outputPath,
		[]byte(buildOutput.CSS),
		0o644,
	); writeError != nil {
		return writeError
	}

	processor.mu.Lock()
	processor.cachedCriticalCSS = buildOutput.CSS
	processor.criticalFreshOutputAvailable = true
	processor.criticalImports = trackedCriticalImports
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

	processor.mu.Lock()
	processor.normalFreshOutputAvailable = false
	processor.mu.Unlock()
	entrySourceBytes, readEntrySourceError := os.ReadFile(entryPath)
	if readEntrySourceError != nil {
		return fmt.Errorf(
			"read normal css %q: %w",
			entryPath,
			readEntrySourceError,
		)
	}
	buildOutput, buildError := buildSingleCSSEntry(
		entryPath,
		string(entrySourceBytes),
		processor.resolvePublicURL,
	)
	if buildError != nil {
		return fmt.Errorf("build normal css %q: %w", entryPath, buildError)
	}
	trackedNormalImports := resolveTrackedCSSImportPaths(
		entryPath,
		buildOutput.InputPaths,
	)

	hashSum := sha256.Sum256([]byte(buildOutput.CSS))
	hashPrefix := hex.EncodeToString(hashSum[:])[:16]
	fileName := fmt.Sprintf("vorma_internal_normal_%s.css", hashPrefix)
	outputFilePath := filepath.Join(outputDirectoryPath, fileName)

	existingRefPathBytes, existingRefReadError := os.ReadFile(refPath)
	existingRefPath := ""
	if existingRefReadError == nil {
		existingRefPath = strings.TrimSpace(string(existingRefPathBytes))
	}

	if existingRefPath == fileName {
		existingOutputBytes, readExistingOutputError := os.ReadFile(
			outputFilePath,
		)
		if readExistingOutputError == nil &&
			string(existingOutputBytes) == buildOutput.CSS {
			normalURL := joinPublicURL(
				processor.cfg.PublicPathPrefix(),
				fileName,
			)
			processor.mu.Lock()
			processor.cachedNormalURL = normalURL
			processor.normalFreshOutputAvailable = true
			processor.normalImports = trackedNormalImports
			processor.mu.Unlock()
			return nil
		}
	}

	if existingRefPath != "" && existingRefPath != fileName {
		_ = os.Remove(filepath.Join(outputDirectoryPath, existingRefPath))
	}

	if writeError := shared.WriteFileAtomically(
		outputFilePath,
		[]byte(buildOutput.CSS),
		0o644,
	); writeError != nil {
		return writeError
	}
	if writeRefError := shared.WriteFileAtomically(refPath, []byte(fileName), 0o644); writeRefError != nil {
		return writeRefError
	}

	normalURL := joinPublicURL(processor.cfg.PublicPathPrefix(), fileName)
	processor.mu.Lock()
	processor.cachedNormalURL = normalURL
	processor.normalFreshOutputAvailable = true
	processor.normalImports = trackedNormalImports
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
	resolvePublicURL func(originalPath string) (string, bool, error),
) (
	singleCSSEntryBuildOutput,
	error,
) {
	buildPlugins := []api.Plugin{
		buildPublicCSSURLResolverPlugin(resolvePublicURL),
	}
	buildResult := api.Build(api.BuildOptions{
		Bundle:            true,
		Write:             false,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		Metafile:          true,
		LogLevel:          api.LogLevelSilent,
		Plugins:           buildPlugins,
		External: []string{
			"*.avif",
			"*.eot",
			"*.gif",
			"*.ico",
			"*.jpeg",
			"*.jpg",
			"*.otf",
			"*.png",
			"*.svg",
			"*.ttf",
			"*.webp",
			"*.woff",
			"*.woff2",
		},
		Stdin: &api.StdinOptions{
			Contents:   entrySource,
			ResolveDir: filepath.Dir(entryPath),
			Sourcefile: filepath.Base(entryPath),
			Loader:     api.LoaderCSS,
		},
	})
	if len(buildResult.Errors) > 0 {
		return singleCSSEntryBuildOutput{}, errors.New(
			buildResult.Errors[0].Text,
		)
	}
	if len(buildResult.OutputFiles) == 0 {
		return singleCSSEntryBuildOutput{}, errors.New(
			"esbuild produced no CSS output files",
		)
	}

	inputPaths, inputPathParseError := parseCSSInputPathsFromBuildResultMetafile(
		buildResult.Metafile,
	)
	if inputPathParseError != nil {
		return singleCSSEntryBuildOutput{}, inputPathParseError
	}

	return singleCSSEntryBuildOutput{
		CSS:        string(buildResult.OutputFiles[0].Contents),
		InputPaths: inputPaths,
	}, nil
}

func buildPublicCSSURLResolverPlugin(
	resolvePublicURL func(originalPath string) (string, bool, error),
) api.Plugin {
	return api.Plugin{
		Name: "wave_css_public_url_resolver",
		Setup: func(build api.PluginBuild) {
			build.OnResolve(
				api.OnResolveOptions{
					Filter:    ".*",
					Namespace: "file",
				},
				func(args api.OnResolveArgs) (api.OnResolveResult, error) {
					if args.Kind != api.ResolveCSSURLToken {
						return api.OnResolveResult{}, nil
					}
					if resolvePublicURL == nil {
						return api.OnResolveResult{}, nil
					}

					cssPath := strings.TrimSpace(args.Path)
					if shouldSkipPublicURLResolution(cssPath) {
						return api.OnResolveResult{
							Path:     cssPath,
							External: true,
						}, nil
					}

					cssLookupPath, cssPathSuffix := splitCSSPathTokenForLookup(
						cssPath,
					)
					if strings.TrimSpace(cssLookupPath) == "" {
						cssLookupPath = cssPath
					}
					resolvedPublicURL, found, resolveError := resolvePublicURL(
						cssLookupPath,
					)
					if resolveError != nil {
						return api.OnResolveResult{}, fmt.Errorf(
							"resolve css url token %q: %w",
							cssPath,
							resolveError,
						)
					}
					if !found {
						return api.OnResolveResult{}, fmt.Errorf(
							"resolve css url token %q: no hashed public asset found",
							cssPath,
						)
					}

					return api.OnResolveResult{
						Path:     resolvedPublicURL + cssPathSuffix,
						External: true,
					}, nil
				},
			)
		},
	}
}

type singleCSSEntryBuildOutput struct {
	CSS        string
	InputPaths []string
}

type cssBuildResultMetafile struct {
	Inputs map[string]struct{} `json:"inputs"`
}

func parseCSSInputPathsFromBuildResultMetafile(
	metafileJSON string,
) ([]string, error) {
	var metafile cssBuildResultMetafile
	if unmarshalError := json.Unmarshal(
		[]byte(metafileJSON),
		&metafile,
	); unmarshalError != nil {
		return nil, fmt.Errorf("parse css build metafile: %w", unmarshalError)
	}

	inputPaths := make([]string, 0, len(metafile.Inputs))
	for inputPath := range metafile.Inputs {
		inputPaths = append(inputPaths, inputPath)
	}
	sort.Strings(inputPaths)
	return inputPaths, nil
}

func resolveTrackedCSSImportPaths(
	entryPath string,
	buildInputPaths []string,
) map[string]struct{} {
	trackedImportPaths := make(map[string]struct{}, len(buildInputPaths)+1)
	entryDirectoryPath := filepath.Dir(entryPath)
	normalizedEntryPath := normalizeCSSFilePathForImportTracking(entryPath)
	if normalizedEntryPath != "" {
		trackedImportPaths[normalizedEntryPath] = struct{}{}
	}

	for _, buildInputPath := range buildInputPaths {
		trimmedBuildInputPath := strings.TrimSpace(buildInputPath)
		if trimmedBuildInputPath == "" {
			continue
		}
		if strings.HasPrefix(trimmedBuildInputPath, "<") &&
			strings.HasSuffix(trimmedBuildInputPath, ">") {
			continue
		}

		normalizedImportPath := resolveBuildInputPathForTracking(
			entryDirectoryPath,
			trimmedBuildInputPath,
		)
		if normalizedImportPath == "" {
			continue
		}
		trackedImportPaths[normalizedImportPath] = struct{}{}
	}

	return trackedImportPaths
}

func resolveBuildInputPathForTracking(
	entryDirectoryPath string,
	buildInputPath string,
) string {
	trimmedBuildInputPath := strings.TrimSpace(buildInputPath)
	if trimmedBuildInputPath == "" {
		return ""
	}

	candidatePaths := make([]string, 0, 3)
	if filepath.IsAbs(trimmedBuildInputPath) {
		candidatePaths = append(candidatePaths, trimmedBuildInputPath)
	} else {
		candidatePaths = append(candidatePaths, trimmedBuildInputPath)
		candidatePaths = append(
			candidatePaths,
			filepath.Join(entryDirectoryPath, trimmedBuildInputPath),
		)
		candidatePaths = append(
			candidatePaths,
			string(filepath.Separator)+trimmedBuildInputPath,
		)
	}
	candidatePaths = deduplicateCSSCandidatePathsForTracking(candidatePaths)

	for _, candidatePath := range candidatePaths {
		if _, statError := os.Stat(candidatePath); statError != nil {
			continue
		}
		normalizedCandidatePath := normalizeCSSFilePathForImportTracking(
			candidatePath,
		)
		if normalizedCandidatePath != "" {
			return normalizedCandidatePath
		}
	}

	if len(candidatePaths) == 0 {
		return normalizeCSSFilePathForImportTracking(trimmedBuildInputPath)
	}
	for _, candidatePath := range candidatePaths {
		normalizedCandidatePath := normalizeCSSFilePathForImportTracking(
			candidatePath,
		)
		if normalizedCandidatePath != "" {
			return normalizedCandidatePath
		}
	}
	return normalizeCSSFilePathForImportTracking(trimmedBuildInputPath)
}

func deduplicateCSSCandidatePathsForTracking(
	candidatePaths []string,
) []string {
	if len(candidatePaths) == 0 {
		return nil
	}

	seenCandidatePaths := make(map[string]struct{}, len(candidatePaths))
	deduplicatedCandidatePaths := make([]string, 0, len(candidatePaths))
	for _, candidatePath := range candidatePaths {
		normalizedCandidatePath := strings.TrimSpace(candidatePath)
		if normalizedCandidatePath == "" {
			continue
		}
		if _, alreadySeen := seenCandidatePaths[normalizedCandidatePath]; alreadySeen {
			continue
		}
		seenCandidatePaths[normalizedCandidatePath] = struct{}{}
		deduplicatedCandidatePaths = append(
			deduplicatedCandidatePaths,
			normalizedCandidatePath,
		)
	}
	return deduplicatedCandidatePaths
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
	return wavecore.PathsReferToSameLocation(leftPath, rightPath)
}

func normalizeCSSFilePathForImportTracking(filePath string) string {
	trimmedFilePath := strings.TrimSpace(filePath)
	if trimmedFilePath == "" {
		return ""
	}

	canonicalPath := wavecore.CanonicalizePathForLocationComparison(
		trimmedFilePath,
	)
	if canonicalPath != "" {
		return filepath.Clean(canonicalPath)
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
	if strings.HasPrefix(trimmedPath, "?") {
		return true
	}
	if strings.HasPrefix(trimmedPath, "#") {
		return true
	}

	schemeSeparatorIndex := strings.Index(trimmedPath, ":")
	if schemeSeparatorIndex > 0 &&
		isValidURIPathScheme(trimmedPath[:schemeSeparatorIndex]) {
		return true
	}

	return false
}

func splitCSSPathTokenForLookup(
	cssPath string,
) (lookupPath string, suffix string) {
	trimmedPath := strings.TrimSpace(cssPath)
	if trimmedPath == "" {
		return "", ""
	}

	queryIndex := strings.Index(trimmedPath, "?")
	fragmentIndex := strings.Index(trimmedPath, "#")
	suffixStartIndex := -1
	if queryIndex >= 0 && fragmentIndex >= 0 {
		if queryIndex < fragmentIndex {
			suffixStartIndex = queryIndex
		} else {
			suffixStartIndex = fragmentIndex
		}
	} else if queryIndex >= 0 {
		suffixStartIndex = queryIndex
	} else if fragmentIndex >= 0 {
		suffixStartIndex = fragmentIndex
	}

	if suffixStartIndex <= 0 {
		return trimmedPath, ""
	}
	return trimmedPath[:suffixStartIndex], trimmedPath[suffixStartIndex:]
}

func isValidURIPathScheme(scheme string) bool {
	if scheme == "" {
		return false
	}
	for index, character := range scheme {
		if index == 0 {
			if (character < 'a' || character > 'z') &&
				(character < 'A' || character > 'Z') {
				return false
			}
			continue
		}
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '+' ||
			character == '-' ||
			character == '.' {
			continue
		}
		return false
	}
	return true
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
