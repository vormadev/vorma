/// <reference types="vite/client" />

import { get_global } from "./global_state.ts";
import type {
	ComponentModulesMap,
	Nullable,
	RuntimeRouteSnapshot,
} from "./types.ts";

// skip empty/falsy import URLs — these represent server-only
// pass-through routes that have no client component module.
export async function load_modules(
	urls: string[],
): Promise<ComponentModulesMap> {
	const map: ComponentModulesMap = new Map();
	const unique = [...new Set(urls)].filter((url) => !!url);
	await Promise.all(
		unique.map(async (url) => {
			const mod = (await import(/* @vite-ignore */ url)) as Record<
				string,
				unknown
			>;
			map.set(url, mod);
		}),
	);
	return map;
}

export function build_active_components(
	snapshot: RuntimeRouteSnapshot,
	modules: ComponentModulesMap,
): unknown[] {
	const result: unknown[] = [];
	for (let i = 0; i < snapshot.import_urls.length; i++) {
		const mod = modules.get(snapshot.import_urls[i] as string);
		result.push(mod ? mod[snapshot.export_keys[i] as string] : undefined);
	}
	return result;
}

export function compute_effective_error_idx(
	server_idx: Nullable<number>,
	client_idx: Nullable<number>,
): Nullable<number> {
	if (server_idx != null && client_idx != null) {
		return Math.min(server_idx, client_idx);
	}
	return server_idx ?? client_idx;
}

export function resolve_error_boundary(
	snapshot: RuntimeRouteSnapshot,
	modules: ComponentModulesMap,
	error_idx: number,
): unknown {
	const error_key = snapshot.error_export_keys[error_idx] as string;
	if (!error_key) {
		return get_global().default_error_boundary;
	}
	const mod = modules.get(snapshot.import_urls[error_idx] as string);
	if (!mod) {
		return get_global().default_error_boundary;
	}
	return mod[error_key] ?? get_global().default_error_boundary;
}
