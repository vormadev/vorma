package broadcast

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newDiscardLoggerForBroadcastManagerTests() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTCP4HTTPTestServerForBroadcastManagerTests(
	t *testing.T,
	handler http.Handler,
) *httptest.Server {
	t.Helper()

	listener, listenError := net.Listen("tcp4", "127.0.0.1:0")
	if listenError != nil {
		t.Fatalf("create tcp4 listener: %v", listenError)
	}

	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.Start()
	return server
}

func TestWebsocketHandler_ReturnsServiceUnavailableWhenShuttingDown(
	t *testing.T,
) {
	manager := NewManager(
		newDiscardLoggerForBroadcastManagerTests(),
		ManagerConfig{},
	)
	manager.Close()

	request := httptest.NewRequest(
		http.MethodGet,
		"http://example.com/events",
		nil,
	)
	recorder := httptest.NewRecorder()

	manager.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusServiceUnavailable,
			recorder.Code,
		)
	}
}

func TestClientManager_StartBroadcastAndWait(t *testing.T) {
	manager := NewManager(
		newDiscardLoggerForBroadcastManagerTests(),
		ManagerConfig{},
	)

	runContext, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()

	runDone := make(chan struct{})
	go func() {
		manager.Run(runContext)
		close(runDone)
	}()

	server := newTCP4HTTPTestServerForBroadcastManagerTests(t, manager)
	defer server.Close()

	websocketURL := "ws" + strings.TrimPrefix(server.URL, "http")
	connection, _, dialError := websocket.DefaultDialer.Dial(websocketURL, nil)
	if dialError != nil {
		t.Fatalf("failed dialing websocket handler: %v", dialError)
	}
	defer connection.Close()

	expectedPayload := Payload{
		ChangeType:   ChangeTypeOther,
		NormalCSSURL: "/styles.css",
	}

	var receivedPayload Payload
	payloadDelivered := false
	for attemptIndex := 0; attemptIndex < 10; attemptIndex++ {
		manager.Broadcast(expectedPayload)

		connection.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
		readError := connection.ReadJSON(&receivedPayload)
		if readError == nil {
			payloadDelivered = true
			break
		}
	}
	if !payloadDelivered {
		t.Fatal("timed out waiting for broadcast payload on websocket")
	}

	if receivedPayload.ChangeType != expectedPayload.ChangeType ||
		receivedPayload.NormalCSSURL != expectedPayload.NormalCSSURL {
		t.Fatalf(
			"unexpected payload over websocket: got=%#v want=%#v",
			receivedPayload,
			expectedPayload,
		)
	}

	cancelRun()

	select {
	case <-runDone:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for client manager shutdown")
	}
}

func TestClientManager_DrainChannels(t *testing.T) {
	manager := NewManager(
		newDiscardLoggerForBroadcastManagerTests(),
		ManagerConfig{},
	)

	websocketServer := newTCP4HTTPTestServerForBroadcastManagerTests(
		t,
		http.HandlerFunc(func(
			responseWriter http.ResponseWriter,
			request *http.Request,
		) {
			upgrader := &websocket.Upgrader{
				CheckOrigin: func(*http.Request) bool { return true },
			}
			connection, upgradeError := upgrader.Upgrade(
				responseWriter,
				request,
				nil,
			)
			if upgradeError != nil {
				return
			}
			defer connection.Close()
			for {
				if _, _, readError := connection.ReadMessage(); readError != nil {
					return
				}
			}
		}),
	)
	defer websocketServer.Close()

	websocketURL := "ws" + strings.TrimPrefix(websocketServer.URL, "http")
	connectionA, _, dialErrorA := websocket.DefaultDialer.Dial(websocketURL, nil)
	if dialErrorA != nil {
		t.Fatalf("failed dialing ws server (A): %v", dialErrorA)
	}
	defer connectionA.Close()

	connectionB, _, dialErrorB := websocket.DefaultDialer.Dial(websocketURL, nil)
	if dialErrorB != nil {
		t.Fatalf("failed dialing ws server (B): %v", dialErrorB)
	}
	defer connectionB.Close()

	manager.registerQueue <- &clientConnection{
		conn:      connectionA,
		sendQueue: make(chan Payload, 1),
	}
	manager.unregisterQueue <- &clientConnection{
		conn:      connectionB,
		sendQueue: make(chan Payload, 1),
	}
	manager.broadcastQueue <- Payload{ChangeType: ChangeTypeRebuilding}

	manager.Close()

	if len(manager.registerQueue) != 0 ||
		len(manager.unregisterQueue) != 0 ||
		len(manager.broadcastQueue) != 0 {
		t.Fatalf(
			"expected all manager channels to be drained, lengths: register=%d unregister=%d broadcast=%d",
			len(manager.registerQueue),
			len(manager.unregisterQueue),
			len(manager.broadcastQueue),
		)
	}
}

func TestPayloadJSON_CriticalChangeIncludesCriticalCSSFieldWhenEmpty(
	t *testing.T,
) {
	payloadJSON, marshalError := json.Marshal(
		Payload{
			ChangeType:  ChangeTypeCriticalCSS,
			CriticalCSS: "",
		},
	)
	if marshalError != nil {
		t.Fatalf("marshal payload: %v", marshalError)
	}

	payloadJSONString := string(payloadJSON)
	if !strings.Contains(payloadJSONString, `"criticalCSS":""`) {
		t.Fatalf(
			"expected critical css field to be serialized for critical payloads, got %s",
			payloadJSONString,
		)
	}
}
