package vormabuild

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
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
	Sequence uint64
	Workflow buildLifecycleWorkflow
	From     buildLifecyclePhase
	To       buildLifecyclePhase
	Reason   string
	Error    string
}

type buildLifecycleTransitionObserver func(buildLifecycleTransitionRecord)

type buildLifecycleStateMachine struct {
	workflow           buildLifecycleWorkflow
	currentPhase       buildLifecyclePhase
	transitionSeq      uint64
	logger             *slog.Logger
	allowedTransitions map[buildLifecyclePhase]map[buildLifecyclePhase]struct{}
	transitionObserver buildLifecycleTransitionObserver
}

func newBuildLifecycleStateMachine(
	workflow buildLifecycleWorkflow,
	logger *slog.Logger,
	transitionObserver buildLifecycleTransitionObserver,
) (*buildLifecycleStateMachine, error) {
	allowedTransitions, err := buildLifecycleAllowedTransitions(workflow)
	if err != nil {
		return nil, err
	}

	return &buildLifecycleStateMachine{
		workflow:           workflow,
		currentPhase:       buildLifecyclePhaseIdle,
		logger:             logger,
		allowedTransitions: allowedTransitions,
		transitionObserver: transitionObserver,
	}, nil
}

func (buildLifecycleMachine *buildLifecycleStateMachine) currentPhaseSnapshot() buildLifecyclePhase {
	return buildLifecycleMachine.currentPhase
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
		Sequence: buildLifecycleMachine.transitionSeq,
		Workflow: buildLifecycleMachine.workflow,
		From:     currentPhase,
		To:       nextPhase,
		Reason:   trimmedReason,
		Error:    errorText,
	}

	if buildLifecycleMachine.logger != nil {
		buildLifecycleMachine.logger.Debug(
			"Vorma build lifecycle transition",
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
