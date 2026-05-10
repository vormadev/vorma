/// <reference types="vite/client" />

import { is_abort_cause } from "./abort.ts";
import type {
	CommandDispatch,
	CoreHost,
	PublicCallRegistry,
	TimerHandle,
} from "./host.ts";
import type { IDSource } from "./ids.ts";
import {
	command_type,
	event_type,
	route_operation_kind,
	type ClientCommand,
	type OperationID,
	type TimerID,
} from "./model.ts";

export type CommandInterpreter = {
	execute: (commands: ClientCommand[]) => void;
};

export function create_command_interpreter(
	host: CoreHost,
	dispatch: CommandDispatch,
	id_source: IDSource,
	public_calls: PublicCallRegistry,
): CommandInterpreter {
	const operations = new Map<OperationID, AbortController>();
	const timers = new Map<TimerID, TimerHandle>();
	let browser_listeners_cleanup: (() => void) | undefined;

	function cancel_operation(operation_id: OperationID): void {
		const controller = operations.get(operation_id);
		if (!controller) {
			return;
		}
		controller.abort();
		operations.delete(operation_id);
	}

	function remember_operation(
		operation_id: OperationID,
		controller: AbortController,
	): void {
		cancel_operation(operation_id);
		operations.set(operation_id, controller);
	}

	function forget_operation(
		operation_id: OperationID,
		controller: AbortController,
	): void {
		if (operations.get(operation_id) === controller) {
			operations.delete(operation_id);
		}
	}

	function start_operation<A>(
		operation_id: OperationID,
		run: (signal: AbortSignal) => Promise<A>,
		on_success: (value: A) => void,
		on_failure: (cause: unknown) => void,
	): void {
		const controller = new AbortController();
		remember_operation(operation_id, controller);
		let promise: Promise<A>;
		try {
			promise = run(controller.signal);
		} catch (cause) {
			forget_operation(operation_id, controller);
			try {
				on_failure(cause);
			} catch (failure_cause) {
				host.report_unhandled_error(failure_cause);
			}
			return;
		}
		void promise
			.then(
				(value) => {
					on_success(value);
				},
				(cause) => {
					on_failure(cause);
				},
			)
			.catch((cause) => {
				host.report_unhandled_error(cause);
			})
			.finally(() => {
				forget_operation(operation_id, controller);
			});
	}

	function clear_timer(timer_id: TimerID): void {
		const timer = timers.get(timer_id);
		if (!timer) {
			return;
		}
		host.clear_timer(timer);
		timers.delete(timer_id);
	}

	function execute(commands: ClientCommand[]): void {
		if (commands.length === 0) {
			return;
		}
		void execute_commands(commands).catch((cause) => {
			host.report_unhandled_error(cause);
		});
	}

	async function execute_commands(commands: ClientCommand[]): Promise<void> {
		for (const command of commands) {
			switch (command.type) {
				case command_type.abort_operation: {
					cancel_operation(command.operation_id);
					break;
				}
				case command_type.apply_route_dom: {
					await run_host_task(host, () => {
						return host.apply_route_dom(command.prepared);
					});
					break;
				}
				case command_type.commit: {
					await run_host_task(host, () => {
						return host.commit(command.commit);
					});
					break;
				}
				case command_type.fetch_api: {
					start_operation(
						command.operation_id,
						(signal) => {
							return host.fetch_api({
								deployment_id: command.deployment_id,
								href: command.href,
								method: command.method,
								request_init: command.request_init,
								signal,
							});
						},
						(result) => {
							dispatch({
								type: event_type.api_response_received,
								operation_id: command.operation_id,
								result,
								submission_id: command.submission_id,
							});
						},
						(cause) => {
							dispatch({
								type: event_type.api_response_failed,
								cause,
								operation_id: command.operation_id,
								submission_id: command.submission_id,
							});
						},
					);
					break;
				}
				case command_type.fetch_route: {
					start_operation(
						command.operation_id,
						(signal) => {
							return host.fetch_route({
								client_build_id: command.client_build_id,
								deployment_id: command.deployment_id,
								current_route: command.current_route,
								history_state: command.history_state,
								href: command.href,
								operation_id: command.operation_id,
								signal,
								trigger: command.trigger,
							});
						},
						(response) => {
							dispatch({
								type: event_type.route_response_received,
								operation_id: command.operation_id,
								response,
							});
						},
						(cause) => {
							dispatch({
								type: event_type.route_response_failed,
								cause,
								operation_id: command.operation_id,
							});
						},
					);
					break;
				}
				case command_type.hard_redirect: {
					await run_host_task(host, () => {
						return host.hard_redirect(command.href);
					});
					break;
				}
				case command_type.install_browser_listeners: {
					if (browser_listeners_cleanup) {
						break;
					}
					browser_listeners_cleanup = host.install_browser_listeners({
						on_popstate: (event) => {
							dispatch({
								type: event_type.popstate_observed,
								browser: event.browser,
								leaving_scroll: event.leaving_scroll,
								operation_id: id_source.next_operation_id(),
								popstate_scroll: event.scroll,
							});
						},
						on_focus: (event) => {
							dispatch({
								type: event_type.window_focus_observed,
								now_ms: event.now_ms,
								operation_id: id_source.next_operation_id(),
							});
						},
						on_hmr_update: (event) => {
							if (!import.meta.env.DEV) {
								return;
							}
							dispatch({
								type: event_type.hmr_update_observed,
								module: event.module,
								module_url: event.module_url,
								now_ms: event.now_ms,
								operation_id: id_source.next_operation_id(),
							});
						},
					});
					break;
				}
				case command_type.prepare_hmr_route: {
					if (!import.meta.env.DEV) {
						break;
					}
					start_operation(
						command.operation_id,
						(signal) => {
							return host.prepare_hmr_route({
								module: command.module,
								module_url: command.module_url,
								position: command.position,
								render: command.render,
								rerun_client_loader:
									command.rerun_client_loader,
								route: command.route,
								signal,
							});
						},
						(prepared) => {
							dispatch({
								type: event_type.hmr_prepared,
								operation_id: command.operation_id,
								prepared,
							});
						},
						(cause) => {
							dispatch({
								type: event_type.hmr_preparation_failed,
								cause,
								operation_id: command.operation_id,
							});
						},
					);
					break;
				}
				case command_type.prepare_route: {
					start_operation(
						command.operation_id,
						(signal) => {
							return host.prepare_route({
								client_build_id: command.client_build_id,
								current_route: command.current_route,
								history_state: command.history_state,
								href: command.href,
								on_provisional_route:
									command.trigger ===
									route_operation_kind.boot
										? (prepared) => {
												dispatch({
													type: event_type.route_provisionally_prepared,
													operation_id:
														command.operation_id,
													prepared,
												});
											}
										: undefined,
								operation_id: command.operation_id,
								payload: command.payload,
								signal,
								trigger: command.trigger,
							});
						},
						(prepared) => {
							dispatch({
								type: event_type.route_prepared,
								now_ms: host.now_ms(),
								operation_id: command.operation_id,
								prepared,
							});
						},
						(cause) => {
							dispatch({
								type: event_type.route_preparation_failed,
								cause,
								operation_id: command.operation_id,
							});
						},
					);
					break;
				}
				case command_type.read_boot_payload: {
					start_operation(
						command.operation_id,
						() => {
							return host.read_boot_payload(
								command.fallback_browser_key,
							);
						},
						(result) => {
							dispatch({
								type: event_type.boot_payload_read,
								browser: result.browser,
								operation_id: command.operation_id,
								payload: result.payload,
								reload_scroll: result.reload_scroll,
							});
						},
						(cause) => {
							dispatch({
								type: event_type.boot_failed,
								cause,
								operation_id: command.operation_id,
							});
						},
					);
					break;
				}
				case command_type.reject_public_call: {
					public_calls.reject(command.public_call_id, command.cause);
					break;
				}
				case command_type.report_build_skew: {
					await run_host_task(host, () => {
						return host.report_build_skew(command.event);
					});
					break;
				}
				case command_type.render: {
					await run_host_task(host, () => {
						return host.render();
					});
					break;
				}
				case command_type.resolve_public_call: {
					public_calls.resolve(
						command.public_call_id,
						command.result,
					);
					break;
				}
				case command_type.run_route_hooks: {
					start_operation(
						command.operation_id,
						(signal) => {
							return host.run_route_hooks({
								current_render: command.current_render,
								current_route: command.current_route,
								prepared: command.prepared,
								signal,
								trigger: command.trigger,
							});
						},
						(prepared) => {
							dispatch({
								type: event_type.route_hooks_completed,
								now_ms: host.now_ms(),
								operation_id: command.operation_id,
								prepared,
							});
						},
						(cause) => {
							dispatch({
								type: event_type.route_hooks_failed,
								cause,
								operation_id: command.operation_id,
							});
						},
					);
					break;
				}
				case command_type.run_view_transition: {
					start_operation(
						command.operation_id,
						(signal) => {
							return host.run_view_transition(async () => {
								if (signal.aborted) {
									for (const public_call_id of command.public_call_ids) {
										public_calls.resolve(public_call_id, {
											didNavigate: false,
										});
									}
									return;
								}
								await execute_commands(command.commands);
							});
						},
						() => {
							dispatch({
								type: event_type.view_transition_completed,
								operation_id: command.operation_id,
							});
						},
						(cause) => {
							if (!is_abort_cause(cause)) {
								host.report_unhandled_error(cause);
							}
							dispatch({
								type: event_type.view_transition_completed,
								operation_id: command.operation_id,
							});
						},
					);
					break;
				}
				case command_type.save_current_scroll: {
					await run_host_task(host, () => {
						return host.save_current_scroll();
					});
					break;
				}
				case command_type.save_scroll_position: {
					await run_host_task(host, () => {
						return host.save_scroll_position(
							command.key,
							command.scroll,
						);
					});
					break;
				}
				case command_type.start_timer: {
					clear_timer(command.timer_id);
					const timer = host.set_timer(command.delay_ms, () => {
						timers.delete(command.timer_id);
						dispatch({
							type: event_type.refresh_timer_fired,
							operation_id: command.operation_id,
							timer_id: command.timer_id,
						});
					});
					timers.set(command.timer_id, timer);
					break;
				}
				case command_type.write_history: {
					await run_host_task(host, () => {
						return host.write_history({
							href: command.href,
							key: command.key,
							kind: command.kind,
							state: command.state,
						});
					});
					break;
				}
			}
		}
	}

	return { execute };
}

async function run_host_task(
	host: CoreHost,
	task: () => void | Promise<void>,
): Promise<void> {
	try {
		await task();
	} catch (cause) {
		host.report_unhandled_error(cause);
	}
}
