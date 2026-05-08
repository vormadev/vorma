import { Context, Effect, Layer } from "effect";
import {
	type BrowserHistory,
	make_browser_history,
} from "./browser_history.ts";
import type { CommitFn } from "./client_contract.ts";
import {
	type RuntimeLifecycle,
	make_runtime_lifecycle,
} from "./runtime_lifecycle.ts";
import {
	type ScrollRestoration,
	make_scroll_restoration,
} from "./scroll_restoration.ts";
import {
	type WorkIndicatorActivity,
	type WorkStateActor,
	make_work_state_actor,
} from "./work_state_actor.ts";

const browser_history_service_tag = "vorma/BrowserHistoryService";
const runtime_lifecycle_service_tag = "vorma/RuntimeLifecycleService";
const scroll_restoration_service_tag = "vorma/ScrollRestorationService";
const work_state_actor_service_tag = "vorma/WorkStateActorService";

export class BrowserHistoryService extends Context.Tag(
	browser_history_service_tag,
)<BrowserHistoryService, BrowserHistory>() {}

export class RuntimeLifecycleService extends Context.Tag(
	runtime_lifecycle_service_tag,
)<RuntimeLifecycleService, RuntimeLifecycle>() {}

export class ScrollRestorationService extends Context.Tag(
	scroll_restoration_service_tag,
)<ScrollRestorationService, ScrollRestoration>() {}

export class WorkStateActorService extends Context.Tag(
	work_state_actor_service_tag,
)<WorkStateActorService, WorkStateActor>() {}

export type ClientKernelResources = {
	browser_history: BrowserHistory;
	lifecycle: RuntimeLifecycle;
	scroll_restoration: ScrollRestoration;
	work_actor: WorkStateActor;
};

export type ClientKernelResourcesOptions = {
	commit: CommitFn;
	on_indicator_update: (
		activity: WorkIndicatorActivity,
	) => Effect.Effect<void>;
};

export type ClientKernelResourcesLayerContext =
	| BrowserHistoryService
	| RuntimeLifecycleService
	| ScrollRestorationService
	| WorkStateActorService;

export function make_client_kernel_resources_layer(
	options: ClientKernelResourcesOptions,
): Layer.Layer<ClientKernelResourcesLayerContext, never> {
	return Layer.effectContext(
		make_client_kernel_resources(options).pipe(
			Effect.map((resources) => {
				return Context.mergeAll(
					Context.make(
						BrowserHistoryService,
						resources.browser_history,
					),
					Context.make(RuntimeLifecycleService, resources.lifecycle),
					Context.make(
						ScrollRestorationService,
						resources.scroll_restoration,
					),
					Context.make(WorkStateActorService, resources.work_actor),
				);
			}),
		),
	);
}

export function client_kernel_resources_to_layer(
	resources: ClientKernelResources,
): Layer.Layer<ClientKernelResourcesLayerContext, never> {
	return Layer.mergeAll(
		Layer.succeed(BrowserHistoryService, resources.browser_history),
		Layer.succeed(RuntimeLifecycleService, resources.lifecycle),
		Layer.succeed(ScrollRestorationService, resources.scroll_restoration),
		Layer.succeed(WorkStateActorService, resources.work_actor),
	);
}

export function make_client_kernel_resources(
	options: ClientKernelResourcesOptions,
): Effect.Effect<ClientKernelResources, never> {
	return Effect.gen(function* () {
		const lifecycle = yield* make_runtime_lifecycle();
		const browser_history = yield* make_browser_history();
		const scroll_restoration = yield* make_scroll_restoration();
		const work_actor = yield* make_work_state_actor({
			commit: options.commit,
			on_indicator_update: options.on_indicator_update,
		});

		return {
			browser_history,
			lifecycle,
			scroll_restoration,
			work_actor,
		};
	});
}
