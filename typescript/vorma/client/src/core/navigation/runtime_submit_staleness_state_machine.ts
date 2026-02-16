export type SubmitStalenessCheckpoint =
	| "post_request"
	| "pre_finalize"
	| "post_response_classification"
	| "post_redirect_effectuation"
	| "pre_success_return"
	| "post_auto_revalidate";

type SubmitStalenessContinueReason =
	`submit_staleness_${SubmitStalenessCheckpoint}_continue_current`;
type SubmitStalenessStopReason =
	`submit_staleness_${SubmitStalenessCheckpoint}_stop_not_current`;

export type SubmitStalenessCheckpointExecutionPlan =
	| {
			checkpoint: SubmitStalenessCheckpoint;
			type: "continue";
			reason: SubmitStalenessContinueReason;
	  }
	| {
			checkpoint: SubmitStalenessCheckpoint;
			type: "stop";
			reason: SubmitStalenessStopReason;
	  };

export function decideSubmitStalenessCheckpointExecutionPlan(props: {
	checkpoint: SubmitStalenessCheckpoint;
	isSubmissionCurrent: boolean;
}): SubmitStalenessCheckpointExecutionPlan {
	if (props.isSubmissionCurrent) {
		return {
			checkpoint: props.checkpoint,
			type: "continue",
			reason: `submit_staleness_${props.checkpoint}_continue_current`,
		};
	}

	return {
		checkpoint: props.checkpoint,
		type: "stop",
		reason: `submit_staleness_${props.checkpoint}_stop_not_current`,
	};
}
