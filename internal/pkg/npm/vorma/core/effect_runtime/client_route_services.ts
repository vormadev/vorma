import { Context, Effect, Layer } from "effect";
import { BrowserViewTransitionFailed } from "./browser_view_runtime.ts";
import {
	type BuildSkewReporter,
	make_build_skew_reporter,
} from "./build_skew_reporter.ts";
import type { BuildSkewDetectedEvent, CommitFn } from "./client_contract.ts";
import {
	type ClientKernelResourcesLayerContext,
	WorkStateActorService,
} from "./client_kernel_resources.ts";
import {
	BrowserFetchRuntimeService,
	BrowserViewRuntimeService,
	type ClientRuntimeServicesLayerContext,
	ModuleRuntimeService,
	RouteDOMRuntimeService,
} from "./client_runtime_services.ts";
import { type RouteFetcher, make_route_fetcher } from "./route_fetcher.ts";
import {
	type RoutePrepareInput,
	type RoutePreparer,
	type RouteRecord,
	make_route_preparer,
} from "./route_preparer.ts";
import {
	RouteCommitFailed,
	type RoutePublisher,
	make_route_publisher,
} from "./route_publisher.ts";

const route_fetcher_service_tag = "vorma/RouteFetcherService";
const route_preparer_service_tag = "vorma/RoutePreparerService";
const route_publisher_service_tag = "vorma/RoutePublisherService";
const build_skew_reporter_service_tag = "vorma/BuildSkewReporterService";

export class RouteFetcherService extends Context.Service<
	RouteFetcherService,
	RouteFetcher
>()(route_fetcher_service_tag) {}

export class RoutePreparerService extends Context.Service<
	RoutePreparerService,
	RoutePreparer
>()(route_preparer_service_tag) {}

export class RoutePublisherService extends Context.Service<
	RoutePublisherService,
	RoutePublisher
>()(route_publisher_service_tag) {}

export class BuildSkewReporterService extends Context.Service<
	BuildSkewReporterService,
	BuildSkewReporter
>()(build_skew_reporter_service_tag) {}

export type ClientRouteServicesOptions = {
	client_build_id: string;
	commit: CommitFn;
	deployment_id: string;
	on_build_skew_detected: (
		event: BuildSkewDetectedEvent,
	) => Effect.Effect<void>;
	on_provisional_route: (input: {
		route: RouteRecord;
		prepare_input: RoutePrepareInput;
	}) => Effect.Effect<void>;
	use_view_transitions: Effect.Effect<boolean>;
};

export type ClientRouteServicesLayerContext =
	| BuildSkewReporterService
	| RouteFetcherService
	| RoutePreparerService
	| RoutePublisherService;

export function make_client_route_services_layer(
	options: ClientRouteServicesOptions,
): Layer.Layer<
	ClientRouteServicesLayerContext,
	never,
	ClientKernelResourcesLayerContext | ClientRuntimeServicesLayerContext
> {
	const route_services_layer = Layer.mergeAll(
		Layer.effect(
			RouteFetcherService,
			Effect.gen(function* () {
				const browser_fetch_runtime = yield* BrowserFetchRuntimeService;
				return make_route_fetcher({
					client_build_id: options.client_build_id,
					deployment_id: options.deployment_id,
					fetch: browser_fetch_runtime.fetch,
				});
			}),
		),
		Layer.effect(
			RoutePreparerService,
			Effect.gen(function* () {
				const module_runtime = yield* ModuleRuntimeService;
				const route_dom_runtime = yield* RouteDOMRuntimeService;
				return yield* make_route_preparer({
					client_build_id: options.client_build_id,
					load_module: (module_url) => {
						return module_runtime.load_module(module_url);
					},
					preload_css: route_dom_runtime.preload_css,
					wait_for_css: route_dom_runtime.wait_for_css,
					apply_payload_side_effects:
						route_dom_runtime.apply_payload_side_effects,
					on_provisional_route: options.on_provisional_route,
					decode_title: route_dom_runtime.decode_title,
				});
			}),
		),
		Layer.effect(
			RoutePublisherService,
			Effect.gen(function* () {
				const browser_view_runtime = yield* BrowserViewRuntimeService;
				return yield* make_route_publisher({
					commit: options.commit,
					apply_scroll: browser_view_runtime.apply_scroll,
					run_view_transition: (publish_effect) => {
						return Effect.gen(function* () {
							const enabled = yield* options.use_view_transitions;
							return yield* browser_view_runtime
								.run_view_transition({
									enabled,
									publish: publish_effect,
								})
								.pipe(
									Effect.mapError((error) => {
										if (
											error instanceof
											BrowserViewTransitionFailed
										) {
											return new RouteCommitFailed({
												error,
											});
										}
										return error;
									}),
								);
						});
					},
				});
			}),
		),
	);
	const build_skew_reporter_layer = Layer.effect(
		BuildSkewReporterService,
		Effect.gen(function* () {
			const route_publisher = yield* RoutePublisherService;
			const work_actor = yield* WorkStateActorService;
			return make_build_skew_reporter({
				client_build_id: options.client_build_id,
				current_route_state: route_publisher.route_state,
				current_work_state: work_actor.snapshot,
				on_detected: options.on_build_skew_detected,
			});
		}),
	);
	return Layer.provideMerge(route_services_layer)(build_skew_reporter_layer);
}
