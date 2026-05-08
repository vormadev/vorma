import { Context, Effect, Layer } from "effect";
import { BrowserViewTransitionFailed } from "./browser_view_runtime.ts";
import {
	type BuildSkewReporter,
	make_build_skew_reporter,
} from "./build_skew_reporter.ts";
import type { BuildSkewDetectedEvent, CommitFn } from "./client_contract.ts";
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
import type { WorkStateActor } from "./work_state_actor.ts";

const route_fetcher_service_tag = "vorma/RouteFetcherService";
const route_preparer_service_tag = "vorma/RoutePreparerService";
const route_publisher_service_tag = "vorma/RoutePublisherService";
const build_skew_reporter_service_tag = "vorma/BuildSkewReporterService";

export class RouteFetcherService extends Context.Tag(route_fetcher_service_tag)<
	RouteFetcherService,
	RouteFetcher
>() {}

export class RoutePreparerService extends Context.Tag(
	route_preparer_service_tag,
)<RoutePreparerService, RoutePreparer>() {}

export class RoutePublisherService extends Context.Tag(
	route_publisher_service_tag,
)<RoutePublisherService, RoutePublisher>() {}

export class BuildSkewReporterService extends Context.Tag(
	build_skew_reporter_service_tag,
)<BuildSkewReporterService, BuildSkewReporter>() {}

export type ClientRouteServices = {
	build_skew_reporter: BuildSkewReporter;
	route_fetcher: RouteFetcher;
	route_preparer: RoutePreparer;
	route_publisher: RoutePublisher;
};

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
	work_actor: WorkStateActor;
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
	ClientRuntimeServicesLayerContext
> {
	return Layer.effectContext(
		make_client_route_services(options).pipe(
			Effect.map((services) => {
				return Context.mergeAll(
					Context.make(
						BuildSkewReporterService,
						services.build_skew_reporter,
					),
					Context.make(RouteFetcherService, services.route_fetcher),
					Context.make(RoutePreparerService, services.route_preparer),
					Context.make(
						RoutePublisherService,
						services.route_publisher,
					),
				);
			}),
		),
	);
}

export function client_route_services_to_layer(
	services: ClientRouteServices,
): Layer.Layer<ClientRouteServicesLayerContext, never> {
	return Layer.mergeAll(
		Layer.succeed(BuildSkewReporterService, services.build_skew_reporter),
		Layer.succeed(RouteFetcherService, services.route_fetcher),
		Layer.succeed(RoutePreparerService, services.route_preparer),
		Layer.succeed(RoutePublisherService, services.route_publisher),
	);
}

export function make_client_route_services(
	options: ClientRouteServicesOptions,
): Effect.Effect<
	ClientRouteServices,
	never,
	ClientRuntimeServicesLayerContext
> {
	return Effect.gen(function* () {
		const browser_fetch_runtime = yield* BrowserFetchRuntimeService;
		const browser_view_runtime = yield* BrowserViewRuntimeService;
		const module_runtime = yield* ModuleRuntimeService;
		const route_dom_runtime = yield* RouteDOMRuntimeService;

		const route_preparer = yield* make_route_preparer({
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
		const route_publisher = yield* make_route_publisher({
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
									error instanceof BrowserViewTransitionFailed
								) {
									return new RouteCommitFailed({ error });
								}
								return error;
							}),
						);
				});
			},
		});
		const route_fetcher = make_route_fetcher({
			client_build_id: options.client_build_id,
			deployment_id: options.deployment_id,
			fetch: browser_fetch_runtime.fetch,
		});
		const build_skew_reporter = make_build_skew_reporter({
			client_build_id: options.client_build_id,
			current_route_state: route_publisher.route_state,
			current_work_state: options.work_actor.snapshot,
			on_detected: options.on_build_skew_detected,
		});

		return {
			build_skew_reporter,
			route_fetcher,
			route_preparer,
			route_publisher,
		};
	});
}
