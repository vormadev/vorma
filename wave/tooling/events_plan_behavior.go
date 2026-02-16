package tooling

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

func shouldShowRebuildingOverlay(
	classifiedEvents []classifiedEvent,
) bool {
	for _, classifiedEventForOverlay := range classifiedEvents {
		if classifiedEventForOverlay.fileType == fileTypeCriticalCSS ||
			classifiedEventForOverlay.fileType == fileTypeNormalCSS ||
			classifiedEventForOverlay.fileType == fileTypeCriticalAndNormalCSS {
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
	classifiedEventForOverlay classifiedEvent,
) bool {
	watchedFileForOverlay := classifiedEventForOverlay.watchedFile
	if watchedFileForOverlay == nil {
		return false
	}

	if watchedFileForOverlay.SkipRebuildingNotification {
		return true
	}

	return watchedFileForOverlay.OnlyRunClientDefinedRevalidateFunc &&
		classifiedEventForOverlay.fileType != fileTypeGo &&
		!needsHardReload(watchedFileForOverlay)
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
