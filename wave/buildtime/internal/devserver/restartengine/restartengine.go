// Package restartengine decides and executes dev-server restart/build behavior.
//
// Keeping restart policy here prevents runloop and hook layers from duplicating
// process ordering and cancellation logic.
package restartengine

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// RestartRequest is one queued restart intent from watcher or lifecycle actions.
type RestartRequest struct {
	RecompileGo     bool
	IsConfigRestart bool
}

// NormalizeRestartRequest normalizes an incoming restart request.
func NormalizeRestartRequest(request RestartRequest) RestartRequest {
	if request.IsConfigRestart {
		request.RecompileGo = true
	}
	return request
}

// MergeRestartRequests merges restart intents with strict upgrade semantics.
func MergeRestartRequests(
	pendingRequest RestartRequest,
	incomingRequest RestartRequest,
) RestartRequest {
	resolvedPendingRequest := NormalizeRestartRequest(pendingRequest)
	resolvedIncomingRequest := NormalizeRestartRequest(incomingRequest)
	return RestartRequest{
		RecompileGo: resolvedPendingRequest.RecompileGo ||
			resolvedIncomingRequest.RecompileGo,
		IsConfigRestart: resolvedPendingRequest.IsConfigRestart ||
			resolvedIncomingRequest.IsConfigRestart,
	}
}

// ResolveQueuedRestartRequest resolves incoming request against optional pending one.
func ResolveQueuedRestartRequest(
	pendingRequest *RestartRequest,
	incomingRequest RestartRequest,
) RestartRequest {
	resolvedIncomingRequest := NormalizeRestartRequest(incomingRequest)
	if pendingRequest == nil {
		return resolvedIncomingRequest
	}
	return MergeRestartRequests(*pendingRequest, resolvedIncomingRequest)
}

// TryEnqueueRestartRequest attempts non-blocking enqueue.
func TryEnqueueRestartRequest(
	restartRequests chan RestartRequest,
	request RestartRequest,
) bool {
	if restartRequests == nil {
		return false
	}
	select {
	case restartRequests <- NormalizeRestartRequest(request):
		return true
	default:
		return false
	}
}

// TryDequeueRestartRequest attempts non-blocking dequeue.
func TryDequeueRestartRequest(
	restartRequests chan RestartRequest,
) (RestartRequest, bool) {
	if restartRequests == nil {
		return RestartRequest{}, false
	}
	select {
	case request := <-restartRequests:
		return NormalizeRestartRequest(request), true
	default:
		return RestartRequest{}, false
	}
}

// RestartIntentAccumulator coalesces restart intents without losing upgrades.
type RestartIntentAccumulator struct {
	mu sync.Mutex

	restartRequests chan RestartRequest
	pendingRequest  *RestartRequest
}

// NewRestartIntentAccumulator creates an intent accumulator around request channel.
func NewRestartIntentAccumulator(
	restartRequests chan RestartRequest,
) *RestartIntentAccumulator {
	if restartRequests == nil {
		restartRequests = make(chan RestartRequest, 1)
	}
	return &RestartIntentAccumulator{restartRequests: restartRequests}
}

// HasQueuedOrPendingRequest reports whether any restart request is currently queued.
func (accumulator *RestartIntentAccumulator) HasQueuedOrPendingRequest() bool {
	if accumulator == nil {
		return false
	}

	accumulator.mu.Lock()
	defer accumulator.mu.Unlock()
	return accumulator.pendingRequest != nil ||
		len(accumulator.restartRequests) > 0
}

// Queue queues restart request, coalescing when queue is already full.
func (accumulator *RestartIntentAccumulator) Queue(
	incomingRequest RestartRequest,
) {
	if accumulator == nil {
		return
	}

	accumulator.mu.Lock()
	defer accumulator.mu.Unlock()

	resolvedIncomingRequest := NormalizeRestartRequest(incomingRequest)
	if TryEnqueueRestartRequest(
		accumulator.restartRequests,
		resolvedIncomingRequest,
	) {
		return
	}

	queuedRequest, queueHadValue := TryDequeueRestartRequest(
		accumulator.restartRequests,
	)
	if queueHadValue {
		mergedRequest := MergeRestartRequests(
			queuedRequest,
			resolvedIncomingRequest,
		)
		if TryEnqueueRestartRequest(
			accumulator.restartRequests,
			mergedRequest,
		) {
			return
		}
		resolvedIncomingRequest = mergedRequest
	}

	resolvedPendingRequest := ResolveQueuedRestartRequest(
		accumulator.pendingRequest,
		resolvedIncomingRequest,
	)
	accumulator.pendingRequest = &resolvedPendingRequest
}

// ConsumePending returns pending request if present and clears pending state.
func (accumulator *RestartIntentAccumulator) ConsumePending() (RestartRequest, bool) {
	if accumulator == nil {
		return RestartRequest{}, false
	}

	accumulator.mu.Lock()
	defer accumulator.mu.Unlock()

	if accumulator.pendingRequest != nil {
		pending := *accumulator.pendingRequest
		accumulator.pendingRequest = nil
		return NormalizeRestartRequest(pending), true
	}

	queuedRequest, hasQueuedRequest := TryDequeueRestartRequest(
		accumulator.restartRequests,
	)
	if hasQueuedRequest {
		return NormalizeRestartRequest(queuedRequest), true
	}

	return RestartRequest{}, false
}

// ConsumeBlocking blocks until a restart request becomes available.
func (accumulator *RestartIntentAccumulator) ConsumeBlocking() RestartRequest {
	if accumulator == nil {
		return RestartRequest{}
	}

	if pendingRequest, hasPendingRequest := accumulator.ConsumePending(); hasPendingRequest {
		return pendingRequest
	}

	request := <-accumulator.restartRequests
	return NormalizeRestartRequest(request)
}

// RunIntent is one cycle-level intent derived from restart request.
type RunIntent struct {
	RecompileGo     bool
	IsRebuild       bool
	IsConfigRestart bool
}

// DeriveRunIntentFromRestartRequest derives run-cycle execution intent.
func DeriveRunIntentFromRestartRequest(
	restartRequestForIntent RestartRequest,
) RunIntent {
	resolvedRequest := NormalizeRestartRequest(restartRequestForIntent)
	return RunIntent{
		RecompileGo:     resolvedRequest.RecompileGo,
		IsRebuild:       true,
		IsConfigRestart: resolvedRequest.IsConfigRestart,
	}
}

// GoCompilationOrderingPolicy controls relative ordering of hook/build work.
type GoCompilationOrderingPolicy int

const (
	// GoCompilationOrderingPolicyConcurrentWithBuildHooks allows parallel go build + hooks.
	GoCompilationOrderingPolicyConcurrentWithBuildHooks GoCompilationOrderingPolicy = iota
	// GoCompilationOrderingPolicyAfterBuildHooks serializes go build after hooks.
	GoCompilationOrderingPolicyAfterBuildHooks
	// GoCompilationOrderingPolicyNotRequested disables go compilation.
	GoCompilationOrderingPolicyNotRequested
)

// RunBuildExecutionOrderingDecision determines one run-build execution strategy.
type RunBuildExecutionOrderingDecision struct {
	ShouldCompileGo bool
	OrderingPolicy  GoCompilationOrderingPolicy
}

// DeriveRunBuildExecutionOrderingDecision derives build ordering from intent/config.
func DeriveRunBuildExecutionOrderingDecision(
	shouldRecompileGo bool,
	sequentialGoBuild bool,
) RunBuildExecutionOrderingDecision {
	if !shouldRecompileGo {
		return RunBuildExecutionOrderingDecision{
			ShouldCompileGo: false,
			OrderingPolicy:  GoCompilationOrderingPolicyNotRequested,
		}
	}

	if sequentialGoBuild {
		return RunBuildExecutionOrderingDecision{
			ShouldCompileGo: true,
			OrderingPolicy:  GoCompilationOrderingPolicyAfterBuildHooks,
		}
	}
	return RunBuildExecutionOrderingDecision{
		ShouldCompileGo: true,
		OrderingPolicy:  GoCompilationOrderingPolicyConcurrentWithBuildHooks,
	}
}

// RunCycleScope owns cancellation and async work joining for one cycle.
type RunCycleScope struct {
	CycleID          uint64
	ExecutionContext context.Context

	cancelExecution context.CancelFunc
	waitGroup       sync.WaitGroup
}

// NewRunCycleScope creates one cycle scope with cancelable context.
func NewRunCycleScope(cycleID uint64) *RunCycleScope {
	executionContext, cancelExecution := context.WithCancel(
		context.Background(),
	)
	return &RunCycleScope{
		CycleID:          cycleID,
		ExecutionContext: executionContext,
		cancelExecution:  cancelExecution,
	}
}

// LaunchAsyncWork launches one asynchronous unit under cycle context.
func (runCycleScope *RunCycleScope) LaunchAsyncWork(
	runAsyncWork func(context.Context),
) {
	if runCycleScope == nil || runAsyncWork == nil {
		return
	}
	runCycleScope.waitGroup.Add(1)
	go func() {
		defer runCycleScope.waitGroup.Done()
		runAsyncWork(runCycleScope.ExecutionContext)
	}()
}

// Cancel requests cancellation for cycle-scoped work.
func (runCycleScope *RunCycleScope) Cancel() {
	if runCycleScope == nil || runCycleScope.cancelExecution == nil {
		return
	}
	runCycleScope.cancelExecution()
}

// Wait blocks until all launched async work has completed.
func (runCycleScope *RunCycleScope) Wait() {
	if runCycleScope == nil {
		return
	}
	runCycleScope.waitGroup.Wait()
}

// CancelAndWait cancels and then waits for all scoped work.
func (runCycleScope *RunCycleScope) CancelAndWait() {
	if runCycleScope == nil {
		return
	}
	runCycleScope.Cancel()
	runCycleScope.Wait()
}

// RunLifecycleState identifies one run-loop state.
type RunLifecycleState int

const (
	// RunLifecycleStatePreparingCycle performs cleanup and cycle setup.
	RunLifecycleStatePreparingCycle RunLifecycleState = iota
	// RunLifecycleStateBuildingCycle performs one build for current intent.
	RunLifecycleStateBuildingCycle
	// RunLifecycleStateAwaitingBuildRetry waits for file changes after build failure.
	RunLifecycleStateAwaitingBuildRetry
	// RunLifecycleStateStartingRuntime starts app/vite/runtime watchers.
	RunLifecycleStateStartingRuntime
	// RunLifecycleStateAwaitingRestart waits for explicit restart requests.
	RunLifecycleStateAwaitingRestart
	// RunLifecycleStateCleaningUpForNextCycle performs stop/cancel transitions.
	RunLifecycleStateCleaningUpForNextCycle
)

// RunLifecycleEvent identifies one transition trigger.
type RunLifecycleEvent int

const (
	// RunLifecycleEventCyclePrepared transitions preparation to build.
	RunLifecycleEventCyclePrepared RunLifecycleEvent = iota
	// RunLifecycleEventBuildSucceeded transitions build to runtime start.
	RunLifecycleEventBuildSucceeded
	// RunLifecycleEventBuildFailed transitions build to retry wait.
	RunLifecycleEventBuildFailed
	// RunLifecycleEventBuildRetryRestartReceived transitions retry wait to cleanup.
	RunLifecycleEventBuildRetryRestartReceived
	// RunLifecycleEventRuntimeStarted transitions runtime-start to restart wait.
	RunLifecycleEventRuntimeStarted
	// RunLifecycleEventRestartRequestReceived transitions restart wait to cleanup.
	RunLifecycleEventRestartRequestReceived
	// RunLifecycleEventCleanupCompleted transitions cleanup to prepare cycle.
	RunLifecycleEventCleanupCompleted
)

// TransitionLogger is the minimal logging contract for lifecycle transitions.
type TransitionLogger interface {
	Info(string, ...any)
}

// DeriveRunLifecycleStateAfterEvent applies pure transition logic.
func DeriveRunLifecycleStateAfterEvent(
	currentRunLifecycleState RunLifecycleState,
	runLifecycleEventForTransition RunLifecycleEvent,
) (RunLifecycleState, error) {
	switch currentRunLifecycleState {
	case RunLifecycleStatePreparingCycle:
		if runLifecycleEventForTransition == RunLifecycleEventCyclePrepared {
			return RunLifecycleStateBuildingCycle, nil
		}

	case RunLifecycleStateBuildingCycle:
		switch runLifecycleEventForTransition {
		case RunLifecycleEventBuildSucceeded:
			return RunLifecycleStateStartingRuntime, nil
		case RunLifecycleEventBuildFailed:
			return RunLifecycleStateAwaitingBuildRetry, nil
		}

	case RunLifecycleStateAwaitingBuildRetry:
		if runLifecycleEventForTransition == RunLifecycleEventBuildRetryRestartReceived {
			return RunLifecycleStateCleaningUpForNextCycle, nil
		}

	case RunLifecycleStateStartingRuntime:
		if runLifecycleEventForTransition == RunLifecycleEventRuntimeStarted {
			return RunLifecycleStateAwaitingRestart, nil
		}

	case RunLifecycleStateAwaitingRestart:
		if runLifecycleEventForTransition == RunLifecycleEventRestartRequestReceived {
			return RunLifecycleStateCleaningUpForNextCycle, nil
		}

	case RunLifecycleStateCleaningUpForNextCycle:
		if runLifecycleEventForTransition == RunLifecycleEventCleanupCompleted {
			return RunLifecycleStatePreparingCycle, nil
		}
	}

	return RunLifecycleStatePreparingCycle, fmt.Errorf(
		"invalid lifecycle transition: state=%s event=%s",
		RunLifecycleStateString(currentRunLifecycleState),
		RunLifecycleEventString(runLifecycleEventForTransition),
	)
}

// TransitionRunLifecycleState applies one transition and optionally logs it.
func TransitionRunLifecycleState(
	transitionLogger TransitionLogger,
	currentRunLifecycleState RunLifecycleState,
	runLifecycleEventForTransition RunLifecycleEvent,
	currentLifecycleCycleID uint64,
) (RunLifecycleState, error) {
	nextRunLifecycleState, transitionError := DeriveRunLifecycleStateAfterEvent(
		currentRunLifecycleState,
		runLifecycleEventForTransition,
	)
	if transitionError != nil {
		return RunLifecycleStatePreparingCycle, transitionError
	}

	if transitionLogger != nil {
		transitionLogger.Info(
			"run lifecycle transition",
			"cycle_id",
			currentLifecycleCycleID,
			"from",
			RunLifecycleStateString(currentRunLifecycleState),
			"event",
			RunLifecycleEventString(runLifecycleEventForTransition),
			"to",
			RunLifecycleStateString(nextRunLifecycleState),
		)
	}

	return nextRunLifecycleState, nil
}

// RunLifecycleCommand identifies a concrete command implementation for state.
type RunLifecycleCommand int

const (
	// RunLifecycleCommandPrepareCycle executes pre-cycle setup.
	RunLifecycleCommandPrepareCycle RunLifecycleCommand = iota
	// RunLifecycleCommandBuildCycle executes build-stage work.
	RunLifecycleCommandBuildCycle
	// RunLifecycleCommandAwaitBuildRetry waits for retry trigger.
	RunLifecycleCommandAwaitBuildRetry
	// RunLifecycleCommandStartRuntime starts runtime services.
	RunLifecycleCommandStartRuntime
	// RunLifecycleCommandAwaitRestartRequest blocks for restart request.
	RunLifecycleCommandAwaitRestartRequest
	// RunLifecycleCommandCleanupForNextCycle executes restart cleanup.
	RunLifecycleCommandCleanupForNextCycle
)

// DeriveRunLifecycleCommandForState maps lifecycle state to command.
func DeriveRunLifecycleCommandForState(
	currentRunLifecycleState RunLifecycleState,
) (RunLifecycleCommand, error) {
	switch currentRunLifecycleState {
	case RunLifecycleStatePreparingCycle:
		return RunLifecycleCommandPrepareCycle, nil
	case RunLifecycleStateBuildingCycle:
		return RunLifecycleCommandBuildCycle, nil
	case RunLifecycleStateAwaitingBuildRetry:
		return RunLifecycleCommandAwaitBuildRetry, nil
	case RunLifecycleStateStartingRuntime:
		return RunLifecycleCommandStartRuntime, nil
	case RunLifecycleStateAwaitingRestart:
		return RunLifecycleCommandAwaitRestartRequest, nil
	case RunLifecycleStateCleaningUpForNextCycle:
		return RunLifecycleCommandCleanupForNextCycle, nil
	default:
		return RunLifecycleCommandPrepareCycle, fmt.Errorf(
			"unknown lifecycle state: %d",
			currentRunLifecycleState,
		)
	}
}

// RunLifecycleCommandInput carries cycle-level command dependencies.
type RunLifecycleCommandInput struct {
	CurrentCycleID uint64
	CurrentIntent  RunIntent
	FirstRun       bool
}

// RunLifecycleCommandResult carries event and next-intent from command execution.
type RunLifecycleCommandResult struct {
	RunLifecycleEvent RunLifecycleEvent
	NextRunIntent     RunIntent
}

// RunLifecycleStateString provides stable log labels for states.
func RunLifecycleStateString(state RunLifecycleState) string {
	switch state {
	case RunLifecycleStatePreparingCycle:
		return "preparing_cycle"
	case RunLifecycleStateBuildingCycle:
		return "building_cycle"
	case RunLifecycleStateAwaitingBuildRetry:
		return "awaiting_build_retry"
	case RunLifecycleStateStartingRuntime:
		return "starting_runtime"
	case RunLifecycleStateAwaitingRestart:
		return "awaiting_restart"
	case RunLifecycleStateCleaningUpForNextCycle:
		return "cleaning_up_for_next_cycle"
	default:
		return "unknown"
	}
}

// RunLifecycleEventString provides stable log labels for transition events.
func RunLifecycleEventString(event RunLifecycleEvent) string {
	switch event {
	case RunLifecycleEventCyclePrepared:
		return "cycle_prepared"
	case RunLifecycleEventBuildSucceeded:
		return "build_succeeded"
	case RunLifecycleEventBuildFailed:
		return "build_failed"
	case RunLifecycleEventBuildRetryRestartReceived:
		return "build_retry_restart_received"
	case RunLifecycleEventRuntimeStarted:
		return "runtime_started"
	case RunLifecycleEventRestartRequestReceived:
		return "restart_request_received"
	case RunLifecycleEventCleanupCompleted:
		return "cleanup_completed"
	default:
		return "unknown"
	}
}

// ValidateLifecycleTransitionTable runs simple consistency checks over transitions.
func ValidateLifecycleTransitionTable() error {
	requiredTransitions := []struct {
		state RunLifecycleState
		event RunLifecycleEvent
	}{
		{RunLifecycleStatePreparingCycle, RunLifecycleEventCyclePrepared},
		{RunLifecycleStateBuildingCycle, RunLifecycleEventBuildSucceeded},
		{RunLifecycleStateBuildingCycle, RunLifecycleEventBuildFailed},
		{
			RunLifecycleStateAwaitingBuildRetry,
			RunLifecycleEventBuildRetryRestartReceived,
		},
		{RunLifecycleStateStartingRuntime, RunLifecycleEventRuntimeStarted},
		{
			RunLifecycleStateAwaitingRestart,
			RunLifecycleEventRestartRequestReceived,
		},
		{
			RunLifecycleStateCleaningUpForNextCycle,
			RunLifecycleEventCleanupCompleted,
		},
	}

	for _, transition := range requiredTransitions {
		if _, transitionError := DeriveRunLifecycleStateAfterEvent(transition.state, transition.event); transitionError != nil {
			return transitionError
		}
	}

	if _, transitionError := DeriveRunLifecycleStateAfterEvent(
		RunLifecycleStatePreparingCycle,
		RunLifecycleEventBuildSucceeded,
	); transitionError == nil {
		return errors.New(
			"lifecycle transition table accepted invalid transition",
		)
	}

	return nil
}
