/// <reference types="vite/client" />

import { get_global } from "./global_state.ts";
import type { ComponentModulesMap, RuntimeRouteSnapshot } from "./types.ts";

export async function load_modules(
	urls: string[],
): Promise<ComponentModulesMap> {
	const map: ComponentModulesMap = new Map();
	const unique = [...new Set(urls)];
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

export function resolve_error_boundary(
	snapshot: RuntimeRouteSnapshot,
	modules: ComponentModulesMap,
): unknown {
	const idx = snapshot.outermost_server_error_idx;
	if (idx == null) return undefined;
	const error_key = snapshot.error_export_keys[idx] as string;
	if (!error_key) return get_global().default_error_boundary;
	const mod = modules.get(snapshot.import_urls[idx] as string);
	return mod ? mod[error_key] : undefined;
}
