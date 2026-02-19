package broadcast

import (
	"context"
	"net/http"

	"github.com/gorilla/websocket"
)

// ChangeType identifies what changed so browser clients can apply the
// appropriate refresh behavior.
type ChangeType string

const (
	ChangeTypeNormalCSS   ChangeType = "normal"
	ChangeTypeCriticalCSS ChangeType = "critical"
	ChangeTypeOther       ChangeType = "other"
	ChangeTypeRebuilding  ChangeType = "rebuilding"
	ChangeTypeRevalidate  ChangeType = "revalidate"
)

// Payload is the websocket message delivered to browser refresh clients.
type Payload struct {
	ChangeType   ChangeType `json:"changeType"`
	CriticalCSS  string     `json:"criticalCSS"` // base64
	NormalCSSURL string     `json:"normalCSSURL"`
}

// Manager coordinates websocket refresh clients and fanout broadcasts.
type Manager struct {
	Clients    map[*Client]bool
	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan Payload
	Done       chan struct{} // signals manager has fully stopped
}

// Client holds one websocket connection and its outbound queue.
type Client struct {
	ID     string
	Conn   *websocket.Conn
	Notify chan Payload
}

// NewManager constructs a refresh client manager with non-blocking register and
// unregister queues.
func NewManager() *Manager {
	return &Manager{
		Clients:    make(map[*Client]bool),
		Register:   make(chan *Client, 16), // buffered to prevent handler blocking
		Unregister: make(chan *Client, 16), // buffered to prevent handler blocking
		Broadcast:  make(chan Payload),     // unbuffered
		Done:       make(chan struct{}),
	}
}

// Start runs the client manager loop until context is cancelled.
// It drains channels after context cancellation to prevent handler deadlocks.
func (manager *Manager) Start(ctx context.Context) {
	defer close(manager.Done)

	for {
		select {
		case <-ctx.Done():
			manager.cleanupClientsOnShutdown()
			manager.DrainChannels()
			return

		case clientToRegister := <-manager.Register:
			manager.Clients[clientToRegister] = true

		case clientToUnregister := <-manager.Unregister:
			manager.unregisterClient(clientToUnregister)

		case message := <-manager.Broadcast:
			manager.broadcastMessageToClients(message)
		}
	}
}

func (manager *Manager) cleanupClientsOnShutdown() {
	for client := range manager.Clients {
		close(client.Notify)
		client.Conn.Close()
	}
}

func (manager *Manager) unregisterClient(clientToUnregister *Client) {
	if _, ok := manager.Clients[clientToUnregister]; ok {
		delete(manager.Clients, clientToUnregister)
		close(clientToUnregister.Notify)
		clientToUnregister.Conn.Close()
	}
}

func (manager *Manager) broadcastMessageToClients(message Payload) {
	for client := range manager.Clients {
		select {
		case client.Notify <- message:
		default:
			// Skip clients that are not ready to receive messages.
		}
	}
}

// DrainChannels empties buffered manager channels to prevent goroutine leaks.
func (manager *Manager) DrainChannels() {
	for {
		select {
		case client := <-manager.Register:
			client.Conn.Close()
		case client := <-manager.Unregister:
			client.Conn.Close()
		case <-manager.Broadcast:
			// discard
		default:
			return
		}
	}
}

// Wait blocks until the manager has fully stopped.
func (manager *Manager) Wait() {
	<-manager.Done
}

// ReloadOptions configures browser reload orchestration behavior.
type ReloadOptions struct {
	Payload   Payload
	WaitApp   bool
	WaitVite  bool
	CycleVite bool
}

// ReloadReadinessOutcome captures what readiness gating was applied before a
// reload payload decision was made.
type ReloadReadinessOutcome struct {
	WaitedForApp     bool
	WaitedForVite    bool
	CycleViteApplied bool
}

// ReloadOrchestrationOutcome captures the broadcast and readiness decision path
// for one reload request.
type ReloadOrchestrationOutcome struct {
	BroadcastEnabled       bool
	BroadcastContextActive bool

	ReadinessOutcome       ReloadReadinessOutcome
	ShouldBroadcastPayload bool
	PayloadBroadcasted     bool
}

// Orchestrator coordinates reload payload delivery and optional readiness
// waiting for one devserver instance.
type Orchestrator struct {
	Manager               *Manager
	ManagerContext        context.Context
	BroadcastEnabled      bool
	WaitForApp            func()
	WaitForVite           func()
	CanCycleViteForReload func() bool
	CycleViteAndWait      func() bool
}

// BroadcastRebuilding sends the rebuilding overlay payload when broadcast
// delivery is currently enabled.
func (orchestrator Orchestrator) BroadcastRebuilding() {
	if !orchestrator.BroadcastEnabled {
		return
	}
	if !isBroadcastContextActive(orchestrator.ManagerContext) {
		return
	}

	_ = orchestrator.sendPayloadWhenContextActive(Payload{
		ChangeType: ChangeTypeRebuilding,
	})
}

// BroadcastReload executes readiness and payload broadcast orchestration for
// one reload request.
func (orchestrator Orchestrator) BroadcastReload(
	reloadOptions ReloadOptions,
) ReloadOrchestrationOutcome {
	reloadOutcome := ReloadOrchestrationOutcome{
		BroadcastEnabled: orchestrator.BroadcastEnabled,
	}
	if !reloadOutcome.BroadcastEnabled {
		return reloadOutcome
	}

	reloadOutcome.BroadcastContextActive = isBroadcastContextActive(
		orchestrator.ManagerContext,
	)
	if !reloadOutcome.BroadcastContextActive {
		return reloadOutcome
	}

	reloadOutcome.ReadinessOutcome = orchestrator.waitForReloadReadiness(
		reloadOptions,
	)
	reloadOutcome.ShouldBroadcastPayload = ShouldBroadcastReloadPayloadAfterReadiness(
		reloadOptions,
		reloadOutcome.ReadinessOutcome.CycleViteApplied,
	)
	if !reloadOutcome.ShouldBroadcastPayload {
		return reloadOutcome
	}

	reloadOutcome.PayloadBroadcasted = orchestrator.sendPayloadWhenContextActive(
		reloadOptions.Payload,
	)
	return reloadOutcome
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

func (orchestrator Orchestrator) sendPayloadWhenContextActive(payload Payload) bool {
	if orchestrator.Manager == nil {
		return false
	}
	if !isBroadcastContextActive(orchestrator.ManagerContext) {
		return false
	}

	select {
	case orchestrator.Manager.Broadcast <- payload:
		return true
	case <-orchestrator.ManagerContext.Done():
		return false
	}
}

func (orchestrator Orchestrator) waitForReloadReadiness(
	reloadOptions ReloadOptions,
) ReloadReadinessOutcome {
	reloadReadinessOutcomeForReload := ReloadReadinessOutcome{}

	if reloadOptions.WaitApp {
		reloadReadinessOutcomeForReload.WaitedForApp = true
		if orchestrator.WaitForApp != nil {
			orchestrator.WaitForApp()
		}
	}

	reloadReadinessOutcomeForReload.CycleViteApplied = orchestrator.cycleViteForReloadIfRequested(
		reloadOptions.CycleVite,
	)
	if reloadReadinessOutcomeForReload.CycleViteApplied {
		return reloadReadinessOutcomeForReload
	}

	if reloadOptions.WaitVite {
		reloadReadinessOutcomeForReload.WaitedForVite = true
		if orchestrator.WaitForVite != nil {
			orchestrator.WaitForVite()
		}
	}

	return reloadReadinessOutcomeForReload
}

func (orchestrator Orchestrator) cycleViteForReloadIfRequested(
	cycleViteRequested bool,
) bool {
	if !cycleViteRequested {
		return false
	}
	if orchestrator.CanCycleViteForReload == nil ||
		!orchestrator.CanCycleViteForReload() {
		return false
	}
	if orchestrator.CycleViteAndWait == nil {
		return false
	}

	return orchestrator.CycleViteAndWait()
}

// ShouldBroadcastReloadPayloadAfterReadiness determines if Wave should deliver
// a reload payload after readiness gates have completed.
func ShouldBroadcastReloadPayloadAfterReadiness(
	reloadOptions ReloadOptions,
	cycleViteApplied bool,
) bool {
	if !reloadOptions.CycleVite {
		return true
	}
	return !cycleViteApplied
}

// Upgrader upgrades the refresh endpoint HTTP connection to websocket.
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(_ *http.Request) bool { return true },
}

// WebsocketHandler serves refresh websocket clients for one manager instance.
func WebsocketHandler(
	manager *Manager,
	ctx context.Context,
) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !shouldAcceptWebsocketConnection(ctx) {
			http.Error(
				writer,
				"server shutting down",
				http.StatusServiceUnavailable,
			)
			return
		}

		connection, upgradeErr := Upgrader.Upgrade(writer, request, nil)
		if upgradeErr != nil {
			return
		}

		client := &Client{
			ID:     request.RemoteAddr,
			Conn:   connection,
			Notify: make(chan Payload, 1),
		}

		if !tryRegisterWebsocketClient(manager, client, connection, ctx) {
			return
		}

		defer unregisterWebsocketClientOnExit(manager, client, ctx)
		go runWebsocketClientReadLoop(manager, client, connection, ctx)
		runWebsocketClientWriteLoop(client, connection, ctx)
	}
}

func shouldAcceptWebsocketConnection(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	default:
		return true
	}
}

func tryRegisterWebsocketClient(
	manager *Manager,
	clientToRegister *Client,
	connection *websocket.Conn,
	ctx context.Context,
) bool {
	select {
	case manager.Register <- clientToRegister:
		return true
	case <-ctx.Done():
		connection.Close()
		return false
	}
}

func unregisterWebsocketClientOnExit(
	manager *Manager,
	clientToUnregister *Client,
	ctx context.Context,
) {
	select {
	case manager.Unregister <- clientToUnregister:
	case <-ctx.Done():
	default:
		// Channel full or closed, manager will clean up.
	}
}

func runWebsocketClientReadLoop(
	manager *Manager,
	client *Client,
	connection *websocket.Conn,
	ctx context.Context,
) {
	defer connection.Close()
	for {
		if _, _, readErr := connection.ReadMessage(); readErr != nil {
			select {
			case manager.Unregister <- client:
			case <-ctx.Done():
			default:
			}
			break
		}
	}
}

func runWebsocketClientWriteLoop(
	client *Client,
	connection *websocket.Conn,
	ctx context.Context,
) {
	for {
		select {
		case payload, ok := <-client.Notify:
			if !ok {
				return
			}
			if writeErr := connection.WriteJSON(payload); writeErr != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
