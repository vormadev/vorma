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

export class NavigationActorService extends Context.Tag(
	navigation_actor_service_tag,
)<NavigationActorService, NavigationActor>() {}

export class PrefetchManagerService extends Context.Tag(
	prefetch_manager_service_tag,
)<PrefetchManagerService, PrefetchManager>() {}

export class RouteRevalidatorService extends Context.Tag(
	route_revalidator_service_tag,
)<RouteRevalidatorService, RouteRevalidator>() {}

export class SubmitManagerService extends Context.Tag(
	submit_manager_service_tag,
)<SubmitManagerService, SubmitManager>() {}

export type ClientNavigationServices = {
	navigation_actor: NavigationActor;
	prefetch_manager: PrefetchManager;
	route_revalidator: RouteRevalidator;
	submit_manager: SubmitManager;
};

export type ClientNavigationServicesOptions = {
	deployment_id: string;
	on_client_redirect: (href: string) => Effect.Effect<void>;
	revalidate_api_request: (
		route_revalidator: RouteRevalidator,
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
	return Layer.effectContext(
		make_client_navigation_services(options).pipe(
			Effect.map((services) => {
				return Context.mergeAll(
					Context.make(
						NavigationActorService,
						services.navigation_actor,
					),
					Context.make(
						PrefetchManagerService,
						services.prefetch_manager,
					),
					Context.make(
						RouteRevalidatorService,
						services.route_revalidator,
					),
					Context.make(SubmitManagerService, services.submit_manager),
				);
			}),
		),
	);
}

export function client_navigation_services_to_layer(
	services: ClientNavigationServices,
): Layer.Layer<ClientNavigationServicesLayerContext, never> {
	return Layer.mergeAll(
		Layer.succeed(NavigationActorService, services.navigation_actor),
		Layer.succeed(PrefetchManagerService, services.prefetch_manager),
		Layer.succeed(RouteRevalidatorService, services.route_revalidator),
		Layer.succeed(SubmitManagerService, services.submit_manager),
	);
}

export function make_client_navigation_services(
	options: ClientNavigationServicesOptions,
): Effect.Effect<
	ClientNavigationServices,
	never,
	ClientNavigationServicesRequirements
> {
	return Effect.gen(function* () {
		const browser_history = yield* BrowserHistoryService;
		const browser_location = yield* BrowserLocationService;
		const build_skew_reporter = yield* BuildSkewReporterService;
		const route_fetcher = yield* RouteFetcherService;
		const route_preparer = yield* RoutePreparerService;
		const route_publisher = yield* RoutePublisherService;
		const scroll_restoration = yield* ScrollRestorationService;
		const submit_dispatcher = yield* SubmitDispatcherService;
		const work_actor = yield* WorkStateActorService;

		const prefetch_manager = yield* make_prefetch_manager({
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

		let navigation_actor: NavigationActor | null = null;
		const created_navigation_actor = yield* make_navigation_actor({
			isExternal: (href) => {
				return !browser_location.is_same_origin_href(href);
			},
			routeKey: browser_location.route_key,
			prestart: (attempt) => {
				const url = browser_location.resolve_url(attempt.href);
				return Effect.gen(function* () {
					const prefetch_snapshot = yield* prefetch_manager.snapshot;
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
					const prefetched = yield* prefetch_manager.take(url.href);
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
						skipWorkIndicator: attempt.skipWorkIndicator === true,
						source: navigation_source(attempt),
					});
					const fetch_result = yield* route_fetcher
						.fetch_route({ url, signal: attempt.signal })
						.pipe(Effect.mapError(map_navigation_failure));
					if ("response" in fetch_result && fetch_result.response) {
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
						if (!browser_location.is_http_href(fetch_result.href)) {
							return yield* Effect.fail(
								new NavigationLoadFailed({
									error: fetch_result,
								}),
							);
						}
						const current_route = yield* route_publisher.snapshot;
						if (
							current_route &&
							browser_location.route_key(
								current_route.position.href,
							) === browser_location.route_key(fetch_result.href)
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
			publish: (loaded, attempt) => {
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
								guard: active_navigation_guard(() => {
									return navigation_actor;
								}, attempt),
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
		navigation_actor = created_navigation_actor;

		const route_revalidator = yield* make_route_revalidator({
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
		});

		const submit_manager = yield* make_submit_manager({
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
			revalidate: () => {
				return options.revalidate_api_request(route_revalidator);
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

		return {
			navigation_actor,
			prefetch_manager,
			route_revalidator,
			submit_manager,
		};
	});
}

function active_navigation_guard(
	current_navigation_actor: () => NavigationActor | null,
	attempt: NavigationAttempt,
): Effect.Effect<boolean> {
	return Effect.gen(function* () {
		const navigation_actor = current_navigation_actor();
		if (!navigation_actor) {
			return false;
		}
		const snapshot = yield* navigation_actor.snapshot;
		return snapshot.active?.id === attempt.id;
	});
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
