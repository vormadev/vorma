import { Effect, Exit, Scope } from "effect";
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
import {
	BrowserLocationService,
	ModuleRuntimeService,
	type ClientRuntimeServicesLayerContext,
} from "./client_runtime_services.ts";
import { install_window_focus_revalidator } from "./focus_revalidator.ts";
import type { RoutePrepareInput, RouteRecord } from "./route_preparer.ts";
import type { RouteRevalidator } from "./route_revalidator.ts";
import { WINDOW_EVENT_POPSTATE } from "./runtime_lifecycle.ts";
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

export type EffectClientKernelHandle = {
	kernel: EffectClientKernel;
	shutdown: Effect.Effect<void>;
};

export function make_effect_client_kernel(
	options: EffectClientKernelOptions,
): Effect.Effect<EffectClientKernel, never, ClientRuntimeServicesLayerContext> {
	return Effect.gen(function* () {
		const kernel_resources = yield* make_client_kernel_resources({
			commit: options.commit,
			on_indicator_update: options.on_indicator_update,
		});
		const browser_location = yield* BrowserLocationService;
		const module_runtime = yield* ModuleRuntimeService;
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
		const boot_initial_route: EffectClientKernel["boot_initial_route"] = (
			raw_payload,
		) => {
			return Effect.gen(function* () {
				yield* scroll_restoration.set_manual_restoration;
				const position = yield* browser_history.ensure_current;
				const prepared = yield* route_preparer.prepare_route({
					raw_payload,
					url: new URL(position.href),
					trigger: "boot",
					href: position.href,
					history_state: position.state,
				});
				yield* route_publisher.publish({
					reason: "initial",
					prepared,
					position,
					scroll: yield* scroll_restoration.boot_scroll(position),
				});
			});
		};
		const handle_popstate: EffectClientKernel["handle_popstate"] =
			Effect.gen(function* () {
				const previous = yield* browser_history.current;
				const position = yield* browser_history.adopt_current;
				if (
					position.key === previous.key &&
					position.href === previous.href
				) {
					return;
				}
				if (previous.key.length > 0 && previous.key !== position.key) {
					yield* scroll_restoration.save_current(previous);
				}
				if (
					browser_location.route_key(position.href) ===
					browser_location.route_key(previous.href)
				) {
					const scroll =
						yield* scroll_restoration.popstate_scroll(position);
					const work = yield* work_actor.snapshot;
					yield* route_publisher.move_position({
						reason: "popstate",
						position,
						scroll,
						work,
					});
					return;
				}
				yield* route_revalidator.cancel;
				yield* navigation_actor.navigate(position.href, {
					source: "popstate",
					state: position.state,
					scrollToTop: false,
					skipWorkIndicator: true,
				});
			}).pipe(
				Effect.catchAll(() => {
					return Effect.void;
				}),
			);
		const install_browser_handlers: EffectClientKernel["install_browser_handlers"] =
			Effect.gen(function* () {
				yield* lifecycle.listen_window(WINDOW_EVENT_POPSTATE, () => {
					void Effect.runPromise(handle_popstate);
				});
				yield* scroll_restoration.install_reload_scroll_saver(
					lifecycle,
				);
				yield* module_runtime.install_hmr_handler({
					lifecycle,
					route_preparer,
					route_publisher,
					work_actor,
				});
			});
		const install_focus_revalidator: EffectClientKernel["install_focus_revalidator"] =
			(stale_ms) => {
				return install_window_focus_revalidator({
					lifecycle,
					stale_ms,
					get_work_state: work_actor.snapshot,
					request_revalidation: route_revalidator.request,
				});
			};
		const move_within_current_route: EffectClientKernel["move_within_current_route"] =
			(target_href, input_options) => {
				return Effect.gen(function* () {
					const snapshot = yield* route_publisher.snapshot;
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
					const target_hash =
						browser_location.hash_fragment(target_href);
					const did_change_hash = current_hash !== target_hash;
					const should_commit_history =
						did_change_hash || input_options?.replace === true;
					const scroll = yield* scroll_restoration.navigation_scroll(
						target_href,
						input_options?.scrollToTop,
					);
					if (!should_commit_history) {
						if (scroll) {
							const work = yield* work_actor.snapshot;
							yield* route_publisher.move_position({
								reason: "navigation",
								position: snapshot.position,
								scroll,
								work,
							});
						}
						return { didNavigate: false };
					}
					const previous_position = yield* browser_history.current;
					yield* scroll_restoration.save_current(previous_position);
					const position = yield* browser_history.commit(
						target_href,
						input_options?.replace === true,
						input_options?.state,
					);
					const work = yield* work_actor.snapshot;
					yield* route_publisher.move_position({
						reason: "navigation",
						position,
						scroll,
						work,
					});
					return { didNavigate: did_change_hash };
				});
			};
		const kernel: EffectClientKernel = {
			browser_history,
			boot_initial_route,
			client_build_id: options.client_build_id,
			deployment_id: options.deployment_id,
			handle_popstate,
			install_browser_handlers,
			install_focus_revalidator,
			lifecycle,
			move_within_current_route,
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

export function acquire_effect_client_kernel(
	options: EffectClientKernelOptions,
): Effect.Effect<
	EffectClientKernelHandle,
	never,
	ClientRuntimeServicesLayerContext
> {
	return Effect.gen(function* () {
		const scope = yield* Scope.make();
		const kernel = yield* make_effect_client_kernel(options).pipe(
			Scope.extend(scope),
		);
		yield* Scope.addFinalizer(scope, kernel.lifecycle.shutdown);
		return {
			kernel,
			shutdown: Scope.close(scope, Exit.void),
		};
	});
}
