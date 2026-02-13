package tooling

import (
	"strings"

	"github.com/vormadev/vorma/wave"
)

type hookStageExecutionDescriptor struct {
	eventIndex     int
	eventWithHooks eventWithHooks
}

type hookStageType int

const (
	hookStageTypePre hookStageType = iota
	hookStageTypeConcurrent
	hookStageTypePost
	hookStageTypeConcurrentNoWait
)

type hookExecutionPlan struct {
	callback func(*wave.HookContext) (*wave.RefreshAction, error)
	command  string
}

func deriveHookStageExecutionDescriptors(
	eventsWithHooks []eventWithHooks,
) []hookStageExecutionDescriptor {
	if len(eventsWithHooks) == 0 {
		return nil
	}

	descriptors := make([]hookStageExecutionDescriptor, 0, len(eventsWithHooks))
	for eventIndex, eventWithHooksForExecution := range eventsWithHooks {
		if eventWithHooksForExecution.skipDuplicateHooks {
			continue
		}
		descriptors = append(descriptors, hookStageExecutionDescriptor{
			eventIndex:     eventIndex,
			eventWithHooks: eventWithHooksForExecution,
		})
	}
	return descriptors
}

func deriveStageHooksAndRunOnChangePolicyForEvent(
	eventWithHooksForStage eventWithHooks,
	stageType hookStageType,
) ([]wave.OnChangeHook, bool) {
	if eventWithHooksForStage.hooks == nil {
		return nil, false
	}

	switch stageType {
	case hookStageTypePre:
		return eventWithHooksForStage.hooks.Pre, false
	case hookStageTypeConcurrent:
		return eventWithHooksForStage.hooks.Concurrent, true
	case hookStageTypePost:
		return eventWithHooksForStage.hooks.Post, true
	case hookStageTypeConcurrentNoWait:
		return eventWithHooksForStage.hooks.ConcurrentNoWait, false
	default:
		return nil, false
	}
}

func deriveHookExecutionPlanFromHook(
	hook wave.OnChangeHook,
	resolveHookCommand func(wave.OnChangeHook) string,
) hookExecutionPlan {
	resolvedCommand := hook.Cmd
	if resolveHookCommand != nil {
		resolvedCommand = resolveHookCommand(hook)
	}

	return hookExecutionPlan{
		callback: hook.Callback,
		command:  resolvedCommand,
	}
}

func deriveHookExecutionPlansForEventStage(
	watcher *Watcher,
	eventWithHooksForStage eventWithHooks,
	stageType hookStageType,
	resolveHookCommand func(wave.OnChangeHook) string,
) []hookExecutionPlan {
	stageHooks, shouldApplyRunOnChangeOnlyRules := deriveStageHooksAndRunOnChangePolicyForEvent(
		eventWithHooksForStage,
		stageType,
	)
	executableHooksForStage := deriveExecutableHooksForStage(
		watcher,
		eventWithHooksForStage.classified.event.Name,
		eventWithHooksForStage.runOnChangeOnly,
		shouldApplyRunOnChangeOnlyRules,
		stageHooks,
	)
	if len(executableHooksForStage) == 0 {
		return nil
	}

	plans := make([]hookExecutionPlan, 0, len(executableHooksForStage))
	for _, executableHook := range executableHooksForStage {
		plans = append(
			plans,
			deriveHookExecutionPlanFromHook(executableHook, resolveHookCommand),
		)
	}

	return plans
}

func deriveExecutableHooksForStage(
	watcher *Watcher,
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
		hookForExecution, shouldRunHook := resolveHookForStageExecution(
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

func resolveHookForStageExecution(
	watcher *Watcher,
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
