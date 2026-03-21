/// <reference types="vite/client" />

import { createPatternRegistry } from "vorma/kit/matcher/register";
import { register_client_loader } from "./client_loaders.ts";
import { VORMA_SYMBOL, ensure_global } from "./global_state.ts";
import { create_nav_manager } from "./navigation.ts";
import type { VormaClientGlobal } from "./types.ts";

const SCROLL_KEY = "__vorma__scrollStateMap";
const PAGE_REFRESH_KEY = "__vorma__pageRefreshScrollState";

// ─── Reset ───────────────────────────────────────────────────────

function clear_listeners(): void {
	const g = (globalThis as any)[VORMA_SYMBOL] as
		| VormaClientGlobal
		| undefined;
	if (!g) {
		return;
	}
	g.window_listeners?.forEach((set, name) => {
		set.forEach((fn) => window.removeEventListener(name, fn));
	});
	g.window_listeners?.clear();
}

export function reset_client_runtime_for_testing(
	opts: { clear_persisted_scroll_state?: boolean } = {},
): void {
	clear_listeners();
	delete (globalThis as any)[VORMA_SYMBOL];
	if (opts.clear_persisted_scroll_state !== false) {
		sessionStorage.removeItem(SCROLL_KEY);
		sessionStorage.removeItem(PAGE_REFRESH_KEY);
	}
}

export function create_isolated_client_test_runtime(
	opts: { clear_persisted_scroll_state?: boolean } = {},
): { reset: () => void } {
	reset_client_runtime_for_testing(opts);
	return { reset: () => reset_client_runtime_for_testing(opts) };
}

// ─── Seed Snapshot ───────────────────────────────────────────────

export type SeedSnapshotInput = {
	matched_patterns?: string[];
	loaders_data?: unknown[];
	import_urls?: string[];
	export_keys?: string[];
	error_export_keys?: string[];
	has_root_data?: boolean;
	params?: Record<string, string>;
	splat_values?: string[];
	outermost_server_error?: unknown;
	outermost_server_error_idx?: number | null;
	root_element_id?: string;
};

export function seed_runtime_route_snapshot_for_testing(
	input: SeedSnapshotInput,
): void {
	const g = ensure_global();
	if (!g.nav_state_manager) {
		g.nav_state_manager = create_nav_manager();
	}

	const s = g.snapshot;
	const count = input.matched_patterns?.length ?? s.matched_patterns.length;

	g.snapshot = {
		...s,
		matched_patterns: input.matched_patterns ?? s.matched_patterns,
		loaders_data:
			input.loaders_data ?? Array.from({ length: count }, () => null),
		import_urls:
			input.import_urls ?? Array.from({ length: count }, () => ""),
		export_keys:
			input.export_keys ?? Array.from({ length: count }, () => ""),
		error_export_keys:
			input.error_export_keys ?? Array.from({ length: count }, () => ""),
		has_root_data: input.has_root_data ?? false,
		params: input.params ?? {},
		splat_values: input.splat_values ?? [],
		outermost_server_error: input.outermost_server_error,
		outermost_server_error_idx: input.outermost_server_error_idx,
		root_element_id: input.root_element_id ?? s.root_element_id,
	};
}

// ─── Simple Setters / Readers ────────────────────────────────────

export function set_deployment_id_for_testing(id: string): void {
	ensure_global().deployment_id = id;
}

export function set_hard_redirect_handler_for_testing(
	handler: ((href: string) => void) | undefined,
): void {
	ensure_global().hard_redirect_for_testing = handler;
}

export function set_route_manifest_for_testing(
	manifest: Record<string, unknown> | undefined,
): void {
	ensure_global().route_manifest = manifest;
}

export function read_route_manifest_for_testing():
	| Record<string, unknown>
	| undefined {
	return ensure_global().route_manifest;
}

export function replace_pattern_registry_for_testing(): void {
	const g = ensure_global();
	const c = g.app_config;
	g.pattern_registry = createPatternRegistry({
		dynamicParamPrefixRune: c.loadersDynamicRune,
		splatSegmentRune: c.loadersSplatRune,
		explicitIndexSegment: c.loadersExplicitIndexSegmentIdentifier,
	});
}

export function read_is_touch_input_modality_active_for_testing(): boolean {
	return ensure_global().is_touch_active;
}

export function register_client_loader_for_testing(props: {
	pattern: string;
	client_loader: (input: unknown) => Promise<unknown>;
	re_run_on_module_change?: ImportMeta;
}): void {
	ensure_global();
	register_client_loader({
		pattern: props.pattern,
		clientLoader: props.client_loader,
		reRunOnModuleChange: props.re_run_on_module_change,
	} as any);
}

export function clear_all_navigation_state_for_testing(): void {
	const g = ensure_global();
	if (!g.nav_state_manager) {
		g.nav_state_manager = create_nav_manager();
	}
	g.nav_state_manager.clearAll();
}

// ─── Router Data ─────────────────────────────────────────────────

export type TestingRouterData = {
	client_build_id: string;
	matched_patterns: string[];
	splat_values: string[];
	params: Record<string, string>;
	root_data: unknown;
};

export function read_router_data_for_testing(): TestingRouterData {
	const s = ensure_global().snapshot;
	return {
		client_build_id: s.client_build_id,
		matched_patterns: s.matched_patterns,
		splat_values: s.splat_values,
		params: s.params,
		root_data: s.has_root_data ? s.loaders_data[0] : null,
	};
}

// ─── Scroll State ────────────────────────────────────────────────

export type TestingScrollEntry = {
	history_key: string;
	x: number;
	y: number;
};

export type TestingPageRefreshState = {
	x: number;
	y: number;
	unix: number;
	href: string;
};

export function seed_scroll_state_for_testing(
	entries: TestingScrollEntry[],
): void {
	sessionStorage.setItem(
		SCROLL_KEY,
		JSON.stringify(entries.map((e) => [e.history_key, { x: e.x, y: e.y }])),
	);
}

export function write_raw_scroll_state_storage_for_testing(
	raw: string | null,
): void {
	if (raw === null) {
		sessionStorage.removeItem(SCROLL_KEY);
		return;
	}
	sessionStorage.setItem(SCROLL_KEY, raw);
}

export function read_scroll_state_for_testing(): TestingScrollEntry[] {
	const raw = sessionStorage.getItem(SCROLL_KEY);
	if (!raw) {
		return [];
	}
	try {
		const parsed = JSON.parse(raw);
		if (!Array.isArray(parsed)) {
			return [];
		}
		return parsed
			.filter(
				(e: any) =>
					Array.isArray(e) &&
					e.length === 2 &&
					typeof e[0] === "string" &&
					e[1] &&
					typeof e[1].x === "number" &&
					typeof e[1].y === "number",
			)
			.map((e: any) => ({
				history_key: e[0],
				x: e[1].x,
				y: e[1].y,
			}));
	} catch {
		return [];
	}
}

export function seed_page_refresh_scroll_state_for_testing(
	state: TestingPageRefreshState | null,
): void {
	if (!state) {
		sessionStorage.removeItem(PAGE_REFRESH_KEY);
		return;
	}
	sessionStorage.setItem(PAGE_REFRESH_KEY, JSON.stringify(state));
}

export function read_page_refresh_scroll_state_for_testing(): TestingPageRefreshState | null {
	const raw = sessionStorage.getItem(PAGE_REFRESH_KEY);
	if (!raw) {
		return null;
	}
	try {
		const p = JSON.parse(raw);
		if (
			typeof p?.x !== "number" ||
			typeof p?.y !== "number" ||
			typeof p?.unix !== "number" ||
			typeof p?.href !== "string"
		) {
			return null;
		}
		return { x: p.x, y: p.y, unix: p.unix, href: p.href };
	} catch {
		return null;
	}
}

export function is_scroll_state_storage_key_for_testing(key: string): boolean {
	return key === SCROLL_KEY;
}

export function is_page_refresh_scroll_state_storage_key_for_testing(
	key: string,
): boolean {
	return key === PAGE_REFRESH_KEY;
}

// ─── HMR ─────────────────────────────────────────────────────────

export async function simulate_vite_after_update_for_testing(
	updates: Array<{ type: string; path: string }>,
): Promise<void> {
	ensure_global();
	const { apply_vite_update } = await import("./_hmr_dev.ts");
	apply_vite_update(updates);
}
