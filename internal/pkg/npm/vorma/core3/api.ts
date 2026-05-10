import type {
	APIOutcome,
	APISettlementResult,
	CoreEffect,
	CoreModel,
	CorePhase,
	RefreshDemand,
	SubmissionRecord,
} from "./model.ts";
import { request_route_revalidation } from "./refresh.ts";

export type APIRefreshDemandInput = {
	outcome: APIOutcome;
	phase: CorePhase;
	submission: SubmissionRecord | undefined;
};

export type APICompletionInput = {
	debounce_refresh: boolean;
	debounce_refresh_ms: number;
	model: CoreModel;
	outcome: APIOutcome;
	refresh_timer_id?: string;
};

export type APICompletionPlan = {
	effects: readonly CoreEffect[];
	model: CoreModel;
};

export function api_refresh_demand(
	input: APIRefreshDemandInput,
): RefreshDemand | undefined {
	if (!input.submission) {
		return undefined;
	}
	if (
		input.outcome.kind === "stale" ||
		input.outcome.operation_id !== input.submission.operation_id ||
		input.outcome.submission_key !== input.submission.submission_key
	) {
		return undefined;
	}
	if (
		(input.outcome.kind !== "success" &&
			input.outcome.kind !== "failure") ||
		!input.outcome.revalidation_required ||
		!input.submission.revalidate
	) {
		return undefined;
	}
	return {
		operation_id: input.submission.revalidation_operation_id,
		public_call_ids:
			input.phase === "ready" &&
			input.submission.revalidation_public_call_id
				? [input.submission.revalidation_public_call_id]
				: [],
		reason: "apiRequest",
		skip_work_indicator: input.submission.skip_work_indicator,
	};
}

export function complete_api_submit(
	input: APICompletionInput,
): APICompletionPlan | undefined {
	if (input.outcome.kind === "stale") {
		return undefined;
	}
	if (input.outcome.kind !== "success" && input.outcome.kind !== "failure") {
		return undefined;
	}
	const submission = input.model.submissions[input.outcome.submission_key];
	if (!submission) {
		return undefined;
	}
	if (submission.operation_id !== input.outcome.operation_id) {
		return undefined;
	}

	let model = input.model;
	const effects: CoreEffect[] = [];
	const demand = api_refresh_demand({
		outcome: input.outcome,
		phase: input.model.phase,
		submission,
	});
	if (demand) {
		const refresh_plan = request_route_revalidation({
			debounce: input.debounce_refresh,
			debounce_ms: input.debounce_refresh_ms,
			demand,
			model,
			timer_id: input.refresh_timer_id,
		});
		model = refresh_plan.model;
		effects.push(...refresh_plan.effects);
	}

	const revalidation_public_call_id =
		input.outcome.revalidation_required && submission.revalidate
			? submission.revalidation_public_call_id
			: undefined;
	let result: APISettlementResult;
	if (input.outcome.kind === "success") {
		result = {
			data: input.outcome.data,
			response: input.outcome.response,
			revalidation_public_call_id,
			success: true,
		};
	} else {
		result = {
			error: input.outcome.error,
			revalidation_public_call_id,
			success: false,
		};
		if (input.outcome.response !== undefined) {
			result.response = input.outcome.response;
		}
	}

	const submissions = { ...model.submissions };
	delete submissions[input.outcome.submission_key];

	effects.push({
		public_call_id: submission.public_call_id,
		result,
		type: "resolve_api_call",
	});

	return {
		effects,
		model: {
			...model,
			submissions,
		},
	};
}
