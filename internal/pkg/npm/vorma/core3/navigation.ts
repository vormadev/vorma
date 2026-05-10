import { route_hrefs_share_document } from "./href.ts";
import type {
	BrowserKey,
	CoreEffect,
	CoreModel,
	NavigationOperation,
	NavigationRequestOutcome,
	OperationID,
	PublicCallID,
} from "./model.ts";
import {
	append_resolve_public_calls,
	plan_same_document_route,
	publication_effects,
} from "./publication.ts";

export type RouteNavigationStartInput = {
	browser_key: BrowserKey;
	href: string;
	model: CoreModel;
	operation_id: OperationID;
	public_call_ids: readonly PublicCallID[];
	replace: boolean;
	scroll_to_top?: boolean;
	skip_work_indicator: boolean;
	state: unknown;
};

export type RouteNavigationStartPlan = {
	effects: readonly CoreEffect[];
	model: CoreModel;
};

export type NavigationRequestTransition =
	| {
			effects: readonly CoreEffect[];
			kind: "hard_redirect" | "merged" | "promoted" | "started";
			model: CoreModel;
	  }
	| {
			effects: readonly CoreEffect[];
			kind: "publishing" | "scroll_only";
			model: CoreModel;
	  };

export type NavigationRequestTransitionInput = {
	browser_key: BrowserKey;
	model: CoreModel;
	operation_id: OperationID;
	outcome: NavigationRequestOutcome;
	public_call_ids: readonly PublicCallID[];
	replace: boolean;
	scroll_to_top?: boolean;
	skip_work_indicator: boolean;
	state: unknown;
	view_transition_available: boolean;
};

export function begin_route_navigation(
	input: RouteNavigationStartInput,
): RouteNavigationStartPlan | undefined {
	if (
		input.model.phase !== "ready" ||
		!input.model.browser ||
		!input.model.current ||
		input.model.active_route_operation_id !== null ||
		input.model.publication_owner_id !== null ||
		input.model.operations[input.operation_id]
	) {
		return undefined;
	}

	const operation = navigation_operation({
		browser_key: input.browser_key,
		href: input.href,
		operation_id: input.operation_id,
		public_call_ids: input.public_call_ids,
		replace: input.replace,
		scroll_to_top: input.scroll_to_top,
		skip_work_indicator: input.skip_work_indicator,
		state: input.state,
		status: "started",
	});

	return {
		effects: [
			{
				client_build_id: input.model.client_build_id,
				href: operation.href,
				operation_id: operation.id,
				trigger: "navigation",
				type: "fetch_route",
			},
		],
		model: {
			...input.model,
			active_route_operation_id: operation.id,
			operations: {
				...input.model.operations,
				[operation.id]: operation,
			},
			publication_owner_id: operation.id,
		},
	};
}

export function advance_navigation_request(
	input: NavigationRequestTransitionInput,
): NavigationRequestTransition | undefined {
	if (
		input.model.phase !== "ready" ||
		!input.model.browser ||
		!input.model.current ||
		input.outcome.kind === "invalid_href"
	) {
		return undefined;
	}

	if (input.outcome.kind === "hard_redirect") {
		const effects: CoreEffect[] = [
			{
				href: input.outcome.href,
				type: "hard_redirect",
			},
		];
		append_resolve_public_calls(effects, input.public_call_ids, {
			didNavigate: false,
		});
		return {
			effects,
			kind: "hard_redirect",
			model: input.model,
		};
	}

	const merged = merge_active_navigation(input);
	if (merged) {
		return merged;
	}

	const cleared = supersede_active_route_operation(input.model);
	if (input.outcome.kind === "same_document") {
		return advance_same_document_navigation(input, cleared);
	}

	const promoted = promote_matching_prefetch(input, cleared);
	if (promoted) {
		return promoted;
	}

	if (cleared.model.operations[input.operation_id]) {
		return undefined;
	}
	const started = begin_route_navigation({
		browser_key: input.browser_key,
		href: input.outcome.href,
		model: cleared.model,
		operation_id: input.operation_id,
		public_call_ids: input.public_call_ids,
		replace: input.replace,
		scroll_to_top: input.scroll_to_top,
		skip_work_indicator: input.skip_work_indicator,
		state: input.state,
	});
	if (!started) {
		return undefined;
	}
	return {
		effects: cleared.effects.concat(started.effects),
		kind: "started",
		model: started.model,
	};
}

function advance_same_document_navigation(
	input: NavigationRequestTransitionInput,
	cleared: RouteNavigationStartPlan,
): NavigationRequestTransition | undefined {
	if (cleared.model.operations[input.operation_id]) {
		return undefined;
	}
	const operation = navigation_operation({
		browser_key: input.browser_key,
		href: input.outcome.href,
		operation_id: input.operation_id,
		public_call_ids: input.public_call_ids,
		replace: input.replace,
		scroll_to_top: input.scroll_to_top,
		skip_work_indicator: input.skip_work_indicator,
		state: input.state,
		status: "publishing",
	});
	const plan = plan_same_document_route({
		operation,
		previous: cleared.model.current,
	});
	if (!plan) {
		return undefined;
	}
	if (plan.kind === "scroll_only") {
		return {
			effects: cleared.effects.concat(plan.effects),
			kind: "scroll_only",
			model: cleared.model,
		};
	}
	return {
		effects: cleared.effects.concat(
			publication_effects({
				operation,
				transaction: plan.transaction,
				use_view_transitions: cleared.model.use_view_transitions,
				view_transition_available: input.view_transition_available,
			}),
		),
		kind: "publishing",
		model: {
			...cleared.model,
			active_route_operation_id: operation.id,
			operations: {
				...cleared.model.operations,
				[operation.id]: operation,
			},
			publication_owner_id: operation.id,
		},
	};
}

function merge_active_navigation(
	input: NavigationRequestTransitionInput,
): NavigationRequestTransition | undefined {
	const active_operation =
		input.model.active_route_operation_id === null
			? undefined
			: input.model.operations[input.model.active_route_operation_id];
	if (
		input.outcome.kind !== "route_navigation" ||
		active_operation?.kind !== "navigation" ||
		!route_hrefs_share_document(active_operation.href, input.outcome.href)
	) {
		return undefined;
	}
	return {
		effects: [],
		kind: "merged",
		model: {
			...input.model,
			operations: {
				...input.model.operations,
				[active_operation.id]: {
					...active_operation,
					browser_key: input.browser_key,
					href: input.outcome.href,
					public_call_ids: active_operation.public_call_ids.concat(
						input.public_call_ids,
					),
					replace: input.replace,
					scroll_to_top: input.scroll_to_top,
					skip_work_indicator:
						active_operation.skip_work_indicator &&
						input.skip_work_indicator,
					state: input.state,
				},
			},
		},
	};
}

function promote_matching_prefetch(
	input: NavigationRequestTransitionInput,
	cleared: RouteNavigationStartPlan,
): NavigationRequestTransition | undefined {
	const prefetch =
		cleared.model.prefetch_operation_id === null
			? undefined
			: cleared.model.operations[cleared.model.prefetch_operation_id];
	if (
		prefetch?.kind !== "route_prefetch" ||
		!route_hrefs_share_document(prefetch.href, input.outcome.href) ||
		(prefetch.status !== "started" &&
			prefetch.status !== "classified" &&
			prefetch.status !== "prepared")
	) {
		return undefined;
	}
	if (prefetch.status === "prepared" && !prefetch.prepared_resource_key) {
		return undefined;
	}

	const status =
		prefetch.status === "prepared" ? "classified" : prefetch.status;
	const operation = navigation_operation({
		browser_key: input.browser_key,
		href: input.outcome.href,
		operation_id: prefetch.id,
		public_call_ids: input.public_call_ids,
		replace: input.replace,
		scroll_to_top: input.scroll_to_top,
		skip_work_indicator: input.skip_work_indicator,
		state: input.state,
		status,
	});
	const effects = cleared.effects.slice();
	if (prefetch.status === "prepared" && prefetch.prepared_resource_key) {
		effects.push({
			client_build_id: cleared.model.client_build_id,
			history_state: operation.state,
			href: operation.href,
			operation_id: operation.id,
			resource_key: prefetch.prepared_resource_key,
			type: "promote_prefetch_route",
		});
	}
	return {
		effects,
		kind: "promoted",
		model: {
			...cleared.model,
			active_route_operation_id: operation.id,
			operations: {
				...cleared.model.operations,
				[operation.id]: operation,
			},
			prefetch_operation_id: null,
			publication_owner_id: operation.id,
		},
	};
}

function supersede_active_route_operation(
	model: CoreModel,
): RouteNavigationStartPlan {
	const active_operation =
		model.active_route_operation_id === null
			? undefined
			: model.operations[model.active_route_operation_id];
	if (!active_operation) {
		return { effects: [], model };
	}

	const effects: CoreEffect[] = [];
	if (active_operation.rights.includes("abort_effects")) {
		effects.push({
			operation_id: active_operation.id,
			type: "abort_operation",
		});
	}
	if (active_operation.kind !== "boot") {
		append_resolve_public_calls(effects, active_operation.public_call_ids, {
			didNavigate: false,
		});
	}
	return {
		effects,
		model: {
			...model,
			active_route_operation_id: null,
			operations: {
				...model.operations,
				[active_operation.id]: {
					...active_operation,
					status: "superseded",
				},
			},
			publication_owner_id:
				model.publication_owner_id === active_operation.id
					? null
					: model.publication_owner_id,
		},
	};
}

function navigation_operation(input: {
	browser_key: BrowserKey;
	href: string;
	operation_id: OperationID;
	public_call_ids: readonly PublicCallID[];
	replace: boolean;
	scroll_to_top?: boolean;
	skip_work_indicator: boolean;
	state: unknown;
	status: NavigationOperation["status"];
}): NavigationOperation {
	return {
		browser_key: input.browser_key,
		href: input.href,
		id: input.operation_id,
		kind: "navigation",
		public_call_ids: input.public_call_ids,
		redirect_count: 0,
		replace: input.replace,
		rights: ["abort_effects", "own_visible_transition", "publish_route"],
		source: "navigate",
		state: input.state,
		status: input.status,
		skip_work_indicator: input.skip_work_indicator,
		...(input.scroll_to_top === undefined
			? {}
			: { scroll_to_top: input.scroll_to_top }),
	};
}
