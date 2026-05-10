import { jsonDeepEquals } from "vorma/kit/json";
import { parse_href, route_hrefs_share_document } from "./href.ts";
import type {
	BrowserPosition,
	CoreEffect,
	CoreOperation,
	OperationBase,
	PreparedRoute,
	PublicationReason,
	PublicationTransaction,
	PublicCallID,
	RouteCommit,
	RouteFacts,
	RouteRenderFacts,
	RouteSnapshot,
	RouteState,
	ScrollIntent,
	ScrollState,
} from "./model.ts";
import {
	prepared_route_at_position,
	route_facts_at_position,
	route_facts_to_state,
	route_render_facts_at_position,
} from "./route.ts";

export type RoutePublicationOperation = Extract<
	CoreOperation,
	{
		kind: "boot" | "navigation" | "popstate" | "route_revalidation";
	}
>;

export type RoutePublicationTargetInput = {
	browser: BrowserPosition | null;
	operation: RoutePublicationOperation;
};

export type RoutePublicationTarget = {
	position: BrowserPosition;
	reason: PublicationReason;
};

export type RoutePublicationInput = {
	browser: BrowserPosition | null;
	operation: RoutePublicationOperation;
	prepared: PreparedRoute;
	previous: RouteSnapshot | null;
};

export type RoutePublicationPlan = {
	next: RouteSnapshot;
	transaction: PublicationTransaction;
};

export type PublicationEffectsInput = {
	operation: Pick<OperationBase, "rights">;
	transaction: PublicationTransaction;
	use_view_transitions: boolean;
	view_transition_available: boolean;
};

export type SameDocumentPublicationOperation = Extract<
	CoreOperation,
	{ kind: "navigation" | "popstate" }
>;

export type SameDocumentPublicationInput = {
	operation: SameDocumentPublicationOperation;
	previous: RouteSnapshot | null;
};

export type SameDocumentPublicationPlan = RoutePublicationPlan & {
	did_navigate: boolean;
	kind: "publication";
};

export type SameDocumentScrollOnlyPlan = {
	did_navigate: false;
	effects: readonly CoreEffect[];
	kind: "scroll_only";
	next: RouteSnapshot;
};

export type SameDocumentPlan =
	| SameDocumentPublicationPlan
	| SameDocumentScrollOnlyPlan;

export function append_resolve_public_calls(
	effects: CoreEffect[],
	public_call_ids: readonly PublicCallID[],
	result: unknown,
): void {
	for (const public_call_id of public_call_ids) {
		effects.push({
			public_call_id,
			result,
			type: "resolve_public_call",
		});
	}
}

export function append_reject_public_calls(
	effects: CoreEffect[],
	public_call_ids: readonly PublicCallID[],
	cause: unknown,
): void {
	for (const public_call_id of public_call_ids) {
		effects.push({
			cause,
			public_call_id,
			type: "reject_public_call",
		});
	}
}

export function route_publication_target(
	input: RoutePublicationTargetInput,
): RoutePublicationTarget | undefined {
	if (input.operation.kind === "boot") {
		return {
			position: {
				href: input.operation.href,
				key: input.operation.browser_key,
				state: input.operation.state,
			},
			reason: "boot",
		};
	}
	if (input.operation.kind === "navigation") {
		return {
			position: {
				href: input.operation.href,
				key: input.operation.browser_key,
				state: input.operation.state,
			},
			reason: "navigation",
		};
	}
	if (input.operation.kind === "popstate") {
		return {
			position: input.operation.browser,
			reason: "popstate",
		};
	}
	if (!input.browser) {
		return undefined;
	}
	return {
		position: input.browser,
		reason: "revalidation",
	};
}

export function route_publication_commit(input: {
	previous: RouteSnapshot | null;
	reason: PublicationReason;
	render: RouteRenderFacts;
	route: RouteFacts;
	scroll_intent?: ScrollIntent;
}): RouteCommit {
	const previous_route =
		input.previous && !input.previous.provisional
			? route_facts_to_state(input.previous.route)
			: null;
	const route = route_facts_to_state(input.route);
	const commit: RouteCommit = {
		route_render: {
			scroll_intent: input.scroll_intent,
			state: input.render,
		},
	};
	if (route_update_required(previous_route, route)) {
		commit.route_update = {
			previous_route,
			reason: input.reason,
			route,
		};
	}
	return commit;
}

export function route_update_required(
	previous: RouteState | null,
	next: RouteState,
): boolean {
	if (!previous) {
		return true;
	}
	return !jsonDeepEquals(previous, next);
}

export function publication_effects(
	input: PublicationEffectsInput,
): readonly CoreEffect[] {
	if (
		input.use_view_transitions &&
		input.view_transition_available &&
		input.operation.rights.includes("own_visible_transition")
	) {
		return [
			{
				transaction: input.transaction,
				type: "run_view_transition",
			},
		];
	}
	return input.transaction.before_transition.concat(
		input.transaction.inside_transition,
		input.transaction.after_transition,
	);
}

export function plan_route_publication(
	input: RoutePublicationInput,
): RoutePublicationPlan | undefined {
	if (!input.operation.rights.includes("publish_route")) {
		return undefined;
	}

	const target = route_publication_target(input);
	if (!target) {
		return undefined;
	}

	const prepared = prepared_route_at_position(
		input.prepared,
		target.position,
	);
	const scroll_intent = derive_scroll_intent(input.operation, prepared.route);
	const next = {
		position: target.position,
		provisional: false,
		render: prepared.render,
		route: prepared.route,
		scroll_intent,
	};
	const inside_transition: CoreEffect[] = [];
	if (input.operation.kind === "navigation") {
		inside_transition.push({
			position: target.position,
			replace: input.operation.replace,
			type: "write_history",
		});
	}
	inside_transition.push(
		{
			prepared,
			type: "apply_publication_dom",
		},
		{
			commit: route_publication_commit({
				previous: input.previous,
				reason: target.reason,
				render: prepared.render,
				route: prepared.route,
				scroll_intent,
			}),
			next,
			operation_id: input.operation.id,
			type: "commit",
		},
		{ type: "render" },
	);
	const after_transition: CoreEffect[] = [];
	if (input.operation.kind === "boot") {
		after_transition.push({ type: "install_browser_listeners" });
	}
	const result =
		input.operation.kind === "route_revalidation"
			? { ok: true }
			: input.operation.kind === "boot"
				? undefined
				: { didNavigate: true };
	append_resolve_public_calls(
		after_transition,
		input.operation.public_call_ids,
		result,
	);

	return {
		next,
		transaction: {
			after_transition,
			before_transition: before_route_publication_effects(
				input.previous,
				input.operation,
			),
			inside_transition,
			operation_id: input.operation.id,
			reason: target.reason,
		},
	};
}

export function plan_same_document_publication(
	input: SameDocumentPublicationInput,
): SameDocumentPublicationPlan | undefined {
	if (!input.operation.rights.includes("publish_route")) {
		return undefined;
	}
	if (!input.previous) {
		return undefined;
	}
	const position =
		input.operation.kind === "navigation"
			? {
					href: input.operation.href,
					key: input.operation.browser_key,
					state: input.operation.state,
				}
			: input.operation.browser;
	if (
		!route_hrefs_share_document(input.previous.position.href, position.href)
	) {
		return undefined;
	}

	let did_navigate = true;
	let should_write_history = false;
	let history_replace = false;
	if (input.operation.kind === "navigation") {
		did_navigate =
			normalized_href_hash(input.previous.position.href) !==
			normalized_href_hash(position.href);
		should_write_history = did_navigate || input.operation.replace;
		history_replace = input.operation.replace;
		if (!should_write_history) {
			return undefined;
		}
	}

	const next_route = route_facts_at_position(input.previous.route, position);
	const next_render = route_render_facts_at_position(
		input.previous.render,
		position,
	);
	const next = {
		position,
		provisional: false,
		render: next_render,
		route: next_route,
		scroll_intent: same_document_scroll_intent(
			input.operation,
			next_route,
			did_navigate,
		),
	};
	const inside_transition: CoreEffect[] = [];
	if (should_write_history) {
		inside_transition.push({
			position,
			replace: history_replace,
			type: "write_history",
		});
	}
	inside_transition.push(
		{
			commit: route_publication_commit({
				previous: input.previous,
				reason: input.operation.kind,
				render: next_render,
				route: next_route,
				scroll_intent: next.scroll_intent,
			}),
			next,
			operation_id: input.operation.id,
			type: "commit",
		},
		{ type: "render" },
	);
	const after_transition: CoreEffect[] = [];
	append_resolve_public_calls(
		after_transition,
		input.operation.public_call_ids,
		{ didNavigate: did_navigate },
	);

	return {
		did_navigate,
		kind: "publication",
		next,
		transaction: {
			after_transition,
			before_transition: before_same_document_publication_effects(
				input.previous,
				input.operation,
				did_navigate,
			),
			inside_transition,
			operation_id: input.operation.id,
			reason: input.operation.kind,
		},
	};
}

export function plan_same_document_route(
	input: SameDocumentPublicationInput,
): SameDocumentPlan | undefined {
	if (!input.previous) {
		return undefined;
	}
	const position =
		input.operation.kind === "navigation"
			? {
					href: input.operation.href,
					key: input.operation.browser_key,
					state: input.operation.state,
				}
			: input.operation.browser;
	if (
		!route_hrefs_share_document(input.previous.position.href, position.href)
	) {
		return undefined;
	}
	if (input.operation.kind === "popstate") {
		return plan_same_document_publication(input);
	}
	const did_navigate =
		normalized_href_hash(input.previous.position.href) !==
		normalized_href_hash(position.href);
	if (did_navigate || input.operation.replace) {
		return plan_same_document_publication(input);
	}

	const effects: CoreEffect[] = [
		{
			scroll: { x: 0, y: 0 },
			type: "apply_scroll",
		},
	];
	append_resolve_public_calls(effects, input.operation.public_call_ids, {
		didNavigate: false,
	});
	return {
		did_navigate: false,
		effects,
		kind: "scroll_only",
		next: input.previous,
	};
}

export function derive_scroll_intent(
	operation: RoutePublicationOperation,
	route: Pick<RouteFacts, "matches">,
): ScrollIntent | undefined {
	let href = "";
	let scroll: ScrollState | undefined;
	if (operation.kind === "boot") {
		href = operation.href;
	} else if (operation.kind === "navigation") {
		href = operation.href;
	} else if (operation.kind === "popstate") {
		href = operation.browser.href;
	} else {
		return undefined;
	}

	const hash = parse_href(href)?.hash ?? "";

	if (operation.kind === "boot") {
		scroll = operation.reload_scroll ?? (hash ? { hash } : undefined);
	} else if (operation.kind === "popstate") {
		scroll = hash
			? { hash }
			: (operation.popstate_scroll ?? { x: 0, y: 0 });
	} else if (hash) {
		scroll = { hash };
	} else if (operation.scroll_to_top !== false) {
		scroll = { x: 0, y: 0 };
	}

	if (!scroll) {
		return undefined;
	}
	return scroll_intent_for_route(route, scroll);
}

function before_route_publication_effects(
	previous: RouteSnapshot | null,
	operation: RoutePublicationOperation,
): CoreEffect[] {
	if (operation.kind === "navigation") {
		return [{ type: "save_current_scroll" }];
	}
	if (
		operation.kind === "popstate" &&
		previous?.position.key &&
		previous.position.key !== operation.browser.key
	) {
		return [
			{
				key: previous.position.key,
				scroll: operation.leaving_scroll,
				type: "save_scroll_position",
			},
		];
	}
	return [];
}

function before_same_document_publication_effects(
	previous: RouteSnapshot,
	operation: SameDocumentPublicationOperation,
	did_navigate: boolean,
): CoreEffect[] {
	if (operation.kind === "navigation") {
		return did_navigate ? [{ type: "save_current_scroll" }] : [];
	}
	return before_route_publication_effects(previous, operation);
}

function normalized_href_hash(href: string): string {
	const hash = parse_href(href)?.hash ?? "";
	const raw_hash = hash.startsWith("#") ? hash.slice(1) : hash;
	if (!raw_hash) {
		return "";
	}
	try {
		return decodeURIComponent(raw_hash);
	} catch {
		return raw_hash;
	}
}

function same_document_scroll_intent(
	operation: SameDocumentPublicationOperation,
	route: Pick<RouteFacts, "matches">,
	did_navigate: boolean,
): ScrollIntent | undefined {
	if (operation.kind === "popstate") {
		return derive_scroll_intent(operation, route);
	}
	const hash = did_navigate ? (parse_href(operation.href)?.hash ?? "") : "";
	return scroll_intent_for_route(route, hash ? { hash } : { x: 0, y: 0 });
}

function scroll_intent_for_route(
	route: Pick<RouteFacts, "matches">,
	scroll: ScrollState,
): ScrollIntent {
	const idx = route.matches.length - 1;
	return {
		scroll,
		target_route_id: `${idx}:${route.matches[idx]?.pattern ?? ""}`,
	};
}
