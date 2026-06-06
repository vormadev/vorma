export type RevalidationResult =
	| { ok: true }
	| { ok: false; reason: "build_skew" | "max_retries_exhausted" };

export type RouteErrorState = {
	idx: number;
	error: unknown;
	source: "server" | "clientLoader";
};

export type RouteMatchState = {
	pattern: string;
	input: unknown;
	viewData: unknown;
	clientLoaderData: unknown;
};

export type RouteState = {
	href: string;
	historyState: unknown;
	clientBuildId: string;
	params: Record<string, string>;
	splatValues: string[];
	matches: RouteMatchState[];
	error: RouteErrorState | null;
};

export type RouteUpdateReason = "boot" | "navigation" | "popstate" | "revalidation";

export type BeforeRouteTransitionArgs = {
	trigger: Exclude<RouteUpdateReason, "boot">;
	signal: AbortSignal;
	current: RouteState;
	next: RouteState;
};

export type BeforeRouteCommitFn = (
	args: BeforeRouteTransitionArgs,
) => void | Promise<void>;

export type BeforeRouteYieldFn = (
	args: BeforeRouteTransitionArgs,
) => void | Promise<void>;

export type LinkAttributeMatchRules =
	| {
			skip?: false;
			includeSearch?: boolean;
			includeHash?: boolean;
	  }
	| {
			skip: true;
			includeSearch?: never;
			includeHash?: never;
	  };

export type LinkPropsBase = {
	prefetch?: "intent" | "none";
	prefetchDelayMs?: number;
	attributeMatchRules?: LinkAttributeMatchRules;
	visitOnPointerDown?: boolean;
	replace?: boolean;
	scrollToTop?: boolean;
	skipWorkIndicator?: boolean;
};
