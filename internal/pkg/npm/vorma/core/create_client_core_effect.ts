import { Effect } from "effect";
import { R, type Result } from "vorma/kit/result";
import { make_boot_revalidation_gate } from "./effect_runtime/boot_revalidation_gate.ts";
import { make_boot_route_state } from "./effect_runtime/boot_route_state.ts";
import type {
	APIResult,
	ClientCore,
	ClientOptions,
	CommitFn,
	TestOptions,
	ViewDefinition,
} from "./effect_runtime/client_contract.ts";
import { type EffectClientKernel } from "./effect_runtime/client_kernel.ts";
import {
	acquire_effect_client_kernel,
	type EffectClientKernelHandle,
} from "./effect_runtime/client_kernel_assembly.ts";
import {
	client_runtime_services_to_layer,
	make_client_runtime_services,
} from "./effect_runtime/client_runtime_services.ts";
import { make_client_session } from "./effect_runtime/client_session.ts";
import { type FocusRevalidator } from "./effect_runtime/focus_revalidator.ts";
import type { SubmitResult } from "./effect_runtime/submit_manager.ts";
import type { WorkIndicatorActivity } from "./effect_runtime/work_state_actor.ts";
import type { APIRouteKind, AppConfig, RevalidationResult } from "./types.ts";

export const EFFECT_CLIENT_BUILD_ID_FIELD = "ClientBuildID";
export const EFFECT_DEPLOYMENT_ID_FIELD = "DeploymentID";

const effect_client_session = Effect.runSync(make_client_session());

export function create_client_core_effect(
	app_config: Omit<AppConfig, "__vormaViews" | "__vormaAPIRoutes">,
	commit: CommitFn,
	test_options?: TestOptions,
): Result<ClientCore> {
	void app_config;

	let kernel: EffectClientKernel | null = null;
	let client_options: ClientOptions = {};
	let focus_revalidator: FocusRevalidator | null = null;
	const runtime_services = Effect.runSync(
		make_client_runtime_services({
			hard_redirect: test_options?.hard_redirect,
			scroll_to: test_options?.scroll_to,
		}),
	);
	const {
		browser_location,
		browser_view_runtime,
		module_runtime,
		work_indicator_runtime,
	} = runtime_services;
	const boot_revalidation_gate = Effect.runSync(
		make_boot_revalidation_gate(),
	);
	const boot_route_state = Effect.runSync(make_boot_route_state());
	let default_error_boundary:
		| ((props: { error: unknown }) => any)
		| undefined;

	function work_indicator_active_for(
		activity: WorkIndicatorActivity,
	): boolean {
		const options = client_options.workIndicator;
		if (!options) {
			return false;
		}
		if (activity.navigation && options.skipNavigations !== true) {
			return true;
		}
		if (activity.revalidation && options.skipRevalidations !== true) {
			return true;
		}
		if (activity.apiRequests && options.skipAPIRequests !== true) {
			return true;
		}
		return false;
	}

	const emit_client_commit: CommitFn = (client_commit) => {
		commit(client_commit);
		if (client_commit.route_update) {
			client_options.onRouteUpdate?.(
				client_commit.route_update.route,
				client_commit.route_update.previous_route,
				client_commit.route_update.reason,
			);
		}
		if (client_commit.work) {
			client_options.onWorkUpdate?.(client_commit.work);
		}
	};

	function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
		return Effect.runPromise(program);
	}

	function require_kernel(): EffectClientKernel {
		if (!kernel) {
			throw new Error("Vorma not booted");
		}
		return kernel;
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
		const revalidationPromise = run_effect(result.revalidation);
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
	): EffectClientKernelHandle {
		return Effect.runSync(
			acquire_effect_client_kernel({
				client_build_id,
				commit: emit_client_commit,
				deployment_id,
				on_build_skew_detected: (event) => {
					return Effect.sync(() => {
						client_options.onBuildSkewDetected?.(event);
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
					return work_indicator_runtime.set_vorma_active(
						work_indicator_active_for(activity),
					);
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
					);
				},
				use_view_transitions: Effect.sync(() => {
					return client_options.useViewTransitions === true;
				}),
			}).pipe(
				Effect.provide(
					client_runtime_services_to_layer(runtime_services),
				),
			),
		);
	}

	async function boot(options: ClientOptions): Promise<Result<void>> {
		client_options = options;
		default_error_boundary = options.defaultErrorBoundary;
		Effect.runSync(work_indicator_runtime.configure(options.workIndicator));
		const existing_kernel = kernel;
		if (existing_kernel) {
			Effect.runSync(
				existing_kernel.work_actor.indicator_activity.pipe(
					Effect.flatMap((activity) => {
						return work_indicator_runtime.set_vorma_active(
							work_indicator_active_for(activity),
						);
					}),
				),
			);
		}
		const payload_result = Effect.runSync(
			Effect.either(browser_view_runtime.read_initial_payload),
		);
		if (payload_result._tag === "Left") {
			return R.err(payload_result.left.reason);
		}
		const payload = payload_result.right;
		const client_build_id = string_payload_field(
			payload,
			EFFECT_CLIENT_BUILD_ID_FIELD,
		);
		const deployment_id = string_payload_field(
			payload,
			EFFECT_DEPLOYMENT_ID_FIELD,
		);
		const next_kernel_handle = assemble_kernel(
			client_build_id,
			deployment_id,
		);
		const next_kernel = next_kernel_handle.kernel;
		Effect.runSync(boot_route_state.clear);
		Effect.runSync(boot_revalidation_gate.start_boot);
		kernel = next_kernel;
		focus_revalidator = null;
		const boot_result = await run_effect(
			Effect.either(next_kernel.boot_initial_route(payload)),
		);
		if (boot_result._tag === "Left") {
			if (kernel === next_kernel) {
				kernel = existing_kernel;
			}
			Effect.runSync(boot_route_state.clear);
			Effect.runSync(boot_revalidation_gate.cancel_boot);
			await run_effect(next_kernel_handle.shutdown);
			return R.err(String(boot_result.left));
		}
		const should_revalidate_after_boot = Effect.runSync(
			boot_revalidation_gate.finish_boot,
		);
		kernel = next_kernel;
		focus_revalidator = null;
		Effect.runSync(boot_route_state.clear);
		Effect.runSync(
			next_kernel.lifecycle.add_finalizer(
				Effect.gen(function* () {
					yield* Effect.sync(() => {
						if (kernel === next_kernel) {
							kernel = null;
						}
						focus_revalidator = null;
					});
					yield* boot_route_state.clear;
				}),
			),
		);
		await run_effect(
			effect_client_session.replace_active(next_kernel_handle),
		);
		if (options.revalidateOnWindowFocus) {
			const stale_ms =
				typeof options.revalidateOnWindowFocus === "object"
					? options.revalidateOnWindowFocus.staleTimeMS
					: 5_000;
			const next_focus_revalidator = Effect.runSync(
				next_kernel.install_focus_revalidator(stale_ms),
			);
			focus_revalidator = next_focus_revalidator;
		}
		if (should_revalidate_after_boot) {
			void run_effect(
				next_kernel.route_revalidator.request("apiRequest"),
			);
		}
		Effect.runSync(next_kernel.install_browser_handlers);
		const render_result = await run_effect(
			Effect.either(
				Effect.tryPromise({
					try: async () => {
						await options.render?.();
					},
					catch: (error) => {
						return error;
					},
				}),
			),
		);
		if (render_result._tag === "Left") {
			await run_effect(
				effect_client_session.shutdown_if_active(next_kernel_handle),
			);
			return R.err(String(render_result.left));
		}
		return R.ok(undefined);
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
		if (!browser_location.is_http_href(target_href)) {
			return Promise.resolve({ didNavigate: false });
		}
		if (!browser_location.is_same_origin_href(target_href)) {
			Effect.runSync(browser_location.hard_redirect(target_href));
			return Promise.resolve({ didNavigate: false });
		}
		const target_key = browser_location.route_key(target_href);
		const current_snapshot = Effect.runSync(
			active_kernel.route_publisher.snapshot,
		);
		const previous_route_key = current_snapshot
			? browser_location.route_key(current_snapshot.position.href)
			: null;
		const is_route_change =
			!current_snapshot ||
			browser_location.route_key(current_snapshot.position.href) !==
				target_key;
		if (is_route_change) {
			const navigation_snapshot = Effect.runSync(
				active_kernel.navigation_actor.snapshot,
			);
			if (
				navigation_snapshot.active &&
				navigation_snapshot.active.key !== target_key
			) {
				Effect.runSync(
					active_kernel.navigation_actor.abort_active_signal,
				);
			}
			Effect.runSync(active_kernel.route_revalidator.cancel);
		}
		return run_effect(
			Effect.gen(function* () {
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
				return { didNavigate: result.didNavigate };
			}),
		).then((result) => {
			const navigation_snapshot = Effect.runSync(
				active_kernel.navigation_actor.snapshot,
			);
			if (!navigation_snapshot.active) {
				Effect.runSync(active_kernel.work_actor.set_navigation(null));
			}
			if (result.didNavigate && focus_revalidator) {
				const next_snapshot = Effect.runSync(
					active_kernel.route_publisher.snapshot,
				);
				const next_route_key = next_snapshot
					? browser_location.route_key(next_snapshot.position.href)
					: null;
				if (
					next_route_key !== null &&
					next_route_key !== previous_route_key
				) {
					void run_effect(focus_revalidator.mark_activity);
				}
			}
			return result;
		});
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
		return run_effect(
			active_kernel.submit_manager.submit<T>({
				href: String(url),
				init: request_init,
				options,
			}),
		).then(submit_result_to_api_result);
	}

	function revalidate(): Promise<RevalidationResult> {
		const active_kernel = require_kernel();
		return run_effect(
			active_kernel.route_revalidator.request("manual", {
				debounce: true,
			}),
		).then((result) => {
			if (result.ok && focus_revalidator) {
				void run_effect(focus_revalidator.mark_activity);
			}
			return result;
		});
	}

	function start_prefetch(href: string): void {
		const active_kernel = kernel;
		const target_href = browser_location.resolve_href_or_null(href);
		if (!active_kernel || !target_href) {
			return;
		}
		void run_effect(
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
				Effect.catchAll(() => {
					return Effect.void;
				}),
			),
		);
	}

	function stop_prefetch(href: string): void {
		const active_kernel = kernel;
		const target_href = browser_location.resolve_href_or_null(href);
		if (!active_kernel || !target_href) {
			return;
		}
		void run_effect(
			active_kernel.prefetch_manager.stop(target_href).pipe(
				Effect.catchAll(() => {
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
			return kernel?.client_build_id ?? "";
		},
		getRootEl,
		defineView,
		start_prefetch,
		stop_prefetch,
		save_current_scroll: () => {
			const active_kernel = kernel;
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
			return default_error_boundary;
		},
	});
}
