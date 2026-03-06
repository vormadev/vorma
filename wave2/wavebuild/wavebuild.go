// Package wavebuild defines the build/dev orchestration contracts for Wave2.
//
// The contract is intentionally phase-oriented:
// events -> build -> backend_settling -> frontend_settling.
//
// Mode policy is runner-level:
// - dev executes all four phases.
// - prod bypasses events/backend_settling/frontend_settling and runs build only.
//
// Each phase owns its own terminal goals and executes through kit/tasks. Phase
// boundaries provide cross-phase ordering; inside a phase, dependency ordering
// and parallelism come only from task prerequisites.
package wavebuild

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	// EventTypeAppDefinedWatchActionOnlyChanged represents app watch classes that request direct actions without implicit build work.
	EventTypeAppDefinedWatchActionOnlyChanged EventType = "app_defined_watch_action_only_changed"
	// EventTypeAppDefinedWatchWithRebuildChanged represents app watch classes that participate in normal build-phase planning; concrete goals still come from reduced outcomes.
	EventTypeAppDefinedWatchWithRebuildChanged EventType = "app_defined_watch_with_rebuild_changed"
	// EventTypeIgnoredOrNoiseChanged represents ignored/noise-only batches.
	EventTypeIgnoredOrNoiseChanged EventType = "ignored_or_noise_changed"
	// EventTypeUnclassifiedNoWatchRuleChanged represents meaningful input with no matching watch rule.
	EventTypeUnclassifiedNoWatchRuleChanged EventType = "unclassified_no_watch_rule_changed"
)

// ObservedBatchEvent is one normalized, already-classified batch event input.
//
// The event type is expected to be produced by the watcher/classification layer;
// this contract intentionally avoids path-shape guessing.
type ObservedBatchEvent struct {
	Type EventType

	// NoiseOnly marks events that should not trigger user-visible work.
	NoiseOnly bool
	// NoOpConfigMutation marks config events where semantic config did not change.
	NoOpConfigMutation bool
}

// AppRequestedOutcomes captures app-requested observable outcomes for one batch.
type AppRequestedOutcomes struct {
	RequestFrameworkRefresh               bool
	RequestNotifyVitePublicFileMapChanged bool
	RequestBrowserRevalidate              bool
	RequestBrowserHardReload              bool
	RequestRestart                        bool
	RequestGoCompile                      bool
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

	// EventTypes contains deduplicated actionable event types, preserving
	// first-seen input order.
	EventTypes []EventType

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
		return EventsPhaseFacts{}, errors.New(
			"wavebuild: generation id is required",
		)
	}

	eventTypeSet := make(map[EventType]struct{}, len(input.Events))
	actionableEventTypes := make([]EventType, 0, len(input.Events))

	for eventIndex, rawEvent := range input.Events {
		if rawEvent.NoiseOnly {
			continue
		}

		if rawEvent.Type == EventTypeConfigFileChanged &&
			rawEvent.NoOpConfigMutation {
			continue
		}

		if !isSupportedEventType(rawEvent.Type) {
			return EventsPhaseFacts{}, fmt.Errorf(
				"wavebuild: unsupported event type %q at index %d",
				rawEvent.Type,
				eventIndex,
			)
		}
		if _, alreadySeenEventType := eventTypeSet[rawEvent.Type]; alreadySeenEventType {
			continue
		}
		eventTypeSet[rawEvent.Type] = struct{}{}
		actionableEventTypes = append(actionableEventTypes, rawEvent.Type)
	}

	return EventsPhaseFacts{
		Mode:         input.Mode,
		GenerationID: normalizedGenerationID,
		EventTypes:   actionableEventTypes,
		AppRequestedOutcomes: reduceAppRequestedOutcomes(
			input.AppRequestedOutcomes,
		),
		WaitingForBuildRetry: input.WaitingForBuildRetry,
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

// PhaseEffectID identifies one terminal side effect emitted by a phase task.
type PhaseEffectID string

const (
	// PhaseEffectIDBuildCompileGoBinary compiles the Go binary.
	PhaseEffectIDBuildCompileGoBinary PhaseEffectID = "build_compile_go_binary"
	// PhaseEffectIDBuildCriticalCSS builds critical CSS artifacts.
	PhaseEffectIDBuildCriticalCSS PhaseEffectID = "build_critical_css"
	// PhaseEffectIDBuildNormalCSS builds normal CSS artifacts.
	PhaseEffectIDBuildNormalCSS PhaseEffectID = "build_normal_css"
	// PhaseEffectIDBuildProcessPublicStaticAssets processes public static assets.
	PhaseEffectIDBuildProcessPublicStaticAssets PhaseEffectID = "build_process_public_static_assets"
	// PhaseEffectIDBuildCleanupStalePublicStaticOutputs removes stale public static outputs.
	PhaseEffectIDBuildCleanupStalePublicStaticOutputs PhaseEffectID = "build_cleanup_stale_public_static_outputs"
	// PhaseEffectIDBuildProcessPrivateStaticAssets processes private static assets.
	PhaseEffectIDBuildProcessPrivateStaticAssets PhaseEffectID = "build_process_private_static_assets"
	// PhaseEffectIDBuildGeneratePublicFileMapArtifacts regenerates public file-map artifacts.
	PhaseEffectIDBuildGeneratePublicFileMapArtifacts PhaseEffectID = "build_generate_public_filemap_artifacts"
	// PhaseEffectIDBuildValidateOutputs validates output coherence.
	PhaseEffectIDBuildValidateOutputs PhaseEffectID = "build_validate_outputs"
	// PhaseEffectIDBackendApplyDevServerRestart applies a dev-server restart cycle.
	PhaseEffectIDBackendApplyDevServerRestart PhaseEffectID = "backend_apply_devserver_restart"
	// PhaseEffectIDBackendQueueRetryWaitRestart queues retry-wait restart behavior.
	PhaseEffectIDBackendQueueRetryWaitRestart PhaseEffectID = "backend_queue_retry_wait_restart"
	// PhaseEffectIDBackendRestartAppProcess restarts the app process.
	PhaseEffectIDBackendRestartAppProcess PhaseEffectID = "backend_restart_app_process"
	// PhaseEffectIDBackendRestartViteProcess restarts the Vite process.
	PhaseEffectIDBackendRestartViteProcess PhaseEffectID = "backend_restart_vite_process"
	// PhaseEffectIDBackendRefreshFrameworkRoute refreshes framework routes.
	PhaseEffectIDBackendRefreshFrameworkRoute PhaseEffectID = "backend_refresh_framework_route"
	// PhaseEffectIDBackendRefreshFrameworkTemplate refreshes framework template state.
	PhaseEffectIDBackendRefreshFrameworkTemplate PhaseEffectID = "backend_refresh_framework_template"
	// PhaseEffectIDBackendRefreshFrameworkPublicFileMap refreshes framework public file-map state.
	PhaseEffectIDBackendRefreshFrameworkPublicFileMap PhaseEffectID = "backend_refresh_framework_public_filemap"
	// PhaseEffectIDBackendAwaitReadiness waits for backend readiness.
	PhaseEffectIDBackendAwaitReadiness PhaseEffectID = "backend_await_readiness"
	// PhaseEffectIDFrontendBroadcastCSSHotReload broadcasts CSS hot reload.
	PhaseEffectIDFrontendBroadcastCSSHotReload PhaseEffectID = "frontend_broadcast_css_hotreload"
	// PhaseEffectIDFrontendNotifyVitePublicFileMapChanged notifies Vite that the public file map changed.
	PhaseEffectIDFrontendNotifyVitePublicFileMapChanged PhaseEffectID = "frontend_notify_vite_public_filemap_changed"
	// PhaseEffectIDFrontendBroadcastRevalidate broadcasts browser revalidation.
	PhaseEffectIDFrontendBroadcastRevalidate PhaseEffectID = "frontend_broadcast_revalidate"
	// PhaseEffectIDFrontendBroadcastHardReload broadcasts browser hard reload.
	PhaseEffectIDFrontendBroadcastHardReload PhaseEffectID = "frontend_broadcast_hard_reload"
	// PhaseEffectIDFrontendPublishNoReloadNeededNotice publishes no-reload messaging.
	PhaseEffectIDFrontendPublishNoReloadNeededNotice PhaseEffectID = "frontend_publish_no_reload_needed_notice"
)

// PhaseEffectRequest carries one terminal effect execution request.
type PhaseEffectRequest struct {
	EffectID PhaseEffectID

	Mode         Mode
	GenerationID string
}

// PhaseEffectExecutor executes terminal effect requests emitted by phase tasks.
type PhaseEffectExecutor interface {
	ExecutePhaseEffect(
		nativeContext context.Context,
		request PhaseEffectRequest,
	) error
}

// NoopPhaseEffectExecutor is an explicit no-op effect executor for design-time wiring.
type NoopPhaseEffectExecutor struct{}

// ExecutePhaseEffect implements PhaseEffectExecutor with no-op behavior.
func (NoopPhaseEffectExecutor) ExecutePhaseEffect(
	nativeContext context.Context,
	request PhaseEffectRequest,
) error {
	return nil
}

// PhaseExecutionScope carries execution adapters needed by terminal phase tasks.
type PhaseExecutionScope struct {
	EffectExecutor PhaseEffectExecutor
}

// FrameworkSignalsFromBackendSettlingGoals reduces backend-settling goals into
// framework-agnostic signals for framework adapter handoff.
func FrameworkSignalsFromBackendSettlingGoals(
	generationID string,
	backendGoals Phase2BackendSettlingGoals,
) []FrameworkSignal {
	freshnessToken := strings.TrimSpace(generationID)
	if freshnessToken == "" {
		return nil
	}
	signals := make([]FrameworkSignal, 0, 3)
	if backendGoals.RefreshFrameworkRoute {
		signals = append(
			signals,
			FrameworkSignal{
				Type:           FrameworkSignalTypeRoutesChanged,
				FreshnessToken: freshnessToken,
				Trigger:        "backend_settling_refresh_framework_route",
			},
		)
	}
	if backendGoals.RefreshFrameworkTemplate {
		signals = append(
			signals,
			FrameworkSignal{
				Type:           FrameworkSignalTypeTemplateChanged,
				FreshnessToken: freshnessToken,
				Trigger:        "backend_settling_refresh_framework_template",
			},
		)
	}
	if backendGoals.RefreshFrameworkPublicFileMap {
		signals = append(
			signals,
			FrameworkSignal{
				Type:           FrameworkSignalTypePublicFileMapChanged,
				FreshnessToken: freshnessToken,
				Trigger:        "backend_settling_refresh_framework_public_filemap",
			},
		)
	}
	return signals
}

var (
	errEventsPhaseInputRequired = errors.New(
		"wavebuild: events phase input is required",
	)
	errPhaseExecutionScopeRequired = errors.New(
		"wavebuild: phase execution scope is required",
	)
	errPhaseEffectExecutorRequired = errors.New(
		"wavebuild: phase effect executor is required",
	)
)

// RunFourPhasePipeline executes one phase batch with one batch-scoped tasks context.
//
// Mode policy:
// - dev runs phases 1-4.
// - prod bypasses phases 1/3/4 and runs phase 2 only.
func RunFourPhasePipeline(
	parentContext context.Context,
	input EventsPhaseBatchInput,
) (FourPhaseRunResult, error) {
	if input.Events == nil {
		return FourPhaseRunResult{}, errEventsPhaseInputRequired
	}
	if input.Execution == nil {
		return FourPhaseRunResult{}, errPhaseExecutionScopeRequired
	}
	if input.Execution.EffectExecutor == nil {
		return FourPhaseRunResult{}, errPhaseEffectExecutorRequired
	}
	if modeError := validateEventsPhaseMode(input.Events.Mode); modeError != nil {
		return FourPhaseRunResult{}, modeError
	}

	batchTaskContext := tasks.NewCtx(parentContext)
	phase1Task := tasks.NewTask(
		func(
			taskContext *tasks.Ctx,
			phaseInput EventsPhaseBatchInput,
		) (Phase1BuildGoals, error) {
			if phaseInput.Events.Mode == ModeProd {
				return canonicalProdBuildGoals(), nil
			}
			return RunPhase1TaskGraph(taskContext, phaseInput)
		},
	)

	phase2Task := tasks.NewTask(
		func(
			taskContext *tasks.Ctx,
			phaseInput EventsPhaseBatchInput,
		) (Phase2Output, error) {
			phase1BuildGoals, phase1Error := phase1Task.Run(
				taskContext,
				phaseInput,
			)
			if phase1Error != nil {
				return Phase2Output{}, phase1Error
			}
			return RunPhase2TaskGraph(
				taskContext,
				Phase2BatchInput{
					Batch: PhaseBatchInput{
						Mode: phaseInput.Events.Mode,
						GenerationID: strings.TrimSpace(
							phaseInput.Events.GenerationID,
						),
						Execution: phaseInput.Execution,
					},
					BuildGoals: phase1BuildGoals,
				},
			)
		},
	)

	phase3Task := tasks.NewTask(
		func(
			taskContext *tasks.Ctx,
			phaseInput EventsPhaseBatchInput,
		) (Phase3FrontendSettlingGoals, error) {
			if phaseInput.Events.Mode == ModeProd {
				return Phase3FrontendSettlingGoals{}, nil
			}
			phase2Output, phase2Error := phase2Task.Run(
				taskContext,
				phaseInput,
			)
			if phase2Error != nil {
				return Phase3FrontendSettlingGoals{}, phase2Error
			}
			return RunPhase3TaskGraph(
				taskContext,
				Phase3BatchInput{
					Batch: PhaseBatchInput{
						Mode: phaseInput.Events.Mode,
						GenerationID: strings.TrimSpace(
							phaseInput.Events.GenerationID,
						),
						Execution: phaseInput.Execution,
					},
					BackendGoals: phase2Output.BackendSettlingGoals,
					BuildFacts:   phase2Output.BuildOutcomeFacts,
				},
			)
		},
	)

	phase4Task := tasks.NewTask(
		func(
			taskContext *tasks.Ctx,
			phaseInput EventsPhaseBatchInput,
		) (Phase4CompletionSummary, error) {
			if phaseInput.Events.Mode == ModeProd {
				return Phase4CompletionSummary{}, nil
			}
			phase3FrontendGoals, phase3Error := phase3Task.Run(
				taskContext,
				phaseInput,
			)
			if phase3Error != nil {
				return Phase4CompletionSummary{}, phase3Error
			}
			return RunPhase4TaskGraph(
				taskContext,
				Phase4BatchInput{
					Batch: PhaseBatchInput{
						Mode: phaseInput.Events.Mode,
						GenerationID: strings.TrimSpace(
							phaseInput.Events.GenerationID,
						),
						Execution: phaseInput.Execution,
					},
					FrontendGoals: phase3FrontendGoals,
				},
			)
		},
	)

	rootTask := tasks.NewTask(
		func(
			taskContext *tasks.Ctx,
			phaseInput EventsPhaseBatchInput,
		) (FourPhaseRunResult, error) {
			phase1BuildGoals, phase1Error := phase1Task.Run(
				taskContext,
				phaseInput,
			)
			if phase1Error != nil {
				return FourPhaseRunResult{}, phase1Error
			}
			phase2Output, phase2Error := phase2Task.Run(taskContext, phaseInput)
			if phase2Error != nil {
				return FourPhaseRunResult{}, phase2Error
			}
			if phaseInput.Events.Mode == ModeProd {
				return FourPhaseRunResult{
					Phase1BuildGoals:   phase1BuildGoals,
					Phase2BackendGoals: phase2Output.BackendSettlingGoals,
				}, nil
			}
			phase3FrontendGoals, phase3Error := phase3Task.Run(
				taskContext,
				phaseInput,
			)
			if phase3Error != nil {
				return FourPhaseRunResult{}, phase3Error
			}
			phase4CompletionSummary, phase4Error := phase4Task.Run(
				taskContext,
				phaseInput,
			)
			if phase4Error != nil {
				return FourPhaseRunResult{}, phase4Error
			}
			return FourPhaseRunResult{
				Phase1BuildGoals:        phase1BuildGoals,
				Phase2BackendGoals:      phase2Output.BackendSettlingGoals,
				Phase3FrontendGoals:     phase3FrontendGoals,
				Phase4CompletionSummary: phase4CompletionSummary,
				FrameworkSignals: FrameworkSignalsFromBackendSettlingGoals(
					strings.TrimSpace(phaseInput.Events.GenerationID),
					phase2Output.BackendSettlingGoals,
				),
			}, nil
		},
	)

	return rootTask.Run(batchTaskContext, input)
}

func validateEventsPhaseMode(mode Mode) error {
	switch mode {
	case ModeDev, ModeProd:
		return nil
	default:
		return fmt.Errorf("wavebuild: unsupported mode %q", mode)
	}
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

func reduceAppRequestedOutcomes(
	rawOutcomes AppRequestedOutcomes,
) AppRequestedOutcomes {
	reducedOutcomes := rawOutcomes
	if reducedOutcomes.RequestBrowserHardReload {
		reducedOutcomes.RequestNotifyVitePublicFileMapChanged = false
		reducedOutcomes.RequestBrowserRevalidate = false
		return reducedOutcomes
	}
	if reducedOutcomes.RequestNotifyVitePublicFileMapChanged {
		reducedOutcomes.RequestBrowserRevalidate = false
	}
	return reducedOutcomes
}

func canonicalProdBuildGoals() Phase1BuildGoals {
	return Phase1BuildGoals{
		CompileGoBinary:                 true,
		BuildCriticalCSS:                true,
		BuildNormalCSS:                  true,
		ProcessPublicStaticAssets:       true,
		CleanupStalePublicStaticOutputs: true,
		ProcessPrivateStaticAssets:      true,
		GeneratePublicFileMap:           true,
		ValidateBuildOutputs:            true,
	}
}

func executePhaseEffect(
	taskContext *tasks.Ctx,
	batchInput PhaseBatchInput,
	effectID PhaseEffectID,
) error {
	if taskContext == nil {
		return errors.New("wavebuild: task context is required")
	}
	if batchInput.Execution == nil {
		return errPhaseExecutionScopeRequired
	}
	if batchInput.Execution.EffectExecutor == nil {
		return errPhaseEffectExecutorRequired
	}
	if os.Getenv("__WAVE_INTERNAL_TEST_MODE") == "1" {
		fmt.Printf("wave2_effect %s\n", effectID)
		return nil
	}
	return batchInput.Execution.EffectExecutor.ExecutePhaseEffect(
		taskContext.NativeContext(),
		PhaseEffectRequest{
			EffectID:     effectID,
			Mode:         batchInput.Mode,
			GenerationID: batchInput.GenerationID,
		},
	)
}
