package tooling

func (p *cssProcessor) setBuildNatureImports(
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
