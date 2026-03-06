// Package wavebuild defines the build/dev orchestration contracts for Wave2.
//
// The contract is intentionally phase-oriented:
// events -> build -> backend_settling -> frontend_settling.
// Symbol naming uses numeric phase labels:
// phase_1 -> phase_2 -> phase_3 -> phase_4.
//
// mode policy is pipeline-level:
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

// mode selects development or production behavior.
type mode string

const (
	// modeDev applies development-time policy.
	modeDev mode = "dev"
	// modeProd applies production-time policy.
	modeProd mode = "prod"
)

// eventType is one normalized change category from watcher input.
type eventType string

const (
	// eventTypeConfigFileChanged represents semantic config changes.
	eventTypeConfigFileChanged eventType = "config_file_changed"
	// eventTypeGoSourceChanged represents application Go source changes.
	eventTypeGoSourceChanged eventType = "go_source_changed"
	// eventTypeCriticalCSSSourceChanged represents critical CSS source changes.
	eventTypeCriticalCSSSourceChanged eventType = "critical_css_source_changed"
	// eventTypeNormalCSSSourceChanged represents normal CSS source changes.
	eventTypeNormalCSSSourceChanged eventType = "normal_css_source_changed"
	// eventTypePublicStaticAssetChanged represents public static asset changes.
	eventTypePublicStaticAssetChanged eventType = "public_static_asset_changed"
	// eventTypePrivateStaticAssetChanged represents private static asset changes.
	eventTypePrivateStaticAssetChanged eventType = "private_static_asset_changed"
	// eventTypeFrameworkRouteDefinitionChanged represents framework route-definition changes.
	eventTypeFrameworkRouteDefinitionChanged eventType = "framework_route_definition_changed"
	// eventTypeFrameworkTemplateChanged represents framework template changes.
	eventTypeFrameworkTemplateChanged eventType = "framework_template_changed"
	// eventTypeAppDefinedWatchActionOnlyChanged represents app watch classes that request direct actions without implicit build work.
	eventTypeAppDefinedWatchActionOnlyChanged eventType = "app_defined_watch_action_only_changed"
	// eventTypeAppDefinedWatchWithRebuildChanged represents app watch classes that participate in normal build-phase planning; concrete goals still come from reduced outcomes.
	eventTypeAppDefinedWatchWithRebuildChanged eventType = "app_defined_watch_with_rebuild_changed"
	// eventTypeIgnoredOrNoiseChanged represents ignored/noise-only batches.
	eventTypeIgnoredOrNoiseChanged eventType = "ignored_or_noise_changed"
	// eventTypeUnclassifiedNoWatchRuleChanged represents meaningful input with no matching watch rule.
	eventTypeUnclassifiedNoWatchRuleChanged eventType = "unclassified_no_watch_rule_changed"
)

// frontendTerminalBrowserAction is the single terminal browser action selected
// for one batch.
type frontendTerminalBrowserAction string

const (
	// frontendTerminalBrowserActionNone represents no browser action.
	frontendTerminalBrowserActionNone frontendTerminalBrowserAction = "none"
	// frontendTerminalBrowserActionCSSHotReload represents CSS-only hot reload.
	frontendTerminalBrowserActionCSSHotReload frontendTerminalBrowserAction = "css_hot_reload"
	// frontendTerminalBrowserActionNotifyVitePublicFileMapChanged notifies Vite that public file-map artifacts changed.
	frontendTerminalBrowserActionNotifyVitePublicFileMapChanged frontendTerminalBrowserAction = "notify_vite_public_filemap_changed"
	// frontendTerminalBrowserActionRevalidate represents browser revalidation.
	frontendTerminalBrowserActionRevalidate frontendTerminalBrowserAction = "revalidate"
	// frontendTerminalBrowserActionHardReload represents browser hard reload.
	frontendTerminalBrowserActionHardReload frontendTerminalBrowserAction = "hard_reload"
)

func (action frontendTerminalBrowserAction) priority() int {
	switch action {
	case frontendTerminalBrowserActionHardReload:
		return 4
	case frontendTerminalBrowserActionNotifyVitePublicFileMapChanged:
		return 3
	case frontendTerminalBrowserActionRevalidate:
		return 2
	case frontendTerminalBrowserActionCSSHotReload:
		return 1
	default:
		return 0
	}
}

func (leftAction frontendTerminalBrowserAction) dominantWith(
	rightAction frontendTerminalBrowserAction,
) frontendTerminalBrowserAction {
	if rightAction.priority() > leftAction.priority() {
		return rightAction
	}
	return leftAction
}

// observedBatchEvent is one normalized, already-classified batch event input.
//
// The event type is expected to be produced by the watcher/classification layer;
// this contract intentionally avoids path-shape guessing.
type observedBatchEvent struct {
	eventType eventType

	// noiseOnly marks events that should not trigger user-visible work.
	noiseOnly bool
	// noOpConfigMutation marks config events where semantic config did not change.
	noOpConfigMutation bool
}

// appRequestedOutcomes captures app-requested observable outcomes for one batch.
type appRequestedOutcomes struct {
	requestFrameworkRefresh        bool
	requestedTerminalBrowserAction frontendTerminalBrowserAction
	requestRestart                 bool
	requestGoCompile               bool
}

// phase1Input is the phase-1 input contract in observable terms.
type phase1Input struct {
	mode         mode
	generationID string
	events       []observedBatchEvent

	appRequestedOutcomes appRequestedOutcomes
	waitingForBuildRetry bool
}

// phase1Facts is the minimal phase-1 output used by planners.
type phase1Facts struct {
	mode         mode
	generationID string

	// eventTypes contains deduplicated actionable event types, preserving
	// first-seen input order.
	eventTypes []eventType

	appRequestedOutcomes appRequestedOutcomes
	waitingForBuildRetry bool
}

// buildFacts reduces one phase-1 input into deterministic planner facts.
func (input phase1Input) buildFacts() (phase1Facts, error) {
	if modeError := input.mode.validate(); modeError != nil {
		return phase1Facts{}, modeError
	}
	normalizedGenerationID := strings.TrimSpace(input.generationID)
	if normalizedGenerationID == "" {
		return phase1Facts{}, errors.New(
			"wavebuild: generation id is required",
		)
	}

	appRequestedOutcomes, appRequestedOutcomesError := input.appRequestedOutcomes.reduce()
	if appRequestedOutcomesError != nil {
		return phase1Facts{}, appRequestedOutcomesError
	}

	eventTypeSet := make(map[eventType]struct{}, len(input.events))
	actionableEventTypes := make([]eventType, 0, len(input.events))

	for eventIndex, rawEvent := range input.events {
		if rawEvent.noiseOnly {
			continue
		}

		if rawEvent.eventType == eventTypeConfigFileChanged &&
			rawEvent.noOpConfigMutation {
			continue
		}

		if !rawEvent.eventType.isSupported() {
			return phase1Facts{}, fmt.Errorf(
				"wavebuild: unsupported event type %q at index %d",
				rawEvent.eventType,
				eventIndex,
			)
		}
		if _, alreadySeenEventType := eventTypeSet[rawEvent.eventType]; alreadySeenEventType {
			continue
		}
		eventTypeSet[rawEvent.eventType] = struct{}{}
		actionableEventTypes = append(actionableEventTypes, rawEvent.eventType)
	}

	return phase1Facts{
		mode:                 input.mode,
		generationID:         normalizedGenerationID,
		eventTypes:           actionableEventTypes,
		appRequestedOutcomes: appRequestedOutcomes,
		waitingForBuildRetry: input.waitingForBuildRetry,
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
	signalType FrameworkSignalType
	// freshnessToken prevents stale refresh work from winning.
	freshnessToken string
	// trigger describes why the signal exists for observability.
	trigger string
	// metadata carries optional stable key/value context.
	metadata map[string]string
}

// SignalType returns the framework signal type.
func (signal FrameworkSignal) SignalType() FrameworkSignalType {
	return signal.signalType
}

// FreshnessToken returns the freshness token carried by this signal.
func (signal FrameworkSignal) FreshnessToken() string {
	return signal.freshnessToken
}

// Trigger returns the stable trigger label for this signal.
func (signal FrameworkSignal) Trigger() string {
	return signal.trigger
}

// Metadata returns optional stable metadata context for this signal.
func (signal FrameworkSignal) Metadata() map[string]string {
	return signal.metadata
}

// phaseEffectID identifies one terminal side effect emitted by a phase task.
type phaseEffectID string

const (
	// phaseEffectIDBuildCompileGoBinary compiles the Go binary.
	phaseEffectIDBuildCompileGoBinary phaseEffectID = "build_compile_go_binary"
	// phaseEffectIDBuildCriticalCSS builds critical CSS artifacts.
	phaseEffectIDBuildCriticalCSS phaseEffectID = "build_critical_css"
	// phaseEffectIDBuildNormalCSS builds normal CSS artifacts.
	phaseEffectIDBuildNormalCSS phaseEffectID = "build_normal_css"
	// phaseEffectIDBuildProcessPublicStaticAssets processes public static assets.
	phaseEffectIDBuildProcessPublicStaticAssets phaseEffectID = "build_process_public_static_assets"
	// phaseEffectIDBuildCleanupStalePublicStaticOutputs removes stale public static outputs.
	phaseEffectIDBuildCleanupStalePublicStaticOutputs phaseEffectID = "build_cleanup_stale_public_static_outputs"
	// phaseEffectIDBuildProcessPrivateStaticAssets processes private static assets.
	phaseEffectIDBuildProcessPrivateStaticAssets phaseEffectID = "build_process_private_static_assets"
	// phaseEffectIDBuildGeneratePublicFileMapArtifacts regenerates public file-map artifacts.
	phaseEffectIDBuildGeneratePublicFileMapArtifacts phaseEffectID = "build_generate_public_filemap_artifacts"
	// phaseEffectIDBuildValidateOutputs validates output coherence.
	phaseEffectIDBuildValidateOutputs phaseEffectID = "build_validate_outputs"
	// phaseEffectIDBackendApplyDevServerRestart applies a dev-server restart cycle.
	phaseEffectIDBackendApplyDevServerRestart phaseEffectID = "backend_apply_devserver_restart"
	// phaseEffectIDBackendQueueRetryWaitRestart queues retry-wait restart behavior.
	phaseEffectIDBackendQueueRetryWaitRestart phaseEffectID = "backend_queue_retry_wait_restart"
	// phaseEffectIDBackendRestartAppProcess restarts the app process.
	phaseEffectIDBackendRestartAppProcess phaseEffectID = "backend_restart_app_process"
	// phaseEffectIDBackendRestartViteProcess restarts the Vite process.
	phaseEffectIDBackendRestartViteProcess phaseEffectID = "backend_restart_vite_process"
	// phaseEffectIDBackendRefreshFrameworkRoute refreshes framework routes.
	phaseEffectIDBackendRefreshFrameworkRoute phaseEffectID = "backend_refresh_framework_route"
	// phaseEffectIDBackendRefreshFrameworkTemplate refreshes framework template state.
	phaseEffectIDBackendRefreshFrameworkTemplate phaseEffectID = "backend_refresh_framework_template"
	// phaseEffectIDBackendRefreshFrameworkPublicFileMap refreshes framework public file-map state.
	phaseEffectIDBackendRefreshFrameworkPublicFileMap phaseEffectID = "backend_refresh_framework_public_filemap"
	// phaseEffectIDBackendAwaitReadiness waits for backend readiness.
	phaseEffectIDBackendAwaitReadiness phaseEffectID = "backend_await_readiness"
	// phaseEffectIDFrontendBroadcastCSSHotReload broadcasts CSS hot reload.
	phaseEffectIDFrontendBroadcastCSSHotReload phaseEffectID = "frontend_broadcast_css_hotreload"
	// phaseEffectIDFrontendNotifyVitePublicFileMapChanged notifies Vite that the public file map changed.
	phaseEffectIDFrontendNotifyVitePublicFileMapChanged phaseEffectID = "frontend_notify_vite_public_filemap_changed"
	// phaseEffectIDFrontendBroadcastRevalidate broadcasts browser revalidation.
	phaseEffectIDFrontendBroadcastRevalidate phaseEffectID = "frontend_broadcast_revalidate"
	// phaseEffectIDFrontendBroadcastHardReload broadcasts browser hard reload.
	phaseEffectIDFrontendBroadcastHardReload phaseEffectID = "frontend_broadcast_hard_reload"
	// phaseEffectIDFrontendPublishNoReloadNeededNotice publishes no-reload messaging.
	phaseEffectIDFrontendPublishNoReloadNeededNotice phaseEffectID = "frontend_publish_no_reload_needed_notice"
)

// phaseEffectRequest carries one terminal effect execution request.
type phaseEffectRequest struct {
	effectID phaseEffectID

	mode         mode
	generationID string
}

// phaseEffectResult captures observable execution outcomes from one effect.
//
// Most effects return the zero value. Fields are populated only where downstream
// planning needs concrete "what happened" facts instead of requested goals.
type phaseEffectResult struct {
	publicFileMapArtifactsChanged  bool
	publicFileMapArtifactsRepaired bool
}

// noopPhaseEffectTask is the explicit no-op phase task for design-time pipeline
// wiring.
var noopPhaseEffectTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		request phaseEffectRequest,
	) (phaseEffectResult, error) {
		return phaseEffectResult{}, nil
	},
)

// phaseExecutionScope carries execution tasks needed by terminal phase tasks.
type phaseExecutionScope struct {
	phaseEffectTask *tasks.Task[phaseEffectRequest, phaseEffectResult]
}

// frameworkSignals reduces backend-settling goals into framework-agnostic
// signals for framework adapter handoff.
func (backendGoals phase2BackendSettlingGoals) frameworkSignals(
	generationID string,
) []FrameworkSignal {
	freshnessToken := strings.TrimSpace(generationID)
	if freshnessToken == "" {
		return nil
	}
	signals := make([]FrameworkSignal, 0, 3)
	if backendGoals.refreshFrameworkRoute {
		signals = append(
			signals,
			FrameworkSignal{
				signalType:     FrameworkSignalTypeRoutesChanged,
				freshnessToken: freshnessToken,
				trigger:        "backend_settling_refresh_framework_route",
			},
		)
	}
	if backendGoals.refreshFrameworkTemplate {
		signals = append(
			signals,
			FrameworkSignal{
				signalType:     FrameworkSignalTypeTemplateChanged,
				freshnessToken: freshnessToken,
				trigger:        "backend_settling_refresh_framework_template",
			},
		)
	}
	if backendGoals.refreshFrameworkPublicFileMap {
		signals = append(
			signals,
			FrameworkSignal{
				signalType:     FrameworkSignalTypePublicFileMapChanged,
				freshnessToken: freshnessToken,
				trigger:        "backend_settling_refresh_framework_public_filemap",
			},
		)
	}
	return signals
}

var (
	errPhase1InputRequired = errors.New(
		"wavebuild: phase-1 input is required",
	)
	errGenerationIDRequired = errors.New(
		"wavebuild: generation id is required",
	)
	errPhaseExecutionScopeRequired = errors.New(
		"wavebuild: phase execution scope is required",
	)
	errPhaseEffectTaskRequired = errors.New(
		"wavebuild: phase effect task is required",
	)
)

// runFourPhasePipeline executes one phase batch with one batch-scoped tasks context.
//
// mode policy:
// - dev runs phases 1-4.
// - prod bypasses phases 1/3/4 and runs phase 2 only.
func runFourPhasePipeline(
	parentContext context.Context,
	input phase1BatchInput,
) (fourPhaseRunResult, error) {
	if input.phase1 == nil {
		return fourPhaseRunResult{}, errPhase1InputRequired
	}
	if input.execution == nil {
		return fourPhaseRunResult{}, errPhaseExecutionScopeRequired
	}
	if input.execution.phaseEffectTask == nil {
		return fourPhaseRunResult{}, errPhaseEffectTaskRequired
	}
	normalizedGenerationID := strings.TrimSpace(input.phase1.generationID)
	if normalizedGenerationID == "" {
		return fourPhaseRunResult{}, errGenerationIDRequired
	}
	if modeError := input.phase1.mode.validate(); modeError != nil {
		return fourPhaseRunResult{}, modeError
	}

	batchTaskContext := tasks.NewCtx(parentContext)
	phase1BuildGoals := canonicalProdBuildGoals()
	if input.phase1.mode != modeProd {
		var phase1Error error
		phase1BuildGoals, phase1Error = phase1PlanBuildGoalsTask.Run(
			batchTaskContext,
			input,
		)
		if phase1Error != nil {
			return fourPhaseRunResult{}, phase1Error
		}
	}

	phaseBatchInput := phaseBatchInput{
		mode:         input.phase1.mode,
		generationID: normalizedGenerationID,
		execution:    input.execution,
	}
	phase2Output, phase2Error := phase2PlanOutputTask.Run(
		batchTaskContext,
		phase2BatchInput{
			batch:      phaseBatchInput,
			buildGoals: phase1BuildGoals,
		},
	)
	if phase2Error != nil {
		return fourPhaseRunResult{}, phase2Error
	}
	if input.phase1.mode == modeProd {
		return fourPhaseRunResult{
			phase1BuildGoals:   phase1BuildGoals,
			phase2BackendGoals: phase2Output.backendSettlingGoals,
		}, nil
	}

	phase3FrontendGoals, phase3Error := phase3PlanFrontendSettlingGoalsTask.Run(
		batchTaskContext,
		phase3BatchInput{
			batch:        phaseBatchInput,
			backendGoals: phase2Output.backendSettlingGoals,
		},
	)
	if phase3Error != nil {
		return fourPhaseRunResult{}, phase3Error
	}
	phase4CompletionSummary, phase4Error := phase4ExecuteTerminalBrowserActionTask.Run(
		batchTaskContext,
		phase4BatchInput{
			batch:         phaseBatchInput,
			frontendGoals: phase3FrontendGoals,
		},
	)
	if phase4Error != nil {
		return fourPhaseRunResult{}, phase4Error
	}
	if phase4CompletionSummary.requiresBackendViteHealing {
		healingBackendGoals := phase2BackendSettlingGoals{
			restartViteProcess:    true,
			awaitBackendReadiness: true,
		}
		healingFrontendGoals, healingPhase3Error := phase3PlanFrontendSettlingGoalsTask.Run(
			batchTaskContext,
			phase3BatchInput{
				batch:        phaseBatchInput,
				backendGoals: healingBackendGoals,
			},
		)
		if healingPhase3Error != nil {
			return fourPhaseRunResult{}, healingPhase3Error
		}
		healingCompletionSummary, healingPhase4Error := phase4ExecuteTerminalBrowserActionTask.Run(
			batchTaskContext,
			phase4BatchInput{
				batch:         phaseBatchInput,
				frontendGoals: healingFrontendGoals,
			},
		)
		if healingPhase4Error != nil {
			return fourPhaseRunResult{}, healingPhase4Error
		}
		phase2Output.backendSettlingGoals = phase2Output.backendSettlingGoals.merge(
			healingBackendGoals,
		)
		phase3FrontendGoals = healingFrontendGoals
		phase4CompletionSummary = healingCompletionSummary
	}
	return fourPhaseRunResult{
		phase1BuildGoals:        phase1BuildGoals,
		phase2BackendGoals:      phase2Output.backendSettlingGoals,
		phase3FrontendGoals:     phase3FrontendGoals,
		phase4CompletionSummary: phase4CompletionSummary,
		frameworkSignals: phase2Output.backendSettlingGoals.frameworkSignals(
			normalizedGenerationID,
		),
	}, nil
}

func (mode mode) validate() error {
	switch mode {
	case modeDev, modeProd:
		return nil
	default:
		return fmt.Errorf("wavebuild: unsupported mode %q", mode)
	}
}

func (eventType eventType) isSupported() bool {
	switch eventType {
	case eventTypeConfigFileChanged:
		return true
	case eventTypeGoSourceChanged:
		return true
	case eventTypeCriticalCSSSourceChanged:
		return true
	case eventTypeNormalCSSSourceChanged:
		return true
	case eventTypePublicStaticAssetChanged:
		return true
	case eventTypePrivateStaticAssetChanged:
		return true
	case eventTypeFrameworkRouteDefinitionChanged:
		return true
	case eventTypeFrameworkTemplateChanged:
		return true
	case eventTypeAppDefinedWatchActionOnlyChanged:
		return true
	case eventTypeAppDefinedWatchWithRebuildChanged:
		return true
	case eventTypeIgnoredOrNoiseChanged:
		return true
	case eventTypeUnclassifiedNoWatchRuleChanged:
		return true
	default:
		return false
	}
}

func (rawOutcomes appRequestedOutcomes) reduce() (appRequestedOutcomes, error) {
	reducedOutcomes := rawOutcomes
	switch reducedOutcomes.requestedTerminalBrowserAction {
	case "":
		reducedOutcomes.requestedTerminalBrowserAction = frontendTerminalBrowserActionNone
	case frontendTerminalBrowserActionNone,
		frontendTerminalBrowserActionCSSHotReload,
		frontendTerminalBrowserActionNotifyVitePublicFileMapChanged,
		frontendTerminalBrowserActionRevalidate,
		frontendTerminalBrowserActionHardReload:
	default:
		return appRequestedOutcomes{}, fmt.Errorf(
			"wavebuild: unsupported requested terminal browser action %q",
			reducedOutcomes.requestedTerminalBrowserAction,
		)
	}
	return reducedOutcomes, nil
}

func canonicalProdBuildGoals() phase1BuildGoals {
	return phase1BuildGoals{
		compileGoBinary:                 true,
		buildCriticalCSS:                true,
		buildNormalCSS:                  true,
		processPublicStaticAssets:       true,
		cleanupStalePublicStaticOutputs: true,
		processPrivateStaticAssets:      true,
		generatePublicFileMap:           true,
		validateBuildOutputs:            true,
	}
}

func (batchInput phaseBatchInput) runPhaseEffect(
	taskContext *tasks.Ctx,
	effectID phaseEffectID,
) (phaseEffectResult, error) {
	if taskContext == nil {
		return phaseEffectResult{}, errors.New(
			"wavebuild: task context is required",
		)
	}
	if batchInput.execution == nil {
		return phaseEffectResult{}, errPhaseExecutionScopeRequired
	}
	if batchInput.execution.phaseEffectTask == nil {
		return phaseEffectResult{}, errPhaseEffectTaskRequired
	}
	if os.Getenv("__WAVE_INTERNAL_TEST_MODE") == "1" {
		fmt.Printf("wave2_effect %s\n", effectID)
		return phaseEffectResult{}, nil
	}
	return batchInput.execution.phaseEffectTask.Run(
		taskContext,
		phaseEffectRequest{
			effectID:     effectID,
			mode:         batchInput.mode,
			generationID: batchInput.generationID,
		},
	)
}
