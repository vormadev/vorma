import { Effect, Result as EffectResult } from "effect";
import { R, type Result } from "vorma/kit/result";
import type {
	APIResult,
	ClientCore,
	ClientOptions,
	CommitFn,
	TestOptions,
	ViewDefinition,
} from "./effect_runtime/client_contract.ts";
import { make_client_core_runtime } from "./effect_runtime/client_core_runtime.ts";
import { type EffectClientKernel } from "./effect_runtime/client_kernel.ts";
import {
	acquire_effect_client_kernel,
	type EffectClientKernelHandle,
} from "./effect_runtime/client_kernel_assembly.ts";
import type { SubmitResult } from "./effect_runtime/submit_manager.ts";
import type { WorkIndicatorActivity } from "./effect_runtime/work_state_actor.ts";
import type { APIRouteKind, AppConfig, RevalidationResult } from "./types.ts";

export const EFFECT_CLIENT_BUILD_ID_FIELD = "ClientBuildID";
export const EFFECT_DEPLOYMENT_ID_FIELD = "DeploymentID";

export function create_client_core_effect(
	app_config: Omit<AppConfig, "__vormaViews" | "__vormaAPIRoutes">,
	commit: CommitFn,
	test_options?: TestOptions,
): Result<ClientCore> {
	void app_config;

	const client_core_runtime = Effect.runSync(
		make_client_core_runtime({
			hard_redirect: test_options?.hard_redirect,
			scroll_to: test_options?.scroll_to,
		}),
	);
	const {
		boot_revalidation_gate,
		boot_route_state,
		client_core_state,
		client_session,
		runtime_services,
		runtime_services_layer,
	} = client_core_runtime;
	const {
		browser_location,
		browser_view_runtime,
		module_runtime,
		work_indicator_runtime,
	} = runtime_services;

	function work_indicator_active_for(
		options: ClientOptions,
		activity: WorkIndicatorActivity,
	): boolean {
		const work_indicator_options = options.workIndicator;
		if (!work_indicator_options) {
			return false;
		}
		if (
			activity.navigation &&
			work_indicator_options.skipNavigations !== true
		) {
			return true;
		}
		if (
			activity.revalidation &&
			work_indicator_options.skipRevalidations !== true
		) {
			return true;
		}
		if (
			activity.apiRequests &&
			work_indicator_options.skipAPIRequests !== true
		) {
			return true;
		}
		return false;
	}

	const emit_client_commit: CommitFn = (client_commit) => {
		commit(client_commit);
		const options = Effect.runSync(client_core_state.client_options);
		if (client_commit.route_update) {
			options.onRouteUpdate?.(
				client_commit.route_update.route,
				client_commit.route_update.previous_route,
				client_commit.route_update.reason,
			);
		}
		if (client_commit.work) {
			options.onWorkUpdate?.(client_commit.work);
		}
	};

	function require_kernel(): EffectClientKernel {
		const active_kernel = Effect.runSync(client_core_state.active_kernel);
		if (!active_kernel) {
			throw new Error("Vorma not booted");
		}
		return active_kernel;
	}

	function string_payload_field(
		payload: Record<string, unknown>,
		field: string,
	): string {
		const value = payload[field];
		return typeof value === "string" ? value : "";
	}

	function submit_result_to_api_result<T>(
		result: SubmitResult<T>,
	): APIResult<T> {
		const revalidationPromise = Effect.runPromise(result.revalidation);
		if (result.success) {
			return {
				success: true,
				data: result.data,
				response: result.response,
				revalidationPromise,
			};
		}
		return {
			success: false,
			error: result.error,
			response: result.response,
			revalidationPromise,
		};
	}

	function assemble_kernel(
		client_build_id: string,
		deployment_id: string,
	): Effect.Effect<EffectClientKernelHandle> {
		return acquire_effect_client_kernel({
			client_build_id,
			commit: emit_client_commit,
			deployment_id,
			on_build_skew_detected: (event) => {
				return Effect.gen(function* () {
					const options = yield* client_core_state.client_options;
					yield* Effect.sync(() => {
						options.onBuildSkewDetected?.(event);
					});
				});
			},
			on_client_redirect: (href) => {
				return Effect.sync(() => {
					void navigate(href, {
						replace: true,
					});
				});
			},
			on_indicator_update: (activity) => {
				return Effect.gen(function* () {
					const options = yield* client_core_state.client_options;
					yield* work_indicator_runtime.set_vorma_active(
						work_indicator_active_for(options, activity),
					);
				});
			},
			on_provisional_route: (input) => {
				return boot_route_state.capture(input);
			},
			revalidate_api_request: (
				route_revalidator,
				revalidation_options,
			) => {
				return boot_revalidation_gate.request_or_defer(
					route_revalidator.request(
						"apiRequest",
						revalidation_options,
					),
					revalidation_options,
				);
			},
			use_view_transitions: client_core_state.client_options.pipe(
				Effect.map((options) => {
					return options.useViewTransitions === true;
				}),
			),
		}).pipe(Effect.provide(runtime_services_layer));
	}

	function boot(options: ClientOptions): Promise<Result<void>> {
		return Effect.runPromise(
			Effect.gen(function* () {
				yield* client_core_state.set_boot_options(options);
				yield* work_indicator_runtime.configure(options.workIndicator);
				const existing_kernel = yield* client_core_state.active_kernel;
				if (existing_kernel) {
					yield* existing_kernel.work_actor.indicator_activity.pipe(
						Effect.flatMap((activity) => {
							return client_core_state.client_options.pipe(
								Effect.flatMap((options) => {
									return work_indicator_runtime.set_vorma_active(
										work_indicator_active_for(
											options,
											activity,
										),
									);
								}),
							);
						}),
					);
				}
				const payload_result = yield* Effect.result(
					browser_view_runtime.read_initial_payload,
				);
				if (EffectResult.isFailure(payload_result)) {
					return R.err(payload_result.failure.reason);
				}
				const payload = payload_result.success;
				const client_build_id = string_payload_field(
					payload,
					EFFECT_CLIENT_BUILD_ID_FIELD,
				);
				const deployment_id = string_payload_field(
					payload,
					EFFECT_DEPLOYMENT_ID_FIELD,
				);
				const next_kernel_handle = yield* assemble_kernel(
					client_build_id,
					deployment_id,
				);
				const next_kernel = next_kernel_handle.kernel;
				yield* boot_route_state.clear;
				yield* boot_revalidation_gate.start_boot;
				yield* client_core_state.set_active_kernel(next_kernel);
				yield* client_core_state.clear_focus_revalidator;
				const boot_result = yield* Effect.result(
					next_kernel.boot_initial_route(payload),
				);
				if (EffectResult.isFailure(boot_result)) {
					yield* client_core_state.restore_kernel_if_current(
						next_kernel,
						existing_kernel,
					);
					yield* boot_route_state.clear;
					yield* boot_revalidation_gate.cancel_boot;
					yield* next_kernel_handle.shutdown;
					return R.err(String(boot_result.failure));
				}
				const boot_revalidation_decision =
					yield* boot_revalidation_gate.finish_boot;
				yield* client_core_state.set_active_kernel(next_kernel);
				yield* client_core_state.clear_focus_revalidator;
				yield* boot_route_state.clear;
				yield* next_kernel.lifecycle.add_finalizer(
					Effect.gen(function* () {
						yield* client_core_state.clear_kernel_if_current(
							next_kernel,
						);
						yield* client_core_state.clear_focus_revalidator;
						yield* boot_route_state.clear;
					}),
				);
				yield* client_session.replace_active(next_kernel_handle);
				if (options.revalidateOnWindowFocus) {
					const stale_ms =
						typeof options.revalidateOnWindowFocus === "object"
							? options.revalidateOnWindowFocus.staleTimeMS
							: 5_000;
					const next_focus_revalidator =
						yield* next_kernel.install_focus_revalidator(stale_ms);
					yield* client_core_state.set_focus_revalidator(
						next_focus_revalidator,
					);
				}
				if (boot_revalidation_decision.requested) {
					yield* Effect.forkDetach(
						next_kernel.route_revalidator.request("apiRequest", {
							skipWorkIndicator:
								boot_revalidation_decision.skip_work_indicator,
						}),
						{ startImmediately: true },
					);
				}
				yield* next_kernel.install_browser_handlers;
				const render_result = yield* Effect.result(
					Effect.tryPromise({
						try: async () => {
							await options.render?.();
						},
						catch: (error) => {
							return error;
						},
					}),
				);
				if (EffectResult.isFailure(render_result)) {
					yield* client_session.shutdown_if_active(
						next_kernel_handle,
					);
					return R.err(String(render_result.failure));
				}
				return R.ok(undefined);
			}),
		);
	}

	function navigate(
		href: string | URL,
		options?: {
			replace?: boolean;
			scrollToTop?: boolean;
			state?: unknown;
			skipWorkIndicator?: boolean;
		},
	): Promise<{ didNavigate: boolean }> {
		const active_kernel = require_kernel();
		const target_href = browser_location.resolve_href(href);
		return Effect.runPromise(
			Effect.gen(function* () {
				if (!browser_location.is_http_href(target_href)) {
					return { didNavigate: false };
				}
				if (!browser_location.is_same_origin_href(target_href)) {
					yield* browser_location.hard_redirect(target_href);
					return { didNavigate: false };
				}
				const target_key = browser_location.route_key(target_href);
				const current_snapshot =
					yield* active_kernel.route_publisher.snapshot;
				const previous_route_key = current_snapshot
					? browser_location.route_key(current_snapshot.position.href)
					: null;
				const is_route_change =
					!current_snapshot ||
					browser_location.route_key(
						current_snapshot.position.href,
					) !== target_key;
				if (is_route_change) {
					const navigation_snapshot =
						yield* active_kernel.navigation_actor.snapshot;
					if (
						navigation_snapshot.active &&
						navigation_snapshot.active.key !== target_key
					) {
						yield* active_kernel.navigation_actor
							.abort_active_signal;
					}
					yield* active_kernel.route_revalidator.cancel;
				}
				const route_movement =
					yield* active_kernel.move_within_current_route(
						target_href,
						options,
					);
				if (route_movement) {
					return route_movement;
				}
				const result = yield* active_kernel.navigation_actor.navigate(
					target_href,
					{
						replace: options?.replace,
						state: options?.state,
						scrollToTop: options?.scrollToTop,
						skipWorkIndicator: options?.skipWorkIndicator,
					},
				);
				const public_result = { didNavigate: result.didNavigate };
				const navigation_snapshot =
					yield* active_kernel.navigation_actor.snapshot;
				if (!navigation_snapshot.active) {
					yield* active_kernel.work_actor.set_navigation(null);
				}
				const active_focus_revalidator =
					yield* client_core_state.focus_revalidator;
				if (public_result.didNavigate && active_focus_revalidator) {
					const next_snapshot =
						yield* active_kernel.route_publisher.snapshot;
					const next_route_key = next_snapshot
						? browser_location.route_key(
								next_snapshot.position.href,
							)
						: null;
					if (
						next_route_key !== null &&
						next_route_key !== previous_route_key
					) {
						yield* active_focus_revalidator.mark_activity;
					}
				}
				return public_result;
			}),
		);
	}

	function getRouteState() {
		const active_kernel = require_kernel();
		const route_state = Effect.runSync(
			active_kernel.route_publisher.route_state,
		);
		if (!route_state) {
			const provisional_route_state = Effect.runSync(
				boot_route_state.snapshot,
			);
			if (provisional_route_state) {
				return provisional_route_state;
			}
			throw new Error("Vorma not booted");
		}
		return route_state;
	}

	function getWorkState() {
		const active_kernel = require_kernel();
		return Effect.runSync(active_kernel.work_actor.snapshot);
	}

	function getRootEl(): HTMLElement {
		return Effect.runSync(browser_view_runtime.get_root_el);
	}

	function defineView<T = any>(input: {
		pattern: string;
		component: (props: any) => any;
		errorBoundary?: (props: { error: unknown }) => any;
		clientLoader?: (props: any) => Promise<T>;
		beforeRouteCommit?: ViewDefinition["before_route_commit"];
		beforeRouteYield?: ViewDefinition["before_route_yield"];
		runClientLoaderOnHMR?: boolean;
	}): ViewDefinition & { __phantom_client_loader_data?: T } {
		Effect.runSync(
			module_runtime.set_hmr_rerun(
				input.pattern,
				input.runClientLoaderOnHMR === true,
			),
		);
		return {
			pattern: input.pattern,
			component: input.component,
			error_boundary: input.errorBoundary,
			client_loader:
				input.clientLoader as ViewDefinition["client_loader"],
			before_route_commit: input.beforeRouteCommit,
			before_route_yield: input.beforeRouteYield,
		};
	}

	function submit_inner<T = unknown>(
		url: string | URL,
		request_init?: RequestInit,
		options?: {
			apiRouteKind?: APIRouteKind;
			dedupeKey?: string;
			revalidate?: boolean;
			skipWorkIndicator?: boolean;
		},
	): Promise<APIResult<T>> {
		const active_kernel = require_kernel();
		return Effect.runPromise(
			active_kernel.submit_manager
				.submit<T>({
					href: String(url),
					init: request_init,
					options,
				})
				.pipe(Effect.map(submit_result_to_api_result)),
		);
	}

	function revalidate(): Promise<RevalidationResult> {
		const active_kernel = require_kernel();
		return Effect.runPromise(
			Effect.gen(function* () {
				const await_result =
					yield* active_kernel.route_revalidator.request_started(
						"manual",
						{
							debounce: true,
						},
					);
				const result = yield* await_result;
				const active_focus_revalidator =
					yield* client_core_state.focus_revalidator;
				if (result.ok && active_focus_revalidator) {
					yield* active_focus_revalidator.mark_activity;
				}
				return result;
			}),
		);
	}

	function start_prefetch(href: string): void {
		const active_kernel = Effect.runSync(client_core_state.active_kernel);
		const target_href = browser_location.resolve_href_or_null(href);
		if (!active_kernel || !target_href) {
			return;
		}
		Effect.runFork(
			Effect.gen(function* () {
				const navigation_snapshot =
					yield* active_kernel.navigation_actor.snapshot;
				const position =
					yield* active_kernel.browser_history.ensure_current;
				yield* active_kernel.prefetch_manager.start({
					href: target_href,
					current_route_key: browser_location.route_key(
						position.href,
					),
					active_navigation_key:
						navigation_snapshot.active?.key ?? null,
					history_state: position.state,
				});
			}).pipe(
				Effect.asVoid,
				Effect.catchCause(() => {
					return Effect.void;
				}),
			),
		);
	}

	function stop_prefetch(href: string): void {
		const active_kernel = Effect.runSync(client_core_state.active_kernel);
		const target_href = browser_location.resolve_href_or_null(href);
		if (!active_kernel || !target_href) {
			return;
		}
		Effect.runFork(
			active_kernel.prefetch_manager.stop(target_href).pipe(
				Effect.asVoid,
				Effect.catchCause(() => {
					return Effect.void;
				}),
			),
		);
	}

	return R.ok({
		boot,
		workIndicator: work_indicator_runtime.indicator,
		navigate,
		revalidate,
		submit_inner,
		getRouteState,
		getWorkState,
		getClientBuildID: () => {
			const active_kernel = Effect.runSync(
				client_core_state.active_kernel,
			);
			return active_kernel?.client_build_id ?? "";
		},
		getRootEl,
		defineView,
		start_prefetch,
		stop_prefetch,
		save_current_scroll: () => {
			const active_kernel = Effect.runSync(
				client_core_state.active_kernel,
			);
			if (!active_kernel) {
				return;
			}
			Effect.runSync(
				Effect.gen(function* () {
					const position =
						yield* active_kernel.browser_history.current;
					yield* active_kernel.scroll_restoration.save_current(
						position,
					);
				}),
			);
		},
		get_default_error_boundary: () => {
			return Effect.runSync(client_core_state.default_error_boundary);
		},
	});
}
