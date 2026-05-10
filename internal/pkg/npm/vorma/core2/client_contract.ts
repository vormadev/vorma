import type { Result } from "vorma/kit/result";
import type {
	APIRouteKind,
	BeforeRouteCommitFn,
	BeforeRouteYieldFn,
	RevalidationResult,
	RouteErrorState,
	RouteState,
	RouteUpdateReason,
} from "../core/types.ts";

export type ScrollState = { x: number; y: number } | { hash: string };

export type ScrollIntent = {
	scroll: ScrollState;
	target_route_id: string;
};

export type RouteRenderEntry = {
	client_loader_data: unknown;
	input: unknown;
	loader_data: unknown;
	module: Record<string, unknown>;
	module_url: string;
	pattern: string;
};

export type RouteRenderState = {
	client_build_id: string;
	entries: RouteRenderEntry[];
	error: RouteErrorState | null;
	history_state: unknown;
	params: Record<string, string>;
	splat_values: string[];
};

export type HeadEl = {
	attributesKnownSafe: Record<string, string>;
	booleanAttributes?: string[] | null;
	dangerousInnerHTML?: string;
	tag: string;
};

export type WorkState = {
	apiRequests: Array<{
		href: string;
		key: string;
		method: string;
	}>;
	navigation: null | {
		href: string;
		replace: boolean;
		source: "navigate" | "popstate" | "redirect";
	};
	prefetch: null | {
		href: string;
	};
	revalidation: null | {
		attempt: number;
		status: "debouncing" | "retrying" | "running";
	};
};

export const work_activity_kind = {
	api_request: "api_request",
	navigation: "navigation",
	prefetch: "prefetch",
	revalidation: "revalidation",
} as const;

export type WorkActivity = Array<
	| {
			kind: typeof work_activity_kind.api_request;
			skip_work_indicator: boolean;
	  }
	| {
			kind: typeof work_activity_kind.navigation;
			skip_work_indicator: boolean;
	  }
	| {
			kind: typeof work_activity_kind.prefetch;
	  }
	| {
			kind: typeof work_activity_kind.revalidation;
			skip_work_indicator: boolean;
	  }
>;

export type RevalidationReason =
	| "apiRequest"
	| "manual"
	| "retry"
	| "windowFocus";

export type BuildSkewDetectedEvent = {
	activeClientBuildID: string;
	currentRouteState: RouteState;
	currentWorkState: WorkState;
	defaultBehavior: "dropResponse" | "hardReload" | "notifyOnly";
	serverBuildID: string;
	triggeringResponse:
		| {
				kind: "apiRoute";
				apiRouteKind: APIRouteKind;
				method: string;
				ok: boolean;
				requestedHref: string;
				status: number;
		  }
		| {
				kind: "route";
				ok: boolean;
				requestedHref: string;
				status: number;
				trigger: "navigation" | "popstate" | "prefetch";
		  }
		| {
				kind: "route";
				ok: boolean;
				requestedHref: string;
				revalidationReason: RevalidationReason;
				status: number;
				trigger: "revalidation";
		  };
};

export type WorkIndicator = {
	isActive: () => boolean;
	track: <T>(promise: PromiseLike<T>) => Promise<T>;
};

export type WorkIndicatorOptions = {
	skipAPIRequests?: boolean;
	skipNavigations?: boolean;
	skipRevalidations?: boolean;
	start: () => void;
	startDelayMS?: number;
	stop: () => void;
	stopDelayMS?: number;
};

export type ClientOptions = {
	defaultErrorBoundary?: (props: { error: unknown }) => unknown;
	onBuildSkewDetected?: (event: BuildSkewDetectedEvent) => void;
	onRouteUpdate?: (
		route: RouteState,
		previousRoute: RouteState | null,
		reason: RouteUpdateReason,
	) => void;
	onWorkUpdate?: (work: WorkState) => void;
	render?: () => void | Promise<void>;
	revalidateOnWindowFocus?: boolean | { staleTimeMS: number };
	useViewTransitions?: boolean;
	workIndicator?: WorkIndicatorOptions;
};

export type ClientCommit = {
	route_render?: {
		scroll_intent?: ScrollIntent;
		state: RouteRenderState;
	};
	route_update?: {
		previous_route: RouteState | null;
		reason: RouteUpdateReason;
		route: RouteState;
	};
	work?: WorkState;
	work_activity?: WorkActivity;
};

export type ClientLoaderFn = (args: {
	current?: RouteState;
	href: string;
	historyState: unknown;
	input: unknown;
	knownMatches: Array<{
		input: unknown;
		pattern: string;
	}>;
	params: Record<string, string>;
	pattern: string;
	serverPromise: Promise<unknown>;
	signal: AbortSignal;
	splatValues: string[];
	trigger: "boot" | "navigation" | "prefetch" | "revalidation";
}) => Promise<unknown>;

export type ViewDefinition = {
	before_route_commit?: BeforeRouteCommitFn;
	before_route_yield?: BeforeRouteYieldFn;
	client_loader?: ClientLoaderFn;
	component: (props: any) => any;
	error_boundary?: (props: { error: unknown }) => any;
	pattern: string;
};

export type APIResult<T> =
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

export type NavigateOptions = {
	replace?: boolean;
	scrollToTop?: boolean;
	skipWorkIndicator?: boolean;
	state?: unknown;
};

export type SubmitOptions = {
	apiRouteKind?: APIRouteKind;
	dedupeKey?: string;
	revalidate?: boolean;
	skipWorkIndicator?: boolean;
};

export type ClientCore = {
	boot: (options: ClientOptions) => Promise<Result<void>>;
	defineView: <T = any>(input: {
		beforeRouteCommit?: BeforeRouteCommitFn;
		beforeRouteYield?: BeforeRouteYieldFn;
		clientLoader?: (props: any) => Promise<T>;
		component: (props: any) => any;
		errorBoundary?: (props: { error: unknown }) => any;
		pattern: string;
		runClientLoaderOnHMR?: boolean;
	}) => ViewDefinition & { __phantom_client_loader_data?: T };
	getClientBuildID: () => string;
	getRootEl: () => HTMLElement;
	getRouteState: () => RouteState;
	getWorkState: () => WorkState;
	get_default_error_boundary: () =>
		| ((props: { error: unknown }) => any)
		| undefined;
	navigate: (
		href: string | URL,
		options?: NavigateOptions,
	) => Promise<{ didNavigate: boolean }>;
	revalidate: () => Promise<RevalidationResult>;
	save_current_scroll: () => void;
	start_prefetch: (href: string) => void;
	stop_prefetch: (href: string) => void;
	submit_inner: <T = unknown>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: SubmitOptions,
	) => Promise<APIResult<T>>;
	workIndicator: WorkIndicator;
};
