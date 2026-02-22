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
	"github.com/vormadev/vorma/wave/internal/wavecore"
	"github.com/vormadev/vorma/wave/tooling/builder"
	"github.com/vormadev/vorma/wave/tooling/devserver/internal/eventpipeline"
	"github.com/vormadev/vorma/wave/tooling/devserver/internal/hooks"
	"github.com/vormadev/vorma/wave/tooling/devserver/internal/restartengine"
	"github.com/vormadev/vorma/wave/tooling/devserver/internal/runloop"
	"github.com/vormadev/vorma/wave/tooling/devserver/internal/runtimeprocess"
	"github.com/vormadev/vorma/wave/tooling/internal/broadcast"
	"github.com/vormadev/vorma/wave/tooling/internal/shared"
	"github.com/vormadev/vorma/wave/tooling/internal/watch"
	"github.com/vormadev/vorma/wave/tooling/internal/watch/classification"
	"github.com/vormadev/vorma/wave/tooling/internal/watch/dedup"
	"golang.org/x/sync/errgroup"
)

const defaultRefreshPort = 10000

// watcherExecutionTraceContext carries watcher cycle and batch identifiers.
type watcherExecutionTraceContext struct {
	CycleID uint64
	BatchID uint64
}

// runtimeServer owns dev runtime orchestration and mutable lifecycle state.
type runtimeServer struct {
	Cfg *wave.ParsedConfig
	Log *slog.Logger

	PortResolver *wavecore.Resolver
	Lock         *shared.DevLock

	Mu sync.Mutex

	Builder *builder.Builder
	Watcher *watch.Watcher

	AppCommand        *exec.Cmd
	AppProcessManager *runtimeprocess.AppProcessManager
	ViteContext       *vitecmd.BuildCtx

	RefreshManager   *broadcast.Manager
	RefreshServer    *http.Server
	RefreshPort      int
	RefreshMgrCancel context.CancelFunc

	RestartIntents *restartengine.RestartIntentAccumulator

	WaitingForBuildRetry bool
	WatcherStartCh       chan struct{}

	NextRunCycleID       uint64
	CurrentRunCycleScope *restartengine.RunCycleScope

	NextWatcherBatchID                  uint64
	CurrentWatcherExecutionTraceContext watcherExecutionTraceContext

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
	if validationError := builder.ValidateConfig(cfg); validationError != nil {
		return fmt.Errorf("config validation failed: %w", validationError)
	}
	if log == nil {
		log = slog.Default()
	}

	lock := shared.NewDevLock(cfg.Dist.Static())
	if lockAcquireError := lock.Acquire(); lockAcquireError != nil {
		return lockAcquireError
	}
	defer func() {
		_ = lock.Release()
	}()

	server := &runtimeServer{
		Cfg:          cfg,
		Log:          log,
		PortResolver: wavecore.NewResolver(),
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
func (server *runtimeServer) BuilderInstance() *builder.Builder {
	server.Mu.Lock()
	defer server.Mu.Unlock()
	return server.Builder
}

// getBuilder returns current builder instance.
func (server *runtimeServer) getBuilder() *builder.Builder {
	return server.BuilderInstance()
}

// setBuilder sets current builder instance.
func (server *runtimeServer) setBuilder(builderInstance *builder.Builder) {
	server.Mu.Lock()
	defer server.Mu.Unlock()
	server.Builder = builderInstance
}

// watcherInstance returns current watcher instance.
func (server *runtimeServer) WatcherInstance() *watch.Watcher {
	server.Mu.Lock()
	defer server.Mu.Unlock()
	return server.Watcher
}

// startRunCycleScope creates and sets one new run-cycle scope.
func (server *runtimeServer) startRunCycleScope() *restartengine.RunCycleScope {
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

// currentRunCycleScopeSnapshot returns current run-cycle scope pointer.
func (server *runtimeServer) currentRunCycleScopeSnapshot() *restartengine.RunCycleScope {
	if server == nil {
		return nil
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	return server.CurrentRunCycleScope
}

// CurrentRunCycleContextOrBackground returns cycle context or background.
func (server *runtimeServer) CurrentRunCycleContextOrBackground() context.Context {
	scope := server.currentRunCycleScopeSnapshot()
	if scope == nil || scope.ExecutionContext == nil {
		return context.Background()
	}
	return scope.ExecutionContext
}

// cancelAndJoinCurrentRunCycleScope cancels current run cycle and waits for work.
func (server *runtimeServer) cancelAndJoinCurrentRunCycleScope() {
	scope := server.currentRunCycleScopeSnapshot()
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

// launchRunCycleScopedAsyncWorkOrDetached launches work under cycle scope when present.
func (server *runtimeServer) launchRunCycleScopedAsyncWorkOrDetached(
	runAsyncWork func(context.Context),
) {
	if runAsyncWork == nil {
		return
	}
	scope := server.currentRunCycleScopeSnapshot()
	if scope != nil {
		scope.LaunchAsyncWork(runAsyncWork)
		return
	}
	go runAsyncWork(context.Background())
}

// DeriveWatcherExecutionTraceContext allocates next batch trace context.
func (server *runtimeServer) DeriveWatcherExecutionTraceContext() watcherExecutionTraceContext {
	if server == nil {
		return watcherExecutionTraceContext{}
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
	return watcherExecutionTraceContext{CycleID: cycleID, BatchID: batchID}
}

// SetCurrentWatcherExecutionTraceContext sets current trace context.
func (server *runtimeServer) SetCurrentWatcherExecutionTraceContext(
	traceContext watcherExecutionTraceContext,
) {
	if server == nil {
		return
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	server.CurrentWatcherExecutionTraceContext = traceContext
}

// ClearCurrentWatcherExecutionTraceContext clears current trace context.
func (server *runtimeServer) ClearCurrentWatcherExecutionTraceContext() {
	if server == nil {
		return
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	server.CurrentWatcherExecutionTraceContext = watcherExecutionTraceContext{}
}

// CurrentWatcherExecutionTraceContextSnapshot returns current trace context.
func (server *runtimeServer) CurrentWatcherExecutionTraceContextSnapshot() watcherExecutionTraceContext {
	if server == nil {
		return watcherExecutionTraceContext{}
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	return server.CurrentWatcherExecutionTraceContext
}

// BuildRunloopEngine creates runloop engine with dependency wiring.
func (server *runtimeServer) BuildRunloopEngine() *runloop.Engine {
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
				watcherExecutionTraceContext{
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

// ensureAppProcessManager returns existing manager or creates one.
func (server *runtimeServer) ensureAppProcessManager() *runtimeprocess.AppProcessManager {
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
func (server *runtimeServer) StartApp() {
	manager := server.ensureAppProcessManager()
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
func (server *runtimeServer) StopApp() error {
	manager := server.ensureAppProcessManager()
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
func (server *runtimeServer) InitWatcher() error {
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
	if addConfigDirectoryError := server.addConfigFileDirectory(); addConfigDirectoryError != nil {
		_ = watcherInstance.Close()
		server.Mu.Lock()
		if server.Watcher == watcherInstance {
			server.Watcher = nil
		}
		server.Mu.Unlock()
		return addConfigDirectoryError
	}
	return nil
}

// addConfigFileDirectory ensures configuration file directory is watched.
func (server *runtimeServer) addConfigFileDirectory() error {
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
func (server *runtimeServer) ReloadConfig() (*wave.ParsedConfig, error) {
	if server == nil || server.Cfg == nil {
		return nil, errors.New("server config unavailable")
	}
	if strings.TrimSpace(server.Cfg.Core.ConfigLocation) == "" {
		return server.Cfg, nil
	}

	newConfig, loadError := server.loadParsedConfigForReload(
		server.Cfg.Core.ConfigLocation,
	)
	if loadError != nil {
		return nil, loadError
	}
	if validationError := builder.ValidateConfig(newConfig); validationError != nil {
		return nil, validationError
	}

	server.Mu.Lock()
	server.Cfg = newConfig
	server.Mu.Unlock()
	return newConfig, nil
}

// loadParsedConfigForReload loads parsed config while preserving framework runtime callbacks.
func (server *runtimeServer) loadParsedConfigForReload(
	configLocation string,
) (*wave.ParsedConfig, error) {
	currentConfig := server.Cfg
	newConfig, loadError := wave.ParseConfigFile(configLocation)
	if loadError != nil {
		return nil, loadError
	}
	if currentConfig != nil {
		wave.CopyFrameworkRuntimeFieldsForToolingReload(newConfig, currentConfig)
	}
	return newConfig, nil
}

// WaitForApp waits until app healthcheck becomes ready.
func (server *runtimeServer) WaitForApp() bool {
	readyURL := runtimeprocess.ResolveAppReadyURL(
		server.MustGetPort(),
		server.Cfg.HealthcheckEndpoint(),
	)
	ready := server.waitForReadyURL(readyURL)
	if !ready {
		server.Log.Warn(
			"app did not become ready before timeout",
			"url",
			readyURL,
		)
	}
	return ready
}

// waitForReadyURL waits for one readiness URL.
func (server *runtimeServer) waitForReadyURL(url string) bool {
	return server.WaitForAnyReady([]string{url})
}

// WaitForAnyReady waits until at least one URL is ready.
func (server *runtimeServer) WaitForAnyReady(urls []string) bool {
	return runtimeprocess.WaitForAnyReady(
		urls,
		runtimeprocess.DefaultReadinessWaitPolicy(),
	)
}

// MustGetPort returns app runtime port for devserver orchestration.
func (server *runtimeServer) MustGetPort() int {
	if server == nil {
		return 0
	}
	if server.PortResolver == nil {
		server.PortResolver = wavecore.NewResolver()
	}
	return server.PortResolver.MustGetPort()
}

// StartRefreshServer starts websocket refresh HTTP server.
func (server *runtimeServer) StartRefreshServer(preferredPort int) (int, error) {
	if server != nil && server.Cfg != nil && server.Cfg.Core != nil &&
		server.Cfg.Core.ServerOnlyMode {
		return 0, nil
	}
	if preferredPort < 0 {
		return 0, fmt.Errorf("invalid refresh server port: %d", preferredPort)
	}

	server.Mu.Lock()
	if server.RefreshServer != nil {
		refreshPort := server.RefreshPort
		server.Mu.Unlock()
		return refreshPort, nil
	}
	refreshManager := broadcast.NewManager(
		server.Log,
		broadcast.ManagerConfig{},
	)
	listenOnPort := func(port int) (net.Listener, error) {
		return net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	}

	listener, listenError := listenOnPort(preferredPort)
	if listenError != nil && preferredPort > 0 {
		fallbackListener, fallbackListenError := listenOnPort(0)
		if fallbackListenError != nil {
			server.Mu.Unlock()
			return 0, fmt.Errorf(
				"listen refresh server on preferred port %d: %w (fallback listen failed: %v)",
				preferredPort,
				listenError,
				fallbackListenError,
			)
		}
		listener = fallbackListener
		listenError = nil
	}
	if listenError != nil {
		server.Mu.Unlock()
		return 0, fmt.Errorf("listen refresh server: %w", listenError)
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port

	refreshServerContext, cancelRefreshServer := context.WithCancel(
		context.Background(),
	)
	refreshServer := &http.Server{
		Addr:    listener.Addr().String(),
		Handler: server.newRefreshServerMux(refreshManager),
	}

	server.RefreshMgrCancel = cancelRefreshServer
	server.RefreshManager = refreshManager
	server.RefreshServer = refreshServer
	server.RefreshPort = actualPort
	server.Mu.Unlock()

	server.launchRunCycleScopedAsyncWorkOrDetached(func(context.Context) {
		refreshManager.Run(refreshServerContext)
	})
	server.launchRunCycleScopedAsyncWorkOrDetached(func(context.Context) {
		if serveError := refreshServer.Serve(listener); serveError != nil &&
			!errors.Is(serveError, http.ErrServerClosed) {
			server.Log.Error("refresh server serve failed", "error", serveError)
		}
	})
	return actualPort, nil
}

// newRefreshServerMux builds mux for refresh endpoints.
func (server *runtimeServer) newRefreshServerMux(
	refreshManager *broadcast.Manager,
) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(
		"/events",
		func(responseWriter http.ResponseWriter, request *http.Request) {
			responseWriter.Header().Set("Access-Control-Allow-Origin", "*")
			responseWriter.Header().Set(
				"Access-Control-Allow-Methods",
				"GET, OPTIONS",
			)
			if request.Method == http.MethodOptions {
				responseWriter.WriteHeader(http.StatusNoContent)
				return
			}
			refreshManager.ServeHTTP(responseWriter, request)
		},
	)
	mux.Handle("/refresh", refreshManager)
	mux.HandleFunc(
		"/get-refresh-script-inner",
		func(responseWriter http.ResponseWriter, _ *http.Request) {
			responseWriter.Header().Set("Content-Type", "text/plain")
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write(
				[]byte("// wave refresh script placeholder"),
			)
		},
	)
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
func (server *runtimeServer) StopRefreshServer() error {
	server.Mu.Lock()
	refreshServer := server.RefreshServer
	refreshManager := server.RefreshManager
	cancelRefreshManager := server.RefreshMgrCancel
	server.RefreshServer = nil
	server.RefreshManager = nil
	server.RefreshPort = 0
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
	return nil
}

// getOrCreateRestartIntentAccumulator returns existing or initializes restart accumulator.
func (server *runtimeServer) getOrCreateRestartIntentAccumulator() *restartengine.RestartIntentAccumulator {
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
func (server *runtimeServer) SetWaitingForBuildRetry(waiting bool) {
	if server == nil {
		return
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	server.WaitingForBuildRetry = waiting
}

// QueueRestartRequest queues restart request into intent accumulator.
func (server *runtimeServer) QueueRestartRequest(
	request restartengine.RestartRequest,
) {
	accumulator := server.getOrCreateRestartIntentAccumulator()
	if accumulator == nil {
		return
	}

	server.Mu.Lock()
	waitingForBuildRetry := server.WaitingForBuildRetry
	server.Mu.Unlock()
	if waitingForBuildRetry && accumulator.HasQueuedOrPendingRequest() {
		return
	}

	accumulator.Queue(request)
}

// ConsumePendingRestartRequest consumes pending restart request.
func (server *runtimeServer) ConsumePendingRestartRequest() (restartengine.RestartRequest, bool) {
	accumulator := server.getOrCreateRestartIntentAccumulator()
	if accumulator == nil {
		return restartengine.RestartRequest{}, false
	}
	return accumulator.ConsumePending()
}

// consumeRestartRequestBlocking consumes restart request blocking.
func (server *runtimeServer) consumeRestartRequestBlocking() restartengine.RestartRequest {
	accumulator := server.getOrCreateRestartIntentAccumulator()
	if accumulator == nil {
		return restartengine.RestartRequest{}
	}
	return accumulator.ConsumeBlocking()
}

// TriggerRestart requests restart with go recompilation.
func (server *runtimeServer) TriggerRestart() {
	server.triggerRestartWithOpts(true, false)
}

// TriggerRestartNoGo requests restart without go recompilation.
func (server *runtimeServer) TriggerRestartNoGo() {
	server.triggerRestartWithOpts(false, false)
}

// TriggerConfigRestart requests config restart semantics.
func (server *runtimeServer) TriggerConfigRestart() {
	server.triggerRestartWithOpts(true, true)
}

// triggerRestartWithOpts queues normalized restart request.
func (server *runtimeServer) triggerRestartWithOpts(
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
func (server *runtimeServer) Run() error {
	wave.SetModeToDev()

	if _, startRefreshServerError := server.StartRefreshServer(
		resolveRefreshPortFromEnvironmentOrDefault(defaultRefreshPort),
	); startRefreshServerError != nil {
		return startRefreshServerError
	}
	defer server.StopRefreshServer()
	defer server.CleanupForRebuild()

	currentState := restartengine.RunLifecycleStatePreparingCycle
	currentIntent := restartengine.RunIntent{
		RecompileGo:     true,
		IsRebuild:       false,
		IsConfigRestart: false,
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

		result, executeCommandError := server.executeRunLifecycleCommand(
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

// executeRunLifecycleCommand executes one lifecycle command.
func (server *runtimeServer) executeRunLifecycleCommand(
	command restartengine.RunLifecycleCommand,
	input restartengine.RunLifecycleCommandInput,
) (restartengine.RunLifecycleCommandResult, error) {
	switch command {
	case restartengine.RunLifecycleCommandPrepareCycle:
		return server.executeRunLifecycleCommandPrepareCycle(input)
	case restartengine.RunLifecycleCommandBuildCycle:
		return server.executeRunLifecycleCommandBuildCycle(input)
	case restartengine.RunLifecycleCommandAwaitBuildRetry:
		return server.executeRunLifecycleCommandAwaitBuildRetry(input)
	case restartengine.RunLifecycleCommandStartRuntime:
		return server.executeRunLifecycleCommandStartRuntime(input)
	case restartengine.RunLifecycleCommandAwaitRestartRequest:
		return server.executeRunLifecycleCommandAwaitRestartRequest(input)
	case restartengine.RunLifecycleCommandCleanupForNextCycle:
		return server.executeRunLifecycleCommandCleanupForNextCycle(input)
	default:
		return restartengine.RunLifecycleCommandResult{}, fmt.Errorf(
			"unknown run lifecycle command: %d",
			command,
		)
	}
}

// prepareRunCycle prepares run-cycle state and process resources.
func (server *runtimeServer) prepareRunCycle(
	firstRun bool,
	currentRunIntent restartengine.RunIntent,
) error {
	if firstRun {
		server.MustGetPort()
	}
	server.cancelConcurrentNoWaitHookLifecycleContext()
	_ = server.StopVite()

	if !firstRun && server.Cfg != nil && server.Cfg.Core != nil &&
		strings.TrimSpace(server.Cfg.Core.ConfigLocation) != "" {
		if currentRunIntent.IsConfigRestart {
			_ = server.WaitForApp()
		}
		if _, reloadConfigError := server.ReloadConfig(); reloadConfigError != nil {
			return fmt.Errorf("reload config: %w", reloadConfigError)
		}
	}

	if initWatcherError := server.InitWatcher(); initWatcherError != nil {
		return fmt.Errorf("init watcher: %w", initWatcherError)
	}

	builderInstance := builder.NewBuilder(
		server.Cfg,
		server.Log,
	)
	if builderInstance == nil {
		return errors.New("builder initialization returned nil")
	}
	server.setBuilder(builderInstance)
	return nil
}

// executeRunBuildForIntent executes build phase for provided run intent.
func (server *runtimeServer) executeRunBuildForIntent(
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

// startRunCycleRuntime starts app/vite and watcher runloop for runtime phase.
func (server *runtimeServer) startRunCycleRuntime() {
	if server.Cfg.UsingVite() {
		if startViteError := server.StartVite(); startViteError != nil {
			server.Log.Error("start vite failed", "error", startViteError)
		}
	}

	server.StartApp()

	cycleScope := server.startRunCycleScope()
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
func (server *runtimeServer) WaitForBuildRetry() restartengine.RestartRequest {
	server.SetWaitingForBuildRetry(true)
	defer server.SetWaitingForBuildRetry(false)

	if pendingRequest, hasPendingRequest := server.ConsumePendingRestartRequest(); hasPendingRequest {
		return restartengine.NormalizeRestartRequest(pendingRequest)
	}

	server.WatcherStartCh = make(chan struct{})
	cycleScope := server.startRunCycleScope()
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

	return server.consumeRestartRequestBlocking()
}

// CleanupForRebuild performs stop/cancel tasks before next run cycle.
func (server *runtimeServer) CleanupForRebuild() {
	server.cancelAndJoinCurrentRunCycleScope()
	server.cancelConcurrentNoWaitHookLifecycleContext()

	server.Mu.Lock()
	watcherForCleanup := server.Watcher
	builderForCleanup := server.Builder
	server.Watcher = nil
	server.Builder = nil
	server.Mu.Unlock()

	if watcherForCleanup != nil {
		_ = watcherForCleanup.Close()
	}
	if builderForCleanup != nil {
		builderForCleanup.Close()
	}

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

// executeRunLifecycleCommandPrepareCycle handles prepare-cycle lifecycle command.
func (server *runtimeServer) executeRunLifecycleCommandPrepareCycle(
	input restartengine.RunLifecycleCommandInput,
) (restartengine.RunLifecycleCommandResult, error) {
	if prepareError := server.prepareRunCycle(
		input.FirstRun,
		input.CurrentIntent,
	); prepareError != nil {
		return restartengine.RunLifecycleCommandResult{}, prepareError
	}
	return restartengine.RunLifecycleCommandResult{
		RunLifecycleEvent: restartengine.RunLifecycleEventCyclePrepared,
	}, nil
}

// executeRunLifecycleCommandBuildCycle handles build-cycle lifecycle command.
func (server *runtimeServer) executeRunLifecycleCommandBuildCycle(
	input restartengine.RunLifecycleCommandInput,
) (restartengine.RunLifecycleCommandResult, error) {
	buildError := server.executeRunBuildForIntent(
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

// executeRunLifecycleCommandAwaitBuildRetry handles retry wait after failed build.
func (server *runtimeServer) executeRunLifecycleCommandAwaitBuildRetry(
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

// executeRunLifecycleCommandStartRuntime handles runtime start lifecycle command.
func (server *runtimeServer) executeRunLifecycleCommandStartRuntime(
	_ restartengine.RunLifecycleCommandInput,
) (restartengine.RunLifecycleCommandResult, error) {
	server.startRunCycleRuntime()
	return restartengine.RunLifecycleCommandResult{
		RunLifecycleEvent: restartengine.RunLifecycleEventRuntimeStarted,
	}, nil
}

// executeRunLifecycleCommandAwaitRestartRequest handles restart wait lifecycle command.
func (server *runtimeServer) executeRunLifecycleCommandAwaitRestartRequest(
	_ restartengine.RunLifecycleCommandInput,
) (restartengine.RunLifecycleCommandResult, error) {
	restartRequest := server.consumeRestartRequestBlocking()
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

// executeRunLifecycleCommandCleanupForNextCycle handles cleanup lifecycle command.
func (server *runtimeServer) executeRunLifecycleCommandCleanupForNextCycle(
	_ restartengine.RunLifecycleCommandInput,
) (restartengine.RunLifecycleCommandResult, error) {
	server.CleanupForRebuild()
	return restartengine.RunLifecycleCommandResult{
		RunLifecycleEvent: restartengine.RunLifecycleEventCleanupCompleted,
	}, nil
}

// StartVite starts Vite dev process context.
func (server *runtimeServer) StartVite() error {
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
	if viteBuildError := server.ViteContext.DevBuild(); viteBuildError != nil {
		server.ViteContext = nil
		return viteBuildError
	}
	return nil
}

// StopVite stops Vite process context.
func (server *runtimeServer) StopVite() error {
	server.Mu.Lock()
	viteContext := server.ViteContext
	server.ViteContext = nil
	server.Mu.Unlock()
	if viteContext == nil {
		return nil
	}
	viteContext.Cleanup()
	return nil
}

// CycleVite restarts Vite and does not wait for readiness.
func (server *runtimeServer) CycleVite() {
	_ = server.StopVite()
	if startViteError := server.StartVite(); startViteError != nil {
		server.Log.Warn("cycle vite failed", "error", startViteError)
	}
}

// cycleViteAndWaitForReadiness cycles Vite and waits for readiness.
func (server *runtimeServer) cycleViteAndWaitForReadiness() bool {
	server.CycleVite()
	return server.WaitForVite()
}

// CallViteFilemapInvalidate calls configured invalidate endpoint in Vite runtime.
func (server *runtimeServer) CallViteFilemapInvalidate() error {
	viteContext := server.ViteContext
	if viteContext == nil {
		return errors.New("vite not running")
	}

	callInvalidateEndpoint := func(invalidatePath string) (int, error) {
		invalidateURL := "http://127.0.0.1:" + strconv.Itoa(
			viteContext.Port(),
		) + invalidatePath
		request, requestCreateError := http.NewRequest(
			http.MethodPost,
			invalidateURL,
			nil,
		)
		if requestCreateError != nil {
			return 0, requestCreateError
		}
		response, requestError := (&http.Client{
			Timeout: 2 * time.Second,
		}).Do(request)
		if requestError != nil {
			return 0, requestError
		}
		defer response.Body.Close()
		return response.StatusCode, nil
	}

	primaryStatusCode, primaryRequestError := callInvalidateEndpoint(
		"/__vorma_invalidate_filemap",
	)
	if primaryRequestError != nil {
		return primaryRequestError
	}
	if primaryStatusCode >= 400 && primaryStatusCode != http.StatusNotFound {
		return fmt.Errorf(
			"vite invalidate endpoint returned %d",
			primaryStatusCode,
		)
	}
	if primaryStatusCode < 400 {
		return nil
	}

	fallbackStatusCode, fallbackRequestError := callInvalidateEndpoint(
		"/__wave/vite-filemap-invalidate",
	)
	if fallbackRequestError != nil {
		return fallbackRequestError
	}
	if fallbackStatusCode >= 400 {
		return fmt.Errorf(
			"vite invalidate endpoint returned %d",
			fallbackStatusCode,
		)
	}
	return nil
}

// WaitForVite waits for vite readiness on known probe URLs.
func (server *runtimeServer) WaitForVite() bool {
	viteContext := server.ViteContext
	if viteContext == nil {
		return true
	}
	return server.WaitForAnyReady(resolveViteReadyURLs(viteContext.Port()))
}

// resolveViteReadyURL resolves default Vite readiness probe URL.
func resolveViteReadyURL(vitePort int) string {
	return runtimeprocess.ResolveReadinessProbeURL(
		runtimeprocess.LocalReadinessProbeHostIPv4,
		vitePort,
		"/@vite/client",
	)
}

// resolveViteReadyURLs resolves Vite readiness probe URLs.
func resolveViteReadyURLs(vitePort int) []string {
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
func (server *runtimeServer) BroadcastRebuilding() {
	if !server.shouldBroadcastToBrowserClients() {
		return
	}
	refreshManager := server.currentRefreshManager()
	if refreshManager == nil {
		return
	}
	refreshManager.BroadcastRebuilding()
}

// BroadcastReload broadcasts reload payload with readiness handling.
func (server *runtimeServer) BroadcastReload(reloadOptions eventpipeline.ReloadOpts) {
	if !server.shouldBroadcastToBrowserClients() {
		return
	}

	if !server.waitForReloadReadiness(reloadOptions) {
		server.Log.Warn("reload readiness failed; skipping browser broadcast")
		return
	}

	if !server.shouldBroadcastReloadPayloadAfterReadiness(reloadOptions) {
		return
	}

	refreshManager := server.currentRefreshManager()
	if refreshManager == nil {
		return
	}
	refreshManager.Broadcast(reloadOptions.Payload)
}

// shouldBroadcastToBrowserClients reports whether browser broadcast should run.
func (server *runtimeServer) shouldBroadcastToBrowserClients() bool {
	return server.Cfg != nil &&
		server.Cfg.UsingBrowser() &&
		server.currentRefreshManager() != nil
}

// waitForReloadReadiness applies readiness policy for reload.
func (server *runtimeServer) waitForReloadReadiness(
	reloadOptions eventpipeline.ReloadOpts,
) bool {
	if reloadOptions.CycleVite {
		cycleViteRequestedAndApplicable := false
		if server.Cfg != nil && server.Cfg.UsingVite() {
			server.Mu.Lock()
			cycleViteRequestedAndApplicable = server.ViteContext != nil
			server.Mu.Unlock()
		}
		if cycleViteRequestedAndApplicable {
			if !server.cycleViteAndWaitForReadiness() {
				server.Log.Warn(
					"cycle vite readiness failed; falling back to payload broadcast",
				)
			}
		}
	}

	if reloadOptions.WaitApp {
		if !server.waitForReadyURL(
			runtimeprocess.ResolveAppReadyURL(
				server.MustGetPort(),
				server.Cfg.HealthcheckEndpoint(),
			),
		) {
			return false
		}
	}
	if reloadOptions.WaitVite && server.ViteContext != nil {
		if !server.WaitForAnyReady(resolveViteReadyURLs(server.ViteContext.Port())) {
			return false
		}
	}
	return true
}

// ShouldBroadcastReloadPayloadAfterReadiness returns whether payload should be sent.
func (server *runtimeServer) shouldBroadcastReloadPayloadAfterReadiness(
	reloadOptions eventpipeline.ReloadOpts,
) bool {
	if !reloadOptions.CycleVite {
		return true
	}
	if server == nil || server.Cfg == nil || !server.Cfg.UsingVite() {
		return true
	}

	server.Mu.Lock()
	defer server.Mu.Unlock()
	if server.ViteContext != nil {
		return false
	}
	return true
}

// BuildEventExecutionPlan classifies events and builds hook-ready execution plan.
func (server *runtimeServer) BuildEventExecutionPlan(
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
func (server *runtimeServer) ExecuteBrowserPhase(work *eventpipeline.WorkSet) {
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
		server.executeHotReloadCSSBrowserPhase(work)
	}
}

// executeHotReloadCSSBrowserPhase executes CSS hot reload payloads.
func (server *runtimeServer) executeHotReloadCSSBrowserPhase(
	work *eventpipeline.WorkSet,
) {
	builderInstance := server.BuilderInstance()
	if builderInstance == nil {
		return
	}

	criticalCSS := ""
	criticalCSSAvailable := false
	if work != nil && work.Build.BuildCriticalCSS {
		if freshCriticalCSS, readCriticalCSSError := builderInstance.ReadCriticalCSSForHotReload(true); readCriticalCSSError == nil {
			criticalCSS = freshCriticalCSS
			criticalCSSAvailable = true
		}
	}

	normalCSSURL := ""
	normalCSSURLAvailable := false
	if work != nil && work.Build.BuildNormalCSS {
		if freshNormalCSSURL, readNormalCSSURLError := builderInstance.ReadNormalCSSURLForHotReload(true); readNormalCSSURLError == nil {
			normalCSSURL = freshNormalCSSURL
			normalCSSURLAvailable = true
		}
	}
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
func (server *runtimeServer) ExecuteBuildPhase(work *eventpipeline.WorkSet) error {
	if work == nil {
		return nil
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

	builderInstance := server.BuilderInstance()
	if builderInstance == nil {
		return errors.New("builder is unavailable")
	}

	if executionDecision.BuildCriticalCSS ||
		executionDecision.BuildNormalCSS ||
		executionDecision.WriteFrameworkPublicFileMap ||
		eventpipeline.ShouldExecuteAnyFileProcessingForBuildDecision(
			executionDecision,
		) {
		if setupDistError := builder.SetupDistDir(server.Cfg); setupDistError != nil {
			return setupDistError
		}
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
			if publicProcessingError := server.executePublicStaticProcessingForBuildPhase(builderInstance, executionDecision.PublicStaticProcessing); publicProcessingError != nil {
				return publicProcessingError
			}
			if privateProcessingError := server.executePrivateStaticProcessingForBuildPhase(builderInstance, executionDecision.PrivateStaticProcessing); privateProcessingError != nil {
				return privateProcessingError
			}
			return nil
		})
	}

	return buildGroup.Wait()
}

// executePublicStaticProcessingForBuildPhase executes public static processing decision.
func (server *runtimeServer) executePublicStaticProcessingForBuildPhase(
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

// executePrivateStaticProcessingForBuildPhase executes private static processing decision.
func (server *runtimeServer) executePrivateStaticProcessingForBuildPhase(
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
func (server *runtimeServer) ClassifyWatcherEventsForProcessing(
	watcherEvents []fsnotify.Event,
	watcher *watch.Watcher,
	builderInstance *builder.Builder,
) ([]eventpipeline.ClassifiedEvent, bool) {
	classifiedEvents, configChanged := server.classifyWatcherEventsFromPreClassificationPlan(
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

// applyWatcherEventPreClassificationSideEffects handles side-effects before semantic classification.
func (server *runtimeServer) applyWatcherEventPreClassificationSideEffects(
	watcherEvent fsnotify.Event,
) (bool, error) {
	if server.IsConfigFile(watcherEvent.Name) {
		_, reloadError := server.ReloadConfig()
		if reloadError != nil {
			return true, reloadError
		}
		return true, nil
	}
	return false, nil
}

// classifyWatcherEventsFromPreClassificationPlan classifies events after pre side effects.
func (server *runtimeServer) classifyWatcherEventsFromPreClassificationPlan(
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
		configChanged, sideEffectError := server.applyWatcherEventPreClassificationSideEffects(
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
func (server *runtimeServer) IsConfigFile(path string) bool {
	if server == nil || server.Cfg == nil || server.Cfg.Core == nil {
		return false
	}
	return classification.IsConfigurationPathChange(
		path,
		server.Cfg.Core.ConfigLocation,
	)
}

// ClassifyEventWithWatcherAndBuilder classifies watcher event with semantic file-type logic.
func (server *runtimeServer) ClassifyEventWithWatcherAndBuilder(
	watcherEvent fsnotify.Event,
	watcher *watch.Watcher,
	builderInstance *builder.Builder,
) eventpipeline.ClassifiedEvent {
	classifiedEvent := eventpipeline.ClassifiedEvent{Event: watcherEvent}
	preDecision := classification.DerivePreClassificationDecision(
		watcherEvent,
		classification.PathClassifierDependencies{
			IsIgnoredPathFunc: watcher.IsIgnoredFile,
			LockFileName:      shared.LockFileName,
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
func (server *runtimeServer) ResolveHookExecutionPlan(
	hook wave.OnChangeHook,
) hooks.HookExecutionPlan {
	if server == nil || server.Cfg == nil {
		return hooks.DeriveHookExecutionPlanFromHook(hook, nil)
	}
	if !hook.RunCombinedDevBuildHookCommands ||
		server.Cfg.FrameworkRunBuildHook == nil {
		return hooks.DeriveHookExecutionPlanFromHook(
			hook,
			server.ResolveHookCommand,
		)
	}

	frameworkBuildHookRunner := server.Cfg.FrameworkRunBuildHook
	userAndExplicitCommand := hooks.ResolveSequentialShellCommands(
		hook.Cmd,
		getUserDevBuildHook(server.Cfg),
	)
	originalCallback := hook.Callback

	return hooks.HookExecutionPlan{
		Callback: func(
			hookContext *wave.HookContext,
		) (*wave.RefreshAction, error) {
			var callbackAction *wave.RefreshAction
			var callbackError error
			if originalCallback != nil {
				callbackAction, callbackError = originalCallback(hookContext)
				if callbackError != nil {
					return callbackAction, callbackError
				}
			}

			hookExecutionContext := context.Background()
			if hookContext != nil && hookContext.ExecutionContext != nil {
				hookExecutionContext = hookContext.ExecutionContext
			}

			if strings.TrimSpace(userAndExplicitCommand) != "" {
				if executeCommandError := hooks.ExecuteHookCommandWithContext(
					hookExecutionContext,
					userAndExplicitCommand,
				); executeCommandError != nil {
					return callbackAction, executeCommandError
				}
			}

			if frameworkRunError := frameworkBuildHookRunner(
				hookExecutionContext,
				true,
			); frameworkRunError != nil {
				return callbackAction, frameworkRunError
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

// ResolveHookCommand resolves configured command behavior for one hook.
func (server *runtimeServer) ResolveHookCommand(hook wave.OnChangeHook) string {
	if hook.RunCombinedDevBuildHookCommands {
		if strings.TrimSpace(hook.Cmd) != "" {
			return hooks.ResolveSequentialShellCommands(
				hook.Cmd,
				getUserDevBuildHook(server.Cfg),
				getFrameworkDevBuildHook(server.Cfg),
			)
		}
		return hooks.ResolveSequentialShellCommands(
			getUserDevBuildHook(server.Cfg),
			getFrameworkDevBuildHook(server.Cfg),
		)
	}
	return hook.Cmd
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

// RunNoWaitHookWithConcurrencyLimit executes callback under bounded semaphore.
func (server *runtimeServer) RunNoWaitHookWithConcurrencyLimit(runNoWaitHook func()) {
	if runNoWaitHook == nil {
		return
	}
	limiter := server.ensureConcurrentNoWaitHookExecutionLimiter()
	if limiter == nil {
		go runNoWaitHook()
		return
	}
	go func() {
		limiter <- struct{}{}
		defer func() { <-limiter }()
		runNoWaitHook()
	}()
}

// ensureConcurrentNoWaitHookExecutionLimiter ensures semaphore exists.
func (server *runtimeServer) ensureConcurrentNoWaitHookExecutionLimiter() chan struct{} {
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
func (server *runtimeServer) GetOrCreateConcurrentNoWaitHookLifecycleContext() context.Context {
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

// cancelConcurrentNoWaitHookLifecycleContext cancels detached no-wait hook lifecycle.
func (server *runtimeServer) cancelConcurrentNoWaitHookLifecycleContext() {
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
func (server *runtimeServer) currentRefreshManager() *broadcast.Manager {
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
