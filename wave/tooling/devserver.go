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

type goCompilationOrderingPolicy string

const (
	goCompilationOrderingPolicyConcurrentWithBuildHooks goCompilationOrderingPolicy = "compile_concurrently_with_build_hooks"
	goCompilationOrderingPolicyAfterBuildHooks          goCompilationOrderingPolicy = "compile_after_build_hooks"
	goCompilationOrderingPolicyNotRequested             goCompilationOrderingPolicy = "compile_not_requested"
)

type runBuildExecutionOrderingDecision struct {
	goCompilationOrderingPolicy goCompilationOrderingPolicy
	runCompileInParallel        bool
	runCompileAfterBuildHooks   bool
}

func deriveRunBuildExecutionOrderingDecision(
	shouldRecompileGo bool,
	sequentialGoBuild bool,
) runBuildExecutionOrderingDecision {
	if !shouldRecompileGo {
		return runBuildExecutionOrderingDecision{
			goCompilationOrderingPolicy: goCompilationOrderingPolicyNotRequested,
		}
	}

	if sequentialGoBuild {
		return runBuildExecutionOrderingDecision{
			goCompilationOrderingPolicy: goCompilationOrderingPolicyAfterBuildHooks,
			runCompileAfterBuildHooks:   true,
		}
	}

	return runBuildExecutionOrderingDecision{
		goCompilationOrderingPolicy: goCompilationOrderingPolicyConcurrentWithBuildHooks,
		runCompileInParallel:        true,
	}
}

type runCycleScope struct {
	cycleID uint64

	executionContext       context.Context
	cancelExecutionContext context.CancelFunc

	asyncWorkGroup sync.WaitGroup
}

func newRunCycleScope(
	cycleID uint64,
) *runCycleScope {
	executionContext, cancelExecutionContext := context.WithCancel(
		context.Background(),
	)
	return &runCycleScope{
		cycleID:                cycleID,
		executionContext:       executionContext,
		cancelExecutionContext: cancelExecutionContext,
	}
}

func (scope *runCycleScope) launchAsyncWork(
	runAsyncWork func(context.Context),
) {
	if scope == nil || runAsyncWork == nil {
		return
	}

	scope.asyncWorkGroup.Add(1)
	go func() {
		defer scope.asyncWorkGroup.Done()
		runAsyncWork(scope.executionContext)
	}()
}

func (scope *runCycleScope) cancelAndJoin() {
	if scope == nil {
		return
	}

	if scope.cancelExecutionContext != nil {
		scope.cancelExecutionContext()
	}
	scope.asyncWorkGroup.Wait()
}

func (s *server) startRunCycleScope() *runCycleScope {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	s.nextRunCycleID++
	cycleScope := newRunCycleScope(s.nextRunCycleID)
	s.currentRunCycleScope = cycleScope
	s.mu.Unlock()

	return cycleScope
}

func (s *server) getCurrentRunCycleScope() *runCycleScope {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	cycleScope := s.currentRunCycleScope
	s.mu.Unlock()
	return cycleScope
}

func (s *server) currentRunCycleContextOrBackground() context.Context {
	currentRunCycleScope := s.getCurrentRunCycleScope()
	if currentRunCycleScope == nil || currentRunCycleScope.executionContext == nil {
		return context.Background()
	}
	return currentRunCycleScope.executionContext
}

func (s *server) cancelAndJoinCurrentRunCycleScope() {
	if s == nil {
		return
	}

	s.mu.Lock()
	currentRunCycleScope := s.currentRunCycleScope
	s.currentRunCycleScope = nil
	s.mu.Unlock()

	if currentRunCycleScope != nil {
		currentRunCycleScope.cancelAndJoin()
	}
}

func (s *server) launchRunCycleScopedAsyncWorkOrDetached(
	runAsyncWork func(context.Context),
) {
	if runAsyncWork == nil {
		return
	}

	currentRunCycleScope := s.getCurrentRunCycleScope()
	if currentRunCycleScope != nil {
		currentRunCycleScope.launchAsyncWork(runAsyncWork)
		return
	}

	go runAsyncWork(context.Background())
}

type watcherExecutionTraceContext struct {
	cycleID uint64
	batchID uint64
}

func (s *server) deriveWatcherExecutionTraceContext() watcherExecutionTraceContext {
	if s == nil {
		return watcherExecutionTraceContext{}
	}

	s.mu.Lock()
	s.nextWatcherBatchID++
	nextBatchID := s.nextWatcherBatchID
	currentRunCycleScope := s.currentRunCycleScope
	s.mu.Unlock()

	currentCycleID := uint64(0)
	if currentRunCycleScope != nil {
		currentCycleID = currentRunCycleScope.cycleID
	}

	return watcherExecutionTraceContext{
		cycleID: currentCycleID,
		batchID: nextBatchID,
	}
}

func (s *server) setCurrentWatcherExecutionTraceContext(
	traceContextForWatcherExecution watcherExecutionTraceContext,
) {
	if s == nil {
		return
	}

	s.mu.Lock()
	s.currentWatcherExecutionTraceContext = traceContextForWatcherExecution
	s.mu.Unlock()
}

func (s *server) clearCurrentWatcherExecutionTraceContext() {
	if s == nil {
		return
	}

	s.mu.Lock()
	s.currentWatcherExecutionTraceContext = watcherExecutionTraceContext{}
	s.mu.Unlock()
}

func (s *server) getCurrentWatcherExecutionTraceContext() watcherExecutionTraceContext {
	if s == nil {
		return watcherExecutionTraceContext{}
	}

	s.mu.Lock()
	traceContextForWatcherExecution := s.currentWatcherExecutionTraceContext
	s.mu.Unlock()
	return traceContextForWatcherExecution
}
