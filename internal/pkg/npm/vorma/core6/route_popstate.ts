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
	core6_same_document_href,
	core6_scroll_for_href,
} from "./route_url.ts";
import { core6_route_transaction_kind } from "./transaction.ts";

export const core6_route_popstate_result_kind = {
	ignored: "ignored",
	reloaded: "reloaded",
	routed: "routed",
	same_document: "same_document",
} as const;

export type Core6RoutePopstateResultKind =
	(typeof core6_route_popstate_result_kind)[keyof typeof core6_route_popstate_result_kind];

export type Core6RoutePopstatePosition = {
	href: string;
	key: string;
	state: unknown;
};

export type Core6RoutePopstateScroll = Core6RouteScroll;

export type Core6RoutePopstateRuntime = Pick<
	Core6RouteRuntime,
	| "cancel_current"
	| "current_route"
	| "current_transaction_kind"
	| "publish_same_document"
	| "run_route_fetch"
	| "run_route_prepared"
>;

export type Core6RoutePopstateHost = Core6RouteNavigationHost & {
	reload: (href: string) => void;
	save_scroll_for_key: (key: string) => void;
};

export type Core6RoutePopstateConfig = {
	active_client_build_id: () => string;
	deployment_id?: () => string | null | undefined;
	host: Core6RoutePopstateHost;
	initial_position: Core6RoutePopstatePosition;
	prefetch?: Pick<Core6RoutePrefetchOwner, "take_prepared">;
	runtime: Core6RoutePopstateRuntime;
};

export type Core6RoutePopstateInput = {
	position: Core6RoutePopstatePosition;
	restored_scroll?: Core6RoutePopstateScroll;
};

export type Core6RoutePopstateStatus = {
	href: string;
	key: string;
	restored_scroll?: Core6RoutePopstateScroll;
};

export type Core6RoutePopstateIgnoredResult = {
	kind: typeof core6_route_popstate_result_kind.ignored;
	position: Core6RoutePopstatePosition;
};

export type Core6RoutePopstateSameDocumentResult = {
	kind: typeof core6_route_popstate_result_kind.same_document;
	position: Core6RoutePopstatePosition;
	previous_position: Core6RoutePopstatePosition;
	publication: Core6RoutePublication;
	scroll: Core6RoutePopstateScroll;
};

export type Core6RoutePopstateRoutedResult = {
	kind: typeof core6_route_popstate_result_kind.routed;
	navigation: Core6RouteNavigationResult;
	position: Core6RoutePopstatePosition;
};

export type Core6RoutePopstateReloadedResult = {
	kind: typeof core6_route_popstate_result_kind.reloaded;
	navigation: Core6RouteNavigationResult;
	position: Core6RoutePopstatePosition;
};

export type Core6RoutePopstateResult =
	| Core6RoutePopstateIgnoredResult
	| Core6RoutePopstateSameDocumentResult
	| Core6RoutePopstateRoutedResult
	| Core6RoutePopstateReloadedResult;

export type Core6RoutePopstateOwner = {
	current_position: () => Core6RoutePopstatePosition;
	current_status: () => Core6RoutePopstateStatus | null;
	handle: (
		input: Core6RoutePopstateInput,
	) => Promise<Core6RoutePopstateResult>;
};

export function create_core6_route_popstate_owner(
	config: Core6RoutePopstateConfig,
): Core6RoutePopstateOwner {
	let current_position = { ...config.initial_position };
	let current_run: Core6RoutePopstateStatus | null = null;
	let next_run_id = 0;

	function same_document_result(input: {
		position: Core6RoutePopstatePosition;
		previous_position: Core6RoutePopstatePosition;
		restored_scroll?: Core6RoutePopstateScroll;
	}): Core6RoutePopstateSameDocumentResult {
		const active_kind = config.runtime.current_transaction_kind();
		if (
			active_kind === core6_route_transaction_kind.navigation ||
			active_kind === core6_route_transaction_kind.popstate
		) {
			config.runtime.cancel_current();
		}
		const scroll = core6_scroll_for_href(
			input.position.href,
			input.restored_scroll,
		);
		const publication = config.runtime.publish_same_document({
			history_state: input.position.state,
			href: input.position.href,
			reason: core6_route_publish_reason.popstate,
			scroll,
		});
		if (!publication) {
			throw new Error("same-document popstate requires current route");
		}
		return {
			kind: core6_route_popstate_result_kind.same_document,
			position: input.position,
			previous_position: input.previous_position,
			publication,
			scroll,
		};
	}

	async function cross_document_result(input: {
		position: Core6RoutePopstatePosition;
		restored_scroll?: Core6RoutePopstateScroll;
		run_id: number;
	}): Promise<Core6RoutePopstateResult> {
		current_run = {
			href: input.position.href,
			key: input.position.key,
			restored_scroll: input.restored_scroll,
		};
		try {
			const target = new URL(input.position.href);
			const navigation = await run_core6_route_navigation({
				active_client_build_id: config.active_client_build_id(),
				deployment_id: config.deployment_id?.(),
				host: config.host,
				intent: {
					history_state: input.position.state,
					href: target.href,
					preparation_trigger:
						core6_route_preparation_trigger.navigation,
					publish_reason: core6_route_publish_reason.popstate,
					scroll: core6_scroll_for_href(
						target.href,
						input.restored_scroll,
					),
					search_params: target.searchParams,
				},
				kind: core6_route_transaction_kind.popstate,
				prefetch: config.prefetch,
				runtime: config.runtime,
			});
			if (input.run_id !== next_run_id) {
				return {
					kind: core6_route_popstate_result_kind.routed,
					navigation,
					position: input.position,
				};
			}
			if (
				core6_route_popstate_should_reload(
					config.runtime,
					navigation,
					input.position,
				)
			) {
				config.host.reload(input.position.href);
				return {
					kind: core6_route_popstate_result_kind.reloaded,
					navigation,
					position: input.position,
				};
			}
			return {
				kind: core6_route_popstate_result_kind.routed,
				navigation,
				position: input.position,
			};
		} finally {
			if (input.run_id === next_run_id) {
				current_run = null;
			}
		}
	}

	return {
		current_position: () => {
			return { ...current_position };
		},
		current_status: () => {
			if (!current_run) {
				return null;
			}
			return {
				...current_run,
			};
		},
		handle: async (input) => {
			const previous_position = current_position;
			const position = { ...input.position };
			if (
				position.key === previous_position.key &&
				position.href === previous_position.href
			) {
				return {
					kind: core6_route_popstate_result_kind.ignored,
					position,
				};
			}
			if (
				previous_position.key.length > 0 &&
				previous_position.key !== position.key
			) {
				config.host.save_scroll_for_key(previous_position.key);
			}
			current_position = position;
			next_run_id++;
			const current_route = config.runtime.current_route();
			if (
				current_route &&
				core6_same_document_href(position.href, current_route.href)
			) {
				return same_document_result({
					position,
					previous_position,
					restored_scroll: input.restored_scroll,
				});
			}
			return await cross_document_result({
				position,
				restored_scroll: input.restored_scroll,
				run_id: next_run_id,
			});
		},
	};
}

function core6_route_popstate_should_reload(
	runtime: Core6RoutePopstateRuntime,
	navigation: Core6RouteNavigationResult,
	position: Core6RoutePopstatePosition,
): boolean {
	if (
		navigation.kind !== core6_route_navigation_result_kind.failed &&
		navigation.kind !==
			core6_route_navigation_result_kind.invalid_redirect &&
		navigation.kind !== core6_route_navigation_result_kind.redirect_loop &&
		navigation.kind !== core6_route_navigation_result_kind.interrupted
	) {
		return false;
	}
	const active_kind = runtime.current_transaction_kind();
	if (active_kind !== null) {
		return false;
	}
	const route = runtime.current_route();
	if (!route) {
		return true;
	}
	return !core6_same_document_href(position.href, route.href);
}
