import type { LinkPropsBase } from "./types.ts";

/** The route-state slice {@link make_link_props} needs — derived via {@link select_link_route_state}. */
export type LinkRouteState = {
	href: string;
	matched_patterns: string[];
};

/** The work-state slice {@link make_link_props} needs — derived via {@link select_link_work_state}. */
export type LinkWorkState = {
	navigation_href: string | null;
};

/** {@link make_link_props}'s return: derived anchor props and event handlers to spread onto a rendered `<a>` element. */
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

/** The navigation/prefetch/link-state functions {@link make_link_props} needs from the client core — supplied by each adapter's `create_adapter_base` wiring. */
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

export type HrefDetails =
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
