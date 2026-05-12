import type { Core6RouteFetchHost } from "./route_fetch.ts";
import {
	type Core6RouteFetchDriverHost,
	type Core6RouteFetchDriverResult,
	run_core6_route_fetch_driver,
} from "./route_fetch_driver.ts";
import type { Core6RouteNavigationHost } from "./route_navigation.ts";
import type {
	Core6ClientLoaderFn,
	Core6PreparedRoute,
	Core6RoutePreparationHost,
} from "./route_preparation.ts";
import type {
	Core6RoutePublication,
	Core6RoutePublishReason,
	Core6RouteRenderState,
	Core6RouteScroll,
	Core6RouteState,
} from "./route_publication.ts";
import { publish_core6_route } from "./route_publication.ts";
import {
	type Core6RoutePublicationStoreHost,
	create_core6_route_publication_store,
} from "./route_publication_store.ts";
import {
	type Core6RouteTransactionFlowFetchArgs,
	type Core6RouteTransactionFlowHost,
	type Core6RouteTransactionFlowIntent,
	run_core6_route_transaction_flow,
} from "./route_transaction_flow.ts";
import type { Core6ScopeCommitResult } from "./scope.ts";
import {
	type Core6RouteTransactionKind,
	create_core6_route_transaction_manager,
} from "./transaction.ts";

export type Core6RouteRuntimeHost = Core6RoutePreparationHost &
	Core6RoutePublicationStoreHost &
	Core6RouteFetchHost &
	Pick<Core6RouteNavigationHost, "notify_build_skew"> & {
		fetch_route_payload: (
			args: Core6RouteTransactionFlowFetchArgs,
		) => Promise<unknown>;
		route_state_equal: (
			previous_route: Core6RouteState,
			next_route: Core6RouteState,
		) => boolean;
	};

export type Core6RouteRuntimeRunInput = {
	intent: Core6RouteTransactionFlowIntent;
	kind: Core6RouteTransactionKind;
};

export type Core6RouteRuntimeRunFetchInput = Core6RouteRuntimeRunInput & {
	active_client_build_id: string;
	deployment_id?: string | null;
};

export type Core6RouteRuntimeRunPreparedInput = Core6RouteRuntimeRunInput & {
	prepared: Core6PreparedRoute;
};

export type Core6RouteRuntimeRunPayloadInput = Core6RouteRuntimeRunInput & {
	payload: unknown;
};

export type Core6RouteRuntimePublishSameDocumentInput = {
	history?: Core6RouteTransactionFlowIntent["history"];
	href: string;
	history_state: unknown;
	reason: Core6RoutePublishReason;
	scroll?: Core6RouteScroll;
};

export type Core6RouteRuntime = {
	cancel_current: () => boolean;
	client_loader: (pattern: string) => Core6ClientLoaderFn | undefined;
	client_loader_patterns: () => string[];
	current_render_state: () => Core6RouteRenderState | null;
	current_route: () => Core6RouteState | null;
	current_transaction_kind: () => Core6RouteTransactionKind | null;
	publish_same_document: (
		input: Core6RouteRuntimePublishSameDocumentInput,
	) => Core6RoutePublication | null;
	run_route: (
		input: Core6RouteRuntimeRunInput,
	) => Promise<Core6ScopeCommitResult<Core6RoutePublication>>;
	run_route_fetch: (
		input: Core6RouteRuntimeRunFetchInput,
	) => Promise<Core6ScopeCommitResult<Core6RouteFetchDriverResult>>;
	run_route_payload: (
		input: Core6RouteRuntimeRunPayloadInput,
	) => Promise<Core6ScopeCommitResult<Core6RoutePublication>>;
	run_route_prepared: (
		input: Core6RouteRuntimeRunPreparedInput,
	) => Core6ScopeCommitResult<Core6RoutePublication>;
};

export function create_core6_route_runtime(
	host: Core6RouteRuntimeHost,
): Core6RouteRuntime {
	const publication_store = create_core6_route_publication_store({
		commit_publication: host.commit_publication,
	});
	const transaction_manager = create_core6_route_transaction_manager<
		Core6RouteTransactionKind,
		Core6RouteTransactionFlowIntent
	>();
	const flow_host: Core6RouteTransactionFlowHost = {
		current_render_state: publication_store.current_render_state,
		current_route: publication_store.current_route,
		fetch_route_payload: host.fetch_route_payload,
		import_module: host.import_module,
		parse_input: host.parse_input,
		publish_route: publication_store.publish_route,
		preload_css: host.preload_css,
		route_state_equal: host.route_state_equal,
		set_provisional_route: host.set_provisional_route,
		wait_for_css: host.wait_for_css,
	};
	const fetch_driver_host: Core6RouteFetchDriverHost = {
		current_render_state: publication_store.current_render_state,
		current_route: publication_store.current_route,
		fetch_route_response: host.fetch_route_response,
		import_module: host.import_module,
		notify_build_skew: host.notify_build_skew,
		parse_input: host.parse_input,
		publish_route: publication_store.publish_route,
		preload_css: host.preload_css,
		route_state_equal: host.route_state_equal,
		set_provisional_route: host.set_provisional_route,
		wait_for_css: host.wait_for_css,
	};

	return {
		cancel_current: () => {
			return transaction_manager.cancel_current();
		},
		client_loader: (pattern) => {
			return publication_store.client_loader(pattern);
		},
		client_loader_patterns: () => {
			return publication_store.client_loader_patterns();
		},
		current_render_state: () => {
			return publication_store.current_render_state();
		},
		current_route: () => {
			return publication_store.current_route();
		},
		current_transaction_kind: () => {
			return transaction_manager.current()?.kind ?? null;
		},
		publish_same_document: (input) => {
			return publication_store.publish_same_document({
				href: input.href,
				history_state: input.history_state,
				reason: input.reason,
				route_state_equal: host.route_state_equal,
				scroll: input.scroll,
				...(input.history ? { history: input.history } : {}),
			});
		},
		run_route: async (input) => {
			return await run_core6_route_transaction_flow({
				host: flow_host,
				intent: input.intent,
				kind: input.kind,
				transaction_manager,
			});
		},
		run_route_fetch: async (input) => {
			return await run_core6_route_fetch_driver({
				active_client_build_id: input.active_client_build_id,
				deployment_id: input.deployment_id,
				host: fetch_driver_host,
				intent: input.intent,
				kind: input.kind,
				transaction_manager,
			});
		},
		run_route_payload: async (input) => {
			return await run_core6_route_transaction_flow({
				host: {
					...flow_host,
					fetch_route_payload: async () => {
						return input.payload;
					},
				},
				intent: input.intent,
				kind: input.kind,
				transaction_manager,
			});
		},
		run_route_prepared: (input) => {
			const transaction = transaction_manager.start({
				intent: input.intent,
				kind: input.kind,
			});
			return transaction.complete((intent) => {
				return publish_core6_route({
					apply_side_effects: intent.apply_side_effects,
					history: intent.history,
					host: publication_store,
					prepared: input.prepared,
					previous_route: publication_store.current_route(),
					reason: intent.publish_reason,
					route_state_equal: host.route_state_equal,
					scroll: intent.scroll,
				});
			});
		},
	};
}
