package tooling

import (
	"encoding/json"
	"fmt"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/lab/esbuildutil"
)

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

	cssInputPaths, parseMetafileError := parseCSSInputPathsFromBuildResultMetafile(result)
	if parseMetafileError != nil {
		return parseMetafileError
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
