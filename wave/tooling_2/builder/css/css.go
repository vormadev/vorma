package css

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/lab/esbuildutil"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/waveshared"
	"github.com/vormadev/vorma/wave/tooling_2/builder/static"
)

type Processor struct {
	cfg                           *wave.ParsedConfig
	log                           *slog.Logger
	resolvePublicURLBuildtimePath func(string) string

	mu                                             sync.RWMutex
	criticalCtx                                    esbuild.BuildContext
	normalCtx                                      esbuild.BuildContext
	hasCriticalCtx                                 bool
	hasNormalCtx                                   bool
	criticalEntry                                  string
	normalEntry                                    string
	criticalCtxIsDev                               bool
	normalCtxIsDev                                 bool
	criticalImports                                map[string]struct{}
	normalImports                                  map[string]struct{}
	cachedCriticalCSSHotReloadOutput               string
	hasCachedCriticalCSSHotReloadOutput            bool
	criticalCSSHotReloadOutputInvalidatedByRebuild bool
	cachedNormalCSSHotReloadURL                    string
	hasCachedNormalCSSHotReloadURL                 bool
	normalCSSHotReloadOutputInvalidatedByRebuild   bool
}

type cssBuildNature string

const (
	cssBuildNatureCritical cssBuildNature = "critical"
	cssBuildNatureNormal   cssBuildNature = "normal"
)

// NewProcessor constructs a CSS processor for one parsed config.
func NewProcessor(
	cfg *wave.ParsedConfig,
	log *slog.Logger,
	resolvePublicURLBuildtimePath func(string) string,
) *Processor {
	if log == nil {
		log = colorlog.New("wave")
	}

	return &Processor{
		cfg:                           cfg,
		log:                           log,
		resolvePublicURLBuildtimePath: resolvePublicURLBuildtimePath,
		criticalImports:               make(map[string]struct{}),
		normalImports:                 make(map[string]struct{}),
	}
}

func (p *Processor) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.resetBuildNatureStateLocked(cssBuildNatureCritical)
	p.resetBuildNatureStateLocked(cssBuildNatureNormal)
	return nil
}

func (p *Processor) clearBuildNatureState(buildNature cssBuildNature) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resetBuildNatureStateLocked(buildNature)
}

func (p *Processor) resetBuildNatureStateLocked(buildNature cssBuildNature) {
	if buildNature == cssBuildNatureCritical {
		if p.hasCriticalCtx {
			p.criticalCtx.Dispose()
		}
		p.hasCriticalCtx = false
		p.criticalEntry = ""
		p.criticalCtxIsDev = false
		p.criticalImports = make(map[string]struct{})
		p.cachedCriticalCSSHotReloadOutput = ""
		p.hasCachedCriticalCSSHotReloadOutput = false
		p.criticalCSSHotReloadOutputInvalidatedByRebuild = false
		return
	}

	if p.hasNormalCtx {
		p.normalCtx.Dispose()
	}
	p.hasNormalCtx = false
	p.normalEntry = ""
	p.normalCtxIsDev = false
	p.normalImports = make(map[string]struct{})
	p.cachedNormalCSSHotReloadURL = ""
	p.hasCachedNormalCSSHotReloadURL = false
	p.normalCSSHotReloadOutputInvalidatedByRebuild = false
}

func (p *Processor) getBuildNatureContextState(
	buildNature cssBuildNature,
) (esbuild.BuildContext, bool, string, bool) {
	if buildNature == cssBuildNatureCritical {
		return p.criticalCtx, p.hasCriticalCtx, p.criticalEntry, p.criticalCtxIsDev
	}
	return p.normalCtx, p.hasNormalCtx, p.normalEntry, p.normalCtxIsDev
}

func (p *Processor) setBuildNatureContextState(
	buildNature cssBuildNature,
	context esbuild.BuildContext,
	entryPoint string,
	isDev bool,
) {
	if buildNature == cssBuildNatureCritical {
		p.criticalCtx = context
		p.hasCriticalCtx = true
		p.criticalEntry = entryPoint
		p.criticalCtxIsDev = isDev
		return
	}

	p.normalCtx = context
	p.hasNormalCtx = true
	p.normalEntry = entryPoint
	p.normalCtxIsDev = isDev
}

func (p *Processor) getOrCreateContextForBuildNature(
	buildNature cssBuildNature,
	entryPoint string,
	isDev bool,
) (esbuild.BuildContext, error) {
	p.mu.RLock()
	existingContext, hasExistingContext, existingEntryPoint, existingContextIsDev := p.getBuildNatureContextState(
		buildNature,
	)
	p.mu.RUnlock()
	if hasExistingContext && existingEntryPoint == entryPoint && existingContextIsDev == isDev {
		return existingContext, nil
	}

	newContext, contextError := p.createContext(entryPoint, isDev)
	if contextError != nil {
		return nil, contextError
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	existingContext, hasExistingContext, existingEntryPoint, existingContextIsDev = p.getBuildNatureContextState(
		buildNature,
	)
	if hasExistingContext && existingEntryPoint == entryPoint && existingContextIsDev == isDev {
		newContext.Dispose()
		return existingContext, nil
	}

	if hasExistingContext {
		existingContext.Dispose()
	}

	p.setBuildNatureContextState(buildNature, newContext, entryPoint, isDev)
	return newContext, nil
}

func (p *Processor) createContext(
	entryPoint string,
	isDev bool,
) (esbuild.BuildContext, error) {
	context, contextError := esbuild.Context(esbuild.BuildOptions{
		EntryPoints:       []string{entryPoint},
		Bundle:            true,
		MinifyWhitespace:  !isDev,
		MinifyIdentifiers: !isDev,
		MinifySyntax:      !isDev,
		Write:             false,
		Metafile:          true,
		Plugins:           []esbuild.Plugin{p.urlResolverPlugin()},
	})
	if contextError != nil {
		return nil, fmt.Errorf("esbuild context: %v", contextError.Errors)
	}

	return context, nil
}

func (p *Processor) entryPointForBuildNature(buildNature cssBuildNature) string {
	if buildNature == cssBuildNatureCritical {
		return p.cfg.CriticalCSSEntry()
	}
	return p.cfg.NonCriticalCSSEntry()
}

func (p *Processor) setBuildNatureImports(
	buildNature cssBuildNature,
	cssInputPaths []string,
) {
	importsForBuildNature := buildCSSImportPathSetFromInputPaths(cssInputPaths)

	p.mu.Lock()
	defer p.mu.Unlock()
	if buildNature == cssBuildNatureCritical {
		p.criticalImports = importsForBuildNature
		return
	}
	p.normalImports = importsForBuildNature
}

func buildCSSImportPathSetFromInputPaths(
	cssInputPaths []string,
) map[string]struct{} {
	importsForBuildNature := make(map[string]struct{}, len(cssInputPaths))
	for _, filePath := range cssInputPaths {
		normalizedFilePath := normalizeCSSFilePathForImportTracking(filePath)
		if normalizedFilePath == "" {
			continue
		}
		importsForBuildNature[normalizedFilePath] = struct{}{}
	}
	return importsForBuildNature
}

func (p *Processor) BuildAll(isDev bool) error {
	if err := p.BuildCritical(isDev); err != nil {
		return fmt.Errorf("critical CSS: %w", err)
	}
	if err := p.BuildNormal(isDev); err != nil {
		return fmt.Errorf("normal CSS: %w", err)
	}
	return nil
}

// BuildCritical builds critical CSS output.
func (p *Processor) BuildCritical(isDev bool) error {
	return p.build(cssBuildNatureCritical, isDev)
}

// BuildNormal builds normal CSS output.
func (p *Processor) BuildNormal(isDev bool) error {
	return p.build(cssBuildNatureNormal, isDev)
}

func (p *Processor) build(buildNature cssBuildNature, isDev bool) error {
	p.invalidateHotReloadOutputForBuildNature(buildNature)

	entryPoint := p.entryPointForBuildNature(buildNature)
	if entryPoint == "" {
		p.clearBuildNatureState(buildNature)
		return nil
	}

	ctx, err := p.getOrCreateContextForBuildNature(
		buildNature,
		entryPoint,
		isDev,
	)
	if err != nil {
		return err
	}

	result := ctx.Rebuild()
	if err := esbuildutil.CollectErrors(result); err != nil {
		return err
	}

	if len(result.OutputFiles) == 0 {
		return fmt.Errorf(
			"esbuild produced no output files for %s CSS",
			buildNature,
		)
	}

	cssInputPaths, parseMetafileError := parseCSSInputPathsFromBuildResultMetafile(
		result,
	)
	if parseMetafileError != nil {
		return parseMetafileError
	}
	p.setBuildNatureImports(buildNature, cssInputPaths)

	if buildNature == cssBuildNatureCritical {
		if writeError := p.writeCriticalCSSOutput(result.OutputFiles[0].Contents); writeError != nil {
			return writeError
		}
		p.setCachedCriticalCSSHotReloadOutput(
			string(result.OutputFiles[0].Contents),
		)
		return nil
	}

	normalCSSOutputFileName, writeError := p.writeNormalCSSOutput(
		result.OutputFiles[0].Contents,
	)
	if writeError != nil {
		return writeError
	}
	p.setCachedNormalCSSHotReloadURL(waveshared.ResolveFromReferencedPath(
		p.cfg.PublicPathPrefix(),
		normalCSSOutputFileName,
	))
	return nil
}

func parseCSSInputPathsFromBuildResultMetafile(
	result esbuild.BuildResult,
) ([]string, error) {
	var metafile esbuildutil.ESBuildMetafileSubset
	if err := json.Unmarshal([]byte(result.Metafile), &metafile); err != nil {
		return nil, fmt.Errorf("parse metafile: %w", err)
	}

	cssInputPaths := make([]string, 0, len(metafile.Inputs))
	for inputPath := range metafile.Inputs {
		cssInputPaths = append(cssInputPaths, inputPath)
	}
	return cssInputPaths, nil
}

func (p *Processor) writeCriticalCSSOutput(cssBytes []byte) error {
	criticalCSSOutputPath := p.cfg.Dist.CriticalCSS()
	if _, writeError := static.WriteFileAtomicBytesIfChanged(criticalCSSOutputPath, cssBytes); writeError != nil {
		return writeError
	}
	return nil
}

func (p *Processor) writeNormalCSSOutput(cssBytes []byte) (string, error) {
	normalCSSOutputDirectoryPath := p.cfg.Dist.StaticPublic()
	normalCSSRefPath := p.cfg.Dist.NormalCSSRef()
	normalCSSOutputFileName := static.HashBytes(cssBytes, wave.NormalCSSBaseName)

	publishedNormalCSSOutputFileName, publishError := static.PublishHashedArtifactWithRef(
		static.HashedArtifactPublishOptions{
			Log:                   p.log,
			OutputDirectoryPath:   normalCSSOutputDirectoryPath,
			RefFilePath:           normalCSSRefPath,
			DesiredHashedFileName: normalCSSOutputFileName,
			Content:               cssBytes,
			GlobPattern:           wave.NormalCSSGlobPattern,
		},
	)
	if publishError != nil {
		return "", fmt.Errorf("publish CSS output: %w", publishError)
	}

	return publishedNormalCSSOutputFileName, nil
}

func (p *Processor) invalidateHotReloadOutputForBuildNature(
	buildNature cssBuildNature,
) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if buildNature == cssBuildNatureCritical {
		p.cachedCriticalCSSHotReloadOutput = ""
		p.hasCachedCriticalCSSHotReloadOutput = false
		p.criticalCSSHotReloadOutputInvalidatedByRebuild = true
		return
	}

	p.cachedNormalCSSHotReloadURL = ""
	p.hasCachedNormalCSSHotReloadURL = false
	p.normalCSSHotReloadOutputInvalidatedByRebuild = true
}

func (p *Processor) setCachedCriticalCSSHotReloadOutput(
	criticalCSSOutput string,
) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cachedCriticalCSSHotReloadOutput = criticalCSSOutput
	p.hasCachedCriticalCSSHotReloadOutput = true
	p.criticalCSSHotReloadOutputInvalidatedByRebuild = false
}

func (p *Processor) setCachedNormalCSSHotReloadURL(
	normalCSSHotReloadURL string,
) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cachedNormalCSSHotReloadURL = normalCSSHotReloadURL
	p.hasCachedNormalCSSHotReloadURL = true
	p.normalCSSHotReloadOutputInvalidatedByRebuild = false
}

// ReadCriticalCSSHotReloadOutput reads critical CSS output for hot reload.
func (p *Processor) ReadCriticalCSSHotReloadOutput(
	requireFreshBuildOutput bool,
) (string, error) {
	p.mu.RLock()
	cachedCriticalCSSHotReloadOutput := p.cachedCriticalCSSHotReloadOutput
	hasCachedCriticalCSSHotReloadOutput := p.hasCachedCriticalCSSHotReloadOutput
	criticalCSSHotReloadOutputInvalidatedByRebuild := p.criticalCSSHotReloadOutputInvalidatedByRebuild
	p.mu.RUnlock()

	if hasCachedCriticalCSSHotReloadOutput {
		return cachedCriticalCSSHotReloadOutput, nil
	}
	if requireFreshBuildOutput &&
		criticalCSSHotReloadOutputInvalidatedByRebuild {
		return "", fmt.Errorf(
			"critical CSS hot-reload output unavailable from the latest rebuild",
		)
	}

	criticalCSSOutputPath := p.cfg.Dist.CriticalCSS()
	criticalCSSBytes, readError := os.ReadFile(criticalCSSOutputPath)
	if readError != nil {
		return "", readError
	}

	return string(criticalCSSBytes), nil
}

// ReadNormalCSSHotReloadURL reads normal CSS URL for hot reload.
func (p *Processor) ReadNormalCSSHotReloadURL(
	requireFreshBuildOutput bool,
) (string, error) {
	p.mu.RLock()
	cachedNormalCSSHotReloadURL := p.cachedNormalCSSHotReloadURL
	hasCachedNormalCSSHotReloadURL := p.hasCachedNormalCSSHotReloadURL
	normalCSSHotReloadOutputInvalidatedByRebuild := p.normalCSSHotReloadOutputInvalidatedByRebuild
	p.mu.RUnlock()

	if hasCachedNormalCSSHotReloadURL {
		return cachedNormalCSSHotReloadURL, nil
	}
	if requireFreshBuildOutput && normalCSSHotReloadOutputInvalidatedByRebuild {
		return "", fmt.Errorf(
			"normal CSS hot-reload URL unavailable from the latest rebuild",
		)
	}

	normalCSSRefBytes, readError := os.ReadFile(p.cfg.Dist.NormalCSSRef())
	if readError != nil {
		return "", readError
	}

	return waveshared.ResolveFromReferencedPath(
		p.cfg.PublicPathPrefix(),
		string(normalCSSRefBytes),
	), nil
}

func (p *Processor) urlResolverPlugin() esbuild.Plugin {
	return esbuild.Plugin{
		Name: "url-resolver",
		Setup: func(build esbuild.PluginBuild) {
			build.OnResolve(esbuild.OnResolveOptions{Filter: ".*", Namespace: "file"},
				func(args esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
					if args.Kind != esbuild.ResolveCSSURLToken {
						return esbuild.OnResolveResult{}, nil
					}

					u, err := url.Parse(args.Path)
					if err == nil && u.Scheme != "" {
						return esbuild.OnResolveResult{}, nil
					}
					if strings.HasPrefix(args.Path, "//") {
						return esbuild.OnResolveResult{}, nil
					}

					if p.resolvePublicURLBuildtimePath == nil {
						return esbuild.OnResolveResult{}, nil
					}
					resolved := p.resolvePublicURLBuildtimePath(args.Path)
					return esbuild.OnResolveResult{
						Path:     resolved,
						External: true,
					}, nil
				},
			)
		},
	}
}

// IsCriticalFile reports whether path is tracked as critical CSS input.
func (p *Processor) IsCriticalFile(path string) bool {
	normalizedPath := normalizeCSSFilePathForImportTracking(path)
	if normalizedPath == "" {
		return false
	}

	p.mu.RLock()
	_, ok := p.criticalImports[normalizedPath]
	p.mu.RUnlock()
	return ok
}

// IsNormalFile reports whether path is tracked as non-critical CSS input.
func (p *Processor) IsNormalFile(path string) bool {
	normalizedPath := normalizeCSSFilePathForImportTracking(path)
	if normalizedPath == "" {
		return false
	}

	p.mu.RLock()
	_, ok := p.normalImports[normalizedPath]
	p.mu.RUnlock()
	return ok
}

func normalizeCSSFilePathForImportTracking(filePath string) string {
	absoluteFilePath, absolutePathError := filepath.Abs(filePath)
	if absolutePathError != nil {
		absoluteFilePath = filepath.Clean(filePath)
	}

	resolvedFilePath, resolveError := filepath.EvalSymlinks(absoluteFilePath)
	if resolveError == nil && resolvedFilePath != "" {
		return filepath.Clean(resolvedFilePath)
	}

	return filepath.Clean(absoluteFilePath)
}

// IsCSSFile reports whether path is tracked as either critical or normal CSS.
func (p *Processor) IsCSSFile(path string) bool {
	return p.IsCriticalFile(path) || p.IsNormalFile(path)
}

// SetTrackedCriticalCSSImportPaths replaces tracked critical CSS import paths.
func (p *Processor) SetTrackedCriticalCSSImportPaths(importPaths []string) {
	normalizedImportPaths := make(map[string]struct{}, len(importPaths))
	for _, importPath := range importPaths {
		normalizedImportPath := normalizeCSSFilePathForImportTracking(importPath)
		if normalizedImportPath == "" {
			continue
		}
		normalizedImportPaths[normalizedImportPath] = struct{}{}
	}

	p.mu.Lock()
	p.criticalImports = normalizedImportPaths
	p.mu.Unlock()
}

// SetTrackedNormalCSSImportPaths replaces tracked normal CSS import paths.
func (p *Processor) SetTrackedNormalCSSImportPaths(importPaths []string) {
	normalizedImportPaths := make(map[string]struct{}, len(importPaths))
	for _, importPath := range importPaths {
		normalizedImportPath := normalizeCSSFilePathForImportTracking(importPath)
		if normalizedImportPath == "" {
			continue
		}
		normalizedImportPaths[normalizedImportPath] = struct{}{}
	}

	p.mu.Lock()
	p.normalImports = normalizedImportPaths
	p.mu.Unlock()
}

// ListTrackedCriticalCSSImportPaths returns tracked critical CSS import paths.
func (p *Processor) ListTrackedCriticalCSSImportPaths() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	importPaths := make([]string, 0, len(p.criticalImports))
	for importPath := range p.criticalImports {
		importPaths = append(importPaths, importPath)
	}
	sort.Strings(importPaths)
	return importPaths
}

// CountTrackedCriticalCSSImportPaths returns tracked critical CSS import count.
func (p *Processor) CountTrackedCriticalCSSImportPaths() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.criticalImports)
}

// CriticalCSSBuildContext returns the current critical CSS esbuild context.
func (p *Processor) CriticalCSSBuildContext() esbuild.BuildContext {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.criticalCtx
}

// NormalCSSBuildContext returns the current normal CSS esbuild context.
func (p *Processor) NormalCSSBuildContext() esbuild.BuildContext {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.normalCtx
}
