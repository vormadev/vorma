/// <reference types="vite/client" />

import { R } from "vorma/kit/result";
import type { RevalidationResult } from "../core/types.ts";
import type {
	APIResult,
	ClientCore,
	ClientOptions,
	SubmitOptions,
	ViewDefinition,
	WorkIndicator,
} from "./client_contract.ts";
import type { CoreController } from "./controller.ts";
import type { CoreHost, PublicCallRegistry } from "./host.ts";
import type { IDSource } from "./ids.ts";
import {
	api_result_kind,
	client_error_message,
	event_type,
	revalidation_reason,
	type APIResultData,
} from "./model.ts";
import { derive_work_state } from "./update.ts";

const api_route_kind = {
	mutation: "mutation",
	query: "query",
} as const;

const http_method = {
	get: "GET",
	head: "HEAD",
} as const;

const no_revalidation_needed: RevalidationResult = { ok: true };

export type ClientShellOptions = {
	configure_client_options: (client_options: ClientOptions) => void;
	controller: CoreController;
	get_root_el: () => HTMLElement;
	host: Pick<CoreHost, "save_current_scroll">;
	id_source: IDSource;
	public_calls: PublicCallRegistry & {
		wait: (public_call_id: string) => Promise<unknown>;
	};
	work_indicator: WorkIndicator;
};

export function create_client_shell(options: ClientShellOptions): ClientCore {
	let default_error_boundary:
		| ((props: { error: unknown }) => any)
		| undefined;

	return {
		boot: async (client_options: ClientOptions) => {
			default_error_boundary = client_options.defaultErrorBoundary;
			options.configure_client_options(client_options);
			const browser_key = options.id_source.next_browser_key();
			const operation_id = options.id_source.next_operation_id();
			const public_call_id = options.id_source.next_public_call_id();
			const boot_result = options.public_calls.wait(public_call_id);
			options.controller.dispatch({
				type: event_type.boot_requested,
				browser_key,
				operation_id,
				options: client_options,
				public_call_id,
			});
			try {
				await boot_result;
				return R.ok(undefined);
			} catch (cause) {
				return R.err(String(cause));
			}
		},
		defineView: <T = any>(input: {
			beforeRouteCommit?: ViewDefinition["before_route_commit"];
			beforeRouteYield?: ViewDefinition["before_route_yield"];
			clientLoader?: (props: any) => Promise<T>;
			component: (props: any) => any;
			errorBoundary?: (props: { error: unknown }) => any;
			pattern: string;
			runClientLoaderOnHMR?: boolean;
		}) => {
			if (import.meta.env.DEV) {
				options.controller.dispatch({
					type: event_type.view_defined,
					pattern: input.pattern,
					run_client_loader_on_hmr:
						input.runClientLoaderOnHMR === true,
				});
			}
			return {
				before_route_commit: input.beforeRouteCommit,
				before_route_yield: input.beforeRouteYield,
				client_loader: input.clientLoader,
				component: input.component,
				error_boundary: input.errorBoundary,
				pattern: input.pattern,
			};
		},
		getClientBuildID: () => {
			return options.controller.get_state().client_build_id;
		},
		getRootEl: () => {
			return options.get_root_el();
		},
		getRouteState: () => {
			const current = options.controller.get_state().current;
			if (!current) {
				throw new Error(client_error_message.not_booted);
			}
			return current.route;
		},
		getWorkState: () => {
			const state = options.controller.get_state();
			if (!state.current) {
				throw new Error(client_error_message.not_booted);
			}
			return derive_work_state(state);
		},
		get_default_error_boundary: () => {
			return default_error_boundary;
		},
		navigate: async (href, nav_options) => {
			const operation_id = options.id_source.next_operation_id();
			const public_call_id = options.id_source.next_public_call_id();
			const navigation_result = options.public_calls.wait(public_call_id);
			options.controller.dispatch({
				type: event_type.navigation_requested,
				browser_key: options.id_source.next_browser_key(),
				href: String(href),
				operation_id,
				options: {
					replace: nav_options?.replace === true,
					scroll_to_top: nav_options?.scrollToTop,
					skip_work_indicator:
						nav_options?.skipWorkIndicator === true,
					state: nav_options?.state,
				},
				public_call_id,
			});
			return (await navigation_result) as { didNavigate: boolean };
		},
		revalidate: async () => {
			const operation_id = options.id_source.next_operation_id();
			const public_call_id = options.id_source.next_public_call_id();
			const revalidation_result =
				options.public_calls.wait(public_call_id);
			options.controller.dispatch({
				type: event_type.revalidation_requested,
				debounce: true,
				operation_id,
				public_call_id,
				reason: revalidation_reason.manual,
				skip_work_indicator: false,
			});
			return (await revalidation_result) as RevalidationResult;
		},
		save_current_scroll: () => {
			options.host.save_current_scroll();
		},
		start_prefetch: (href) => {
			options.controller.dispatch({
				type: event_type.prefetch_requested,
				href,
				operation_id: options.id_source.next_operation_id(),
			});
		},
		stop_prefetch: (href) => {
			options.controller.dispatch({
				type: event_type.prefetch_canceled,
				href,
			});
		},
		submit_inner: async <T = unknown>(
			url: string | URL,
			requestInit?: RequestInit,
			submit_options?: SubmitOptions,
		): Promise<APIResult<T>> => {
			const method = requestInit?.method
				? requestInit.method.toUpperCase().trim()
				: http_method.get;
			const route_kind =
				submit_options?.apiRouteKind ??
				(method === http_method.get || method === http_method.head
					? api_route_kind.query
					: api_route_kind.mutation);
			const revalidate =
				submit_options?.revalidate ??
				route_kind === api_route_kind.mutation;
			const public_call_id = options.id_source.next_public_call_id();
			const operation_id = options.id_source.next_operation_id();
			const redirect_operation_id = options.id_source.next_operation_id();
			const redirect_browser_key = options.id_source.next_browser_key();
			const revalidation_operation_id =
				options.id_source.next_operation_id();
			const revalidation_public_call_id = revalidate
				? options.id_source.next_public_call_id()
				: undefined;
			const submission_id =
				submit_options?.dedupeKey ??
				options.id_source.next_submission_id();
			const api_result = options.public_calls.wait(public_call_id);
			const revalidation_result = revalidation_public_call_id
				? options.public_calls.wait(revalidation_public_call_id)
				: Promise.resolve(no_revalidation_needed);
			options.controller.dispatch({
				type: event_type.api_submit_requested,
				api_route_kind: route_kind,
				href: String(url),
				method,
				operation_id,
				public_call_id,
				redirect_browser_key,
				redirect_operation_id,
				revalidate,
				revalidation_operation_id,
				revalidation_public_call_id,
				request_init: requestInit,
				skip_work_indicator: submit_options?.skipWorkIndicator === true,
				submission_id,
			});
			const result = (await api_result) as APIResultData;
			const revalidationPromise =
				revalidation_result as Promise<RevalidationResult>;
			if (result.kind === api_result_kind.success) {
				return {
					data: result.data as T,
					response: result.response,
					revalidationPromise,
					success: true,
				};
			}
			if (result.kind === api_result_kind.redirect) {
				return {
					data: undefined as T,
					response: result.response,
					revalidationPromise,
					success: true,
				};
			}
			return {
				error: result.error,
				response: result.response,
				revalidationPromise,
				success: false,
			};
		},
		workIndicator: options.work_indicator,
	};
}
