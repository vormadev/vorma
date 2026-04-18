import {
	getHrefDetails,
	getIsModifiedNavigationClick,
	getIsPrimaryNavigationClick,
} from "vorma/kit/url";
import {
	LINK_ACTIVE_ANCESTOR_ATTR,
	LINK_ACTIVE_EXACT_ATTR,
	LINK_PENDING_ANCESTOR_ATTR,
	LINK_PENDING_EXACT_ATTR,
} from "./constants.ts";
import type { RouteState, WorkState } from "./create_client_core.ts";
import type { LinkPropsBase } from "./types.ts";

export type LinkPropsResult = {
	is_external: boolean;
	anchor_props: Record<string, unknown>;
	onClick?: (e: unknown) => void;
	onPointerDown?: (e: unknown) => void;
	onPointerEnter?: (e: unknown) => void;
	onFocus?: (e: unknown) => void;
	onPointerLeave?: (e: unknown) => void;
	onBlur?: (e: unknown) => void;
	onTouchCancel?: (e: unknown) => void;
};

export type LinkNavFns = {
	navigate: (props: {
		href: string;
		replace?: boolean;
		scrollToTop?: boolean;
		state?: unknown;
	}) => Promise<{ didNavigate: boolean }>;
	start_prefetch: (href: string) => void;
	stop_prefetch: (href: string) => void;
	save_current_scroll: () => void;
	register_link_pattern: (pattern: string) => void;
	get_link_attribute_state: (
		href: string,
		match_rules: LinkPropsBase["attributeMatchRules"],
		route_state: RouteState | null,
		work_state: WorkState,
	) => {
		active_exact: boolean;
		active_ancestor: boolean;
		pending_exact: boolean;
		pending_ancestor: boolean;
	};
};

const VORMA_KEYS = new Set([
	"attributeMatchRules",
	"pattern",
	"prefetch",
	"prefetchDelayMs",
	"replace",
	"scrollToTop",
	"state",
	"visitOnPointerDown",
]);

const COMPOSED_EVENT_KEYS = new Set([
	"onClick",
	"onPointerDown",
	"onPointerEnter",
	"onFocus",
	"onPointerLeave",
	"onBlur",
	"onTouchCancel",
]);

let is_touch_active = false;
let input_modality_registered = false;

function register_input_modality(): void {
	if (input_modality_registered) {
		return;
	}
	input_modality_registered = true;

	window.addEventListener("touchstart", () => {
		is_touch_active = true;
	});

	const on_pointer = (e: PointerEvent) => {
		const pt = e.pointerType;
		if (pt === "touch") {
			is_touch_active = true;
		} else if (pt === "mouse" || pt === "pen") {
			is_touch_active = false;
		}
	};

	window.addEventListener("pointerdown", on_pointer);
	window.addEventListener("pointermove", on_pointer);
}

function strip_keys(
	props: Record<string, unknown>,
	keys: Set<string>,
): Record<string, unknown> {
	const out: Record<string, unknown> = {};
	for (const [k, v] of Object.entries(props)) {
		if (!keys.has(k)) {
			out[k] = v;
		}
	}
	return out;
}

export function make_link_props(
	props: Record<string, unknown>,
	nav: LinkNavFns,
	route_state: RouteState | null = null,
	work_state?: WorkState,
): LinkPropsResult {
	register_input_modality();

	const href = (props.href as string) ?? "";
	const pattern = props.pattern as string | undefined;
	if (pattern) {
		nav.register_link_pattern(pattern);
	}

	const details = getHrefDetails(href);
	const is_external = details.isHTTP ? details.isExternal : true;
	const consumer_click = props.onClick as ((e: unknown) => void) | undefined;

	if (is_external) {
		return {
			is_external: true,
			anchor_props: strip_keys(props, VORMA_KEYS),
			onClick: consumer_click,
		};
	}

	const anchor_props = strip_keys(
		strip_keys(props, VORMA_KEYS),
		COMPOSED_EVENT_KEYS,
	);
	const attribute_match_rules =
		props.attributeMatchRules as LinkPropsBase["attributeMatchRules"];
	const link_state =
		attribute_match_rules?.skip === true || !route_state || !work_state
			? null
			: nav.get_link_attribute_state(
					href,
					attribute_match_rules,
					route_state,
					work_state,
				);
	if (link_state?.active_exact) {
		anchor_props[LINK_ACTIVE_EXACT_ATTR] = "";
	}
	if (link_state?.active_ancestor) {
		anchor_props[LINK_ACTIVE_ANCESTOR_ATTR] = "";
	}
	if (link_state?.pending_exact) {
		anchor_props[LINK_PENDING_EXACT_ATTR] = "";
	}
	if (link_state?.pending_ancestor) {
		anchor_props[LINK_PENDING_ANCESTOR_ATTR] = "";
	}
	if (
		link_state?.active_exact &&
		anchor_props["aria-current"] === undefined
	) {
		anchor_props["aria-current"] = "page";
	}

	const prefetch_mode = props.prefetch as string | undefined;
	const prefetch_delay = (props.prefetchDelayMs as number | undefined) ?? 100;
	const replace = props.replace as boolean | undefined;
	const scroll_to_top = props.scrollToTop as boolean | undefined;
	const state = props.state as unknown;
	const target_attr = props.target as string | undefined;
	const visit_on_pointer_down = props.visitOnPointerDown as
		| boolean
		| undefined;
	const consumer_pointer_down = props.onPointerDown as
		| ((e: unknown) => void)
		| undefined;
	const consumer_pointer_enter = props.onPointerEnter as
		| ((e: unknown) => void)
		| undefined;
	const consumer_focus = props.onFocus as ((e: unknown) => void) | undefined;
	const consumer_pointer_leave = props.onPointerLeave as
		| ((e: unknown) => void)
		| undefined;
	const consumer_blur = props.onBlur as ((e: unknown) => void) | undefined;
	const consumer_touch_cancel = props.onTouchCancel as
		| ((e: unknown) => void)
		| undefined;

	let prefetch_timer: number | undefined;
	const wants_prefetch = prefetch_mode === "intent";

	const start_pf = () => {
		if (prefetch_timer !== undefined) {
			clearTimeout(prefetch_timer);
		}
		prefetch_timer = window.setTimeout(() => {
			prefetch_timer = undefined;
			nav.start_prefetch(href);
		}, prefetch_delay);
	};

	const stop_pf = () => {
		if (prefetch_timer !== undefined) {
			clearTimeout(prefetch_timer);
			prefetch_timer = undefined;
		}
		nav.stop_prefetch(href);
	};

	return {
		is_external: false,
		anchor_props,

		onClick: async (e: unknown) => {
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
			if (target_attr && target_attr !== "" && target_attr !== "_self") {
				return;
			}

			ev.preventDefault?.();

			try {
				await nav.navigate({
					href,
					replace,
					scrollToTop: scroll_to_top,
					state,
				});
			} catch (err) {
				console.error("Vorma: Link click failed", err);
			}
		},

		onPointerDown:
			visit_on_pointer_down || consumer_pointer_down
				? async (e: unknown) => {
						const ev = e as any;
						consumer_pointer_down?.(e);
						if (!visit_on_pointer_down) {
							return;
						}
						if (ev.defaultPrevented) {
							return;
						}
						const pt = ev.pointerType;
						if (pt !== "mouse" && pt !== "pen") {
							return;
						}
						if (getIsModifiedNavigationClick(ev)) {
							return;
						}
						if (!getIsPrimaryNavigationClick(ev)) {
							return;
						}
						if (
							target_attr &&
							target_attr !== "" &&
							target_attr !== "_self"
						) {
							return;
						}

						ev.preventDefault?.();

						const el = ev.currentTarget as HTMLElement;
						el.addEventListener(
							"click",
							(ce: Event) => {
								ce.preventDefault();
							},
							{ once: true },
						);

						try {
							await nav.navigate({
								href,
								replace,
								scrollToTop: scroll_to_top,
							});
						} catch (err) {
							console.error(
								"Vorma: Link pointerdown navigation failed",
								err,
							);
						}
					}
				: undefined,

		onPointerEnter:
			wants_prefetch || consumer_pointer_enter
				? (e: unknown) => {
						if (wants_prefetch) {
							start_pf();
						}
						consumer_pointer_enter?.(e);
					}
				: undefined,

		onFocus:
			wants_prefetch || consumer_focus
				? (e: unknown) => {
						if (wants_prefetch) {
							start_pf();
						}
						consumer_focus?.(e);
					}
				: undefined,

		onPointerLeave:
			wants_prefetch || consumer_pointer_leave
				? (e: unknown) => {
						if (wants_prefetch && !is_touch_active) {
							stop_pf();
						}
						consumer_pointer_leave?.(e);
					}
				: undefined,

		onBlur:
			wants_prefetch || consumer_blur
				? (e: unknown) => {
						if (wants_prefetch) {
							stop_pf();
						}
						consumer_blur?.(e);
					}
				: undefined,

		onTouchCancel:
			wants_prefetch || consumer_touch_cancel
				? (e: unknown) => {
						if (wants_prefetch) {
							stop_pf();
						}
						consumer_touch_cancel?.(e);
					}
				: undefined,
	};
}
