import type {
	APIRouteKind,
	RouteState,
	RouteUpdateReason,
} from "../core/types.ts";
import type {
	BuildSkewDetectedEvent,
	ClientCommit,
	ClientOptions,
	HeadEl,
	RevalidationReason,
	RouteRenderState,
	ScrollIntent,
	ScrollState,
} from "./client_contract.ts";

export const client_phase = {
	idle: "idle",
	booting: "booting",
	ready: "ready",
} as const;

export const client_error_message = {
	aborted: "Aborted",
	not_booted: "Vorma not booted.",
} as const;

export const route_operation_kind = {
	boot: "boot",
	navigation: "navigation",
	popstate: "popstate",
	revalidation: "revalidation",
} as const;

export const route_fetch_trigger = {
	navigation: "navigation",
	popstate: "popstate",
	prefetch: "prefetch",
	revalidation: "revalidation",
} as const;

export const event_type = {
	api_response_failed: "api_response_failed",
	api_response_received: "api_response_received",
	api_submit_requested: "api_submit_requested",
	boot_failed: "boot_failed",
	boot_payload_read: "boot_payload_read",
	boot_requested: "boot_requested",
	hmr_preparation_failed: "hmr_preparation_failed",
	hmr_prepared: "hmr_prepared",
	hmr_update_observed: "hmr_update_observed",
	navigation_requested: "navigation_requested",
	popstate_observed: "popstate_observed",
	prefetch_canceled: "prefetch_canceled",
	prefetch_requested: "prefetch_requested",
	refresh_timer_fired: "refresh_timer_fired",
	revalidation_requested: "revalidation_requested",
	route_hooks_completed: "route_hooks_completed",
	route_hooks_failed: "route_hooks_failed",
	route_preparation_failed: "route_preparation_failed",
	route_prepared: "route_prepared",
	route_provisionally_prepared: "route_provisionally_prepared",
	route_response_failed: "route_response_failed",
	route_response_received: "route_response_received",
	view_defined: "view_defined",
	view_transition_completed: "view_transition_completed",
	window_focus_observed: "window_focus_observed",
} as const;

export const command_type = {
	abort_operation: "abort_operation",
	apply_route_dom: "apply_route_dom",
	commit: "commit",
	fetch_api: "fetch_api",
	fetch_route: "fetch_route",
	hard_redirect: "hard_redirect",
	install_browser_listeners: "install_browser_listeners",
	prepare_hmr_route: "prepare_hmr_route",
	prepare_route: "prepare_route",
	read_boot_payload: "read_boot_payload",
	reject_public_call: "reject_public_call",
	render: "render",
	report_build_skew: "report_build_skew",
	resolve_public_call: "resolve_public_call",
	run_route_hooks: "run_route_hooks",
	run_view_transition: "run_view_transition",
	save_current_scroll: "save_current_scroll",
	save_scroll_position: "save_scroll_position",
	start_timer: "start_timer",
	write_history: "write_history",
} as const;

export const public_call_kind = {
	boot: "boot",
	navigation: "navigation",
	revalidation: "revalidation",
	submit: "submit",
} as const;

export const history_write_kind = {
	push: "push",
	replace: "replace",
} as const;

export const navigation_source = {
	navigate: "navigate",
	popstate: "popstate",
	redirect: "redirect",
} as const;

export const refresh_status = {
	debouncing: "debouncing",
	idle: "idle",
	pending: "pending",
	retrying: "retrying",
	running: "running",
} as const;

export const api_result_kind = {
	failure: "failure",
	redirect: "redirect",
	success: "success",
} as const;

export const route_response_kind = {
	build_skew: "build_skew",
	data: "data",
	error: "error",
	redirect: "redirect",
} as const;

export const revalidation_reason = {
	api_request: "apiRequest",
	manual: "manual",
	retry: "retry",
	window_focus: "windowFocus",
} as const;

export const revalidation_result_reason = {
	build_skew: "build_skew",
	max_retries_exhausted: "max_retries_exhausted",
} as const;

export const build_skew_default_behavior = {
	drop_response: "dropResponse",
	hard_reload: "hardReload",
	notify_only: "notifyOnly",
} as const;

export const build_skew_trigger_kind = {
	api_route: "apiRoute",
	route: "route",
} as const;

export const route_limit = {
	max_redirects: 10,
} as const;

export const refresh_limit = {
	backoff_base_ms: 500,
	backoff_cap_ms: 30000,
	debounce_ms: 8,
	max_retries: 8,
} as const;

export const focus_revalidation_limit = {
	default_stale_ms: 5000,
} as const;

export type ClientPhase = (typeof client_phase)[keyof typeof client_phase];

export type RouteOperationKind =
	(typeof route_operation_kind)[keyof typeof route_operation_kind];

export type RouteFetchTrigger =
	(typeof route_fetch_trigger)[keyof typeof route_fetch_trigger];

export type RoutePrepareTrigger =
	| RouteUpdateReason
	| typeof route_fetch_trigger.prefetch;

export type PublicCallKind =
	(typeof public_call_kind)[keyof typeof public_call_kind];

export type HistoryWriteKind =
	(typeof history_write_kind)[keyof typeof history_write_kind];

export type NavigationSource =
	(typeof navigation_source)[keyof typeof navigation_source];

export type OperationID = string;

export type PublicCallID = string;

export type TimerID = string;

export type SubmissionID = string;

export type BrowserPosition = {
	href: string;
	key: string;
	state: unknown;
};

export type RouteSnapshot = {
	position: BrowserPosition;
	provisional: boolean;
	route: RouteState;
	render: RouteRenderState;
	scroll_intent?: ScrollIntent;
};

export type ViewRecord = {
	pattern: string;
	run_client_loader_on_hmr: boolean;
};

export type RouteOperation = {
	boot_scroll?: ScrollState;
	browser_key: string;
	id: OperationID;
	kind: RouteOperationKind;
	href: string;
	public_call_ids: PublicCallID[];
	redirect_count: number;
	replace: boolean;
	popstate_scroll?: ScrollState;
	scroll_to_top?: boolean;
	source: NavigationSource;
	state: unknown;
	skip_work_indicator: boolean;
};

export type PrefetchOperation = {
	id: OperationID;
	href: string;
	is_pending: boolean;
	prepared?: PreparedRoute;
};

export type RefreshDemand = {
	after_operation_id?: OperationID;
	operation_id: OperationID;
	reason: RevalidationReason;
	public_call_ids: PublicCallID[];
	skip_work_indicator: boolean;
};

export type RefreshState =
	| {
			kind: typeof refresh_status.idle;
	  }
	| {
			kind: typeof refresh_status.debouncing;
			demand: RefreshDemand;
			timer_id: TimerID;
	  }
	| {
			kind: typeof refresh_status.pending;
			attempt: number;
			demand: RefreshDemand;
	  }
	| {
			kind: typeof refresh_status.retrying;
			attempt: number;
			demand: RefreshDemand;
			timer_id: TimerID;
	  }
	| {
			kind: typeof refresh_status.running;
			attempt: number;
			demand: RefreshDemand;
			operation_id: OperationID;
	  };

export type SubmissionRecord = {
	id: SubmissionID;
	api_route_kind: APIRouteKind;
	href: string;
	method: string;
	operation_id: OperationID;
	public_call_id: PublicCallID;
	redirect_browser_key: string;
	redirect_operation_id: OperationID;
	revalidate: boolean;
	revalidation_operation_id: OperationID;
	revalidation_public_call_id?: PublicCallID;
	skip_work_indicator: boolean;
};

export type DeferredAPIRedirect = {
	browser_key: string;
	href: string;
	operation_id: OperationID;
	skip_work_indicator: boolean;
};

export type FocusRevalidationState = {
	last_activity_ms: number | null;
	stale_ms: number;
};

export type HMROperation = {
	id: OperationID;
	module_url: string;
};

export type ClientState = {
	active_route_operation: RouteOperation | null;
	browser: BrowserPosition | null;
	client_build_id: string;
	current: RouteSnapshot | null;
	default_error_boundary_enabled: boolean;
	deferred_api_redirect: DeferredAPIRedirect | null;
	deployment_id: string;
	focus_revalidation: FocusRevalidationState | null;
	hmr_operation: HMROperation | null;
	phase: ClientPhase;
	prefetch: PrefetchOperation | null;
	refresh: RefreshState;
	registered_views: Record<string, ViewRecord>;
	submissions: Record<SubmissionID, SubmissionRecord>;
	use_view_transitions: boolean;
	view_transition_operation_id: OperationID | null;
};

export type BootOptions = Pick<
	ClientOptions,
	"useViewTransitions" | "defaultErrorBoundary" | "revalidateOnWindowFocus"
>;

export type NavigationRequestOptions = {
	replace: boolean;
	scroll_to_top?: boolean;
	skip_work_indicator: boolean;
	state: unknown;
};

export type RoutePayloadData = {
	client_build_id: string;
	deployment_id: string;
	raw: unknown;
};

export type RouteDomPatch = {
	css_bundles: string[];
	deps: string[];
	meta_head_els: HeadEl[];
	rest_head_els: HeadEl[];
	title: string | undefined;
};

export type PreparedRoute = {
	dom: RouteDomPatch;
	render: RouteRenderState;
	route: RouteState;
	scroll_intent?: ScrollIntent;
};

export type PreparedHMRRoute = {
	render: RouteRenderState;
	route: RouteState;
};

export type RouteResponseData =
	| {
			kind: typeof route_response_kind.data;
			ok: boolean;
			payload: unknown;
			server_build_id: string;
			status: number;
	  }
	| {
			kind: typeof route_response_kind.error;
			ok: boolean;
			server_build_id: string;
			status: number;
			status_text: string;
	  }
	| {
			kind: typeof route_response_kind.redirect;
			hard: boolean;
			href: string;
			http: boolean;
			ok: boolean;
			server_build_id: string;
			status: number;
	  }
	| {
			kind: typeof route_response_kind.build_skew;
			ok: boolean;
			server_build_id: string;
			status: number;
	  };

export type APIResultData =
	| {
			kind: typeof api_result_kind.success;
			data: unknown;
			ok: boolean;
			response: Response;
			server_build_id: string;
			status: number;
	  }
	| {
			kind: typeof api_result_kind.failure;
			error: string;
			ok?: boolean;
			response?: Response;
			server_build_id?: string;
			should_revalidate?: boolean;
			status?: number;
	  }
	| {
			kind: typeof api_result_kind.redirect;
			hard: boolean;
			href: string;
			ok: boolean;
			response: Response;
			server_build_id: string;
			status: number;
	  };

export type ClientEvent =
	| {
			type: typeof event_type.boot_requested;
			browser_key: string;
			operation_id: OperationID;
			public_call_id: PublicCallID;
			options: BootOptions;
	  }
	| {
			type: typeof event_type.boot_payload_read;
			browser: BrowserPosition;
			operation_id: OperationID;
			payload: RoutePayloadData;
			reload_scroll?: ScrollState;
	  }
	| {
			type: typeof event_type.boot_failed;
			cause: unknown;
			operation_id: OperationID;
	  }
	| {
			type: typeof event_type.view_defined;
			pattern: string;
			run_client_loader_on_hmr: boolean;
	  }
	| {
			type: typeof event_type.view_transition_completed;
			operation_id: OperationID;
	  }
	| {
			type: typeof event_type.hmr_update_observed;
			module: Record<string, unknown>;
			module_url: string;
			now_ms: number;
			operation_id: OperationID;
	  }
	| {
			type: typeof event_type.hmr_prepared;
			operation_id: OperationID;
			prepared: PreparedHMRRoute;
	  }
	| {
			type: typeof event_type.hmr_preparation_failed;
			cause: unknown;
			operation_id: OperationID;
	  }
	| {
			type: typeof event_type.window_focus_observed;
			now_ms: number;
			operation_id: OperationID;
	  }
	| {
			type: typeof event_type.navigation_requested;
			browser_key: string;
			href: string;
			operation_id: OperationID;
			options: NavigationRequestOptions;
			public_call_id: PublicCallID;
	  }
	| {
			type: typeof event_type.popstate_observed;
			browser: BrowserPosition;
			leaving_scroll: ScrollState;
			operation_id: OperationID;
			popstate_scroll?: ScrollState;
	  }
	| {
			type: typeof event_type.route_response_received;
			operation_id: OperationID;
			response: RouteResponseData;
	  }
	| {
			type: typeof event_type.route_response_failed;
			cause: unknown;
			operation_id: OperationID;
	  }
	| {
			type: typeof event_type.route_prepared;
			now_ms: number;
			operation_id: OperationID;
			prepared: PreparedRoute;
	  }
	| {
			type: typeof event_type.route_provisionally_prepared;
			operation_id: OperationID;
			prepared: PreparedRoute;
	  }
	| {
			type: typeof event_type.route_preparation_failed;
			cause: unknown;
			operation_id: OperationID;
	  }
	| {
			type: typeof event_type.route_hooks_completed;
			now_ms: number;
			operation_id: OperationID;
			prepared: PreparedRoute;
	  }
	| {
			type: typeof event_type.route_hooks_failed;
			cause: unknown;
			operation_id: OperationID;
	  }
	| {
			type: typeof event_type.revalidation_requested;
			debounce: boolean;
			operation_id: OperationID;
			public_call_id?: PublicCallID;
			reason: RevalidationReason;
			skip_work_indicator: boolean;
	  }
	| {
			type: typeof event_type.refresh_timer_fired;
			operation_id?: OperationID;
			timer_id: TimerID;
	  }
	| {
			type: typeof event_type.api_submit_requested;
			api_route_kind: APIRouteKind;
			href: string;
			method: string;
			operation_id: OperationID;
			public_call_id: PublicCallID;
			redirect_browser_key: string;
			redirect_operation_id: OperationID;
			revalidate: boolean;
			revalidation_operation_id: OperationID;
			revalidation_public_call_id?: PublicCallID;
			request_init?: RequestInit;
			skip_work_indicator: boolean;
			submission_id: SubmissionID;
	  }
	| {
			type: typeof event_type.api_response_received;
			operation_id: OperationID;
			result: APIResultData;
			submission_id: SubmissionID;
	  }
	| {
			type: typeof event_type.api_response_failed;
			cause: unknown;
			operation_id: OperationID;
			submission_id: SubmissionID;
	  }
	| {
			type: typeof event_type.prefetch_requested;
			href: string;
			operation_id: OperationID;
	  }
	| {
			type: typeof event_type.prefetch_canceled;
			href: string;
	  };

export type ClientCommand =
	| {
			type: typeof command_type.abort_operation;
			operation_id: OperationID;
	  }
	| {
			type: typeof command_type.apply_route_dom;
			prepared: PreparedRoute;
	  }
	| {
			type: typeof command_type.commit;
			commit: ClientCommit;
	  }
	| {
			type: typeof command_type.fetch_api;
			deployment_id: string;
			href: string;
			method: string;
			operation_id: OperationID;
			request_init?: RequestInit;
			submission_id: SubmissionID;
	  }
	| {
			type: typeof command_type.fetch_route;
			client_build_id: string;
			deployment_id: string;
			current_route: RouteState | null;
			history_state: unknown;
			href: string;
			operation_id: OperationID;
			trigger: RouteFetchTrigger;
	  }
	| {
			type: typeof command_type.hard_redirect;
			href: string;
	  }
	| {
			type: typeof command_type.install_browser_listeners;
	  }
	| {
			type: typeof command_type.prepare_hmr_route;
			module: Record<string, unknown>;
			module_url: string;
			operation_id: OperationID;
			position: BrowserPosition;
			render: RouteRenderState;
			rerun_client_loader: boolean;
			route: RouteState;
	  }
	| {
			type: typeof command_type.prepare_route;
			client_build_id: string;
			current_route: RouteState | null;
			history_state: unknown;
			href: string;
			operation_id: OperationID;
			payload: unknown;
			trigger: RoutePrepareTrigger;
	  }
	| {
			type: typeof command_type.read_boot_payload;
			fallback_browser_key: string;
			operation_id: OperationID;
	  }
	| {
			type: typeof command_type.reject_public_call;
			cause: unknown;
			public_call_id: PublicCallID;
	  }
	| {
			type: typeof command_type.report_build_skew;
			event: BuildSkewDetectedEvent;
	  }
	| {
			type: typeof command_type.render;
	  }
	| {
			type: typeof command_type.resolve_public_call;
			public_call_id: PublicCallID;
			result: unknown;
	  }
	| {
			type: typeof command_type.run_route_hooks;
			current_render: RouteRenderState | null;
			current_route: RouteState | null;
			operation_id: OperationID;
			prepared: PreparedRoute;
			trigger: RouteUpdateReason;
	  }
	| {
			type: typeof command_type.run_view_transition;
			commands: ClientCommand[];
			operation_id: OperationID;
			public_call_ids: PublicCallID[];
	  }
	| {
			type: typeof command_type.save_current_scroll;
	  }
	| {
			type: typeof command_type.save_scroll_position;
			key: string;
			scroll: ScrollState;
	  }
	| {
			type: typeof command_type.start_timer;
			delay_ms: number;
			operation_id?: OperationID;
			timer_id: TimerID;
	  }
	| {
			type: typeof command_type.write_history;
			href: string;
			key: string;
			kind: HistoryWriteKind;
			state: unknown;
	  };

export type UpdateResult = {
	commands: ClientCommand[];
	state: ClientState;
};
