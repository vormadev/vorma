package tooling

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"sync"

	"github.com/vormadev/vorma/internal/waveport"
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
	cfg          *wave.ParsedConfig
	log          *slog.Logger
	portResolver *waveport.Resolver

	// File watching
	watcher *watcher

	// Running processes
	mu                sync.Mutex
	appCmd            *exec.Cmd
	appProcessManager *appProcessManager
	viteCtx           *vitecmd.BuildCtx
	builder           *Builder

	// Browser refresh
	refreshServer    *http.Server
	refreshMgr       *clientManager
	refreshMgrCtx    context.Context
	refreshMgrCancel context.CancelFunc

	// Lifecycle restart intents
	restartIntents *restartIntentAccumulator

	// Concurrent-no-wait hook execution gate
	concurrentNoWaitHookExecutionLimiter         chan struct{}
	concurrentNoWaitHookExecutionLimiterInitOnce sync.Once
	concurrentNoWaitHookLifecycleCtx             context.Context
	concurrentNoWaitHookLifecycleCancel          context.CancelFunc

	// watcher control - used to delay watcher start until after config restart reload
	watcherStartCh chan struct{}

	// Cycle-scoped async lifecycle management
	nextRunCycleID       uint64
	currentRunCycleScope *runCycleScope

	// Trace correlation for watcher batches and hook-stage logs
	nextWatcherBatchID                  uint64
	currentWatcherExecutionTraceContext watcherExecutionTraceContext
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
		portResolver: waveport.NewResolver(),
		concurrentNoWaitHookExecutionLimiter: make(
			chan struct{},
			maxConcurrentNoWaitHookExecutions,
		),
	}
	s.restartIntents = newRestartIntentAccumulator(make(chan restartRequest, 1))

	return s.run()
}
