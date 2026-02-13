package tooling

import (
	"encoding/json"
	"fmt"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/lab/esbuildutil"
)

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
		normalizedFilePath := normalizeCSSFilePathForImportTracking(filePath)
		if normalizedFilePath == "" {
			continue
		}
		importsForBuildNature[normalizedFilePath] = struct{}{}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if buildNature == cssBuildNatureCritical {
		p.criticalImports = importsForBuildNature
		return
	}
	p.normalImports = importsForBuildNature
}
