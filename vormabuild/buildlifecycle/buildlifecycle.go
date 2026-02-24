// Package buildlifecycle centralizes build-lifecycle tracing and rollback
// execution helpers for build orchestration.
//
// Without this package, lifecycle transitions and rollback behavior become
// duplicated across full builds and fast-rebuild flows, which makes failure
// behavior inconsistent and harder to test.
package buildlifecycle

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/vormadev/vorma/internal/vormaruntime"
)

// LifecycleWorkflow identifies which build flow is executing.
type LifecycleWorkflow string

const (
	// WorkflowFullBuild tracks the full build orchestration flow.
	WorkflowFullBuild LifecycleWorkflow = "full-build"
	// WorkflowFastRouteRebuild tracks the fast route-only rebuild flow.
	WorkflowFastRouteRebuild LifecycleWorkflow = "fast-route-rebuild"
)

// LifecyclePhase identifies the current step in a build workflow.
type LifecyclePhase string

const (
	// PhaseIdle indicates no work has started yet.
	PhaseIdle LifecyclePhase = "idle"
	// PhaseStarted indicates orchestration has started.
	PhaseStarted LifecyclePhase = "started"
	// PhaseRuntimeStateInitialized indicates runtime state init completed.
	PhaseRuntimeStateInitialized LifecyclePhase = "runtime-state-initialized"
	// PhaseRoutesSynchronized indicates route sync completed.
	PhaseRoutesSynchronized LifecyclePhase = "routes-synchronized"
	// PhasePublicOutputCleaned indicates static output cleanup completed.
	PhasePublicOutputCleaned LifecyclePhase = "public-output-cleaned"
	// PhasePublicFileMapWritten indicates public file map generation completed.
	PhasePublicFileMapWritten LifecyclePhase = "public-file-map-written"
	// PhaseRouteArtifactsWritten indicates route artifact writes completed.
	PhaseRouteArtifactsWritten LifecyclePhase = "route-artifacts-written"
	// PhaseCompleted indicates workflow success.
	PhaseCompleted LifecyclePhase = "completed"
	// PhaseFailed indicates workflow failure.
	PhaseFailed LifecyclePhase = "failed"
)

// LifecycleTransitionRecord captures one lifecycle phase transition.
type LifecycleTransitionRecord struct {
	AttemptID string
	AtUTC     string
	Sequence  uint64
	Workflow  LifecycleWorkflow
	From      LifecyclePhase
	To        LifecyclePhase
	Reason    string
	Error     string
}

// LifecycleTransitionObserver observes lifecycle transitions.
type LifecycleTransitionObserver func(LifecycleTransitionRecord)

// LifecycleAttemptInput captures normalized key/value metadata for an attempt.
type LifecycleAttemptInput struct {
	Key   string
	Value string
}

// LifecycleRollbackDecision records whether rollback was required.
type LifecycleRollbackDecision string

const (
	// DecisionRequired indicates rollback was required by the failure path.
	DecisionRequired LifecycleRollbackDecision = "required"
	// DecisionNotRequired indicates rollback was not required.
	DecisionNotRequired LifecycleRollbackDecision = "not-required"
)

// LifecycleRollbackOutcome records what happened during rollback handling.
type LifecycleRollbackOutcome string

const (
	// OutcomeSucceeded indicates rollback completed successfully.
	OutcomeSucceeded LifecycleRollbackOutcome = "succeeded"
	// OutcomeFailed indicates rollback failed.
	OutcomeFailed LifecycleRollbackOutcome = "failed"
	// OutcomeNotRequired indicates rollback was not needed.
	OutcomeNotRequired LifecycleRollbackOutcome = "not-required"
	// OutcomeNotAttempted indicates rollback was skipped.
	OutcomeNotAttempted LifecycleRollbackOutcome = "not-attempted"
)

// LifecycleRollbackRecord captures one rollback decision event.
type LifecycleRollbackRecord struct {
	AttemptID string
	AtUTC     string
	Sequence  uint64
	Workflow  LifecycleWorkflow
	Decision  LifecycleRollbackDecision
	Outcome   LifecycleRollbackOutcome
	Reason    string
	Error     string
}

// LifecycleStateMachineOptions controls lifecycle machine construction.
type LifecycleStateMachineOptions struct {
	TransitionObserver LifecycleTransitionObserver
	AttemptInputs      []LifecycleAttemptInput
	Dependencies       LifecycleStateMachineDependencies
}

// LifecycleStateMachineDependencies injects deterministic behavior for tests.
type LifecycleStateMachineDependencies struct {
	NowUTC        func() time.Time
	NextAttemptID func(LifecycleWorkflow) string
}

var lifecycleTraceAttemptSequence atomic.Uint64

func defaultLifecycleStateMachineDependencies() LifecycleStateMachineDependencies {
	return LifecycleStateMachineDependencies{
		NowUTC: time.Now().UTC,
		NextAttemptID: func(workflow LifecycleWorkflow) string {
			attemptSequence := lifecycleTraceAttemptSequence.Add(1)
			return fmt.Sprintf("%s-attempt-%d", workflow, attemptSequence)
		},
	}
}

func normalizeLifecycleStateMachineDependencies(
	dependencies LifecycleStateMachineDependencies,
) LifecycleStateMachineDependencies {
	defaultDependencies := defaultLifecycleStateMachineDependencies()
	if dependencies.NowUTC == nil {
		dependencies.NowUTC = defaultDependencies.NowUTC
	}
	if dependencies.NextAttemptID == nil {
		dependencies.NextAttemptID = defaultDependencies.NextAttemptID
	}
	return dependencies
}

// LifecycleStateMachine enforces allowed transitions for one workflow attempt.
type LifecycleStateMachine struct {
	workflow           LifecycleWorkflow
	dependencies       LifecycleStateMachineDependencies
	attemptID          string
	attemptInputs      []LifecycleAttemptInput
	currentPhase       LifecyclePhase
	transitionSeq      uint64
	rollbackSeq        uint64
	logger             *slog.Logger
	allowedTransitions map[LifecyclePhase]map[LifecyclePhase]struct{}
	transitionObserver LifecycleTransitionObserver
	transitionHistory  []LifecycleTransitionRecord
	rollbackHistory    []LifecycleRollbackRecord
}

// NewLifecycleStateMachine creates a lifecycle machine with basic options.
func NewLifecycleStateMachine(
	workflow LifecycleWorkflow,
	logger *slog.Logger,
	transitionObserver LifecycleTransitionObserver,
) (*LifecycleStateMachine, error) {
	return NewLifecycleStateMachineWithOptions(
		workflow,
		logger,
		LifecycleStateMachineOptions{
			TransitionObserver: transitionObserver,
		},
	)
}

// NewLifecycleStateMachineWithOptions creates a lifecycle machine.
func NewLifecycleStateMachineWithOptions(
	workflow LifecycleWorkflow,
	logger *slog.Logger,
	options LifecycleStateMachineOptions,
) (*LifecycleStateMachine, error) {
	allowedTransitions, err := lifecycleAllowedTransitions(workflow)
	if err != nil {
		return nil, err
	}

	dependencies := normalizeLifecycleStateMachineDependencies(
		options.Dependencies,
	)
	attemptInputs := normalizeLifecycleAttemptInputs(options.AttemptInputs)
	attemptID := strings.TrimSpace(dependencies.NextAttemptID(workflow))
	if attemptID == "" {
		return nil, errors.New("build lifecycle attempt ID is required")
	}

	return &LifecycleStateMachine{
		workflow:           workflow,
		dependencies:       dependencies,
		attemptID:          attemptID,
		attemptInputs:      attemptInputs,
		currentPhase:       PhaseIdle,
		logger:             logger,
		allowedTransitions: allowedTransitions,
		transitionObserver: options.TransitionObserver,
	}, nil
}

// CurrentPhase returns the machine's current phase.
func (machine *LifecycleStateMachine) CurrentPhase() LifecyclePhase {
	return machine.currentPhase
}

// AttemptID returns the machine's immutable attempt identifier.
func (machine *LifecycleStateMachine) AttemptID() string {
	return machine.attemptID
}

// AttemptInputs returns normalized attempt metadata in sorted key order.
func (machine *LifecycleStateMachine) AttemptInputs() []LifecycleAttemptInput {
	if len(machine.attemptInputs) == 0 {
		return nil
	}
	return append(
		[]LifecycleAttemptInput(nil),
		machine.attemptInputs...,
	)
}

// TransitionHistory returns a copy of transition history.
func (machine *LifecycleStateMachine) TransitionHistory() []LifecycleTransitionRecord {
	if len(machine.transitionHistory) == 0 {
		return nil
	}
	return append(
		[]LifecycleTransitionRecord(nil),
		machine.transitionHistory...,
	)
}

// RollbackHistory returns a copy of rollback decision history.
func (machine *LifecycleStateMachine) RollbackHistory() []LifecycleRollbackRecord {
	if len(machine.rollbackHistory) == 0 {
		return nil
	}
	return append(
		[]LifecycleRollbackRecord(nil),
		machine.rollbackHistory...,
	)
}

// TransitionTo transitions to the requested phase.
func (machine *LifecycleStateMachine) TransitionTo(
	nextPhase LifecyclePhase,
	reason string,
) error {
	return machine.transition(nextPhase, reason, "")
}

// TransitionToFailed transitions to the failure phase and captures error text.
func (machine *LifecycleStateMachine) TransitionToFailed(
	reason string,
	buildErr error,
) error {
	if buildErr == nil {
		return errors.New(
			"build error is required for failed lifecycle transition",
		)
	}
	return machine.transition(
		nextPhaseForFailedTransition(),
		reason,
		buildErr.Error(),
	)
}

// RecordRollback records a rollback decision/outcome event.
func (machine *LifecycleStateMachine) RecordRollback(
	decision LifecycleRollbackDecision,
	outcome LifecycleRollbackOutcome,
	reason string,
	rollbackErr error,
) error {
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return errors.New("rollback reason is required")
	}

	machine.rollbackSeq++
	record := LifecycleRollbackRecord{
		AttemptID: machine.attemptID,
		AtUTC: machine.dependencies.NowUTC().
			Format(time.RFC3339Nano),
		Sequence: machine.rollbackSeq,
		Workflow: machine.workflow,
		Decision: decision,
		Outcome:  outcome,
		Reason:   trimmedReason,
	}
	if rollbackErr != nil {
		record.Error = rollbackErr.Error()
	}

	machine.rollbackHistory = append(
		machine.rollbackHistory,
		record,
	)

	if machine.logger != nil {
		machine.logger.Debug(
			"Vorma build lifecycle rollback decision",
			"attempt_id",
			record.AttemptID,
			"at_utc",
			record.AtUTC,
			"seq",
			record.Sequence,
			"workflow",
			record.Workflow,
			"decision",
			record.Decision,
			"outcome",
			record.Outcome,
			"reason",
			record.Reason,
			"error",
			record.Error,
		)
	}

	return nil
}

func (machine *LifecycleStateMachine) transition(
	nextPhase LifecyclePhase,
	reason string,
	errorText string,
) error {
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return errors.New("lifecycle transition reason is required")
	}

	currentPhase := machine.currentPhase
	allowedNextPhases, hasPhase := machine.allowedTransitions[currentPhase]
	if !hasPhase {
		return fmt.Errorf(
			"workflow %q is terminal at phase %q and cannot transition to %q",
			machine.workflow,
			currentPhase,
			nextPhase,
		)
	}

	if _, transitionAllowed := allowedNextPhases[nextPhase]; !transitionAllowed {
		return fmt.Errorf(
			"invalid workflow %q lifecycle transition %q -> %q (allowed: %s)",
			machine.workflow,
			currentPhase,
			nextPhase,
			strings.Join(
				sortedLifecyclePhaseNames(allowedNextPhases),
				", ",
			),
		)
	}

	machine.transitionSeq++
	machine.currentPhase = nextPhase

	record := LifecycleTransitionRecord{
		AttemptID: machine.attemptID,
		AtUTC: machine.dependencies.NowUTC().
			Format(time.RFC3339Nano),
		Sequence: machine.transitionSeq,
		Workflow: machine.workflow,
		From:     currentPhase,
		To:       nextPhase,
		Reason:   trimmedReason,
		Error:    errorText,
	}

	machine.transitionHistory = append(
		machine.transitionHistory,
		record,
	)

	if machine.logger != nil {
		machine.logger.Debug(
			"Vorma build lifecycle transition",
			"attempt_id",
			record.AttemptID,
			"at_utc",
			record.AtUTC,
			"seq",
			record.Sequence,
			"workflow",
			record.Workflow,
			"from",
			record.From,
			"to",
			record.To,
			"reason",
			record.Reason,
			"error",
			record.Error,
		)
	}
	if machine.transitionObserver != nil {
		machine.transitionObserver(record)
	}
	return nil
}

func normalizeLifecycleAttemptInputs(
	attemptInputs []LifecycleAttemptInput,
) []LifecycleAttemptInput {
	if len(attemptInputs) == 0 {
		return nil
	}

	normalizedAttemptInputs := make(
		[]LifecycleAttemptInput,
		0,
		len(attemptInputs),
	)
	for _, attemptInput := range attemptInputs {
		trimmedKey := strings.TrimSpace(attemptInput.Key)
		if trimmedKey == "" {
			continue
		}

		normalizedAttemptInputs = append(
			normalizedAttemptInputs,
			LifecycleAttemptInput{
				Key:   trimmedKey,
				Value: strings.TrimSpace(attemptInput.Value),
			},
		)
	}
	sort.Slice(normalizedAttemptInputs, func(i int, j int) bool {
		return normalizedAttemptInputs[i].Key < normalizedAttemptInputs[j].Key
	})
	return normalizedAttemptInputs
}

func lifecycleAllowedTransitions(
	workflow LifecycleWorkflow,
) (map[LifecyclePhase]map[LifecyclePhase]struct{}, error) {
	switch workflow {
	case WorkflowFullBuild:
		return map[LifecyclePhase]map[LifecyclePhase]struct{}{
			PhaseIdle: {
				PhaseStarted: {},
			},
			PhaseStarted: {
				PhaseRuntimeStateInitialized: {},
				PhaseFailed:                  {},
			},
			PhaseRuntimeStateInitialized: {
				PhaseRoutesSynchronized: {},
				PhaseFailed:             {},
			},
			PhaseRoutesSynchronized: {
				PhasePublicOutputCleaned: {},
				PhaseFailed:              {},
			},
			PhasePublicOutputCleaned: {
				PhasePublicFileMapWritten: {},
				PhaseFailed:               {},
			},
			PhasePublicFileMapWritten: {
				PhaseRouteArtifactsWritten: {},
				PhaseFailed:                {},
			},
			PhaseRouteArtifactsWritten: {
				PhaseCompleted: {},
				PhaseFailed:    {},
			},
		}, nil
	case WorkflowFastRouteRebuild:
		return map[LifecyclePhase]map[LifecyclePhase]struct{}{
			PhaseIdle: {
				PhaseStarted: {},
			},
			PhaseStarted: {
				PhaseRoutesSynchronized: {},
				PhaseFailed:             {},
			},
			PhaseRoutesSynchronized: {
				PhaseRouteArtifactsWritten: {},
				PhaseFailed:                {},
			},
			PhaseRouteArtifactsWritten: {
				PhaseCompleted: {},
				PhaseFailed:    {},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unknown build lifecycle workflow %q", workflow)
	}
}

func nextPhaseForFailedTransition() LifecyclePhase {
	return PhaseFailed
}

func sortedLifecyclePhaseNames(
	allowedPhases map[LifecyclePhase]struct{},
) []string {
	phaseNames := make([]string, 0, len(allowedPhases))
	for allowedPhase := range allowedPhases {
		phaseNames = append(phaseNames, string(allowedPhase))
	}
	sort.Strings(phaseNames)
	return phaseNames
}

// RollbackTransactionOptions configures operation+rollback orchestration.
type RollbackTransactionOptions struct {
	Run                          func() error
	RollbackOnFailure            func() error
	RollbackErrorContext         string
	LogRollbackFailureAfterPanic func(error)
}

// RunWithRollbackOnFailureAndPanic runs options.Run and executes rollback when
// run fails or panics. It re-panics after panic handling.
func RunWithRollbackOnFailureAndPanic(
	options RollbackTransactionOptions,
) (operationErr error) {
	if options.Run == nil {
		return errors.New("rollback transaction run step is required")
	}

	defer func() {
		recoveredPanicValue := recover()
		if recoveredPanicValue == nil {
			return
		}

		rollbackPanicValue, rollbackErr := runRollbackIfConfigured(
			options.RollbackOnFailure,
		)
		if rollbackErr != nil && options.LogRollbackFailureAfterPanic != nil {
			options.LogRollbackFailureAfterPanic(rollbackErr)
		}
		if rollbackPanicValue != nil &&
			options.LogRollbackFailureAfterPanic != nil {
			options.LogRollbackFailureAfterPanic(
				fmt.Errorf("rollback panic: %v", rollbackPanicValue),
			)
		}

		panic(recoveredPanicValue)
	}()

	operationErr = options.Run()
	if operationErr == nil {
		return nil
	}

	rollbackPanicValue, rollbackErr := runRollbackIfConfigured(
		options.RollbackOnFailure,
	)
	if rollbackPanicValue != nil {
		panic(rollbackPanicValue)
	}
	if rollbackErr == nil {
		return operationErr
	}

	if options.RollbackErrorContext != "" {
		rollbackErr = fmt.Errorf(
			"%s: %w",
			options.RollbackErrorContext,
			rollbackErr,
		)
	}

	return errors.Join(operationErr, rollbackErr)
}

func runRollbackIfConfigured(
	rollbackStep func() error,
) (rollbackPanicValue any, rollbackErr error) {
	if rollbackStep == nil {
		return nil, nil
	}

	defer func() {
		recoveredPanicValue := recover()
		if recoveredPanicValue != nil {
			rollbackPanicValue = recoveredPanicValue
		}
	}()

	rollbackErr = rollbackStep()
	return nil, rollbackErr
}

// RuntimeStateRoutePathsUpdateMode controls how route paths are mutated.
type RuntimeStateRoutePathsUpdateMode uint8

const (
	// RuntimeStateRoutePathsUpdateModeNoPathMutation leaves paths unchanged.
	RuntimeStateRoutePathsUpdateModeNoPathMutation RuntimeStateRoutePathsUpdateMode = iota
	// RuntimeStateRoutePathsUpdateModeReplaceParsedPathsForInit replaces paths.
	RuntimeStateRoutePathsUpdateModeReplaceParsedPathsForInit
	// RuntimeStateRoutePathsUpdateModeSyncFromDevReload syncs paths incrementally.
	RuntimeStateRoutePathsUpdateModeSyncFromDevReload
)

// RuntimeStateCommitInput describes one runtime-state commit operation.
type RuntimeStateCommitInput struct {
	ShouldCommitIsDev             bool
	IsDev                         bool
	ShouldCommitBuildID           bool
	BuildID                       string
	ShouldCommitRouteManifestFile bool
	RouteManifestFile             string
	RoutePaths                    map[string]*vormaruntime.Path
	RoutePathsUpdateMode          RuntimeStateRoutePathsUpdateMode
	ShouldRebuildNestedRouter     bool
}

// CommitRuntimeState applies a runtime commit under the runtime write lock.
func CommitRuntimeState(
	v *vormaruntime.Vorma,
	runtimeStateCommitInput RuntimeStateCommitInput,
) {
	v.WithLock(func(l *vormaruntime.LockedVorma) {
		CommitRuntimeStateWithLock(l, runtimeStateCommitInput)
	})
}

// CommitRuntimeStateWithLock applies a runtime commit while holding the lock.
func CommitRuntimeStateWithLock(
	l *vormaruntime.LockedVorma,
	runtimeStateCommitInput RuntimeStateCommitInput,
) {
	if runtimeStateCommitInput.ShouldCommitIsDev {
		l.SetIsDev(runtimeStateCommitInput.IsDev)
	}

	switch runtimeStateCommitInput.RoutePathsUpdateMode {
	case RuntimeStateRoutePathsUpdateModeNoPathMutation:
		// no-op
	case RuntimeStateRoutePathsUpdateModeReplaceParsedPathsForInit:
		l.Routes().ReplaceParsedPathsForInit(
			runtimeStateCommitInput.RoutePaths,
			runtimeStateCommitInput.ShouldRebuildNestedRouter,
		)
	case RuntimeStateRoutePathsUpdateModeSyncFromDevReload:
		l.Routes().SyncFromDevReload(runtimeStateCommitInput.RoutePaths)
	default:
		panic(fmt.Sprintf(
			"unsupported runtime state route paths update mode: %d",
			runtimeStateCommitInput.RoutePathsUpdateMode,
		))
	}

	if runtimeStateCommitInput.ShouldCommitBuildID {
		l.SetBuildID(runtimeStateCommitInput.BuildID)
	}

	if runtimeStateCommitInput.ShouldCommitRouteManifestFile {
		l.SetRouteManifestFile(runtimeStateCommitInput.RouteManifestFile)
	}
}

func shouldRebuildNestedRouterFromCurrentRuntimeState(
	l *vormaruntime.LockedVorma,
) bool {
	return l.Vorma().LoadersRouter() != nil &&
		l.Vorma().LoadersRouter().NestedRouter != nil
}

// CurrentBuildIDWithReadLock reads the current build ID under read lock.
func CurrentBuildIDWithReadLock(v *vormaruntime.Vorma) string {
	var currentBuildID string
	v.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
		currentBuildID = l.BuildID()
	})
	return currentBuildID
}

// ShouldRestoreRuntimeStateSnapshotForAttemptBuildID guards stale rollbacks.
func ShouldRestoreRuntimeStateSnapshotForAttemptBuildID(
	currentBuildID string,
	currentAttemptCommittedBuildID string,
) bool {
	if currentAttemptCommittedBuildID == "" {
		return true
	}
	return currentBuildID == currentAttemptCommittedBuildID
}

// BuildRuntimeStateSnapshot captures full runtime state used by build flows.
type BuildRuntimeStateSnapshot struct {
	IsDev                  bool
	RouteBuildRuntimeState RouteBuildRuntimeStateSnapshot
}

// RouteBuildRuntimeStateSnapshot captures route/build-id/manifest state.
type RouteBuildRuntimeStateSnapshot struct {
	Paths             map[string]*vormaruntime.Path
	BuildID           string
	RouteManifestFile string
}

// BuildRuntimeStateReader reads full build runtime state.
type BuildRuntimeStateReader interface {
	IsDev() bool
	Paths() map[string]*vormaruntime.Path
	BuildID() string
	RouteManifestFile() string
}

// RouteBuildRuntimeStateReader reads route build runtime state.
type RouteBuildRuntimeStateReader interface {
	Paths() map[string]*vormaruntime.Path
	BuildID() string
	RouteManifestFile() string
}

// CaptureBuildRuntimeState snapshots full build runtime state.
func CaptureBuildRuntimeState(
	l BuildRuntimeStateReader,
) BuildRuntimeStateSnapshot {
	return BuildRuntimeStateSnapshot{
		IsDev:                  l.IsDev(),
		RouteBuildRuntimeState: CaptureRouteBuildRuntimeState(l),
	}
}

// CaptureRouteBuildRuntimeState snapshots route build runtime state.
func CaptureRouteBuildRuntimeState(
	l RouteBuildRuntimeStateReader,
) RouteBuildRuntimeStateSnapshot {
	return RouteBuildRuntimeStateSnapshot{
		Paths:             CloneRouteBuildRuntimePathsMap(l.Paths()),
		BuildID:           l.BuildID(),
		RouteManifestFile: l.RouteManifestFile(),
	}
}

// RestoreBuildRuntimeState restores full build runtime state from snapshot.
func RestoreBuildRuntimeState(
	l *vormaruntime.LockedVorma,
	snapshot BuildRuntimeStateSnapshot,
) {
	CommitRuntimeStateWithLock(
		l,
		RuntimeStateCommitInput{
			ShouldCommitIsDev:             true,
			IsDev:                         snapshot.IsDev,
			ShouldCommitBuildID:           true,
			BuildID:                       snapshot.RouteBuildRuntimeState.BuildID,
			ShouldCommitRouteManifestFile: true,
			RouteManifestFile:             snapshot.RouteBuildRuntimeState.RouteManifestFile,
			RoutePaths:                    snapshot.RouteBuildRuntimeState.Paths,
			RoutePathsUpdateMode:          RuntimeStateRoutePathsUpdateModeReplaceParsedPathsForInit,
			ShouldRebuildNestedRouter: shouldRebuildNestedRouterFromCurrentRuntimeState(
				l,
			),
		},
	)
}

// RestoreRouteBuildRuntimeState restores route-focused runtime state snapshot.
func RestoreRouteBuildRuntimeState(
	l *vormaruntime.LockedVorma,
	snapshot RouteBuildRuntimeStateSnapshot,
) {
	CommitRuntimeStateWithLock(
		l,
		RuntimeStateCommitInput{
			ShouldCommitBuildID:           true,
			BuildID:                       snapshot.BuildID,
			ShouldCommitRouteManifestFile: true,
			RouteManifestFile:             snapshot.RouteManifestFile,
			RoutePaths:                    snapshot.Paths,
			RoutePathsUpdateMode:          RuntimeStateRoutePathsUpdateModeReplaceParsedPathsForInit,
			ShouldRebuildNestedRouter: shouldRebuildNestedRouterFromCurrentRuntimeState(
				l,
			),
		},
	)
}

// BuildRuntimeStateSnapshotMatches compares runtime state to a full snapshot.
func BuildRuntimeStateSnapshotMatches(
	l BuildRuntimeStateReader,
	snapshot BuildRuntimeStateSnapshot,
) bool {
	if l.IsDev() != snapshot.IsDev {
		return false
	}
	return RouteBuildRuntimeStateSnapshotMatches(
		l,
		snapshot.RouteBuildRuntimeState,
	)
}

// RouteBuildRuntimeStateSnapshotMatches compares route runtime state to snapshot.
func RouteBuildRuntimeStateSnapshotMatches(
	l RouteBuildRuntimeStateReader,
	snapshot RouteBuildRuntimeStateSnapshot,
) bool {
	if l.BuildID() != snapshot.BuildID {
		return false
	}
	if l.RouteManifestFile() != snapshot.RouteManifestFile {
		return false
	}
	return RouteBuildRuntimePathsMapMatches(l.Paths(), snapshot.Paths)
}

// RouteBuildRuntimePathsMapMatches compares route path maps deeply.
func RouteBuildRuntimePathsMapMatches(
	currentPaths map[string]*vormaruntime.Path,
	expectedPaths map[string]*vormaruntime.Path,
) bool {
	if len(currentPaths) != len(expectedPaths) {
		return false
	}
	for routePattern, currentPath := range currentPaths {
		expectedPath, hasExpectedPath := expectedPaths[routePattern]
		if !hasExpectedPath {
			return false
		}
		if !RouteBuildRuntimePathMatches(currentPath, expectedPath) {
			return false
		}
	}
	return true
}

// RouteBuildRuntimePathMatches compares two runtime path entries deeply.
func RouteBuildRuntimePathMatches(
	currentPath *vormaruntime.Path,
	expectedPath *vormaruntime.Path,
) bool {
	if currentPath == nil || expectedPath == nil {
		return currentPath == expectedPath
	}
	if currentPath.OriginalPattern != expectedPath.OriginalPattern {
		return false
	}
	if currentPath.SrcPath != expectedPath.SrcPath {
		return false
	}
	if currentPath.ExportKey != expectedPath.ExportKey {
		return false
	}
	if currentPath.ErrorExportKey != expectedPath.ErrorExportKey {
		return false
	}
	if currentPath.OutPath != expectedPath.OutPath {
		return false
	}
	if len(currentPath.Deps) != len(expectedPath.Deps) {
		return false
	}
	for depIndex := range currentPath.Deps {
		if currentPath.Deps[depIndex] != expectedPath.Deps[depIndex] {
			return false
		}
	}
	return true
}

// CloneRouteBuildRuntimePathsMap deep-clones a runtime path map.
func CloneRouteBuildRuntimePathsMap(
	paths map[string]*vormaruntime.Path,
) map[string]*vormaruntime.Path {
	if paths == nil {
		return nil
	}

	clonedPaths := make(map[string]*vormaruntime.Path, len(paths))
	for pattern, path := range paths {
		clonedPaths[pattern] = CloneRouteBuildRuntimePath(path)
	}
	return clonedPaths
}

// CloneRouteBuildRuntimePath deep-clones a runtime path value.
func CloneRouteBuildRuntimePath(path *vormaruntime.Path) *vormaruntime.Path {
	if path == nil {
		return nil
	}

	clonedPath := *path
	if path.Deps != nil {
		clonedPath.Deps = append([]string(nil), path.Deps...)
	}
	return &clonedPath
}

// RouteBuildRuntimeStateSnapshotIsCurrent checks stale snapshot guard by build ID.
func RouteBuildRuntimeStateSnapshotIsCurrent(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot RouteBuildRuntimeStateSnapshot,
) bool {
	return CurrentBuildIDWithReadLock(v) == runtimeStateSnapshot.BuildID
}

// ShouldCommitRouteManifestFileForRuntimeState guards manifest commit by build ID.
func ShouldCommitRouteManifestFileForRuntimeState(
	currentBuildID string,
	expectedBuildID string,
) bool {
	return currentBuildID == expectedBuildID
}

// PostRouteSyncHook executes after route sync commits state.
type PostRouteSyncHook func(*vormaruntime.Vorma) error

// RouteSyncExecutionOptions configures parse/sync/build-id route sync flow.
type RouteSyncExecutionOptions struct {
	ParseClientRoutes          func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error)
	GenerateBuildID            func() (string, error)
	ParseClientRoutesErrorText string
	PostSyncHook               PostRouteSyncHook
}

// PrepareParsedRouteSyncInput parses routes and optionally generates build ID.
func PrepareParsedRouteSyncInput(
	v *vormaruntime.Vorma,
	parseClientRoutes func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error),
	generateBuildID func() (string, error),
	parseClientRoutesErrorContext string,
) (map[string]*vormaruntime.Path, string, error) {
	clientPaths, err := parseClientRoutes(v)
	if err != nil {
		if parseClientRoutesErrorContext != "" {
			return nil, "", fmt.Errorf(
				"%s: %w",
				parseClientRoutesErrorContext,
				err,
			)
		}
		return nil, "", err
	}

	buildID := ""
	if generateBuildID != nil {
		buildID, err = generateBuildID()
		if err != nil {
			return nil, "", err
		}
	}

	return clientPaths, buildID, nil
}

// RunRouteSyncExecution runs parse/build-id/sync/post-sync in order.
func RunRouteSyncExecution(
	v *vormaruntime.Vorma,
	options RouteSyncExecutionOptions,
) error {
	if options.ParseClientRoutes == nil {
		return errors.New("route sync parse function is required")
	}

	clientPaths, buildID, err := PrepareParsedRouteSyncInput(
		v,
		options.ParseClientRoutes,
		options.GenerateBuildID,
		options.ParseClientRoutesErrorText,
	)
	if err != nil {
		return err
	}

	return SyncClientRoutesFromParsedPathsWithLock(
		v,
		clientPaths,
		buildID,
		options.PostSyncHook,
	)
}

// SyncClientRoutesFromParsedPathsWithLock syncs parsed routes under lock and
// rolls state back when post-sync hooks fail.
func SyncClientRoutesFromParsedPathsWithLock(
	v *vormaruntime.Vorma,
	clientPaths map[string]*vormaruntime.Path,
	buildID string,
	postSyncHook PostRouteSyncHook,
) error {
	var previousRuntimeState BuildRuntimeStateSnapshot
	var currentAttemptCommittedBuildID string
	return RunWithRollbackOnFailureAndPanic(
		RollbackTransactionOptions{
			Run: func() error {
				v.WithLock(func(l *vormaruntime.LockedVorma) {
					previousRuntimeState = CaptureBuildRuntimeState(l)
					CommitRuntimeStateWithLock(
						l,
						RuntimeStateCommitInput{
							ShouldCommitBuildID:  buildID != "",
							BuildID:              buildID,
							RoutePaths:           clientPaths,
							RoutePathsUpdateMode: RuntimeStateRoutePathsUpdateModeSyncFromDevReload,
						},
					)
					currentAttemptCommittedBuildID = l.BuildID()
				})

				if postSyncHook != nil {
					return postSyncHook(v)
				}
				return nil
			},
			RollbackOnFailure: func() error {
				v.WithLock(func(l *vormaruntime.LockedVorma) {
					RollbackRouteSyncStateAfterPostSyncFailure(
						l,
						previousRuntimeState,
						currentAttemptCommittedBuildID,
					)
				})
				return nil
			},
		},
	)
}

// RollbackRouteSyncStateAfterPostSyncFailure restores prior state when allowed.
func RollbackRouteSyncStateAfterPostSyncFailure(
	l *vormaruntime.LockedVorma,
	previousRuntimeState BuildRuntimeStateSnapshot,
	currentAttemptCommittedBuildID string,
) {
	if !ShouldRollbackRouteSyncStateAfterPostSyncFailure(
		l.BuildID(),
		currentAttemptCommittedBuildID,
	) {
		return
	}
	RestoreBuildRuntimeState(l, previousRuntimeState)
}

// ShouldRollbackRouteSyncStateAfterPostSyncFailure applies stale-rollback guard.
func ShouldRollbackRouteSyncStateAfterPostSyncFailure(
	currentBuildID string,
	currentAttemptCommittedBuildID string,
) bool {
	return ShouldRestoreRuntimeStateSnapshotForAttemptBuildID(
		currentBuildID,
		currentAttemptCommittedBuildID,
	)
}
