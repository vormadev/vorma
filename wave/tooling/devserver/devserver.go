package devserver

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/executil"
	"github.com/vormadev/vorma/lab/vitecmd"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/waveshared"
	"github.com/vormadev/vorma/wave/tooling/broadcast"
	"github.com/vormadev/vorma/wave/tooling/builder"
	"github.com/vormadev/vorma/wave/tooling/devserver/devserverengine"
	"github.com/vormadev/vorma/wave/tooling/devserver/devserverruntime"
	"github.com/vormadev/vorma/wave/tooling/toolingshared"
	"github.com/vormadev/vorma/wave/tooling/watch"
	"github.com/vormadev/vorma/wave/tooling/watch/classification"
	"github.com/vormadev/vorma/wave/tooling/watch/dedup"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultRefreshPort = 10000

// restartRequest signals what kind of restart is needed
type restartRequest = devserverengine.RestartRequest

// Server is the dev Server instance
type Server struct {
	Cfg          *wave.ParsedConfig
	Log          *slog.Logger
	PortResolver *waveshared.Resolver

	// File watching
	Watcher *watch.Watcher

	// Running processes
	Mu                sync.Mutex
	AppCmd            *exec.Cmd
	AppProcessManager *devserverruntime.AppProcessManager
	ViteCtx           *vitecmd.BuildCtx
	Builder           *toolingbuilder.Builder

	// Browser refresh
	RefreshServer    *http.Server
	RefreshMgr       *broadcast.Manager
	RefreshMgrCtx    context.Context
	RefreshMgrCancel context.CancelFunc

	// Lifecycle restart intents
	RestartIntents *devserverengine.RestartIntentAccumulator

	// Concurrent-no-wait hook execution gate
	ConcurrentNoWaitHookExecutionLimiter         chan struct{}
	ConcurrentNoWaitHookExecutionLimiterInitOnce sync.Once
	ConcurrentNoWaitHookLifecycleCtx             context.Context
	ConcurrentNoWaitHookLifecycleCancel          context.CancelFunc

	// watcher control - used to delay watcher start until after config restart reload
	WatcherStartCh chan struct{}

	// Cycle-scoped async lifecycle management
	NextRunCycleID       uint64
	CurrentRunCycleScope *devserverengine.RunCycleScope

	// Trace correlation for watcher batches and hook-stage logs
	NextWatcherBatchID                  uint64
	CurrentWatcherExecutionTraceContext WatcherExecutionTraceContext
}

// RunDev starts the development Server
func RunDev(cfg *wave.ParsedConfig, log *slog.Logger) error {
	if log == nil {
		log = colorlog.New("wave")
	}

	wave.SetModeToDev()

	if err := toolingbuilder.ValidateConfig(cfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	lock := toolingshared.NewDevLock(cfg.Dist.Static())
	if err := lock.Acquire(); err != nil {
		return fmt.Errorf("cannot start dev Server: %w", err)
	}
	defer lock.Release()

	s := &Server{
		Cfg:          cfg,
		Log:          log,
		PortResolver: waveshared.NewResolver(),
		ConcurrentNoWaitHookExecutionLimiter: make(
			chan struct{},
			maxConcurrentNoWaitHookExecutions,
		),
	}
	s.RestartIntents = devserverengine.NewRestartIntentAccumulator(make(chan devserverengine.RestartRequest, 1))

	return s.Run()
}

// GetBuilder returns the current builder instance safely.
func (s *Server) GetBuilder() *toolingbuilder.Builder {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return s.Builder
}

// SetBuilder sets the builder instance safely.
func (s *Server) SetBuilder(b *toolingbuilder.Builder) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.Builder = b
}

type goCompilationOrderingPolicy = devserverengine.GoCompilationOrderingPolicy

const (
	goCompilationOrderingPolicyConcurrentWithBuildHooks = devserverengine.GoCompilationOrderingPolicyConcurrentWithBuildHooks
	goCompilationOrderingPolicyAfterBuildHooks          = devserverengine.GoCompilationOrderingPolicyAfterBuildHooks
	goCompilationOrderingPolicyNotRequested             = devserverengine.GoCompilationOrderingPolicyNotRequested
)

type runBuildExecutionOrderingDecision = devserverengine.RunBuildExecutionOrderingDecision
type runCycleScope = devserverengine.RunCycleScope

func deriveRunBuildExecutionOrderingDecision(
	shouldRecompileGo bool,
	sequentialGoBuild bool,
) runBuildExecutionOrderingDecision {
	return devserverengine.DeriveRunBuildExecutionOrderingDecision(
		shouldRecompileGo,
		sequentialGoBuild,
	)
}

func newRunCycleScope(
	cycleID uint64,
) *runCycleScope {
	return devserverengine.NewRunCycleScope(cycleID)
}

func (s *Server) StartRunCycleScope() *devserverengine.RunCycleScope {
	if s == nil {
		return nil
	}

	s.Mu.Lock()
	s.NextRunCycleID++
	cycleScope := devserverengine.NewRunCycleScope(s.NextRunCycleID)
	s.CurrentRunCycleScope = cycleScope
	s.Mu.Unlock()

	return cycleScope
}

func (s *Server) GetCurrentRunCycleScope() *devserverengine.RunCycleScope {
	if s == nil {
		return nil
	}

	s.Mu.Lock()
	cycleScope := s.CurrentRunCycleScope
	s.Mu.Unlock()
	return cycleScope
}

func (s *Server) CurrentRunCycleContextOrBackground() context.Context {
	currentRunCycleScope := s.GetCurrentRunCycleScope()
	if currentRunCycleScope == nil || currentRunCycleScope.ExecutionContext == nil {
		return context.Background()
	}
	return currentRunCycleScope.ExecutionContext
}

func (s *Server) CancelAndJoinCurrentRunCycleScope() {
	if s == nil {
		return
	}

	s.Mu.Lock()
	currentRunCycleScope := s.CurrentRunCycleScope
	s.CurrentRunCycleScope = nil
	s.Mu.Unlock()

	if currentRunCycleScope != nil {
		currentRunCycleScope.CancelAndJoin()
	}
}

func (s *Server) LaunchRunCycleScopedAsyncWorkOrDetached(
	runAsyncWork func(context.Context),
) {
	if runAsyncWork == nil {
		return
	}

	currentRunCycleScope := s.GetCurrentRunCycleScope()
	if currentRunCycleScope != nil {
		currentRunCycleScope.LaunchAsyncWork(runAsyncWork)
		return
	}

	go runAsyncWork(context.Background())
}

type WatcherExecutionTraceContext struct {
	CycleID uint64
	BatchID uint64
}

func (s *Server) DeriveWatcherExecutionTraceContext() WatcherExecutionTraceContext {
	if s == nil {
		return WatcherExecutionTraceContext{}
	}

	s.Mu.Lock()
	s.NextWatcherBatchID++
	nextBatchID := s.NextWatcherBatchID
	currentRunCycleScope := s.CurrentRunCycleScope
	s.Mu.Unlock()

	currentCycleID := uint64(0)
	if currentRunCycleScope != nil {
		currentCycleID = currentRunCycleScope.CycleID
	}

	return WatcherExecutionTraceContext{
		CycleID: currentCycleID,
		BatchID: nextBatchID,
	}
}

func (s *Server) SetCurrentWatcherExecutionTraceContext(
	traceContextForWatcherExecution WatcherExecutionTraceContext,
) {
	if s == nil {
		return
	}

	s.Mu.Lock()
	s.CurrentWatcherExecutionTraceContext = traceContextForWatcherExecution
	s.Mu.Unlock()
}

func (s *Server) ClearCurrentWatcherExecutionTraceContext() {
	if s == nil {
		return
	}

	s.Mu.Lock()
	s.CurrentWatcherExecutionTraceContext = WatcherExecutionTraceContext{}
	s.Mu.Unlock()
}

func (s *Server) GetCurrentWatcherExecutionTraceContext() WatcherExecutionTraceContext {
	if s == nil {
		return WatcherExecutionTraceContext{}
	}

	s.Mu.Lock()
	traceContextForWatcherExecution := s.CurrentWatcherExecutionTraceContext
	s.Mu.Unlock()
	return traceContextForWatcherExecution
}

const defaultAppProcessGracefulStopTimeout = devserverruntime.DefaultAppProcessGracefulStopTimeout

func (s *Server) EnsureAppProcessManager() *devserverruntime.AppProcessManager {
	if s == nil {
		return nil
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	if s.AppProcessManager == nil {
		s.AppProcessManager = devserverruntime.NewAppProcessManager()
	}
	return s.AppProcessManager
}

func (s *Server) StartApp() {
	manager := s.EnsureAppProcessManager()
	if manager == nil {
		return
	}

	cmd, err := manager.StartApp(s.Cfg.Dist.Binary())
	if err != nil {
		s.Log.Error("start app failed", "error", err)
		return
	}

	s.Mu.Lock()
	s.AppCmd = cmd
	s.Mu.Unlock()
	s.Log.Info("Started app", "pid", cmd.Process.Pid)
}

func (s *Server) StopApp() error {
	manager := s.EnsureAppProcessManager()
	if manager == nil {
		return nil
	}

	s.Mu.Lock()
	cmd := s.AppCmd
	s.AppCmd = nil
	s.Mu.Unlock()

	return manager.StopApp(cmd)
}

func shouldIgnoreProcessTerminationError(
	processTerminationError error,
) bool {
	return devserverruntime.ShouldIgnoreProcessTerminationError(processTerminationError)
}

func shouldIgnoreProcessWaitError(processWaitError error) bool {
	return devserverruntime.ShouldIgnoreProcessWaitError(processWaitError)
}
func (s *Server) InitWatcher() error {
	watcher, err := watch.NewWatcher(s.Cfg, s.Log)
	if err != nil {
		return fmt.Errorf("create Watcher: %w", err)
	}

	s.Mu.Lock()
	s.Watcher = watcher
	s.Mu.Unlock()

	if err := watcher.AddDir(s.Cfg.WatchRoot()); err != nil {
		return fmt.Errorf("watch root: %w", err)
	}

	if err := s.AddConfigFileDirectory(watcher, s.Cfg.Core.ConfigLocation); err != nil {
		return fmt.Errorf("watch config file directory: %w", err)
	}

	return nil
}

func (s *Server) AddConfigFileDirectory(
	watcher *watch.Watcher,
	configFilePath string,
) error {
	normalizedConfigDirectoryPath := waveshared.AbsoluteDirectory(configFilePath)
	if normalizedConfigDirectoryPath == "" {
		return nil
	}

	if err := watcher.AddDir(normalizedConfigDirectoryPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (s *Server) ReloadConfig() error {
	newCfg, err := s.LoadParsedConfigForReload()
	if err != nil {
		return err
	}
	if newCfg == nil {
		return nil
	}

	if err := toolingbuilder.ValidateConfig(newCfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	newCfg.CopyFrameworkRuntimeFieldsFrom(s.Cfg)

	s.Cfg = newCfg
	return nil
}

func (s *Server) LoadParsedConfigForReload() (*wave.ParsedConfig, error) {
	configFilePath := waveshared.Absolute(s.Cfg.Core.ConfigLocation)
	if configFilePath == "" {
		return nil, nil
	}

	s.Log.Info("Reloading config", "path", configFilePath)
	newCfg, err := wave.ParseConfigFile(configFilePath)
	if err != nil {
		return nil, err
	}

	return newCfg, nil
}

type readinessWaitPolicy = devserverruntime.ReadinessWaitPolicy

const localReadinessProbeHostIPv4 = devserverruntime.LocalReadinessProbeHostIPv4
const localReadinessProbeHostLocalhost = devserverruntime.LocalReadinessProbeHostLocalhost

func defaultReadinessWaitPolicy() readinessWaitPolicy {
	return devserverruntime.DefaultReadinessWaitPolicy()
}

func (s *Server) WaitForApp() bool {
	url := ResolveAppReadyURL(s.MustGetPort(), s.Cfg.HealthcheckEndpoint())
	ok := s.WaitForReady(url)
	if !ok {
		s.Log.Warn("App did not become ready in time", "url", url)
	}
	return ok
}

func ResolveAppReadyURL(appPort int, healthcheckEndpoint string) string {
	return devserverruntime.ResolveAppReadyURL(
		appPort,
		healthcheckEndpoint,
	)
}

func (s *Server) WaitForReady(url string) bool {
	return s.WaitForAnyReady([]string{url})
}

func (s *Server) WaitForAnyReady(urls []string) bool {
	policy := defaultReadinessWaitPolicy()
	return devserverruntime.WaitForAnyReady(urls, policy)
}

func resolveReadinessProbeURL(
	host string,
	port int,
	endpoint string,
) string {
	return devserverruntime.ResolveReadinessProbeURL(host, port, endpoint)
}

func DeriveReadinessWaitDelay(
	attemptIndex int,
	baseDelay time.Duration,
) time.Duration {
	return devserverruntime.DeriveReadinessWaitDelay(attemptIndex, baseDelay)
}

func ShouldContinueReadinessWait(
	total time.Duration,
	maxTotal time.Duration,
) bool {
	return devserverruntime.ShouldContinueReadinessWait(total, maxTotal)
}

// MustGetPort returns the app runtime port for devserver orchestration.
// It panics in dev mode if a free port cannot be resolved.
// It panics in non-dev mode when PORT is missing or invalid.
func (s *Server) MustGetPort() int {
	if s == nil || s.PortResolver == nil {
		return wave.MustGetPort()
	}
	return s.PortResolver.MustGetPort()
}
func (s *Server) StartRefreshServer(port int) (int, error) {
	if !s.Cfg.UsingBrowser() {
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
		return 0, fmt.Errorf(
			"unexpected listener address type: %T",
			listener.Addr(),
		)
	}

	actualPort := tcpAddress.Port
	wave.SetRefreshServerPort(actualPort)

	refreshServer := &http.Server{
		Addr:    ":" + strconv.Itoa(actualPort),
		Handler: mux,
	}
	s.RefreshServer = refreshServer

	go func() {
		s.Log.Info("Refresh Server started", "port", actualPort)
		if err := refreshServer.Serve(listener); err != nil &&
			err != http.ErrServerClosed {
			s.Log.Error("Refresh Server error", "error", err)
		}
	}()

	return actualPort, nil
}

func newRefreshServerMux(s *Server) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		broadcast.WebsocketHandler(s.RefreshMgr, s.RefreshMgrCtx)(w, r)
	})

	mux.HandleFunc(
		"/get-refresh-script-inner",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Content-Type", "text/javascript")
			w.Write(
				[]byte(
					wave.RefreshScriptInnerWithParsedConfig(
						wave.GetRefreshServerPort(),
						s.Cfg,
					),
				),
			)
		},
	)

	return mux
}

func (s *Server) StopRefreshServer() error {
	if s.RefreshServer == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.RefreshServer.Shutdown(ctx); err != nil {
		return err
	}

	s.RefreshServer = nil
	return nil
}

type restartIntentAccumulator = devserverengine.RestartIntentAccumulator

func newRestartIntentAccumulator(
	restartRequests chan restartRequest,
) *restartIntentAccumulator {
	return devserverengine.NewRestartIntentAccumulator(restartRequests)
}

func (s *Server) GetOrCreateRestartIntentAccumulator() *restartIntentAccumulator {
	if s == nil {
		return nil
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	if s.RestartIntents == nil {
		s.RestartIntents = newRestartIntentAccumulator(make(chan restartRequest, 1))
	}
	return s.RestartIntents
}

func (s *Server) SetWaitingForBuildRetry(waitingForBuildRetry bool) {
	if s == nil {
		return
	}

	accumulator := s.GetOrCreateRestartIntentAccumulator()
	if accumulator != nil {
		accumulator.SetWaitingForBuildRetry(waitingForBuildRetry)
	}
}

func (s *Server) QueueRestartRequest(
	restartRequestForQueue restartRequest,
) {
	accumulator := s.GetOrCreateRestartIntentAccumulator()
	if accumulator == nil {
		return
	}
	accumulator.QueueRestartRequest(restartRequestForQueue)
}

func (s *Server) ConsumePendingRestartRequest() (restartRequest, bool) {
	accumulator := s.GetOrCreateRestartIntentAccumulator()
	if accumulator == nil {
		return restartRequest{}, false
	}
	return accumulator.ConsumePendingRestartRequest()
}

func (s *Server) ConsumeRestartRequestBlocking() restartRequest {
	accumulator := s.GetOrCreateRestartIntentAccumulator()
	if accumulator == nil {
		return restartRequest{}
	}
	return accumulator.ConsumeRestartRequestBlocking()
}

// TriggerRestart triggers a restart with Go recompilation
func (s *Server) TriggerRestart() {
	s.TriggerRestartWithOpts(true, false)
}

// TriggerRestartNoGo triggers a restart without Go recompilation
func (s *Server) TriggerRestartNoGo() {
	s.TriggerRestartWithOpts(false, false)
}

// TriggerConfigRestart triggers a restart due to config file change.
// Config restarts always recompile Go and take precedence over other pending restarts.
func (s *Server) TriggerConfigRestart() {
	s.TriggerRestartWithOpts(true, true)
}

// TriggerRestartWithOpts handles restart requests with upgrade semantics.
func (s *Server) TriggerRestartWithOpts(recompileGo bool, isConfigRestart bool) {
	incomingRequest := normalizeRestartRequest(restartRequest{
		RecompileGo:     recompileGo,
		IsConfigRestart: isConfigRestart,
	})
	s.QueueRestartRequest(incomingRequest)
}

func normalizeRestartRequest(request restartRequest) restartRequest {
	return devserverengine.NormalizeRestartRequest(request)
}

func resolveQueuedRestartRequest(
	pendingRequest *restartRequest,
	incomingRequest restartRequest,
) restartRequest {
	return devserverengine.ResolveQueuedRestartRequest(
		pendingRequest,
		incomingRequest,
	)
}

func tryEnqueueRestartRequest(
	restartRequests chan restartRequest,
	request restartRequest,
) bool {
	return devserverengine.TryEnqueueRestartRequest(restartRequests, request)
}

func tryDequeueRestartRequest(
	restartRequests chan restartRequest,
) (restartRequest, bool) {
	return devserverengine.TryDequeueRestartRequest(restartRequests)
}

func mergeRestartRequests(
	pendingRequest restartRequest,
	incomingRequest restartRequest,
) restartRequest {
	return devserverengine.MergeRestartRequests(pendingRequest, incomingRequest)
}

type runIntent = devserverengine.RunIntent

const (
	runLifecycleCommandPrepareCycle        = devserverengine.RunLifecycleCommandPrepareCycle
	runLifecycleCommandBuildCycle          = devserverengine.RunLifecycleCommandBuildCycle
	runLifecycleCommandAwaitBuildRetry     = devserverengine.RunLifecycleCommandAwaitBuildRetry
	runLifecycleCommandStartRuntime        = devserverengine.RunLifecycleCommandStartRuntime
	runLifecycleCommandAwaitRestartRequest = devserverengine.RunLifecycleCommandAwaitRestartRequest
	runLifecycleCommandCleanupForNextCycle = devserverengine.RunLifecycleCommandCleanupForNextCycle
)

type runLifecycleCommand = devserverengine.RunLifecycleCommand

type runLifecycleCommandInput = devserverengine.RunLifecycleCommandInput
type runLifecycleCommandResult = devserverengine.RunLifecycleCommandResult

func DeriveRunIntentFromRestartRequest(
	restartRequestForIntent restartRequest,
) runIntent {
	return devserverengine.DeriveRunIntentFromRestartRequest(
		restartRequestForIntent,
	)
}

func (s *Server) Run() error {
	wave.SetModeToDev()

	firstRun := true
	currentRunIntent := runIntent{
		RecompileGo: true,
	}
	currentRunLifecycleState := RunLifecycleStatePreparingCycle
	currentLifecycleCycleID := uint64(1)

	s.MustGetPort()

	if s.Cfg.UsingBrowser() {
		s.RefreshMgrCtx, s.RefreshMgrCancel = context.WithCancel(
			context.Background(),
		)
		s.RefreshMgr = broadcast.NewManager()
		go s.RefreshMgr.Start(s.RefreshMgrCtx)
		if _, err := s.StartRefreshServer(defaultRefreshPort); err != nil {
			return fmt.Errorf("start refresh Server: %w", err)
		}
	}

	defer s.CleanupRefreshServer()

	for {
		runLifecycleCommandForState, err := deriveRunLifecycleCommandForState(
			currentRunLifecycleState,
		)
		if err != nil {
			return err
		}

		runLifecycleCommandResultForState, err := s.ExecuteRunLifecycleCommand(
			runLifecycleCommandForState,
			runLifecycleCommandInput{
				FirstRun:         firstRun,
				CurrentRunIntent: currentRunIntent,
			},
		)
		if err != nil {
			return err
		}

		if runLifecycleCommandResultForState.UpdatedRunIntent != nil {
			currentRunIntent = *runLifecycleCommandResultForState.UpdatedRunIntent
		}
		if runLifecycleCommandResultForState.ShouldMarkFirstRunAsNotFirstRun {
			firstRun = false
		}

		nextRunLifecycleState, err := TransitionRunLifecycleState(
			s.Log,
			currentRunLifecycleState,
			runLifecycleCommandResultForState.RunLifecycleEvent,
			currentLifecycleCycleID,
		)
		if err != nil {
			return err
		}
		currentRunLifecycleState = nextRunLifecycleState
		if currentRunLifecycleState == RunLifecycleStatePreparingCycle &&
			runLifecycleCommandForState != runLifecycleCommandPrepareCycle {
			currentLifecycleCycleID++
		}
	}
}

func deriveRunLifecycleCommandForState(
	currentRunLifecycleState RunLifecycleState,
) (runLifecycleCommand, error) {
	return devserverengine.DeriveRunLifecycleCommandForState(
		currentRunLifecycleState,
	)
}

func (s *Server) ExecuteRunLifecycleCommand(
	runLifecycleCommandForState runLifecycleCommand,
	runLifecycleCommandInputForState runLifecycleCommandInput,
) (runLifecycleCommandResult, error) {
	switch runLifecycleCommandForState {
	case runLifecycleCommandPrepareCycle:
		return s.ExecuteRunLifecycleCommandPrepareCycle(
			runLifecycleCommandInputForState,
		)
	case runLifecycleCommandBuildCycle:
		return s.ExecuteRunLifecycleCommandBuildCycle(
			runLifecycleCommandInputForState,
		)
	case runLifecycleCommandAwaitBuildRetry:
		return s.ExecuteRunLifecycleCommandAwaitBuildRetry(
			runLifecycleCommandInputForState,
		)
	case runLifecycleCommandStartRuntime:
		return s.ExecuteRunLifecycleCommandStartRuntime(
			runLifecycleCommandInputForState,
		)
	case runLifecycleCommandAwaitRestartRequest:
		return s.ExecuteRunLifecycleCommandAwaitRestartRequest(
			runLifecycleCommandInputForState,
		)
	case runLifecycleCommandCleanupForNextCycle:
		return s.ExecuteRunLifecycleCommandCleanupForNextCycle(
			runLifecycleCommandInputForState,
		)
	default:
		return runLifecycleCommandResult{}, fmt.Errorf(
			"unknown Run lifecycle Command: %q",
			runLifecycleCommandForState,
		)
	}
}

func (s *Server) PrepareRunCycle(firstRun bool) error {
	if !firstRun {
		if err := s.ReloadConfig(); err != nil {
			s.Log.Error("config reload failed", "error", err)
		}
	}

	s.SetBuilder(toolingbuilder.NewBuilder(s.Cfg, s.Log))

	if err := s.InitWatcher(); err != nil {
		return fmt.Errorf("init watcher: %w", err)
	}

	return nil
}

func (s *Server) ExecuteRunBuildForIntent(
	isRebuild bool,
	currentRunIntent runIntent,
) error {
	buildExecutionOrderingDecision := deriveRunBuildExecutionOrderingDecision(
		currentRunIntent.RecompileGo,
		s.Cfg.Core.SequentialGoBuild,
	)
	s.Log.Debug(
		"resolved build execution ordering decision",
		"go_compilation_ordering_policy",
		buildExecutionOrderingDecision.GoCompilationOrderingPolicy,
		"compile_in_parallel",
		buildExecutionOrderingDecision.RunCompileInParallel,
		"compile_after_build_hooks",
		buildExecutionOrderingDecision.RunCompileAfterBuildHooks,
	)

	// Run builds - either in parallel or sequentially based on config.
	var buildGroup errgroup.Group

	buildGroup.Go(func() error {
		builderForBuild := s.GetBuilder()
		if builderForBuild == nil {
			return fmt.Errorf("builder is nil")
		}
		return builderForBuild.Build(toolingbuilder.BuildOpts{
			IsDev:     true,
			CompileGo: false,
			IsRebuild: isRebuild,
		})
	})

	if buildExecutionOrderingDecision.RunCompileInParallel {
		buildGroup.Go(func() error {
			builderForCompile := s.GetBuilder()
			if builderForCompile == nil {
				return fmt.Errorf("builder is nil")
			}
			return builderForCompile.CompileGoOnly(true)
		})
	}

	if err := buildGroup.Wait(); err != nil {
		return err
	}

	if buildExecutionOrderingDecision.RunCompileAfterBuildHooks {
		builderForSequentialCompile := s.GetBuilder()
		if builderForSequentialCompile == nil {
			return fmt.Errorf("builder is nil for sequential Go compile")
		}
		if err := builderForSequentialCompile.CompileGoOnly(true); err != nil {
			return fmt.Errorf("go compilation failed: %w", err)
		}
	}

	return nil
}

func (s *Server) StartRunCycleRuntime(currentRunIntent *runIntent) {

	if s.ViteCtx == nil && s.Cfg.UsingVite() {
		if err := s.StartVite(); err != nil {
			s.Log.Error("vite start failed", "error", err)
		}
	}

	s.StartApp()

	currentRunCycleScope := s.StartRunCycleScope()

	s.WatcherStartCh = make(chan struct{})

	if currentRunCycleScope != nil {
		currentRunCycleScope.LaunchAsyncWork(func(
			currentRunCycleContext context.Context,
		) {
			select {
			case <-s.WatcherStartCh:
			case <-currentRunCycleContext.Done():
				return
			}
			s.RunWatcherWithContext(currentRunCycleContext)
		})
	}

	if currentRunIntent != nil && currentRunIntent.IsConfigRestart {
		s.BroadcastReload(ReloadOpts{
			Payload:   broadcast.Payload{ChangeType: broadcast.ChangeTypeOther},
			WaitApp:   true,
			WaitVite:  true,
			CycleVite: true,
		})
		currentRunIntent.IsConfigRestart = false
	}

	close(s.WatcherStartCh)
}

// WaitForBuildRetry waits for a file change that might fix the build error.
// It starts the watcher and waits for any restart request.
func (s *Server) WaitForBuildRetry() restartRequest {
	s.SetWaitingForBuildRetry(true)
	defer s.SetWaitingForBuildRetry(false)

	if pendingRestartRequest, hasPendingRestartRequest := s.ConsumePendingRestartRequest(); hasPendingRestartRequest {
		normalizedPendingRestartRequest := normalizeRestartRequest(
			pendingRestartRequest,
		)
		s.CleanupForRebuild()
		return normalizedPendingRestartRequest
	}

	s.WatcherStartCh = make(chan struct{})
	buildRetryRunCycleScope := s.StartRunCycleScope()

	if buildRetryRunCycleScope != nil {
		buildRetryRunCycleScope.LaunchAsyncWork(func(
			buildRetryRunCycleContext context.Context,
		) {
			select {
			case <-s.WatcherStartCh:
			case <-buildRetryRunCycleContext.Done():
				return
			}
			s.RunWatcherWithContext(buildRetryRunCycleContext)
		})
	}
	close(s.WatcherStartCh)

	restartRequestForRetry := s.ConsumeRestartRequestBlocking()

	s.CleanupForRebuild()
	return restartRequestForRetry
}

// CleanupForRebuild cleans up resources but keeps refresh Server and Vite alive
func (s *Server) CleanupForRebuild() {
	s.CancelAndJoinCurrentRunCycleScope()
	s.CancelConcurrentNoWaitHookLifecycleContext()

	if err := s.StopApp(); err != nil {
		s.Log.Error("stop app failed", "error", err)
	}

	s.Mu.Lock()
	watcher := s.Watcher
	s.Watcher = nil
	s.Mu.Unlock()

	if watcher != nil {
		if err := watcher.Close(); err != nil {
			s.Log.Error("close watcher failed", "error", err)
		}
	}

	s.Mu.Lock()
	builder := s.Builder
	s.Builder = nil
	s.Mu.Unlock()

	if builder != nil {
		if err := builder.Close(); err != nil {
			s.Log.Error("close builder failed", "error", err)
		}
	}
}

// CleanupRefreshServer cleans up the refresh Server (called on full shutdown)
func (s *Server) CleanupRefreshServer() {
	s.CancelAndJoinCurrentRunCycleScope()
	s.CancelConcurrentNoWaitHookLifecycleContext()

	if err := s.StopRefreshServer(); err != nil {
		s.Log.Error("stop refresh Server failed", "error", err)
	}

	if s.RefreshMgrCancel != nil {
		s.RefreshMgrCancel()
		if s.RefreshMgr != nil {
			s.RefreshMgr.Wait()
		}
		s.RefreshMgrCancel = nil
	}
}
func (s *Server) ExecuteRunLifecycleCommandPrepareCycle(
	runLifecycleCommandInputForState runLifecycleCommandInput,
) (runLifecycleCommandResult, error) {
	if err := s.PrepareRunCycle(runLifecycleCommandInputForState.FirstRun); err != nil {
		return runLifecycleCommandResult{}, err
	}
	return runLifecycleCommandResult{
		RunLifecycleEvent: RunLifecycleEventCyclePrepared,
	}, nil
}

func (s *Server) ExecuteRunLifecycleCommandBuildCycle(
	runLifecycleCommandInputForState runLifecycleCommandInput,
) (runLifecycleCommandResult, error) {
	if err := s.ExecuteRunBuildForIntent(
		!runLifecycleCommandInputForState.FirstRun,
		runLifecycleCommandInputForState.CurrentRunIntent,
	); err != nil {
		s.Log.Error("build failed", "error", err)
		return runLifecycleCommandResult{
			RunLifecycleEvent: RunLifecycleEventBuildFailed,
		}, nil
	}
	return runLifecycleCommandResult{
		RunLifecycleEvent: RunLifecycleEventBuildSucceeded,
	}, nil
}

func (s *Server) ExecuteRunLifecycleCommandAwaitBuildRetry(
	_ runLifecycleCommandInput,
) (runLifecycleCommandResult, error) {
	s.Log.Info("Waiting for file changes to retry build...")
	restartRequestForRetry := s.WaitForBuildRetry()
	currentRunIntentForRetry := DeriveRunIntentFromRestartRequest(
		restartRequestForRetry,
	)
	return runLifecycleCommandResult{
		RunLifecycleEvent:               RunLifecycleEventBuildRetryRestartReceived,
		UpdatedRunIntent:                &currentRunIntentForRetry,
		ShouldMarkFirstRunAsNotFirstRun: true,
	}, nil
}

func (s *Server) ExecuteRunLifecycleCommandStartRuntime(
	runLifecycleCommandInputForState runLifecycleCommandInput,
) (runLifecycleCommandResult, error) {
	currentRunIntentForRuntime := runLifecycleCommandInputForState.CurrentRunIntent
	s.StartRunCycleRuntime(&currentRunIntentForRuntime)
	return runLifecycleCommandResult{
		RunLifecycleEvent:               RunLifecycleEventRuntimeStarted,
		UpdatedRunIntent:                &currentRunIntentForRuntime,
		ShouldMarkFirstRunAsNotFirstRun: true,
	}, nil
}

func (s *Server) ExecuteRunLifecycleCommandAwaitRestartRequest(
	_ runLifecycleCommandInput,
) (runLifecycleCommandResult, error) {
	restartRequestForRun := s.ConsumeRestartRequestBlocking()
	currentRunIntentForRestart := DeriveRunIntentFromRestartRequest(
		restartRequestForRun,
	)
	s.Log.Info(
		"Restarting dev Server...",
		"recompile_go",
		currentRunIntentForRestart.RecompileGo,
		"config_restart",
		currentRunIntentForRestart.IsConfigRestart,
	)
	return runLifecycleCommandResult{
		RunLifecycleEvent: RunLifecycleEventRestartRequestReceived,
		UpdatedRunIntent:  &currentRunIntentForRestart,
	}, nil
}

func (s *Server) ExecuteRunLifecycleCommandCleanupForNextCycle(
	_ runLifecycleCommandInput,
) (runLifecycleCommandResult, error) {

	s.BroadcastRebuilding()

	s.CleanupForRebuild()

	return runLifecycleCommandResult{
		RunLifecycleEvent: RunLifecycleEventCleanupCompleted,
	}, nil
}

// RunLifecycleState defines one state of the devserver Run loop.
type RunLifecycleState = devserverengine.RunLifecycleState

const (
	RunLifecycleStatePreparingCycle         = devserverengine.RunLifecycleStatePreparingCycle
	RunLifecycleStateBuildingCycle          = devserverengine.RunLifecycleStateBuildingCycle
	RunLifecycleStateAwaitingBuildRetry     = devserverengine.RunLifecycleStateAwaitingBuildRetry
	RunLifecycleStateStartingRuntime        = devserverengine.RunLifecycleStateStartingRuntime
	RunLifecycleStateAwaitingRestart        = devserverengine.RunLifecycleStateAwaitingRestart
	RunLifecycleStateCleaningUpForNextCycle = devserverengine.RunLifecycleStateCleaningUpForNextCycle
)

// RunLifecycleEvent defines one transition trigger inside the Run loop state
// machine.
type RunLifecycleEvent = devserverengine.RunLifecycleEvent

const (
	RunLifecycleEventCyclePrepared             = devserverengine.RunLifecycleEventCyclePrepared
	RunLifecycleEventBuildSucceeded            = devserverengine.RunLifecycleEventBuildSucceeded
	RunLifecycleEventBuildFailed               = devserverengine.RunLifecycleEventBuildFailed
	RunLifecycleEventBuildRetryRestartReceived = devserverengine.RunLifecycleEventBuildRetryRestartReceived
	RunLifecycleEventRuntimeStarted            = devserverengine.RunLifecycleEventRuntimeStarted
	RunLifecycleEventRestartRequestReceived    = devserverengine.RunLifecycleEventRestartRequestReceived
	RunLifecycleEventCleanupCompleted          = devserverengine.RunLifecycleEventCleanupCompleted
)

// TransitionLogger is the minimal logging capability needed during lifecycle
// transitions.
type TransitionLogger = devserverengine.TransitionLogger

// DeriveRunLifecycleStateAfterEvent computes the next state from the current
// state and transition event.
func DeriveRunLifecycleStateAfterEvent(
	currentRunLifecycleState RunLifecycleState,
	runLifecycleEventForTransition RunLifecycleEvent,
) (RunLifecycleState, error) {
	return devserverengine.DeriveRunLifecycleStateAfterEvent(
		currentRunLifecycleState,
		runLifecycleEventForTransition,
	)
}

// TransitionRunLifecycleState applies one transition and logs it when a logger
// is provided.
func TransitionRunLifecycleState(
	transitionLogger TransitionLogger,
	currentRunLifecycleState RunLifecycleState,
	runLifecycleEventForTransition RunLifecycleEvent,
	currentLifecycleCycleID uint64,
) (RunLifecycleState, error) {
	return devserverengine.TransitionRunLifecycleState(
		transitionLogger,
		currentRunLifecycleState,
		runLifecycleEventForTransition,
		currentLifecycleCycleID,
	)
}
func (s *Server) StartVite() error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	ctx, err := s.Builder.NewViteDevContext()
	if err != nil {
		return err
	}
	if ctx == nil {
		return nil
	}

	s.ViteCtx = ctx
	return nil
}

func (s *Server) StopVite() error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if s.ViteCtx != nil {
		s.ViteCtx.Cleanup()
		s.ViteCtx = nil
	}
	return nil
}

// CycleVite stops and restarts Vite, waiting for it to be ready.
// Called after the Go app is ready so Vite's client reconnect hits a working Server.
func (s *Server) CycleVite() {
	_ = s.CycleViteAndWaitForReadiness()
}

func (s *Server) CycleViteAndWaitForReadiness() bool {
	if s == nil || s.Cfg == nil || !s.Cfg.UsingVite() {
		return false
	}

	s.Mu.Lock()
	hasVite := s.ViteCtx != nil
	s.Mu.Unlock()
	if !hasVite {
		return false
	}

	s.Log.Info("Cycling Vite...")
	if err := s.StopVite(); err != nil {
		s.Log.Error("stop vite failed during cycle", "error", err)
		return false
	}
	if err := s.StartVite(); err != nil {
		s.Log.Error("start vite failed during cycle", "error", err)
		return false
	}

	s.Mu.Lock()
	hasVite = s.ViteCtx != nil
	s.Mu.Unlock()
	if !hasVite {
		return false
	}

	if !s.WaitForVite() {
		return false
	}

	s.Mu.Lock()
	hasVite = s.ViteCtx != nil
	s.Mu.Unlock()
	if !hasVite {
		return false
	}

	s.Log.Info("Vite cycled and ready")
	return true
}

// CallViteFilemapInvalidate calls the Vite plugin's filemap invalidation endpoint.
// This clears the plugin's cached filemap and invalidates all modules, triggering
// a browser reload through Vite's HMR system.
func (s *Server) CallViteFilemapInvalidate() error {
	s.Mu.Lock()
	viteCtx := s.ViteCtx
	s.Mu.Unlock()

	if viteCtx == nil {
		return fmt.Errorf("vite not running")
	}

	url := fmt.Sprintf(
		"http://localhost:%d/__vorma_invalidate_filemap",
		viteCtx.Port(),
	)

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

	s.Log.Info("Vite filemap invalidated successfully")
	return nil
}

func (s *Server) WaitForVite() bool {
	s.Mu.Lock()
	viteCtx := s.ViteCtx
	s.Mu.Unlock()

	if viteCtx == nil {
		return true
	}
	urls := ResolveViteReadyURLs(viteCtx.Port())
	ok := s.WaitForAnyReady(urls)
	if !ok {
		s.Log.Warn(
			"Vite did not become ready in time",
			"urls",
			strings.Join(urls, ", "),
		)
	}
	return ok
}

func ResolveViteReadyURL(vitePort int) string {
	return ResolveViteReadyURLs(vitePort)[0]
}

func ResolveViteReadyURLs(vitePort int) []string {
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

// ReloadOpts configures BroadcastReload behavior.
type ReloadOpts struct {
	Payload   broadcast.Payload
	WaitApp   bool
	WaitVite  bool
	CycleVite bool
}

type reloadReadinessOutcome struct {
	WaitedForApp     bool
	WaitedForVite    bool
	CycleViteApplied bool
}

type reloadOrchestrationOutcome struct {
	BroadcastEnabled       bool
	BroadcastContextActive bool

	ReadinessOutcome       reloadReadinessOutcome
	ShouldBroadcastPayload bool
	PayloadBroadcasted     bool
}

// BroadcastRebuilding sends the "rebuilding" signal to show UI overlay.
// Uses blocking send, but guarded by context to prevent deadlock during shutdown.
func (s *Server) BroadcastRebuilding() {
	if !s.ShouldBroadcastToBrowserClients() {
		return
	}

	if !isBroadcastContextActive(s.RefreshMgrCtx) {
		return
	}

	_ = s.SendRefreshPayloadWhenBroadcastContextActive(
		broadcast.Payload{
			ChangeType: broadcast.ChangeTypeRebuilding,
		},
	)
}

// BroadcastReload handles browser reload orchestration.
//
// When CycleVite is true:
// 1. Wait for app to be ready.
// 2. Stop and restart Vite.
// 3. Wait for Vite to be ready.
// 4. Vite's client reconnect triggers the browser reload automatically.
// 5. Do not send Wave's reload signal (would cause double reload).
//
// When CycleVite is false:
// 1. Wait for app/vite as specified.
// 2. Send Wave's reload signal to trigger browser reload.
func (s *Server) BroadcastReload(
	reloadOptions ReloadOpts,
) reloadOrchestrationOutcome {
	reloadOutcome := reloadOrchestrationOutcome{
		BroadcastEnabled: s.ShouldBroadcastToBrowserClients(),
	}
	if !reloadOutcome.BroadcastEnabled {
		return reloadOutcome
	}

	reloadOutcome.BroadcastContextActive = isBroadcastContextActive(s.RefreshMgrCtx)
	if !reloadOutcome.BroadcastContextActive {
		return reloadOutcome
	}

	reloadOutcome.ReadinessOutcome = s.WaitForReloadReadiness(reloadOptions)
	reloadOutcome.ShouldBroadcastPayload = ShouldBroadcastReloadPayloadAfterReadiness(
		reloadOptions,
		reloadOutcome.ReadinessOutcome.CycleViteApplied,
	)
	if !reloadOutcome.ShouldBroadcastPayload {
		return reloadOutcome
	}

	reloadOutcome.PayloadBroadcasted = s.SendRefreshPayloadWhenBroadcastContextActive(
		reloadOptions.Payload,
	)
	return reloadOutcome
}

func (s *Server) ShouldBroadcastToBrowserClients() bool {
	return s.Cfg.UsingBrowser() && s.RefreshMgr != nil
}

func isBroadcastContextActive(refreshManagerContext context.Context) bool {
	if refreshManagerContext == nil {
		return true
	}

	select {
	case <-refreshManagerContext.Done():
		return false
	default:
		return true
	}
}

func (s *Server) SendRefreshPayloadWhenBroadcastContextActive(
	payload broadcast.Payload,
) bool {
	if s.RefreshMgr == nil {
		return false
	}

	if !isBroadcastContextActive(s.RefreshMgrCtx) {
		return false
	}

	select {
	case s.RefreshMgr.Broadcast <- payload:
		return true
	case <-s.RefreshMgrCtx.Done():
		return false
	}
}

func (s *Server) WaitForReloadReadiness(
	reloadOptions ReloadOpts,
) reloadReadinessOutcome {
	reloadReadinessOutcomeForReload := reloadReadinessOutcome{}

	if reloadOptions.WaitApp {
		reloadReadinessOutcomeForReload.WaitedForApp = true
		s.WaitForApp()
	}

	reloadReadinessOutcomeForReload.CycleViteApplied = s.CycleViteForReloadIfRequested(
		reloadOptions.CycleVite,
	)
	if reloadReadinessOutcomeForReload.CycleViteApplied {
		return reloadReadinessOutcomeForReload
	}

	if reloadOptions.WaitVite {
		reloadReadinessOutcomeForReload.WaitedForVite = true
		s.WaitForVite()
	}

	return reloadReadinessOutcomeForReload
}

func (s *Server) CycleViteForReloadIfRequested(
	cycleViteRequested bool,
) bool {
	if !cycleViteRequested || !s.Cfg.UsingVite() {
		return false
	}

	s.Mu.Lock()
	hasActiveViteContext := s.ViteCtx != nil
	s.Mu.Unlock()
	if !hasActiveViteContext {
		return false
	}

	return s.CycleViteAndWaitForReadiness()
}

func ShouldBroadcastReloadPayloadAfterReadiness(
	reloadOptions ReloadOpts,
	cycleViteApplied bool,
) bool {
	if !reloadOptions.CycleVite {
		return true
	}
	return !cycleViteApplied
}

type FileType int

const (
	FileTypeOther FileType = iota
	FileTypeGo
	FileTypeCriticalCSS
	FileTypeNormalCSS
	FileTypeCriticalAndNormalCSS
	FileTypePublicStatic
	FileTypePrivateStatic
)

type ClassifiedEvent struct {
	Event       fsnotify.Event
	FileType    FileType
	WatchedFile *wave.WatchedFile
	Ignored     bool
	ChmodOnly   bool
}

// EventWithHooks pairs a classified event with its sorted hooks.
type EventWithHooks struct {
	Classified         ClassifiedEvent
	Hooks              *wave.SortedHooks
	HookCtx            *wave.HookContext
	RunOnChangeOnly    bool
	NeedsHardReload    bool
	SkipDuplicateHooks bool
}

type BuildPhaseDecision struct {
	CompileGo                       bool
	BuildCriticalCSS                bool
	BuildNormalCSS                  bool
	ProcessPublicFiles              bool
	ProcessPrivateFiles             bool
	PublicStaticChangedFilePaths    []string
	PrivateStaticChangedFilePaths   []string
	PublicStaticChangedFilePathSet  map[string]struct{}
	PrivateStaticChangedFilePathSet map[string]struct{}
}

type RestartPhaseDecision struct {
	RestartApp bool
}

type BrowserPhaseAction int

const (
	BrowserPhaseActionNone BrowserPhaseAction = iota
	BrowserPhaseActionHotReloadCSS
	BrowserPhaseActionRevalidate
	BrowserPhaseActionHardReload
	BrowserPhaseActionInvalidateVite
)

type BrowserPhaseDecision struct {
	Action      BrowserPhaseAction
	WaitForApp  bool
	WaitForVite bool
	CycleVite   bool
}

type BrowserPhaseResolution struct {
	Action         BrowserPhaseAction
	ApplyWaitFlags bool
	WaitForApp     bool
	WaitForVite    bool
}

// WorkSet collects per-phase execution decisions for a watcher cycle.
type WorkSet struct {
	Build   BuildPhaseDecision
	Restart RestartPhaseDecision
	Browser BrowserPhaseDecision

	// User preference collected from watched files and applied during resolve.
	PreferRevalidate bool
}

type AppStopStrategy int

const (
	AppStopStrategyNone AppStopStrategy = iota
	AppStopStrategySingleEventHardReload
	AppStopStrategyBatchHardReload
)

type EventExecutionPlanningResult struct {
	EventsWithHooks []EventWithHooks
	ConfigChanged   bool
}

type WatcherEventFlowDecision struct {
	TriggerConfigRestart       bool
	BroadcastRebuildingOverlay bool
	BehavioralDecision         EventExecutionPlanBehavioralDecision
}

type WatcherEventExecutionInput struct {
	FlowDecision            WatcherEventFlowDecision
	EventsWithHooks         []EventWithHooks
	WatcherEventLogPayloads []WatcherEventLogPayload
}

type EventExecutionPlanBehavioralDecision struct {
	ShowRebuildingOverlay bool
	AppStopStrategy       AppStopStrategy
	RunImplicitBuild      bool
}

type WatcherEventLogPayload struct {
	Operation string
	FilePath  string
}

type RefreshActionApplicationResult struct {
	RestartRequested bool
	RecompileGo      bool
}

type RefreshActionWorkMutationDecision struct {
	RestartApp           bool
	CompileGo            bool
	RequestBrowserAction bool
	BrowserAction        BrowserPhaseAction
	WaitForApp           bool
	WaitForVite          bool
}

type refreshActionReductionDecision struct {
	ActionsBeforeRestart     []wave.RefreshAction
	RestartActionEncountered bool
	RestartActionIndex       int
	ApplicationResult        RefreshActionApplicationResult
}

type ImplicitWorkDecision struct {
	CompileGo                    bool
	BuildCriticalCSS             bool
	BuildNormalCSS               bool
	ProcessPublicFiles           bool
	ProcessPrivateFiles          bool
	RestartApp                   bool
	PreferRevalidate             bool
	PublicStaticChangedFilePath  string
	PrivateStaticChangedFilePath string
}

func (s *Server) BuildEventExecutionPlan(
	events []fsnotify.Event,
	watcher *watch.Watcher,
	builder *toolingbuilder.Builder,
) EventExecutionPlanningResult {
	deduplicatedEvents := dedup.DeduplicateWatcherEventsByPath(events)
	classifiedEvents, configChanged := s.ClassifyWatcherEventsForProcessing(
		deduplicatedEvents,
		watcher,
		builder,
	)
	if configChanged {
		return EventExecutionPlanningResult{
			ConfigChanged: true,
		}
	}

	if len(classifiedEvents) == 0 {
		return EventExecutionPlanningResult{}
	}

	eventsWithHooks := BuildEventExecutionPlanFromClassifiedEvents(classifiedEvents)
	if len(eventsWithHooks) == 0 {
		return EventExecutionPlanningResult{}
	}

	return EventExecutionPlanningResult{
		EventsWithHooks: eventsWithHooks,
	}
}

func BuildEventExecutionPlanFromClassifiedEvents(
	classifiedEvents []ClassifiedEvent,
) []EventWithHooks {
	if len(classifiedEvents) == 0 {
		return nil
	}

	eventsWithHooks := BuildEventHooksForProcessing(classifiedEvents)
	if len(eventsWithHooks) == 0 {
		return nil
	}

	return eventsWithHooks
}
func PlanBrowserReloadForAction(
	action BrowserPhaseAction,
	browserDecision BrowserPhaseDecision,
) (ReloadOpts, bool) {
	switch action {
	case BrowserPhaseActionHardReload:
		return ReloadOpts{
			Payload:   broadcast.Payload{ChangeType: broadcast.ChangeTypeOther},
			WaitApp:   browserDecision.WaitForApp,
			WaitVite:  browserDecision.WaitForVite,
			CycleVite: browserDecision.CycleVite,
		}, true
	case BrowserPhaseActionRevalidate:
		return ReloadOpts{
			Payload:   broadcast.Payload{ChangeType: broadcast.ChangeTypeRevalidate},
			WaitApp:   browserDecision.WaitForApp,
			WaitVite:  browserDecision.WaitForVite,
			CycleVite: false,
		}, true
	default:
		return ReloadOpts{}, false
	}
}

func PlanInvalidateViteFallbackBrowserDecision(
	usingVite bool,
) BrowserPhaseDecision {
	return BrowserPhaseDecision{
		Action:      BrowserPhaseActionHardReload,
		WaitForApp:  true,
		WaitForVite: usingVite,
	}
}

func PlanHotReloadCSSPayloads(
	includeCriticalCSS bool,
	criticalCSS string,
	criticalCSSAvailable bool,
	includeNormalCSS bool,
	normalCSSURL string,
	normalCSSURLAvailable bool,
) []broadcast.Payload {
	payloads := make([]broadcast.Payload, 0, 2)
	if includeCriticalCSS && criticalCSSAvailable {
		payloads = append(payloads, broadcast.Payload{
			ChangeType:  broadcast.ChangeTypeCriticalCSS,
			CriticalCSS: base64.StdEncoding.EncodeToString([]byte(criticalCSS)),
		})
	}
	if includeNormalCSS && normalCSSURLAvailable {
		payloads = append(payloads, broadcast.Payload{
			ChangeType:   broadcast.ChangeTypeNormalCSS,
			NormalCSSURL: normalCSSURL,
		})
	}
	return payloads
}

type BrowserPhaseExecutionCategory int

const (
	BrowserPhaseExecutionCategoryNone BrowserPhaseExecutionCategory = iota
	BrowserPhaseExecutionCategoryReload
	BrowserPhaseExecutionCategoryHotReloadCSS
)

func ShouldAttemptViteInvalidateForBrowserDecision(
	browserDecision BrowserPhaseDecision,
	usingVite bool,
) bool {
	return browserDecision.Action == BrowserPhaseActionInvalidateVite && usingVite
}

func ResolveBrowserDecisionAfterInvalidateViteFallback(
	browserDecision BrowserPhaseDecision,
	usingVite bool,
) BrowserPhaseDecision {
	if browserDecision.Action != BrowserPhaseActionInvalidateVite {
		return browserDecision
	}

	fallbackDecision := PlanInvalidateViteFallbackBrowserDecision(usingVite)
	browserDecision.Action = fallbackDecision.Action
	browserDecision.WaitForApp = fallbackDecision.WaitForApp
	browserDecision.WaitForVite = fallbackDecision.WaitForVite
	return browserDecision
}

func DeriveBrowserPhaseExecutionCategory(
	action BrowserPhaseAction,
) BrowserPhaseExecutionCategory {
	switch action {
	case BrowserPhaseActionHardReload, BrowserPhaseActionRevalidate:
		return BrowserPhaseExecutionCategoryReload
	case BrowserPhaseActionHotReloadCSS:
		return BrowserPhaseExecutionCategoryHotReloadCSS
	default:
		return BrowserPhaseExecutionCategoryNone
	}
}

func (s *Server) ExecuteBrowserPhase(work *WorkSet) {
	if !s.Cfg.UsingBrowser() {
		return
	}

	builder := s.GetBuilder()
	browserDecisionForExecution := work.Browser
	if browserDecisionForExecution.Action == BrowserPhaseActionInvalidateVite {
		if ShouldAttemptViteInvalidateForBrowserDecision(
			browserDecisionForExecution,
			s.Cfg.UsingVite(),
		) {
			if err := s.CallViteFilemapInvalidate(); err != nil {
				s.Log.Warn("Vite filemap invalidate failed, falling back to reload", "error", err)
			} else {
				return
			}
		}
		browserDecisionForExecution = ResolveBrowserDecisionAfterInvalidateViteFallback(
			browserDecisionForExecution,
			s.Cfg.UsingVite(),
		)
		work.Browser = browserDecisionForExecution
	}

	switch DeriveBrowserPhaseExecutionCategory(browserDecisionForExecution.Action) {
	case BrowserPhaseExecutionCategoryReload:
		reloadPlan, hasReloadPlan := PlanBrowserReloadForAction(
			browserDecisionForExecution.Action,
			browserDecisionForExecution,
		)
		if !hasReloadPlan {
			return
		}
		if browserDecisionForExecution.Action == BrowserPhaseActionHardReload {
			s.Log.Info("Hard reloading browser")
		} else {
			s.Log.Info("Running client-defined revalidate function")
		}
		s.BroadcastReload(reloadPlan)
		return

	case BrowserPhaseExecutionCategoryHotReloadCSS:
		if builder == nil {
			return
		}
		s.ExecuteHotReloadCSSBrowserPhase(builder, work.Build)
		return

	case BrowserPhaseExecutionCategoryNone:
		return
	}
}

func (s *Server) ExecuteHotReloadCSSBrowserPhase(
	builder *toolingbuilder.Builder,
	buildDecision BuildPhaseDecision,
) {
	s.Log.Info("Hot reloading CSS")

	criticalCSS := ""
	criticalCSSAvailable := false
	if buildDecision.BuildCriticalCSS {
		var readCriticalCSSError error
		criticalCSS, readCriticalCSSError = builder.ReadCriticalCSSForHotReload(true)
		if readCriticalCSSError != nil {
			s.Log.Warn(
				"Skipping critical CSS hot reload payload due to missing fresh build output",
				"error",
				readCriticalCSSError,
			)
		} else {
			criticalCSSAvailable = true
		}
	}

	normalCSSURL := ""
	normalCSSURLAvailable := false
	if buildDecision.BuildNormalCSS {
		var readNormalCSSURLError error
		normalCSSURL, readNormalCSSURLError = builder.ReadNormalCSSURLForHotReload(true)
		if readNormalCSSURLError != nil {
			s.Log.Warn(
				"Skipping normal CSS hot reload payload due to missing fresh build output",
				"error",
				readNormalCSSURLError,
			)
		} else {
			normalCSSURLAvailable = true
		}
	}

	payloads := PlanHotReloadCSSPayloads(
		buildDecision.BuildCriticalCSS,
		criticalCSS,
		criticalCSSAvailable,
		buildDecision.BuildNormalCSS,
		normalCSSURL,
		normalCSSURLAvailable,
	)
	for _, payload := range payloads {
		s.BroadcastReload(ReloadOpts{
			Payload: payload,
		})
	}
}

type StaticFileProcessingExecutionMode int

const (
	StaticFileProcessingExecutionModeNone StaticFileProcessingExecutionMode = iota
	StaticFileProcessingExecutionModeFullScan
	StaticFileProcessingExecutionModeChangedPaths
)

type StaticFileProcessingExecutionDecision struct {
	Mode             StaticFileProcessingExecutionMode
	ChangedFilePaths []string
}

func (decision StaticFileProcessingExecutionDecision) ShouldProcess() bool {
	return decision.Mode != StaticFileProcessingExecutionModeNone
}

type BuildPhaseExecutionDecision struct {
	CompileGo                   bool
	PublicStaticProcessing      StaticFileProcessingExecutionDecision
	PrivateStaticProcessing     StaticFileProcessingExecutionDecision
	BuildCriticalCSS            bool
	BuildNormalCSS              bool
	WriteFrameworkPublicFileMap bool
}

func ShouldExecuteBuildPhaseForExecutionDecision(
	executionDecision BuildPhaseExecutionDecision,
) bool {
	return executionDecision.CompileGo ||
		ShouldExecuteAnyFileProcessingForBuildDecision(executionDecision)
}

func DeriveStaticFileProcessingExecutionDecision(
	shouldProcess bool,
	changedFilePaths []string,
) StaticFileProcessingExecutionDecision {
	if !shouldProcess {
		return StaticFileProcessingExecutionDecision{}
	}
	if len(changedFilePaths) == 0 {
		return StaticFileProcessingExecutionDecision{
			Mode: StaticFileProcessingExecutionModeFullScan,
		}
	}
	return StaticFileProcessingExecutionDecision{
		Mode:             StaticFileProcessingExecutionModeChangedPaths,
		ChangedFilePaths: append([]string(nil), changedFilePaths...),
	}
}

func ShouldWriteFrameworkPublicFileMapTSForBuildDecision(
	shouldProcessPublicStaticFiles bool,
	frameworkPublicFileMapOutDir string,
) bool {
	return shouldProcessPublicStaticFiles && frameworkPublicFileMapOutDir != ""
}

func DeriveBuildPhaseExecutionDecision(
	buildDecision BuildPhaseDecision,
	frameworkPublicFileMapOutDir string,
) BuildPhaseExecutionDecision {
	return BuildPhaseExecutionDecision{
		CompileGo: buildDecision.CompileGo,
		PublicStaticProcessing: DeriveStaticFileProcessingExecutionDecision(
			buildDecision.ProcessPublicFiles,
			buildDecision.PublicStaticChangedFilePaths,
		),
		PrivateStaticProcessing: DeriveStaticFileProcessingExecutionDecision(
			buildDecision.ProcessPrivateFiles,
			buildDecision.PrivateStaticChangedFilePaths,
		),
		BuildCriticalCSS: buildDecision.BuildCriticalCSS,
		BuildNormalCSS:   buildDecision.BuildNormalCSS,
		WriteFrameworkPublicFileMap: ShouldWriteFrameworkPublicFileMapTSForBuildDecision(
			buildDecision.ProcessPublicFiles,
			frameworkPublicFileMapOutDir,
		),
	}
}

func ShouldExecuteAnyFileProcessingForBuildDecision(
	executionDecision BuildPhaseExecutionDecision,
) bool {
	return executionDecision.PublicStaticProcessing.ShouldProcess() ||
		executionDecision.PrivateStaticProcessing.ShouldProcess() ||
		executionDecision.BuildCriticalCSS ||
		executionDecision.BuildNormalCSS
}

func (s *Server) ExecuteBuildPhase(work *WorkSet) error {
	if work == nil {
		return nil
	}

	buildExecutionDecision := DeriveBuildPhaseExecutionDecision(
		work.Build,
		s.Cfg.FrameworkPublicFileMapOutDir,
	)
	if !ShouldExecuteBuildPhaseForExecutionDecision(buildExecutionDecision) {
		return nil
	}

	builder := s.GetBuilder()
	if builder == nil {
		builderNilError := errors.New("builder is nil during build phase")
		s.Log.Error("Builder is nil during build phase", "error", builderNilError)
		return builderNilError
	}

	var buildPhaseGroup errgroup.Group

	if buildExecutionDecision.CompileGo {
		buildPhaseGroup.Go(func() error {
			if err := builder.CompileGoOnly(true); err != nil {
				s.Log.Error("Go compilation failed", "error", err)
				return err
			}
			return nil
		})
	}

	if ShouldExecuteAnyFileProcessingForBuildDecision(buildExecutionDecision) {
		buildPhaseGroup.Go(func() error {
			if err := s.ExecutePublicStaticProcessingForBuildPhase(
				builder,
				buildExecutionDecision,
			); err != nil {
				return err
			}

			var assetAndCSSBuildGroup errgroup.Group

			if buildExecutionDecision.PrivateStaticProcessing.ShouldProcess() {
				assetAndCSSBuildGroup.Go(func() error {
					return s.ExecutePrivateStaticProcessingForBuildPhase(
						builder,
						buildExecutionDecision.PrivateStaticProcessing,
					)
				})
			}

			if buildExecutionDecision.BuildCriticalCSS {
				assetAndCSSBuildGroup.Go(func() error {
					if err := builder.BuildCriticalCSS(true); err != nil {
						s.Log.Error("Critical CSS build failed", "error", err)
						return err
					}
					return nil
				})
			}

			if buildExecutionDecision.BuildNormalCSS {
				assetAndCSSBuildGroup.Go(func() error {
					if err := builder.BuildNormalCSS(true); err != nil {
						s.Log.Error("Normal CSS build failed", "error", err)
						return err
					}
					return nil
				})
			}

			return assetAndCSSBuildGroup.Wait()
		})
	}

	if err := buildPhaseGroup.Wait(); err != nil {
		s.Log.Error("Build phase had errors", "error", err)
		return err
	}

	return nil
}

func (s *Server) ExecutePublicStaticProcessingForBuildPhase(
	builder *toolingbuilder.Builder,
	buildExecutionDecision BuildPhaseExecutionDecision,
) error {
	if !buildExecutionDecision.PublicStaticProcessing.ShouldProcess() {
		return nil
	}

	publicStaticProcessingError := ExecuteStaticFileProcessingForBuildPhase(
		builder.ProcessPublicFilesOnly,
		builder.ProcessPublicFilesOnlyForChangedPaths,
		buildExecutionDecision.PublicStaticProcessing,
	)
	if publicStaticProcessingError != nil {
		s.Log.Error("Public files processing failed", "error", publicStaticProcessingError)
		return publicStaticProcessingError
	}

	if buildExecutionDecision.WriteFrameworkPublicFileMap {
		if err := builder.WritePublicFileMapTS(s.Cfg.FrameworkPublicFileMapOutDir); err != nil {
			s.Log.Error("Write public file map TS failed", "error", err)
			return err
		}
	}

	return nil
}

func (s *Server) ExecutePrivateStaticProcessingForBuildPhase(
	builder *toolingbuilder.Builder,
	privateStaticProcessingDecision StaticFileProcessingExecutionDecision,
) error {
	privateStaticProcessingError := ExecuteStaticFileProcessingForBuildPhase(
		builder.ProcessPrivateFilesOnly,
		builder.ProcessPrivateFilesOnlyForChangedPaths,
		privateStaticProcessingDecision,
	)
	if privateStaticProcessingError != nil {
		s.Log.Error("Private files processing failed", "error", privateStaticProcessingError)
		return privateStaticProcessingError
	}
	return nil
}

func ExecuteStaticFileProcessingForBuildPhase(
	executeFullStaticProcessing func() error,
	executeChangedPathsStaticProcessing func([]string) error,
	staticProcessingDecision StaticFileProcessingExecutionDecision,
) error {
	switch staticProcessingDecision.Mode {
	case StaticFileProcessingExecutionModeNone:
		return nil

	case StaticFileProcessingExecutionModeChangedPaths:
		if executeChangedPathsStaticProcessing == nil {
			return nil
		}
		return executeChangedPathsStaticProcessing(staticProcessingDecision.ChangedFilePaths)

	case StaticFileProcessingExecutionModeFullScan:
		if executeFullStaticProcessing == nil {
			return nil
		}
		return executeFullStaticProcessing()

	default:
		return nil
	}
}

// ClassifyWatcherEventsForProcessing applies pre-classification side effects,
// maps events into file categories, then drops ignored/chmod-only entries.
func (s *Server) ClassifyWatcherEventsForProcessing(
	events []fsnotify.Event,
	watcher *watch.Watcher,
	builder *toolingbuilder.Builder,
) ([]ClassifiedEvent, bool) {
	if len(events) == 0 {
		return nil, false
	}

	eventClassificationProber := classification.NewEventClassificationProber(
		s.IsConfigFile,
	)
	preClassificationPlan := classification.BuildPreClassificationPlanFromEvents(
		events,
		eventClassificationProber.ProbeIsConfigFile,
		eventClassificationProber.ProbeEventDirectoryStatus,
	)
	if preClassificationPlan.ConfigChanged {
		return nil, true
	}

	s.ApplyWatcherEventPreClassificationSideEffects(watcher, preClassificationPlan)
	classifiedEvents := s.ClassifyWatcherEventsFromPreClassificationPlan(
		preClassificationPlan,
		watcher,
		builder,
	)

	return FilterClassifiedEventsForProcessingByPostClassificationDecision(
		classifiedEvents,
	), false
}

// ApplyWatcherEventPreClassificationSideEffects runs watcher mutations derived
// from pre-classification. All logging suppression decisions live in
// shouldLogWatcherAddDirectoryError so this loop stays strictly orchestration.
func (s *Server) ApplyWatcherEventPreClassificationSideEffects(
	watcher *watch.Watcher,
	preClassificationPlan classification.PreClassificationPlan,
) {
	for _, directoryPathToWatch := range preClassificationPlan.AddDirectoryWatchPaths {
		addDirectoryWatchError := watcher.AddDir(directoryPathToWatch)
		if classification.ShouldLogAddDirectoryWatchError(addDirectoryWatchError) {
			s.Log.Warn(
				"failed to add directory watch",
				"path",
				directoryPathToWatch,
				"error",
				addDirectoryWatchError,
			)
		}
	}
}

// ClassifyWatcherEventsFromPreClassificationPlan maps each event that survived
// pre-classification into a typed ClassifiedEvent.
func (s *Server) ClassifyWatcherEventsFromPreClassificationPlan(
	preClassificationPlan classification.PreClassificationPlan,
	watcher *watch.Watcher,
	builder *toolingbuilder.Builder,
) []ClassifiedEvent {
	if len(preClassificationPlan.EventsToClassify) == 0 {
		return nil
	}

	classifiedEvents := make([]ClassifiedEvent, 0, len(preClassificationPlan.EventsToClassify))
	for _, eventToClassify := range preClassificationPlan.EventsToClassify {
		classifiedEvents = append(
			classifiedEvents,
			s.ClassifyEventWithWatcherAndBuilder(eventToClassify, watcher, builder),
		)
	}
	return classifiedEvents
}

// IsConfigFile checks whether a watcher path points at the active config file.
// It intentionally tolerates nil/empty state so early startup or teardown
// phases can classify events without panicking.
func (s *Server) IsConfigFile(path string) bool {
	if s == nil || s.Cfg == nil || s.Cfg.Core == nil {
		return false
	}

	configPath := s.Cfg.Core.ConfigLocation
	if configPath == "" {
		return false
	}

	return waveshared.PathsReferToSameLocation(path, configPath)
}

// filterClassifiedEventsForProcessingByPostClassificationDecision removes
// ignored and chmod-only classified events before planning/execution.
func FilterClassifiedEventsForProcessingByPostClassificationDecision(
	classifiedEvents []ClassifiedEvent,
) []ClassifiedEvent {
	if len(classifiedEvents) == 0 {
		return nil
	}

	filteredClassifiedEvents := make([]ClassifiedEvent, 0, len(classifiedEvents))
	for _, classifiedEventForProcessing := range classifiedEvents {
		postClassificationDecision := classification.DerivePostClassificationDecision(
			classifiedEventForProcessing.Ignored,
			classifiedEventForProcessing.ChmodOnly,
		)
		if !postClassificationDecision.IncludeClassifiedEvent {
			continue
		}
		filteredClassifiedEvents = append(
			filteredClassifiedEvents,
			classifiedEventForProcessing,
		)
	}
	return filteredClassifiedEvents
}

// ClassifyEventWithWatcherAndBuilder maps a raw watcher event into file type,
// watch metadata, ignore status, and chmod-only state used by later phases.
func (s *Server) ClassifyEventWithWatcherAndBuilder(
	watcherEvent fsnotify.Event,
	watcher *watch.Watcher,
	builder *toolingbuilder.Builder,
) ClassifiedEvent {
	classifiedEventForProcessing := ClassifiedEvent{Event: watcherEvent}

	if watcherEvent.Name == "" {
		classifiedEventForProcessing.Ignored = true
		return classifiedEventForProcessing
	}

	classifiedEventForProcessing.Ignored = watcher.IsIgnoredFile(watcherEvent.Name)
	classifiedEventForProcessing.FileType = deriveInitialFileTypeForWatcherEvent(
		watcherEvent.Name,
		watcher,
		builder,
	)
	classifiedEventForProcessing.WatchedFile = watcher.FindWatchedFile(watcherEvent.Name)
	classifiedEventForProcessing.FileType = deriveFileTypeWithWatchedFileOverrides(
		classifiedEventForProcessing.FileType,
		classifiedEventForProcessing.WatchedFile,
	)
	classifiedEventForProcessing.Ignored = deriveWatcherEventIgnoredStatus(
		classifiedEventForProcessing.Ignored,
		classifiedEventForProcessing.FileType,
		classifiedEventForProcessing.WatchedFile,
	)
	classifiedEventForProcessing.ChmodOnly = watch.IsNonEmptyChmodOnly(watcherEvent)

	return classifiedEventForProcessing
}

// deriveInitialFileTypeForWatcherEvent resolves baseline file type before
// watched-file overrides are applied.
func deriveInitialFileTypeForWatcherEvent(
	watcherEventPath string,
	watcher *watch.Watcher,
	builder *toolingbuilder.Builder,
) FileType {
	isCriticalCSSFile := builder.IsCriticalCSSFile(watcherEventPath)
	isNormalCSSFile := builder.IsNormalCSSFile(watcherEventPath)

	if isCriticalCSSFile && isNormalCSSFile {
		return FileTypeCriticalAndNormalCSS
	}
	if isCriticalCSSFile {
		return FileTypeCriticalCSS
	}
	if isNormalCSSFile {
		return FileTypeNormalCSS
	}
	if filepath.Ext(watcherEventPath) == ".go" {
		return FileTypeGo
	}
	if watcher.IsPublicStaticFile(watcherEventPath) {
		return FileTypePublicStatic
	}
	if watcher.IsPrivateStaticFile(watcherEventPath) {
		return FileTypePrivateStatic
	}
	return FileTypeOther
}

// deriveFileTypeWithWatchedFileOverrides applies per-watched-file overrides
// after extension/pattern-based initial classification.
func deriveFileTypeWithWatchedFileOverrides(
	initialFileType FileType,
	watchedFile *wave.WatchedFile,
) FileType {
	if initialFileType == FileTypeGo &&
		watchedFile != nil &&
		watchedFile.TreatAsNonGo {
		return FileTypeOther
	}
	return initialFileType
}

// deriveWatcherEventIgnoredStatus resolves whether a classified event should be
// ignored before post-classification filtering.
func deriveWatcherEventIgnoredStatus(
	initialIgnoredStatus bool,
	resolvedFileType FileType,
	watchedFile *wave.WatchedFile,
) bool {
	if initialIgnoredStatus {
		return true
	}
	return resolvedFileType == FileTypeOther && watchedFile == nil
}

type implicitBuildExecutionDecision struct {
	ShouldRunImplicitBuild    bool
	SkipImplicitBuildLogEntry string
}

type HookStageResult struct {
	Actions             []wave.RefreshAction
	RefreshActionResult RefreshActionApplicationResult
	StageType           HookStageType
	ExecutionErrors     []error
}

type hookStageContinuationDecision struct {
	ShouldContinue      bool
	StopReason          HookStageContinuationStopReason
	RestartActionResult RefreshActionApplicationResult
}

func (s *Server) ExecuteEventExecutionPlan(
	eventsWithHooks []EventWithHooks,
	behavioralDecision EventExecutionPlanBehavioralDecision,
	work *WorkSet,
	watcher *watch.Watcher,
) {
	if len(eventsWithHooks) == 0 {
		return
	}

	eventsWithHooksForExecution := DeriveEventsWithHooksForExecution(
		eventsWithHooks,
		behavioralDecision.AppStopStrategy,
	)
	if len(eventsWithHooksForExecution) == 0 {
		return
	}

	s.ExecuteAppStopStrategy(behavioralDecision.AppStopStrategy)
	s.ProcessEventsWithDeterministicPipeline(
		behavioralDecision,
		work,
		watcher,
		eventsWithHooksForExecution,
	)
}

func (s *Server) ProcessEventsWithDeterministicPipeline(
	behavioralDecision EventExecutionPlanBehavioralDecision,
	work *WorkSet,
	watcher *watch.Watcher,
	eventsWithHooks []EventWithHooks,
) {
	if len(eventsWithHooks) == 0 {
		return
	}

	s.FireNoWaitHooksForEvents(eventsWithHooks, watcher)

	preHookStageResult := RunAndApplyHookStageActionsAndErrorsToWorkSet(
		HookStageTypePre,
		func() ([]wave.RefreshAction, []error) {
			return s.RunPreHooksForEventsWithErrors(
				eventsWithHooks,
				work,
				watcher,
			)
		},
		work,
	)
	if !s.ContinuePipelineAfterHookStageOrTriggerRestart(
		preHookStageResult,
	) {
		return
	}

	implicitBuildDecision := DeriveImplicitBuildExecutionDecision(
		behavioralDecision.RunImplicitBuild,
		len(eventsWithHooks),
	)
	if !implicitBuildDecision.ShouldRunImplicitBuild {
		s.Log.Info(implicitBuildDecision.SkipImplicitBuildLogEntry)
	} else {
		work.Resolve(s.Cfg.UsingVite())
	}

	buildAndConcurrentHooksContext, cancelBuildAndConcurrentHooks := context.WithCancel(
		s.CurrentRunCycleContextOrBackground(),
	)
	defer cancelBuildAndConcurrentHooks()

	var buildAndConcurrentHooksGroup errgroup.Group
	if implicitBuildDecision.ShouldRunImplicitBuild {
		buildAndConcurrentHooksGroup.Go(func() error {
			buildPhaseError := s.ExecuteBuildPhase(work)
			if buildPhaseError != nil {
				cancelBuildAndConcurrentHooks()
				return buildPhaseError
			}
			return nil
		})
	}

	var concurrentActions []wave.RefreshAction
	var concurrentHookExecutionErrors []error
	buildAndConcurrentHooksGroup.Go(func() error {
		concurrentActions, concurrentHookExecutionErrors = s.RunConcurrentHooksForEventsWithContextAndErrors(
			buildAndConcurrentHooksContext,
			eventsWithHooks,
			watcher,
		)
		return nil
	})
	buildAndConcurrentHooksError := buildAndConcurrentHooksGroup.Wait()
	if buildAndConcurrentHooksError != nil {
		s.Log.Warn(
			"Stopping pipeline after build phase failure",
			"error",
			buildAndConcurrentHooksError,
		)
		return
	}

	concurrentHookStageResult := applyHookStageActionsAndErrorsToWorkSet(
		HookStageTypeConcurrent,
		concurrentActions,
		concurrentHookExecutionErrors,
		work,
	)
	if !s.ContinuePipelineAfterHookStageOrTriggerRestart(
		concurrentHookStageResult,
	) {
		return
	}

	postHookStageResult := RunAndApplyHookStageActionsAndErrorsToWorkSet(
		HookStageTypePost,
		func() ([]wave.RefreshAction, []error) {
			return s.RunPostHooksForEventsWithErrors(eventsWithHooks, watcher)
		},
		work,
	)
	if !s.ContinuePipelineAfterHookStageOrTriggerRestart(
		postHookStageResult,
	) {
		return
	}

	if ShouldStartAppAfterImplicitBuild(
		implicitBuildDecision.ShouldRunImplicitBuild,
		work.Restart,
	) {
		s.Log.Info("Restarting app")
		s.StartApp()
	}

	if ShouldExecuteBrowserPhaseAfterHookStageResults(
		preHookStageResult,
		concurrentHookStageResult,
		postHookStageResult,
	) {
		s.ExecuteBrowserPhase(work)
	}
}

func (s *Server) ExecuteAppStopStrategy(
	appStopStrategyForExecution AppStopStrategy,
) {
	switch appStopStrategyForExecution {
	case AppStopStrategySingleEventHardReload:
		s.Log.Info("Terminating running app")
		if err := s.StopApp(); err != nil {
			s.Log.Error("Failed to terminate app", "error", err)
		}

	case AppStopStrategyBatchHardReload:
		s.Log.Info("Stopping app for batch rebuild")
		if err := s.StopApp(); err != nil {
			s.Log.Error("Failed to stop app", "error", err)
		}

	case AppStopStrategyNone:
	}
}

func (s *Server) ContinuePipelineAfterHookStageOrTriggerRestart(
	hookStageResultForContinuation HookStageResult,
) bool {
	configuredHookStageFailurePolicy := ""
	if s != nil && s.Cfg != nil && s.Cfg.Watch != nil {
		configuredHookStageFailurePolicy = s.Cfg.Watch.HookStageFailurePolicy
	}

	return s.ContinuePipelineAfterHookStageOrTriggerRestartWithFailurePolicy(
		hookStageResultForContinuation,
		DeriveHookStageFailurePolicy(
			hookStageResultForContinuation.StageType,
			configuredHookStageFailurePolicy,
		),
	)
}

func (s *Server) ContinuePipelineAfterHookStageOrTriggerRestartWithFailurePolicy(
	hookStageResultForContinuation HookStageResult,
	hookStageFailurePolicyForContinuation HookStageFailurePolicy,
) bool {
	continuationDecision := DeriveHookStageContinuationDecisionWithFailurePolicy(
		hookStageResultForContinuation,
		hookStageFailurePolicyForContinuation,
	)
	if continuationDecision.ShouldContinue {
		return true
	}

	if continuationDecision.StopReason == HookStageContinuationStopReasonRestartRequested {
		s.TriggerRestartFromRefreshActions(continuationDecision.RestartActionResult)
	}
	if continuationDecision.StopReason == HookStageContinuationStopReasonStageFailure {
		traceContextForContinuation := s.GetCurrentWatcherExecutionTraceContext()
		s.Log.Warn(
			"Stopping pipeline after hook stage errors",
			"stage",
			deriveHookStageLabel(hookStageResultForContinuation.StageType),
			"error_count",
			len(hookStageResultForContinuation.ExecutionErrors),
			"cycle_id",
			traceContextForContinuation.CycleID,
			"batch_id",
			traceContextForContinuation.BatchID,
		)
	}
	return false
}

func (s *Server) TriggerRestartFromRefreshActions(
	actionResult RefreshActionApplicationResult,
) {
	if actionResult.RecompileGo {
		s.TriggerRestart()
		return
	}
	s.TriggerRestartNoGo()
}

func ApplyHookStageActionsToWorkSet(
	hookStageActions []wave.RefreshAction,
	work *WorkSet,
) HookStageResult {
	hookStageResultForWork := HookStageResult{
		Actions: append([]wave.RefreshAction(nil), hookStageActions...),
	}
	if work == nil {
		return hookStageResultForWork
	}

	hookStageResultForWork.RefreshActionResult = work.ApplyRefreshActions(hookStageActions)
	return hookStageResultForWork
}

func applyHookStageActionsAndErrorsToWorkSet(
	stageType HookStageType,
	hookStageActions []wave.RefreshAction,
	hookStageExecutionErrors []error,
	work *WorkSet,
) HookStageResult {
	hookStageResultForWork := ApplyHookStageActionsToWorkSet(
		hookStageActions,
		work,
	)
	hookStageResultForWork.StageType = stageType
	hookStageResultForWork.ExecutionErrors = append(
		[]error(nil),
		hookStageExecutionErrors...,
	)
	return hookStageResultForWork
}

func RunAndApplyHookStageActionsToWorkSet(
	runHookStageActions func() []wave.RefreshAction,
	work *WorkSet,
) HookStageResult {
	if runHookStageActions == nil {
		return ApplyHookStageActionsToWorkSet(nil, work)
	}
	return ApplyHookStageActionsToWorkSet(runHookStageActions(), work)
}

func RunAndApplyHookStageActionsAndErrorsToWorkSet(
	stageType HookStageType,
	runHookStageActions func() ([]wave.RefreshAction, []error),
	work *WorkSet,
) HookStageResult {
	if runHookStageActions == nil {
		return applyHookStageActionsAndErrorsToWorkSet(
			stageType,
			nil,
			nil,
			work,
		)
	}
	hookStageActions, hookStageExecutionErrors := runHookStageActions()
	return applyHookStageActionsAndErrorsToWorkSet(
		stageType,
		hookStageActions,
		hookStageExecutionErrors,
		work,
	)
}

type HookStageFailurePolicy int

const (
	HookStageFailurePolicyFailOpen HookStageFailurePolicy = iota
	HookStageFailurePolicyFailClosed
)

const (
	ConfiguredHookStageFailurePolicyFailOpen   = "fail-open"
	ConfiguredHookStageFailurePolicyFailClosed = "fail-closed"
)

type HookStageContinuationStopReason int

const (
	HookStageContinuationStopReasonNone HookStageContinuationStopReason = iota
	HookStageContinuationStopReasonRestartRequested
	HookStageContinuationStopReasonStageFailure
)

func DeriveImplicitBuildExecutionDecision(
	shouldRunImplicitBuild bool,
	eventCount int,
) implicitBuildExecutionDecision {
	if shouldRunImplicitBuild {
		return implicitBuildExecutionDecision{
			ShouldRunImplicitBuild: true,
		}
	}

	if eventCount == 1 {
		return implicitBuildExecutionDecision{
			SkipImplicitBuildLogEntry: "RunOnChangeOnly: skipping implicit build phase",
		}
	}

	return implicitBuildExecutionDecision{
		SkipImplicitBuildLogEntry: "All events are RunOnChangeOnly, skipping implicit build phase",
	}
}

func ShouldShortCircuitPipelineForHookStageResult(
	hookStageResultForCheck HookStageResult,
) bool {
	return !DeriveHookStageContinuationDecisionWithFailurePolicy(
		hookStageResultForCheck,
		HookStageFailurePolicyFailOpen,
	).ShouldContinue

}

func DeriveHookStageContinuationDecision(
	hookStageResultForContinuation HookStageResult,
) hookStageContinuationDecision {
	return DeriveHookStageContinuationDecisionWithFailurePolicy(
		hookStageResultForContinuation,
		HookStageFailurePolicyFailOpen,
	)
}

func DeriveHookStageContinuationDecisionWithFailurePolicy(
	hookStageResultForContinuation HookStageResult,
	hookStageFailurePolicyForStage HookStageFailurePolicy,
) hookStageContinuationDecision {
	if hookStageResultForContinuation.RefreshActionResult.RestartRequested {
		return hookStageContinuationDecision{
			StopReason:          HookStageContinuationStopReasonRestartRequested,
			RestartActionResult: hookStageResultForContinuation.RefreshActionResult,
		}
	}

	hasHookStageExecutionErrors := len(
		hookStageResultForContinuation.ExecutionErrors,
	) > 0
	if hasHookStageExecutionErrors &&
		hookStageFailurePolicyForStage == HookStageFailurePolicyFailClosed {
		return hookStageContinuationDecision{
			StopReason: HookStageContinuationStopReasonStageFailure,
		}
	}

	return hookStageContinuationDecision{
		ShouldContinue: true,
		StopReason:     HookStageContinuationStopReasonNone,
	}
}

func DeriveHookStageFailurePolicy(
	stageType HookStageType,
	configuredHookStageFailurePolicy string,
) HookStageFailurePolicy {
	resolvedHookStageFailurePolicy := DeriveHookStageFailurePolicyFromConfiguredValue(
		configuredHookStageFailurePolicy,
	)

	switch stageType {
	case HookStageTypePre, HookStageTypeConcurrent, HookStageTypePost:
		return resolvedHookStageFailurePolicy
	default:
		return resolvedHookStageFailurePolicy
	}
}

func DeriveHookStageFailurePolicyFromConfiguredValue(
	configuredHookStageFailurePolicy string,
) HookStageFailurePolicy {
	switch normalizeConfiguredHookStageFailurePolicy(configuredHookStageFailurePolicy) {
	case ConfiguredHookStageFailurePolicyFailClosed:
		return HookStageFailurePolicyFailClosed
	case ConfiguredHookStageFailurePolicyFailOpen, "":
		return HookStageFailurePolicyFailOpen
	default:
		return HookStageFailurePolicyFailOpen
	}
}

func normalizeConfiguredHookStageFailurePolicy(
	configuredHookStageFailurePolicy string,
) string {
	return strings.TrimSpace(strings.ToLower(configuredHookStageFailurePolicy))
}

func ShouldStartAppAfterImplicitBuild(
	shouldRunImplicitBuild bool,
	restart RestartPhaseDecision,
) bool {
	return shouldRunImplicitBuild && restart.RestartApp
}

func ShouldExecuteBrowserPhaseAfterHookStageResults(
	hookStageResults ...HookStageResult,
) bool {
	return !slices.ContainsFunc(
		hookStageResults,
		ShouldShortCircuitPipelineForHookStageResult,
	)
}

func DeriveEventsWithHooksForExecution(
	eventsWithHooks []EventWithHooks,
	appStopStrategyForExecution AppStopStrategy,
) []EventWithHooks {
	if len(eventsWithHooks) == 0 {
		return nil
	}
	if appStopStrategyForExecution != AppStopStrategyBatchHardReload {
		return eventsWithHooks
	}

	executionEventsWithHooks := make([]EventWithHooks, len(eventsWithHooks))
	copy(executionEventsWithHooks, eventsWithHooks)
	for eventIndex := range executionEventsWithHooks {
		executionEventWithHooks := executionEventsWithHooks[eventIndex]
		if executionEventWithHooks.HookCtx == nil {
			continue
		}

		executionHookContext := *executionEventWithHooks.HookCtx
		executionHookContext.AppStoppedForBatch = true
		executionEventWithHooks.HookCtx = &executionHookContext
		executionEventsWithHooks[eventIndex] = executionEventWithHooks
	}

	return executionEventsWithHooks
}
func DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
	eventsWithHooks []EventWithHooks,
) EventExecutionPlanBehavioralDecision {
	return EventExecutionPlanBehavioralDecision{
		ShowRebuildingOverlay: ShouldShowRebuildingOverlayForEventsWithHooks(eventsWithHooks),
		AppStopStrategy:       ResolveAppStopStrategy(eventsWithHooks),
		RunImplicitBuild:      ShouldRunImplicitBuildForEvents(eventsWithHooks),
	}
}

func ShouldShowRebuildingOverlayForEventsWithHooks(
	eventsWithHooks []EventWithHooks,
) bool {
	if len(eventsWithHooks) == 0 {
		return false
	}

	classifiedEvents := make([]ClassifiedEvent, 0, len(eventsWithHooks))
	for _, eventWithHooksForOverlay := range eventsWithHooks {
		classifiedEvents = append(classifiedEvents, eventWithHooksForOverlay.Classified)
	}
	return ShouldShowRebuildingOverlay(classifiedEvents)
}

func BuildWatcherEventLogPayloadsForEventsWithHooks(
	eventsWithHooks []EventWithHooks,
) []WatcherEventLogPayload {
	if len(eventsWithHooks) == 0 {
		return nil
	}

	watcherEventLogPayloads := make(
		[]WatcherEventLogPayload,
		0,
		len(eventsWithHooks),
	)
	for _, eventWithHooksForLogging := range eventsWithHooks {
		watcherEventLogPayloads = append(
			watcherEventLogPayloads,
			WatcherEventLogPayload{
				Operation: eventWithHooksForLogging.Classified.Event.Op.String(),
				FilePath:  eventWithHooksForLogging.Classified.Event.Name,
			},
		)
	}
	return watcherEventLogPayloads
}

func ShouldShowRebuildingOverlay(
	classifiedEvents []ClassifiedEvent,
) bool {
	for _, classifiedEventForOverlay := range classifiedEvents {
		if classifiedEventForOverlay.FileType == FileTypeCriticalCSS ||
			classifiedEventForOverlay.FileType == FileTypeNormalCSS ||
			classifiedEventForOverlay.FileType == FileTypeCriticalAndNormalCSS {
			continue
		}

		if shouldSuppressRebuildingNotificationForClassifiedEvent(
			classifiedEventForOverlay,
		) {
			continue
		}

		return true
	}

	return false
}

func shouldSuppressRebuildingNotificationForClassifiedEvent(
	classifiedEventForOverlay ClassifiedEvent,
) bool {
	watchedFileForOverlay := classifiedEventForOverlay.WatchedFile
	if watchedFileForOverlay == nil {
		return false
	}

	if watchedFileForOverlay.SkipRebuildingNotification {
		return true
	}

	return watchedFileForOverlay.OnlyRunClientDefinedRevalidateFunc &&
		classifiedEventForOverlay.FileType != FileTypeGo &&
		!NeedsHardReload(watchedFileForOverlay)
}

func AnyEventNeedsHardReload(eventsWithHooks []EventWithHooks) bool {
	for _, eventWithHooksForCheck := range eventsWithHooks {
		if eventWithHooksForCheck.NeedsHardReload {
			return true
		}
	}
	return false
}

func ResolveAppStopStrategy(eventsWithHooks []EventWithHooks) AppStopStrategy {
	if len(eventsWithHooks) == 0 {
		return AppStopStrategyNone
	}

	if len(eventsWithHooks) == 1 {
		if eventsWithHooks[0].NeedsHardReload {
			return AppStopStrategySingleEventHardReload
		}
		return AppStopStrategyNone
	}

	if AnyEventNeedsHardReload(eventsWithHooks) {
		return AppStopStrategyBatchHardReload
	}

	return AppStopStrategyNone
}

func ShouldRunImplicitBuildForEvents(eventsWithHooks []EventWithHooks) bool {
	for _, eventWithHooksForCheck := range eventsWithHooks {
		if !eventWithHooksForCheck.RunOnChangeOnly {
			return true
		}
	}
	return false
}
func (s *Server) RunWatcher() {
	s.RunWatcherWithContext(context.Background())
}

func (s *Server) RunWatcherWithContext(
	watcherExecutionContext context.Context,
) {
	s.Mu.Lock()
	watcher := s.Watcher
	s.Mu.Unlock()

	if watcher == nil {
		return
	}

	debouncer := watch.NewDebouncer(30*time.Millisecond, func(events []fsnotify.Event) {
		if watcherExecutionContext != nil {
			select {
			case <-watcherExecutionContext.Done():
				return
			default:
			}
		}
		s.ProcessEvents(events)
	})
	defer debouncer.Stop()

	for {
		select {
		case <-watcherExecutionContext.Done():
			return
		case watcherEvent, ok := <-watcher.Events():
			if !ok {
				return
			}
			debouncer.Add(watcherEvent)
		case watcherError, ok := <-watcher.Errors():
			if !ok {
				return
			}
			if watcherError != nil {
				s.Log.Error("watcher error", "error", watcherError)
			}
		}
	}
}

func (s *Server) ProcessEvents(events []fsnotify.Event) {
	s.Mu.Lock()
	watcher := s.Watcher
	builder := s.Builder
	s.Mu.Unlock()

	if watcher == nil || builder == nil {
		return
	}

	traceContextForWatcherExecution := s.DeriveWatcherExecutionTraceContext()
	s.SetCurrentWatcherExecutionTraceContext(traceContextForWatcherExecution)
	defer s.ClearCurrentWatcherExecutionTraceContext()

	executionPlanningResult := s.BuildEventExecutionPlan(events, watcher, builder)
	watcherEventExecutionInputForPlanningResult := BuildWatcherEventExecutionInputFromPlanningResult(
		executionPlanningResult,
	)
	watcherEventFlowDecisionForPlanningResult := watcherEventExecutionInputForPlanningResult.FlowDecision
	if watcherEventFlowDecisionForPlanningResult.TriggerConfigRestart {
		s.Log.Info("Config changed, restarting")
		s.TriggerConfigRestart()
		return
	}

	eventsWithHooksForExecution := watcherEventExecutionInputForPlanningResult.EventsWithHooks
	if len(eventsWithHooksForExecution) == 0 {
		return
	}

	if watcherEventFlowDecisionForPlanningResult.BroadcastRebuildingOverlay {
		s.BroadcastRebuilding()
	}

	work := &WorkSet{}
	for _, watcherEventLogPayloadForExecutionPlan := range watcherEventExecutionInputForPlanningResult.WatcherEventLogPayloads {
		s.Log.Info(
			"[watcher]",
			"op",
			watcherEventLogPayloadForExecutionPlan.Operation,
			"file",
			watcherEventLogPayloadForExecutionPlan.FilePath,
			"cycle_id",
			traceContextForWatcherExecution.CycleID,
			"batch_id",
			traceContextForWatcherExecution.BatchID,
		)
	}

	s.ExecuteEventExecutionPlan(
		eventsWithHooksForExecution,
		watcherEventFlowDecisionForPlanningResult.BehavioralDecision,
		work,
		watcher,
	)

	watcher.RemoveStale()
}

func BuildWatcherEventExecutionInputFromPlanningResult(
	executionPlanningResult EventExecutionPlanningResult,
) WatcherEventExecutionInput {
	flowDecision := DeriveWatcherEventFlowDecisionFromPlanningResult(
		executionPlanningResult,
	)
	executionInput := WatcherEventExecutionInput{
		FlowDecision: flowDecision,
	}
	if flowDecision.TriggerConfigRestart || len(executionPlanningResult.EventsWithHooks) == 0 {
		return executionInput
	}
	executionInput.EventsWithHooks = executionPlanningResult.EventsWithHooks
	executionInput.WatcherEventLogPayloads = BuildWatcherEventLogPayloadsForEventsWithHooks(
		executionInput.EventsWithHooks,
	)
	return executionInput
}

func DeriveWatcherEventFlowDecisionFromPlanningResult(
	executionPlanningResult EventExecutionPlanningResult,
) WatcherEventFlowDecision {
	if executionPlanningResult.ConfigChanged {
		return WatcherEventFlowDecision{
			TriggerConfigRestart: true,
		}
	}
	if len(executionPlanningResult.EventsWithHooks) == 0 {
		return WatcherEventFlowDecision{}
	}
	behavioralDecisionForExecutionPlan := DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
		executionPlanningResult.EventsWithHooks,
	)

	return WatcherEventFlowDecision{
		BroadcastRebuildingOverlay: behavioralDecisionForExecutionPlan.ShowRebuildingOverlay,
		BehavioralDecision:         behavioralDecisionForExecutionPlan,
	}
}

// resolve determines browser behavior based on build work and user preferences.
func (work *WorkSet) Resolve(usingVite bool) {
	if work.Build.CompileGo {
		work.Restart.RestartApp = true
	}
	work.DetermineBrowserBehavior(usingVite)
}

func (work *WorkSet) DetermineBrowserBehavior(usingVite bool) {
	browserPhaseResolutionForWork := DeriveBrowserPhaseResolutionForWorkSet(
		work.Build,
		work.Restart,
		work.PreferRevalidate,
		usingVite,
	)
	work.requestBrowserAction(browserPhaseResolutionForWork.Action)
	if browserPhaseResolutionForWork.ApplyWaitFlags {
		work.Browser.WaitForApp = browserPhaseResolutionForWork.WaitForApp
		work.Browser.WaitForVite = browserPhaseResolutionForWork.WaitForVite
	}
}

func (work *WorkSet) requestBrowserAction(action BrowserPhaseAction) {
	if action > work.Browser.Action {
		work.Browser.Action = action
	}
}

func DeriveBrowserPhaseResolutionForWorkSet(
	buildDecision BuildPhaseDecision,
	restartDecision RestartPhaseDecision,
	preferRevalidate bool,
	usingVite bool,
) BrowserPhaseResolution {
	if shouldUseRestartBrowserResolution(restartDecision) {
		return BrowserPhaseResolution{
			Action:         BrowserPhaseActionHardReload,
			ApplyWaitFlags: true,
			WaitForApp:     true,
			WaitForVite:    usingVite,
		}
	}

	if shouldUseRevalidateBrowserResolution(preferRevalidate) {
		return BrowserPhaseResolution{
			Action:         BrowserPhaseActionRevalidate,
			ApplyWaitFlags: true,
			WaitForApp:     true,
			WaitForVite:    usingVite,
		}
	}

	if isCSSOnlyBuildWorkForBrowserPhase(buildDecision) {
		return BrowserPhaseResolution{
			Action: BrowserPhaseActionHotReloadCSS,
		}
	}

	if buildDecision.ProcessPublicFiles {
		return BrowserPhaseResolution{
			Action: BrowserPhaseActionInvalidateVite,
		}
	}

	if buildDecision.ProcessPrivateFiles || buildDecision.BuildCriticalCSS || buildDecision.BuildNormalCSS {
		return BrowserPhaseResolution{
			Action:         BrowserPhaseActionHardReload,
			ApplyWaitFlags: true,
			WaitForApp:     true,
			WaitForVite:    usingVite,
		}
	}

	return BrowserPhaseResolution{
		Action: BrowserPhaseActionNone,
	}
}

func shouldUseRestartBrowserResolution(
	restartDecision RestartPhaseDecision,
) bool {
	return restartDecision.RestartApp
}

func shouldUseRevalidateBrowserResolution(
	preferRevalidate bool,
) bool {

	return preferRevalidate
}

func isCSSOnlyBuildWorkForBrowserPhase(
	buildDecision BuildPhaseDecision,
) bool {
	cssWork := buildDecision.BuildCriticalCSS || buildDecision.BuildNormalCSS
	return cssWork &&
		!buildDecision.ProcessPublicFiles &&
		!buildDecision.ProcessPrivateFiles
}

// addImplicitWork adds build work implied by a file type.
func (work *WorkSet) AddImplicitWork(classifiedEventForWork ClassifiedEvent) {
	implicitWorkDecisionForClassifiedEvent := DeriveImplicitWorkDecisionForClassifiedEvent(classifiedEventForWork)
	work.ApplyImplicitWorkDecision(implicitWorkDecisionForClassifiedEvent)
}

func DeriveImplicitWorkDecisionForClassifiedEvent(
	classifiedEventForWork ClassifiedEvent,
) ImplicitWorkDecision {
	watchedFileForWork := classifiedEventForWork.WatchedFile
	if watchedFileForWork != nil && watchedFileForWork.RunOnChangeOnly {
		return ImplicitWorkDecision{}
	}

	decision := ImplicitWorkDecision{}
	if watchedFileForWork != nil && watchedFileForWork.OnlyRunClientDefinedRevalidateFunc {
		decision.PreferRevalidate = true
	}

	switch classifiedEventForWork.FileType {
	case FileTypeGo:
		decision.CompileGo = true
		decision.RestartApp = true

	case FileTypeCriticalCSS:
		decision.BuildCriticalCSS = true
		decision.RestartApp = watchedFileForWork != nil && NeedsHardReload(watchedFileForWork)

	case FileTypeNormalCSS:
		decision.BuildNormalCSS = true
		decision.RestartApp = watchedFileForWork != nil && NeedsHardReload(watchedFileForWork)

	case FileTypeCriticalAndNormalCSS:
		decision.BuildCriticalCSS = true
		decision.BuildNormalCSS = true
		decision.RestartApp = watchedFileForWork != nil && NeedsHardReload(watchedFileForWork)

	case FileTypePublicStatic:
		decision.ProcessPublicFiles = true
		decision.PublicStaticChangedFilePath = classifiedEventForWork.Event.Name

	case FileTypePrivateStatic:
		decision.ProcessPrivateFiles = true
		decision.PrivateStaticChangedFilePath = classifiedEventForWork.Event.Name

	case FileTypeOther:
		if watchedFileForWork != nil {
			decision.CompileGo = watchedFileForWork.RecompileGoBinary
			decision.RestartApp = watchedFileForWork.RestartApp || watchedFileForWork.RecompileGoBinary
		}
	}

	return decision
}

func (work *WorkSet) ApplyImplicitWorkDecision(
	decision ImplicitWorkDecision,
) {
	if decision.PreferRevalidate {
		work.PreferRevalidate = true
	}
	if decision.CompileGo {
		work.Build.CompileGo = true
	}
	if decision.BuildCriticalCSS {
		work.Build.BuildCriticalCSS = true
	}
	if decision.BuildNormalCSS {
		work.Build.BuildNormalCSS = true
	}
	if decision.ProcessPublicFiles {
		work.Build.ProcessPublicFiles = true
		work.Build.addPublicStaticChangedFilePath(decision.PublicStaticChangedFilePath)
	}
	if decision.ProcessPrivateFiles {
		work.Build.ProcessPrivateFiles = true
		work.Build.addPrivateStaticChangedFilePath(decision.PrivateStaticChangedFilePath)
	}
	if decision.RestartApp {
		work.Restart.RestartApp = true
	}
}
func normalizeChangedSourceFilePathForWorkSet(
	filePath string,
) string {
	return waveshared.Absolute(filePath)
}

func appendNormalizedFilePathIfMissing(
	existingFilePaths []string,
	existingFilePathSet map[string]struct{},
	filePath string,
) ([]string, map[string]struct{}) {
	normalizedFilePath := normalizeChangedSourceFilePathForWorkSet(filePath)
	if normalizedFilePath == "" {
		return existingFilePaths, existingFilePathSet
	}

	if existingFilePathSet == nil {
		existingFilePathSet = make(map[string]struct{}, len(existingFilePaths)+1)
		for _, existingFilePath := range existingFilePaths {
			existingFilePathSet[existingFilePath] = struct{}{}
		}
	}
	if _, alreadyExists := existingFilePathSet[normalizedFilePath]; alreadyExists {
		return existingFilePaths, existingFilePathSet
	}

	existingFilePaths = append(existingFilePaths, normalizedFilePath)
	existingFilePathSet[normalizedFilePath] = struct{}{}
	return existingFilePaths, existingFilePathSet
}

func (buildDecision *BuildPhaseDecision) addPublicStaticChangedFilePath(
	filePath string,
) {
	buildDecision.PublicStaticChangedFilePaths, buildDecision.PublicStaticChangedFilePathSet = appendNormalizedFilePathIfMissing(
		buildDecision.PublicStaticChangedFilePaths,
		buildDecision.PublicStaticChangedFilePathSet,
		filePath,
	)
}

func (buildDecision *BuildPhaseDecision) addPrivateStaticChangedFilePath(
	filePath string,
) {
	buildDecision.PrivateStaticChangedFilePaths, buildDecision.PrivateStaticChangedFilePathSet = appendNormalizedFilePathIfMissing(
		buildDecision.PrivateStaticChangedFilePaths,
		buildDecision.PrivateStaticChangedFilePathSet,
		filePath,
	)
}

// addFromRefreshAction merges a RefreshAction from a callback into the work set.
func (work *WorkSet) AddFromRefreshAction(action wave.RefreshAction) {
	workMutationDecision := DeriveRefreshActionWorkMutationDecision(action)
	work.ApplyRefreshActionWorkMutationDecision(workMutationDecision)
}

func DeriveRefreshActionWorkMutationDecision(
	action wave.RefreshAction,
) RefreshActionWorkMutationDecision {
	workMutationDecision := RefreshActionWorkMutationDecision{}
	if action.TriggerRestart {
		workMutationDecision.RestartApp = true
		workMutationDecision.CompileGo = action.RecompileGo
	}
	if action.ReloadBrowser {
		workMutationDecision.RequestBrowserAction = true
		workMutationDecision.BrowserAction = BrowserPhaseActionHardReload
	}
	workMutationDecision.WaitForApp = action.WaitForApp
	workMutationDecision.WaitForVite = action.WaitForVite
	return workMutationDecision
}

func (work *WorkSet) ApplyRefreshActionWorkMutationDecision(
	workMutationDecision RefreshActionWorkMutationDecision,
) {
	if workMutationDecision.RestartApp {
		work.Restart.RestartApp = true
	}
	if workMutationDecision.CompileGo {
		work.Build.CompileGo = true
	}
	if workMutationDecision.RequestBrowserAction {
		work.requestBrowserAction(workMutationDecision.BrowserAction)
	}
	if workMutationDecision.WaitForApp {
		work.Browser.WaitForApp = true
	}
	if workMutationDecision.WaitForVite {
		work.Browser.WaitForVite = true
	}
}

func (work *WorkSet) ApplyRefreshActions(
	actions []wave.RefreshAction,
) RefreshActionApplicationResult {
	reductionDecision := ReduceRefreshActionsInStableOrder(actions)
	for _, action := range reductionDecision.ActionsBeforeRestart {
		work.AddFromRefreshAction(action)
	}

	return reductionDecision.ApplicationResult
}

func ReduceRefreshActionsInStableOrder(
	actions []wave.RefreshAction,
) refreshActionReductionDecision {
	reductionDecision := refreshActionReductionDecision{
		ActionsBeforeRestart: make([]wave.RefreshAction, 0, len(actions)),
		RestartActionIndex:   -1,
	}

	for actionIndex, action := range actions {
		if action.TriggerRestart {
			reductionDecision.RestartActionEncountered = true
			reductionDecision.RestartActionIndex = actionIndex
			reductionDecision.ApplicationResult = RefreshActionApplicationResult{
				RestartRequested: true,
				RecompileGo:      action.RecompileGo,
			}
			return reductionDecision
		}
		reductionDecision.ActionsBeforeRestart = append(reductionDecision.ActionsBeforeRestart, action)
	}

	return reductionDecision
}
func NeedsHardReload(watchedFile *wave.WatchedFile) bool {
	if watchedFile == nil {
		return false
	}
	return watchedFile.RecompileGoBinary || watchedFile.RestartApp
}

func (s *Server) ResolveHookCommand(hook wave.OnChangeHook) string {
	if hook.RunCombinedDevBuildHookCommands {
		if strings.TrimSpace(hook.Cmd) != "" {
			return ResolveSequentialShellCommands(
				hook.Cmd,
				getUserDevBuildHook(s.Cfg),
				getFrameworkDevBuildHook(s.Cfg),
			)
		}
		return ResolveSequentialShellCommands(
			getUserDevBuildHook(s.Cfg),
			getFrameworkDevBuildHook(s.Cfg),
		)
	}
	return hook.Cmd
}

func (s *Server) ResolveHookExecutionPlan(
	hook wave.OnChangeHook,
) HookExecutionPlan {
	if s == nil || s.Cfg == nil {
		return DeriveHookExecutionPlanFromHook(hook, nil)
	}
	if !hook.RunCombinedDevBuildHookCommands || s.Cfg.FrameworkRunBuildHook == nil {
		return DeriveHookExecutionPlanFromHook(hook, s.ResolveHookCommand)
	}

	frameworkBuildHookRunner := s.Cfg.FrameworkRunBuildHook
	userAndExplicitCommand := ResolveSequentialShellCommands(
		hook.Cmd,
		getUserDevBuildHook(s.Cfg),
	)
	originalCallback := hook.Callback

	return HookExecutionPlan{
		Callback: func(hookContext *wave.HookContext) (*wave.RefreshAction, error) {
			var callbackAction *wave.RefreshAction
			var callbackErr error
			if originalCallback != nil {
				callbackAction, callbackErr = originalCallback(hookContext)
				if callbackErr != nil {
					return callbackAction, callbackErr
				}
			}

			hookExecutionContext := context.Background()
			if hookContext != nil && hookContext.ExecutionContext != nil {
				hookExecutionContext = hookContext.ExecutionContext
			}

			if strings.TrimSpace(userAndExplicitCommand) != "" {
				if err := executeHookCommandWithContext(
					hookExecutionContext,
					userAndExplicitCommand,
				); err != nil {
					return callbackAction, err
				}
			}

			if err := frameworkBuildHookRunner(
				hookExecutionContext,
				true,
			); err != nil {
				return callbackAction, err
			}
			return callbackAction, nil
		},
		Command:                     "",
		CommandTimeoutMilliseconds:  hook.CommandTimeoutMilliseconds,
		DisableStageCommandTimeout:  hook.DisableStageCommandTimeout,
		CallbackTimeoutMilliseconds: hook.CallbackTimeoutMilliseconds,
		DisableStageCallbackTimeout: hook.DisableStageCallbackTimeout,
	}
}

func getUserDevBuildHook(parsedConfig *wave.ParsedConfig) string {
	if parsedConfig == nil || parsedConfig.Core == nil {
		return ""
	}
	return parsedConfig.Core.DevBuildHook
}

func getFrameworkDevBuildHook(parsedConfig *wave.ParsedConfig) string {
	if parsedConfig == nil {
		return ""
	}
	return parsedConfig.FrameworkDevBuildHook
}

// resolveSequentialShellCommands combines non-empty shell commands in order.
// The resulting command preserves "fail fast" behavior by chaining with &&.
func ResolveSequentialShellCommands(commands ...string) string {
	nonEmptyCommands := make([]string, 0, len(commands))
	for _, command := range commands {
		trimmedCommand := strings.TrimSpace(command)
		if trimmedCommand != "" {
			nonEmptyCommands = append(nonEmptyCommands, trimmedCommand)
		}
	}
	return strings.Join(nonEmptyCommands, " && ")
}

type hookStageExecutionDescriptor struct {
	EventIndex     int
	EventWithHooks EventWithHooks
}

type HookStageType int

const (
	HookStageTypePre HookStageType = iota
	HookStageTypeConcurrent
	HookStageTypePost
	HookStageTypeConcurrentNoWait
)

type HookExecutionPlan struct {
	Callback                    func(*wave.HookContext) (*wave.RefreshAction, error)
	Command                     string
	CommandTimeoutMilliseconds  int
	DisableStageCommandTimeout  bool
	CallbackTimeoutMilliseconds int
	DisableStageCallbackTimeout bool
}

type HookExecutionPlanResolver func(wave.OnChangeHook) HookExecutionPlan

func DeriveHookStageExecutionDescriptors(
	eventsWithHooks []EventWithHooks,
) []hookStageExecutionDescriptor {
	if len(eventsWithHooks) == 0 {
		return nil
	}

	descriptors := make([]hookStageExecutionDescriptor, 0, len(eventsWithHooks))
	for eventIndex, eventWithHooksForExecution := range eventsWithHooks {
		if eventWithHooksForExecution.SkipDuplicateHooks {
			continue
		}
		descriptors = append(descriptors, hookStageExecutionDescriptor{
			EventIndex:     eventIndex,
			EventWithHooks: eventWithHooksForExecution,
		})
	}
	return descriptors
}

func DeriveStageHooksAndRunOnChangePolicyForEvent(
	eventWithHooksForStage EventWithHooks,
	stageType HookStageType,
) ([]wave.OnChangeHook, bool) {
	if eventWithHooksForStage.Hooks == nil {
		return nil, false
	}

	switch stageType {
	case HookStageTypePre:
		return eventWithHooksForStage.Hooks.Pre, false
	case HookStageTypeConcurrent:
		return eventWithHooksForStage.Hooks.Concurrent, true
	case HookStageTypePost:
		return eventWithHooksForStage.Hooks.Post, true
	case HookStageTypeConcurrentNoWait:
		return eventWithHooksForStage.Hooks.ConcurrentNoWait, false
	default:
		return nil, false
	}
}

func DeriveHookExecutionPlanFromHook(
	hook wave.OnChangeHook,
	ResolveHookCommand func(wave.OnChangeHook) string,
) HookExecutionPlan {
	resolvedCommand := hook.Cmd
	if ResolveHookCommand != nil {
		resolvedCommand = ResolveHookCommand(hook)
	}

	return HookExecutionPlan{
		Callback:                    hook.Callback,
		Command:                     resolvedCommand,
		CommandTimeoutMilliseconds:  hook.CommandTimeoutMilliseconds,
		DisableStageCommandTimeout:  hook.DisableStageCommandTimeout,
		CallbackTimeoutMilliseconds: hook.CallbackTimeoutMilliseconds,
		DisableStageCallbackTimeout: hook.DisableStageCallbackTimeout,
	}
}

func DeriveHookExecutionPlansForEventStage(
	watcher *watch.Watcher,
	eventWithHooksForStage EventWithHooks,
	stageType HookStageType,
	ResolveHookExecutionPlan HookExecutionPlanResolver,
) []HookExecutionPlan {
	stageHooks, shouldApplyRunOnChangeOnlyRules := DeriveStageHooksAndRunOnChangePolicyForEvent(
		eventWithHooksForStage,
		stageType,
	)
	executableHooksForStage := DeriveExecutableHooksForStage(
		watcher,
		eventWithHooksForStage.Classified.Event.Name,
		eventWithHooksForStage.RunOnChangeOnly,
		shouldApplyRunOnChangeOnlyRules,
		stageHooks,
	)
	if len(executableHooksForStage) == 0 {
		return nil
	}

	plans := make([]HookExecutionPlan, 0, len(executableHooksForStage))
	for _, executableHook := range executableHooksForStage {
		if ResolveHookExecutionPlan != nil {
			plans = append(
				plans,
				ResolveHookExecutionPlan(executableHook),
			)
			continue
		}
		plans = append(
			plans,
			DeriveHookExecutionPlanFromHook(executableHook, nil),
		)
	}

	return plans
}

func DeriveExecutableHooksForStage(
	watcher *watch.Watcher,
	eventPath string,
	isRunOnChangeOnly bool,
	shouldApplyRunOnChangeOnlyRules bool,
	stageHooks []wave.OnChangeHook,
) []wave.OnChangeHook {
	if len(stageHooks) == 0 {
		return nil
	}

	hooksForExecution := make([]wave.OnChangeHook, 0, len(stageHooks))
	for _, stageHook := range stageHooks {
		hookForExecution, shouldRunHook := ResolveHookForStageExecution(
			watcher,
			eventPath,
			isRunOnChangeOnly,
			shouldApplyRunOnChangeOnlyRules,
			stageHook,
		)
		if !shouldRunHook {
			continue
		}
		hooksForExecution = append(hooksForExecution, hookForExecution)
	}

	return hooksForExecution
}

func ResolveHookForStageExecution(
	watcher *watch.Watcher,
	eventPath string,
	isRunOnChangeOnly bool,
	shouldApplyRunOnChangeOnlyRules bool,
	hook wave.OnChangeHook,
) (wave.OnChangeHook, bool) {
	if watcher.IsIgnored(eventPath, hook.Exclude) {
		return wave.OnChangeHook{}, false
	}
	if !shouldApplyRunOnChangeOnlyRules {
		return hook, true
	}
	return prepareHookForExecutionWithRunOnChangeOnlyRules(
		isRunOnChangeOnly,
		hook,
	)
}

func prepareHookForExecutionWithRunOnChangeOnlyRules(
	isRunOnChangeOnly bool,
	hook wave.OnChangeHook,
) (wave.OnChangeHook, bool) {
	if !isRunOnChangeOnly || !hookHasCommandAction(hook) {
		return hook, true
	}

	if hook.Callback == nil {
		return wave.OnChangeHook{}, false
	}

	hook.Cmd = ""
	hook.RunCombinedDevBuildHookCommands = false
	return hook, true
}

func hookHasCommandAction(hook wave.OnChangeHook) bool {
	return strings.TrimSpace(hook.Cmd) != "" || hook.RunCombinedDevBuildHookCommands
}

const maxConcurrentNoWaitHookExecutions = 16

func formatHookCallbackPanicError(panicValue any) error {
	return fmt.Errorf("hook callback panicked: %v", panicValue)
}

func deriveHookStageLabel(
	stageType HookStageType,
) string {
	switch stageType {
	case HookStageTypePre:
		return "pre"
	case HookStageTypeConcurrent:
		return "concurrent"
	case HookStageTypePost:
		return "post"
	case HookStageTypeConcurrentNoWait:
		return "concurrent-no-wait"
	default:
		return "unknown"
	}
}

func deriveHookExecutionContext(
	parentHookExecutionContext context.Context,
) context.Context {
	if parentHookExecutionContext == nil {
		return context.Background()
	}
	return parentHookExecutionContext
}

func cloneHookContextForExecution(
	hookContext *wave.HookContext,
	hookExecutionContext context.Context,
) *wave.HookContext {
	if hookContext == nil {
		return &wave.HookContext{
			ExecutionContext: hookExecutionContext,
		}
	}

	clonedHookContext := *hookContext
	clonedHookContext.ChangedFilePaths = append(
		[]string(nil),
		hookContext.ChangedFilePaths...)
	clonedHookContext.ExecutionContext = hookExecutionContext
	return &clonedHookContext
}

func wrapHookExecutionErrorWithStageAndPath(
	stageType HookStageType,
	changedFilePath string,
	hookExecutionError error,
) error {
	if hookExecutionError == nil {
		return nil
	}

	if strings.TrimSpace(changedFilePath) == "" {
		return fmt.Errorf(
			"%s hook failed: %w",
			deriveHookStageLabel(stageType),
			hookExecutionError,
		)
	}

	return fmt.Errorf(
		"%s hook failed for %s: %w",
		deriveHookStageLabel(stageType),
		changedFilePath,
		hookExecutionError,
	)
}

func joinHookExecutionErrorsInOrder(
	hookExecutionErrorsByHookIndex []error,
) error {
	if len(hookExecutionErrorsByHookIndex) == 0 {
		return nil
	}

	orderedHookExecutionErrors := make(
		[]error,
		0,
		len(hookExecutionErrorsByHookIndex),
	)
	for _, hookExecutionError := range hookExecutionErrorsByHookIndex {
		if hookExecutionError == nil {
			continue
		}
		orderedHookExecutionErrors = append(
			orderedHookExecutionErrors,
			hookExecutionError,
		)
	}
	if len(orderedHookExecutionErrors) == 0 {
		return nil
	}
	return errors.Join(orderedHookExecutionErrors...)
}

func executeHookCallbackSafely(
	callback func(*wave.HookContext) (*wave.RefreshAction, error),
	hookContext *wave.HookContext,
) (
	callbackAction *wave.RefreshAction,
	callbackExecutionError error,
) {
	if callback == nil {
		return nil, nil
	}

	defer func() {
		if panicValue := recover(); panicValue != nil {
			callbackExecutionError = formatHookCallbackPanicError(panicValue)
			callbackAction = nil
		}
	}()

	return callback(hookContext)
}

func (s *Server) RunNoWaitHookWithConcurrencyLimit(
	runNoWaitHook func(),
) {
	if s == nil || runNoWaitHook == nil {
		return
	}

	concurrentNoWaitHookExecutionLimiter := s.EnsureConcurrentNoWaitHookExecutionLimiter()
	if concurrentNoWaitHookExecutionLimiter == nil {
		return
	}
	lifecycleExecutionContext := s.GetOrCreateConcurrentNoWaitHookLifecycleContext()

	s.LaunchRunCycleScopedAsyncWorkOrDetached(func(
		context.Context,
	) {
		if lifecycleExecutionContext == nil {
			lifecycleExecutionContext = context.Background()
		}

		select {
		case concurrentNoWaitHookExecutionLimiter <- struct{}{}:
		case <-lifecycleExecutionContext.Done():
			return
		}
		defer func() {
			<-concurrentNoWaitHookExecutionLimiter
		}()

		if !shouldContinueConcurrentHookExecution(lifecycleExecutionContext) {
			return
		}
		runNoWaitHook()
	})
}

func (s *Server) EnsureConcurrentNoWaitHookExecutionLimiter() chan struct{} {
	if s == nil {
		return nil
	}

	s.ConcurrentNoWaitHookExecutionLimiterInitOnce.Do(func() {
		if s.ConcurrentNoWaitHookExecutionLimiter == nil {
			s.ConcurrentNoWaitHookExecutionLimiter = make(
				chan struct{},
				maxConcurrentNoWaitHookExecutions,
			)
		}
	})
	return s.ConcurrentNoWaitHookExecutionLimiter
}

func (s *Server) GetOrCreateConcurrentNoWaitHookLifecycleContext() context.Context {
	if s == nil {
		return context.Background()
	}

	currentRunCycleScope := s.GetCurrentRunCycleScope()
	if currentRunCycleScope != nil {
		return currentRunCycleScope.ExecutionContext
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	if s.ConcurrentNoWaitHookLifecycleCtx == nil ||
		!shouldContinueConcurrentHookExecution(
			s.ConcurrentNoWaitHookLifecycleCtx,
		) {
		s.ConcurrentNoWaitHookLifecycleCtx, s.ConcurrentNoWaitHookLifecycleCancel = context.WithCancel(
			context.Background(),
		)
	}

	return s.ConcurrentNoWaitHookLifecycleCtx
}

func (s *Server) CancelConcurrentNoWaitHookLifecycleContext() {
	if s == nil {
		return
	}

	s.Mu.Lock()
	CancelConcurrentNoWaitHookLifecycleContext := s.ConcurrentNoWaitHookLifecycleCancel
	s.ConcurrentNoWaitHookLifecycleCtx = nil
	s.ConcurrentNoWaitHookLifecycleCancel = nil
	s.Mu.Unlock()

	if CancelConcurrentNoWaitHookLifecycleContext != nil {
		CancelConcurrentNoWaitHookLifecycleContext()
	}
}

func (s *Server) RunSequentialHookStageForEligibleEventsWithErrors(
	eventsWithHooks []EventWithHooks,
	watcher *watch.Watcher,
	runHooksForEvent func(EventWithHooks, *watch.Watcher) ([]wave.RefreshAction, error),
	hookExecutionFailureLogMessage string,
) ([]wave.RefreshAction, []error) {
	if runHooksForEvent == nil {
		return nil, nil
	}

	descriptors := DeriveHookStageExecutionDescriptors(eventsWithHooks)
	allStageActions := make([]wave.RefreshAction, 0)
	stageExecutionErrors := make([]error, 0)
	traceContextForHookStage := s.GetCurrentWatcherExecutionTraceContext()
	for _, descriptor := range descriptors {
		stageActions, err := runHooksForEvent(
			descriptor.EventWithHooks,
			watcher,
		)
		if err != nil {
			s.Log.Error(
				hookExecutionFailureLogMessage,
				"error",
				err,
				"cycle_id",
				traceContextForHookStage.CycleID,
				"batch_id",
				traceContextForHookStage.BatchID,
			)
			stageExecutionErrors = append(stageExecutionErrors, err)
		}
		allStageActions = append(allStageActions, stageActions...)
	}
	return allStageActions, stageExecutionErrors
}

func (s *Server) FireNoWaitHooksForEvents(
	eventsWithHooks []EventWithHooks,
	watcher *watch.Watcher,
) {
	descriptors := DeriveHookStageExecutionDescriptors(eventsWithHooks)
	for _, descriptor := range descriptors {
		s.FireNoWaitHooks(descriptor.EventWithHooks, watcher)
	}
}

func (s *Server) RunPreHooksForEvents(
	eventsWithHooks []EventWithHooks,
	work *WorkSet,
	watcher *watch.Watcher,
) []wave.RefreshAction {
	stageActions, _ := s.RunPreHooksForEventsWithErrors(
		eventsWithHooks,
		work,
		watcher,
	)
	return stageActions
}

func (s *Server) RunPreHooksForEventsWithErrors(
	eventsWithHooks []EventWithHooks,
	work *WorkSet,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, []error) {
	for _, eventWithHooksForPre := range eventsWithHooks {
		work.AddImplicitWork(eventWithHooksForPre.Classified)
	}
	return s.RunSequentialHookStageForEligibleEventsWithErrors(
		eventsWithHooks,
		watcher,
		s.RunPreHooks,
		"Pre-hook execution failed",
	)
}

func (s *Server) RunConcurrentHooksForEvents(
	eventsWithHooks []EventWithHooks,
	watcher *watch.Watcher,
) []wave.RefreshAction {
	concurrentActions, _ := s.RunConcurrentHooksForEventsWithContextAndErrors(
		context.Background(),
		eventsWithHooks,
		watcher,
	)
	return concurrentActions
}

func shouldContinueConcurrentHookExecution(
	concurrentHookExecutionContext context.Context,
) bool {
	if concurrentHookExecutionContext == nil {
		return true
	}

	select {
	case <-concurrentHookExecutionContext.Done():
		return false
	default:
		return true
	}
}

func (s *Server) RunConcurrentHooksForEventsWithContext(
	concurrentHookExecutionContext context.Context,
	eventsWithHooks []EventWithHooks,
	watcher *watch.Watcher,
) []wave.RefreshAction {
	concurrentActions, _ := s.RunConcurrentHooksForEventsWithContextAndErrors(
		concurrentHookExecutionContext,
		eventsWithHooks,
		watcher,
	)
	return concurrentActions
}

func (s *Server) RunConcurrentHooksForEventsWithContextAndErrors(
	concurrentHookExecutionContext context.Context,
	eventsWithHooks []EventWithHooks,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, []error) {
	descriptors := DeriveHookStageExecutionDescriptors(eventsWithHooks)
	traceContextForHookStage := s.GetCurrentWatcherExecutionTraceContext()
	actionsByDescriptorIndex := make([][]wave.RefreshAction, len(descriptors))
	executionErrorsByDescriptorIndex := make([]error, len(descriptors))
	var concurrentHooksGroup errgroup.Group

	for descriptorIndex := range descriptors {
		if !shouldContinueConcurrentHookExecution(
			concurrentHookExecutionContext,
		) {
			break
		}

		descriptorForExecution := descriptors[descriptorIndex]
		descriptorIndexForResult := descriptorIndex
		concurrentHooksGroup.Go(func() error {
			if !shouldContinueConcurrentHookExecution(
				concurrentHookExecutionContext,
			) {
				return nil
			}

			concurrentActions, err := s.RunConcurrentHooksWithContext(
				concurrentHookExecutionContext,
				descriptorForExecution.EventWithHooks,
				watcher,
			)
			if err != nil {
				if shouldContinueConcurrentHookExecution(
					concurrentHookExecutionContext,
				) {
					s.Log.Error(
						"Concurrent hook execution failed",
						"error",
						err,
						"cycle_id",
						traceContextForHookStage.CycleID,
						"batch_id",
						traceContextForHookStage.BatchID,
					)
					executionErrorsByDescriptorIndex[descriptorIndexForResult] = err
				}
			}
			actionsByDescriptorIndex[descriptorIndexForResult] = concurrentActions
			return nil
		})
	}

	_ = concurrentHooksGroup.Wait()

	allConcurrentActions := make([]wave.RefreshAction, 0)
	for _, concurrentActionsForEvent := range actionsByDescriptorIndex {
		allConcurrentActions = append(
			allConcurrentActions,
			concurrentActionsForEvent...)
	}
	concurrentHookExecutionErrors := make([]error, 0)
	for _, concurrentHookExecutionError := range executionErrorsByDescriptorIndex {
		if concurrentHookExecutionError == nil {
			continue
		}
		concurrentHookExecutionErrors = append(
			concurrentHookExecutionErrors,
			concurrentHookExecutionError,
		)
	}

	return allConcurrentActions, concurrentHookExecutionErrors
}

func (s *Server) RunPostHooksForEvents(
	eventsWithHooks []EventWithHooks,
	watcher *watch.Watcher,
) []wave.RefreshAction {
	stageActions, _ := s.RunPostHooksForEventsWithErrors(
		eventsWithHooks,
		watcher,
	)
	return stageActions
}

func (s *Server) RunPostHooksForEventsWithErrors(
	eventsWithHooks []EventWithHooks,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, []error) {
	return s.RunSequentialHookStageForEligibleEventsWithErrors(
		eventsWithHooks,
		watcher,
		s.RunPostHooks,
		"Post-hook execution failed",
	)
}

func (s *Server) FireNoWaitHooks(ewh EventWithHooks, watcher *watch.Watcher) {
	concurrentNoWaitHookLifecycleContext := s.GetOrCreateConcurrentNoWaitHookLifecycleContext()
	traceContextForHookExecution := s.GetCurrentWatcherExecutionTraceContext()

	plans := DeriveHookExecutionPlansForEventStage(
		watcher,
		ewh,
		HookStageTypeConcurrentNoWait,
		s.ResolveHookExecutionPlan,
	)
	for _, plan := range plans {
		if plan.Callback != nil {
			callbackForExecution := plan.Callback
			changedFilePathForExecution := ewh.Classified.Event.Name
			hookCallbackTimeoutForExecution := s.DeriveHookCallbackTimeoutForExecutionPlan(
				HookStageTypeConcurrentNoWait,
				plan,
			)
			s.RunNoWaitHookWithConcurrencyLimit(func() {
				hookCallbackExecutionContext, cancelHookCallbackExecutionContext := DeriveExecutionContextWithOptionalTimeout(
					concurrentNoWaitHookLifecycleContext,
					hookCallbackTimeoutForExecution,
				)
				if cancelHookCallbackExecutionContext != nil {
					defer cancelHookCallbackExecutionContext()
				}

				hookContextForExecution := cloneHookContextForExecution(
					ewh.HookCtx,
					deriveHookExecutionContext(hookCallbackExecutionContext),
				)

				if _, err := executeHookCallbackSafely(
					callbackForExecution,
					hookContextForExecution,
				); err != nil {
					s.Log.Warn(
						"concurrent-no-wait callback failed",
						"stage",
						deriveHookStageLabel(HookStageTypeConcurrentNoWait),
						"path",
						changedFilePathForExecution,
						"error",
						err,
						"cycle_id",
						traceContextForHookExecution.CycleID,
						"batch_id",
						traceContextForHookExecution.BatchID,
					)
				}
			})
		}
		if strings.TrimSpace(plan.Command) != "" {
			commandForExecution := plan.Command
			changedFilePathForExecution := ewh.Classified.Event.Name
			hookCommandTimeoutForExecution := s.DeriveHookCommandTimeoutForExecutionPlan(
				HookStageTypeConcurrentNoWait,
				plan,
			)
			s.RunNoWaitHookWithConcurrencyLimit(func() {
				hookCommandExecutionContext, cancelHookCommandExecutionContext := DeriveExecutionContextWithOptionalTimeout(
					concurrentNoWaitHookLifecycleContext,
					hookCommandTimeoutForExecution,
				)
				if cancelHookCommandExecutionContext != nil {
					defer cancelHookCommandExecutionContext()
				}

				if err := executeHookCommandWithContext(
					hookCommandExecutionContext,
					commandForExecution,
				); err != nil {
					s.Log.Warn(
						"concurrent-no-wait hook failed",
						"stage",
						deriveHookStageLabel(HookStageTypeConcurrentNoWait),
						"path",
						changedFilePathForExecution,
						"cmd",
						commandForExecution,
						"error",
						err,
						"cycle_id",
						traceContextForHookExecution.CycleID,
						"batch_id",
						traceContextForHookExecution.BatchID,
					)
				}
			})
		}
	}
}

func (s *Server) RunPreHooks(
	ewh EventWithHooks,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, error) {
	var actions []wave.RefreshAction

	plans := DeriveHookExecutionPlansForEventStage(
		watcher,
		ewh,
		HookStageTypePre,
		s.ResolveHookExecutionPlan,
	)
	for _, plan := range plans {
		action, err := s.ExecuteHookExecutionPlanWithContext(
			context.Background(),
			HookStageTypePre,
			plan,
			ewh.HookCtx,
		)
		if action != nil {
			actions = append(actions, *action)
		}
		if err != nil {
			return actions, wrapHookExecutionErrorWithStageAndPath(
				HookStageTypePre,
				ewh.Classified.Event.Name,
				err,
			)
		}
	}

	return actions, nil
}

func (s *Server) RunConcurrentHooks(
	ewh EventWithHooks,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, error) {
	return s.RunConcurrentHooksWithContext(context.Background(), ewh, watcher)
}

func (s *Server) RunConcurrentHooksWithContext(
	concurrentHookExecutionContext context.Context,
	ewh EventWithHooks,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, error) {
	plans := DeriveHookExecutionPlansForEventStage(
		watcher,
		ewh,
		HookStageTypeConcurrent,
		s.ResolveHookExecutionPlan,
	)
	if len(plans) == 0 {
		return nil, nil
	}

	actionsByHookIndex := make([]*wave.RefreshAction, len(plans))
	hookExecutionErrorsByHookIndex := make([]error, len(plans))
	var executionGroup errgroup.Group

	for hookIndex := range plans {
		if !shouldContinueConcurrentHookExecution(
			concurrentHookExecutionContext,
		) {
			break
		}

		planForExecution := plans[hookIndex]
		hookIndexForResult := hookIndex
		executionGroup.Go(func() error {
			if !shouldContinueConcurrentHookExecution(
				concurrentHookExecutionContext,
			) {
				return nil
			}

			action, err := s.ExecuteHookExecutionPlanWithContext(
				concurrentHookExecutionContext,
				HookStageTypeConcurrent,
				planForExecution,
				ewh.HookCtx,
			)
			actionsByHookIndex[hookIndexForResult] = action
			if err != nil {
				hookExecutionErrorsByHookIndex[hookIndexForResult] = wrapHookExecutionErrorWithStageAndPath(
					HookStageTypeConcurrent,
					ewh.Classified.Event.Name,
					err,
				)
			}
			return nil
		})
	}

	_ = executionGroup.Wait()
	actions := make([]wave.RefreshAction, 0, len(actionsByHookIndex))
	for _, action := range actionsByHookIndex {
		if action != nil {
			actions = append(actions, *action)
		}
	}

	return actions, joinHookExecutionErrorsInOrder(
		hookExecutionErrorsByHookIndex,
	)
}

func (s *Server) RunPostHooks(
	ewh EventWithHooks,
	watcher *watch.Watcher,
) ([]wave.RefreshAction, error) {
	var actions []wave.RefreshAction

	plans := DeriveHookExecutionPlansForEventStage(
		watcher,
		ewh,
		HookStageTypePost,
		s.ResolveHookExecutionPlan,
	)
	for _, plan := range plans {
		action, err := s.ExecuteHookExecutionPlanWithContext(
			context.Background(),
			HookStageTypePost,
			plan,
			ewh.HookCtx,
		)
		if action != nil {
			actions = append(actions, *action)
		}
		if err != nil {
			return actions, wrapHookExecutionErrorWithStageAndPath(
				HookStageTypePost,
				ewh.Classified.Event.Name,
				err,
			)
		}
	}

	return actions, nil
}

func (s *Server) ExecuteHookExecutionPlan(
	stageType HookStageType,
	plan HookExecutionPlan,
	hookContext *wave.HookContext,
) (*wave.RefreshAction, error) {
	return s.ExecuteHookExecutionPlanWithContext(
		context.Background(),
		stageType,
		plan,
		hookContext,
	)
}

func deriveHookExecutionContextError(
	hookExecutionContext context.Context,
) error {
	if hookExecutionContext == nil {
		return nil
	}
	return hookExecutionContext.Err()
}

func executeHookCommandWithContext(
	hookCommandExecutionContext context.Context,
	command string,
) error {
	return executil.RunShellWithContext(hookCommandExecutionContext, command)
}

func (s *Server) DeriveHookCommandTimeoutForExecutionPlan(
	stageType HookStageType,
	executionPlan HookExecutionPlan,
) time.Duration {
	if s == nil || s.Cfg == nil {
		return DeriveHookCommandTimeoutDurationForExecutionPlan(
			nil,
			stageType,
			executionPlan,
		)
	}
	return DeriveHookCommandTimeoutDurationForExecutionPlan(
		s.Cfg.Watch,
		stageType,
		executionPlan,
	)
}

func (s *Server) DeriveHookCallbackTimeoutForExecutionPlan(
	stageType HookStageType,
	executionPlan HookExecutionPlan,
) time.Duration {
	if s == nil || s.Cfg == nil {
		return DeriveHookCallbackTimeoutDurationForExecutionPlan(
			nil,
			stageType,
			executionPlan,
		)
	}
	return DeriveHookCallbackTimeoutDurationForExecutionPlan(
		s.Cfg.Watch,
		stageType,
		executionPlan,
	)
}

func (s *Server) ExecuteHookExecutionPlanWithContext(
	parentHookExecutionContext context.Context,
	stageType HookStageType,
	plan HookExecutionPlan,
	hookContext *wave.HookContext,
) (*wave.RefreshAction, error) {
	var action *wave.RefreshAction
	if plan.Callback != nil {
		hookCallbackExecutionContext, cancelHookCallbackExecutionContext := DeriveExecutionContextWithOptionalTimeout(
			parentHookExecutionContext,
			s.DeriveHookCallbackTimeoutForExecutionPlan(stageType, plan),
		)
		if cancelHookCallbackExecutionContext != nil {
			defer cancelHookCallbackExecutionContext()
		}

		hookExecutionContext := deriveHookExecutionContext(
			hookCallbackExecutionContext,
		)
		hookContextForExecution := cloneHookContextForExecution(
			hookContext,
			hookExecutionContext,
		)

		callbackAction, err := executeHookCallbackSafely(
			plan.Callback,
			hookContextForExecution,
		)
		if err != nil {
			return nil, err
		}
		action = callbackAction
	}

	if strings.TrimSpace(plan.Command) != "" {
		hookCommandExecutionContext, cancelHookCommandExecutionContext := DeriveExecutionContextWithOptionalTimeout(
			parentHookExecutionContext,
			s.DeriveHookCommandTimeoutForExecutionPlan(stageType, plan),
		)
		if cancelHookCommandExecutionContext != nil {
			defer cancelHookCommandExecutionContext()
		}

		if !shouldContinueConcurrentHookExecution(hookCommandExecutionContext) {
			return action, deriveHookExecutionContextError(
				hookCommandExecutionContext,
			)
		}

		if err := executeHookCommandWithContext(
			hookCommandExecutionContext,
			plan.Command,
		); err != nil {
			return action, err
		}
	}

	return action, nil
}
func BuildEventHooksForProcessing(
	classifiedEvents []ClassifiedEvent,
) []EventWithHooks {
	if len(classifiedEvents) == 0 {
		return nil
	}

	normalizedChangedFilePathsByWatchedPattern := BuildNormalizedChangedFilePathsByWatchedPatternForHookContexts(
		classifiedEvents,
	)
	watchedPatternOccurrenceCount := buildWatchedPatternOccurrenceCountForHookContexts(
		classifiedEvents,
	)
	skipDuplicateHooksByClassifiedEventIndex := BuildSkipDuplicateHooksByClassifiedEventIndex(
		classifiedEvents,
	)

	eventsWithHooks := make([]EventWithHooks, 0, len(classifiedEvents))
	for eventIndex, classifiedEventForProcessing := range classifiedEvents {
		normalizedEventPathForHookContext := NormalizeHookContextPathShape(
			classifiedEventForProcessing.Event.Name,
		)
		changedFilePathsForHookContext := deriveChangedFilePathsForHookContext(
			classifiedEventForProcessing,
			normalizedChangedFilePathsByWatchedPattern,
			watchedPatternOccurrenceCount,
			normalizedEventPathForHookContext,
		)
		eventWithHooksForProcessing := buildEventWithHooksForClassifiedEvent(
			classifiedEventForProcessing,
			skipDuplicateHooksByClassifiedEventIndex[eventIndex],
			changedFilePathsForHookContext,
		)
		eventsWithHooks = append(eventsWithHooks, eventWithHooksForProcessing)
	}

	return eventsWithHooks
}

func BuildNormalizedChangedFilePathsByWatchedPatternForHookContexts(
	classifiedEvents []ClassifiedEvent,
) map[string][]string {
	normalizedChangedFilePathsByWatchedPattern := make(map[string][]string)
	seenNormalizedChangedFilePathsByWatchedPattern := make(map[string]map[string]struct{})

	for _, classifiedEventForProcessing := range classifiedEvents {
		watchedFileForProcessing := classifiedEventForProcessing.WatchedFile
		if watchedFileForProcessing == nil {
			continue
		}
		normalizedEventPathForHookContext := NormalizeHookContextPathShape(
			classifiedEventForProcessing.Event.Name,
		)
		recordChangedPathForWatchedPatternIfNew(
			watchedFileForProcessing.Pattern,
			normalizedEventPathForHookContext,
			normalizedChangedFilePathsByWatchedPattern,
			seenNormalizedChangedFilePathsByWatchedPattern,
		)
	}

	return normalizedChangedFilePathsByWatchedPattern
}

func BuildSkipDuplicateHooksByClassifiedEventIndex(
	classifiedEvents []ClassifiedEvent,
) []bool {
	skipDuplicateHooksByClassifiedEventIndex := make([]bool, len(classifiedEvents))
	handledWatchedPatterns := make(map[string]struct{})

	for eventIndex, classifiedEventForProcessing := range classifiedEvents {
		watchedFileForProcessing := classifiedEventForProcessing.WatchedFile
		if watchedFileForProcessing == nil {
			continue
		}

		watchedPattern := watchedFileForProcessing.Pattern
		if _, alreadyHandled := handledWatchedPatterns[watchedPattern]; alreadyHandled {
			skipDuplicateHooksByClassifiedEventIndex[eventIndex] = true
			continue
		}
		handledWatchedPatterns[watchedPattern] = struct{}{}
	}

	return skipDuplicateHooksByClassifiedEventIndex
}

func buildWatchedPatternOccurrenceCountForHookContexts(
	classifiedEvents []ClassifiedEvent,
) map[string]int {
	watchedPatternOccurrenceCount := make(map[string]int)
	for _, classifiedEventForProcessing := range classifiedEvents {
		watchedFileForProcessing := classifiedEventForProcessing.WatchedFile
		if watchedFileForProcessing == nil {
			continue
		}
		watchedPatternOccurrenceCount[watchedFileForProcessing.Pattern]++
	}
	return watchedPatternOccurrenceCount
}

func deriveChangedFilePathsForHookContext(
	classifiedEventForProcessing ClassifiedEvent,
	normalizedChangedFilePathsByWatchedPattern map[string][]string,
	watchedPatternOccurrenceCount map[string]int,
	normalizedEventPathForHookContext string,
) []string {
	watchedFileForProcessing := classifiedEventForProcessing.WatchedFile
	if watchedFileForProcessing == nil {
		return []string{normalizedEventPathForHookContext}
	}

	watchedPattern := watchedFileForProcessing.Pattern
	normalizedChangedFilePathsForWatchedPattern := normalizedChangedFilePathsByWatchedPattern[watchedPattern]
	if len(normalizedChangedFilePathsForWatchedPattern) == 0 {
		return []string{normalizedEventPathForHookContext}
	}

	if watchedPatternOccurrenceCount[watchedPattern] <= 1 {
		return normalizedChangedFilePathsForWatchedPattern
	}

	return append(
		[]string(nil),
		normalizedChangedFilePathsForWatchedPattern...,
	)
}

func buildEventWithHooksForClassifiedEvent(
	classifiedEventForProcessing ClassifiedEvent,
	skipDuplicateHooks bool,
	changedFilePathsForHookContext []string,
) EventWithHooks {
	watchedFileForProcessing := classifiedEventForProcessing.WatchedFile
	if watchedFileForProcessing == nil {
		watchedFileForProcessing = &wave.WatchedFile{}
	}
	sortedHooksForProcessing := deriveSortedHooksForWatchedFileWithoutMutation(
		watchedFileForProcessing,
	)

	normalizedEventPathForHookContext := NormalizeHookContextPathShape(
		classifiedEventForProcessing.Event.Name,
	)
	if len(changedFilePathsForHookContext) == 0 {
		changedFilePathsForHookContext = []string{normalizedEventPathForHookContext}
	}
	eventNeedsHardReload := classifiedEventForProcessing.FileType == FileTypeGo ||
		NeedsHardReload(watchedFileForProcessing)

	return EventWithHooks{
		Classified:         classifiedEventForProcessing,
		Hooks:              sortedHooksForProcessing,
		RunOnChangeOnly:    watchedFileForProcessing.RunOnChangeOnly,
		NeedsHardReload:    eventNeedsHardReload,
		SkipDuplicateHooks: skipDuplicateHooks,
		HookCtx: &wave.HookContext{
			FilePath:           normalizedEventPathForHookContext,
			ChangedFilePaths:   changedFilePathsForHookContext,
			AppStoppedForBatch: false,
		},
	}
}

func deriveSortedHooksForWatchedFileWithoutMutation(
	watchedFileForProcessing *wave.WatchedFile,
) *wave.SortedHooks {
	if watchedFileForProcessing == nil {
		return &wave.SortedHooks{}
	}

	if watchedFileForProcessing.SortedHooks != nil {
		return watch.CloneSortedHooksForWatcherPlan(
			watchedFileForProcessing.SortedHooks,
		)
	}

	return watch.DeriveSortedHooksForWatcherPlan(
		watchedFileForProcessing.OnChangeHooks,
	)
}

func NormalizeHookContextPathShape(path string) string {
	return waveshared.Absolute(path)
}

func recordChangedPathForWatchedPatternIfNew(
	watchedPattern string,
	normalizedChangedPath string,
	changedFilePathsByWatchedPattern map[string][]string,
	seenChangedFilePathsByWatchedPattern map[string]map[string]struct{},
) {
	if changedFilePathsByWatchedPattern == nil || seenChangedFilePathsByWatchedPattern == nil {
		return
	}

	seenChangedPathsForWatchedPattern, hasSeenSet := seenChangedFilePathsByWatchedPattern[watchedPattern]
	if !hasSeenSet || seenChangedPathsForWatchedPattern == nil {
		seenChangedPathsForWatchedPattern = make(map[string]struct{})
		seenChangedFilePathsByWatchedPattern[watchedPattern] = seenChangedPathsForWatchedPattern
	}

	if _, alreadyTracked := seenChangedPathsForWatchedPattern[normalizedChangedPath]; alreadyTracked {
		return
	}
	seenChangedPathsForWatchedPattern[normalizedChangedPath] = struct{}{}
	changedFilePathsByWatchedPattern[watchedPattern] = append(
		changedFilePathsByWatchedPattern[watchedPattern],
		normalizedChangedPath,
	)
}
func deriveTimeoutDurationFromMilliseconds(
	timeoutMilliseconds int,
) time.Duration {
	if timeoutMilliseconds <= 0 {
		return 0
	}
	return time.Duration(timeoutMilliseconds) * time.Millisecond
}

func DeriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
	stageTimeoutMilliseconds int,
	executionTimeoutMilliseconds int,
	disableStageTimeout bool,
) time.Duration {
	if disableStageTimeout {
		return 0
	}
	if executionTimeoutMilliseconds > 0 {
		return deriveTimeoutDurationFromMilliseconds(
			executionTimeoutMilliseconds,
		)
	}
	return deriveTimeoutDurationFromMilliseconds(stageTimeoutMilliseconds)
}

func DeriveHookCommandStageTimeoutMilliseconds(
	watchConfig *wave.WatchConfig,
	stageType HookStageType,
) int {
	if watchConfig == nil {
		return 0
	}

	return deriveHookStageTimeoutMilliseconds(
		stageType,
		hookStageTimeoutMilliseconds{
			Pre:              watchConfig.HookCommandTimeouts.PreCommandTimeoutMilliseconds,
			Concurrent:       watchConfig.HookCommandTimeouts.ConcurrentCommandTimeoutMilliseconds,
			Post:             watchConfig.HookCommandTimeouts.PostCommandTimeoutMilliseconds,
			ConcurrentNoWait: watchConfig.HookCommandTimeouts.ConcurrentNoWaitCommandTimeoutMilliseconds,
		},
	)
}

func DeriveHookCallbackStageTimeoutMilliseconds(
	watchConfig *wave.WatchConfig,
	stageType HookStageType,
) int {
	if watchConfig == nil {
		return 0
	}

	return deriveHookStageTimeoutMilliseconds(
		stageType,
		hookStageTimeoutMilliseconds{
			Pre:              watchConfig.HookCallbackTimeouts.PreCallbackTimeoutMilliseconds,
			Concurrent:       watchConfig.HookCallbackTimeouts.ConcurrentCallbackTimeoutMilliseconds,
			Post:             watchConfig.HookCallbackTimeouts.PostCallbackTimeoutMilliseconds,
			ConcurrentNoWait: watchConfig.HookCallbackTimeouts.ConcurrentNoWaitCallbackTimeoutMilliseconds,
		},
	)
}

type hookStageTimeoutMilliseconds struct {
	Pre              int
	Concurrent       int
	Post             int
	ConcurrentNoWait int
}

func deriveHookStageTimeoutMilliseconds(
	stageType HookStageType,
	timeoutMillisecondsByStage hookStageTimeoutMilliseconds,
) int {
	switch stageType {
	case HookStageTypePre:
		return timeoutMillisecondsByStage.Pre
	case HookStageTypeConcurrent:
		return timeoutMillisecondsByStage.Concurrent
	case HookStageTypePost:
		return timeoutMillisecondsByStage.Post
	case HookStageTypeConcurrentNoWait:
		return timeoutMillisecondsByStage.ConcurrentNoWait
	default:
		return 0
	}
}

func DeriveHookCommandTimeoutDurationForExecutionPlan(
	watchConfig *wave.WatchConfig,
	stageType HookStageType,
	executionPlan HookExecutionPlan,
) time.Duration {
	return DeriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
		DeriveHookCommandStageTimeoutMilliseconds(watchConfig, stageType),
		executionPlan.CommandTimeoutMilliseconds,
		executionPlan.DisableStageCommandTimeout,
	)
}

func DeriveHookCallbackTimeoutDurationForExecutionPlan(
	watchConfig *wave.WatchConfig,
	stageType HookStageType,
	executionPlan HookExecutionPlan,
) time.Duration {
	return DeriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
		DeriveHookCallbackStageTimeoutMilliseconds(watchConfig, stageType),
		executionPlan.CallbackTimeoutMilliseconds,
		executionPlan.DisableStageCallbackTimeout,
	)
}

func deriveBuildHookCommandTimeoutMilliseconds(
	coreConfig *wave.CoreConfig,
	isDev bool,
) int {
	if coreConfig == nil {
		return 0
	}

	if isDev {
		return coreConfig.DevBuildHookTimeoutMilliseconds
	}
	return coreConfig.ProdBuildHookTimeoutMilliseconds
}

func DeriveBuildHookCommandTimeoutDuration(
	coreConfig *wave.CoreConfig,
	isDev bool,
) time.Duration {
	return DeriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
		deriveBuildHookCommandTimeoutMilliseconds(coreConfig, isDev),
		0,
		false,
	)
}

func DeriveExecutionContextWithOptionalTimeout(
	parentExecutionContext context.Context,
	executionTimeoutDuration time.Duration,
) (
	executionContext context.Context,
	cancelExecutionContext context.CancelFunc,
) {
	if executionTimeoutDuration <= 0 {
		if parentExecutionContext == nil {
			return context.Background(), nil
		}
		return parentExecutionContext, nil
	}

	if parentExecutionContext == nil {
		return context.WithTimeout(context.Background(), executionTimeoutDuration)
	}
	return context.WithTimeout(parentExecutionContext, executionTimeoutDuration)
}
