import type {
	AppConfig,
	AppView,
	ConditionalSplat,
	ConditionalViewParams,
	ViewByPattern,
	ViewParamsRecord,
} from "./generated_contract_types.ts";
import type {
	BeforeRouteCommitFn,
	BeforeRouteYieldFn,
	LinkPropsBase,
} from "./route_types.ts";

/////// VIEW PATTERNS

/** Every view pattern declared in `A`'s generated `AppConfig` — the type-level source of `Link`/`navigate`/`defineView`'s pattern autocompletion. */
export type ToViewPattern<A extends AppConfig> = AppView<A>["pattern"];

type PermissiveViewPattern<
	A extends AppConfig,
	P extends ToViewPattern<A>,
> = P extends `${infer Prefix}/_index` ? P | (Prefix extends "" ? "/" : Prefix) : P;

/////// VIEW I/O

/** A view's typed output (the data its handler returns) for pattern `P` — what `useViewData`/`usePatternViewData` return. */
export type ToViewOutput<A extends AppConfig, P extends ToViewPattern<A>> =
	ViewByPattern<A, P> extends { __o: infer O } ? O : never;

/** A view's typed input (its search-param schema) for pattern `P` — what the view's own `input` (and `ToClientLoaderArgs.input`) carry. */
export type ToViewInput<A extends AppConfig, P extends ToViewPattern<A>> =
	ViewByPattern<A, P> extends { __i: infer I } ? I : never;

/////// PARENT VIEW INPUT

type ViewParents<A extends AppConfig, P extends string> =
	ViewByPattern<A, P> extends {
		parents: ReadonlyArray<infer Parent>;
	}
		? Extract<Parent, ToViewPattern<A>>
		: never;

/*
A matched URL's query feeds EVERY matched view's input, so the typed
search is the intersection of the whole chain's inputs. Views with no
input contribute nothing — without the `undefined -> {}` normalization a
plain layout would annihilate the intersection and type every child's
search as `undefined`.
*/
type ViewInputWithParentViews<A extends AppConfig, P extends ToViewPattern<A>> = (
	P | ViewParents<A, P> extends infer Pattern
		? Pattern extends ToViewPattern<A>
			? (
					input: [ToViewInput<A, Pattern>] extends [undefined]
						? // oxlint-disable-next-line no-empty-object-type -- identity for intersection
							{}
						: ToViewInput<A, Pattern>,
				) => void
			: never
		: never
) extends (input: infer Input) => void
	? Input
	: never;

/////// ROUTE TARGETS

/**
 * A typed pattern-based navigation target: `pattern` plus whatever `params`/
 * `splatValues` the pattern requires and a `search` typed as the
 * intersection of the matched chain's own input schemas (a plain layout
 * with no input contributes nothing to that intersection, so a child
 * route's search type is never widened to `undefined` by an input-less
 * parent). `href` is mutually exclusive with this form — see
 * {@link ToNavigationTarget} for the union that also allows a plain string
 * `href`.
 */
export type ToRouteDestination<A extends AppConfig, P extends ToViewPattern<A>> = {
	href?: never;
	pattern: PermissiveViewPattern<A, P>;
	search?: ViewInputWithParentViews<A, P>;
	hash?: string;
} & ConditionalViewParams<A, P> &
	ConditionalSplat<P>;

/**
 * Either a plain string `href` or a typed {@link ToRouteDestination} — the
 * target shape `navigate`/`prefetch`/`cancelPrefetch`/`Link` all accept.
 * The union is deliberately exclusive: `href` and typed pattern fields
 * (`pattern`/`params`/`splatValues`/`search`/`hash`) can never be supplied
 * together, so a caller cannot accidentally typecheck a `href` string
 * alongside typed `search` params that would silently be ignored.
 */
export type ToNavigationTarget<A extends AppConfig, P extends ToViewPattern<A>> =
	| {
			href: string;
			pattern?: never;
			params?: never;
			splatValues?: never;
			search?: never;
			hash?: never;
	  }
	| ToRouteDestination<A, P>;

/** Args for the typed `navigate(args)` call — a {@link ToNavigationTarget} plus navigation options (`replace`, `scrollToTop`, `state`, and `skipWorkIndicator` to exclude this navigation from `workIndicator`). */
export type ToNavigateArgs<
	A extends AppConfig,
	P extends ToViewPattern<A>,
> = ToNavigationTarget<A, P> & {
	replace?: boolean;
	scrollToTop?: boolean;
	state?: unknown;
	skipWorkIndicator?: boolean;
};

/**
 * Args for `useRouteSync(args)` — keeps the URL in sync with local
 * component state (a search box, a filter panel) by navigating whenever
 * the derived {@link ToRouteDestination} differs from the current route,
 * without the caller managing the navigation lifecycle by hand.
 *
 * - `enabled` (default `true`) turns syncing off without unmounting the
 *   hook — useful when the sync should only run once some other condition
 *   is met.
 * - `debounceMs` (default `0`) delays the navigation after the target
 *   changes, coalescing rapid updates (e.g. as-you-type search) into one
 *   navigation instead of one per keystroke.
 * - `replace` (default `true`, unlike `navigate`'s own default) avoids
 *   filling browser history with one entry per synced change; `scrollToTop`
 *   defaults `false` since a sync is rarely a "go to a new page" gesture.
 */
export type ToRouteSyncArgs<
	A extends AppConfig,
	P extends ToViewPattern<A>,
> = ToRouteDestination<A, P> & {
	enabled?: boolean;
	debounceMs?: number;
	replace?: boolean;
	scrollToTop?: boolean;
};

/////// VIEW COMPONENT PROPS

/**
 * Props every view component receives (via `defineView({ component })`).
 * `idx` is this view's position in the matched chain; call `Outlet(...)` to
 * render the next-deeper matched view (or nothing, past the leaf). The
 * `__phantom_*` fields carry no runtime value — they exist purely so
 * `ToViewComponentProps<A, P>`'s generic parameters are inferable at
 * `useViewData(props)`/`useClientLoaderData(props)` call sites without the
 * caller repeating the pattern.
 */
export type ToViewComponentProps<
	A extends AppConfig,
	P extends ToViewPattern<A>,
	ClientLoaderData = unknown,
> = {
	idx: number;
	Outlet: (local?: Record<string, unknown>) => any;
	__phantom_pattern?: P;
	__phantom_client_loader_data?: ClientLoaderData;
};

/////// CLIENT LOADER PROPS

/** One matched route's identity as seen by a client loader mid-navigation, before that route's data has necessarily resolved — see {@link ToClientLoaderArgs.knownMatches}. */
export type ClientLoaderKnownMatch = {
	pattern: string;
	input: unknown;
};

/**
 * What a client loader's `args.serverPromise` resolves to — the server's
 * authoritative view data for the whole matched chain, arriving
 * independently of (and generally after) `knownMatches`. `outermostServerError`
 * identifies the shallowest server-side handler error in the chain, if any.
 */
export type ClientLoaderServerState<A extends AppConfig, P extends ToViewPattern<A>> = {
	clientBuildId: string;
	matches: Array<{
		pattern: string;
		input: unknown;
		viewData: unknown;
	}>;
	outermostServerError: null | {
		idx: number;
		error: unknown;
	};
	viewData: ToViewOutput<A, P>;
};

/**
 * Args passed to a view's `clientLoader(args)` (declared via
 * `defineView({ clientLoader })`). A client loader runs client-side,
 * concurrently with the server request for the same route, and its
 * resolved value becomes this view's `useClientLoaderData()` — the
 * mechanism for reading a client-only data source (localStorage, an
 * IndexedDB cache, a third-party SDK) into a route without blocking on it
 * server-side.
 *
 * - `trigger` distinguishes why this run happened; `knownMatches` is what
 *   the client already knows about the matched chain's identity BEFORE the
 *   server response lands (pattern + input only — no view data yet); await
 *   `serverPromise` for the server's own data once needed.
 * - `signal` aborts if a later navigation supersedes this one mid-flight —
 *   an async client loader should treat that as cancellation.
 * - Client loaders for different views in a matched chain, and across
 *   independent navigations, always run in parallel to the maximum extent
 *   the router already knows about at fetch time (a contract, not merely
 *   an optimization) — a loader should not assume serialized execution
 *   relative to siblings.
 */
export type ToClientLoaderArgs<A extends AppConfig, P extends ToViewPattern<A>> = {
	trigger: "boot" | "navigation" | "revalidation" | "prefetch";
	href: string;
	historyState: unknown;
	pattern: P;
	params: ViewParamsRecord<A, P>;
	splatValues: string[];
	input: ToViewInput<A, P>;
	knownMatches: ClientLoaderKnownMatch[];
	serverPromise: Promise<ClientLoaderServerState<A, P>>;
	signal: AbortSignal;
};

/////// DEFINE VIEW INPUT

/**
 * Args for `defineView(args)` — the typed, camelCase surface an app writes;
 * `defineView` itself lowers this into a {@link ViewDefinition} (the
 * snake_case shape the router core reads at runtime).
 *
 * - `component`: the view's render function, receiving
 *   {@link ToViewComponentProps}.
 * - `errorBoundary`: renders in place of `component` when this view (or a
 *   view deeper in its chain) errors; falls back to
 *   `ClientOptions.defaultErrorBoundary` when omitted.
 * - `clientLoader`: see {@link ToClientLoaderArgs}.
 * - `beforeRouteCommit`/`beforeRouteYield`: see {@link BeforeRouteCommitFn}/
 *   {@link BeforeRouteYieldFn}.
 * - `runClientLoaderOnHmr` (dev only, default `false`): re-run this view's
 *   client loader when the module hot-reloads, useful while iterating on
 *   client-loader logic without a full page reload.
 */
export type ToDefineViewArgs<
	A extends AppConfig,
	P extends ToViewPattern<A>,
	T = any,
	Element = unknown,
> = {
	pattern: P;
	component: (props: ToViewComponentProps<A, P, T>) => Element;
	errorBoundary?: (props: { error: unknown }) => Element;
	clientLoader?: (args: ToClientLoaderArgs<A, P>) => Promise<T>;
	beforeRouteCommit?: BeforeRouteCommitFn;
	beforeRouteYield?: BeforeRouteYieldFn;
	runClientLoaderOnHmr?: boolean;
};

/////// LINK PROPS

/** Full typed prop set for an adapter's `Link` component: {@link LinkPropsBase} plus a typed navigation target and an opaque `state` to carry into `history.state`. */
export type ToLinkProps<A extends AppConfig, P extends ToViewPattern<A>> = LinkPropsBase &
	ToNavigationTarget<A, P> & {
		state?: unknown;
	};
