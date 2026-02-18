import { describe, expect, it } from "vitest";
import { decideRenderCommitCheckpointExecutionPlan } from "../../core/render_commit_runtime.ts";

describe("render runtime commit checkpoint state machine", () => {
	it("allows pre-module-load checkpoint when commit is allowed", () => {
		const plan = decideRenderCommitCheckpointExecutionPlan({
			checkpoint: "pre_module_load",
			shouldCommitRender: true,
		});

		expect(plan).toEqual({
			checkpoint: "pre_module_load",
			type: "continue",
			reason: "render_commit_allowed_pre_module_load",
		});
	});

	it("rejects pre-module-load checkpoint when commit is disallowed", () => {
		const plan = decideRenderCommitCheckpointExecutionPlan({
			checkpoint: "pre_module_load",
			shouldCommitRender: false,
		});

		expect(plan).toEqual({
			checkpoint: "pre_module_load",
			type: "stop",
			reason: "render_commit_rejected_pre_module_load",
		});
	});

	it("allows post-module-load checkpoint when commit is allowed", () => {
		const plan = decideRenderCommitCheckpointExecutionPlan({
			checkpoint: "post_module_load",
			shouldCommitRender: true,
		});

		expect(plan).toEqual({
			checkpoint: "post_module_load",
			type: "continue",
			reason: "render_commit_allowed_post_module_load",
		});
	});

	it("rejects post-module-load checkpoint when commit is disallowed", () => {
		const plan = decideRenderCommitCheckpointExecutionPlan({
			checkpoint: "post_module_load",
			shouldCommitRender: false,
		});

		expect(plan).toEqual({
			checkpoint: "post_module_load",
			type: "stop",
			reason: "render_commit_rejected_post_module_load",
		});
	});
});
