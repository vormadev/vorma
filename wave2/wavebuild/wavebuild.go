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
	"sync"
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

// Condition gates one effect group in a mode plan.
type Condition string

const (
	// ConditionAlways means one effect group is always eligible.
	ConditionAlways Condition = ""
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
	Mode      Mode
	Events    []RawWatchEvent
	Metadata  map[string]string
	TraceID   string
	BatchID   string
	IsInitial bool
}

// BatchFacts is the single-pass parsed fact set used by planning.
type BatchFacts struct {
	ChangedPaths []string
	Bool         map[string]bool
	Text         map[string]string
	Number       map[string]int
}

// PlannerInput is the canonical plan-input structure after facts and
// classification are complete.
type PlannerInput struct {
	Mode           Mode
	Events         []EventType
	ConditionFacts ConditionFacts
	Facts          BatchFacts
	Metadata       map[string]string
}

// ConditionFacts stores per-condition truth values for one planning operation.
// Missing keys are interpreted as false, except ConditionAlways.
type ConditionFacts map[Condition]bool

// ParallelEffectGroup is one ordered stage with effects that may run in
// parallel.
type ParallelEffectGroup struct {
	Condition Condition
	Effects   []EffectID
}

// ModePlan is one ordered set of parallel effect groups.
type ModePlan struct {
	OrderedParallelEffectGroups []ParallelEffectGroup
}

// EventRule maps one event class into dev/prod plans.
type EventRule struct {
	Event EventType
	Dev   ModePlan
	Prod  ModePlan
}

// ExecutionPlan is one deterministic plan ready for execution.
type ExecutionPlan struct {
	Mode                   Mode
	Events                 []EventType
	ConditionFacts         ConditionFacts
	Facts                  BatchFacts
	Metadata               map[string]string
	OrderedParallelEffects [][]EffectID
}

// ArbitrationInput captures data needed to resolve mutually exclusive goals.
type ArbitrationInput struct {
	Mode    Mode
	Draft   ExecutionPlan
	GoalSet []GoalSpec
}

// EffectExecutionRequest is one effect callback invocation payload.
type EffectExecutionRequest struct {
	Effect EffectID
	Plan   *ExecutionPlan
}

// EffectCallback executes one terminal effect.
type EffectCallback func(ctx context.Context, request EffectExecutionRequest) error

// EffectCallbackRegistration binds one effect id to one callback.
type EffectCallbackRegistration struct {
	Effect   EffectID
	Callback EffectCallback
}

// FactsCollector parses raw batch input into a canonical fact set.
type FactsCollector interface {
	CollectFacts(input RawBatchInput) (BatchFacts, error)
}

// EventClassifier maps facts into normalized event classes.
type EventClassifier interface {
	ClassifyEvents(mode Mode, facts BatchFacts) ([]EventType, error)
}

// Planner reduces classified events + facts into an execution plan.
type Planner interface {
	Plan(input PlannerInput) (ExecutionPlan, error)
}

// Arbiter resolves mutually exclusive or precedence-constrained effects.
type Arbiter interface {
	Arbitrate(input ArbitrationInput) (ExecutionPlan, error)
}

// Executor runs one execution plan using registered effect callbacks.
type Executor interface {
	Execute(
		ctx context.Context,
		plan ExecutionPlan,
		effectCallbacks map[EffectID]EffectCallback,
	) error
}

// EngineConfig configures one contract-level orchestration engine.
type EngineConfig struct {
	InitialEventRules      []EventRule
	InitialGoalSpecs       []GoalSpec
	InitialEffectCallbacks []EffectCallbackRegistration
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
	effectCallbackByID map[EffectID]EffectCallback
	factsCollector     FactsCollector
	eventClassifier    EventClassifier
	planner            Planner
	arbiter            Arbiter
	executor           Executor
}

// NewEngine constructs one contract-first engine.
func NewEngine(config EngineConfig) (*Engine, error) {
	engine := &Engine{
		eventRuleByType:    make(map[EventType]EventRule),
		goalSpecByEffectID: make(map[EffectID]GoalSpec),
		effectCallbackByID: make(map[EffectID]EffectCallback),
		factsCollector:     config.FactsCollector,
		eventClassifier:    config.EventClassifier,
		planner:            config.Planner,
		arbiter:            config.Arbiter,
		executor:           config.Executor,
	}
	if registerRulesError := engine.RegisterEventRules(
		config.InitialEventRules...,
	); registerRulesError != nil {
		return nil, registerRulesError
	}
	if registerGoalSpecsError := engine.RegisterGoalSpecs(
		config.InitialGoalSpecs...,
	); registerGoalSpecsError != nil {
		return nil, registerGoalSpecsError
	}
	if registerCallbacksError := engine.RegisterEffectCallbacks(
		config.InitialEffectCallbacks...,
	); registerCallbacksError != nil {
		return nil, registerCallbacksError
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

// RegisterEffectCallbacks inserts or replaces effect callbacks.
func (engine *Engine) RegisterEffectCallbacks(
	registrations ...EffectCallbackRegistration,
) error {
	if engine == nil {
		return errors.New("wavebuild: engine is required")
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	for _, registration := range registrations {
		if registration.Effect == "" {
			return errors.New("wavebuild: effect callback effect is required")
		}
		if registration.Callback == nil {
			return fmt.Errorf(
				"wavebuild: effect callback for %q is required",
				registration.Effect,
			)
		}
		engine.effectCallbackByID[registration.Effect] = registration.Callback
	}
	return nil
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
			Mode:           input.Mode,
			Events:         append([]EventType(nil), events...),
			ConditionFacts: nil,
			Facts:          cloneBatchFacts(facts),
			Metadata:       cloneStringMap(input.Metadata),
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
			Events:         append([]EventType(nil), batch.Events...),
			ConditionFacts: cloneConditionFacts(batch.ConditionFacts),
			Facts:          cloneBatchFacts(batch.Facts),
			Metadata:       cloneStringMap(batch.Metadata),
		},
	)
}

// Execute invokes the configured executor with registered callbacks.
func (engine *Engine) Execute(ctx context.Context, plan ExecutionPlan) error {
	if engine == nil {
		return errors.New("wavebuild: engine is required")
	}
	engine.mu.RLock()
	executor := engine.executor
	effectCallbacks := cloneEffectCallbacks(engine.effectCallbackByID)
	engine.mu.RUnlock()
	if executor == nil {
		return fmt.Errorf("wavebuild: executor: %w", ErrNotImplemented)
	}
	return executor.Execute(ctx, cloneExecutionPlan(plan), effectCallbacks)
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
	Events         []EventType
	ConditionFacts ConditionFacts
	Facts          BatchFacts
	Metadata       map[string]string
}

func cloneEventRule(rule EventRule) EventRule {
	return EventRule{
		Event: rule.Event,
		Dev: ModePlan{
			OrderedParallelEffectGroups: cloneParallelEffectGroups(
				rule.Dev.OrderedParallelEffectGroups,
			),
		},
		Prod: ModePlan{
			OrderedParallelEffectGroups: cloneParallelEffectGroups(
				rule.Prod.OrderedParallelEffectGroups,
			),
		},
	}
}

func cloneParallelEffectGroups(
	groups []ParallelEffectGroup,
) []ParallelEffectGroup {
	if len(groups) == 0 {
		return nil
	}
	cloned := make([]ParallelEffectGroup, 0, len(groups))
	for _, group := range groups {
		cloned = append(
			cloned,
			ParallelEffectGroup{
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

func cloneBatchFacts(facts BatchFacts) BatchFacts {
	return BatchFacts{
		ChangedPaths: append([]string(nil), facts.ChangedPaths...),
		Bool:         cloneBoolMap(facts.Bool),
		Text:         cloneStringMap(facts.Text),
		Number:       cloneIntMap(facts.Number),
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
	clonedParallelEffects := make(
		[][]EffectID,
		0,
		len(plan.OrderedParallelEffects),
	)
	for _, group := range plan.OrderedParallelEffects {
		clonedParallelEffects = append(
			clonedParallelEffects,
			append([]EffectID(nil), group...),
		)
	}
	return ExecutionPlan{
		Mode:                   plan.Mode,
		Events:                 append([]EventType(nil), plan.Events...),
		ConditionFacts:         cloneConditionFacts(plan.ConditionFacts),
		Facts:                  cloneBatchFacts(plan.Facts),
		Metadata:               cloneStringMap(plan.Metadata),
		OrderedParallelEffects: clonedParallelEffects,
	}
}

func cloneEffectCallbacks(
	effectCallbacks map[EffectID]EffectCallback,
) map[EffectID]EffectCallback {
	if len(effectCallbacks) == 0 {
		return nil
	}
	cloned := make(map[EffectID]EffectCallback, len(effectCallbacks))
	maps.Copy(cloned, effectCallbacks)
	return cloned
}

func cloneBoolMap(input map[string]bool) map[string]bool {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]bool, len(input))
	maps.Copy(cloned, input)
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

func cloneIntMap(input map[string]int) map[string]int {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]int, len(input))
	maps.Copy(cloned, input)
	return cloned
}
