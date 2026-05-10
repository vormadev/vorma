import {
	CSS_BUNDLE_ATTR,
	CSS_PRELOAD_ATTR,
	CSS_PRELOAD_SETTLED_ATTR,
	HISTORY_KEY_FIELD,
	HISTORY_USER_STATE_FIELD,
	SCROLL_STORAGE_KEY,
	SCROLL_STORAGE_RELOAD_KEY,
	VORMA_ROOT_EL_ID,
} from "../core/constants.ts";
import type { HeadEl, ScrollState } from "./client_contract.ts";
import type { HistoryWrite, TimerHandle } from "./host.ts";
import type { BrowserPosition, RouteDomPatch } from "./model.ts";

const max_scroll_entries = 50;
const reload_scroll_max_age_ms = 3333;

const browser_dom_attr = {
	head_section: "data-vorma-core2-head-section",
	modulepreload_rel: "modulepreload",
	preload_as_attr: "as",
	preload_rel: "preload",
	style_rel: "stylesheet",
	style_type: "style",
} as const;

const css_preload_settled = {
	no: "0",
	yes: "1",
} as const;

const dom_event = {
	error: "error",
	load: "load",
} as const;

const head_section = {
	meta: "meta",
	rest: "rest",
} as const;

const reload_scroll_field = {
	href: "href",
	unix: "unix",
	x: "x",
	y: "y",
} as const;

type StoredScrollEntry = [string, { x: number; y: number }];

export function decode_html_text(html: string): string {
	const el = document.createElement("textarea");
	el.innerHTML = html;
	return el.value;
}

export function read_browser_position(fallback_key?: string): BrowserPosition {
	const state = window.history.state;
	const state_record =
		state && typeof state === "object"
			? (state as Record<string, unknown>)
			: {};
	let key =
		typeof state_record[HISTORY_KEY_FIELD] === "string"
			? state_record[HISTORY_KEY_FIELD]
			: "";
	if (!key && fallback_key) {
		key = fallback_key;
		window.history.replaceState(
			{
				...state_record,
				[HISTORY_KEY_FIELD]: key,
			},
			"",
			window.location.href,
		);
	}
	return {
		href: window.location.href,
		key,
		state: state_record[HISTORY_USER_STATE_FIELD],
	};
}

export function write_browser_history(write: HistoryWrite): void {
	const next_state = {
		...(window.history.state &&
		typeof window.history.state === "object" &&
		write.kind === "replace"
			? (window.history.state as Record<string, unknown>)
			: {}),
		[HISTORY_KEY_FIELD]: write.key,
		[HISTORY_USER_STATE_FIELD]: write.state,
	};
	if (write.kind === "replace") {
		window.history.replaceState(next_state, "", write.href);
	} else {
		window.history.pushState(next_state, "", write.href);
	}
}

export function get_or_create_root_el(): HTMLElement {
	const existing = document.getElementById(VORMA_ROOT_EL_ID);
	if (existing) {
		return existing;
	}
	const fresh = document.createElement("div");
	fresh.id = VORMA_ROOT_EL_ID;
	document.body.insertBefore(fresh, document.body.firstChild);
	return fresh;
}

export function set_browser_timer(
	delay_ms: number,
	callback: () => void,
): TimerHandle {
	return window.setTimeout(callback, delay_ms);
}

export function clear_browser_timer(timer: TimerHandle): void {
	window.clearTimeout(timer as ReturnType<typeof window.setTimeout>);
}

export function apply_route_dom_patch(patch: RouteDomPatch): void {
	apply_head_patch(patch.title, patch.meta_head_els, patch.rest_head_els);
	apply_css_bundles(patch.css_bundles);
	preload_modules(patch.deps);
}

export async function prepare_css_bundles(
	bundles: string[],
	signal: AbortSignal,
): Promise<void> {
	preload_css_bundles(bundles);
	await wait_for_css_bundles(bundles, signal);
}

export function save_current_scroll(): void {
	const position = read_browser_position();
	if (!position.key) {
		return;
	}
	save_scroll_position(position.key, read_current_scroll());
}

export function save_reload_scroll(now_ms: number): void {
	try {
		sessionStorage.setItem(
			SCROLL_STORAGE_RELOAD_KEY,
			JSON.stringify({
				[reload_scroll_field.x]: window.scrollX,
				[reload_scroll_field.y]: window.scrollY,
				[reload_scroll_field.unix]: now_ms,
				[reload_scroll_field.href]: window.location.href,
			}),
		);
	} catch {}
}

export function read_current_scroll(): ScrollState {
	return { x: window.scrollX, y: window.scrollY };
}

export function save_scroll_position(key: string, scroll: ScrollState): void {
	if (!key || "hash" in scroll) {
		return;
	}
	const entries = read_scroll_entries().filter((entry) => {
		return entry[0] !== key;
	});
	entries.push([key, scroll]);
	if (entries.length > max_scroll_entries) {
		entries.splice(0, entries.length - max_scroll_entries);
	}
	try {
		sessionStorage.setItem(SCROLL_STORAGE_KEY, JSON.stringify(entries));
	} catch {}
}

export function read_saved_scroll(key: string): ScrollState | undefined {
	if (!key) {
		return undefined;
	}
	for (const [entry_key, scroll] of read_scroll_entries()) {
		if (entry_key === key) {
			return scroll;
		}
	}
	return undefined;
}

export function read_reload_scroll(now_ms: number): ScrollState | undefined {
	let raw: string | null;
	try {
		raw = sessionStorage.getItem(SCROLL_STORAGE_RELOAD_KEY);
		sessionStorage.removeItem(SCROLL_STORAGE_RELOAD_KEY);
	} catch {
		raw = null;
	}
	if (!raw) {
		return undefined;
	}
	try {
		const parsed = JSON.parse(raw);
		if (!parsed || typeof parsed !== "object") {
			return undefined;
		}
		const record = parsed as Record<string, unknown>;
		const x = record[reload_scroll_field.x];
		const y = record[reload_scroll_field.y];
		const unix = record[reload_scroll_field.unix];
		const href = record[reload_scroll_field.href];
		if (
			typeof x !== "number" ||
			typeof y !== "number" ||
			typeof unix !== "number" ||
			typeof href !== "string" ||
			now_ms - unix > reload_scroll_max_age_ms ||
			!same_href_without_hash(href, window.location.href)
		) {
			return undefined;
		}
		return { x, y };
	} catch {
		return undefined;
	}
}

function read_scroll_entries(): StoredScrollEntry[] {
	try {
		const raw = sessionStorage.getItem(SCROLL_STORAGE_KEY);
		if (!raw) {
			return [];
		}
		const parsed = JSON.parse(raw);
		if (!Array.isArray(parsed)) {
			return [];
		}
		return parsed.filter((entry): entry is StoredScrollEntry => {
			return (
				Array.isArray(entry) &&
				entry.length === 2 &&
				typeof entry[0] === "string" &&
				!!entry[1] &&
				typeof entry[1] === "object" &&
				Number.isFinite((entry[1] as { x?: unknown }).x) &&
				Number.isFinite((entry[1] as { y?: unknown }).y)
			);
		});
	} catch {
		return [];
	}
}

function same_href_without_hash(left: string, right: string): boolean {
	try {
		const left_url = new URL(left, window.location.href);
		const right_url = new URL(right, window.location.href);
		left_url.hash = "";
		right_url.hash = "";
		return left_url.href === right_url.href;
	} catch {
		return false;
	}
}

function preload_css_bundles(bundles: string[]): void {
	for (const href of new Set(bundles)) {
		if (find_css_bundle(href) || find_css_preload(href)) {
			continue;
		}
		const link = document.createElement("link");
		link.rel = browser_dom_attr.preload_rel;
		link.setAttribute(
			browser_dom_attr.preload_as_attr,
			browser_dom_attr.style_type,
		);
		link.setAttribute(CSS_PRELOAD_ATTR, href);
		link.setAttribute(CSS_PRELOAD_SETTLED_ATTR, css_preload_settled.no);
		const mark = (): void => {
			link.removeEventListener(dom_event.load, mark);
			link.removeEventListener(dom_event.error, mark);
			link.setAttribute(
				CSS_PRELOAD_SETTLED_ATTR,
				css_preload_settled.yes,
			);
		};
		link.addEventListener(dom_event.load, mark);
		link.addEventListener(dom_event.error, mark);
		link.href = href;
		document.head.appendChild(link);
	}
}

function wait_for_css_bundles(
	bundles: string[],
	signal: AbortSignal,
): Promise<void> {
	return new Promise((resolve) => {
		const pending = [...new Set(bundles)]
			.map((href) => {
				return find_css_preload(href);
			})
			.filter((node): node is HTMLLinkElement => {
				return (
					!!node &&
					node.getAttribute(CSS_PRELOAD_SETTLED_ATTR) !==
						css_preload_settled.yes
				);
			});
		if (signal.aborted || pending.length === 0) {
			resolve();
			return;
		}
		let remaining = pending.length;
		let settled = false;
		const cleanups: Array<() => void> = [];
		const cleanup = (): void => {
			for (const remove of cleanups.splice(0)) {
				remove();
			}
		};
		const complete = (): void => {
			if (settled) {
				return;
			}
			settled = true;
			cleanup();
			resolve();
		};
		const on_abort = (): void => {
			complete();
		};
		signal.addEventListener("abort", on_abort, { once: true });
		cleanups.push(() => {
			signal.removeEventListener("abort", on_abort);
		});
		for (const node of pending) {
			const done = (): void => {
				node.setAttribute(
					CSS_PRELOAD_SETTLED_ATTR,
					css_preload_settled.yes,
				);
				remaining -= 1;
				if (remaining === 0) {
					complete();
				}
			};
			node.addEventListener(dom_event.load, done, { once: true });
			node.addEventListener(dom_event.error, done, { once: true });
			cleanups.push(() => {
				node.removeEventListener(dom_event.load, done);
				node.removeEventListener(dom_event.error, done);
			});
		}
	});
}

function apply_css_bundles(bundles: string[]): void {
	for (const href of new Set(bundles)) {
		if (find_css_bundle(href)) {
			continue;
		}
		const link = document.createElement("link");
		link.rel = browser_dom_attr.style_rel;
		link.setAttribute(CSS_BUNDLE_ATTR, href);
		link.href = href;
		document.head.appendChild(link);
	}
}

function find_css_bundle(href: string): HTMLLinkElement | null {
	return document.head.querySelector<HTMLLinkElement>(
		`link[${CSS_BUNDLE_ATTR}="${href}"]`,
	);
}

function find_css_preload(href: string): HTMLLinkElement | null {
	return document.head.querySelector<HTMLLinkElement>(
		`link[${CSS_PRELOAD_ATTR}="${href}"]`,
	);
}

function preload_modules(deps: string[]): void {
	for (const href of new Set(deps)) {
		if (
			document.head.querySelector(
				`link[rel="${browser_dom_attr.modulepreload_rel}"][href="${href}"]`,
			)
		) {
			continue;
		}
		const link = document.createElement("link");
		link.rel = browser_dom_attr.modulepreload_rel;
		link.href = href;
		document.head.appendChild(link);
	}
}

function apply_head_patch(
	title: string | undefined,
	meta_els: HeadEl[],
	rest_els: HeadEl[],
): void {
	if (
		title !== undefined &&
		(title !== "" || document.head.querySelector("title"))
	) {
		document.title = title;
	}
	remove_managed_head_section(head_section.meta);
	remove_managed_head_section(head_section.rest);
	append_head_section(head_section.meta, meta_els);
	append_head_section(head_section.rest, rest_els);
}

function remove_managed_head_section(section: string): void {
	for (const node of Array.from(
		document.head.querySelectorAll(
			`[${browser_dom_attr.head_section}="${section}"]`,
		),
	)) {
		node.remove();
	}
}

function append_head_section(section: string, els: HeadEl[]): void {
	for (const spec of els) {
		const el = document.createElement(spec.tag);
		el.setAttribute(browser_dom_attr.head_section, section);
		for (const [key, value] of Object.entries(spec.attributesKnownSafe)) {
			el.setAttribute(key, value);
		}
		for (const key of spec.booleanAttributes ?? []) {
			el.setAttribute(key, "");
		}
		if (spec.dangerousInnerHTML !== undefined) {
			el.innerHTML = spec.dangerousInnerHTML;
		}
		document.head.appendChild(el);
	}
}
