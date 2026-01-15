package tooling

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/grace"
	"github.com/vormadev/vorma/kit/netutil"
	"github.com/vormadev/vorma/lab/viteutil"
	"github.com/vormadev/vorma/wave"
	"golang.org/x/sync/errgroup"
)

const defaultRefreshPort = 10000

// restartRequest signals what kind of restart is needed
type restartRequest struct {
	recompileGo     bool
	isConfigRestart bool
}

// server is the dev server instance
type server struct {
	cfg     *wave.ParsedConfig
	log     *slog.Logger
	builder *Builder

	// File watching
	watcher *Watcher

	// Running processes
	mu      sync.Mutex
	appCmd  *exec.Cmd
	viteCtx *viteutil.BuildCtx

	// Browser refresh
	refreshServer    *http.Server
	refreshMgr       *clientManager
	refreshMgrCtx    context.Context
	refreshMgrCancel context.CancelFunc

	// Lifecycle - buffered channel for restart requests
	restartCh   chan restartRequest
	restartChMu sync.Mutex

	// Watcher control - used to delay watcher start until after config restart reload
	watcherStartCh chan struct{}
}

// RunDev starts the development server
func RunDev(cfg *wave.ParsedConfig, log *slog.Logger) error {
	if log == nil {
		log = colorlog.New("wave")
	}

	wave.SetModeToDev()

	s := &server{
		cfg:       cfg,
		log:       log,
		restartCh: make(chan restartRequest, 1),
	}

	return s.run()
}

func (s *server) run() error {
	firstRun := true
	recompileGo := true // First run always compiles
	isConfigRestart := false

	// Initialize refresh server once (crucial -- persists across rebuilds)
	wave.MustGetPort()

	refreshPort, err := netutil.GetFreePort(defaultRefreshPort)
	if err != nil {
		return fmt.Errorf("get refresh port: %w", err)
	}
	wave.SetRefreshServerPort(refreshPort)

	if s.cfg.UsingBrowser() {
		s.refreshMgrCtx, s.refreshMgrCancel = context.WithCancel(context.Background())
		s.refreshMgr = newClientManager()
		go s.refreshMgr.start(s.refreshMgrCtx)
		s.startRefreshServer(refreshPort)
	}

	// Ensure refresh server is cleaned up on exit
	defer s.cleanupRefreshServer()

	for {
		if !firstRun {
			if err := s.reloadConfig(); err != nil {
				s.log.Error("config reload failed", "error", err)
			}
		}

		// Create/recreate builder with current config
		s.builder = NewBuilder(s.cfg, s.log)

		if err := s.initWatcher(); err != nil {
			return fmt.Errorf("init watcher: %w", err)
		}

		isRebuild := !firstRun

		// Run the build of the Builder binary and Server binary in parallel
		var buildEg errgroup.Group

		buildEg.Go(func() error {
			return s.builder.Build(BuildOpts{
				IsDev:     true,
				CompileGo: false,
				IsRebuild: isRebuild,
			})
		})

		if recompileGo {
			buildEg.Go(func() error {
				return s.builder.CompileGoOnly(true)
			})
		}

		if err := buildEg.Wait(); err != nil {
			s.log.Error("build failed", "error", err)
			s.log.Info("Waiting for file changes to retry build...")
			s.waitForBuildRetry()
			firstRun = false
			continue
		}

		// Start Vite AFTER build completes (TypeScript files now exist)
		if s.viteCtx == nil && s.cfg.UsingVite() {
			if err := s.startVite(); err != nil {
				s.log.Error("vite start failed", "error", err)
			}
		}

		// Start the app
		s.startApp()

		// Initialize watcher start channel for this iteration
		s.watcherStartCh = make(chan struct{})

		// Start watching in a goroutine that waits for signal
		go func() {
			<-s.watcherStartCh
			s.runWatcher()
		}()

		// If this was a config restart, broadcast reload after app is ready,
		// THEN signal watcher to start (prevents watcher from triggering reload first)
		if isConfigRestart {
			s.broadcastReload(reloadOpts{
				payload:   refreshPayload{ChangeType: changeTypeOther},
				waitApp:   true,
				waitVite:  true,
				cycleVite: true,
			})
			isConfigRestart = false
		}

		// Now signal watcher to start processing events
		close(s.watcherStartCh)

		firstRun = false

		// Wait for restart request
		req := <-s.restartCh
		recompileGo = req.recompileGo
		isConfigRestart = req.isConfigRestart
		s.log.Info("Restarting dev server...", "recompile_go", recompileGo, "config_restart", isConfigRestart)

		// Send rebuilding signal while refresh server is still alive
		s.broadcastRebuilding()

		// Clean up everything except refresh server and Vite
		s.cleanupForRebuild()
	}
}

// waitForBuildRetry waits for a file change that might fix the build error.
// It starts the watcher and waits for any restart request.
func (s *server) waitForBuildRetry() {
	// Initialize watcher start channel
	s.watcherStartCh = make(chan struct{})

	// Start watcher immediately since we're waiting for fixes
	go func() {
		<-s.watcherStartCh
		s.runWatcher()
	}()
	close(s.watcherStartCh)

	// Wait for any file change to trigger a restart
	<-s.restartCh

	// Clean up for the retry
	s.cleanupForRebuild()
}

func (s *server) initWatcher() error {
	watcher, err := NewWatcher(s.cfg, s.log)
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	s.watcher = watcher

	if err := s.watcher.AddDir(s.cfg.WatchRoot()); err != nil {
		return fmt.Errorf("watch root: %w", err)
	}

	return nil
}

func (s *server) reloadConfig() error {
	configPath := s.cfg.Core.ConfigLocation
	if configPath == "" {
		return nil
	}

	s.log.Info("Reloading config", "path", configPath)
	newCfg, err := wave.ParseConfigFile(configPath)
	if err != nil {
		return err
	}

	// Preserve framework-injected runtime configuration.
	// These are set by frameworks (like Vorma) via AddFrameworkWatchPatterns()
	// and are not persisted in the JSON config file.
	newCfg.FrameworkWatchPatterns = s.cfg.FrameworkWatchPatterns
	newCfg.FrameworkIgnoredPatterns = s.cfg.FrameworkIgnoredPatterns
	newCfg.FrameworkPublicFileMapOutDir = s.cfg.FrameworkPublicFileMapOutDir

	s.cfg = newCfg
	return nil
}

// cleanupForRebuild cleans up resources but keeps refresh server and Vite alive
func (s *server) cleanupForRebuild() {
	if err := s.stopApp(); err != nil {
		s.log.Error("stop app failed", "error", err)
	}

	// Don't stop Vite here - it's cycled in broadcastReload when needed

	if s.watcher != nil {
		if err := s.watcher.Close(); err != nil {
			s.log.Error("close watcher failed", "error", err)
		}
		s.watcher = nil
	}

	if s.builder != nil {
		if err := s.builder.Close(); err != nil {
			s.log.Error("close builder failed", "error", err)
		}
		s.builder = nil
	}
}

// cleanupRefreshServer cleans up the refresh server (called on full shutdown)
func (s *server) cleanupRefreshServer() {
	if err := s.stopRefreshServer(); err != nil {
		s.log.Error("stop refresh server failed", "error", err)
	}

	if s.refreshMgrCancel != nil {
		s.refreshMgrCancel()
		if s.refreshMgr != nil {
			s.refreshMgr.wait()
		}
		s.refreshMgrCancel = nil
	}
}

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
	return grace.TerminateProcess(cmd.Process, 5*time.Second, s.log)
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
	go s.viteCtx.Wait()
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

func (s *server) startRefreshServer(port int) {
	if !s.cfg.UsingBrowser() {
		return
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
		w.Write([]byte(wave.RefreshScriptInner(wave.GetRefreshServerPort())))
	})

	s.refreshServer = &http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: mux,
	}

	go func() {
		s.log.Info("Refresh server started", "port", port)
		if err := s.refreshServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error("Refresh server error", "error", err)
		}
	}()
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

// triggerRestart triggers a restart with Go recompilation
func (s *server) triggerRestart() {
	s.triggerRestartWithOpts(true, false)
}

// triggerRestartNoGo triggers a restart without Go recompilation
func (s *server) triggerRestartNoGo() {
	s.triggerRestartWithOpts(false, false)
}

// triggerConfigRestart triggers a restart due to config file change.
// Config restarts always recompile Go and take precedence over other pending restarts.
func (s *server) triggerConfigRestart() {
	s.triggerRestartWithOpts(true, true)
}

// triggerRestartWithOpts handles restart requests with upgrade semantics.
func (s *server) triggerRestartWithOpts(recompileGo bool, isConfigRestart bool) {
	s.restartChMu.Lock()
	defer s.restartChMu.Unlock()

	req := restartRequest{recompileGo: recompileGo, isConfigRestart: isConfigRestart}

	// Config restarts have absolute priority
	if isConfigRestart {
		// Drain any pending request (we supersede everything)
		select {
		case <-s.restartCh:
		default:
		}
		// Send our config restart (channel is now guaranteed empty)
		s.restartCh <- req
		return
	}

	// Non-config restart: try to send directly
	select {
	case s.restartCh <- req:
		return
	default:
		// Channel is full - there's a pending request
	}

	// Channel full. Check what's pending before deciding to upgrade.
	select {
	case pending := <-s.restartCh:
		if pending.isConfigRestart {
			// Config restart takes absolute priority - put it back unchanged
			s.restartCh <- pending
			s.log.Debug("Dropped restart request: config restart pending",
				"dropped_recompile_go", recompileGo)
			return
		}

		// Pending is not a config restart. Decide whether to upgrade.
		if recompileGo && !pending.recompileGo {
			// We need Go recompile but pending doesn't - upgrade
			s.restartCh <- req
			s.log.Debug("Upgraded pending restart to include Go recompile")
		} else {
			// Pending is at least as strong as us - keep it
			s.restartCh <- pending
		}
	default:
		// Channel became empty (consumer took it) - send ours
		select {
		case s.restartCh <- req:
		default:
			// Someone else sent first - that's fine, a restart is happening
		}
	}
}

func (s *server) waitForApp() bool {
	url := fmt.Sprintf("http://localhost:%d%s", wave.MustGetPort(), s.cfg.HealthcheckEndpoint())
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
