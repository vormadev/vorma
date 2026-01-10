package devserver

import (
	"context"
	"net/http"

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
			// Clean up all clients
			for c := range m.clients {
				close(c.notify)
				c.conn.Close()
			}
			// Drain any pending registrations/unregistrations to unblock handlers
			m.drainChannels()
			return

		case c := <-m.register:
			m.clients[c] = true

		case c := <-m.unregister:
			if _, ok := m.clients[c]; ok {
				delete(m.clients, c)
				close(c.notify)
				c.conn.Close()
			}

		case msg := <-m.broadcast:
			for c := range m.clients {
				select {
				case c.notify <- msg:
				default:
					// Skip clients that are not ready to receive messages
				}
			}
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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func websocketHandler(manager *clientManager, ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Don't accept new connections if shutting down
		select {
		case <-ctx.Done():
			http.Error(w, "server shutting down", http.StatusServiceUnavailable)
			return
		default:
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		c := &client{
			id:     r.RemoteAddr,
			conn:   conn,
			notify: make(chan refreshPayload, 1),
		}

		// Non-blocking send in case manager is shutting down
		select {
		case manager.register <- c:
		case <-ctx.Done():
			conn.Close()
			return
		}

		defer func() {
			// Non-blocking unregister
			select {
			case manager.unregister <- c:
			case <-ctx.Done():
			default:
				// Channel full or closed, manager will clean up
			}
		}()

		go func() {
			defer conn.Close()
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					select {
					case manager.unregister <- c:
					case <-ctx.Done():
					default:
					}
					break
				}
			}
		}()

		for {
			select {
			case msg, ok := <-c.notify:
				if !ok {
					return
				}
				if err := conn.WriteJSON(msg); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}
}

// broadcastRebuilding sends the "rebuilding" signal to show UI overlay.
// Uses blocking send, but guarded by context to prevent deadlock during shutdown.
func (s *server) broadcastRebuilding() {
	if !s.cfg.UsingBrowser() || s.refreshMgr == nil {
		return
	}

	select {
	case <-s.refreshMgrCtx.Done():
		return
	default:
	}

	select {
	case s.refreshMgr.broadcast <- refreshPayload{ChangeType: changeTypeRebuilding}:
	case <-s.refreshMgrCtx.Done():
		return
	}
}

// reloadOpts configures broadcastReload behavior
type reloadOpts struct {
	payload   refreshPayload
	waitApp   bool
	waitVite  bool
	cycleVite bool
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
	if !s.cfg.UsingBrowser() || s.refreshMgr == nil {
		return
	}

	// If shutting down, don't block forever
	select {
	case <-s.refreshMgrCtx.Done():
		return
	default:
	}

	if opts.waitApp {
		s.waitForApp()
	}

	// If we need to cycle Vite, do it AFTER app is ready.
	// Vite's client reconnect will trigger the browser reload.
	if opts.cycleVite {
		s.cycleVite()
	} else if opts.waitVite {
		// cycleVite already waits for Vite internally, hence the else if
		s.waitForVite()
	}

	// Blocking send when alive, but won't deadlock on shutdown
	select {
	case s.refreshMgr.broadcast <- opts.payload:
	case <-s.refreshMgrCtx.Done():
		return
	}
}
