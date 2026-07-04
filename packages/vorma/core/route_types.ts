/**
 * Outcome of a completed (or awaited) revalidation — from `revalidate()`,
 * `useApiMutation`-style auto-revalidation, or a `revalidateOnWindowFocus`
 * refresh. `ok: false` distinguishes a detected build skew (the server has
 * shipped a new build; a hard reload is likely coming) from exhausting the
 * retry budget against a still-failing server.
 */
export type RevalidationResult =
	| { ok: true }
	| { ok: false; reason: "build_skew" | "max_retries_exhausted" };

/**
 * The current route's error, if any — carried on {@link RouteState.error}.
 * `idx` is the position in `RouteState.matches` where the error originated
 * (a nested route's own segment, not necessarily the outermost one);
 * `source` distinguishes a server-side handler error from a client loader
 * throwing.
 */
export type RouteErrorState = {
	idx: number;
	error: unknown;
	source: "server" | "clientLoader";
};

/** One matched route segment's data, as carried on {@link RouteState.matches}. */
export type RouteMatchState = {
	pattern: string;
	input: unknown;
	viewData: unknown;
	clientLoaderData: unknown;
};

/**
 * The router's read model for the current route — what `useRouteState()`
 * (and the imperative `getRouteState()`) return.
 *
 * `matches` is ordered outermost-to-innermost (the layout chain, ending in
 * the leaf view); `params`/`splatValues` are the union across the whole
 * chain. `error`, when non-null, identifies which match in the chain failed
 * and whether the failure was server- or client-loader-sourced — deeper
 * matches past the error index still render (see the outlet-error-boundary
 * behavior in `RootOutlet`), so a nested error does not blank the whole
 * page.
 */
export type RouteState = {
	href: string;
	historyState: unknown;
	clientBuildId: string;
	params: Record<string, string>;
	splatValues: string[];
	matches: RouteMatchState[];
	error: RouteErrorState | null;
};

/** Why a route commit happened — passed to `onRouteUpdate` and folded into {@link BeforeRouteTransitionArgs.trigger}. */
export type RouteUpdateReason = "boot" | "navigation" | "popstate" | "revalidation";

/** Args passed to {@link BeforeRouteCommitFn}/{@link BeforeRouteYieldFn}. */
export type BeforeRouteTransitionArgs = {
	trigger: Exclude<RouteUpdateReason, "boot">;
	signal: AbortSignal;
	current: RouteState;
	next: RouteState;
};

/**
 * A view's `beforeRouteCommit` hook — runs once the next route's data is
 * ready, just before it replaces the current one. `signal` aborts if a
 * later navigation supersedes this transition mid-hook; an async hook
 * should treat that as cancellation, not commit-blocking. Contrast with
 * {@link BeforeRouteYieldFn}, which runs on the OUTGOING route instead.
 */
export type BeforeRouteCommitFn = (
	args: BeforeRouteTransitionArgs,
) => void | Promise<void>;

/**
 * A view's `beforeRouteYield` hook — runs on the route being navigated AWAY
 * FROM, before the incoming route commits. The natural place for an
 * unsaved-changes guard: inspect `args.next` and, e.g., prompt the user or
 * throw/reject to signal the app wants to intervene (the hook itself does
 * not cancel navigation — see the component's own usage for how a guard is
 * actually wired). Contrast with {@link BeforeRouteCommitFn}, which runs on
 * the INCOMING route.
 */
export type BeforeRouteYieldFn = (
	args: BeforeRouteTransitionArgs,
) => void | Promise<void>;

/**
 * Controls which parts of the target URL a `Link`'s active/pending state
 * comparisons consider. `skip: true` opts a `Link` out of active/pending
 * attribute computation entirely (cheapest option when a link never needs
 * the styling). Otherwise, `includeSearch`/`includeHash` (both default
 * `false`) widen an "active" match to also require matching search params
 * or hash — by default, only the pathname (via matched pattern) is
 * compared.
 */
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

/**
 * Vorma-specific props every adapter's `Link` component accepts, layered
 * onto the framework's native anchor props (React `ComponentProps<"a">`,
 * Solid's `JSX.AnchorHTMLAttributes`, etc.) via {@link ToLinkProps}.
 *
 * - `prefetch: "intent"` starts a prefetch on pointer-enter/focus (see
 *   `prefetchDelayMs` for the hover-intent debounce, default 100ms);
 *   `"none"` (the default) never prefetches from this `Link` alone.
 * - `attributeMatchRules` tunes active/pending detection — see
 *   {@link LinkAttributeMatchRules}. When active/pending, the link renders
 *   `data-vorma-active-exact`/`data-vorma-active-ancestor`/
 *   `data-vorma-pending-exact`/`data-vorma-pending-ancestor` attributes
 *   (style off these, not a class prop) and, when exactly active with no
 *   explicit `aria-current`, an `aria-current="page"` for free.
 * - `visitOnPointerDown` navigates on pointerdown instead of waiting for
 *   click — a snappier feel for primary navigation UI; leave it off for
 *   ordinary content links.
 * - `replace`/`scrollToTop` mirror `navigate()`'s own options for a
 *   click-triggered navigation.
 * - `skipWorkIndicator` excludes this link's navigation from driving
 *   `workIndicator` (see {@link WorkIndicatorOptions}), useful for
 *   background-ish links that should not flash a global loading bar.
 */
export type LinkPropsBase = {
	prefetch?: "intent" | "none";
	prefetchDelayMs?: number;
	attributeMatchRules?: LinkAttributeMatchRules;
	visitOnPointerDown?: boolean;
	replace?: boolean;
	scrollToTop?: boolean;
	skipWorkIndicator?: boolean;
};
