import {
	type Core6RouteFetchBuildSkewResult,
	type Core6RouteFetchFailureResult,
	type Core6RouteFetchHost,
	type Core6RouteFetchRedirectResult,
	type Core6RouteFetchResponseMeta,
	core6_route_fetch_result_kind,
	fetch_core6_route,
} from "./route_fetch.ts";
import { run_core6_route_transition_hooks } from "./route_hooks.ts";
import {
	type Core6RouteNavigationBuildSkewEvent,
	core6_route_navigation_build_skew_default_behavior,
} from "./route_navigation.ts";
import { run_core6_route_operation } from "./route_operation.ts";
import {
	type Core6PreparedRoute,
	type Core6RoutePreparationHost,
	decode_core6_route_payload,
	prepare_core6_route,
} from "./route_preparation.ts";
import {
	type Core6RoutePublication,
	type Core6RoutePublicationHost,
	type Core6RouteRenderState,
	type Core6RouteState,
	publish_core6_route,
} from "./route_publication.ts";
import type { Core6RouteTransactionFlowIntent } from "./route_transaction_flow.ts";
import type { Core6ScopeCommitResult } from "./scope.ts";
import {
	type Core6RouteTransactionKind,
	type Core6RouteTransactionManager,
	core6_route_transaction_kind,
} from "./transaction.ts";

export const core6_route_fetch_driver_result_kind = {
	build_skew: core6_route_fetch_result_kind.build_skew,
	failure: core6_route_fetch_result_kind.failure,
	published: "published",
	redirect: core6_route_fetch_result_kind.redirect,
} as const;

const core6_route_fetch_driver_prepare_kind = {
	prepared: "prepared",
} as const;

export type Core6RouteFetchDriverHost = Core6RouteFetchHost &
	Core6RoutePreparationHost &
	Core6RoutePublicationHost & {
		current_render_state?: () => Core6RouteRenderState | null;
		current_route: () => Core6RouteState | null;
		notify_build_skew?: (event: Core6RouteNavigationBuildSkewEvent) => void;
		route_state_equal: (
			previous_route: Core6RouteState,
			next_route: Core6RouteState,
		) => boolean;
	};

export type Core6RouteFetchDriverInput<Kind extends Core6RouteTransactionKind> =
	{
		active_client_build_id: string;
		deployment_id?: string | null;
		host: Core6RouteFetchDriverHost;
		intent: Core6RouteTransactionFlowIntent;
		kind: Kind;
		transaction_manager: Core6RouteTransactionManager<
			Kind,
			Core6RouteTransactionFlowIntent
		>;
	};

export type Core6RouteFetchDriverPublishedResult = {
	kind: typeof core6_route_fetch_driver_result_kind.published;
	publication: Core6RoutePublication;
	response: Core6RouteFetchResponseMeta;
};

export type Core6RouteFetchDriverResult =
	| Core6RouteFetchDriverPublishedResult
	| Core6RouteFetchRedirectResult
	| Core6RouteFetchBuildSkewResult
	| Core6RouteFetchFailureResult;

type Core6RouteFetchDriverPrepared =
	| {
			kind: typeof core6_route_fetch_driver_prepare_kind.prepared;
			prepared: Core6PreparedRoute;
			response: Core6RouteFetchResponseMeta;
	  }
	| Core6RouteFetchRedirectResult
	| Core6RouteFetchBuildSkewResult
	| Core6RouteFetchFailureResult;

export async function run_core6_route_fetch_driver<
	Kind extends Core6RouteTransactionKind,
>(
	input: Core6RouteFetchDriverInput<Kind>,
): Promise<Core6ScopeCommitResult<Core6RouteFetchDriverResult>> {
	return await run_core6_route_operation<
		Kind,
		Core6RouteTransactionFlowIntent,
		Awaited<ReturnType<typeof fetch_core6_route>>,
		Core6RouteFetchDriverPrepared,
		Core6RouteFetchDriverResult
	>({
		host: {
			fetch_route: async ({ intent, signal }) => {
				return await fetch_core6_route({
					active_client_build_id: input.active_client_build_id,
					deployment_id: input.deployment_id,
					href: intent.href,
					host: input.host,
					is_revalidation:
						input.kind ===
						core6_route_transaction_kind.revalidation,
					signal,
				});
			},
			prepare_route: async ({ fetched, intent, signal }) => {
				if (fetched.kind !== core6_route_fetch_result_kind.payload) {
					return fetched;
				}
				const payload = decode_core6_route_payload({
					client_build_id: fetched.response.server_build_id,
					host: input.host,
					raw_payload: fetched.payload,
					search_params: intent.search_params,
				});
				const prepared = await prepare_core6_route({
					history_state: intent.history_state,
					host: input.host,
					href: intent.href,
					payload,
					signal,
					trigger: intent.preparation_trigger,
				});
				const current_render_state =
					input.host.current_render_state?.() ?? null;
				await run_core6_route_transition_hooks({
					current_render_state,
					current_route: current_render_state
						? input.host.current_route()
						: null,
					next_prepared: prepared,
					reason: intent.publish_reason,
					signal,
				});
				return {
					kind: core6_route_fetch_driver_prepare_kind.prepared,
					prepared,
					response: fetched.response,
				};
			},
			publish_route: ({ intent, prepared }) => {
				if (
					prepared.kind !==
					core6_route_fetch_driver_prepare_kind.prepared
				) {
					return prepared;
				}
				notify_core6_route_fetch_driver_payload_build_skew({
					active_client_build_id: input.active_client_build_id,
					host: input.host,
					kind: input.kind,
					response: prepared.response,
				});
				return {
					kind: core6_route_fetch_driver_result_kind.published,
					publication: publish_core6_route({
						apply_side_effects: intent.apply_side_effects,
						history: intent.history,
						host: input.host,
						prepared: prepared.prepared,
						previous_route: input.host.current_route(),
						reason: intent.publish_reason,
						route_state_equal: input.host.route_state_equal,
						scroll: intent.scroll,
					}),
					response: prepared.response,
				};
			},
		},
		intent: input.intent,
		kind: input.kind,
		transaction_manager: input.transaction_manager,
	});
}

function notify_core6_route_fetch_driver_payload_build_skew(input: {
	active_client_build_id: string;
	host: Pick<Core6RouteFetchDriverHost, "notify_build_skew">;
	kind: Core6RouteTransactionKind;
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
		trigger: input.kind,
	});
}
