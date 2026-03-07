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
	// mode_dev applies development-time policy.
	mode_dev mode = "dev"
	// mode_prod applies production-time policy.
	mode_prod mode = "prod"
)

// event_type is one normalized change category from watcher input.
type event_type string

const (
	// event_type_config_file_changed represents semantic config changes.
	event_type_config_file_changed event_type = "config-file-changed"
	// event_type_go_source_changed represents application Go source changes.
	event_type_go_source_changed event_type = "go-source-changed"
	// event_type_critical_css_source_changed represents critical CSS source changes.
	event_type_critical_css_source_changed event_type = "critical-css-source-changed"
	// event_type_normal_css_source_changed represents normal CSS source changes.
	event_type_normal_css_source_changed event_type = "normal-css-source-changed"
	// event_type_public_static_asset_changed represents public static asset changes.
	event_type_public_static_asset_changed event_type = "public-static-asset-changed"
	// event_type_private_static_asset_changed represents private static asset changes.
	event_type_private_static_asset_changed event_type = "private-static-asset-changed"
	// fw_event_type_requested_effects_changed represents fw-requested
	// effect payload changes carried by watcher classification.
	fw_event_type_requested_effects_changed event_type = "fw-requested-effects-changed"
	// event_type_app_defined_watch_action_only_changed represents app watch classes that request direct actions without implicit build work.
	event_type_app_defined_watch_action_only_changed event_type = "app-defined-watch-action-only-changed"
	// event_type_app_defined_watch_with_rebuild_changed represents app watch classes that participate in normal build-phase planning; concrete effects still come from reduced outcomes.
	event_type_app_defined_watch_with_rebuild_changed event_type = "app-defined-watch-with-rebuild-changed"
	// event_type_ignored_or_noise_changed represents ignored/noise-only batches.
	event_type_ignored_or_noise_changed event_type = "ignored-or-noise-changed"
	// event_type_unclassified_no_watch_rule_changed represents meaningful input with no matching watch rule.
	event_type_unclassified_no_watch_rule_changed event_type = "unclassified-no-watch-rule-changed"
)

// frontend_terminal_browser_action is the single terminal browser action selected
// for one batch.
type frontend_terminal_browser_action string

const (
	// frontend_terminal_browser_action_none represents no browser action.
	frontend_terminal_browser_action_none frontend_terminal_browser_action = "none"
	// frontend_terminal_browser_action_css_hot_reload represents CSS-only hot reload.
	frontend_terminal_browser_action_css_hot_reload frontend_terminal_browser_action = "css-hot-reload"
	// frontend_terminal_browser_action_notify_vite_public_file_map_changed notifies Vite that public file-map artifacts changed.
	frontend_terminal_browser_action_notify_vite_public_file_map_changed frontend_terminal_browser_action = "notify-vite-public-filemap-changed"
	// frontend_terminal_browser_action_revalidate represents browser revalidation.
	frontend_terminal_browser_action_revalidate frontend_terminal_browser_action = "revalidate"
	// frontend_terminal_browser_action_hard_reload represents browser hard reload.
	frontend_terminal_browser_action_hard_reload frontend_terminal_browser_action = "hard-reload"
)

// fw_mutation_effect_key identifies one fw-owned backend-mutation
// effect registration key.
type fw_mutation_effect_key string

// fw_notif_destination_key identifies one fw-owned
// notification destination registration key.
type fw_notif_destination_key string

// fw_notif_failure_policy selects what Wave does when fw
// notification transport fails.
type fw_notif_failure_policy string

const (
	// fw_notif_failure_policy_fail_pipeline surfaces notification
	// transport failure as pipeline failure.
	fw_notif_failure_policy_fail_pipeline fw_notif_failure_policy = "fail-pipeline"
	// fw_notif_failure_policy_restart_backend_without_go_compile requests
	// backend restart without Go recompilation and skips frontend settling.
	fw_notif_failure_policy_restart_backend_without_go_compile fw_notif_failure_policy = "restart-backend-without-go-compile"
)

type fw_notif_request struct {
	destination_key fw_notif_destination_key
	freshness_token string
	trigger         string
	metadata        map[string]string
	wait_for_app    bool
	wait_for_vite   bool
	failure_policy  fw_notif_failure_policy
}

type fw_requested_effects struct {
	backend_mutation_effect_keys    []fw_mutation_effect_key
	backend_convergence_notif_queue []fw_notif_request
}

type fw_execution_registrations struct {
	backend_mutation_effects_by_key           map[fw_mutation_effect_key]*tasks.Task[p3_batch_input, struct{}]
	backend_convergence_notifs_by_destination map[fw_notif_destination_key]*tasks.Task[p4_fw_notif_task_input, struct{}]
}

/////////////////////////////////////////////////////////////////////
/////// Public Exposure
/////////////////////////////////////////////////////////////////////

// FrameworkNotificationFailurePolicy controls how Wave handles delivery
// failures when sending framework notifications during backend convergence.
type FrameworkNotificationFailurePolicy = fw_notif_failure_policy

// FrameworkNotification describes one framework notification emitted by Wave
// after a batch converges.
type FrameworkNotification = fw_notif

const (
	// FrameworkNotificationFailurePolicyFailPipeline surfaces notification
	// transport failure as pipeline failure.
	FrameworkNotificationFailurePolicyFailPipeline FrameworkNotificationFailurePolicy = fw_notif_failure_policy_fail_pipeline
	// FrameworkNotificationFailurePolicyRestartBackendWithoutGoCompile requests
	// backend restart without Go recompilation and skips frontend settling.
	FrameworkNotificationFailurePolicyRestartBackendWithoutGoCompile FrameworkNotificationFailurePolicy = fw_notif_failure_policy_restart_backend_without_go_compile
)

func fw_requested_effects_from_pointer(
	fw_requested_effects_pointer *fw_requested_effects,
) fw_requested_effects {
	if fw_requested_effects_pointer == nil {
		return fw_requested_effects{}
	}
	return *fw_requested_effects_pointer
}

func fw_new_requested_effects_pointer_if_any(
	fw_requested_effects_val fw_requested_effects,
) *fw_requested_effects {
	if !fw_requested_effects_val.has_any() {
		return nil
	}
	fw_requested_effects_copy := fw_requested_effects_val
	return &fw_requested_effects_copy
}

func (action frontend_terminal_browser_action) priority() int {
	switch action {
	case frontend_terminal_browser_action_hard_reload:
		return 4
	case frontend_terminal_browser_action_notify_vite_public_file_map_changed:
		return 3
	case frontend_terminal_browser_action_revalidate:
		return 2
	case frontend_terminal_browser_action_css_hot_reload:
		return 1
	default:
		return 0
	}
}

func (left frontend_terminal_browser_action) dominant_with(
	right frontend_terminal_browser_action,
) frontend_terminal_browser_action {
	if right.priority() > left.priority() {
		return right
	}
	return left
}

// observed_batch_event is one normalized, already-classified batch event input.
//
// The event type is expected to be produced by the watcher/classification layer;
// this contract intentionally avoids path-shape guessing.
type observed_batch_event struct {
	event_type event_type

	// noise_only marks events that should not trigger user-visible work.
	noise_only bool
	// no_op_config_mutation marks config events where semantic config did not change.
	no_op_config_mutation bool
	// fw_requested_effects carries fw-owned requested effects
	// discovered by watcher classification for this event.
	fw_requested_effects fw_requested_effects
}

// app_requested_outcomes captures app-requested observable outcomes for one batch.
type app_requested_outcomes struct {
	requested_terminal_browser_action frontend_terminal_browser_action
	request_restart                   bool
	request_go_compile                bool
	fw_requested_effects              fw_requested_effects
}

// p1_input is the phase-1 input contract in observable terms.
type p1_input struct {
	mode          mode
	generation_id string
	events        []observed_batch_event

	wave_public_file_map_notif_destination_key fw_notif_destination_key
	fw_execution_registrations                 *fw_execution_registrations

	app_requested_outcomes  app_requested_outcomes
	waiting_for_build_retry bool
}

// p1_facts is the minimal phase-1 output used by planners.
type p1_facts struct {
	mode          mode
	generation_id string

	// event_types contains deduplicated actionable event types, preserving
	// first-seen input order.
	event_types []event_type

	wave_public_file_map_notif_destination_key fw_notif_destination_key
	fw_execution_registrations                 *fw_execution_registrations

	app_requested_outcomes  app_requested_outcomes
	fw_requested_effects    fw_requested_effects
	waiting_for_build_retry bool
}

// build_facts reduces one phase-1 input into deterministic planner facts.
func (in p1_input) build_facts() (p1_facts, error) {
	if err := in.mode.validate(); err != nil {
		return p1_facts{}, err
	}
	normalized_generation_id := strings.TrimSpace(in.generation_id)
	if normalized_generation_id == "" {
		return p1_facts{}, errors.New(
			"wavebuild: generation id is required",
		)
	}

	app_requested_outcomes, err := in.app_requested_outcomes.reduce()
	if err != nil {
		return p1_facts{}, err
	}

	event_type_set := make(map[event_type]struct{}, len(in.events))
	actionable_event_types := make([]event_type, 0, len(in.events))
	fw_requested_effects := fw_requested_effects{}

	for event_index, raw_event := range in.events {
		if raw_event.noise_only {
			continue
		}

		if raw_event.event_type == event_type_config_file_changed &&
			raw_event.no_op_config_mutation {
			continue
		}

		if !raw_event.event_type.is_supported() {
			return p1_facts{}, fmt.Errorf(
				"wavebuild: unsupported event type %q at index %d",
				raw_event.event_type,
				event_index,
			)
		}
		fw_requested_effects = fw_requested_effects.merge(
			raw_event.fw_requested_effects,
		)
		if _, already_seen_event_type := event_type_set[raw_event.event_type]; already_seen_event_type {
			continue
		}
		event_type_set[raw_event.event_type] = struct{}{}
		actionable_event_types = append(
			actionable_event_types,
			raw_event.event_type,
		)
	}

	return p1_facts{
		mode:          in.mode,
		generation_id: normalized_generation_id,
		event_types:   actionable_event_types,
		wave_public_file_map_notif_destination_key: fw_notif_destination_key(
			strings.TrimSpace(
				string(in.wave_public_file_map_notif_destination_key),
			),
		),
		fw_execution_registrations: in.fw_execution_registrations,
		app_requested_outcomes:     app_requested_outcomes,
		fw_requested_effects:       fw_requested_effects,
		waiting_for_build_retry:    in.waiting_for_build_retry,
	}, nil
}

// fw_notif is one framework-agnostic notification payload emitted by
// Wave2 planning.
type fw_notif struct {
	destination_key fw_notif_destination_key
	freshness_token string
	trigger         string
	metadata        map[string]string
	wait_for_app    bool
	wait_for_vite   bool
	failure_policy  fw_notif_failure_policy
}

// DestinationKey returns framework notification destination registration key.
func (notif fw_notif) DestinationKey() string {
	return string(notif.destination_key)
}

// FreshnessToken returns the freshness token for stale-attempt rejection.
func (notif fw_notif) FreshnessToken() string {
	return notif.freshness_token
}

// Trigger returns stable notification trigger text.
func (notif fw_notif) Trigger() string {
	return notif.trigger
}

// Metadata returns optional stable key/value metadata.
func (notif fw_notif) Metadata() map[string]string {
	return notif.metadata
}

// WaitForApp returns whether notification transport is app-readiness gated.
func (notif fw_notif) WaitForApp() bool {
	return notif.wait_for_app
}

// WaitForVite returns whether notification transport is vite-readiness gated.
func (notif fw_notif) WaitForVite() bool {
	return notif.wait_for_vite
}

// FailurePolicy returns policy used when notification transport fails.
func (notif fw_notif) FailurePolicy() FrameworkNotificationFailurePolicy {
	return notif.failure_policy
}

func (effects fw_requested_effects) fw_notifications(
	generation_id string,
) []fw_notif {
	if !effects.has_backend_convergence_notifs() {
		return nil
	}
	fallback_freshness_token := strings.TrimSpace(generation_id)
	notifications := make(
		[]fw_notif,
		0,
		len(effects.backend_convergence_notif_queue),
	)
	for _, request := range effects.backend_convergence_notif_queue {
		normalized_request := request.normalize()
		if normalized_request.destination_key == "" {
			continue
		}
		if normalized_request.freshness_token == "" {
			normalized_request.freshness_token = fallback_freshness_token
		}
		notifications = append(
			notifications,
			fw_notif{
				destination_key: normalized_request.destination_key,
				freshness_token: normalized_request.freshness_token,
				trigger:         normalized_request.trigger,
				metadata:        normalized_request.metadata,
				wait_for_app:    normalized_request.wait_for_app,
				wait_for_vite:   normalized_request.wait_for_vite,
				failure_policy:  normalized_request.failure_policy,
			},
		)
	}
	return notifications
}

func (effects fw_requested_effects) has_any() bool {
	return len(effects.backend_mutation_effect_keys) > 0 ||
		len(effects.backend_convergence_notif_queue) > 0
}

func (effects fw_requested_effects) has_backend_mutation_effects() bool {
	return len(effects.backend_mutation_effect_keys) > 0
}

func (effects fw_requested_effects) has_backend_convergence_notifs() bool {
	return len(
		effects.backend_convergence_notif_queue,
	) > 0
}

func (left fw_requested_effects) merge(
	right fw_requested_effects,
) fw_requested_effects {
	merged_mutation_effect_keys := make(
		[]fw_mutation_effect_key,
		0,
		len(left.backend_mutation_effect_keys)+
			len(right.backend_mutation_effect_keys),
	)
	seen_mutation_effect_keys := make(
		map[fw_mutation_effect_key]struct{},
		len(left.backend_mutation_effect_keys)+
			len(right.backend_mutation_effect_keys),
	)
	append_mutation_effect_key := func(effect_key fw_mutation_effect_key) {
		normalized_effect_key := fw_mutation_effect_key(
			strings.TrimSpace(string(effect_key)),
		)
		if normalized_effect_key == "" {
			return
		}
		if _, already_seen := seen_mutation_effect_keys[normalized_effect_key]; already_seen {
			return
		}
		seen_mutation_effect_keys[normalized_effect_key] = struct{}{}
		merged_mutation_effect_keys = append(
			merged_mutation_effect_keys,
			normalized_effect_key,
		)
	}
	for _, effect_key := range left.backend_mutation_effect_keys {
		append_mutation_effect_key(effect_key)
	}
	for _, effect_key := range right.backend_mutation_effect_keys {
		append_mutation_effect_key(effect_key)
	}

	merged_notif_queue := fw_merge_notif_queue(
		left.backend_convergence_notif_queue,
		right.backend_convergence_notif_queue,
	)

	return fw_requested_effects{
		backend_mutation_effect_keys:    merged_mutation_effect_keys,
		backend_convergence_notif_queue: merged_notif_queue,
	}
}

func fw_merge_notif_queue(
	left_queue []fw_notif_request,
	right_queue []fw_notif_request,
) []fw_notif_request {
	merged_queue := make(
		[]fw_notif_request,
		0,
		len(left_queue)+len(right_queue),
	)
	notif_index_by_destination := make(
		map[fw_notif_destination_key]int,
		len(left_queue)+len(right_queue),
	)
	append_notif := func(notif fw_notif_request) {
		normalized_notif := notif.normalize()
		if normalized_notif.destination_key == "" {
			return
		}
		existing_index, already_exists := notif_index_by_destination[normalized_notif.destination_key]
		if already_exists {
			merged_queue[existing_index] = merged_queue[existing_index].merge(
				normalized_notif,
			)
			return
		}
		notif_index_by_destination[normalized_notif.destination_key] = len(
			merged_queue,
		)
		merged_queue = append(merged_queue, normalized_notif)
	}
	for _, notif := range left_queue {
		append_notif(notif)
	}
	for _, notif := range right_queue {
		append_notif(notif)
	}
	return merged_queue
}

func (notif fw_notif_request) normalize() fw_notif_request {
	normalized_notif := notif
	normalized_notif.destination_key = fw_notif_destination_key(
		strings.TrimSpace(string(notif.destination_key)),
	)
	normalized_notif.freshness_token = strings.TrimSpace(
		notif.freshness_token,
	)
	normalized_notif.trigger = strings.TrimSpace(notif.trigger)
	if normalized_notif.failure_policy == "" {
		normalized_notif.failure_policy = fw_notif_failure_policy_fail_pipeline
	}
	return normalized_notif
}

func (left fw_notif_request) merge(
	right fw_notif_request,
) fw_notif_request {
	merged_notif := left
	if right.freshness_token != "" {
		merged_notif.freshness_token = right.freshness_token
	}
	if right.trigger != "" {
		merged_notif.trigger = right.trigger
	}
	if len(right.metadata) > 0 {
		if merged_notif.metadata == nil {
			merged_notif.metadata = make(
				map[string]string,
				len(right.metadata),
			)
		}
		maps.Copy(merged_notif.metadata, right.metadata)
	}
	merged_notif.wait_for_app =
		merged_notif.wait_for_app || right.wait_for_app
	merged_notif.wait_for_vite =
		merged_notif.wait_for_vite || right.wait_for_vite
	if right.failure_policy != "" {
		merged_notif.failure_policy = right.failure_policy
	}
	return merged_notif
}

var (
	err_p1_input_required = errors.New(
		"wavebuild: phase-1 input is required",
	)
	err_generation_id_required = errors.New(
		"wavebuild: generation id is required",
	)
)

type phase_effect_sets struct {
	p1 p1_effects
	p2 p2_effects
	p3 p3_effects
	p4 p4_effects
	p5 p5_effects
}

var real_phase_effect_sets = phase_effect_sets{
	p1: p1_effects_def,
	p2: p2_effects_def,
	p3: p3_effects_def,
	p4: p4_effects_def,
	p5: p5_effects_def,
}

// run_five_phase_pipeline executes one phase batch with one batch-scoped tasks
// context.
//
// mode policy:
// - dev runs phases 1-5.
// - prod bypasses phases 1/3/4/5 and runs phase 2 only.
func run_five_phase_pipeline(
	parent_context context.Context,
	input p1_batch_input,
) (five_phase_run_result, error) {
	return run_five_phase_pipeline_with_effect_sets(
		parent_context,
		input,
		real_phase_effect_sets,
	)
}

func run_five_phase_pipeline_with_effect_sets(
	parent_context context.Context,
	input p1_batch_input,
	effects phase_effect_sets,
) (five_phase_run_result, error) {
	if input.p1 == nil {
		return five_phase_run_result{}, err_p1_input_required
	}
	normalized_generation_id := strings.TrimSpace(input.p1.generation_id)
	if normalized_generation_id == "" {
		return five_phase_run_result{}, err_generation_id_required
	}
	if err := input.p1.mode.validate(); err != nil {
		return five_phase_run_result{}, err
	}

	batch_task_context := tasks.NewCtx(parent_context)
	p1_requested_effects := canonical_p1_requested_effects()
	if input.p1.mode != mode_prod {
		var err error
		p1_requested_effects, err = effects.p1.plan_p1_requested_effects.Run(
			batch_task_context,
			input,
		)
		if err != nil {
			return five_phase_run_result{}, err
		}
	}

	phase_batch_input := phase_batch_input{
		mode:                       input.p1.mode,
		generation_id:              normalized_generation_id,
		fw_execution_registrations: p1_requested_effects.fw_execution_registrations,
	}
	p2_output, err := effects.p2.plan_p2_output.Run(
		batch_task_context,
		p2_batch_input{
			batch:                phase_batch_input,
			p1_requested_effects: p1_requested_effects,
		},
	)
	if err != nil {
		return five_phase_run_result{}, err
	}
	if input.p1.mode == mode_prod {
		return five_phase_run_result{
			p1_requested_effects: p1_requested_effects,
			p2_requested_effects: p2_output.p2_requested_effects,
		}, nil
	}

	p3_output, err := effects.p3.plan_p4_requested_effects.Run(
		batch_task_context,
		p3_batch_input{
			batch:                phase_batch_input,
			p2_requested_effects: p2_output.p2_requested_effects,
		},
	)
	if err != nil {
		return five_phase_run_result{}, err
	}
	p4_output, err := effects.p4.plan_p5_requested_effects.Run(
		batch_task_context,
		p4_batch_input{
			batch:                phase_batch_input,
			p4_requested_effects: p3_output.p4_requested_effects,
		},
	)
	if err != nil {
		return five_phase_run_result{}, err
	}
	if p4_output.requires_backend_restart_without_go_compile {
		healing_p2_requested_effects := p2_requested_effects{
			restart_app_process:        true,
			await_backend_readiness:    true,
			fw_execution_registrations: p1_requested_effects.fw_execution_registrations,
		}
		healing_p3_output, err := effects.p3.plan_p4_requested_effects.Run(
			batch_task_context,
			p3_batch_input{
				batch:                phase_batch_input,
				p2_requested_effects: healing_p2_requested_effects,
			},
		)
		if err != nil {
			return five_phase_run_result{}, err
		}
		healing_p4_output, err := effects.p4.plan_p5_requested_effects.Run(
			batch_task_context,
			p4_batch_input{
				batch:                phase_batch_input,
				p4_requested_effects: healing_p3_output.p4_requested_effects,
			},
		)
		if err != nil {
			return five_phase_run_result{}, err
		}
		p2_output.p2_requested_effects = p2_output.p2_requested_effects.merge(
			healing_p2_requested_effects,
		)
		p3_output = healing_p3_output
		p4_output = healing_p4_output
		return five_phase_run_result{
			p1_requested_effects: p1_requested_effects,
			p2_requested_effects: p2_output.p2_requested_effects,
			p3_output:            p3_output,
			p4_output:            p4_output,
			p5_completion_summary: p5_completion_summary{
				terminal_action: frontend_terminal_browser_action_none,
			},
			fw_notifications: fw_requested_effects_from_pointer(
				p2_output.p2_requested_effects.fw_requested_effects,
			).fw_notifications(
				normalized_generation_id,
			),
		}, nil
	}
	if p4_output.skip_frontend_settling {
		return five_phase_run_result{
			p1_requested_effects: p1_requested_effects,
			p2_requested_effects: p2_output.p2_requested_effects,
			p3_output:            p3_output,
			p4_output:            p4_output,
			p5_completion_summary: p5_completion_summary{
				terminal_action: frontend_terminal_browser_action_none,
			},
			fw_notifications: fw_requested_effects_from_pointer(
				p2_output.p2_requested_effects.fw_requested_effects,
			).fw_notifications(
				normalized_generation_id,
			),
		}, nil
	}
	p5_completion_summary, err := effects.p5.execute_terminal_browser_action.Run(
		batch_task_context,
		p5_batch_input{
			batch:                phase_batch_input,
			p5_requested_effects: p4_output.p5_requested_effects,
		},
	)
	if err != nil {
		return five_phase_run_result{}, err
	}
	if p5_completion_summary.requires_backend_vite_healing {
		healing_p2_requested_effects := p2_requested_effects{
			restart_vite_process:    true,
			await_backend_readiness: true,
		}
		healing_p3_output, err := effects.p3.plan_p4_requested_effects.Run(
			batch_task_context,
			p3_batch_input{
				batch:                phase_batch_input,
				p2_requested_effects: healing_p2_requested_effects,
			},
		)
		if err != nil {
			return five_phase_run_result{}, err
		}
		healing_p4_output, err := effects.p4.plan_p5_requested_effects.Run(
			batch_task_context,
			p4_batch_input{
				batch:                phase_batch_input,
				p4_requested_effects: healing_p3_output.p4_requested_effects,
			},
		)
		if err != nil {
			return five_phase_run_result{}, err
		}
		healing_completion_summary, err := effects.p5.execute_terminal_browser_action.Run(
			batch_task_context,
			p5_batch_input{
				batch:                phase_batch_input,
				p5_requested_effects: healing_p4_output.p5_requested_effects,
			},
		)
		if err != nil {
			return five_phase_run_result{}, err
		}
		p2_output.p2_requested_effects = p2_output.p2_requested_effects.merge(
			healing_p2_requested_effects,
		)
		p3_output = healing_p3_output
		p4_output = healing_p4_output
		p5_completion_summary = healing_completion_summary
	}
	return five_phase_run_result{
		p1_requested_effects:  p1_requested_effects,
		p2_requested_effects:  p2_output.p2_requested_effects,
		p3_output:             p3_output,
		p4_output:             p4_output,
		p5_completion_summary: p5_completion_summary,
		fw_notifications: fw_requested_effects_from_pointer(
			p2_output.p2_requested_effects.fw_requested_effects,
		).fw_notifications(
			normalized_generation_id,
		),
	}, nil
}

func (current_mode mode) validate() error {
	switch current_mode {
	case mode_dev, mode_prod:
		return nil
	default:
		return fmt.Errorf("wavebuild: unsupported mode %q", current_mode)
	}
}

func (event event_type) is_supported() bool {
	switch event {
	case event_type_config_file_changed:
		return true
	case event_type_go_source_changed:
		return true
	case event_type_critical_css_source_changed:
		return true
	case event_type_normal_css_source_changed:
		return true
	case event_type_public_static_asset_changed:
		return true
	case event_type_private_static_asset_changed:
		return true
	case fw_event_type_requested_effects_changed:
		return true
	case event_type_app_defined_watch_action_only_changed:
		return true
	case event_type_app_defined_watch_with_rebuild_changed:
		return true
	case event_type_ignored_or_noise_changed:
		return true
	case event_type_unclassified_no_watch_rule_changed:
		return true
	default:
		return false
	}
}

func (outcomes app_requested_outcomes) reduce() (app_requested_outcomes, error) {
	reduced_outcomes := outcomes
	switch reduced_outcomes.requested_terminal_browser_action {
	case "":
		reduced_outcomes.requested_terminal_browser_action = frontend_terminal_browser_action_none
	case frontend_terminal_browser_action_none,
		frontend_terminal_browser_action_css_hot_reload,
		frontend_terminal_browser_action_notify_vite_public_file_map_changed,
		frontend_terminal_browser_action_revalidate,
		frontend_terminal_browser_action_hard_reload:
	default:
		return app_requested_outcomes{}, fmt.Errorf(
			"wavebuild: unsupported requested terminal browser action %q",
			reduced_outcomes.requested_terminal_browser_action,
		)
	}
	reduced_outcomes.fw_requested_effects = fw_requested_effects{}.merge(
		reduced_outcomes.fw_requested_effects,
	)
	for _, fw_notif_request := range reduced_outcomes.fw_requested_effects.backend_convergence_notif_queue {
		switch fw_notif_request.failure_policy {
		case "",
			fw_notif_failure_policy_fail_pipeline,
			fw_notif_failure_policy_restart_backend_without_go_compile:
		default:
			return app_requested_outcomes{}, fmt.Errorf(
				"wavebuild: unsupported fw-notif failure policy %q",
				fw_notif_request.failure_policy,
			)
		}
	}
	return reduced_outcomes, nil
}

func canonical_p1_requested_effects() p1_requested_effects {
	return p1_requested_effects{
		compile_go_binary:                   true,
		build_critical_css:                  true,
		build_normal_css:                    true,
		process_public_static_assets:        true,
		cleanup_stale_public_static_outputs: true,
		process_private_static_assets:       true,
		generate_public_file_map:            true,
		run_requested_build_effects:         true,
	}
}
