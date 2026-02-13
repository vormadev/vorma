package tooling

import "context"

// broadcastRebuilding sends the "rebuilding" signal to show UI overlay.
// Uses blocking send, but guarded by context to prevent deadlock during shutdown.
func (s *server) broadcastRebuilding() {
	if !s.shouldBroadcastToBrowserClients() {
		return
	}

	if !isBroadcastContextActive(s.refreshMgrCtx) {
		return
	}

	if !s.sendRefreshPayloadWhenBroadcastContextActive(refreshPayload{ChangeType: changeTypeRebuilding}) {
		return
	}
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
func (s *server) broadcastReload(opts reloadOpts) {
	if !s.shouldBroadcastToBrowserClients() {
		return
	}

	if !isBroadcastContextActive(s.refreshMgrCtx) {
		return
	}

	cycleViteApplied := s.waitForReloadReadiness(opts)
	if !shouldBroadcastReloadPayloadAfterReadiness(opts, cycleViteApplied) {
		return
	}

	s.sendRefreshPayloadWhenBroadcastContextActive(opts.payload)
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
) bool {
	if reloadOptions.waitApp {
		s.waitForApp()
	}

	cycleViteApplied := s.cycleViteForReloadIfRequested(reloadOptions.cycleVite)
	if cycleViteApplied {
		return true
	}

	if reloadOptions.waitVite {
		s.waitForVite()
	}

	return false
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

	s.cycleVite()
	return true
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
