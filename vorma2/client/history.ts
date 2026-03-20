/// <reference types="vite/client" />

////////////////////////////////////////////////////////////////////////////////
// Native History API wrapper — replaces npm `history` dependency.
// Stores a unique key per entry via history.state for scroll restoration.
////////////////////////////////////////////////////////////////////////////////

import { dispatch_route_change } from "./events.ts";
import { ensure_global } from "./global_state.ts";
import {
	apply_scroll_state,
	get_scroll_for_current_key_or_top,
	normalize_hash,
	save_current_scroll,
	save_page_refresh_scroll,
	save_scroll_for_key,
} from "./scroll.ts";
import type { ScrollState } from "./types.ts";
import { classify_target } from "./url.ts";

const KEY_FIELD = "_vk";

function make_key(): string {
	return Math.random().toString(36).slice(2, 10);
}

function read_state(): { key: string } {
	const s = window.history.state;
	if (s && typeof s === "object" && KEY_FIELD in s) {
		return { key: s[KEY_FIELD] as string };
	}
	return { key: "initial" };
}

export function get_history_key(): string {
	return read_state().key;
}

// ─── Push / Replace ──────────────────────────────────────────────

export function history_push(url: string): void {
	const key = make_key();
	window.history.pushState({ [KEY_FIELD]: key }, "", url);
	const g = ensure_global();
	g.last_known_history_key = key;
	g.last_known_history_href = window.location.href;
}

export function history_replace(url: string): void {
	const key = make_key();
	window.history.replaceState({ [KEY_FIELD]: key }, "", url);
	const g = ensure_global();
	g.last_known_history_key = key;
	g.last_known_history_href = window.location.href;
}

// ─── Commit (saves scroll, then push/replace, then dispatches) ──

export function commit_history(props: {
	target_url: string;
	replace?: boolean;
}): void {
	save_current_scroll();
	if (props.replace) {
		history_replace(props.target_url);
	} else {
		history_push(props.target_url);
	}
}

// ─── Popstate Listener ──────────────────────────────────────────

export function init_popstate_listener(): void {
	const g = ensure_global();
	if (g.popstate_registered) return;
	g.popstate_registered = true;
	g.last_known_history_key = get_history_key();
	g.last_known_history_href = window.location.href;
	window.addEventListener("popstate", () => void handle_popstate());
}

async function handle_popstate(): Promise<void> {
	const g = ensure_global();
	const prev_key = g.last_known_history_key;
	const prev_href = g.last_known_history_href ?? window.location.href;
	const next_key = get_history_key();
	const next_href = window.location.href;

	// Dedup: if the key hasn't changed, nothing to do.
	if (next_key === prev_key) return;

	// Save scroll for the page we're leaving.
	if (prev_key) {
		save_scroll_for_key(prev_key, {
			x: window.scrollX,
			y: window.scrollY,
		});
	}

	g.last_known_history_key = next_key;
	g.last_known_history_href = next_href;

	// Hash-only change within the same document — apply scroll
	// directly without entering the navigation system.
	const classification = classify_target(next_href, prev_href);
	if (classification === "same-document-hash-change") {
		const next_hash = normalize_hash(
			new URL(next_href, window.location.href).hash,
		);
		const scroll: ScrollState =
			next_hash.length > 0
				? { hash: new URL(next_href, window.location.href).hash }
				: get_scroll_for_current_key_or_top();
		// Hash target already exists in the DOM — apply directly.
		apply_scroll_state(scroll);
		// Dispatch without __scrollState so adapters don't re-apply.
		dispatch_route_change({});
		return;
	}

	const nav = g.nav_state_manager;
	if (!nav) return;

	try {
		await nav.navigate({
			href: next_href,
			intent: "navigate",
			replace: true,
			skip_history_commit: true,
			current_href_for_classification: prev_href,
		});
	} catch (err) {
		console.error("Vorma:", "POP navigation failed; hard reloading.", err);
		perform_hard_redirect(next_href);
	}
}

// ─── Hard Redirect ──────────────────────────────────────────────

export function perform_hard_redirect(href: string): void {
	const g = ensure_global();
	if (
		import.meta.env.MODE === "test" &&
		typeof g.hard_redirect_for_testing === "function"
	) {
		g.hard_redirect_for_testing(href);
		return;
	}
	window.location.assign(href);
}

// ─── Scroll Restoration Setup ───────────────────────────────────

export function set_manual_scroll_restoration(): void {
	try {
		window.history.scrollRestoration = "manual";
	} catch {
		// Not supported in this environment.
	}
}

export function register_beforeunload_once(): void {
	const g = ensure_global();
	if (g.has_registered_beforeunload) return;
	window.addEventListener("beforeunload", () => save_page_refresh_scroll());
	g.has_registered_beforeunload = true;
}
