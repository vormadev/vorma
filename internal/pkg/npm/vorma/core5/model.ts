import type {
	APIToken,
	BrowserKey,
	ClientLoaderID,
	PublicCallID,
	RefreshTimerID,
	RouteToken,
	WorkState,
} from "./events.ts";
import type { AbortHandle } from "./platform.ts";

/////////////////////////////////////////////////////////////////////
/////// Public Contract
/////////////////////////////////////////////////////////////////////

export type APIRouteKind = "query" | "mutation";

export type NavigationSource = "navigate" | "popstate" | "redirect";

export type RevalidationReason =
	| "manual"
	| "retry"
	| "apiRequest"
	| "windowFocus";

export type RevalidationResult =
	| { ok: true }
	| { ok: false; reason: "build_skew" | "max_retries_exhausted" };

export type PublicNavigationResult = { didNavigate: boolean };

export type ScrollState = { x: number; y: number } | { hash: string };

export type ScrollIntent = {
	scroll: ScrollState;
	target_route_id: string;
};

export type RouteErrorState = {
	idx: number;
	error: unknown;
	source: "server" | "clientLoader";
};

export type RouteStateMatch = {
	pattern: string;
	input: unknown;
	loaderData: unknown;
	clientLoaderData: unknown;
};

export type RouteState = {
	href: string;
	historyState: unknown;
	clientBuildID: string;
	params: Record<string, string>;
	splatValues: string[];
	matches: RouteStateMatch[];
	error: RouteErrorState | null;
};

export type RouteRenderEntry = {
	pattern: string;
	input: unknown;
	module_url: string;
	module: Record<string, unknown>;
	loader_data: unknown;
	client_loader_data: unknown;
};

export type RouteRenderState = {
	entries: RouteRenderEntry[];
	error: RouteErrorState | null;
	params: Record<string, string>;
	splat_values: string[];
	client_build_id: string;
	history_state: unknown;
};

export type RouteRender = {
	state: RouteRenderState;
	scroll_intent?: ScrollIntent;
};

export type RouteRenderPlanEntry = {
	pattern: string;
	input: unknown;
	module_url: string;
	loader_data: unknown;
	client_loader_data: unknown;
};

export type RouteRenderPlanState = {
	entries: RouteRenderPlanEntry[];
	error: RouteErrorState | null;
	params: Record<string, string>;
	splat_values: string[];
	client_build_id: string;
	history_state: unknown;
};

export type RouteRenderPlan = {
	state: RouteRenderPlanState;
	scroll_intent?: ScrollIntent;
};

export type RouteUpdate = {
	previous_route: RouteState | null;
	reason: "boot" | "navigation" | "popstate" | "revalidation";
	route: RouteState;
};

export type ClientCommit = {
	route_render?: RouteRender;
	route_update?: RouteUpdate;
	work?: WorkState;
};

export type ClientCommitPlan = {
	route_render?: RouteRenderPlan;
	route_update?: RouteUpdate;
	work?: WorkState;
};

export type WorkIndicatorOptions = {
	start: () => void;
	stop: () => void;
	startDelayMS?: number;
	stopDelayMS?: number;
	skipNavigations?: boolean;
	skipAPIRequests?: boolean;
	skipRevalidations?: boolean;
};

export type WorkIndicatorPolicy = {
	skipNavigations?: boolean;
	skipAPIRequests?: boolean;
	skipRevalidations?: boolean;
};

export type BrowserPosition = {
	href: string;
	key: BrowserKey;
	state: unknown;
};

/////////////////////////////////////////////////////////////////////
/////// Route Data
/////////////////////////////////////////////////////////////////////

export type RoutePayloadMatch = {
	pattern: string;
	input: unknown;
	module_url: string;
	loader_data: unknown;
	server_error: unknown;
};

export type RoutePayload = {
	routes: readonly RoutePayloadMatch[];
	params: Record<string, string>;
	splat_values: readonly string[];
	server_build_id: string;
	deployment_id: string;
	title: string | undefined;
	meta_head_els: readonly unknown[];
	rest_head_els: readonly unknown[];
	css_bundles: readonly string[];
	deps: readonly string[];
};

export type PreparedRouteMatch = {
	pattern: string;
	input: unknown;
	module_url: string;
	loader_data: unknown;
	client_loader_data: unknown;
	client_loader_id: ClientLoaderID;
};

export type PreparedRoute = {
	matches: readonly PreparedRouteMatch[];
	params: Record<string, string>;
	splat_values: readonly string[];
	error: RouteErrorState | null;
	client_build_id: string;
	title: string | undefined;
	meta_head_els: readonly unknown[];
	rest_head_els: readonly unknown[];
	css_bundles: readonly string[];
	deps: readonly string[];
};

export type CurrentRoute = {
	position: BrowserPosition;
	prepared: PreparedRoute;
	route_state: RouteState;
	sequence: number;
};

/////////////////////////////////////////////////////////////////////
/////// Route Slots
/////////////////////////////////////////////////////////////////////

export type NavigationIntent = {
	href: string;
	replace: boolean;
	scroll_to_top: boolean | undefined;
	state: unknown;
	skip_work_indicator: boolean;
	source: NavigationSource;
	is_popstate: boolean;
	is_initial: boolean;
	popstate_restored_scroll: ScrollState | undefined;
	browser_key: BrowserKey;
};

export type RevalidationIntent = {
	attempt: number;
	reason: RevalidationReason;
	skip_work_indicator: boolean;
};

export type ActiveRouteIntent =
	| {
			kind: "navigation";
			nav: NavigationIntent;
			public_calls: readonly PublicCallID[];
	  }
	| { kind: "revalidation"; reval: RevalidationIntent };

export type FetchingActiveRoute = {
	phase: "fetching";
	token: RouteToken;
	abort_handle: AbortHandle;
	url: string;
	intent: ActiveRouteIntent;
	redirect_count: number;
	sequence: number;
};

export type PreparingActiveRoute = {
	phase: "preparing";
	token: RouteToken;
	abort_handle: AbortHandle;
	url: string;
	intent: ActiveRouteIntent;
	redirect_count: number;
	sequence: number;
	payload: RoutePayload;
};

export type PublishingActiveRoute = {
	phase: "publishing";
	token: RouteToken;
	abort_handle: AbortHandle | null;
	url: string;
	intent: ActiveRouteIntent;
	redirect_count: number;
	sequence: number;
	prepared: PreparedRoute;
};

export type ActiveRoute =
	| FetchingActiveRoute
	| PreparingActiveRoute
	| PublishingActiveRoute;

export type FetchingPrefetch = {
	phase: "fetching";
	token: RouteToken;
	abort_handle: AbortHandle;
	url: string;
};

export type PreparingPrefetch = {
	phase: "preparing";
	token: RouteToken;
	abort_handle: AbortHandle;
	url: string;
	payload: RoutePayload;
};

export type PreparedPrefetch = {
	phase: "prepared";
	token: RouteToken;
	url: string;
	prepared: PreparedRoute;
};

export type Prefetch = FetchingPrefetch | PreparingPrefetch | PreparedPrefetch;

/////////////////////////////////////////////////////////////////////
/////// Refresh Slot
/////////////////////////////////////////////////////////////////////

export type RefreshWaiter = { call_id: PublicCallID };

export type RefreshDemand = {
	after_sequence: number;
	reason: RevalidationReason;
	skip_work_indicator: boolean;
	waiters: readonly RefreshWaiter[];
};

export type IdleRefresh = { kind: "idle" };

export type DebouncingRefresh = {
	kind: "debouncing";
	demand: RefreshDemand;
	timer_id: RefreshTimerID;
};

export type PendingRefresh = {
	kind: "pending";
	demand: RefreshDemand;
	attempt: number;
};

export type RetryingRefresh = {
	kind: "retrying";
	demand: RefreshDemand;
	attempt: number;
	timer_id: RefreshTimerID;
};

export type RunningRefresh = {
	kind: "running";
	demand: RefreshDemand;
	attempt: number;
	route_token: RouteToken;
};

export type RefreshSlot =
	| IdleRefresh
	| DebouncingRefresh
	| PendingRefresh
	| RetryingRefresh
	| RunningRefresh;

/////////////////////////////////////////////////////////////////////
/////// API Submissions
/////////////////////////////////////////////////////////////////////

export type Submission = {
	token: APIToken;
	abort_handle: AbortHandle;
	dedupe_key: string | undefined;
	href: string;
	method: string;
	route_kind: APIRouteKind;
	should_revalidate: boolean;
	skip_work_indicator: boolean;
	public_call_id: PublicCallID;
	revalidation_call_id: PublicCallID | null;
};

export type DeferredAPIRedirect = { href: string };

export type PendingBootRevalidations = {
	count: number;
	waiter_call_ids: readonly PublicCallID[];
	skip_work_indicator: boolean;
};

export type FocusRevalidationConfig = {
	stale_time_ms: number;
	skip_work_indicator: boolean;
};

export type Config = {
	client_build_id: string;
	deployment_id: string;
	use_view_transitions: boolean;
	revalidate_on_focus: FocusRevalidationConfig | null;
	work_indicator: WorkIndicatorPolicy | null;
};

export type WorkIndicatorState = { should_be_active: boolean };

export type ActivityState = { last_activity_ms: number };

/////////////////////////////////////////////////////////////////////
/////// Model
/////////////////////////////////////////////////////////////////////

export type Counters = {
	sequence: number;
	route_token: number;
	api_token: number;
	public_call: number;
	browser_key: number;
	refresh_timer: number;
};

export type ModelPhase = "uninitialized" | "booting" | "ready";

export type UninitializedModel = { phase: "uninitialized" };

export type BootingModel = {
	phase: "booting";
	config: Config;
	browser: BrowserPosition;
	active_route:
		| FetchingActiveRoute
		| PreparingActiveRoute
		| PublishingActiveRoute
		| null;
	current: CurrentRoute | null;
	submissions: Readonly<Record<APIToken, Submission>>;
	submissions_by_dedupe: Readonly<Record<string, APIToken>>;
	pending_boot_revalidations: PendingBootRevalidations | null;
	deferred_api_redirect: DeferredAPIRedirect | null;
	counters: Counters;
	activity: ActivityState;
	work_indicator: WorkIndicatorState;
};

export type ReadyModel = {
	phase: "ready";
	config: Config;
	browser: BrowserPosition;
	current: CurrentRoute;
	active_route: ActiveRoute | null;
	prefetch: Prefetch | null;
	refresh: RefreshSlot;
	submissions: Readonly<Record<APIToken, Submission>>;
	submissions_by_dedupe: Readonly<Record<string, APIToken>>;
	deferred_api_redirect: DeferredAPIRedirect | null;
	counters: Counters;
	activity: ActivityState;
	work_indicator: WorkIndicatorState;
};

export type Model = UninitializedModel | BootingModel | ReadyModel;

export function initial_model(): UninitializedModel {
	return { phase: "uninitialized" };
}

export function empty_counters(): Counters {
	return {
		sequence: 0,
		route_token: 0,
		api_token: 0,
		public_call: 0,
		browser_key: 0,
		refresh_timer: 0,
	};
}

/////////////////////////////////////////////////////////////////////
/////// Identity Allocation
/////////////////////////////////////////////////////////////////////

export function next_sequence(c: Counters) {
	const sequence = c.sequence + 1;
	return { counters: { ...c, sequence }, sequence };
}

export function next_route_token(c: Counters) {
	const next = c.route_token + 1;
	return {
		counters: { ...c, route_token: next },
		token: `route-${next}` as RouteToken,
	};
}

export function client_loader_id_for(
	token: RouteToken,
	match_idx: number,
): ClientLoaderID {
	return `${token}:client-loader:${match_idx}` as ClientLoaderID;
}

export function next_api_token(c: Counters) {
	const next = c.api_token + 1;
	return {
		counters: { ...c, api_token: next },
		token: `api-${next}` as APIToken,
	};
}

export function next_public_call(c: Counters) {
	const next = c.public_call + 1;
	return {
		counters: { ...c, public_call: next },
		call_id: `call-${next}` as PublicCallID,
	};
}

export function next_browser_key(c: Counters) {
	const next = c.browser_key + 1;
	return {
		counters: { ...c, browser_key: next },
		key: `bk-${next}` as BrowserKey,
	};
}

export function next_refresh_timer(c: Counters) {
	const next = c.refresh_timer + 1;
	return {
		counters: { ...c, refresh_timer: next },
		id: `timer-${next}` as RefreshTimerID,
	};
}
