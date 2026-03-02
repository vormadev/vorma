package runloop_test

import (
	"context"
	"errors"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveframework"
	"github.com/vormadev/vorma/wave/wavewatch"
	"log/slog"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave/buildtime/builder"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/eventpipeline"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/hooks"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/restartengine"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/runloop"
	"github.com/vormadev/vorma/wave/buildtime/internal/watch"
	"github.com/vormadev/vorma/wave/buildtime/internal/watch/classification"
	"github.com/vormadev/vorma/wave/buildtime/internal/watch/dedup"
	"github.com/vormadev/vorma/wave/internal/wavelock"
	"golang.org/x/sync/errgroup"
)

func isConfigFileForRunloopTests(cfg *waveconfig.ParsedConfig, path string) bool {
	if cfg == nil || cfg.Core == nil {
		return false
	}
	return classification.IsConfigurationPathChange(
		path,
		cfg.Core.ConfigLocation,
	)
}

func isConfigMutationWatcherEventForRunloopTests(
	watcherEvent fsnotify.Event,
) bool {
	return watcherEvent.Has(fsnotify.Write) ||
		watcherEvent.Has(fsnotify.Create) ||
		watcherEvent.Has(fsnotify.Remove) ||
		watcherEvent.Has(fsnotify.Rename)
}

func classifyEventWithWatcherAndBuilderForRunloopTests(
	watcherEvent fsnotify.Event,
	watcherForClassification *watch.Watcher,
	builderForClassification *builder.Builder,
) eventpipeline.ClassifiedEvent {
	classifiedEvent := eventpipeline.ClassifiedEvent{Event: watcherEvent}

	isIgnoredPathFunc := func(string) bool { return false }
	findWatchedFile := func(string) *wavewatch.WatchedFile { return nil }
	if watcherForClassification != nil {
		isIgnoredPathFunc = watcherForClassification.IsIgnoredFile
		findWatchedFile = watcherForClassification.FindWatchedFile
	}

	preClassificationDecision := classification.DerivePreClassificationDecision(
		watcherEvent,
		classification.PathClassifierDependencies{
			IsIgnoredPathFunc: isIgnoredPathFunc,
			LockFileName:      wavelock.LockFileName,
		},
	)
	if !preClassificationDecision.IncludeEvent {
		classifiedEvent.Ignored = true
		classifiedEvent.ChmodOnly = true
		return classifiedEvent
	}

	classifiedEvent.Ignored = isIgnoredPathFunc(watcherEvent.Name)
	classifiedEvent.FileType = eventpipeline.DeriveInitialFileTypeForWatcherEvent(
		watcherEvent.Name,
		watcherForClassification,
		builderForClassification,
	)
	classifiedEvent.WatchedFile = findWatchedFile(watcherEvent.Name)
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

func classifyWatcherEventsForProcessingForRunloopTests(
	cfg *waveconfig.ParsedConfig,
	watcherEvents []fsnotify.Event,
	watcherForClassification *watch.Watcher,
	builderForClassification *builder.Builder,
) ([]eventpipeline.ClassifiedEvent, bool) {
	classifiedEvents, configChanged := classifyWatcherEventsFromPreClassificationPlanForRunloopTests(
		cfg,
		watcherEvents,
		watcherForClassification,
		builderForClassification,
	)
	if configChanged {
		return nil, true
	}
	return eventpipeline.FilterClassifiedEventsForProcessingByPostClassificationDecision(
		classifiedEvents,
	), false
}

func classifyWatcherEventsFromPreClassificationPlanForRunloopTests(
	cfg *waveconfig.ParsedConfig,
	watcherEvents []fsnotify.Event,
	watcherForClassification *watch.Watcher,
	builderForClassification *builder.Builder,
) ([]eventpipeline.ClassifiedEvent, bool) {
	deduplicatedEvents := dedup.DeduplicateWatcherEvents(
		watcherEvents,
		dedup.DeduplicationPolicy{},
	).Events
	classifiedEvents := make(
		[]eventpipeline.ClassifiedEvent,
		0,
		len(deduplicatedEvents),
	)
	for _, watcherEvent := range deduplicatedEvents {
		if isConfigFileForRunloopTests(cfg, watcherEvent.Name) &&
			isConfigMutationWatcherEventForRunloopTests(watcherEvent) {
			return nil, true
		}
		classifiedEvents = append(
			classifiedEvents,
			classifyEventWithWatcherAndBuilderForRunloopTests(
				watcherEvent,
				watcherForClassification,
				builderForClassification,
			),
		)
	}
	return classifiedEvents, false
}

func buildEventExecutionPlanForRunloopTests(
	cfg *waveconfig.ParsedConfig,
	watcherEvents []fsnotify.Event,
	watcherForPlan *watch.Watcher,
	builderForPlan *builder.Builder,
) eventpipeline.EventExecutionPlanningResult {
	classifiedEvents, configChanged := classifyWatcherEventsForProcessingForRunloopTests(
		cfg,
		watcherEvents,
		watcherForPlan,
		builderForPlan,
	)
	if configChanged {
		return eventpipeline.EventExecutionPlanningResult{
			ConfigChanged: true,
		}
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

func resolveHookCommandForRunloopTests(
	parsedConfig *waveconfig.ParsedConfig,
	hook wavewatch.OnChangeHook,
) string {
	if !hook.RunCombinedDevBuildHookCommands {
		return hook.Cmd
	}
	if strings.TrimSpace(hook.Cmd) != "" {
		return hooks.ResolveSequentialShellCommands(
			hook.Cmd,
			getUserDevBuildHookForRunloopTests(parsedConfig),
			getFrameworkDevBuildHookForRunloopTests(parsedConfig),
		)
	}
	return hooks.ResolveSequentialShellCommands(
		getUserDevBuildHookForRunloopTests(parsedConfig),
		getFrameworkDevBuildHookForRunloopTests(parsedConfig),
	)
}

func resolveHookExecutionPlanForRunloopTests(
	parsedConfig *waveconfig.ParsedConfig,
	hook wavewatch.OnChangeHook,
) hooks.HookExecutionPlan {
	if !hook.RunCombinedDevBuildHookCommands || parsedConfig == nil ||
		waveframework.StateForConfig(parsedConfig).RunBuildHook == nil {
		return hooks.DeriveHookExecutionPlanFromHook(
			hook,
			func(hookForResolution wavewatch.OnChangeHook) string {
				return resolveHookCommandForRunloopTests(
					parsedConfig,
					hookForResolution,
				)
			},
		)
	}

	frameworkBuildHookRunner := waveframework.StateForConfig(parsedConfig).RunBuildHook
	userAndExplicitCommand := hooks.ResolveSequentialShellCommands(
		hook.Cmd,
		getUserDevBuildHookForRunloopTests(parsedConfig),
	)
	originalCallback := hook.Callback

	return hooks.HookExecutionPlan{
		Callback: func(
			hookContext *wavewatch.HookContext,
		) (*wavewatch.RefreshAction, error) {
			var callbackAction *wavewatch.RefreshAction
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

func getUserDevBuildHookForRunloopTests(
	parsedConfig *waveconfig.ParsedConfig,
) string {
	if parsedConfig == nil || parsedConfig.Core == nil {
		return ""
	}
	return parsedConfig.Core.DevBuildHook
}

func getFrameworkDevBuildHookForRunloopTests(
	parsedConfig *waveconfig.ParsedConfig,
) string {
	if parsedConfig == nil {
		return ""
	}
	return waveframework.StateForConfig(parsedConfig).DevBuildHook
}

type restartIntentQueueHarness struct {
	waitingForBuildRetry bool
	waitingMutex         sync.Mutex
	restartIntents       *restartengine.RestartIntentAccumulator
}

func newRestartIntentQueueHarness() *restartIntentQueueHarness {
	return &restartIntentQueueHarness{
		restartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}
}

func (harness *restartIntentQueueHarness) QueueRestartRequest(
	request restartengine.RestartRequest,
) {
	if harness == nil || harness.restartIntents == nil {
		return
	}

	harness.waitingMutex.Lock()
	waitingForBuildRetry := harness.waitingForBuildRetry
	harness.waitingMutex.Unlock()
	if waitingForBuildRetry &&
		harness.restartIntents.HasQueuedOrPendingRequest() {
		return
	}

	harness.restartIntents.Queue(request)
}

func (harness *restartIntentQueueHarness) ConsumePendingRestartRequest() (
	restartengine.RestartRequest,
	bool,
) {
	if harness == nil || harness.restartIntents == nil {
		return restartengine.RestartRequest{}, false
	}
	return harness.restartIntents.ConsumePending()
}

func (harness *restartIntentQueueHarness) WaitForBuildRetry() restartengine.RestartRequest {
	if harness == nil || harness.restartIntents == nil {
		return restartengine.RestartRequest{}
	}

	harness.waitingMutex.Lock()
	harness.waitingForBuildRetry = true
	harness.waitingMutex.Unlock()
	defer func() {
		harness.waitingMutex.Lock()
		harness.waitingForBuildRetry = false
		harness.waitingMutex.Unlock()
	}()

	if pendingRequest, hasPendingRequest := harness.ConsumePendingRestartRequest(); hasPendingRequest {
		return restartengine.NormalizeRestartRequest(pendingRequest)
	}
	return harness.restartIntents.ConsumeBlocking()
}

func runNoWaitHookWithConcurrencyLimitForRunloopTests(
	concurrencyLimiter chan struct{},
	runNoWaitHook func(),
) {
	if runNoWaitHook == nil {
		return
	}
	if concurrencyLimiter == nil {
		go runNoWaitHook()
		return
	}
	go func() {
		concurrencyLimiter <- struct{}{}
		defer func() { <-concurrencyLimiter }()
		runNoWaitHook()
	}()
}

type runloopTestServer struct {
	Cfg     *waveconfig.ParsedConfig
	Log     *slog.Logger
	Mu      sync.Mutex
	Builder *builder.Builder
	Watcher *watch.Watcher

	RestartIntents *restartengine.RestartIntentAccumulator

	currentRunCycleContext context.Context

	ConcurrentNoWaitHookExecutionLimiter chan struct{}
	concurrentNoWaitHookLifecycleContext context.Context
	concurrentNoWaitHookLifecycleCancel  context.CancelFunc
	concurrentNoWaitHookContextMutex     sync.Mutex

	nextWatcherBatchID                  uint64
	currentWatcherExecutionTraceContext runloop.WatcherExecutionTraceContext
}

func newRunloopTestServer(
	cfg *waveconfig.ParsedConfig,
	log *slog.Logger,
) *runloopTestServer {
	if log == nil {
		log = slog.Default()
	}
	return &runloopTestServer{
		Cfg: cfg,
		Log: log,
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
		ConcurrentNoWaitHookExecutionLimiter: make(
			chan struct{},
			hooks.MaxConcurrentNoWaitHookExecutions,
		),
	}
}

func (server *runloopTestServer) BuilderInstance() *builder.Builder {
	if server == nil {
		return nil
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	return server.Builder
}

func (server *runloopTestServer) WatcherInstance() *watch.Watcher {
	if server == nil {
		return nil
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	return server.Watcher
}

func (server *runloopTestServer) CurrentRunCycleContextOrBackground() context.Context {
	if server == nil {
		return context.Background()
	}
	server.Mu.Lock()
	runCycleContext := server.currentRunCycleContext
	server.Mu.Unlock()
	if runCycleContext == nil {
		return context.Background()
	}
	return runCycleContext
}

func (server *runloopTestServer) SetCurrentRunCycleContext(
	runCycleContext context.Context,
) {
	if server == nil {
		return
	}
	server.Mu.Lock()
	server.currentRunCycleContext = runCycleContext
	server.Mu.Unlock()
}

func (server *runloopTestServer) QueueRestartRequest(
	request restartengine.RestartRequest,
) {
	if server == nil {
		return
	}
	server.Mu.Lock()
	if server.RestartIntents == nil {
		server.RestartIntents = restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		)
	}
	restartIntents := server.RestartIntents
	server.Mu.Unlock()
	restartIntents.Queue(restartengine.NormalizeRestartRequest(request))
}

func (server *runloopTestServer) TriggerRestart() {
	server.QueueRestartRequest(
		restartengine.RestartRequest{
			RecompileGo:     true,
			IsConfigRestart: false,
		},
	)
}

func (server *runloopTestServer) TriggerRestartNoGo() {
	server.QueueRestartRequest(
		restartengine.RestartRequest{
			RecompileGo:     false,
			IsConfigRestart: false,
		},
	)
}

func (server *runloopTestServer) TriggerConfigRestart() {
	server.QueueRestartRequest(
		restartengine.RestartRequest{
			RecompileGo:     true,
			IsConfigRestart: true,
		},
	)
}

func (server *runloopTestServer) StartApp() {}

func (server *runloopTestServer) StopApp() error {
	return nil
}

func (server *runloopTestServer) BroadcastRebuilding() {}

func (server *runloopTestServer) BuildEventExecutionPlan(
	events []fsnotify.Event,
	watcherForPlan *watch.Watcher,
	builderForPlan *builder.Builder,
) eventpipeline.EventExecutionPlanningResult {
	if server == nil {
		return eventpipeline.EventExecutionPlanningResult{}
	}
	return buildEventExecutionPlanForRunloopTests(
		server.Cfg,
		events,
		watcherForPlan,
		builderForPlan,
	)
}

func (server *runloopTestServer) DeriveWatcherExecutionTraceContext() runloop.WatcherExecutionTraceContext {
	if server == nil {
		return runloop.WatcherExecutionTraceContext{}
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	server.nextWatcherBatchID++
	return runloop.WatcherExecutionTraceContext{
		CycleID: 0,
		BatchID: server.nextWatcherBatchID,
	}
}

func (server *runloopTestServer) SetCurrentWatcherExecutionTraceContext(
	traceContext runloop.WatcherExecutionTraceContext,
) {
	if server == nil {
		return
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	server.currentWatcherExecutionTraceContext = traceContext
}

func (server *runloopTestServer) ClearCurrentWatcherExecutionTraceContext() {
	if server == nil {
		return
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	server.currentWatcherExecutionTraceContext = runloop.WatcherExecutionTraceContext{}
}

func (server *runloopTestServer) CurrentWatcherExecutionTraceContextSnapshot() runloop.WatcherExecutionTraceContext {
	if server == nil {
		return runloop.WatcherExecutionTraceContext{}
	}
	server.Mu.Lock()
	defer server.Mu.Unlock()
	return server.currentWatcherExecutionTraceContext
}

func (server *runloopTestServer) RunNoWaitHookWithConcurrencyLimit(
	runNoWaitHook func(),
) {
	if server == nil {
		return
	}
	runNoWaitHookWithConcurrencyLimitForRunloopTests(
		server.ConcurrentNoWaitHookExecutionLimiter,
		runNoWaitHook,
	)
}

func (server *runloopTestServer) GetOrCreateConcurrentNoWaitHookLifecycleContext() context.Context {
	if server == nil {
		return context.Background()
	}
	server.concurrentNoWaitHookContextMutex.Lock()
	defer server.concurrentNoWaitHookContextMutex.Unlock()
	if server.concurrentNoWaitHookLifecycleContext != nil {
		return server.concurrentNoWaitHookLifecycleContext
	}
	lifecycleContext, cancelLifecycleContext := context.WithCancel(
		server.CurrentRunCycleContextOrBackground(),
	)
	server.concurrentNoWaitHookLifecycleContext = lifecycleContext
	server.concurrentNoWaitHookLifecycleCancel = cancelLifecycleContext
	return lifecycleContext
}

func (server *runloopTestServer) cancelConcurrentNoWaitHookLifecycleContext() {
	if server == nil {
		return
	}
	server.concurrentNoWaitHookContextMutex.Lock()
	cancelLifecycleContext := server.concurrentNoWaitHookLifecycleCancel
	server.concurrentNoWaitHookLifecycleContext = nil
	server.concurrentNoWaitHookLifecycleCancel = nil
	server.concurrentNoWaitHookContextMutex.Unlock()
	if cancelLifecycleContext != nil {
		cancelLifecycleContext()
	}
}

func (server *runloopTestServer) ResolveHookCommand(
	hook wavewatch.OnChangeHook,
) string {
	if server == nil {
		return hook.Cmd
	}
	return resolveHookCommandForRunloopTests(server.Cfg, hook)
}

func (server *runloopTestServer) ResolveHookExecutionPlan(
	hook wavewatch.OnChangeHook,
) hooks.HookExecutionPlan {
	if server == nil {
		return hooks.HookExecutionPlan{}
	}
	return resolveHookExecutionPlanForRunloopTests(server.Cfg, hook)
}

func (server *runloopTestServer) ExecuteBuildPhase(
	work *eventpipeline.WorkSet,
) error {
	if work == nil {
		return nil
	}

	executionDecision := eventpipeline.DeriveBuildPhaseExecutionDecision(
		work.Build,
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
			if publicProcessingError := eventpipeline.ExecuteStaticFileProcessingForBuildPhase(
				builderInstance.ProcessPublicFilesOnly,
				builderInstance.ProcessPublicFilesOnlyForChangedPaths,
				executionDecision.PublicStaticProcessing,
			); publicProcessingError != nil {
				return publicProcessingError
			}
			if privateProcessingError := eventpipeline.ExecuteStaticFileProcessingForBuildPhase(
				builderInstance.ProcessPrivateFilesOnly,
				builderInstance.ProcessPrivateFilesOnlyForChangedPaths,
				executionDecision.PrivateStaticProcessing,
			); privateProcessingError != nil {
				return privateProcessingError
			}
			return nil
		})
	}

	return buildGroup.Wait()
}

func (server *runloopTestServer) BuildRunloopEngine() *runloop.Engine {
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
		ExecuteBrowserPhase:                func(*eventpipeline.WorkSet) {},
		StartApp:                           server.StartApp,
		StopApp:                            server.StopApp,
		TriggerRestart:                     server.TriggerRestart,
		TriggerRestartNoGo:                 server.TriggerRestartNoGo,
		TriggerConfigRestart:               server.TriggerConfigRestart,
		BroadcastRebuilding:                server.BroadcastRebuilding,
		BuildEventExecutionPlan:            server.BuildEventExecutionPlan,
		DeriveWatcherExecutionTraceContext: server.DeriveWatcherExecutionTraceContext,
		SetCurrentWatcherExecutionTraceContext: func(
			traceContext runloop.WatcherExecutionTraceContext,
		) {
			server.SetCurrentWatcherExecutionTraceContext(traceContext)
		},
		ClearCurrentWatcherExecutionTraceContext: server.ClearCurrentWatcherExecutionTraceContext,
		GetCurrentWatcherExecutionTraceContext: func() runloop.WatcherExecutionTraceContext {
			return server.CurrentWatcherExecutionTraceContextSnapshot()
		},
		RunNoWaitHookWithConcurrencyLimit:               server.RunNoWaitHookWithConcurrencyLimit,
		GetOrCreateConcurrentNoWaitHookLifecycleContext: server.GetOrCreateConcurrentNoWaitHookLifecycleContext,
		ResolveHookExecutionPlan:                        server.ResolveHookExecutionPlan,
	})
}

func (server *runloopTestServer) CleanupForRebuild() {
	if server == nil {
		return
	}
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
}
