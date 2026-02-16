package vormabuild

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

type buildLifecycleWorkflow string

const (
	buildLifecycleWorkflowFullBuild        buildLifecycleWorkflow = "full-build"
	buildLifecycleWorkflowFastRouteRebuild buildLifecycleWorkflow = "fast-route-rebuild"
)

type buildLifecyclePhase string

const (
	buildLifecyclePhaseIdle                    buildLifecyclePhase = "idle"
	buildLifecyclePhaseStarted                 buildLifecyclePhase = "started"
	buildLifecyclePhaseRuntimeStateInitialized buildLifecyclePhase = "runtime-state-initialized"
	buildLifecyclePhaseRoutesSynchronized      buildLifecyclePhase = "routes-synchronized"
	buildLifecyclePhasePublicOutputCleaned     buildLifecyclePhase = "public-output-cleaned"
	buildLifecyclePhasePublicFileMapWritten    buildLifecyclePhase = "public-file-map-written"
	buildLifecyclePhaseRouteArtifactsWritten   buildLifecyclePhase = "route-artifacts-written"
	buildLifecyclePhaseCompleted               buildLifecyclePhase = "completed"
	buildLifecyclePhaseFailed                  buildLifecyclePhase = "failed"
)

type buildLifecycleTransitionRecord struct {
	AttemptID string
	AtUTC     string
	Sequence  uint64
	Workflow  buildLifecycleWorkflow
	From      buildLifecyclePhase
	To        buildLifecyclePhase
	Reason    string
	Error     string
}

type buildLifecycleTransitionObserver func(buildLifecycleTransitionRecord)

type buildLifecycleAttemptInput struct {
	Key   string
	Value string
}

type buildLifecycleRollbackDecision string

const (
	buildLifecycleRollbackDecisionRequired    buildLifecycleRollbackDecision = "required"
	buildLifecycleRollbackDecisionNotRequired buildLifecycleRollbackDecision = "not-required"
)

type buildLifecycleRollbackOutcome string

const (
	buildLifecycleRollbackOutcomeSucceeded    buildLifecycleRollbackOutcome = "succeeded"
	buildLifecycleRollbackOutcomeFailed       buildLifecycleRollbackOutcome = "failed"
	buildLifecycleRollbackOutcomeNotRequired  buildLifecycleRollbackOutcome = "not-required"
	buildLifecycleRollbackOutcomeNotAttempted buildLifecycleRollbackOutcome = "not-attempted"
)

type buildLifecycleRollbackRecord struct {
	AttemptID string
	AtUTC     string
	Sequence  uint64
	Workflow  buildLifecycleWorkflow
	Decision  buildLifecycleRollbackDecision
	Outcome   buildLifecycleRollbackOutcome
	Reason    string
	Error     string
}

type buildLifecycleStateMachineOptions struct {
	transitionObserver buildLifecycleTransitionObserver
	attemptInputs      []buildLifecycleAttemptInput
	dependencies       buildLifecycleStateMachineDependencies
}

type buildLifecycleStateMachineDependencies struct {
	nowUTC        func() time.Time
	nextAttemptID func(buildLifecycleWorkflow) string
}

var buildLifecycleTraceAttemptSequence atomic.Uint64

func defaultBuildLifecycleStateMachineDependencies() buildLifecycleStateMachineDependencies {
	return buildLifecycleStateMachineDependencies{
		nowUTC: time.Now().UTC,
		nextAttemptID: func(workflow buildLifecycleWorkflow) string {
			attemptSequence := buildLifecycleTraceAttemptSequence.Add(1)
			return fmt.Sprintf("%s-attempt-%d", workflow, attemptSequence)
		},
	}
}

func normalizeBuildLifecycleStateMachineDependencies(
	dependencies buildLifecycleStateMachineDependencies,
) buildLifecycleStateMachineDependencies {
	defaultDependencies := defaultBuildLifecycleStateMachineDependencies()
	if dependencies.nowUTC == nil {
		dependencies.nowUTC = defaultDependencies.nowUTC
	}
	if dependencies.nextAttemptID == nil {
		dependencies.nextAttemptID = defaultDependencies.nextAttemptID
	}
	return dependencies
}

type buildLifecycleStateMachine struct {
	workflow           buildLifecycleWorkflow
	dependencies       buildLifecycleStateMachineDependencies
	attemptID          string
	attemptInputs      []buildLifecycleAttemptInput
	currentPhase       buildLifecyclePhase
	transitionSeq      uint64
	rollbackSeq        uint64
	logger             *slog.Logger
	allowedTransitions map[buildLifecyclePhase]map[buildLifecyclePhase]struct{}
	transitionObserver buildLifecycleTransitionObserver
	transitionHistory  []buildLifecycleTransitionRecord
	rollbackHistory    []buildLifecycleRollbackRecord
}

func newBuildLifecycleStateMachine(
	workflow buildLifecycleWorkflow,
	logger *slog.Logger,
	transitionObserver buildLifecycleTransitionObserver,
) (*buildLifecycleStateMachine, error) {
	return newBuildLifecycleStateMachineWithOptions(
		workflow,
		logger,
		buildLifecycleStateMachineOptions{
			transitionObserver: transitionObserver,
		},
	)
}

func newBuildLifecycleStateMachineWithOptions(
	workflow buildLifecycleWorkflow,
	logger *slog.Logger,
	options buildLifecycleStateMachineOptions,
) (*buildLifecycleStateMachine, error) {
	allowedTransitions, err := buildLifecycleAllowedTransitions(workflow)
	if err != nil {
		return nil, err
	}

	dependencies := normalizeBuildLifecycleStateMachineDependencies(options.dependencies)
	attemptInputs := normalizeBuildLifecycleAttemptInputs(options.attemptInputs)
	attemptID := strings.TrimSpace(dependencies.nextAttemptID(workflow))
	if attemptID == "" {
		return nil, errors.New("build lifecycle attempt ID is required")
	}

	return &buildLifecycleStateMachine{
		workflow:           workflow,
		dependencies:       dependencies,
		attemptID:          attemptID,
		attemptInputs:      attemptInputs,
		currentPhase:       buildLifecyclePhaseIdle,
		logger:             logger,
		allowedTransitions: allowedTransitions,
		transitionObserver: options.transitionObserver,
	}, nil
}

func (buildLifecycleMachine *buildLifecycleStateMachine) currentPhaseSnapshot() buildLifecyclePhase {
	return buildLifecycleMachine.currentPhase
}

func (buildLifecycleMachine *buildLifecycleStateMachine) attemptIDSnapshot() string {
	return buildLifecycleMachine.attemptID
}

func (buildLifecycleMachine *buildLifecycleStateMachine) attemptInputsSnapshot() []buildLifecycleAttemptInput {
	if len(buildLifecycleMachine.attemptInputs) == 0 {
		return nil
	}
	return append([]buildLifecycleAttemptInput(nil), buildLifecycleMachine.attemptInputs...)
}

func (buildLifecycleMachine *buildLifecycleStateMachine) transitionHistorySnapshot() []buildLifecycleTransitionRecord {
	if len(buildLifecycleMachine.transitionHistory) == 0 {
		return nil
	}
	return append([]buildLifecycleTransitionRecord(nil), buildLifecycleMachine.transitionHistory...)
}

func (buildLifecycleMachine *buildLifecycleStateMachine) rollbackHistorySnapshot() []buildLifecycleRollbackRecord {
	if len(buildLifecycleMachine.rollbackHistory) == 0 {
		return nil
	}
	return append([]buildLifecycleRollbackRecord(nil), buildLifecycleMachine.rollbackHistory...)
}

func (buildLifecycleMachine *buildLifecycleStateMachine) transitionTo(
	nextPhase buildLifecyclePhase,
	reason string,
) error {
	return buildLifecycleMachine.transition(nextPhase, reason, "")
}

func (buildLifecycleMachine *buildLifecycleStateMachine) transitionToFailed(
	reason string,
	buildErr error,
) error {
	if buildErr == nil {
		return errors.New("build error is required for failed lifecycle transition")
	}
	return buildLifecycleMachine.transition(nextPhaseForFailedTransition(), reason, buildErr.Error())
}

func (buildLifecycleMachine *buildLifecycleStateMachine) recordRollback(
	decision buildLifecycleRollbackDecision,
	outcome buildLifecycleRollbackOutcome,
	reason string,
	rollbackErr error,
) error {
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return errors.New("rollback reason is required")
	}

	buildLifecycleMachine.rollbackSeq++
	rollbackRecord := buildLifecycleRollbackRecord{
		AttemptID: buildLifecycleMachine.attemptID,
		AtUTC:     buildLifecycleMachine.dependencies.nowUTC().Format(time.RFC3339Nano),
		Sequence:  buildLifecycleMachine.rollbackSeq,
		Workflow:  buildLifecycleMachine.workflow,
		Decision:  decision,
		Outcome:   outcome,
		Reason:    trimmedReason,
	}
	if rollbackErr != nil {
		rollbackRecord.Error = rollbackErr.Error()
	}

	buildLifecycleMachine.rollbackHistory = append(
		buildLifecycleMachine.rollbackHistory,
		rollbackRecord,
	)

	if buildLifecycleMachine.logger != nil {
		buildLifecycleMachine.logger.Debug(
			"Vorma build lifecycle rollback decision",
			"attempt_id",
			rollbackRecord.AttemptID,
			"at_utc",
			rollbackRecord.AtUTC,
			"seq",
			rollbackRecord.Sequence,
			"workflow",
			rollbackRecord.Workflow,
			"decision",
			rollbackRecord.Decision,
			"outcome",
			rollbackRecord.Outcome,
			"reason",
			rollbackRecord.Reason,
			"error",
			rollbackRecord.Error,
		)
	}

	return nil
}

func (buildLifecycleMachine *buildLifecycleStateMachine) transition(
	nextPhase buildLifecyclePhase,
	reason string,
	errorText string,
) error {
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return errors.New("lifecycle transition reason is required")
	}

	currentPhase := buildLifecycleMachine.currentPhase
	allowedNextPhases, hasPhase := buildLifecycleMachine.allowedTransitions[currentPhase]
	if !hasPhase {
		return fmt.Errorf(
			"workflow %q is terminal at phase %q and cannot transition to %q",
			buildLifecycleMachine.workflow,
			currentPhase,
			nextPhase,
		)
	}

	if _, transitionAllowed := allowedNextPhases[nextPhase]; !transitionAllowed {
		return fmt.Errorf(
			"invalid workflow %q lifecycle transition %q -> %q (allowed: %s)",
			buildLifecycleMachine.workflow,
			currentPhase,
			nextPhase,
			strings.Join(sortedBuildLifecyclePhaseNames(allowedNextPhases), ", "),
		)
	}

	buildLifecycleMachine.transitionSeq++
	buildLifecycleMachine.currentPhase = nextPhase

	transitionRecord := buildLifecycleTransitionRecord{
		AttemptID: buildLifecycleMachine.attemptID,
		AtUTC:     buildLifecycleMachine.dependencies.nowUTC().Format(time.RFC3339Nano),
		Sequence:  buildLifecycleMachine.transitionSeq,
		Workflow:  buildLifecycleMachine.workflow,
		From:      currentPhase,
		To:        nextPhase,
		Reason:    trimmedReason,
		Error:     errorText,
	}

	buildLifecycleMachine.transitionHistory = append(
		buildLifecycleMachine.transitionHistory,
		transitionRecord,
	)

	if buildLifecycleMachine.logger != nil {
		buildLifecycleMachine.logger.Debug(
			"Vorma build lifecycle transition",
			"attempt_id",
			transitionRecord.AttemptID,
			"at_utc",
			transitionRecord.AtUTC,
			"seq",
			transitionRecord.Sequence,
			"workflow",
			transitionRecord.Workflow,
			"from",
			transitionRecord.From,
			"to",
			transitionRecord.To,
			"reason",
			transitionRecord.Reason,
			"error",
			transitionRecord.Error,
		)
	}
	if buildLifecycleMachine.transitionObserver != nil {
		buildLifecycleMachine.transitionObserver(transitionRecord)
	}
	return nil
}

func normalizeBuildLifecycleAttemptInputs(
	attemptInputs []buildLifecycleAttemptInput,
) []buildLifecycleAttemptInput {
	if len(attemptInputs) == 0 {
		return nil
	}

	normalizedAttemptInputs := make([]buildLifecycleAttemptInput, 0, len(attemptInputs))
	for _, attemptInput := range attemptInputs {
		trimmedKey := strings.TrimSpace(attemptInput.Key)
		if trimmedKey == "" {
			continue
		}

		normalizedAttemptInputs = append(
			normalizedAttemptInputs,
			buildLifecycleAttemptInput{
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

func buildLifecycleAllowedTransitions(
	workflow buildLifecycleWorkflow,
) (map[buildLifecyclePhase]map[buildLifecyclePhase]struct{}, error) {
	switch workflow {
	case buildLifecycleWorkflowFullBuild:
		return map[buildLifecyclePhase]map[buildLifecyclePhase]struct{}{
			buildLifecyclePhaseIdle: {
				buildLifecyclePhaseStarted: {},
			},
			buildLifecyclePhaseStarted: {
				buildLifecyclePhaseRuntimeStateInitialized: {},
				buildLifecyclePhaseFailed:                  {},
			},
			buildLifecyclePhaseRuntimeStateInitialized: {
				buildLifecyclePhaseRoutesSynchronized: {},
				buildLifecyclePhaseFailed:             {},
			},
			buildLifecyclePhaseRoutesSynchronized: {
				buildLifecyclePhasePublicOutputCleaned: {},
				buildLifecyclePhaseFailed:              {},
			},
			buildLifecyclePhasePublicOutputCleaned: {
				buildLifecyclePhasePublicFileMapWritten: {},
				buildLifecyclePhaseFailed:               {},
			},
			buildLifecyclePhasePublicFileMapWritten: {
				buildLifecyclePhaseRouteArtifactsWritten: {},
				buildLifecyclePhaseFailed:                {},
			},
			buildLifecyclePhaseRouteArtifactsWritten: {
				buildLifecyclePhaseCompleted: {},
				buildLifecyclePhaseFailed:    {},
			},
		}, nil
	case buildLifecycleWorkflowFastRouteRebuild:
		return map[buildLifecyclePhase]map[buildLifecyclePhase]struct{}{
			buildLifecyclePhaseIdle: {
				buildLifecyclePhaseStarted: {},
			},
			buildLifecyclePhaseStarted: {
				buildLifecyclePhaseRoutesSynchronized: {},
				buildLifecyclePhaseFailed:             {},
			},
			buildLifecyclePhaseRoutesSynchronized: {
				buildLifecyclePhaseRouteArtifactsWritten: {},
				buildLifecyclePhaseFailed:                {},
			},
			buildLifecyclePhaseRouteArtifactsWritten: {
				buildLifecyclePhaseCompleted: {},
				buildLifecyclePhaseFailed:    {},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unknown build lifecycle workflow %q", workflow)
	}
}

func nextPhaseForFailedTransition() buildLifecyclePhase {
	return buildLifecyclePhaseFailed
}

func sortedBuildLifecyclePhaseNames(allowedPhases map[buildLifecyclePhase]struct{}) []string {
	phaseNames := make([]string, 0, len(allowedPhases))
	for allowedPhase := range allowedPhases {
		phaseNames = append(phaseNames, string(allowedPhase))
	}
	sort.Strings(phaseNames)
	return phaseNames
}
