import { Effect } from "effect";
import type {
	ClientLoaderServerState,
	ViewDefinition,
} from "./client_contract.ts";
import {
	type RouteModule,
	RouteModuleLoadFailed,
	type RoutePreparer,
	type RouteRecord,
} from "./route_preparer.ts";
import type { RoutePublisher } from "./route_publisher.ts";
import type { RuntimeLifecycle } from "./runtime_lifecycle.ts";
import type { WorkStateActor } from "./work_state_actor.ts";

const hmr_client_loader_error_message =
	"Vorma: HMR client loader re-run failed";

export type ModuleRuntime = {
	load_module: (
		module_url: string,
	) => Effect.Effect<RouteModule, RouteModuleLoadFailed>;
	set_hmr_rerun: (pattern: string, enabled: boolean) => Effect.Effect<void>;
	install_hmr_handler: (deps: ModuleRuntimeHMRDeps) => Effect.Effect<void>;
};

export type ModuleRuntimeHMRDeps = {
	lifecycle: RuntimeLifecycle;
	route_preparer: RoutePreparer;
	route_publisher: RoutePublisher;
	work_actor: WorkStateActor;
};

export type ModuleRuntimeOptions = {
	dev?: boolean;
	import_module?: (module_url: string) => Promise<RouteModule>;
	console_error?: (message: string, error: unknown) => void;
};

export function make_module_runtime(
	options: ModuleRuntimeOptions = {},
): Effect.Effect<ModuleRuntime> {
	return Effect.sync(() => {
		const dev = options.dev ?? default_dev_mode();
		const module_cache = new Map<string, RouteModule>();
		const hmr_rerun_patterns = new Set<string>();
		const import_module =
			options.import_module ??
			((module_url: string): Promise<RouteModule> => {
				return import(
					/* @vite-ignore */ module_url
				) as Promise<RouteModule>;
			});
		const console_error =
			options.console_error ??
			((message: string, error: unknown): void => {
				console.error(message, error);
			});

		const load_module = (
			module_url: string,
		): Effect.Effect<RouteModule, RouteModuleLoadFailed> => {
			return Effect.tryPromise({
				try: async () => {
					const cache_key = normalize_module_url(module_url);
					if (dev) {
						const cached = module_cache.get(cache_key);
						if (cached) {
							return cached;
						}
					}
					const loaded_module = await import_module(module_url);
					if (dev) {
						module_cache.set(cache_key, loaded_module);
					}
					return loaded_module;
				},
				catch: (error) => {
					return new RouteModuleLoadFailed({ module_url, error });
				},
			});
		};

		const set_hmr_rerun = (
			pattern: string,
			enabled: boolean,
		): Effect.Effect<void> => {
			return Effect.sync(() => {
				if (!dev) {
					return;
				}
				if (enabled) {
					hmr_rerun_patterns.add(pattern);
					return;
				}
				hmr_rerun_patterns.delete(pattern);
			});
		};

		const install_hmr_handler = (
			deps: ModuleRuntimeHMRDeps,
		): Effect.Effect<void> => {
			return Effect.gen(function* () {
				if (!dev) {
					return;
				}
				const handler: Window["__vorma_hmr_route_update"] = async (
					raw_url,
					mod,
				) => {
					await Effect.runPromise(
						handle_hmr_route_update(
							deps,
							module_cache,
							hmr_rerun_patterns,
							console_error,
							raw_url,
							mod as RouteModule,
						).pipe(
							Effect.catchAll(() => {
								return Effect.void;
							}),
							Effect.catchAllDefect(() => {
								return Effect.void;
							}),
						),
					);
				};
				yield* Effect.sync(() => {
					window.__vorma_hmr_route_update = handler;
				});
				yield* deps.lifecycle.add_finalizer(
					Effect.sync(() => {
						if (window.__vorma_hmr_route_update === handler) {
							window.__vorma_hmr_route_update = undefined;
						}
					}),
				);
			});
		};

		return {
			load_module,
			set_hmr_rerun,
			install_hmr_handler,
		};
	});
}

function default_dev_mode(): boolean {
	const meta = import.meta as {
		env?: { DEV?: boolean };
	};
	return meta.env?.DEV === true;
}

function normalize_module_url(url: string): string {
	return new URL(url, window.location.href).pathname;
}

function handle_hmr_route_update(
	deps: ModuleRuntimeHMRDeps,
	module_cache: Map<string, RouteModule>,
	hmr_rerun_patterns: Set<string>,
	console_error: (message: string, error: unknown) => void,
	raw_url: string,
	route_module: RouteModule,
): Effect.Effect<void, unknown> {
	return Effect.gen(function* () {
		const snapshot = yield* deps.route_publisher.snapshot;
		if (!snapshot) {
			return;
		}
		const module_url = normalize_module_url(raw_url);
		module_cache.set(module_url, route_module);
		const idx = snapshot.route.matches.findIndex((match) => {
			return normalize_module_url(match.module_url) === module_url;
		});
		if (idx === -1) {
			return;
		}
		const match = snapshot.route.matches[idx]!;
		const view_definition = route_module.default as
			| Partial<ViewDefinition>
			| undefined;
		if (typeof view_definition?.client_loader === "function") {
			yield* deps.route_preparer.catalog.set(
				match.pattern,
				view_definition.client_loader,
			);
		}
		let client_loader_data = match.client_loader_data;
		if (
			hmr_rerun_patterns.has(match.pattern) &&
			typeof view_definition?.client_loader === "function"
		) {
			client_loader_data = yield* run_hmr_client_loader(
				view_definition.client_loader,
				snapshot.route,
				idx,
				snapshot.position.href,
				snapshot.position.state,
				console_error,
			);
		}
		const route: RouteRecord = {
			...snapshot.route,
			matches: snapshot.route.matches.map((item, item_idx) => {
				if (item_idx !== idx) {
					return item;
				}
				return {
					...item,
					module: route_module,
					client_loader_data,
				};
			}),
		};
		const work = yield* deps.work_actor.snapshot;
		yield* deps.route_publisher.replace_current_route({
			reason: "hmr",
			route,
			work,
		});
	});
}

function run_hmr_client_loader(
	client_loader: NonNullable<ViewDefinition["client_loader"]>,
	route: RouteRecord,
	idx: number,
	href: string,
	history_state: unknown,
	console_error: (message: string, error: unknown) => void,
): Effect.Effect<unknown> {
	return Effect.gen(function* () {
		const match = route.matches[idx]!;
		const result = yield* Effect.either(
			Effect.tryPromise({
				try: () => {
					return client_loader({
						trigger: "revalidation",
						href,
						historyState: history_state,
						pattern: match.pattern,
						params: route.params,
						splatValues: route.splat_values,
						input: match.input,
						knownMatches: hmr_known_matches(route),
						serverPromise: Promise.resolve(
							hmr_server_state(route, idx),
						),
						signal: new AbortController().signal,
					});
				},
				catch: (error) => {
					return error;
				},
			}),
		);
		if (result._tag === "Right") {
			return result.right;
		}
		yield* Effect.sync(() => {
			console_error(hmr_client_loader_error_message, result.left);
		});
		return match.client_loader_data;
	});
}

function hmr_known_matches(route: RouteRecord) {
	return route.matches.map((match) => {
		return {
			pattern: match.pattern,
			input: match.input,
		};
	});
}

function hmr_server_state(
	route: RouteRecord,
	idx: number,
): ClientLoaderServerState {
	const server_error = route.error?.source === "server" ? route.error : null;
	return {
		clientBuildID: route.client_build_id,
		matches: route.matches.map((match) => {
			return {
				pattern: match.pattern,
				input: match.input,
				loaderData: match.loader_data,
			};
		}),
		outermostServerError: server_error
			? {
					idx: server_error.idx,
					error: server_error.error,
				}
			: null,
		loaderData: route.matches[idx]?.loader_data,
	};
}

declare global {
	interface Window {
		__vorma_hmr_route_update?: (
			raw_url: string,
			mod: Record<string, unknown>,
		) => Promise<void>;
	}
}
