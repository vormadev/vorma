import { parseSearchParams } from "vorma/kit/json";
import {
	findNestedMatches,
	registerPattern,
	type PatternRegistry,
} from "vorma/kit/matcher";
import type { ClientLoaderID, RouteToken } from "./events.ts";
import type {
	PreparedRoute,
	PreparedRouteMatch,
	RouteErrorState,
	RoutePayload,
	RouteState,
} from "./model.ts";
import { client_loader_id_for } from "./model.ts";
import type { SearchSchemaRegistry } from "./payload.ts";
import type { Platform } from "./platform.ts";

export type ClientLoaderFn = (args: {
	trigger: "boot" | "navigation" | "revalidation" | "prefetch";
	href: string;
	historyState: unknown;
	pattern: string;
	params: Record<string, string>;
	splatValues: string[];
	input: unknown;
	knownMatches: Array<{ pattern: string; input: unknown }>;
	serverPromise: Promise<ClientLoaderServerState>;
	signal: AbortSignal;
}) => Promise<unknown>;

export type ClientLoaderServerState = {
	clientBuildID: string;
	matches: Array<{ pattern: string; input: unknown; loaderData: unknown }>;
	outermostServerError: null | { idx: number; error: unknown };
	loaderData: unknown;
};

export type BeforeRouteHook = (args: {
	trigger: "navigation" | "popstate" | "revalidation";
	signal: AbortSignal;
	current: RouteState;
	next: RouteState;
}) => Promise<void> | void;

export type MatchHooks = {
	before_yield?: BeforeRouteHook;
	before_commit?: BeforeRouteHook;
};

/////////////////////////////////////////////////////////////////////
/////// Registries
/////////////////////////////////////////////////////////////////////

export type LoaderRegistry = {
	get: (pattern: string) => ClientLoaderFn | undefined;
	set: (pattern: string, loader: ClientLoaderFn | undefined) => void;
	get_rerun_on_hmr: (pattern: string) => boolean;
	set_rerun_on_hmr: (pattern: string, value: boolean) => void;
};

export function create_loader_registry(): LoaderRegistry {
	const loaders = new Map<string, ClientLoaderFn>();
	const rerun_on_hmr = new Set<string>();
	return {
		get: (pattern) => loaders.get(pattern),
		set: (pattern, loader) => {
			if (loader) {
				loaders.set(pattern, loader);
			} else {
				loaders.delete(pattern);
			}
		},
		get_rerun_on_hmr: (pattern) => rerun_on_hmr.has(pattern),
		set_rerun_on_hmr: (pattern, value) => {
			if (value) {
				rerun_on_hmr.add(pattern);
			} else {
				rerun_on_hmr.delete(pattern);
			}
		},
	};
}

export type ModuleCache = {
	get: (url: string) => Record<string, unknown> | undefined;
	set: (url: string, mod: Record<string, unknown>) => void;
};

export function create_module_cache(): ModuleCache {
	const map = new Map<string, Record<string, unknown>>();
	return {
		get: (url) => map.get(url),
		set: (url, mod) => {
			map.set(url, mod);
		},
	};
}

export function module_cache_key(url: string, base: string): string {
	try {
		return new URL(url, base).pathname;
	} catch {
		return url;
	}
}

/////////////////////////////////////////////////////////////////////
/////// Speculative Client Loaders
/////////////////////////////////////////////////////////////////////

export type Deferred<T> = {
	promise: Promise<T>;
	resolve: (value: T) => void;
	reject: (error: unknown) => void;
};

function make_deferred<T>(): Deferred<T> {
	let resolve!: (v: T) => void;
	let reject!: (e: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	promise.catch(() => {});
	return { promise, resolve, reject };
}

function is_abort_error(e: unknown): boolean {
	return e instanceof DOMException && e.name === "AbortError";
}

export type ClientLoaderPrefetch = {
	pattern: string;
	input: unknown;
	resolve_server_state: (state: ClientLoaderServerState) => void;
	reject_server_state: (error: unknown) => void;
	abort: () => void;
	result_promise: Promise<unknown>;
};

export type StartSpeculativeLoadersDeps = {
	pattern_registry: PatternRegistry;
	loader_registry: LoaderRegistry;
	search_schemas: SearchSchemaRegistry;
};

export type StartSpeculativeLoadersInput = {
	url: URL;
	href: string;
	history_state: unknown;
	signal: AbortSignal;
	trigger: "navigation" | "popstate" | "prefetch" | "revalidation";
};

export function start_speculative_loaders(
	deps: StartSpeculativeLoadersDeps,
	input: StartSpeculativeLoadersInput,
): ClientLoaderPrefetch[] {
	let match = findNestedMatches(deps.pattern_registry, input.url.pathname);
	if (!match) {
		const segments = input.url.pathname.split("/").filter(Boolean);
		for (let i = segments.length; i >= 0; i--) {
			const partial =
				i === 0 ? "/" : "/" + segments.slice(0, i).join("/");
			match = findNestedMatches(deps.pattern_registry, partial);
			if (match) {
				break;
			}
		}
	}
	if (!match) {
		return [];
	}

	const known_matches = match.matches.map((m) => {
		const pattern = m.registeredPattern.originalPattern;
		return {
			pattern,
			input: parseSearchParams(
				deps.search_schemas.get(pattern),
				input.url.searchParams,
			),
		};
	});

	const prefetches: ClientLoaderPrefetch[] = [];
	for (let i = 0; i < match.matches.length; i++) {
		const m = match.matches[i]!;
		const pattern = m.registeredPattern.originalPattern;
		const loader = deps.loader_registry.get(pattern);
		if (!loader) {
			continue;
		}
		const known = known_matches[i]!;
		const server_deferred = make_deferred<ClientLoaderServerState>();
		const child = new AbortController();
		const propagate_abort = (): void => {
			child.abort();
			server_deferred.reject(new DOMException("Aborted", "AbortError"));
		};
		if (input.signal.aborted) {
			propagate_abort();
		} else {
			input.signal.addEventListener("abort", propagate_abort, {
				once: true,
			});
		}
		const result_promise = loader({
			trigger:
				input.trigger === "popstate" ? "navigation" : input.trigger,
			href: input.href,
			historyState: input.history_state,
			pattern,
			params: match.params,
			splatValues: [...match.splatValues],
			input: known.input,
			knownMatches: known_matches,
			serverPromise: server_deferred.promise,
			signal: child.signal,
		});
		result_promise.catch(() => {});
		prefetches.push({
			pattern,
			input: known.input,
			resolve_server_state: server_deferred.resolve,
			reject_server_state: server_deferred.reject,
			abort: propagate_abort,
			result_promise,
		});
	}
	return prefetches;
}

export function register_payload_patterns(
	registry: PatternRegistry,
	payload: RoutePayload,
): void {
	for (const route of payload.routes) {
		registerPattern(registry, route.pattern);
	}
}

/////////////////////////////////////////////////////////////////////
/////// Route Preparation
/////////////////////////////////////////////////////////////////////

export type PrepareRouteDeps = {
	platform: Platform;
	loader_registry: LoaderRegistry;
	module_cache: ModuleCache;
	current_base_href: () => string;
	register_hooks_for_match: (id: ClientLoaderID, hooks: MatchHooks) => void;
};

export type PrepareRouteResult =
	| { kind: "prepared"; prepared: PreparedRoute }
	| { kind: "failed"; error: string }
	| { kind: "aborted" };

export type PrepareRouteRequest = {
	payload: RoutePayload;
	href: string;
	history_state: unknown;
	signal: AbortSignal;
	trigger: "boot" | "navigation" | "prefetch" | "revalidation";
	token: RouteToken;
	client_build_id: string;
	speculative_prefetches: readonly ClientLoaderPrefetch[];
};

export async function prepare_route(
	deps: PrepareRouteDeps,
	request: PrepareRouteRequest,
): Promise<PrepareRouteResult> {
	deps.platform.preload_css(request.payload.css_bundles);
	deps.platform.preload_modules(request.payload.deps);

	let modules: Map<string, Record<string, unknown>>;
	try {
		modules = await import_modules(deps, request);
	} catch (err) {
		abort_all_prefetches(request.speculative_prefetches);
		if (request.signal.aborted) {
			return { kind: "aborted" };
		}
		return {
			kind: "failed",
			error: err instanceof Error ? err.message : String(err),
		};
	}
	if (request.signal.aborted) {
		abort_all_prefetches(request.speculative_prefetches);
		return { kind: "aborted" };
	}

	register_view_loaders(deps, request.payload, modules);

	const loader_results = await run_client_loaders(deps, request, modules);
	if (request.signal.aborted) {
		return { kind: "aborted" };
	}

	try {
		await deps.platform.wait_for_css(
			request.payload.css_bundles,
			request.signal,
		);
	} catch {
		if (request.signal.aborted) {
			return { kind: "aborted" };
		}
	}
	if (request.signal.aborted) {
		return { kind: "aborted" };
	}

	return {
		kind: "prepared",
		prepared: build_prepared(deps, request, modules, loader_results),
	};
}

function abort_all_prefetches(
	prefetches: readonly ClientLoaderPrefetch[],
): void {
	for (const pf of prefetches) {
		pf.abort();
	}
}

async function import_modules(
	deps: PrepareRouteDeps,
	request: PrepareRouteRequest,
): Promise<Map<string, Record<string, unknown>>> {
	const urls = request.payload.routes
		.map((r) => r.module_url)
		.filter((u): u is string => !!u);
	const unique = [...new Set(urls)];
	const base = deps.current_base_href();
	const pairs = await Promise.all(
		unique.map(async (url) => {
			const cache_key = module_cache_key(url, base);
			const cached = deps.module_cache.get(cache_key);
			if (cached) {
				return [url, cached] as const;
			}
			const mod = await deps.platform.import_module(url);
			deps.module_cache.set(cache_key, mod);
			return [url, mod] as const;
		}),
	);
	return new Map(pairs);
}

function register_view_loaders(
	deps: PrepareRouteDeps,
	payload: RoutePayload,
	modules: Map<string, Record<string, unknown>>,
): void {
	for (const route of payload.routes) {
		const mod = modules.get(route.module_url);
		if (!mod) {
			continue;
		}
		const view = mod.default as
			| { client_loader?: ClientLoaderFn }
			| undefined;
		if (view?.client_loader) {
			deps.loader_registry.set(route.pattern, view.client_loader);
		}
	}
}

type LoaderResult =
	| { kind: "data"; data: unknown }
	| { kind: "error"; error: unknown }
	| { kind: "skipped" };

async function run_client_loaders(
	deps: PrepareRouteDeps,
	request: PrepareRouteRequest,
	modules: Map<string, Record<string, unknown>>,
): Promise<LoaderResult[]> {
	const prefetches_by_pattern = new Map<string, ClientLoaderPrefetch>();
	for (const pf of request.speculative_prefetches) {
		prefetches_by_pattern.set(pf.pattern, pf);
	}
	const used_patterns = new Set<string>();

	const known_matches = request.payload.routes.map((r) => {
		return {
			pattern: r.pattern,
			input: r.input,
		};
	});
	const server_error_idx = request.payload.routes.findIndex(
		(r) => r.server_error !== undefined,
	);
	const trigger =
		request.trigger === "prefetch"
			? "prefetch"
			: request.trigger === "revalidation"
				? "revalidation"
				: request.trigger === "boot"
					? "boot"
					: "navigation";

	const child_controllers: AbortController[] = [];
	const promises: Promise<LoaderResult>[] = [];

	const build_server_state = (idx: number): ClientLoaderServerState => {
		const route = request.payload.routes[idx]!;
		return {
			clientBuildID: request.client_build_id,
			matches: request.payload.routes.map((r) => {
				return {
					pattern: r.pattern,
					input: r.input,
					loaderData: r.loader_data,
				};
			}),
			outermostServerError:
				server_error_idx === -1
					? null
					: {
							idx: server_error_idx,
							error: request.payload.routes[server_error_idx]!
								.server_error,
						},
			loaderData: route.loader_data,
		};
	};

	for (let idx = 0; idx < request.payload.routes.length; idx++) {
		const route = request.payload.routes[idx]!;

		if (server_error_idx !== -1 && idx >= server_error_idx) {
			const pf = prefetches_by_pattern.get(route.pattern);
			if (pf) {
				pf.abort();
				used_patterns.add(route.pattern);
			}
			promises.push(Promise.resolve({ kind: "skipped" }));
			child_controllers.push(new AbortController());
			continue;
		}

		const pf = prefetches_by_pattern.get(route.pattern);
		if (pf) {
			pf.resolve_server_state(build_server_state(idx));
			used_patterns.add(route.pattern);
			const dummy = new AbortController();
			child_controllers.push(dummy);
			if (request.signal.aborted) {
				pf.abort();
			} else {
				request.signal.addEventListener("abort", () => pf.abort(), {
					once: true,
				});
			}
			const captured_idx = idx;
			promises.push(
				pf.result_promise
					.then((data): LoaderResult => {
						return data === undefined
							? { kind: "skipped" }
							: { kind: "data", data };
					})
					.catch((err): LoaderResult => {
						if (is_abort_error(err)) {
							return { kind: "skipped" };
						}
						for (
							let later = captured_idx + 1;
							later < child_controllers.length;
							later++
						) {
							child_controllers[later]?.abort();
						}
						return {
							kind: "error",
							error:
								err instanceof Error
									? err.message
									: String(err),
						};
					}),
			);
			continue;
		}

		const loader = deps.loader_registry.get(route.pattern);
		if (!loader) {
			promises.push(Promise.resolve({ kind: "skipped" }));
			child_controllers.push(new AbortController());
			continue;
		}

		const child = new AbortController();
		child_controllers.push(child);
		if (request.signal.aborted) {
			child.abort();
		} else {
			request.signal.addEventListener("abort", () => child.abort(), {
				once: true,
			});
		}
		const captured_idx = idx;
		promises.push(
			(async (): Promise<LoaderResult> => {
				try {
					const data = await loader({
						trigger,
						href: request.href,
						historyState: request.history_state,
						pattern: route.pattern,
						params: request.payload.params,
						splatValues: [...request.payload.splat_values],
						input: route.input,
						knownMatches: known_matches,
						serverPromise: Promise.resolve(
							build_server_state(captured_idx),
						),
						signal: child.signal,
					});
					return data === undefined
						? { kind: "skipped" }
						: { kind: "data", data };
				} catch (err) {
					if (is_abort_error(err)) {
						return { kind: "skipped" };
					}
					for (
						let later = captured_idx + 1;
						later < child_controllers.length;
						later++
					) {
						child_controllers[later]?.abort();
					}
					return {
						kind: "error",
						error: err instanceof Error ? err.message : String(err),
					};
				}
			})(),
		);
	}

	for (const pf of request.speculative_prefetches) {
		if (!used_patterns.has(pf.pattern)) {
			pf.abort();
		}
	}

	return Promise.all(promises);
}

function build_prepared(
	deps: PrepareRouteDeps,
	request: PrepareRouteRequest,
	modules: Map<string, Record<string, unknown>>,
	loader_results: LoaderResult[],
): PreparedRoute {
	const server_error_idx = request.payload.routes.findIndex(
		(r) => r.server_error !== undefined,
	);
	const client_error_idx = loader_results.findIndex(
		(r) => r.kind === "error",
	);

	let error: RouteErrorState | null = null;
	if (server_error_idx !== -1) {
		error = {
			idx: server_error_idx,
			error: request.payload.routes[server_error_idx]!.server_error,
			source: "server",
		};
	} else if (client_error_idx !== -1) {
		const r = loader_results[client_error_idx]!;
		if (r.kind === "error") {
			error = {
				idx: client_error_idx,
				error: r.error,
				source: "clientLoader",
			};
		}
	}

	const matches: PreparedRouteMatch[] = request.payload.routes.map(
		(route, idx) => {
			const lr = loader_results[idx];
			const cl_data = lr && lr.kind === "data" ? lr.data : undefined;
			const client_loader_id = client_loader_id_for(request.token, idx);
			const mod = modules.get(route.module_url);
			const view = mod?.default as
				| {
						before_route_yield?: BeforeRouteHook;
						before_route_commit?: BeforeRouteHook;
				  }
				| undefined;
			deps.register_hooks_for_match(client_loader_id, {
				before_yield: view?.before_route_yield,
				before_commit: view?.before_route_commit,
			});
			return {
				pattern: route.pattern,
				input: route.input,
				module_url: route.module_url,
				loader_data: route.loader_data,
				client_loader_data: cl_data,
				client_loader_id,
			};
		},
	);

	return {
		matches,
		params: request.payload.params,
		splat_values: request.payload.splat_values,
		error,
		client_build_id: request.client_build_id,
		title: request.payload.title,
		meta_head_els: request.payload.meta_head_els,
		rest_head_els: request.payload.rest_head_els,
		css_bundles: request.payload.css_bundles,
		deps: request.payload.deps,
	};
}
