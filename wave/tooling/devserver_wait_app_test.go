package tooling

import (
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestWaitForApp_UsesConfiguredHealthcheckEndpoint(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Watch.HealthcheckEndpoint = "/healthz"

	port := wave.MustGetPort()
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Skipf("unable to bind app port %d for waitForApp test: %v", port, err)
	}
	defer listener.Close()

	httpServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}),
	}
	defer httpServer.Close()
	go httpServer.Serve(listener)

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	if !s.waitForApp() {
		t.Fatal("expected waitForApp to return true when healthcheck endpoint is ready")
	}
}
