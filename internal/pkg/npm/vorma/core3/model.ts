export type CorePhase = "booting" | "disposed" | "ready" | "unbooted";

export type OperationKind =
	| "api_submit"
	| "boot"
	| "navigation"
	| "popstate"
	| "route_prefetch"
	| "route_revalidation";

export type OperationStatus =
	| "aborted"
	| "classified"
	| "completed"
	| "failed"
	| "hooks_running"
	| "ignored_stale"
	| "prepared"
	| "publishable"
	| "published"
	| "publishing"
	| "requested"
	| "settled"
	| "started"
	| "superseded";

export type OperationRight =
	| "abort_effects"
	| "own_visible_transition"
	| "publish_route"
	| "satisfy_refresh_demand";

export type NavigationSource = "navigate" | "popstate" | "redirect";

export type RouteTrigger =
	| "boot"
	| "navigation"
	| "popstate"
	| "prefetch"
	| "revalidation";

export type VisibleRouteTrigger = Exclude<RouteTrigger, "prefetch">;

export type RefreshStatus =
	| "debouncing"
	| "idle"
	| "pending"
	| "retrying"
	| "running";

export type RevalidationReason =
	| "apiRequest"
	| "manual"
	| "retry"
	| "windowFocus";

export type BuildSkewBehavior = "drop" | "notify" | "reload" | "settle";

export const build_skew_default_behavior = {
	drop_response: "dropResponse",
	hard_reload: "hardReload",
	notify_only: "notifyOnly",
} as const;

export const build_skew_trigger_kind = {
	api_route: "apiRoute",
	route: "route",
} as const;

export type PublicationReason =
	| "boot"
	| "navigation"
	| "popstate"
	| "revalidation";

export type OperationID = string;

export type PublicCallID = string;

export type BrowserKey = string;

export type SubmissionKey = string;

export type ResourceKey = string;

export type TimerID = string;

export type BrowserPosition = {
	href: string;
	key: BrowserKey;
	state: unknown;
};

export type ScrollState = { hash: string } | { x: number; y: number };

export type ScrollIntent = {
	scroll: ScrollState;
	target_route_id: string;
};

export type RouteMatchFacts = {
	client_loader_data: unknown;
	input: unknown;
	loader_data: unknown;
	module: unknown;
	module_url: string;
	pattern: string;
};

export type RouteErrorFacts = {
	error: unknown;
	idx: number;
	source: "clientLoader" | "server";
};

export type RouteFacts = {
	client_build_id: string;
	error: RouteErrorFacts | null;
	history_state: unknown;
	href: string;
	matches: readonly RouteMatchFacts[];
	params: Readonly<Record<string, string>>;
	splat_values: readonly string[];
};

export type RouteRenderFacts = {
	client_build_id: string;
	entries: readonly RouteMatchFacts[];
	error: RouteErrorFacts | null;
	history_state: unknown;
	params: Readonly<Record<string, string>>;
	splat_values: readonly string[];
};

export type RouteMatchState = {
	clientLoaderData: unknown;
	input: unknown;
	loaderData: unknown;
	pattern: string;
};

export type RouteState = {
	clientBuildID: string;
	error: RouteErrorFacts | null;
	historyState: unknown;
	href: string;
	matches: RouteMatchState[];
	params: Record<string, string>;
	splatValues: string[];
};

export type RouteSnapshot = {
	position: BrowserPosition;
	provisional: boolean;
	render: RouteRenderFacts;
	route: RouteFacts;
	scroll_intent?: ScrollIntent;
};

export type OperationBase = {
	id: OperationID;
	public_call_ids: readonly PublicCallID[];
	rights: readonly OperationRight[];
	status: OperationStatus;
};

export type BootOperation = OperationBase & {
	browser_key: BrowserKey;
	href: string;
	kind: "boot";
	reload_scroll?: ScrollState;
	state: unknown;
};

export type NavigationOperation = OperationBase & {
	browser_key: BrowserKey;
	href: string;
	kind: "navigation";
	redirect_count: number;
	replace: boolean;
	scroll_to_top?: boolean;
	source: NavigationSource;
	state: unknown;
	skip_work_indicator: boolean;
};

export type PopstateOperation = OperationBase & {
	browser: BrowserPosition;
	kind: "popstate";
	leaving_scroll: ScrollState;
	popstate_scroll?: ScrollState;
};

export type RouteRevalidationOperation = OperationBase & {
	attempt: number;
	href: string;
	kind: "route_revalidation";
	reason: RevalidationReason;
};

export type RoutePrefetchOperation = OperationBase & {
	href: string;
	kind: "route_prefetch";
	prepared_resource_key?: ResourceKey;
};

export type APISubmitOperation = OperationBase & {
	href: string;
	kind: "api_submit";
	method: string;
	revalidate: boolean;
	submission_key: SubmissionKey;
};

export type CoreOperation =
	| APISubmitOperation
	| BootOperation
	| NavigationOperation
	| PopstateOperation
	| RoutePrefetchOperation
	| RouteRevalidationOperation;

export type RefreshDemand = {
	after_operation_id?: OperationID;
	operation_id: OperationID;
	public_call_ids: readonly PublicCallID[];
	reason: RevalidationReason;
	skip_work_indicator: boolean;
};

export type RefreshState =
	| {
			kind: "idle";
	  }
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
			operation_id: OperationID;
	  };

export type SubmissionRecordBase = {
	href: string;
	method: string;
	operation_id: OperationID;
	public_call_id: PublicCallID;
	skip_work_indicator: boolean;
	submission_key: SubmissionKey;
};

export type SubmissionRecord =
	| (SubmissionRecordBase & {
			revalidate: false;
			revalidation_operation_id?: undefined;
			revalidation_public_call_id?: undefined;
	  })
	| (SubmissionRecordBase & {
			revalidate: true;
			revalidation_operation_id: OperationID;
			revalidation_public_call_id?: PublicCallID;
	  });

export type FocusRevalidationState = {
	last_activity_ms: number | null;
	stale_ms: number;
};

export type WorkProjection = {
	apiRequests: ReadonlyArray<{
		href: string;
		key: string;
		method: string;
	}>;
	navigation: null | {
		href: string;
		replace: boolean;
		source: NavigationSource;
	};
	prefetch: null | {
		href: string;
	};
	revalidation: null | {
		attempt: number;
		status: "debouncing" | "retrying" | "running";
	};
};

export type WorkActivity = ReadonlyArray<
	| {
			kind: "api_request";
			skip_work_indicator: boolean;
	  }
	| {
			kind: "navigation";
			skip_work_indicator: boolean;
	  }
	| {
			kind: "prefetch";
	  }
	| {
			kind: "revalidation";
			skip_work_indicator: boolean;
	  }
>;

export type BuildSkewReport = {
	behavior: BuildSkewBehavior;
	ok: boolean;
	server_build_id: string;
	status: number;
};

export type BuildSkewDefaultBehavior =
	(typeof build_skew_default_behavior)[keyof typeof build_skew_default_behavior];

export type BuildSkewTriggeringResponse =
	| {
			kind: typeof build_skew_trigger_kind.route;
			ok: boolean;
			requestedHref: string;
			status: number;
			trigger: "navigation" | "popstate" | "prefetch";
	  }
	| {
			kind: typeof build_skew_trigger_kind.route;
			ok: boolean;
			requestedHref: string;
			revalidationReason: RevalidationReason;
			status: number;
			trigger: "revalidation";
	  }
	| {
			apiRouteKind: unknown;
			kind: typeof build_skew_trigger_kind.api_route;
			method: string;
			ok: boolean;
			requestedHref: string;
			status: number;
	  };

export type BuildSkewDetectedEvent = {
	activeClientBuildID: string;
	currentRouteState: RouteState;
	currentWorkState: WorkProjection;
	defaultBehavior: BuildSkewDefaultBehavior;
	serverBuildID: string;
	triggeringResponse: BuildSkewTriggeringResponse;
};

export type CoreModel = {
	active_route_operation_id: OperationID | null;
	browser: BrowserPosition | null;
	client_build_id: string;
	current: RouteSnapshot | null;
	deployment_id: string;
	focus_revalidation: FocusRevalidationState | null;
	last_work_projection: WorkProjection | null;
	operations: Readonly<Record<OperationID, CoreOperation>>;
	phase: CorePhase;
	prefetch_operation_id: OperationID | null;
	publication_owner_id: OperationID | null;
	refresh: RefreshState;
	submissions: Readonly<Record<SubmissionKey, SubmissionRecord>>;
	use_view_transitions: boolean;
};

export type RouteResponseFacts =
	| {
			kind: "build_skew";
			ok: boolean;
			server_build_id: string;
			status: number;
	  }
	| {
			kind: "data";
			ok: boolean;
			payload: unknown;
			server_build_id: string;
			status: number;
	  }
	| {
			kind: "error";
			ok: boolean;
			server_build_id: string;
			status: number;
			status_text: string;
	  }
	| {
			hard: boolean;
			href: string;
			http: boolean;
			kind: "redirect";
			ok: boolean;
			server_build_id: string;
			status: number;
	  };

export type NavigationRequestOutcome =
	| {
			href: string;
			kind: "hard_redirect";
	  }
	| {
			href: string;
			kind: "invalid_href";
	  }
	| {
			href: string;
			kind: "route_navigation";
	  }
	| {
			href: string;
			kind: "same_document";
	  };

export type RouteOutcome =
	| {
			kind: "stale";
			operation_id: OperationID;
	  }
	| {
			kind: "route_data";
			operation_id: OperationID;
			payload: unknown;
	  }
	| {
			hard: boolean;
			href: string;
			kind: "hard_redirect" | "soft_redirect";
			operation_id: OperationID;
	  }
	| {
			href: string;
			kind: "invalid_redirect";
			operation_id: OperationID;
	  }
	| {
			href: string;
			kind: "redirect_loop";
			operation_id: OperationID;
	  }
	| {
			behavior: BuildSkewBehavior;
			kind: "build_skew_drop" | "build_skew_reload" | "build_skew_settle";
			operation_id: OperationID;
			server_build_id: string;
	  }
	| {
			kind: "retryable_revalidation_failure";
			operation_id: OperationID;
			status_text: string;
	  }
	| {
			kind: "terminal_revalidation_failure";
			operation_id: OperationID;
			status_text: string;
	  }
	| {
			kind: "route_error";
			operation_id: OperationID;
			status_text: string;
	  }
	| {
			kind: "ignored_background_refresh";
			operation_id: OperationID;
	  };

export type APIResponseFacts =
	| {
			data: unknown;
			kind: "success";
			ok: boolean;
			response: unknown;
			server_build_id: string;
			status: number;
	  }
	| {
			dispatched: boolean;
			error: string;
			kind: "failure";
			ok?: boolean;
			response?: unknown;
			server_build_id?: string;
			should_revalidate?: boolean;
			status?: number;
	  }
	| {
			hard: boolean;
			href: string;
			http: boolean;
			kind: "redirect";
			ok: boolean;
			response: unknown;
			server_build_id: string;
			status: number;
	  };

export type APIOutcome =
	| {
			kind: "stale";
			operation_id: OperationID;
			submission_key: SubmissionKey;
	  }
	| {
			data: unknown;
			kind: "success";
			operation_id: OperationID;
			revalidation_required: boolean;
			response: unknown;
			submission_key: SubmissionKey;
	  }
	| {
			error: string;
			kind: "failure";
			operation_id: OperationID;
			revalidation_required: boolean;
			response?: unknown;
			submission_key: SubmissionKey;
	  }
	| {
			href: string;
			kind: "hard_redirect" | "soft_redirect";
			operation_id: OperationID;
			response: unknown;
			submission_key: SubmissionKey;
	  }
	| {
			kind: "revalidation_required";
			operation_id: OperationID;
			submission_key: SubmissionKey;
	  };

export type APISettlementResult =
	| {
			data: unknown;
			response: unknown;
			revalidation_public_call_id?: PublicCallID;
			success: true;
	  }
	| {
			error: string;
			response?: unknown;
			revalidation_public_call_id?: PublicCallID;
			success: false;
	  };

export type PreparedRoute = {
	dom: unknown;
	render: RouteRenderFacts;
	route: RouteFacts;
	scroll_intent?: ScrollIntent;
};

export type PreparationOutcome =
	| {
			kind: "stale";
			operation_id: OperationID;
	  }
	| {
			kind: "aborted";
			operation_id: OperationID;
	  }
	| {
			cause: unknown;
			kind: "failed";
			operation_id: OperationID;
	  }
	| {
			kind: "prepared";
			operation_id: OperationID;
			prepared: PreparedRoute;
	  };

export type RouteHooksOutcome =
	| {
			kind: "stale";
			operation_id: OperationID;
	  }
	| {
			kind: "aborted";
			operation_id: OperationID;
	  }
	| {
			cause: unknown;
			kind: "failed";
			operation_id: OperationID;
	  }
	| {
			kind: "publishable";
			operation_id: OperationID;
			prepared: PreparedRoute;
	  };

export type RouteHooksRequest = {
	current_route: RouteState;
	next_route: RouteState;
	prepared: PreparedRoute;
	trigger: Exclude<PublicationReason, "boot">;
};

export type PublicationTransaction = {
	after_transition: readonly CoreEffect[];
	before_transition: readonly CoreEffect[];
	inside_transition: readonly CoreEffect[];
	operation_id: OperationID;
	reason: PublicationReason;
};

export type RouteCommit = {
	route_render: {
		scroll_intent?: ScrollIntent;
		state: RouteRenderFacts;
	};
	route_update?: {
		previous_route: RouteState | null;
		reason: PublicationReason;
		route: RouteState;
	};
};

export type CoreEffect =
	| {
			fallback_browser_key: BrowserKey;
			operation_id: OperationID;
			type: "read_boot_payload";
	  }
	| {
			client_build_id: string;
			deployment_id?: string;
			href: string;
			operation_id: OperationID;
			trigger: RouteTrigger;
			type: "fetch_route";
	  }
	| {
			client_build_id: string;
			history_state: unknown;
			href: string;
			operation_id: OperationID;
			payload: unknown;
			trigger: VisibleRouteTrigger;
			type: "prepare_route";
	  }
	| {
			client_build_id: string;
			history_state: unknown;
			href: string;
			operation_id: OperationID;
			payload: unknown;
			trigger: "prefetch";
			type: "prepare_route";
	  }
	| {
			client_build_id: string;
			history_state: unknown;
			href: string;
			operation_id: OperationID;
			resource_key: ResourceKey;
			type: "promote_prefetch_route";
	  }
	| {
			operation_id: OperationID;
			request: RouteHooksRequest;
			type: "run_route_hooks";
	  }
	| {
			deployment_id?: string;
			href: string;
			method: string;
			operation_id: OperationID;
			request_init?: RequestInit;
			submission_key: SubmissionKey;
			type: "fetch_api";
	  }
	| {
			operation_id: OperationID;
			type: "abort_operation";
	  }
	| {
			delay_ms: number;
			operation_id?: OperationID;
			timer_id: TimerID;
			type: "start_timer";
	  }
	| {
			timer_id: TimerID;
			type: "clear_timer";
	  }
	| {
			type: "save_current_scroll";
	  }
	| {
			key: BrowserKey;
			scroll: ScrollState;
			type: "save_scroll_position";
	  }
	| {
			position: BrowserPosition;
			replace: boolean;
			type: "write_history";
	  }
	| {
			prepared: PreparedRoute;
			type: "apply_publication_dom";
	  }
	| {
			commit: RouteCommit;
			next: RouteSnapshot;
			operation_id: OperationID;
			type: "commit";
	  }
	| {
			type: "render";
	  }
	| {
			scroll: ScrollState;
			type: "apply_scroll";
	  }
	| {
			href: string;
			type: "hard_redirect";
	  }
	| {
			type: "reload";
	  }
	| {
			type: "install_browser_listeners";
	  }
	| {
			event: BuildSkewDetectedEvent;
			type: "report_build_skew";
	  }
	| {
			public_call_id: PublicCallID;
			result: unknown;
			type: "resolve_public_call";
	  }
	| {
			public_call_id: PublicCallID;
			result: APISettlementResult;
			type: "resolve_api_call";
	  }
	| {
			cause: unknown;
			public_call_id: PublicCallID;
			type: "reject_public_call";
	  }
	| {
			transaction: PublicationTransaction;
			type: "run_view_transition";
	  };
