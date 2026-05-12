import { run_core6_route_pipeline } from "./pipeline.ts";
import {
	type Core6RouteFetchBuildSkewResult,
	type Core6RouteFetchFailureResult,
	type Core6RouteFetchHost,
	type Core6RouteFetchRedirectResult,
	type Core6RouteFetchResponseMeta,
	core6_route_fetch_result_kind,
	fetch_core6_route,
} from "./route_fetch.ts";
import {
	type Core6RouteNavigationHost,
	core6_route_navigation_build_skew_default_behavior,
} from "./route_navigation.ts";
import {
	type Core6PreparedRoute,
	type Core6RoutePreparationHost,
	core6_route_preparation_trigger,
	decode_core6_route_payload,
	prepare_core6_route,
} from "./route_preparation.ts";
import type { Core6RouteState } from "./route_publication.ts";
import { core6_same_document_href } from "./route_url.ts";
import {
	core6_scope_cancelled_reason,
	core6_scope_stale_reason,
} from "./scope.ts";
import {
	type Core6RouteTransaction,
	core6_route_transaction_kind,
	create_core6_route_transaction_manager,
} from "./transaction.ts";

export const core6_route_prefetch_status = {
	fetching: "fetching",
	prepared: "prepared",
	preparing: "preparing",
} as const;

export const core6_route_prefetch_result_kind = {
	interrupted: "interrupted",
	prepared: "prepared",
} as const;

export type Core6RoutePrefetchHost = Core6RouteFetchHost &
	Core6RoutePreparationHost &
	Pick<Core6RouteNavigationHost, "notify_build_skew">;

export type Core6RoutePrefetchIntent = {
	history_state: unknown;
	href: string;
	search_params: URLSearchParams;
};

export type Core6RoutePrefetchPreparedResult = {
	kind: typeof core6_route_prefetch_result_kind.prepared;
	prepared: Core6PreparedRoute;
	response: Core6RouteFetchResponseMeta;
};

export type Core6RoutePrefetchResult =
	| Core6RoutePrefetchPreparedResult
	| Core6RouteFetchRedirectResult
	| Core6RouteFetchBuildSkewResult
	| Core6RouteFetchFailureResult
	| {
			kind: typeof core6_route_prefetch_result_kind.interrupted;
			reason:
				| typeof core6_scope_cancelled_reason
				| typeof core6_scope_stale_reason;
	  };

export type Core6RoutePrefetchPromotion = {
	href: string;
	prepared: Core6PreparedRoute;
	response: Core6RouteFetchResponseMeta;
};

export type Core6RoutePrefetchStartInput = {
	history_state?: unknown;
	href: string;
};

export type Core6RoutePrefetchStatus =
	| {
			href: string;
			status: typeof core6_route_prefetch_status.fetching;
	  }
	| {
			href: string;
			status: typeof core6_route_prefetch_status.preparing;
	  }
	| {
			href: string;
			status: typeof core6_route_prefetch_status.prepared;
	  };

export type Core6RoutePrefetchOwner = {
	cancel: (href: string) => boolean;
	cancel_current: () => boolean;
	current_status: () => Core6RoutePrefetchStatus | null;
	start: (
		input: Core6RoutePrefetchStartInput,
	) => Promise<Core6RoutePrefetchResult> | null;
	take_prepared: (href: string) => Core6RoutePrefetchPromotion | null;
};

export type Core6RoutePrefetchConfig = {
	active_client_build_id: () => string;
	current_route: () => Core6RouteState | null;
	deployment_id?: () => string | null | undefined;
	host: Core6RoutePrefetchHost;
};

type Core6RoutePrefetchPipelinePrepared =
	| Core6RoutePrefetchPreparedResult
	| Core6RouteFetchRedirectResult
	| Core6RouteFetchBuildSkewResult
	| Core6RouteFetchFailureResult;

type Core6RoutePrefetchState =
	| {
			href: string;
			kind: typeof core6_route_prefetch_status.fetching;
			transaction: Core6RouteTransaction<
				typeof core6_route_transaction_kind.prefetch,
				Core6RoutePrefetchIntent
			>;
	  }
	| {
			href: string;
			kind: typeof core6_route_prefetch_status.preparing;
			transaction: Core6RouteTransaction<
				typeof core6_route_transaction_kind.prefetch,
				Core6RoutePrefetchIntent
			>;
	  }
	| {
			href: string;
			kind: typeof core6_route_prefetch_status.prepared;
			prepared: Core6PreparedRoute;
			response: Core6RouteFetchResponseMeta;
	  };

export function create_core6_route_prefetch_owner(
	config: Core6RoutePrefetchConfig,
): Core6RoutePrefetchOwner {
	const transaction_manager = create_core6_route_transaction_manager<
		typeof core6_route_transaction_kind.prefetch,
		Core6RoutePrefetchIntent
	>();
	let state: Core6RoutePrefetchState | null = null;

	function resolve_target(href: string): URL | null {
		const current_route = config.current_route();
		if (!current_route) {
			return null;
		}
		try {
			return new URL(href, current_route.href);
		} catch {
			return null;
		}
	}

	function cancel_current(): boolean {
		if (!state) {
			return false;
		}
		if (state.kind === core6_route_prefetch_status.prepared) {
			state = null;
			return true;
		}
		const transaction = state.transaction;
		state = null;
		transaction.cancel();
		return true;
	}

	return {
		cancel: (href) => {
			if (!state) {
				return false;
			}
			const target = resolve_target(href);
			if (!target || !core6_same_document_href(target.href, state.href)) {
				return false;
			}
			return cancel_current();
		},
		cancel_current,
		current_status: () => {
			if (!state) {
				return null;
			}
			return {
				href: state.href,
				status: state.kind,
			};
		},
		start: (input) => {
			const current_route = config.current_route();
			const target = resolve_target(input.href);
			if (!current_route || !target) {
				return null;
			}
			if (
				(target.protocol !== "http:" && target.protocol !== "https:") ||
				target.origin !== new URL(current_route.href).origin ||
				core6_same_document_href(target.href, current_route.href)
			) {
				return null;
			}
			if (state && core6_same_document_href(target.href, state.href)) {
				return null;
			}
			cancel_current();
			const intent: Core6RoutePrefetchIntent = {
				history_state:
					input.history_state ?? current_route.historyState,
				href: target.href,
				search_params: new URLSearchParams(target.search),
			};
			const transaction = transaction_manager.start({
				intent,
				kind: core6_route_transaction_kind.prefetch,
			});
			state = {
				href: target.href,
				kind: core6_route_prefetch_status.fetching,
				transaction,
			};
			return run_core6_route_prefetch_pipeline(
				config,
				transaction,
				(prepared) => {
					if (
						!(
							(state?.kind ===
								core6_route_prefetch_status.fetching ||
								state?.kind ===
									core6_route_prefetch_status.preparing) &&
							state.transaction.id === transaction.id
						)
					) {
						return;
					}
					state = prepared;
				},
			);
		},
		take_prepared: (href) => {
			if (state?.kind !== core6_route_prefetch_status.prepared) {
				return null;
			}
			const target = resolve_target(href);
			if (!target || !core6_same_document_href(target.href, state.href)) {
				return null;
			}
			const promotion = {
				href: state.href,
				prepared: state.prepared,
				response: state.response,
			};
			state = null;
			return promotion;
		},
	};
}

async function run_core6_route_prefetch_pipeline(
	config: Core6RoutePrefetchConfig,
	transaction: Core6RouteTransaction<
		typeof core6_route_transaction_kind.prefetch,
		Core6RoutePrefetchIntent
	>,
	set_state: (state: Core6RoutePrefetchState | null) => void,
): Promise<Core6RoutePrefetchResult> {
	try {
		const result = await run_core6_route_pipeline<
			typeof core6_route_transaction_kind.prefetch,
			Core6RoutePrefetchIntent,
			Awaited<ReturnType<typeof fetch_core6_route>>,
			Core6RoutePrefetchPipelinePrepared,
			Core6RoutePrefetchResult
		>(transaction, {
			fetch: async ({ intent, signal }) => {
				return await fetch_core6_route({
					active_client_build_id: config.active_client_build_id(),
					deployment_id: config.deployment_id?.(),
					href: intent.href,
					host: config.host,
					signal,
				});
			},
			prepare: async ({ fetched, intent, signal }) => {
				if (fetched.kind !== core6_route_fetch_result_kind.payload) {
					return fetched;
				}
				set_state({
					href: intent.href,
					kind: core6_route_prefetch_status.preparing,
					transaction,
				});
				notify_core6_route_prefetch_payload_build_skew({
					active_client_build_id: config.active_client_build_id(),
					host: config.host,
					response: fetched.response,
				});
				const payload = decode_core6_route_payload({
					client_build_id: fetched.response.server_build_id,
					host: config.host,
					raw_payload: fetched.payload,
					search_params: intent.search_params,
				});
				return {
					kind: core6_route_prefetch_result_kind.prepared,
					prepared: await prepare_core6_route({
						history_state: intent.history_state,
						host: config.host,
						href: intent.href,
						payload,
						signal,
						trigger: core6_route_preparation_trigger.prefetch,
					}),
					response: fetched.response,
				};
			},
			publish: ({ intent, prepared }) => {
				if (
					prepared.kind === core6_route_prefetch_result_kind.prepared
				) {
					set_state({
						href: intent.href,
						kind: core6_route_prefetch_status.prepared,
						prepared: prepared.prepared,
						response: prepared.response,
					});
					return prepared;
				}
				if (
					prepared.kind === core6_route_fetch_result_kind.build_skew
				) {
					config.host.notify_build_skew?.({
						active_client_build_id: config.active_client_build_id(),
						default_behavior:
							core6_route_navigation_build_skew_default_behavior.notify_only,
						response: prepared.response,
						trigger: core6_route_transaction_kind.prefetch,
					});
				}
				set_state(null);
				return prepared;
			},
		});
		if (!result.ok) {
			set_state(null);
			return {
				kind: core6_route_prefetch_result_kind.interrupted,
				reason: result.reason,
			};
		}
		return result.value;
	} catch (error) {
		set_state(null);
		throw error;
	}
}

function notify_core6_route_prefetch_payload_build_skew(input: {
	active_client_build_id: string;
	host: Pick<Core6RoutePrefetchHost, "notify_build_skew">;
	response: Core6RouteFetchResponseMeta;
}): void {
	if (
		input.response.server_build_id.length === 0 ||
		input.active_client_build_id.length === 0 ||
		input.response.server_build_id === input.active_client_build_id
	) {
		return;
	}
	input.host.notify_build_skew?.({
		active_client_build_id: input.active_client_build_id,
		default_behavior:
			core6_route_navigation_build_skew_default_behavior.notify_only,
		response: input.response,
		trigger: core6_route_transaction_kind.prefetch,
	});
}
