package tooling

import (
	"fmt"
	"os"

	"github.com/vormadev/vorma/internal/waveurl"
	"github.com/vormadev/vorma/wave"
)

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

	publishedNormalCSSOutputFileName, publishError := publishHashedArtifactWithRef(
		hashedArtifactPublishOptions{
			log:                   p.log,
			outputDirectoryPath:   normalCSSOutputDirectoryPath,
			refFilePath:           normalCSSRefPath,
			desiredHashedFileName: normalCSSOutputFileName,
			content:               cssBytes,
			globPattern:           wave.NormalCSSGlobPattern,
		},
	)
	if publishError != nil {
		return "", fmt.Errorf("publish CSS output: %w", publishError)
	}

	return publishedNormalCSSOutputFileName, nil
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
		return "", fmt.Errorf(
			"normal CSS hot-reload URL unavailable from the latest rebuild",
		)
	}

	normalCSSRefBytes, readError := os.ReadFile(p.cfg.Dist.NormalCSSRef())
	if readError != nil {
		return "", readError
	}

	return waveurl.ResolveFromReferencedPath(
		p.cfg.PublicPathPrefix(),
		string(normalCSSRefBytes),
	), nil
}
