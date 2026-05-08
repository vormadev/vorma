import { Effect } from "effect";
import type { RevalidationResult } from "../types.ts";
import type { BuildSkewDetectedEvent, CommitFn } from "./client_contract.ts";
import {
	add_client_kernel_finalizers,
	type EffectClientKernel,
} from "./client_kernel.ts";
import {
	client_kernel_resources_to_layer,
	make_client_kernel_resources,
} from "./client_kernel_resources.ts";
import { make_client_navigation_services } from "./client_navigation_services.ts";
import {
	client_route_services_to_layer,
	make_client_route_services,
} from "./client_route_services.ts";
import type { ClientRuntimeServicesLayerContext } from "./client_runtime_services.ts";
import type { RoutePrepareInput, RouteRecord } from "./route_preparer.ts";
import type { RouteRevalidator } from "./route_revalidator.ts";
import type { WorkIndicatorActivity } from "./work_state_actor.ts";

export type EffectClientKernelOptions = {
	client_build_id: string;
	commit: CommitFn;
	deployment_id: string;
	on_build_skew_detected: (
		event: BuildSkewDetectedEvent,
	) => Effect.Effect<void>;
	on_client_redirect: (href: string) => Effect.Effect<void>;
	on_indicator_update: (
		activity: WorkIndicatorActivity,
	) => Effect.Effect<void>;
	on_provisional_route: (input: {
		route: RouteRecord;
		prepare_input: RoutePrepareInput;
	}) => Effect.Effect<void>;
	revalidate_api_request: (
		route_revalidator: RouteRevalidator,
	) => Effect.Effect<RevalidationResult>;
	use_view_transitions: Effect.Effect<boolean>;
};

export function make_effect_client_kernel(
	options: EffectClientKernelOptions,
): Effect.Effect<EffectClientKernel, never, ClientRuntimeServicesLayerContext> {
	return Effect.gen(function* () {
		const kernel_resources = yield* make_client_kernel_resources({
			commit: options.commit,
			on_indicator_update: options.on_indicator_update,
		});
		const { browser_history, lifecycle, scroll_restoration, work_actor } =
			kernel_resources;
		const route_services = yield* make_client_route_services({
			client_build_id: options.client_build_id,
			commit: options.commit,
			deployment_id: options.deployment_id,
			on_build_skew_detected: options.on_build_skew_detected,
			on_provisional_route: options.on_provisional_route,
			use_view_transitions: options.use_view_transitions,
			work_actor,
		});
		const { route_preparer, route_publisher } = route_services;
		const navigation_services = yield* make_client_navigation_services({
			deployment_id: options.deployment_id,
			on_client_redirect: options.on_client_redirect,
			revalidate_api_request: options.revalidate_api_request,
		}).pipe(
			Effect.provide([
				client_kernel_resources_to_layer(kernel_resources),
				client_route_services_to_layer(route_services),
			]),
		);
		const {
			navigation_actor,
			prefetch_manager,
			route_revalidator,
			submit_manager,
		} = navigation_services;
		const kernel: EffectClientKernel = {
			browser_history,
			client_build_id: options.client_build_id,
			deployment_id: options.deployment_id,
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
		yield* add_client_kernel_finalizers(kernel);
		return kernel;
	});
}
