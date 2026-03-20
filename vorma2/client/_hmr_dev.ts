/// <reference types="vite/client" />

import { VORMA_SYMBOL } from "./global_state.ts";
import type {
	ClientLoaderWaitFn,
	Nullable,
	RuntimeRouteSnapshot,
} from "./types.ts";

const ROUTE_CHANGE_EVENT = "vorma:route-change";

type ViteUpdate = { type: string; path: string };
type HotRuntime = {
	on: (
		event: string,
		cb: (payload: { updates: ViteUpdate[] }) => void,
	) => void;
};

const tracked_by_path = new Map<string, Set<string>>();
const registered_hots = new WeakSet<HotRuntime>();

function normalize_path(path: string): string {
	return new URL(path, window.location.href).pathname;
}

function get_runtime_snapshot(): RuntimeRouteSnapshot | undefined {
	return (globalThis as any)[VORMA_SYMBOL]?.snapshot;
}

function get_wait_fn_map(): Record<string, ClientLoaderWaitFn> | undefined {
	return (globalThis as any)[VORMA_SYMBOL]?.pattern_to_wait_fn;
}

export function register_for_hmr_rerun(props: {
	import_meta: Pick<ImportMeta, "url">;
	pattern: string;
}): void {
	if (!import.meta.env.DEV) return;
	if (
		!props.pattern ||
		typeof props.import_meta.url !== "string" ||
		!props.import_meta.url
	)
		return;
	const key = normalize_path(props.import_meta.url);
	const set = tracked_by_path.get(key) ?? new Set<string>();
	set.add(props.pattern);
	tracked_by_path.set(key, set);
}

function resolve_patterns(updates: ViteUpdate[], current: string[]): string[] {
	const current_set = new Set(current);
	if (current_set.size === 0) return [];
	const result = new Set<string>();
	for (const u of updates) {
		if (u.type !== "js-update") continue;
		const tracked = tracked_by_path.get(normalize_path(u.path));
		if (!tracked) continue;
		for (const p of tracked) {
			if (current_set.has(p)) result.add(p);
		}
	}
	return Array.from(result);
}

async function refresh(patterns: string[]): Promise<boolean> {
	if (patterns.length === 0) return false;
	const snapshot = get_runtime_snapshot();
	const wait_fns = get_wait_fn_map();
	if (!snapshot || !wait_fns) return false;

	const refresh_set = new Set(patterns);
	const data = Array.from(snapshot.client_loaders_data);
	let outermost_error: unknown;
	let outermost_idx: Nullable<number>;

	await Promise.all(
		snapshot.matched_patterns.map(async (pattern, i) => {
			if (!refresh_set.has(pattern)) return;
			const fn = wait_fns[pattern];
			if (!fn) return;
			const server_data = Promise.resolve({
				matchedPatterns: snapshot.matched_patterns,
				rootData: snapshot.has_root_data
					? snapshot.loaders_data[0]
					: null,
				loaderData: snapshot.loaders_data[i],
				buildID: snapshot.build_id,
			});
			try {
				data[i] = await fn({
					params: snapshot.params,
					splatValues: snapshot.splat_values,
					serverDataPromise: server_data,
					signal: new AbortController().signal,
				});
			} catch (err) {
				if (outermost_idx == null) {
					outermost_error = err;
					outermost_idx = i;
				}
			}
		}),
	);

	// Merge error state
	const prev_idx = snapshot.outermost_client_error_idx;
	const should_drop_prev =
		prev_idx != null &&
		refresh_set.has(snapshot.matched_patterns[prev_idx] as string);
	const prev =
		!should_drop_prev && prev_idx != null
			? {
					error: snapshot.outermost_client_error,
					idx: prev_idx,
				}
			: undefined;
	const fresh =
		outermost_idx != null
			? { error: outermost_error, idx: outermost_idx }
			: undefined;

	let final_error: unknown;
	let final_idx: Nullable<number>;
	if (prev && fresh) {
		if (fresh.idx < prev.idx) {
			final_error = fresh.error;
			final_idx = fresh.idx;
		} else {
			final_error = prev.error;
			final_idx = prev.idx;
		}
	} else if (fresh) {
		final_error = fresh.error;
		final_idx = fresh.idx;
	} else if (prev) {
		final_error = prev.error;
		final_idx = prev.idx;
	}

	const g = (globalThis as any)[VORMA_SYMBOL];
	if (!g) return false;
	g.snapshot = {
		...snapshot,
		client_loaders_data: data,
		outermost_client_error: final_error,
		outermost_client_error_idx: final_idx,
		outermost_error: final_error ?? snapshot.outermost_server_error,
		outermost_error_idx: final_idx ?? snapshot.outermost_server_error_idx,
	};

	window.dispatchEvent(new CustomEvent(ROUTE_CHANGE_EVENT, { detail: {} }));
	return true;
}

export function apply_vite_update(updates: ViteUpdate[]): void {
	const snapshot = get_runtime_snapshot();
	const patterns = resolve_patterns(
		updates,
		snapshot?.matched_patterns ?? [],
	);
	if (patterns.length === 0) return;
	void refresh(patterns).catch(() => undefined);
}

export function register_vite_listener(hot: HotRuntime | undefined): void {
	if (!import.meta.env.DEV || !hot) return;
	if (registered_hots.has(hot)) return;
	hot.on("vite:afterUpdate", ({ updates }) => apply_vite_update(updates));
	registered_hots.add(hot);
}
