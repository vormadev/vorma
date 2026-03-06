// Package wavebuild defines the build/dev orchestration contracts for Wave2.
//
// The contract is intentionally phase-oriented:
// events -> build -> backend_mutation -> backend_convergence -> frontend_settling.
// Symbol naming uses numeric phase labels:
// phase_1 -> phase_2 -> phase_3 -> phase_4 -> phase_5.
//
// mode policy is pipeline-level:
//   - dev executes all five phases.
//   - prod bypasses events/backend_mutation/backend_convergence/frontend_settling
//     and runs build only.
//
// Each phase owns its own terminal effects and executes through kit/tasks. Phase
// boundaries provide cross-phase ordering; inside a phase, dependency ordering
// and parallelism come only from task prerequisites.
package wavebuild

import (
	"context"
	"errors"
	"fmt"
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
	// eventTypeAppDefinedWatchWithRebuildChanged represents app watch classes that participate in normal build-phase planning; concrete effects still come from reduced outcomes.
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

// p1_Input is the phase-1 input contract in observable terms.
type p1_Input struct {
	mode         mode
	generationID string
	events       []observedBatchEvent

	appRequestedOutcomes appRequestedOutcomes
	waitingForBuildRetry bool
}

// p1_Facts is the minimal phase-1 output used by planners.
type p1_Facts struct {
	mode         mode
	generationID string

	// eventTypes contains deduplicated actionable event types, preserving
	// first-seen input order.
	eventTypes []eventType

	appRequestedOutcomes appRequestedOutcomes
	waitingForBuildRetry bool
}

// buildFacts reduces one phase-1 input into deterministic planner facts.
func (input p1_Input) buildFacts() (p1_Facts, error) {
	if modeError := input.mode.validate(); modeError != nil {
		return p1_Facts{}, modeError
	}
	normalizedGenerationID := strings.TrimSpace(input.generationID)
	if normalizedGenerationID == "" {
		return p1_Facts{}, errors.New(
			"wavebuild: generation id is required",
		)
	}

	appRequestedOutcomes, appRequestedOutcomesError := input.appRequestedOutcomes.reduce()
	if appRequestedOutcomesError != nil {
		return p1_Facts{}, appRequestedOutcomesError
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
			return p1_Facts{}, fmt.Errorf(
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

	return p1_Facts{
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

// frameworkSignals reduces backend-mutation effects into framework-agnostic
// signals for framework adapter handoff.
func (p2_RequestedEffects p2_RequestedEffects) frameworkSignals(
	generationID string,
) []FrameworkSignal {
	freshnessToken := strings.TrimSpace(generationID)
	if freshnessToken == "" {
		return nil
	}
	signals := make([]FrameworkSignal, 0, 3)
	if p2_RequestedEffects.refreshFrameworkRoute {
		signals = append(
			signals,
			FrameworkSignal{
				signalType:     FrameworkSignalTypeRoutesChanged,
				freshnessToken: freshnessToken,
				trigger:        "backend_mutation_refresh_framework_route",
			},
		)
	}
	if p2_RequestedEffects.refreshFrameworkTemplate {
		signals = append(
			signals,
			FrameworkSignal{
				signalType:     FrameworkSignalTypeTemplateChanged,
				freshnessToken: freshnessToken,
				trigger:        "backend_mutation_refresh_framework_template",
			},
		)
	}
	if p2_RequestedEffects.refreshFrameworkPublicFileMap {
		signals = append(
			signals,
			FrameworkSignal{
				signalType:     FrameworkSignalTypePublicFileMapChanged,
				freshnessToken: freshnessToken,
				trigger:        "backend_mutation_refresh_framework_public_filemap",
			},
		)
	}
	return signals
}

var (
	errP1_InputRequired = errors.New(
		"wavebuild: phase-1 input is required",
	)
	errGenerationIDRequired = errors.New(
		"wavebuild: generation id is required",
	)
)

type phaseEffectSets struct {
	p1 p1_Effects
	p2 p2_Effects
	p3 p3_Effects
	p4 p4_Effects
	p5 p5_Effects
}

var realPhaseEffectSets = phaseEffectSets{
	p1: p1_EffectsDef,
	p2: p2_EffectsDef,
	p3: p3_EffectsDef,
	p4: p4_EffectsDef,
	p5: p5_EffectsDef,
}

// runFivePhasePipeline executes one phase batch with one batch-scoped tasks
// context.
//
// mode policy:
// - dev runs phases 1-5.
// - prod bypasses phases 1/3/4/5 and runs phase 2 only.
func runFivePhasePipeline(
	parentContext context.Context,
	input p1_BatchInput,
) (fivePhaseRunResult, error) {
	return runFivePhasePipelineWithEffectSets(
		parentContext,
		input,
		realPhaseEffectSets,
	)
}

func runFivePhasePipelineWithEffectSets(
	parentContext context.Context,
	input p1_BatchInput,
	effects phaseEffectSets,
) (fivePhaseRunResult, error) {
	if input.p1 == nil {
		return fivePhaseRunResult{}, errP1_InputRequired
	}
	normalizedGenerationID := strings.TrimSpace(input.p1.generationID)
	if normalizedGenerationID == "" {
		return fivePhaseRunResult{}, errGenerationIDRequired
	}
	if modeError := input.p1.mode.validate(); modeError != nil {
		return fivePhaseRunResult{}, modeError
	}

	batchTaskContext := tasks.NewCtx(parentContext)
	p1_RequestedEffects := canonicalP1_RequestedEffects()
	if input.p1.mode != modeProd {
		var p1_Error error
		p1_RequestedEffects, p1_Error = effects.p1.planP1_RequestedEffects.Run(
			batchTaskContext,
			input,
		)
		if p1_Error != nil {
			return fivePhaseRunResult{}, p1_Error
		}
	}

	phaseBatchInput := phaseBatchInput{
		mode:         input.p1.mode,
		generationID: normalizedGenerationID,
	}
	p2_Output, p2_Error := effects.p2.planP2_Output.Run(
		batchTaskContext,
		p2_BatchInput{
			batch:               phaseBatchInput,
			p1_RequestedEffects: p1_RequestedEffects,
		},
	)
	if p2_Error != nil {
		return fivePhaseRunResult{}, p2_Error
	}
	if input.p1.mode == modeProd {
		return fivePhaseRunResult{
			p1_RequestedEffects: p1_RequestedEffects,
			p2_RequestedEffects: p2_Output.p2_RequestedEffects,
		}, nil
	}

	p3_Output, p3_Error := effects.p3.planP4_RequestedEffects.Run(
		batchTaskContext,
		p3_BatchInput{
			batch:               phaseBatchInput,
			p2_RequestedEffects: p2_Output.p2_RequestedEffects,
		},
	)
	if p3_Error != nil {
		return fivePhaseRunResult{}, p3_Error
	}
	p4_Output, p4_Error := effects.p4.planP5_RequestedEffects.Run(
		batchTaskContext,
		p4_BatchInput{
			batch:               phaseBatchInput,
			p4_RequestedEffects: p3_Output.p4_RequestedEffects,
		},
	)
	if p4_Error != nil {
		return fivePhaseRunResult{}, p4_Error
	}
	p5_CompletionSummary, p5_Error := effects.p5.executeTerminalBrowserAction.Run(
		batchTaskContext,
		p5_BatchInput{
			batch:               phaseBatchInput,
			p5_RequestedEffects: p4_Output.p5_RequestedEffects,
		},
	)
	if p5_Error != nil {
		return fivePhaseRunResult{}, p5_Error
	}
	if p5_CompletionSummary.requiresBackendViteHealing {
		healingP2_RequestedEffects := p2_RequestedEffects{
			restartViteProcess:    true,
			awaitBackendReadiness: true,
		}
		healingP3_Output, healingP3_Error := effects.p3.planP4_RequestedEffects.Run(
			batchTaskContext,
			p3_BatchInput{
				batch:               phaseBatchInput,
				p2_RequestedEffects: healingP2_RequestedEffects,
			},
		)
		if healingP3_Error != nil {
			return fivePhaseRunResult{}, healingP3_Error
		}
		healingP4_Output, healingP4_Error := effects.p4.planP5_RequestedEffects.Run(
			batchTaskContext,
			p4_BatchInput{
				batch:               phaseBatchInput,
				p4_RequestedEffects: healingP3_Output.p4_RequestedEffects,
			},
		)
		if healingP4_Error != nil {
			return fivePhaseRunResult{}, healingP4_Error
		}
		healingCompletionSummary, healingP5_Error := effects.p5.executeTerminalBrowserAction.Run(
			batchTaskContext,
			p5_BatchInput{
				batch:               phaseBatchInput,
				p5_RequestedEffects: healingP4_Output.p5_RequestedEffects,
			},
		)
		if healingP5_Error != nil {
			return fivePhaseRunResult{}, healingP5_Error
		}
		p2_Output.p2_RequestedEffects = p2_Output.p2_RequestedEffects.merge(
			healingP2_RequestedEffects,
		)
		p3_Output = healingP3_Output
		p4_Output = healingP4_Output
		p5_CompletionSummary = healingCompletionSummary
	}
	return fivePhaseRunResult{
		p1_RequestedEffects:  p1_RequestedEffects,
		p2_RequestedEffects:  p2_Output.p2_RequestedEffects,
		p3_Output:            p3_Output,
		p4_Output:            p4_Output,
		p5_CompletionSummary: p5_CompletionSummary,
		frameworkSignals: p2_Output.p2_RequestedEffects.frameworkSignals(
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

func canonicalP1_RequestedEffects() p1_RequestedEffects {
	return p1_RequestedEffects{
		compileGoBinary:                 true,
		buildCriticalCSS:                true,
		buildNormalCSS:                  true,
		processPublicStaticAssets:       true,
		cleanupStalePublicStaticOutputs: true,
		processPrivateStaticAssets:      true,
		generatePublicFileMap:           true,
		runRequestedBuildEffects:        true,
	}
}
