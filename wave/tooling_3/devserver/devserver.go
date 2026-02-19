package devserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/lab/vitecmd"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/waveshared"
	"github.com/vormadev/vorma/wave/tooling_3/broadcast"
	"github.com/vormadev/vorma/wave/tooling_3/builder"
	"github.com/vormadev/vorma/wave/tooling_3/devserver/eventpipeline"
	"github.com/vormadev/vorma/wave/tooling_3/devserver/hooks"
	"github.com/vormadev/vorma/wave/tooling_3/devserver/restartengine"
	"github.com/vormadev/vorma/wave/tooling_3/devserver/runloop"
	"github.com/vormadev/vorma/wave/tooling_3/devserver/runtimeprocess"
	"github.com/vormadev/vorma/wave/tooling_3/toolingshared"
	"github.com/vormadev/vorma/wave/tooling_3/watch"
	"github.com/vormadev/vorma/wave/tooling_3/watch/classification"
	"github.com/vormadev/vorma/wave/tooling_3/watch/dedup"
	"golang.org/x/sync/errgroup"
)

const defaultRefreshPort = 10000

// WatcherExecutionTraceContext carries watcher cycle and batch identifiers.
type WatcherExecutionTraceContext struct {
	CycleID uint64
	BatchID uint64
}

// Server owns dev runtime orchestration and mutable lifecycle state.
type Server struct {
	Cfg *wave.ParsedConfig
	Log *slog.Logger

	PortResolver *waveshared.Resolver
	Lock         *toolingshared.DevLock

	Mu sync.Mutex

	Builder *builder.Builder
	Watcher *watch.Watcher

	AppCommand        *exec.Cmd
	AppProcessManager *runtimeprocess.AppProcessManager
	ViteContext       *vitecmd.BuildCtx

	RefreshManager   *broadcast.Manager
	RefreshServer    *http.Server
	RefreshMgrCancel context.CancelFunc

	RestartIntents *restartengine.RestartIntentAccumulator

	WaitingForBuildRetry bool
	WatcherStartCh       chan struct{}

	NextRunCycleID       uint64
	CurrentRunCycleScope *restartengine.RunCycleScope

	NextWatcherBatchID                  uint64
	CurrentWatcherExecutionTraceContext WatcherExecutionTraceContext

	ConcurrentNoWaitHookExecutionLimiter      chan struct{}
	ConcurrentNoWaitHookLifecycleContext      context.Context
	ConcurrentNoWaitHookLifecycleCancel       context.CancelFunc
	ConcurrentNoWaitHookLifecycleContextMutex sync.Mutex
}

// RunDev runs the devserver lifecycle for one parsed config.
func RunDev(cfg *wave.ParsedConfig, log *slog.Logger) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	if log == nil {
		log = slog.Default()
	}

	lock := toolingshared.NewDevLock(cfg.Dist.Static())
	if lockAcquireError := lock.Acquire(); lockAcquireError != nil {
		return lockAcquireError
	}
	defer func() {
		_ = lock.Release()
	}()

	server := &Server{
		Cfg:          cfg,
		Log:          log,
		PortResolver: waveshared.NewResolver(),
		Lock:         lock,
		ConcurrentNoWaitHookExecutionLimiter: make(
			chan struct{},
			hooks.MaxConcurrentNoWaitHookExecutions,
		),
	}
	server.RestartIntents = restartengine.NewRestartIntentAccumulator(
		make(chan restartengine.RestartRequest, 1),
	)
	return server.Run()
}

// Builder returns current builder instance.
func (server *Server) BuilderInstance() *builder.Builder {
	server.Mu.Lock()
	defer server.Mu.Unlock()
	return server.Builder
}

// SetBuilder sets current builder instance.
func (server *Server) SetBuilder(builderInstance *builder.Builder) {
	server.Mu.Lock()
	defer server.Mu.Unlock()
	server.Builder = builderInstance
}

// WatcherInstance returns current watcher instance.
func (server *Server) WatcherInstance() *watch.Watcher {
	server.Mu.Lock()
	defer server.Mu.Unlock()
	return server.Watcher
}

// StartRunCycleScope creates and sets one new run-cycle scope.
func (server *Server) StartRunCycleScope() *restartengine.RunCycleScope {
	if server == nil {
		return nil
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	server.NextRunCycleID++
	scope := restartengine.NewRunCycleScope(server.NextRunCycleID)
	server.CurrentRunCycleScope = scope
	return scope
}

// CurrentRunCycleScopeSnapshot returns current run-cycle scope pointer.
func (server *Server) CurrentRunCycleScopeSnapshot() *restartengine.RunCycleScope {
	if server == nil {
		return nil
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	return server.CurrentRunCycleScope
}

// CurrentRunCycleContextOrBackground returns cycle context or background.
func (server *Server) CurrentRunCycleContextOrBackground() context.Context {
	scope := server.CurrentRunCycleScopeSnapshot()
	if scope == nil || scope.ExecutionContext == nil {
		return context.Background()
	}
	return scope.ExecutionContext
}

// CancelAndJoinCurrentRunCycleScope cancels current run cycle and waits for work.
func (server *Server) CancelAndJoinCurrentRunCycleScope() {
	scope := server.CurrentRunCycleScopeSnapshot()
	if scope == nil {
		return
	}
	scope.CancelAndWait()
	server.Mu.Lock()
	if server.CurrentRunCycleScope == scope {
		server.CurrentRunCycleScope = nil
	}
	server.Mu.Unlock()
}

// LaunchRunCycleScopedAsyncWorkOrDetached launches work under cycle scope when present.
func (server *Server) LaunchRunCycleScopedAsyncWorkOrDetached(
	runAsyncWork func(context.Context),
) {
	if runAsyncWork == nil {
		return
	}
	scope := server.CurrentRunCycleScopeSnapshot()
	if scope != nil {
		scope.LaunchAsyncWork(runAsyncWork)
		return
	}
	go runAsyncWork(context.Background())
}

// DeriveWatcherExecutionTraceContext allocates next batch trace context.
func (server *Server) DeriveWatcherExecutionTraceContext() WatcherExecutionTraceContext {
	if server == nil {
		return WatcherExecutionTraceContext{}
	}

	server.Mu.Lock()
	server.NextWatcherBatchID++
	batchID := server.NextWatcherBatchID
	currentCycleScope := server.CurrentRunCycleScope
	server.Mu.Unlock()

	cycleID := uint64(0)
	if currentCycleScope != nil {
		cycleID = currentCycleScope.CycleID
	}
	return WatcherExecutionTraceContext{CycleID: cycleID, BatchID: batchID}
}

// SetCurrentWatcherExecutionTraceContext sets current trace context.
func (server *Server) SetCurrentWatcherExecutionTraceContext(
	traceContext WatcherExecutionTraceContext,
) {
	if server == nil {
		return
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	server.CurrentWatcherExecutionTraceContext = traceContext
}

// ClearCurrentWatcherExecutionTraceContext clears current trace context.
func (server *Server) ClearCurrentWatcherExecutionTraceContext() {
	if server == nil {
		return
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	server.CurrentWatcherExecutionTraceContext = WatcherExecutionTraceContext{}
}

// CurrentWatcherExecutionTraceContextSnapshot returns current trace context.
func (server *Server) CurrentWatcherExecutionTraceContextSnapshot() WatcherExecutionTraceContext {
	if server == nil {
		return WatcherExecutionTraceContext{}
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	return server.CurrentWatcherExecutionTraceContext
}

// BuildRunloopEngine creates runloop engine with dependency wiring.
func (server *Server) BuildRunloopEngine() *runloop.Engine {
	if server == nil {
		return runloop.New(runloop.Dependencies{})
	}

	return runloop.New(runloop.Dependencies{
		Log:                                server.Log,
		Config:                             server.Cfg,
		GetCurrentWatcher:                  server.WatcherInstance,
		GetCurrentBuilder:                  server.BuilderInstance,
		CurrentRunCycleContextOrBackground: server.CurrentRunCycleContextOrBackground,
		ExecuteBuildPhase:                  server.ExecuteBuildPhase,
		ExecuteBrowserPhase:                server.ExecuteBrowserPhase,
		StartApp:                           server.StartApp,
		StopApp:                            server.StopApp,
		TriggerRestart:                     server.TriggerRestart,
		TriggerRestartNoGo:                 server.TriggerRestartNoGo,
		TriggerConfigRestart:               server.TriggerConfigRestart,
		BroadcastRebuilding:                server.BroadcastRebuilding,
		BuildEventExecutionPlan:            server.BuildEventExecutionPlan,
		DeriveWatcherExecutionTraceContext: func() runloop.WatcherExecutionTraceContext {
			trace := server.DeriveWatcherExecutionTraceContext()
			return runloop.WatcherExecutionTraceContext{
				CycleID: trace.CycleID,
				BatchID: trace.BatchID,
			}
		},
		SetCurrentWatcherExecutionTraceContext: func(trace runloop.WatcherExecutionTraceContext) {
			server.SetCurrentWatcherExecutionTraceContext(
				WatcherExecutionTraceContext{
					CycleID: trace.CycleID,
					BatchID: trace.BatchID,
				},
			)
		},
		ClearCurrentWatcherExecutionTraceContext: server.ClearCurrentWatcherExecutionTraceContext,
		GetCurrentWatcherExecutionTraceContext: func() runloop.WatcherExecutionTraceContext {
			trace := server.CurrentWatcherExecutionTraceContextSnapshot()
			return runloop.WatcherExecutionTraceContext{
				CycleID: trace.CycleID,
				BatchID: trace.BatchID,
			}
		},
		RunNoWaitHookWithConcurrencyLimit:               server.RunNoWaitHookWithConcurrencyLimit,
		GetOrCreateConcurrentNoWaitHookLifecycleContext: server.GetOrCreateConcurrentNoWaitHookLifecycleContext,
		ResolveHookExecutionPlan:                        server.ResolveHookExecutionPlan,
	})
}

// EnsureAppProcessManager returns existing manager or creates one.
func (server *Server) EnsureAppProcessManager() *runtimeprocess.AppProcessManager {
	if server == nil {
		return nil
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	if server.AppProcessManager == nil {
		server.AppProcessManager = runtimeprocess.NewAppProcessManager()
	}
	return server.AppProcessManager
}

// StartApp starts application binary process.
func (server *Server) StartApp() {
	manager := server.EnsureAppProcessManager()
	if manager == nil {
		return
	}
	command, startError := manager.StartApp(server.Cfg.Dist.Binary())
	if startError != nil {
		server.Log.Error("start app failed", "error", startError)
		return
	}
	server.Mu.Lock()
	server.AppCommand = command
	server.Mu.Unlock()
}

// StopApp stops current application process.
func (server *Server) StopApp() error {
	manager := server.EnsureAppProcessManager()
	if manager == nil {
		return nil
	}
	server.Mu.Lock()
	command := server.AppCommand
	server.Mu.Unlock()
	stopError := manager.StopApp(command)
	if stopError == nil {
		server.Mu.Lock()
		server.AppCommand = nil
		server.Mu.Unlock()
	}
	return stopError
}

// InitWatcher initializes watcher and stores instance.
func (server *Server) InitWatcher() error {
	watcherInstance, watcherCreateError := watch.NewWatcher(
		server.Cfg,
		server.Log,
	)
	if watcherCreateError != nil {
		return watcherCreateError
	}
	server.Mu.Lock()
	server.Watcher = watcherInstance
	server.Mu.Unlock()
	return nil
}

// AddConfigFileDirectory ensures configuration file directory is watched.
func (server *Server) AddConfigFileDirectory() error {
	configurationDirectory := filepathDir(server.Cfg.Core.ConfigLocation)
	if strings.TrimSpace(configurationDirectory) == "" {
		return nil
	}
	watcher := server.WatcherInstance()
	if watcher == nil {
		return errors.New("watcher is not initialized")
	}
	return watcher.AddDirectoryRecursively(configurationDirectory)
}

// ReloadConfig reloads parsed config from disk and preserves runtime framework hooks.
func (server *Server) ReloadConfig() (*wave.ParsedConfig, error) {
	if server == nil || server.Cfg == nil {
		return nil, errors.New("server config unavailable")
	}
	newConfig, loadError := server.LoadParsedConfigForReload(
		server.Cfg.Core.ConfigLocation,
	)
	if loadError != nil {
		return nil, loadError
	}
	server.Mu.Lock()
	server.Cfg = newConfig
	server.Mu.Unlock()
	return newConfig, nil
}

// LoadParsedConfigForReload loads parsed config while preserving framework runtime callbacks.
func (server *Server) LoadParsedConfigForReload(
	configLocation string,
) (*wave.ParsedConfig, error) {
	currentConfig := server.Cfg
	newConfig, loadError := wave.ParseConfigFile(configLocation)
	if loadError != nil {
		return nil, loadError
	}
	if currentConfig != nil {
		newConfig.FrameworkDevBuildHook = currentConfig.FrameworkDevBuildHook
		newConfig.FrameworkProdBuildHook = currentConfig.FrameworkProdBuildHook
		newConfig.FrameworkRunBuildHook = currentConfig.FrameworkRunBuildHook
		newConfig.FrameworkPrepareGoBuildOverlay = currentConfig.FrameworkPrepareGoBuildOverlay
		newConfig.FrameworkBrowserRuntimeNamespace = currentConfig.FrameworkBrowserRuntimeNamespace
		newConfig.FrameworkBrowserPublicURLResolverFunctionName = currentConfig.FrameworkBrowserPublicURLResolverFunctionName
		newConfig.FrameworkBrowserRevalidateFunctionName = currentConfig.FrameworkBrowserRevalidateFunctionName
		newConfig.FrameworkRefreshRebuildingOverlayElementID = currentConfig.FrameworkRefreshRebuildingOverlayElementID
		newConfig.FrameworkCriticalCSSStyleElementID = currentConfig.FrameworkCriticalCSSStyleElementID
		newConfig.FrameworkNonCriticalCSSLinkElementID = currentConfig.FrameworkNonCriticalCSSLinkElementID
	}
	return newConfig, nil
}

// WaitForApp waits until app healthcheck becomes ready.
func (server *Server) WaitForApp() bool {
	readyURL := runtimeprocess.ResolveAppReadyURL(
		server.MustGetPort(),
		server.Cfg.HealthcheckEndpoint(),
	)
	ready := server.WaitForReady(readyURL)
	if !ready {
		server.Log.Warn(
			"app did not become ready before timeout",
			"url",
			readyURL,
		)
	}
	return ready
}

// WaitForReady waits for one readiness URL.
func (server *Server) WaitForReady(url string) bool {
	return server.WaitForAnyReady([]string{url})
}

// WaitForAnyReady waits until at least one URL is ready.
func (server *Server) WaitForAnyReady(urls []string) bool {
	return runtimeprocess.WaitForAnyReady(
		urls,
		runtimeprocess.DefaultReadinessWaitPolicy(),
	)
}

// MustGetPort returns app runtime port for devserver orchestration.
func (server *Server) MustGetPort() int {
	if server == nil || server.PortResolver == nil {
		return wave.MustGetPort()
	}
	return server.PortResolver.MustGetPort()
}

// StartRefreshServer starts websocket refresh HTTP server.
func (server *Server) StartRefreshServer() error {
	server.Mu.Lock()
	if server.RefreshServer != nil {
		server.Mu.Unlock()
		return nil
	}
	refreshManager := broadcast.NewManager(
		server.Log,
		broadcast.ManagerConfig{},
	)
	refreshPort := resolveRefreshPortFromEnvironmentOrDefault(
		defaultRefreshPort,
	)
	listener, listenError := net.Listen("tcp", ":"+strconv.Itoa(refreshPort))
	if listenError != nil {
		server.Mu.Unlock()
		return fmt.Errorf("listen refresh server: %w", listenError)
	}

	refreshServerContext, cancelRefreshServer := context.WithCancel(
		context.Background(),
	)
	refreshServer := &http.Server{
		Handler: server.newRefreshServerMux(refreshManager),
	}

	server.RefreshMgrCancel = cancelRefreshServer
	server.RefreshManager = refreshManager
	server.RefreshServer = refreshServer
	server.Mu.Unlock()

	server.LaunchRunCycleScopedAsyncWorkOrDetached(func(context.Context) {
		refreshManager.Run(refreshServerContext)
	})
	server.LaunchRunCycleScopedAsyncWorkOrDetached(func(context.Context) {
		if serveError := refreshServer.Serve(listener); serveError != nil &&
			!errors.Is(serveError, http.ErrServerClosed) {
			server.Log.Error("refresh server serve failed", "error", serveError)
		}
	})
	return nil
}

// newRefreshServerMux builds mux for refresh endpoints.
func (server *Server) newRefreshServerMux(
	refreshManager *broadcast.Manager,
) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/refresh", refreshManager)
	mux.HandleFunc(
		"/healthz",
		func(responseWriter http.ResponseWriter, _ *http.Request) {
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write([]byte("ok"))
		},
	)
	return mux
}

// StopRefreshServer stops websocket refresh server and manager.
func (server *Server) StopRefreshServer() {
	server.Mu.Lock()
	refreshServer := server.RefreshServer
	refreshManager := server.RefreshManager
	cancelRefreshManager := server.RefreshMgrCancel
	server.RefreshServer = nil
	server.RefreshManager = nil
	server.RefreshMgrCancel = nil
	server.Mu.Unlock()

	if cancelRefreshManager != nil {
		cancelRefreshManager()
	}
	if refreshServer != nil {
		_ = refreshServer.Close()
	}
	if refreshManager != nil {
		refreshManager.Close()
	}
}

// GetOrCreateRestartIntentAccumulator returns existing or initializes restart accumulator.
func (server *Server) GetOrCreateRestartIntentAccumulator() *restartengine.RestartIntentAccumulator {
	if server == nil {
		return nil
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	if server.RestartIntents == nil {
		server.RestartIntents = restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		)
	}
	return server.RestartIntents
}

// SetWaitingForBuildRetry sets waiting flag.
func (server *Server) SetWaitingForBuildRetry(waiting bool) {
	if server == nil {
		return
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	server.WaitingForBuildRetry = waiting
}

// QueueRestartRequest queues restart request into intent accumulator.
func (server *Server) QueueRestartRequest(
	request restartengine.RestartRequest,
) {
	accumulator := server.GetOrCreateRestartIntentAccumulator()
	if accumulator == nil {
		return
	}
	accumulator.Queue(request)
}

// ConsumePendingRestartRequest consumes pending restart request.
func (server *Server) ConsumePendingRestartRequest() (restartengine.RestartRequest, bool) {
	accumulator := server.GetOrCreateRestartIntentAccumulator()
	if accumulator == nil {
		return restartengine.RestartRequest{}, false
	}
	return accumulator.ConsumePending()
}

// ConsumeRestartRequestBlocking consumes restart request blocking.
func (server *Server) ConsumeRestartRequestBlocking() restartengine.RestartRequest {
	accumulator := server.GetOrCreateRestartIntentAccumulator()
	if accumulator == nil {
		return restartengine.RestartRequest{}
	}
	return accumulator.ConsumeBlocking()
}

// TriggerRestart requests restart with go recompilation.
func (server *Server) TriggerRestart() {
	server.TriggerRestartWithOpts(true, false)
}

// TriggerRestartNoGo requests restart without go recompilation.
func (server *Server) TriggerRestartNoGo() {
	server.TriggerRestartWithOpts(false, false)
}

// TriggerConfigRestart requests config restart semantics.
func (server *Server) TriggerConfigRestart() {
	server.TriggerRestartWithOpts(true, true)
}

// TriggerRestartWithOpts queues normalized restart request.
func (server *Server) TriggerRestartWithOpts(
	recompileGo bool,
	isConfigRestart bool,
) {
	request := restartengine.NormalizeRestartRequest(
		restartengine.RestartRequest{
			RecompileGo:     recompileGo,
			IsConfigRestart: isConfigRestart,
		},
	)
	server.QueueRestartRequest(request)
}

// Run executes main devserver lifecycle state machine.
func (server *Server) Run() error {
	wave.SetModeToDev()

	if initWatcherError := server.InitWatcher(); initWatcherError != nil {
		return initWatcherError
	}
	if addConfigDirectoryError := server.AddConfigFileDirectory(); addConfigDirectoryError != nil {
		return addConfigDirectoryError
	}

	builderInstance := builder.NewBuilder(
		server.Cfg,
		server.Log,
	)
	if builderInstance == nil {
		return errors.New("builder initialization returned nil")
	}
	server.SetBuilder(builderInstance)
	defer builderInstance.Close()

	if startRefreshServerError := server.StartRefreshServer(); startRefreshServerError != nil {
		return startRefreshServerError
	}
	defer server.StopRefreshServer()

	currentState := restartengine.RunLifecycleStatePreparingCycle
	currentIntent := restartengine.RunIntent{
		RecompileGo: true,
		IsRebuild:   false,
	}
	firstRun := true
	currentCycleID := uint64(0)

	for {
		commandForState, deriveCommandError := restartengine.DeriveRunLifecycleCommandForState(
			currentState,
		)
		if deriveCommandError != nil {
			return deriveCommandError
		}

		input := restartengine.RunLifecycleCommandInput{
			CurrentCycleID: currentCycleID,
			CurrentIntent:  currentIntent,
			FirstRun:       firstRun,
		}

		result, executeCommandError := server.ExecuteRunLifecycleCommand(
			commandForState,
			input,
		)
		if executeCommandError != nil {
			return executeCommandError
		}

		nextState, transitionError := restartengine.TransitionRunLifecycleState(
			server.Log,
			currentState,
			result.RunLifecycleEvent,
			currentCycleID,
		)
		if transitionError != nil {
			return transitionError
		}

		currentState = nextState
		if result.NextRunIntent != (restartengine.RunIntent{}) {
			currentIntent = result.NextRunIntent
		}
		if currentState == restartengine.RunLifecycleStatePreparingCycle {
			currentCycleID++
			firstRun = false
		}
	}
}

// ExecuteRunLifecycleCommand executes one lifecycle command.
func (server *Server) ExecuteRunLifecycleCommand(
	command restartengine.RunLifecycleCommand,
	input restartengine.RunLifecycleCommandInput,
) (restartengine.RunLifecycleCommandResult, error) {
	switch command {
	case restartengine.RunLifecycleCommandPrepareCycle:
		return server.ExecuteRunLifecycleCommandPrepareCycle(input)
	case restartengine.RunLifecycleCommandBuildCycle:
		return server.ExecuteRunLifecycleCommandBuildCycle(input)
	case restartengine.RunLifecycleCommandAwaitBuildRetry:
		return server.ExecuteRunLifecycleCommandAwaitBuildRetry(input)
	case restartengine.RunLifecycleCommandStartRuntime:
		return server.ExecuteRunLifecycleCommandStartRuntime(input)
	case restartengine.RunLifecycleCommandAwaitRestartRequest:
		return server.ExecuteRunLifecycleCommandAwaitRestartRequest(input)
	case restartengine.RunLifecycleCommandCleanupForNextCycle:
		return server.ExecuteRunLifecycleCommandCleanupForNextCycle(input)
	default:
		return restartengine.RunLifecycleCommandResult{}, fmt.Errorf(
			"unknown run lifecycle command: %d",
			command,
		)
	}
}

// PrepareRunCycle prepares run-cycle state and process resources.
func (server *Server) PrepareRunCycle(firstRun bool) error {
	if firstRun {
		server.MustGetPort()
	}
	server.CancelConcurrentNoWaitHookLifecycleContext()
	_ = server.StopVite()
	return nil
}

// ExecuteRunBuildForIntent executes build phase for provided run intent.
func (server *Server) ExecuteRunBuildForIntent(
	isRebuild bool,
	intent restartengine.RunIntent,
) error {
	orderingDecision := restartengine.DeriveRunBuildExecutionOrderingDecision(
		intent.RecompileGo,
		server.Cfg.Core.SequentialGoBuild,
	)
	builderInstance := server.BuilderInstance()
	if builderInstance == nil {
		return errors.New("builder is unavailable")
	}

	return builderInstance.Build(builder.BuildOpts{
		CompileGo:    orderingDecision.ShouldCompileGo,
		IsDev:        true,
		IsRebuild:    isRebuild,
		FileOnlyMode: false,
	})
}

// StartRunCycleRuntime starts app/vite and watcher runloop for runtime phase.
func (server *Server) StartRunCycleRuntime() {
	if server.Cfg.UsingVite() {
		if startViteError := server.StartVite(); startViteError != nil {
			server.Log.Error("start vite failed", "error", startViteError)
		}
	}

	server.StartApp()

	cycleScope := server.StartRunCycleScope()
	runloopEngine := server.BuildRunloopEngine()
	server.WatcherStartCh = make(chan struct{})

	if cycleScope != nil {
		cycleScope.LaunchAsyncWork(func(cycleContext context.Context) {
			select {
			case <-server.WatcherStartCh:
			case <-cycleContext.Done():
				return
			}
			runloopEngine.RunWatcherWithContext(cycleContext)
		})
	}

	close(server.WatcherStartCh)
}

// WaitForBuildRetry waits for file events that trigger a restart after build failure.
func (server *Server) WaitForBuildRetry() restartengine.RestartRequest {
	server.SetWaitingForBuildRetry(true)
	defer server.SetWaitingForBuildRetry(false)

	if pendingRequest, hasPendingRequest := server.ConsumePendingRestartRequest(); hasPendingRequest {
		return restartengine.NormalizeRestartRequest(pendingRequest)
	}

	server.WatcherStartCh = make(chan struct{})
	cycleScope := server.StartRunCycleScope()
	runloopEngine := server.BuildRunloopEngine()

	if cycleScope != nil {
		cycleScope.LaunchAsyncWork(func(cycleContext context.Context) {
			select {
			case <-server.WatcherStartCh:
			case <-cycleContext.Done():
				return
			}
			runloopEngine.RunWatcherWithContext(cycleContext)
		})
	}
	close(server.WatcherStartCh)

	return server.ConsumeRestartRequestBlocking()
}

// CleanupForRebuild performs stop/cancel tasks before next run cycle.
func (server *Server) CleanupForRebuild() {
	server.CancelAndJoinCurrentRunCycleScope()
	server.CancelConcurrentNoWaitHookLifecycleContext()
	if stopError := server.StopApp(); stopError != nil {
		server.Log.Warn("stop app during cleanup failed", "error", stopError)
	}
	if stopViteError := server.StopVite(); stopViteError != nil {
		server.Log.Warn(
			"stop vite during cleanup failed",
			"error",
			stopViteError,
		)
	}
}

// CleanupRefreshServer stops refresh server resources.
func (server *Server) CleanupRefreshServer() {
	server.StopRefreshServer()
}

// ExecuteRunLifecycleCommandPrepareCycle handles prepare-cycle lifecycle command.
func (server *Server) ExecuteRunLifecycleCommandPrepareCycle(
	input restartengine.RunLifecycleCommandInput,
) (restartengine.RunLifecycleCommandResult, error) {
	if prepareError := server.PrepareRunCycle(input.FirstRun); prepareError != nil {
		return restartengine.RunLifecycleCommandResult{}, prepareError
	}
	return restartengine.RunLifecycleCommandResult{
		RunLifecycleEvent: restartengine.RunLifecycleEventCyclePrepared,
	}, nil
}

// ExecuteRunLifecycleCommandBuildCycle handles build-cycle lifecycle command.
func (server *Server) ExecuteRunLifecycleCommandBuildCycle(
	input restartengine.RunLifecycleCommandInput,
) (restartengine.RunLifecycleCommandResult, error) {
	buildError := server.ExecuteRunBuildForIntent(
		input.CurrentIntent.IsRebuild,
		input.CurrentIntent,
	)
	if buildError != nil {
		server.Log.Error("build failed", "error", buildError)
		return restartengine.RunLifecycleCommandResult{
			RunLifecycleEvent: restartengine.RunLifecycleEventBuildFailed,
		}, nil
	}
	return restartengine.RunLifecycleCommandResult{
		RunLifecycleEvent: restartengine.RunLifecycleEventBuildSucceeded,
	}, nil
}

// ExecuteRunLifecycleCommandAwaitBuildRetry handles retry wait after failed build.
func (server *Server) ExecuteRunLifecycleCommandAwaitBuildRetry(
	_ restartengine.RunLifecycleCommandInput,
) (restartengine.RunLifecycleCommandResult, error) {
	server.Log.Info("waiting for file changes to retry build")
	restartRequest := server.WaitForBuildRetry()
	nextIntent := restartengine.DeriveRunIntentFromRestartRequest(
		restartRequest,
	)
	return restartengine.RunLifecycleCommandResult{
		RunLifecycleEvent: restartengine.RunLifecycleEventBuildRetryRestartReceived,
		NextRunIntent:     nextIntent,
	}, nil
}

// ExecuteRunLifecycleCommandStartRuntime handles runtime start lifecycle command.
func (server *Server) ExecuteRunLifecycleCommandStartRuntime(
	_ restartengine.RunLifecycleCommandInput,
) (restartengine.RunLifecycleCommandResult, error) {
	server.StartRunCycleRuntime()
	return restartengine.RunLifecycleCommandResult{
		RunLifecycleEvent: restartengine.RunLifecycleEventRuntimeStarted,
	}, nil
}

// ExecuteRunLifecycleCommandAwaitRestartRequest handles restart wait lifecycle command.
func (server *Server) ExecuteRunLifecycleCommandAwaitRestartRequest(
	_ restartengine.RunLifecycleCommandInput,
) (restartengine.RunLifecycleCommandResult, error) {
	restartRequest := server.ConsumeRestartRequestBlocking()
	nextIntent := restartengine.DeriveRunIntentFromRestartRequest(
		restartRequest,
	)
	server.Log.Info(
		"restart requested",
		"recompile_go",
		restartRequest.RecompileGo,
		"config_restart",
		restartRequest.IsConfigRestart,
	)
	return restartengine.RunLifecycleCommandResult{
		RunLifecycleEvent: restartengine.RunLifecycleEventRestartRequestReceived,
		NextRunIntent:     nextIntent,
	}, nil
}

// ExecuteRunLifecycleCommandCleanupForNextCycle handles cleanup lifecycle command.
func (server *Server) ExecuteRunLifecycleCommandCleanupForNextCycle(
	_ restartengine.RunLifecycleCommandInput,
) (restartengine.RunLifecycleCommandResult, error) {
	server.CleanupForRebuild()
	return restartengine.RunLifecycleCommandResult{
		RunLifecycleEvent: restartengine.RunLifecycleEventCleanupCompleted,
	}, nil
}

// StartVite starts Vite dev process context.
func (server *Server) StartVite() error {
	server.Mu.Lock()
	defer server.Mu.Unlock()
	if server.Cfg == nil || server.Cfg.Vite == nil {
		return nil
	}

	if server.ViteContext == nil {
		server.ViteContext = vitecmd.NewBuildCtx(&vitecmd.BuildCtxOptions{
			JSPackageManagerBaseCmd: server.Cfg.Vite.JSPackageManagerBaseCmd,
			JSPackageManagerCmdDir:  server.Cfg.Vite.JSPackageManagerCmdDir,
			OutDir:                  server.Cfg.Dist.StaticPublic(),
			ManifestOut:             server.Cfg.ViteManifestPath(),
			DefaultPort:             server.Cfg.Vite.DefaultPort,
			ViteConfigFile:          server.Cfg.Vite.ViteConfigFile,
		})
	}
	return server.ViteContext.DevBuild()
}

// StopVite stops Vite process context.
func (server *Server) StopVite() error {
	server.Mu.Lock()
	viteContext := server.ViteContext
	server.Mu.Unlock()
	if viteContext == nil {
		return nil
	}
	viteContext.Cleanup()
	return nil
}

// CycleVite restarts Vite and does not wait for readiness.
func (server *Server) CycleVite() {
	_ = server.StopVite()
	if startViteError := server.StartVite(); startViteError != nil {
		server.Log.Warn("cycle vite failed", "error", startViteError)
	}
}

// CycleViteAndWaitForReadiness cycles Vite and waits for readiness.
func (server *Server) CycleViteAndWaitForReadiness() bool {
	server.CycleVite()
	return server.WaitForVite()
}

// CallViteFilemapInvalidate calls configured invalidate endpoint in Vite runtime.
func (server *Server) CallViteFilemapInvalidate() error {
	viteContext := server.ViteContext
	if viteContext == nil {
		return errors.New("vite context is nil")
	}
	invalidateURL := "http://127.0.0.1:" + strconv.Itoa(
		viteContext.Port(),
	) + "/__wave/vite-filemap-invalidate"
	request, requestCreateError := http.NewRequest(
		http.MethodPost,
		invalidateURL,
		nil,
	)
	if requestCreateError != nil {
		return requestCreateError
	}
	response, requestError := (&http.Client{Timeout: 2 * time.Second}).Do(
		request,
	)
	if requestError != nil {
		return requestError
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		return fmt.Errorf(
			"vite invalidate failed with status %d",
			response.StatusCode,
		)
	}
	return nil
}

// WaitForVite waits for vite readiness on known probe URLs.
func (server *Server) WaitForVite() bool {
	viteContext := server.ViteContext
	if viteContext == nil {
		return false
	}
	return server.WaitForAnyReady(ResolveViteReadyURLs(viteContext.Port()))
}

// ResolveViteReadyURLs resolves Vite readiness probe URLs.
func ResolveViteReadyURLs(vitePort int) []string {
	return []string{
		runtimeprocess.ResolveReadinessProbeURL(
			runtimeprocess.LocalReadinessProbeHostIPv4,
			vitePort,
			"/@vite/client",
		),
		runtimeprocess.ResolveReadinessProbeURL(
			runtimeprocess.LocalReadinessProbeHostLocalhost,
			vitePort,
			"/@vite/client",
		),
	}
}

// BroadcastRebuilding broadcasts rebuilding overlay payload to clients.
func (server *Server) BroadcastRebuilding() {
	refreshManager := server.currentRefreshManager()
	if refreshManager == nil {
		return
	}
	refreshManager.BroadcastRebuilding()
}

// BroadcastReload broadcasts reload payload with readiness handling.
func (server *Server) BroadcastReload(reloadOptions eventpipeline.ReloadOpts) {
	if !server.ShouldBroadcastToBrowserClients() {
		return
	}

	if !server.WaitForReloadReadiness(reloadOptions) {
		server.Log.Warn("reload readiness failed; skipping browser broadcast")
		return
	}

	if !server.ShouldBroadcastReloadPayloadAfterReadiness(reloadOptions) {
		return
	}

	refreshManager := server.currentRefreshManager()
	if refreshManager == nil {
		return
	}
	refreshManager.Broadcast(reloadOptions.Payload)
}

// ShouldBroadcastToBrowserClients reports whether browser broadcast should run.
func (server *Server) ShouldBroadcastToBrowserClients() bool {
	return server.Cfg != nil && server.Cfg.UsingBrowser()
}

// WaitForReloadReadiness applies readiness policy for reload.
func (server *Server) WaitForReloadReadiness(
	reloadOptions eventpipeline.ReloadOpts,
) bool {
	if reloadOptions.CycleVite {
		if !server.CycleViteAndWaitForReadiness() {
			return false
		}
	}

	urls := make([]string, 0, 2)
	if reloadOptions.WaitApp {
		urls = append(
			urls,
			runtimeprocess.ResolveAppReadyURL(
				server.MustGetPort(),
				server.Cfg.HealthcheckEndpoint(),
			),
		)
	}
	if reloadOptions.WaitVite && server.ViteContext != nil {
		urls = append(urls, ResolveViteReadyURLs(server.ViteContext.Port())...)
	}
	if len(urls) == 0 {
		return true
	}
	return server.WaitForAnyReady(urls)
}

// ShouldBroadcastReloadPayloadAfterReadiness returns whether payload should be sent.
func (server *Server) ShouldBroadcastReloadPayloadAfterReadiness(
	_ eventpipeline.ReloadOpts,
) bool {
	return true
}

// BuildEventExecutionPlan classifies events and builds hook-ready execution plan.
func (server *Server) BuildEventExecutionPlan(
	events []fsnotify.Event,
	watcher *watch.Watcher,
	builder *builder.Builder,
) eventpipeline.EventExecutionPlanningResult {
	classifiedEvents, configChanged := server.ClassifyWatcherEventsForProcessing(
		events,
		watcher,
		builder,
	)
	if configChanged {
		return eventpipeline.EventExecutionPlanningResult{ConfigChanged: true}
	}
	if len(classifiedEvents) == 0 {
		return eventpipeline.EventExecutionPlanningResult{}
	}

	eventsWithHooks := hooks.BuildEventHooksForProcessing(classifiedEvents)
	if len(eventsWithHooks) == 0 {
		return eventpipeline.EventExecutionPlanningResult{}
	}

	return eventpipeline.EventExecutionPlanningResult{
		EventsWithHooks: eventsWithHooks,
	}
}

// ExecuteBrowserPhase executes browser-phase side effects for workset.
func (server *Server) ExecuteBrowserPhase(work *eventpipeline.WorkSet) {
	if work == nil || !server.Cfg.UsingBrowser() {
		return
	}

	browserDecision := work.Browser
	if browserDecision.Action == eventpipeline.BrowserPhaseActionInvalidateVite {
		if eventpipeline.ShouldAttemptViteInvalidateForBrowserDecision(
			browserDecision,
			server.Cfg.UsingVite(),
		) {
			if invalidateError := server.CallViteFilemapInvalidate(); invalidateError == nil {
				return
			}
		}
		browserDecision = eventpipeline.ResolveBrowserDecisionAfterInvalidateViteFallback(
			browserDecision,
			server.Cfg.UsingVite(),
		)
		work.Browser = browserDecision
	}

	switch eventpipeline.DeriveBrowserPhaseExecutionCategory(browserDecision.Action) {
	case eventpipeline.BrowserPhaseExecutionCategoryReload:
		reloadOptions, hasReloadOptions := eventpipeline.PlanBrowserReloadForAction(
			browserDecision.Action,
			browserDecision,
		)
		if !hasReloadOptions {
			return
		}
		server.BroadcastReload(reloadOptions)

	case eventpipeline.BrowserPhaseExecutionCategoryHotReloadCSS:
		server.ExecuteHotReloadCSSBrowserPhase(work)
	}
}

// ExecuteHotReloadCSSBrowserPhase executes CSS hot reload payloads.
func (server *Server) ExecuteHotReloadCSSBrowserPhase(
	work *eventpipeline.WorkSet,
) {
	builderInstance := server.BuilderInstance()
	if builderInstance == nil {
		return
	}

	criticalCSS, criticalCSSAvailable := builderInstance.GetCriticalCSS()
	normalCSSURL, normalCSSURLAvailable := builderInstance.GetNormalCSSURL()
	payloads := eventpipeline.PlanHotReloadCSSPayloads(
		work.Build.BuildCriticalCSS,
		criticalCSS,
		criticalCSSAvailable,
		work.Build.BuildNormalCSS,
		normalCSSURL,
		normalCSSURLAvailable,
	)
	if len(payloads) == 0 {
		return
	}

	refreshManager := server.currentRefreshManager()
	if refreshManager == nil {
		return
	}

	for _, payload := range payloads {
		refreshManager.Broadcast(payload)
	}
}

// ExecuteBuildPhase executes build-phase work from resolved workset decision.
func (server *Server) ExecuteBuildPhase(work *eventpipeline.WorkSet) error {
	if work == nil {
		return nil
	}

	builderInstance := server.BuilderInstance()
	if builderInstance == nil {
		return errors.New("builder is unavailable")
	}

	executionDecision := eventpipeline.DeriveBuildPhaseExecutionDecision(
		work.Build,
		server.Cfg.FrameworkPublicFileMapOutDir,
	)
	if !eventpipeline.ShouldExecuteBuildPhaseForExecutionDecision(
		executionDecision,
	) {
		return nil
	}

	var buildGroup errgroup.Group

	if executionDecision.CompileGo {
		buildGroup.Go(func() error {
			return builderInstance.CompileGo()
		})
	}

	if executionDecision.BuildCriticalCSS || executionDecision.BuildNormalCSS {
		buildGroup.Go(func() error {
			return builderInstance.BuildCSS(builder.CSSBuildOptions{
				BuildCriticalCSS: executionDecision.BuildCriticalCSS,
				BuildNormalCSS:   executionDecision.BuildNormalCSS,
			})
		})
	}

	if eventpipeline.ShouldExecuteAnyFileProcessingForBuildDecision(
		executionDecision,
	) {
		buildGroup.Go(func() error {
			if publicProcessingError := server.ExecutePublicStaticProcessingForBuildPhase(builderInstance, executionDecision.PublicStaticProcessing); publicProcessingError != nil {
				return publicProcessingError
			}
			if privateProcessingError := server.ExecutePrivateStaticProcessingForBuildPhase(builderInstance, executionDecision.PrivateStaticProcessing); privateProcessingError != nil {
				return privateProcessingError
			}
			return nil
		})
	}

	if executionDecision.WriteFrameworkPublicFileMap {
		buildGroup.Go(func() error {
			return builderInstance.WriteFrameworkPublicFileMapTS()
		})
	}

	return buildGroup.Wait()
}

// ExecutePublicStaticProcessingForBuildPhase executes public static processing decision.
func (server *Server) ExecutePublicStaticProcessingForBuildPhase(
	builderInstance *builder.Builder,
	publicStaticProcessingDecision eventpipeline.StaticFileProcessingExecutionDecision,
) error {
	if builderInstance == nil {
		return nil
	}
	return eventpipeline.ExecuteStaticFileProcessingForBuildPhase(
		builderInstance.ProcessPublicFilesOnly,
		builderInstance.ProcessPublicFilesOnlyForChangedPaths,
		publicStaticProcessingDecision,
	)
}

// ExecutePrivateStaticProcessingForBuildPhase executes private static processing decision.
func (server *Server) ExecutePrivateStaticProcessingForBuildPhase(
	builderInstance *builder.Builder,
	privateStaticProcessingDecision eventpipeline.StaticFileProcessingExecutionDecision,
) error {
	if builderInstance == nil {
		return nil
	}
	return eventpipeline.ExecuteStaticFileProcessingForBuildPhase(
		builderInstance.ProcessPrivateFilesOnly,
		builderInstance.ProcessPrivateFilesOnlyForChangedPaths,
		privateStaticProcessingDecision,
	)
}

// ClassifyWatcherEventsForProcessing classifies watcher events and handles config reload behavior.
func (server *Server) ClassifyWatcherEventsForProcessing(
	watcherEvents []fsnotify.Event,
	watcher *watch.Watcher,
	builderInstance *builder.Builder,
) ([]eventpipeline.ClassifiedEvent, bool) {
	classifiedEvents, configChanged := server.ClassifyWatcherEventsFromPreClassificationPlan(
		watcherEvents,
		watcher,
		builderInstance,
	)
	if configChanged {
		return nil, true
	}
	return eventpipeline.FilterClassifiedEventsForProcessingByPostClassificationDecision(
		classifiedEvents,
	), false
}

// ApplyWatcherEventPreClassificationSideEffects handles side-effects before semantic classification.
func (server *Server) ApplyWatcherEventPreClassificationSideEffects(
	watcherEvent fsnotify.Event,
) (bool, error) {
	if server.IsConfigFile(watcherEvent.Name) {
		_, reloadError := server.ReloadConfig()
		if reloadError != nil {
			return false, reloadError
		}
		return true, nil
	}
	return false, nil
}

// ClassifyWatcherEventsFromPreClassificationPlan classifies events after pre side effects.
func (server *Server) ClassifyWatcherEventsFromPreClassificationPlan(
	watcherEvents []fsnotify.Event,
	watcher *watch.Watcher,
	builderInstance *builder.Builder,
) ([]eventpipeline.ClassifiedEvent, bool) {
	classifiedEvents := make(
		[]eventpipeline.ClassifiedEvent,
		0,
		len(watcherEvents),
	)
	for _, watcherEvent := range dedup.DeduplicateWatcherEvents(watcherEvents, dedup.DeduplicationPolicy{}).Events {
		configChanged, sideEffectError := server.ApplyWatcherEventPreClassificationSideEffects(
			watcherEvent,
		)
		if sideEffectError != nil {
			server.Log.Error("config reload failed", "error", sideEffectError)
		}
		if configChanged {
			return nil, true
		}

		classifiedEvent := server.ClassifyEventWithWatcherAndBuilder(
			watcherEvent,
			watcher,
			builderInstance,
		)
		classifiedEvents = append(classifiedEvents, classifiedEvent)
	}
	return classifiedEvents, false
}

// IsConfigFile reports whether path matches active config file location.
func (server *Server) IsConfigFile(path string) bool {
	if server == nil || server.Cfg == nil || server.Cfg.Core == nil {
		return false
	}
	return classification.IsConfigurationPathChange(
		path,
		server.Cfg.Core.ConfigLocation,
	)
}

// ClassifyEventWithWatcherAndBuilder classifies watcher event with semantic file-type logic.
func (server *Server) ClassifyEventWithWatcherAndBuilder(
	watcherEvent fsnotify.Event,
	watcher *watch.Watcher,
	builderInstance *builder.Builder,
) eventpipeline.ClassifiedEvent {
	classifiedEvent := eventpipeline.ClassifiedEvent{Event: watcherEvent}
	preDecision := classification.DerivePreClassificationDecision(
		watcherEvent,
		classification.PathClassifierDependencies{
			IsIgnoredPathFunc: watcher.IsIgnoredFile,
			LockFileName:      toolingshared.LockFileName,
		},
	)
	if !preDecision.IncludeEvent {
		classifiedEvent.Ignored = true
		classifiedEvent.ChmodOnly = true
		return classifiedEvent
	}

	classifiedEvent.Ignored = watcher.IsIgnoredFile(watcherEvent.Name)
	classifiedEvent.FileType = eventpipeline.DeriveInitialFileTypeForWatcherEvent(
		watcherEvent.Name,
		watcher,
		builderInstance,
	)
	classifiedEvent.WatchedFile = watcher.FindWatchedFile(watcherEvent.Name)
	classifiedEvent.FileType = eventpipeline.DeriveFileTypeWithWatchedFileOverrides(
		classifiedEvent.FileType,
		classifiedEvent.WatchedFile,
	)
	classifiedEvent.Ignored = eventpipeline.DeriveWatcherEventIgnoredStatus(
		classifiedEvent.Ignored,
		classifiedEvent.FileType,
		classifiedEvent.WatchedFile,
	)
	classifiedEvent.ChmodOnly = classification.DeriveChmodOnlyDecision(
		watcherEvent,
	)
	return classifiedEvent
}

// ResolveHookExecutionPlan resolves one hook into executable plan.
func (server *Server) ResolveHookExecutionPlan(
	hook wave.OnChangeHook,
) hooks.HookExecutionPlan {
	if hook.RunCombinedDevBuildHookCommands {
		userDevBuildHook := ""
		frameworkDevBuildHook := ""
		if server.Cfg != nil && server.Cfg.Core != nil {
			userDevBuildHook = server.Cfg.Core.DevBuildHook
		}
		if server.Cfg != nil {
			frameworkDevBuildHook = server.Cfg.FrameworkDevBuildHook
		}

		if strings.TrimSpace(hook.Cmd) != "" {
			hook.Cmd = hooks.ResolveSequentialShellCommands(
				hook.Cmd,
				userDevBuildHook,
				frameworkDevBuildHook,
			)
		} else {
			hook.Cmd = hooks.ResolveSequentialShellCommands(userDevBuildHook, frameworkDevBuildHook)
		}
	}
	return hooks.DeriveHookExecutionPlanFromHook(hook, nil)
}

// RunNoWaitHookWithConcurrencyLimit executes callback under bounded semaphore.
func (server *Server) RunNoWaitHookWithConcurrencyLimit(runNoWaitHook func()) {
	if runNoWaitHook == nil {
		return
	}
	limiter := server.EnsureConcurrentNoWaitHookExecutionLimiter()
	if limiter == nil {
		runNoWaitHook()
		return
	}
	limiter <- struct{}{}
	go func() {
		defer func() { <-limiter }()
		runNoWaitHook()
	}()
}

// EnsureConcurrentNoWaitHookExecutionLimiter ensures semaphore exists.
func (server *Server) EnsureConcurrentNoWaitHookExecutionLimiter() chan struct{} {
	if server == nil {
		return nil
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	if server.ConcurrentNoWaitHookExecutionLimiter == nil {
		server.ConcurrentNoWaitHookExecutionLimiter = make(
			chan struct{},
			hooks.MaxConcurrentNoWaitHookExecutions,
		)
	}
	return server.ConcurrentNoWaitHookExecutionLimiter
}

// GetOrCreateConcurrentNoWaitHookLifecycleContext returns lifecycle context for detached hooks.
func (server *Server) GetOrCreateConcurrentNoWaitHookLifecycleContext() context.Context {
	if server == nil {
		return context.Background()
	}
	server.ConcurrentNoWaitHookLifecycleContextMutex.Lock()
	defer server.ConcurrentNoWaitHookLifecycleContextMutex.Unlock()
	if server.ConcurrentNoWaitHookLifecycleContext != nil {
		return server.ConcurrentNoWaitHookLifecycleContext
	}
	baseContext := server.CurrentRunCycleContextOrBackground()
	if baseContext == nil {
		baseContext = context.Background()
	}
	lifecycleContext, cancelLifecycleContext := context.WithCancel(baseContext)
	server.ConcurrentNoWaitHookLifecycleContext = lifecycleContext
	server.ConcurrentNoWaitHookLifecycleCancel = cancelLifecycleContext
	return lifecycleContext
}

// CancelConcurrentNoWaitHookLifecycleContext cancels detached no-wait hook lifecycle.
func (server *Server) CancelConcurrentNoWaitHookLifecycleContext() {
	if server == nil {
		return
	}
	server.ConcurrentNoWaitHookLifecycleContextMutex.Lock()
	cancelLifecycleContext := server.ConcurrentNoWaitHookLifecycleCancel
	server.ConcurrentNoWaitHookLifecycleContext = nil
	server.ConcurrentNoWaitHookLifecycleCancel = nil
	server.ConcurrentNoWaitHookLifecycleContextMutex.Unlock()
	if cancelLifecycleContext != nil {
		cancelLifecycleContext()
	}
}

// currentRefreshManager returns refresh manager snapshot.
func (server *Server) currentRefreshManager() *broadcast.Manager {
	server.Mu.Lock()
	defer server.Mu.Unlock()
	return server.RefreshManager
}

// filepathDir returns cleaned parent directory path.
func filepathDir(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	return filepath.Clean(filepath.Dir(path))
}

// resolveRefreshPortFromEnvironmentOrDefault resolves refresh port from env.
func resolveRefreshPortFromEnvironmentOrDefault(defaultPort int) int {
	fromEnvironment := strings.TrimSpace(os.Getenv("WAVE_REFRESH_PORT"))
	if fromEnvironment == "" {
		return defaultPort
	}
	parsedPort, parseError := strconv.Atoi(fromEnvironment)
	if parseError != nil || parsedPort <= 0 {
		return defaultPort
	}
	return parsedPort
}
