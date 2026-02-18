package tooling

import (
	"context"

	"github.com/gorilla/websocket"
)

type changeType string

const (
	changeTypeNormalCSS   changeType = "normal"
	changeTypeCriticalCSS changeType = "critical"
	changeTypeOther       changeType = "other"
	changeTypeRebuilding  changeType = "rebuilding"
	changeTypeRevalidate  changeType = "revalidate"
)

type refreshPayload struct {
	ChangeType   changeType `json:"changeType"`
	CriticalCSS  string     `json:"criticalCSS"` // base64
	NormalCSSURL string     `json:"normalCSSURL"`
}

type clientManager struct {
	clients    map[*client]bool
	register   chan *client
	unregister chan *client
	broadcast  chan refreshPayload
	done       chan struct{} // signals manager has fully stopped
}

type client struct {
	id     string
	conn   *websocket.Conn
	notify chan refreshPayload
}

// reloadOpts configures broadcastReload behavior
type reloadOpts struct {
	payload   refreshPayload
	waitApp   bool
	waitVite  bool
	cycleVite bool
}

func newClientManager() *clientManager {
	return &clientManager{
		clients:    make(map[*client]bool),
		register:   make(chan *client, 16),    // buffered to prevent handler blocking
		unregister: make(chan *client, 16),    // buffered to prevent handler blocking
		broadcast:  make(chan refreshPayload), // unbuffered
		done:       make(chan struct{}),
	}
}

// start runs the client manager loop until context is cancelled.
// Drains channels after context cancellation to prevent handler deadlocks.
func (m *clientManager) start(ctx context.Context) {
	defer close(m.done)

	for {
		select {
		case <-ctx.Done():
			m.cleanupClientsOnShutdown()
			m.drainChannels()
			return

		case clientToRegister := <-m.register:
			m.clients[clientToRegister] = true

		case clientToUnregister := <-m.unregister:
			m.unregisterClient(clientToUnregister)

		case message := <-m.broadcast:
			m.broadcastMessageToClients(message)
		}
	}
}

func (m *clientManager) cleanupClientsOnShutdown() {
	for c := range m.clients {
		close(c.notify)
		c.conn.Close()
	}
}

func (m *clientManager) unregisterClient(clientToUnregister *client) {
	if _, ok := m.clients[clientToUnregister]; ok {
		delete(m.clients, clientToUnregister)
		close(clientToUnregister.notify)
		clientToUnregister.conn.Close()
	}
}

func (m *clientManager) broadcastMessageToClients(message refreshPayload) {
	for c := range m.clients {
		select {
		case c.notify <- message:
		default:
			// Skip clients that are not ready to receive messages
		}
	}
}

// drainChannels empties buffered channels to prevent goroutine leaks
func (m *clientManager) drainChannels() {
	for {
		select {
		case c := <-m.register:
			c.conn.Close()
		case c := <-m.unregister:
			c.conn.Close()
		case <-m.broadcast:
			// discard
		default:
			return
		}
	}
}

// wait blocks until the manager has fully stopped
func (m *clientManager) wait() {
	<-m.done
}

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
