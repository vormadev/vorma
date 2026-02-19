package devserver

import (
	"context"
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/lab/vitecmd"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/waveshared"
	"github.com/vormadev/vorma/wave/tooling_2/broadcast"
	"github.com/vormadev/vorma/wave/tooling_2/builder"
	"github.com/vormadev/vorma/wave/tooling_2/devserver/eventpipeline"
	"github.com/vormadev/vorma/wave/tooling_2/devserver/executionengine"
	"github.com/vormadev/vorma/wave/tooling_2/devserver/hooks"
	"github.com/vormadev/vorma/wave/tooling_2/devserver/restartengine"
	"github.com/vormadev/vorma/wave/tooling_2/devserver/runtimeprocess"
	"github.com/vormadev/vorma/wave/tooling_2/toolingshared"
	"github.com/vormadev/vorma/wave/tooling_2/watch"
	"github.com/vormadev/vorma/wave/tooling_2/watch/classification"
	"github.com/vormadev/vorma/wave/tooling_2/watch/dedup"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultRefreshPort = 10000

// restartRequest signals what kind of restart is needed
type restartRequest = restartengine.RestartRequest

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
	AppProcessManager *runtimeprocess.AppProcessManager
	ViteCtx           *vitecmd.BuildCtx
	Builder           *toolingbuilder.Builder

	// Browser refresh
	RefreshServer    *http.Server
	RefreshMgr       *broadcast.Manager
	RefreshMgrCtx    context.Context
	RefreshMgrCancel context.CancelFunc

	// Lifecycle restart intents
	RestartIntents *restartengine.RestartIntentAccumulator

	// Concurrent-no-wait hook execution gate
	ConcurrentNoWaitHookExecutionLimiter         chan struct{}
	ConcurrentNoWaitHookExecutionLimiterInitOnce sync.Once
	ConcurrentNoWaitHookLifecycleCtx             context.Context
	ConcurrentNoWaitHookLifecycleCancel          context.CancelFunc

	// watcher control - used to delay watcher start until after config restart reload
	WatcherStartCh chan struct{}

	// Cycle-scoped async lifecycle management
	NextRunCycleID       uint64
	CurrentRunCycleScope *restartengine.RunCycleScope

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
			hooks.MaxConcurrentNoWaitHookExecutions,
		),
	}
	s.RestartIntents = restartengine.NewRestartIntentAccumulator(make(chan restartengine.RestartRequest, 1))

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

// GetWatcher returns the current watcher instance safely.
func (s *Server) GetWatcher() *watch.Watcher {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return s.Watcher
}

func (s *Server) StartRunCycleScope() *restartengine.RunCycleScope {
	if s == nil {
		return nil
	}

	s.Mu.Lock()
	s.NextRunCycleID++
	cycleScope := restartengine.NewRunCycleScope(s.NextRunCycleID)
	s.CurrentRunCycleScope = cycleScope
	s.Mu.Unlock()

	return cycleScope
}

func (s *Server) GetCurrentRunCycleScope() *restartengine.RunCycleScope {
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

func (s *Server) BuildExecutionEngine() *executionengine.Engine {
	if s == nil {
		return executionengine.New(executionengine.Dependencies{})
	}

	return executionengine.New(executionengine.Dependencies{
		Log:                                s.Log,
		Config:                             s.Cfg,
		GetCurrentWatcher:                  s.GetWatcher,
		GetCurrentBuilder:                  s.GetBuilder,
		CurrentRunCycleContextOrBackground: s.CurrentRunCycleContextOrBackground,
		ExecuteBuildPhase:                  s.ExecuteBuildPhase,
		ExecuteBrowserPhase:                s.ExecuteBrowserPhase,
		StartApp:                           s.StartApp,
		StopApp:                            s.StopApp,
		TriggerRestart:                     s.TriggerRestart,
		TriggerRestartNoGo:                 s.TriggerRestartNoGo,
		TriggerConfigRestart:               s.TriggerConfigRestart,
		BroadcastRebuilding:                s.BroadcastRebuilding,
		BuildEventExecutionPlan:            s.BuildEventExecutionPlan,
		DeriveWatcherExecutionTraceContext: func() executionengine.WatcherExecutionTraceContext {
			traceContext := s.DeriveWatcherExecutionTraceContext()
			return executionengine.WatcherExecutionTraceContext{
				CycleID: traceContext.CycleID,
				BatchID: traceContext.BatchID,
			}
		},
		SetCurrentWatcherExecutionTraceContext: func(
			traceContext executionengine.WatcherExecutionTraceContext,
		) {
			s.SetCurrentWatcherExecutionTraceContext(WatcherExecutionTraceContext{
				CycleID: traceContext.CycleID,
				BatchID: traceContext.BatchID,
			})
		},
		ClearCurrentWatcherExecutionTraceContext: s.ClearCurrentWatcherExecutionTraceContext,
		GetCurrentWatcherExecutionTraceContext: func() executionengine.WatcherExecutionTraceContext {
			traceContext := s.GetCurrentWatcherExecutionTraceContext()
			return executionengine.WatcherExecutionTraceContext{
				CycleID: traceContext.CycleID,
				BatchID: traceContext.BatchID,
			}
		},
		RunNoWaitHookWithConcurrencyLimit:               s.RunNoWaitHookWithConcurrencyLimit,
		GetOrCreateConcurrentNoWaitHookLifecycleContext: s.GetOrCreateConcurrentNoWaitHookLifecycleContext,
	})
}

func (s *Server) EnsureAppProcessManager() *runtimeprocess.AppProcessManager {
	if s == nil {
		return nil
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	if s.AppProcessManager == nil {
		s.AppProcessManager = runtimeprocess.NewAppProcessManager()
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

const localReadinessProbeHostIPv4 = runtimeprocess.LocalReadinessProbeHostIPv4
const localReadinessProbeHostLocalhost = runtimeprocess.LocalReadinessProbeHostLocalhost

func (s *Server) WaitForApp() bool {
	url := runtimeprocess.ResolveAppReadyURL(s.MustGetPort(), s.Cfg.HealthcheckEndpoint())
	ok := s.WaitForReady(url)
	if !ok {
		s.Log.Warn("App did not become ready in time", "url", url)
	}
	return ok
}

func (s *Server) WaitForReady(url string) bool {
	return s.WaitForAnyReady([]string{url})
}

func (s *Server) WaitForAnyReady(urls []string) bool {
	policy := runtimeprocess.DefaultReadinessWaitPolicy()
	return runtimeprocess.WaitForAnyReady(urls, policy)
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

type restartIntentAccumulator = restartengine.RestartIntentAccumulator

func (s *Server) GetOrCreateRestartIntentAccumulator() *restartIntentAccumulator {
	if s == nil {
		return nil
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	if s.RestartIntents == nil {
		s.RestartIntents = restartengine.NewRestartIntentAccumulator(make(chan restartRequest, 1))
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
	incomingRequest := restartengine.NormalizeRestartRequest(restartRequest{
		RecompileGo:     recompileGo,
		IsConfigRestart: isConfigRestart,
	})
	s.QueueRestartRequest(incomingRequest)
}

type runIntent = restartengine.RunIntent

const (
	runLifecycleCommandPrepareCycle        = restartengine.RunLifecycleCommandPrepareCycle
	runLifecycleCommandBuildCycle          = restartengine.RunLifecycleCommandBuildCycle
	runLifecycleCommandAwaitBuildRetry     = restartengine.RunLifecycleCommandAwaitBuildRetry
	runLifecycleCommandStartRuntime        = restartengine.RunLifecycleCommandStartRuntime
	runLifecycleCommandAwaitRestartRequest = restartengine.RunLifecycleCommandAwaitRestartRequest
	runLifecycleCommandCleanupForNextCycle = restartengine.RunLifecycleCommandCleanupForNextCycle
)

type runLifecycleCommand = restartengine.RunLifecycleCommand

type runLifecycleCommandInput = restartengine.RunLifecycleCommandInput
type runLifecycleCommandResult = restartengine.RunLifecycleCommandResult

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
		runLifecycleCommandForState, err := restartengine.DeriveRunLifecycleCommandForState(
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

		nextRunLifecycleState, err := restartengine.TransitionRunLifecycleState(
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
	buildExecutionOrderingDecision := restartengine.DeriveRunBuildExecutionOrderingDecision(
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
	executionEngine := s.BuildExecutionEngine()

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
			executionEngine.RunWatcherWithContext(currentRunCycleContext)
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
		normalizedPendingRestartRequest := restartengine.NormalizeRestartRequest(
			pendingRestartRequest,
		)
		s.CleanupForRebuild()
		return normalizedPendingRestartRequest
	}

	s.WatcherStartCh = make(chan struct{})
	buildRetryRunCycleScope := s.StartRunCycleScope()
	executionEngine := s.BuildExecutionEngine()

	if buildRetryRunCycleScope != nil {
		buildRetryRunCycleScope.LaunchAsyncWork(func(
			buildRetryRunCycleContext context.Context,
		) {
			select {
			case <-s.WatcherStartCh:
			case <-buildRetryRunCycleContext.Done():
				return
			}
			executionEngine.RunWatcherWithContext(buildRetryRunCycleContext)
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
	currentRunIntentForRetry := restartengine.DeriveRunIntentFromRestartRequest(
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
	currentRunIntentForRestart := restartengine.DeriveRunIntentFromRestartRequest(
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
type RunLifecycleState = restartengine.RunLifecycleState

const (
	RunLifecycleStatePreparingCycle         = restartengine.RunLifecycleStatePreparingCycle
	RunLifecycleStateBuildingCycle          = restartengine.RunLifecycleStateBuildingCycle
	RunLifecycleStateAwaitingBuildRetry     = restartengine.RunLifecycleStateAwaitingBuildRetry
	RunLifecycleStateStartingRuntime        = restartengine.RunLifecycleStateStartingRuntime
	RunLifecycleStateAwaitingRestart        = restartengine.RunLifecycleStateAwaitingRestart
	RunLifecycleStateCleaningUpForNextCycle = restartengine.RunLifecycleStateCleaningUpForNextCycle
)

// RunLifecycleEvent defines one transition trigger inside the Run loop state
// machine.
type RunLifecycleEvent = restartengine.RunLifecycleEvent

const (
	RunLifecycleEventCyclePrepared             = restartengine.RunLifecycleEventCyclePrepared
	RunLifecycleEventBuildSucceeded            = restartengine.RunLifecycleEventBuildSucceeded
	RunLifecycleEventBuildFailed               = restartengine.RunLifecycleEventBuildFailed
	RunLifecycleEventBuildRetryRestartReceived = restartengine.RunLifecycleEventBuildRetryRestartReceived
	RunLifecycleEventRuntimeStarted            = restartengine.RunLifecycleEventRuntimeStarted
	RunLifecycleEventRestartRequestReceived    = restartengine.RunLifecycleEventRestartRequestReceived
	RunLifecycleEventCleanupCompleted          = restartengine.RunLifecycleEventCleanupCompleted
)

// TransitionLogger is the minimal logging capability needed during lifecycle
// transitions.
type TransitionLogger = restartengine.TransitionLogger

// DeriveRunLifecycleStateAfterEvent computes the next state from the current
// state and transition event.
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

func ResolveViteReadyURLs(vitePort int) []string {
	return []string{
		runtimeprocess.ResolveReadinessProbeURL(
			localReadinessProbeHostIPv4,
			vitePort,
			"/@vite/client",
		),
		runtimeprocess.ResolveReadinessProbeURL(
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

type FileType = eventpipeline.FileType

const (
	FileTypeOther                = eventpipeline.FileTypeOther
	FileTypeGo                   = eventpipeline.FileTypeGo
	FileTypeCriticalCSS          = eventpipeline.FileTypeCriticalCSS
	FileTypeNormalCSS            = eventpipeline.FileTypeNormalCSS
	FileTypeCriticalAndNormalCSS = eventpipeline.FileTypeCriticalAndNormalCSS
	FileTypePublicStatic         = eventpipeline.FileTypePublicStatic
	FileTypePrivateStatic        = eventpipeline.FileTypePrivateStatic
)

type ClassifiedEvent = eventpipeline.ClassifiedEvent

// EventWithHooks pairs a classified event with its sorted hooks.
type EventWithHooks = eventpipeline.EventWithHooks

type BuildPhaseDecision = eventpipeline.BuildPhaseDecision
type RestartPhaseDecision = eventpipeline.RestartPhaseDecision
type BrowserPhaseAction = eventpipeline.BrowserPhaseAction

const (
	BrowserPhaseActionNone           = eventpipeline.BrowserPhaseActionNone
	BrowserPhaseActionHotReloadCSS   = eventpipeline.BrowserPhaseActionHotReloadCSS
	BrowserPhaseActionRevalidate     = eventpipeline.BrowserPhaseActionRevalidate
	BrowserPhaseActionHardReload     = eventpipeline.BrowserPhaseActionHardReload
	BrowserPhaseActionInvalidateVite = eventpipeline.BrowserPhaseActionInvalidateVite
)

type BrowserPhaseDecision = eventpipeline.BrowserPhaseDecision
type BrowserPhaseResolution = eventpipeline.BrowserPhaseResolution

// WorkSet collects per-phase execution decisions for a watcher cycle.
type WorkSet = eventpipeline.WorkSet

type AppStopStrategy = eventpipeline.AppStopStrategy

const (
	AppStopStrategyNone                  = eventpipeline.AppStopStrategyNone
	AppStopStrategySingleEventHardReload = eventpipeline.AppStopStrategySingleEventHardReload
	AppStopStrategyBatchHardReload       = eventpipeline.AppStopStrategyBatchHardReload
)

type EventExecutionPlanningResult = eventpipeline.EventExecutionPlanningResult
type WatcherEventFlowDecision = eventpipeline.WatcherEventFlowDecision
type WatcherEventExecutionInput = eventpipeline.WatcherEventExecutionInput
type EventExecutionPlanBehavioralDecision = eventpipeline.EventExecutionPlanBehavioralDecision
type WatcherEventLogPayload = eventpipeline.WatcherEventLogPayload
type RefreshActionApplicationResult = eventpipeline.RefreshActionApplicationResult
type RefreshActionWorkMutationDecision = eventpipeline.RefreshActionWorkMutationDecision
type ImplicitWorkDecision = eventpipeline.ImplicitWorkDecision

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

	eventsWithHooks := hooks.BuildEventHooksForProcessing(classifiedEvents)
	if len(eventsWithHooks) == 0 {
		return EventExecutionPlanningResult{}
	}

	return EventExecutionPlanningResult{
		EventsWithHooks: eventsWithHooks,
	}
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

type BrowserPhaseExecutionCategory = eventpipeline.BrowserPhaseExecutionCategory

const (
	BrowserPhaseExecutionCategoryNone         = eventpipeline.BrowserPhaseExecutionCategoryNone
	BrowserPhaseExecutionCategoryReload       = eventpipeline.BrowserPhaseExecutionCategoryReload
	BrowserPhaseExecutionCategoryHotReloadCSS = eventpipeline.BrowserPhaseExecutionCategoryHotReloadCSS
)

func (s *Server) ExecuteBrowserPhase(work *WorkSet) {
	if !s.Cfg.UsingBrowser() {
		return
	}

	builder := s.GetBuilder()
	browserDecisionForExecution := work.Browser
	if browserDecisionForExecution.Action == BrowserPhaseActionInvalidateVite {
		if eventpipeline.ShouldAttemptViteInvalidateForBrowserDecision(
			browserDecisionForExecution,
			s.Cfg.UsingVite(),
		) {
			if err := s.CallViteFilemapInvalidate(); err != nil {
				s.Log.Warn("Vite filemap invalidate failed, falling back to reload", "error", err)
			} else {
				return
			}
		}
		browserDecisionForExecution = eventpipeline.ResolveBrowserDecisionAfterInvalidateViteFallback(
			browserDecisionForExecution,
			s.Cfg.UsingVite(),
		)
		work.Browser = browserDecisionForExecution
	}

	switch eventpipeline.DeriveBrowserPhaseExecutionCategory(browserDecisionForExecution.Action) {
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

	payloads := eventpipeline.PlanHotReloadCSSPayloads(
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

type StaticFileProcessingExecutionMode = eventpipeline.StaticFileProcessingExecutionMode

const (
	StaticFileProcessingExecutionModeNone         = eventpipeline.StaticFileProcessingExecutionModeNone
	StaticFileProcessingExecutionModeFullScan     = eventpipeline.StaticFileProcessingExecutionModeFullScan
	StaticFileProcessingExecutionModeChangedPaths = eventpipeline.StaticFileProcessingExecutionModeChangedPaths
)

type StaticFileProcessingExecutionDecision = eventpipeline.StaticFileProcessingExecutionDecision
type BuildPhaseExecutionDecision = eventpipeline.BuildPhaseExecutionDecision

func (s *Server) ExecuteBuildPhase(work *WorkSet) error {
	if work == nil {
		return nil
	}

	buildExecutionDecision := eventpipeline.DeriveBuildPhaseExecutionDecision(
		work.Build,
		s.Cfg.FrameworkPublicFileMapOutDir,
	)
	if !eventpipeline.ShouldExecuteBuildPhaseForExecutionDecision(buildExecutionDecision) {
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

	if eventpipeline.ShouldExecuteAnyFileProcessingForBuildDecision(buildExecutionDecision) {
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

	publicStaticProcessingError := eventpipeline.ExecuteStaticFileProcessingForBuildPhase(
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
	privateStaticProcessingError := eventpipeline.ExecuteStaticFileProcessingForBuildPhase(
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

	return eventpipeline.FilterClassifiedEventsForProcessingByPostClassificationDecision(
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
	classifiedEventForProcessing.FileType = eventpipeline.DeriveInitialFileTypeForWatcherEvent(
		watcherEvent.Name,
		watcher,
		builder,
	)
	classifiedEventForProcessing.WatchedFile = watcher.FindWatchedFile(watcherEvent.Name)
	classifiedEventForProcessing.FileType = eventpipeline.DeriveFileTypeWithWatchedFileOverrides(
		classifiedEventForProcessing.FileType,
		classifiedEventForProcessing.WatchedFile,
	)
	classifiedEventForProcessing.Ignored = eventpipeline.DeriveWatcherEventIgnoredStatus(
		classifiedEventForProcessing.Ignored,
		classifiedEventForProcessing.FileType,
		classifiedEventForProcessing.WatchedFile,
	)
	classifiedEventForProcessing.ChmodOnly = watch.IsNonEmptyChmodOnly(watcherEvent)

	return classifiedEventForProcessing
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
				hooks.MaxConcurrentNoWaitHookExecutions,
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

func shouldContinueConcurrentHookExecution(
	concurrentHookExecutionContext context.Context,
) bool {
	return hooks.ShouldContinueConcurrentHookExecution(
		concurrentHookExecutionContext,
	)
}
