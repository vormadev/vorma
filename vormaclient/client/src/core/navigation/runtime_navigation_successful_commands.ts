import type { NavigationOutcome, NavigationPhase } from "./types.ts";
import type {
	BuildIDSyncTiming,
	SuccessfulNavigationLifecycleStageExecutionPlan,
	SuccessfulNavigationPostAssetExecutionPlan,
	SuccessfulNavigationPostAssetSideEffectPlan,
	SuccessfulNavigationPostWaitingExecutionPlan,
	SuccessfulNavigationPreWaitingExecutionPlan,
} from "./runtime_navigation_outcome_state_machine.ts";

type SuccessfulNavigationSuccessOutcome = Extract<
	NavigationOutcome,
	{ type: "success" }
>;
type SuccessfulNavigationClientLoadersResult =
	| Awaited<SuccessfulNavigationSuccessOutcome["waitFnPromise"]>
	| undefined;

export type SuccessfulNavigationLifecycleCommand =
	| {
			type: "stop";
			reason:
				| SuccessfulNavigationPreWaitingExecutionPlan["reason"]
				| SuccessfulNavigationPostWaitingExecutionPlan["reason"]
				| SuccessfulNavigationPostAssetExecutionPlan["reason"];
	  }
	| {
			type: "delete_navigation";
			targetUrl: string;
			reason: SuccessfulNavigationPreWaitingExecutionPlan["reason"];
	  }
	| {
			type: "transition_phase";
			phase: NavigationPhase;
			reason: string;
	  }
	| {
			type: "complete_without_render";
			reason: SuccessfulNavigationPostAssetExecutionPlan["reason"];
	  }
	| {
			type: "render";
			reason: SuccessfulNavigationPostAssetExecutionPlan["reason"];
	  }
	| {
			type: "commit_client_loaders_state";
			clientLoadersResult: SuccessfulNavigationClientLoadersResult;
	  }
	| {
			type: "sync_build_id_from_response";
			response: SuccessfulNavigationSuccessOutcome["response"];
	  }
	| {
			type: "apply_response_artifacts_when_build_matches";
			response: SuccessfulNavigationSuccessOutcome["response"];
			json: SuccessfulNavigationSuccessOutcome["json"];
			expectedBuildID: string;
	  };

export function buildSuccessfulNavigationPreWaitingCommands(props: {
	preWaitingExecutionPlan: SuccessfulNavigationPreWaitingExecutionPlan;
}): SuccessfulNavigationLifecycleCommand[] {
	const { preWaitingExecutionPlan } = props;
	switch (preWaitingExecutionPlan.type) {
		case "stop":
			return [
				{
					type: "stop",
					reason: preWaitingExecutionPlan.reason,
				},
			];
		case "deleteAndStop":
			return [
				{
					type: "delete_navigation",
					targetUrl: preWaitingExecutionPlan.targetUrl,
					reason: preWaitingExecutionPlan.reason,
				},
				{
					type: "stop",
					reason: preWaitingExecutionPlan.reason,
				},
			];
		case "continue":
			return [
				{
					type: "transition_phase",
					phase: "waiting",
					reason: "process_successful_navigation_waiting",
				},
			];
	}
}

export function buildSuccessfulNavigationPostWaitingCommands(props: {
	postWaitingExecutionPlan: SuccessfulNavigationPostWaitingExecutionPlan;
}): SuccessfulNavigationLifecycleCommand[] {
	const { postWaitingExecutionPlan } = props;
	if (postWaitingExecutionPlan.type === "stop") {
		return [
			{
				type: "stop",
				reason: postWaitingExecutionPlan.reason,
			},
		];
	}
	return [];
}

export function buildSuccessfulNavigationPostAssetCommands(props: {
	postAssetExecutionPlan: SuccessfulNavigationPostAssetExecutionPlan;
}): SuccessfulNavigationLifecycleCommand[] {
	const { postAssetExecutionPlan } = props;
	switch (postAssetExecutionPlan.type) {
		case "stop":
			return [
				{
					type: "stop",
					reason: postAssetExecutionPlan.reason,
				},
			];
		case "completeWithoutRender":
			return [
				{
					type: "complete_without_render",
					reason: postAssetExecutionPlan.reason,
				},
			];
		case "render":
			return [
				{
					type: "render",
					reason: postAssetExecutionPlan.reason,
				},
			];
	}
}

export function buildSuccessfulNavigationPreAssetWaitCommands(props: {
	buildIDSyncTiming: BuildIDSyncTiming;
	response: SuccessfulNavigationSuccessOutcome["response"];
}): SuccessfulNavigationLifecycleCommand[] {
	if (props.buildIDSyncTiming !== "before_asset_wait") {
		return [];
	}

	return [
		{
			type: "sync_build_id_from_response",
			response: props.response,
		},
	];
}

export function buildSuccessfulNavigationPostAssetLifecycleCommands(props: {
	postAssetExecutionPlan: SuccessfulNavigationPostAssetExecutionPlan;
	postAssetSideEffectPlan: SuccessfulNavigationPostAssetSideEffectPlan;
	response: SuccessfulNavigationSuccessOutcome["response"];
	json: SuccessfulNavigationSuccessOutcome["json"];
	expectedBuildID: string;
	clientLoadersResult: SuccessfulNavigationClientLoadersResult;
}): SuccessfulNavigationLifecycleCommand[] {
	const sideEffectCommands: SuccessfulNavigationLifecycleCommand[] = [];

	if (props.postAssetSideEffectPlan.shouldCommitClientLoadersState) {
		sideEffectCommands.push({
			type: "commit_client_loaders_state",
			clientLoadersResult: props.clientLoadersResult,
		});
	}

	if (props.postAssetSideEffectPlan.shouldSyncBuildIDAfterAssetWait) {
		sideEffectCommands.push({
			type: "sync_build_id_from_response",
			response: props.response,
		});
	}

	if (props.postAssetSideEffectPlan.shouldApplyResponseArtifacts) {
		sideEffectCommands.push({
			type: "apply_response_artifacts_when_build_matches",
			response: props.response,
			json: props.json,
			expectedBuildID: props.expectedBuildID,
		});
	}

	return [
		...sideEffectCommands,
		...buildSuccessfulNavigationPostAssetCommands({
			postAssetExecutionPlan: props.postAssetExecutionPlan,
		}),
	];
}

export function buildSuccessfulNavigationLifecycleStageCommands(props: {
	stageExecutionPlan: SuccessfulNavigationLifecycleStageExecutionPlan;
}): SuccessfulNavigationLifecycleCommand[] {
	const { stageExecutionPlan } = props;

	switch (stageExecutionPlan.stage) {
		case "pre_waiting":
			return buildSuccessfulNavigationPreWaitingCommands({
				preWaitingExecutionPlan: stageExecutionPlan.plan,
			});
		case "post_waiting":
			return buildSuccessfulNavigationPostWaitingCommands({
				postWaitingExecutionPlan: stageExecutionPlan.plan,
			});
		case "post_asset":
			return buildSuccessfulNavigationPostAssetCommands({
				postAssetExecutionPlan: stageExecutionPlan.plan,
			});
	}
}
