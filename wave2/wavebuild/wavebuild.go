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
	"maps"
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
	// eventTypeFWRequestedEffectsChanged represents fw-requested
	// effect payload changes carried by watcher classification.
	eventTypeFWRequestedEffectsChanged eventType = "fw_requested_effects_changed"
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

// fwMutationEffectKey identifies one fw-owned backend-mutation
// effect registration key.
type fwMutationEffectKey string

// fwNotificationDestinationKey identifies one fw-owned
// notification destination registration key.
type fwNotificationDestinationKey string

// FrameworkNotificationFailurePolicy selects what Wave does when fw
// notification transport fails.
type FrameworkNotificationFailurePolicy string

const (
	// FrameworkNotificationFailurePolicyFailPipeline surfaces notification
	// transport failure as pipeline failure.
	FrameworkNotificationFailurePolicyFailPipeline FrameworkNotificationFailurePolicy = "fail_pipeline"
	// FrameworkNotificationFailurePolicyRestartBackendWithoutGoCompile requests
	// backend restart without Go recompilation and skips frontend settling.
	FrameworkNotificationFailurePolicyRestartBackendWithoutGoCompile FrameworkNotificationFailurePolicy = "restart_backend_without_go_compile"
)

type fwNotificationRequest struct {
	destinationKey fwNotificationDestinationKey
	freshnessToken string
	trigger        string
	metadata       map[string]string
	waitForApp     bool
	waitForVite    bool
	failurePolicy  FrameworkNotificationFailurePolicy
}

type fwRequestedEffects struct {
	backendMutationEffectKeys           []fwMutationEffectKey
	backendConvergenceNotificationQueue []fwNotificationRequest
}

type fwExecutionRegistrations struct {
	backendMutationEffectsByKey                  map[fwMutationEffectKey]*tasks.Task[p3_BatchInput, struct{}]
	backendConvergenceNotificationsByDestination map[fwNotificationDestinationKey]*tasks.Task[p4_FWNotificationTaskInput, struct{}]
}

func fwRequestedEffectsFromPointer(
	fwRequestedEffectsPointer *fwRequestedEffects,
) fwRequestedEffects {
	if fwRequestedEffectsPointer == nil {
		return fwRequestedEffects{}
	}
	return *fwRequestedEffectsPointer
}

func newFWRequestedEffectsPointerIfAny(
	fwRequestedEffectsValue fwRequestedEffects,
) *fwRequestedEffects {
	if !fwRequestedEffectsValue.hasAny() {
		return nil
	}
	fwRequestedEffectsCopy := fwRequestedEffectsValue
	return &fwRequestedEffectsCopy
}

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
	// fwRequestedEffects carries fw-owned requested effects
	// discovered by watcher classification for this event.
	fwRequestedEffects fwRequestedEffects
}

// appRequestedOutcomes captures app-requested observable outcomes for one batch.
type appRequestedOutcomes struct {
	requestedTerminalBrowserAction frontendTerminalBrowserAction
	requestRestart                 bool
	requestGoCompile               bool
	fwRequestedEffects             fwRequestedEffects
}

// p1_Input is the phase-1 input contract in observable terms.
type p1_Input struct {
	mode         mode
	generationID string
	events       []observedBatchEvent

	wavePublicFileMapNotificationDestinationKey fwNotificationDestinationKey
	fwExecutionRegistrations                    *fwExecutionRegistrations

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

	wavePublicFileMapNotificationDestinationKey fwNotificationDestinationKey
	fwExecutionRegistrations                    *fwExecutionRegistrations

	appRequestedOutcomes appRequestedOutcomes
	fwRequestedEffects   fwRequestedEffects
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
	fwRequestedEffects := fwRequestedEffects{}

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
		fwRequestedEffects = fwRequestedEffects.merge(
			rawEvent.fwRequestedEffects,
		)
		if _, alreadySeenEventType := eventTypeSet[rawEvent.eventType]; alreadySeenEventType {
			continue
		}
		eventTypeSet[rawEvent.eventType] = struct{}{}
		actionableEventTypes = append(actionableEventTypes, rawEvent.eventType)
	}

	return p1_Facts{
		mode:         input.mode,
		generationID: normalizedGenerationID,
		eventTypes:   actionableEventTypes,
		wavePublicFileMapNotificationDestinationKey: fwNotificationDestinationKey(
			strings.TrimSpace(
				string(input.wavePublicFileMapNotificationDestinationKey),
			),
		),
		fwExecutionRegistrations: input.fwExecutionRegistrations,
		appRequestedOutcomes:     appRequestedOutcomes,
		fwRequestedEffects:       fwRequestedEffects,
		waitingForBuildRetry:     input.waitingForBuildRetry,
	}, nil
}

// FrameworkNotification is one framework-agnostic notification payload emitted
// by Wave2 planning.
type FrameworkNotification struct {
	destinationKey fwNotificationDestinationKey
	freshnessToken string
	trigger        string
	metadata       map[string]string
	waitForApp     bool
	waitForVite    bool
	failurePolicy  FrameworkNotificationFailurePolicy
}

// DestinationKey returns framework notification destination registration key.
func (notification FrameworkNotification) DestinationKey() string {
	return string(notification.destinationKey)
}

// FreshnessToken returns the freshness token for stale-attempt rejection.
func (notification FrameworkNotification) FreshnessToken() string {
	return notification.freshnessToken
}

// Trigger returns stable notification trigger text.
func (notification FrameworkNotification) Trigger() string {
	return notification.trigger
}

// Metadata returns optional stable key/value metadata.
func (notification FrameworkNotification) Metadata() map[string]string {
	return notification.metadata
}

// WaitForApp returns whether notification transport is app-readiness gated.
func (notification FrameworkNotification) WaitForApp() bool {
	return notification.waitForApp
}

// WaitForVite returns whether notification transport is vite-readiness gated.
func (notification FrameworkNotification) WaitForVite() bool {
	return notification.waitForVite
}

// FailurePolicy returns policy used when notification transport fails.
func (notification FrameworkNotification) FailurePolicy() FrameworkNotificationFailurePolicy {
	return notification.failurePolicy
}

func (fwRequestedEffects fwRequestedEffects) fwNotifications(
	generationID string,
) []FrameworkNotification {
	if !fwRequestedEffects.hasBackendConvergenceNotifications() {
		return nil
	}
	fallbackFreshnessToken := strings.TrimSpace(generationID)
	notifications := make(
		[]FrameworkNotification,
		0,
		len(fwRequestedEffects.backendConvergenceNotificationQueue),
	)
	for _, request := range fwRequestedEffects.backendConvergenceNotificationQueue {
		normalizedRequest := request.normalize()
		if normalizedRequest.destinationKey == "" {
			continue
		}
		if normalizedRequest.freshnessToken == "" {
			normalizedRequest.freshnessToken = fallbackFreshnessToken
		}
		notifications = append(
			notifications,
			FrameworkNotification{
				destinationKey: normalizedRequest.destinationKey,
				freshnessToken: normalizedRequest.freshnessToken,
				trigger:        normalizedRequest.trigger,
				metadata:       normalizedRequest.metadata,
				waitForApp:     normalizedRequest.waitForApp,
				waitForVite:    normalizedRequest.waitForVite,
				failurePolicy:  normalizedRequest.failurePolicy,
			},
		)
	}
	return notifications
}

func (fwRequestedEffects fwRequestedEffects) hasAny() bool {
	return len(fwRequestedEffects.backendMutationEffectKeys) > 0 ||
		len(fwRequestedEffects.backendConvergenceNotificationQueue) > 0
}

func (fwRequestedEffects fwRequestedEffects) hasBackendMutationEffects() bool {
	return len(fwRequestedEffects.backendMutationEffectKeys) > 0
}

func (fwRequestedEffects fwRequestedEffects) hasBackendConvergenceNotifications() bool {
	return len(
		fwRequestedEffects.backendConvergenceNotificationQueue,
	) > 0
}

func (leftRequestedEffects fwRequestedEffects) merge(
	rightRequestedEffects fwRequestedEffects,
) fwRequestedEffects {
	mergedMutationEffectKeys := make(
		[]fwMutationEffectKey,
		0,
		len(leftRequestedEffects.backendMutationEffectKeys)+
			len(rightRequestedEffects.backendMutationEffectKeys),
	)
	seenMutationEffectKeys := make(
		map[fwMutationEffectKey]struct{},
		len(leftRequestedEffects.backendMutationEffectKeys)+
			len(rightRequestedEffects.backendMutationEffectKeys),
	)
	appendMutationEffectKey := func(effectKey fwMutationEffectKey) {
		normalizedEffectKey := fwMutationEffectKey(
			strings.TrimSpace(string(effectKey)),
		)
		if normalizedEffectKey == "" {
			return
		}
		if _, alreadySeen := seenMutationEffectKeys[normalizedEffectKey]; alreadySeen {
			return
		}
		seenMutationEffectKeys[normalizedEffectKey] = struct{}{}
		mergedMutationEffectKeys = append(
			mergedMutationEffectKeys,
			normalizedEffectKey,
		)
	}
	for _, effectKey := range leftRequestedEffects.backendMutationEffectKeys {
		appendMutationEffectKey(effectKey)
	}
	for _, effectKey := range rightRequestedEffects.backendMutationEffectKeys {
		appendMutationEffectKey(effectKey)
	}

	mergedNotificationQueue := mergeFWNotificationQueue(
		leftRequestedEffects.backendConvergenceNotificationQueue,
		rightRequestedEffects.backendConvergenceNotificationQueue,
	)

	return fwRequestedEffects{
		backendMutationEffectKeys:           mergedMutationEffectKeys,
		backendConvergenceNotificationQueue: mergedNotificationQueue,
	}
}

func mergeFWNotificationQueue(
	leftQueue []fwNotificationRequest,
	rightQueue []fwNotificationRequest,
) []fwNotificationRequest {
	mergedQueue := make(
		[]fwNotificationRequest,
		0,
		len(leftQueue)+len(rightQueue),
	)
	notificationIndexByDestination := make(
		map[fwNotificationDestinationKey]int,
		len(leftQueue)+len(rightQueue),
	)
	appendNotification := func(notification fwNotificationRequest) {
		normalizedNotification := notification.normalize()
		if normalizedNotification.destinationKey == "" {
			return
		}
		existingIndex, alreadyExists := notificationIndexByDestination[normalizedNotification.destinationKey]
		if alreadyExists {
			mergedQueue[existingIndex] = mergedQueue[existingIndex].merge(
				normalizedNotification,
			)
			return
		}
		notificationIndexByDestination[normalizedNotification.destinationKey] = len(
			mergedQueue,
		)
		mergedQueue = append(mergedQueue, normalizedNotification)
	}
	for _, notification := range leftQueue {
		appendNotification(notification)
	}
	for _, notification := range rightQueue {
		appendNotification(notification)
	}
	return mergedQueue
}

func (notification fwNotificationRequest) normalize() fwNotificationRequest {
	normalizedNotification := notification
	normalizedNotification.destinationKey = fwNotificationDestinationKey(
		strings.TrimSpace(string(notification.destinationKey)),
	)
	normalizedNotification.freshnessToken = strings.TrimSpace(
		notification.freshnessToken,
	)
	normalizedNotification.trigger = strings.TrimSpace(notification.trigger)
	if normalizedNotification.failurePolicy == "" {
		normalizedNotification.failurePolicy = FrameworkNotificationFailurePolicyFailPipeline
	}
	return normalizedNotification
}

func (existingNotification fwNotificationRequest) merge(
	incomingNotification fwNotificationRequest,
) fwNotificationRequest {
	mergedNotification := existingNotification
	if incomingNotification.freshnessToken != "" {
		mergedNotification.freshnessToken = incomingNotification.freshnessToken
	}
	if incomingNotification.trigger != "" {
		mergedNotification.trigger = incomingNotification.trigger
	}
	if len(incomingNotification.metadata) > 0 {
		if mergedNotification.metadata == nil {
			mergedNotification.metadata = make(
				map[string]string,
				len(incomingNotification.metadata),
			)
		}
		maps.Copy(mergedNotification.metadata, incomingNotification.metadata)
	}
	mergedNotification.waitForApp =
		mergedNotification.waitForApp || incomingNotification.waitForApp
	mergedNotification.waitForVite =
		mergedNotification.waitForVite || incomingNotification.waitForVite
	if incomingNotification.failurePolicy != "" {
		mergedNotification.failurePolicy = incomingNotification.failurePolicy
	}
	return mergedNotification
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
		mode:                     input.p1.mode,
		generationID:             normalizedGenerationID,
		fwExecutionRegistrations: p1_RequestedEffects.fwExecutionRegistrations,
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
	if p4_Output.requiresBackendRestartWithoutGoCompile {
		healingP2_RequestedEffects := p2_RequestedEffects{
			restartAppProcess:        true,
			awaitBackendReadiness:    true,
			fwExecutionRegistrations: p1_RequestedEffects.fwExecutionRegistrations,
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
		p2_Output.p2_RequestedEffects = p2_Output.p2_RequestedEffects.merge(
			healingP2_RequestedEffects,
		)
		p3_Output = healingP3_Output
		p4_Output = healingP4_Output
		return fivePhaseRunResult{
			p1_RequestedEffects: p1_RequestedEffects,
			p2_RequestedEffects: p2_Output.p2_RequestedEffects,
			p3_Output:           p3_Output,
			p4_Output:           p4_Output,
			p5_CompletionSummary: p5_CompletionSummary{
				terminalAction: frontendTerminalBrowserActionNone,
			},
			fwNotifications: fwRequestedEffectsFromPointer(
				p2_Output.p2_RequestedEffects.fwRequestedEffects,
			).fwNotifications(
				normalizedGenerationID,
			),
		}, nil
	}
	if p4_Output.skipFrontendSettling {
		return fivePhaseRunResult{
			p1_RequestedEffects: p1_RequestedEffects,
			p2_RequestedEffects: p2_Output.p2_RequestedEffects,
			p3_Output:           p3_Output,
			p4_Output:           p4_Output,
			p5_CompletionSummary: p5_CompletionSummary{
				terminalAction: frontendTerminalBrowserActionNone,
			},
			fwNotifications: fwRequestedEffectsFromPointer(
				p2_Output.p2_RequestedEffects.fwRequestedEffects,
			).fwNotifications(
				normalizedGenerationID,
			),
		}, nil
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
		fwNotifications: fwRequestedEffectsFromPointer(
			p2_Output.p2_RequestedEffects.fwRequestedEffects,
		).fwNotifications(
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
	case eventTypeFWRequestedEffectsChanged:
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
	reducedOutcomes.fwRequestedEffects = fwRequestedEffects{}.merge(
		reducedOutcomes.fwRequestedEffects,
	)
	for _, fwNotificationRequest := range reducedOutcomes.fwRequestedEffects.backendConvergenceNotificationQueue {
		switch fwNotificationRequest.failurePolicy {
		case "",
			FrameworkNotificationFailurePolicyFailPipeline,
			FrameworkNotificationFailurePolicyRestartBackendWithoutGoCompile:
		default:
			return appRequestedOutcomes{}, fmt.Errorf(
				"wavebuild: unsupported fw notification failure policy %q",
				fwNotificationRequest.failurePolicy,
			)
		}
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
