import { route_build_skew_effect } from "./build_skew.ts";
import type {
	BrowserKey,
	BuildSkewReport,
	CoreEffect,
	CoreModel,
	CoreOperation,
	OperationID,
	PreparationOutcome,
	PreparedRoute,
	RouteHooksOutcome,
	RouteOutcome,
	RouteSnapshot,
	TimerID,
} from "./model.ts";
import {
	can_publish_route,
	is_route_operation,
	owns_route_completion,
	satisfiable_refresh_for_operation,
} from "./operation.ts";
import {
	append_reject_public_calls,
	append_resolve_public_calls,
	plan_route_publication,
	publication_effects,
	type RoutePublicationOperation,
} from "./publication.ts";
import { plan_route_readiness } from "./readiness.ts";
import { begin_pending_refresh, plan_refresh_retry } from "./refresh.ts";

export const REVALIDATION_BUILD_SKEW_REASON = "build_skew";
export const REVALIDATION_MAX_RETRIES_EXHAUSTED_REASON =
	"max_retries_exhausted";

export type PreparedRouteTransitionInput = {
	hooks_required: boolean;
	model: CoreModel;
	operation_id: OperationID;
	prepared: PreparedRoute;
	view_transition_available: boolean;
};

export type PreparedRouteTransition =
	| {
			effects: readonly CoreEffect[];
			kind: "hooks_running";
			model: CoreModel;
	  }
	| {
			effects: readonly CoreEffect[];
			kind: "publishing";
			model: CoreModel;
	  };

export type RouteStoppedTransition = {
	effects: readonly CoreEffect[];
	kind: "aborted" | "completed" | "failed" | "ignored_stale";
	model: CoreModel;
};

export type RoutePreparationTransitionInput = {
	hooks_required: boolean;
	model: CoreModel;
	outcome: PreparationOutcome;
	view_transition_available: boolean;
};

export type RoutePreparationTransition =
	| PreparedRouteTransition
	| RouteStoppedTransition;

export type RouteHooksTransitionInput = {
	model: CoreModel;
	outcome: RouteHooksOutcome;
	view_transition_available: boolean;
};

export type RouteHooksTransition =
	| PreparedRouteTransition
	| RouteStoppedTransition;

export type RouteOutcomeTransitionInput = {
	build_skew_report?: BuildSkewReport;
	model: CoreModel;
	outcome: RouteOutcome;
	revalidation_redirect?: {
		browser_key: BrowserKey;
		navigation_operation_id: OperationID;
		skip_work_indicator: boolean;
		state: unknown;
	};
	revalidation_retry?: {
		backoff_base_ms: number;
		backoff_cap_ms: number;
		max_retries: number;
		next_operation_id: OperationID;
		timer_id: TimerID;
	};
};

export type RouteOutcomeTransition =
	| {
			effects: readonly CoreEffect[];
			kind: "preparing";
			model: CoreModel;
	  }
	| {
			effects: readonly CoreEffect[];
			kind: "redirecting";
			model: CoreModel;
	  }
	| RouteStoppedTransition
	| {
			effects: readonly CoreEffect[];
			kind: "retrying";
			model: CoreModel;
	  };

export type RoutePublicationCommitInput = {
	model: CoreModel;
	next: RouteSnapshot;
	operation_id: OperationID;
};

export type RoutePublicationSettlementInput = {
	model: CoreModel;
	operation_id: OperationID;
};

export type RoutePublicationSettlementPlan = {
	effects: readonly CoreEffect[];
	model: CoreModel;
};

export function advance_route_outcome(
	input: RouteOutcomeTransitionInput,
): RouteOutcomeTransition | undefined {
	const operation = input.model.operations[input.outcome.operation_id];
	if (!is_route_operation(operation) || input.outcome.kind === "stale") {
		return undefined;
	}

	const owns_completion = owns_route_completion(input.model, operation);
	const build_skew_effect = route_build_skew_effect({
		model: input.model,
		operation,
		report: input.build_skew_report,
	});

	if (input.outcome.kind === "route_data") {
		if (!owns_completion) {
			return undefined;
		}

		let trigger: "boot" | "navigation" | "popstate" | "revalidation";
		if (operation.kind === "boot") {
			trigger = "boot";
		} else if (operation.kind === "navigation") {
			trigger = "navigation";
		} else if (operation.kind === "popstate") {
			trigger = "popstate";
		} else {
			trigger = "revalidation";
		}
		const href =
			operation.kind === "popstate"
				? operation.browser.href
				: operation.href;
		const history_state =
			operation.kind === "boot" || operation.kind === "navigation"
				? operation.state
				: operation.kind === "popstate"
					? operation.browser.state
					: input.model.browser?.state;
		return prepend_effect(
			{
				effects: [
					{
						client_build_id: input.model.client_build_id,
						history_state,
						href,
						operation_id: operation.id,
						payload: input.outcome.payload,
						trigger,
						type: "prepare_route",
					},
				],
				kind: "preparing",
				model: {
					...input.model,
					operations: {
						...input.model.operations,
						[operation.id]: {
							...operation,
							status: "classified",
						},
					},
				},
			},
			build_skew_effect,
		);
	}

	if (
		input.outcome.kind === "soft_redirect" &&
		operation.kind === "navigation" &&
		owns_completion
	) {
		const redirected_operation = {
			...operation,
			href: input.outcome.href,
			redirect_count: operation.redirect_count + 1,
			source: "redirect",
			status: "started",
		} satisfies typeof operation;
		return prepend_effect(
			{
				effects: [
					{
						client_build_id: input.model.client_build_id,
						href: redirected_operation.href,
						operation_id: redirected_operation.id,
						trigger: "navigation",
						type: "fetch_route",
					},
				],
				kind: "redirecting",
				model: {
					...input.model,
					operations: {
						...input.model.operations,
						[operation.id]: redirected_operation,
					},
				},
			},
			build_skew_effect,
		);
	}

	if (
		input.outcome.kind === "ignored_background_refresh" &&
		operation.kind === "route_revalidation"
	) {
		return prepend_effect(
			settle_route_revalidation_without_publication({
				model: input.model,
				operation,
				result: { ok: true },
				status: "ignored_stale",
			}),
			build_skew_effect,
		);
	}

	if (!owns_completion) {
		return undefined;
	}

	if (input.outcome.kind === "hard_redirect") {
		const stopped = stop_route_operation_without_publication({
			model: input.model,
			operation,
			status: "completed",
		});
		if (!stopped) {
			return undefined;
		}
		return prepend_effect(
			{
				...stopped,
				effects: [
					{
						href: input.outcome.href,
						type: "hard_redirect",
					},
					...stopped.effects,
				],
			},
			build_skew_effect,
		);
	}

	if (input.outcome.kind === "build_skew_reload") {
		const stopped = stop_route_operation_without_publication({
			model: input.model,
			operation,
			status: "completed",
		});
		if (!stopped) {
			return undefined;
		}
		return prepend_effect(
			{
				...stopped,
				effects: [
					{
						href:
							operation.kind === "popstate"
								? operation.browser.href
								: operation.href,
						type: "hard_redirect",
					},
					...stopped.effects,
				],
			},
			build_skew_effect,
		);
	}

	if (
		input.outcome.kind === "invalid_redirect" ||
		input.outcome.kind === "redirect_loop" ||
		input.outcome.kind === "route_error"
	) {
		if (
			input.outcome.kind === "invalid_redirect" &&
			operation.kind === "route_revalidation"
		) {
			return prepend_effect(
				settle_route_revalidation_without_publication({
					model: input.model,
					operation,
					result: { ok: true },
					status: "completed",
				}),
				build_skew_effect,
			);
		}
		return prepend_effect(
			stop_route_operation_without_publication({
				cause:
					input.outcome.kind === "route_error"
						? input.outcome.status_text
						: undefined,
				model: input.model,
				operation,
				status: "failed",
			}),
			build_skew_effect,
		);
	}

	if (
		input.outcome.kind === "soft_redirect" &&
		operation.kind === "route_revalidation"
	) {
		const refresh = satisfiable_refresh_for_operation(
			input.model,
			operation,
		);
		if (!input.revalidation_redirect || !refresh) {
			return undefined;
		}
		const redirected_operation = {
			browser_key: input.revalidation_redirect.browser_key,
			href: input.outcome.href,
			id: input.revalidation_redirect.navigation_operation_id,
			kind: "navigation",
			public_call_ids: [],
			redirect_count: 0,
			replace: true,
			rights: [
				"abort_effects",
				"own_visible_transition",
				"publish_route",
				"satisfy_refresh_demand",
			],
			source: "redirect",
			state: input.revalidation_redirect.state,
			status: "started",
			skip_work_indicator:
				input.revalidation_redirect.skip_work_indicator,
		} satisfies CoreOperation;
		return prepend_effect(
			{
				effects: [
					{
						client_build_id: input.model.client_build_id,
						href: redirected_operation.href,
						operation_id: redirected_operation.id,
						trigger: "navigation",
						type: "fetch_route",
					},
				],
				kind: "redirecting",
				model: {
					...input.model,
					active_route_operation_id: redirected_operation.id,
					operations: {
						...input.model.operations,
						[operation.id]: {
							...operation,
							status: "completed",
						},
						[redirected_operation.id]: redirected_operation,
					},
					publication_owner_id: redirected_operation.id,
					refresh: {
						attempt: refresh.attempt,
						demand: refresh.demand,
						kind: "running",
						operation_id: redirected_operation.id,
					},
				},
			},
			build_skew_effect,
		);
	}

	if (
		input.outcome.kind === "build_skew_drop" &&
		operation.kind === "route_revalidation"
	) {
		return prepend_effect(
			settle_route_revalidation_without_publication({
				model: input.model,
				operation,
				result: {
					ok: false,
					reason: REVALIDATION_BUILD_SKEW_REASON,
				},
				status: "completed",
			}),
			build_skew_effect,
		);
	}

	if (
		input.outcome.kind === "terminal_revalidation_failure" &&
		operation.kind === "route_revalidation"
	) {
		return prepend_effect(
			settle_route_revalidation_without_publication({
				model: input.model,
				operation,
				result: {
					ok: false,
					reason: REVALIDATION_MAX_RETRIES_EXHAUSTED_REASON,
				},
				status: "failed",
			}),
			build_skew_effect,
		);
	}

	if (
		input.outcome.kind === "retryable_revalidation_failure" &&
		operation.kind === "route_revalidation"
	) {
		const refresh = satisfiable_refresh_for_operation(
			input.model,
			operation,
		);
		if (!input.revalidation_retry || !refresh) {
			return undefined;
		}
		const retry = plan_refresh_retry({
			backoff_base_ms: input.revalidation_retry.backoff_base_ms,
			backoff_cap_ms: input.revalidation_retry.backoff_cap_ms,
			demand: refresh.demand,
			max_retries: input.revalidation_retry.max_retries,
			next_operation_id: input.revalidation_retry.next_operation_id,
			previous_attempt: operation.attempt,
			timer_id: input.revalidation_retry.timer_id,
		});
		if (retry.kind === "exhausted") {
			return prepend_effect(
				settle_route_revalidation_without_publication({
					model: input.model,
					operation,
					result: {
						ok: false,
						reason: REVALIDATION_MAX_RETRIES_EXHAUSTED_REASON,
					},
					status: "failed",
				}),
				build_skew_effect,
			);
		}
		return prepend_effect(
			{
				effects: retry.effects,
				kind: "retrying",
				model: {
					...input.model,
					operations: {
						...input.model.operations,
						[operation.id]: {
							...operation,
							status: "failed",
						},
					},
					publication_owner_id:
						input.model.publication_owner_id === operation.id
							? null
							: input.model.publication_owner_id,
					refresh: retry.refresh,
				},
			},
			build_skew_effect,
		);
	}

	return undefined;
}

export function advance_route_preparation(
	input: RoutePreparationTransitionInput,
): RoutePreparationTransition | undefined {
	const operation = input.model.operations[input.outcome.operation_id];
	if (!is_route_operation(operation) || input.outcome.kind === "stale") {
		return undefined;
	}
	if (!owns_route_completion(input.model, operation)) {
		return undefined;
	}
	if (input.outcome.kind === "prepared") {
		const prepared_model: CoreModel = {
			...input.model,
			operations: {
				...input.model.operations,
				[operation.id]: {
					...operation,
					status: "prepared",
				},
			},
		};
		const publication_model =
			operation.kind === "route_revalidation"
				? grant_revalidation_publication_ownership(
						prepared_model,
						operation.id,
					)
				: prepared_model;
		if (!publication_model) {
			return undefined;
		}
		return advance_prepared_route({
			hooks_required: input.hooks_required,
			model: publication_model,
			operation_id: operation.id,
			prepared: input.outcome.prepared,
			view_transition_available: input.view_transition_available,
		});
	}

	const status = input.outcome.kind === "aborted" ? "aborted" : "failed";
	return stop_route_operation_without_publication({
		cause:
			input.outcome.kind === "failed" ? input.outcome.cause : undefined,
		model: input.model,
		operation,
		status,
	});
}

export function advance_route_hooks(
	input: RouteHooksTransitionInput,
): RouteHooksTransition | undefined {
	const operation = input.model.operations[input.outcome.operation_id];
	if (
		!is_route_operation(operation) ||
		input.outcome.kind === "stale" ||
		operation.status !== "hooks_running"
	) {
		return undefined;
	}
	if (!owns_route_completion(input.model, operation)) {
		return undefined;
	}
	if (input.outcome.kind === "publishable") {
		return advance_prepared_route({
			hooks_required: false,
			model: {
				...input.model,
				operations: {
					...input.model.operations,
					[operation.id]: {
						...operation,
						status: "publishable",
					},
				},
			},
			operation_id: operation.id,
			prepared: input.outcome.prepared,
			view_transition_available: input.view_transition_available,
		});
	}

	return stop_route_operation_without_publication({
		cause:
			input.outcome.kind === "failed" ? input.outcome.cause : undefined,
		model: input.model,
		operation,
		status: input.outcome.kind === "aborted" ? "aborted" : "failed",
	});
}

export function advance_prepared_route(
	input: PreparedRouteTransitionInput,
): PreparedRouteTransition | undefined {
	const operation = input.model.operations[input.operation_id];
	if (!is_route_operation(operation)) {
		return undefined;
	}
	if (!can_publish_route(input.model, operation.id)) {
		return undefined;
	}

	const readiness = plan_route_readiness({
		browser: input.model.browser,
		current: input.model.current,
		hooks_required: input.hooks_required,
		operation,
		prepared: input.prepared,
	});
	if (!readiness) {
		return undefined;
	}
	if (readiness.kind === "hooks_running") {
		return {
			effects: readiness.effects,
			kind: "hooks_running",
			model: {
				...input.model,
				operations: {
					...input.model.operations,
					[operation.id]: {
						...operation,
						status: "hooks_running",
					},
				},
			},
		};
	}

	const publishing_operation = {
		...operation,
		status: "publishing",
	} satisfies RoutePublicationOperation;
	const publication = plan_route_publication({
		browser: input.model.browser,
		operation: publishing_operation,
		prepared: readiness.prepared,
		previous: input.model.current,
	});
	if (!publication) {
		return undefined;
	}

	return {
		effects: publication_effects({
			operation: publishing_operation,
			transaction: publication.transaction,
			use_view_transitions: input.model.use_view_transitions,
			view_transition_available: input.view_transition_available,
		}),
		kind: "publishing",
		model: {
			...input.model,
			operations: {
				...input.model.operations,
				[operation.id]: publishing_operation,
			},
		},
	};
}

export function commit_route_publication(
	input: RoutePublicationCommitInput,
): CoreModel | undefined {
	const operation = input.model.operations[input.operation_id];
	if (!is_route_operation(operation)) {
		return undefined;
	}
	if (
		operation.status !== "publishing" ||
		!can_publish_route(input.model, operation.id)
	) {
		return undefined;
	}

	return {
		...input.model,
		active_route_operation_id:
			input.model.active_route_operation_id === operation.id
				? null
				: input.model.active_route_operation_id,
		browser: input.next.position,
		current: input.next,
		operations: {
			...input.model.operations,
			[operation.id]: {
				...operation,
				status: "published",
			},
		},
		publication_owner_id:
			input.model.publication_owner_id === operation.id
				? null
				: input.model.publication_owner_id,
	};
}

export function settle_route_publication(
	input: RoutePublicationSettlementInput,
): RoutePublicationSettlementPlan | undefined {
	const operation = input.model.operations[input.operation_id];
	if (!is_route_operation(operation) || operation.status !== "published") {
		return undefined;
	}

	const running_refresh = satisfiable_refresh_for_operation(
		input.model,
		operation,
	);
	const refresh = running_refresh
		? { kind: "idle" as const }
		: input.model.refresh;
	const settled_model: CoreModel = {
		...input.model,
		operations: {
			...input.model.operations,
			[operation.id]: {
				...operation,
				status: "settled",
			},
		},
		phase: operation.kind === "boot" ? "ready" : input.model.phase,
		refresh,
	};
	const effects: CoreEffect[] = [];
	if (running_refresh && operation.kind !== "route_revalidation") {
		append_resolve_public_calls(
			effects,
			running_refresh.demand.public_call_ids,
			{ ok: true },
		);
	}
	const refresh_start = begin_pending_refresh({ model: settled_model });
	if (refresh_start) {
		return {
			effects: effects.concat(refresh_start.effects),
			model: refresh_start.model,
		};
	}
	return {
		effects,
		model: settled_model,
	};
}

function grant_revalidation_publication_ownership(
	model: CoreModel,
	operation_id: OperationID,
): CoreModel | undefined {
	const operation = model.operations[operation_id];
	if (operation?.kind !== "route_revalidation") {
		return undefined;
	}
	if (
		model.publication_owner_id !== null ||
		!owns_route_completion(model, operation)
	) {
		return undefined;
	}
	return {
		...model,
		publication_owner_id: operation.id,
	};
}

function stop_route_operation_without_publication(input: {
	cause?: unknown;
	model: CoreModel;
	operation: RoutePublicationOperation;
	status: "aborted" | "completed" | "failed";
}): RouteStoppedTransition | undefined {
	if (input.operation.kind === "route_revalidation") {
		return undefined;
	}

	const effects: CoreEffect[] = [];
	if (input.status === "failed" && input.operation.kind === "boot") {
		append_reject_public_calls(
			effects,
			input.operation.public_call_ids,
			input.cause,
		);
	} else if (input.operation.kind !== "boot") {
		append_resolve_public_calls(effects, input.operation.public_call_ids, {
			didNavigate: false,
		});
	}
	return {
		effects,
		kind: input.status,
		model: {
			...input.model,
			active_route_operation_id:
				input.model.active_route_operation_id === input.operation.id
					? null
					: input.model.active_route_operation_id,
			operations: {
				...input.model.operations,
				[input.operation.id]: {
					...input.operation,
					status: input.status,
				},
			},
			publication_owner_id:
				input.model.publication_owner_id === input.operation.id
					? null
					: input.model.publication_owner_id,
		},
	};
}

function settle_route_revalidation_without_publication(input: {
	model: CoreModel;
	operation: Extract<
		RoutePublicationOperation,
		{ kind: "route_revalidation" }
	>;
	result:
		| { ok: true }
		| {
				ok: false;
				reason:
					| typeof REVALIDATION_BUILD_SKEW_REASON
					| typeof REVALIDATION_MAX_RETRIES_EXHAUSTED_REASON;
		  };
	status: "completed" | "failed" | "ignored_stale";
}): RouteStoppedTransition | undefined {
	const refresh = satisfiable_refresh_for_operation(
		input.model,
		input.operation,
	);
	if (!refresh) {
		return undefined;
	}

	const effects: CoreEffect[] = [];
	append_resolve_public_calls(
		effects,
		refresh.demand.public_call_ids,
		input.result,
	);
	return {
		effects,
		kind: input.status,
		model: {
			...input.model,
			operations: {
				...input.model.operations,
				[input.operation.id]: {
					...input.operation,
					status: input.status,
				},
			},
			publication_owner_id:
				input.model.publication_owner_id === input.operation.id
					? null
					: input.model.publication_owner_id,
			refresh: { kind: "idle" },
		},
	};
}

function prepend_effect<T extends { effects: readonly CoreEffect[] }>(
	transition: T | undefined,
	effect: CoreEffect | undefined,
): T | undefined {
	if (!transition || !effect) {
		return transition;
	}
	return {
		...transition,
		effects: [effect, ...transition.effects],
	};
}
