export type RenderCommitCheckpoint = "pre_module_load" | "post_module_load";

export type RenderCommitCheckpointExecutionPlan =
	| {
			checkpoint: RenderCommitCheckpoint;
			type: "continue";
			reason:
				| "render_commit_allowed_pre_module_load"
				| "render_commit_allowed_post_module_load";
	  }
	| {
			checkpoint: RenderCommitCheckpoint;
			type: "stop";
			reason:
				| "render_commit_rejected_pre_module_load"
				| "render_commit_rejected_post_module_load";
	  };

export function decideRenderCommitCheckpointExecutionPlan(props: {
	checkpoint: RenderCommitCheckpoint;
	shouldCommitRender: boolean;
}): RenderCommitCheckpointExecutionPlan {
	switch (props.checkpoint) {
		case "pre_module_load":
			return props.shouldCommitRender
				? {
						checkpoint: props.checkpoint,
						type: "continue",
						reason: "render_commit_allowed_pre_module_load",
					}
				: {
						checkpoint: props.checkpoint,
						type: "stop",
						reason: "render_commit_rejected_pre_module_load",
					};
		case "post_module_load":
			return props.shouldCommitRender
				? {
						checkpoint: props.checkpoint,
						type: "continue",
						reason: "render_commit_allowed_post_module_load",
					}
				: {
						checkpoint: props.checkpoint,
						type: "stop",
						reason: "render_commit_rejected_post_module_load",
					};
	}
}
