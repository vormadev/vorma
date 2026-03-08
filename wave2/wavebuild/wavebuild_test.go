package wavebuild

import (
	"context"
	"strings"
	"testing"
)

func Test_run_phase_task_pipeline_black_box_event_matrix(
	t *testing.T,
) {
	t.Setenv(wave_internal_test_mode_env_var, "1")

	test_cases := []struct {
		name                  string
		events                []batch_watcher_event
		expect_error_contains string
		expect_present_labels []string
		expect_absent_labels  []string
		expect_min_hits       map[string]int
	}{
		{
			name:                  "empty-batch-publishes-no-reload-notice",
			events:                nil,
			expect_error_contains: "",
			expect_present_labels: []string{
				_LABEL_TASK_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_MATERIALIZE_GO_BINARY_STATE,
				_LABEL_TASK_RESTART_APP_RUNTIME_PROCESS,
				_LABEL_TASK_AWAIT_APP_READINESS_IF_REQUIRED,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "chmod-only-batch-publishes-no-reload-notice",
			events: []batch_watcher_event{
				{
					path: "backend/main.go",
					op:   batch_watcher_event_op_chmod,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
		},
		{
			name: "unmatched-other-file-batch-publishes-no-reload-notice",
			events: []batch_watcher_event{
				{
					path: "README.md",
					op:   batch_watcher_event_op_write,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_MATERIALIZE_GO_BINARY_STATE,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "run-on-change-only-event-publishes-no-reload-notice",
			events: []batch_watcher_event{
				{
					path:               "src/feature.flag",
					op:                 batch_watcher_event_op_write,
					run_on_change_only: true,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_MATERIALIZE_GO_BINARY_STATE,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "config-change-short-circuits",
			events: []batch_watcher_event{
				{
					path: "backend/wave.config.json",
					op:   batch_watcher_event_op_write,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_RESTART_DEV_SERVER_CYCLE,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_MATERIALIZE_GO_BINARY_STATE,
				_LABEL_TASK_MATERIALIZE_NORMAL_CSS_STATE,
				_LABEL_TASK_PROCESS_PUBLIC_STATIC_ASSETS,
				_LABEL_TASK_RESTART_APP_RUNTIME_PROCESS,
				_LABEL_TASK_AWAIT_APP_READINESS_IF_REQUIRED,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
				_LABEL_TASK_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
				_LABEL_TASK_BROADCAST_CSS_HOT_RELOAD,
				_LABEL_TASK_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
		},
		{
			name: "config-create-short-circuits",
			events: []batch_watcher_event{
				{
					path: "backend/wave.config.json",
					op:   batch_watcher_event_op_create,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_RESTART_DEV_SERVER_CYCLE,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_MATERIALIZE_GO_BINARY_STATE,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
				_LABEL_TASK_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
		},
		{
			name: "config-remove-short-circuits",
			events: []batch_watcher_event{
				{
					path: "backend/wave.config.json",
					op:   batch_watcher_event_op_remove,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_RESTART_DEV_SERVER_CYCLE,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_MATERIALIZE_GO_BINARY_STATE,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
				_LABEL_TASK_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
		},
		{
			name: "config-rename-short-circuits",
			events: []batch_watcher_event{
				{
					path: "backend/wave.config.json",
					op:   batch_watcher_event_op_rename,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_RESTART_DEV_SERVER_CYCLE,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_MATERIALIZE_GO_BINARY_STATE,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
				_LABEL_TASK_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
		},
		{
			name: "config-path-alias-still-short-circuits",
			events: []batch_watcher_event{
				{
					path: "backend\\./wave.config.json",
					op:   batch_watcher_event_op_write,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_RESTART_DEV_SERVER_CYCLE,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_MATERIALIZE_GO_BINARY_STATE,
				_LABEL_TASK_RESTART_APP_RUNTIME_PROCESS,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
				_LABEL_TASK_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
		},
		{
			name: "build-retry-wait-short-circuits",
			events: []batch_watcher_event{
				{
					path:                    "backend/main.go",
					op:                      batch_watcher_event_op_write,
					waiting_for_build_retry: true,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_QUEUE_RETRY_WAIT_RESTART,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_MATERIALIZE_GO_BINARY_STATE,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
				_LABEL_TASK_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
		},
		{
			name: "config-short-circuit-precedence-over-build-retry",
			events: []batch_watcher_event{
				{
					path:                    "backend/wave.config.json",
					op:                      batch_watcher_event_op_write,
					waiting_for_build_retry: true,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_RESTART_DEV_SERVER_CYCLE,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_QUEUE_RETRY_WAIT_RESTART,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "single-go-event-drives-hard-reload-path",
			events: []batch_watcher_event{
				{
					path: "backend/main.go",
					op:   batch_watcher_event_op_write,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_MATERIALIZE_GO_BINARY_STATE,
				_LABEL_TASK_RESTART_APP_RUNTIME_PROCESS,
				_LABEL_TASK_RESTART_VITE_RUNTIME_PROCESS,
				_LABEL_TASK_EXECUTE_FRAMEWORK_BACKEND_MUTATION_EFFECTS,
				_LABEL_TASK_AWAIT_APP_READINESS_IF_REQUIRED,
				_LABEL_TASK_EXECUTE_FRAMEWORK_BACKEND_NOTIFICATIONS,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_BROADCAST_CSS_HOT_RELOAD,
				_LABEL_TASK_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
				_LABEL_TASK_BROADCAST_REVALIDATE,
			},
		},
		{
			name: "go-event-treated-as-non-go-downgrades-to-no-reload-notice",
			events: []batch_watcher_event{
				{
					path:            "backend/main.go",
					op:              batch_watcher_event_op_write,
					treat_as_non_go: true,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_MATERIALIZE_GO_BINARY_STATE,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "run-on-change-only-with-revalidate-flag-still-publishes-no-reload-notice",
			events: []batch_watcher_event{
				{
					path:                                    "src/content.md",
					op:                                      batch_watcher_event_op_write,
					run_on_change_only:                      true,
					only_run_client_defined_revalidate_func: true,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_BROADCAST_REVALIDATE,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "run-on-change-only-with-restart-flags-still-publishes-no-reload-notice",
			events: []batch_watcher_event{
				{
					path:                "src/feature.flag",
					op:                  batch_watcher_event_op_write,
					run_on_change_only:  true,
					recompile_go_binary: true,
					restart_app:         true,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_MATERIALIZE_GO_BINARY_STATE,
				_LABEL_TASK_RESTART_APP_RUNTIME_PROCESS,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "go-event-treated-as-non-go-can-still-request-go-recompile-hard-reload",
			events: []batch_watcher_event{
				{
					path:                "backend/main.go",
					op:                  batch_watcher_event_op_write,
					treat_as_non_go:     true,
					recompile_go_binary: true,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_MATERIALIZE_GO_BINARY_STATE,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "other-file-with-restart-app-flag-uses-hard-reload",
			events: []batch_watcher_event{
				{
					path:        "src/template.txt",
					op:          batch_watcher_event_op_write,
					restart_app: true,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_RESTART_APP_RUNTIME_PROCESS,
				_LABEL_TASK_AWAIT_APP_READINESS_IF_REQUIRED,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "revalidate-only-event-uses-revalidate-terminal-action",
			events: []batch_watcher_event{
				{
					path:                                    "src/content.md",
					op:                                      batch_watcher_event_op_write,
					only_run_client_defined_revalidate_func: true,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_BROADCAST_REVALIDATE,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
				_LABEL_TASK_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
			},
		},
		{
			name: "critical-css-only-uses-css-hot-reload",
			events: []batch_watcher_event{
				{
					path: "frontend/site.critical.css",
					op:   batch_watcher_event_op_write,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_MATERIALIZE_CRITICAL_CSS_STATE,
				_LABEL_TASK_BROADCAST_CSS_HOT_RELOAD,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
				_LABEL_TASK_BROADCAST_REVALIDATE,
			},
		},
		{
			name: "css-with-revalidate-preference-uses-revalidate",
			events: []batch_watcher_event{
				{
					path:                                    "frontend/site.css",
					op:                                      batch_watcher_event_op_write,
					only_run_client_defined_revalidate_func: true,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_MATERIALIZE_NORMAL_CSS_STATE,
				_LABEL_TASK_BROADCAST_REVALIDATE,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_BROADCAST_CSS_HOT_RELOAD,
			},
		},
		{
			name: "public-static-only-notifies-vite",
			events: []batch_watcher_event{
				{
					path: "assets/public/logo.svg",
					op:   batch_watcher_event_op_write,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_PROCESS_PUBLIC_STATIC_ASSETS,
				_LABEL_TASK_CLEANUP_STALE_PUBLIC_STATIC_OUTPUTS,
				_LABEL_TASK_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
				_LABEL_TASK_BROADCAST_REVALIDATE,
			},
		},
		{
			name: "public-static-create-notifies-vite",
			events: []batch_watcher_event{
				{
					path: "assets/public/logo.svg",
					op:   batch_watcher_event_op_create,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "public-static-remove-notifies-vite",
			events: []batch_watcher_event{
				{
					path: "assets/public/logo.svg",
					op:   batch_watcher_event_op_remove,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "public-static-rename-notifies-vite",
			events: []batch_watcher_event{
				{
					path: "assets/public/logo.svg",
					op:   batch_watcher_event_op_rename,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "public-static-with-revalidate-preference-still-notifies-vite",
			events: []batch_watcher_event{
				{
					path:                                    "assets/public/logo.svg",
					op:                                      batch_watcher_event_op_write,
					only_run_client_defined_revalidate_func: true,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_BROADCAST_REVALIDATE,
			},
		},
		{
			name: "private-static-only-uses-hard-reload",
			events: []batch_watcher_event{
				{
					path: "assets/private/home.html",
					op:   batch_watcher_event_op_write,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_PROCESS_PRIVATE_STATIC_ASSETS,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
				_LABEL_TASK_BROADCAST_REVALIDATE,
			},
		},
		{
			name: "private-static-create-uses-hard-reload",
			events: []batch_watcher_event{
				{
					path: "assets/private/home.html",
					op:   batch_watcher_event_op_create,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
			},
		},
		{
			name: "private-static-remove-uses-hard-reload",
			events: []batch_watcher_event{
				{
					path: "assets/private/home.html",
					op:   batch_watcher_event_op_remove,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
			},
		},
		{
			name: "private-static-rename-uses-hard-reload",
			events: []batch_watcher_event{
				{
					path: "assets/private/home.html",
					op:   batch_watcher_event_op_rename,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
			},
		},
		{
			name: "public-and-css-prefers-notify-vite-over-css-hot-reload",
			events: []batch_watcher_event{
				{
					path: "assets/public/logo.svg",
					op:   batch_watcher_event_op_write,
				},
				{
					path: "frontend/site.css",
					op:   batch_watcher_event_op_write,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_PROCESS_PUBLIC_STATIC_ASSETS,
				_LABEL_TASK_MATERIALIZE_NORMAL_CSS_STATE,
				_LABEL_TASK_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_BROADCAST_CSS_HOT_RELOAD,
			},
		},
		{
			name: "public-and-private-prefers-hard-reload-over-notify-vite",
			events: []batch_watcher_event{
				{
					path: "assets/public/logo.svg",
					op:   batch_watcher_event_op_write,
				},
				{
					path: "assets/private/home.html",
					op:   batch_watcher_event_op_write,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_PROCESS_PUBLIC_STATIC_ASSETS,
				_LABEL_TASK_PROCESS_PRIVATE_STATIC_ASSETS,
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
			},
		},
		{
			name: "hard-reload-precedence-over-revalidate",
			events: []batch_watcher_event{
				{
					path:                                    "src/content.md",
					op:                                      batch_watcher_event_op_write,
					only_run_client_defined_revalidate_func: true,
				},
				{
					path: "assets/private/home.html",
					op:   batch_watcher_event_op_write,
				},
			},
			expect_present_labels: []string{
				_LABEL_TASK_BROADCAST_HARD_RELOAD,
			},
			expect_absent_labels: []string{
				_LABEL_TASK_BROADCAST_REVALIDATE,
			},
		},
	}

	for _, test_case := range test_cases {
		test_case := test_case
		t.Run(test_case.name, func(t *testing.T) {
			external_effect_labels, run_error := run_phase_task_pipeline_for_event_batch_for_tests(
				t,
				test_case.events,
			)
			if strings.TrimSpace(test_case.expect_error_contains) == "" {
				if run_error != nil {
					t.Fatalf("run error: %v", run_error)
				}
			} else {
				if run_error == nil {
					t.Fatalf(
						"expected run error containing %q",
						test_case.expect_error_contains,
					)
				}
				if !strings.Contains(
					run_error.Error(),
					test_case.expect_error_contains,
				) {
					t.Fatalf(
						"run error mismatch: got %q, want contains %q",
						run_error.Error(),
						test_case.expect_error_contains,
					)
				}
			}
			assert_effect_labels_contain_all(
				t,
				external_effect_labels,
				test_case.expect_present_labels,
			)
			assert_effect_labels_contain_none(
				t,
				external_effect_labels,
				test_case.expect_absent_labels,
			)
			assert_effect_labels_min_hits(
				t,
				external_effect_labels,
				test_case.expect_min_hits,
			)
		})
	}
}

func Test_run_phase_task_pipeline_input_contract(
	t *testing.T,
) {
	t.Setenv(wave_internal_test_mode_env_var, "1")

	test_cases := []struct {
		name                  string
		input                 p1_batch_input
		expect_error_contains string
	}{
		{
			name:                  "missing-phase-1-input-errors",
			input:                 p1_batch_input{},
			expect_error_contains: err_p1_input_required.Error(),
		},
		{
			name: "empty-generation-id-errors",
			input: p1_batch_input{
				p1: &p1_input{
					mode:                 mode_dev,
					generation_id:        "   ",
					batch_watcher_events: nil,
				},
			},
			expect_error_contains: err_generation_id_required.Error(),
		},
		{
			name: "invalid-mode-errors",
			input: p1_batch_input{
				p1: &p1_input{
					mode:          mode("bad"),
					generation_id: "matrix-tests",
					batch_watcher_events: []batch_watcher_event{
						{
							path: "backend/main.go",
							op:   batch_watcher_event_op_write,
						},
					},
				},
			},
			expect_error_contains: "mode \"bad\" is unsupported",
		},
	}

	for _, test_case := range test_cases {
		test_case := test_case
		t.Run(test_case.name, func(t *testing.T) {
			run_error := run_phase_task_pipeline(
				context.Background(),
				test_case.input,
			)
			if run_error == nil {
				t.Fatalf(
					"expected run error containing %q",
					test_case.expect_error_contains,
				)
			}
			if strings.Contains(
				run_error.Error(),
				test_case.expect_error_contains,
			) {
				return
			}
			t.Fatalf(
				"run error mismatch: got %q, want contains %q",
				run_error.Error(),
				test_case.expect_error_contains,
			)
		})
	}
}

func run_phase_task_pipeline_for_event_batch_for_tests(
	t *testing.T,
	events []batch_watcher_event,
) ([]string, error) {
	t.Helper()
	effect_recorder := &test_effect_recorder{}
	parent_context := with_test_effect_recorder(
		context.Background(),
		effect_recorder,
	)
	run_error := run_phase_task_pipeline(
		parent_context,
		p1_batch_input{
			p1: &p1_input{
				mode:                 mode_dev,
				generation_id:        "matrix-tests",
				batch_watcher_events: events,
			},
		},
	)
	return extract_external_effect_labels_for_tests(
		effect_recorder.snapshot_effect_labels(),
	), run_error
}

func extract_external_effect_labels_for_tests(
	effect_labels []string,
) []string {
	external_effect_labels := make([]string, 0, len(effect_labels))
	for _, effect_label := range effect_labels {
		if !strings.HasSuffix(effect_label, ":done") {
			continue
		}
		external_effect_labels = append(
			external_effect_labels,
			strings.TrimSuffix(effect_label, ":done"),
		)
	}
	return external_effect_labels
}

func assert_effect_labels_contain_all(
	t *testing.T,
	effect_labels []string,
	expected_labels []string,
) {
	t.Helper()
	for _, expected_label := range expected_labels {
		if count_effect_label_hits(effect_labels, expected_label) > 0 {
			continue
		}
		t.Fatalf(
			"missing expected effect %q in labels=%v",
			expected_label,
			effect_labels,
		)
	}
}

func assert_effect_labels_contain_none(
	t *testing.T,
	effect_labels []string,
	unexpected_labels []string,
) {
	t.Helper()
	for _, unexpected_label := range unexpected_labels {
		if count_effect_label_hits(effect_labels, unexpected_label) == 0 {
			continue
		}
		t.Fatalf(
			"found unexpected effect %q in labels=%v",
			unexpected_label,
			effect_labels,
		)
	}
}

func assert_effect_labels_min_hits(
	t *testing.T,
	effect_labels []string,
	expected_min_hits map[string]int,
) {
	t.Helper()
	for effect_label, min_hits := range expected_min_hits {
		if count_effect_label_hits(effect_labels, effect_label) >= min_hits {
			continue
		}
		t.Fatalf(
			"effect %q hit count too low: got=%d min=%d labels=%v",
			effect_label,
			count_effect_label_hits(effect_labels, effect_label),
			min_hits,
			effect_labels,
		)
	}
}

func count_effect_label_hits(effect_labels []string, effect_label string) int {
	hit_count := 0
	for _, current_label := range effect_labels {
		if current_label == effect_label {
			hit_count++
		}
	}
	return hit_count
}
