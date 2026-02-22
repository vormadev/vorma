package broadcast

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ChangeType communicates the browser-side refresh behavior for one message.
type ChangeType string

const (
	// ChangeTypeNormalCSS asks clients to swap or refresh normal stylesheets.
	ChangeTypeNormalCSS ChangeType = "normal"
	// ChangeTypeCriticalCSS asks clients to update inlined critical CSS.
	ChangeTypeCriticalCSS ChangeType = "critical"
	// ChangeTypeOther asks clients to perform a hard reload.
	ChangeTypeOther ChangeType = "other"
	// ChangeTypeRebuilding toggles rebuilding overlays in client runtimes.
	ChangeTypeRebuilding ChangeType = "rebuilding"
	// ChangeTypeRevalidate asks clients to run framework-specific revalidation.
	ChangeTypeRevalidate ChangeType = "revalidate"
)

// Payload is the websocket message contract sent to browser runtime clients.
type Payload struct {
	ChangeType   ChangeType `json:"changeType"`
	CriticalCSS  string     `json:"criticalCSS"`
	NormalCSSURL string     `json:"normalCSSURL,omitempty"`
}

// ManagerConfig controls websocket manager behavior.
type ManagerConfig struct {
	ReadBufferSize  int
	WriteBufferSize int
	WriteTimeout    time.Duration
	PongWait        time.Duration
	PingPeriod      time.Duration
	SendQueueDepth  int
}

// managerConfigWithDefaults normalizes zero-value config fields.
func managerConfigWithDefaults(inputConfig ManagerConfig) ManagerConfig {
	resolvedConfig := inputConfig
	if resolvedConfig.ReadBufferSize <= 0 {
		resolvedConfig.ReadBufferSize = 1024
	}
	if resolvedConfig.WriteBufferSize <= 0 {
		resolvedConfig.WriteBufferSize = 1024
	}
	if resolvedConfig.WriteTimeout <= 0 {
		resolvedConfig.WriteTimeout = 3 * time.Second
	}
	if resolvedConfig.PongWait <= 0 {
		resolvedConfig.PongWait = 60 * time.Second
	}
	if resolvedConfig.PingPeriod <= 0 {
		resolvedConfig.PingPeriod = (resolvedConfig.PongWait * 9) / 10
	}
	if resolvedConfig.SendQueueDepth <= 0 {
		resolvedConfig.SendQueueDepth = 32
	}
	return resolvedConfig
}

// Manager owns websocket clients and fanout delivery for refresh events.
type Manager struct {
	log    *slog.Logger
	config ManagerConfig

	upgrader websocket.Upgrader

	mu              sync.RWMutex
	clients         map[*clientConnection]struct{}
	registerQueue   chan *clientConnection
	unregisterQueue chan *clientConnection
	broadcastQueue  chan Payload

	closeOnce sync.Once
	closed    chan struct{}
}

// clientConnection wraps one websocket client socket and delivery queue.
type clientConnection struct {
	conn      *websocket.Conn
	sendQueue chan Payload
}

// NewManager creates a websocket broadcast manager with stable defaults.
func NewManager(log *slog.Logger, config ManagerConfig) *Manager {
	resolvedConfig := managerConfigWithDefaults(config)
	if log == nil {
		log = slog.Default()
	}

	manager := &Manager{
		log:             log,
		config:          resolvedConfig,
		clients:         make(map[*clientConnection]struct{}),
		registerQueue:   make(chan *clientConnection, 64),
		unregisterQueue: make(chan *clientConnection, 64),
		broadcastQueue:  make(chan Payload, 256),
		closed:          make(chan struct{}),
	}

	manager.upgrader = websocket.Upgrader{
		ReadBufferSize:  resolvedConfig.ReadBufferSize,
		WriteBufferSize: resolvedConfig.WriteBufferSize,
		CheckOrigin: func(*http.Request) bool {
			// Browser clients are expected to run on local dev hosts and ports.
			// We accept all origins here and rely on local-process trust boundary.
			return true
		},
	}

	return manager
}

// Run starts client registration and payload fanout processing.
func (manager *Manager) Run(runContext context.Context) {
	if manager == nil {
		return
	}

	for {
		select {
		case <-runContext.Done():
			manager.Close()
			return

		case client := <-manager.registerQueue:
			if client == nil {
				continue
			}
			manager.addClient(client)

		case client := <-manager.unregisterQueue:
			if client == nil {
				continue
			}
			manager.removeClient(client)

		case payload := <-manager.broadcastQueue:
			manager.broadcastPayload(payload)

		case <-manager.closed:
			return
		}
	}
}

// ServeHTTP upgrades one request into a websocket client session.
func (manager *Manager) ServeHTTP(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	if manager == nil {
		http.Error(
			responseWriter,
			"broadcast manager unavailable",
			http.StatusServiceUnavailable,
		)
		return
	}
	if manager.isClosed() {
		http.Error(
			responseWriter,
			"broadcast manager unavailable",
			http.StatusServiceUnavailable,
		)
		return
	}

	if request.Method != http.MethodGet {
		http.Error(
			responseWriter,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	connection, upgradeError := manager.upgrader.Upgrade(
		responseWriter,
		request,
		nil,
	)
	if upgradeError != nil {
		manager.log.Warn("websocket upgrade failed", "error", upgradeError)
		return
	}

	client := &clientConnection{
		conn:      connection,
		sendQueue: make(chan Payload, manager.config.SendQueueDepth),
	}

	manager.enqueueClientRegister(client)
	go manager.readPump(client)
	go manager.writePump(client)
}

// Broadcast enqueues one payload to every connected client.
func (manager *Manager) Broadcast(payload Payload) {
	if manager == nil {
		return
	}
	select {
	case manager.broadcastQueue <- payload:
	default:
		manager.log.Warn(
			"dropping broadcast payload because queue is full",
			"change_type",
			payload.ChangeType,
		)
	}
}

// isClosed reports whether the manager shutdown signal has been fired.
func (manager *Manager) isClosed() bool {
	if manager == nil {
		return true
	}
	select {
	case <-manager.closed:
		return true
	default:
		return false
	}
}

// BroadcastRebuilding sends the canonical rebuilding overlay message.
func (manager *Manager) BroadcastRebuilding() {
	manager.Broadcast(Payload{ChangeType: ChangeTypeRebuilding})
}

// ConnectionCount reports the number of currently connected clients.
func (manager *Manager) ConnectionCount() int {
	if manager == nil {
		return 0
	}
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return len(manager.clients)
}

// Close disconnects all clients and stops manager processing.
func (manager *Manager) Close() {
	if manager == nil {
		return
	}
	manager.closeOnce.Do(func() {
		close(manager.closed)
		manager.mu.Lock()
		for client := range manager.clients {
			delete(manager.clients, client)
			_ = client.conn.Close()
			close(client.sendQueue)
		}
		manager.mu.Unlock()
		manager.drainPendingQueuesOnClose()
	})
}

// drainPendingQueuesOnClose discards queued work once shutdown begins.
func (manager *Manager) drainPendingQueuesOnClose() {
	for {
		drainedAny := false

		select {
		case queuedClient := <-manager.registerQueue:
			drainedAny = true
			if queuedClient != nil {
				_ = queuedClient.conn.Close()
			}
		default:
		}

		select {
		case queuedClient := <-manager.unregisterQueue:
			drainedAny = true
			if queuedClient != nil {
				_ = queuedClient.conn.Close()
			}
		default:
		}

		select {
		case <-manager.broadcastQueue:
			drainedAny = true
		default:
		}

		if !drainedAny {
			return
		}
	}
}

// enqueueClientRegister pushes client registration onto the manager queue.
func (manager *Manager) enqueueClientRegister(client *clientConnection) {
	select {
	case manager.registerQueue <- client:
	case <-manager.closed:
		_ = client.conn.Close()
		close(client.sendQueue)
	}
}

// enqueueClientUnregister pushes client removal onto the manager queue.
func (manager *Manager) enqueueClientUnregister(client *clientConnection) {
	select {
	case manager.unregisterQueue <- client:
	case <-manager.closed:
	}
}

// addClient inserts the client into the active connection set.
func (manager *Manager) addClient(client *clientConnection) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	select {
	case <-manager.closed:
		_ = client.conn.Close()
		close(client.sendQueue)
		return
	default:
	}
	manager.clients[client] = struct{}{}
}

// removeClient removes and closes one client connection.
func (manager *Manager) removeClient(client *clientConnection) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if _, found := manager.clients[client]; !found {
		return
	}
	delete(manager.clients, client)
	_ = client.conn.Close()
	close(client.sendQueue)
}

// broadcastPayload fanouts payload to clients and drops backpressured peers.
func (manager *Manager) broadcastPayload(payload Payload) {
	manager.mu.RLock()
	clientsSnapshot := make([]*clientConnection, 0, len(manager.clients))
	for client := range manager.clients {
		clientsSnapshot = append(clientsSnapshot, client)
	}
	manager.mu.RUnlock()

	for _, client := range clientsSnapshot {
		select {
		case client.sendQueue <- payload:
		default:
			manager.log.Warn("disconnecting slow websocket client")
			manager.enqueueClientUnregister(client)
		}
	}
}

// readPump consumes incoming messages to detect disconnects promptly.
func (manager *Manager) readPump(client *clientConnection) {
	defer manager.enqueueClientUnregister(client)

	_ = client.conn.SetReadDeadline(time.Now().Add(manager.config.PongWait))
	client.conn.SetPongHandler(func(string) error {
		return client.conn.SetReadDeadline(
			time.Now().Add(manager.config.PongWait),
		)
	})

	for {
		if _, _, readError := client.conn.ReadMessage(); readError != nil {
			if !isExpectedSocketClose(readError) {
				manager.log.Debug(
					"websocket read loop ended",
					"error",
					readError,
				)
			}
			return
		}
	}
}

// writePump serializes payload delivery and periodic ping frames.
func (manager *Manager) writePump(client *clientConnection) {
	pingTicker := time.NewTicker(manager.config.PingPeriod)
	defer pingTicker.Stop()
	defer manager.enqueueClientUnregister(client)

	for {
		select {
		case payload, queueOpen := <-client.sendQueue:
			if !queueOpen {
				_ = client.conn.SetWriteDeadline(
					time.Now().Add(manager.config.WriteTimeout),
				)
				_ = client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if writeError := writeJSONMessageWithDeadline(client.conn, payload, manager.config.WriteTimeout); writeError != nil {
				if !isExpectedSocketClose(writeError) {
					manager.log.Debug(
						"websocket write loop ended",
						"error",
						writeError,
					)
				}
				return
			}

		case <-pingTicker.C:
			if pingError := writeControlWithDeadline(client.conn, websocket.PingMessage, manager.config.WriteTimeout); pingError != nil {
				if !isExpectedSocketClose(pingError) {
					manager.log.Debug(
						"websocket ping failed",
						"error",
						pingError,
					)
				}
				return
			}

		case <-manager.closed:
			return
		}
	}
}

// writeJSONMessageWithDeadline writes one JSON payload with write deadline.
func writeJSONMessageWithDeadline(
	conn *websocket.Conn,
	payload Payload,
	timeout time.Duration,
) error {
	if setDeadlineError := conn.SetWriteDeadline(time.Now().Add(timeout)); setDeadlineError != nil {
		return setDeadlineError
	}
	return conn.WriteJSON(payload)
}

// writeControlWithDeadline writes one control message with deadline.
func writeControlWithDeadline(
	conn *websocket.Conn,
	messageType int,
	timeout time.Duration,
) error {
	return conn.WriteControl(messageType, []byte{}, time.Now().Add(timeout))
}

// isExpectedSocketClose reports whether an error is a routine close outcome.
func isExpectedSocketClose(err error) bool {
	if err == nil {
		return false
	}
	if websocket.IsCloseError(
		err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
	) {
		return true
	}
	return errors.Is(err, context.Canceled)
}
