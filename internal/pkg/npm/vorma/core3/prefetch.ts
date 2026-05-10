import { route_build_skew_effect } from "./build_skew.ts";
import { parse_href, route_hrefs_share_document } from "./href.ts";
import type {
	BuildSkewReport,
	CoreEffect,
	CoreModel,
	OperationID,
	ResourceKey,
	RouteOutcome,
	RoutePrefetchOperation,
} from "./model.ts";

export type RoutePrefetchStartInput = {
	href: string;
	model: CoreModel;
	operation_id: OperationID;
};

export type RoutePrefetchCancelInput = {
	href: string;
	model: CoreModel;
};

export type RoutePrefetchTransition = {
	effects: readonly CoreEffect[];
	model: CoreModel;
};

export type RoutePrefetchOutcomeInput = {
	build_skew_report?: BuildSkewReport;
	model: CoreModel;
	outcome: RouteOutcome;
};

export type RoutePrefetchOutcomeTransition =
	| {
			effects: readonly CoreEffect[];
			kind: "preparing";
			model: CoreModel;
	  }
	| {
			effects: readonly CoreEffect[];
			kind: "completed" | "failed";
			model: CoreModel;
	  };

export type PrefetchPreparationOutcome =
	| {
			kind: "stale";
			operation_id: OperationID;
	  }
	| {
			kind: "aborted";
			operation_id: OperationID;
	  }
	| {
			cause: unknown;
			kind: "failed";
			operation_id: OperationID;
	  }
	| {
			kind: "prepared";
			operation_id: OperationID;
			prepared_resource_key: ResourceKey;
	  };

export type RoutePrefetchPreparationInput = {
	model: CoreModel;
	outcome: PrefetchPreparationOutcome;
};

export type RoutePrefetchPreparationTransition = {
	effects: readonly CoreEffect[];
	kind: "aborted" | "failed" | "prepared";
	model: CoreModel;
};

export function begin_route_prefetch(
	input: RoutePrefetchStartInput,
): RoutePrefetchTransition | undefined {
	const href = prefetch_target_href(input.model, input.href);
	if (!href || input.model.operations[input.operation_id]) {
		return undefined;
	}

	const active_operation =
		input.model.active_route_operation_id === null
			? undefined
			: input.model.operations[input.model.active_route_operation_id];
	if (
		active_operation?.kind === "navigation" &&
		route_hrefs_share_document(active_operation.href, href)
	) {
		return undefined;
	}

	const previous_operation = current_prefetch_operation(input.model);
	if (previous_operation) {
		if (route_hrefs_share_document(previous_operation.href, href)) {
			return undefined;
		}
	}

	const operation = {
		href,
		id: input.operation_id,
		kind: "route_prefetch",
		public_call_ids: [],
		rights: ["abort_effects"],
		status: "started",
	} satisfies RoutePrefetchOperation;

	const canceled = previous_operation
		? abort_prefetch(input.model, previous_operation)
		: { effects: [], model: input.model };
	return {
		effects: canceled.effects.concat({
			client_build_id: canceled.model.client_build_id,
			href: operation.href,
			operation_id: operation.id,
			trigger: "prefetch",
			type: "fetch_route",
		}),
		model: {
			...canceled.model,
			operations: {
				...canceled.model.operations,
				[operation.id]: operation,
			},
			prefetch_operation_id: operation.id,
		},
	};
}

export function cancel_route_prefetch(
	input: RoutePrefetchCancelInput,
): RoutePrefetchTransition | undefined {
	const href = prefetch_target_href(input.model, input.href);
	const operation = current_prefetch_operation(input.model);
	if (
		!href ||
		!operation ||
		!route_hrefs_share_document(operation.href, href)
	) {
		return undefined;
	}
	return abort_prefetch(input.model, operation);
}

export function advance_route_prefetch_outcome(
	input: RoutePrefetchOutcomeInput,
): RoutePrefetchOutcomeTransition | undefined {
	const operation = input.model.operations[input.outcome.operation_id];
	if (
		operation?.kind !== "route_prefetch" ||
		input.model.prefetch_operation_id !== operation.id ||
		input.outcome.kind === "stale"
	) {
		return undefined;
	}

	const build_skew_effect = route_build_skew_effect({
		model: input.model,
		operation,
		report: input.build_skew_report,
	});
	if (input.outcome.kind === "route_data") {
		const transition: RoutePrefetchOutcomeTransition = {
			effects: [
				{
					client_build_id: input.model.client_build_id,
					history_state: undefined,
					href: operation.href,
					operation_id: operation.id,
					payload: input.outcome.payload,
					trigger: "prefetch",
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
		};
		return prepend_prefetch_effect(transition, build_skew_effect);
	}

	const status =
		input.outcome.kind === "route_error" ||
		input.outcome.kind === "invalid_redirect" ||
		input.outcome.kind === "redirect_loop"
			? "failed"
			: "completed";
	return prepend_prefetch_effect(
		complete_prefetch_without_resource(input.model, operation, status),
		build_skew_effect,
	);
}

export function advance_route_prefetch_preparation(
	input: RoutePrefetchPreparationInput,
): RoutePrefetchPreparationTransition | undefined {
	const operation = input.model.operations[input.outcome.operation_id];
	if (
		operation?.kind !== "route_prefetch" ||
		input.model.prefetch_operation_id !== operation.id ||
		input.outcome.kind === "stale"
	) {
		return undefined;
	}

	if (input.outcome.kind === "prepared") {
		return {
			effects: [],
			kind: "prepared",
			model: {
				...input.model,
				operations: {
					...input.model.operations,
					[operation.id]: {
						...operation,
						prepared_resource_key:
							input.outcome.prepared_resource_key,
						status: "prepared",
					},
				},
			},
		};
	}

	const status = input.outcome.kind === "aborted" ? "aborted" : "failed";
	return {
		effects: [],
		kind: status,
		model: {
			...input.model,
			operations: {
				...input.model.operations,
				[operation.id]: {
					...operation,
					status,
				},
			},
			prefetch_operation_id: null,
		},
	};
}

function prefetch_target_href(
	model: CoreModel,
	href: string,
): string | undefined {
	if (
		model.phase !== "ready" ||
		!model.browser ||
		!model.current ||
		!model.current.route.href
	) {
		return undefined;
	}
	const browser_url = parse_href(model.browser.href);
	if (!browser_url) {
		return undefined;
	}
	const target_url = parse_href(href, browser_url.href);
	if (!target_url || target_url.origin !== browser_url.origin) {
		return undefined;
	}
	if (route_hrefs_share_document(model.current.route.href, target_url.href)) {
		return undefined;
	}
	return target_url.href;
}

function current_prefetch_operation(
	model: CoreModel,
): RoutePrefetchOperation | undefined {
	const operation =
		model.prefetch_operation_id === null
			? undefined
			: model.operations[model.prefetch_operation_id];
	return operation?.kind === "route_prefetch" ? operation : undefined;
}

function abort_prefetch(
	model: CoreModel,
	operation: RoutePrefetchOperation,
): RoutePrefetchTransition {
	return {
		effects: operation.rights.includes("abort_effects")
			? [
					{
						operation_id: operation.id,
						type: "abort_operation",
					},
				]
			: [],
		model: {
			...model,
			operations: {
				...model.operations,
				[operation.id]: {
					...operation,
					status: "aborted",
				},
			},
			prefetch_operation_id: null,
		},
	};
}

function complete_prefetch_without_resource(
	model: CoreModel,
	operation: RoutePrefetchOperation,
	status: "completed" | "failed",
): RoutePrefetchOutcomeTransition {
	return {
		effects: [],
		kind: status,
		model: {
			...model,
			operations: {
				...model.operations,
				[operation.id]: {
					...operation,
					status,
				},
			},
			prefetch_operation_id: null,
		},
	};
}

function prepend_prefetch_effect<T extends { effects: readonly CoreEffect[] }>(
	transition: T,
	effect: CoreEffect | undefined,
): T {
	if (!effect) {
		return transition;
	}
	return {
		...transition,
		effects: [effect, ...transition.effects],
	};
}
