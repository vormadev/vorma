package tooling

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

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

func (s *server) waitForVite() bool {
	s.mu.Lock()
	viteCtx := s.viteCtx
	s.mu.Unlock()

	if viteCtx == nil {
		return true
	}
	url := resolveViteReadyURL(viteCtx.GetPort())
	ok := s.waitForReady(url)
	if !ok {
		s.log.Warn("Vite did not become ready in time", "url", url)
	}
	return ok
}

func resolveViteReadyURL(vitePort int) string {
	return fmt.Sprintf(
		"http://%s:%d/@vite/client",
		localReadinessProbeHostIPv4,
		vitePort,
	)
}
