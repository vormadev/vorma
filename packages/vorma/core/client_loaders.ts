import { parseSearchParams } from "vorma/kit/json";
import { is_abort_error, new_abort_error, to_error_string } from "./abort_error.ts";
import type {
	ClientLoaderFn,
	ClientLoaderKnownMatch,
	ClientLoaderPrefetch,
	ClientLoaderPrefetchTrigger,
	ClientLoaderResult,
	ClientLoaderServerState,
	DecodedPayload,
	DecodedRoute,
	RouteClientLoaderTrigger,
} from "./client_core_types.ts";
import { create_client_matcher, type ClientMatcher } from "./client_wasm/matcher.ts";

/*
Owns everything client loaders need across navigations: the loader registry,
the search-schema registry, and the WASM matcher (with its pre-ready pattern
queue). Prefetch reconciliation — matching in-flight loader prefetches to
routes, seeding server state, and aborting whatever no route claimed — lives
here as one shared algorithm for the nav, prefetch, and HMR paths.
*/

export interface ClientLoaderOrchestratorDeps {
	// Active client build id stamped into loader server state.
	client_build_id(): string;
}

export interface PrestartInput {
	url: URL;
	// Computed when the matcher is ready, not at call time.
	history_state(): unknown;
	trigger: ClientLoaderPrefetchTrigger;
	signal: AbortSignal;
	is_open(): boolean;
	push(prefetch: ClientLoaderPrefetch): void;
}

export function create_client_loader_orchestrator(deps: ClientLoaderOrchestratorDeps) {
	const client_loader_map: Record<string, ClientLoaderFn> = {};
	const search_schema_map: Record<string, unknown> = {};
	let route_matcher: ClientMatcher | null = null;
	const queued_route_patterns = new Set<string>();
	const route_matcher_ready = create_client_matcher()
		.then((matcher) => {
			route_matcher = matcher;
			for (const pattern of queued_route_patterns) {
				matcher.register_pattern(pattern);
			}
		})
		.catch(() => {});

	function register_search_schema(pattern: string, schema: unknown): void {
		search_schema_map[pattern] = schema;
	}

	function register(pattern: string, client_loader: ClientLoaderFn): void {
		client_loader_map[pattern] = client_loader;
		if (queued_route_patterns.has(pattern)) {
			return;
		}
		queued_route_patterns.add(pattern);
		route_matcher?.register_pattern(pattern);
	}

	function unregister(pattern: string): void {
		delete client_loader_map[pattern];
	}

	function get(pattern: string): ClientLoaderFn | undefined {
		return client_loader_map[pattern];
	}

	// If the full path does not match, try progressively shorter path
	// prefixes: `/a/b/c`, then `/a/b`, then `/a`, then `/`. The matcher only
	// knows patterns for view modules that have already been loaded and that
	// have client loaders registered. So if the user navigates to
	// `/parent/child`, but the client currently only knows `/parent`, exact
	// matching would return null even though `/parent` is a real parent view
	// whose client loader can and must be started early.
	function find_view_match(path: string) {
		const matcher = route_matcher;
		if (!matcher) {
			return null;
		}
		let match = matcher.find_nested_matches(path);
		if (match) {
			return match;
		}
		const segments = path.split("/").filter(Boolean);
		for (let i = segments.length; i >= 0; i--) {
			const partial = i === 0 ? "/" : "/" + segments.slice(0, i).join("/");
			match = matcher.find_nested_matches(partial);
			if (match) {
				return match;
			}
		}
		return null;
	}

	function make_prefetch(input: {
		href: string;
		history_state: unknown;
		known_matches: ClientLoaderKnownMatch[];
		client_loader: ClientLoaderFn;
		params: Record<string, string>;
		pattern: string;
		route_input: unknown;
		signal: AbortSignal;
		splat_values: string[];
		trigger: ClientLoaderPrefetchTrigger;
	}): ClientLoaderPrefetch {
		let resolve_server_state!: (v: ClientLoaderServerState) => void;
		let reject_server_state!: (err: unknown) => void;
		const server_promise = new Promise<ClientLoaderServerState>((res, rej) => {
			resolve_server_state = res;
			reject_server_state = rej;
		});
		server_promise.catch(() => {});

		const ac = new AbortController();
		if (input.signal.aborted) {
			ac.abort();
			reject_server_state(new_abort_error());
		} else {
			input.signal.addEventListener(
				"abort",
				() => {
					ac.abort();
					reject_server_state(new_abort_error());
				},
				{ once: true },
			);
		}

		const result_promise = input.client_loader({
			trigger: input.trigger,
			href: input.href,
			historyState: input.history_state,
			pattern: input.pattern,
			params: input.params,
			splatValues: input.splat_values,
			input: input.route_input,
			knownMatches: input.known_matches,
			serverPromise: server_promise,
			signal: ac.signal,
		});
		result_promise.catch(() => {});

		return {
			pattern: input.pattern,
			resolve_server_state,
			abort: () => {
				ac.abort();
				reject_server_state(new_abort_error());
			},
			result_promise,
		};
	}

	function prestart(input: PrestartInput): void {
		const run = () => {
			if (!input.is_open() || input.signal.aborted) {
				return;
			}
			const match = find_view_match(input.url.pathname);
			if (!match || !input.is_open() || input.signal.aborted) {
				return;
			}
			const known_matches = match.patterns.map((pattern) => {
				return {
					pattern,
					input: parseSearchParams(
						search_schema_map[pattern],
						input.url.searchParams,
					),
				};
			});
			const input_by_pattern = new Map(
				known_matches.map((m) => {
					return [m.pattern, m.input] as const;
				}),
			);
			const history_state = input.history_state();
			for (const pattern of match.patterns) {
				const client_loader = client_loader_map[pattern];
				if (client_loader) {
					input.push(
						make_prefetch({
							href: input.url.href,
							history_state,
							known_matches,
							client_loader,
							params: match.params,
							pattern,
							route_input: input_by_pattern.get(pattern),
							signal: input.signal,
							splat_values: match.splat_values,
							trigger: input.trigger,
						}),
					);
				}
			}
		};
		if (route_matcher) {
			run();
		} else {
			void route_matcher_ready.then(run);
		}
	}

	function routes_to_known_matches(routes: DecodedRoute[]): ClientLoaderKnownMatch[] {
		return routes.map((r) => {
			return {
				pattern: r.pattern,
				input: r.input,
			};
		});
	}

	function build_server_state(
		routes: DecodedRoute[],
		idx: number,
	): ClientLoaderServerState {
		const err_idx = routes.findIndex((r) => {
			return r.server_error !== undefined;
		});
		return {
			clientBuildId: deps.client_build_id(),
			matches: routes.map((r) => {
				return {
					pattern: r.pattern,
					input: r.input,
					viewData: r.view_data,
				};
			}),
			outermostServerError:
				err_idx === -1
					? null
					: {
							idx: err_idx,
							error: routes[err_idx]!.server_error,
						},
			viewData: routes[idx]?.view_data,
		};
	}

	// Shared ownership algorithm for in-flight client-loader prefetches: match
	// prefetches to routes by pattern, abort everything at or after the
	// outermost server error, seed server state into retained prefetches, and
	// abort any prefetch no route claimed. Per-route behavior beyond that is
	// supplied by the caller.
	function reconcile(
		routes: DecodedRoute[],
		client_loader_prefetches: ClientLoaderPrefetch[],
		handle: (
			route: DecodedRoute,
			idx: number,
			retained: ClientLoaderPrefetch | undefined,
			suppressed: boolean,
		) => void,
	): void {
		const by_pattern = new Map<string, ClientLoaderPrefetch>();
		for (const p of client_loader_prefetches) {
			by_pattern.set(p.pattern, p);
		}
		const err_idx = routes.findIndex((r) => r.server_error !== undefined);
		const retained_prefetches = new Set<ClientLoaderPrefetch>();
		for (let i = 0; i < routes.length; i++) {
			const route = routes[i]!;
			const existing = by_pattern.get(route.pattern);
			if (err_idx !== -1 && i >= err_idx) {
				existing?.abort();
				handle(route, i, undefined, true);
				continue;
			}
			if (existing) {
				existing.resolve_server_state(build_server_state(routes, i));
				retained_prefetches.add(existing);
			}
			handle(route, i, existing, false);
		}
		for (const p of client_loader_prefetches) {
			if (!retained_prefetches.has(p)) {
				p.abort();
			}
		}
	}

	async function run(
		routes: DecodedRoute[],
		payload: DecodedPayload,
		client_loader_prefetches: ClientLoaderPrefetch[],
		trigger: RouteClientLoaderTrigger,
		href: string,
		history_state: unknown,
		signal: AbortSignal,
	): Promise<ClientLoaderResult[]> {
		const known_matches = routes_to_known_matches(routes);
		const abort_later: Array<(() => void) | null> = [];
		const promises: Array<Promise<unknown>> = [];

		reconcile(routes, client_loader_prefetches, (route, i, retained, suppressed) => {
			if (suppressed) {
				promises.push(Promise.resolve(undefined));
				abort_later.push(null);
				return;
			}
			if (retained) {
				abort_later.push(retained.abort);
				promises.push(retained.result_promise);
				return;
			}
			const client_loader = client_loader_map[route.pattern];
			if (!client_loader) {
				promises.push(Promise.resolve(undefined));
				abort_later.push(null);
				return;
			}
			const ac = new AbortController();
			abort_later.push(() => {
				ac.abort();
			});
			if (signal.aborted) {
				ac.abort();
			} else {
				signal.addEventListener("abort", () => ac.abort(), {
					once: true,
				});
			}
			promises.push(
				client_loader({
					trigger,
					href,
					historyState: history_state,
					pattern: route.pattern,
					params: payload.params,
					splatValues: payload.splat_values,
					input: route.input,
					knownMatches: known_matches,
					serverPromise: Promise.resolve(build_server_state(routes, i)),
					signal: ac.signal,
				}),
			);
		});

		const wrapped = promises.map(async (p, i) => {
			return p.catch((err) => {
				// On non-abort failure, cascade abort to later routes to avoid
				// running client loaders that can never be used.
				if (!is_abort_error(err)) {
					for (let j = i + 1; j < abort_later.length; j++) {
						abort_later[j]?.();
					}
				}
				throw err;
			});
		});

		const settled = await Promise.allSettled(wrapped);
		const out: ClientLoaderResult[] = [];
		for (const r of settled) {
			if (r.status === "fulfilled") {
				out.push(r.value !== undefined ? { data: r.value } : undefined);
			} else if (!is_abort_error(r.reason)) {
				out.push({ error: to_error_string(r.reason) });
				break;
			} else {
				out.push(undefined);
				break;
			}
		}
		return out;
	}

	return {
		register_search_schema,
		register,
		unregister,
		get,
		prestart,
		make_prefetch,
		routes_to_known_matches,
		build_server_state,
		reconcile,
		run,
	};
}

export type ClientLoaderOrchestrator = ReturnType<
	typeof create_client_loader_orchestrator
>;
