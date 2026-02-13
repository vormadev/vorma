package tooling

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/vormadev/vorma/wave"
)

func (s *server) startRefreshServer(port int) (int, error) {
	if !s.cfg.UsingBrowser() {
		return 0, nil
	}

	mux := newRefreshServerMux(s)

	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		if port > 0 {
			listener, err = net.Listen("tcp", ":0")
		}
		if err != nil {
			return 0, err
		}
	}

	tcpAddress, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		listener.Close()
		return 0, fmt.Errorf("unexpected listener address type: %T", listener.Addr())
	}

	actualPort := tcpAddress.Port
	wave.SetRefreshServerPort(actualPort)

	refreshServer := &http.Server{
		Addr:    ":" + strconv.Itoa(actualPort),
		Handler: mux,
	}
	s.refreshServer = refreshServer

	go func() {
		s.log.Info("Refresh server started", "port", actualPort)
		if err := refreshServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.log.Error("Refresh server error", "error", err)
		}
	}()

	return actualPort, nil
}

func newRefreshServerMux(s *server) *http.ServeMux {
	mux := http.NewServeMux()

	// WebSocket endpoint for live reload
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		websocketHandler(s.refreshMgr, s.refreshMgrCtx)(w, r)
	})

	// Script endpoint for dynamic script loading
	mux.HandleFunc("/get-refresh-script-inner", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/javascript")
		w.Write(
			[]byte(
				wave.RefreshScriptInnerWithParsedConfig(
					wave.GetRefreshServerPort(),
					s.cfg,
				),
			),
		)
	})

	return mux
}

func (s *server) stopRefreshServer() error {
	if s.refreshServer == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.refreshServer.Shutdown(ctx); err != nil {
		return err
	}

	s.refreshServer = nil
	return nil
}
