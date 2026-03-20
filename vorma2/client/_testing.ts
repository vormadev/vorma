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
	if (!g) return;
	g.window_listeners?.forEach((set, name) => {
		set.forEach((fn) => window.removeEventListener(name, fn));
	});
	g.window_listeners?.clear();
}

export function resetClientRuntimeForTesting(
	opts: { clearPersistedScrollState?: boolean } = {},
): void {
	clear_listeners();
	delete (globalThis as any)[VORMA_SYMBOL];
	if (opts.clearPersistedScrollState !== false) {
		sessionStorage.removeItem(SCROLL_KEY);
		sessionStorage.removeItem(PAGE_REFRESH_KEY);
	}
}

export function createIsolatedClientTestRuntime(
	opts: { clearPersistedScrollState?: boolean } = {},
): { reset: () => void } {
	resetClientRuntimeForTesting(opts);
	return { reset: () => resetClientRuntimeForTesting(opts) };
}

// ─── Seed Snapshot ───────────────────────────────────────────────

export type SeedSnapshotInput = {
	matchedPatterns?: string[];
	loadersData?: unknown[];
	importURLs?: string[];
	exportKeys?: string[];
	errorExportKeys?: string[];
	hasRootData?: boolean;
	params?: Record<string, string>;
	splatValues?: string[];
	outermostServerError?: unknown;
	outermostServerErrorIdx?: number | null;
	rootElementID?: string;
};

export function seedRuntimeRouteSnapshotForTesting(
	input: SeedSnapshotInput,
): void {
	const g = ensure_global();
	if (!g.nav_state_manager) g.nav_state_manager = create_nav_manager();

	const s = g.snapshot;
	const count = input.matchedPatterns?.length ?? s.matched_patterns.length;

	g.snapshot = {
		...s,
		matched_patterns: input.matchedPatterns ?? s.matched_patterns,
		loaders_data:
			input.loadersData ?? Array.from({ length: count }, () => null),
		import_urls:
			input.importURLs ?? Array.from({ length: count }, () => ""),
		export_keys:
			input.exportKeys ?? Array.from({ length: count }, () => ""),
		error_export_keys:
			input.errorExportKeys ?? Array.from({ length: count }, () => ""),
		has_root_data: input.hasRootData ?? false,
		params: input.params ?? {},
		splat_values: input.splatValues ?? [],
		outermost_server_error: input.outermostServerError,
		outermost_server_error_idx: input.outermostServerErrorIdx,
		root_element_id: input.rootElementID ?? s.root_element_id,
	};
}

// ─── Simple Setters / Readers ────────────────────────────────────

export function setDeploymentIDForTesting(id: string): void {
	ensure_global().deployment_id = id;
}

export function setHardRedirectHandlerForTesting(
	handler: ((href: string) => void) | undefined,
): void {
	ensure_global().hard_redirect_for_testing = handler;
}

export function setRouteManifestForTesting(
	manifest: Record<string, unknown> | undefined,
): void {
	ensure_global().route_manifest = manifest;
}

export function readRouteManifestForTesting():
	| Record<string, unknown>
	| undefined {
	return ensure_global().route_manifest;
}

export function replacePatternRegistryForTesting(): void {
	const g = ensure_global();
	const c = g.app_config;
	g.pattern_registry = createPatternRegistry({
		dynamicParamPrefixRune: c.loadersDynamicRune,
		splatSegmentRune: c.loadersSplatRune,
		explicitIndexSegment: c.loadersExplicitIndexSegmentIdentifier,
	});
}

export function readIsTouchInputModalityActiveForTesting(): boolean {
	return ensure_global().is_touch_active;
}

export function registerClientLoaderForTesting(props: {
	pattern: string;
	clientLoader: (input: unknown) => Promise<unknown>;
	reRunOnModuleChange?: ImportMeta;
}): void {
	ensure_global();
	register_client_loader(props as any);
}

export function clearAllNavigationStateForTesting(): void {
	const g = ensure_global();
	if (!g.nav_state_manager) g.nav_state_manager = create_nav_manager();
	g.nav_state_manager.clearAll();
}

// ─── Router Data ─────────────────────────────────────────────────

export type TestingRouterData = {
	buildID: string;
	matchedPatterns: string[];
	splatValues: string[];
	params: Record<string, string>;
	rootData: unknown;
};

export function readRouterDataForTesting(): TestingRouterData {
	const s = ensure_global().snapshot;
	return {
		buildID: s.build_id,
		matchedPatterns: s.matched_patterns,
		splatValues: s.splat_values,
		params: s.params,
		rootData: s.has_root_data ? s.loaders_data[0] : null,
	};
}

// ─── Scroll State ────────────────────────────────────────────────

export type TestingScrollEntry = {
	historyKey: string;
	x: number;
	y: number;
};
export type TestingPageRefreshState = {
	x: number;
	y: number;
	unix: number;
	href: string;
};

export function seedScrollStateForTesting(entries: TestingScrollEntry[]): void {
	sessionStorage.setItem(
		SCROLL_KEY,
		JSON.stringify(entries.map((e) => [e.historyKey, { x: e.x, y: e.y }])),
	);
}

export function writeRawScrollStateStorageForTesting(raw: string | null): void {
	if (raw === null) {
		sessionStorage.removeItem(SCROLL_KEY);
		return;
	}
	sessionStorage.setItem(SCROLL_KEY, raw);
}

export function readScrollStateForTesting(): TestingScrollEntry[] {
	const raw = sessionStorage.getItem(SCROLL_KEY);
	if (!raw) return [];
	try {
		const parsed = JSON.parse(raw);
		if (!Array.isArray(parsed)) return [];
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
				historyKey: e[0],
				x: e[1].x,
				y: e[1].y,
			}));
	} catch {
		return [];
	}
}

export function seedPageRefreshScrollStateForTesting(
	state: TestingPageRefreshState | null,
): void {
	if (!state) {
		sessionStorage.removeItem(PAGE_REFRESH_KEY);
		return;
	}
	sessionStorage.setItem(PAGE_REFRESH_KEY, JSON.stringify(state));
}

export function readPageRefreshScrollStateForTesting(): TestingPageRefreshState | null {
	const raw = sessionStorage.getItem(PAGE_REFRESH_KEY);
	if (!raw) return null;
	try {
		const p = JSON.parse(raw);
		if (
			typeof p?.x !== "number" ||
			typeof p?.y !== "number" ||
			typeof p?.unix !== "number" ||
			typeof p?.href !== "string"
		)
			return null;
		return { x: p.x, y: p.y, unix: p.unix, href: p.href };
	} catch {
		return null;
	}
}

export function isScrollStateStorageKeyForTesting(key: string): boolean {
	return key === SCROLL_KEY;
}

export function isPageRefreshScrollStateStorageKeyForTesting(
	key: string,
): boolean {
	return key === PAGE_REFRESH_KEY;
}

// ─── HMR ─────────────────────────────────────────────────────────

export async function simulateViteAfterUpdateForTesting(
	updates: Array<{ type: string; path: string }>,
): Promise<void> {
	ensure_global();
	const { apply_vite_update } = await import("./_hmr_dev.ts");
	apply_vite_update(updates);
}
