/// <reference types="vite/client" />

////////////////////////////////////////////////////////////////////////////////
// Adapter helpers — thin utilities for UI adapters (React, Preact, Solid).
////////////////////////////////////////////////////////////////////////////////

import { addRouteChangeListener } from "./events.ts";
import { get_global, get_snapshot } from "./global_state.ts";
import { apply_scroll_state } from "./scroll.ts";
import type {
	AdapterRouterData,
	AdapterStoreState,
	RuntimeRouteSnapshot,
	ScrollState,
} from "./types.ts";

// ─── Route Pattern Prop (safety check) ──────────────────────────

export const ROUTE_PATTERN_KEY = "__vorma_p";

export function verify_route_data_access(
	matched_patterns: string[],
	idx: number,
	expected_pattern: string | undefined,
): void {
	if (!expected_pattern) return;
	if (matched_patterns[idx] !== expected_pattern) {
		console.error(
			`Vorma: route data mismatch at index ${idx}. ` +
				`Expected "${expected_pattern}", got "${matched_patterns[idx]}".`,
		);
	}
}

// ─── Build Store State From Snapshot ─────────────────────────────

function build_router_data(s: RuntimeRouteSnapshot): AdapterRouterData {
	return {
		buildID: s.build_id,
		matchedPatterns: s.matched_patterns,
		splatValues: s.splat_values,
		params: s.params,
		rootData: s.has_root_data ? s.loaders_data[0] : null,
	};
}

// Snapshot-reference cache: if the snapshot object hasn't changed
// and no location event fired, return the previous store state.
// This avoids creating new derived objects (router_data, location)
// on every access, which matters for React's useSyncExternalStore.
let cached_snapshot: RuntimeRouteSnapshot | undefined;
let cached_state: AdapterStoreState | undefined;

function build_from_snapshot(): AdapterStoreState {
	const s = get_snapshot();
	if (s === cached_snapshot && cached_state) return cached_state;
	cached_state = {
		loaders_data: s.loaders_data,
		client_loaders_data: s.client_loaders_data,
		router_data: build_router_data(s),
		matched_patterns: s.matched_patterns,
		outermost_error: s.outermost_error,
		outermost_error_idx: s.outermost_error_idx,
		active_components: s.active_components,
		active_error_boundary: s.active_error_boundary,
		import_urls: s.import_urls,
		export_keys: s.export_keys,
		error_export_keys: s.error_export_keys,
	};
	cached_snapshot = s;
	return cached_state;
}

export function build_initial_store(): AdapterStoreState {
	return build_from_snapshot();
}

// ─── Scroll State (for adapter layout effects) ─────────────────

let _pending_scroll: ScrollState | undefined;
let _pending_scroll_counter = 0;

/**
 * Returns the latest pending scroll state and a monotonic id.
 * Adapters use the id to detect new scroll states without
 * re-applying stale ones.
 */
export function consume_pending_scroll(): {
	scroll: ScrollState | undefined;
	id: number;
} {
	return { scroll: _pending_scroll, id: _pending_scroll_counter };
}

/**
 * Applies scroll state on the next animation frame, giving the
 * UI framework time to commit new DOM nodes (e.g. hash targets).
 */
export function apply_pending_scroll(scroll: ScrollState | undefined): void {
	window.requestAnimationFrame(() => {
		apply_scroll_state(scroll);
	});
}

// ─── Subscribe ──────────────────────────────────────────────────

export function subscribe_to_store(props: {
	get_state: () => AdapterStoreState;
	set_state: (next: AdapterStoreState) => void;
}): () => void {
	function sync() {
		const next = build_from_snapshot();
		if (next !== props.get_state()) props.set_state(next);
	}
	const unsub_route = addRouteChangeListener((e) => {
		// Only capture scroll state when explicitly provided by
		// commit_navigation. Hash-only changes and HMR dispatches
		// send {} (no __scrollState key) and handle scroll directly.
		if ("__scrollState" in e.detail) {
			_pending_scroll = e.detail.__scrollState;
			_pending_scroll_counter++;
		}
		sync();
	});
	sync();
	return () => {
		unsub_route();
	};
}

// ─── Outlet Helpers ─────────────────────────────────────────────

export function get_route_key(s: AdapterStoreState, idx: number): string {
	return `${s.matched_patterns[idx] ?? ""}::${s.import_urls[idx] ?? ""}::${s.export_keys[idx] ?? ""}`;
}

// ─── Pattern Index Lookup ───────────────────────────────────────

export function find_pattern_index(matched: string[], pattern: string): number {
	const seg = get_global().app_config.loadersExplicitIndexSegmentIdentifier;
	const suffix = `/${seg}`;
	const candidates =
		pattern === "/" || pattern.endsWith(suffix)
			? [pattern]
			: [pattern, `${pattern}${suffix}`];
	return matched.findIndex((p) => candidates.includes(p));
}
