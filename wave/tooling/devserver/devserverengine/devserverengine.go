package devserverengine

import (
	"context"
	"fmt"
	"sync"
)

// RestartRequest signals what kind of restart is needed.
type RestartRequest struct {
	RecompileGo     bool
	IsConfigRestart bool
}

// NormalizeRestartRequest upgrades invalid restart combinations into canonical
// values.
func NormalizeRestartRequest(request RestartRequest) RestartRequest {
	if request.IsConfigRestart {
		request.RecompileGo = true
	}
	return request
}

// ResolveQueuedRestartRequest merges incomingRequest with any pending request.
func ResolveQueuedRestartRequest(
	pendingRequest *RestartRequest,
	incomingRequest RestartRequest,
) RestartRequest {
	normalizedIncomingRequest := NormalizeRestartRequest(incomingRequest)
	if pendingRequest == nil {
		return normalizedIncomingRequest
	}

	normalizedPendingRequest := NormalizeRestartRequest(*pendingRequest)
	return MergeRestartRequests(normalizedPendingRequest, normalizedIncomingRequest)
}

// TryEnqueueRestartRequest attempts to enqueue request without blocking.
func TryEnqueueRestartRequest(
	restartRequests chan RestartRequest,
	request RestartRequest,
) bool {
	select {
	case restartRequests <- request:
		return true
	default:
		return false
	}
}

// TryDequeueRestartRequest attempts to dequeue one request without blocking.
func TryDequeueRestartRequest(
	restartRequests chan RestartRequest,
) (RestartRequest, bool) {
	select {
	case pendingRequest := <-restartRequests:
		return pendingRequest, true
	default:
		return RestartRequest{}, false
	}
}

// MergeRestartRequests combines two restart requests with config-restart
// precedence.
func MergeRestartRequests(
	pendingRequest RestartRequest,
	incomingRequest RestartRequest,
) RestartRequest {
	if pendingRequest.IsConfigRestart || incomingRequest.IsConfigRestart {
		return RestartRequest{
			RecompileGo:     true,
			IsConfigRestart: true,
		}
	}

	return RestartRequest{
		RecompileGo:     pendingRequest.RecompileGo || incomingRequest.RecompileGo,
		IsConfigRestart: false,
	}
}

// RestartIntentAccumulator coalesces rapid restart requests into deterministic
// intent.
type RestartIntentAccumulator struct {
	Mu                   sync.Mutex
	RestartRequests      chan RestartRequest
	WaitingForBuildRetry bool
}

// NewRestartIntentAccumulator constructs a restart accumulator.
func NewRestartIntentAccumulator(
	restartRequests chan RestartRequest,
) *RestartIntentAccumulator {
	if restartRequests == nil {
		restartRequests = make(chan RestartRequest, 1)
	}
	return &RestartIntentAccumulator{
		RestartRequests: restartRequests,
	}
}

// SetWaitingForBuildRetry configures queue behavior while waiting for retry.
func (accumulator *RestartIntentAccumulator) SetWaitingForBuildRetry(
	waitingForBuildRetry bool,
) {
	if accumulator == nil {
		return
	}

	accumulator.Mu.Lock()
	accumulator.WaitingForBuildRetry = waitingForBuildRetry
	accumulator.Mu.Unlock()
}

// QueueRestartRequest records a restart intent with coalescing semantics.
func (accumulator *RestartIntentAccumulator) QueueRestartRequest(
	restartRequestForQueue RestartRequest,
) {
	if accumulator == nil {
		return
	}

	accumulator.Mu.Lock()
	defer accumulator.Mu.Unlock()

	incomingRequest := NormalizeRestartRequest(restartRequestForQueue)
	if accumulator.WaitingForBuildRetry {
		_ = TryEnqueueRestartRequest(accumulator.RestartRequests, incomingRequest)
		return
	}

	if TryEnqueueRestartRequest(accumulator.RestartRequests, incomingRequest) {
		return
	}

	pendingRequest, hasPendingRequest := TryDequeueRestartRequest(
		accumulator.RestartRequests,
	)
	if !hasPendingRequest {
		_ = TryEnqueueRestartRequest(accumulator.RestartRequests, incomingRequest)
		return
	}

	queuedRequest := ResolveQueuedRestartRequest(
		&pendingRequest,
		incomingRequest,
	)
	_ = TryEnqueueRestartRequest(accumulator.RestartRequests, queuedRequest)
}

// ConsumePendingRestartRequest dequeues one pending request without blocking.
func (accumulator *RestartIntentAccumulator) ConsumePendingRestartRequest() (RestartRequest, bool) {
	if accumulator == nil {
		return RestartRequest{}, false
	}

	accumulator.Mu.Lock()
	defer accumulator.Mu.Unlock()

	return TryDequeueRestartRequest(accumulator.RestartRequests)
}

// ConsumeRestartRequestBlocking dequeues one request, blocking if needed.
func (accumulator *RestartIntentAccumulator) ConsumeRestartRequestBlocking() RestartRequest {
	if accumulator == nil {
		return RestartRequest{}
	}

	if pendingRequest, hasPendingRequest := accumulator.ConsumePendingRestartRequest(); hasPendingRequest {
		return NormalizeRestartRequest(pendingRequest)
	}

	return NormalizeRestartRequest(<-accumulator.RestartRequests)
}

// GoCompilationOrderingPolicy controls whether Go compilation runs in parallel
// with build hooks.
type GoCompilationOrderingPolicy string

const (
	GoCompilationOrderingPolicyConcurrentWithBuildHooks GoCompilationOrderingPolicy = "compile_concurrently_with_build_hooks"
	GoCompilationOrderingPolicyAfterBuildHooks          GoCompilationOrderingPolicy = "compile_after_build_hooks"
	GoCompilationOrderingPolicyNotRequested             GoCompilationOrderingPolicy = "compile_not_requested"
)

// RunBuildExecutionOrderingDecision resolves Go compilation strategy for a run.
type RunBuildExecutionOrderingDecision struct {
	GoCompilationOrderingPolicy GoCompilationOrderingPolicy
	RunCompileInParallel        bool
	RunCompileAfterBuildHooks   bool
}

// DeriveRunBuildExecutionOrderingDecision computes build execution ordering.
func DeriveRunBuildExecutionOrderingDecision(
	shouldRecompileGo bool,
	sequentialGoBuild bool,
) RunBuildExecutionOrderingDecision {
	if !shouldRecompileGo {
		return RunBuildExecutionOrderingDecision{
			GoCompilationOrderingPolicy: GoCompilationOrderingPolicyNotRequested,
		}
	}

	if sequentialGoBuild {
		return RunBuildExecutionOrderingDecision{
			GoCompilationOrderingPolicy: GoCompilationOrderingPolicyAfterBuildHooks,
			RunCompileAfterBuildHooks:   true,
		}
	}

	return RunBuildExecutionOrderingDecision{
		GoCompilationOrderingPolicy: GoCompilationOrderingPolicyConcurrentWithBuildHooks,
		RunCompileInParallel:        true,
	}
}

// RunCycleScope owns cancellable async work launched for one run cycle.
type RunCycleScope struct {
	CycleID uint64

	ExecutionContext       context.Context
	CancelExecutionContext context.CancelFunc

	asyncWorkGroup sync.WaitGroup
}

// NewRunCycleScope allocates a cycle scope with a cancellable execution
// context.
func NewRunCycleScope(cycleID uint64) *RunCycleScope {
	executionContext, cancelExecutionContext := context.WithCancel(
		context.Background(),
	)
	return &RunCycleScope{
		CycleID:                cycleID,
		ExecutionContext:       executionContext,
		CancelExecutionContext: cancelExecutionContext,
	}
}

// LaunchAsyncWork runs runAsyncWork under the cycle execution context.
func (scope *RunCycleScope) LaunchAsyncWork(
	runAsyncWork func(context.Context),
) {
	if scope == nil || runAsyncWork == nil {
		return
	}

	scope.asyncWorkGroup.Add(1)
	go func() {
		defer scope.asyncWorkGroup.Done()
		runAsyncWork(scope.ExecutionContext)
	}()
}

// CancelAndJoin cancels scope context and waits for launched work to complete.
func (scope *RunCycleScope) CancelAndJoin() {
	if scope == nil {
		return
	}

	if scope.CancelExecutionContext != nil {
		scope.CancelExecutionContext()
	}
	scope.asyncWorkGroup.Wait()
}

// RunIntent expresses what must happen during the next run cycle.
type RunIntent struct {
	RecompileGo     bool
	IsConfigRestart bool
}

// DeriveRunIntentFromRestartRequest converts restart intent to run intent.
func DeriveRunIntentFromRestartRequest(
	restartRequestForIntent RestartRequest,
) RunIntent {
	normalizedRestartRequest := NormalizeRestartRequest(restartRequestForIntent)
	return RunIntent(normalizedRestartRequest)
}

// RunLifecycleCommand is the command executed for a run-loop state.
type RunLifecycleCommand string

const (
	RunLifecycleCommandPrepareCycle        RunLifecycleCommand = "prepare_cycle"
	RunLifecycleCommandBuildCycle          RunLifecycleCommand = "build_cycle"
	RunLifecycleCommandAwaitBuildRetry     RunLifecycleCommand = "await_build_retry"
	RunLifecycleCommandStartRuntime        RunLifecycleCommand = "start_runtime"
	RunLifecycleCommandAwaitRestartRequest RunLifecycleCommand = "await_restart_request"
	RunLifecycleCommandCleanupForNextCycle RunLifecycleCommand = "cleanup_for_next_cycle"
)

// RunLifecycleCommandInput is the input for one lifecycle-command execution.
type RunLifecycleCommandInput struct {
	FirstRun         bool
	CurrentRunIntent RunIntent
}

// RunLifecycleCommandResult is the output of one lifecycle-command execution.
type RunLifecycleCommandResult struct {
	RunLifecycleEvent               RunLifecycleEvent
	UpdatedRunIntent                *RunIntent
	ShouldMarkFirstRunAsNotFirstRun bool
}

// RunLifecycleState defines one state of the devserver run loop.
type RunLifecycleState string

const (
	RunLifecycleStatePreparingCycle         RunLifecycleState = "preparing_cycle"
	RunLifecycleStateBuildingCycle          RunLifecycleState = "building_cycle"
	RunLifecycleStateAwaitingBuildRetry     RunLifecycleState = "awaiting_build_retry"
	RunLifecycleStateStartingRuntime        RunLifecycleState = "starting_runtime"
	RunLifecycleStateAwaitingRestart        RunLifecycleState = "awaiting_restart"
	RunLifecycleStateCleaningUpForNextCycle RunLifecycleState = "cleaning_up_for_next_cycle"
)

// RunLifecycleEvent defines one transition trigger inside the run loop state
// machine.
type RunLifecycleEvent string

const (
	RunLifecycleEventCyclePrepared             RunLifecycleEvent = "cycle_prepared"
	RunLifecycleEventBuildSucceeded            RunLifecycleEvent = "build_succeeded"
	RunLifecycleEventBuildFailed               RunLifecycleEvent = "build_failed"
	RunLifecycleEventBuildRetryRestartReceived RunLifecycleEvent = "build_retry_restart_received"
	RunLifecycleEventRuntimeStarted            RunLifecycleEvent = "runtime_started"
	RunLifecycleEventRestartRequestReceived    RunLifecycleEvent = "restart_request_received"
	RunLifecycleEventCleanupCompleted          RunLifecycleEvent = "cleanup_completed"
)

// TransitionLogger is the minimal logging capability needed during lifecycle
// transitions.
type TransitionLogger interface {
	Info(msg string, args ...any)
}

// DeriveRunLifecycleCommandForState maps state to lifecycle command.
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
		return "", fmt.Errorf(
			"unknown run lifecycle state: %q",
			currentRunLifecycleState,
		)
	}
}

// DeriveRunLifecycleStateAfterEvent computes the next state from the current
// state and transition event.
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
		if runLifecycleEventForTransition == RunLifecycleEventBuildSucceeded {
			return RunLifecycleStateStartingRuntime, nil
		}
		if runLifecycleEventForTransition == RunLifecycleEventBuildFailed {
			return RunLifecycleStateAwaitingBuildRetry, nil
		}

	case RunLifecycleStateAwaitingBuildRetry:
		if runLifecycleEventForTransition == RunLifecycleEventBuildRetryRestartReceived {
			return RunLifecycleStatePreparingCycle, nil
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

	return "", fmt.Errorf(
		"invalid run lifecycle transition: state=%q event=%q",
		currentRunLifecycleState,
		runLifecycleEventForTransition,
	)
}

// TransitionRunLifecycleState applies one transition and logs it when a logger
// is provided.
func TransitionRunLifecycleState(
	transitionLogger TransitionLogger,
	currentRunLifecycleState RunLifecycleState,
	runLifecycleEventForTransition RunLifecycleEvent,
	currentLifecycleCycleID uint64,
) (RunLifecycleState, error) {
	nextRunLifecycleState, err := DeriveRunLifecycleStateAfterEvent(
		currentRunLifecycleState,
		runLifecycleEventForTransition,
	)
	if err != nil {
		return "", err
	}

	if transitionLogger != nil {
		transitionLogger.Info(
			"run lifecycle transition",
			"cycle_id",
			currentLifecycleCycleID,
			"from",
			currentRunLifecycleState,
			"event",
			runLifecycleEventForTransition,
			"to",
			nextRunLifecycleState,
		)
	}

	return nextRunLifecycleState, nil
}
