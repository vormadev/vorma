package wavebuild

import (
	"context"
	"reflect"
	"testing"
)

type pipelineEffectMatrixTestCase struct {
	name                                        string
	mode                                        mode
	waitingForBuildRetry                        bool
	events                                      []observedBatchEvent
	wavePublicFileMapNotificationDestinationKey fwNotificationDestinationKey
	appRequestedOutcomes                        appRequestedOutcomes
	expectedEffectLabels                        []string
}

func TestRunFivePhasePipeline_ExplicitEffectMatrix(t *testing.T) {
	t.Setenv(waveInternalTestModeEnvVar, "1")

	testCases := []pipelineEffectMatrixTestCase{
		{
			name: "dev_noise_only_batch_produces_no_reload_notice",
			mode: modeDev,
			events: []observedBatchEvent{
				{eventType: eventTypeIgnoredOrNoiseChanged, noiseOnly: true},
			},
			expectedEffectLabels: []string{
				_LABEL_P5_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
		},
		{
			name: "dev_noop_config_mutation_produces_no_reload_notice",
			mode: modeDev,
			events: []observedBatchEvent{
				{
					eventType:          eventTypeConfigFileChanged,
					noOpConfigMutation: true,
				},
			},
			expectedEffectLabels: []string{
				_LABEL_P5_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
		},
		{
			name: "dev_critical_css_change_builds_critical_css_and_css_hot_reload",
			mode: modeDev,
			events: []observedBatchEvent{
				{eventType: eventTypeCriticalCSSSourceChanged},
			},
			expectedEffectLabels: []string{
				_LABEL_P2_BUILD_CRITICAL_CSS,
				_LABEL_P5_BROADCAST_CSS_HOT_RELOAD,
			},
		},
		{
			name: "dev_normal_css_change_builds_normal_css_and_css_hot_reload",
			mode: modeDev,
			events: []observedBatchEvent{
				{eventType: eventTypeNormalCSSSourceChanged},
			},
			expectedEffectLabels: []string{
				_LABEL_P2_BUILD_NORMAL_CSS,
				_LABEL_P5_BROADCAST_CSS_HOT_RELOAD,
			},
		},
		{
			name: "dev_go_source_change_builds_binary_restarts_app_and_hard_reloads",
			mode: modeDev,
			events: []observedBatchEvent{
				{eventType: eventTypeGoSourceChanged},
			},
			expectedEffectLabels: []string{
				_LABEL_P2_BUILD_GO_BINARY,
				_LABEL_P3_RESTART_APP_PROCESS,
				_LABEL_P4_AWAIT_BACKEND_READINESS,
				_LABEL_P5_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "dev_public_static_change_processes_assets_emits_fw_notification_and_notifies_vite",
			mode: modeDev,
			events: []observedBatchEvent{
				{eventType: eventTypePublicStaticAssetChanged},
			},
			wavePublicFileMapNotificationDestinationKey: fwNotificationDestinationKey(
				"fw.public_filemap_reload",
			),
			expectedEffectLabels: []string{
				_LABEL_P2_PROCESS_PUBLIC_STATIC_ASSETS,
				_LABEL_P2_CLEANUP_STALE_PUBLIC_STATIC,
				_LABEL_P4_AWAIT_BACKEND_READINESS,
				_LABEL_P4_EXECUTE_FW_NOTIFICATION + "[fw.public_filemap_reload]",
				_LABEL_P5_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
			},
		},
		{
			name: "dev_private_static_change_processes_private_assets_and_hard_reloads",
			mode: modeDev,
			events: []observedBatchEvent{
				{eventType: eventTypePrivateStaticAssetChanged},
			},
			expectedEffectLabels: []string{
				_LABEL_P2_PROCESS_PRIVATE_STATIC_ASSETS,
				_LABEL_P5_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "dev_fw_requested_backend_mutation_effect_executes_and_hard_reloads",
			mode: modeDev,
			events: []observedBatchEvent{
				{
					eventType: eventTypeFWRequestedEffectsChanged,
					fwRequestedEffects: fwRequestedEffects{
						backendMutationEffectKeys: []fwMutationEffectKey{
							"fw.reload_routes",
						},
					},
				},
			},
			appRequestedOutcomes: appRequestedOutcomes{
				requestedTerminalBrowserAction: frontendTerminalBrowserActionHardReload,
			},
			expectedEffectLabels: []string{
				_LABEL_P3_EXECUTE_FW_MUTATION_EFFECT + "[fw.reload_routes]",
				_LABEL_P4_AWAIT_BACKEND_READINESS,
				_LABEL_P5_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "dev_fw_requested_notification_executes_and_allows_no_reload_terminal_action",
			mode: modeDev,
			events: []observedBatchEvent{
				{
					eventType: eventTypeFWRequestedEffectsChanged,
					fwRequestedEffects: fwRequestedEffects{
						backendConvergenceNotificationQueue: []fwNotificationRequest{
							{
								destinationKey: "fw.runtime_notify",
								trigger:        "fw_watch_change",
							},
						},
					},
				},
			},
			expectedEffectLabels: []string{
				_LABEL_P4_AWAIT_BACKEND_READINESS,
				_LABEL_P4_EXECUTE_FW_NOTIFICATION + "[fw.runtime_notify]",
				_LABEL_P5_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
		},
		{
			name: "dev_config_change_restarts_dev_server_cycle_and_vite_then_hard_reloads",
			mode: modeDev,
			events: []observedBatchEvent{
				{eventType: eventTypeConfigFileChanged},
			},
			expectedEffectLabels: []string{
				_LABEL_P3_APPLY_DEV_SERVER_RESTART,
				_LABEL_P3_RESTART_VITE_PROCESS,
				_LABEL_P4_AWAIT_BACKEND_READINESS,
				_LABEL_P5_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name: "dev_app_action_only_revalidate_requests_revalidate_without_build_or_backend_settling",
			mode: modeDev,
			events: []observedBatchEvent{
				{eventType: eventTypeAppDefinedWatchActionOnlyChanged},
			},
			appRequestedOutcomes: appRequestedOutcomes{
				requestedTerminalBrowserAction: frontendTerminalBrowserActionRevalidate,
			},
			expectedEffectLabels: []string{
				_LABEL_P5_BROADCAST_REVALIDATE,
			},
		},
		{
			name: "dev_app_action_only_fw_effect_request_executes_and_hard_reloads",
			mode: modeDev,
			events: []observedBatchEvent{
				{eventType: eventTypeAppDefinedWatchActionOnlyChanged},
			},
			appRequestedOutcomes: appRequestedOutcomes{
				requestedTerminalBrowserAction: frontendTerminalBrowserActionHardReload,
				fwRequestedEffects: fwRequestedEffects{
					backendMutationEffectKeys: []fwMutationEffectKey{
						"fw.refresh_runtime_cache",
					},
				},
			},
			expectedEffectLabels: []string{
				_LABEL_P3_EXECUTE_FW_MUTATION_EFFECT + "[fw.refresh_runtime_cache]",
				_LABEL_P4_AWAIT_BACKEND_READINESS,
				_LABEL_P5_BROADCAST_HARD_RELOAD,
			},
		},
		{
			name:                 "dev_waiting_for_build_retry_queues_retry_wait_restart_and_skips_reload",
			mode:                 modeDev,
			waitingForBuildRetry: true,
			events: []observedBatchEvent{
				{eventType: eventTypeGoSourceChanged},
			},
			expectedEffectLabels: []string{
				_LABEL_P3_QUEUE_RETRY_WAIT_RESTART,
				_LABEL_P5_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
			},
		},
		{
			name: "dev_mixed_go_css_and_public_static_runs_deduped_build_and_hard_reload",
			mode: modeDev,
			events: []observedBatchEvent{
				{eventType: eventTypeGoSourceChanged},
				{eventType: eventTypeNormalCSSSourceChanged},
				{eventType: eventTypePublicStaticAssetChanged},
			},
			expectedEffectLabels: []string{
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
			name: "dev_app_requested_outcomes_are_ignored_without_app_defined_event",
			mode: modeDev,
			events: []observedBatchEvent{
				{eventType: eventTypeNormalCSSSourceChanged},
			},
			appRequestedOutcomes: appRequestedOutcomes{
				requestedTerminalBrowserAction: frontendTerminalBrowserActionHardReload,
				requestRestart:                 true,
				requestGoCompile:               true,
				fwRequestedEffects: fwRequestedEffects{
					backendMutationEffectKeys: []fwMutationEffectKey{
						"fw.unused_without_app_defined_event",
					},
				},
			},
			expectedEffectLabels: []string{
				_LABEL_P2_BUILD_NORMAL_CSS,
				_LABEL_P5_BROADCAST_CSS_HOT_RELOAD,
			},
		},
		{
			name: "prod_runs_build_only_with_canonical_build_effects",
			mode: modeProd,
			events: []observedBatchEvent{
				{eventType: eventTypeIgnoredOrNoiseChanged, noiseOnly: true},
			},
			expectedEffectLabels: []string{
				_LABEL_P2_BUILD_GO_BINARY,
				_LABEL_P2_BUILD_CRITICAL_CSS,
				_LABEL_P2_BUILD_NORMAL_CSS,
				_LABEL_P2_PROCESS_PUBLIC_STATIC_ASSETS,
				_LABEL_P2_CLEANUP_STALE_PUBLIC_STATIC,
				_LABEL_P2_PROCESS_PRIVATE_STATIC_ASSETS,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			recordedEffectLabels := runPipelineAndRecordEffectLabels(
				t,
				testCase,
			)
			assertUnorderedEffectLabels(
				t,
				recordedEffectLabels,
				testCase.expectedEffectLabels,
			)
		})
	}
}

func runPipelineAndRecordEffectLabels(
	t *testing.T,
	testCase pipelineEffectMatrixTestCase,
) []string {
	t.Helper()

	effectRecorder := &testEffectRecorder{}
	nativeContextWithRecorder := withTestEffectRecorder(
		context.Background(),
		effectRecorder,
	)
	_, runError := runFivePhasePipeline(
		nativeContextWithRecorder,
		p1_BatchInput{
			p1: &p1_Input{
				mode:         testCase.mode,
				generationID: "test_generation",
				events:       testCase.events,
				wavePublicFileMapNotificationDestinationKey: testCase.
					wavePublicFileMapNotificationDestinationKey,
				appRequestedOutcomes: testCase.appRequestedOutcomes,
				waitingForBuildRetry: testCase.waitingForBuildRetry,
			},
		},
	)
	if runError != nil {
		t.Fatalf("runFivePhasePipeline returned error: %v", runError)
	}
	return effectRecorder.snapshotEffectLabels()
}

func assertUnorderedEffectLabels(
	t *testing.T,
	recordedEffectLabels []string,
	expectedEffectLabels []string,
) {
	t.Helper()
	if !reflect.DeepEqual(
		countEffectLabels(recordedEffectLabels),
		countEffectLabels(expectedEffectLabels),
	) {
		t.Fatalf(
			"effect labels mismatch\nrecorded=%v\nexpected=%v",
			recordedEffectLabels,
			expectedEffectLabels,
		)
	}
}

func countEffectLabels(labels []string) map[string]int {
	labelCounts := make(map[string]int, len(labels))
	for _, label := range labels {
		labelCounts[label]++
	}
	return labelCounts
}
