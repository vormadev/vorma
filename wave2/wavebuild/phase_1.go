package wavebuild

import (
	"slices"
)

/////////////////////////////////////////////////////////////////////
/////// Phase Contracts
/////////////////////////////////////////////////////////////////////

// phase_batch_input is the shared per-batch phase envelope passed after phase 1.
type phase_batch_input struct {
	mode                       mode
	generation_id              string
	fw_execution_registrations *fw_execution_registrations
}

// p1_batch_input is the phase-1 input contract for one reduced batch.
type p1_batch_input struct {
	p1 *p1_input
}

// p1_requested_effects are phase-2 build effects plus carry-forward settle intents.
type p1_requested_effects struct {
	restart_dev_server_cycle            bool
	compile_go_binary                   bool
	build_critical_css                  bool
	build_normal_css                    bool
	process_public_static_assets        bool
	cleanup_stale_public_static_outputs bool
	process_private_static_assets       bool
	generate_public_file_map            bool
	run_requested_build_effects         bool
	queue_retry_wait_restart            bool

	request_backend_restart                    bool
	request_vite_restart                       bool
	requested_terminal_browser_action          frontend_terminal_browser_action
	wave_public_file_map_notif_destination_key fw_notif_destination_key
	fw_execution_registrations                 *fw_execution_registrations
	fw_requested_effects                       *fw_requested_effects
}

/////////////////////////////////////////////////////////////////////
/////// Phase Reductions
/////////////////////////////////////////////////////////////////////

func (facts p1_facts) has_event_type(
	event_type event_type,
) bool {
	return slices.Contains(facts.event_types, event_type)
}

func (facts p1_facts) derive_implicit_browser_action_intent() frontend_terminal_browser_action {
	action_intent := frontend_terminal_browser_action_none
	if facts.has_event_type(
		event_type_critical_css_source_changed,
	) ||
		facts.has_event_type(
			event_type_normal_css_source_changed,
		) {
		action_intent = action_intent.dominant_with(
			frontend_terminal_browser_action_css_hot_reload,
		)
	}
	if facts.has_event_type(
		event_type_public_static_asset_changed,
	) {
		action_intent = action_intent.dominant_with(
			frontend_terminal_browser_action_notify_vite_public_file_map_changed,
		)
	}
	if facts.has_event_type(
		event_type_go_source_changed,
	) ||
		facts.has_event_type(
			event_type_private_static_asset_changed,
		) {
		action_intent = action_intent.dominant_with(
			frontend_terminal_browser_action_hard_reload,
		)
	}
	return action_intent
}

func (facts p1_facts) derive_p1_requested_effects() p1_requested_effects {
	if facts.mode == mode_dev &&
		facts.waiting_for_build_retry {
		return p1_requested_effects{
			queue_retry_wait_restart: true,
		}
	}
	config_changed := facts.has_event_type(
		event_type_config_file_changed,
	)
	go_source_changed := facts.has_event_type(
		event_type_go_source_changed,
	)
	critical_css_changed := facts.has_event_type(
		event_type_critical_css_source_changed,
	)
	normal_css_changed := facts.has_event_type(
		event_type_normal_css_source_changed,
	)
	public_static_changed := facts.has_event_type(
		event_type_public_static_asset_changed,
	)
	private_static_changed := facts.has_event_type(
		event_type_private_static_asset_changed,
	)
	app_defined_watch_action_only_changed := facts.has_event_type(
		event_type_app_defined_watch_action_only_changed,
	)
	app_defined_watch_with_rebuild_changed := facts.has_event_type(
		event_type_app_defined_watch_with_rebuild_changed,
	)
	app_defined_watch_changed := app_defined_watch_action_only_changed ||
		app_defined_watch_with_rebuild_changed

	fw_requested_effects_val := facts.fw_requested_effects
	app_requested_outcomes := app_requested_outcomes{}
	if app_defined_watch_changed || fw_requested_effects_val.has_any() {
		app_requested_outcomes = facts.app_requested_outcomes
	}
	fw_requested_effects_val = fw_requested_effects_val.merge(
		app_requested_outcomes.fw_requested_effects,
	)
	merged_browser_action_intent := facts.derive_implicit_browser_action_intent().
		dominant_with(
			app_requested_outcomes.requested_terminal_browser_action,
		)

	request_backend_restart := go_source_changed ||
		app_requested_outcomes.request_restart
	request_vite_restart := config_changed

	if facts.mode == mode_prod {
		request_backend_restart = false
		request_vite_restart = false
		merged_browser_action_intent = frontend_terminal_browser_action_none
		fw_requested_effects_val = fw_requested_effects{}
	}

	requested_effects := p1_requested_effects{
		restart_dev_server_cycle: config_changed,
		compile_go_binary: go_source_changed ||
			app_requested_outcomes.request_go_compile,
		build_critical_css:                  critical_css_changed,
		build_normal_css:                    normal_css_changed,
		process_public_static_assets:        public_static_changed,
		cleanup_stale_public_static_outputs: public_static_changed,
		process_private_static_assets:       private_static_changed,
		generate_public_file_map:            public_static_changed,

		request_backend_restart:                    request_backend_restart,
		request_vite_restart:                       request_vite_restart,
		requested_terminal_browser_action:          merged_browser_action_intent,
		wave_public_file_map_notif_destination_key: facts.wave_public_file_map_notif_destination_key,
		fw_execution_registrations:                 facts.fw_execution_registrations,
		fw_requested_effects: fw_new_requested_effects_pointer_if_any(
			fw_requested_effects_val,
		),
	}
	if facts.mode == mode_prod {
		requested_effects.restart_dev_server_cycle = false
	}

	requested_effects.run_requested_build_effects =
		requested_effects.compile_go_binary ||
			requested_effects.build_critical_css ||
			requested_effects.build_normal_css ||
			requested_effects.process_public_static_assets ||
			requested_effects.cleanup_stale_public_static_outputs ||
			requested_effects.process_private_static_assets ||
			requested_effects.generate_public_file_map

	return requested_effects
}
