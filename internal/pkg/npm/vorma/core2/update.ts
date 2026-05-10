/// <reference types="vite/client" />

import type {
	RevalidationResult,
	RouteState,
	RouteUpdateReason,
} from "../core/types.ts";
import { is_abort_cause } from "./abort.ts";
import type {
	BuildSkewDetectedEvent,
	ClientCommit,
	RouteRenderState,
	ScrollIntent,
	ScrollState,
	WorkActivity,
	WorkState,
} from "./client_contract.ts";
import { work_activity_kind } from "./client_contract.ts";
import {
	api_result_kind,
	build_skew_default_behavior,
	build_skew_trigger_kind,
	client_error_message,
	client_phase,
	command_type,
	event_type,
	focus_revalidation_limit,
	history_write_kind,
	navigation_source,
	refresh_limit,
	refresh_status,
	revalidation_reason,
	revalidation_result_reason,
	route_fetch_trigger,
	route_limit,
	route_operation_kind,
	route_response_kind,
	type APIResultData,
	type BootOptions,
	type ClientCommand,
	type ClientEvent,
	type ClientState,
	type DeferredAPIRedirect,
	type FocusRevalidationState,
	type OperationID,
	type PreparedHMRRoute,
	type PreparedRoute,
	type PublicCallID,
	type RefreshDemand,
	type RouteFetchTrigger,
	type RouteOperation,
	type RouteResponseData,
	type SubmissionRecord,
	type UpdateResult,
} from "./model.ts";

const revalidation_ok: RevalidationResult = { ok: true };
const revalidation_build_skew: RevalidationResult = {
	ok: false,
	reason: revalidation_result_reason.build_skew,
};
const revalidation_exhausted: RevalidationResult = {
	ok: false,
	reason: revalidation_result_reason.max_retries_exhausted,
};
const url_parse_base = "https://v.invalid";

type NavigationRequestedEvent = Extract<
	ClientEvent,
	{ type: typeof event_type.navigation_requested }
>;

type PopstateObservedEvent = Extract<
	ClientEvent,
	{ type: typeof event_type.popstate_observed }
>;

type ActiveNavigationRetarget = {
	commands: ClientCommand[];
	operation: RouteOperation;
};

export function initial_client_state(): ClientState {
	return {
		active_route_operation: null,
		browser: null,
		client_build_id: "",
		current: null,
		default_error_boundary_enabled: false,
		deferred_api_redirect: null,
		deployment_id: "",
		focus_revalidation: null,
		hmr_operation: null,
		phase: client_phase.idle,
		prefetch: null,
		refresh: { kind: refresh_status.idle },
		registered_views: {},
		submissions: {},
		use_view_transitions: false,
		view_transition_operation_id: null,
	};
}

function focus_revalidation_from_boot_options(
	options: BootOptions,
): FocusRevalidationState | null {
	if (!options.revalidateOnWindowFocus) {
		return null;
	}
	return {
		last_activity_ms: null,
		stale_ms:
			typeof options.revalidateOnWindowFocus === "object"
				? options.revalidateOnWindowFocus.staleTimeMS
				: focus_revalidation_limit.default_stale_ms,
	};
}

function record_focus_activity(
	state: FocusRevalidationState | null,
	now_ms: number,
): FocusRevalidationState | null {
	if (!state) {
		return null;
	}
	return {
		...state,
		last_activity_ms: now_ms,
	};
}

export function update(state: ClientState, event: ClientEvent): UpdateResult {
	switch (event.type) {
		case event_type.boot_requested: {
			const operation: RouteOperation = {
				browser_key: event.browser_key,
				id: event.operation_id,
				kind: route_operation_kind.boot,
				href: "",
				public_call_ids: [event.public_call_id],
				redirect_count: 0,
				replace: true,
				scroll_to_top: undefined,
				source: navigation_source.navigate,
				state: undefined,
				skip_work_indicator: true,
			};
			return {
				state: {
					...state,
					active_route_operation: operation,
					default_error_boundary_enabled:
						event.options.defaultErrorBoundary !== undefined,
					focus_revalidation: focus_revalidation_from_boot_options(
						event.options,
					),
					phase: client_phase.booting,
					use_view_transitions:
						event.options.useViewTransitions === true,
				},
				commands: [
					{
						type: command_type.read_boot_payload,
						fallback_browser_key: event.browser_key,
						operation_id: operation.id,
					},
				],
			};
		}
		case event_type.boot_payload_read: {
			if (!is_current_route_operation(state, event.operation_id)) {
				return { state, commands: [] };
			}
			return {
				state: {
					...state,
					active_route_operation: state.active_route_operation
						? {
								...state.active_route_operation,
								boot_scroll: event.reload_scroll,
								browser_key: event.browser.key,
								href: event.browser.href,
								state: event.browser.state,
							}
						: null,
					browser: event.browser,
					client_build_id: event.payload.client_build_id,
					deployment_id: event.payload.deployment_id,
				},
				commands: [
					{
						type: command_type.prepare_route,
						client_build_id: event.payload.client_build_id,
						current_route: null,
						history_state: event.browser.state,
						href: event.browser.href,
						operation_id: event.operation_id,
						payload: event.payload.raw,
						trigger: route_operation_kind.boot,
					},
				],
			};
		}
		case event_type.boot_failed: {
			if (!is_current_route_operation(state, event.operation_id)) {
				return { state, commands: [] };
			}
			return {
				state: {
					...state,
					active_route_operation: null,
					phase: client_phase.idle,
				},
				commands: public_failure_commands(
					state.active_route_operation?.public_call_ids,
					event.cause,
				),
			};
		}
		case event_type.view_defined: {
			if (!import.meta.env.DEV) {
				return { state, commands: [] };
			}
			return {
				state: {
					...state,
					registered_views: {
						...state.registered_views,
						[event.pattern]: {
							pattern: event.pattern,
							run_client_loader_on_hmr:
								event.run_client_loader_on_hmr,
						},
					},
				},
				commands: [],
			};
		}
		case event_type.view_transition_completed: {
			if (state.view_transition_operation_id !== event.operation_id) {
				return { state, commands: [] };
			}
			return {
				state: {
					...state,
					view_transition_operation_id: null,
				},
				commands: [],
			};
		}
		case event_type.hmr_update_observed: {
			if (!import.meta.env.DEV) {
				return { state, commands: [] };
			}
			if (state.phase !== client_phase.ready || !state.current) {
				return { state, commands: [] };
			}
			const entry = state.current.render.entries.find((candidate) => {
				return candidate.module_url === event.module_url;
			});
			if (!entry) {
				return { state, commands: [] };
			}
			const commands: ClientCommand[] = [];
			if (state.hmr_operation) {
				commands.push({
					type: command_type.abort_operation,
					operation_id: state.hmr_operation.id,
				});
			}
			commands.push({
				type: command_type.prepare_hmr_route,
				module: event.module,
				module_url: event.module_url,
				operation_id: event.operation_id,
				position: state.current.position,
				render: state.current.render,
				rerun_client_loader:
					state.registered_views[entry.pattern]
						?.run_client_loader_on_hmr === true,
				route: state.current.route,
			});
			return {
				state: {
					...state,
					hmr_operation: {
						id: event.operation_id,
						module_url: event.module_url,
					},
				},
				commands,
			};
		}
		case event_type.hmr_prepared: {
			if (!import.meta.env.DEV) {
				return { state, commands: [] };
			}
			if (state.hmr_operation?.id !== event.operation_id) {
				return { state, commands: [] };
			}
			return publish_hmr_route(state, event.prepared);
		}
		case event_type.hmr_preparation_failed: {
			if (!import.meta.env.DEV) {
				return { state, commands: [] };
			}
			if (state.hmr_operation?.id !== event.operation_id) {
				return { state, commands: [] };
			}
			return {
				state: {
					...state,
					hmr_operation: null,
				},
				commands: [],
			};
		}
		case event_type.window_focus_observed: {
			if (
				state.phase !== client_phase.ready ||
				!state.current ||
				!state.focus_revalidation
			) {
				return { state, commands: [] };
			}
			if (
				state.active_route_operation ||
				state.refresh.kind !== refresh_status.idle ||
				Object.keys(state.submissions).length > 0
			) {
				return { state, commands: [] };
			}
			const last_activity_ms =
				state.focus_revalidation.last_activity_ms ?? event.now_ms;
			if (
				event.now_ms - last_activity_ms <
				state.focus_revalidation.stale_ms
			) {
				return { state, commands: [] };
			}
			return request_revalidation(
				state,
				{
					operation_id: event.operation_id,
					public_call_ids: [],
					reason: revalidation_reason.window_focus,
					skip_work_indicator: true,
				},
				true,
			);
		}
		case event_type.navigation_requested: {
			return navigation_requested(state, event);
		}
		case event_type.popstate_observed: {
			return popstate_observed(state, event);
		}
		case event_type.route_response_received: {
			if (state.prefetch?.id === event.operation_id) {
				const report_commands = report_prefetch_build_skew_commands(
					state,
					state.prefetch.href,
					event.response,
				);
				if (event.response.kind === route_response_kind.data) {
					return {
						state,
						commands: report_commands.concat([
							{
								type: command_type.prepare_route,
								client_build_id: state.client_build_id,
								current_route: state.current?.route ?? null,
								history_state: undefined,
								href: state.prefetch.href,
								operation_id: event.operation_id,
								payload: event.response.payload,
								trigger: route_fetch_trigger.prefetch,
							},
						]),
					};
				}
				const next_state = {
					...state,
					prefetch: null,
				};
				const commands: ClientCommand[] = report_commands;
				return finish_work_transition(state, next_state, commands);
			}
			if (
				is_background_revalidation_response(state, event.operation_id)
			) {
				return background_revalidation_response_received(
					state,
					event.response,
				);
			}
			if (!is_current_route_operation(state, event.operation_id)) {
				return { state, commands: [] };
			}
			return route_response_commands(
				state,
				event.operation_id,
				event.response,
			);
		}
		case event_type.route_response_failed: {
			if (state.prefetch?.id === event.operation_id) {
				const next_state = {
					...state,
					prefetch: null,
				};
				const commands: ClientCommand[] = [];
				return finish_work_transition(state, next_state, commands);
			}
			if (
				is_background_revalidation_response(state, event.operation_id)
			) {
				return route_operation_failed(state, event.cause);
			}
			if (!is_current_route_operation(state, event.operation_id)) {
				return { state, commands: [] };
			}
			return route_operation_failed(state, event.cause);
		}
		case event_type.route_prepared: {
			if (state.prefetch?.id === event.operation_id) {
				const next_state = {
					...state,
					prefetch: {
						...state.prefetch,
						is_pending: false,
						prepared: event.prepared,
					},
				};
				const commands: ClientCommand[] = [];
				return finish_work_transition(state, next_state, commands);
			}
			if (!is_current_route_operation(state, event.operation_id)) {
				return { state, commands: [] };
			}
			const operation = state.active_route_operation;
			if (!operation) {
				return { state, commands: [] };
			}
			if (operation.kind !== route_operation_kind.boot) {
				return {
					state,
					commands: [
						{
							type: command_type.run_route_hooks,
							current_render: state.current?.render ?? null,
							current_route: state.current?.route ?? null,
							operation_id: event.operation_id,
							prepared: event.prepared,
							trigger:
								route_update_reason_for_operation(operation),
						},
					],
				};
			}
			return publish_prepared_route(state, event.prepared, event.now_ms);
		}
		case event_type.route_provisionally_prepared: {
			if (!is_current_route_operation(state, event.operation_id)) {
				return { state, commands: [] };
			}
			const operation = state.active_route_operation;
			if (
				!operation ||
				operation.kind !== route_operation_kind.boot ||
				state.current
			) {
				return { state, commands: [] };
			}
			const position = {
				href: operation.href || event.prepared.route.href,
				key: operation.browser_key,
				state: operation.state,
			};
			return {
				state: {
					...state,
					current: {
						position,
						provisional: true,
						render: event.prepared.render,
						route: event.prepared.route,
						scroll_intent: scroll_intent_for_route(
							operation,
							event.prepared.route,
						),
					},
				},
				commands: [],
			};
		}
		case event_type.route_preparation_failed: {
			if (state.prefetch?.id === event.operation_id) {
				const next_state = {
					...state,
					prefetch: null,
				};
				const commands: ClientCommand[] = [];
				return finish_work_transition(state, next_state, commands);
			}
			if (!is_current_route_operation(state, event.operation_id)) {
				return { state, commands: [] };
			}
			return route_operation_failed(state, event.cause);
		}
		case event_type.route_hooks_completed: {
			if (!is_current_route_operation(state, event.operation_id)) {
				return { state, commands: [] };
			}
			return publish_prepared_route(state, event.prepared, event.now_ms);
		}
		case event_type.route_hooks_failed: {
			if (!is_current_route_operation(state, event.operation_id)) {
				return { state, commands: [] };
			}
			return route_operation_failed(state, event.cause);
		}
		case event_type.revalidation_requested: {
			if (state.phase !== client_phase.ready || !state.current) {
				return {
					state,
					commands: event.public_call_id
						? [
								{
									type: command_type.resolve_public_call,
									public_call_id: event.public_call_id,
									result: revalidation_ok,
								},
							]
						: [],
				};
			}
			const demand: RefreshDemand = {
				after_operation_id: state.active_route_operation?.id,
				operation_id: event.operation_id,
				public_call_ids: event.public_call_id
					? [event.public_call_id]
					: [],
				reason: event.reason,
				skip_work_indicator: event.skip_work_indicator,
			};
			return request_revalidation(state, demand, event.debounce);
		}
		case event_type.refresh_timer_fired: {
			if (
				state.refresh.kind === refresh_status.retrying &&
				state.refresh.timer_id === event.timer_id
			) {
				if (
					event.operation_id &&
					event.operation_id !== state.refresh.demand.operation_id
				) {
					return { state, commands: [] };
				}
				if (state.active_route_operation) {
					const next_state = {
						...state,
						refresh: {
							kind: refresh_status.pending,
							demand: state.refresh.demand,
							attempt: state.refresh.attempt,
						},
					};
					const commands: ClientCommand[] = [];
					return finish_work_transition(state, next_state, commands);
				}
				return begin_revalidation(
					state,
					state.refresh.demand,
					state.refresh.attempt,
				);
			}
			if (
				state.refresh.kind !== refresh_status.debouncing ||
				state.refresh.timer_id !== event.timer_id
			) {
				return { state, commands: [] };
			}
			if (
				event.operation_id &&
				event.operation_id !== state.refresh.demand.operation_id
			) {
				return { state, commands: [] };
			}
			if (state.active_route_operation) {
				const next_state = {
					...state,
					refresh: {
						kind: refresh_status.pending,
						demand: state.refresh.demand,
						attempt: 0,
					},
				};
				const commands: ClientCommand[] = [];
				return finish_work_transition(state, next_state, commands);
			}
			return begin_revalidation(state, state.refresh.demand, 0);
		}
		case event_type.api_submit_requested: {
			const previous_submission = state.submissions[event.submission_id];
			const href = state.browser
				? resolve_href(event.href, state.browser.href)
				: event.href;
			const submission = {
				id: event.submission_id,
				api_route_kind: event.api_route_kind,
				href,
				method: event.method,
				operation_id: event.operation_id,
				public_call_id: event.public_call_id,
				redirect_browser_key: event.redirect_browser_key,
				redirect_operation_id: event.redirect_operation_id,
				revalidate: event.revalidate,
				revalidation_operation_id: event.revalidation_operation_id,
				revalidation_public_call_id: event.revalidation_public_call_id,
				skip_work_indicator: event.skip_work_indicator,
			};
			const commands: ClientCommand[] = [];
			let deduped_revalidation_demand: RefreshDemand | undefined;
			if (previous_submission) {
				commands.push({
					type: command_type.abort_operation,
					operation_id: previous_submission.operation_id,
				});
				commands.push({
					type: command_type.resolve_public_call,
					public_call_id: previous_submission.public_call_id,
					result: {
						kind: api_result_kind.failure,
						error: client_error_message.aborted,
					},
				});
				if (previous_submission.revalidation_public_call_id) {
					deduped_revalidation_demand = {
						after_operation_id: state.active_route_operation?.id,
						operation_id:
							previous_submission.revalidation_operation_id,
						public_call_ids: [
							previous_submission.revalidation_public_call_id,
						],
						reason: revalidation_reason.api_request,
						skip_work_indicator:
							previous_submission.skip_work_indicator,
					};
				}
			}
			const next_state = {
				...state,
				submissions: {
					...state.submissions,
					[event.submission_id]: submission,
				},
			};
			commands.push({
				type: command_type.fetch_api,
				deployment_id: state.deployment_id,
				submission_id: event.submission_id,
				href,
				method: event.method,
				operation_id: event.operation_id,
				request_init: event.request_init,
			});
			const transition = finish_work_transition(
				state,
				next_state,
				commands,
			);
			if (deduped_revalidation_demand) {
				const revalidation = request_revalidation(
					transition.state,
					deduped_revalidation_demand,
					false,
				);
				return {
					state: revalidation.state,
					commands: transition.commands.concat(revalidation.commands),
				};
			}
			return transition;
		}
		case event_type.api_response_received: {
			const submission = state.submissions[event.submission_id];
			if (!submission || submission.operation_id !== event.operation_id) {
				return { state, commands: [] };
			}
			const next_submissions = { ...state.submissions };
			delete next_submissions[event.submission_id];
			const next_state = {
				...state,
				submissions: next_submissions,
			};
			const commands: ClientCommand[] = [];
			const skew_command = api_build_skew_command(
				state,
				submission,
				event.result,
			);
			if (skew_command) {
				commands.push(skew_command);
			}
			commands.push({
				type: command_type.resolve_public_call,
				public_call_id: submission.public_call_id,
				result:
					event.result.kind === api_result_kind.redirect
						? {
								kind: api_result_kind.success,
								data: undefined,
								ok: event.result.ok,
								response: event.result.response,
								server_build_id: event.result.server_build_id,
								status: event.result.status,
							}
						: event.result,
			});
			if (event.result.kind === api_result_kind.redirect) {
				if (submission.revalidation_public_call_id) {
					commands.push({
						type: command_type.resolve_public_call,
						public_call_id: submission.revalidation_public_call_id,
						result: revalidation_ok,
					});
				}
				if (event.result.hard) {
					commands.push({
						type: command_type.hard_redirect,
						href: event.result.href,
					});
					return finish_work_transition(state, next_state, commands);
				}
				if (state.phase !== client_phase.ready || !state.current) {
					const next_state_with_redirect = {
						...next_state,
						deferred_api_redirect: {
							browser_key: submission.redirect_browser_key,
							href: event.result.href,
							operation_id: submission.redirect_operation_id,
							skip_work_indicator: submission.skip_work_indicator,
						},
					};
					return finish_work_transition(
						state,
						next_state_with_redirect,
						commands,
					);
				}
				const operation: RouteOperation = {
					browser_key: submission.redirect_browser_key,
					id: submission.redirect_operation_id,
					kind: route_operation_kind.navigation,
					href: event.result.href,
					public_call_ids: [],
					redirect_count: 0,
					replace: true,
					scroll_to_top: undefined,
					source: navigation_source.redirect,
					state: undefined,
					skip_work_indicator: submission.skip_work_indicator,
				};
				if (state.active_route_operation) {
					commands.push({
						type: command_type.abort_operation,
						operation_id: state.active_route_operation.id,
					});
					commands.push(
						...resolve_public_call_commands(
							state.active_route_operation.public_call_ids,
							{ didNavigate: false },
						),
					);
				}
				const next_state_with_redirect = {
					...next_state,
					active_route_operation: operation,
				};
				return finish_work_transition(
					state,
					next_state_with_redirect,
					commands,
					[
						{
							type: command_type.fetch_route,
							client_build_id: state.client_build_id,
							deployment_id: state.deployment_id,
							current_route: state.current.route,
							history_state: operation.state,
							operation_id: operation.id,
							href: operation.href,
							trigger: route_fetch_trigger.navigation,
						},
					],
				);
			}
			if (
				submission.revalidate &&
				(event.result.kind === api_result_kind.success ||
					(event.result.kind === api_result_kind.failure &&
						event.result.should_revalidate === true))
			) {
				return request_submission_revalidation(
					state,
					next_state,
					submission,
					commands,
				);
			}
			if (submission.revalidation_public_call_id) {
				commands.push({
					type: command_type.resolve_public_call,
					public_call_id: submission.revalidation_public_call_id,
					result: revalidation_ok,
				});
			}
			return finish_work_transition(state, next_state, commands);
		}
		case event_type.api_response_failed: {
			const submission = state.submissions[event.submission_id];
			if (!submission || submission.operation_id !== event.operation_id) {
				return { state, commands: [] };
			}
			const next_submissions = { ...state.submissions };
			delete next_submissions[event.submission_id];
			const next_state = {
				...state,
				submissions: next_submissions,
			};
			const commands: ClientCommand[] = [];
			const transition = finish_work_transition(
				state,
				next_state,
				commands,
				[
					{
						type: command_type.resolve_public_call,
						public_call_id: submission.public_call_id,
						result: {
							kind: api_result_kind.failure,
							error: error_message(event.cause),
							should_revalidate: true,
						},
					},
				],
			);
			if (submission.revalidate) {
				return request_submission_revalidation(
					state,
					next_state,
					submission,
					transition.commands,
				);
			}
			if (!submission.revalidation_public_call_id) {
				return transition;
			}
			return {
				state: transition.state,
				commands: transition.commands.concat({
					type: command_type.resolve_public_call,
					public_call_id: submission.revalidation_public_call_id,
					result: revalidation_ok,
				}),
			};
		}
		case event_type.prefetch_requested: {
			const commands: ClientCommand[] = [];
			const href = state.browser
				? resolve_href(event.href, state.browser.href)
				: event.href;
			const current_url = state.browser
				? parse_href(state.browser.href, url_parse_base)
				: null;
			const prefetch_url = parse_href(
				href,
				current_url?.href ?? url_parse_base,
			);
			if (
				!current_url ||
				!prefetch_url ||
				current_url.origin !== prefetch_url.origin ||
				same_document_href(state.browser?.href ?? "", href) ||
				(state.active_route_operation &&
					same_document_href(state.active_route_operation.href, href))
			) {
				return { state, commands: [] };
			}
			if (state.prefetch) {
				commands.push({
					type: command_type.abort_operation,
					operation_id: state.prefetch.id,
				});
			}
			if (state.hmr_operation) {
				commands.push({
					type: command_type.abort_operation,
					operation_id: state.hmr_operation.id,
				});
			}
			const next_state = {
				...state,
				prefetch: {
					id: event.operation_id,
					href,
					is_pending: true,
				},
			};
			return finish_work_transition(state, next_state, commands, [
				{
					type: command_type.fetch_route,
					client_build_id: state.client_build_id,
					deployment_id: state.deployment_id,
					current_route: state.current?.route ?? null,
					history_state: undefined,
					operation_id: event.operation_id,
					href,
					trigger: route_fetch_trigger.prefetch,
				},
			]);
		}
		case event_type.prefetch_canceled: {
			const href = state.browser
				? resolve_href(event.href, state.browser.href)
				: event.href;
			if (
				!state.prefetch ||
				!same_document_href(state.prefetch.href, href)
			) {
				return { state, commands: [] };
			}
			const prefetch = state.prefetch;
			const next_state = {
				...state,
				prefetch: null,
			};
			const commands: ClientCommand[] = [
				{
					type: command_type.abort_operation,
					operation_id: prefetch.id,
				},
			];
			return finish_work_transition(state, next_state, commands);
		}
	}
}

function navigation_requested(
	state: ClientState,
	event: NavigationRequestedEvent,
): UpdateResult {
	if (state.phase !== client_phase.ready || !state.current) {
		return {
			state,
			commands: [
				{
					type: command_type.reject_public_call,
					public_call_id: event.public_call_id,
					cause: client_error_message.not_booted,
				},
			],
		};
	}
	const current_url = state.browser
		? parse_href(state.browser.href, url_parse_base)
		: null;
	const requested_url = state.browser
		? parse_href(event.href, state.browser.href)
		: null;
	const requested_href = requested_url?.href ?? event.href;
	if (
		current_url &&
		requested_url &&
		current_url.origin !== requested_url.origin
	) {
		return {
			state,
			commands: [
				{
					type: command_type.hard_redirect,
					href: requested_href,
				},
				{
					type: command_type.resolve_public_call,
					public_call_id: event.public_call_id,
					result: { didNavigate: false },
				},
			],
		};
	}
	const operation: RouteOperation = {
		browser_key: event.browser_key,
		id: event.operation_id,
		kind: route_operation_kind.navigation,
		href: requested_href,
		public_call_ids: [event.public_call_id],
		redirect_count: 0,
		replace: event.options.replace,
		scroll_to_top: event.options.scroll_to_top,
		source: navigation_source.navigate,
		state: event.options.state,
		skip_work_indicator: event.options.skip_work_indicator,
	};
	const view_transition_commands: ClientCommand[] =
		state.view_transition_operation_id
			? [
					{
						type: command_type.abort_operation,
						operation_id: state.view_transition_operation_id,
					},
				]
			: [];
	const retarget = retarget_active_navigation(
		state.active_route_operation,
		operation,
	);
	if (retarget) {
		const next_state = {
			...state,
			active_route_operation: retarget.operation,
			view_transition_operation_id: null,
		};
		return finish_work_transition(
			state,
			next_state,
			view_transition_commands.concat(retarget.commands),
		);
	}
	const route_interruption_commands = view_transition_commands.concat(
		cancel_active_route_commands(state.active_route_operation),
		state.hmr_operation
			? [
					{
						type: command_type.abort_operation,
						operation_id: state.hmr_operation.id,
					},
				]
			: [],
	);
	if (
		state.browser &&
		same_document_href(state.browser.href, requested_href)
	) {
		const should_write_history =
			requested_href !== state.browser.href ||
			event.options.replace ||
			event.options.state !== state.browser.state;
		const position = should_write_history
			? {
					href: requested_href,
					key: event.browser_key,
					state: event.options.state,
				}
			: state.browser;
		const history_commands: ClientCommand[] = [
			{ type: command_type.save_current_scroll },
		];
		if (should_write_history) {
			history_commands.push({
				type: command_type.write_history,
				kind: event.options.replace
					? history_write_kind.replace
					: history_write_kind.push,
				href: requested_href,
				key: event.browser_key,
				state: event.options.state,
			});
		}
		return publish_same_document_route(
			state,
			operation,
			position,
			route_interruption_commands.concat(history_commands),
		);
	}
	if (
		state.prefetch?.prepared &&
		same_document_href(state.prefetch.href, requested_href)
	) {
		const position = {
			href: requested_href,
			key: event.browser_key,
			state: event.options.state,
		};
		const prepared = prepared_route_at_position(
			state.prefetch.prepared,
			position,
		);
		const next_state = {
			...state,
			active_route_operation: operation,
			hmr_operation: null,
			prefetch: null,
			view_transition_operation_id: null,
		};
		return finish_work_transition(
			state,
			next_state,
			route_interruption_commands,
			[
				{
					type: command_type.run_route_hooks,
					current_render: state.current.render,
					current_route: state.current.route,
					operation_id: operation.id,
					prepared,
					trigger: route_operation_kind.navigation,
				},
			],
		);
	}
	const next_state = {
		...state,
		active_route_operation: operation,
		hmr_operation: null,
		prefetch: null,
		view_transition_operation_id: null,
	};
	return finish_work_transition(
		state,
		next_state,
		route_interruption_commands.concat(
			state.prefetch
				? [
						{
							type: command_type.abort_operation,
							operation_id: state.prefetch.id,
						},
					]
				: [],
		),
		[
			{
				type: command_type.fetch_route,
				client_build_id: state.client_build_id,
				deployment_id: state.deployment_id,
				current_route: state.current.route,
				history_state: operation.state,
				operation_id: operation.id,
				href: operation.href,
				trigger: route_fetch_trigger.navigation,
			},
		],
	);
}

function popstate_observed(
	state: ClientState,
	event: PopstateObservedEvent,
): UpdateResult {
	if (state.phase !== client_phase.ready || !state.current) {
		return { state, commands: [] };
	}
	if (
		state.browser &&
		state.browser.key === event.browser.key &&
		state.browser.href === event.browser.href
	) {
		return { state, commands: [] };
	}
	const operation: RouteOperation = {
		browser_key: event.browser.key,
		id: event.operation_id,
		kind: route_operation_kind.popstate,
		href: event.browser.href,
		popstate_scroll: event.popstate_scroll,
		public_call_ids: [],
		redirect_count: 0,
		replace: true,
		scroll_to_top: undefined,
		source: navigation_source.popstate,
		state: event.browser.state,
		skip_work_indicator: false,
	};
	const scroll_commands: ClientCommand[] =
		state.browser?.key && state.browser.key !== event.browser.key
			? [
					{
						type: command_type.save_scroll_position,
						key: state.browser.key,
						scroll: event.leaving_scroll,
					},
				]
			: [];
	const active_operation = state.active_route_operation;
	if (
		(active_operation?.kind === route_operation_kind.navigation ||
			active_operation?.kind === route_operation_kind.popstate) &&
		same_document_href(active_operation.href, operation.href)
	) {
		const next_state = {
			...state,
			active_route_operation: {
				...operation,
				id: active_operation.id,
				redirect_count: active_operation.redirect_count,
			},
			browser: event.browser,
			hmr_operation: null,
			prefetch: null,
		};
		return finish_work_transition(
			state,
			next_state,
			scroll_commands.concat(
				resolve_public_call_commands(active_operation.public_call_ids, {
					didNavigate: false,
				}),
				state.hmr_operation
					? [
							{
								type: command_type.abort_operation,
								operation_id: state.hmr_operation.id,
							},
						]
					: [],
				state.prefetch
					? [
							{
								type: command_type.abort_operation,
								operation_id: state.prefetch.id,
							},
						]
					: [],
			),
		);
	}
	const route_interruption_commands = scroll_commands.concat(
		cancel_active_route_commands(state.active_route_operation),
		state.hmr_operation
			? [
					{
						type: command_type.abort_operation,
						operation_id: state.hmr_operation.id,
					},
				]
			: [],
	);
	if (
		state.browser &&
		same_document_href(state.browser.href, event.browser.href)
	) {
		return publish_same_document_route(
			state,
			operation,
			event.browser,
			route_interruption_commands,
		);
	}
	if (
		state.prefetch?.prepared &&
		same_document_href(state.prefetch.href, event.browser.href)
	) {
		const prepared = prepared_route_at_position(
			state.prefetch.prepared,
			event.browser,
		);
		const next_state = {
			...state,
			active_route_operation: operation,
			browser: event.browser,
			hmr_operation: null,
			prefetch: null,
		};
		return finish_work_transition(
			state,
			next_state,
			route_interruption_commands,
			[
				{
					type: command_type.run_route_hooks,
					current_render: state.current.render,
					current_route: state.current.route,
					operation_id: operation.id,
					prepared,
					trigger: route_operation_kind.popstate,
				},
			],
		);
	}
	const next_state = {
		...state,
		active_route_operation: operation,
		browser: event.browser,
		hmr_operation: null,
		prefetch: null,
	};
	return finish_work_transition(
		state,
		next_state,
		route_interruption_commands.concat(
			state.prefetch
				? [
						{
							type: command_type.abort_operation,
							operation_id: state.prefetch.id,
						},
					]
				: [],
		),
		[
			{
				type: command_type.fetch_route,
				client_build_id: state.client_build_id,
				deployment_id: state.deployment_id,
				current_route: state.current.route,
				history_state: event.browser.state,
				operation_id: operation.id,
				href: operation.href,
				trigger: route_fetch_trigger.popstate,
			},
		],
	);
}

function retarget_active_navigation(
	active_operation: RouteOperation | null,
	operation: RouteOperation,
): ActiveNavigationRetarget | null {
	if (
		active_operation?.kind !== route_operation_kind.navigation ||
		!same_document_href(active_operation.href, operation.href)
	) {
		return null;
	}
	const same_intent =
		active_operation.href === operation.href &&
		active_operation.replace === operation.replace &&
		active_operation.scroll_to_top === operation.scroll_to_top &&
		active_operation.state === operation.state &&
		active_operation.skip_work_indicator ===
			operation.skip_work_indicator &&
		active_operation.source === operation.source;
	if (same_intent) {
		return {
			commands: [],
			operation: {
				...active_operation,
				public_call_ids: active_operation.public_call_ids.concat(
					operation.public_call_ids,
				),
			},
		};
	}
	return {
		commands: resolve_public_call_commands(
			active_operation.public_call_ids,
			{ didNavigate: false },
		),
		operation: {
			...operation,
			id: active_operation.id,
			redirect_count: active_operation.redirect_count,
		},
	};
}

function cancel_active_route_commands(
	operation: RouteOperation | null,
): ClientCommand[] {
	if (!operation) {
		return [];
	}
	const commands: ClientCommand[] = [
		{
			type: command_type.abort_operation,
			operation_id: operation.id,
		},
	];
	return commands.concat(
		resolve_public_call_commands(operation.public_call_ids, {
			didNavigate: false,
		}),
	);
}

function is_current_route_operation(
	state: ClientState,
	operation_id: OperationID,
): boolean {
	return state.active_route_operation?.id === operation_id;
}

function report_prefetch_build_skew_commands(
	state: ClientState,
	href: string,
	response: RouteResponseData,
): ClientCommand[] {
	return report_build_skew_commands(
		state,
		response.server_build_id,
		build_skew_default_behavior.notify_only,
		{
			kind: build_skew_trigger_kind.route,
			ok: response.ok,
			requestedHref: href,
			status: response.status,
			trigger: route_fetch_trigger.prefetch,
		},
	);
}

function report_route_build_skew_commands(
	state: ClientState,
	operation: RouteOperation | null,
	response: RouteResponseData,
	default_behavior: BuildSkewDetectedEvent["defaultBehavior"],
): ClientCommand[] {
	if (!operation) {
		return [];
	}
	return report_build_skew_commands(
		state,
		response.server_build_id,
		default_behavior,
		route_build_skew_triggering_response(state, operation, response),
	);
}

function route_build_skew_triggering_response(
	state: ClientState,
	operation: RouteOperation,
	response: RouteResponseData,
): Extract<BuildSkewDetectedEvent["triggeringResponse"], { kind: "route" }> {
	const base = {
		kind: build_skew_trigger_kind.route,
		ok: response.ok,
		requestedHref: operation.href,
		status: response.status,
	};
	if (operation.kind === route_operation_kind.revalidation) {
		return {
			...base,
			revalidationReason:
				state.refresh.kind === refresh_status.running
					? state.refresh.demand.reason
					: revalidation_reason.manual,
			trigger: route_fetch_trigger.revalidation,
		};
	}
	return {
		...base,
		trigger:
			operation.kind === route_operation_kind.popstate
				? route_fetch_trigger.popstate
				: route_fetch_trigger.navigation,
	};
}

function route_redirect_skew_default_behavior(
	operation: RouteOperation,
	response: Extract<RouteResponseData, { kind: "redirect" }>,
): BuildSkewDetectedEvent["defaultBehavior"] {
	if (!response.hard) {
		return build_skew_default_behavior.notify_only;
	}
	if (operation.kind === route_operation_kind.revalidation) {
		return build_skew_default_behavior.drop_response;
	}
	return build_skew_default_behavior.hard_reload;
}

function api_build_skew_command(
	state: ClientState,
	submission: SubmissionRecord,
	result: APIResultData,
): ClientCommand | undefined {
	if (
		!result.server_build_id ||
		typeof result.status !== "number" ||
		!is_build_skew_detected(state, result.server_build_id)
	) {
		return undefined;
	}
	return {
		type: command_type.report_build_skew,
		event: {
			activeClientBuildID: state.client_build_id,
			currentRouteState: state.current!.route,
			currentWorkState: derive_work_state(state),
			defaultBehavior:
				result.kind === api_result_kind.redirect && result.hard
					? build_skew_default_behavior.hard_reload
					: build_skew_default_behavior.notify_only,
			serverBuildID: result.server_build_id,
			triggeringResponse: {
				kind: build_skew_trigger_kind.api_route,
				apiRouteKind: submission.api_route_kind,
				method: submission.method,
				ok: result.ok === true,
				requestedHref: submission.href,
				status: result.status,
			},
		},
	};
}

function report_build_skew_commands(
	state: ClientState,
	server_build_id: string,
	default_behavior: BuildSkewDetectedEvent["defaultBehavior"],
	triggering_response: BuildSkewDetectedEvent["triggeringResponse"],
): ClientCommand[] {
	if (!is_build_skew_detected(state, server_build_id)) {
		return [];
	}
	return [
		{
			type: command_type.report_build_skew,
			event: {
				activeClientBuildID: state.client_build_id,
				currentRouteState: state.current!.route,
				currentWorkState: derive_work_state(state),
				defaultBehavior: default_behavior,
				serverBuildID: server_build_id,
				triggeringResponse: triggering_response,
			},
		},
	];
}

function is_build_skew_detected(
	state: ClientState,
	server_build_id: string,
): boolean {
	return (
		!!state.current &&
		server_build_id.length > 0 &&
		server_build_id !== state.client_build_id
	);
}

function route_response_commands(
	state: ClientState,
	operation_id: OperationID,
	response: RouteResponseData,
): UpdateResult {
	switch (response.kind) {
		case route_response_kind.data: {
			const commands = report_route_build_skew_commands(
				state,
				state.active_route_operation,
				response,
				build_skew_default_behavior.notify_only,
			);
			commands.push({
				type: command_type.prepare_route,
				client_build_id: state.client_build_id,
				current_route: state.current?.route ?? null,
				history_state: state.active_route_operation?.state,
				href:
					state.active_route_operation?.href ??
					state.current?.route.href ??
					"",
				operation_id,
				payload: response.payload,
				trigger: state.active_route_operation
					? route_update_reason_for_operation(
							state.active_route_operation,
						)
					: route_operation_kind.navigation,
			});
			return {
				state,
				commands,
			};
		}
		case route_response_kind.redirect: {
			if (!state.active_route_operation) {
				return { state, commands: [] };
			}
			const operation = state.active_route_operation;
			const commands = report_route_build_skew_commands(
				state,
				operation,
				response,
				route_redirect_skew_default_behavior(operation, response),
			);
			if (!response.http) {
				if (
					operation.kind === route_operation_kind.revalidation &&
					state.refresh.kind === refresh_status.running
				) {
					const next_state = {
						...state,
						active_route_operation: null,
						refresh: { kind: refresh_status.idle },
					};
					commands.push(
						...resolve_public_call_commands(
							state.refresh.demand.public_call_ids,
							revalidation_ok,
						),
					);
					return finish_work_transition(state, next_state, commands);
				}
				const next_state = {
					...state,
					active_route_operation: null,
				};
				commands.push(
					...resolve_public_call_commands(operation.public_call_ids, {
						didNavigate: false,
					}),
				);
				return finish_work_transition(state, next_state, commands);
			}
			if (
				response.hard &&
				operation.kind === route_operation_kind.revalidation &&
				is_build_skew_detected(state, response.server_build_id)
			) {
				const next_state = {
					...state,
					active_route_operation: null,
					refresh: { kind: refresh_status.idle },
				};
				if (state.refresh.kind === refresh_status.running) {
					commands.push(
						...resolve_public_call_commands(
							state.refresh.demand.public_call_ids,
							revalidation_build_skew,
						),
					);
				}
				return finish_work_transition(state, next_state, commands);
			}
			if (
				!response.hard &&
				operation.redirect_count < route_limit.max_redirects
			) {
				const next_operation: RouteOperation = {
					...operation,
					href: response.href,
					redirect_count: operation.redirect_count + 1,
					source: navigation_source.redirect,
				};
				const next_state = {
					...state,
					active_route_operation: next_operation,
				};
				return finish_work_transition(state, next_state, commands, [
					{
						type: command_type.fetch_route,
						client_build_id: state.client_build_id,
						deployment_id: state.deployment_id,
						current_route: state.current?.route ?? null,
						history_state: next_operation.state,
						operation_id: next_operation.id,
						href: next_operation.href,
						trigger:
							route_fetch_trigger_for_operation(next_operation),
					},
				]);
			}
			const next_state = {
				...state,
				active_route_operation: null,
			};
			if (response.hard) {
				commands.push({
					type: command_type.hard_redirect,
					href: response.href,
				});
			}
			commands.push(
				...resolve_public_call_commands(operation.public_call_ids, {
					didNavigate: false,
				}),
			);
			return finish_work_transition(state, next_state, commands);
		}
		case route_response_kind.build_skew: {
			const commands = report_route_build_skew_commands(
				state,
				state.active_route_operation,
				response,
				state.refresh.kind === refresh_status.running
					? build_skew_default_behavior.drop_response
					: build_skew_default_behavior.hard_reload,
			);
			if (state.refresh.kind === refresh_status.running) {
				const next_state = {
					...state,
					active_route_operation: null,
					refresh: { kind: refresh_status.idle },
				};
				commands.push(
					...resolve_public_call_commands(
						state.refresh.demand.public_call_ids,
						revalidation_build_skew,
					),
				);
				return finish_work_transition(state, next_state, commands);
			}
			if (state.active_route_operation) {
				commands.push({
					type: command_type.hard_redirect,
					href: state.active_route_operation.href,
				});
			}
			const next_state = {
				...state,
				active_route_operation: null,
			};
			commands.push(
				...resolve_public_call_commands(
					state.active_route_operation?.public_call_ids ?? [],
					{ didNavigate: false },
				),
			);
			return finish_work_transition(state, next_state, commands);
		}
		case route_response_kind.error: {
			const commands = report_route_build_skew_commands(
				state,
				state.active_route_operation,
				response,
				build_skew_default_behavior.notify_only,
			);
			const failure = route_operation_failed(state, response.status_text);
			return {
				state: failure.state,
				commands: commands.concat(failure.commands),
			};
		}
	}
}

function is_background_revalidation_response(
	state: ClientState,
	operation_id: OperationID,
): boolean {
	return (
		state.refresh.kind === refresh_status.running &&
		state.refresh.operation_id === operation_id &&
		state.active_route_operation?.id !== operation_id
	);
}

function background_revalidation_response_received(
	state: ClientState,
	response: RouteResponseData,
): UpdateResult {
	const result =
		response.kind === route_response_kind.build_skew
			? revalidation_build_skew
			: revalidation_ok;
	const next_state = {
		...state,
		refresh: { kind: refresh_status.idle },
	};
	const commands =
		state.refresh.kind === refresh_status.running
			? resolve_public_call_commands(
					state.refresh.demand.public_call_ids,
					result,
				)
			: [];
	return finish_work_transition(state, next_state, commands);
}

function route_operation_failed(
	state: ClientState,
	cause: unknown,
): UpdateResult {
	const aborted = is_abort_cause(cause);
	if (state.refresh.kind === refresh_status.running) {
		if (aborted) {
			const next_state = {
				...state,
				active_route_operation: null,
				refresh: { kind: refresh_status.idle },
			};
			const commands = resolve_public_call_commands(
				state.refresh.demand.public_call_ids,
				revalidation_ok,
			);
			return finish_work_transition(state, next_state, commands);
		}
		const next_attempt = state.refresh.attempt + 1;
		if (next_attempt > refresh_limit.max_retries) {
			const next_state = {
				...state,
				active_route_operation: null,
				refresh: { kind: refresh_status.idle },
			};
			const commands = resolve_public_call_commands(
				state.refresh.demand.public_call_ids,
				revalidation_exhausted,
			);
			return finish_work_transition(state, next_state, commands);
		}
		const operation_id = `${state.refresh.demand.operation_id}:retry:${next_attempt}`;
		const timer_id = `${operation_id}:timer`;
		const demand: RefreshDemand = {
			...state.refresh.demand,
			operation_id,
			reason: revalidation_reason.retry,
		};
		const next_state = {
			...state,
			active_route_operation: null,
			refresh: {
				kind: refresh_status.retrying,
				attempt: next_attempt,
				demand,
				timer_id,
			},
		};
		const commands: ClientCommand[] = [];
		return finish_work_transition(state, next_state, commands, [
			{
				type: command_type.start_timer,
				delay_ms: Math.min(
					refresh_limit.backoff_cap_ms,
					refresh_limit.backoff_base_ms * 2 ** (next_attempt - 1),
				),
				operation_id,
				timer_id,
			},
		]);
	}
	const next_state = {
		...state,
		active_route_operation: null,
	};
	const operation = state.active_route_operation;
	const commands = aborted
		? resolve_public_call_commands(operation?.public_call_ids ?? [], {
				didNavigate: false,
			})
		: operation &&
			  operation.kind !== route_operation_kind.boot &&
			  operation.kind !== route_operation_kind.revalidation
			? resolve_public_call_commands(operation.public_call_ids, {
					didNavigate: false,
				})
			: public_failure_commands(operation?.public_call_ids, cause);
	return finish_work_transition(state, next_state, commands);
}

function publish_hmr_route(
	state: ClientState,
	prepared: PreparedHMRRoute,
): UpdateResult {
	if (!state.current) {
		return {
			state: {
				...state,
				hmr_operation: null,
			},
			commands: [],
		};
	}
	const previous_snapshot = state.current;
	const next_snapshot = {
		...previous_snapshot,
		provisional: false,
		render: prepared.render,
		route: prepared.route,
	};
	const commit: ClientCommit = {
		route_render: {
			state: prepared.render,
			scroll_intent: previous_snapshot.scroll_intent,
		},
	};
	if (!same_route_state(previous_snapshot.route, prepared.route)) {
		commit.route_update = {
			previous_route: previous_snapshot.route,
			reason: route_operation_kind.revalidation,
			route: prepared.route,
		};
	}
	return {
		state: {
			...state,
			current: next_snapshot,
			hmr_operation: null,
		},
		commands: [
			{
				type: command_type.commit,
				commit,
			},
			{ type: command_type.render },
		],
	};
}

function publish_same_document_route(
	state: ClientState,
	operation: RouteOperation,
	position: { href: string; key: string; state: unknown },
	before_route_commands: ClientCommand[],
): UpdateResult {
	if (!state.current) {
		return { state, commands: before_route_commands };
	}
	const previous_snapshot = state.current;
	const next_route = route_state_at_position(
		previous_snapshot.route,
		position,
	);
	const next_render = render_state_at_position(
		previous_snapshot.render,
		position,
	);
	const scroll_intent = scroll_intent_for_route(operation, next_route);
	const next_snapshot = {
		position,
		provisional: false,
		route: next_route,
		render: next_render,
		scroll_intent,
	};
	const next_state = {
		...state,
		active_route_operation: null,
		browser: position,
		current: next_snapshot,
		hmr_operation: null,
		view_transition_operation_id: null,
	};
	const commit: ClientCommit = {
		route_render: {
			state: next_render,
			scroll_intent,
		},
	};
	if (
		previous_snapshot.route.href !== next_route.href ||
		previous_snapshot.route.historyState !== next_route.historyState
	) {
		commit.route_update = {
			previous_route: previous_snapshot.route,
			reason: route_update_reason_for_operation(operation),
			route: next_route,
		};
	}
	const did_navigate =
		previous_snapshot.route.href !== next_route.href ||
		previous_snapshot.route.historyState !== next_route.historyState;
	const route_commit_commands: ClientCommand[] = [
		{ type: command_type.commit, commit },
		{ type: command_type.render },
	];
	const route_commands = route_commit_commands.concat(
		resolve_public_call_commands(operation.public_call_ids, {
			didNavigate: did_navigate,
		}),
	);
	return finish_work_transition(
		state,
		next_state,
		before_route_commands,
		route_commands,
	);
}

function route_state_at_position(
	route: RouteState,
	position: { href: string; state: unknown },
): RouteState {
	return {
		...route,
		href: position.href,
		historyState: position.state,
	};
}

function render_state_at_position(
	render: RouteRenderState,
	position: { state: unknown },
): RouteRenderState {
	return {
		...render,
		history_state: position.state,
	};
}

function prepared_route_at_position(
	prepared: PreparedRoute,
	position: { href: string; state: unknown },
): PreparedRoute {
	return {
		...prepared,
		render: render_state_at_position(prepared.render, position),
		route: route_state_at_position(prepared.route, position),
	};
}

function publish_prepared_route(
	state: ClientState,
	prepared: PreparedRoute,
	now_ms: number,
): UpdateResult {
	const operation = state.active_route_operation;
	if (!operation) {
		return { state, commands: [] };
	}
	const previous_snapshot = state.current;
	const position = {
		href:
			operation.kind === route_operation_kind.revalidation
				? (state.browser?.href ?? prepared.route.href)
				: operation.href || prepared.route.href,
		key: operation.browser_key,
		state: operation.state,
	};
	const next_snapshot = {
		position,
		provisional: false,
		route: prepared.route,
		render: prepared.render,
		scroll_intent: scroll_intent_for_route(operation, prepared.route),
	};
	const next_refresh =
		state.refresh.kind === refresh_status.running &&
		state.refresh.operation_id === operation.id
			? { kind: refresh_status.idle }
			: state.refresh;
	const next_state = {
		...state,
		active_route_operation: null,
		browser: position,
		current: next_snapshot,
		focus_revalidation: record_focus_activity(
			state.focus_revalidation,
			now_ms,
		),
		hmr_operation: null,
		phase: client_phase.ready,
		refresh: next_refresh,
		view_transition_operation_id:
			state.use_view_transitions &&
			operation.kind === route_operation_kind.navigation
				? operation.id
				: null,
	};
	const history_commands: ClientCommand[] =
		operation.kind === route_operation_kind.navigation
			? [
					{ type: command_type.save_current_scroll },
					{
						type: command_type.write_history,
						kind: operation.replace
							? history_write_kind.replace
							: history_write_kind.push,
						href: operation.href,
						key: operation.browser_key,
						state: operation.state,
					},
				]
			: [];
	const publication_commands: ClientCommand[] = [
		{
			type: command_type.apply_route_dom,
			prepared,
		},
		{
			type: command_type.commit,
			commit: {
				route_render: {
					state: prepared.render,
					scroll_intent: next_snapshot.scroll_intent,
				},
				route_update: {
					previous_route:
						previous_snapshot && !previous_snapshot.provisional
							? previous_snapshot.route
							: null,
					reason: route_update_reason_for_operation(operation),
					route: prepared.route,
				},
			},
		},
		{ type: command_type.render },
	];
	const boot_commands: ClientCommand[] =
		operation.kind === route_operation_kind.boot
			? [{ type: command_type.install_browser_listeners }]
			: [];
	const settlement_commands: ClientCommand[] = resolve_public_call_commands(
		operation.public_call_ids,
		operation.kind === route_operation_kind.revalidation
			? revalidation_ok
			: { didNavigate: true },
	).concat(
		state.refresh.kind === refresh_status.running
			? resolve_public_call_commands(
					state.refresh.demand.public_call_ids,
					revalidation_ok,
				)
			: [],
	);
	const route_commands = wrap_publication_commands_for_view_transition(
		state,
		operation,
		history_commands.concat(
			publication_commands,
			boot_commands,
			settlement_commands,
		),
	);
	if (next_state.deferred_api_redirect) {
		return begin_deferred_api_redirect(
			state,
			next_state,
			next_state.deferred_api_redirect,
			route_commands,
		);
	}
	if (next_state.refresh.kind === refresh_status.pending) {
		const revalidation = begin_revalidation(
			next_state,
			next_state.refresh.demand,
			next_state.refresh.attempt,
		);
		return {
			state: revalidation.state,
			commands: route_commands.concat(revalidation.commands),
		};
	}
	return finish_work_transition(state, next_state, route_commands);
}

function request_revalidation(
	state: ClientState,
	demand: RefreshDemand,
	debounce: boolean,
): UpdateResult {
	if (state.active_route_operation) {
		const next_state = {
			...state,
			refresh: {
				kind: refresh_status.pending,
				attempt: 0,
				demand,
			},
		};
		const commands: ClientCommand[] = [];
		return finish_work_transition(state, next_state, commands);
	}
	if (debounce) {
		const timer_id = `${demand.operation_id}:debounce`;
		const next_state = {
			...state,
			refresh: {
				kind: refresh_status.debouncing,
				demand,
				timer_id,
			},
		};
		const commands: ClientCommand[] = [];
		return finish_work_transition(state, next_state, commands, [
			{
				type: command_type.start_timer,
				timer_id,
				delay_ms: refresh_limit.debounce_ms,
				operation_id: demand.operation_id,
			},
		]);
	}
	return begin_revalidation(state, demand, 0);
}

function request_submission_revalidation(
	prev_state: ClientState,
	state: ClientState,
	submission: SubmissionRecord,
	commands: ClientCommand[],
): UpdateResult {
	const demand: RefreshDemand = {
		after_operation_id: prev_state.active_route_operation?.id,
		operation_id: submission.revalidation_operation_id,
		public_call_ids: submission.revalidation_public_call_id
			? [submission.revalidation_public_call_id]
			: [],
		reason: revalidation_reason.api_request,
		skip_work_indicator: submission.skip_work_indicator,
	};
	if (
		state.active_route_operation?.kind === route_operation_kind.boot &&
		state.current
	) {
		const revalidation = begin_background_revalidation(state, demand, 0);
		return {
			state: revalidation.state,
			commands: commands.concat(revalidation.commands),
		};
	}
	if (!state.active_route_operation && state.current) {
		const revalidation = begin_revalidation(state, demand, 0);
		return {
			state: revalidation.state,
			commands: commands.concat(revalidation.commands),
		};
	}
	const next_state = {
		...state,
		refresh: {
			kind: refresh_status.pending,
			attempt: 0,
			demand,
		},
	};
	return finish_work_transition(prev_state, next_state, commands);
}

function begin_background_revalidation(
	state: ClientState,
	demand: RefreshDemand,
	attempt: number,
): UpdateResult {
	if (!state.current) {
		return { state, commands: [] };
	}
	const next_state = {
		...state,
		refresh: {
			kind: refresh_status.running,
			demand,
			attempt,
			operation_id: demand.operation_id,
		},
	};
	const commands: ClientCommand[] = [];
	return finish_work_transition(state, next_state, commands, [
		{
			type: command_type.fetch_route,
			client_build_id: state.client_build_id,
			deployment_id: state.deployment_id,
			current_route: state.current.route,
			history_state: state.current.position.state,
			operation_id: demand.operation_id,
			href: state.current.route.href,
			trigger: route_fetch_trigger.revalidation,
		},
	]);
}

function begin_revalidation(
	state: ClientState,
	demand: RefreshDemand,
	attempt: number,
): UpdateResult {
	if (!state.current) {
		return { state, commands: [] };
	}
	const operation: RouteOperation = {
		browser_key: state.browser?.key ?? "",
		id: demand.operation_id,
		kind: route_operation_kind.revalidation,
		href: state.current.route.href,
		public_call_ids: [],
		redirect_count: 0,
		replace: true,
		scroll_to_top: undefined,
		source: navigation_source.navigate,
		state: state.current.position.state,
		skip_work_indicator: demand.skip_work_indicator,
	};
	const next_state = {
		...state,
		active_route_operation: operation,
		hmr_operation: null,
		refresh: {
			kind: refresh_status.running,
			demand,
			attempt,
			operation_id: demand.operation_id,
		},
	};
	const commands: ClientCommand[] = [];
	if (state.hmr_operation) {
		commands.push({
			type: command_type.abort_operation,
			operation_id: state.hmr_operation.id,
		});
	}
	return finish_work_transition(state, next_state, commands, [
		{
			type: command_type.fetch_route,
			client_build_id: state.client_build_id,
			deployment_id: state.deployment_id,
			current_route: state.current?.route ?? null,
			history_state: operation.state,
			operation_id: demand.operation_id,
			href: operation.href,
			trigger: route_fetch_trigger.revalidation,
		},
	]);
}

function begin_deferred_api_redirect(
	prev_state: ClientState,
	state: ClientState,
	redirect: DeferredAPIRedirect,
	commands: ClientCommand[],
): UpdateResult {
	const operation: RouteOperation = {
		browser_key: redirect.browser_key,
		id: redirect.operation_id,
		kind: route_operation_kind.navigation,
		href: redirect.href,
		public_call_ids: [],
		redirect_count: 0,
		replace: true,
		scroll_to_top: undefined,
		source: navigation_source.redirect,
		state: undefined,
		skip_work_indicator: redirect.skip_work_indicator,
	};
	const next_state = {
		...state,
		active_route_operation: operation,
		deferred_api_redirect: null,
	};
	return finish_work_transition(prev_state, next_state, commands, [
		{
			type: command_type.fetch_route,
			client_build_id: state.client_build_id,
			deployment_id: state.deployment_id,
			current_route: state.current?.route ?? null,
			history_state: operation.state,
			operation_id: operation.id,
			href: operation.href,
			trigger: route_fetch_trigger.navigation,
		},
	]);
}

function wrap_publication_commands_for_view_transition(
	state: ClientState,
	operation: RouteOperation,
	commands: ClientCommand[],
): ClientCommand[] {
	if (
		!state.use_view_transitions ||
		operation.kind !== route_operation_kind.navigation ||
		commands.length === 0
	) {
		return commands;
	}
	return [
		{
			type: command_type.run_view_transition,
			commands,
			operation_id: operation.id,
			public_call_ids: operation.public_call_ids,
		},
	];
}

function scroll_intent_for_route(
	operation: RouteOperation,
	route: RouteState,
): ScrollIntent | undefined {
	const scroll = scroll_state_for_operation(operation);
	if (!scroll) {
		return undefined;
	}
	const idx = route.matches.length - 1;
	const pattern = route.matches[idx]?.pattern ?? "";
	return {
		scroll,
		target_route_id: `${idx}:${pattern}`,
	};
}

function scroll_state_for_operation(
	operation: RouteOperation,
): ScrollState | undefined {
	const hash = href_hash(operation.href);
	if (operation.kind === route_operation_kind.boot) {
		return operation.boot_scroll ?? (hash ? { hash } : undefined);
	}
	if (operation.kind === route_operation_kind.popstate) {
		if (hash) {
			return { hash };
		}
		return operation.popstate_scroll ?? { x: 0, y: 0 };
	}
	if (operation.kind === route_operation_kind.navigation) {
		if (hash) {
			return { hash };
		}
		if (operation.scroll_to_top !== false) {
			return { x: 0, y: 0 };
		}
	}
	return undefined;
}

function href_hash(href: string): string {
	try {
		return new URL(href, url_parse_base).hash;
	} catch {
		return "";
	}
}

function same_document_href(left: string, right: string): boolean {
	const left_url = parse_href(left, url_parse_base);
	if (!left_url) {
		return false;
	}
	const right_url = parse_href(right, left_url.href);
	if (!right_url) {
		return false;
	}
	return (
		left_url.origin === right_url.origin &&
		left_url.pathname === right_url.pathname &&
		left_url.search === right_url.search
	);
}

function resolve_href(href: string, base: string): string {
	const url = parse_href(href, base);
	return url ? url.href : href;
}

function parse_href(href: string, base: string): URL | null {
	try {
		return new URL(href, base);
	} catch {
		return null;
	}
}

function route_update_reason_for_operation(
	operation: RouteOperation,
): RouteUpdateReason {
	if (operation.kind === route_operation_kind.boot) {
		return route_operation_kind.boot;
	}
	if (operation.kind === route_operation_kind.popstate) {
		return route_operation_kind.popstate;
	}
	if (operation.kind === route_operation_kind.revalidation) {
		return route_operation_kind.revalidation;
	}
	return route_operation_kind.navigation;
}

function route_fetch_trigger_for_operation(
	operation: RouteOperation,
): RouteFetchTrigger {
	if (operation.kind === route_operation_kind.popstate) {
		return route_fetch_trigger.popstate;
	}
	if (operation.kind === route_operation_kind.revalidation) {
		return route_fetch_trigger.revalidation;
	}
	return route_fetch_trigger.navigation;
}

function public_failure_commands(
	public_call_ids: PublicCallID[] | undefined,
	cause: unknown,
): ClientCommand[] {
	if (!public_call_ids) {
		return [];
	}
	return public_call_ids.map((public_call_id) => {
		return {
			type: command_type.reject_public_call,
			public_call_id,
			cause,
		};
	});
}

function error_message(cause: unknown): string {
	if (cause instanceof DOMException && cause.name === "AbortError") {
		return client_error_message.aborted;
	}
	if (cause instanceof Error) {
		return cause.message;
	}
	return String(cause);
}

function resolve_public_call_commands(
	public_call_ids: PublicCallID[],
	result: unknown,
): ClientCommand[] {
	return public_call_ids.map((public_call_id) => {
		return {
			type: command_type.resolve_public_call,
			public_call_id,
			result,
		};
	});
}

function finish_work_transition(
	prev_state: ClientState,
	next_state: ClientState,
	before_work_commands: ClientCommand[] = [],
	after_work_commands: ClientCommand[] = [],
): UpdateResult {
	const commit = work_commit(next_state);
	const work_changed =
		JSON.stringify(work_commit(prev_state)) !== JSON.stringify(commit);
	const work_commands: ClientCommand[] = work_changed
		? [
				{
					type: command_type.commit,
					commit,
				},
			]
		: [];
	return {
		state: next_state,
		commands: before_work_commands.concat(
			work_commands,
			after_work_commands,
		),
	};
}

function work_commit(state: ClientState): ClientCommit {
	return {
		work: derive_work_state(state),
		work_activity: derive_work_activity(state),
	};
}

export function derive_work_state(state: ClientState): WorkState {
	const active = state.active_route_operation;
	const refresh = state.refresh;
	let revalidation: WorkState["revalidation"] = null;
	if (
		refresh.kind === refresh_status.debouncing ||
		refresh.kind === refresh_status.retrying ||
		refresh.kind === refresh_status.running
	) {
		revalidation = {
			status: refresh.kind,
			attempt:
				refresh.kind === refresh_status.debouncing
					? 0
					: refresh.attempt,
		};
	}
	return {
		navigation:
			active?.kind === route_operation_kind.navigation ||
			active?.kind === route_operation_kind.popstate
				? {
						href: active.href,
						replace: active.replace,
						source: active.source,
					}
				: null,
		revalidation,
		prefetch: state.prefetch?.is_pending
			? { href: state.prefetch.href }
			: null,
		apiRequests: Object.values(state.submissions).map((submission) => {
			return {
				key: submission.id,
				method: submission.method,
				href: submission.href,
			};
		}),
	};
}

function derive_work_activity(state: ClientState): WorkActivity {
	const active = state.active_route_operation;
	const refresh = state.refresh;
	const activity: WorkActivity = [];
	if (
		active?.kind === route_operation_kind.navigation ||
		active?.kind === route_operation_kind.popstate
	) {
		activity.push({
			kind: work_activity_kind.navigation,
			skip_work_indicator: active.skip_work_indicator,
		});
	}
	if (
		refresh.kind === refresh_status.debouncing ||
		refresh.kind === refresh_status.retrying ||
		refresh.kind === refresh_status.running
	) {
		activity.push({
			kind: work_activity_kind.revalidation,
			skip_work_indicator: refresh.demand.skip_work_indicator,
		});
	}
	if (state.prefetch?.is_pending) {
		activity.push({ kind: work_activity_kind.prefetch });
	}
	for (const submission of Object.values(state.submissions)) {
		activity.push({
			kind: work_activity_kind.api_request,
			skip_work_indicator: submission.skip_work_indicator,
		});
	}
	return activity;
}

function same_route_state(left: RouteState, right: RouteState): boolean {
	if (
		left.href !== right.href ||
		left.historyState !== right.historyState ||
		left.clientBuildID !== right.clientBuildID ||
		left.error !== right.error ||
		left.matches.length !== right.matches.length
	) {
		return false;
	}
	if (!same_string_record(left.params, right.params)) {
		return false;
	}
	if (!same_string_array(left.splatValues, right.splatValues)) {
		return false;
	}
	for (let i = 0; i < left.matches.length; i++) {
		const left_match = left.matches[i]!;
		const right_match = right.matches[i]!;
		if (
			left_match.pattern !== right_match.pattern ||
			left_match.input !== right_match.input ||
			left_match.loaderData !== right_match.loaderData ||
			left_match.clientLoaderData !== right_match.clientLoaderData
		) {
			return false;
		}
	}
	return true;
}

function same_string_record(
	left: Record<string, string>,
	right: Record<string, string>,
): boolean {
	const left_keys = Object.keys(left);
	if (left_keys.length !== Object.keys(right).length) {
		return false;
	}
	return left_keys.every((key) => {
		return left[key] === right[key];
	});
}

function same_string_array(left: string[], right: string[]): boolean {
	if (left.length !== right.length) {
		return false;
	}
	for (let i = 0; i < left.length; i++) {
		if (left[i] !== right[i]) {
			return false;
		}
	}
	return true;
}
