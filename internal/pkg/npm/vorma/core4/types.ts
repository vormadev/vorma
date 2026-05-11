import type {
	BuildSkewDetectedEvent,
	RouteRenderState,
	WorkIndicator,
	WorkIndicatorOptions,
	WorkState,
} from "../core/create_client_core.ts";
import type { HeadEl } from "../core/head.ts";
import type {
	APIRouteKind,
	ClientLoaderKnownMatch,
	RevalidationResult,
	RouteState,
	RouteUpdateReason,
} from "../core/types.ts";

export type {
	APIRouteKind,
	BuildSkewDetectedEvent,
	ClientLoaderKnownMatch,
	RevalidationResult,
	RouteState,
	RouteUpdateReason,
	WorkState,
};

type Core4Identity<Name extends string> = string & {
	readonly __core4_identity: Name;
};

export type Core4Token = Core4Identity<"Core4Token">;
export type PublicCallID = Core4Identity<"PublicCallID">;
export type BrowserKey = Core4Identity<"BrowserKey">;
export type SubmissionKey = string;
export type TimerID = Core4Identity<"TimerID">;

export type BrowserPosition = {
	href: string;
	key: BrowserKey;
	state: unknown;
};

export type Core4ScrollState = { hash: string } | { x: number; y: number };

export type RouteSnapshot = {
	position: BrowserPosition;
	route: RouteState;
	sequence: number;
};

export type RoutePayload = {
	css_bundles: readonly string[];
	deps: readonly string[];
	meta_head_els: readonly unknown[];
	params: Record<string, string>;
	rest_head_els: readonly unknown[];
	routes: readonly RoutePayloadMatch[];
	server_build_id: string;
	splat_values: readonly string[];
	title?: string;
};

export type RoutePayloadMatch = {
	input: unknown;
	loader_data: unknown;
	module_url: string;
	pattern: string;
	server_error?: unknown;
};

export type Core4ResponseHeaders = {
	get(name: string): string | null;
};

export type Core4ResponseFacts = {
	headers: Core4ResponseHeaders;
	ok: boolean;
	raw_response?: unknown;
	redirected: boolean;
	status: number;
	status_text: string;
	url: string;
};

export type BuildSkewResponseFacts = {
	ok: boolean;
	server_build_id: string;
	status: number;
};

export type BuildSkewDefaultBehavior =
	BuildSkewDetectedEvent["defaultBehavior"];

export type BuildSkewTriggeringResponse =
	BuildSkewDetectedEvent["triggeringResponse"];

export type BuildSkewNotification = BuildSkewDetectedEvent;

export type PreparedRoute = {
	route: RouteState;
	css_bundles: readonly string[];
	deps: readonly string[];
	render_payload: unknown;
};

export type Core4ClientLoaderServerState = {
	clientBuildID: string;
	loaderData: unknown;
	matches: Array<{
		input: unknown;
		loaderData: unknown;
		pattern: string;
	}>;
	outermostServerError: null | {
		error: unknown;
		idx: number;
	};
};

export type Core4ClientLoader = (args: {
	historyState: unknown;
	href: string;
	input: unknown;
	knownMatches: ClientLoaderKnownMatch[];
	loaderData: unknown;
	params: Record<string, string>;
	pattern: string;
	serverPromise: Promise<Core4ClientLoaderServerState>;
	signal: AbortSignal;
	splatValues: string[];
	trigger: "boot" | "navigation" | "prefetch" | "revalidation";
}) => Promise<unknown>;

export type RouteFetchTrigger =
	| "boot"
	| "navigation"
	| "popstate"
	| "prefetch"
	| "revalidation";

export type ActiveRouteKind =
	| "boot"
	| "navigation"
	| "popstate"
	| "revalidation";

export type ActiveRoutePhase = "fetching" | "preparing" | "publishing";

export type NavigationSource = "navigate" | "popstate" | "redirect";

type ActiveRouteBase = {
	browser_key: BrowserKey;
	href: string;
	phase: ActiveRoutePhase;
	redirect_count: number;
	sequence: number;
	state: unknown;
	token: Core4Token;
};

export type BootActiveRouteSlot = ActiveRouteBase & {
	kind: "boot";
	public_call_ids: readonly [];
	replace: true;
	restored_scroll?: Core4ScrollState;
	scroll_to_top: true;
	skip_work_indicator: true;
	source: null;
};

export type NavigationActiveRouteSlot = ActiveRouteBase & {
	kind: "navigation";
	public_call_ids: readonly PublicCallID[];
	replace: boolean;
	restored_scroll?: Core4ScrollState;
	scroll_to_top?: boolean;
	skip_work_indicator: boolean;
	source: Exclude<NavigationSource, "popstate">;
};

export type PopstateActiveRouteSlot = ActiveRouteBase & {
	kind: "popstate";
	public_call_ids: readonly PublicCallID[];
	replace: true;
	restored_scroll?: Core4ScrollState;
	scroll_to_top: true;
	skip_work_indicator: true;
	source: "popstate";
};

export type RevalidationActiveRouteSlot = ActiveRouteBase & {
	kind: "revalidation";
	public_call_ids: readonly [];
	replace: true;
	scroll_to_top: false;
	skip_work_indicator: boolean;
	source: null;
};

export type ActiveRouteSlot =
	| BootActiveRouteSlot
	| NavigationActiveRouteSlot
	| PopstateActiveRouteSlot
	| RevalidationActiveRouteSlot;

export type PrefetchSlot =
	| {
			href: string;
			phase: "fetching" | "preparing";
			token: Core4Token;
	  }
	| {
			href: string;
			phase: "prepared";
			prepared: PreparedRoute;
			token: Core4Token;
	  };

export type RevalidationReason =
	| "apiRequest"
	| "manual"
	| "retry"
	| "windowFocus";

export type RefreshWaiter = {
	id: PublicCallID;
};

export type RefreshDemand = {
	after_sequence: number;
	reason: RevalidationReason;
	skip_work_indicator: boolean;
	waiters: readonly RefreshWaiter[];
};

export type RefreshSlot =
	| { kind: "idle" }
	| {
			demand: RefreshDemand;
			kind: "debouncing";
			timer_id: TimerID;
	  }
	| {
			attempt: number;
			demand: RefreshDemand;
			kind: "pending";
	  }
	| {
			attempt: number;
			demand: RefreshDemand;
			kind: "retrying";
			timer_id: TimerID;
	  }
	| {
			attempt: number;
			demand: RefreshDemand;
			kind: "running";
	  }
	| {
			demand: RefreshDemand;
			kind: "settling";
	  };

export type SubmissionSlot = {
	dedupe_key: string | null;
	href: string;
	key: SubmissionKey;
	method: string;
	refresh_waiter_id?: PublicCallID;
	route_kind: APIRouteKind;
	should_revalidate: boolean;
	skip_work_indicator: boolean;
	token: Core4Token;
};

export type DeferredAPIRedirect = {
	href: string;
};

export type PublicationReason = RouteUpdateReason;

export type PublicationSlot = {
	did_navigate: boolean;
	phase: "publishing" | "committed";
	plan: PublicationPlan;
	public_call_ids: readonly PublicCallID[];
	token: Core4Token;
};

type RunningRefreshSlot = Extract<RefreshSlot, { kind: "running" }>;
type SettlingRefreshSlot = Extract<RefreshSlot, { kind: "settling" }>;
type NonRunningRefreshSlot = Exclude<RefreshSlot, RunningRefreshSlot>;
type NonRunningSettlingRefreshSlot = Exclude<
	RefreshSlot,
	RunningRefreshSlot | SettlingRefreshSlot
>;
type PublishingPublicationSlot = PublicationSlot & { phase: "publishing" };
type CommittedPublicationSlot = PublicationSlot & { phase: "committed" };

type Core4OwnerSlots =
	| {
			active_route: RevalidationActiveRouteSlot;
			publication: PublishingPublicationSlot | null;
			refresh: RunningRefreshSlot;
	  }
	| {
			active_route: ActiveRouteSlot | null;
			publication: CommittedPublicationSlot;
			refresh: NonRunningRefreshSlot;
	  }
	| {
			active_route: ActiveRouteSlot;
			publication: PublishingPublicationSlot;
			refresh: NonRunningSettlingRefreshSlot;
	  }
	| {
			active_route: ActiveRouteSlot | null;
			publication: null;
			refresh: NonRunningSettlingRefreshSlot;
	  };

type Core4ModelBase = Core4OwnerSlots & {
	client_build_id: string;
	deferred_api_redirect: DeferredAPIRedirect | null;
	prefetch: PrefetchSlot | null;
	sequence: number;
	submissions: Readonly<Record<Core4Token, SubmissionSlot | undefined>>;
	use_view_transitions: boolean;
};

export type Core4BootingModel = Core4ModelBase & {
	browser: BrowserPosition | null;
	current: RouteSnapshot | null;
	phase: "booting";
};

export type Core4ReadyModel = Core4ModelBase & {
	browser: BrowserPosition;
	current: RouteSnapshot;
	phase: "ready";
};

export type Core4Model = Core4BootingModel | Core4ReadyModel;

export type PublicNavigationResult = {
	didNavigate: boolean;
};

export type PublicationHistoryAction =
	| {
			href: string;
			kind: "push" | "replace";
			state: unknown;
	  }
	| {
			kind: "none";
	  };

export type PublicationScrollPlan =
	| {
			kind: "apply";
			scroll: Core4ScrollState;
			target_route_id: string;
	  }
	| {
			kind: "none";
	  };

export type PublicationHookPlan =
	| {
			kind: "none";
	  }
	| {
			kind: "run";
			trigger: Exclude<PublicationReason, "boot">;
	  };

export type Core4Effect =
	| {
			href: string;
			type: "hard_redirect";
	  }
	| {
			notification: BuildSkewNotification;
			type: "notify_build_skew";
	  }
	| {
			type: "abort_route_work";
			token: Core4Token;
	  }
	| {
			type: "release_route_work";
			token: Core4Token;
	  }
	| {
			type: "abort_api_submission";
			token: Core4Token;
	  }
	| {
			type: "release_api_submission";
			token: Core4Token;
	  }
	| {
			href: string;
			method: string;
			request_init?: RequestInit;
			route_kind: APIRouteKind;
			token: Core4Token;
			type: "fetch_api";
	  }
	| {
			client_build_id: string;
			href: string;
			token: Core4Token;
			trigger: RouteFetchTrigger;
			type: "fetch_route";
	  }
	| {
			history_state: unknown;
			href: string;
			payload: RoutePayload;
			target: "active_route" | "prefetch";
			token: Core4Token;
			trigger: RouteFetchTrigger;
			type: "prepare_route";
	  }
	| {
			plan: PublicationPlan;
			type: "publish_route";
	  }
	| {
			ids: readonly PublicCallID[];
			result: PublicNavigationResult;
			type: "settle_navigation_calls";
	  }
	| {
			ids: readonly PublicCallID[];
			result: RevalidationResult;
			type: "settle_refresh_calls";
	  }
	| {
			result: APISubmissionResult;
			token: Core4Token;
			type: "settle_api_submission";
	  }
	| {
			id: TimerID;
			ms: number;
			type: "start_refresh_timer";
	  }
	| {
			id: TimerID;
			type: "clear_refresh_timer";
	  }
	| {
			scroll: Core4ScrollState;
			type: "apply_scroll";
	  }
	| {
			history: PublicationHistoryAction;
			position: BrowserPosition;
			type: "apply_history";
	  };

export type PublicationPlan = {
	history: PublicationHistoryAction;
	hooks: PublicationHookPlan;
	next: PreparedRoute;
	position: BrowserPosition;
	previous: RouteSnapshot | null;
	reason: PublicationReason;
	route_sequence: number;
	save_current_scroll: boolean;
	scroll: PublicationScrollPlan;
	token: Core4Token;
	use_view_transition: boolean;
};

export type PublicationOptions = {
	did_navigate?: boolean;
	hooks?: PublicationHookPlan;
	save_current_scroll?: boolean;
	scroll?: PublicationScrollPlan;
	use_view_transition?: boolean;
};

export type Core4Transition<Kind extends string = string> = {
	effects: readonly Core4Effect[];
	kind: Kind;
	model: Core4Model;
};

type Core4InitBase = {
	client_build_id: string;
	deferred_api_redirect?: DeferredAPIRedirect | null;
	use_view_transitions?: boolean;
};

export type Core4BootingInit = Core4InitBase & {
	browser?: BrowserPosition | null;
	current?: RouteSnapshot | null;
	phase?: "booting";
};

export type Core4ReadyInit = Core4InitBase & {
	browser: BrowserPosition;
	current: RouteSnapshot;
	phase: "ready";
};

export type Core4Init = Core4BootingInit | Core4ReadyInit;

export type Core4WorkProjection =
	| {
			kind: "navigation";
			skip_work_indicator: boolean;
	  }
	| {
			kind: "revalidation";
			skip_work_indicator: boolean;
	  }
	| {
			kind: "apiRequest";
			skip_work_indicator: boolean;
	  }
	| {
			kind: "prefetch";
	  };

export type BootRequest = {
	browser: BrowserPosition;
	payload: RoutePayload;
	restored_scroll?: Core4ScrollState;
	token: Core4Token;
};

export type NavigationRequest = {
	browser_key: BrowserKey;
	href: string;
	public_call_ids: readonly PublicCallID[];
	restored_scroll?: Core4ScrollState;
	replace: boolean;
	scroll_to_top?: boolean;
	skip_work_indicator: boolean;
	source?: NavigationSource;
	state: unknown;
	token: Core4Token;
};

export type NavigationStartTransition =
	| Core4Transition<"hard_redirect">
	| Core4Transition<"same_document">
	| Core4Transition<"started">
	| Core4Transition<"promoted_prefetch">;

export type PopstateRequest = {
	browser: BrowserPosition;
	public_call_ids: readonly PublicCallID[];
	restored_scroll?: Core4ScrollState;
	token: Core4Token;
};

export type PrefetchRequest = {
	href: string;
	token: Core4Token;
};

export type RouteBuildSkewReport = {
	default_behavior: BuildSkewDefaultBehavior;
	requested_href: string;
	response: BuildSkewResponseFacts;
};

export type RouteResponseOwner =
	| {
			active_route: ActiveRouteSlot;
			kind: "active_route";
	  }
	| {
			kind: "prefetch";
			prefetch: PrefetchSlot;
	  }
	| {
			kind: "stale";
	  };

export type RouteResponseOwnerKind = RouteResponseOwner["kind"];

type RouteResponseOutcomeBase = {
	build_skew_report?: RouteBuildSkewReport;
	token: Core4Token;
};

type OwnedRouteResponseOutcomeBase = RouteResponseOutcomeBase & {
	owner_kind: Exclude<RouteResponseOwnerKind, "stale">;
};

export type RouteResponseOutcome =
	| (RouteResponseOutcomeBase & {
			kind: "ignored_stale";
			owner_kind: "stale";
	  })
	| (OwnedRouteResponseOutcomeBase &
			(
				| {
						behavior: "drop" | "reload";
						default_behavior: BuildSkewDefaultBehavior;
						href: string;
						kind: "build_skew";
						response: BuildSkewResponseFacts;
				  }
				| {
						href: string;
						kind: "soft_redirect";
				  }
				| {
						kind: "failed";
						retryable: boolean;
				  }
				| {
						kind: "data";
						payload: RoutePayload;
				  }
			));

export type RouteResponseClassificationInput = {
	owner: RouteResponseOwner;
	payload?: RoutePayload;
	requested_href: string;
	response: Core4ResponseFacts;
	token: Core4Token;
};

export type RouteResponseTransition =
	| Core4Transition<"ignored_stale">
	| Core4Transition<"preparing_active_route">
	| Core4Transition<"preparing_prefetch">
	| Core4Transition<"redirecting">
	| Core4Transition<"failed">
	| Core4Transition<"build_skew">;

export type RoutePreparationOutcome =
	| {
			kind: "aborted";
			token: Core4Token;
	  }
	| {
			kind: "failed";
			retryable: boolean;
			token: Core4Token;
	  }
	| {
			kind: "prepared";
			prepared: PreparedRoute;
			token: Core4Token;
	  };

export type RoutePreparationTransition =
	| Core4Transition<"prefetch_prepared">
	| Core4Transition<"publishing">
	| RouteResponseTransition;

export type PublicationCommit = {
	token: Core4Token;
};

export type RevalidationRequest = {
	reason: RevalidationReason;
	skip_work_indicator: boolean;
	timer_id?: TimerID;
	waiter_id?: PublicCallID;
};

export type RevalidationStartRequest = {
	token: Core4Token;
};

export type APISubmissionRequest = {
	dedupe_key: string | null;
	href: string;
	key: SubmissionKey;
	method: string;
	refresh_waiter_id?: PublicCallID;
	request_init?: RequestInit;
	route_kind: APIRouteKind;
	should_revalidate: boolean;
	skip_work_indicator: boolean;
	token: Core4Token;
};

export type APISubmissionResult =
	| {
			data: unknown;
			response?: unknown;
			success: true;
	  }
	| {
			error: string;
			response?: unknown;
			success: false;
	  };

export type APISubmissionBuildSkewReport = {
	default_behavior: BuildSkewDefaultBehavior;
	response: BuildSkewResponseFacts;
};

type APISubmissionOutcomeBase = {
	build_skew_report?: APISubmissionBuildSkewReport;
	token: Core4Token;
};

export type APISubmissionOutcome =
	| (APISubmissionOutcomeBase & {
			kind: "ignored_stale";
	  })
	| (APISubmissionOutcomeBase &
			(
				| {
						data: unknown;
						kind: "success";
						response?: unknown;
				  }
				| {
						error: string;
						kind: "http_error";
						response?: unknown;
				  }
				| {
						dispatched: boolean;
						kind: "aborted";
				  }
				| {
						dispatched: boolean;
						error: string;
						kind: "network_error";
				  }
				| {
						href: string;
						kind: "hard_redirect";
						response?: unknown;
				  }
				| {
						browser_key: BrowserKey;
						href: string;
						kind: "soft_redirect";
						navigation_token: Core4Token;
						response?: unknown;
						state: unknown;
				  }
				| {
						error: string;
						kind: "invalid_redirect";
						response?: unknown;
				  }
			));

export type APIResponseClassificationInput = {
	browser_key: BrowserKey;
	data: unknown;
	navigation_token: Core4Token;
	requested_href: string;
	response: Core4ResponseFacts;
	state: unknown;
	submission: SubmissionSlot | null;
	token: Core4Token;
};

export type APISubmissionTransition =
	| Core4Transition<"deferred_redirect">
	| Core4Transition<"hard_redirect">
	| Core4Transition<"ignored_stale">
	| Core4Transition<"rejected_cross_origin">
	| Core4Transition<"replaced">
	| Core4Transition<"settled">
	| Core4Transition<"soft_redirect">
	| Core4Transition<"started">;

export type DeferredAPIRedirectRequest = {
	browser_key: BrowserKey;
	state: unknown;
	token: Core4Token;
};

export type Core4TestOptions = {
	hard_redirect?: (url: string) => void;
	reload?: () => void;
	scroll_to?: (x: number, y: number) => void;
};

export type Deferred<T> = {
	promise: Promise<T>;
	resolve: (value: T) => void;
};

export type Core4APIResult<T> =
	| {
			data: T;
			response: Response;
			revalidationPromise: Promise<RevalidationResult>;
			success: true;
	  }
	| {
			error: string;
			response?: Response;
			revalidationPromise: Promise<RevalidationResult>;
			success: false;
	  };

export type Core4ClientLoaderPrefetch = {
	abort: () => void;
	pattern: string;
	resolve_server_state: (server_state: Core4ClientLoaderServerState) => void;
	result_promise: Promise<unknown>;
};

export type Core4PreparedRenderPayload = {
	css_bundles: readonly string[];
	deps: readonly string[];
	meta_head_els: readonly HeadEl[];
	render_state: RouteRenderState;
	rest_head_els: readonly HeadEl[];
	title: string | undefined;
};

export type Core4FetchResult =
	| {
			kind: "data";
			payload: RoutePayload;
			response: Response;
	  }
	| {
			kind: "failed";
			response?: Response;
	  };

export type Core4WorkIndicatorController = {
	configure: (options: WorkIndicatorOptions | undefined) => void;
	indicator: WorkIndicator;
	set_vorma_active: (active: boolean) => void;
};
