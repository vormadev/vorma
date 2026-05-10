import { parse_href } from "../href.ts";
import type {
	CoreEffect,
	CoreModel,
	CoreOperation,
	OperationBase,
	OperationID,
	PreparedRoute,
	PublicationTransaction,
	RouteSnapshot,
} from "../model.ts";
import { can_publish_route } from "../operation.ts";
import {
	publication_effects,
	route_publication_commit,
} from "../publication.ts";
import { prepared_route_at_position } from "../route.ts";

export type HMRUpdateOperation = OperationBase & {
	kind: "hmr_update";
	module_url: string;
	pattern: string;
	rerun_client_loader: boolean;
};

export type HMRDevOperation = CoreOperation | HMRUpdateOperation;

export type HMRDevModel = Omit<CoreModel, "operations"> & {
	hmr_operation_id: OperationID | null;
	operations: Readonly<Record<OperationID, HMRDevOperation>>;
};

export type HMRPrepareRouteEffect = {
	module_url: string;
	operation_id: OperationID;
	pattern: string;
	rerun_client_loader: boolean;
	type: "prepare_hmr_route";
};

export type HMRDevEffect = CoreEffect | HMRPrepareRouteEffect;

export type HMRUpdateStartInput = {
	model: HMRDevModel;
	module_url: string;
	operation_id: OperationID;
	rerun_client_loader_patterns: readonly string[];
};

export type HMRUpdateStartPlan = {
	effects: readonly HMRDevEffect[];
	model: HMRDevModel;
};

export type HMRPreparationOutcome =
	| {
			kind: "stale";
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
			prepared: PreparedRoute;
	  };

export type HMRPreparationInput = {
	model: HMRDevModel;
	outcome: HMRPreparationOutcome;
	view_transition_available: boolean;
};

export type HMRPreparationTransition =
	| {
			effects: readonly CoreEffect[];
			kind: "publishing";
			model: HMRDevModel;
	  }
	| {
			effects: readonly CoreEffect[];
			kind: "failed";
			model: HMRDevModel;
	  };

export type HMRPublicationCommitInput = {
	model: HMRDevModel;
	next: RouteSnapshot;
	operation_id: OperationID;
};

export type HMRPublicationSettlementInput = {
	model: HMRDevModel;
	operation_id: OperationID;
};

export function begin_hmr_update(
	input: HMRUpdateStartInput,
): HMRUpdateStartPlan | undefined {
	if (!import.meta.env.DEV) {
		return undefined;
	}

	const match = current_hmr_match(input.model, input.module_url);
	if (
		!match ||
		input.model.operations[input.operation_id] ||
		input.model.active_route_operation_id !== null ||
		input.model.publication_owner_id !== null
	) {
		return undefined;
	}

	const previous_operation = current_hmr_operation(input.model);
	const operation = {
		id: input.operation_id,
		kind: "hmr_update",
		module_url: match.module_url,
		pattern: match.pattern,
		public_call_ids: [],
		rerun_client_loader: input.rerun_client_loader_patterns.includes(
			match.pattern,
		),
		rights: ["abort_effects", "publish_route"],
		status: "started",
	} satisfies HMRUpdateOperation;
	const effects: HMRDevEffect[] = [];
	const operations = { ...input.model.operations };
	if (previous_operation) {
		if (previous_operation.rights.includes("abort_effects")) {
			effects.push({
				operation_id: previous_operation.id,
				type: "abort_operation",
			});
		}
		operations[previous_operation.id] = {
			...previous_operation,
			status: "superseded",
		};
	}
	effects.push({
		module_url: operation.module_url,
		operation_id: operation.id,
		pattern: operation.pattern,
		rerun_client_loader: operation.rerun_client_loader,
		type: "prepare_hmr_route",
	});

	return {
		effects,
		model: {
			...input.model,
			hmr_operation_id: operation.id,
			operations: {
				...operations,
				[operation.id]: operation,
			},
			publication_owner_id: operation.id,
		},
	};
}

export function advance_hmr_preparation(
	input: HMRPreparationInput,
): HMRPreparationTransition | undefined {
	const operation = input.model.operations[input.outcome.operation_id];
	if (
		operation?.kind !== "hmr_update" ||
		input.model.hmr_operation_id !== operation.id ||
		input.outcome.kind === "stale"
	) {
		return undefined;
	}

	if (input.outcome.kind === "failed") {
		return {
			effects: [],
			kind: "failed",
			model: {
				...input.model,
				hmr_operation_id: null,
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
			},
		};
	}

	if (!can_publish_route(input.model, operation.id)) {
		return undefined;
	}
	const publishing_operation = {
		...operation,
		status: "publishing",
	} satisfies HMRUpdateOperation;
	const publication = plan_hmr_publication({
		model: input.model,
		operation: publishing_operation,
		prepared: input.outcome.prepared,
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

export function commit_hmr_publication(
	input: HMRPublicationCommitInput,
): HMRDevModel | undefined {
	const operation = input.model.operations[input.operation_id];
	if (
		operation?.kind !== "hmr_update" ||
		operation.status !== "publishing" ||
		!can_publish_route(input.model, operation.id)
	) {
		return undefined;
	}
	return {
		...input.model,
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

export function settle_hmr_publication(
	input: HMRPublicationSettlementInput,
): HMRDevModel | undefined {
	const operation = input.model.operations[input.operation_id];
	if (operation?.kind !== "hmr_update" || operation.status !== "published") {
		return undefined;
	}
	return {
		...input.model,
		hmr_operation_id:
			input.model.hmr_operation_id === operation.id
				? null
				: input.model.hmr_operation_id,
		operations: {
			...input.model.operations,
			[operation.id]: {
				...operation,
				status: "settled",
			},
		},
	};
}

function plan_hmr_publication(input: {
	model: HMRDevModel;
	operation: HMRUpdateOperation;
	prepared: PreparedRoute;
}): { next: RouteSnapshot; transaction: PublicationTransaction } | undefined {
	if (!input.model.browser || !input.model.current) {
		return undefined;
	}
	if (
		input.model.current.position.href !== input.model.browser.href ||
		input.model.current.position.key !== input.model.browser.key
	) {
		return undefined;
	}

	const prepared = prepared_route_at_position(
		input.prepared,
		input.model.browser,
	);
	const next = {
		position: input.model.browser,
		provisional: false,
		render: prepared.render,
		route: prepared.route,
	} satisfies RouteSnapshot;
	const inside_transition: CoreEffect[] = [
		{
			prepared,
			type: "apply_publication_dom",
		},
		{
			commit: route_publication_commit({
				previous: input.model.current,
				reason: "revalidation",
				render: prepared.render,
				route: prepared.route,
			}),
			next,
			operation_id: input.operation.id,
			type: "commit",
		},
		{ type: "render" },
	];

	return {
		next,
		transaction: {
			after_transition: [],
			before_transition: [],
			inside_transition,
			operation_id: input.operation.id,
			reason: "revalidation",
		},
	};
}

function current_hmr_match(
	model: HMRDevModel,
	module_url: string,
): { module_url: string; pattern: string } | undefined {
	if (model.phase !== "ready" || !model.browser || !model.current) {
		return undefined;
	}
	const normalized_module_url = normalized_hmr_module_url(
		module_url,
		model.browser.href,
	);
	if (!normalized_module_url) {
		return undefined;
	}
	for (const match of model.current.route.matches) {
		if (
			normalized_hmr_module_url(match.module_url, model.browser.href) ===
			normalized_module_url
		) {
			return {
				module_url: normalized_module_url,
				pattern: match.pattern,
			};
		}
	}
	return undefined;
}

function current_hmr_operation(
	model: HMRDevModel,
): HMRUpdateOperation | undefined {
	const operation =
		model.hmr_operation_id === null
			? undefined
			: model.operations[model.hmr_operation_id];
	return operation?.kind === "hmr_update" ? operation : undefined;
}

function normalized_hmr_module_url(
	module_url: string,
	base_href: string,
): string | undefined {
	return parse_href(module_url, base_href)?.href;
}
