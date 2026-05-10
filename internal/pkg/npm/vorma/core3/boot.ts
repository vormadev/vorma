import type {
	BootOperation,
	BrowserPosition,
	CoreEffect,
	CoreModel,
	OperationID,
	PublicCallID,
	RouteFacts,
	RouteRenderFacts,
	RouteSnapshot,
	ScrollState,
} from "./model.ts";
import {
	route_facts_at_position,
	route_render_facts_at_position,
} from "./route.ts";

export type BootStartInput = {
	browser: BrowserPosition;
	model: CoreModel;
	operation_id: OperationID;
	public_call_ids: readonly PublicCallID[];
	reload_scroll?: ScrollState;
};

export type BootStartPlan = {
	effects: readonly CoreEffect[];
	model: CoreModel;
};

export type BootPayloadCompletionInput = {
	client_build_id: string;
	deployment_id: string;
	model: CoreModel;
	operation_id: OperationID;
	payload: unknown;
	render: RouteRenderFacts;
	route: RouteFacts;
};

export type BootPayloadCompletionPlan = {
	effects: readonly CoreEffect[];
	model: CoreModel;
};

export type BootProvisionalSnapshotInput = {
	browser: BrowserPosition;
	operation: BootOperation;
	render: RouteRenderFacts;
	route: RouteFacts;
};

export function begin_boot(input: BootStartInput): BootStartPlan | undefined {
	if (
		input.model.phase !== "unbooted" ||
		input.model.active_route_operation_id !== null ||
		input.model.browser !== null ||
		input.model.current !== null ||
		input.model.operations[input.operation_id] ||
		input.model.publication_owner_id !== null
	) {
		return undefined;
	}

	const operation = {
		browser_key: input.browser.key,
		href: input.browser.href,
		id: input.operation_id,
		kind: "boot",
		public_call_ids: input.public_call_ids,
		...(input.reload_scroll !== undefined
			? { reload_scroll: input.reload_scroll }
			: {}),
		rights: ["publish_route"],
		state: input.browser.state,
		status: "started",
	} satisfies BootOperation;

	return {
		effects: [
			{
				fallback_browser_key: input.browser.key,
				operation_id: operation.id,
				type: "read_boot_payload",
			},
		],
		model: {
			...input.model,
			active_route_operation_id: operation.id,
			browser: input.browser,
			operations: {
				...input.model.operations,
				[operation.id]: operation,
			},
			phase: "booting",
			publication_owner_id: operation.id,
		},
	};
}

export function complete_boot_payload(
	input: BootPayloadCompletionInput,
): BootPayloadCompletionPlan | undefined {
	const operation = input.model.operations[input.operation_id];
	if (
		operation?.kind !== "boot" ||
		operation.status !== "started" ||
		input.model.active_route_operation_id !== operation.id ||
		input.model.publication_owner_id !== operation.id
	) {
		return undefined;
	}

	const browser = {
		href: operation.href,
		key: operation.browser_key,
		state: operation.state,
	};
	const current = boot_provisional_snapshot({
		browser,
		operation,
		render: input.render,
		route: input.route,
	});
	if (!current) {
		return undefined;
	}

	return {
		effects: [
			{
				client_build_id: input.client_build_id,
				history_state: operation.state,
				href: operation.href,
				operation_id: operation.id,
				payload: input.payload,
				trigger: "boot",
				type: "prepare_route",
			},
		],
		model: {
			...input.model,
			browser,
			client_build_id: input.client_build_id,
			current,
			deployment_id: input.deployment_id,
			operations: {
				...input.model.operations,
				[operation.id]: {
					...operation,
					status: "classified",
				},
			},
		},
	};
}

export function boot_provisional_snapshot(
	input: BootProvisionalSnapshotInput,
): RouteSnapshot | undefined {
	if (!input.operation.rights.includes("publish_route")) {
		return undefined;
	}
	return {
		position: input.browser,
		provisional: true,
		render: route_render_facts_at_position(input.render, input.browser),
		route: route_facts_at_position(input.route, input.browser),
	};
}
