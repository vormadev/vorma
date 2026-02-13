package tooling

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/vormadev/vorma/wave"
)

func (s *server) startApp() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd := exec.Command(s.cfg.Dist.Binary())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		s.log.Error("start app failed", "error", err)
		return
	}

	s.appCmd = cmd
	s.log.Info("Started app", "pid", cmd.Process.Pid)
}

func (s *server) stopApp() error {
	s.mu.Lock()
	cmd := s.appCmd
	s.appCmd = nil
	s.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	s.log.Info("Stopping app", "pid", cmd.Process.Pid)
	cmd.Process.Kill()
	cmd.Wait() // reap zombie, ensure port is released
	return nil
}

func (s *server) startVite() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, err := s.builder.NewViteDevContext()
	if err != nil {
		return err
	}
	if ctx == nil {
		return nil
	}

	s.viteCtx = ctx
	return nil
}

func (s *server) stopVite() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.viteCtx != nil {
		s.viteCtx.Cleanup()
		s.viteCtx = nil
	}
	return nil
}

// cycleVite stops and restarts Vite, waiting for it to be ready.
// Called after the Go app is ready so Vite's client reconnect hits a working server.
func (s *server) cycleVite() {
	if !s.cfg.UsingVite() {
		return
	}

	s.mu.Lock()
	hasVite := s.viteCtx != nil
	s.mu.Unlock()

	if !hasVite {
		return
	}

	s.log.Info("Cycling Vite...")
	if err := s.stopVite(); err != nil {
		s.log.Error("stop vite failed during cycle", "error", err)
	}
	if err := s.startVite(); err != nil {
		s.log.Error("start vite failed during cycle", "error", err)
	}
	s.waitForVite()
	s.log.Info("Vite cycled and ready")
}

// callViteFilemapInvalidate calls the Vite plugin's filemap invalidation endpoint.
// This clears the plugin's cached filemap and invalidates all modules, triggering
// a browser reload through Vite's HMR system.
func (s *server) callViteFilemapInvalidate() error {
	s.mu.Lock()
	viteCtx := s.viteCtx
	s.mu.Unlock()

	if viteCtx == nil {
		return fmt.Errorf("vite not running")
	}

	url := fmt.Sprintf("http://localhost:%d/__vorma_invalidate_filemap", viteCtx.GetPort())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("endpoint returned %d", resp.StatusCode)
	}

	s.log.Info("Vite filemap invalidated successfully")
	return nil
}

func (s *server) startRefreshServer(port int) (int, error) {
	if !s.cfg.UsingBrowser() {
		return 0, nil
	}

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

func (s *server) waitForApp() bool {
	url := fmt.Sprintf("http://localhost:%d%s", s.mustGetPort(), s.cfg.HealthcheckEndpoint())
	ok := s.waitForReady(url)
	if !ok {
		s.log.Warn("App did not become ready in time", "url", url)
	}
	return ok
}

func (s *server) waitForVite() bool {
	s.mu.Lock()
	viteCtx := s.viteCtx
	s.mu.Unlock()

	if viteCtx == nil {
		return true
	}
	url := fmt.Sprintf("http://localhost:%d/@vite/client", viteCtx.GetPort())
	ok := s.waitForReady(url)
	if !ok {
		s.log.Warn("Vite did not become ready in time", "url", url)
	}
	return ok
}

func (s *server) waitForReady(url string) bool {
	const maxAttempts = 100
	const baseDelay = 20 * time.Millisecond
	const maxTotal = 10 * time.Second
	const requestTimeout = 500 * time.Millisecond

	client := &http.Client{Timeout: requestTimeout}
	var total time.Duration

	for i := range maxAttempts {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return true
		}
		if resp != nil {
			resp.Body.Close()
		}

		delay := baseDelay + time.Duration(i)*baseDelay
		total += delay

		if total > maxTotal {
			return false
		}

		time.Sleep(delay)
	}

	return false
}

func (s *server) mustGetPort() int {
	if s == nil || s.portResolver == nil {
		return wave.MustGetPort()
	}
	return s.portResolver.MustGetPort()
}

// getBuilder returns the current builder instance safely.
func (s *server) getBuilder() *Builder {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.builder
}

// setBuilder sets the builder instance safely.
func (s *server) setBuilder(b *Builder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.builder = b
}
