import type { RouteState, RouteUpdateReason } from "../core/types.ts";
import type {
	BuildSkewDetectedEvent,
	ClientCommit,
	RouteRenderState,
	ScrollState,
} from "./client_contract.ts";
import type {
	APIResultData,
	BrowserPosition,
	ClientEvent,
	HistoryWriteKind,
	OperationID,
	PreparedHMRRoute,
	PreparedRoute,
	RouteFetchTrigger,
	RoutePayloadData,
	RoutePrepareTrigger,
	RouteResponseData,
} from "./model.ts";

export type BootPayloadResult = {
	browser: BrowserPosition;
	payload: RoutePayloadData;
	reload_scroll?: ScrollState;
};

export type RouteFetchRequest = {
	client_build_id: string;
	deployment_id: string;
	current_route: RouteState | null;
	history_state: unknown;
	href: string;
	operation_id: OperationID;
	signal: AbortSignal;
	trigger: RouteFetchTrigger;
};

export type RoutePrepareRequest = {
	client_build_id: string;
	current_route: RouteState | null;
	history_state: unknown;
	href: string;
	on_provisional_route?: (prepared: PreparedRoute) => void;
	operation_id: OperationID;
	payload: unknown;
	signal: AbortSignal;
	trigger: RoutePrepareTrigger;
};

export type RouteHookRequest = {
	current_render: RouteRenderState | null;
	current_route: RouteState | null;
	prepared: PreparedRoute;
	signal: AbortSignal;
	trigger: RouteUpdateReason;
};

export type HMRRoutePrepareRequest = {
	module: Record<string, unknown>;
	module_url: string;
	position: BrowserPosition;
	render: RouteRenderState;
	rerun_client_loader: boolean;
	route: RouteState;
	signal: AbortSignal;
};

export type APIFetchRequest = {
	deployment_id: string;
	href: string;
	method: string;
	request_init?: RequestInit;
	signal: AbortSignal;
};

export type HistoryWrite = {
	href: string;
	key: string;
	kind: HistoryWriteKind;
	state: unknown;
};

export type TimerHandle = unknown;

export type BrowserListenerHandlers = {
	on_focus: (event: BrowserFocusEvent) => void;
	on_hmr_update: (event: BrowserHMREvent) => void;
	on_popstate: (event: BrowserPopstateEvent) => void;
};

export type BrowserFocusEvent = {
	now_ms: number;
};

export type BrowserPopstateEvent = {
	browser: BrowserPosition;
	leaving_scroll: ScrollState;
	scroll?: ScrollState;
};

export type BrowserHMREvent = {
	module: Record<string, unknown>;
	module_url: string;
	now_ms: number;
};

export type CoreHost = {
	apply_route_dom: (prepared: PreparedRoute) => void | Promise<void>;
	clear_timer: (timer: TimerHandle) => void;
	commit: (commit: ClientCommit) => void;
	fetch_api: (request: APIFetchRequest) => Promise<APIResultData>;
	fetch_route: (request: RouteFetchRequest) => Promise<RouteResponseData>;
	hard_redirect: (href: string) => void;
	install_browser_listeners: (
		handlers: BrowserListenerHandlers,
	) => () => void;
	now_ms: () => number;
	prepare_hmr_route: (
		request: HMRRoutePrepareRequest,
	) => Promise<PreparedHMRRoute>;
	prepare_route: (request: RoutePrepareRequest) => Promise<PreparedRoute>;
	read_boot_payload: (
		fallback_browser_key: string,
	) => Promise<BootPayloadResult>;
	render: () => void | Promise<void>;
	report_unhandled_error: (cause: unknown) => void;
	report_build_skew: (event: BuildSkewDetectedEvent) => void;
	run_route_hooks: (request: RouteHookRequest) => Promise<PreparedRoute>;
	run_view_transition: (publish: () => Promise<void>) => Promise<void>;
	save_current_scroll: () => void;
	save_scroll_position: (key: string, scroll: ScrollState) => void;
	set_timer: (delay_ms: number, callback: () => void) => TimerHandle;
	write_history: (write: HistoryWrite) => void;
};

export type PublicCallRegistry = {
	reject: (public_call_id: string, cause: unknown) => void;
	resolve: (public_call_id: string, result: unknown) => void;
};

export type CommandDispatch = (event: ClientEvent) => void;
