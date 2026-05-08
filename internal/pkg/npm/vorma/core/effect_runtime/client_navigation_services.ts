import { Context, Effect, Layer } from "effect";
import type { RevalidationResult } from "../types.ts";
import type { BrowserHistory } from "./browser_history.ts";
import type { BrowserLocation } from "./browser_location.ts";
import type { BuildSkewDetectedEvent } from "./client_contract.ts";
import {
	BrowserHistoryService,
	type ClientKernelResourcesLayerContext,
	ScrollRestorationService,
	WorkStateActorService,
} from "./client_kernel_resources.ts";
import {
	BuildSkewReporterService,
	type ClientRouteServicesLayerContext,
	RouteFetcherService,
	RoutePreparerService,
	RoutePublisherService,
} from "./client_route_services.ts";
import {
	BrowserLocationService,
	BrowserTimerRuntimeService,
	type ClientRuntimeServicesLayerContext,
	SubmitDispatcherService,
} from "./client_runtime_services.ts";
import {
	type NavigationActor,
	type NavigationAttempt,
	NavigationLoadFailed,
	NavigationRedirect,
	make_navigation_actor,
} from "./navigation_actor.ts";
import {
	type PrefetchManager,
	make_prefetch_manager,
} from "./prefetch_manager.ts";
import type { RouteFetchResult } from "./route_fetcher.ts";
import type { PreparedRoute } from "./route_preparer.ts";
import {
	type HistoryPosition,
	type RoutePublishResult,
} from "./route_publisher.ts";
import {
	type RouteRevalidator,
	make_route_revalidator,
} from "./route_revalidator.ts";
import type { ScrollRestoration } from "./scroll_restoration.ts";
import {
	type SubmitManager,
	type SubmitSnapshot,
	make_submit_manager,
} from "./submit_manager.ts";
import {
	WORK_NAVIGATION_SOURCE_NAVIGATE,
	WORK_NAVIGATION_SOURCE_POPSTATE,
	WORK_NAVIGATION_SOURCE_REDIRECT,
} from "./work_state_actor.ts";

const navigation_actor_service_tag = "vorma/NavigationActorService";
const prefetch_manager_service_tag = "vorma/PrefetchManagerService";
const route_revalidator_service_tag = "vorma/RouteRevalidatorService";
const submit_manager_service_tag = "vorma/SubmitManagerService";

export class NavigationActorService extends Context.Service<
	NavigationActorService,
	NavigationActor
>()(navigation_actor_service_tag) {}

export class PrefetchManagerService extends Context.Service<
	PrefetchManagerService,
	PrefetchManager
>()(prefetch_manager_service_tag) {}

export class RouteRevalidatorService extends Context.Service<
	RouteRevalidatorService,
	RouteRevalidator
>()(route_revalidator_service_tag) {}

export class SubmitManagerService extends Context.Service<
	SubmitManagerService,
	SubmitManager
>()(submit_manager_service_tag) {}

export type ClientNavigationServicesOptions = {
	deployment_id: string;
	on_client_redirect: (href: string) => Effect.Effect<void>;
	revalidate_api_request: (
		route_revalidator: RouteRevalidator,
		options?: { skipWorkIndicator?: boolean },
	) => Effect.Effect<RevalidationResult>;
};

export type ClientNavigationServicesLayerContext =
	| NavigationActorService
	| PrefetchManagerService
	| RouteRevalidatorService
	| SubmitManagerService;

export type ClientNavigationServicesRequirements =
	| ClientKernelResourcesLayerContext
	| ClientRouteServicesLayerContext
	| ClientRuntimeServicesLayerContext;

export function make_client_navigation_services_layer(
	options: ClientNavigationServicesOptions,
): Layer.Layer<
	ClientNavigationServicesLayerContext,
	never,
	ClientNavigationServicesRequirements
> {
	const prefetch_manager_layer = Layer.effect(
		PrefetchManagerService,
		Effect.gen(function* () {
			const browser_location = yield* BrowserLocationService;
			const route_fetcher = yield* RouteFetcherService;
			const route_preparer = yield* RoutePreparerService;
			const work_actor = yield* WorkStateActorService;

			return yield* make_prefetch_manager({
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
			});
		}),
	);
	const navigation_actor_layer = Layer.effect(
		NavigationActorService,
		Effect.gen(function* () {
			const browser_history = yield* BrowserHistoryService;
			const browser_location = yield* BrowserLocationService;
			const build_skew_reporter = yield* BuildSkewReporterService;
			const prefetch_manager = yield* PrefetchManagerService;
			const route_fetcher = yield* RouteFetcherService;
			const route_preparer = yield* RoutePreparerService;
			const route_publisher = yield* RoutePublisherService;
			const scroll_restoration = yield* ScrollRestorationService;
			const work_actor = yield* WorkStateActorService;

			return yield* make_navigation_actor({
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
								skipWorkIndicator:
									attempt.skipWorkIndicator === true,
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
							skipWorkIndicator:
								attempt.skipWorkIndicator === true,
							source: navigation_source(attempt),
						});
						const fetch_result = yield* route_fetcher
							.fetch_route({
								url,
								signal: attempt.signal,
							})
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
								default_behavior: route_skew_default_behavior(
									browser_location,
									fetch_result,
								),
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
							if (
								!browser_location.is_http_href(
									fetch_result.href,
								)
							) {
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
				publish: (loaded, attempt, is_current) => {
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
									guard: is_current,
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
			});
		}),
	);
	const route_revalidator_layer = Layer.effect(
		RouteRevalidatorService,
		Effect.gen(function* () {
			const browser_history = yield* BrowserHistoryService;
			const browser_location = yield* BrowserLocationService;
			const browser_timer_runtime = yield* BrowserTimerRuntimeService;
			const build_skew_reporter = yield* BuildSkewReporterService;
			const navigation_actor = yield* NavigationActorService;
			const route_fetcher = yield* RouteFetcherService;
			const route_preparer = yield* RoutePreparerService;
			const route_publisher = yield* RoutePublisherService;
			const work_actor = yield* WorkStateActorService;

			return yield* make_route_revalidator({
				build_skew_reporter,
				current_position: () => {
					return browser_history.ensure_current;
				},
				fetcher: route_fetcher,
				navigation_actor,
				route_key: browser_location.route_key,
				route_preparer,
				route_publisher,
				schedule_ms: browser_timer_runtime.schedule_ms,
				sleep_ms: browser_timer_runtime.sleep_ms,
				work_actor,
			});
		}),
	);
	const submit_manager_layer = Layer.effect(
		SubmitManagerService,
		Effect.gen(function* () {
			const browser_location = yield* BrowserLocationService;
			const build_skew_reporter = yield* BuildSkewReporterService;
			const route_revalidator = yield* RouteRevalidatorService;
			const submit_dispatcher = yield* SubmitDispatcherService;
			const work_actor = yield* WorkStateActorService;

			return yield* make_submit_manager({
				baseHref: browser_location.current_href(),
				deploymentID: options.deployment_id,
				dispatch: submit_dispatcher.dispatch,
				isSameOrigin: (url) => {
					return browser_location.is_same_origin_href(url.href);
				},
				on_snapshot_change: (snapshot) => {
					return work_actor.set_api_requests(
						api_requests_from_submit_snapshot(snapshot),
					);
				},
				redirect: (href, kind) => {
					if (kind === "hard") {
						return browser_location.hard_redirect(href);
					}
					return options.on_client_redirect(href);
				},
				revalidate: (_reason, revalidation_options) => {
					return options.revalidate_api_request(
						route_revalidator,
						revalidation_options,
					);
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
			});
		}),
	);
	const navigation_graph_layer = Layer.provideMerge(prefetch_manager_layer)(
		navigation_actor_layer,
	);
	const revalidation_graph_layer = Layer.provideMerge(navigation_graph_layer)(
		route_revalidator_layer,
	);
	return Layer.provideMerge(revalidation_graph_layer)(submit_manager_layer);
}

function api_requests_from_submit_snapshot(snapshot: SubmitSnapshot) {
	return snapshot.active.map((item) => {
		return {
			key: item.key,
			method: item.method,
			href: item.href,
			skipWorkIndicator: item.skipWorkIndicator,
		};
	});
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
	browser_location: BrowserLocation,
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

function commit_navigation_position(
	attempt: NavigationAttempt,
	href: string,
	browser_history: BrowserHistory,
	scroll_restoration: ScrollRestoration,
): Effect.Effect<HistoryPosition> {
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
