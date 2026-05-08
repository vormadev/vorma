import type { Result } from "vorma/kit/result";
import type {
	APIRouteKind,
	AppConfig,
	BeforeRouteCommitFn,
	BeforeRouteYieldFn,
	RevalidationResult,
	RouteErrorState,
	RouteState,
	RouteUpdateReason,
} from "../types.ts";

export type ScrollState = { x: number; y: number } | { hash: string };

export type ScrollIntent = {
	scroll: ScrollState;
	target_route_id: string;
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

export type WorkState = {
	navigation: null | {
		href: string;
		replace: boolean;
		source: "navigate" | "popstate" | "redirect";
	};
	revalidation: null | {
		status: "debouncing" | "running" | "retrying";
		attempt: number;
	};
	prefetch: null | {
		href: string;
	};
	apiRequests: Array<{
		key: string;
		method: string;
		href: string;
	}>;
};

export type RevalidationReason =
	| "manual"
	| "retry"
	| "apiRequest"
	| "windowFocus";

export type BuildSkewDetectedEvent = {
	activeClientBuildID: string;
	serverBuildID: string;
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
				kind: "apiRoute";
				apiRouteKind: APIRouteKind;
				requestedHref: string;
				method: string;
				status: number;
				ok: boolean;
		  };
	currentRouteState: RouteState;
	currentWorkState: WorkState;
	defaultBehavior: "dropResponse" | "hardReload" | "notifyOnly";
};

export type WorkIndicator = {
	track: <T>(promise: PromiseLike<T>) => Promise<T>;
	isActive: () => boolean;
};

export type WorkIndicatorOptions = {
	show: () => void;
	hide: () => void;
	showDelayMS?: number;
	hideDelayMS?: number;
	skipNavigations?: boolean;
	skipAPIRequests?: boolean;
	skipRevalidations?: boolean;
};

export type ClientOptions = {
	render?: () => void | Promise<void>;
	workIndicator?: WorkIndicatorOptions;
	revalidateOnWindowFocus?: boolean | { staleTimeMS: number };
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

export type CommitFn = (commit: ClientCommit) => void;

export type ViewDefinition = {
	pattern: string;
	component: (props: any) => any;
	error_boundary?: (props: { error: unknown }) => any;
	client_loader?: ClientLoaderFn;
	before_route_commit?: BeforeRouteCommitFn;
	before_route_yield?: BeforeRouteYieldFn;
};

export type ClientLoaderFn = (args: {
	trigger: "boot" | "navigation" | "revalidation" | "prefetch";
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
	clientBuildID: string;
	matches: Array<{
		pattern: string;
		input: unknown;
		loaderData: unknown;
	}>;
	outermostServerError: null | {
		idx: number;
		error: unknown;
	};
	loaderData: unknown;
};

export type APIResult<T> =
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
			skipworkIndicator?: boolean;
		},
	) => Promise<{ didNavigate: boolean }>;
	revalidate: () => Promise<RevalidationResult>;
	submit_inner: <T = unknown>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: {
			apiRouteKind?: APIRouteKind;
			dedupeKey?: string;
			revalidate?: boolean;
			skipworkIndicator?: boolean;
		},
	) => Promise<APIResult<T>>;
	getRouteState: () => RouteState;
	getWorkState: () => WorkState;
	getClientBuildID: () => string;
	getRootEl: () => HTMLElement;
	defineView: <T = any>(input: {
		pattern: string;
		component: (props: any) => any;
		errorBoundary?: (props: { error: unknown }) => any;
		clientLoader?: (props: any) => Promise<T>;
		beforeRouteCommit?: BeforeRouteCommitFn;
		beforeRouteYield?: BeforeRouteYieldFn;
		runClientLoaderOnHMR?: boolean;
	}) => ViewDefinition & { __phantom_client_loader_data?: T };
	start_prefetch: (href: string) => void;
	stop_prefetch: (href: string) => void;
	save_current_scroll: () => void;
	get_default_error_boundary: () =>
		| ((props: { error: unknown }) => any)
		| undefined;
};

export type TestOptions = {
	reload?: () => void;
	hard_redirect?: (url: string) => void;
	scroll_to?: (x: number, y: number) => void;
};

export type ClientCoreFactory = (
	app_config: Omit<AppConfig, "__vormaViews" | "__vormaAPIRoutes">,
	commit: CommitFn,
	test_options?: TestOptions,
) => Result<ClientCore>;
