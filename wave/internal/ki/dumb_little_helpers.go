package ki

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/vormadev/vorma/kit/grace"
)

/////////////////////////////////////////////////////////////////////
/////// PANIC
/////////////////////////////////////////////////////////////////////

func (c *Config) panic(msg string, err error) {
	errMsg := fmt.Sprintf("error: %s: %v", msg, err)
	c.Logger.Error(errMsg)
	panic(err)
}

/////////////////////////////////////////////////////////////////////
/////// IS USING BROWSER
/////////////////////////////////////////////////////////////////////

func (c *Config) is_using_browser() bool {
	return !c._uc.Core.ServerOnlyMode
}

/////////////////////////////////////////////////////////////////////
/////// SETUP BROWSER REFRESH MUX
/////////////////////////////////////////////////////////////////////

func (c *Config) setup_browser_refresh_mux() {
	mux := http.NewServeMux()

	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		websocketHandler(c.browserTabManager)(w, r)
	})

	mux.HandleFunc("/get-refresh-script-inner", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/javascript")
		w.Write([]byte(GetRefreshScriptInner(getRefreshServerPort())))
	})

	server := &http.Server{Addr: ":" + strconv.Itoa(getRefreshServerPort()), Handler: mux}

	shutdownComplete := make(chan struct{})

	go func() {
		c.Logger.Info("Starting sidecar refresh server...", "port", getRefreshServerPort())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			c.panic("Failed to start refresh server", err)
		}
		close(shutdownComplete)
	}()

	<-c._rebuild_cleanup_chan
	c.Logger.Info("Shutting down sidecar refresh server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		c.panic("Failed to shutdown sidecar refresh server", err)
	}

	<-shutdownComplete
	c.Logger.Info("DONE shutting down sidecar refresh server")
}

func (c *Config) kill_browser_refresh_mux() {
	if c._rebuild_cleanup_chan != nil {
		close(c._rebuild_cleanup_chan)
		c._rebuild_cleanup_chan = nil
	}
}

/////////////////////////////////////////////////////////////////////
/////// ADD DIRECTORY TO WATCHER
/////////////////////////////////////////////////////////////////////

func (c *Config) add_directory_to_watcher(path string) error {
	return filepath.Walk(path, func(walkedPath string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking path: %w", err)
		}
		if info.IsDir() {
			if c.get_is_ignored(walkedPath, c.ignoredDirPatterns) {
				return filepath.SkipDir
			}

			// Check if already watching using thread-safe Load
			if _, exists := c.watchedDirs.Load(walkedPath); exists {
				return nil
			}

			err := c.watcher.Add(walkedPath)
			if err != nil {
				return fmt.Errorf("error adding directory to watcher: %w", err)
			}

			// Mark as watched using thread-safe Store
			c.watchedDirs.Store(walkedPath, true)
		}
		return nil
	})
}

/////////////////////////////////////////////////////////////////////
/////// WAIT FOR APP READINESS
/////////////////////////////////////////////////////////////////////

func (c *Config) wait_for_app_readiness() bool {
	return c.wait_for_readiness(fmt.Sprintf(
		"http://localhost:%d%s",
		MustGetAppPort(),
		c._uc.Watch.HealthcheckEndpoint,
	))
}

func (c *Config) wait_for_vite_readiness() bool {
	return c.wait_for_readiness(fmt.Sprintf(
		"http://localhost:%d/@vite/client",
		c._vite_dev_ctx.GetPort(),
	))
}

func (c *Config) wait_for_readiness(url string) bool {
	maxReadinessAttempts := 100
	baseReadinessDelay := 20 * time.Millisecond

	var total_delay time.Duration

	for attempts := range maxReadinessAttempts {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			return true
		}

		delay := baseReadinessDelay + time.Duration(attempts)*baseReadinessDelay

		total_delay += delay
		if total_delay > 3*time.Second {
			return false
		}

		time.Sleep(delay)
	}
	return false
}

/////////////////////////////////////////////////////////////////////
/////// KILL RUNNING GO BINARY
/////////////////////////////////////////////////////////////////////

func (c *Config) kill_running_go_binary() {
	c.dev.mu.Lock()
	defer c.dev.mu.Unlock()
	if c.lastBuildCmd != nil {
		if err := grace.TerminateProcess(c.lastBuildCmd.Process, 5*time.Second, c.Logger); err != nil {
			c.panic("failed to terminate process", err)
		}
		c.Logger.Info("Terminated previous process", "pid", c.lastBuildCmd.Process.Pid)
		c.lastBuildCmd = nil
	}
}

/////////////////////////////////////////////////////////////////////
/////// RUN GO BINARY
/////////////////////////////////////////////////////////////////////

func (c *Config) run_go_binary() {
	c.dev.mu.Lock()
	defer c.dev.mu.Unlock()
	c.lastBuildCmd = exec.Command(c.get_binary_output_path())
	c.lastBuildCmd.Stdout = os.Stdout
	c.lastBuildCmd.Stderr = os.Stderr
	if err := c.lastBuildCmd.Start(); err != nil {
		c.panic("failed to start app binary", err)
	}
	c.Logger.Info("Running app binary...", "pid", c.lastBuildCmd.Process.Pid)
}

/////////////////////////////////////////////////////////////////////
/////// SEND REBUILDING SIGNAL
/////////////////////////////////////////////////////////////////////

func (c *Config) send_rebuilding_signal() {
	if c.is_using_browser() {
		c.browserTabManager.broadcast <- refreshFilePayload{
			ChangeType: changeTypeRebuilding,
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// MUST RELOAD BROADCAST
/////////////////////////////////////////////////////////////////////

type must_reload_broadcast_opts struct {
	wait_for_app  bool
	wait_for_vite bool
	message       string
}

func (c *Config) must_reload_broadcast(rfp refreshFilePayload, opts must_reload_broadcast_opts) {
	if !c.is_using_browser() {
		return
	}
	if opts.wait_for_app {
		if ok := c.wait_for_app_readiness(); !ok {
			c.panic("app server never became ready", nil)
		}
	}
	if opts.wait_for_vite {
		if ok := c.wait_for_vite_readiness(); !ok {
			c.panic("vite never became ready", nil)
		}
	}
	c.Logger.Info(opts.message)
	c.browserTabManager.broadcast <- rfp
}

/////////////////////////////////////////////////////////////////////
/////// GET BINARY OUTPUT PATH
/////////////////////////////////////////////////////////////////////

func (c *Config) get_binary_output_path() string {
	return c._dist.S().Binary.FullPath()
}

/////////////////////////////////////////////////////////////////////
/////// COMPILE GO BINARY
/////////////////////////////////////////////////////////////////////

func (c *Config) compile_go_binary(isDev bool) error {
	a := time.Now()
	c.Logger.Info("Compiling Go binary...")
	buildDest := c.get_binary_output_path()
	in := fmt.Sprintf(".%c%s", filepath.Separator, filepath.Clean(c._uc.Core.MainAppEntry))
	buildCmd := exec.Command("go", "build", "-o", buildDest, in)
	if !isDev {
		buildCmd = exec.Command("go", "build", "-tags=prod", "-o", buildDest, in)
	}
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	err := buildCmd.Run()
	if err != nil {
		return fmt.Errorf("error compiling binary: %w", err)
	}
	c.Logger.Info("DONE compiling Go binary", "duration", time.Since(a))
	return nil
}
