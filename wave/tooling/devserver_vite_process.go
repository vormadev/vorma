package tooling

import (
	"context"
	"fmt"
	"net/http"
	"strings"
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
	_ = s.cycleViteAndWaitForReadiness()
}

func (s *server) cycleViteAndWaitForReadiness() bool {
	if s == nil || s.cfg == nil || !s.cfg.UsingVite() {
		return false
	}

	s.mu.Lock()
	hasVite := s.viteCtx != nil
	s.mu.Unlock()
	if !hasVite {
		return false
	}

	s.log.Info("Cycling Vite...")
	if err := s.stopVite(); err != nil {
		s.log.Error("stop vite failed during cycle", "error", err)
		return false
	}
	if err := s.startVite(); err != nil {
		s.log.Error("start vite failed during cycle", "error", err)
		return false
	}

	s.mu.Lock()
	hasVite = s.viteCtx != nil
	s.mu.Unlock()
	if !hasVite {
		return false
	}

	if !s.waitForVite() {
		return false
	}

	s.mu.Lock()
	hasVite = s.viteCtx != nil
	s.mu.Unlock()
	if !hasVite {
		return false
	}

	s.log.Info("Vite cycled and ready")
	return true
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
	urls := resolveViteReadyURLs(viteCtx.GetPort())
	ok := s.waitForAnyReady(urls)
	if !ok {
		s.log.Warn(
			"Vite did not become ready in time",
			"urls",
			strings.Join(urls, ", "),
		)
	}
	return ok
}

func resolveViteReadyURL(vitePort int) string {
	return resolveViteReadyURLs(vitePort)[0]
}

func resolveViteReadyURLs(vitePort int) []string {
	return []string{
		resolveReadinessProbeURL(
			localReadinessProbeHostIPv4,
			vitePort,
			"/@vite/client",
		),
		resolveReadinessProbeURL(
			localReadinessProbeHostLocalhost,
			vitePort,
			"/@vite/client",
		),
	}
}
