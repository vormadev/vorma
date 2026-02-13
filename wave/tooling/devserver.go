package tooling

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"sync"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/lab/vitecmd"
	"github.com/vormadev/vorma/wave"
)

const defaultRefreshPort = 10000

// restartRequest signals what kind of restart is needed
type restartRequest struct {
	recompileGo     bool
	isConfigRestart bool
}

// server is the dev server instance
type server struct {
	cfg *wave.ParsedConfig
	log *slog.Logger

	portResolver *wave.PortResolver

	// File watching
	watcher *Watcher

	// Running processes
	mu      sync.Mutex
	appCmd  *exec.Cmd
	viteCtx *vitecmd.BuildCtx
	builder *Builder

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

	// Validate config before starting dev server
	if err := ValidateConfig(cfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Acquire project-level lock before doing anything else.
	// This prevents multiple wave dev instances on the same project.
	lock := newDevLock(cfg.Dist.Static())
	if err := lock.acquire(); err != nil {
		return fmt.Errorf("cannot start dev server: %w", err)
	}
	defer lock.release()

	s := &server{
		cfg:          cfg,
		log:          log,
		portResolver: wave.NewPortResolver(),
		restartCh:    make(chan restartRequest, 1),
	}

	return s.run()
}
