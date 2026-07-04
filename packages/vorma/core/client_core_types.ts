import type { Result } from "vorma/kit/result";
import type { HeadEl } from "./head.ts";
import type { RouteRecord, RouteRenderState } from "./route_state_projection.ts";
import type { ScrollIntent, ScrollState } from "./scroll.ts";
import type {
	BeforeRouteCommitFn,
	BeforeRouteYieldFn,
	ResourceKind,
	RevalidationResult,
	RouteState,
	RouteUpdateReason,
} from "./types.ts";
import type { WorkIndicator, WorkIndicatorOptions } from "./work_indicator.ts";
import type { WorkState } from "./work_state.ts";

// Everything below `ViewDefinition` in this file (`ClientLoaderTrigger` and
// later) is the client core's own internal working-state shape — fetch
// intents, deferred results, submission bookkeeping — never reachable
// through a package entry point. Documented where genuinely non-obvious;
// otherwise self-describing field names carry it.

/** Why a revalidation ran — carried in {@link BuildSkewDetectedEvent.triggeringResponse} for the `"revalidation"` trigger case. */
export type RevalidationReason = "manual" | "retry" | "apiRequest" | "windowFocus";

/**
 * Payload for `ClientOptions.onBuildSkewDetected` — fires when a response
 * carries a build id different from the client's own, meaning the server
 * has shipped a new build since this page loaded. `triggeringResponse`
 * identifies exactly what request surfaced the skew (a route fetch, a
 * resource call), and `currentRouteState`/`currentWorkState` are snapshots
 * at detection time — useful for showing a "a new version is available,
 * refresh" banner with context about what the user was doing.
 */
export type BuildSkewDetectedEvent = {
	activeClientBuildId: string;
	serverBuildId: string;
	triggeringResponse:
		| {
				kind: "route";
				trigger: "navigation" | "popstate" | "prefetch";
				requestedHref: string;
				status: number;
				ok: boolean;
		  }
		| {
				kind: "route";
				trigger: "revalidation";
				revalidationReason: RevalidationReason;
				requestedHref: string;
				status: number;
				ok: boolean;
		  }
		| {
				kind: "resource";
				resourceKind: ResourceKind;
				requestedHref: string;
				method: string;
				status: number;
				ok: boolean;
		  };
	currentRouteState: RouteState;
	currentWorkState: WorkState;
};

/**
 * Options passed to `client.boot(options)` (each adapter's
 * `AdapterClientOptions` extends this with a framework-specific `render`
 * signature — see the adapter's own docs for that divergence).
 *
 * - `render`: called once boot's initial route is ready, for mounting the
 *   framework's root component. Adapters that manage mounting themselves
 *   (all three official ones do) provide their own typed `render` in
 *   `AdapterClientOptions`; a custom adapter built on `vorma/__internal`
 *   would use this raw form directly.
 * - `workIndicator`: see {@link WorkIndicatorOptions} — the nprogress-style
 *   contract for a global loading bar.
 * - `revalidateOnWindowFocus`: when the window regains focus after being
 *   hidden, revalidate route data if it has gone stale. `true` uses the
 *   5000ms default staleness window; an object customizes `staleTimeMs`
 *   and/or excludes the refresh from `workIndicator` via
 *   `skipWorkIndicator`. Default `false` (no focus-triggered revalidation).
 * - `defaultErrorBoundary`: fallback error boundary for a matched view that
 *   defines none of its own — see `defineView`'s `errorBoundary`.
 * - `useViewTransitions`: wrap same-document navigations in the browser's
 *   View Transitions API (`document.startViewTransition`) when available;
 *   a no-op fallback elsewhere. Default `false`.
 * - `onRouteUpdate`: fires on every committed route change (SPA-style
 *   analytics/page-view hooks are the typical use).
 * - `onWorkUpdate`: low-level twin of `workIndicator` — fires on every
 *   {@link WorkState} change with the raw state, for apps that want to
 *   drive their own UI directly instead of the indicator abstraction.
 * - `onBuildSkewDetected`: see {@link BuildSkewDetectedEvent}.
 */
export type ClientOptions = {
	render?: () => void | Promise<void>;
	workIndicator?: WorkIndicatorOptions;
	revalidateOnWindowFocus?:
		| boolean
		| { staleTimeMs: number; skipWorkIndicator?: boolean };
	defaultErrorBoundary?: (props: { error: unknown }) => any;
	useViewTransitions?: boolean;
	onRouteUpdate?: (
		route: RouteState,
		previousRoute: RouteState | null,
		reason: RouteUpdateReason,
	) => void;
	onWorkUpdate?: (work: WorkState) => void;
	onBuildSkewDetected?: (event: BuildSkewDetectedEvent) => void;
};

/**
 * One commit the client core publishes to an adapter's `on_commit` callback
 * — every field is independently optional because a given tick may update
 * route render state, route/work read models, or any combination, and an
 * adapter only re-renders what actually changed. Adapter-internal; an app
 * never constructs or reads one directly (it consumes the higher-level
 * `useRouteState`/`useWorkState`/`RootOutlet` surface an adapter derives
 * from these).
 */
export type ClientCommit = {
	route_render?: {
		state: RouteRenderState;
		scroll_intent?: ScrollIntent;
	};
	route_update?: {
		previous_route: RouteState | null;
		reason: RouteUpdateReason;
		route: RouteState;
	};
	work?: WorkState;
};

/** The callback signature `create_client_core` drives with every {@link ClientCommit}. Adapter-internal wiring. */
export type CommitFn = (commit: ClientCommit) => void;

// Internal: injection points for test harnesses (vitest suites and
// `_test_helpers.ts`) to replace real browser APIs (page reload, hard
// navigation, scroll) with observable stand-ins. Never part of the public
// client surface.
export type TestOptions = {
	reload?: () => void;
	hard_redirect?: (url: string) => void;
	scroll_to?: (x: number, y: number) => void;
};

/**
 * A view module's default export, as produced by `defineView(...)`. Every
 * adapter's `defineView` is a thin typed wrapper over this shape — see
 * {@link ToDefineViewArgs} for the camelCase, per-app-typed args form an
 * app actually writes; this snake_case shape is what the router core reads
 * back off the imported module at runtime.
 */
export type ViewDefinition = {
	pattern: string;
	component: (props: any) => any;
	error_boundary?: (props: { error: unknown }) => any;
	client_loader?: ClientLoaderFn;
	before_route_commit?: BeforeRouteCommitFn;
	before_route_yield?: BeforeRouteYieldFn;
};

export type ClientLoaderTrigger = "boot" | "navigation" | "revalidation" | "prefetch";
export type ClientLoaderPrefetchTrigger = Exclude<ClientLoaderTrigger, "boot">;
export type RouteClientLoaderTrigger = Exclude<ClientLoaderTrigger, "prefetch">;
export type ActiveRouteClientLoaderTrigger = Exclude<
	ClientLoaderTrigger,
	"boot" | "prefetch"
>;

export type ClientLoaderFn = (args: {
	trigger: ClientLoaderTrigger;
	href: string;
	historyState: unknown;
	pattern: string;
	params: Record<string, string>;
	splatValues: string[];
	input: unknown;
	knownMatches: ClientLoaderKnownMatch[];
	serverPromise: Promise<ClientLoaderServerState>;
	signal: AbortSignal;
}) => Promise<unknown>;

export type ClientLoaderKnownMatch = {
	pattern: string;
	input: unknown;
};

export type ClientLoaderServerState = {
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
	viewData: unknown;
};

export type ClientLoaderResult = { data: unknown } | { error: unknown } | undefined;

export type ApiResult<T> =
	| {
			success: true;
			data: T;
			response: Response;
			revalidationPromise: Promise<RevalidationResult>;
	  }
	| {
			success: false;
			error: string;
			response?: Response;
			revalidationPromise: Promise<RevalidationResult>;
	  };

export type ClientCore = {
	boot: (options: ClientOptions) => Promise<Result<void>>;
	workIndicator: WorkIndicator;
	navigate: (
		href: string | URL,
		options?: {
			replace?: boolean;
			scrollToTop?: boolean;
			state?: unknown;
			skipWorkIndicator?: boolean;
		},
	) => Promise<{ didNavigate: boolean }>;
	revalidate: () => Promise<RevalidationResult>;
	submit_inner: <T = unknown>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: {
			resourceKind?: ResourceKind;
			dedupeKey?: string;
			revalidate?: boolean;
			skipWorkIndicator?: boolean;
		},
	) => Promise<ApiResult<T>>;
	getRouteState: () => RouteState;
	getWorkState: () => WorkState;
	getClientBuildId: () => string;
	getRootEl: () => HTMLElement;
	defineView: <T = any>(input: {
		pattern: string;
		component: (props: any) => any;
		errorBoundary?: (props: { error: unknown }) => any;
		clientLoader?: (props: any) => Promise<T>;
		beforeRouteCommit?: BeforeRouteCommitFn;
		beforeRouteYield?: BeforeRouteYieldFn;
		runClientLoaderOnHmr?: boolean;
	}) => ViewDefinition & { __phantom_client_loader_data?: T };
	start_prefetch: (href: string) => void;
	stop_prefetch: (href: string) => void;
	save_current_scroll: () => void;
	get_default_error_boundary: () => ((props: { error: unknown }) => any) | undefined;
};

export type Deferred<T> = {
	promise: Promise<T>;
	resolve: (value: T) => void;
};

export type RouteRenderCommitReason =
	| "initial"
	| "navigation"
	| "popstate"
	| "revalidation"
	| "hmr";

export type WorkProjection =
	| {
			kind: "navigation";
			skip_work_indicator?: boolean;
	  }
	| {
			kind: "revalidation";
			skip_work_indicator?: boolean;
	  }
	| {
			kind: "apiRequest";
			skip_work_indicator?: boolean;
	  }
	| {
			kind: "prefetch";
	  };

export type NavOptions = {
	replace?: boolean;
	scroll_to_top?: boolean;
	state?: unknown;
	is_popstate?: boolean;
	popstate_scroll?: ScrollState;
	skip_work_indicator?: boolean;
};

export type NavResult = { didNavigate: boolean };

export type RedirectResult = "settled" | "transferred";

export type NavigationSource = "navigate" | "popstate" | "redirect";

export type NavFetchIntent = {
	kind: "nav";
	url: URL;
	options: NavOptions;
	source: NavigationSource;
	redirect_count: number;
	deferred: Deferred<NavResult>;
};

export type RevalidationFetchIntent = {
	kind: "reval";
	attempt: number;
	reason: RevalidationReason;
	skip_work_indicator?: boolean;
};

export type ActiveFetchIntent = NavFetchIntent | RevalidationFetchIntent;

export type PrefetchFetchIntent = { kind: "prefetch" };

export type FetchIntent = ActiveFetchIntent | PrefetchFetchIntent;

export type FetchBase = {
	url: URL;
	ac: AbortController;
	seq: number;
	data_promise: Promise<FetchResult>;
	client_loader_prefetches: ClientLoaderPrefetch[];
	close_client_loader_prefetches: () => void;
};

export type ActiveFetch = FetchBase & {
	prepare_ready: Promise<void>;
	intent: ActiveFetchIntent;
};

export type PrefetchFetch = FetchBase & {
	is_pending: boolean;
	prepare_ready: Promise<void>;
	intent: PrefetchFetchIntent;
};

export type FetchResult =
	| { kind: "data"; data: unknown; response: Response }
	| { kind: "build_skew"; response: Response }
	| { kind: "redirect"; href: string; hard: boolean; response: Response }
	| { kind: "error"; response: Response };

export type ClientLoaderPrefetch = {
	pattern: string;
	resolve_server_state: (data: ClientLoaderServerState) => void;
	abort: () => void;
	result_promise: Promise<unknown>;
};

export type DecodedRoute = {
	pattern: string;
	input: unknown;
	module_url: string;
	view_data: unknown;
	server_error: unknown;
};

export type DecodedPayload = {
	routes: DecodedRoute[];
	params: Record<string, string>;
	splat_values: string[];
	title: string | undefined;
	meta_head_els: HeadEl[];
	rest_head_els: HeadEl[];
	css_bundles: string[];
	deps: string[];
};

export type PreparedRoute = {
	route: RouteRecord;
	apply_dom_side_effects: () => void;
};

export type Submission = {
	ac: AbortController;
	deferred: Deferred<ApiResult<unknown>>;
	did_dispatch: boolean;
	key: string;
	method: string;
	href: string;
	revalidation_promise: Promise<RevalidationResult>;
	settled: boolean;
	should_revalidate: boolean;
	skip_work_indicator?: boolean;
};
