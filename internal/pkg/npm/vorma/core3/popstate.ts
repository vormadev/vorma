import type {
	BrowserPosition,
	CoreEffect,
	CoreModel,
	OperationID,
	PopstateOperation,
	ScrollState,
} from "./model.ts";

export type RoutePopstateStartInput = {
	browser: BrowserPosition;
	leaving_scroll: ScrollState;
	model: CoreModel;
	operation_id: OperationID;
	popstate_scroll?: ScrollState;
};

export type RoutePopstateStartPlan = {
	effects: readonly CoreEffect[];
	model: CoreModel;
};

export function begin_route_popstate(
	input: RoutePopstateStartInput,
): RoutePopstateStartPlan | undefined {
	if (
		input.model.phase !== "ready" ||
		input.model.active_route_operation_id !== null ||
		!input.model.browser ||
		!input.model.current ||
		input.model.operations[input.operation_id] ||
		input.model.publication_owner_id !== null
	) {
		return undefined;
	}
	if (
		input.model.browser.href === input.browser.href &&
		input.model.browser.key === input.browser.key
	) {
		return undefined;
	}

	const operation = {
		browser: input.browser,
		id: input.operation_id,
		kind: "popstate",
		leaving_scroll: input.leaving_scroll,
		...(input.popstate_scroll !== undefined
			? { popstate_scroll: input.popstate_scroll }
			: {}),
		public_call_ids: [],
		rights: ["abort_effects", "own_visible_transition", "publish_route"],
		status: "started",
	} satisfies PopstateOperation;

	return {
		effects: [
			{
				client_build_id: input.model.client_build_id,
				href: operation.browser.href,
				operation_id: operation.id,
				trigger: "popstate",
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
