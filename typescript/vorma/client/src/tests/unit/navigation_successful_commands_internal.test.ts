import { describe, expect, it } from "vitest";
import {
	buildSuccessfulNavigationCleanupCommands,
	buildSuccessfulNavigationLifecycleCheckpointCommands,
	buildSuccessfulNavigationPostAssetLifecycleCommands,
	buildSuccessfulNavigationPostAssetCommands,
	buildSuccessfulNavigationPostWaitingCommands,
	buildSuccessfulNavigationPreAssetWaitCommands,
	buildSuccessfulNavigationPreWaitingCommands,
} from "../../core/navigation/runtime_navigation_successful_commands.ts";

describe("successful navigation command builder", () => {
	it("builds pre-waiting commands for stop and delete-and-stop plans", () => {
		expect(
			buildSuccessfulNavigationPreWaitingCommands({
				preWaitingExecutionPlan: {
					type: "stop",
					reason: "non_current_entry",
				},
			}),
		).toEqual([
			{
				type: "stop",
				reason: "non_current_entry",
			},
		]);

		expect(
			buildSuccessfulNavigationPreWaitingCommands({
				preWaitingExecutionPlan: {
					type: "deleteAndStop",
					targetUrl: "http://localhost:3000/test",
					reason: "stale_revalidation_pre_waiting",
				},
			}),
		).toEqual([
			{
				type: "delete_navigation",
				targetUrl: "http://localhost:3000/test",
				reason: "stale_revalidation_pre_waiting",
			},
			{
				type: "stop",
				reason: "stale_revalidation_pre_waiting",
			},
		]);
	});

	it("builds pre-waiting transition command for continue plans", () => {
		expect(
			buildSuccessfulNavigationPreWaitingCommands({
				preWaitingExecutionPlan: {
					type: "continue",
					reason: "entry_current_and_fresh",
				},
			}),
		).toEqual([
			{
				type: "transition_phase",
				phase: "waiting",
				reason: "process_successful_navigation_waiting",
			},
		]);
	});

	it("builds post-waiting stop command only when post-wait plan stops", () => {
		expect(
			buildSuccessfulNavigationPostWaitingCommands({
				postWaitingExecutionPlan: {
					type: "stop",
					reason: "post_waiting_entry_lost",
				},
			}),
		).toEqual([
			{
				type: "stop",
				reason: "post_waiting_entry_lost",
			},
		]);

		expect(
			buildSuccessfulNavigationPostWaitingCommands({
				postWaitingExecutionPlan: {
					type: "continue",
					reason: "post_waiting_entry_current",
				},
			}),
		).toEqual([]);
	});

	it("builds post-asset commands for stop, complete-without-render, and render plans", () => {
		expect(
			buildSuccessfulNavigationPostAssetCommands({
				postAssetExecutionPlan: {
					type: "stop",
					reason: "post_asset_entry_lost",
				},
			}),
		).toEqual([
			{
				type: "stop",
				reason: "post_asset_entry_lost",
			},
		]);

		expect(
			buildSuccessfulNavigationPostAssetCommands({
				postAssetExecutionPlan: {
					type: "completeWithoutRender",
					reason: "post_asset_idle_prefetch",
				},
			}),
		).toEqual([
			{
				type: "complete_without_render",
				reason: "post_asset_idle_prefetch",
			},
		]);

		expect(
			buildSuccessfulNavigationPostAssetCommands({
				postAssetExecutionPlan: {
					type: "render",
					reason: "post_asset_render",
				},
			}),
		).toEqual([
			{
				type: "render",
				reason: "post_asset_render",
			},
		]);
	});

	it("builds cleanup commands from cleanup execution plans", () => {
		expect(
			buildSuccessfulNavigationCleanupCommands({
				cleanupExecutionPlan: {
					type: "deleteNavigation",
					targetUrl: "http://localhost:3000/cleanup-target",
					reason: "successful_navigation_cleanup",
				},
			}),
		).toEqual([
			{
				type: "delete_navigation",
				targetUrl: "http://localhost:3000/cleanup-target",
				reason: "successful_navigation_cleanup",
			},
		]);

		expect(
			buildSuccessfulNavigationCleanupCommands({
				cleanupExecutionPlan: {
					type: "skip",
					reason: "cleanup_skipped_idle_prefetch_or_non_current_entry",
				},
			}),
		).toEqual([]);
	});

	it("builds checkpoint commands through one checkpoint command seam", () => {
		expect(
			buildSuccessfulNavigationLifecycleCheckpointCommands({
				checkpointExecutionPlan: {
					checkpoint: "pre_waiting",
					preWaitingExecutionPlan: {
						type: "deleteAndStop",
						targetUrl:
							"http://localhost:3000/checkpoint-stage-target",
						reason: "stale_revalidation_pre_waiting",
					},
				},
			}),
		).toEqual([
			{
				type: "delete_navigation",
				targetUrl: "http://localhost:3000/checkpoint-stage-target",
				reason: "stale_revalidation_pre_waiting",
			},
			{
				type: "stop",
				reason: "stale_revalidation_pre_waiting",
			},
		]);

		const response = new Response(JSON.stringify({ ok: true }), {
			status: 200,
			headers: {
				"Content-Type": "application/json",
				"X-Vorma-Build-Id": "1",
			},
		});
		expect(
			buildSuccessfulNavigationLifecycleCheckpointCommands({
				checkpointExecutionPlan: {
					checkpoint: "pre_asset_wait",
					preAssetWaitExecutionPlan: {
						shouldSyncBuildIDBeforeAssetWait: true,
						reason: "pre_asset_wait_sync_build_id_before_asset_wait",
					},
				},
				response,
			}),
		).toEqual([
			{
				type: "sync_build_id_from_response",
				response,
			},
		]);
	});

	it("builds pre-asset-wait commands from build-id sync timing policy", () => {
		const response = new Response(JSON.stringify({ ok: true }), {
			status: 200,
			headers: {
				"Content-Type": "application/json",
				"X-Vorma-Build-Id": "1",
			},
		});

		expect(
			buildSuccessfulNavigationPreAssetWaitCommands({
				preAssetWaitExecutionPlan: {
					shouldSyncBuildIDBeforeAssetWait: true,
					reason: "pre_asset_wait_sync_build_id_before_asset_wait",
				},
				response,
			}),
		).toEqual([
			{
				type: "sync_build_id_from_response",
				response,
			},
		]);

		expect(
			buildSuccessfulNavigationPreAssetWaitCommands({
				preAssetWaitExecutionPlan: {
					shouldSyncBuildIDBeforeAssetWait: false,
					reason: "pre_asset_wait_skip_sync_build_id_before_asset_wait",
				},
				response,
			}),
		).toEqual([]);
	});

	it("builds post-asset side effects and stage command in deterministic order", () => {
		const response = new Response(JSON.stringify({ ok: true }), {
			status: 200,
			headers: {
				"Content-Type": "application/json",
				"X-Vorma-Build-Id": "1",
			},
		});
		const json = {
			matchedPatterns: [],
			loadersData: [],
			importURLs: [],
			exportKeys: [],
			errorExportKeys: [],
			hasRootData: false,
			params: {},
			splatValues: [],
			deps: [],
			cssBundles: [],
			outermostServerError: undefined,
			outermostServerErrorIdx: undefined,
			title: undefined,
			metaHeadEls: undefined,
			restHeadEls: undefined,
		};
		const clientLoadersResult = {
			data: [{ value: "from-client-loader" }],
		};

		expect(
			buildSuccessfulNavigationPostAssetLifecycleCommands({
				postAssetLifecycleExecutionPlan: {
					postAssetExecutionPlan: {
						type: "render",
						reason: "post_asset_render",
					},
					postAssetSideEffectPlan: {
						shouldCommitClientLoadersState: true,
						shouldSyncBuildIDAfterAssetWait: true,
						shouldApplyResponseArtifacts: true,
					},
				},
				response,
				json,
				expectedBuildID: "1",
				clientLoadersResult,
			}),
		).toEqual([
			{
				type: "commit_client_loaders_state",
				clientLoadersResult,
			},
			{
				type: "sync_build_id_from_response",
				response,
			},
			{
				type: "apply_response_artifacts_when_build_matches",
				response,
				json,
				expectedBuildID: "1",
				shouldApplyCSSBundles: false,
			},
			{
				type: "render",
				reason: "post_asset_render",
			},
		]);

		expect(
			buildSuccessfulNavigationPostAssetLifecycleCommands({
				postAssetLifecycleExecutionPlan: {
					postAssetExecutionPlan: {
						type: "stop",
						reason: "post_asset_entry_lost",
					},
					postAssetSideEffectPlan: {
						shouldCommitClientLoadersState: false,
						shouldSyncBuildIDAfterAssetWait: false,
						shouldApplyResponseArtifacts: false,
					},
				},
				response,
				json,
				expectedBuildID: "1",
				clientLoadersResult: undefined,
			}),
		).toEqual([
			{
				type: "stop",
				reason: "post_asset_entry_lost",
			},
		]);

		expect(
			buildSuccessfulNavigationPostAssetLifecycleCommands({
				postAssetLifecycleExecutionPlan: {
					postAssetExecutionPlan: {
						type: "completeWithoutRender",
						reason: "post_asset_idle_prefetch",
					},
					postAssetSideEffectPlan: {
						shouldCommitClientLoadersState: false,
						shouldSyncBuildIDAfterAssetWait: false,
						shouldApplyResponseArtifacts: true,
					},
				},
				response,
				json,
				expectedBuildID: "1",
				clientLoadersResult: undefined,
			}),
		).toEqual([
			{
				type: "apply_response_artifacts_when_build_matches",
				response,
				json,
				expectedBuildID: "1",
				shouldApplyCSSBundles: true,
			},
			{
				type: "complete_without_render",
				reason: "post_asset_idle_prefetch",
			},
		]);
	});
});
