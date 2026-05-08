import { Context, Effect, Layer } from "effect";
import {
	type BrowserFetchRuntime,
	make_browser_fetch_runtime,
} from "./browser_fetch_runtime.ts";
import {
	type BrowserLocation,
	make_browser_location,
} from "./browser_location.ts";
import {
	type BrowserViewRuntime,
	make_browser_view_runtime,
} from "./browser_view_runtime.ts";
import { type ModuleRuntime, make_module_runtime } from "./module_runtime.ts";
import {
	type RouteDOMRuntime,
	make_route_dom_runtime,
} from "./route_dom_runtime.ts";
import {
	type SubmitDispatcher,
	make_submit_dispatcher,
} from "./submit_dispatcher.ts";
import {
	type WorkIndicatorRuntime,
	make_work_indicator,
} from "./work_indicator.ts";

const browser_fetch_runtime_service_tag = "vorma/BrowserFetchRuntimeService";
const browser_location_service_tag = "vorma/BrowserLocationService";
const browser_view_runtime_service_tag = "vorma/BrowserViewRuntimeService";
const module_runtime_service_tag = "vorma/ModuleRuntimeService";
const route_dom_runtime_service_tag = "vorma/RouteDOMRuntimeService";
const submit_dispatcher_service_tag = "vorma/SubmitDispatcherService";
const work_indicator_runtime_service_tag = "vorma/WorkIndicatorRuntimeService";

export class BrowserFetchRuntimeService extends Context.Tag(
	browser_fetch_runtime_service_tag,
)<BrowserFetchRuntimeService, BrowserFetchRuntime>() {}

export class BrowserLocationService extends Context.Tag(
	browser_location_service_tag,
)<BrowserLocationService, BrowserLocation>() {}

export class BrowserViewRuntimeService extends Context.Tag(
	browser_view_runtime_service_tag,
)<BrowserViewRuntimeService, BrowserViewRuntime>() {}

export class ModuleRuntimeService extends Context.Tag(
	module_runtime_service_tag,
)<ModuleRuntimeService, ModuleRuntime>() {}

export class RouteDOMRuntimeService extends Context.Tag(
	route_dom_runtime_service_tag,
)<RouteDOMRuntimeService, RouteDOMRuntime>() {}

export class SubmitDispatcherService extends Context.Tag(
	submit_dispatcher_service_tag,
)<SubmitDispatcherService, SubmitDispatcher>() {}

export class WorkIndicatorRuntimeService extends Context.Tag(
	work_indicator_runtime_service_tag,
)<WorkIndicatorRuntimeService, WorkIndicatorRuntime>() {}

export type ClientRuntimeServices = {
	browser_fetch_runtime: BrowserFetchRuntime;
	browser_location: BrowserLocation;
	browser_view_runtime: BrowserViewRuntime;
	module_runtime: ModuleRuntime;
	route_dom_runtime: RouteDOMRuntime;
	submit_dispatcher: SubmitDispatcher;
	work_indicator_runtime: WorkIndicatorRuntime;
};

export type ClientRuntimeServicesOptions = {
	hard_redirect?: (href: string) => void;
	scroll_to?: (x: number, y: number) => void;
};

export type ClientRuntimeServicesLayerContext =
	| BrowserFetchRuntimeService
	| BrowserLocationService
	| BrowserViewRuntimeService
	| ModuleRuntimeService
	| RouteDOMRuntimeService
	| SubmitDispatcherService
	| WorkIndicatorRuntimeService;

export function make_client_runtime_services_layer(
	options: ClientRuntimeServicesOptions = {},
): Layer.Layer<ClientRuntimeServicesLayerContext, never> {
	const browser_runtime_layer = Layer.mergeAll(
		Layer.effect(BrowserFetchRuntimeService, make_browser_fetch_runtime()),
		Layer.effect(
			BrowserLocationService,
			make_browser_location({
				hard_redirect: options.hard_redirect,
			}),
		),
		Layer.effect(
			BrowserViewRuntimeService,
			make_browser_view_runtime({
				scroll_to: options.scroll_to,
			}),
		),
		Layer.effect(ModuleRuntimeService, make_module_runtime()),
		Layer.effect(RouteDOMRuntimeService, make_route_dom_runtime()),
		Layer.effect(WorkIndicatorRuntimeService, make_work_indicator()),
	);
	const submit_dispatcher_layer = Layer.effect(
		SubmitDispatcherService,
		Effect.gen(function* () {
			const browser_fetch_runtime = yield* BrowserFetchRuntimeService;
			return yield* make_submit_dispatcher({
				fetch: browser_fetch_runtime.fetch,
			});
		}),
	);
	return Layer.provideMerge(browser_runtime_layer)(submit_dispatcher_layer);
}

export function client_runtime_services_to_layer(
	services: ClientRuntimeServices,
): Layer.Layer<ClientRuntimeServicesLayerContext, never> {
	return Layer.mergeAll(
		Layer.succeed(
			BrowserFetchRuntimeService,
			services.browser_fetch_runtime,
		),
		Layer.succeed(BrowserLocationService, services.browser_location),
		Layer.succeed(BrowserViewRuntimeService, services.browser_view_runtime),
		Layer.succeed(ModuleRuntimeService, services.module_runtime),
		Layer.succeed(RouteDOMRuntimeService, services.route_dom_runtime),
		Layer.succeed(SubmitDispatcherService, services.submit_dispatcher),
		Layer.succeed(
			WorkIndicatorRuntimeService,
			services.work_indicator_runtime,
		),
	);
}

export function make_client_runtime_services(
	options: ClientRuntimeServicesOptions = {},
): Effect.Effect<ClientRuntimeServices, never> {
	return collect_client_runtime_services.pipe(
		Effect.provide(make_client_runtime_services_layer(options)),
	);
}

const collect_client_runtime_services: Effect.Effect<
	ClientRuntimeServices,
	never,
	ClientRuntimeServicesLayerContext
> = Effect.gen(function* () {
	const browser_fetch_runtime = yield* BrowserFetchRuntimeService;
	const browser_location = yield* BrowserLocationService;
	const browser_view_runtime = yield* BrowserViewRuntimeService;
	const module_runtime = yield* ModuleRuntimeService;
	const route_dom_runtime = yield* RouteDOMRuntimeService;
	const submit_dispatcher = yield* SubmitDispatcherService;
	const work_indicator_runtime = yield* WorkIndicatorRuntimeService;

	return {
		browser_fetch_runtime,
		browser_location,
		browser_view_runtime,
		module_runtime,
		route_dom_runtime,
		submit_dispatcher,
		work_indicator_runtime,
	};
});
