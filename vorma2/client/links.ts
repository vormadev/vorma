/// <reference types="vite/client" />

////////////////////////////////////////////////////////////////////////////////
// Link props builder. Returns camelCase handler names so adapters can
// spread directly onto <a> without remapping.
////////////////////////////////////////////////////////////////////////////////

import {
	getIsModifiedNavigationClick,
	getIsPrimaryNavigationClick,
} from "vorma/kit/url";
import { ABORT_REASON, encode_abort_reason } from "./abort.ts";
import { ensure_global } from "./global_state.ts";
import { apply_scroll_state, normalize_hash } from "./scroll.ts";
import type { ScrollState, VormaLinkPropsBase } from "./types.ts";
import { classify_target, getHrefDetails, get_target_data_key } from "./url.ts";

// Keys that are ours, not standard <a> attributes — strip before spreading.
const NAV_PROP_KEYS = new Set<string>([
	"prefetch",
	"prefetchDelayMs",
	"replace",
	"scrollToTop",
	"beforeBegin",
	"beforeRender",
	"afterRender",
	"pattern",
	"params",
	"splatValues",
	"search",
	"hash",
]);

function strip_nav_props(
	props: Record<string, unknown>,
): Record<string, unknown> {
	const out: Record<string, unknown> = {};
	for (const [key, value] of Object.entries(props)) {
		if (!NAV_PROP_KEYS.has(key)) out[key] = value;
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
	if (!details.isHTTP || !details.isInternal) return null;
	const target = details.absoluteURL;
	let timer: number | undefined;

	return {
		start: (event) => {
			if (
				classify_target(target, window.location.href) !==
				"requires-fetch"
			)
				return;
			if (timer !== undefined) clearTimeout(timer);
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
			if (!store) return;
			const key = get_target_data_key(target);
			const clear_cache = opts?.clear_cache ?? true;
			const abort = opts?.abort_in_flight ?? true;
			// Don't abort if a full navigation is targeting the same key
			if (
				store.navigate_op &&
				get_target_data_key(store.navigate_op.target_url) === key
			) {
				if (clear_cache) store.prefetch_cache.delete(key);
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
			if (clear_cache) store.prefetch_cache.delete(key);
		},
	};
}

function has_prefetch_for(href: string): boolean {
	const store = ensure_global().nav_state_manager?.get_store();
	if (!store) return false;
	const key = get_target_data_key(href);
	return store.prefetch_ops.has(key) || store.prefetch_cache.has(key);
}

// ─── Link Props Result ──────────────────────────────────────────
// camelCase handler names match DOM event handler props exactly,
// so React/Preact/Solid adapters can spread without remapping.

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

export function make_link_props<LinkEvent>(
	link_props: { href?: string } & VormaLinkPropsBase<LinkEvent>,
): LinkPropsResult<LinkEvent> {
	const href = link_props.href ?? "";
	const details = getHrefDetails(href);
	const is_external = details.isHTTP ? details.isExternal : true;
	const anchorProps = strip_nav_props(link_props);
	const consumer_click = (link_props as any).onClick as
		| ((e: LinkEvent) => void | Promise<void>)
		| undefined;

	if (is_external) {
		return {
			dataExternal: true,
			anchorProps,
			onClick: async (e) => {
				try {
					await consumer_click?.(e);
					await link_props.beforeBegin?.(e);
				} catch {
					/* swallow in DOM event handler */
				}
			},
		};
	}

	const pf =
		link_props.prefetch === "intent"
			? create_prefetch_handlers({
					href,
					delay_ms: link_props.prefetchDelayMs,
					before_begin: link_props.beforeBegin as any,
				})
			: null;
	const should_stop = () => !ensure_global().is_touch_active;

	return {
		dataExternal: undefined,
		anchorProps,
		onPointerEnter: pf ? (e) => pf.start(e) : undefined,
		onFocus: pf ? (e) => pf.start(e) : undefined,
		onPointerLeave: pf
			? () => {
					if (should_stop()) pf.stop();
				}
			: undefined,
		onBlur: pf ? () => pf.stop() : undefined,
		onTouchCancel: pf ? () => pf.stop() : undefined,
		onClick: async (e) => {
			try {
				const ev = e as any;
				await consumer_click?.(e);
				if (ev.defaultPrevented) return;
				if (getIsModifiedNavigationClick(ev)) return;
				if (!getIsPrimaryNavigationClick(ev)) return;
				const target_attr = (link_props as any).target;
				if (
					typeof target_attr === "string" &&
					target_attr !== "" &&
					target_attr !== "_self"
				)
					return;

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

				if (should_run_begin) await link_props.beforeBegin?.(e);
				await link_props.beforeRender?.(e);
				const result = await vormaNavigate(href, {
					replace: link_props.replace,
					scrollToTop: link_props.scrollToTop,
				});
				if (result.didNavigate) await link_props.afterRender?.(e);
			} catch (err) {
				console.error("Vorma:", "Link click failed", err);
			}
		},
	};
}
