import {
	LINK_ACTIVE_ANCESTOR_ATTR,
	LINK_ACTIVE_EXACT_ATTR,
	LINK_PENDING_ANCESTOR_ATTR,
	LINK_PENDING_EXACT_ATTR,
} from "./constants.ts";
import type { WorkState } from "./create_client_core.ts";
import type { LinkPropsBase, RouteState } from "./types.ts";

export type LinkRouteState = {
	href: string;
	matched_patterns: string[];
};

export type LinkWorkState = {
	navigation_href: string | null;
};

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
	navigate: (args: {
		href: string;
		replace?: boolean;
		scrollToTop?: boolean;
		skipWorkIndicator?: boolean;
		state?: unknown;
	}) => Promise<{ didNavigate: boolean }>;
	start_prefetch: (href: string) => void;
	stop_prefetch: (href: string) => void;
	save_current_scroll: () => void;
	register_link_pattern: (pattern: string) => void;
	get_link_state_version: () => number;
	subscribe_link_state: (listener: () => void) => () => void;
	get_link_attribute_state: (
		href: string,
		match_rules: LinkPropsBase["attributeMatchRules"],
		route_state: LinkRouteState | null,
		work_state: LinkWorkState,
	) => {
		active_exact: boolean;
		active_ancestor: boolean;
		pending_exact: boolean;
		pending_ancestor: boolean;
	};
};

export const skip_work_indicator_link_prop = "skipWorkIndicator";

const VORMA_KEYS = new Set([
	"attributeMatchRules",
	"pattern",
	"prefetch",
	"prefetchDelayMs",
	"replace",
	"scrollToTop",
	skip_work_indicator_link_prop,
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

	const on_pointer = (e: PointerEvent): void => {
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

export function select_link_route_state(route: RouteState): LinkRouteState {
	return {
		href: route.href,
		matched_patterns: route.matches.map((m) => {
			return m.pattern;
		}),
	};
}

export function select_link_work_state(work: WorkState): LinkWorkState {
	return {
		navigation_href: work.navigation?.href ?? null,
	};
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
	route_state: LinkRouteState | null = null,
	work_state?: LinkWorkState,
): LinkPropsResult {
	register_input_modality();

	const href = (props.href as string) ?? "";
	const pattern = props.pattern as string | undefined;
	if (pattern) {
		nav.register_link_pattern(pattern);
	}

	const details = get_href_details(href);
	const is_external = details.is_http ? details.is_external : true;
	const consumer_click = props.onClick as ((e: unknown) => void) | undefined;

	if (is_external) {
		return {
			is_external: true,
			anchor_props: strip_keys(props, VORMA_KEYS),
			onClick: consumer_click,
		};
	}

	const anchor_props = strip_keys(strip_keys(props, VORMA_KEYS), COMPOSED_EVENT_KEYS);
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
	if (link_state?.active_exact && anchor_props["aria-current"] === undefined) {
		anchor_props["aria-current"] = "page";
	}

	const prefetch_mode = props.prefetch as string | undefined;
	const prefetch_delay = (props.prefetchDelayMs as number | undefined) ?? 100;
	const replace = props.replace as boolean | undefined;
	const scroll_to_top = props.scrollToTop as boolean | undefined;
	const skip_work_indicator = props[skip_work_indicator_link_prop] as
		| boolean
		| undefined;
	const state = props.state as unknown;
	const target_attr = props.target as string | undefined;
	const visit_on_pointer_down = props.visitOnPointerDown as boolean | undefined;
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

	const start_pf = (): void => {
		if (prefetch_timer !== undefined) {
			clearTimeout(prefetch_timer);
		}
		prefetch_timer = window.setTimeout(() => {
			prefetch_timer = undefined;
			nav.start_prefetch(href);
		}, prefetch_delay);
	};

	const stop_pf = (): void => {
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
			if (get_is_modified_navigation_click(ev)) {
				return;
			}
			if (!get_is_primary_navigation_click(ev)) {
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
					...(skip_work_indicator === undefined
						? {}
						: { skipWorkIndicator: skip_work_indicator }),
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
						if (get_is_modified_navigation_click(ev)) {
							return;
						}
						if (!get_is_primary_navigation_click(ev)) {
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
								...(skip_work_indicator === undefined
									? {}
									: {
											skipWorkIndicator: skip_work_indicator,
										}),
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

function get_is_modified_navigation_click(event: {
	metaKey?: boolean;
	altKey?: boolean;
	ctrlKey?: boolean;
	shiftKey?: boolean;
}): boolean {
	return (
		Boolean(event.metaKey) ||
		Boolean(event.altKey) ||
		Boolean(event.ctrlKey) ||
		Boolean(event.shiftKey)
	);
}

function get_is_primary_navigation_click(event: { button?: number }): boolean {
	return event.button === undefined || event.button === 0;
}

type HrefDetails =
	| {
			url: URL;
			is_http: true;
			absoluteUrl: string;
			relativeUrl: string;
			is_external: boolean;
			is_internal: boolean;
	  }
	| {
			is_http: false;
	  };

export function get_href_details(href: string): HrefDetails {
	if (!href) {
		return { is_http: false };
	}

	let url: URL;

	try {
		url = new URL(href, window.location.href);
	} catch {
		return { is_http: false };
	}

	const is_external = url.origin !== window.location.origin;
	const is_internal = !is_external;

	// Filter out things like "#", "tel:", "mailto:", or custom schemes.
	const is_http = url.protocol === "http:" || url.protocol === "https:";
	if (!is_http) {
		return { is_http: false };
	}

	if (is_external) {
		return {
			url,
			is_http: true,
			absoluteUrl: url.href,
			relativeUrl: "",
			is_external,
			is_internal,
		};
	}

	return {
		url,
		is_http: true,
		absoluteUrl: url.href,
		relativeUrl: url.href.replace(url.origin, ""),
		is_external,
		is_internal,
	};
}
