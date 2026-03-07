package wavebuild

import (
	"context"
	"reflect"
	"testing"
)

type pipeline_effect_matrix_test_case struct {
	name                                       string
	mode                                       mode
	waiting_for_build_retry                    bool
	events                                     []observed_batch_event
	wave_public_file_map_notif_destination_key fw_notif_destination_key
	app_requested_outcomes                     app_requested_outcomes
	expected_effect_labels                     []string
}

func Test_run_five_phase_pipeline_explicit_effect_matrix(t *testing.T) {
	t.Setenv(wave_internal_test_mode_env_var, "1")

	test_cases := []pipeline_effect_matrix_test_case{
		{
			name: "dev-noise-only-batch-produces-no-reload-notice",
			mode: mode_dev,
			events: []observed_batch_event{
				{
					event_type: event_type_ignored_or_noise_changed,
					noise_only: true,
				},
			},
			expected_effect_labels: []string{
				_LABEL_P5_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
		},
		{
			name: "dev-noop-config-mutation-produces-no-reload-notice",
			mode: mode_dev,
			events: []observed_batch_event{
				{
					event_type:            event_type_config_file_changed,
					no_op_config_mutation: true,
				},
			},
			expected_effect_labels: []string{
				_LABEL_P5_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
		},
		{
			name: "dev-critical-css-change-builds-critical-css-and-css-hot-reload",
			mode: mode_dev,
			events: []observed_batch_event{
				{event_type: event_type_critical_css_source_changed},
			},
			expected_effect_labels: []string{
				_LABEL_P2_BUILD_CRITICAL_CSS,
				_LABEL_P5_BROADCAST_CSS_HOT_RELOAD,
			},
		},
		{
			name: "dev-normal-css-change-builds-normal-css-and-css-hot-reload",
			mode: mode_dev,
			events: []observed_batch_event{
				{event_type: event_type_normal_css_source_changed},
			},
			expected_effect_labels: []string{
				_LABEL_P2_BUILD_NORMAL_CSS,
				_LABEL_P5_BROADCAST_CSS_HOT_RELOAD,
			},
		},
		{
			name: "dev-go-source-change-builds-binary-restarts-app-and-hard-reloads",
			mode: mode_dev,
			events: []observed_batch_event{
				{event_type: event_type_go_source_changed},
			},
			expected_effect_labels: []string{
				_LABEL_P2_BUILD_GO_BINARY,
				_LABEL_P3_RESTART_APP_PROCESS,
				_LABEL_P4_AWAIT_BACKEND_READINESS,
				_LABEL_P5_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "dev-public-static-change-without-destination-key-notifies-vite-without-backend-convergence",
			mode: mode_dev,
			events: []observed_batch_event{
				{event_type: event_type_public_static_asset_changed},
			},
			expected_effect_labels: []string{
				_LABEL_P2_PROCESS_PUBLIC_STATIC_ASSETS,
				_LABEL_P2_CLEANUP_STALE_PUBLIC_STATIC,
				_LABEL_P5_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
			},
		},
		{
			name: "dev-public-static-change-processes-assets-emits-fw-notif-and-notifies-vite",
			mode: mode_dev,
			events: []observed_batch_event{
				{event_type: event_type_public_static_asset_changed},
			},
			wave_public_file_map_notif_destination_key: fw_notif_destination_key(
				"fw.public-filemap-reload",
			),
			expected_effect_labels: []string{
				_LABEL_P2_PROCESS_PUBLIC_STATIC_ASSETS,
				_LABEL_P2_CLEANUP_STALE_PUBLIC_STATIC,
				_LABEL_P4_AWAIT_BACKEND_READINESS,
				_LABEL_P4_EXECUTE_Fw_NOTIFICATION + "[fw.public-filemap-reload]",
				_LABEL_P5_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
			},
		},
		{
			name: "dev-public-static-and-config-change-executes-fw-notif-and-hard-reload-instead-of-vite-notify",
			mode: mode_dev,
			events: []observed_batch_event{
				{event_type: event_type_public_static_asset_changed},
				{event_type: event_type_config_file_changed},
			},
			wave_public_file_map_notif_destination_key: fw_notif_destination_key(
				"fw.public-filemap-reload",
			),
			expected_effect_labels: []string{
				_LABEL_P2_PROCESS_PUBLIC_STATIC_ASSETS,
				_LABEL_P2_CLEANUP_STALE_PUBLIC_STATIC,
				_LABEL_P3_APPLY_DEV_SERVER_RESTART,
				_LABEL_P3_RESTART_VITE_PROCESS,
				_LABEL_P4_AWAIT_BACKEND_READINESS,
				_LABEL_P4_EXECUTE_Fw_NOTIFICATION + "[fw.public-filemap-reload]",
				_LABEL_P5_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "dev-private-static-change-processes-private-assets-and-hard-reloads",
			mode: mode_dev,
			events: []observed_batch_event{
				{event_type: event_type_private_static_asset_changed},
			},
			expected_effect_labels: []string{
				_LABEL_P2_PROCESS_PRIVATE_STATIC_ASSETS,
				_LABEL_P5_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "dev-fw-requested-backend-mutation-effect-executes-and-hard-reloads",
			mode: mode_dev,
			events: []observed_batch_event{
				{
					event_type: fw_event_type_requested_effects_changed,
					fw_requested_effects: fw_requested_effects{
						backend_mutation_effect_keys: []fw_mutation_effect_key{
							"fw.reload-routes",
						},
					},
				},
			},
			app_requested_outcomes: app_requested_outcomes{
				requested_terminal_browser_action: frontend_terminal_browser_action_hard_reload,
			},
			expected_effect_labels: []string{
				_LABEL_P3_EXECUTE_Fw_MUTATION_EFFECT + "[fw.reload-routes]",
				_LABEL_P4_AWAIT_BACKEND_READINESS,
				_LABEL_P5_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "dev-fw-requested-notif-executes-and-allows-no-reload-terminal-action",
			mode: mode_dev,
			events: []observed_batch_event{
				{
					event_type: fw_event_type_requested_effects_changed,
					fw_requested_effects: fw_requested_effects{
						backend_convergence_notif_queue: []fw_notif_request{
							{
								destination_key: "fw.runtime-notify",
								trigger:         "fw-watch-change",
							},
						},
					},
				},
			},
			expected_effect_labels: []string{
				_LABEL_P4_AWAIT_BACKEND_READINESS,
				_LABEL_P4_EXECUTE_Fw_NOTIFICATION + "[fw.runtime-notify]",
				_LABEL_P5_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
		},
		{
			name: "dev-fw-notif-and-app-requested-hard-reload-both-execute",
			mode: mode_dev,
			events: []observed_batch_event{
				{event_type: event_type_app_defined_watch_action_only_changed},
			},
			app_requested_outcomes: app_requested_outcomes{
				requested_terminal_browser_action: frontend_terminal_browser_action_hard_reload,
				fw_requested_effects: fw_requested_effects{
					backend_convergence_notif_queue: []fw_notif_request{
						{
							destination_key: "fw.runtime-notify",
							trigger:         "fw-watch-change",
						},
					},
				},
			},
			expected_effect_labels: []string{
				_LABEL_P4_AWAIT_BACKEND_READINESS,
				_LABEL_P4_EXECUTE_Fw_NOTIFICATION + "[fw.runtime-notify]",
				_LABEL_P5_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "dev-config-change-restarts-dev-server-cycle-and-vite-then-hard-reloads",
			mode: mode_dev,
			events: []observed_batch_event{
				{event_type: event_type_config_file_changed},
			},
			expected_effect_labels: []string{
				_LABEL_P3_APPLY_DEV_SERVER_RESTART,
				_LABEL_P3_RESTART_VITE_PROCESS,
				_LABEL_P4_AWAIT_BACKEND_READINESS,
				_LABEL_P5_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "dev-app-action-only-revalidate-requests-revalidate-without-build-or-backend-settling",
			mode: mode_dev,
			events: []observed_batch_event{
				{event_type: event_type_app_defined_watch_action_only_changed},
			},
			app_requested_outcomes: app_requested_outcomes{
				requested_terminal_browser_action: frontend_terminal_browser_action_revalidate,
			},
			expected_effect_labels: []string{
				_LABEL_P5_BROADCAST_REVALIDATE,
			},
		},
		{
			name: "dev-app-action-only-fw-effect-request-executes-and-hard-reloads",
			mode: mode_dev,
			events: []observed_batch_event{
				{event_type: event_type_app_defined_watch_action_only_changed},
			},
			app_requested_outcomes: app_requested_outcomes{
				requested_terminal_browser_action: frontend_terminal_browser_action_hard_reload,
				fw_requested_effects: fw_requested_effects{
					backend_mutation_effect_keys: []fw_mutation_effect_key{
						"fw.refresh-runtime-cache",
					},
				},
			},
			expected_effect_labels: []string{
				_LABEL_P3_EXECUTE_Fw_MUTATION_EFFECT + "[fw.refresh-runtime-cache]",
				_LABEL_P4_AWAIT_BACKEND_READINESS,
				_LABEL_P5_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name:                    "dev-waiting-for-build-retry-queues-retry-wait-restart-and-skips-reload",
			mode:                    mode_dev,
			waiting_for_build_retry: true,
			events: []observed_batch_event{
				{event_type: event_type_go_source_changed},
			},
			expected_effect_labels: []string{
				_LABEL_P3_QUEUE_RETRY_WAIT_RESTART,
				_LABEL_P5_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
		},
		{
			name: "dev-mixed-go-css-and-public-static-runs-deduped-build-and-hard-reload",
			mode: mode_dev,
			events: []observed_batch_event{
				{event_type: event_type_go_source_changed},
				{event_type: event_type_normal_css_source_changed},
				{event_type: event_type_public_static_asset_changed},
			},
			expected_effect_labels: []string{
				_LABEL_P2_BUILD_GO_BINARY,
				_LABEL_P2_BUILD_NORMAL_CSS,
				_LABEL_P2_PROCESS_PUBLIC_STATIC_ASSETS,
				_LABEL_P2_CLEANUP_STALE_PUBLIC_STATIC,
				_LABEL_P3_RESTART_APP_PROCESS,
				_LABEL_P4_AWAIT_BACKEND_READINESS,
				_LABEL_P5_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "dev-app-requested-outcomes-are-ignored-without-app-defined-event",
			mode: mode_dev,
			events: []observed_batch_event{
				{event_type: event_type_normal_css_source_changed},
			},
			app_requested_outcomes: app_requested_outcomes{
				requested_terminal_browser_action: frontend_terminal_browser_action_hard_reload,
				request_restart:                   true,
				request_go_compile:                true,
				fw_requested_effects: fw_requested_effects{
					backend_mutation_effect_keys: []fw_mutation_effect_key{
						"fw.unused-without-app-defined-event",
					},
				},
			},
			expected_effect_labels: []string{
				_LABEL_P2_BUILD_NORMAL_CSS,
				_LABEL_P5_BROADCAST_CSS_HOT_RELOAD,
			},
		},
		{
			name: "prod-runs-build-only-with-canonical-build-effects",
			mode: mode_prod,
			events: []observed_batch_event{
				{
					event_type: event_type_ignored_or_noise_changed,
					noise_only: true,
				},
			},
			expected_effect_labels: []string{
				_LABEL_P2_BUILD_GO_BINARY,
				_LABEL_P2_BUILD_CRITICAL_CSS,
				_LABEL_P2_BUILD_NORMAL_CSS,
				_LABEL_P2_PROCESS_PUBLIC_STATIC_ASSETS,
				_LABEL_P2_CLEANUP_STALE_PUBLIC_STATIC,
				_LABEL_P2_PROCESS_PRIVATE_STATIC_ASSETS,
			},
		},
		{
			name: "prod-ignores-dev-runtime-and-fw-settling-requests",
			mode: mode_prod,
			events: []observed_batch_event{
				{event_type: event_type_config_file_changed},
				{event_type: event_type_go_source_changed},
				{event_type: fw_event_type_requested_effects_changed},
			},
			wave_public_file_map_notif_destination_key: fw_notif_destination_key(
				"fw.public-filemap-reload",
			),
			app_requested_outcomes: app_requested_outcomes{
				requested_terminal_browser_action: frontend_terminal_browser_action_hard_reload,
				request_restart:                   true,
				request_go_compile:                true,
				fw_requested_effects: fw_requested_effects{
					backend_mutation_effect_keys: []fw_mutation_effect_key{
						"fw.should-not-run-in-prod",
					},
					backend_convergence_notif_queue: []fw_notif_request{
						{
							destination_key: "fw.should-not-notify-in-prod",
							trigger:         "prod-ignore",
						},
					},
				},
			},
			expected_effect_labels: []string{
				_LABEL_P2_BUILD_GO_BINARY,
				_LABEL_P2_BUILD_CRITICAL_CSS,
				_LABEL_P2_BUILD_NORMAL_CSS,
				_LABEL_P2_PROCESS_PUBLIC_STATIC_ASSETS,
				_LABEL_P2_CLEANUP_STALE_PUBLIC_STATIC,
				_LABEL_P2_PROCESS_PRIVATE_STATIC_ASSETS,
			},
		},
	}

	for _, test_case := range test_cases {
		t.Run(test_case.name, func(t *testing.T) {
			recorded_effect_labels := run_pipeline_and_record_effect_labels(
				t,
				test_case,
			)
			assert_unordered_effect_labels(
				t,
				recorded_effect_labels,
				test_case.expected_effect_labels,
			)
		})
	}
}

func run_pipeline_and_record_effect_labels(
	t *testing.T,
	test_case pipeline_effect_matrix_test_case,
) []string {
	t.Helper()

	effect_recorder := &test_effect_recorder{}
	native_context_with_recorder := with_test_effect_recorder(
		context.Background(),
		effect_recorder,
	)
	_, err := run_five_phase_pipeline(
		native_context_with_recorder,
		p1_batch_input{
			p1: &p1_input{
				mode:          test_case.mode,
				generation_id: "test-generation",
				events:        test_case.events,
				wave_public_file_map_notif_destination_key: test_case.
					wave_public_file_map_notif_destination_key,
				app_requested_outcomes:  test_case.app_requested_outcomes,
				waiting_for_build_retry: test_case.waiting_for_build_retry,
			},
		},
	)
	if err != nil {
		t.Fatalf("runFivePhasePipeline returned error: %v", err)
	}
	return effect_recorder.snapshot_effect_labels()
}

func assert_unordered_effect_labels(
	t *testing.T,
	recorded_effect_labels []string,
	expected_effect_labels []string,
) {
	t.Helper()
	if !reflect.DeepEqual(
		count_effect_labels(recorded_effect_labels),
		count_effect_labels(expected_effect_labels),
	) {
		t.Fatalf(
			"effect labels mismatch\nrecorded=%v\nexpected=%v",
			recorded_effect_labels,
			expected_effect_labels,
		)
	}
}

func count_effect_labels(labels []string) map[string]int {
	label_counts := make(map[string]int, len(labels))
	for _, label := range labels {
		label_counts[label]++
	}
	return label_counts
}
