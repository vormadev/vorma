import { run_core6_route_transition_hooks } from "./route_hooks.ts";
import { run_core6_route_operation } from "./route_operation.ts";
import {
	type Core6PreparedRoute,
	type Core6RoutePreparationHost,
	type Core6RoutePreparationTrigger,
	decode_core6_route_payload,
	prepare_core6_route,
} from "./route_preparation.ts";
import {
	type Core6RouteHistoryMutation,
	type Core6RoutePublication,
	type Core6RoutePublicationHost,
	type Core6RoutePublishReason,
	type Core6RouteRenderState,
	type Core6RouteScroll,
	type Core6RouteState,
	publish_core6_route,
} from "./route_publication.ts";
import type { Core6ScopeCommitResult } from "./scope.ts";
import type {
	Core6RouteTransactionKind,
	Core6RouteTransactionManager,
} from "./transaction.ts";

export type Core6RouteTransactionFlowIntent = {
	apply_side_effects?: boolean;
	history?: Core6RouteHistoryMutation;
	history_state: unknown;
	href: string;
	preparation_trigger: Core6RoutePreparationTrigger;
	publish_reason: Core6RoutePublishReason;
	search_params: URLSearchParams;
	scroll?: Core6RouteScroll;
};

export type Core6RouteTransactionFlowFetchArgs = {
	intent: Core6RouteTransactionFlowIntent;
	signal: AbortSignal;
};

export type Core6RouteTransactionFlowHost = Core6RoutePreparationHost &
	Core6RoutePublicationHost & {
		current_render_state?: () => Core6RouteRenderState | null;
		current_route: () => Core6RouteState | null;
		fetch_route_payload: (
			args: Core6RouteTransactionFlowFetchArgs,
		) => Promise<unknown>;
		route_state_equal: (
			previous_route: Core6RouteState,
			next_route: Core6RouteState,
		) => boolean;
	};

export type Core6RouteTransactionFlowInput<
	Kind extends Core6RouteTransactionKind,
> = {
	host: Core6RouteTransactionFlowHost;
	intent: Core6RouteTransactionFlowIntent;
	kind: Kind;
	transaction_manager: Core6RouteTransactionManager<
		Kind,
		Core6RouteTransactionFlowIntent
	>;
};

export async function run_core6_route_transaction_flow<
	Kind extends Core6RouteTransactionKind,
>(
	input: Core6RouteTransactionFlowInput<Kind>,
): Promise<Core6ScopeCommitResult<Core6RoutePublication>> {
	return await run_core6_route_operation<
		Kind,
		Core6RouteTransactionFlowIntent,
		unknown,
		Core6PreparedRoute,
		Core6RoutePublication
	>({
		host: {
			fetch_route: async ({ intent, signal }) => {
				return await input.host.fetch_route_payload({
					intent,
					signal,
				});
			},
			prepare_route: async ({ fetched, intent, signal }) => {
				const payload = decode_core6_route_payload({
					host: input.host,
					raw_payload: fetched,
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
				return prepared;
			},
			publish_route: ({ intent, prepared }) => {
				return publish_core6_route({
					apply_side_effects: intent.apply_side_effects,
					history: intent.history,
					host: input.host,
					prepared,
					previous_route: input.host.current_route(),
					reason: intent.publish_reason,
					route_state_equal: input.host.route_state_equal,
					scroll: intent.scroll,
				});
			},
		},
		intent: input.intent,
		kind: input.kind,
		transaction_manager: input.transaction_manager,
	});
}
