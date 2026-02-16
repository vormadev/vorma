import { describe, expect, it } from "vitest";
import {
	buildSuccessfulNavigationLifecycleStageCommands,
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

	it("builds stage commands through the unified stage-plan command seam", () => {
		expect(
			buildSuccessfulNavigationLifecycleStageCommands({
				stageExecutionPlan: {
					stage: "pre_waiting",
					plan: {
						type: "deleteAndStop",
						targetUrl: "http://localhost:3000/stage-test",
						reason: "stale_revalidation_pre_waiting",
					},
				},
			}),
		).toEqual([
			{
				type: "delete_navigation",
				targetUrl: "http://localhost:3000/stage-test",
				reason: "stale_revalidation_pre_waiting",
			},
			{
				type: "stop",
				reason: "stale_revalidation_pre_waiting",
			},
		]);

		expect(
			buildSuccessfulNavigationLifecycleStageCommands({
				stageExecutionPlan: {
					stage: "post_waiting",
					plan: {
						type: "continue",
						reason: "post_waiting_entry_current",
					},
				},
			}),
		).toEqual([]);
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
				buildIDSyncTiming: "before_asset_wait",
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
				buildIDSyncTiming: "after_asset_wait_if_not_stopped",
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
				postAssetExecutionPlan: {
					type: "render",
					reason: "post_asset_render",
				},
				postAssetSideEffectPlan: {
					shouldCommitClientLoadersState: true,
					shouldSyncBuildIDAfterAssetWait: true,
					shouldApplyResponseArtifacts: true,
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
			},
			{
				type: "render",
				reason: "post_asset_render",
			},
		]);

		expect(
			buildSuccessfulNavigationPostAssetLifecycleCommands({
				postAssetExecutionPlan: {
					type: "stop",
					reason: "post_asset_entry_lost",
				},
				postAssetSideEffectPlan: {
					shouldCommitClientLoadersState: false,
					shouldSyncBuildIDAfterAssetWait: false,
					shouldApplyResponseArtifacts: false,
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
	});
});
