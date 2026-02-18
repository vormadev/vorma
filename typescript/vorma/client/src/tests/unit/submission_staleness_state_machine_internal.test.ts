import { describe, expect, it } from "vitest";
import { decideSubmitStalenessCheckpointExecutionPlan } from "../../core/navigation/runtime_submit.ts";

describe("submission staleness checkpoint state machine", () => {
	it("continues for current submissions across all checkpoints", () => {
		expect(
			decideSubmitStalenessCheckpointExecutionPlan({
				checkpoint: "post_request",
				isSubmissionCurrent: true,
			}),
		).toEqual({
			checkpoint: "post_request",
			type: "continue",
			reason: "submit_staleness_post_request_continue_current",
		});
		expect(
			decideSubmitStalenessCheckpointExecutionPlan({
				checkpoint: "pre_finalize",
				isSubmissionCurrent: true,
			}),
		).toEqual({
			checkpoint: "pre_finalize",
			type: "continue",
			reason: "submit_staleness_pre_finalize_continue_current",
		});
		expect(
			decideSubmitStalenessCheckpointExecutionPlan({
				checkpoint: "post_response_classification",
				isSubmissionCurrent: true,
			}),
		).toEqual({
			checkpoint: "post_response_classification",
			type: "continue",
			reason: "submit_staleness_post_response_classification_continue_current",
		});
		expect(
			decideSubmitStalenessCheckpointExecutionPlan({
				checkpoint: "post_redirect_effectuation",
				isSubmissionCurrent: true,
			}),
		).toEqual({
			checkpoint: "post_redirect_effectuation",
			type: "continue",
			reason: "submit_staleness_post_redirect_effectuation_continue_current",
		});
		expect(
			decideSubmitStalenessCheckpointExecutionPlan({
				checkpoint: "pre_success_return",
				isSubmissionCurrent: true,
			}),
		).toEqual({
			checkpoint: "pre_success_return",
			type: "continue",
			reason: "submit_staleness_pre_success_return_continue_current",
		});
		expect(
			decideSubmitStalenessCheckpointExecutionPlan({
				checkpoint: "post_auto_revalidate",
				isSubmissionCurrent: true,
			}),
		).toEqual({
			checkpoint: "post_auto_revalidate",
			type: "continue",
			reason: "submit_staleness_post_auto_revalidate_continue_current",
		});
	});

	it("stops for stale submissions across all checkpoints", () => {
		expect(
			decideSubmitStalenessCheckpointExecutionPlan({
				checkpoint: "post_request",
				isSubmissionCurrent: false,
			}),
		).toEqual({
			checkpoint: "post_request",
			type: "stop",
			reason: "submit_staleness_post_request_stop_not_current",
		});
		expect(
			decideSubmitStalenessCheckpointExecutionPlan({
				checkpoint: "pre_finalize",
				isSubmissionCurrent: false,
			}),
		).toEqual({
			checkpoint: "pre_finalize",
			type: "stop",
			reason: "submit_staleness_pre_finalize_stop_not_current",
		});
		expect(
			decideSubmitStalenessCheckpointExecutionPlan({
				checkpoint: "post_response_classification",
				isSubmissionCurrent: false,
			}),
		).toEqual({
			checkpoint: "post_response_classification",
			type: "stop",
			reason: "submit_staleness_post_response_classification_stop_not_current",
		});
		expect(
			decideSubmitStalenessCheckpointExecutionPlan({
				checkpoint: "post_redirect_effectuation",
				isSubmissionCurrent: false,
			}),
		).toEqual({
			checkpoint: "post_redirect_effectuation",
			type: "stop",
			reason: "submit_staleness_post_redirect_effectuation_stop_not_current",
		});
		expect(
			decideSubmitStalenessCheckpointExecutionPlan({
				checkpoint: "pre_success_return",
				isSubmissionCurrent: false,
			}),
		).toEqual({
			checkpoint: "pre_success_return",
			type: "stop",
			reason: "submit_staleness_pre_success_return_stop_not_current",
		});
		expect(
			decideSubmitStalenessCheckpointExecutionPlan({
				checkpoint: "post_auto_revalidate",
				isSubmissionCurrent: false,
			}),
		).toEqual({
			checkpoint: "post_auto_revalidate",
			type: "stop",
			reason: "submit_staleness_post_auto_revalidate_stop_not_current",
		});
	});
});
