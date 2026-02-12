package tooling

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newTCP4HTTPTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create tcp4 listener: %v", err)
	}

	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.Start()
	return server
}

func TestWebsocketHandler_ReturnsServiceUnavailableWhenShuttingDown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler := websocketHandler(newClientManager(), ctx)
	request := httptest.NewRequest(http.MethodGet, "http://example.com/events", nil)
	recorder := httptest.NewRecorder()

	handler(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}
}

func TestClientManager_StartBroadcastAndWait(t *testing.T) {
	manager := newClientManager()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.start(ctx)

	server := newTCP4HTTPTestServer(t, websocketHandler(manager, ctx))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed dialing websocket handler: %v", err)
	}
	defer conn.Close()

	want := refreshPayload{
		ChangeType:   changeTypeOther,
		NormalCSSURL: "/styles.css",
	}

	var got refreshPayload
	delivered := false
	for i := 0; i < 10; i++ {
		manager.broadcast <- want

		conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
		readErr := conn.ReadJSON(&got)
		if readErr == nil {
			delivered = true
			break
		}
	}

	if !delivered {
		t.Fatal("timed out waiting for broadcast payload on websocket")
	}

	if got.ChangeType != want.ChangeType || got.NormalCSSURL != want.NormalCSSURL {
		t.Fatalf("unexpected payload over websocket: got=%#v want=%#v", got, want)
	}

	cancel()

	done := make(chan struct{})
	go func() {
		manager.wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for client manager shutdown")
	}
}

func TestClientManager_DrainChannels(t *testing.T) {
	manager := newClientManager()
	manager.broadcast = make(chan refreshPayload, 1)

	wsServer := newTCP4HTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer wsServer.Close()

	wsURL := "ws" + strings.TrimPrefix(wsServer.URL, "http")

	connA, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed dialing ws server (A): %v", err)
	}
	defer connA.Close()

	connB, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed dialing ws server (B): %v", err)
	}
	defer connB.Close()

	manager.register <- &client{id: "a", conn: connA, notify: make(chan refreshPayload, 1)}
	manager.unregister <- &client{id: "b", conn: connB, notify: make(chan refreshPayload, 1)}
	manager.broadcast <- refreshPayload{ChangeType: changeTypeRebuilding}

	manager.drainChannels()

	if len(manager.register) != 0 || len(manager.unregister) != 0 || len(manager.broadcast) != 0 {
		t.Fatalf(
			"expected all manager channels to be drained, lengths: register=%d unregister=%d broadcast=%d",
			len(manager.register),
			len(manager.unregister),
			len(manager.broadcast),
		)
	}
}
