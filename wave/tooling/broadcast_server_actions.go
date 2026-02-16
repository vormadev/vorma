package tooling

import "context"

type reloadReadinessOutcome struct {
	waitedForApp     bool
	waitedForVite    bool
	cycleViteApplied bool
}

type reloadOrchestrationOutcome struct {
	broadcastEnabled       bool
	broadcastContextActive bool

	readinessOutcome       reloadReadinessOutcome
	shouldBroadcastPayload bool
	payloadBroadcasted     bool
}

// broadcastRebuilding sends the "rebuilding" signal to show UI overlay.
// Uses blocking send, but guarded by context to prevent deadlock during shutdown.
func (s *server) broadcastRebuilding() {
	if !s.shouldBroadcastToBrowserClients() {
		return
	}

	if !isBroadcastContextActive(s.refreshMgrCtx) {
		return
	}

	_ = s.sendRefreshPayloadWhenBroadcastContextActive(
		refreshPayload{ChangeType: changeTypeRebuilding},
	)
}

// broadcastReload handles browser reload orchestration.
//
// When cycleVite is true:
//  1. Wait for app to be ready
//  2. Stop and restart Vite
//  3. Wait for Vite to be ready
//  4. Vite's client reconnect triggers the browser reload automatically
//  5. Do NOT send Wave's reload signal (would cause double reload)
//
// When cycleVite is false:
//  1. Wait for app/vite as specified
//  2. Send Wave's reload signal to trigger browser reload
func (s *server) broadcastReload(
	reloadOptions reloadOpts,
) reloadOrchestrationOutcome {
	reloadOutcome := reloadOrchestrationOutcome{
		broadcastEnabled: s.shouldBroadcastToBrowserClients(),
	}
	if !reloadOutcome.broadcastEnabled {
		return reloadOutcome
	}

	reloadOutcome.broadcastContextActive = isBroadcastContextActive(s.refreshMgrCtx)
	if !reloadOutcome.broadcastContextActive {
		return reloadOutcome
	}

	reloadOutcome.readinessOutcome = s.waitForReloadReadiness(reloadOptions)
	reloadOutcome.shouldBroadcastPayload = shouldBroadcastReloadPayloadAfterReadiness(
		reloadOptions,
		reloadOutcome.readinessOutcome.cycleViteApplied,
	)
	if !reloadOutcome.shouldBroadcastPayload {
		return reloadOutcome
	}

	reloadOutcome.payloadBroadcasted = s.sendRefreshPayloadWhenBroadcastContextActive(
		reloadOptions.payload,
	)
	return reloadOutcome
}

func (s *server) shouldBroadcastToBrowserClients() bool {
	return s.cfg.UsingBrowser() && s.refreshMgr != nil
}

func isBroadcastContextActive(refreshManagerContext context.Context) bool {
	if refreshManagerContext == nil {
		return true
	}

	select {
	case <-refreshManagerContext.Done():
		return false
	default:
		return true
	}
}

func (s *server) sendRefreshPayloadWhenBroadcastContextActive(
	payload refreshPayload,
) bool {
	if s.refreshMgr == nil {
		return false
	}

	if !isBroadcastContextActive(s.refreshMgrCtx) {
		return false
	}

	select {
	case s.refreshMgr.broadcast <- payload:
		return true
	case <-s.refreshMgrCtx.Done():
		return false
	}
}

func (s *server) waitForReloadReadiness(
	reloadOptions reloadOpts,
) reloadReadinessOutcome {
	reloadReadinessOutcomeForReload := reloadReadinessOutcome{}

	if reloadOptions.waitApp {
		reloadReadinessOutcomeForReload.waitedForApp = true
		s.waitForApp()
	}

	reloadReadinessOutcomeForReload.cycleViteApplied = s.cycleViteForReloadIfRequested(
		reloadOptions.cycleVite,
	)
	if reloadReadinessOutcomeForReload.cycleViteApplied {
		return reloadReadinessOutcomeForReload
	}

	if reloadOptions.waitVite {
		reloadReadinessOutcomeForReload.waitedForVite = true
		s.waitForVite()
	}

	return reloadReadinessOutcomeForReload
}

func (s *server) cycleViteForReloadIfRequested(
	cycleViteRequested bool,
) bool {
	if !cycleViteRequested || !s.cfg.UsingVite() {
		return false
	}

	s.mu.Lock()
	hasActiveViteContext := s.viteCtx != nil
	s.mu.Unlock()
	if !hasActiveViteContext {
		return false
	}

	return s.cycleViteAndWaitForReadiness()
}

func shouldBroadcastReloadPayloadAfterReadiness(
	reloadOptions reloadOpts,
	cycleViteApplied bool,
) bool {
	if !reloadOptions.cycleVite {
		return true
	}
	return !cycleViteApplied
}
