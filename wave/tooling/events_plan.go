package tooling

import (
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/pathnorm"
)

// eventWithHooks pairs a classified event with its sorted hooks.
type eventWithHooks struct {
	classified         classifiedEvent
	hooks              *wave.SortedHooks
	hookCtx            *wave.HookContext
	runOnChangeOnly    bool
	needsHardReload    bool
	skipDuplicateHooks bool
}

func deriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
	eventsWithHooks []eventWithHooks,
) eventExecutionPlanBehavioralDecision {
	return eventExecutionPlanBehavioralDecision{
		showRebuildingOverlay: shouldShowRebuildingOverlayForEventsWithHooks(eventsWithHooks),
		appStopStrategy:       resolveAppStopStrategy(eventsWithHooks),
		runImplicitBuild:      shouldRunImplicitBuildForEvents(eventsWithHooks),
	}
}

func shouldShowRebuildingOverlayForEventsWithHooks(
	eventsWithHooks []eventWithHooks,
) bool {
	if len(eventsWithHooks) == 0 {
		return false
	}

	classifiedEvents := make([]classifiedEvent, 0, len(eventsWithHooks))
	for _, eventWithHooksForOverlay := range eventsWithHooks {
		classifiedEvents = append(classifiedEvents, eventWithHooksForOverlay.classified)
	}
	return shouldShowRebuildingOverlay(classifiedEvents)
}

func buildWatcherEventLogPayloadsForEventsWithHooks(
	eventsWithHooks []eventWithHooks,
) []watcherEventLogPayload {
	if len(eventsWithHooks) == 0 {
		return nil
	}

	watcherEventLogPayloads := make(
		[]watcherEventLogPayload,
		0,
		len(eventsWithHooks),
	)
	for _, eventWithHooksForLogging := range eventsWithHooks {
		watcherEventLogPayloads = append(
			watcherEventLogPayloads,
			watcherEventLogPayload{
				operation: eventWithHooksForLogging.classified.event.Op.String(),
				filePath:  eventWithHooksForLogging.classified.event.Name,
			},
		)
	}
	return watcherEventLogPayloads
}

func buildEventHooksForProcessing(
	classifiedEvents []classifiedEvent,
) []eventWithHooks {
	if len(classifiedEvents) == 0 {
		return nil
	}

	normalizedChangedFilePathsByWatchedPattern := buildNormalizedChangedFilePathsByWatchedPatternForHookContexts(
		classifiedEvents,
	)
	watchedPatternOccurrenceCount := buildWatchedPatternOccurrenceCountForHookContexts(
		classifiedEvents,
	)
	skipDuplicateHooksByClassifiedEventIndex := buildSkipDuplicateHooksByClassifiedEventIndex(
		classifiedEvents,
	)

	eventsWithHooks := make([]eventWithHooks, 0, len(classifiedEvents))
	for eventIndex, classifiedEventForProcessing := range classifiedEvents {
		normalizedEventPathForHookContext := normalizeHookContextPathShape(
			classifiedEventForProcessing.event.Name,
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

func buildNormalizedChangedFilePathsByWatchedPatternForHookContexts(
	classifiedEvents []classifiedEvent,
) map[string][]string {
	normalizedChangedFilePathsByWatchedPattern := make(map[string][]string)
	seenNormalizedChangedFilePathsByWatchedPattern := make(map[string]map[string]struct{})

	for _, classifiedEventForProcessing := range classifiedEvents {
		watchedFileForProcessing := classifiedEventForProcessing.watchedFile
		if watchedFileForProcessing == nil {
			continue
		}
		normalizedEventPathForHookContext := normalizeHookContextPathShape(
			classifiedEventForProcessing.event.Name,
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

func buildSkipDuplicateHooksByClassifiedEventIndex(
	classifiedEvents []classifiedEvent,
) []bool {
	skipDuplicateHooksByClassifiedEventIndex := make([]bool, len(classifiedEvents))
	handledWatchedPatterns := make(map[string]struct{})

	for eventIndex, classifiedEventForProcessing := range classifiedEvents {
		watchedFileForProcessing := classifiedEventForProcessing.watchedFile
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
	classifiedEvents []classifiedEvent,
) map[string]int {
	watchedPatternOccurrenceCount := make(map[string]int)
	for _, classifiedEventForProcessing := range classifiedEvents {
		watchedFileForProcessing := classifiedEventForProcessing.watchedFile
		if watchedFileForProcessing == nil {
			continue
		}
		watchedPatternOccurrenceCount[watchedFileForProcessing.Pattern]++
	}
	return watchedPatternOccurrenceCount
}

func deriveChangedFilePathsForHookContext(
	classifiedEventForProcessing classifiedEvent,
	normalizedChangedFilePathsByWatchedPattern map[string][]string,
	watchedPatternOccurrenceCount map[string]int,
	normalizedEventPathForHookContext string,
) []string {
	watchedFileForProcessing := classifiedEventForProcessing.watchedFile
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
	classifiedEventForProcessing classifiedEvent,
	skipDuplicateHooks bool,
	changedFilePathsForHookContext []string,
) eventWithHooks {
	watchedFileForProcessing := classifiedEventForProcessing.watchedFile
	if watchedFileForProcessing == nil {
		watchedFileForProcessing = &wave.WatchedFile{}
	}
	if watchedFileForProcessing.SortedHooks == nil {
		watchedFileForProcessing.Sort()
	}
	if watchedFileForProcessing.SortedHooks == nil {
		watchedFileForProcessing.SortedHooks = &wave.SortedHooks{}
	}

	normalizedEventPathForHookContext := normalizeHookContextPathShape(
		classifiedEventForProcessing.event.Name,
	)
	if len(changedFilePathsForHookContext) == 0 {
		changedFilePathsForHookContext = []string{normalizedEventPathForHookContext}
	}
	eventNeedsHardReload := classifiedEventForProcessing.fileType == fileTypeGo ||
		needsHardReload(watchedFileForProcessing)

	return eventWithHooks{
		classified:         classifiedEventForProcessing,
		hooks:              watchedFileForProcessing.SortedHooks,
		runOnChangeOnly:    watchedFileForProcessing.RunOnChangeOnly,
		needsHardReload:    eventNeedsHardReload,
		skipDuplicateHooks: skipDuplicateHooks,
		hookCtx: &wave.HookContext{
			FilePath:           normalizedEventPathForHookContext,
			ChangedFilePaths:   changedFilePathsForHookContext,
			AppStoppedForBatch: false,
		},
	}
}

func normalizeHookContextPathShape(path string) string {
	return pathnorm.TrimAndCleanPath(path)
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

func shouldShowRebuildingOverlay(
	classifiedEvents []classifiedEvent,
) bool {
	for _, classifiedEventForOverlay := range classifiedEvents {
		if classifiedEventForOverlay.fileType == fileTypeCriticalCSS ||
			classifiedEventForOverlay.fileType == fileTypeNormalCSS ||
			classifiedEventForOverlay.fileType == fileTypeCriticalAndNormalCSS {
			continue
		}

		if classifiedEventForOverlay.watchedFile == nil ||
			!classifiedEventForOverlay.watchedFile.SkipRebuildingNotification {
			return true
		}
	}

	return false
}

func anyEventNeedsHardReload(eventsWithHooks []eventWithHooks) bool {
	for _, eventWithHooksForCheck := range eventsWithHooks {
		if eventWithHooksForCheck.needsHardReload {
			return true
		}
	}
	return false
}

func resolveAppStopStrategy(eventsWithHooks []eventWithHooks) appStopStrategy {
	if len(eventsWithHooks) == 0 {
		return appStopStrategyNone
	}

	if len(eventsWithHooks) == 1 {
		if eventsWithHooks[0].needsHardReload {
			return appStopStrategySingleEventHardReload
		}
		return appStopStrategyNone
	}

	if anyEventNeedsHardReload(eventsWithHooks) {
		return appStopStrategyBatchHardReload
	}

	return appStopStrategyNone
}

func shouldRunImplicitBuildForEvents(eventsWithHooks []eventWithHooks) bool {
	for _, eventWithHooksForCheck := range eventsWithHooks {
		if !eventWithHooksForCheck.runOnChangeOnly {
			return true
		}
	}
	return false
}
