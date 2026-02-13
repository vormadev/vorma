package tooling

import (
	"context"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func websocketHandler(manager *clientManager, ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !shouldAcceptWebsocketConnection(ctx) {
			http.Error(w, "server shutting down", http.StatusServiceUnavailable)
			return
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

		if !tryRegisterWebsocketClient(manager, c, conn, ctx) {
			return
		}

		defer unregisterWebsocketClientOnExit(manager, c, ctx)
		go runWebsocketClientReadLoop(manager, c, conn, ctx)
		runWebsocketClientWriteLoop(c, conn, ctx)
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
	manager *clientManager,
	clientToRegister *client,
	conn *websocket.Conn,
	ctx context.Context,
) bool {
	select {
	case manager.register <- clientToRegister:
		return true
	case <-ctx.Done():
		conn.Close()
		return false
	}
}

func unregisterWebsocketClientOnExit(
	manager *clientManager,
	clientToUnregister *client,
	ctx context.Context,
) {
	select {
	case manager.unregister <- clientToUnregister:
	case <-ctx.Done():
	default:
		// Channel full or closed, manager will clean up
	}
}

func runWebsocketClientReadLoop(
	manager *clientManager,
	c *client,
	conn *websocket.Conn,
	ctx context.Context,
) {
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
}

func runWebsocketClientWriteLoop(
	c *client,
	conn *websocket.Conn,
	ctx context.Context,
) {
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
