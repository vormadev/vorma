package tooling

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/lab/esbuildutil"
	"github.com/vormadev/vorma/wave"
)

type cssProcessor struct {
	cfg *wave.ParsedConfig
	log *slog.Logger
	b   *Builder

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

	// Cached file map for URL resolution during build
	cachedFileMap   wave.FileMap
	cachedFileMapMu sync.Mutex
}

type cssBuildNature string

const (
	cssBuildNatureCritical cssBuildNature = "critical"
	cssBuildNatureNormal   cssBuildNature = "normal"
)

// newCSSProcessor creates a CSS processor with the builder reference
func newCSSProcessor(cfg *wave.ParsedConfig, log *slog.Logger, b *Builder) *cssProcessor {
	if log == nil {
		log = colorlog.New("wave")
	}

	return &cssProcessor{
		cfg:             cfg,
		log:             log,
		b:               b,
		criticalImports: make(map[string]struct{}),
		normalImports:   make(map[string]struct{}),
	}
}

// close disposes of esbuild contexts to prevent resource leaks
func (p *cssProcessor) close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.hasCriticalCtx {
		p.criticalCtx.Dispose()
		p.hasCriticalCtx = false
		p.criticalEntry = ""
		p.criticalCtxIsDev = false
	}
	if p.hasNormalCtx {
		p.normalCtx.Dispose()
		p.hasNormalCtx = false
		p.normalEntry = ""
		p.normalCtxIsDev = false
	}
	p.criticalImports = make(map[string]struct{})
	p.normalImports = make(map[string]struct{})
	p.cachedCriticalCSSHotReloadOutput = ""
	p.hasCachedCriticalCSSHotReloadOutput = false
	p.criticalCSSHotReloadOutputInvalidatedByRebuild = false
	p.cachedNormalCSSHotReloadURL = ""
	p.hasCachedNormalCSSHotReloadURL = false
	p.normalCSSHotReloadOutputInvalidatedByRebuild = false
	return nil
}

func (p *cssProcessor) buildAll(isDev bool) error {
	if err := p.buildCritical(isDev); err != nil {
		return fmt.Errorf("critical CSS: %w", err)
	}
	if err := p.buildNormal(isDev); err != nil {
		return fmt.Errorf("normal CSS: %w", err)
	}
	return nil
}

func (p *cssProcessor) buildCritical(isDev bool) error {
	return p.build(cssBuildNatureCritical, isDev)
}

func (p *cssProcessor) buildNormal(isDev bool) error {
	return p.build(cssBuildNatureNormal, isDev)
}

func (p *cssProcessor) build(buildNature cssBuildNature, isDev bool) error {
	p.cachedFileMapMu.Lock()
	p.cachedFileMap = nil
	p.cachedFileMapMu.Unlock()
	p.invalidateHotReloadOutputForBuildNature(buildNature)

	entryPoint := p.entryPointForBuildNature(buildNature)

	if entryPoint == "" {
		p.clearBuildNatureState(buildNature)
		return nil
	}

	ctx, err := p.getOrCreateContextForBuildNature(buildNature, entryPoint, isDev)
	if err != nil {
		return err
	}

	result := ctx.Rebuild()
	if err := esbuildutil.CollectErrors(result); err != nil {
		return err
	}

	if len(result.OutputFiles) == 0 {
		return fmt.Errorf("esbuild produced no output files for %s CSS", buildNature)
	}

	// Track imports for file watching
	var metafile esbuildutil.ESBuildMetafileSubset
	if err := json.Unmarshal([]byte(result.Metafile), &metafile); err != nil {
		return fmt.Errorf("parse metafile: %w", err)
	}

	cssInputPaths := make([]string, 0, len(metafile.Inputs))
	for inputPath := range metafile.Inputs {
		cssInputPaths = append(cssInputPaths, inputPath)
	}
	p.setBuildNatureImports(buildNature, cssInputPaths)

	// Write output
	if buildNature == cssBuildNatureCritical {
		if writeError := p.writeCriticalCSSOutput(result.OutputFiles[0].Contents); writeError != nil {
			return writeError
		}
		p.setCachedCriticalCSSHotReloadOutput(string(result.OutputFiles[0].Contents))
		return nil
	}

	normalCSSOutputFileName, writeError := p.writeNormalCSSOutput(result.OutputFiles[0].Contents)
	if writeError != nil {
		return writeError
	}
	p.setCachedNormalCSSHotReloadURL(p.cfg.PublicPathPrefix() + normalCSSOutputFileName)
	return nil
}

func (p *cssProcessor) entryPointForBuildNature(buildNature cssBuildNature) string {
	if buildNature == cssBuildNatureCritical {
		return p.cfg.CriticalCSSEntry()
	}
	return p.cfg.NonCriticalCSSEntry()
}

func (p *cssProcessor) clearBuildNatureState(buildNature cssBuildNature) {
	p.mu.Lock()
	defer p.mu.Unlock()

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

func (p *cssProcessor) getBuildNatureContextState(
	buildNature cssBuildNature,
) (esbuild.BuildContext, bool, string, bool) {
	if buildNature == cssBuildNatureCritical {
		return p.criticalCtx, p.hasCriticalCtx, p.criticalEntry, p.criticalCtxIsDev
	}
	return p.normalCtx, p.hasNormalCtx, p.normalEntry, p.normalCtxIsDev
}

func (p *cssProcessor) setBuildNatureContextState(
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

func (p *cssProcessor) getOrCreateContextForBuildNature(
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

func (p *cssProcessor) createContext(
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

func (p *cssProcessor) setBuildNatureImports(
	buildNature cssBuildNature,
	cssInputPaths []string,
) {
	importsForBuildNature := make(map[string]struct{}, len(cssInputPaths))
	for _, filePath := range cssInputPaths {
		absolutePath, absolutePathError := filepath.Abs(filePath)
		if absolutePathError != nil {
			absolutePath = filePath
		}
		importsForBuildNature[absolutePath] = struct{}{}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if buildNature == cssBuildNatureCritical {
		p.criticalImports = importsForBuildNature
		return
	}
	p.normalImports = importsForBuildNature
}

func (p *cssProcessor) writeCriticalCSSOutput(cssBytes []byte) error {
	criticalCSSOutputPath := p.cfg.Dist.CriticalCSS()
	if _, writeError := writeFileAtomicBytesIfChanged(criticalCSSOutputPath, cssBytes); writeError != nil {
		return writeError
	}
	return nil
}

func (p *cssProcessor) writeNormalCSSOutput(cssBytes []byte) (string, error) {
	normalCSSOutputDirectoryPath := p.cfg.Dist.StaticPublic()
	normalCSSRefPath := p.cfg.Dist.NormalCSSRef()
	normalCSSOutputFileName := hashBytes(cssBytes, wave.NormalCSSBaseName)

	if err := os.MkdirAll(normalCSSOutputDirectoryPath, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	previousNormalCSSOutputFileName, hasPreviousRefFile, readRefError := p.readNormalCSSRefFileName(normalCSSRefPath)
	if readRefError != nil {
		return "", readRefError
	}

	normalCSSOutputFilePath := filepath.Join(normalCSSOutputDirectoryPath, normalCSSOutputFileName)
	if hasPreviousRefFile && previousNormalCSSOutputFileName == normalCSSOutputFileName {
		if _, statError := os.Stat(normalCSSOutputFilePath); statError == nil {
			return normalCSSOutputFileName, nil
		} else if !os.IsNotExist(statError) {
			return "", statError
		}
	}

	if hasPreviousRefFile &&
		previousNormalCSSOutputFileName != "" &&
		previousNormalCSSOutputFileName != normalCSSOutputFileName {
		p.removeNormalCSSArtifact(
			filepath.Join(normalCSSOutputDirectoryPath, previousNormalCSSOutputFileName),
		)
	}
	if !hasPreviousRefFile {
		p.cleanupOldNormalCSSFilesWhenRefFileMissing(
			normalCSSOutputDirectoryPath,
			normalCSSOutputFileName,
		)
	}

	if _, writeError := writeFileAtomicBytesIfChanged(normalCSSOutputFilePath, cssBytes); writeError != nil {
		return "", writeError
	}

	if _, writeError := writeFileAtomicBytesIfChanged(normalCSSRefPath, []byte(normalCSSOutputFileName)); writeError != nil {
		return "", fmt.Errorf("write CSS ref: %w", writeError)
	}
	return normalCSSOutputFileName, nil
}

func (p *cssProcessor) readNormalCSSRefFileName(
	normalCSSRefPath string,
) (string, bool, error) {
	existingRefData, readError := os.ReadFile(normalCSSRefPath)
	if readError != nil {
		if os.IsNotExist(readError) {
			return "", false, nil
		}
		return "", false, readError
	}

	return strings.TrimSpace(string(existingRefData)), true, nil
}

func (p *cssProcessor) removeNormalCSSArtifact(artifactPath string) {
	if removeError := os.Remove(artifactPath); removeError != nil && !os.IsNotExist(removeError) {
		p.log.Warn("failed to remove old CSS file", "file", artifactPath, "error", removeError)
	}
}

func (p *cssProcessor) cleanupOldNormalCSSFilesWhenRefFileMissing(
	normalCSSOutputDirectoryPath string,
	normalCSSOutputFileName string,
) {
	oldFiles, globError := filepath.Glob(filepath.Join(normalCSSOutputDirectoryPath, wave.NormalCSSGlobPattern))
	if globError != nil {
		p.log.Warn("failed to glob old CSS files", "error", globError)
		return
	}

	for _, oldFilePath := range oldFiles {
		if filepath.Base(oldFilePath) == normalCSSOutputFileName {
			continue
		}
		if removeError := os.Remove(oldFilePath); removeError != nil {
			p.log.Warn("failed to remove old CSS file", "file", oldFilePath, "error", removeError)
		}
	}
}

func (p *cssProcessor) invalidateHotReloadOutputForBuildNature(
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

func (p *cssProcessor) setCachedCriticalCSSHotReloadOutput(
	criticalCSSOutput string,
) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cachedCriticalCSSHotReloadOutput = criticalCSSOutput
	p.hasCachedCriticalCSSHotReloadOutput = true
	p.criticalCSSHotReloadOutputInvalidatedByRebuild = false
}

func (p *cssProcessor) setCachedNormalCSSHotReloadURL(
	normalCSSHotReloadURL string,
) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cachedNormalCSSHotReloadURL = normalCSSHotReloadURL
	p.hasCachedNormalCSSHotReloadURL = true
	p.normalCSSHotReloadOutputInvalidatedByRebuild = false
}

func (p *cssProcessor) readCriticalCSSHotReloadOutput(
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
	if requireFreshBuildOutput && criticalCSSHotReloadOutputInvalidatedByRebuild {
		return "", fmt.Errorf("critical CSS hot-reload output unavailable from the latest rebuild")
	}

	criticalCSSOutputPath := p.cfg.Dist.CriticalCSS()
	criticalCSSBytes, readError := os.ReadFile(criticalCSSOutputPath)
	if readError != nil {
		return "", readError
	}

	return string(criticalCSSBytes), nil
}

func (p *cssProcessor) readNormalCSSHotReloadURL(
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
		return "", fmt.Errorf("normal CSS hot-reload URL unavailable from the latest rebuild")
	}

	normalCSSRefBytes, readError := os.ReadFile(p.cfg.Dist.NormalCSSRef())
	if readError != nil {
		return "", readError
	}

	return p.cfg.PublicPathPrefix() + string(normalCSSRefBytes), nil
}

func (p *cssProcessor) urlResolverPlugin() esbuild.Plugin {
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

					resolved := p.b.getPublicURLBuildtimeCached(args.Path)
					return esbuild.OnResolveResult{
						Path:     resolved,
						External: true,
					}, nil
				},
			)
		},
	}
}

func (p *cssProcessor) isCriticalFile(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	p.mu.RLock()
	_, ok := p.criticalImports[absPath]
	p.mu.RUnlock()
	return ok
}

func (p *cssProcessor) isNormalFile(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	p.mu.RLock()
	_, ok := p.normalImports[absPath]
	p.mu.RUnlock()
	return ok
}

// IsCriticalCSSFile checks if a path is a critical CSS file or import
func (b *Builder) IsCriticalCSSFile(path string) bool {
	return b.css.isCriticalFile(path)
}

// IsNormalCSSFile checks if a path is a normal CSS file or import
func (b *Builder) IsNormalCSSFile(path string) bool {
	return b.css.isNormalFile(path)
}

// IsCSSFile checks if a path is any CSS file tracked by the builder
func (b *Builder) IsCSSFile(path string) bool {
	return b.IsCriticalCSSFile(path) || b.IsNormalCSSFile(path)
}
