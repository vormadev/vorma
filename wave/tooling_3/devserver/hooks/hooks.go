package hooks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling_3/devserver/eventpipeline"
	"github.com/vormadev/vorma/wave/tooling_3/watch"
)

const (
	// MaxConcurrentNoWaitHookExecutions bounds background hook concurrency.
	MaxConcurrentNoWaitHookExecutions = 8
)

// HookStageType identifies one hook execution stage.
type HookStageType int

const (
	// HookStageTypePre executes before build work starts.
	HookStageTypePre HookStageType = iota
	// HookStageTypeConcurrent executes in parallel with build stage.
	HookStageTypeConcurrent
	// HookStageTypeConcurrentNoWait executes detached from cycle completion.
	HookStageTypeConcurrentNoWait
	// HookStageTypePost executes after build and concurrent stages complete.
	HookStageTypePost
)

// HookExecutionPlan is one executable hook plan after policy derivation.
type HookExecutionPlan struct {
	Callback func(*wave.HookContext) (*wave.RefreshAction, error)
	Command  string

	CommandTimeoutMilliseconds  int
	DisableStageCommandTimeout  bool
	CallbackTimeoutMilliseconds int
	DisableStageCallbackTimeout bool
}

// HookStageResult captures actions and errors produced by one stage.
type HookStageResult struct {
	Actions             []wave.RefreshAction
	RefreshActionResult eventpipeline.RefreshActionApplicationResult
	StageType           HookStageType
	ExecutionErrors     []error
}

// HookStageFailurePolicy controls pipeline behavior after hook errors.
type HookStageFailurePolicy int

const (
	// HookStageFailurePolicyContinue keeps pipeline moving despite stage errors.
	HookStageFailurePolicyContinue HookStageFailurePolicy = iota
	// HookStageFailurePolicyStop stops pipeline after stage errors.
	HookStageFailurePolicyStop
)

// HookStageContinuationStopReason explains why stage continuation halted.
type HookStageContinuationStopReason int

const (
	// HookStageContinuationStopReasonNone means pipeline should continue.
	HookStageContinuationStopReasonNone HookStageContinuationStopReason = iota
	// HookStageContinuationStopReasonRestartRequested means hook action requested restart.
	HookStageContinuationStopReasonRestartRequested
	// HookStageContinuationStopReasonStageFailure means hook stage failure policy stopped execution.
	HookStageContinuationStopReasonStageFailure
)

// HookStageContinuationDecision resolves whether pipeline should continue.
type HookStageContinuationDecision struct {
	ShouldContinue      bool
	StopReason          HookStageContinuationStopReason
	RestartActionResult eventpipeline.RefreshActionApplicationResult
}

// ImplicitBuildExecutionDecision resolves whether implicit build should execute.
type ImplicitBuildExecutionDecision struct {
	ShouldRunImplicitBuild    bool
	SkipImplicitBuildLogEntry string
}

// HookStageExecutionDescriptor binds deterministic event order to stage execution.
type HookStageExecutionDescriptor struct {
	DescriptorIndex int
	EventWithHooks  eventpipeline.EventWithHooks
}

// ResolveSequentialShellCommands joins non-empty commands into one shell expression.
func ResolveSequentialShellCommands(commands ...string) string {
	resolvedCommands := make([]string, 0, len(commands))
	for _, command := range commands {
		trimmedCommand := strings.TrimSpace(command)
		if trimmedCommand == "" {
			continue
		}
		resolvedCommands = append(resolvedCommands, trimmedCommand)
	}
	if len(resolvedCommands) == 0 {
		return ""
	}
	return strings.Join(resolvedCommands, " && ")
}

// DeriveHookStageExecutionDescriptors returns deterministic execution descriptors.
func DeriveHookStageExecutionDescriptors(
	eventsWithHooks []eventpipeline.EventWithHooks,
) []HookStageExecutionDescriptor {
	descriptors := make([]HookStageExecutionDescriptor, 0, len(eventsWithHooks))
	for eventIndex := range eventsWithHooks {
		descriptors = append(descriptors, HookStageExecutionDescriptor{
			DescriptorIndex: eventIndex,
			EventWithHooks:  eventsWithHooks[eventIndex],
		})
	}
	return descriptors
}

// DeriveStageHooksAndRunOnChangePolicyForEvent resolves stage hooks for one event.
func DeriveStageHooksAndRunOnChangePolicyForEvent(
	eventWithHooks eventpipeline.EventWithHooks,
	stageType HookStageType,
) ([]wave.OnChangeHook, bool) {
	sortedHooks := eventWithHooks.Hooks
	if sortedHooks == nil {
		return nil, eventWithHooks.RunOnChangeOnly
	}

	switch stageType {
	case HookStageTypePre:
		return sortedHooks.Pre, eventWithHooks.RunOnChangeOnly
	case HookStageTypeConcurrent:
		return sortedHooks.Concurrent, eventWithHooks.RunOnChangeOnly
	case HookStageTypeConcurrentNoWait:
		return sortedHooks.ConcurrentNoWait, eventWithHooks.RunOnChangeOnly
	case HookStageTypePost:
		return sortedHooks.Post, eventWithHooks.RunOnChangeOnly
	default:
		return nil, eventWithHooks.RunOnChangeOnly
	}
}

// DeriveHookExecutionPlanFromHook converts hook configuration into executable plan.
func DeriveHookExecutionPlanFromHook(
	hook wave.OnChangeHook,
	commandResolver func(wave.OnChangeHook) string,
) HookExecutionPlan {
	resolvedCommand := hook.Cmd
	if commandResolver != nil {
		resolvedCommand = commandResolver(hook)
	}
	return HookExecutionPlan{
		Callback:                    hook.Callback,
		Command:                     strings.TrimSpace(resolvedCommand),
		CommandTimeoutMilliseconds:  hook.CommandTimeoutMilliseconds,
		DisableStageCommandTimeout:  hook.DisableStageCommandTimeout,
		CallbackTimeoutMilliseconds: hook.CallbackTimeoutMilliseconds,
		DisableStageCallbackTimeout: hook.DisableStageCallbackTimeout,
	}
}

// DeriveHookExecutionPlansForEventStage builds executable plans for one stage.
func DeriveHookExecutionPlansForEventStage(
	watcher *watch.Watcher,
	eventWithHooks eventpipeline.EventWithHooks,
	stageType HookStageType,
	planResolver func(wave.OnChangeHook) HookExecutionPlan,
) []HookExecutionPlan {
	resolvedHooks, _ := DeriveStageHooksAndRunOnChangePolicyForEvent(
		eventWithHooks,
		stageType,
	)
	if len(resolvedHooks) == 0 {
		return nil
	}

	plans := make([]HookExecutionPlan, 0, len(resolvedHooks))
	for _, hookForStage := range resolvedHooks {
		if !ResolveHookForStageExecution(
			watcher,
			eventWithHooks.Classified,
			hookForStage,
		) {
			continue
		}
		if planResolver == nil {
			plans = append(
				plans,
				DeriveHookExecutionPlanFromHook(hookForStage, nil),
			)
			continue
		}
		plans = append(plans, planResolver(hookForStage))
	}
	return plans
}

// DeriveExecutableHooksForStage filters hooks for current path and stage semantics.
func DeriveExecutableHooksForStage(
	watcher *watch.Watcher,
	eventWithHooks eventpipeline.EventWithHooks,
	stageType HookStageType,
) []wave.OnChangeHook {
	hooksForStage, _ := DeriveStageHooksAndRunOnChangePolicyForEvent(
		eventWithHooks,
		stageType,
	)
	if len(hooksForStage) == 0 {
		return nil
	}

	executableHooks := make([]wave.OnChangeHook, 0, len(hooksForStage))
	for _, hookForStage := range hooksForStage {
		if ResolveHookForStageExecution(
			watcher,
			eventWithHooks.Classified,
			hookForStage,
		) {
			executableHooks = append(executableHooks, hookForStage)
		}
	}
	return executableHooks
}

// ResolveHookForStageExecution returns true when hook should run for event path.
func ResolveHookForStageExecution(
	watcher *watch.Watcher,
	classifiedEvent eventpipeline.ClassifiedEvent,
	hook wave.OnChangeHook,
) bool {
	if watcher == nil {
		return true
	}
	if len(hook.Exclude) == 0 {
		return true
	}
	changedPath := NormalizeHookContextPathShape(classifiedEvent.Event.Name)
	for _, excludedPattern := range hook.Exclude {
		normalizedPattern := NormalizeHookContextPathShape(excludedPattern)
		if normalizedPattern == "" {
			continue
		}
		if strings.Contains(normalizedPattern, "*") {
			if watcher.IsIgnoredFile(changedPath) {
				continue
			}
		}
		matched := strings.HasSuffix(changedPath, normalizedPattern)
		if matched {
			return false
		}
	}
	return true
}

// BuildEventHooksForProcessing builds stage-partitioned hook entries per event.
func BuildEventHooksForProcessing(
	classifiedEvents []eventpipeline.ClassifiedEvent,
) []eventpipeline.EventWithHooks {
	if len(classifiedEvents) == 0 {
		return nil
	}

	changedPathsByPattern := BuildNormalizedChangedFilePathsByWatchedPatternForHookContexts(
		classifiedEvents,
	)
	skipDuplicateByIndex := BuildSkipDuplicateHooksByClassifiedEventIndex(
		classifiedEvents,
	)

	eventsWithHooks := make(
		[]eventpipeline.EventWithHooks,
		0,
		len(classifiedEvents),
	)
	for eventIndex, classifiedEvent := range classifiedEvents {
		if classifiedEvent.WatchedFile == nil ||
			classifiedEvent.WatchedFile.SortedHooks == nil {
			continue
		}

		normalizedEventPath := NormalizeHookContextPathShape(
			classifiedEvent.Event.Name,
		)
		changedPathsForPattern := changedPathsByPattern[NormalizeHookContextPathShape(classifiedEvent.WatchedFile.Pattern)]
		if len(changedPathsForPattern) == 0 {
			changedPathsForPattern = []string{normalizedEventPath}
		}

		hookContext := &wave.HookContext{
			ExecutionContext: nil,
			FilePath:         normalizedEventPath,
			ChangedFilePaths: append(
				[]string(nil),
				changedPathsForPattern...),
			AppStoppedForBatch: false,
		}

		eventsWithHooks = append(eventsWithHooks, eventpipeline.EventWithHooks{
			Classified:      classifiedEvent,
			Hooks:           classifiedEvent.WatchedFile.SortedHooks,
			HookCtx:         hookContext,
			RunOnChangeOnly: classifiedEvent.WatchedFile.RunOnChangeOnly,
			NeedsHardReload: eventpipeline.NeedsHardReload(
				classifiedEvent.WatchedFile,
			),
			SkipDuplicateHooks: skipDuplicateByIndex[eventIndex],
		})
	}

	return eventsWithHooks
}

// BuildNormalizedChangedFilePathsByWatchedPatternForHookContexts batches changed paths by pattern.
func BuildNormalizedChangedFilePathsByWatchedPatternForHookContexts(
	classifiedEvents []eventpipeline.ClassifiedEvent,
) map[string][]string {
	changedPathsByPattern := make(map[string][]string)
	for _, classifiedEvent := range classifiedEvents {
		if classifiedEvent.WatchedFile == nil {
			continue
		}
		normalizedPattern := NormalizeHookContextPathShape(
			classifiedEvent.WatchedFile.Pattern,
		)
		normalizedPath := NormalizeHookContextPathShape(
			classifiedEvent.Event.Name,
		)
		if normalizedPattern == "" || normalizedPath == "" {
			continue
		}
		changedPathsByPattern[normalizedPattern] = append(
			changedPathsByPattern[normalizedPattern],
			normalizedPath,
		)
	}
	for pattern := range changedPathsByPattern {
		changedPathsByPattern[pattern] = dedupeStablePaths(
			changedPathsByPattern[pattern],
		)
	}
	return changedPathsByPattern
}

// BuildSkipDuplicateHooksByClassifiedEventIndex returns per-event duplicate hook policy.
func BuildSkipDuplicateHooksByClassifiedEventIndex(
	classifiedEvents []eventpipeline.ClassifiedEvent,
) map[int]bool {
	skipByIndex := make(map[int]bool)
	firstSeenPatternIndex := make(map[string]int)

	for eventIndex, classifiedEvent := range classifiedEvents {
		if classifiedEvent.WatchedFile == nil {
			continue
		}
		normalizedPattern := NormalizeHookContextPathShape(
			classifiedEvent.WatchedFile.Pattern,
		)
		if normalizedPattern == "" {
			continue
		}
		if _, alreadySeen := firstSeenPatternIndex[normalizedPattern]; !alreadySeen {
			firstSeenPatternIndex[normalizedPattern] = eventIndex
			continue
		}
		skipByIndex[eventIndex] = true
	}

	return skipByIndex
}

// NormalizeHookContextPathShape normalizes path formatting for hook contexts.
func NormalizeHookContextPathShape(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	normalizedPath := filepath.Clean(path)
	return strings.ReplaceAll(normalizedPath, "\\", "/")
}

// DeriveHookCommandStageTimeoutMilliseconds resolves stage command timeout override.
func DeriveHookCommandStageTimeoutMilliseconds(
	watchConfig *wave.WatchConfig,
	stageType HookStageType,
) int {
	if watchConfig == nil {
		return 0
	}
	switch stageType {
	case HookStageTypePre:
		return watchConfig.HookCommandTimeouts.PreCommandTimeoutMilliseconds
	case HookStageTypeConcurrent:
		return watchConfig.HookCommandTimeouts.ConcurrentCommandTimeoutMilliseconds
	case HookStageTypeConcurrentNoWait:
		return watchConfig.HookCommandTimeouts.ConcurrentNoWaitCommandTimeoutMilliseconds
	case HookStageTypePost:
		return watchConfig.HookCommandTimeouts.PostCommandTimeoutMilliseconds
	default:
		return 0
	}
}

// DeriveHookCallbackStageTimeoutMilliseconds resolves stage callback timeout override.
func DeriveHookCallbackStageTimeoutMilliseconds(
	watchConfig *wave.WatchConfig,
	stageType HookStageType,
) int {
	if watchConfig == nil {
		return 0
	}
	switch stageType {
	case HookStageTypePre:
		return watchConfig.HookCallbackTimeouts.PreCallbackTimeoutMilliseconds
	case HookStageTypeConcurrent:
		return watchConfig.HookCallbackTimeouts.ConcurrentCallbackTimeoutMilliseconds
	case HookStageTypeConcurrentNoWait:
		return watchConfig.HookCallbackTimeouts.ConcurrentNoWaitCallbackTimeoutMilliseconds
	case HookStageTypePost:
		return watchConfig.HookCallbackTimeouts.PostCallbackTimeoutMilliseconds
	default:
		return 0
	}
}

// DeriveResolvedTimeoutDurationFromStageAndExecutionPolicy resolves timeout duration.
func DeriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
	overrideMilliseconds int,
	stageTimeoutMilliseconds int,
	disableStageTimeout bool,
) time.Duration {
	if overrideMilliseconds > 0 {
		return time.Duration(overrideMilliseconds) * time.Millisecond
	}
	if disableStageTimeout {
		return 0
	}
	if stageTimeoutMilliseconds <= 0 {
		return 0
	}
	return time.Duration(stageTimeoutMilliseconds) * time.Millisecond
}

// DeriveHookCommandTimeoutDurationForExecutionPlan resolves command timeout for plan.
func DeriveHookCommandTimeoutDurationForExecutionPlan(
	watchConfig *wave.WatchConfig,
	stageType HookStageType,
	executionPlan HookExecutionPlan,
) time.Duration {
	return DeriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
		executionPlan.CommandTimeoutMilliseconds,
		DeriveHookCommandStageTimeoutMilliseconds(watchConfig, stageType),
		executionPlan.DisableStageCommandTimeout,
	)
}

// DeriveHookCallbackTimeoutDurationForExecutionPlan resolves callback timeout for plan.
func DeriveHookCallbackTimeoutDurationForExecutionPlan(
	watchConfig *wave.WatchConfig,
	stageType HookStageType,
	executionPlan HookExecutionPlan,
) time.Duration {
	return DeriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
		executionPlan.CallbackTimeoutMilliseconds,
		DeriveHookCallbackStageTimeoutMilliseconds(watchConfig, stageType),
		executionPlan.DisableStageCallbackTimeout,
	)
}

// DeriveBuildHookCommandTimeoutDuration resolves build-hook command timeout.
func DeriveBuildHookCommandTimeoutDuration(
	coreConfig *wave.CoreConfig,
	isDev bool,
) time.Duration {
	if coreConfig == nil {
		return 0
	}
	if isDev {
		if coreConfig.DevBuildHookTimeoutMilliseconds <= 0 {
			return 0
		}
		return time.Duration(
			coreConfig.DevBuildHookTimeoutMilliseconds,
		) * time.Millisecond
	}
	if coreConfig.ProdBuildHookTimeoutMilliseconds <= 0 {
		return 0
	}
	return time.Duration(
		coreConfig.ProdBuildHookTimeoutMilliseconds,
	) * time.Millisecond
}

// DeriveExecutionContextWithOptionalTimeout derives child context only when timeout is set.
func DeriveExecutionContextWithOptionalTimeout(
	parentExecutionContext context.Context,
	timeoutDuration time.Duration,
) (context.Context, context.CancelFunc) {
	if parentExecutionContext == nil {
		parentExecutionContext = context.Background()
	}
	if timeoutDuration <= 0 {
		return parentExecutionContext, nil
	}
	return context.WithTimeout(parentExecutionContext, timeoutDuration)
}

// ApplyHookStageActionsToWorkSet mutates workset based on hook refresh actions.
func ApplyHookStageActionsToWorkSet(
	work *eventpipeline.WorkSet,
	actions []wave.RefreshAction,
) eventpipeline.RefreshActionApplicationResult {
	if work == nil || len(actions) == 0 {
		return eventpipeline.RefreshActionApplicationResult{}
	}
	applicationResult := eventpipeline.ReduceRefreshActionsInStableOrder(
		work,
		actions,
	)
	return applicationResult
}

// RunAndApplyHookStageActionsToWorkSet runs actions producer and applies actions.
func RunAndApplyHookStageActionsToWorkSet(
	stageType HookStageType,
	runStage func() []wave.RefreshAction,
	work *eventpipeline.WorkSet,
) HookStageResult {
	if runStage == nil {
		return HookStageResult{StageType: stageType}
	}
	actions := runStage()
	return HookStageResult{
		StageType:           stageType,
		Actions:             actions,
		RefreshActionResult: ApplyHookStageActionsToWorkSet(work, actions),
	}
}

// RunAndApplyHookStageActionsAndErrorsToWorkSet runs stage producer and applies actions.
func RunAndApplyHookStageActionsAndErrorsToWorkSet(
	stageType HookStageType,
	runStage func() ([]wave.RefreshAction, []error),
	work *eventpipeline.WorkSet,
) HookStageResult {
	if runStage == nil {
		return HookStageResult{StageType: stageType}
	}
	actions, executionErrors := runStage()
	return HookStageResult{
		StageType:           stageType,
		Actions:             actions,
		ExecutionErrors:     executionErrors,
		RefreshActionResult: ApplyHookStageActionsToWorkSet(work, actions),
	}
}

// DeriveEventsWithHooksForExecution resolves events to execute from app-stop strategy.
func DeriveEventsWithHooksForExecution(
	eventsWithHooks []eventpipeline.EventWithHooks,
	appStopStrategy eventpipeline.AppStopStrategy,
) []eventpipeline.EventWithHooks {
	if len(eventsWithHooks) == 0 {
		return nil
	}

	switch appStopStrategy {
	case eventpipeline.AppStopStrategySingleEventHardReload:
		return []eventpipeline.EventWithHooks{eventsWithHooks[0]}
	default:
		return eventsWithHooks
	}
}

// DeriveImplicitBuildExecutionDecision resolves implicit build behavior.
func DeriveImplicitBuildExecutionDecision(
	shouldRunImplicitBuild bool,
	eventCount int,
) ImplicitBuildExecutionDecision {
	if !shouldRunImplicitBuild {
		return ImplicitBuildExecutionDecision{
			ShouldRunImplicitBuild:    false,
			SkipImplicitBuildLogEntry: "Skipping implicit build because event plan disabled it",
		}
	}
	if eventCount == 0 {
		return ImplicitBuildExecutionDecision{
			ShouldRunImplicitBuild:    false,
			SkipImplicitBuildLogEntry: "Skipping implicit build because no executable events remain",
		}
	}
	return ImplicitBuildExecutionDecision{ShouldRunImplicitBuild: true}
}

// ShouldShortCircuitPipelineForHookStageResult returns true when stage should stop pipeline.
func ShouldShortCircuitPipelineForHookStageResult(
	hookStageResult HookStageResult,
	configuredFailurePolicy HookStageFailurePolicy,
) bool {
	if hookStageResult.RefreshActionResult.RestartRequested {
		return true
	}
	if configuredFailurePolicy == HookStageFailurePolicyStop &&
		len(hookStageResult.ExecutionErrors) > 0 {
		return true
	}
	return false
}

// DeriveHookStageContinuationDecision derives continuation for default policy.
func DeriveHookStageContinuationDecision(
	hookStageResult HookStageResult,
) HookStageContinuationDecision {
	return DeriveHookStageContinuationDecisionWithFailurePolicy(
		hookStageResult,
		HookStageFailurePolicyContinue,
	)
}

// DeriveHookStageContinuationDecisionWithFailurePolicy applies explicit failure policy.
func DeriveHookStageContinuationDecisionWithFailurePolicy(
	hookStageResult HookStageResult,
	hookStageFailurePolicy HookStageFailurePolicy,
) HookStageContinuationDecision {
	if hookStageResult.RefreshActionResult.RestartRequested {
		return HookStageContinuationDecision{
			ShouldContinue:      false,
			StopReason:          HookStageContinuationStopReasonRestartRequested,
			RestartActionResult: hookStageResult.RefreshActionResult,
		}
	}

	if hookStageFailurePolicy == HookStageFailurePolicyStop &&
		len(hookStageResult.ExecutionErrors) > 0 {
		return HookStageContinuationDecision{
			ShouldContinue: false,
			StopReason:     HookStageContinuationStopReasonStageFailure,
		}
	}

	return HookStageContinuationDecision{ShouldContinue: true}
}

// DeriveHookStageFailurePolicy resolves stage failure policy from config value.
func DeriveHookStageFailurePolicy(
	stageType HookStageType,
	configuredValue string,
) HookStageFailurePolicy {
	_ = stageType
	return DeriveHookStageFailurePolicyFromConfiguredValue(configuredValue)
}

// DeriveHookStageFailurePolicyFromConfiguredValue parses failure policy text.
func DeriveHookStageFailurePolicyFromConfiguredValue(
	configuredValue string,
) HookStageFailurePolicy {
	switch strings.TrimSpace(strings.ToLower(configuredValue)) {
	case "stop", "fail", "strict", "halt":
		return HookStageFailurePolicyStop
	default:
		return HookStageFailurePolicyContinue
	}
}

// ShouldStartAppAfterImplicitBuild resolves whether app should restart after build.
func ShouldStartAppAfterImplicitBuild(
	runImplicitBuild bool,
	restartDecision eventpipeline.RestartPhaseDecision,
) bool {
	if !runImplicitBuild {
		return false
	}
	return restartDecision.RestartApp
}

// ShouldExecuteBrowserPhaseAfterHookStageResults resolves browser phase execution.
func ShouldExecuteBrowserPhaseAfterHookStageResults(
	preHookStageResult HookStageResult,
	concurrentHookStageResult HookStageResult,
	postHookStageResult HookStageResult,
) bool {
	if preHookStageResult.RefreshActionResult.RestartRequested {
		return false
	}
	if concurrentHookStageResult.RefreshActionResult.RestartRequested {
		return false
	}
	if postHookStageResult.RefreshActionResult.RestartRequested {
		return false
	}
	return true
}

// DeriveHookStageLabel returns stable label for logs.
func DeriveHookStageLabel(stageType HookStageType) string {
	switch stageType {
	case HookStageTypePre:
		return "pre"
	case HookStageTypeConcurrent:
		return "concurrent"
	case HookStageTypeConcurrentNoWait:
		return "concurrent_no_wait"
	case HookStageTypePost:
		return "post"
	default:
		return "unknown"
	}
}

// DeriveHookExecutionContext returns context or background fallback.
func DeriveHookExecutionContext(
	executionContext context.Context,
) context.Context {
	if executionContext == nil {
		return context.Background()
	}
	return executionContext
}

// CloneHookContextForExecution creates a context copy for execution.
func CloneHookContextForExecution(
	hookContext *wave.HookContext,
	executionContext context.Context,
) *wave.HookContext {
	if hookContext == nil {
		return &wave.HookContext{
			ExecutionContext: DeriveHookExecutionContext(executionContext),
		}
	}
	clonedChangedFilePaths := append(
		[]string(nil),
		hookContext.ChangedFilePaths...)
	return &wave.HookContext{
		ExecutionContext:   DeriveHookExecutionContext(executionContext),
		FilePath:           hookContext.FilePath,
		ChangedFilePaths:   clonedChangedFilePaths,
		AppStoppedForBatch: hookContext.AppStoppedForBatch,
	}
}

// WrapHookExecutionErrorWithStageAndPath adds stage/path context to hook errors.
func WrapHookExecutionErrorWithStageAndPath(
	stageType HookStageType,
	path string,
	hookExecutionError error,
) error {
	if hookExecutionError == nil {
		return nil
	}
	return fmt.Errorf(
		"hook stage %s failed for %q: %w",
		DeriveHookStageLabel(stageType),
		path,
		hookExecutionError,
	)
}

// JoinHookExecutionErrorsInOrder joins non-nil errors preserving order.
func JoinHookExecutionErrorsInOrder(executionErrors []error) error {
	filteredErrors := make([]error, 0, len(executionErrors))
	for _, executionError := range executionErrors {
		if executionError != nil {
			filteredErrors = append(filteredErrors, executionError)
		}
	}
	if len(filteredErrors) == 0 {
		return nil
	}
	return errors.Join(filteredErrors...)
}

// ExecuteHookCallbackSafely executes callback with panic-to-error conversion.
func ExecuteHookCallbackSafely(
	callback func(*wave.HookContext) (*wave.RefreshAction, error),
	hookContext *wave.HookContext,
) (callbackAction *wave.RefreshAction, callbackError error) {
	if callback == nil {
		return nil, nil
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			callbackAction = nil
			callbackError = fmt.Errorf("hook callback panicked: %v", recovered)
		}
	}()

	return callback(hookContext)
}

// ShouldContinueConcurrentHookExecution reports whether context still allows work.
func ShouldContinueConcurrentHookExecution(
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

// DeriveHookExecutionContextError derives context cancellation reason for hook execution.
func DeriveHookExecutionContextError(executionContext context.Context) error {
	if executionContext == nil {
		return nil
	}
	if executionContext.Err() == nil {
		return nil
	}
	return executionContext.Err()
}

// ExecuteHookCommandWithContext executes a shell command under provided context.
func ExecuteHookCommandWithContext(
	executionContext context.Context,
	command string,
) error {
	trimmedCommand := strings.TrimSpace(command)
	if trimmedCommand == "" {
		return nil
	}
	if executionContext == nil {
		executionContext = context.Background()
	}

	hookCommand := exec.CommandContext(
		executionContext,
		"sh",
		"-c",
		trimmedCommand,
	)
	hookCommand.Stdout = os.Stdout
	hookCommand.Stderr = os.Stderr
	hookCommand.Env = os.Environ()
	if runError := hookCommand.Run(); runError != nil {
		return fmt.Errorf(
			"execute hook command %q: %w",
			trimmedCommand,
			runError,
		)
	}
	return nil
}

// dedupeStablePaths removes duplicates while preserving first-seen ordering.
func dedupeStablePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	stableUniquePaths := make([]string, 0, len(paths))
	for _, path := range paths {
		normalizedPath := NormalizeHookContextPathShape(path)
		if normalizedPath == "" {
			continue
		}
		if _, alreadySeen := seen[normalizedPath]; alreadySeen {
			continue
		}
		seen[normalizedPath] = struct{}{}
		stableUniquePaths = append(stableUniquePaths, normalizedPath)
	}
	return stableUniquePaths
}

// SortActionsByRestartPriority sorts actions so restart actions are applied last.
func SortActionsByRestartPriority(actions []wave.RefreshAction) {
	sort.SliceStable(actions, func(leftIndex int, rightIndex int) bool {
		leftRestart := actions[leftIndex].TriggerRestart
		rightRestart := actions[rightIndex].TriggerRestart
		if leftRestart == rightRestart {
			return leftIndex < rightIndex
		}
		return !leftRestart && rightRestart
	})
}
