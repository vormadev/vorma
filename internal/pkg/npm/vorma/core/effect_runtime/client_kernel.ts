import { Effect } from "effect";
import type { BrowserHistory } from "./browser_history.ts";
import type { NavigationActor } from "./navigation_actor.ts";
import type { PrefetchManager } from "./prefetch_manager.ts";
import type { RoutePreparer } from "./route_preparer.ts";
import type { RoutePublisher } from "./route_publisher.ts";
import type { RouteRevalidator } from "./route_revalidator.ts";
import type { RuntimeLifecycle } from "./runtime_lifecycle.ts";
import type { ScrollRestoration } from "./scroll_restoration.ts";
import type { SubmitManager } from "./submit_manager.ts";
import type { WorkStateActor } from "./work_state_actor.ts";

export type EffectClientKernel = {
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

export function add_client_kernel_finalizers(
	kernel: EffectClientKernel,
): Effect.Effect<void> {
	return Effect.gen(function* () {
		yield* kernel.lifecycle.add_finalizer(kernel.work_actor.shutdown);
		yield* kernel.lifecycle.add_finalizer(
			kernel.route_revalidator.shutdown,
		);
		yield* kernel.lifecycle.add_finalizer(kernel.submit_manager.shutdown);
		yield* kernel.lifecycle.add_finalizer(kernel.prefetch_manager.shutdown);
		yield* kernel.lifecycle.add_finalizer(kernel.navigation_actor.shutdown);
	});
}
