// Package wavebuild defines Wave2 build/dev orchestration contracts.
//
// This package intentionally focuses on design-level interfaces and data models.
// Concrete planning and execution strategies are expected to be plugged in via
// the interfaces below.
package wavebuild

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/vormadev/vorma/kit/tasks"
)

var (
	// ErrNotImplemented reports that a contract surface exists but no concrete
	// implementation has been attached yet.
	ErrNotImplemented = errors.New("wavebuild: not implemented")
)

// Mode selects dev or prod planning behavior.
type Mode string

const (
	// ModeDev applies development-time policy.
	ModeDev Mode = "dev"
	// ModeProd applies production-time policy.
	ModeProd Mode = "prod"
)

// ModeApplicability declares whether a goal applies to dev, prod, or both.
type ModeApplicability string

const (
	// ModeApplicabilityDevOnly limits a goal to development planning.
	ModeApplicabilityDevOnly ModeApplicability = "dev"
	// ModeApplicabilityProdOnly limits a goal to production planning.
	ModeApplicabilityProdOnly ModeApplicability = "prod"
	// ModeApplicabilityBoth allows a goal in both modes.
	ModeApplicabilityBoth ModeApplicability = "both"
)

// EventType is one normalized event class.
type EventType string

const (
	// EventTypeConfigFileChanged represents semantic config changes.
	EventTypeConfigFileChanged EventType = "config_file_changed"
	// EventTypeGoSourceChanged represents app Go source changes.
	EventTypeGoSourceChanged EventType = "go_source_changed"
	// EventTypeCriticalCSSSourceChanged represents critical CSS source changes.
	EventTypeCriticalCSSSourceChanged EventType = "critical_css_source_changed"
	// EventTypeNormalCSSSourceChanged represents normal CSS source changes.
	EventTypeNormalCSSSourceChanged EventType = "normal_css_source_changed"
	// EventTypePublicStaticAssetChanged represents public static asset changes.
	EventTypePublicStaticAssetChanged EventType = "public_static_asset_changed"
	// EventTypePrivateStaticAssetChanged represents private static asset changes.
	EventTypePrivateStaticAssetChanged EventType = "private_static_asset_changed"
	// EventTypeFrameworkRouteDefinitionChanged represents framework route
	// definition source changes.
	EventTypeFrameworkRouteDefinitionChanged EventType = "framework_route_definition_changed"
	// EventTypeFrameworkTemplateChanged represents framework template source
	// changes.
	EventTypeFrameworkTemplateChanged EventType = "framework_template_changed"
	// EventTypeAppDefinedWatchCallbackOnlyChanged represents app-defined watch
	// classes that execute callback-only behavior.
	EventTypeAppDefinedWatchCallbackOnlyChanged EventType = "app_defined_watch_callback_only_changed"
	// EventTypeAppDefinedWatchWithRebuildChanged represents app-defined watch
	// classes that can request rebuild/restart/browser behavior.
	EventTypeAppDefinedWatchWithRebuildChanged EventType = "app_defined_watch_with_rebuild_changed"
	// EventTypeIgnoredOrNoiseChanged represents ignored/noise change input.
	EventTypeIgnoredOrNoiseChanged EventType = "ignored_or_noise_changed"
	// EventTypeUnclassifiedNoWatchRuleChanged represents unclassified inputs with
	// no matching watch rule.
	EventTypeUnclassifiedNoWatchRuleChanged EventType = "unclassified_no_watch_rule_changed"
)

// Condition gates one effect group in a mode plan.
type Condition string

const (
	// ConditionAlways means one effect group is always eligible.
	ConditionAlways Condition = ""

	// ConditionPublicFileMapChangedOrRepaired gates actions that should run only
	// when canonical public file-map artifacts changed or required repair.
	ConditionPublicFileMapChangedOrRepaired Condition = "public_file_map_changed_or_repaired"
	// ConditionAppCallbackRequestedFrameworkRefresh gates app-callback requested
	// framework refresh effects.
	ConditionAppCallbackRequestedFrameworkRefresh Condition = "app_callback_requested_framework_refresh"
	// ConditionAppCallbackRequestedBrowserInvalidate gates app-callback requested
	// browser invalidate behavior.
	ConditionAppCallbackRequestedBrowserInvalidate Condition = "app_callback_requested_browser_invalidate"
	// ConditionAppCallbackRequestedBrowserRevalidate gates app-callback requested
	// browser revalidate behavior.
	ConditionAppCallbackRequestedBrowserRevalidate Condition = "app_callback_requested_browser_revalidate"
	// ConditionAppCallbackRequestedBrowserHardReload gates app-callback
	// requested browser hard reload behavior.
	ConditionAppCallbackRequestedBrowserHardReload Condition = "app_callback_requested_browser_hard_reload"
	// ConditionAppCallbackRequestedRestart gates app-callback requested app
	// restart behavior.
	ConditionAppCallbackRequestedRestart Condition = "app_callback_requested_restart"
	// ConditionAppCallbackRequestedGoCompile gates app-callback requested go
	// compile behavior.
	ConditionAppCallbackRequestedGoCompile Condition = "app_callback_requested_go_compile"
	// ConditionWaitingForBuildRetry gates retry-wait restart intent behavior.
	ConditionWaitingForBuildRetry Condition = "waiting_for_build_retry"
)

const (
	// MetadataKeyAppDefinedWatchCallbackOnlyChanged marks callback-only
	// app-defined watch class activity for one batch.
	MetadataKeyAppDefinedWatchCallbackOnlyChanged = "app_defined_watch_callback_only_changed"
	// MetadataKeyAppDefinedWatchWithRebuildChanged marks app-defined watch class
	// activity that can request rebuild/restart/browser behavior.
	MetadataKeyAppDefinedWatchWithRebuildChanged = "app_defined_watch_with_rebuild_changed"
	// MetadataKeyPublicStaticChangedPathsCWD carries CWD-relative public static
	// changed paths as a comma-separated list.
	MetadataKeyPublicStaticChangedPathsCWD = "public_static_changed_paths_cwd"
	// MetadataKeyPrivateStaticChangedPathsCWD carries CWD-relative private static
	// changed paths as a comma-separated list.
	MetadataKeyPrivateStaticChangedPathsCWD = "private_static_changed_paths_cwd"
)

// EffectID is one terminal effect/goal identifier.
type EffectID string

// GoalSpec declares one canonical effect contract.
type GoalSpec struct {
	Effect EffectID
	// Guarantee describes the observable post-condition this effect ensures.
	Guarantee string
	// AppliesTo declares dev/prod/both applicability.
	AppliesTo ModeApplicability
	// DependsOn declares prerequisite effects.
	DependsOn []EffectID
	// MutexGroup declares an exclusivity group for arbitration.
	MutexGroup string
}

const (
	// MutexGroupBrowserAction is the arbitration domain for browser-visible
	// action winners.
	MutexGroupBrowserAction = "browser_action"
)

const (
	// EffectRestartDevServerCycle ensures the dev cycle restarts with updated
	// lifecycle configuration.
	EffectRestartDevServerCycle EffectID = "restart_dev_server_cycle"
	// EffectCompileGoBinary ensures the app binary is compiled for the current
	// sources and mode.
	EffectCompileGoBinary EffectID = "compile_go_binary"
	// EffectRestartAppProcess ensures the running app process uses the latest
	// compiled state and configuration.
	EffectRestartAppProcess EffectID = "restart_app_process"
	// EffectBuildCriticalCSS ensures critical CSS output is regenerated from
	// current sources.
	EffectBuildCriticalCSS EffectID = "build_critical_css"
	// EffectBuildNormalCSS ensures normal CSS output is regenerated from current
	// sources.
	EffectBuildNormalCSS EffectID = "build_normal_css"
	// EffectProcessPublicStaticAssets ensures public static outputs reflect
	// current source assets.
	EffectProcessPublicStaticAssets EffectID = "process_public_static_assets"
	// EffectProcessPrivateStaticAssets ensures private static outputs reflect
	// current source assets.
	EffectProcessPrivateStaticAssets EffectID = "process_private_static_assets"
	// EffectEnsurePublicFileMapArtifacts ensures canonical public file-map
	// artifacts are current and repaired if needed.
	EffectEnsurePublicFileMapArtifacts EffectID = "ensure_public_file_map_artifacts"
	// EffectRequestFrameworkRouteRefresh ensures framework route state is
	// refreshed for the current generation.
	EffectRequestFrameworkRouteRefresh EffectID = "request_framework_route_refresh"
	// EffectRequestFrameworkTemplateRefresh ensures framework template
	// state is refreshed for the current generation.
	EffectRequestFrameworkTemplateRefresh EffectID = "request_framework_template_refresh"
	// EffectRequestFrameworkPublicFileMapRefresh ensures framework public
	// file-map state is refreshed for the current generation.
	EffectRequestFrameworkPublicFileMapRefresh EffectID = "request_framework_public_file_map_refresh"
	// EffectBrowserCSSHotReload ensures browser CSS is refreshed without full
	// document reload when eligible.
	EffectBrowserCSSHotReload EffectID = "browser_css_hot_reload"
	// EffectBrowserRevalidate ensures framework/client revalidation behavior runs
	// for the latest generation.
	EffectBrowserRevalidate EffectID = "browser_revalidate"
	// EffectBrowserInvalidatePublicAssets ensures browser invalidates
	// public asset/file-map state without hard reload when eligible.
	EffectBrowserInvalidatePublicAssets EffectID = "browser_invalidate_public_assets"
	// EffectBrowserHardReload ensures browser fully reloads against the latest
	// app state.
	EffectBrowserHardReload EffectID = "browser_hard_reload"
	// EffectQueueRetryWaitRestart ensures retry-wait mode receives restart work
	// intent without executing normal batch work immediately.
	EffectQueueRetryWaitRestart EffectID = "queue_retry_wait_restart"
)

var canonicalGoalCatalog = []GoalSpec{
	{
		Effect:    EffectRestartDevServerCycle,
		Guarantee: "Development cycle restarts using current configuration and sources.",
		AppliesTo: ModeApplicabilityDevOnly,
	},
	{
		Effect:    EffectCompileGoBinary,
		Guarantee: "App binary output reflects current Go source state for the selected mode.",
		AppliesTo: ModeApplicabilityBoth,
	},
	{
		Effect:    EffectRestartAppProcess,
		Guarantee: "Running app process serves current compiled artifacts and configuration.",
		AppliesTo: ModeApplicabilityDevOnly,
	},
	{
		Effect:    EffectBuildCriticalCSS,
		Guarantee: "Critical CSS output reflects current CSS source state.",
		AppliesTo: ModeApplicabilityBoth,
	},
	{
		Effect:    EffectBuildNormalCSS,
		Guarantee: "Normal CSS output reflects current CSS source state.",
		AppliesTo: ModeApplicabilityBoth,
	},
	{
		Effect:    EffectProcessPublicStaticAssets,
		Guarantee: "Public static outputs reflect current public asset sources.",
		AppliesTo: ModeApplicabilityBoth,
	},
	{
		Effect:    EffectProcessPrivateStaticAssets,
		Guarantee: "Private static outputs reflect current private asset sources.",
		AppliesTo: ModeApplicabilityBoth,
	},
	{
		Effect:    EffectEnsurePublicFileMapArtifacts,
		Guarantee: "Canonical public file-map artifacts are current and not stale.",
		AppliesTo: ModeApplicabilityBoth,
	},
	{
		Effect:    EffectRequestFrameworkRouteRefresh,
		Guarantee: "Framework route state reflects latest route definition sources.",
		AppliesTo: ModeApplicabilityDevOnly,
	},
	{
		Effect:    EffectRequestFrameworkTemplateRefresh,
		Guarantee: "Framework template state reflects latest template sources.",
		AppliesTo: ModeApplicabilityDevOnly,
	},
	{
		Effect:    EffectRequestFrameworkPublicFileMapRefresh,
		Guarantee: "Framework public file-map state reflects latest canonical map artifacts.",
		AppliesTo: ModeApplicabilityDevOnly,
		DependsOn: []EffectID{
			EffectEnsurePublicFileMapArtifacts,
		},
	},
	{
		Effect:     EffectBrowserCSSHotReload,
		Guarantee:  "Browser CSS is refreshed without full reload when compatible.",
		AppliesTo:  ModeApplicabilityDevOnly,
		MutexGroup: MutexGroupBrowserAction,
	},
	{
		Effect:     EffectBrowserRevalidate,
		Guarantee:  "Browser/client revalidation flow runs against the latest generation.",
		AppliesTo:  ModeApplicabilityDevOnly,
		MutexGroup: MutexGroupBrowserAction,
	},
	{
		Effect:    EffectBrowserInvalidatePublicAssets,
		Guarantee: "Browser invalidates public asset/file-map state against current artifacts.",
		AppliesTo: ModeApplicabilityDevOnly,
		DependsOn: []EffectID{
			EffectEnsurePublicFileMapArtifacts,
		},
		MutexGroup: MutexGroupBrowserAction,
	},
	{
		Effect:     EffectBrowserHardReload,
		Guarantee:  "Browser reloads full document against current app readiness state.",
		AppliesTo:  ModeApplicabilityDevOnly,
		MutexGroup: MutexGroupBrowserAction,
	},
	{
		Effect:    EffectQueueRetryWaitRestart,
		Guarantee: "Retry-wait mode captures restart intent for current change generation.",
		AppliesTo: ModeApplicabilityDevOnly,
	},
}

// CanonicalGoalCatalog returns the baseline goal set for Wave2 planning.
func CanonicalGoalCatalog() []GoalSpec {
	return cloneGoalSpecs(canonicalGoalCatalog)
}

// CanonicalGoalDependencyCatalog returns declared goal dependencies from the
// canonical goal catalog.
func CanonicalGoalDependencyCatalog() map[EffectID][]EffectID {
	return buildGoalDependencyCatalog(CanonicalGoalCatalog())
}

// ArbitrationTieBreakPolicy selects deterministic winner strategy when two
// effects are not ordered by explicit precedence.
type ArbitrationTieBreakPolicy string

const (
	// ArbitrationTieBreakByEffectIDLexicographic applies stable lexical ordering
	// on EffectID values for unresolved ties.
	ArbitrationTieBreakByEffectIDLexicographic ArbitrationTieBreakPolicy = "effect_id_lexicographic"
)

// ArbitrationRuleSet is the pure-data arbitration policy for one planner.
type ArbitrationRuleSet struct {
	PrecedenceDomains []ArbitrationPrecedenceDomain
	ImplicationRules  []ArbitrationImplicationRule
	ExclusionRules    []ArbitrationExclusionRule
	TieBreakPolicy    ArbitrationTieBreakPolicy
}

// ArbitrationPrecedenceDomain declares one winner domain and its ordered
// effect precedence.
type ArbitrationPrecedenceDomain struct {
	Domain          string
	HighestToLowest []EffectID
}

// ArbitrationImplicationRule declares that one effect requires another effect.
type ArbitrationImplicationRule struct {
	IfPresent EffectID
	Require   EffectID
}

// ArbitrationExclusionRule declares that one effect suppresses other effects.
type ArbitrationExclusionRule struct {
	IfPresent EffectID
	Exclude   []EffectID
}

var canonicalArbitrationRuleSet = ArbitrationRuleSet{
	PrecedenceDomains: []ArbitrationPrecedenceDomain{
		{
			Domain: MutexGroupBrowserAction,
			HighestToLowest: []EffectID{
				EffectBrowserHardReload,
				EffectBrowserInvalidatePublicAssets,
				EffectBrowserRevalidate,
				EffectBrowserCSSHotReload,
			},
		},
	},
	ImplicationRules: []ArbitrationImplicationRule{
		{
			IfPresent: EffectRestartAppProcess,
			Require:   EffectBrowserHardReload,
		},
		{
			IfPresent: EffectRequestFrameworkPublicFileMapRefresh,
			Require:   EffectEnsurePublicFileMapArtifacts,
		},
		{
			IfPresent: EffectBrowserInvalidatePublicAssets,
			Require:   EffectEnsurePublicFileMapArtifacts,
		},
	},
	ExclusionRules: []ArbitrationExclusionRule{
		{
			IfPresent: EffectQueueRetryWaitRestart,
			Exclude: []EffectID{
				EffectBrowserHardReload,
				EffectBrowserInvalidatePublicAssets,
				EffectBrowserRevalidate,
				EffectBrowserCSSHotReload,
				EffectRequestFrameworkRouteRefresh,
				EffectRequestFrameworkTemplateRefresh,
				EffectRequestFrameworkPublicFileMapRefresh,
			},
		},
	},
	TieBreakPolicy: ArbitrationTieBreakByEffectIDLexicographic,
}

var canonicalEventRuleCatalog = []EventRule{
	{
		Event: EventTypeConfigFileChanged,
		Dev: ModePlan{
			ConditionalEffects: []ConditionalEffectSet{
				{
					Condition: ConditionAlways,
					Effects:   []EffectID{EffectRestartDevServerCycle},
				},
			},
		},
	},
	{
		Event: EventTypeGoSourceChanged,
		Dev: ModePlan{
			ConditionalEffects: []ConditionalEffectSet{
				{
					Condition: ConditionAlways,
					Effects: []EffectID{
						EffectCompileGoBinary,
						EffectRestartAppProcess,
						EffectBrowserHardReload,
					},
				},
			},
		},
		Prod: ModePlan{
			ConditionalEffects: []ConditionalEffectSet{
				{
					Condition: ConditionAlways,
					Effects:   []EffectID{EffectCompileGoBinary},
				},
			},
		},
	},
	{
		Event: EventTypeCriticalCSSSourceChanged,
		Dev: ModePlan{
			ConditionalEffects: []ConditionalEffectSet{
				{
					Condition: ConditionAlways,
					Effects: []EffectID{
						EffectBuildCriticalCSS,
						EffectBrowserCSSHotReload,
					},
				},
			},
		},
		Prod: ModePlan{
			ConditionalEffects: []ConditionalEffectSet{
				{
					Condition: ConditionAlways,
					Effects:   []EffectID{EffectBuildCriticalCSS},
				},
			},
		},
	},
	{
		Event: EventTypeNormalCSSSourceChanged,
		Dev: ModePlan{
			ConditionalEffects: []ConditionalEffectSet{
				{
					Condition: ConditionAlways,
					Effects: []EffectID{
						EffectBuildNormalCSS,
						EffectBrowserCSSHotReload,
					},
				},
			},
		},
		Prod: ModePlan{
			ConditionalEffects: []ConditionalEffectSet{
				{
					Condition: ConditionAlways,
					Effects:   []EffectID{EffectBuildNormalCSS},
				},
			},
		},
	},
	{
		Event: EventTypePublicStaticAssetChanged,
		Dev: ModePlan{
			ConditionalEffects: []ConditionalEffectSet{
				{
					Condition: ConditionAlways,
					Effects: []EffectID{
						EffectProcessPublicStaticAssets,
						EffectEnsurePublicFileMapArtifacts,
					},
				},
				{
					Condition: ConditionPublicFileMapChangedOrRepaired,
					Effects: []EffectID{
						EffectRequestFrameworkPublicFileMapRefresh,
						EffectBrowserInvalidatePublicAssets,
					},
				},
			},
		},
		Prod: ModePlan{
			ConditionalEffects: []ConditionalEffectSet{
				{
					Condition: ConditionAlways,
					Effects: []EffectID{
						EffectProcessPublicStaticAssets,
						EffectEnsurePublicFileMapArtifacts,
					},
				},
			},
		},
	},
	{
		Event: EventTypePrivateStaticAssetChanged,
		Dev: ModePlan{
			ConditionalEffects: []ConditionalEffectSet{
				{
					Condition: ConditionAlways,
					Effects: []EffectID{
						EffectProcessPrivateStaticAssets,
						EffectBrowserHardReload,
					},
				},
			},
		},
		Prod: ModePlan{
			ConditionalEffects: []ConditionalEffectSet{
				{
					Condition: ConditionAlways,
					Effects: []EffectID{
						EffectProcessPrivateStaticAssets,
					},
				},
			},
		},
	},
	{
		Event: EventTypeFrameworkRouteDefinitionChanged,
		Dev: ModePlan{
			ConditionalEffects: []ConditionalEffectSet{
				{
					Condition: ConditionAlways,
					Effects: []EffectID{
						EffectRequestFrameworkRouteRefresh,
						EffectBrowserHardReload,
					},
				},
			},
		},
	},
	{
		Event: EventTypeFrameworkTemplateChanged,
		Dev: ModePlan{
			ConditionalEffects: []ConditionalEffectSet{
				{
					Condition: ConditionAlways,
					Effects: []EffectID{
						EffectRequestFrameworkTemplateRefresh,
						EffectBrowserHardReload,
					},
				},
			},
		},
	},
	{
		Event: EventTypeAppDefinedWatchCallbackOnlyChanged,
		Dev: ModePlan{
			ConditionalEffects: []ConditionalEffectSet{
				{
					Condition: ConditionAppCallbackRequestedFrameworkRefresh,
					Effects:   []EffectID{EffectRequestFrameworkRouteRefresh},
				},
				{
					Condition: ConditionAppCallbackRequestedBrowserInvalidate,
					Effects:   []EffectID{EffectBrowserInvalidatePublicAssets},
				},
				{
					Condition: ConditionAppCallbackRequestedBrowserRevalidate,
					Effects:   []EffectID{EffectBrowserRevalidate},
				},
				{
					Condition: ConditionAppCallbackRequestedBrowserHardReload,
					Effects:   []EffectID{EffectBrowserHardReload},
				},
				{
					Condition: ConditionAppCallbackRequestedRestart,
					Effects:   []EffectID{EffectRestartAppProcess},
				},
				{
					Condition: ConditionAppCallbackRequestedGoCompile,
					Effects:   []EffectID{EffectCompileGoBinary},
				},
			},
		},
	},
	{
		Event: EventTypeAppDefinedWatchWithRebuildChanged,
		Dev: ModePlan{
			ConditionalEffects: []ConditionalEffectSet{
				{
					Condition: ConditionAppCallbackRequestedFrameworkRefresh,
					Effects:   []EffectID{EffectRequestFrameworkRouteRefresh},
				},
				{
					Condition: ConditionAppCallbackRequestedBrowserInvalidate,
					Effects:   []EffectID{EffectBrowserInvalidatePublicAssets},
				},
				{
					Condition: ConditionAppCallbackRequestedBrowserRevalidate,
					Effects:   []EffectID{EffectBrowserRevalidate},
				},
				{
					Condition: ConditionAppCallbackRequestedBrowserHardReload,
					Effects:   []EffectID{EffectBrowserHardReload},
				},
				{
					Condition: ConditionAppCallbackRequestedRestart,
					Effects:   []EffectID{EffectRestartAppProcess},
				},
				{
					Condition: ConditionAppCallbackRequestedGoCompile,
					Effects:   []EffectID{EffectCompileGoBinary},
				},
			},
		},
	},
	{
		Event: EventTypeIgnoredOrNoiseChanged,
		Dev:   ModePlan{},
		Prod:  ModePlan{},
	},
	{
		Event: EventTypeUnclassifiedNoWatchRuleChanged,
		Dev:   ModePlan{},
		Prod:  ModePlan{},
	},
}

// CanonicalEventRuleCatalog returns the baseline event-rule catalog.
func CanonicalEventRuleCatalog() []EventRule {
	return cloneEventRules(canonicalEventRuleCatalog)
}

// CanonicalArbitrationRuleSet returns the baseline arbitration policy for
// Wave2 planning.
func CanonicalArbitrationRuleSet() ArbitrationRuleSet {
	return cloneArbitrationRuleSet(canonicalArbitrationRuleSet)
}

// FrameworkSignalType is one framework-agnostic refresh signal intent.
type FrameworkSignalType string

const (
	// FrameworkSignalTypeRoutesChanged signals that route-definition state must
	// refresh in framework process state.
	FrameworkSignalTypeRoutesChanged FrameworkSignalType = "routes_changed"
	// FrameworkSignalTypeTemplateChanged signals that template/render shell state
	// must refresh in framework process state.
	FrameworkSignalTypeTemplateChanged FrameworkSignalType = "template_changed"
	// FrameworkSignalTypePublicFileMapChanged signals that public-file-map state
	// must refresh in framework process state.
	FrameworkSignalTypePublicFileMapChanged FrameworkSignalType = "public_file_map_changed"
)

// FrameworkSignal is one transport-agnostic framework refresh signal intent.
type FrameworkSignal struct {
	Type FrameworkSignalType
	// FreshnessToken carries staleness-protection identity (for example, build
	// id or generation id) used by framework adapters.
	FreshnessToken string
	// Trigger describes why this signal exists for observability.
	Trigger string
	// Metadata carries optional stable key/value context for adapters.
	Metadata map[string]string
}

// GoalExecutionKey is a comparable key intended for task execution contexts.
//
// This shape is intentionally simple so it can be used as task input identity.
type GoalExecutionKey struct {
	Mode         Mode
	GenerationID string
	Goal         EffectID
}

// RawWatchEvent is one unclassified source event.
type RawWatchEvent struct {
	Operation string
	Path      string
}

// RawBatchInput is the pre-parse event batch at ingestion boundaries.
type RawBatchInput struct {
	Mode         Mode
	GenerationID string
	Events       []RawWatchEvent
	Metadata     map[string]string
	TraceID      string
	BatchID      string
	IsInitial    bool
}

// BatchFacts is the single-pass parsed fact set used by planning.
type BatchFacts struct {
	// ChangedPathsCWD stores normalized changed paths relative to the current
	// working directory for one batch.
	ChangedPathsCWD []string

	// ChangeSummary captures batch-shape facts that affect planning decisions.
	ChangeSummary BatchChangeSummaryFacts

	// NoiseFacts captures watcher-noise and suppression-relevant facts.
	NoiseFacts BatchNoiseFacts

	// BuildFacts captures build-time implications inferred for this batch.
	BuildFacts BatchBuildImplicationFacts

	// DevCycleFacts captures process and framework-refresh implications.
	DevCycleFacts BatchDevCycleImplicationFacts

	// BrowserFacts captures browser-facing implications before arbitration.
	BrowserFacts BatchBrowserImplicationFacts

	// StaticFacts captures static asset and file map implications.
	StaticFacts BatchStaticImplicationFacts

	// HookFacts captures run-on-change and command-policy implications.
	HookFacts BatchHookImplicationFacts
}

// BatchChangeSummaryFacts captures coarse batch shape used by planning policy.
type BatchChangeSummaryFacts struct {
	FilesChangedCount  int
	HasMeaningfulWork  bool
	HasSingleFileInput bool
}

// BatchNoiseFacts captures noise-oriented watcher facts.
type BatchNoiseFacts struct {
	HasLockFileNoise               bool
	HasEditorTempNoise             bool
	HasIgnoredPathNoise            bool
	HasDirectoryMaintenanceNoise   bool
	HasNonContentCHMODOnlyNoise    bool
	HasEmptyFileCHMODContentSignal bool
}

// BatchBuildImplicationFacts captures build-time requirements inferred from one
// batch.
type BatchBuildImplicationFacts struct {
	NeedsDevServerCycleRestart bool
	NeedsGoCompile             bool
	NeedsCriticalCSSBuild      bool
	NeedsNormalCSSBuild        bool
	NeedsPublicStaticProcess   bool
	NeedsPrivateStaticProcess  bool
	NeedsChangedPathStaticScan bool
	NeedsFullStaticScan        bool
}

// BatchDevCycleImplicationFacts captures process-level and in-process refresh
// implications for one batch.
type BatchDevCycleImplicationFacts struct {
	NeedsAppRestart                    bool
	NeedsFrameworkRouteRefresh         bool
	NeedsFrameworkTemplateRefresh      bool
	NeedsFrameworkPublicFileMapRefresh bool
	NeedsDevCycleRefreshStalenessGuard bool
	NeedsRetryWaitRestartRequest       bool
	RetryWaitRestartRequiresGoCompile  bool
}

// BatchBrowserImplicationFacts captures candidate browser behavior prior to
// precedence arbitration.
type BatchBrowserImplicationFacts struct {
	RequestsHardReload            bool
	RequestsPublicAssetInvalidate bool
	RequestsRevalidate            bool
	RequestsCSSHotReload          bool
	WaitForAppReady               bool
	WaitForViteReady              bool
	SuppressBrowserAction         bool
}

// BatchStaticImplicationFacts captures static-asset changed paths and public
// file map implications.
type BatchStaticImplicationFacts struct {
	PublicStaticChangedPathsCWD  []string
	PrivateStaticChangedPathsCWD []string
	PublicFileMapMayHaveChanged  bool
	PublicFileMapMayNeedRepair   bool
}

// BatchHookImplicationFacts captures hook-policy facts for one batch.
type BatchHookImplicationFacts struct {
	HasRunOnChangeOnlyWork                bool
	HasRunOnChangeCommandSuppression      bool
	HasCallbackOnlyHookWork               bool
	AppCallbackRequestedFrameworkRefresh  bool
	AppCallbackRequestedBrowserInvalidate bool
	AppCallbackRequestedBrowserRevalidate bool
	AppCallbackRequestedBrowserHardReload bool
	AppCallbackRequestedRestart           bool
	AppCallbackRequestedGoCompile         bool
	AppDefinedWatchCallbackOnlyChanged    bool
	AppDefinedWatchWithRebuildChanged     bool
}

// PlannerInput is the canonical plan-input structure after facts and
// classification are complete.
type PlannerInput struct {
	Mode           Mode
	GenerationID   string
	Events         []EventType
	ConditionFacts ConditionFacts
	Facts          BatchFacts
	Metadata       map[string]string
}

// ConditionFacts stores per-condition truth values for one planning operation.
// Missing keys are interpreted as false, except ConditionAlways.
type ConditionFacts map[Condition]bool

// ConditionalEffectSet declares candidate effects gated by one condition.
type ConditionalEffectSet struct {
	Condition Condition
	Effects   []EffectID
}

// ModePlan is one condition-gated effect plan for one mode.
type ModePlan struct {
	ConditionalEffects []ConditionalEffectSet
}

// EventRule maps one event class into dev/prod plans.
type EventRule struct {
	Event EventType
	Dev   ModePlan
	Prod  ModePlan
}

// ExecutionPlan is one deterministic plan ready for execution.
type ExecutionPlan struct {
	Mode             Mode
	GenerationID     string
	Events           []EventType
	ConditionFacts   ConditionFacts
	Facts            BatchFacts
	Metadata         map[string]string
	FrameworkSignals []FrameworkSignal
	TerminalGoals    []EffectID
}

// ArbitrationInput captures data needed to resolve mutually exclusive goals.
type ArbitrationInput struct {
	Mode    Mode
	Draft   ExecutionPlan
	GoalSet []GoalSpec
}

// EffectTaskRootInput is one task-root input for terminal effect execution.
type EffectTaskRootInput struct {
	Effect EffectID
	Plan   *ExecutionPlan
}

// EffectTaskRootRegistration binds one effect id to one terminal task root.
type EffectTaskRootRegistration struct {
	Effect   EffectID
	TaskRoot *tasks.Task[GoalExecutionKey, struct{}]
}

// FactsCollector parses raw batch input into a canonical fact set.
type FactsCollector interface {
	CollectFacts(input RawBatchInput) (BatchFacts, error)
}

// EventClassifier maps facts into normalized event classes.
type EventClassifier interface {
	ClassifyEvents(mode Mode, facts BatchFacts) ([]EventType, error)
}

// DefaultFactsCollector derives typed batch facts from raw watcher input and
// optional metadata hints.
type DefaultFactsCollector struct{}

// CollectFacts parses one raw batch input into typed planning facts.
func (collector *DefaultFactsCollector) CollectFacts(
	input RawBatchInput,
) (BatchFacts, error) {
	if collector == nil {
		return BatchFacts{}, errors.New(
			"wavebuild: default facts collector is required",
		)
	}
	normalizedChangedPaths := make([]string, 0, len(input.Events))
	seenPath := make(map[string]struct{}, len(input.Events))
	facts := BatchFacts{}
	for _, rawEvent := range input.Events {
		normalizedPath := normalizeWatcherPathForFacts(rawEvent.Path)
		if normalizedPath == "" {
			continue
		}
		if _, alreadySeen := seenPath[normalizedPath]; !alreadySeen {
			seenPath[normalizedPath] = struct{}{}
			normalizedChangedPaths = append(
				normalizedChangedPaths,
				normalizedPath,
			)
		}
		if strings.EqualFold(strings.TrimSpace(rawEvent.Operation), "chmod") {
			if parseBooleanMetadata(input.Metadata, "chmod_non_content_only") {
				facts.NoiseFacts.HasNonContentCHMODOnlyNoise = true
				continue
			}
			facts.NoiseFacts.HasEmptyFileCHMODContentSignal = true
		}
	}

	facts.ChangedPathsCWD = normalizedChangedPaths
	facts.ChangeSummary.FilesChangedCount = len(normalizedChangedPaths)
	facts.ChangeSummary.HasSingleFileInput = len(normalizedChangedPaths) == 1
	hasConfigFileChanged := parseBooleanMetadata(
		input.Metadata,
		string(EventTypeConfigFileChanged),
	)
	hasGoSourceChanged := parseBooleanMetadata(
		input.Metadata,
		string(EventTypeGoSourceChanged),
	)
	hasCriticalCSSSourceChanged := parseBooleanMetadata(
		input.Metadata,
		string(EventTypeCriticalCSSSourceChanged),
	)
	hasNormalCSSSourceChanged := parseBooleanMetadata(
		input.Metadata,
		string(EventTypeNormalCSSSourceChanged),
	)
	hasFrameworkRouteDefinitionChanged := parseBooleanMetadata(
		input.Metadata,
		string(EventTypeFrameworkRouteDefinitionChanged),
	)
	hasFrameworkTemplateChanged := parseBooleanMetadata(
		input.Metadata,
		string(EventTypeFrameworkTemplateChanged),
	)
	facts.BuildFacts.NeedsDevServerCycleRestart = hasConfigFileChanged
	if hasGoSourceChanged {
		facts.BuildFacts.NeedsGoCompile = true
		facts.DevCycleFacts.NeedsAppRestart = true
		facts.BrowserFacts.RequestsHardReload = true
		facts.BrowserFacts.WaitForAppReady = true
	}
	if hasCriticalCSSSourceChanged {
		facts.BuildFacts.NeedsCriticalCSSBuild = true
		facts.BrowserFacts.RequestsCSSHotReload = true
	}
	if hasNormalCSSSourceChanged {
		facts.BuildFacts.NeedsNormalCSSBuild = true
		facts.BrowserFacts.RequestsCSSHotReload = true
	}
	if hasFrameworkRouteDefinitionChanged {
		facts.DevCycleFacts.NeedsFrameworkRouteRefresh = true
		facts.BrowserFacts.RequestsHardReload = true
		facts.BrowserFacts.WaitForAppReady = true
		facts.BrowserFacts.WaitForViteReady = true
	}
	if hasFrameworkTemplateChanged {
		facts.DevCycleFacts.NeedsFrameworkTemplateRefresh = true
		facts.BrowserFacts.RequestsHardReload = true
		facts.BrowserFacts.WaitForAppReady = true
		facts.BrowserFacts.WaitForViteReady = true
	}

	facts.StaticFacts.PublicStaticChangedPathsCWD = append(
		facts.StaticFacts.PublicStaticChangedPathsCWD,
		parseChangedPathsMetadata(
			input.Metadata,
			MetadataKeyPublicStaticChangedPathsCWD,
		)...,
	)
	facts.StaticFacts.PrivateStaticChangedPathsCWD = append(
		facts.StaticFacts.PrivateStaticChangedPathsCWD,
		parseChangedPathsMetadata(
			input.Metadata,
			MetadataKeyPrivateStaticChangedPathsCWD,
		)...,
	)

	hasExplicitPublicStaticChange := parseBooleanMetadata(
		input.Metadata,
		string(EventTypePublicStaticAssetChanged),
	)
	hasExplicitPrivateStaticChange := parseBooleanMetadata(
		input.Metadata,
		string(EventTypePrivateStaticAssetChanged),
	)
	if hasExplicitPublicStaticChange ||
		len(facts.StaticFacts.PublicStaticChangedPathsCWD) > 0 {
		facts.BuildFacts.NeedsPublicStaticProcess = true
		facts.StaticFacts.PublicFileMapMayHaveChanged = true
		if len(facts.StaticFacts.PublicStaticChangedPathsCWD) > 0 {
			facts.BuildFacts.NeedsChangedPathStaticScan = true
		} else {
			facts.BuildFacts.NeedsFullStaticScan = true
		}
	}
	if hasExplicitPrivateStaticChange ||
		len(facts.StaticFacts.PrivateStaticChangedPathsCWD) > 0 {
		facts.BuildFacts.NeedsPrivateStaticProcess = true
		facts.BrowserFacts.RequestsHardReload = true
		facts.BrowserFacts.WaitForAppReady = true
		if len(facts.StaticFacts.PrivateStaticChangedPathsCWD) > 0 {
			facts.BuildFacts.NeedsChangedPathStaticScan = true
		} else {
			facts.BuildFacts.NeedsFullStaticScan = true
		}
	}

	facts.StaticFacts.PublicFileMapMayHaveChanged =
		facts.StaticFacts.PublicFileMapMayHaveChanged ||
			parseBooleanMetadata(
				input.Metadata,
				string(ConditionPublicFileMapChangedOrRepaired),
			)
	facts.StaticFacts.PublicFileMapMayNeedRepair = parseBooleanMetadata(
		input.Metadata,
		"public_file_map_may_need_repair",
	)
	facts.DevCycleFacts.NeedsRetryWaitRestartRequest = parseBooleanMetadata(
		input.Metadata,
		string(ConditionWaitingForBuildRetry),
	)
	facts.DevCycleFacts.RetryWaitRestartRequiresGoCompile = parseBooleanMetadata(
		input.Metadata,
		"retry_wait_restart_requires_go_compile",
	)
	facts.HookFacts.AppCallbackRequestedFrameworkRefresh = parseBooleanMetadata(
		input.Metadata,
		string(ConditionAppCallbackRequestedFrameworkRefresh),
	)
	facts.HookFacts.AppCallbackRequestedBrowserInvalidate = parseBooleanMetadata(
		input.Metadata,
		string(ConditionAppCallbackRequestedBrowserInvalidate),
	)
	facts.HookFacts.AppCallbackRequestedBrowserRevalidate = parseBooleanMetadata(
		input.Metadata,
		string(ConditionAppCallbackRequestedBrowserRevalidate),
	)
	facts.HookFacts.AppCallbackRequestedBrowserHardReload = parseBooleanMetadata(
		input.Metadata,
		string(ConditionAppCallbackRequestedBrowserHardReload),
	)
	facts.HookFacts.AppCallbackRequestedRestart = parseBooleanMetadata(
		input.Metadata,
		string(ConditionAppCallbackRequestedRestart),
	)
	facts.HookFacts.AppCallbackRequestedGoCompile = parseBooleanMetadata(
		input.Metadata,
		string(ConditionAppCallbackRequestedGoCompile),
	)
	facts.HookFacts.AppDefinedWatchCallbackOnlyChanged = parseBooleanMetadata(
		input.Metadata,
		MetadataKeyAppDefinedWatchCallbackOnlyChanged,
	)
	facts.HookFacts.AppDefinedWatchWithRebuildChanged = parseBooleanMetadata(
		input.Metadata,
		MetadataKeyAppDefinedWatchWithRebuildChanged,
	)
	facts.HookFacts.HasCallbackOnlyHookWork =
		facts.HookFacts.HasCallbackOnlyHookWork ||
			facts.HookFacts.AppDefinedWatchCallbackOnlyChanged

	facts.ChangeSummary.HasMeaningfulWork = deriveHasMeaningfulWorkFromFacts(
		facts,
	)
	return facts, nil
}

// DefaultEventClassifier maps typed batch facts into normalized event classes.
type DefaultEventClassifier struct{}

// ClassifyEvents returns normalized event classes for one batch.
func (classifier *DefaultEventClassifier) ClassifyEvents(
	_ Mode,
	facts BatchFacts,
) ([]EventType, error) {
	if classifier == nil {
		return nil, errors.New(
			"wavebuild: default event classifier is required",
		)
	}
	classifiedEvents := make([]EventType, 0, 10)
	appendEventType := func(eventType EventType) {
		if eventType == "" {
			return
		}
		if slices.Contains(classifiedEvents, eventType) {
			return
		}
		classifiedEvents = append(classifiedEvents, eventType)
	}

	if facts.BuildFacts.NeedsDevServerCycleRestart {
		appendEventType(EventTypeConfigFileChanged)
	}
	if facts.BuildFacts.NeedsGoCompile || facts.DevCycleFacts.NeedsAppRestart {
		appendEventType(EventTypeGoSourceChanged)
	}
	if facts.BuildFacts.NeedsCriticalCSSBuild {
		appendEventType(EventTypeCriticalCSSSourceChanged)
	}
	if facts.BuildFacts.NeedsNormalCSSBuild {
		appendEventType(EventTypeNormalCSSSourceChanged)
	}
	if facts.BuildFacts.NeedsPublicStaticProcess {
		appendEventType(EventTypePublicStaticAssetChanged)
	}
	if facts.BuildFacts.NeedsPrivateStaticProcess {
		appendEventType(EventTypePrivateStaticAssetChanged)
	}
	if facts.DevCycleFacts.NeedsFrameworkRouteRefresh {
		appendEventType(EventTypeFrameworkRouteDefinitionChanged)
	}
	if facts.DevCycleFacts.NeedsFrameworkTemplateRefresh {
		appendEventType(EventTypeFrameworkTemplateChanged)
	}
	if facts.HookFacts.AppDefinedWatchCallbackOnlyChanged {
		appendEventType(EventTypeAppDefinedWatchCallbackOnlyChanged)
	}
	if facts.HookFacts.AppDefinedWatchWithRebuildChanged {
		appendEventType(EventTypeAppDefinedWatchWithRebuildChanged)
	}

	if len(classifiedEvents) == 0 && facts.ChangeSummary.FilesChangedCount > 0 {
		if !facts.ChangeSummary.HasMeaningfulWork {
			appendEventType(EventTypeIgnoredOrNoiseChanged)
		} else {
			appendEventType(EventTypeUnclassifiedNoWatchRuleChanged)
		}
	}
	return classifiedEvents, nil
}

// Planner reduces classified events + facts into an execution plan.
type Planner interface {
	Plan(input PlannerInput) (ExecutionPlan, error)
}

// Arbiter resolves mutually exclusive or precedence-constrained effects.
type Arbiter interface {
	Arbitrate(input ArbitrationInput) (ExecutionPlan, error)
}

// Executor runs one execution plan using registered terminal effect task roots.
type Executor interface {
	Execute(
		ctx context.Context,
		plan ExecutionPlan,
		effectTaskRoots map[EffectID]*tasks.Task[GoalExecutionKey, struct{}],
	) error
}

// EngineConfig configures one contract-level orchestration engine.
type EngineConfig struct {
	InitialEventRules      []EventRule
	InitialGoalSpecs       []GoalSpec
	InitialEffectTaskRoots []EffectTaskRootRegistration
	FactsCollector         FactsCollector
	EventClassifier        EventClassifier
	Planner                Planner
	Arbiter                Arbiter
	Executor               Executor
}

// Engine stores registrations and optional contract implementations.
//
// By design, this type remains minimal until concrete algorithms are attached.
type Engine struct {
	mu                 sync.RWMutex
	eventRuleByType    map[EventType]EventRule
	goalSpecByEffectID map[EffectID]GoalSpec
	effectTaskRootByID map[EffectID]*tasks.Task[GoalExecutionKey, struct{}]
	factsCollector     FactsCollector
	eventClassifier    EventClassifier
	planner            Planner
	arbiter            Arbiter
	executor           Executor
}

// EventRulePlanner reduces classified events using registered event rules.
type EventRulePlanner struct {
	eventRuleByType map[EventType]EventRule
}

// NewEventRulePlanner constructs one canonical event-rule planner.
func NewEventRulePlanner(rules []EventRule) *EventRulePlanner {
	eventRuleByType := make(map[EventType]EventRule, len(rules))
	for _, rule := range rules {
		if rule.Event == "" {
			continue
		}
		eventRuleByType[rule.Event] = cloneEventRule(rule)
	}
	return &EventRulePlanner{
		eventRuleByType: eventRuleByType,
	}
}

// Plan reduces one classified batch into a deduplicated terminal goal set and
// derived framework signals.
func (planner *EventRulePlanner) Plan(
	input PlannerInput,
) (ExecutionPlan, error) {
	if planner == nil {
		return ExecutionPlan{}, errors.New(
			"wavebuild: event rule planner is required",
		)
	}
	candidateGoals := make([]EffectID, 0, len(input.Events))
	for _, event := range input.Events {
		eventRule, foundEventRule := planner.eventRuleByType[event]
		if !foundEventRule {
			continue
		}
		modePlan, resolveModePlanError := resolveModePlanForPlannerInput(
			input.Mode,
			eventRule,
		)
		if resolveModePlanError != nil {
			return ExecutionPlan{}, resolveModePlanError
		}
		for _, effectSet := range modePlan.ConditionalEffects {
			if !conditionMatches(effectSet.Condition, input.ConditionFacts) {
				continue
			}
			if len(effectSet.Effects) == 0 {
				continue
			}
			candidateGoals = append(candidateGoals, effectSet.Effects...)
		}
	}
	deduplicatedTerminalGoals := deduplicateCandidateGoals(candidateGoals)
	frameworkSignals := deriveFrameworkSignalsFromTerminalGoals(
		deduplicatedTerminalGoals,
		input.GenerationID,
	)
	plan := ExecutionPlan{
		Mode:             input.Mode,
		GenerationID:     input.GenerationID,
		Events:           append([]EventType(nil), input.Events...),
		ConditionFacts:   cloneConditionFacts(input.ConditionFacts),
		Facts:            cloneBatchFacts(input.Facts),
		Metadata:         cloneStringMap(input.Metadata),
		FrameworkSignals: frameworkSignals,
	}
	plan.TerminalGoals = append([]EffectID(nil), deduplicatedTerminalGoals...)
	return plan, nil
}

// RuleSetArbiter applies one pure-data arbitration rule set to draft plans.
type RuleSetArbiter struct {
	RuleSet ArbitrationRuleSet
}

// NewRuleSetArbiter constructs one rule-set arbiter.
func NewRuleSetArbiter(ruleSet ArbitrationRuleSet) *RuleSetArbiter {
	return &RuleSetArbiter{RuleSet: cloneArbitrationRuleSet(ruleSet)}
}

// NewCanonicalRuleSetArbiter constructs one arbiter using canonical rules.
func NewCanonicalRuleSetArbiter() *RuleSetArbiter {
	return &RuleSetArbiter{RuleSet: CanonicalArbitrationRuleSet()}
}

// Arbitrate resolves one draft plan using the configured rule set.
func (arbiter *RuleSetArbiter) Arbitrate(
	input ArbitrationInput,
) (ExecutionPlan, error) {
	if arbiter == nil {
		return ExecutionPlan{}, errors.New(
			"wavebuild: rule-set arbiter is required",
		)
	}
	plan := cloneExecutionPlan(input.Draft)
	terminalGoals, arbitrateError := arbitrateTerminalGoalsWithRuleSet(
		deriveOrderedUniqueTerminalGoals(plan.TerminalGoals),
		arbiter.RuleSet,
	)
	if arbitrateError != nil {
		return ExecutionPlan{}, arbitrateError
	}
	if len(terminalGoals) == 0 {
		plan.TerminalGoals = nil
		return plan, nil
	}
	plan.TerminalGoals = append([]EffectID(nil), terminalGoals...)
	return plan, nil
}

// DefaultTaskRootExecutor executes multiple deduplicated terminal goals.
//
// Sequencing and parallelism are delegated to task dependency graphs. The
// executor runs selected terminal roots together in one `RunParallel` call.
type DefaultTaskRootExecutor struct{}

// Execute runs one execution plan using the registered terminal task roots.
func (executor *DefaultTaskRootExecutor) Execute(
	ctx context.Context,
	plan ExecutionPlan,
	effectTaskRoots map[EffectID]*tasks.Task[GoalExecutionKey, struct{}],
) error {
	if executor == nil {
		return errors.New("wavebuild: default task root executor is required")
	}
	if ctx == nil {
		panic("wavebuild: executor requires non-nil context")
	}
	taskExecutionContext := tasks.NewCtx(ctx)
	generationID := strings.TrimSpace(plan.GenerationID)
	terminalGoals := deriveOrderedUniqueTerminalGoals(plan.TerminalGoals)
	boundTasks := make([]tasks.BoundTask, 0, len(terminalGoals))
	for _, effect := range terminalGoals {
		taskRoot := effectTaskRoots[effect]
		if taskRoot == nil {
			return fmt.Errorf(
				"wavebuild: missing task root for effect %q",
				effect,
			)
		}
		taskInput := GoalExecutionKey{
			Mode:         plan.Mode,
			GenerationID: generationID,
			Goal:         effect,
		}
		var ignoredResult struct{}
		boundTasks = append(
			boundTasks,
			taskRoot.Bind(taskInput, &ignoredResult),
		)
	}
	if runParallelError := taskExecutionContext.RunParallel(
		boundTasks...,
	); runParallelError != nil {
		return runParallelError
	}
	return nil
}

func deriveOrderedUniqueTerminalGoals(
	terminalGoals []EffectID,
) []EffectID {
	if len(terminalGoals) == 0 {
		return nil
	}
	seen := make(map[EffectID]struct{})
	deduplicatedTerminalGoals := make([]EffectID, 0, len(terminalGoals))
	for _, terminalGoal := range terminalGoals {
		if _, alreadySeen := seen[terminalGoal]; alreadySeen {
			continue
		}
		seen[terminalGoal] = struct{}{}
		deduplicatedTerminalGoals = append(
			deduplicatedTerminalGoals,
			terminalGoal,
		)
	}
	return deduplicatedTerminalGoals
}

func resolveModePlanForPlannerInput(
	mode Mode,
	eventRule EventRule,
) (ModePlan, error) {
	switch mode {
	case ModeDev:
		return eventRule.Dev, nil
	case ModeProd:
		return eventRule.Prod, nil
	default:
		return ModePlan{}, fmt.Errorf("wavebuild: unsupported mode %q", mode)
	}
}

func conditionMatches(condition Condition, conditionFacts ConditionFacts) bool {
	if condition == ConditionAlways {
		return true
	}
	return conditionFacts[condition]
}

func deduplicateCandidateGoals(candidateGoals []EffectID) []EffectID {
	if len(candidateGoals) == 0 {
		return nil
	}
	seen := make(map[EffectID]struct{})
	deduplicatedGoals := make([]EffectID, 0, len(candidateGoals))
	for _, candidateGoal := range candidateGoals {
		if candidateGoal == "" {
			continue
		}
		if _, alreadySeen := seen[candidateGoal]; alreadySeen {
			continue
		}
		seen[candidateGoal] = struct{}{}
		deduplicatedGoals = append(deduplicatedGoals, candidateGoal)
	}
	return deduplicatedGoals
}

func deriveFrameworkSignalsFromTerminalGoals(
	terminalGoals []EffectID,
	generationID string,
) []FrameworkSignal {
	if len(terminalGoals) == 0 {
		return nil
	}
	freshnessToken := strings.TrimSpace(generationID)
	seenSignalTypes := make(map[FrameworkSignalType]struct{})
	signals := make([]FrameworkSignal, 0, len(terminalGoals))
	for _, goal := range terminalGoals {
		signalType, ok := signalTypeForTerminalGoal(goal)
		if !ok {
			continue
		}
		if _, alreadyAdded := seenSignalTypes[signalType]; alreadyAdded {
			continue
		}
		seenSignalTypes[signalType] = struct{}{}
		signals = append(
			signals,
			FrameworkSignal{
				Type:           signalType,
				FreshnessToken: freshnessToken,
				Trigger:        string(goal),
				Metadata: map[string]string{
					"source_effect": string(goal),
				},
			},
		)
	}
	return signals
}

func signalTypeForTerminalGoal(goal EffectID) (FrameworkSignalType, bool) {
	switch goal {
	case EffectRequestFrameworkRouteRefresh:
		return FrameworkSignalTypeRoutesChanged, true
	case EffectRequestFrameworkTemplateRefresh:
		return FrameworkSignalTypeTemplateChanged, true
	case EffectRequestFrameworkPublicFileMapRefresh:
		return FrameworkSignalTypePublicFileMapChanged, true
	default:
		return "", false
	}
}

func arbitrateTerminalGoalsWithRuleSet(
	orderedTerminalGoals []EffectID,
	ruleSet ArbitrationRuleSet,
) ([]EffectID, error) {
	if len(orderedTerminalGoals) == 0 {
		return nil, nil
	}
	goalSet := make(map[EffectID]struct{}, len(orderedTerminalGoals))
	for _, goal := range orderedTerminalGoals {
		goalSet[goal] = struct{}{}
	}
	applyImplicationClosure(goalSet, ruleSet.ImplicationRules)
	applyExclusionRules(goalSet, ruleSet.ExclusionRules)
	applyPrecedenceDomains(goalSet, ruleSet.PrecedenceDomains)

	orderedPresent := make([]EffectID, 0, len(goalSet))
	seen := make(map[EffectID]struct{}, len(goalSet))
	for _, goal := range orderedTerminalGoals {
		if _, isPresent := goalSet[goal]; !isPresent {
			continue
		}
		orderedPresent = append(orderedPresent, goal)
		seen[goal] = struct{}{}
	}

	impliedOnly := make([]EffectID, 0, len(goalSet))
	for goal := range goalSet {
		if _, alreadyOrdered := seen[goal]; alreadyOrdered {
			continue
		}
		impliedOnly = append(impliedOnly, goal)
	}
	switch ruleSet.TieBreakPolicy {
	case "", ArbitrationTieBreakByEffectIDLexicographic:
		slices.Sort(impliedOnly)
	default:
		return nil, fmt.Errorf(
			"wavebuild: unsupported arbitration tie-break policy %q",
			ruleSet.TieBreakPolicy,
		)
	}
	return append(orderedPresent, impliedOnly...), nil
}

func applyImplicationClosure(
	goalSet map[EffectID]struct{},
	rules []ArbitrationImplicationRule,
) {
	if len(goalSet) == 0 || len(rules) == 0 {
		return
	}
	for {
		changed := false
		for _, rule := range rules {
			if rule.IfPresent == "" || rule.Require == "" {
				continue
			}
			if _, present := goalSet[rule.IfPresent]; !present {
				continue
			}
			if _, alreadyPresent := goalSet[rule.Require]; alreadyPresent {
				continue
			}
			goalSet[rule.Require] = struct{}{}
			changed = true
		}
		if !changed {
			return
		}
	}
}

func applyExclusionRules(
	goalSet map[EffectID]struct{},
	rules []ArbitrationExclusionRule,
) {
	if len(goalSet) == 0 || len(rules) == 0 {
		return
	}
	for _, rule := range rules {
		if rule.IfPresent == "" {
			continue
		}
		if _, present := goalSet[rule.IfPresent]; !present {
			continue
		}
		for _, excludedGoal := range rule.Exclude {
			if excludedGoal == "" || excludedGoal == rule.IfPresent {
				continue
			}
			delete(goalSet, excludedGoal)
		}
	}
}

func applyPrecedenceDomains(
	goalSet map[EffectID]struct{},
	domains []ArbitrationPrecedenceDomain,
) {
	if len(goalSet) == 0 || len(domains) == 0 {
		return
	}
	for _, domain := range domains {
		winner := EffectID("")
		for _, candidate := range domain.HighestToLowest {
			if _, present := goalSet[candidate]; !present {
				continue
			}
			winner = candidate
			break
		}
		if winner == "" {
			continue
		}
		for _, candidate := range domain.HighestToLowest {
			if candidate == winner {
				continue
			}
			delete(goalSet, candidate)
		}
	}
}

func deriveConditionFactsFromBatchFactsAndMetadata(
	facts BatchFacts,
	metadata map[string]string,
) ConditionFacts {
	conditionFacts := ConditionFacts{
		ConditionPublicFileMapChangedOrRepaired: facts.StaticFacts.PublicFileMapMayHaveChanged ||
			facts.StaticFacts.PublicFileMapMayNeedRepair,
		ConditionWaitingForBuildRetry:                  facts.DevCycleFacts.NeedsRetryWaitRestartRequest,
		ConditionAppCallbackRequestedFrameworkRefresh:  facts.HookFacts.AppCallbackRequestedFrameworkRefresh,
		ConditionAppCallbackRequestedBrowserInvalidate: facts.HookFacts.AppCallbackRequestedBrowserInvalidate,
		ConditionAppCallbackRequestedBrowserRevalidate: facts.HookFacts.AppCallbackRequestedBrowserRevalidate,
		ConditionAppCallbackRequestedBrowserHardReload: facts.HookFacts.AppCallbackRequestedBrowserHardReload,
		ConditionAppCallbackRequestedRestart:           facts.HookFacts.AppCallbackRequestedRestart,
		ConditionAppCallbackRequestedGoCompile:         facts.HookFacts.AppCallbackRequestedGoCompile,
	}
	for condition := range conditionFacts {
		if parseBooleanMetadata(metadata, string(condition)) {
			conditionFacts[condition] = true
		}
	}
	return conditionFacts
}

func parseBooleanMetadata(
	metadata map[string]string,
	key string,
) bool {
	if len(metadata) == 0 {
		return false
	}
	rawValue, exists := metadata[key]
	if !exists {
		return false
	}
	parsedValue, parseError := strconv.ParseBool(strings.TrimSpace(rawValue))
	return parseError == nil && parsedValue
}

func parseChangedPathsMetadata(
	metadata map[string]string,
	key string,
) []string {
	if len(metadata) == 0 {
		return nil
	}
	rawValue, exists := metadata[key]
	if !exists {
		return nil
	}
	normalizedRawValue := strings.NewReplacer(";", ",", "\n", ",").Replace(
		rawValue,
	)
	rawParts := strings.Split(normalizedRawValue, ",")
	if len(rawParts) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(rawParts))
	parsedPaths := make([]string, 0, len(rawParts))
	for _, rawPart := range rawParts {
		normalizedPath := normalizeWatcherPathForFacts(rawPart)
		if normalizedPath == "" {
			continue
		}
		if _, alreadySeen := seen[normalizedPath]; alreadySeen {
			continue
		}
		seen[normalizedPath] = struct{}{}
		parsedPaths = append(parsedPaths, normalizedPath)
	}
	if len(parsedPaths) == 0 {
		return nil
	}
	return parsedPaths
}

func normalizeWatcherPathForFacts(rawPath string) string {
	trimmedPath := strings.TrimSpace(rawPath)
	if trimmedPath == "" {
		return ""
	}
	normalizedSlashPath := strings.ReplaceAll(trimmedPath, "\\", "/")
	cleanedPath := path.Clean(normalizedSlashPath)
	if cleanedPath == "." {
		return ""
	}
	return cleanedPath
}

func deriveHasMeaningfulWorkFromFacts(facts BatchFacts) bool {
	return facts.BuildFacts.NeedsDevServerCycleRestart ||
		facts.BuildFacts.NeedsGoCompile ||
		facts.BuildFacts.NeedsCriticalCSSBuild ||
		facts.BuildFacts.NeedsNormalCSSBuild ||
		facts.BuildFacts.NeedsPublicStaticProcess ||
		facts.BuildFacts.NeedsPrivateStaticProcess ||
		facts.BuildFacts.NeedsChangedPathStaticScan ||
		facts.BuildFacts.NeedsFullStaticScan ||
		facts.DevCycleFacts.NeedsAppRestart ||
		facts.DevCycleFacts.NeedsFrameworkRouteRefresh ||
		facts.DevCycleFacts.NeedsFrameworkTemplateRefresh ||
		facts.DevCycleFacts.NeedsFrameworkPublicFileMapRefresh ||
		facts.DevCycleFacts.NeedsDevCycleRefreshStalenessGuard ||
		facts.DevCycleFacts.NeedsRetryWaitRestartRequest ||
		facts.BrowserFacts.RequestsHardReload ||
		facts.BrowserFacts.RequestsPublicAssetInvalidate ||
		facts.BrowserFacts.RequestsRevalidate ||
		facts.BrowserFacts.RequestsCSSHotReload ||
		facts.StaticFacts.PublicFileMapMayHaveChanged ||
		facts.StaticFacts.PublicFileMapMayNeedRepair ||
		facts.HookFacts.HasRunOnChangeOnlyWork ||
		facts.HookFacts.HasRunOnChangeCommandSuppression ||
		facts.HookFacts.HasCallbackOnlyHookWork ||
		facts.HookFacts.AppDefinedWatchCallbackOnlyChanged ||
		facts.HookFacts.AppDefinedWatchWithRebuildChanged
}

// NewEngine constructs one contract-first engine.
func NewEngine(config EngineConfig) (*Engine, error) {
	resolvedFactsCollector := config.FactsCollector
	if resolvedFactsCollector == nil {
		resolvedFactsCollector = &DefaultFactsCollector{}
	}
	resolvedEventClassifier := config.EventClassifier
	if resolvedEventClassifier == nil {
		resolvedEventClassifier = &DefaultEventClassifier{}
	}

	resolvedEventRules := CanonicalEventRuleCatalog()
	if len(config.InitialEventRules) > 0 {
		resolvedEventRules = append(
			resolvedEventRules,
			cloneEventRules(config.InitialEventRules)...,
		)
	}
	resolvedGoalSpecs := CanonicalGoalCatalog()
	if len(config.InitialGoalSpecs) > 0 {
		resolvedGoalSpecs = append(
			resolvedGoalSpecs,
			cloneGoalSpecs(config.InitialGoalSpecs)...,
		)
	}

	resolvedPlanner := config.Planner
	if resolvedPlanner == nil {
		resolvedPlanner = NewEventRulePlanner(resolvedEventRules)
	}
	resolvedArbiter := config.Arbiter
	if resolvedArbiter == nil {
		resolvedArbiter = NewCanonicalRuleSetArbiter()
	}
	resolvedExecutor := config.Executor
	if resolvedExecutor == nil {
		resolvedExecutor = &DefaultTaskRootExecutor{}
	}

	engine := &Engine{
		eventRuleByType:    make(map[EventType]EventRule),
		goalSpecByEffectID: make(map[EffectID]GoalSpec),
		effectTaskRootByID: make(
			map[EffectID]*tasks.Task[GoalExecutionKey, struct{}],
		),
		factsCollector:  resolvedFactsCollector,
		eventClassifier: resolvedEventClassifier,
		planner:         resolvedPlanner,
		arbiter:         resolvedArbiter,
		executor:        resolvedExecutor,
	}
	if registerRulesError := engine.RegisterEventRules(
		resolvedEventRules...,
	); registerRulesError != nil {
		return nil, registerRulesError
	}
	if registerGoalSpecsError := engine.RegisterGoalSpecs(
		resolvedGoalSpecs...,
	); registerGoalSpecsError != nil {
		return nil, registerGoalSpecsError
	}
	if registerTaskRootsError := engine.RegisterEffectTaskRoots(
		config.InitialEffectTaskRoots...,
	); registerTaskRootsError != nil {
		return nil, registerTaskRootsError
	}
	return engine, nil
}

// RegisterEventRules inserts or replaces event rules.
func (engine *Engine) RegisterEventRules(rules ...EventRule) error {
	if engine == nil {
		return errors.New("wavebuild: engine is required")
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	for _, rule := range rules {
		if rule.Event == "" {
			return errors.New("wavebuild: event rule event is required")
		}
		engine.eventRuleByType[rule.Event] = cloneEventRule(rule)
	}
	return nil
}

// RegisterGoalSpecs inserts or replaces goal specifications.
func (engine *Engine) RegisterGoalSpecs(goalSpecs ...GoalSpec) error {
	if engine == nil {
		return errors.New("wavebuild: engine is required")
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	for _, goalSpec := range goalSpecs {
		if goalSpec.Effect == "" {
			return errors.New("wavebuild: goal spec effect is required")
		}
		engine.goalSpecByEffectID[goalSpec.Effect] = cloneGoalSpec(goalSpec)
	}
	return nil
}

// RegisterEffectTaskRoots inserts or replaces terminal effect task roots.
func (engine *Engine) RegisterEffectTaskRoots(
	registrations ...EffectTaskRootRegistration,
) error {
	if engine == nil {
		return errors.New("wavebuild: engine is required")
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	for _, registration := range registrations {
		if registration.Effect == "" {
			return errors.New("wavebuild: effect task root effect is required")
		}
		if registration.TaskRoot == nil {
			return fmt.Errorf(
				"wavebuild: effect task root for %q is required",
				registration.Effect,
			)
		}
		engine.effectTaskRootByID[registration.Effect] = registration.TaskRoot
	}
	return nil
}

// ValidateTaskRootDependencyCoverage checks that registered terminal task roots
// also register roots for their declared goal dependencies.
func (engine *Engine) ValidateTaskRootDependencyCoverage() error {
	if engine == nil {
		return errors.New("wavebuild: engine is required")
	}
	engine.mu.RLock()
	goalSet := snapshotGoalSet(engine.goalSpecByEffectID)
	effectTaskRoots := cloneEffectTaskRoots(engine.effectTaskRootByID)
	engine.mu.RUnlock()

	dependencyCatalog := buildGoalDependencyCatalog(goalSet)
	if len(dependencyCatalog) == 0 {
		return nil
	}
	effects := make([]EffectID, 0, len(dependencyCatalog))
	for effect := range dependencyCatalog {
		effects = append(effects, effect)
	}
	slices.Sort(effects)

	missingCoverageLines := make([]string, 0)
	for _, effect := range effects {
		if effectTaskRoots[effect] == nil {
			continue
		}
		dependencies := dependencyCatalog[effect]
		for _, dependency := range dependencies {
			if effectTaskRoots[dependency] != nil {
				continue
			}
			missingCoverageLines = append(
				missingCoverageLines,
				fmt.Sprintf("%s requires %s", effect, dependency),
			)
		}
	}
	if len(missingCoverageLines) == 0 {
		return nil
	}
	return fmt.Errorf(
		"wavebuild: missing task-root dependency coverage: %s",
		strings.Join(missingCoverageLines, "; "),
	)
}

// SetFactsCollector sets the facts collector implementation.
func (engine *Engine) SetFactsCollector(factsCollector FactsCollector) {
	if engine == nil {
		return
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.factsCollector = factsCollector
}

// SetEventClassifier sets the event classifier implementation.
func (engine *Engine) SetEventClassifier(eventClassifier EventClassifier) {
	if engine == nil {
		return
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.eventClassifier = eventClassifier
}

// SetPlanner sets the planner implementation.
func (engine *Engine) SetPlanner(planner Planner) {
	if engine == nil {
		return
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.planner = planner
}

// SetArbiter sets the arbiter implementation.
func (engine *Engine) SetArbiter(arbiter Arbiter) {
	if engine == nil {
		return
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.arbiter = arbiter
}

// SetExecutor sets the executor implementation.
func (engine *Engine) SetExecutor(executor Executor) {
	if engine == nil {
		return
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.executor = executor
}

// BuildPlanFromRawInput runs the high-level pipeline: facts -> classification
// -> planning -> arbitration.
func (engine *Engine) BuildPlanFromRawInput(
	input RawBatchInput,
) (ExecutionPlan, error) {
	if engine == nil {
		return ExecutionPlan{}, errors.New("wavebuild: engine is required")
	}

	engine.mu.RLock()
	factsCollector := engine.factsCollector
	eventClassifier := engine.eventClassifier
	planner := engine.planner
	arbiter := engine.arbiter
	goalSet := snapshotGoalSet(engine.goalSpecByEffectID)
	engine.mu.RUnlock()

	if factsCollector == nil {
		return ExecutionPlan{}, fmt.Errorf(
			"wavebuild: facts collector: %w",
			ErrNotImplemented,
		)
	}
	if eventClassifier == nil {
		return ExecutionPlan{}, fmt.Errorf(
			"wavebuild: event classifier: %w",
			ErrNotImplemented,
		)
	}
	if planner == nil {
		return ExecutionPlan{}, fmt.Errorf(
			"wavebuild: planner: %w",
			ErrNotImplemented,
		)
	}
	if arbiter == nil {
		return ExecutionPlan{}, fmt.Errorf(
			"wavebuild: arbiter: %w",
			ErrNotImplemented,
		)
	}
	generationID := deriveGenerationIDFromRawBatchInput(input)

	facts, collectFactsError := factsCollector.CollectFacts(input)
	if collectFactsError != nil {
		return ExecutionPlan{}, collectFactsError
	}
	events, classifyEventsError := eventClassifier.ClassifyEvents(
		input.Mode,
		facts,
	)
	if classifyEventsError != nil {
		return ExecutionPlan{}, classifyEventsError
	}
	planDraft, planError := planner.Plan(
		PlannerInput{
			Mode:         input.Mode,
			GenerationID: generationID,
			Events:       append([]EventType(nil), events...),
			ConditionFacts: deriveConditionFactsFromBatchFactsAndMetadata(
				facts,
				input.Metadata,
			),
			Facts:    cloneBatchFacts(facts),
			Metadata: cloneStringMap(input.Metadata),
		},
	)
	if planError != nil {
		return ExecutionPlan{}, planError
	}
	return arbiter.Arbitrate(
		ArbitrationInput{
			Mode:    input.Mode,
			Draft:   cloneExecutionPlan(planDraft),
			GoalSet: goalSet,
		},
	)
}

// Plan invokes the configured planner directly.
func (engine *Engine) Plan(batch BatchInput) (ExecutionPlan, error) {
	if engine == nil {
		return ExecutionPlan{}, errors.New("wavebuild: engine is required")
	}
	engine.mu.RLock()
	planner := engine.planner
	engine.mu.RUnlock()
	if planner == nil {
		return ExecutionPlan{}, fmt.Errorf(
			"wavebuild: planner: %w",
			ErrNotImplemented,
		)
	}
	return planner.Plan(
		PlannerInput{
			Mode:           batch.Mode,
			GenerationID:   strings.TrimSpace(batch.GenerationID),
			Events:         append([]EventType(nil), batch.Events...),
			ConditionFacts: cloneConditionFacts(batch.ConditionFacts),
			Facts:          cloneBatchFacts(batch.Facts),
			Metadata:       cloneStringMap(batch.Metadata),
		},
	)
}

// Execute invokes the configured executor with registered task roots.
func (engine *Engine) Execute(ctx context.Context, plan ExecutionPlan) error {
	if engine == nil {
		return errors.New("wavebuild: engine is required")
	}
	engine.mu.RLock()
	executor := engine.executor
	effectTaskRoots := cloneEffectTaskRoots(engine.effectTaskRootByID)
	engine.mu.RUnlock()
	if executor == nil {
		return fmt.Errorf("wavebuild: executor: %w", ErrNotImplemented)
	}
	return executor.Execute(ctx, cloneExecutionPlan(plan), effectTaskRoots)
}

// PlanAndExecute is a convenience wrapper over Plan + Execute.
func (engine *Engine) PlanAndExecute(
	ctx context.Context,
	batch BatchInput,
) (ExecutionPlan, error) {
	plan, planError := engine.Plan(batch)
	if planError != nil {
		return ExecutionPlan{}, planError
	}
	if executeError := engine.Execute(ctx, plan); executeError != nil {
		return ExecutionPlan{}, executeError
	}
	return plan, nil
}

// BatchInput is one direct planning request for an already-classified batch.
type BatchInput struct {
	Mode           Mode
	GenerationID   string
	Events         []EventType
	ConditionFacts ConditionFacts
	Facts          BatchFacts
	Metadata       map[string]string
}

func cloneEventRule(rule EventRule) EventRule {
	return EventRule{
		Event: rule.Event,
		Dev: ModePlan{
			ConditionalEffects: cloneConditionalEffectSets(
				rule.Dev.ConditionalEffects,
			),
		},
		Prod: ModePlan{
			ConditionalEffects: cloneConditionalEffectSets(
				rule.Prod.ConditionalEffects,
			),
		},
	}
}

func cloneEventRules(rules []EventRule) []EventRule {
	if len(rules) == 0 {
		return nil
	}
	cloned := make([]EventRule, 0, len(rules))
	for _, rule := range rules {
		cloned = append(cloned, cloneEventRule(rule))
	}
	return cloned
}

func cloneConditionalEffectSets(
	groups []ConditionalEffectSet,
) []ConditionalEffectSet {
	if len(groups) == 0 {
		return nil
	}
	cloned := make([]ConditionalEffectSet, 0, len(groups))
	for _, group := range groups {
		cloned = append(
			cloned,
			ConditionalEffectSet{
				Condition: group.Condition,
				Effects:   append([]EffectID(nil), group.Effects...),
			},
		)
	}
	return cloned
}

func cloneGoalSpec(goalSpec GoalSpec) GoalSpec {
	return GoalSpec{
		Effect:     goalSpec.Effect,
		Guarantee:  goalSpec.Guarantee,
		AppliesTo:  goalSpec.AppliesTo,
		DependsOn:  append([]EffectID(nil), goalSpec.DependsOn...),
		MutexGroup: goalSpec.MutexGroup,
	}
}

func snapshotGoalSet(goalSpecByEffectID map[EffectID]GoalSpec) []GoalSpec {
	if len(goalSpecByEffectID) == 0 {
		return nil
	}
	goalSet := make([]GoalSpec, 0, len(goalSpecByEffectID))
	for _, goalSpec := range goalSpecByEffectID {
		goalSet = append(goalSet, cloneGoalSpec(goalSpec))
	}
	return goalSet
}

func cloneGoalSpecs(goalSpecs []GoalSpec) []GoalSpec {
	if len(goalSpecs) == 0 {
		return nil
	}
	cloned := make([]GoalSpec, 0, len(goalSpecs))
	for _, goalSpec := range goalSpecs {
		cloned = append(cloned, cloneGoalSpec(goalSpec))
	}
	return cloned
}

func buildGoalDependencyCatalog(goalSpecs []GoalSpec) map[EffectID][]EffectID {
	if len(goalSpecs) == 0 {
		return nil
	}
	dependencyCatalog := make(map[EffectID][]EffectID, len(goalSpecs))
	for _, goalSpec := range goalSpecs {
		if goalSpec.Effect == "" {
			continue
		}
		if len(goalSpec.DependsOn) == 0 {
			continue
		}
		deduplicatedDependencies := deduplicateCandidateGoals(
			goalSpec.DependsOn,
		)
		if len(deduplicatedDependencies) == 0 {
			continue
		}
		dependencyCatalog[goalSpec.Effect] = deduplicatedDependencies
	}
	if len(dependencyCatalog) == 0 {
		return nil
	}
	return dependencyCatalog
}

func cloneArbitrationRuleSet(ruleSet ArbitrationRuleSet) ArbitrationRuleSet {
	return ArbitrationRuleSet{
		PrecedenceDomains: cloneArbitrationPrecedenceDomains(
			ruleSet.PrecedenceDomains,
		),
		ImplicationRules: cloneArbitrationImplicationRules(
			ruleSet.ImplicationRules,
		),
		ExclusionRules: cloneArbitrationExclusionRules(ruleSet.ExclusionRules),
		TieBreakPolicy: ruleSet.TieBreakPolicy,
	}
}

func cloneArbitrationPrecedenceDomains(
	domains []ArbitrationPrecedenceDomain,
) []ArbitrationPrecedenceDomain {
	if len(domains) == 0 {
		return nil
	}
	cloned := make([]ArbitrationPrecedenceDomain, 0, len(domains))
	for _, domain := range domains {
		cloned = append(
			cloned,
			ArbitrationPrecedenceDomain{
				Domain: domain.Domain,
				HighestToLowest: append(
					[]EffectID(nil),
					domain.HighestToLowest...),
			},
		)
	}
	return cloned
}

func cloneArbitrationImplicationRules(
	rules []ArbitrationImplicationRule,
) []ArbitrationImplicationRule {
	if len(rules) == 0 {
		return nil
	}
	cloned := make([]ArbitrationImplicationRule, 0, len(rules))
	cloned = append(cloned, rules...)
	return cloned
}

func cloneArbitrationExclusionRules(
	rules []ArbitrationExclusionRule,
) []ArbitrationExclusionRule {
	if len(rules) == 0 {
		return nil
	}
	cloned := make([]ArbitrationExclusionRule, 0, len(rules))
	for _, rule := range rules {
		cloned = append(
			cloned,
			ArbitrationExclusionRule{
				IfPresent: rule.IfPresent,
				Exclude:   append([]EffectID(nil), rule.Exclude...),
			},
		)
	}
	return cloned
}

func cloneBatchFacts(facts BatchFacts) BatchFacts {
	return BatchFacts{
		ChangedPathsCWD: append([]string(nil), facts.ChangedPathsCWD...),
		ChangeSummary:   facts.ChangeSummary,
		NoiseFacts:      facts.NoiseFacts,
		BuildFacts:      facts.BuildFacts,
		DevCycleFacts:   facts.DevCycleFacts,
		BrowserFacts:    facts.BrowserFacts,
		StaticFacts: BatchStaticImplicationFacts{
			PublicStaticChangedPathsCWD: append(
				[]string(nil),
				facts.StaticFacts.PublicStaticChangedPathsCWD...,
			),
			PrivateStaticChangedPathsCWD: append(
				[]string(nil),
				facts.StaticFacts.PrivateStaticChangedPathsCWD...,
			),
			PublicFileMapMayHaveChanged: facts.StaticFacts.PublicFileMapMayHaveChanged,
			PublicFileMapMayNeedRepair:  facts.StaticFacts.PublicFileMapMayNeedRepair,
		},
		HookFacts: facts.HookFacts,
	}
}

func cloneConditionFacts(facts ConditionFacts) ConditionFacts {
	if len(facts) == 0 {
		return nil
	}
	cloned := make(ConditionFacts, len(facts))
	maps.Copy(cloned, facts)
	return cloned
}

func cloneExecutionPlan(plan ExecutionPlan) ExecutionPlan {
	return ExecutionPlan{
		Mode:             plan.Mode,
		GenerationID:     plan.GenerationID,
		Events:           append([]EventType(nil), plan.Events...),
		ConditionFacts:   cloneConditionFacts(plan.ConditionFacts),
		Facts:            cloneBatchFacts(plan.Facts),
		Metadata:         cloneStringMap(plan.Metadata),
		FrameworkSignals: cloneFrameworkSignals(plan.FrameworkSignals),
		TerminalGoals:    append([]EffectID(nil), plan.TerminalGoals...),
	}
}

func deriveGenerationIDFromRawBatchInput(input RawBatchInput) string {
	return strings.TrimSpace(input.GenerationID)
}

func cloneEffectTaskRoots(
	effectTaskRoots map[EffectID]*tasks.Task[GoalExecutionKey, struct{}],
) map[EffectID]*tasks.Task[GoalExecutionKey, struct{}] {
	if len(effectTaskRoots) == 0 {
		return nil
	}
	cloned := make(
		map[EffectID]*tasks.Task[GoalExecutionKey, struct{}],
		len(effectTaskRoots),
	)
	maps.Copy(cloned, effectTaskRoots)
	return cloned
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(input))
	maps.Copy(cloned, input)
	return cloned
}

func cloneFrameworkSignals(signals []FrameworkSignal) []FrameworkSignal {
	if len(signals) == 0 {
		return nil
	}
	cloned := make([]FrameworkSignal, 0, len(signals))
	for _, signal := range signals {
		cloned = append(
			cloned,
			FrameworkSignal{
				Type:           signal.Type,
				FreshnessToken: signal.FreshnessToken,
				Trigger:        signal.Trigger,
				Metadata:       cloneStringMap(signal.Metadata),
			},
		)
	}
	return cloned
}
