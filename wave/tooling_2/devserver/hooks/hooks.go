package hooks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vormadev/vorma/kit/executil"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/waveshared"
	"github.com/vormadev/vorma/wave/tooling_2/devserver/eventpipeline"
	"github.com/vormadev/vorma/wave/tooling_2/watch"
)

type EventWithHooks = eventpipeline.EventWithHooks
type ClassifiedEvent = eventpipeline.ClassifiedEvent

const fileTypeGo = eventpipeline.FileTypeGo

// -----------------------------------------------------------------------------
// Hook execution planning.
// -----------------------------------------------------------------------------
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

// HookStageExecutionDescriptor identifies one event that should execute hooks
// for the current stage.
type HookStageExecutionDescriptor struct {
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

// HookStageResult captures one stage execution outcome and resulting actions.
type HookStageResult struct {
	Actions             []wave.RefreshAction
	RefreshActionResult eventpipeline.RefreshActionApplicationResult
	StageType           HookStageType
	ExecutionErrors     []error
}

// HookStageFailurePolicy defines whether stage failures are fail-open or
// fail-closed.
type HookStageFailurePolicy int

const (
	HookStageFailurePolicyFailOpen HookStageFailurePolicy = iota
	HookStageFailurePolicyFailClosed
)

const (
	ConfiguredHookStageFailurePolicyFailOpen   = "fail-open"
	ConfiguredHookStageFailurePolicyFailClosed = "fail-closed"
)

// HookStageContinuationStopReason explains why pipeline continuation stopped.
type HookStageContinuationStopReason int

const (
	HookStageContinuationStopReasonNone HookStageContinuationStopReason = iota
	HookStageContinuationStopReasonRestartRequested
	HookStageContinuationStopReasonStageFailure
)

// HookStageContinuationDecision resolves whether a pipeline should continue.
type HookStageContinuationDecision struct {
	ShouldContinue      bool
	StopReason          HookStageContinuationStopReason
	RestartActionResult eventpipeline.RefreshActionApplicationResult
}

// ImplicitBuildExecutionDecision resolves whether implicit build should run.
type ImplicitBuildExecutionDecision struct {
	ShouldRunImplicitBuild    bool
	SkipImplicitBuildLogEntry string
}

func DeriveHookStageExecutionDescriptors(
	eventsWithHooks []EventWithHooks,
) []HookStageExecutionDescriptor {
	if len(eventsWithHooks) == 0 {
		return nil
	}

	descriptors := make([]HookStageExecutionDescriptor, 0, len(eventsWithHooks))
	for eventIndex, eventWithHooksForExecution := range eventsWithHooks {
		if eventWithHooksForExecution.SkipDuplicateHooks {
			continue
		}
		descriptors = append(descriptors, HookStageExecutionDescriptor{
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

const MaxConcurrentNoWaitHookExecutions = 16

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

// -----------------------------------------------------------------------------
// Hook-context materialization from classified events.
// -----------------------------------------------------------------------------
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
	eventNeedsHardReload := classifiedEventForProcessing.FileType == fileTypeGo ||
		eventpipeline.NeedsHardReload(watchedFileForProcessing)

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

// -----------------------------------------------------------------------------
// Timeout policy derivation.
// -----------------------------------------------------------------------------
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

// ApplyHookStageActionsToWorkSet applies hook actions to work and returns stage
// result metadata.
func ApplyHookStageActionsToWorkSet(
	hookStageActions []wave.RefreshAction,
	work *eventpipeline.WorkSet,
) HookStageResult {
	hookStageResultForWork := HookStageResult{
		Actions: append([]wave.RefreshAction(nil), hookStageActions...),
	}
	if work == nil {
		return hookStageResultForWork
	}

	hookStageResultForWork.RefreshActionResult = work.ApplyRefreshActions(
		hookStageActions,
	)
	return hookStageResultForWork
}

func applyHookStageActionsAndErrorsToWorkSet(
	stageType HookStageType,
	hookStageActions []wave.RefreshAction,
	hookStageExecutionErrors []error,
	work *eventpipeline.WorkSet,
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

// RunAndApplyHookStageActionsToWorkSet executes hook action callback and applies
// resulting actions to work.
func RunAndApplyHookStageActionsToWorkSet(
	runHookStageActions func() []wave.RefreshAction,
	work *eventpipeline.WorkSet,
) HookStageResult {
	if runHookStageActions == nil {
		return ApplyHookStageActionsToWorkSet(nil, work)
	}
	return ApplyHookStageActionsToWorkSet(runHookStageActions(), work)
}

// RunAndApplyHookStageActionsAndErrorsToWorkSet executes hook stage callback and
// applies both actions and error metadata to work.
func RunAndApplyHookStageActionsAndErrorsToWorkSet(
	stageType HookStageType,
	runHookStageActions func() ([]wave.RefreshAction, []error),
	work *eventpipeline.WorkSet,
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

// DeriveEventsWithHooksForExecution clones and annotates hook contexts for
// batch hard-reload execution.
func DeriveEventsWithHooksForExecution(
	eventsWithHooks []EventWithHooks,
	appStopStrategyForExecution eventpipeline.AppStopStrategy,
) []EventWithHooks {
	if len(eventsWithHooks) == 0 {
		return nil
	}
	if appStopStrategyForExecution != eventpipeline.AppStopStrategyBatchHardReload {
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

// DeriveImplicitBuildExecutionDecision resolves whether implicit build should
// execute for the current event batch.
func DeriveImplicitBuildExecutionDecision(
	shouldRunImplicitBuild bool,
	eventCount int,
) ImplicitBuildExecutionDecision {
	if shouldRunImplicitBuild {
		return ImplicitBuildExecutionDecision{
			ShouldRunImplicitBuild: true,
		}
	}

	if eventCount == 1 {
		return ImplicitBuildExecutionDecision{
			SkipImplicitBuildLogEntry: "RunOnChangeOnly: skipping implicit build phase",
		}
	}

	return ImplicitBuildExecutionDecision{
		SkipImplicitBuildLogEntry: "All events are RunOnChangeOnly, skipping implicit build phase",
	}
}

// ShouldShortCircuitPipelineForHookStageResult reports whether stage outcomes
// should stop pipeline execution.
func ShouldShortCircuitPipelineForHookStageResult(
	hookStageResultForCheck HookStageResult,
) bool {
	return !DeriveHookStageContinuationDecisionWithFailurePolicy(
		hookStageResultForCheck,
		HookStageFailurePolicyFailOpen,
	).ShouldContinue
}

// DeriveHookStageContinuationDecision applies default fail-open policy.
func DeriveHookStageContinuationDecision(
	hookStageResultForContinuation HookStageResult,
) HookStageContinuationDecision {
	return DeriveHookStageContinuationDecisionWithFailurePolicy(
		hookStageResultForContinuation,
		HookStageFailurePolicyFailOpen,
	)
}

// DeriveHookStageContinuationDecisionWithFailurePolicy resolves continuation
// based on stage results and policy.
func DeriveHookStageContinuationDecisionWithFailurePolicy(
	hookStageResultForContinuation HookStageResult,
	hookStageFailurePolicyForStage HookStageFailurePolicy,
) HookStageContinuationDecision {
	if hookStageResultForContinuation.RefreshActionResult.RestartRequested {
		return HookStageContinuationDecision{
			StopReason:          HookStageContinuationStopReasonRestartRequested,
			RestartActionResult: hookStageResultForContinuation.RefreshActionResult,
		}
	}

	hasHookStageExecutionErrors := len(hookStageResultForContinuation.ExecutionErrors) > 0
	if hasHookStageExecutionErrors &&
		hookStageFailurePolicyForStage == HookStageFailurePolicyFailClosed {
		return HookStageContinuationDecision{
			StopReason: HookStageContinuationStopReasonStageFailure,
		}
	}

	return HookStageContinuationDecision{
		ShouldContinue: true,
	}
}

// DeriveHookStageFailurePolicy resolves stage policy from config string.
func DeriveHookStageFailurePolicy(
	stageType HookStageType,
	configuredHookStageFailurePolicy string,
) HookStageFailurePolicy {
	if stageType == HookStageTypeConcurrentNoWait {
		return HookStageFailurePolicyFailOpen
	}
	return DeriveHookStageFailurePolicyFromConfiguredValue(
		configuredHookStageFailurePolicy,
	)
}

// DeriveHookStageFailurePolicyFromConfiguredValue resolves configured policy.
func DeriveHookStageFailurePolicyFromConfiguredValue(
	configuredHookStageFailurePolicy string,
) HookStageFailurePolicy {
	switch normalizeConfiguredHookStageFailurePolicy(configuredHookStageFailurePolicy) {
	case ConfiguredHookStageFailurePolicyFailClosed:
		return HookStageFailurePolicyFailClosed
	default:
		return HookStageFailurePolicyFailOpen
	}
}

func normalizeConfiguredHookStageFailurePolicy(
	configuredHookStageFailurePolicy string,
) string {
	return strings.ToLower(strings.TrimSpace(configuredHookStageFailurePolicy))
}

// ShouldStartAppAfterImplicitBuild reports whether app startup should run after
// implicit build processing.
func ShouldStartAppAfterImplicitBuild(
	ranImplicitBuild bool,
	restartDecision eventpipeline.RestartPhaseDecision,
) bool {
	if !ranImplicitBuild {
		return false
	}
	return restartDecision.RestartApp
}

// ShouldExecuteBrowserPhaseAfterHookStageResults reports whether browser phase
// can execute after all hook stages.
func ShouldExecuteBrowserPhaseAfterHookStageResults(
	preHookStageResult HookStageResult,
	concurrentHookStageResult HookStageResult,
	postHookStageResult HookStageResult,
) bool {
	return !ShouldShortCircuitPipelineForHookStageResult(preHookStageResult) &&
		!ShouldShortCircuitPipelineForHookStageResult(concurrentHookStageResult) &&
		!ShouldShortCircuitPipelineForHookStageResult(postHookStageResult)
}

// DeriveHookStageLabel resolves the stable label for one hook stage.
func DeriveHookStageLabel(
	stageType HookStageType,
) string {
	return deriveHookStageLabel(stageType)
}

// DeriveHookExecutionContext normalizes nil parent execution contexts.
func DeriveHookExecutionContext(
	parentHookExecutionContext context.Context,
) context.Context {
	return deriveHookExecutionContext(parentHookExecutionContext)
}

// CloneHookContextForExecution clones hookContext and applies executionContext.
func CloneHookContextForExecution(
	hookContext *wave.HookContext,
	hookExecutionContext context.Context,
) *wave.HookContext {
	return cloneHookContextForExecution(hookContext, hookExecutionContext)
}

// WrapHookExecutionErrorWithStageAndPath decorates hook errors with stage and path.
func WrapHookExecutionErrorWithStageAndPath(
	stageType HookStageType,
	changedFilePath string,
	hookExecutionError error,
) error {
	return wrapHookExecutionErrorWithStageAndPath(
		stageType,
		changedFilePath,
		hookExecutionError,
	)
}

// JoinHookExecutionErrorsInOrder joins hook errors in deterministic order.
func JoinHookExecutionErrorsInOrder(
	hookExecutionErrorsByHookIndex []error,
) error {
	return joinHookExecutionErrorsInOrder(hookExecutionErrorsByHookIndex)
}

// ExecuteHookCallbackSafely executes callback and converts panics into errors.
func ExecuteHookCallbackSafely(
	callback func(*wave.HookContext) (*wave.RefreshAction, error),
	hookContext *wave.HookContext,
) (*wave.RefreshAction, error) {
	return executeHookCallbackSafely(callback, hookContext)
}

// ShouldContinueConcurrentHookExecution reports whether context allows work.
func ShouldContinueConcurrentHookExecution(
	concurrentHookExecutionContext context.Context,
) bool {
	return shouldContinueConcurrentHookExecution(concurrentHookExecutionContext)
}

// DeriveHookExecutionContextError returns the context error for canceled contexts.
func DeriveHookExecutionContextError(
	hookExecutionContext context.Context,
) error {
	return deriveHookExecutionContextError(hookExecutionContext)
}

// ExecuteHookCommandWithContext runs a shell command in hookExecutionContext.
func ExecuteHookCommandWithContext(
	hookCommandExecutionContext context.Context,
	command string,
) error {
	return executeHookCommandWithContext(hookCommandExecutionContext, command)
}
