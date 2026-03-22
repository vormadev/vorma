/// <reference types="vite/client" />

import {
	getIsModifiedNavigationClick,
	getIsPrimaryNavigationClick,
} from "vorma/kit/url";
import { ABORT_REASON, encode_abort_reason } from "./abort.ts";
import { ensure_global } from "./global_state.ts";
import { apply_scroll_state, normalize_hash } from "./scroll.ts";
import type { ScrollState, VormaLinkPropsBase } from "./types.ts";
import { classify_target, getHrefDetails, get_target_data_key } from "./url.ts";

// For external links we only strip nav-specific props, not event handlers
// (since we don't provide overriding handlers except onClick).
const NAV_ONLY_KEYS = new Set<string>([
	"prefetch",
	"prefetchDelayMs",
	"replace",
	"scrollToTop",
	"beforeNavigate",
	"beforeRender",
	"afterRender",
	"pattern",
	"params",
	"splatValues",
	"search",
	"hash",
	"state",
]);

// single set of keys to strip, covering both nav-specific
// props and event handlers that we compose internally.
const INTERNAL_PROP_KEYS = new Set<string>([
	...NAV_ONLY_KEYS.values(),
	// event handlers we compose
	"onPointerEnter",
	"onFocus",
	"onPointerLeave",
	"onBlur",
	"onTouchCancel",
	"onClick",
]);

function strip_props(
	props: Record<string, unknown>,
	keys: Set<string>,
): Record<string, unknown> {
	const out: Record<string, unknown> = {};
	for (const [key, value] of Object.entries(props)) {
		if (!keys.has(key)) {
			out[key] = value;
		}
	}
	return out;
}

// ─── Prefetch Handlers ──────────────────────────────────────────

function create_prefetch_handlers(props: {
	href: string;
	delay_ms?: number;
	before_begin?: (e: unknown) => void | Promise<void>;
}): {
	start: (e: unknown) => void;
	stop: (opts?: { clear_cache?: boolean; abort_in_flight?: boolean }) => void;
} | null {
	const details = getHrefDetails(props.href);
	if (!details.isHTTP || !details.isInternal) {
		return null;
	}
	const target = details.absoluteURL;
	let timer: number | undefined;

	return {
		start: (event) => {
			if (
				classify_target(target, window.location.href) !==
				"requires-fetch"
			) {
				return;
			}
			if (timer !== undefined) {
				clearTimeout(timer);
			}
			timer = window.setTimeout(async () => {
				timer = undefined;
				try {
					await props.before_begin?.(event);
					const { vormaNavigate } = await import("./public_api.ts");
					await vormaNavigate(target, {
						intent: "prefetch",
						skipGlobalLoadingIndicator: true,
					});
				} catch (err) {
					console.error("Vorma:", "Prefetch failed", err);
				}
			}, props.delay_ms ?? 100);
		},
		stop: (opts) => {
			if (timer !== undefined) {
				clearTimeout(timer);
				timer = undefined;
			}
			const store = ensure_global().nav_state_manager?.get_store();
			if (!store) {
				return;
			}
			const key = get_target_data_key(target);
			const clear_cache = opts?.clear_cache ?? true;
			const abort = opts?.abort_in_flight ?? true;
			// Don't abort if a full navigation is targeting the same key
			if (
				store.navigate_op &&
				get_target_data_key(store.navigate_op.target_url) === key
			) {
				if (clear_cache) {
					store.prefetch_cache.delete(key);
				}
				return;
			}
			if (abort) {
				store.prefetch_ops
					.get(key)
					?.abort_controller.abort(
						encode_abort_reason(ABORT_REASON.prefetch_stopped),
					);
				store.prefetch_ops.delete(key);
			}
			if (clear_cache) {
				store.prefetch_cache.delete(key);
			}
		},
	};
}

function has_prefetch_for(href: string): boolean {
	const store = ensure_global().nav_state_manager?.get_store();
	if (!store) {
		return false;
	}
	const key = get_target_data_key(href);
	return store.prefetch_ops.has(key) || store.prefetch_cache.has(key);
}

// ─── Link Props Result ──────────────────────────────────────────

export type LinkPropsResult<E> = {
	dataExternal?: boolean;
	anchorProps: Record<string, unknown>;
	onPointerEnter?: (e: unknown) => void;
	onFocus?: (e: unknown) => void;
	onPointerLeave?: (e: unknown) => void;
	onBlur?: (e: unknown) => void;
	onTouchCancel?: (e: unknown) => void;
	onClick?: (e: E) => void | Promise<void>;
};

function is_fn(v: unknown): v is (...args: any[]) => any {
	return typeof v === "function";
}

export function make_link_props<LinkEvent>(
	link_props: { href?: string } & VormaLinkPropsBase<LinkEvent>,
): LinkPropsResult<LinkEvent> {
	const href = link_props.href ?? "";
	const details = getHrefDetails(href);
	const is_external = details.isHTTP ? details.isExternal : true;
	const consumer_click = (link_props as any).onClick as
		| ((e: LinkEvent) => void | Promise<void>)
		| undefined;

	// For external links, leave consumer event handlers on anchorProps
	// since we don't provide overriding handlers (except onClick).
	if (is_external) {
		return {
			dataExternal: true,
			anchorProps: strip_props(link_props, NAV_ONLY_KEYS),
			onClick: async (e) => {
				try {
					await consumer_click?.(e);
					await link_props.beforeNavigate?.(e);
				} catch {
					/* swallow in DOM event handler */
				}
			},
		};
	}

	// For internal links: strip both nav props and event handlers in
	// one pass, then compose event handlers with internal behavior.
	const anchorProps = strip_props(link_props, INTERNAL_PROP_KEYS);
	const consumer = {
		onPointerEnter: (link_props as any).onPointerEnter as
			| ((e: unknown) => void)
			| undefined,
		onFocus: (link_props as any).onFocus as
			| ((e: unknown) => void)
			| undefined,
		onPointerLeave: (link_props as any).onPointerLeave as
			| ((e: unknown) => void)
			| undefined,
		onBlur: (link_props as any).onBlur as
			| ((e: unknown) => void)
			| undefined,
		onTouchCancel: (link_props as any).onTouchCancel as
			| ((e: unknown) => void)
			| undefined,
	};

	const pf =
		link_props.prefetch === "intent"
			? create_prefetch_handlers({
					href,
					delay_ms: link_props.prefetchDelayMs,
					before_begin: link_props.beforeNavigate as any,
				})
			: null;
	const should_stop = () => !ensure_global().is_touch_active;

	return {
		dataExternal: undefined,
		anchorProps,
		onPointerEnter:
			pf || is_fn(consumer.onPointerEnter)
				? (e: unknown) => {
						pf?.start(e);
						consumer.onPointerEnter?.(e);
					}
				: undefined,
		onFocus:
			pf || is_fn(consumer.onFocus)
				? (e: unknown) => {
						pf?.start(e);
						consumer.onFocus?.(e);
					}
				: undefined,
		onPointerLeave:
			pf || is_fn(consumer.onPointerLeave)
				? (e: unknown) => {
						if (should_stop()) {
							pf?.stop();
						}
						consumer.onPointerLeave?.(e);
					}
				: undefined,
		onBlur:
			pf || is_fn(consumer.onBlur)
				? (e: unknown) => {
						pf?.stop();
						consumer.onBlur?.(e);
					}
				: undefined,
		onTouchCancel:
			pf || is_fn(consumer.onTouchCancel)
				? (e: unknown) => {
						pf?.stop();
						consumer.onTouchCancel?.(e);
					}
				: undefined,
		onClick: async (e) => {
			try {
				const ev = e as any;
				consumer_click?.(e);
				if (ev.defaultPrevented) {
					return;
				}
				if (getIsModifiedNavigationClick(ev)) {
					return;
				}
				if (!getIsPrimaryNavigationClick(ev)) {
					return;
				}
				const target_attr = (link_props as any).target;
				if (
					typeof target_attr === "string" &&
					target_attr !== "" &&
					target_attr !== "_self"
				) {
					return;
				}

				pf?.stop({ clear_cache: false, abort_in_flight: false });
				const should_run_begin = !has_prefetch_for(href);
				const classification = classify_target(
					href,
					window.location.href,
				);

				if (classification === "same-document-noop") {
					ev.preventDefault?.();
					const hash = new URL(href, window.location.href).hash;
					const scroll: ScrollState | undefined =
						normalize_hash(hash).length > 0
							? { hash }
							: link_props.scrollToTop === false
								? undefined
								: { x: 0, y: 0 };
					apply_scroll_state(scroll);
					return;
				}

				ev.preventDefault?.();
				const { vormaNavigate } = await import("./public_api.ts");

				if (classification === "same-document-hash-change") {
					await vormaNavigate(href, {
						replace: link_props.replace,
						scrollToTop: link_props.scrollToTop,
					});
					return;
				}

				if (should_run_begin) {
					await link_props.beforeNavigate?.(e);
				}
				await link_props.beforeRender?.(e);
				const result = await vormaNavigate(href, {
					replace: link_props.replace,
					scrollToTop: link_props.scrollToTop,
				});
				if (result.didNavigate) {
					await link_props.afterRender?.(e);
				}
			} catch (err) {
				console.error("Vorma:", "Link click failed", err);
			}
		},
	};
}
