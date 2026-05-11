/// <reference types="vite/client" />

import { jsonDeepEquals } from "vorma/kit/json";
import { registerPattern, type PatternRegistry } from "vorma/kit/matcher";
import type {
	ClientCommit,
	RouteRenderState,
	ViewDefinition,
} from "../core/create_client_core.ts";
import type {
	Core4ClientLoader,
	Core4Model,
	RouteState,
	RouteUpdateReason,
} from "./types.ts";

export type Core4DevHMRHandler = (
	raw_url: string,
	module: Record<string, unknown>,
) => Promise<void>;

export type Core4DevHMRRuntime = {
	current_render_state: RouteRenderState | null;
	model: Core4Model;
	module_cache: Map<string, Record<string, unknown>>;
	pattern_registry: PatternRegistry;
	route_loaders: Map<string, Core4ClientLoader>;
	user_on_route_update:
		| ((
				route: RouteState,
				previous_route: RouteState | null,
				reason: RouteUpdateReason,
		  ) => void)
		| undefined;
	accept_hmr_route_state(route: RouteState): void;
	commit(commit: ClientCommit): void;
	normalize_core4_module_url(url: string): string;
};

const core4_hmr_rerun_patterns = new WeakMap<Core4DevHMRRuntime, Set<string>>();

export function install_core4_dev_hmr(runtime: Core4DevHMRRuntime): void {
	if (!import.meta.env.DEV || !import.meta.hot) {
		return;
	}
	const on_route_update: Core4DevHMRHandler = async (raw_url, module) => {
		await update_core4_hmr_route(runtime, raw_url, module);
	};
	const hmr_window = window as Window & {
		__vorma_hmr_route_update?: Core4DevHMRHandler;
	};
	hmr_window.__vorma_hmr_route_update = on_route_update;
	import.meta.hot.dispose(() => {
		if (hmr_window.__vorma_hmr_route_update === on_route_update) {
			hmr_window.__vorma_hmr_route_update = undefined;
		}
	});
}

export function configure_core4_dev_hmr_view(
	runtime: Core4DevHMRRuntime,
	pattern: string,
	rerun_client_loader: boolean,
): void {
	let patterns = core4_hmr_rerun_patterns.get(runtime);
	if (!patterns) {
		patterns = new Set();
		core4_hmr_rerun_patterns.set(runtime, patterns);
	}
	if (rerun_client_loader) {
		patterns.add(pattern);
	} else {
		patterns.delete(pattern);
	}
}

async function update_core4_hmr_route(
	runtime: Core4DevHMRRuntime,
	raw_url: string,
	module: Record<string, unknown>,
): Promise<void> {
	const render_state = runtime.current_render_state;
	const current = runtime.model.current;
	if (!render_state || !current) {
		return;
	}
	const module_url = runtime.normalize_core4_module_url(raw_url);
	runtime.module_cache.set(module_url, module);
	const idx = render_state.entries.findIndex((entry) => {
		return (
			runtime.normalize_core4_module_url(entry.module_url) === module_url
		);
	});
	if (idx === -1) {
		return;
	}
	const entry = render_state.entries[idx]!;
	const view = module.default as ViewDefinition | undefined;
	if (view?.client_loader) {
		runtime.route_loaders.set(
			entry.pattern,
			view.client_loader as Core4ClientLoader,
		);
		registerPattern(runtime.pattern_registry, entry.pattern);
	} else {
		runtime.route_loaders.delete(entry.pattern);
	}
	let client_loader_data = entry.client_loader_data;
	const loader = runtime.route_loaders.get(entry.pattern);
	const route_sequence = current.sequence;
	const should_rerun_loader =
		core4_hmr_rerun_patterns.get(runtime)?.has(entry.pattern) === true;
	if (loader && should_rerun_loader) {
		try {
			const entries = render_state.entries;
			const server_error =
				render_state.error?.source === "server"
					? {
							error: render_state.error.error,
							idx: render_state.error.idx,
						}
					: null;
			client_loader_data = await loader({
				historyState: render_state.history_state,
				href: current.route.href,
				input: entry.input,
				knownMatches: entries.map((known_entry) => {
					return {
						input: known_entry.input,
						pattern: known_entry.pattern,
					};
				}),
				loaderData: entry.loader_data,
				params: current.route.params,
				pattern: entry.pattern,
				serverPromise: Promise.resolve({
					clientBuildID: render_state.client_build_id,
					loaderData: entry.loader_data,
					matches: entries.map((known_entry) => {
						return {
							input: known_entry.input,
							loaderData: known_entry.loader_data,
							pattern: known_entry.pattern,
						};
					}),
					outermostServerError: server_error,
				}),
				signal: new AbortController().signal,
				splatValues: [...current.route.splatValues],
				trigger: "revalidation",
			});
		} catch (error) {
			console.error("Vorma: HMR client loader re-run failed", error);
		}
	}
	const live_render_state = runtime.current_render_state;
	const live_current = runtime.model.current;
	if (
		!live_render_state ||
		!live_current ||
		live_current.sequence !== route_sequence
	) {
		return;
	}
	const live_entry = live_render_state.entries[idx];
	if (
		!live_entry ||
		runtime.normalize_core4_module_url(live_entry.module_url) !==
			module_url ||
		!live_current.route.matches[idx]
	) {
		return;
	}
	const previous_route = live_current.route;
	const next_entries = live_render_state.entries.map(
		(current_entry, entry_idx) => {
			if (entry_idx !== idx) {
				return current_entry;
			}
			return {
				...current_entry,
				client_loader_data,
				module,
			};
		},
	);
	const next_render_state: RouteRenderState = {
		...live_render_state,
		entries: next_entries,
	};
	const next_route: RouteState = {
		...previous_route,
		matches: previous_route.matches.map((match, match_idx) => {
			if (match_idx !== idx) {
				return match;
			}
			return {
				...match,
				clientLoaderData: client_loader_data,
			};
		}),
	};
	runtime.current_render_state = next_render_state;
	runtime.accept_hmr_route_state(next_route);
	const client_commit: ClientCommit = {
		route_render: {
			state: next_render_state,
		},
	};
	if (!jsonDeepEquals(previous_route, next_route)) {
		client_commit.route_update = {
			previous_route,
			reason: "revalidation",
			route: next_route,
		};
	}
	runtime.commit(client_commit);
	if (client_commit.route_update) {
		runtime.user_on_route_update?.(
			client_commit.route_update.route,
			client_commit.route_update.previous_route,
			client_commit.route_update.reason,
		);
	}
}
