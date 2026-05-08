import { Effect } from "effect";
import { R, type Result } from "vorma/kit/result";
import { DATA_SCRIPT_ID, VORMA_ROOT_EL_ID } from "./constants.ts";
import {
	type BrowserHistory,
	make_browser_history,
} from "./effect_runtime/browser_history.ts";
import {
	type BrowserLocation,
	make_browser_location,
} from "./effect_runtime/browser_location.ts";
import { make_build_skew_reporter } from "./effect_runtime/build_skew_reporter.ts";
import type {
	APIResult,
	BuildSkewDetectedEvent,
	ClientCore,
	ClientOptions,
	CommitFn,
	ScrollState,
	TestOptions,
	ViewDefinition,
} from "./effect_runtime/client_contract.ts";
import {
	type FocusRevalidator,
	make_focus_revalidator,
} from "./effect_runtime/focus_revalidator.ts";
import {
	type ModuleRuntime,
	make_module_runtime,
} from "./effect_runtime/module_runtime.ts";
import {
	type NavigationActor,
	type NavigationAttempt,
	NavigationLoadFailed,
	NavigationRedirect,
	make_navigation_actor,
} from "./effect_runtime/navigation_actor.ts";
import {
	type PrefetchManager,
	make_prefetch_manager,
} from "./effect_runtime/prefetch_manager.ts";
import {
	type RouteDOMRuntime,
	make_route_dom_runtime,
} from "./effect_runtime/route_dom_runtime.ts";
import type { RouteFetchResult } from "./effect_runtime/route_fetcher.ts";
import { make_route_fetcher } from "./effect_runtime/route_fetcher.ts";
import {
	type PreparedRoute,
	type RoutePreparer,
	make_route_preparer,
} from "./effect_runtime/route_preparer.ts";
import {
	RouteCommitFailed,
	type RoutePublishResult,
	type RoutePublisher,
	make_route_publisher,
	route_record_to_state,
} from "./effect_runtime/route_publisher.ts";
import {
	type RouteRevalidator,
	make_route_revalidator,
} from "./effect_runtime/route_revalidator.ts";
import {
	type RuntimeLifecycle,
	WINDOW_EVENT_BEFOREUNLOAD,
	WINDOW_EVENT_FOCUS,
	WINDOW_EVENT_POPSTATE,
	make_runtime_lifecycle,
} from "./effect_runtime/runtime_lifecycle.ts";
import {
	type ScrollRestoration,
	make_scroll_restoration,
} from "./effect_runtime/scroll_restoration.ts";
import {
	type SubmitDispatcher,
	make_submit_dispatcher,
} from "./effect_runtime/submit_dispatcher.ts";
import {
	type SubmitManager,
	type SubmitResult,
	make_submit_manager,
} from "./effect_runtime/submit_manager.ts";
import {
	type WorkIndicatorRuntime,
	make_work_indicator,
} from "./effect_runtime/work_indicator.ts";
import {
	WORK_NAVIGATION_SOURCE_NAVIGATE,
	WORK_NAVIGATION_SOURCE_POPSTATE,
	WORK_NAVIGATION_SOURCE_REDIRECT,
	type WorkIndicatorActivity,
	type WorkStateActor,
	make_work_state_actor,
} from "./effect_runtime/work_state_actor.ts";
import type { APIRouteKind, AppConfig, RevalidationResult } from "./types.ts";

export const EFFECT_CLIENT_BUILD_ID_FIELD = "ClientBuildID";
export const EFFECT_DEPLOYMENT_ID_FIELD = "DeploymentID";

let effect_active_kernel_shutdown: (() => Promise<void>) | null = null;

type EffectClientKernel = {
	browser_history: BrowserHistory;
	client_build_id: string;
	deployment_id: string;
	lifecycle: RuntimeLifecycle;
	navigation_actor: NavigationActor;
	prefetch_manager: PrefetchManager;
	route_preparer: RoutePreparer;
	route_publisher: RoutePublisher;
	route_revalidator: RouteRevalidator;
	scroll_restoration: ScrollRestoration;
	submit_manager: SubmitManager;
	work_actor: WorkStateActor;
};

export function create_client_core_effect(
	app_config: Omit<AppConfig, "__vormaViews" | "__vormaAPIRoutes">,
	commit: CommitFn,
	test_options?: TestOptions,
): Result<ClientCore> {
	void app_config;

	let kernel: EffectClientKernel | null = null;
	let boot_phase: "idle" | "booting" | "ready" = "idle";
	let boot_provisional_route_state: ReturnType<
		typeof route_record_to_state
	> | null = null;
	let boot_revalidation_requested = false;
	let client_options: ClientOptions = {};
	let focus_revalidator: FocusRevalidator | null = null;
	const browser_location: BrowserLocation = Effect.runSync(
		make_browser_location({
			hard_redirect: test_options?.hard_redirect,
		}),
	);
	const revalidation_ok: RevalidationResult = { ok: true };
	const route_dom_runtime: RouteDOMRuntime = Effect.runSync(
		make_route_dom_runtime(),
	);
	const module_runtime: ModuleRuntime = Effect.runSync(make_module_runtime());
	const submit_dispatcher: SubmitDispatcher = Effect.runSync(
		make_submit_dispatcher(),
	);
	const work_indicator_runtime: WorkIndicatorRuntime = Effect.runSync(
		make_work_indicator(),
	);
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

	function read_initial_payload(): Result<Record<string, unknown>> {
		const script = document.getElementById(DATA_SCRIPT_ID);
		if (!script?.textContent) {
			return R.err("Missing Vorma data script.");
		}
		try {
			const parsed = JSON.parse(script.textContent);
			if (
				!parsed ||
				typeof parsed !== "object" ||
				Array.isArray(parsed)
			) {
				return R.err("Vorma data script must contain an object.");
			}
			return R.ok(parsed as Record<string, unknown>);
		} catch (error) {
			return R.err(
				error instanceof Error ? error.message : String(error),
			);
		}
	}

	function string_payload_field(
		payload: Record<string, unknown>,
		field: string,
	): string {
		const value = payload[field];
		return typeof value === "string" ? value : "";
	}

	function navigation_source(attempt: NavigationAttempt) {
		if (attempt.source === "redirect") {
			return WORK_NAVIGATION_SOURCE_REDIRECT;
		}
		if (attempt.source === "popstate") {
			return WORK_NAVIGATION_SOURCE_POPSTATE;
		}
		return WORK_NAVIGATION_SOURCE_NAVIGATE;
	}

	function map_navigation_failure(error: unknown): NavigationLoadFailed {
		return new NavigationLoadFailed({ error });
	}

	function route_update_trigger(
		attempt: NavigationAttempt,
	): "navigation" | "popstate" {
		if (attempt.source === "popstate") {
			return "popstate";
		}
		return "navigation";
	}

	function route_skew_default_behavior(
		result: RouteFetchResult,
	): BuildSkewDetectedEvent["defaultBehavior"] {
		if (result.kind === "build_skew") {
			return "hardReload";
		}
		if (
			result.kind === "redirect" &&
			(result.hard || !browser_location.is_same_origin_href(result.href))
		) {
			return "hardReload";
		}
		return "notifyOnly";
	}

	function is_http_href(href: string): boolean {
		try {
			const protocol = browser_location.resolve_url(href).protocol;
			return protocol === "http:" || protocol === "https:";
		} catch {
			return false;
		}
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

	function apply_scroll_state(scroll: ScrollState): void {
		if ("hash" in scroll) {
			const raw = scroll.hash.startsWith("#")
				? scroll.hash.slice(1)
				: scroll.hash;
			let id: string;
			try {
				id = decodeURIComponent(raw);
			} catch {
				id = raw;
			}
			document.getElementById(id)?.scrollIntoView();
			return;
		}
		const scroll_to =
			test_options?.scroll_to ??
			((x: number, y: number) => {
				window.scrollTo(x, y);
			});
		scroll_to(scroll.x, scroll.y);
	}

	function commit_navigation_position(
		attempt: NavigationAttempt,
		href: string,
		browser_history: BrowserHistory,
		scroll_restoration: ScrollRestoration,
	) {
		if (attempt.source === "popstate") {
			return browser_history.adopt_current;
		}
		return Effect.gen(function* () {
			const previous_position = yield* browser_history.current;
			yield* scroll_restoration.save_current(previous_position);
			return yield* browser_history.commit(
				href,
				attempt.replace,
				attempt.state,
			);
		});
	}

	function assemble_kernel(
		client_build_id: string,
		deployment_id: string,
	): EffectClientKernel {
		const lifecycle = Effect.runSync(make_runtime_lifecycle());
		const browser_history = Effect.runSync(make_browser_history());
		const scroll_restoration = Effect.runSync(make_scroll_restoration());
		const work_actor = Effect.runSync(
			make_work_state_actor({
				commit: emit_client_commit,
				on_indicator_update: (activity) => {
					return work_indicator_runtime.set_vorma_active(
						work_indicator_active_for(activity),
					);
				},
			}),
		);
		const route_preparer = Effect.runSync(
			make_route_preparer({
				client_build_id,
				load_module: (module_url) => {
					return module_runtime.load_module(module_url);
				},
				preload_css: route_dom_runtime.preload_css,
				wait_for_css: route_dom_runtime.wait_for_css,
				apply_payload_side_effects:
					route_dom_runtime.apply_payload_side_effects,
				on_provisional_route: (input) => {
					return Effect.sync(() => {
						if (input.prepare_input.trigger !== "boot") {
							return;
						}
						boot_provisional_route_state = route_record_to_state(
							input.route,
							input.prepare_input.href,
							input.prepare_input.history_state,
						);
					});
				},
				decode_title: route_dom_runtime.decode_title,
			}),
		);
		const route_publisher = Effect.runSync(
			make_route_publisher({
				commit: emit_client_commit,
				apply_scroll: (scroll) => {
					return Effect.sync(() => {
						apply_scroll_state(scroll);
					});
				},
				run_view_transition: (publish_effect) => {
					if (client_options.useViewTransitions !== true) {
						return publish_effect;
					}
					const start_view_transition = (
						document as {
							startViewTransition?: (callback: () => void) => {
								updateCallbackDone?: Promise<unknown>;
								finished?: Promise<unknown>;
							};
						}
					).startViewTransition;
					if (typeof start_view_transition !== "function") {
						return publish_effect;
					}
					return Effect.tryPromise({
						try: async () => {
							let publish_result: RoutePublishResult | undefined;
							let publish_error: unknown;
							const transition = start_view_transition.call(
								document,
								() => {
									const result = Effect.runSync(
										Effect.either(publish_effect),
									);
									if (result._tag === "Left") {
										publish_error = result.left;
										throw result.left;
									}
									publish_result = result.right;
								},
							);
							await (transition.updateCallbackDone ??
								transition.finished ??
								Promise.resolve());
							if (publish_error !== undefined) {
								throw publish_error;
							}
							if (publish_result === undefined) {
								throw new Error(
									"View transition did not publish.",
								);
							}
							return publish_result;
						},
						catch: (error) => {
							return new RouteCommitFailed({ error });
						},
					});
				},
			}),
		);
		const route_fetcher = make_route_fetcher({
			client_build_id,
			deployment_id,
		});
		const build_skew_reporter = make_build_skew_reporter({
			client_build_id,
			current_route_state: route_publisher.route_state,
			current_work_state: work_actor.snapshot,
			on_detected: (event) => {
				client_options.onBuildSkewDetected?.(event);
			},
		});
		const prefetch_manager = Effect.runSync(
			make_prefetch_manager({
				fetcher: route_fetcher,
				is_external: (href) => {
					return !browser_location.is_same_origin_href(href);
				},
				route_key: browser_location.route_key,
				route_preparer,
				prestart: (input) => {
					return route_preparer.catalog.prestart_client_loaders({
						url: input.url,
						href: input.href,
						history_state: input.history_state,
						trigger: "prefetch",
						signal: input.signal,
					});
				},
				work_actor,
			}),
		);
		const navigation_actor: NavigationActor = Effect.runSync(
			make_navigation_actor({
				isExternal: (href) => {
					return !browser_location.is_same_origin_href(href);
				},
				routeKey: browser_location.route_key,
				prestart: (attempt) => {
					const url = browser_location.resolve_url(attempt.href);
					return Effect.gen(function* () {
						const prefetch_snapshot =
							yield* prefetch_manager.snapshot;
						const key = browser_location.route_key(url.href);
						if (
							prefetch_snapshot.active?.key === key ||
							prefetch_snapshot.prepared?.key === key
						) {
							return [];
						}
						return yield* route_preparer.catalog.prestart_client_loaders(
							{
								url,
								href: url.href,
								history_state: attempt.state,
								trigger: "navigation",
								signal: attempt.signal,
							},
						);
					});
				},
				load: (attempt) => {
					return Effect.gen(function* () {
						const url = browser_location.resolve_url(attempt.href);
						const prefetched = yield* prefetch_manager.take(
							url.href,
						);
						if (prefetched) {
							yield* Effect.forEach(
								attempt.client_loader_prestarts,
								(prestart) => {
									return prestart.abort;
								},
								{ discard: true },
							);
							yield* work_actor.set_navigation({
								href: url.href,
								replace: attempt.replace,
								skipworkIndicator:
									attempt.skipworkIndicator === true,
								source: navigation_source(attempt),
							});
							return {
								href: url.href,
								value: prefetched.prepared,
							};
						}
						yield* work_actor.set_navigation({
							href: url.href,
							replace: attempt.replace,
							skipworkIndicator:
								attempt.skipworkIndicator === true,
							source: navigation_source(attempt),
						});
						const fetch_result = yield* route_fetcher
							.fetch_route({ url, signal: attempt.signal })
							.pipe(Effect.mapError(map_navigation_failure));
						if (
							"response" in fetch_result &&
							fetch_result.response
						) {
							const response = fetch_result.response;
							yield* build_skew_reporter.report({
								response,
								triggering_response: {
									kind: "route",
									trigger: route_update_trigger(attempt),
									requestedHref: url.href,
									status: response.status,
									ok: response.ok,
								},
								default_behavior:
									route_skew_default_behavior(fetch_result),
							});
						}
						if (fetch_result.kind === "build_skew") {
							yield* browser_location.hard_redirect(url.href);
							return yield* Effect.fail(
								new NavigationLoadFailed({
									error: fetch_result,
								}),
							);
						}
						if (fetch_result.kind === "redirect") {
							if (!is_http_href(fetch_result.href)) {
								return yield* Effect.fail(
									new NavigationLoadFailed({
										error: fetch_result,
									}),
								);
							}
							const current_route =
								yield* route_publisher.snapshot;
							if (
								current_route &&
								browser_location.route_key(
									current_route.position.href,
								) ===
									browser_location.route_key(
										fetch_result.href,
									)
							) {
								return {
									href: fetch_result.href,
									value: {
										route: current_route.route,
										apply_dom_side_effects: Effect.void,
									},
								};
							}
							if (
								fetch_result.hard ||
								!browser_location.is_same_origin_href(
									fetch_result.href,
								)
							) {
								yield* browser_location.hard_redirect(
									fetch_result.href,
								);
								return yield* Effect.fail(
									new NavigationLoadFailed({
										error: fetch_result,
									}),
								);
							}
							return yield* Effect.fail(
								new NavigationRedirect({
									href: fetch_result.href,
									hard: fetch_result.hard,
								}),
							);
						}
						if (fetch_result.kind !== "data") {
							return yield* Effect.fail(
								new NavigationLoadFailed({
									error: fetch_result,
								}),
							);
						}
						const prepared = yield* route_preparer
							.prepare_route({
								raw_payload: fetch_result.data,
								url,
								trigger: "navigation",
								href: url.href,
								history_state: attempt.state,
								signal: attempt.signal,
								client_loader_prestarts:
									attempt.client_loader_prestarts,
							})
							.pipe(Effect.mapError(map_navigation_failure));
						return {
							href: url.href,
							value: prepared,
						};
					}).pipe(
						Effect.tapError(() => {
							return work_actor.set_navigation(null);
						}),
					);
				},
				publish: (
					loaded,
					attempt,
				): Effect.Effect<void, NavigationLoadFailed> => {
					return Effect.gen(function* () {
						const prepared = loaded.value as PreparedRoute;
						const position = yield* commit_navigation_position(
							attempt,
							loaded.href,
							browser_history,
							scroll_restoration,
						);
						const scroll =
							attempt.source === "popstate"
								? yield* scroll_restoration.popstate_scroll(
										position,
									)
								: yield* scroll_restoration.navigation_scroll(
										loaded.href,
										attempt.scrollToTop,
									);
						yield* work_actor.set_navigation(null);
						const work = yield* work_actor.snapshot;
						const publish_result: RoutePublishResult =
							yield* route_publisher
								.publish({
									reason:
										attempt.source === "popstate"
											? "popstate"
											: "navigation",
									prepared,
									position,
									scroll,
									work,
									signal: attempt.signal,
									guard: navigation_actor.snapshot.pipe(
										Effect.map((snapshot) => {
											return (
												snapshot.active?.id ===
												attempt.id
											);
										}),
									),
								})
								.pipe(Effect.mapError(map_navigation_failure));
						if (!publish_result.did_publish) {
							return yield* Effect.fail(
								new NavigationLoadFailed({
									error: publish_result,
								}),
							);
						}
					});
				},
			}),
		);
		const route_revalidator = Effect.runSync(
			make_route_revalidator({
				build_skew_reporter,
				current_position: () => {
					return browser_history.ensure_current;
				},
				fetcher: route_fetcher,
				navigation_actor,
				route_key: browser_location.route_key,
				route_preparer,
				route_publisher,
				work_actor,
			}),
		);
		const submit_manager = Effect.runSync(
			make_submit_manager({
				baseHref: browser_location.current_href(),
				deploymentID: deployment_id,
				dispatch: submit_dispatcher.dispatch,
				isSameOrigin: (url) => {
					return browser_location.is_same_origin_href(url.href);
				},
				on_snapshot_change: (snapshot) => {
					return work_actor.set_api_requests(
						snapshot.active.map((item) => {
							return {
								key: item.key,
								method: item.method,
								href: item.href,
								skipworkIndicator: item.skipworkIndicator,
							};
						}),
					);
				},
				redirect: (href, kind) => {
					if (kind === "hard") {
						return browser_location.hard_redirect(href);
					}
					return Effect.sync(() => {
						void navigate(href, {
							replace: true,
						});
					});
				},
				revalidate: () => {
					if (boot_phase === "booting") {
						boot_revalidation_requested = true;
						return Effect.succeed(revalidation_ok);
					}
					return route_revalidator.request("apiRequest");
				},
				report_build_skew: (dispatch, response, default_behavior) => {
					return build_skew_reporter.report({
						response,
						triggering_response: {
							kind: "apiRoute",
							apiRouteKind: dispatch.apiRouteKind,
							requestedHref: dispatch.url.href,
							method: dispatch.method,
							status: response.status,
							ok: response.ok,
						},
						default_behavior,
					});
				},
			}),
		);
		Effect.runSync(
			Effect.gen(function* () {
				yield* lifecycle.add_finalizer(work_actor.shutdown);
				yield* lifecycle.add_finalizer(route_revalidator.shutdown);
				yield* lifecycle.add_finalizer(submit_manager.shutdown);
				yield* lifecycle.add_finalizer(prefetch_manager.shutdown);
				yield* lifecycle.add_finalizer(navigation_actor.shutdown);
			}),
		);
		return {
			browser_history,
			client_build_id,
			deployment_id,
			lifecycle,
			navigation_actor,
			prefetch_manager,
			route_preparer,
			route_publisher,
			route_revalidator,
			scroll_restoration,
			submit_manager,
			work_actor,
		};
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
		const payload_result = read_initial_payload();
		if (!payload_result.ok) {
			return R.err(payload_result.err);
		}
		const payload = payload_result.val;
		const client_build_id = string_payload_field(
			payload,
			EFFECT_CLIENT_BUILD_ID_FIELD,
		);
		const deployment_id = string_payload_field(
			payload,
			EFFECT_DEPLOYMENT_ID_FIELD,
		);
		const next_kernel = assemble_kernel(client_build_id, deployment_id);
		Effect.runSync(next_kernel.scroll_restoration.set_manual_restoration);
		const position = Effect.runSync(
			next_kernel.browser_history.ensure_current,
		);
		boot_phase = "booting";
		boot_provisional_route_state = null;
		boot_revalidation_requested = false;
		kernel = next_kernel;
		focus_revalidator = null;
		const boot_result = await run_effect(
			Effect.either(
				Effect.gen(function* () {
					const prepared =
						yield* next_kernel.route_preparer.prepare_route({
							raw_payload: payload,
							url: new URL(position.href),
							trigger: "boot",
							href: position.href,
							history_state: position.state,
						});
					yield* next_kernel.route_publisher.publish({
						reason: "initial",
						prepared,
						position,
						scroll: yield* next_kernel.scroll_restoration.boot_scroll(
							position,
						),
					});
				}),
			),
		);
		if (boot_result._tag === "Left") {
			if (kernel === next_kernel) {
				kernel = existing_kernel;
			}
			boot_phase = existing_kernel ? "ready" : "idle";
			boot_provisional_route_state = null;
			boot_revalidation_requested = false;
			await run_effect(next_kernel.lifecycle.shutdown);
			return R.err(String(boot_result.left));
		}
		if (effect_active_kernel_shutdown) {
			await effect_active_kernel_shutdown();
			effect_active_kernel_shutdown = null;
		}
		kernel = next_kernel;
		focus_revalidator = null;
		boot_phase = "ready";
		boot_provisional_route_state = null;
		Effect.runSync(
			next_kernel.lifecycle.add_finalizer(
				Effect.sync(() => {
					if (kernel === next_kernel) {
						kernel = null;
						boot_phase = "idle";
					}
					focus_revalidator = null;
					boot_provisional_route_state = null;
				}),
			),
		);
		effect_active_kernel_shutdown = () => {
			return run_effect(next_kernel.lifecycle.shutdown);
		};
		if (options.revalidateOnWindowFocus) {
			const stale_ms =
				typeof options.revalidateOnWindowFocus === "object"
					? options.revalidateOnWindowFocus.staleTimeMS
					: 5_000;
			focus_revalidator = Effect.runSync(
				make_focus_revalidator({
					stale_ms,
					get_work_state: next_kernel.work_actor.snapshot,
					request_revalidation: next_kernel.route_revalidator.request,
				}),
			);
			const focus_listener = (): void => {
				if (!focus_revalidator) {
					return;
				}
				void run_effect(focus_revalidator.focus);
			};
			Effect.runSync(
				next_kernel.lifecycle.listen_window(
					WINDOW_EVENT_FOCUS,
					focus_listener,
				),
			);
		}
		if (boot_revalidation_requested) {
			boot_revalidation_requested = false;
			void run_effect(
				next_kernel.route_revalidator.request("apiRequest"),
			);
		}
		const popstate_listener = (): void => {
			const active_kernel = kernel;
			if (!active_kernel) {
				return;
			}
			void run_effect(handle_popstate(active_kernel));
		};
		Effect.runSync(
			next_kernel.lifecycle.listen_window(
				WINDOW_EVENT_POPSTATE,
				popstate_listener,
			),
		);
		const beforeunload_listener = (): void => {
			const active_kernel = kernel;
			if (!active_kernel) {
				return;
			}
			Effect.runSync(active_kernel.scroll_restoration.save_reload_scroll);
		};
		Effect.runSync(
			next_kernel.lifecycle.listen_window(
				WINDOW_EVENT_BEFOREUNLOAD,
				beforeunload_listener,
			),
		);
		Effect.runSync(
			module_runtime.install_hmr_handler({
				route_preparer: next_kernel.route_preparer,
				route_publisher: next_kernel.route_publisher,
				work_actor: next_kernel.work_actor,
			}),
		);
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
			if (effect_active_kernel_shutdown) {
				await effect_active_kernel_shutdown();
				effect_active_kernel_shutdown = null;
			}
			return R.err(String(render_result.left));
		}
		return R.ok(undefined);
	}

	function handle_popstate(
		active_kernel: EffectClientKernel,
	): Effect.Effect<void> {
		return Effect.gen(function* () {
			const previous = yield* active_kernel.browser_history.current;
			const position = yield* active_kernel.browser_history.adopt_current;
			if (
				position.key === previous.key &&
				position.href === previous.href
			) {
				return;
			}
			if (previous.key.length > 0 && previous.key !== position.key) {
				yield* active_kernel.scroll_restoration.save_current(previous);
			}
			if (
				browser_location.route_key(position.href) ===
				browser_location.route_key(previous.href)
			) {
				const scroll =
					yield* active_kernel.scroll_restoration.popstate_scroll(
						position,
					);
				const work = yield* active_kernel.work_actor.snapshot;
				yield* active_kernel.route_publisher.move_position({
					reason: "popstate",
					position,
					scroll,
					work,
				});
				return;
			}
			yield* active_kernel.route_revalidator.cancel;
			yield* active_kernel.navigation_actor.navigate(position.href, {
				source: "popstate",
				state: position.state,
				scrollToTop: false,
				skipworkIndicator: true,
			});
		}).pipe(
			Effect.catchAll(() => {
				return Effect.void;
			}),
		);
	}

	function move_within_current_route(
		active_kernel: EffectClientKernel,
		target_href: string,
		options:
			| {
					replace?: boolean;
					scrollToTop?: boolean;
					state?: unknown;
			  }
			| undefined,
	) {
		return Effect.gen(function* () {
			const snapshot = yield* active_kernel.route_publisher.snapshot;
			if (!snapshot) {
				return null;
			}
			if (
				browser_location.route_key(snapshot.position.href) !==
				browser_location.route_key(target_href)
			) {
				return null;
			}
			const current_hash = browser_location.hash_fragment(
				snapshot.position.href,
			);
			const target_hash = browser_location.hash_fragment(target_href);
			const did_change_hash = current_hash !== target_hash;
			const should_commit_history =
				did_change_hash || options?.replace === true;
			const scroll =
				yield* active_kernel.scroll_restoration.navigation_scroll(
					target_href,
					options?.scrollToTop,
				);
			if (!should_commit_history) {
				if (scroll) {
					const work = yield* active_kernel.work_actor.snapshot;
					yield* active_kernel.route_publisher.move_position({
						reason: "navigation",
						position: snapshot.position,
						scroll,
						work,
					});
				}
				return { didNavigate: false };
			}
			const previous_position =
				yield* active_kernel.browser_history.current;
			yield* active_kernel.scroll_restoration.save_current(
				previous_position,
			);
			const position = yield* active_kernel.browser_history.commit(
				target_href,
				options?.replace === true,
				options?.state,
			);
			const work = yield* active_kernel.work_actor.snapshot;
			yield* active_kernel.route_publisher.move_position({
				reason: "navigation",
				position,
				scroll,
				work,
			});
			return { didNavigate: did_change_hash };
		});
	}

	function navigate(
		href: string | URL,
		options?: {
			replace?: boolean;
			scrollToTop?: boolean;
			state?: unknown;
			skipworkIndicator?: boolean;
		},
	): Promise<{ didNavigate: boolean }> {
		const active_kernel = require_kernel();
		const target_href = browser_location.resolve_href(href);
		if (!is_http_href(target_href)) {
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
				const route_movement = yield* move_within_current_route(
					active_kernel,
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
						skipworkIndicator: options?.skipworkIndicator,
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
			if (boot_provisional_route_state) {
				return boot_provisional_route_state;
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
		const existing = document.getElementById(VORMA_ROOT_EL_ID);
		if (existing) {
			return existing;
		}
		const root = document.createElement("div");
		root.id = VORMA_ROOT_EL_ID;
		document.body.insertBefore(root, document.body.firstChild);
		return root;
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
			skipworkIndicator?: boolean;
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
