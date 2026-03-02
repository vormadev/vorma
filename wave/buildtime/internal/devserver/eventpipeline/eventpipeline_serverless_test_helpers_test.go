package eventpipeline_test

import (
	"context"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/wavewatch"
	"log/slog"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave/buildtime/builder"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/eventpipeline"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/hooks"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/runloop"
	"github.com/vormadev/vorma/wave/buildtime/internal/watch"
	"github.com/vormadev/vorma/wave/buildtime/internal/watch/classification"
	"github.com/vormadev/vorma/wave/buildtime/internal/watch/dedup"
	"github.com/vormadev/vorma/wave/internal/wavelock"
)

func isConfigFileForEventPipelineTests(
	cfg *waveconfig.ParsedConfig,
	path string,
) bool {
	if cfg == nil || cfg.Core == nil {
		return false
	}
	return classification.IsConfigurationPathChange(
		path,
		cfg.Core.ConfigLocation,
	)
}

func classifyEventWithWatcherAndBuilderForEventPipelineTests(
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

func classifyWatcherEventsForProcessingForEventPipelineTests(
	cfg *waveconfig.ParsedConfig,
	watcherEvents []fsnotify.Event,
	watcherForClassification *watch.Watcher,
	builderForClassification *builder.Builder,
) ([]eventpipeline.ClassifiedEvent, bool) {
	classifiedEvents, configChanged := classifyWatcherEventsFromPreClassificationPlanForEventPipelineTests(
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

func classifyWatcherEventsFromPreClassificationPlanForEventPipelineTests(
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
		if isConfigFileForEventPipelineTests(cfg, watcherEvent.Name) {
			return nil, true
		}
		classifiedEvents = append(
			classifiedEvents,
			classifyEventWithWatcherAndBuilderForEventPipelineTests(
				watcherEvent,
				watcherForClassification,
				builderForClassification,
			),
		)
	}
	return classifiedEvents, false
}

func buildEventExecutionPlanForEventPipelineTests(
	cfg *waveconfig.ParsedConfig,
	watcherEvents []fsnotify.Event,
	watcherForPlan *watch.Watcher,
	builderForPlan *builder.Builder,
) eventpipeline.EventExecutionPlanningResult {
	classifiedEvents, configChanged := classifyWatcherEventsForProcessingForEventPipelineTests(
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

func buildRunloopEngineForEventPipelineTests(
	cfg *waveconfig.ParsedConfig,
	log *slog.Logger,
	watcherForRuntime *watch.Watcher,
	builderForRuntime *builder.Builder,
) *runloop.Engine {
	return runloop.New(runloop.Dependencies{
		Log:    log,
		Config: cfg,
		GetCurrentWatcher: func() *watch.Watcher {
			return watcherForRuntime
		},
		GetCurrentBuilder: func() *builder.Builder {
			return builderForRuntime
		},
		CurrentRunCycleContextOrBackground: context.Background,
		ExecuteBuildPhase: func(*eventpipeline.WorkSet) error {
			return nil
		},
		ExecuteBrowserPhase: func(*eventpipeline.WorkSet) {},
		StartApp:            func() {},
		StopApp: func() error {
			return nil
		},
		TriggerRestart:       func() {},
		TriggerRestartNoGo:   func() {},
		TriggerConfigRestart: func() {},
		BroadcastRebuilding:  func() {},
		BuildEventExecutionPlan: func(
			events []fsnotify.Event,
			watcherForPlan *watch.Watcher,
			builderForPlan *builder.Builder,
		) eventpipeline.EventExecutionPlanningResult {
			return buildEventExecutionPlanForEventPipelineTests(
				cfg,
				events,
				watcherForPlan,
				builderForPlan,
			)
		},
		DeriveWatcherExecutionTraceContext: func() runloop.WatcherExecutionTraceContext {
			return runloop.WatcherExecutionTraceContext{}
		},
		SetCurrentWatcherExecutionTraceContext:   func(runloop.WatcherExecutionTraceContext) {},
		ClearCurrentWatcherExecutionTraceContext: func() {},
		GetCurrentWatcherExecutionTraceContext: func() runloop.WatcherExecutionTraceContext {
			return runloop.WatcherExecutionTraceContext{}
		},
		RunNoWaitHookWithConcurrencyLimit: func(runNoWaitHook func()) {
			if runNoWaitHook == nil {
				return
			}
			go runNoWaitHook()
		},
		GetOrCreateConcurrentNoWaitHookLifecycleContext: context.Background,
		ResolveHookExecutionPlan: func(
			hook wavewatch.OnChangeHook,
		) hooks.HookExecutionPlan {
			return hooks.DeriveHookExecutionPlanFromHook(hook, nil)
		},
	})
}
