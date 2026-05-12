import type { Result } from "vorma/kit/result";
import type {
	APIRouteKind,
	ClientCommitPlan,
	NavigationSource,
	PreparedRoute,
	PublicNavigationResult,
	RevalidationReason,
	RevalidationResult,
	RoutePayload,
	RouteState,
	ScrollState,
	WorkIndicatorPolicy,
} from "./model.ts";
import type { AbortHandle } from "./platform.ts";

/////////////////////////////////////////////////////////////////////
/////// Identities
/////////////////////////////////////////////////////////////////////

declare const route_token_brand: unique symbol;
export type RouteToken = string & { readonly [route_token_brand]: true };

declare const api_token_brand: unique symbol;
export type APIToken = string & { readonly [api_token_brand]: true };

declare const refresh_timer_brand: unique symbol;
export type RefreshTimerID = string & { readonly [refresh_timer_brand]: true };

declare const public_call_brand: unique symbol;
export type PublicCallID = string & { readonly [public_call_brand]: true };

declare const browser_key_brand: unique symbol;
export type BrowserKey = string & { readonly [browser_key_brand]: true };

declare const client_loader_brand: unique symbol;
export type ClientLoaderID = string & {
	readonly [client_loader_brand]: true;
};

/////////////////////////////////////////////////////////////////////
/////// Inputs
/////////////////////////////////////////////////////////////////////

export type PublicNavigateInput = {
	type: "public_navigate";
	call_id: PublicCallID;
	href: string;
	replace: boolean;
	scroll_to_top: boolean | undefined;
	state: unknown;
	skip_work_indicator: boolean;
};

export type PublicRevalidateInput = {
	type: "public_revalidate";
	call_id: PublicCallID;
};

export type PublicSubmitInput = {
	type: "public_submit";
	call_id: PublicCallID;
	href: string;
	method: string;
	request_init: RequestInit | undefined;
	route_kind: APIRouteKind;
	dedupe_key: string | undefined;
	should_revalidate: boolean;
	skip_work_indicator: boolean;
};

export type PublicPrefetchStartInput = {
	type: "public_prefetch_start";
	href: string;
};

export type PublicPrefetchStopInput = {
	type: "public_prefetch_stop";
	href: string;
};

export type BootInput = {
	type: "boot";
	payload: RoutePayload;
	browser_key: BrowserKey;
	browser_state: unknown;
	href: string;
	restored_scroll: ScrollState | undefined;
	options: {
		use_view_transitions: boolean;
		revalidate_on_focus: {
			stale_time_ms: number;
			skip_work_indicator: boolean;
		} | null;
		work_indicator: WorkIndicatorPolicy | null;
	};
};

export type PopstateInput = {
	type: "popstate";
	browser_key: BrowserKey;
	href: string;
	state: unknown;
	restored_scroll: ScrollState | undefined;
	previous_scroll_save:
		| { key: BrowserKey; position: { x: number; y: number } }
		| undefined;
};

export type FocusInput = {
	type: "focus";
	now_ms: number;
};

export type BeforeUnloadInput = {
	type: "before_unload";
	scroll: { x: number; y: number };
	now_ms: number;
	href: string;
};

export type RouteFetchSettledInput = {
	type: "route_fetch_settled";
	token: RouteToken;
	outcome: RouteFetchOutcome;
};

export type RouteFetchOutcome =
	| {
			kind: "data";
			payload: RoutePayload;
			server_build_id: string;
			redirect: { href: string; hard: boolean } | null;
			ok: boolean;
			status: number;
			has_build_skew_header: boolean;
	  }
	| {
			kind: "build_skew";
			server_build_id: string;
			redirect: { href: string; hard: boolean } | null;
			status: number;
			ok: boolean;
	  }
	| {
			kind: "redirect_only";
			redirect: { href: string; hard: boolean };
			server_build_id: string;
			status: number;
			ok: boolean;
	  }
	| { kind: "http_error"; status: number; server_build_id: string }
	| { kind: "network_error"; error: string }
	| { kind: "aborted" };

export type RoutePreparationSettledInput = {
	type: "route_preparation_settled";
	token: RouteToken;
	outcome: RoutePreparationOutcome;
};

export type RoutePreparationOutcome =
	| { kind: "prepared"; prepared: PreparedRoute }
	| { kind: "failed"; error: string }
	| { kind: "aborted" };

export type APIFetchSettledInput = {
	type: "api_fetch_settled";
	token: APIToken;
	outcome: APIFetchOutcome;
};

export type APIFetchOutcome =
	| {
			kind: "ok";
			data: unknown;
			server_build_id: string;
			redirect: { href: string; hard: boolean } | null;
			status: number;
	  }
	| {
			kind: "redirect";
			redirect: { href: string; hard: boolean };
			server_build_id: string;
			status: number;
			ok: boolean;
	  }
	| {
			kind: "http_error";
			status: number;
			status_text: string;
			server_build_id: string;
	  }
	| { kind: "network_error"; error: string; dispatched: boolean }
	| { kind: "aborted"; dispatched: boolean };

export type RefreshTimerFiredInput = {
	type: "refresh_timer_fired";
	id: RefreshTimerID;
};

export type PublicationCommittedInput = {
	type: "publication_committed";
	token: RouteToken;
};

export type PublicationFailedInput = {
	type: "publication_failed";
	token: RouteToken;
	error: string;
};

export type HMRRouteUpdateInput = {
	type: "hmr_route_update";
	route_sequence: number;
	match_idx: number;
	module_url: string;
	client_loader_data: unknown;
};

export type InputEvent =
	| PublicNavigateInput
	| PublicRevalidateInput
	| PublicSubmitInput
	| PublicPrefetchStartInput
	| PublicPrefetchStopInput
	| BootInput
	| PopstateInput
	| FocusInput
	| BeforeUnloadInput
	| RouteFetchSettledInput
	| RoutePreparationSettledInput
	| APIFetchSettledInput
	| RefreshTimerFiredInput
	| PublicationCommittedInput
	| PublicationFailedInput
	| HMRRouteUpdateInput;

/////////////////////////////////////////////////////////////////////
/////// Effects
/////////////////////////////////////////////////////////////////////

export type HistoryAction =
	| { kind: "push"; href: string; state: unknown; browser_key: BrowserKey }
	| {
			kind: "replace";
			href: string;
			state: unknown;
			browser_key: BrowserKey;
	  }
	| { kind: "none" };

export type FetchRouteEffect = {
	type: "fetch_route";
	token: RouteToken;
	url: string;
	abort_handle: AbortHandle;
	is_revalidation: boolean;
	client_build_id: string;
	deployment_id: string;
	trigger: "boot" | "navigation" | "popstate" | "prefetch" | "revalidation";
};

export type PrepareRouteEffect = {
	type: "prepare_route";
	token: RouteToken;
	payload: RoutePayload;
	client_build_id: string;
	href: string;
	history_state: unknown;
	abort_handle: AbortHandle;
	trigger: "boot" | "navigation" | "revalidation" | "prefetch";
};

export type FetchAPIEffect = {
	type: "fetch_api";
	token: APIToken;
	url: string;
	method: string;
	request_init: RequestInit | undefined;
	abort_handle: AbortHandle;
	deployment_id: string;
	route_kind: APIRouteKind;
};

export type AbortEffect = { type: "abort"; handle: AbortHandle };

export type StartTimerEffect = {
	type: "start_timer";
	id: RefreshTimerID;
	ms: number;
};

export type ClearTimerEffect = { type: "clear_timer"; id: RefreshTimerID };

export type PublishRouteEffect = {
	type: "publish_route";
	token: RouteToken;
	abort_handle: AbortHandle | null;
	commit: ClientCommitPlan;
	history_action: HistoryAction;
	save_current_scroll: boolean;
	scroll: ScrollState | null;
	apply_dom_side_effects: {
		title: string | undefined;
		meta_head_els: readonly unknown[];
		rest_head_els: readonly unknown[];
		css_bundles: readonly string[];
		deps: readonly string[];
	} | null;
	use_view_transition: boolean;
	run_yield_hooks_for: readonly ClientLoaderID[];
	run_commit_hooks_for: readonly ClientLoaderID[];
	hook_trigger: "navigation" | "popstate" | "revalidation";
};

export type ApplyScrollEffect = { type: "apply_scroll"; scroll: ScrollState };

export type ApplyHistoryEffect = {
	type: "apply_history";
	action: HistoryAction;
};

export type SaveScrollEffect = {
	type: "save_scroll";
	browser_key: BrowserKey;
	position: { x: number; y: number };
};

export type SaveCurrentScrollEffect = {
	type: "save_current_scroll";
	browser_key: BrowserKey;
};

export type WriteReloadScrollEffect = {
	type: "write_reload_scroll";
	href: string;
	position: { x: number; y: number };
	now_ms: number;
};

export type HardRedirectEffect = { type: "hard_redirect"; url: string };

export type ReloadEffect = { type: "reload" };

export type EmitClientCommitEffect = {
	type: "emit_client_commit";
	commit: ClientCommitPlan;
};

export type NotifyBuildSkewEffect = {
	type: "notify_build_skew";
	event: BuildSkewNotification;
};

export type NotifyWorkUpdateEffect = {
	type: "notify_work_update";
	work: WorkState;
};

export type SyncWorkIndicatorEffect = {
	type: "sync_work_indicator";
	should_be_active: boolean;
};

export type SettleNavigationCallEffect = {
	type: "settle_navigation_call";
	call_id: PublicCallID;
	result: PublicNavigationResult;
};

export type SettleRevalidateCallEffect = {
	type: "settle_revalidate_call";
	call_id: PublicCallID;
	result: RevalidationResult;
};

export type SettleSubmitCallEffect = {
	type: "settle_submit_call";
	call_id: PublicCallID;
	result: SubmitResult;
};

export type SettleBootEffect = {
	type: "settle_boot";
	result: Result<void>;
};

export type DispatchNavigationEffect = {
	type: "dispatch_navigation";
	href: string;
	replace: boolean;
	state: unknown;
};

export type WarnRedirectLoopEffect = {
	type: "warn_redirect_loop";
	url: string;
	redirect_count: number;
};

export type SubmitResult =
	| {
			success: true;
			data: unknown;
			revalidation_call_id: PublicCallID | null;
	  }
	| {
			success: false;
			error: string;
			revalidation_call_id: PublicCallID | null;
	  };

export type Effect =
	| FetchRouteEffect
	| PrepareRouteEffect
	| FetchAPIEffect
	| AbortEffect
	| StartTimerEffect
	| ClearTimerEffect
	| PublishRouteEffect
	| ApplyScrollEffect
	| ApplyHistoryEffect
	| SaveScrollEffect
	| SaveCurrentScrollEffect
	| WriteReloadScrollEffect
	| HardRedirectEffect
	| ReloadEffect
	| EmitClientCommitEffect
	| NotifyBuildSkewEffect
	| NotifyWorkUpdateEffect
	| SyncWorkIndicatorEffect
	| SettleNavigationCallEffect
	| SettleRevalidateCallEffect
	| SettleSubmitCallEffect
	| SettleBootEffect
	| DispatchNavigationEffect
	| WarnRedirectLoopEffect;

export type BuildSkewNotification = {
	activeClientBuildID: string;
	serverBuildID: string;
	triggeringResponse: BuildSkewTriggeringResponse;
	currentRouteState: RouteState;
	currentWorkState: WorkState;
};

export type BuildSkewTriggeringResponse =
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
			kind: "apiRoute";
			apiRouteKind: APIRouteKind;
			requestedHref: string;
			method: string;
			status: number;
			ok: boolean;
	  };

export type RouteUpdateReason =
	| "boot"
	| "navigation"
	| "popstate"
	| "revalidation";

export type WorkState = {
	navigation: null | {
		href: string;
		replace: boolean;
		source: NavigationSource;
	};
	revalidation: null | {
		status: "debouncing" | "running" | "retrying";
		attempt: number;
	};
	prefetch: null | { href: string };
	apiRequests: Array<{ key: string; method: string; href: string }>;
};

export type ReducerOutput<M> = {
	model: M;
	effects: readonly Effect[];
};
