package tooling

import "encoding/base64"

func planBrowserReloadForAction(
	action browserPhaseAction,
	browserDecision browserPhaseDecision,
) (reloadOpts, bool) {
	switch action {
	case browserPhaseActionHardReload:
		return reloadOpts{
			payload:   refreshPayload{ChangeType: changeTypeOther},
			waitApp:   browserDecision.waitForApp,
			waitVite:  browserDecision.waitForVite,
			cycleVite: browserDecision.cycleVite,
		}, true
	case browserPhaseActionRevalidate:
		return reloadOpts{
			payload:   refreshPayload{ChangeType: changeTypeRevalidate},
			waitApp:   browserDecision.waitForApp,
			waitVite:  browserDecision.waitForVite,
			cycleVite: false,
		}, true
	default:
		return reloadOpts{}, false
	}
}

func planInvalidateViteFallbackBrowserDecision(
	usingVite bool,
) browserPhaseDecision {
	return browserPhaseDecision{
		action:      browserPhaseActionHardReload,
		waitForApp:  true,
		waitForVite: usingVite,
	}
}

func planHotReloadCSSPayloads(
	includeCriticalCSS bool,
	criticalCSS string,
	criticalCSSAvailable bool,
	includeNormalCSS bool,
	normalCSSURL string,
	normalCSSURLAvailable bool,
) []refreshPayload {
	payloads := make([]refreshPayload, 0, 2)
	if includeCriticalCSS && criticalCSSAvailable {
		payloads = append(payloads, refreshPayload{
			ChangeType:  changeTypeCriticalCSS,
			CriticalCSS: base64.StdEncoding.EncodeToString([]byte(criticalCSS)),
		})
	}
	if includeNormalCSS && normalCSSURLAvailable {
		payloads = append(payloads, refreshPayload{
			ChangeType:   changeTypeNormalCSS,
			NormalCSSURL: normalCSSURL,
		})
	}
	return payloads
}

type browserPhaseExecutionCategory int

const (
	browserPhaseExecutionCategoryNone browserPhaseExecutionCategory = iota
	browserPhaseExecutionCategoryReload
	browserPhaseExecutionCategoryHotReloadCSS
)

func shouldAttemptViteInvalidateForBrowserDecision(
	browserDecision browserPhaseDecision,
	usingVite bool,
) bool {
	return browserDecision.action == browserPhaseActionInvalidateVite && usingVite
}

func resolveBrowserDecisionAfterInvalidateViteFallback(
	browserDecision browserPhaseDecision,
	usingVite bool,
) browserPhaseDecision {
	if browserDecision.action != browserPhaseActionInvalidateVite {
		return browserDecision
	}

	fallbackDecision := planInvalidateViteFallbackBrowserDecision(usingVite)
	browserDecision.action = fallbackDecision.action
	browserDecision.waitForApp = fallbackDecision.waitForApp
	browserDecision.waitForVite = fallbackDecision.waitForVite
	return browserDecision
}

func deriveBrowserPhaseExecutionCategory(
	action browserPhaseAction,
) browserPhaseExecutionCategory {
	switch action {
	case browserPhaseActionHardReload, browserPhaseActionRevalidate:
		return browserPhaseExecutionCategoryReload
	case browserPhaseActionHotReloadCSS:
		return browserPhaseExecutionCategoryHotReloadCSS
	default:
		return browserPhaseExecutionCategoryNone
	}
}

func (s *server) executeBrowserPhase(work *workSet) {
	if !s.cfg.UsingBrowser() {
		return
	}

	builder := s.getBuilder()
	browserDecisionForExecution := work.browser
	if browserDecisionForExecution.action == browserPhaseActionInvalidateVite {
		if shouldAttemptViteInvalidateForBrowserDecision(
			browserDecisionForExecution,
			s.cfg.UsingVite(),
		) {
			if err := s.callViteFilemapInvalidate(); err != nil {
				s.log.Warn("Vite filemap invalidate failed, falling back to reload", "error", err)
			} else {
				return
			}
		}
		browserDecisionForExecution = resolveBrowserDecisionAfterInvalidateViteFallback(
			browserDecisionForExecution,
			s.cfg.UsingVite(),
		)
		work.browser = browserDecisionForExecution
	}

	switch deriveBrowserPhaseExecutionCategory(browserDecisionForExecution.action) {
	case browserPhaseExecutionCategoryReload:
		reloadPlan, hasReloadPlan := planBrowserReloadForAction(
			browserDecisionForExecution.action,
			browserDecisionForExecution,
		)
		if !hasReloadPlan {
			return
		}
		if browserDecisionForExecution.action == browserPhaseActionHardReload {
			s.log.Info("Hard reloading browser")
		} else {
			s.log.Info("Running client-defined revalidate function")
		}
		s.broadcastReload(reloadPlan)
		return

	case browserPhaseExecutionCategoryHotReloadCSS:
		if builder == nil {
			return
		}
		s.executeHotReloadCSSBrowserPhase(builder, work.build)
		return

	case browserPhaseExecutionCategoryNone:
		return
	}
}

func (s *server) executeHotReloadCSSBrowserPhase(
	builder *Builder,
	buildDecision buildPhaseDecision,
) {
	s.log.Info("Hot reloading CSS")

	criticalCSS := ""
	criticalCSSAvailable := false
	if buildDecision.buildCriticalCSS {
		var readCriticalCSSError error
		criticalCSS, readCriticalCSSError = builder.ReadCriticalCSSForHotReload(true)
		if readCriticalCSSError != nil {
			s.log.Warn(
				"Skipping critical CSS hot reload payload due to missing fresh build output",
				"error",
				readCriticalCSSError,
			)
		} else {
			criticalCSSAvailable = true
		}
	}

	normalCSSURL := ""
	normalCSSURLAvailable := false
	if buildDecision.buildNormalCSS {
		var readNormalCSSURLError error
		normalCSSURL, readNormalCSSURLError = builder.ReadNormalCSSURLForHotReload(true)
		if readNormalCSSURLError != nil {
			s.log.Warn(
				"Skipping normal CSS hot reload payload due to missing fresh build output",
				"error",
				readNormalCSSURLError,
			)
		} else {
			normalCSSURLAvailable = true
		}
	}

	payloads := planHotReloadCSSPayloads(
		buildDecision.buildCriticalCSS,
		criticalCSS,
		criticalCSSAvailable,
		buildDecision.buildNormalCSS,
		normalCSSURL,
		normalCSSURLAvailable,
	)
	for _, payload := range payloads {
		s.broadcastReload(reloadOpts{
			payload: payload,
		})
	}
}
