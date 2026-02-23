import type { NavigationOutcome, NavigationPhase } from "./types.ts";
import type {
	SuccessfulNavigationCleanupExecutionPlan,
	SuccessfulNavigationLifecycleCheckpointExecutionPlan,
	SuccessfulNavigationPostAssetLifecycleExecutionPlan,
	SuccessfulNavigationPostAssetExecutionPlan,
	SuccessfulNavigationPreAssetWaitExecutionPlan,
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
type SuccessfulNavigationDeleteReason =
	| SuccessfulNavigationPreWaitingExecutionPlan["reason"]
	| SuccessfulNavigationCleanupExecutionPlan["reason"];

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
			reason: SuccessfulNavigationDeleteReason;
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
			shouldApplyCSSBundles: boolean;
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
	preAssetWaitExecutionPlan: SuccessfulNavigationPreAssetWaitExecutionPlan;
	response: SuccessfulNavigationSuccessOutcome["response"];
}): SuccessfulNavigationLifecycleCommand[] {
	if (!props.preAssetWaitExecutionPlan.shouldSyncBuildIDBeforeAssetWait) {
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
	postAssetLifecycleExecutionPlan: SuccessfulNavigationPostAssetLifecycleExecutionPlan;
	response: SuccessfulNavigationSuccessOutcome["response"];
	json: SuccessfulNavigationSuccessOutcome["json"];
	expectedBuildID: string;
	clientLoadersResult: SuccessfulNavigationClientLoadersResult;
}): SuccessfulNavigationLifecycleCommand[] {
	const { postAssetSideEffectPlan, postAssetExecutionPlan } =
		props.postAssetLifecycleExecutionPlan;
	const sideEffectCommands: SuccessfulNavigationLifecycleCommand[] = [];
	const shouldApplyCSSBundles =
		postAssetExecutionPlan.type === "completeWithoutRender";

	if (postAssetSideEffectPlan.shouldCommitClientLoadersState) {
		sideEffectCommands.push({
			type: "commit_client_loaders_state",
			clientLoadersResult: props.clientLoadersResult,
		});
	}

	if (postAssetSideEffectPlan.shouldSyncBuildIDAfterAssetWait) {
		sideEffectCommands.push({
			type: "sync_build_id_from_response",
			response: props.response,
		});
	}

	if (postAssetSideEffectPlan.shouldApplyResponseArtifacts) {
		sideEffectCommands.push({
			type: "apply_response_artifacts_when_build_matches",
			response: props.response,
			json: props.json,
			expectedBuildID: props.expectedBuildID,
			shouldApplyCSSBundles,
		});
	}

	return [
		...sideEffectCommands,
		...buildSuccessfulNavigationPostAssetCommands({
			postAssetExecutionPlan,
		}),
	];
}

export function buildSuccessfulNavigationCleanupCommands(props: {
	cleanupExecutionPlan: SuccessfulNavigationCleanupExecutionPlan;
}): SuccessfulNavigationLifecycleCommand[] {
	const { cleanupExecutionPlan } = props;

	switch (cleanupExecutionPlan.type) {
		case "deleteNavigation":
			return [
				{
					type: "delete_navigation",
					targetUrl: cleanupExecutionPlan.targetUrl,
					reason: cleanupExecutionPlan.reason,
				},
			];
		case "skip":
			return [];
	}
}

function requireSuccessfulNavigationCheckpointCommandInput<T>(props: {
	value: T | undefined;
	checkpoint: SuccessfulNavigationLifecycleCheckpointExecutionPlan["checkpoint"];
	field: string;
}): T {
	if (props.value === undefined) {
		throw new Error(
			`Missing '${props.field}' for successful navigation checkpoint command build '${props.checkpoint}'.`,
		);
	}

	return props.value;
}

export function buildSuccessfulNavigationLifecycleCheckpointCommands(props: {
	checkpointExecutionPlan: SuccessfulNavigationLifecycleCheckpointExecutionPlan;
	response?: SuccessfulNavigationSuccessOutcome["response"];
	json?: SuccessfulNavigationSuccessOutcome["json"];
	expectedBuildID?: string;
	clientLoadersResult?: SuccessfulNavigationClientLoadersResult;
}): SuccessfulNavigationLifecycleCommand[] {
	switch (props.checkpointExecutionPlan.checkpoint) {
		case "pre_waiting":
			return buildSuccessfulNavigationPreWaitingCommands({
				preWaitingExecutionPlan:
					props.checkpointExecutionPlan.preWaitingExecutionPlan,
			});
		case "post_waiting":
			return buildSuccessfulNavigationPostWaitingCommands({
				postWaitingExecutionPlan:
					props.checkpointExecutionPlan.postWaitingExecutionPlan,
			});
		case "pre_asset_wait":
			return buildSuccessfulNavigationPreAssetWaitCommands({
				preAssetWaitExecutionPlan:
					props.checkpointExecutionPlan.preAssetWaitExecutionPlan,
				response: requireSuccessfulNavigationCheckpointCommandInput({
					value: props.response,
					checkpoint: props.checkpointExecutionPlan.checkpoint,
					field: "response",
				}),
			});
		case "post_asset":
			return buildSuccessfulNavigationPostAssetLifecycleCommands({
				postAssetLifecycleExecutionPlan:
					props.checkpointExecutionPlan
						.postAssetLifecycleExecutionPlan,
				response: requireSuccessfulNavigationCheckpointCommandInput({
					value: props.response,
					checkpoint: props.checkpointExecutionPlan.checkpoint,
					field: "response",
				}),
				json: requireSuccessfulNavigationCheckpointCommandInput({
					value: props.json,
					checkpoint: props.checkpointExecutionPlan.checkpoint,
					field: "json",
				}),
				expectedBuildID:
					requireSuccessfulNavigationCheckpointCommandInput({
						value: props.expectedBuildID,
						checkpoint: props.checkpointExecutionPlan.checkpoint,
						field: "expectedBuildID",
					}),
				clientLoadersResult: props.clientLoadersResult,
			});
		case "cleanup":
			return buildSuccessfulNavigationCleanupCommands({
				cleanupExecutionPlan:
					props.checkpointExecutionPlan.cleanupExecutionPlan,
			});
	}
}
