package tooling

import "context"

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
