package wavebuild

import (
	"context"
	"os"
	"sync"

	"github.com/vormadev/vorma/kit/tasks"
)

const wave_internal_test_mode_env_var = "__WAVE_INTERNAL_TEST_MODE"

/////////////////////////////////////////////////////////////////////
/////// Effect Labels
/////////////////////////////////////////////////////////////////////

const (
	_LABEL_TASK_DERIVE_BATCH_FACTS                     = "task.derive-batch-facts"
	_LABEL_TASK_TRACK_WATCHER_EVENTS                   = "task.track-watcher-events"
	_LABEL_TASK_ENSURE_DIRECTORY_WATCH_FOR_EVENT_PATHS = "task.ensure-directory-watch-for-event-paths"

	_LABEL_TASK_NORMALIZE_BATCH_WATCHER_EVENTS                                      = "task.normalize-batch-watcher-events"
	_LABEL_TASK_DEDUPLICATE_WATCHER_EVENTS                                          = "task.deduplicate-watcher-events"
	_LABEL_TASK_NORMALIZE_WATCHER_EVENT_PATHS                                       = "task.normalize-watcher-event-paths"
	_LABEL_TASK_APPLY_CONFIG_RELOAD_PRE_CLASSIFICATION_SIDE_EFFECTS                 = "task.apply-config-reload-pre-classification-side-effects"
	_LABEL_TASK_DETECT_CONFIG_MUTATION_WATCHER_EVENTS                               = "task.detect-config-mutation-watcher-events"
	_LABEL_TASK_DERIVE_CONFIG_CHANGE_BATCH_SKIP_NON_CONFIG_HOOK_PROCESSING_DECISION = "task.derive-config-change-batch-skip-non-config-hook-processing-decision"
	_LABEL_TASK_RELOAD_CONFIG_IF_CHANGED                                            = "task.reload-config-if-changed"
	_LABEL_TASK_EMIT_NOOP_CONFIG_RELOAD_NOTICE                                      = "task.emit-noop-config-reload-notice"

	_LABEL_TASK_CLASSIFY_WATCHER_EVENTS_FOR_PROCESSING      = "task.classify-watcher-events-for-processing"
	_LABEL_TASK_DERIVE_PRE_CLASSIFICATION_DECISIONS         = "task.derive-pre-classification-decisions"
	_LABEL_TASK_DERIVE_INITIAL_FILE_TYPE_FOR_WATCHER_EVENTS = "task.derive-initial-file-type-for-watcher-events"
	_LABEL_TASK_APPLY_WATCHED_FILE_OVERRIDES                = "task.apply-watched-file-overrides"
	_LABEL_TASK_DERIVE_WATCHER_EVENT_IGNORED_STATUS         = "task.derive-watcher-event-ignored-status"
	_LABEL_TASK_DERIVE_CHMOD_ONLY_DECISIONS                 = "task.derive-chmod-only-decisions"
	_LABEL_TASK_FILTER_CLASSIFIED_EVENTS_FOR_PROCESSING     = "task.filter-classified-events-for-processing"

	_LABEL_TASK_BUILD_EVENT_HOOKS_FOR_PROCESSING                       = "task.build-event-hooks-for-processing"
	_LABEL_TASK_BUILD_NORMALIZED_CHANGED_FILE_PATHS_BY_WATCHED_PATTERN = "task.build-normalized-changed-file-paths-by-watched-pattern"
	_LABEL_TASK_BUILD_SKIP_DUPLICATE_HOOKS_BY_CLASSIFIED_EVENT_INDEX   = "task.build-skip-duplicate-hooks-by-classified-event-index"
	_LABEL_TASK_BUILD_HOOK_CONTEXTS_FOR_EVENTS                         = "task.build-hook-contexts-for-events"

	_LABEL_TASK_DERIVE_EVENT_EXECUTION_FLOW_DECISION                     = "task.derive-event-execution-flow-decision"
	_LABEL_TASK_DERIVE_EVENT_EXECUTION_PLAN_BEHAVIORAL_DECISION          = "task.derive-event-execution-plan-behavioral-decision"
	_LABEL_TASK_DERIVE_CONFIG_RESTART_SHORT_CIRCUIT_DECISION             = "task.derive-config-restart-short-circuit-decision"
	_LABEL_TASK_DERIVE_WAITING_FOR_BUILD_RETRY_SHORT_CIRCUIT_DECISION    = "task.derive-waiting-for-build-retry-short-circuit-decision"
	_LABEL_TASK_BUILD_WATCHER_EVENT_LOG_PAYLOADS_FOR_EVENTS_WITH_HOOKS   = "task.build-watcher-event-log-payloads-for-events-with-hooks"
	_LABEL_TASK_BUILD_WATCHER_EVENT_EXECUTION_INPUT_FROM_PLANNING_RESULT = "task.build-watcher-event-execution-input-from-planning-result"
	_LABEL_TASK_LOG_WATCHER_BATCH_FROM_PLANNED_PAYLOADS                  = "task.log-watcher-batch-from-planned-payloads"
	_LABEL_TASK_DERIVE_NOOP_PIPELINE_SHORT_CIRCUIT_DECISION              = "task.derive-noop-pipeline-short-circuit-decision"
	_LABEL_TASK_APPLY_CONTROL_FLOW_SHORT_CIRCUIT_SIDE_EFFECTS            = "task.apply-control-flow-short-circuit-side-effects"
	_LABEL_TASK_BROADCAST_REBUILDING_OVERLAY_FOR_CONFIG_RESTART          = "task.broadcast-rebuilding-overlay-for-config-restart"
	_LABEL_TASK_EMIT_BUILD_RETRY_WAIT_NOTICE                             = "task.emit-build-retry-wait-notice"
	_LABEL_TASK_DERIVE_INITIAL_REQUESTED_OUTCOMES                        = "task.derive-initial-requested-outcomes"
	_LABEL_TASK_DERIVE_APP_REQUESTED_OUTCOMES                            = "task.derive-app-requested-outcomes"
	_LABEL_TASK_DERIVE_FRAMEWORK_REQUESTED_OUTCOMES                      = "task.derive-framework-requested-outcomes"
	_LABEL_TASK_DERIVE_INITIAL_BUILD_AND_BROWSER_INTENTS                 = "task.derive-initial-build-and-browser-intents"

	_LABEL_TASK_APPLY_PRE_HOOK_ADJUSTMENTS                              = "task.apply-pre-hook-adjustments"
	_LABEL_TASK_PREPARE_EXECUTION_PIPELINE_FOR_EVENTS                   = "task.prepare-execution-pipeline-for-events"
	_LABEL_TASK_DERIVE_EVENTS_WITH_HOOKS_FOR_EXECUTION                  = "task.derive-events-with-hooks-for-execution"
	_LABEL_TASK_DERIVE_APP_STOP_STRATEGY_FOR_EXECUTION                  = "task.derive-app-stop-strategy-for-execution"
	_LABEL_TASK_APPLY_APP_STOP_STRATEGY_FOR_EXECUTION                   = "task.apply-app-stop-strategy-for-execution"
	_LABEL_TASK_STOP_APP_RUNTIME_FOR_SINGLE_EVENT_HARD_RELOAD           = "task.stop-app-runtime-for-single-event-hard-reload"
	_LABEL_TASK_STOP_APP_RUNTIME_FOR_BATCH_HARD_RELOAD                  = "task.stop-app-runtime-for-batch-hard-reload"
	_LABEL_TASK_MARK_HOOK_CONTEXTS_APP_STOPPED_FOR_BATCH                = "task.mark-hook-contexts-app-stopped-for-batch"
	_LABEL_TASK_FIRE_CONCURRENT_NO_WAIT_HOOKS                           = "task.fire-concurrent-no-wait-hooks"
	_LABEL_TASK_DERIVE_HOOK_STAGE_EXECUTION_DESCRIPTORS                 = "task.derive-hook-stage-execution-descriptors"
	_LABEL_TASK_DERIVE_CONCURRENT_NO_WAIT_HOOK_EXECUTION_PLANS          = "task.derive-concurrent-no-wait-hook-execution-plans"
	_LABEL_TASK_GET_OR_CREATE_CONCURRENT_NO_WAIT_HOOK_LIFECYCLE_CONTEXT = "task.get-or-create-concurrent-no-wait-hook-lifecycle-context"
	_LABEL_TASK_LAUNCH_CONCURRENT_NO_WAIT_HOOK_CALLBACKS                = "task.launch-concurrent-no-wait-hook-callbacks"
	_LABEL_TASK_LAUNCH_CONCURRENT_NO_WAIT_HOOK_COMMANDS                 = "task.launch-concurrent-no-wait-hook-commands"

	_LABEL_TASK_RUN_PRE_HOOKS_FOR_EVENTS_WITH_ERRORS            = "task.run-pre-hooks-for-events-with-errors"
	_LABEL_TASK_ADD_IMPLICIT_WORK_FOR_NON_RUN_ON_CHANGE_EVENTS  = "task.add-implicit-work-for-non-run-on-change-events"
	_LABEL_TASK_RUN_PRE_HOOKS_FOR_EACH_EVENT                    = "task.run-pre-hooks-for-each-event"
	_LABEL_TASK_DERIVE_PRE_HOOK_EXECUTION_PLANS                 = "task.derive-pre-hook-execution-plans"
	_LABEL_TASK_EXECUTE_PRE_HOOK_EXECUTION_PLANS                = "task.execute-pre-hook-execution-plans"
	_LABEL_TASK_EXECUTE_HOOK_EXECUTION_PLAN_WITH_CONTEXT        = "task.execute-hook-execution-plan-with-context"
	_LABEL_TASK_DERIVE_HOOK_CALLBACK_TIMEOUT_FOR_EXECUTION_PLAN = "task.derive-hook-callback-timeout-for-execution-plan"
	_LABEL_TASK_DERIVE_HOOK_COMMAND_TIMEOUT_FOR_EXECUTION_PLAN  = "task.derive-hook-command-timeout-for-execution-plan"
	_LABEL_TASK_CLONE_HOOK_CONTEXT_FOR_EXECUTION                = "task.clone-hook-context-for-execution"
	_LABEL_TASK_RUN_NO_WAIT_HOOK_WITH_CONCURRENCY_LIMIT         = "task.run-no-wait-hook-with-concurrency-limit"
	_LABEL_TASK_EXECUTE_HOOK_CALLBACK_SAFELY                    = "task.execute-hook-callback-safely"
	_LABEL_TASK_EXECUTE_HOOK_COMMAND_WITH_CONTEXT               = "task.execute-hook-command-with-context"
	_LABEL_TASK_TOLERATE_NO_WAIT_HOOK_CALLBACK_FAILURES         = "task.tolerate-no-wait-hook-callback-failures"
	_LABEL_TASK_TOLERATE_NO_WAIT_HOOK_COMMAND_FAILURES          = "task.tolerate-no-wait-hook-command-failures"

	_LABEL_TASK_APPLY_PRE_HOOK_REFRESH_ACTIONS_TO_WORK_SET = "task.apply-pre-hook-refresh-actions-to-work-set"
	_LABEL_TASK_REDUCE_REFRESH_ACTIONS_IN_STABLE_ORDER     = "task.reduce-refresh-actions-in-stable-order"
	_LABEL_TASK_APPLY_REFRESH_ACTION_WORK_MUTATIONS        = "task.apply-refresh-action-work-mutations"
	_LABEL_TASK_CONTINUE_PIPELINE_AFTER_PRE_HOOK_STAGE     = "task.continue-pipeline-after-pre-hook-stage"
	_LABEL_TASK_DERIVE_HOOK_STAGE_FAILURE_POLICY           = "task.derive-hook-stage-failure-policy"
	_LABEL_TASK_DERIVE_HOOK_STAGE_CONTINUATION_DECISION    = "task.derive-hook-stage-continuation-decision"
	_LABEL_TASK_TRIGGER_RESTART_FROM_REFRESH_ACTIONS       = "task.trigger-restart-from-refresh-actions"

	_LABEL_TASK_EXECUTE_MATERIALIZATION_BUILD_LANE = "task.execute-materialization-build-lane"
	_LABEL_TASK_MATERIALIZE_CSS_STATE              = "task.materialize-css-state"
	_LABEL_TASK_MATERIALIZE_STATIC_ASSET_STATE     = "task.materialize-static-asset-state"
	_LABEL_TASK_MATERIALIZE_WAVE_METADATA_STATE    = "task.materialize-wave-metadata-state"
	_LABEL_TASK_ENSURE_DIST_OUTPUT_DIRECTORIES     = "task.ensure-dist-output-directories"
	_LABEL_TASK_EMIT_RUNTIME_CONFIG_ARTIFACT       = "task.emit-runtime-config-artifact"
	_LABEL_TASK_WRITE_SCHEMA_ARTIFACT              = "task.write-schema-artifact"
	_LABEL_TASK_ENSURE_DIST_KEEP_FILE              = "task.ensure-dist-keep-file"

	_LABEL_TASK_MATERIALIZE_GO_BINARY_STATE                                         = "task.materialize-go-binary-state"
	_LABEL_TASK_PREPARE_GO_BUILD_OVERLAY                                            = "task.prepare-go-build-overlay"
	_LABEL_TASK_ENSURE_GO_BINARY_OUTPUT_DIRECTORY                                   = "task.ensure-go-binary-output-directory"
	_LABEL_TASK_COMPILE_GO_BINARY_FOR_MODE                                          = "task.compile-go-binary-for-mode"
	_LABEL_TASK_CLEANUP_GO_BUILD_OVERLAY                                            = "task.cleanup-go-build-overlay"
	_LABEL_TASK_MATERIALIZE_CRITICAL_CSS_STATE                                      = "task.materialize-critical-css-state"
	_LABEL_TASK_DERIVE_CRITICAL_CSS_BUILD_INPUTS                                    = "task.derive-critical-css-build-inputs"
	_LABEL_TASK_BUILD_CRITICAL_CSS_PIPELINE                                         = "task.build-critical-css-pipeline"
	_LABEL_TASK_WRITE_CRITICAL_CSS_ARTIFACT                                         = "task.write-critical-css-artifact"
	_LABEL_TASK_MATERIALIZE_NORMAL_CSS_STATE                                        = "task.materialize-normal-css-state"
	_LABEL_TASK_DERIVE_NORMAL_CSS_BUILD_INPUTS                                      = "task.derive-normal-css-build-inputs"
	_LABEL_TASK_BUILD_NORMAL_CSS_PIPELINE                                           = "task.build-normal-css-pipeline"
	_LABEL_TASK_WRITE_NORMAL_CSS_ARTIFACT                                           = "task.write-normal-css-artifact"
	_LABEL_TASK_WRITE_NORMAL_CSS_REF_ARTIFACT                                       = "task.write-normal-css-ref-artifact"
	_LABEL_TASK_MATERIALIZE_PUBLIC_ASSET_STATE                                      = "task.materialize-public-asset-state"
	_LABEL_TASK_SNAPSHOT_PUBLIC_FILE_MAP_ARTIFACTS_BEFORE_PROCESSING                = "task.snapshot-public-file-map-artifacts-before-processing"
	_LABEL_TASK_DERIVE_PUBLIC_STATIC_PROCESSING_MODE                                = "task.derive-public-static-processing-mode"
	_LABEL_TASK_PROCESS_PUBLIC_STATIC_ASSETS                                        = "task.process-public-static-assets"
	_LABEL_TASK_CLEANUP_STALE_PUBLIC_STATIC_OUTPUTS                                 = "task.cleanup-stale-public-static-outputs"
	_LABEL_TASK_SAVE_PUBLIC_FILEMAP_GOB                                             = "task.save-public-filemap-gob"
	_LABEL_TASK_WRITE_CANONICAL_PUBLIC_FILE_MAP_JSON_AND_REF                        = "task.write-canonical-public-file-map-json-and-ref"
	_LABEL_TASK_DERIVE_PUBLIC_FILE_MAP_ARTIFACT_CHANGE_OR_REPAIR_DECISION           = "task.derive-public-file-map-artifact-change-or-repair-decision"
	_LABEL_TASK_APPLY_PUBLIC_FILE_MAP_ARTIFACT_CHANGE_SIDE_EFFECTS                  = "task.apply-public-file-map-artifact-change-side-effects"
	_LABEL_TASK_CALL_FRAMEWORK_RUNTIME_RELOAD_ENDPOINT_FOR_PUBLIC_FILE_MAP_CHANGE   = "task.call-framework-runtime-reload-endpoint-for-public-file-map-change"
	_LABEL_TASK_RUN_FRAMEWORK_BUILD_HOOK_FOR_PUBLIC_FILE_MAP_CHANGE                 = "task.run-framework-build-hook-for-public-file-map-change"
	_LABEL_TASK_CLEAR_INVALIDATE_VITE_BROWSER_ACTION_WHEN_PUBLIC_FILE_MAP_UNCHANGED = "task.clear-invalidate-vite-browser-action-when-public-file-map-unchanged"
	_LABEL_TASK_MATERIALIZE_PRIVATE_ASSET_STATE                                     = "task.materialize-private-asset-state"
	_LABEL_TASK_DERIVE_PRIVATE_STATIC_PROCESSING_MODE                               = "task.derive-private-static-processing-mode"
	_LABEL_TASK_PROCESS_PRIVATE_STATIC_ASSETS                                       = "task.process-private-static-assets"
	_LABEL_TASK_SAVE_PRIVATE_FILEMAP_GOB                                            = "task.save-private-filemap-gob"
	_LABEL_TASK_MATERIALIZE_FRONTEND_BUNDLE_STATE                                   = "task.materialize-frontend-bundle-state"
	_LABEL_TASK_RUN_VITE_PROD_BUILD                                                 = "task.run-vite-prod-build"
	_LABEL_TASK_EMIT_VITE_MANIFEST_ARTIFACT                                         = "task.emit-vite-manifest-artifact"
	_LABEL_TASK_EXECUTE_CONCURRENT_HOOKS                                            = "task.execute-concurrent-hooks"
	_LABEL_TASK_RUN_CONCURRENT_HOOKS_FOR_EVENTS_WITH_CONTEXT_AND_ERRORS             = "task.run-concurrent-hooks-for-events-with-context-and-errors"
	_LABEL_TASK_RUN_CONCURRENT_HOOKS_FOR_EACH_EVENT                                 = "task.run-concurrent-hooks-for-each-event"
	_LABEL_TASK_DERIVE_CONCURRENT_HOOK_EXECUTION_PLANS                              = "task.derive-concurrent-hook-execution-plans"
	_LABEL_TASK_EXECUTE_CONCURRENT_HOOK_EXECUTION_PLANS                             = "task.execute-concurrent-hook-execution-plans"
	_LABEL_TASK_PRESERVE_CONCURRENT_HOOK_ACTIONS_IN_EVENT_ORDER                     = "task.preserve-concurrent-hook-actions-in-event-order"
	_LABEL_TASK_JOIN_CONCURRENT_HOOK_EXECUTION_ERRORS_IN_ORDER                      = "task.join-concurrent-hook-execution-errors-in-order"
	_LABEL_TASK_APPLY_CONCURRENT_HOOK_REFRESH_ACTIONS_TO_WORK_SET                   = "task.apply-concurrent-hook-refresh-actions-to-work-set"
	_LABEL_TASK_CONTINUE_PIPELINE_AFTER_CONCURRENT_HOOK_STAGE                       = "task.continue-pipeline-after-concurrent-hook-stage"

	_LABEL_TASK_APPLY_POST_HOOK_ADJUSTMENTS                 = "task.apply-post-hook-adjustments"
	_LABEL_TASK_RUN_POST_HOOKS_FOR_EVENTS_WITH_ERRORS       = "task.run-post-hooks-for-events-with-errors"
	_LABEL_TASK_RUN_POST_HOOKS_FOR_EACH_EVENT               = "task.run-post-hooks-for-each-event"
	_LABEL_TASK_DERIVE_POST_HOOK_EXECUTION_PLANS            = "task.derive-post-hook-execution-plans"
	_LABEL_TASK_EXECUTE_POST_HOOK_EXECUTION_PLANS           = "task.execute-post-hook-execution-plans"
	_LABEL_TASK_APPLY_POST_HOOK_REFRESH_ACTIONS_TO_WORK_SET = "task.apply-post-hook-refresh-actions-to-work-set"
	_LABEL_TASK_CONTINUE_PIPELINE_AFTER_POST_HOOK_STAGE     = "task.continue-pipeline-after-post-hook-stage"

	_LABEL_TASK_APPLY_BACKEND_MUTATION_PLAN                  = "task.apply-backend-mutation-plan"
	_LABEL_TASK_EXECUTE_SELECTED_BACKEND_MUTATION_BRANCH     = "task.execute-selected-backend-mutation-branch"
	_LABEL_TASK_DERIVE_BACKEND_MUTATION_PLAN                 = "task.derive-backend-mutation-plan"
	_LABEL_TASK_DERIVE_RUN_BUILD_EXECUTION_ORDERING_DECISION = "task.derive-run-build-execution-ordering-decision"
	_LABEL_TASK_RESOLVE_QUEUED_RESTART_REQUEST               = "task.resolve-queued-restart-request"
	_LABEL_TASK_MERGE_RESTART_REQUESTS                       = "task.merge-restart-requests"
	_LABEL_TASK_CONSUME_PENDING_RESTART_REQUEST              = "task.consume-pending-restart-request"
	_LABEL_TASK_QUEUE_RETRY_WAIT_RESTART                     = "task.queue-retry-wait-restart"
	_LABEL_TASK_NORMALIZE_RESTART_REQUEST                    = "task.normalize-restart-request"
	_LABEL_TASK_QUEUE_RESTART_REQUEST                        = "task.queue-restart-request"
	_LABEL_TASK_SET_WAITING_FOR_BUILD_RETRY                  = "task.set-waiting-for-build-retry"
	_LABEL_TASK_RESTART_DEV_SERVER_CYCLE                     = "task.restart-dev-server-cycle"
	_LABEL_TASK_CANCEL_AND_JOIN_CURRENT_RUN_CYCLE_SCOPE      = "task.cancel-and-join-current-run-cycle-scope"
	_LABEL_TASK_QUEUE_CONFIG_RESTART_REQUEST                 = "task.queue-config-restart-request"
	_LABEL_TASK_APPLY_NORMAL_BACKEND_MUTATIONS               = "task.apply-normal-backend-mutations"
	_LABEL_TASK_RESTART_APP_RUNTIME_PROCESS                  = "task.restart-app-runtime-process"
	_LABEL_TASK_STOP_APP_RUNTIME_PROCESS                     = "task.stop-app-runtime-process"
	_LABEL_TASK_START_APP_RUNTIME_PROCESS                    = "task.start-app-runtime-process"
	_LABEL_TASK_RESTART_VITE_RUNTIME_PROCESS                 = "task.restart-vite-runtime-process"
	_LABEL_TASK_EXECUTE_FRAMEWORK_BACKEND_MUTATION_EFFECTS   = "task.execute-framework-backend-mutation-effects"

	_LABEL_TASK_CONVERGE_BACKEND_STATE                                 = "task.converge-backend-state"
	_LABEL_TASK_DERIVE_BACKEND_CONVERGENCE_REQUIREMENTS                = "task.derive-backend-convergence-requirements"
	_LABEL_TASK_AWAIT_APP_READINESS_IF_REQUIRED                        = "task.await-app-readiness-if-required"
	_LABEL_TASK_AWAIT_VITE_READINESS_IF_REQUIRED                       = "task.await-vite-readiness-if-required"
	_LABEL_TASK_EXECUTE_FRAMEWORK_RUNTIME_RELOAD_REQUESTS              = "task.execute-framework-runtime-reload-requests"
	_LABEL_TASK_EXECUTE_FRAMEWORK_BACKEND_NOTIFICATIONS                = "task.execute-framework-backend-notifications"
	_LABEL_TASK_APPLY_FRAMEWORK_NOTIFICATION_FAILURE_POLICY            = "task.apply-framework-notification-failure-policy"
	_LABEL_TASK_DERIVE_FRAMEWORK_NOTIFICATION_FAILURE_POLICY_DECISIONS = "task.derive-framework-notification-failure-policy-decisions"
	_LABEL_TASK_REQUEST_BACKEND_HEALING_RESTART_WITHOUT_GO_COMPILE     = "task.request-backend-healing-restart-without-go-compile"

	_LABEL_TASK_APPLY_FRONTEND_TERMINAL_ACTION                          = "task.apply-frontend-terminal-action"
	_LABEL_TASK_EXECUTE_SELECTED_FRONTEND_TERMINAL_ACTION               = "task.execute-selected-frontend-terminal-action"
	_LABEL_TASK_DERIVE_FRONTEND_TERMINAL_ACTION                         = "task.derive-frontend-terminal-action"
	_LABEL_TASK_BROADCAST_CSS_HOT_RELOAD                                = "task.broadcast-css-hot-reload"
	_LABEL_TASK_READ_CRITICAL_CSS_HOT_RELOAD_PAYLOAD                    = "task.read-critical-css-hot-reload-payload"
	_LABEL_TASK_READ_NORMAL_CSS_HOT_RELOAD_URL                          = "task.read-normal-css-hot-reload-url"
	_LABEL_TASK_BUILD_CSS_HOT_RELOAD_PAYLOADS                           = "task.build-css-hot-reload-payloads"
	_LABEL_TASK_BROADCAST_REVALIDATE                                    = "task.broadcast-revalidate"
	_LABEL_TASK_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED                      = "task.notify-vite-public-filemap-changed"
	_LABEL_TASK_SHOULD_ATTEMPT_VITE_INVALIDATE_FOR_BROWSER_DECISION     = "task.should-attempt-vite-invalidate-for-browser-decision"
	_LABEL_TASK_NOTIFY_VITE_FILE_MAP_CHANGED_ENDPOINT                   = "task.notify-vite-file-map-changed-endpoint"
	_LABEL_TASK_DERIVE_NOTIFY_VITE_FALLBACK_BROWSER_DECISION            = "task.derive-notify-vite-fallback-browser-decision"
	_LABEL_TASK_RESOLVE_BROWSER_DECISION_AFTER_INVALIDATE_VITE_FALLBACK = "task.resolve-browser-decision-after-invalidate-vite-fallback"
	_LABEL_TASK_BROADCAST_HARD_RELOAD                                   = "task.broadcast-hard-reload"
	_LABEL_TASK_PUBLISH_NO_RELOAD_NEEDED_NOTICE                         = "task.publish-no-reload-needed-notice"

	_LABEL_TASK_PLAN_HEALING_LOOPBACK_TRANSITION = "task.plan-healing-loopback-transition"

	_LABEL_TASK_COMPLETE_BATCH                                  = "task.complete-batch"
	_LABEL_TASK_EMIT_BATCH_COMPLETION_FACTS                     = "task.emit-batch-completion-facts"
	_LABEL_TASK_CLEAR_CONCURRENT_NO_WAIT_HOOK_LIFECYCLE_CONTEXT = "task.clear-concurrent-no-wait-hook-lifecycle-context"
	_LABEL_TASK_REMOVE_STALE_WATCHER_PATHS                      = "task.remove-stale-watcher-paths"
	_LABEL_TASK_CLEAR_BATCH_SCOPED_TEMPORARY_STATE              = "task.clear-batch-scoped-temporary-state"
)

/////////////////////////////////////////////////////////////////////
/////// Task Tracking
/////////////////////////////////////////////////////////////////////

type task_trace_label string

type task_gate_point string

const (
	task_gate_point_after_start task_gate_point = "after_start"
	task_gate_point_before_done task_gate_point = "before_done"
)

type test_task_gate struct {
	reached_channel chan<- struct{}
	release_channel <-chan struct{}
}

type test_task_gate_key struct {
	label task_trace_label
	point task_gate_point
}

type test_task_gate_registry struct {
	mutex sync.Mutex
	gates map[test_task_gate_key]test_task_gate
}

func new_test_task_gate_registry() *test_task_gate_registry {
	return &test_task_gate_registry{
		gates: map[test_task_gate_key]test_task_gate{},
	}
}

func (task_gate_registry *test_task_gate_registry) set_task_gate(
	label task_trace_label,
	point task_gate_point,
	gate test_task_gate,
) {
	task_gate_registry.mutex.Lock()
	defer task_gate_registry.mutex.Unlock()
	task_gate_registry.gates[test_task_gate_key{
		label: label,
		point: point,
	}] = gate
}

func (task_gate_registry *test_task_gate_registry) get_task_gate(
	label task_trace_label,
	point task_gate_point,
) (test_task_gate, bool) {
	task_gate_registry.mutex.Lock()
	defer task_gate_registry.mutex.Unlock()
	task_gate, has_task_gate := task_gate_registry.gates[test_task_gate_key{
		label: label,
		point: point,
	}]
	return task_gate, has_task_gate
}

func (label task_trace_label) start(
	tasks_ctx *tasks.Ctx,
) {
	record_test_effect(tasks_ctx, label.start_effect_label())
}

func (label task_trace_label) done(
	tasks_ctx *tasks.Ctx,
) {
	record_test_effect(tasks_ctx, label.done_effect_label())
}

func (label task_trace_label) start_effect_label() string {
	return string(label) + ":start"
}

func (label task_trace_label) done_effect_label() string {
	return string(label) + ":done"
}

func tracked_task[I comparable, O any](
	label task_trace_label,
	fn func(tasks_ctx *tasks.Ctx, input I) (O, error),
) *tasks.Task[I, O] {
	return tasks.NewTask(
		func(
			tasks_ctx *tasks.Ctx,
			input I,
		) (output O, err error) {
			if !is_test_env() {
				return fn(tasks_ctx, input)
			}
			label.start(tasks_ctx)
			wait_on_task_gate_if_configured(
				tasks_ctx,
				label,
				task_gate_point_after_start,
			)
			defer func() {
				wait_on_task_gate_if_configured(
					tasks_ctx,
					label,
					task_gate_point_before_done,
				)
				label.done(tasks_ctx)
			}()
			return fn(tasks_ctx, input)
		},
	)
}

type test_task_gate_registry_context_key struct{}

func with_test_task_gate_registry(
	native_context context.Context,
	task_gate_registry *test_task_gate_registry,
) context.Context {
	return context.WithValue(
		native_context,
		test_task_gate_registry_context_key{},
		task_gate_registry,
	)
}

func wait_on_task_gate_if_configured(
	tasks_ctx *tasks.Ctx,
	label task_trace_label,
	point task_gate_point,
) {
	task_gate_registry_value := tasks_ctx.NativeContext().Value(
		test_task_gate_registry_context_key{},
	)
	if task_gate_registry_value == nil {
		return
	}
	task_gate_registry := task_gate_registry_value.(*test_task_gate_registry)
	task_gate, has_task_gate := task_gate_registry.get_task_gate(
		label,
		point,
	)
	if !has_task_gate {
		return
	}
	if task_gate.reached_channel != nil {
		select {
		case task_gate.reached_channel <- struct{}{}:
		default:
		}
	}
	if task_gate.release_channel != nil {
		<-task_gate.release_channel
	}
}

/////////////////////////////////////////////////////////////////////
/////// Test Recorder
/////////////////////////////////////////////////////////////////////

type test_effect_recorder struct {
	mutex         sync.Mutex
	effect_labels []string
}

type test_effect_recorder_context_key struct{}

func with_test_effect_recorder(
	native_context context.Context,
	effect_recorder *test_effect_recorder,
) context.Context {
	return context.WithValue(
		native_context,
		test_effect_recorder_context_key{},
		effect_recorder,
	)
}

func (recorder *test_effect_recorder) snapshot_effect_labels() []string {
	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()
	return append([]string(nil), recorder.effect_labels...)
}

func is_test_env() bool {
	return os.Getenv(wave_internal_test_mode_env_var) == "1"
}

func record_test_effect(
	tasks_ctx *tasks.Ctx,
	effect_label string,
) bool {
	if !is_test_env() {
		return false
	}
	effect_recorder := tasks_ctx.NativeContext().Value(
		test_effect_recorder_context_key{},
	).(*test_effect_recorder)
	effect_recorder.mutex.Lock()
	effect_recorder.effect_labels = append(
		effect_recorder.effect_labels,
		effect_label,
	)
	effect_recorder.mutex.Unlock()
	return true
}
