/// <reference types="vite/client" />

import type {
	ClientLoaderFn,
	ClientLoaderResult,
	DecodedPayload,
	ViewDefinition,
} from "./client_core_types.ts";
import { preload_css } from "./css.ts";
import type { RouteMatchRecord, RouteRecord } from "./route_state_projection.ts";
import type { RouteErrorState } from "./types.ts";

/*
Owns view-module materialization: dynamic imports with the dev module cache,
HMR version stamping and the loader-rerun-on-HMR policy, and projection of a
decoded payload plus loaded modules into the immutable RouteRecord. Loader
registration discovered during import is reported through `register_loader`.
*/

export interface RouteModulesDeps {
	// Active client build id stamped into route records.
	client_build_id(): string;
	// A loaded view module declared a client loader for this pattern.
	register_loader(pattern: string, client_loader: ClientLoaderFn): void;
}

export function create_route_modules(deps: RouteModulesDeps) {
	const module_cache = new Map<string, Record<string, unknown>>();
	const hmr_version_map = new Map<string, number>();
	const hmr_rerun_patterns = new Set<string>();

	function normalize(url: string): string {
		return new URL(url, window.location.href).pathname;
	}

	async function prepare(
		payload: DecodedPayload,
		signal: AbortSignal,
	): Promise<Map<string, Record<string, unknown>> | null> {
		preload_css(payload.css_bundles);
		const urls = payload.routes.map((r) => {
			return r.module_url;
		});
		const unique = [...new Set(urls.filter(Boolean))];
		const pairs = await Promise.all(
			unique.map(async (url) => {
				if (import.meta.env.DEV) {
					const key = normalize(url);
					const cached = module_cache.get(key);
					if (cached) {
						return [url, cached] as const;
					}
				}
				const mod = (await import(/* @vite-ignore */ url)) as Record<
					string,
					unknown
				>;
				if (import.meta.env.DEV) {
					module_cache.set(normalize(url), mod);
				}
				return [url, mod] as const;
			}),
		);
		const modules = new Map(pairs);
		if (signal.aborted) {
			return null;
		}
		for (const route of payload.routes) {
			const mod = modules.get(route.module_url);
			if (!mod) {
				continue;
			}
			const def = mod.default as ViewDefinition | undefined;
			if (def?.client_loader) {
				deps.register_loader(route.pattern, def.client_loader);
			}
		}
		return modules;
	}

	function build_route_record(
		payload: DecodedPayload,
		modules: Map<string, Record<string, unknown>>,
		client_loader_results: ClientLoaderResult[],
	): RouteRecord {
		const matches: RouteMatchRecord[] = payload.routes.map((route, i) => {
			const client_loader_result = client_loader_results[i];
			return {
				pattern: route.pattern,
				input: route.input,
				module_url: route.module_url,
				hmr_version: hmr_version_map.get(normalize(route.module_url)) ?? 0,
				module: modules.get(route.module_url) ?? {},
				view_data: route.view_data,
				client_loader_data:
					client_loader_result && "data" in client_loader_result
						? client_loader_result.data
						: undefined,
			};
		});
		let error: RouteErrorState | null = null;
		const server_error_idx = payload.routes.findIndex((route) => {
			return route.server_error !== undefined;
		});
		if (server_error_idx !== -1) {
			error = {
				idx: server_error_idx,
				error: payload.routes[server_error_idx]!.server_error,
				source: "server",
			};
		} else {
			const client_loader_error_idx = client_loader_results.findIndex(
				(client_loader_result) => {
					return (
						client_loader_result !== undefined &&
						"error" in client_loader_result
					);
				},
			);
			if (client_loader_error_idx !== -1) {
				error = {
					idx: client_loader_error_idx,
					error: (
						client_loader_results[client_loader_error_idx] as {
							error: unknown;
						}
					).error,
					source: "clientLoader",
				};
			}
		}
		return {
			params: payload.params,
			splat_values: payload.splat_values,
			matches,
			error,
			client_build_id: deps.client_build_id(),
		};
	}

	function hmr_update(
		raw_url: string,
		mod: Record<string, unknown>,
	): { url: string; hmr_version: number } {
		const url = normalize(raw_url);
		module_cache.set(url, mod);
		const hmr_version = (hmr_version_map.get(url) ?? 0) + 1;
		hmr_version_map.set(url, hmr_version);
		return { url, hmr_version };
	}

	function set_hmr_rerun(pattern: string, rerun: boolean): void {
		if (!import.meta.env.DEV) {
			return;
		}
		if (rerun) {
			hmr_rerun_patterns.add(pattern);
		} else {
			hmr_rerun_patterns.delete(pattern);
		}
	}

	function should_rerun_on_hmr(pattern: string): boolean {
		return hmr_rerun_patterns.has(pattern);
	}

	return {
		normalize,
		prepare,
		build_route_record,
		hmr_update,
		set_hmr_rerun,
		should_rerun_on_hmr,
	};
}

export type RouteModules = ReturnType<typeof create_route_modules>;
