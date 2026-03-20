/// <reference types="vite/client" />

import type { PatternRegistry } from "vorma/kit/matcher/register";

// ─── Utility ─────────────────────────────────────────────────────

export type ScrollState = { x: number; y: number } | { hash: string };

// ─── Events ──────────────────────────────────────────────────────

export type RouteChangeEventDetail = { __scrollState?: ScrollState };
export type RouteChangeEvent = CustomEvent<RouteChangeEventDetail>;
export type StatusEventDetail = {
	isNavigating: boolean;
	isSubmitting: boolean;
	isRevalidating: boolean;
};
export type StatusEvent = CustomEvent<StatusEventDetail>;

// ─── Submit ──────────────────────────────────────────────────────

export type SubmitOptions = {
	dedupeKey?: string;
	revalidate?: boolean;
	skipGlobalLoadingIndicator?: boolean;
};
export type SubmitResult<T> =
	| { success: true; data: T }
	| { success: false; error: string };

// ─── App / Route Phantom Types ───────────────────────────────────

export type VormaAppConfig = {
	actionsRouterMountRoot: string;
	actionsDynamicRune: string;
	actionsSplatRune: string;
	loadersDynamicRune: string;
	loadersSplatRune: string;
	loadersExplicitIndexSegmentIdentifier: string;
	__phantom?: unknown;
};

export type VormaRouteBase = {
	_type: string;
	pattern: string;
	params?: ReadonlyArray<string>;
	isSplat?: boolean;
	method?: string;
	[key: string]: unknown;
};

export type VormaAppBase = {
	routes: readonly VormaRouteBase[];
	appConfig: VormaAppConfig;
	rootData: unknown;
};

export type ExtractApp<C extends VormaAppConfig> =
	C["__phantom"] extends VormaAppBase ? C["__phantom"] : VormaAppBase;

type RouteByType<App extends VormaAppBase, T extends string> = Extract<
	App["routes"][number],
	{ _type: T }
>;
type RouteByPattern<Routes, P> = Extract<Routes, { pattern: P }>;

type VormaLoader<App extends VormaAppBase> = RouteByType<App, "loader">;
type VormaQuery<App extends VormaAppBase> = RouteByType<App, "query">;
type VormaMutation<App extends VormaAppBase> = RouteByType<App, "mutation">;

export type VormaLoaderPattern<App extends VormaAppBase> =
	VormaLoader<App>["pattern"];
export type VormaQueryPattern<App extends VormaAppBase> =
	VormaQuery<App>["pattern"];
export type VormaMutationPattern<App extends VormaAppBase> =
	VormaMutation<App>["pattern"];

export type VormaLoaderOutput<
	App extends VormaAppBase,
	P extends VormaLoaderPattern<App>,
> =
	RouteByPattern<VormaLoader<App>, P> extends { phantomOutputType: infer T }
		? T
		: null | undefined;

export type VormaQueryInput<
	App extends VormaAppBase,
	P extends VormaQueryPattern<App>,
> =
	RouteByPattern<VormaQuery<App>, P> extends { phantomInputType: infer T }
		? T
		: null | undefined;

export type VormaQueryOutput<
	App extends VormaAppBase,
	P extends VormaQueryPattern<App>,
> =
	RouteByPattern<VormaQuery<App>, P> extends { phantomOutputType: infer T }
		? T
		: null | undefined;

export type VormaMutationInput<
	App extends VormaAppBase,
	P extends VormaMutationPattern<App>,
> =
	RouteByPattern<VormaMutation<App>, P> extends { phantomInputType: infer T }
		? T
		: null | undefined;

export type VormaMutationOutput<
	App extends VormaAppBase,
	P extends VormaMutationPattern<App>,
> =
	RouteByPattern<VormaMutation<App>, P> extends { phantomOutputType: infer T }
		? T
		: null | undefined;

type VormaMutationMethod<
	App extends VormaAppBase,
	P extends VormaMutationPattern<App>,
> =
	RouteByPattern<VormaMutation<App>, P> extends { method: infer M }
		? M extends string
			? M
			: "POST"
		: "POST";

type RouteMetadata<App extends VormaAppBase, P extends string> = Extract<
	App["routes"][number],
	{ pattern: P }
>;

export type GetParams<App extends VormaAppBase, P extends string> =
	RouteMetadata<App, P> extends { params: ReadonlyArray<infer Params> }
		? Params extends string
			? Params
			: never
		: never;

export type ParamsForPattern<
	App extends VormaAppBase,
	P extends VormaLoaderPattern<App>,
> = GetParams<App, P>;

type HasParams<App extends VormaAppBase, P extends string> =
	GetParams<App, P> extends never ? false : true;

type IsSplat<App extends VormaAppBase, P extends string> =
	RouteMetadata<App, P> extends { isSplat: true } ? true : false;

type IsEmptyInput<T> = [T] extends [null | undefined] ? true : false;

type QueryInputContractViolation = {
	__queryInputContractViolation: "Query input root must be an object, null, or undefined.";
};
type EnforceQueryInputRootContract<Input> = [Input] extends [null | undefined]
	? Input
	: Input extends Record<string, unknown>
		? Input
		: QueryInputContractViolation;

type ConditionalParams<App extends VormaAppBase, P extends string> =
	HasParams<App, P> extends true
		? { params: { [K in GetParams<App, P>]: string } }
		: {};

type ConditionalSplat<App extends VormaAppBase, P extends string> =
	IsSplat<App, P> extends true ? { splatValues: Array<string> } : {};

export type PatternBasedProps<App extends VormaAppBase, P extends string> = {
	pattern: P;
} & ConditionalParams<App, P> &
	ConditionalSplat<App, P>;

type PermissiveLoaderPattern<
	App extends VormaAppBase,
	P extends VormaLoaderPattern<App>,
> = P extends `${infer Prefix}/${App["appConfig"]["loadersExplicitIndexSegmentIdentifier"]}`
	? P | (Prefix extends "" ? "/" : Prefix)
	: P;

export type PermissivePatternBasedProps<
	App extends VormaAppBase,
	P extends VormaLoaderPattern<App>,
> = {
	pattern: PermissiveLoaderPattern<App, P>;
} & ConditionalParams<App, P> &
	ConditionalSplat<App, P>;

// ─── Route Component Types ──────────────────────────────────────

export type VormaRoutePropsGeneric<
	JSXElement,
	App extends VormaAppBase,
	P extends VormaLoaderPattern<App>,
> = {
	idx: number;
	Outlet: (props: Record<string, unknown>) => JSXElement;
	__phantom_pattern: P;
} & Record<string, unknown>;

export type VormaRouteGeneric<
	JSXElement,
	App extends VormaAppBase,
	P extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = (props: VormaRoutePropsGeneric<JSXElement, App, P>) => JSXElement;

// ─── Link Types ─────────────────────────────────────────────────

export type VormaLinkPropsBase<LinkEvent = unknown> = {
	href?: string;
	prefetch?: "intent" | "none";
	prefetchDelayMs?: number;
	replace?: boolean;
	scrollToTop?: boolean;
	beforeBegin?: (event: LinkEvent) => void | Promise<void>;
	beforeRender?: (event: LinkEvent) => void | Promise<void>;
	afterRender?: (event: LinkEvent) => void | Promise<void>;
};

// ─── Router Data ────────────────────────────────────────────────

export type BaseRouterData<RootData, Params extends string> = {
	buildID: string;
	matchedPatterns: string[];
	splatValues: string[];
	params: Record<Params, string>;
	rootData: RootData;
};

export type UseRouterDataFunction<
	App extends VormaAppBase,
	UsesAccessor extends boolean = false,
> = {
	<P extends VormaLoaderPattern<App>>(
		props: VormaRoutePropsGeneric<unknown, App, P>,
	): UsesAccessor extends false
		? BaseRouterData<App["rootData"], ParamsForPattern<App, P>>
		: () => BaseRouterData<App["rootData"], ParamsForPattern<App, P>>;
	<P extends VormaLoaderPattern<App>>(): UsesAccessor extends false
		? BaseRouterData<App["rootData"], ParamsForPattern<App, P>>
		: () => BaseRouterData<App["rootData"], ParamsForPattern<App, P>>;
	(): UsesAccessor extends false
		? BaseRouterData<App["rootData"], string>
		: () => BaseRouterData<App["rootData"], string>;
};

// ─── Query / Mutation Props ─────────────────────────────────────

export type VormaQueryProps<
	App extends VormaAppBase,
	P extends VormaQueryPattern<App>,
> = PatternBasedProps<App, P> & {
	options?: SubmitOptions;
	requestInit?: Omit<RequestInit, "method"> & { method?: "GET" };
} & (IsEmptyInput<
		EnforceQueryInputRootContract<VormaQueryInput<App, P>>
	> extends true
		? {
				input?: EnforceQueryInputRootContract<VormaQueryInput<App, P>>;
			}
		: {
				input: EnforceQueryInputRootContract<VormaQueryInput<App, P>>;
			});

export type VormaMutationProps<
	App extends VormaAppBase,
	P extends VormaMutationPattern<App>,
> = PatternBasedProps<App, P> & {
	options?: SubmitOptions;
} & (VormaMutationMethod<App, P> extends "POST"
		? { requestInit?: Omit<RequestInit, "method"> & { method?: "POST" } }
		: {
				requestInit: RequestInit & {
					method: VormaMutationMethod<App, P>;
				};
			}) &
	(IsEmptyInput<VormaMutationInput<App, P>> extends true
		? { input?: VormaMutationInput<App, P> }
		: { input: VormaMutationInput<App, P> });

// ─── API Client Types ───────────────────────────────────────────

export type APIRequestInitOverrides = Omit<RequestInit, "method" | "body">;

export type APIRequestInitDecoratorContext<App extends VormaAppBase> =
	| {
			type: "query";
			pattern: VormaQueryPattern<App>;
			requestInit?: RequestInit;
			input?: unknown;
	  }
	| {
			type: "mutation";
			pattern: VormaMutationPattern<App>;
			requestInit?: RequestInit;
			input?: unknown;
	  };

export type APIRequestInitDecorator<App extends VormaAppBase> = (
	context: APIRequestInitDecoratorContext<App>,
) =>
	| APIRequestInitOverrides
	| undefined
	| Promise<APIRequestInitOverrides | undefined>;

export type TypedAPIClient<App extends VormaAppBase> = {
	query: <P extends VormaQueryPattern<App>>(
		props: VormaQueryProps<App, P>,
	) => Promise<SubmitResult<VormaQueryOutput<App, P>>>;
	mutate: <P extends VormaMutationPattern<App>>(
		props: VormaMutationProps<App, P>,
	) => Promise<SubmitResult<VormaMutationOutput<App, P>>>;
};

// ─── Client Loader Types ────────────────────────────────────────

export type ClientLoaderAwaitedServerData<RootData, LoaderData> = {
	matchedPatterns: string[];
	rootData: RootData;
	loaderData: LoaderData;
	buildID: string;
};

// ─── Navigate Options (public facing — camelCase) ───────────────

export type VormaNavigateOptions = {
	replace?: boolean;
	scrollToTop?: boolean;
	intent?: "navigate" | "prefetch" | "revalidate";
	skipGlobalLoadingIndicator?: boolean;
};

// ─── Init Options ───────────────────────────────────────────────

export type InitClientInput = {
	isDev?: boolean;
	publicPathPrefix?: string;
	vormaAppConfig?: VormaAppConfig;
	routeManifestURL?: string;
	renderFn?: () => void | Promise<void>;
	defaultErrorBoundary?: (props: { error: unknown }) => string;
	useViewTransitions?: boolean;
	rootElementID?: string;
};

// ─── Loading Indicator Config ───────────────────────────────────

export type GlobalLoadingIndicatorConfig = {
	start: () => void;
	stop: () => void;
	isRunning: () => boolean;
	include?: "all" | Array<"navigations" | "submissions" | "revalidations">;
	startDelayMS?: number;
	stopDelayMS?: number;
};

/////////////////

export type Nullable<T> = T | null | undefined;

// ─── Head Elements ──────────────────────────────────────────────

export type HeadEl = {
	tag: string;
	attributesKnownSafe: Record<string, string>;
	booleanAttributes?: Nullable<string[]>;
	dangerousInnerHTML?: string;
};

// ─── Navigation Artifacts ───────────────────────────────────────
// Ephemeral data produced by the server during a navigation that is
// consumed once (head updates, CSS insertion) and never persisted
// on the snapshot.

export type NavigationArtifacts = {
	title: string;
	meta_head_els: HeadEl[];
	rest_head_els: HeadEl[];
	css_bundles: string[];
};

// ─── Route Data JSON Payload (wire format — camelCase preserved) ─

export type RouteDataPayload = {
	outermostServerError?: unknown;
	outermostServerErrorIdx?: Nullable<number>;
	matchedPatterns?: string[];
	loadersData?: unknown[];
	importURLs?: string[];
	exportKeys?: string[];
	errorExportKeys?: string[];
	hasRootData?: boolean;
	params?: Record<string, string>;
	splatValues?: string[];
	title?: { dangerousInnerHTML: string } | null;
	metaHeadEls?: HeadEl[];
	restHeadEls?: HeadEl[];
	deps?: string[];
	cssBundles?: string[];
};

// ─── Runtime Route Snapshot ─────────────────────────────────────
// Persistent route state that drives rendering. Does NOT include
// ephemeral navigation artifacts (title, head elements, deps, CSS
// bundles) — those flow through NavigationArtifacts instead.

export type RuntimeRouteSnapshot = {
	outermost_server_error: unknown;
	outermost_server_error_idx: Nullable<number>;
	outermost_client_error: unknown;
	outermost_client_error_idx: Nullable<number>;
	outermost_error: unknown;
	outermost_error_idx: Nullable<number>;
	matched_patterns: string[];
	loaders_data: unknown[];
	import_urls: string[];
	export_keys: string[];
	error_export_keys: string[];
	has_root_data: boolean;
	params: Record<string, string>;
	splat_values: string[];
	build_id: string;
	root_element_id?: string;
	active_components: unknown[];
	active_error_boundary: unknown;
	client_loaders_data: unknown[];
};

// ─── Client Loader Internal ─────────────────────────────────────

export type ClientLoaderWaitFn = (props: {
	params: Record<string, string>;
	splatValues: string[];
	serverDataPromise: Promise<{
		matchedPatterns: string[];
		rootData: unknown;
		loaderData: unknown;
		buildID: string;
	}>;
	signal: AbortSignal;
}) => Promise<unknown>;

export type ComponentModulesMap = Map<string, Record<string, unknown>>;

// ─── Client Module Map ──────────────────────────────────────────

export type ClientModuleMapEntry = {
	import_url: string;
	export_key: string;
	error_export_key: string;
};

// ─── Navigation Internal ────────────────────────────────────────

export type NavigationIntent = "navigate" | "prefetch" | "revalidate";

export type NavigateProps = {
	href: string | URL;
	replace?: boolean;
	scroll_to_top?: boolean;
	intent?: NavigationIntent;
	skip_global_loading_indicator?: boolean;
	skip_history_commit?: boolean;
	current_href_for_classification?: string;
	redirect_hop_count?: number;
};

export type NavigationOperation = {
	id: number;
	intent: NavigationIntent;
	target_url: string;
	allow_trailing_revalidate_pass: boolean;
	abort_controller: AbortController;
	settled_promise: Promise<void>;
	notify_settled: () => void;
};

export type PrefetchCacheEntry = {
	target_data_key: string;
	target_url: string;
	build_id: string;
	route_data: RuntimeRouteSnapshot;
	artifacts?: NavigationArtifacts;
	modules_map?: ComponentModulesMap;
	prestarted_loaders?: Record<string, Promise<unknown>>;
};

export type NavigationStore = {
	navigate_op: NavigationOperation | null;
	revalidate_op: NavigationOperation | null;
	queued_revalidate_data_key: string | null;
	queued_revalidate_promise: Promise<void> | null;
	prefetch_ops: Map<string, NavigationOperation>;
	prefetch_cache: Map<string, PrefetchCacheEntry>;
	skipped_loading_nav_ids: Set<number>;
	skipped_loading_sub_ids: Set<number>;
	next_nav_op_id: number;
	next_sub_op_id: number;
	latest_started_sub_op_id: number;
	active_sub_ids: Set<number>;
	sub_abort_controllers: Map<number, AbortController>;
	sub_id_by_dedupe_key: Map<string, number>;
	last_status: StatusEventDetail;
	last_nav_or_revalidate_ts: number;
};

export type NavigationStateManager = {
	navigate: (props: NavigateProps) => Promise<{ didNavigate: boolean }>;
	submit: <T>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: SubmitOptions,
	) => Promise<SubmitResult<T>>;
	getStatus: () => StatusEventDetail;
	clearAll: () => void;
	get_store: () => NavigationStore;
};

// ─── Route Data Fetch Result ────────────────────────────────────

export type RouteDataFetchResult =
	| {
			status: "success";
			route_snapshot: RuntimeRouteSnapshot;
			artifacts: NavigationArtifacts;
			build_id: string;
	  }
	| {
			status: "redirect";
			href: string;
			build_id: string;
			is_hard_reload: boolean;
	  };

// ─── Global Client State ────────────────────────────────────────

export type VormaClientGlobal = {
	is_dev: boolean;
	public_path_prefix: string;
	is_touch_active: boolean;
	has_registered_input_modality_listeners: boolean;
	next_manifest_load_id: number;
	route_manifest: Record<string, unknown> | undefined;
	pattern_to_wait_fn: Record<string, ClientLoaderWaitFn>;
	default_error_boundary: (props: { error: unknown }) => string;
	use_view_transitions: boolean;
	deployment_id: string;
	app_config: VormaAppConfig;
	route_manifest_url: string;
	pattern_registry: PatternRegistry;
	client_module_map: Record<string, ClientModuleMapEntry>;
	nav_state_manager?: NavigationStateManager;
	popstate_registered: boolean;
	last_known_history_key?: string;
	last_known_history_href?: string;
	has_registered_beforeunload: boolean;
	window_listeners?: Map<string, Set<EventListener>>;
	hard_redirect_for_testing?: ((href: string) => void) | undefined;
	snapshot: RuntimeRouteSnapshot;
};

// ─── Adapter Store Types (used by UI adapters) ──────────────────

export type AdapterRouterData = BaseRouterData<unknown, string>;

export type AdapterStoreState = {
	loaders_data: unknown[];
	client_loaders_data: unknown[];
	router_data: AdapterRouterData;
	matched_patterns: string[];
	outermost_error: unknown;
	outermost_error_idx: Nullable<number>;
	active_components: unknown[];
	active_error_boundary: unknown;
	import_urls: string[];
	export_keys: string[];
	error_export_keys: string[];
};

// ─── Typed Adapter Client Loader ────────────────────────────────

export type VormaTypedAdapterAddClientLoaderProps<
	App extends VormaAppBase,
	P extends VormaLoaderPattern<App>,
	LoaderData extends VormaLoaderOutput<App, P>,
	ResultData,
> = {
	pattern: P;
	clientLoader: (props: {
		params: Record<ParamsForPattern<App, P>, string>;
		splatValues: string[];
		serverDataPromise: Promise<
			ClientLoaderAwaitedServerData<App["rootData"], LoaderData>
		>;
		signal: AbortSignal;
	}) => Promise<ResultData>;
	reRunOnModuleChange?: ImportMeta;
};
