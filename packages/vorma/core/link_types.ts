import type { LinkPropsBase } from "./types.ts";

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
