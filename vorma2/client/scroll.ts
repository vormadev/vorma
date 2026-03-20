/// <reference types="vite/client" />

import type { ScrollState } from "./types.ts";

const STORAGE_KEY = "__vorma__scrollStateMap";
const PAGE_REFRESH_KEY = "__vorma__pageRefreshScrollState";
const MAX_ENTRIES = 50;
const PAGE_REFRESH_MAX_AGE_MS = 5000;

type StoredScrollEntry = [string, { x: number; y: number }];

// ─── Hash Utilities ──────────────────────────────────────────────

export function normalize_hash(hash: string): string {
	const s = hash.startsWith("#") ? hash.slice(1) : hash;
	if (s.length === 0) return s;
	try {
		return decodeURIComponent(s);
	} catch {
		return s;
	}
}

// ─── Apply Scroll State ─────────────────────────────────────────

export function apply_scroll_state(state?: ScrollState): void {
	if (!state) {
		const id = normalize_hash(window.location.hash);
		if (id.length > 0) document.getElementById(id)?.scrollIntoView();
		return;
	}
	if ("hash" in state) {
		const id = normalize_hash(state.hash);
		document.getElementById(id)?.scrollIntoView();
	} else {
		window.scrollTo(state.x, state.y);
	}
}

// ─── Stored Scroll State (sessionStorage) ────────────────────────

function read_entries(): StoredScrollEntry[] {
	try {
		const raw = sessionStorage.getItem(STORAGE_KEY);
		if (!raw) return [];
		const parsed = JSON.parse(raw);
		if (!Array.isArray(parsed)) return [];
		return parsed.filter(
			(e: unknown): e is StoredScrollEntry =>
				Array.isArray(e) &&
				e.length === 2 &&
				typeof e[0] === "string" &&
				!!e[1] &&
				typeof e[1] === "object" &&
				typeof (e[1] as any).x === "number" &&
				typeof (e[1] as any).y === "number",
		);
	} catch {
		return [];
	}
}

function write_entries(entries: StoredScrollEntry[]): void {
	try {
		sessionStorage.setItem(STORAGE_KEY, JSON.stringify(entries));
	} catch {}
}

export function save_scroll_for_key(
	key: string,
	state: { x: number; y: number },
): void {
	if (!key) return;
	const entries = read_entries().filter((e) => e[0] !== key);
	entries.push([key, { x: state.x, y: state.y }]);
	if (entries.length > MAX_ENTRIES)
		entries.splice(0, entries.length - MAX_ENTRIES);
	write_entries(entries);
}

export function get_scroll_for_key(
	key: string,
): { x: number; y: number } | undefined {
	for (const [k, s] of read_entries()) {
		if (k === key) return s;
	}
	return undefined;
}

// Read the history key from window.history.state using our _vk field.
export function get_current_history_key(): string | null {
	const s = window.history.state as Record<string, any> | null;
	return typeof s?._vk === "string" ? s._vk : null;
}

export function save_current_scroll(): void {
	const key = get_current_history_key();
	if (key) save_scroll_for_key(key, { x: window.scrollX, y: window.scrollY });
}

export function get_scroll_for_current_key_or_top(): {
	x: number;
	y: number;
} {
	const key = get_current_history_key();
	if (!key) return { x: 0, y: 0 };
	return get_scroll_for_key(key) ?? { x: 0, y: 0 };
}

// ─── Page Refresh Scroll State ───────────────────────────────────

export function save_page_refresh_scroll(): void {
	try {
		sessionStorage.setItem(
			PAGE_REFRESH_KEY,
			JSON.stringify({
				x: window.scrollX,
				y: window.scrollY,
				unix: Date.now(),
				href: window.location.href,
			}),
		);
	} catch {}
}

export function restore_page_refresh_scroll(
	is_same_page: (href: string) => boolean,
): void {
	try {
		const raw = sessionStorage.getItem(PAGE_REFRESH_KEY);
		if (!raw) return;
		sessionStorage.removeItem(PAGE_REFRESH_KEY);
		const s = JSON.parse(raw);
		if (
			typeof s?.x !== "number" ||
			typeof s?.y !== "number" ||
			typeof s?.unix !== "number" ||
			typeof s?.href !== "string"
		)
			return;
		if (!is_same_page(s.href)) return;
		if (Date.now() - s.unix > PAGE_REFRESH_MAX_AGE_MS) return;
		window.requestAnimationFrame(() => window.scrollTo(s.x, s.y));
	} catch {}
}
