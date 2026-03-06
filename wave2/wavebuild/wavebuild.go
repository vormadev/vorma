// Package wavebuild defines the build/dev orchestration contracts for Wave2.
//
// The contract is intentionally phase-oriented:
// events -> build -> backend_settling -> frontend_settling.
//
// Each phase owns its own terminal goals and executes through kit/tasks. Phase
// boundaries provide cross-phase ordering; inside a phase, dependency ordering
// and parallelism come only from task prerequisites.
package wavebuild

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/kit/tasks"
)

// Mode selects development or production behavior.
type Mode string

const (
	// ModeDev applies development-time policy.
	ModeDev Mode = "dev"
	// ModeProd applies production-time policy.
	ModeProd Mode = "prod"
)

// EventType is one normalized change category from watcher input.
type EventType string

const (
	// EventTypeConfigFileChanged represents semantic config changes.
	EventTypeConfigFileChanged EventType = "config_file_changed"
	// EventTypeGoSourceChanged represents application Go source changes.
	EventTypeGoSourceChanged EventType = "go_source_changed"
	// EventTypeCriticalCSSSourceChanged represents critical CSS source changes.
	EventTypeCriticalCSSSourceChanged EventType = "critical_css_source_changed"
	// EventTypeNormalCSSSourceChanged represents normal CSS source changes.
	EventTypeNormalCSSSourceChanged EventType = "normal_css_source_changed"
	// EventTypePublicStaticAssetChanged represents public static asset changes.
	EventTypePublicStaticAssetChanged EventType = "public_static_asset_changed"
	// EventTypePrivateStaticAssetChanged represents private static asset changes.
	EventTypePrivateStaticAssetChanged EventType = "private_static_asset_changed"
	// EventTypeFrameworkRouteDefinitionChanged represents framework route-definition changes.
	EventTypeFrameworkRouteDefinitionChanged EventType = "framework_route_definition_changed"
	// EventTypeFrameworkTemplateChanged represents framework template changes.
	EventTypeFrameworkTemplateChanged EventType = "framework_template_changed"
	// EventTypeAppDefinedWatchActionOnlyChanged represents app watch classes that request direct actions.
	EventTypeAppDefinedWatchActionOnlyChanged EventType = "app_defined_watch_action_only_changed"
	// EventTypeAppDefinedWatchWithRebuildChanged represents app watch classes that request rebuild work.
	EventTypeAppDefinedWatchWithRebuildChanged EventType = "app_defined_watch_with_rebuild_changed"
	// EventTypeIgnoredOrNoiseChanged represents ignored/noise-only batches.
	EventTypeIgnoredOrNoiseChanged EventType = "ignored_or_noise_changed"
	// EventTypeUnclassifiedNoWatchRuleChanged represents meaningful input with no matching watch rule.
	EventTypeUnclassifiedNoWatchRuleChanged EventType = "unclassified_no_watch_rule_changed"
)

// BatchEventOperation is one normalized filesystem event operation.
type BatchEventOperation string

const (
	// BatchEventOperationCreate represents create operations.
	BatchEventOperationCreate BatchEventOperation = "create"
	// BatchEventOperationWrite represents write operations.
	BatchEventOperationWrite BatchEventOperation = "write"
	// BatchEventOperationRemove represents remove operations.
	BatchEventOperationRemove BatchEventOperation = "remove"
	// BatchEventOperationRename represents rename operations.
	BatchEventOperationRename BatchEventOperation = "rename"
	// BatchEventOperationChmod represents chmod operations.
	BatchEventOperationChmod BatchEventOperation = "chmod"
	// BatchEventOperationUnknown represents unknown operations.
	BatchEventOperationUnknown BatchEventOperation = "unknown"
)

// ObservedBatchEvent is one normalized, already-classified batch event input.
//
// The event type is expected to be produced by the watcher/classification layer;
// this contract intentionally avoids path-shape guessing.
type ObservedBatchEvent struct {
	Type      EventType
	Operation BatchEventOperation
	PathCWD   string

	// NoiseOnly marks events that should not trigger user-visible work.
	NoiseOnly bool
	// NoOpConfigMutation marks config events where semantic config did not change.
	NoOpConfigMutation bool
}

// AppRequestedOutcomes captures app-requested observable outcomes for one batch.
type AppRequestedOutcomes struct {
	RequestFrameworkRefresh  bool
	RequestBrowserInvalidate bool
	RequestBrowserRevalidate bool
	RequestBrowserHardReload bool
	RequestRestart           bool
	RequestGoCompile         bool
}

// EventsPhaseInput is the phase-1 input contract in observable terms.
type EventsPhaseInput struct {
	Mode         Mode
	GenerationID string
	Events       []ObservedBatchEvent

	AppRequestedOutcomes AppRequestedOutcomes
	WaitingForBuildRetry bool
}

// EventsPhaseFacts is the minimal phase-1 output used by planners.
type EventsPhaseFacts struct {
	Mode         Mode
	GenerationID string

	FilesChangedCount int
	ChangedPathsCWD   []string

	// EventTypes contains deduplicated actionable event types, preserving
	// first-seen input order.
	EventTypes []EventType

	HasNoActionableEvents bool
	HasNoiseOnlyEvents    bool

	HasNoOpConfigMutation   bool
	HasSemanticConfigChange bool

	AppRequestedOutcomes AppRequestedOutcomes
	WaitingForBuildRetry bool
}

// BuildEventsPhaseFacts reduces one phase-1 input into deterministic planner facts.
func BuildEventsPhaseFacts(input EventsPhaseInput) (EventsPhaseFacts, error) {
	if modeError := validateEventsPhaseMode(input.Mode); modeError != nil {
		return EventsPhaseFacts{}, modeError
	}
	normalizedGenerationID := strings.TrimSpace(input.GenerationID)
	if normalizedGenerationID == "" {
		return EventsPhaseFacts{}, errors.New("wavebuild: generation id is required")
	}

	changedPathSet := make(map[string]struct{}, len(input.Events))
	changedPathsCWD := make([]string, 0, len(input.Events))
	eventTypeSet := make(map[EventType]struct{}, len(input.Events))
	actionableEventTypes := make([]EventType, 0, len(input.Events))

	hasNoOpConfigMutation := false
	hasSemanticConfigChange := false
	hasNoiseOnlyEvents := len(input.Events) > 0

	for eventIndex, rawEvent := range input.Events {
		normalizedPathCWD, normalizePathError := normalizeBatchEventPathCWD(
			rawEvent.PathCWD,
		)
		if normalizePathError != nil {
			return EventsPhaseFacts{}, fmt.Errorf(
				"wavebuild: normalize event path at index %d: %w",
				eventIndex,
				normalizePathError,
			)
		}
		if normalizedPathCWD != "" {
			if _, alreadySeenPath := changedPathSet[normalizedPathCWD]; !alreadySeenPath {
				changedPathSet[normalizedPathCWD] = struct{}{}
				changedPathsCWD = append(changedPathsCWD, normalizedPathCWD)
			}
		}

		if rawEvent.NoiseOnly {
			continue
		}
		hasNoiseOnlyEvents = false

		if rawEvent.Type == EventTypeConfigFileChanged && rawEvent.NoOpConfigMutation {
			hasNoOpConfigMutation = true
			continue
		}

		if !isSupportedEventType(rawEvent.Type) {
			return EventsPhaseFacts{}, fmt.Errorf(
				"wavebuild: unsupported event type %q at index %d",
				rawEvent.Type,
				eventIndex,
			)
		}
		if rawEvent.Type == EventTypeConfigFileChanged {
			hasSemanticConfigChange = true
		}
		if _, alreadySeenEventType := eventTypeSet[rawEvent.Type]; alreadySeenEventType {
			continue
		}
		eventTypeSet[rawEvent.Type] = struct{}{}
		actionableEventTypes = append(actionableEventTypes, rawEvent.Type)
	}

	hasNoActionableEvents := len(actionableEventTypes) == 0

	return EventsPhaseFacts{
		Mode:                    input.Mode,
		GenerationID:            normalizedGenerationID,
		FilesChangedCount:       len(changedPathsCWD),
		ChangedPathsCWD:         changedPathsCWD,
		EventTypes:              actionableEventTypes,
		HasNoActionableEvents:   hasNoActionableEvents,
		HasNoiseOnlyEvents:      hasNoiseOnlyEvents && !hasNoOpConfigMutation,
		HasNoOpConfigMutation:   hasNoOpConfigMutation,
		HasSemanticConfigChange: hasSemanticConfigChange,
		AppRequestedOutcomes:    input.AppRequestedOutcomes,
		WaitingForBuildRetry:    input.WaitingForBuildRetry,
	}, nil
}

// FrameworkSignalType is one framework-agnostic refresh signal intent.
type FrameworkSignalType string

const (
	// FrameworkSignalTypeRoutesChanged signals route-definition refresh intent.
	FrameworkSignalTypeRoutesChanged FrameworkSignalType = "routes_changed"
	// FrameworkSignalTypeTemplateChanged signals template refresh intent.
	FrameworkSignalTypeTemplateChanged FrameworkSignalType = "template_changed"
	// FrameworkSignalTypePublicFileMapChanged signals public-file-map refresh intent.
	FrameworkSignalTypePublicFileMapChanged FrameworkSignalType = "public_file_map_changed"
)

// FrameworkSignal is one framework-agnostic refresh signal payload.
type FrameworkSignal struct {
	Type FrameworkSignalType
	// FreshnessToken prevents stale refresh work from winning.
	FreshnessToken string
	// Trigger describes why the signal exists for observability.
	Trigger string
	// Metadata carries optional stable key/value context.
	Metadata map[string]string
}

// EventsPhaseRunner executes phase 1 (events) and returns phase-2 build goals.
type EventsPhaseRunner interface {
	RunEventsPhase(
		taskContext *tasks.Ctx,
		input EventsPhaseBatchInput,
	) (Phase1BuildGoals, error)
}

// BuildPhaseRunner executes phase 2 (build) and returns backend-settling goals.
type BuildPhaseRunner interface {
	RunBuildPhase(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (Phase2BackendSettlingGoals, error)
}

// BackendSettlingPhaseRunner executes phase 3 and returns frontend-settling goals.
type BackendSettlingPhaseRunner interface {
	RunBackendSettlingPhase(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (Phase3FrontendSettlingGoals, error)
}

// FrontendSettlingPhaseRunner executes phase 4 and returns completion summary.
type FrontendSettlingPhaseRunner interface {
	RunFrontendSettlingPhase(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (Phase4CompletionSummary, error)
}

// EventsPhaseRunnerFunc adapts a function to EventsPhaseRunner.
type EventsPhaseRunnerFunc func(
	taskContext *tasks.Ctx,
	input EventsPhaseBatchInput,
) (Phase1BuildGoals, error)

// RunEventsPhase executes one events-phase function adapter.
func (runner EventsPhaseRunnerFunc) RunEventsPhase(
	taskContext *tasks.Ctx,
	input EventsPhaseBatchInput,
) (Phase1BuildGoals, error) {
	return runner(taskContext, input)
}

// BuildPhaseRunnerFunc adapts a function to BuildPhaseRunner.
type BuildPhaseRunnerFunc func(
	taskContext *tasks.Ctx,
	input Phase2BatchInput,
) (Phase2BackendSettlingGoals, error)

// RunBuildPhase executes one build-phase function adapter.
func (runner BuildPhaseRunnerFunc) RunBuildPhase(
	taskContext *tasks.Ctx,
	input Phase2BatchInput,
) (Phase2BackendSettlingGoals, error) {
	return runner(taskContext, input)
}

// BackendSettlingPhaseRunnerFunc adapts a function to BackendSettlingPhaseRunner.
type BackendSettlingPhaseRunnerFunc func(
	taskContext *tasks.Ctx,
	input Phase3BatchInput,
) (Phase3FrontendSettlingGoals, error)

// RunBackendSettlingPhase executes one backend-settling phase function adapter.
func (runner BackendSettlingPhaseRunnerFunc) RunBackendSettlingPhase(
	taskContext *tasks.Ctx,
	input Phase3BatchInput,
) (Phase3FrontendSettlingGoals, error) {
	return runner(taskContext, input)
}

// FrontendSettlingPhaseRunnerFunc adapts a function to FrontendSettlingPhaseRunner.
type FrontendSettlingPhaseRunnerFunc func(
	taskContext *tasks.Ctx,
	input Phase4BatchInput,
) (Phase4CompletionSummary, error)

// RunFrontendSettlingPhase executes one frontend-settling phase function adapter.
func (runner FrontendSettlingPhaseRunnerFunc) RunFrontendSettlingPhase(
	taskContext *tasks.Ctx,
	input Phase4BatchInput,
) (Phase4CompletionSummary, error) {
	return runner(taskContext, input)
}

// FourPhaseRunnerConfig configures one phased runner.
type FourPhaseRunnerConfig struct {
	EventsPhaseRunner           EventsPhaseRunner
	BuildPhaseRunner            BuildPhaseRunner
	BackendSettlingPhaseRunner  BackendSettlingPhaseRunner
	FrontendSettlingPhaseRunner FrontendSettlingPhaseRunner
}

// FourPhaseRunner runs the canonical 4-phase pipeline on one shared tasks context.
type FourPhaseRunner struct {
	eventsPhaseRunner           EventsPhaseRunner
	buildPhaseRunner            BuildPhaseRunner
	backendSettlingPhaseRunner  BackendSettlingPhaseRunner
	frontendSettlingPhaseRunner FrontendSettlingPhaseRunner
}

var (
	errEventsPhaseRunnerRequired           = errors.New("wavebuild: events phase runner is required")
	errBuildPhaseRunnerRequired            = errors.New("wavebuild: build phase runner is required")
	errBackendSettlingPhaseRunnerRequired  = errors.New("wavebuild: backend settling phase runner is required")
	errFrontendSettlingPhaseRunnerRequired = errors.New("wavebuild: frontend settling phase runner is required")
)

// NewFourPhaseRunner constructs one phased runner from explicit phase runners.
func NewFourPhaseRunner(config FourPhaseRunnerConfig) (*FourPhaseRunner, error) {
	if config.EventsPhaseRunner == nil {
		return nil, errEventsPhaseRunnerRequired
	}
	if config.BuildPhaseRunner == nil {
		return nil, errBuildPhaseRunnerRequired
	}
	if config.BackendSettlingPhaseRunner == nil {
		return nil, errBackendSettlingPhaseRunnerRequired
	}
	if config.FrontendSettlingPhaseRunner == nil {
		return nil, errFrontendSettlingPhaseRunnerRequired
	}
	return &FourPhaseRunner{
		eventsPhaseRunner:           config.EventsPhaseRunner,
		buildPhaseRunner:            config.BuildPhaseRunner,
		backendSettlingPhaseRunner:  config.BackendSettlingPhaseRunner,
		frontendSettlingPhaseRunner: config.FrontendSettlingPhaseRunner,
	}, nil
}

// NewDefaultFourPhaseRunner constructs one runner wired to the in-package task graphs.
func NewDefaultFourPhaseRunner() *FourPhaseRunner {
	runner, configurationError := NewFourPhaseRunner(
		FourPhaseRunnerConfig{
			EventsPhaseRunner:           EventsPhaseRunnerFunc(RunPhase1TaskGraph),
			BuildPhaseRunner:            BuildPhaseRunnerFunc(RunPhase2TaskGraph),
			BackendSettlingPhaseRunner:  BackendSettlingPhaseRunnerFunc(RunPhase3TaskGraph),
			FrontendSettlingPhaseRunner: FrontendSettlingPhaseRunnerFunc(RunPhase4TaskGraph),
		},
	)
	if configurationError != nil {
		panic(
			fmt.Sprintf(
				"wavebuild: default four-phase runner must be constructible: %v",
				configurationError,
			),
		)
	}
	return runner
}

// Run executes events->build->backend_settling->frontend_settling with one
// batch-scoped tasks context.
func (runner *FourPhaseRunner) Run(
	parentContext context.Context,
	input EventsPhaseBatchInput,
) (FourPhaseRunResult, error) {
	if runner == nil {
		return FourPhaseRunResult{}, errors.New("wavebuild: four-phase runner is required")
	}
	if input.Events == nil {
		return FourPhaseRunResult{}, errors.New("wavebuild: events phase input is required")
	}
	if input.Trace == nil {
		input.Trace = &PhaseTaskTraceRecorder{}
	}
	phaseBatchInput := PhaseBatchInput{
		Mode:         input.Events.Mode,
		GenerationID: strings.TrimSpace(input.Events.GenerationID),
		Trace:        input.Trace,
	}

	batchTaskContext := tasks.NewCtx(parentContext)
	phase1BuildGoals, phase1Error := runner.eventsPhaseRunner.RunEventsPhase(
		batchTaskContext,
		input,
	)
	if phase1Error != nil {
		return FourPhaseRunResult{}, phase1Error
	}

	phase2BackendGoals, phase2Error := runner.buildPhaseRunner.RunBuildPhase(
		batchTaskContext,
		Phase2BatchInput{
			Batch:      phaseBatchInput,
			BuildGoals: phase1BuildGoals,
		},
	)
	if phase2Error != nil {
		return FourPhaseRunResult{}, phase2Error
	}

	phase3FrontendGoals, phase3Error :=
		runner.backendSettlingPhaseRunner.RunBackendSettlingPhase(
			batchTaskContext,
			Phase3BatchInput{
				Batch:        phaseBatchInput,
				BackendGoals: phase2BackendGoals,
			},
		)
	if phase3Error != nil {
		return FourPhaseRunResult{}, phase3Error
	}

	phase4CompletionSummary, phase4Error :=
		runner.frontendSettlingPhaseRunner.RunFrontendSettlingPhase(
			batchTaskContext,
			Phase4BatchInput{
				Batch:         phaseBatchInput,
				FrontendGoals: phase3FrontendGoals,
			},
		)
	if phase4Error != nil {
		return FourPhaseRunResult{}, phase4Error
	}

	return FourPhaseRunResult{
		Phase1BuildGoals:        phase1BuildGoals,
		Phase2BackendGoals:      phase2BackendGoals,
		Phase3FrontendGoals:     phase3FrontendGoals,
		Phase4CompletionSummary: phase4CompletionSummary,
		OrderedTaskNames:        phaseBatchInput.Trace.SnapshotOrderedTaskNames(),
	}, nil
}

func validateEventsPhaseMode(mode Mode) error {
	switch mode {
	case ModeDev, ModeProd:
		return nil
	default:
		return fmt.Errorf("wavebuild: unsupported mode %q", mode)
	}
}

func normalizeBatchEventPathCWD(pathCWD string) (string, error) {
	trimmedPathCWD := strings.TrimSpace(pathCWD)
	if trimmedPathCWD == "" {
		return "", nil
	}
	if filepath.IsAbs(trimmedPathCWD) {
		return "", fmt.Errorf("path must be cwd-relative: %q", pathCWD)
	}
	normalizedPathCWD := filepath.Clean(trimmedPathCWD)
	if normalizedPathCWD == "." {
		return "", nil
	}
	if normalizedPathCWD == ".." ||
		strings.HasPrefix(normalizedPathCWD, "../") {
		return "", fmt.Errorf("path must not escape cwd: %q", pathCWD)
	}
	return filepath.ToSlash(normalizedPathCWD), nil
}

func isSupportedEventType(eventType EventType) bool {
	switch eventType {
	case EventTypeConfigFileChanged:
		return true
	case EventTypeGoSourceChanged:
		return true
	case EventTypeCriticalCSSSourceChanged:
		return true
	case EventTypeNormalCSSSourceChanged:
		return true
	case EventTypePublicStaticAssetChanged:
		return true
	case EventTypePrivateStaticAssetChanged:
		return true
	case EventTypeFrameworkRouteDefinitionChanged:
		return true
	case EventTypeFrameworkTemplateChanged:
		return true
	case EventTypeAppDefinedWatchActionOnlyChanged:
		return true
	case EventTypeAppDefinedWatchWithRebuildChanged:
		return true
	case EventTypeIgnoredOrNoiseChanged:
		return true
	case EventTypeUnclassifiedNoWatchRuleChanged:
		return true
	default:
		return false
	}
}
