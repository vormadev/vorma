import { Effect } from "effect";
import { getHrefDetails } from "vorma/kit/url";
import {
	LINK_ACTIVE_ANCESTOR_ATTR,
	LINK_ACTIVE_EXACT_ATTR,
	LINK_PENDING_ANCESTOR_ATTR,
	LINK_PENDING_EXACT_ATTR,
} from "./constants.ts";
import type { WorkState } from "./create_client_core.ts";
import {
	LINK_CLICK_FAILED_MESSAGE,
	LINK_POINTER_DOWN_FAILED_MESSAGE,
	type LinkIntentRuntime,
	type LinkNavigationEvent,
	make_link_intent_runtime,
} from "./effect_runtime/link_intent_runtime.ts";
import type { LinkPropsBase, RouteState } from "./types.ts";

export type LinkRouteState = {
	href: string;
	matchedPatterns: string[];
};

export type LinkWorkState = {
	navigationHref: string | null;
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

let default_link_intent_runtime: LinkIntentRuntime | undefined;

function get_default_link_intent_runtime(): LinkIntentRuntime {
	if (!default_link_intent_runtime) {
		default_link_intent_runtime = Effect.runSync(
			make_link_intent_runtime(),
		);
	}
	return default_link_intent_runtime;
}

export function select_link_route_state(route: RouteState): LinkRouteState {
	return {
		href: route.href,
		matchedPatterns: route.matches.map((m) => {
			return m.pattern;
		}),
	};
}

export function select_link_work_state(work: WorkState): LinkWorkState {
	return {
		navigationHref: work.navigation?.href ?? null,
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
	link_intent_runtime: LinkIntentRuntime = get_default_link_intent_runtime(),
): LinkPropsResult {
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
	const skip_work_indicator = props[skip_work_indicator_link_prop] as
		| boolean
		| undefined;
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

	const wants_prefetch = prefetch_mode === "intent";
	const prefetch_intent = wants_prefetch
		? Effect.runSync(
				link_intent_runtime.make_prefetch_intent({
					delay_ms: prefetch_delay,
					href,
					nav,
				}),
			)
		: null;

	return {
		is_external: false,
		anchor_props,

		onClick: async (e: unknown) => {
			consumer_click?.(e);
			await Effect.runPromise(
				link_intent_runtime
					.click_navigation({
						event: e as LinkNavigationEvent,
						href,
						nav,
						replace,
						scroll_to_top,
						skip_work_indicator,
						state,
						target_attr,
					})
					.pipe(
						Effect.catch((failure) => {
							return link_intent_runtime.report_navigation_failure(
								LINK_CLICK_FAILED_MESSAGE,
								failure,
							);
						}),
					),
			);
		},

		onPointerDown:
			visit_on_pointer_down || consumer_pointer_down
				? async (e: unknown) => {
						consumer_pointer_down?.(e);
						if (!visit_on_pointer_down) {
							return;
						}
						await Effect.runPromise(
							link_intent_runtime
								.pointer_down_navigation({
									event: e as LinkNavigationEvent,
									href,
									nav,
									replace,
									scroll_to_top,
									skip_work_indicator,
									target_attr,
								})
								.pipe(
									Effect.catch((failure) => {
										return link_intent_runtime.report_navigation_failure(
											LINK_POINTER_DOWN_FAILED_MESSAGE,
											failure,
										);
									}),
								),
						);
					}
				: undefined,

		onPointerEnter:
			wants_prefetch || consumer_pointer_enter
				? (e: unknown) => {
						if (prefetch_intent) {
							Effect.runSync(prefetch_intent.start);
						}
						consumer_pointer_enter?.(e);
					}
				: undefined,

		onFocus:
			wants_prefetch || consumer_focus
				? (e: unknown) => {
						if (prefetch_intent) {
							Effect.runSync(prefetch_intent.start);
						}
						consumer_focus?.(e);
					}
				: undefined,

		onPointerLeave:
			wants_prefetch || consumer_pointer_leave
				? (e: unknown) => {
						if (
							prefetch_intent &&
							!Effect.runSync(link_intent_runtime.is_touch_active)
						) {
							Effect.runSync(prefetch_intent.stop);
						}
						consumer_pointer_leave?.(e);
					}
				: undefined,

		onBlur:
			wants_prefetch || consumer_blur
				? (e: unknown) => {
						if (prefetch_intent) {
							Effect.runSync(prefetch_intent.stop);
						}
						consumer_blur?.(e);
					}
				: undefined,

		onTouchCancel:
			wants_prefetch || consumer_touch_cancel
				? (e: unknown) => {
						if (prefetch_intent) {
							Effect.runSync(prefetch_intent.stop);
						}
						consumer_touch_cancel?.(e);
					}
				: undefined,
	};
}
