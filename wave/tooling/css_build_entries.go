package tooling

func (p *cssProcessor) entryPointForBuildNature(buildNature cssBuildNature) string {
	if buildNature == cssBuildNatureCritical {
		return p.cfg.CriticalCSSEntry()
	}
	return p.cfg.NonCriticalCSSEntry()
}
