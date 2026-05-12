import {
	core6_route_navigation_result_kind,
	run_core6_route_navigation,
	type Core6RouteNavigationHost,
	type Core6RouteNavigationResult,
} from "./route_navigation.ts";
import type { Core6RoutePrefetchOwner } from "./route_prefetch.ts";
import { core6_route_preparation_trigger } from "./route_preparation.ts";
import {
	core6_route_publish_reason,
	type Core6RoutePublication,
	type Core6RouteScroll,
} from "./route_publication.ts";
import type { Core6RouteRuntime } from "./route_runtime.ts";
import {
	core6_is_http_href,
	core6_normalized_hash_from_href,
	core6_same_document_href,
	core6_same_origin_href,
	core6_scroll_for_href,
} from "./route_url.ts";
import { core6_route_transaction_kind } from "./transaction.ts";

export const core6_route_navigation_owner_result_kind = {
	hard_redirect: "hard_redirect",
	invalid: "invalid",
	routed: "routed",
	same_document: "same_document",
} as const;

export type Core6RouteNavigationOwnerResultKind =
	(typeof core6_route_navigation_owner_result_kind)[keyof typeof core6_route_navigation_owner_result_kind];

export type Core6RouteNavigationHistoryIntent = {
	href: string;
	replace: boolean;
	state: unknown;
};

export type Core6RouteNavigationOwnerRuntime = Pick<
	Core6RouteRuntime,
	| "cancel_current"
	| "current_route"
	| "current_transaction_kind"
	| "publish_same_document"
	| "run_route_fetch"
	| "run_route_prepared"
>;

export type Core6RouteNavigationOwnerHost = Core6RouteNavigationHost & {
	current_href: () => string;
};

export type Core6RouteNavigationOwnerConfig = {
	active_client_build_id: () => string;
	deployment_id?: () => string | null | undefined;
	host: Core6RouteNavigationOwnerHost;
	prefetch?: Pick<Core6RoutePrefetchOwner, "take_prepared">;
	runtime: Core6RouteNavigationOwnerRuntime;
};

export type Core6RouteNavigationOwnerInput = {
	href: string;
	replace?: boolean;
	scroll_to_top?: boolean;
	skip_work_indicator?: boolean;
	state?: unknown;
};

export type Core6RouteNavigationOwnerStatus = {
	href: string;
	replace: boolean;
	skip_work_indicator?: boolean;
	state: unknown;
};

export type Core6RouteNavigationOwnerInvalidResult = {
	did_navigate: false;
	history: null;
	href: string;
	kind: typeof core6_route_navigation_owner_result_kind.invalid;
};

export type Core6RouteNavigationOwnerHardRedirectResult = {
	did_navigate: false;
	history: null;
	href: string;
	kind: typeof core6_route_navigation_owner_result_kind.hard_redirect;
};

export type Core6RouteNavigationOwnerSameDocumentResult = {
	did_navigate: boolean;
	history: Core6RouteNavigationHistoryIntent | null;
	kind: typeof core6_route_navigation_owner_result_kind.same_document;
	publication: Core6RoutePublication | null;
	scroll: Core6RouteScroll;
};

export type Core6RouteNavigationOwnerRoutedResult = {
	did_navigate: boolean;
	history: Core6RouteNavigationHistoryIntent | null;
	kind: typeof core6_route_navigation_owner_result_kind.routed;
	navigation: Core6RouteNavigationResult;
};

export type Core6RouteNavigationOwnerResult =
	| Core6RouteNavigationOwnerInvalidResult
	| Core6RouteNavigationOwnerHardRedirectResult
	| Core6RouteNavigationOwnerSameDocumentResult
	| Core6RouteNavigationOwnerRoutedResult;

export type Core6RouteNavigationOwner = {
	current_status: () => Core6RouteNavigationOwnerStatus | null;
	navigate: (
		input: Core6RouteNavigationOwnerInput,
	) => Promise<Core6RouteNavigationOwnerResult>;
};

export function create_core6_route_navigation_owner(
	config: Core6RouteNavigationOwnerConfig,
): Core6RouteNavigationOwner {
	let current_status: Core6RouteNavigationOwnerStatus | null = null;
	let next_run_id = 0;

	function same_document_result(input: {
		replace: boolean;
		state: unknown;
		target: URL;
	}): Core6RouteNavigationOwnerSameDocumentResult {
		const active_kind = config.runtime.current_transaction_kind();
		if (
			active_kind === core6_route_transaction_kind.navigation ||
			active_kind === core6_route_transaction_kind.popstate
		) {
			config.runtime.cancel_current();
		}
		const current_route = config.runtime.current_route();
		if (!current_route) {
			throw new Error("same-document navigation requires current route");
		}
		const target_hash = core6_normalized_hash_from_href(input.target.href);
		const current_hash = core6_normalized_hash_from_href(
			current_route.href,
		);
		const did_navigate = target_hash !== current_hash;
		const scroll = core6_scroll_for_href(input.target.href);
		if (!did_navigate && !input.replace) {
			return {
				did_navigate: false,
				history: null,
				kind: core6_route_navigation_owner_result_kind.same_document,
				publication: null,
				scroll,
			};
		}
		const publication = config.runtime.publish_same_document({
			history: {
				href: input.target.href,
				replace: input.replace,
				state: input.state,
			},
			history_state: input.state,
			href: input.target.href,
			reason: core6_route_publish_reason.navigation,
			scroll,
		});
		if (!publication) {
			throw new Error("same-document navigation requires current route");
		}
		return {
			did_navigate,
			history: {
				href: publication.route.href,
				replace: input.replace,
				state: publication.route.historyState,
			},
			kind: core6_route_navigation_owner_result_kind.same_document,
			publication,
			scroll,
		};
	}

	async function routed_result(input: {
		replace: boolean;
		run_id: number;
		scroll_to_top?: boolean;
		skip_work_indicator?: boolean;
		state: unknown;
		target: URL;
	}): Promise<Core6RouteNavigationOwnerRoutedResult> {
		current_status = {
			href: input.target.href,
			replace: input.replace,
			...(input.skip_work_indicator === true
				? { skip_work_indicator: true }
				: {}),
			state: input.state,
		};
		try {
			const navigation = await run_core6_route_navigation({
				active_client_build_id: config.active_client_build_id(),
				deployment_id: config.deployment_id?.(),
				host: config.host,
				intent: {
					history_state: input.state,
					history: {
						href: input.target.href,
						replace: input.replace,
						state: input.state,
					},
					href: input.target.href,
					preparation_trigger:
						core6_route_preparation_trigger.navigation,
					publish_reason: core6_route_publish_reason.navigation,
					scroll: core6_navigation_scroll_for_target(
						input.target.href,
						input.scroll_to_top,
					),
					search_params: input.target.searchParams,
				},
				kind: core6_route_transaction_kind.navigation,
				prefetch: config.prefetch,
				runtime: config.runtime,
			});
			const history = core6_navigation_history_intent({
				navigation,
			});
			return {
				did_navigate: history !== null,
				history,
				kind: core6_route_navigation_owner_result_kind.routed,
				navigation,
			};
		} finally {
			if (input.run_id === next_run_id) {
				current_status = null;
			}
		}
	}

	return {
		current_status: () => {
			if (!current_status) {
				return null;
			}
			return { ...current_status };
		},
		navigate: async (input) => {
			const replace = input.replace ?? false;
			const target = core6_resolve_navigation_target(
				input.href,
				config.host.current_href(),
			);
			if (!target) {
				return {
					did_navigate: false,
					history: null,
					href: input.href,
					kind: core6_route_navigation_owner_result_kind.invalid,
				};
			}
			if (
				!core6_is_http_href(target.href) ||
				!core6_same_origin_href(target.href, config.host.current_href())
			) {
				config.host.hard_redirect(target.href);
				return {
					did_navigate: false,
					history: null,
					href: target.href,
					kind: core6_route_navigation_owner_result_kind.hard_redirect,
				};
			}
			next_run_id++;
			const current_route = config.runtime.current_route();
			if (
				current_route &&
				core6_same_document_href(target.href, current_route.href)
			) {
				return same_document_result({
					replace,
					state: input.state,
					target,
				});
			}
			return await routed_result({
				replace,
				run_id: next_run_id,
				scroll_to_top: input.scroll_to_top,
				skip_work_indicator: input.skip_work_indicator,
				state: input.state,
				target,
			});
		},
	};
}

function core6_navigation_history_intent(input: {
	navigation: Core6RouteNavigationResult;
}): Core6RouteNavigationHistoryIntent | null {
	if (
		input.navigation.kind === core6_route_navigation_result_kind.published
	) {
		return input.navigation.result.publication.history ?? null;
	}
	if (
		input.navigation.kind ===
		core6_route_navigation_result_kind.same_document
	) {
		return input.navigation.publication.history ?? null;
	}
	return null;
}

function core6_navigation_scroll_for_target(
	href: string,
	scroll_to_top: boolean | undefined,
): Core6RouteScroll | undefined {
	const hash = new URL(href).hash;
	if (hash.length > 0) {
		return { hash };
	}
	if (scroll_to_top === false) {
		return undefined;
	}
	return { x: 0, y: 0 };
}

function core6_resolve_navigation_target(
	href: string,
	current_href: string,
): URL | null {
	try {
		return new URL(href, current_href);
	} catch {
		return null;
	}
}
