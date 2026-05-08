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

export class BrowserHistoryService extends Context.Service<
	BrowserHistoryService,
	BrowserHistory
>()(browser_history_service_tag) {}

export class RuntimeLifecycleService extends Context.Service<
	RuntimeLifecycleService,
	RuntimeLifecycle
>()(runtime_lifecycle_service_tag) {}

export class ScrollRestorationService extends Context.Service<
	ScrollRestorationService,
	ScrollRestoration
>()(scroll_restoration_service_tag) {}

export class WorkStateActorService extends Context.Service<
	WorkStateActorService,
	WorkStateActor
>()(work_state_actor_service_tag) {}

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
	return Layer.mergeAll(
		Layer.effect(BrowserHistoryService, make_browser_history()),
		Layer.effect(RuntimeLifecycleService, make_runtime_lifecycle()),
		Layer.effect(ScrollRestorationService, make_scroll_restoration()),
		Layer.effect(
			WorkStateActorService,
			make_work_state_actor({
				commit: options.commit,
				on_indicator_update: options.on_indicator_update,
			}),
		),
	);
}
