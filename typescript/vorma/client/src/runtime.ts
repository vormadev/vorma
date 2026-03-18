import type { BrowserHistory, Update } from "history";
import { createBrowserHistory } from "history";
import { jsonDeepEquals, serializeToSearchParams } from "vorma/kit/json";
import { addOnWindowFocusListener } from "vorma/kit/listeners";
import { findBestMatch } from "vorma/kit/matcher/find-best";
import type { PatternRegistry } from "vorma/kit/matcher/register";
import {
	createPatternRegistry,
	registerPattern,
} from "vorma/kit/matcher/register";
import { stripTrailingSlash } from "vorma/kit/matcher/utils";
import {
	getHrefDetails,
	getIsModifiedNavigationClick,
	getIsPrimaryNavigationClick,
	resolveAbsoluteHref,
	resolveAbsoluteHrefWithOptionalSearchAndHash,
} from "vorma/kit/url";

/////////////////////////////////////////////////////////////////////
/////// Runtime Events
/////////////////////////////////////////////////////////////////////

export type ScrollState = { x: number; y: number } | { hash: string };

export type RouteChangeEventDetail = { __scrollState?: ScrollState };
export type RouteChangeEvent = CustomEvent<RouteChangeEventDetail>;

export type StatusEventDetail = {
	isNavigating: boolean;
	isSubmitting: boolean;
	isRevalidating: boolean;
};
export type StatusEvent = CustomEvent<StatusEventDetail>;
export type Nullable<T> = T | null | undefined;

/////////////////////////////////////////////////////////////////////
/////// Navigation And Submit
/////////////////////////////////////////////////////////////////////

export type SubmitOptions = {
	dedupeKey?: string;
	revalidate?: boolean;
	skipGlobalLoadingIndicator?: boolean;
};

export type SubmitResult<T> =
	| { success: true; data: T }
	| { success: false; error: string };

/////////////////////////////////////////////////////////////////////
/////// App And Route Typing
/////////////////////////////////////////////////////////////////////

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

export type RouteByType<App extends VormaAppBase, T extends string> = Extract<
	App["routes"][number],
	{ _type: T }
>;

export type RouteByPattern<Routes, Pattern> = Extract<
	Routes,
	{ pattern: Pattern }
>;

export type VormaLoader<App extends VormaAppBase> = RouteByType<App, "loader">;
export type VormaQuery<App extends VormaAppBase> = RouteByType<App, "query">;
export type VormaMutation<App extends VormaAppBase> = RouteByType<
	App,
	"mutation"
>;

export type VormaLoaderPattern<App extends VormaAppBase> =
	VormaLoader<App>["pattern"];
export type VormaQueryPattern<App extends VormaAppBase> =
	VormaQuery<App>["pattern"];
export type VormaMutationPattern<App extends VormaAppBase> =
	VormaMutation<App>["pattern"];

export type VormaLoaderOutput<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
> =
	RouteByPattern<VormaLoader<App>, Pattern> extends {
		phantomOutputType: infer OutputType;
	}
		? OutputType
		: null | undefined;

export type VormaQueryInput<
	App extends VormaAppBase,
	Pattern extends VormaQueryPattern<App>,
> =
	RouteByPattern<VormaQuery<App>, Pattern> extends {
		phantomInputType: infer InputType;
	}
		? InputType
		: null | undefined;

export type VormaQueryOutput<
	App extends VormaAppBase,
	Pattern extends VormaQueryPattern<App>,
> =
	RouteByPattern<VormaQuery<App>, Pattern> extends {
		phantomOutputType: infer OutputType;
	}
		? OutputType
		: null | undefined;

export type VormaMutationInput<
	App extends VormaAppBase,
	Pattern extends VormaMutationPattern<App>,
> =
	RouteByPattern<VormaMutation<App>, Pattern> extends {
		phantomInputType: infer InputType;
	}
		? InputType
		: null | undefined;

export type VormaMutationOutput<
	App extends VormaAppBase,
	Pattern extends VormaMutationPattern<App>,
> =
	RouteByPattern<VormaMutation<App>, Pattern> extends {
		phantomOutputType: infer OutputType;
	}
		? OutputType
		: null | undefined;

export type VormaMutationMethod<
	App extends VormaAppBase,
	Pattern extends VormaMutationPattern<App>,
> =
	RouteByPattern<VormaMutation<App>, Pattern> extends { method: infer Method }
		? Method extends string
			? Method
			: "POST"
		: "POST";

export type RouteMetadata<
	App extends VormaAppBase,
	Pattern extends string,
> = Extract<App["routes"][number], { pattern: Pattern }>;

export type GetParams<App extends VormaAppBase, Pattern extends string> =
	RouteMetadata<App, Pattern> extends { params: ReadonlyArray<infer Params> }
		? Params extends string
			? Params
			: never
		: never;

export type ParamsForPattern<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
> = GetParams<App, Pattern>;

export type HasParams<App extends VormaAppBase, Pattern extends string> =
	GetParams<App, Pattern> extends never ? false : true;

export type IsSplat<App extends VormaAppBase, Pattern extends string> =
	RouteMetadata<App, Pattern> extends { isSplat: true } ? true : false;

export type IsEmptyInput<T> = [T] extends [null | undefined] ? true : false;

export type QueryInputContractViolation = {
	__queryInputContractViolation: "Query input root must be an object, null, or undefined.";
};

export type EnforceQueryInputRootContract<Input> = [Input] extends [
	null | undefined,
]
	? Input
	: Input extends Record<string, unknown>
		? Input
		: QueryInputContractViolation;

export type ConditionalParams<
	App extends VormaAppBase,
	Pattern extends string,
> =
	HasParams<App, Pattern> extends true
		? { params: { [K in GetParams<App, Pattern>]: string } }
		: {};

export type ConditionalSplat<App extends VormaAppBase, Pattern extends string> =
	IsSplat<App, Pattern> extends true ? { splatValues: Array<string> } : {};

export type PatternBasedProps<
	App extends VormaAppBase,
	Pattern extends string,
> = {
	pattern: Pattern;
} & ConditionalParams<App, Pattern> &
	ConditionalSplat<App, Pattern>;

export type PermissiveLoaderPattern<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
> = Pattern extends `${infer Prefix}/${App["appConfig"]["loadersExplicitIndexSegmentIdentifier"]}`
	? Pattern | (Prefix extends "" ? "/" : Prefix)
	: Pattern;

export type PermissivePatternBasedProps<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
> = {
	pattern: PermissiveLoaderPattern<App, Pattern>;
} & ConditionalParams<App, Pattern> &
	ConditionalSplat<App, Pattern>;

export type VormaRoutePropsGeneric<
	JSXElement,
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
> = {
	idx: number;
	Outlet: (props: Record<string, unknown>) => JSXElement;
	__phantom_pattern: Pattern;
} & Record<string, unknown>;

export type VormaQueryProps<
	App extends VormaAppBase,
	Pattern extends VormaQueryPattern<App>,
> = (PatternBasedProps<App, Pattern> & {
	options?: SubmitOptions;
	requestInit?: Omit<RequestInit, "method"> & { method?: "GET" };
}) &
	(IsEmptyInput<
		EnforceQueryInputRootContract<VormaQueryInput<App, Pattern>>
	> extends true
		? {
				input?: EnforceQueryInputRootContract<
					VormaQueryInput<App, Pattern>
				>;
			}
		: {
				input: EnforceQueryInputRootContract<
					VormaQueryInput<App, Pattern>
				>;
			});

export type VormaMutationProps<
	App extends VormaAppBase,
	Pattern extends VormaMutationPattern<App>,
> = PatternBasedProps<App, Pattern> & {
	options?: SubmitOptions;
} & (VormaMutationMethod<App, Pattern> extends "POST"
		? { requestInit?: Omit<RequestInit, "method"> & { method?: "POST" } }
		: {
				requestInit: RequestInit & {
					method: VormaMutationMethod<App, Pattern>;
				};
			}) &
	(IsEmptyInput<VormaMutationInput<App, Pattern>> extends true
		? { input?: VormaMutationInput<App, Pattern> }
		: { input: VormaMutationInput<App, Pattern> });

/////////////////////////////////////////////////////////////////////
/////// API Client
/////////////////////////////////////////////////////////////////////

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
	query: <Pattern extends VormaQueryPattern<App>>(
		props: VormaQueryProps<App, Pattern>,
	) => Promise<SubmitResult<VormaQueryOutput<App, Pattern>>>;
	mutate: <Pattern extends VormaMutationPattern<App>>(
		props: VormaMutationProps<App, Pattern>,
	) => Promise<SubmitResult<VormaMutationOutput<App, Pattern>>>;
};

/////////////////////////////////////////////////////////////////////
/////// Client Loader
/////////////////////////////////////////////////////////////////////

export type ClientLoaderAwaitedServerData<RootData, LoaderData> = {
	matchedPatterns: string[];
	rootData: RootData;
	loaderData: LoaderData;
	buildID: string;
};

/////////////////////////////////////////////////////////////////////
/////// Link And Router Hook Types
/////////////////////////////////////////////////////////////////////

export type VormaLinkPropsBase<LinkEvent = unknown> = {
	href?: string;
	prefetch?: "intent" | "none";
	prefetchDelayMs?: number;
	replace?: boolean;
	scrollToTop?: boolean;
	state?: unknown;
	beforeBegin?: (event: LinkEvent) => void | Promise<void>;
	beforeRender?: (event: LinkEvent) => void | Promise<void>;
	afterRender?: (event: LinkEvent) => void | Promise<void>;
};

export type VormaRouteGeneric<
	JSXElement,
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = (props: VormaRoutePropsGeneric<JSXElement, App, Pattern>) => JSXElement;

export type BaseRouterData<RootData, Params extends string> = {
	buildID: string;
	matchedPatterns: string[];
	splatValues: string[];
	params: Record<Params, string>;
	rootData: RootData;
};

export type UseRouterDataWrapper<
	UsesAccessor extends boolean,
	WrappedValue,
> = UsesAccessor extends false ? WrappedValue : () => WrappedValue;

export type UseRouterDataFunction<
	App extends VormaAppBase,
	UsesAccessor extends boolean = false,
> = {
	<Pattern extends VormaLoaderPattern<App>>(
		props: VormaRoutePropsGeneric<unknown, App, Pattern>,
	): UseRouterDataWrapper<
		UsesAccessor,
		BaseRouterData<App["rootData"], ParamsForPattern<App, Pattern>>
	>;
	<Pattern extends VormaLoaderPattern<App>>(): UseRouterDataWrapper<
		UsesAccessor,
		BaseRouterData<App["rootData"], ParamsForPattern<App, Pattern>>
	>;
	(): UseRouterDataWrapper<
		UsesAccessor,
		BaseRouterData<App["rootData"], string>
	>;
};

/////////////////////////////////////////////////////////////////////
/////// Route Outlet Store
/////////////////////////////////////////////////////////////////////

export type RouteOutletRouterDataState = BaseRouterData<unknown, string>;

export type RouteOutletNavigationState = {
	loadersData: unknown[];
	clientLoadersData: unknown[];
	routerData: RouteOutletRouterDataState;
	outermostError: unknown;
	outermostErrorIdx: Nullable<number>;
	activeComponents: Array<unknown>;
	activeErrorBoundary: unknown;
	importURLs: string[];
	exportKeys: string[];
	matchedPatterns: string[];
};

export type RouteOutletLocationState = {
	pathname: string;
	search: string;
	hash: string;
	state: unknown;
};

export type RouteOutletBranchInputState = {
	routeKeys: string[];
	components: Array<unknown>;
	errorComponents: Array<unknown>;
	matchedPatterns: string[];
	outermostErrorIdx: Nullable<number>;
};

export type RouteOutletStoreState = {
	navigation: RouteOutletNavigationState;
	routeOutletBranchInputState: RouteOutletBranchInputState;
	location: RouteOutletLocationState;
};

/////////////////////////////////////////////////////////////////////
/////// Typed Adapter Client Loader
/////////////////////////////////////////////////////////////////////

export type VormaTypedAdapterClientLoaderFunction<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
	LoaderData extends VormaLoaderOutput<App, Pattern>,
	ResultData,
> = (props: {
	params: Record<ParamsForPattern<App, Pattern>, string>;
	splatValues: string[];
	serverDataPromise: Promise<
		ClientLoaderAwaitedServerData<App["rootData"], LoaderData>
	>;
	signal: AbortSignal;
}) => Promise<ResultData>;

export type VormaTypedAdapterAddClientLoaderProps<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
	LoaderData extends VormaLoaderOutput<App, Pattern>,
	ResultData,
> = {
	pattern: Pattern;
	clientLoader: VormaTypedAdapterClientLoaderFunction<
		App,
		Pattern,
		LoaderData,
		ResultData
	>;
	reRunOnModuleChange?: ImportMeta;
};

/////////////////////////////////////////////////////////////////////
/////// Typed Adapter Link
/////////////////////////////////////////////////////////////////////

export type TypedAdapterLinkProps<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
	AnchorProps,
	LinkEvent,
> = Omit<AnchorProps, "href" | "pattern"> &
	VormaLinkPropsBase<LinkEvent> &
	PermissivePatternBasedProps<App, Pattern> & {
		search?: string;
		hash?: string;
	};

export type TypedAdapterLinkDefaultProps<
	App extends VormaAppBase,
	AnchorProps,
	LinkEvent,
> = Partial<
	Omit<
		TypedAdapterLinkProps<
			App,
			VormaLoaderPattern<App>,
			AnchorProps,
			LinkEvent
		>,
		"pattern" | "params" | "splatValues"
	>
>;

/////////////////////////////////////////////////////////////////////
/////// Snapshot Types
/////////////////////////////////////////////////////////////////////

export type HeadEl = {
	tag: string;
	attributesKnownSafe: Record<string, string>;
	booleanAttributes?: Nullable<string[]>;
	dangerousInnerHTML?: string;
};

export type RuntimeRouteSnapshot = {
	outermostServerError: unknown;
	outermostServerErrorIdx: Nullable<number>;
	outermostClientError: unknown;
	outermostClientErrorIdx: Nullable<number>;
	outermostError: unknown;
	outermostErrorIdx: Nullable<number>;
	matchedPatterns: string[];
	loadersData: unknown[];
	importURLs: string[];
	exportKeys: string[];
	errorExportKeys: string[];
	hasRootData: boolean;
	params: Record<string, string>;
	splatValues: string[];
	buildID: string;
	rootElementID?: string;
	activeComponents: unknown[];
	activeErrorBoundary: unknown;
	clientLoadersData: unknown[];
	title?: string;
	metaHeadEls: HeadEl[];
	restHeadEls: HeadEl[];
	deps: string[];
	cssBundles: string[];
};

/////////////////////////////////////////////////////////////////////
/////// Route Data Contract
/////////////////////////////////////////////////////////////////////

type RouteDataPayloadTitle = {
	dangerousInnerHTML: string;
};

type RuntimeRouteDataJSONPayload = {
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
	title?: RouteDataPayloadTitle | null;
	metaHeadEls?: HeadEl[];
	restHeadEls?: HeadEl[];
	deps?: string[];
	cssBundles?: string[];
};

// Backend owns and enforces the full route-data JSON contract.
// This boundary parser intentionally does only a minimal top-level sanity check:
// the response must be an object.
// Route-data keys are intentionally omittable from backend JSON when empty
// (`omitempty`), so decode logic normalizes omitted keys to runtime defaults.
// Do not add deep nested runtime validation here or elsewhere.
function parseRuntimeRouteDataJSONPayloadFromResponseBodyOrThrow(
	responseBodyJSON: unknown,
): RuntimeRouteDataJSONPayload {
	if (
		responseBodyJSON === null ||
		typeof responseBodyJSON !== "object" ||
		Array.isArray(responseBodyJSON)
	) {
		panic(
			"Route data JSON contract violated: response must be a JSON object.",
		);
	}
	return responseBodyJSON as RuntimeRouteDataJSONPayload;
}

/////////////////////////////////////////////////////////////////////
/////// Runtime Context
/////////////////////////////////////////////////////////////////////

type AnyRecord = Record<string, any>;
type AnyPropertyRecord = Record<PropertyKey, any>;
type RouteManifestRecord = Record<string, unknown>;
type ClientLoaderWaitFn = (props: {
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

type PatternToWaitFnMap = Record<string, ClientLoaderWaitFn>;

type RuntimeNavigationIntent = "navigate" | "prefetch" | "revalidate";

type NavigationOperation = {
	id: number;
	intent: RuntimeNavigationIntent;
	targetUrl: string;
	allowTrailingRevalidatePass: boolean;
	abortController: AbortController;
	settledPromise: Promise<void>;
	notifySettled: () => void;
};

type PrefetchCacheEntry = {
	targetDataKey: string;
	targetUrl: string;
	buildID: string;
	routeData: RuntimeRouteSnapshot;
	modulesMap?: ComponentModulesMap;
};

type NavigationRuntimeStore = {
	navigateOperation: NavigationOperation | null;
	revalidateOperation: NavigationOperation | null;
	queuedRevalidateTargetDataKey: string | null;
	queuedRevalidateSettledPromise: Promise<void> | null;
	prefetchOperationsByDataTarget: Map<string, NavigationOperation>;
	prefetchCacheByDataTarget: Map<string, PrefetchCacheEntry>;
	skippedGlobalLoadingIndicatorNavigationOperationIDs: Set<number>;
	skippedGlobalLoadingIndicatorSubmissionOperationIDs: Set<number>;
	nextNavigationOperationID: number;
	nextSubmissionOperationID: number;
	latestStartedSubmissionOperationID: number;
	activeSubmissionOperationIDs: Set<number>;
	submissionAbortControllerByOperationID: Map<number, AbortController>;
	submissionOperationIDByDedupeKey: Map<string, number>;
	lastStatus: StatusEventDetail;
	lastNavOrRevalidateTimestampMS: number;
};

type NavigationStateManager = {
	navigate: (props: NavigateProps) => Promise<{ didNavigate: boolean }>;
	submit: <T>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: SubmitOptions,
	) => Promise<SubmitResult<T>>;
	getStatus: () => StatusEventDetail;
	clearAll: () => void;
	getUnsafeNavigationStore: () => NavigationRuntimeStore;
};

export type VormaClientGlobalNonSnapshotState = {
	isDev: boolean;
	viteDevURL: string;
	publicPathPrefix: string;
	isTouchInputModalityActive: boolean;
	hasRegisteredInputModalityDetectionListeners: boolean;
	nextRouteManifestProgressiveLoadID: number;
	routeManifest: RouteManifestRecord | undefined;
	patternToWaitFnMap: PatternToWaitFnMap;
	defaultErrorBoundary: typeof defaultErrorBoundary;
	useViewTransitions: boolean;
	deploymentID: string;
	vormaAppConfig: VormaAppConfig;
	routeManifestURL: string;
	patternRegistry: PatternRegistry;
	navigationStateManager?: NavigationStateManager;
	historyInstance?: BrowserHistory;
	removeHistoryListener?: () => void;
	lastKnownHistoryLocationKey?: string;
	lastKnownHistoryLocationHref?: string;
	hasRegisteredBeforeUnloadListener: boolean;
	windowEventListenersByEventName?: Map<string, Set<EventListener>>;
	hardRedirectForTesting?: ((href: string) => void) | undefined;
};

export type VormaClientGlobal = VormaClientGlobalNonSnapshotState & {
	runtimeRouteSnapshot: RuntimeRouteSnapshot;
};

export type VormaRuntimeContext = {
	navigationStateManager: NavigationStateManager;
};

export type RouteChangeListener = (event: RouteChangeEvent) => void;
export type StatusListener = (event: StatusEvent) => void;
export type LocationListener = (
	event: CustomEvent<RouteOutletLocationState>,
) => void;
export type BuildIDListener = (
	event: CustomEvent<{ oldID: string; newID: string }>,
) => void;

const VORMA_SYMBOL = Symbol.for("__vorma_internal__");
const ACCEPTS_CLIENT_REDIRECT_HEADER_NAME = "X-Accepts-Client-Redirect";

function createPatternRegistryForAppConfig(
	vormaAppConfig: VormaAppConfig,
): PatternRegistry {
	return createPatternRegistry({
		dynamicParamPrefixRune: vormaAppConfig.loadersDynamicRune,
		splatSegmentRune: vormaAppConfig.loadersSplatRune,
		explicitIndexSegment:
			vormaAppConfig.loadersExplicitIndexSegmentIdentifier,
	});
}

function createDefaultRuntimeRouteSnapshot(): RuntimeRouteSnapshot {
	return {
		outermostServerError: undefined,
		outermostServerErrorIdx: undefined,
		outermostClientError: undefined,
		outermostClientErrorIdx: undefined,
		outermostError: undefined,
		outermostErrorIdx: undefined,
		matchedPatterns: [],
		loadersData: [],
		importURLs: [],
		exportKeys: [],
		errorExportKeys: [],
		hasRootData: false,
		params: {},
		splatValues: [],
		buildID: "1",
		rootElementID: undefined,
		activeComponents: [],
		activeErrorBoundary: undefined,
		clientLoadersData: [],
		title: undefined,
		metaHeadEls: [],
		restHeadEls: [],
		deps: [],
		cssBundles: [],
	};
}

function createDefaultVormaClientGlobalState(): VormaClientGlobal {
	const defaultAppConfig: VormaAppConfig = {
		actionsRouterMountRoot: "/api/",
		actionsDynamicRune: ":",
		actionsSplatRune: "*",
		loadersDynamicRune: ":",
		loadersSplatRune: "*",
		loadersExplicitIndexSegmentIdentifier: "_index",
	};
	return {
		isDev: false,
		viteDevURL: "",
		publicPathPrefix: "",
		isTouchInputModalityActive: false,
		hasRegisteredInputModalityDetectionListeners: false,
		nextRouteManifestProgressiveLoadID: 0,
		routeManifest: undefined,
		patternToWaitFnMap: {},
		defaultErrorBoundary,
		useViewTransitions: false,
		deploymentID: "",
		vormaAppConfig: defaultAppConfig,
		routeManifestURL: "",
		patternRegistry: createPatternRegistryForAppConfig(defaultAppConfig),
		runtimeRouteSnapshot: createDefaultRuntimeRouteSnapshot(),
		navigationStateManager: undefined,
		historyInstance: undefined,
		removeHistoryListener: undefined,
		lastKnownHistoryLocationKey: undefined,
		lastKnownHistoryLocationHref: undefined,
		hasRegisteredBeforeUnloadListener: false,
		windowEventListenersByEventName: new Map<string, Set<EventListener>>(),
		hardRedirectForTesting: undefined,
	};
}

function getGlobalStateOrThrow(): VormaClientGlobal {
	const maybeState = (globalThis as AnyPropertyRecord)[VORMA_SYMBOL] as
		| VormaClientGlobal
		| undefined;
	if (!maybeState) {
		throw new Error(
			'Vorma client runtime state is not initialized on globalThis[Symbol.for("__vorma_internal__")].',
		);
	}
	return maybeState;
}

function ensureGlobalState(): VormaClientGlobal {
	const maybeState = (globalThis as AnyPropertyRecord)[VORMA_SYMBOL] as
		| VormaClientGlobal
		| undefined;
	if (maybeState) {
		return maybeState;
	}
	const nextState = createDefaultVormaClientGlobalState();
	(globalThis as AnyPropertyRecord)[VORMA_SYMBOL] = nextState;
	return nextState;
}

/////////////////////////////////////////////////////////////////////
/////// Snapshot
/////////////////////////////////////////////////////////////////////

function getRuntimeRouteSnapshot(): RuntimeRouteSnapshot {
	return ensureGlobalState().runtimeRouteSnapshot;
}

function setRuntimeRouteSnapshot(
	nextSnapshot: RuntimeRouteSnapshot,
): RuntimeRouteSnapshot {
	const globalState = getGlobalStateOrThrow();
	globalState.runtimeRouteSnapshot = nextSnapshot;
	return nextSnapshot;
}

function updateRuntimeRouteSnapshot(props: {
	patch: Partial<RuntimeRouteSnapshot>;
}): RuntimeRouteSnapshot {
	const previousSnapshot = getRuntimeRouteSnapshot();
	const nextSnapshot = {
		...previousSnapshot,
		...props.patch,
	} as RuntimeRouteSnapshot;
	return setRuntimeRouteSnapshot(nextSnapshot);
}

function setRuntimeBuildID(props: { nextBuildID: string }): void {
	const previousBuildID = getRuntimeRouteSnapshot().buildID;
	if (previousBuildID === props.nextBuildID) {
		return;
	}
	updateRuntimeRouteSnapshot({
		patch: {
			buildID: props.nextBuildID,
		},
	});
	dispatchBuildIDEvent({
		oldID: previousBuildID,
		newID: props.nextBuildID,
	});
}

function buildRouterDataSnapshotView(
	snapshot: RuntimeRouteSnapshot,
): RouteOutletRouterDataState {
	return {
		buildID: snapshot.buildID,
		matchedPatterns: snapshot.matchedPatterns,
		splatValues: snapshot.splatValues,
		params: snapshot.params,
		rootData: (snapshot.hasRootData
			? snapshot.loadersData[0]
			: null) as unknown,
	};
}

export function getRouterData<
	App extends VormaAppBase = VormaAppBase,
	UsesAccessor extends boolean = false,
	Pattern extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
>(
	routeProps?: VormaRoutePropsGeneric<unknown, App, Pattern>,
): UseRouterDataWrapper<
	UsesAccessor,
	BaseRouterData<App["rootData"], ParamsForPattern<App, Pattern>>
> {
	void routeProps;
	const snapshot = getRuntimeRouteSnapshot();
	const routerData = buildRouterDataSnapshotView(snapshot) as BaseRouterData<
		App["rootData"],
		ParamsForPattern<App, Pattern>
	>;
	return routerData as UseRouterDataWrapper<
		UsesAccessor,
		BaseRouterData<App["rootData"], ParamsForPattern<App, Pattern>>
	>;
}

/////////////////////////////////////////////////////////////////////
/////// Events
/////////////////////////////////////////////////////////////////////

const STATUS_EVENT_NAME = "vorma:status";
const ROUTE_CHANGE_EVENT_NAME = "vorma:route-change";
const LOCATION_EVENT_NAME = "vorma:location";
const BUILD_ID_EVENT_NAME = "vorma:build-id";

function getWindowEventListenersByEventName(
	globalState: VormaClientGlobal,
): Map<string, Set<EventListener>> {
	const listenersByEventName =
		globalState.windowEventListenersByEventName ??
		new Map<string, Set<EventListener>>();
	globalState.windowEventListenersByEventName = listenersByEventName;
	return listenersByEventName;
}

function addWindowEventListener<EventType extends Event>(props: {
	eventName: string;
	listener: (event: EventType) => void;
}): () => void {
	const globalState = ensureGlobalState();
	const runtimeWindowEventListenersByEventName =
		getWindowEventListenersByEventName(globalState);
	const listener = props.listener as EventListener;
	const listenersForEventName =
		runtimeWindowEventListenersByEventName.get(props.eventName) ??
		new Set<EventListener>();
	listenersForEventName.add(listener);
	runtimeWindowEventListenersByEventName.set(
		props.eventName,
		listenersForEventName,
	);
	window.addEventListener(props.eventName, listener);
	return () => {
		window.removeEventListener(props.eventName, listener);
		listenersForEventName.delete(listener);
		if (listenersForEventName.size === 0) {
			runtimeWindowEventListenersByEventName.delete(props.eventName);
		}
	};
}

function dispatchWindowEvent<EventDetail>(props: {
	eventName: string;
	detail: EventDetail;
}): void {
	window.dispatchEvent(
		new CustomEvent<EventDetail>(props.eventName, {
			detail: props.detail,
		}),
	);
}

function dispatchStatusEvent(detail: StatusEventDetail): void {
	dispatchWindowEvent({
		eventName: STATUS_EVENT_NAME,
		detail,
	});
}

function dispatchRouteChangeEvent(detail: RouteChangeEventDetail): void {
	dispatchWindowEvent({
		eventName: ROUTE_CHANGE_EVENT_NAME,
		detail,
	});
}

function dispatchLocationEvent(detail: RouteOutletLocationState): void {
	dispatchWindowEvent({
		eventName: LOCATION_EVENT_NAME,
		detail,
	});
}

function dispatchBuildIDEvent(detail: { oldID: string; newID: string }): void {
	dispatchWindowEvent({
		eventName: BUILD_ID_EVENT_NAME,
		detail,
	});
}

export function addStatusListener(listener: StatusListener): () => void {
	return addWindowEventListener<StatusEvent>({
		eventName: STATUS_EVENT_NAME,
		listener: listener as (event: StatusEvent) => void,
	});
}

export function addRouteChangeListener(
	listener: RouteChangeListener,
): () => void {
	return addWindowEventListener<RouteChangeEvent>({
		eventName: ROUTE_CHANGE_EVENT_NAME,
		listener: listener as (event: RouteChangeEvent) => void,
	});
}

export function addLocationListener(listener: LocationListener): () => void {
	return addWindowEventListener<CustomEvent<RouteOutletLocationState>>({
		eventName: LOCATION_EVENT_NAME,
		listener: listener as (
			event: CustomEvent<RouteOutletLocationState>,
		) => void,
	});
}

export function addBuildIDListener(listener: BuildIDListener): () => void {
	return addWindowEventListener<
		CustomEvent<{ oldID: string; newID: string }>
	>({
		eventName: BUILD_ID_EVENT_NAME,
		listener: listener as (
			event: CustomEvent<{ oldID: string; newID: string }>,
		) => void,
	});
}

/////////////////////////////////////////////////////////////////////
/////// History And Scroll
/////////////////////////////////////////////////////////////////////

const SCROLL_STATE_STORAGE_KEY = "__vorma__scrollStateMap";
const PAGE_REFRESH_SCROLL_STATE_STORAGE_KEY = "__vorma__pageRefreshScrollState";
const PAGE_REFRESH_SCROLL_STATE_MAX_AGE_MS = 5000;

function getOrCreateHistoryInstance(): BrowserHistory {
	const globalState = ensureGlobalState();
	if (globalState.historyInstance) {
		return globalState.historyInstance;
	}
	const history = createBrowserHistory();
	globalState.historyInstance = history;
	globalState.lastKnownHistoryLocationKey = history.location.key;
	globalState.lastKnownHistoryLocationHref = new URL(
		`${history.location.pathname}${history.location.search}${history.location.hash}`,
		window.location.origin,
	).href;
	globalState.removeHistoryListener = history.listen((update) => {
		void customHistoryListener(update);
	});
	return history;
}

function buildAbsoluteHrefFromHistoryUpdate(update: Update): string {
	return new URL(
		`${update.location.pathname}${update.location.search}${update.location.hash}`,
		window.location.origin,
	).href;
}

async function customHistoryListener(update: Update): Promise<void> {
	const globalState = ensureGlobalState();
	const nextLocationKey = update.location.key;
	if (nextLocationKey === globalState.lastKnownHistoryLocationKey) {
		return;
	}
	const previousLocationKey = globalState.lastKnownHistoryLocationKey;
	const previousLocationHref =
		globalState.lastKnownHistoryLocationHref ?? window.location.href;
	const nextLocationHref = buildAbsoluteHrefFromHistoryUpdate(update);
	if (update.action === "POP" && previousLocationKey) {
		saveStoredScrollStateForHistoryKey(previousLocationKey, {
			x: window.scrollX,
			y: window.scrollY,
		});
	}
	globalState.lastKnownHistoryLocationKey = nextLocationKey;
	globalState.lastKnownHistoryLocationHref = nextLocationHref;
	dispatchLocationEvent(getLocation());
	if (update.action !== "POP") {
		return;
	}
	try {
		await getDefaultVormaRuntimeContext().navigationStateManager.navigate({
			href: nextLocationHref,
			intent: "navigate",
			replace: true,
			skipHistoryCommit: true,
			currentHrefForClassification: previousLocationHref,
		});
	} catch (error) {
		console.error(
			"Vorma:",
			"Cross-document POP navigation failed; falling back to hard redirect.",
			error,
		);
		try {
			performHardRedirect(nextLocationHref);
		} catch (hardRedirectError) {
			console.error(
				"Vorma:",
				"Hard redirect fallback failed after cross-document POP navigation error.",
				hardRedirectError,
			);
		}
	}
}

function normalizeHashForElementLookup(hash: string): string {
	const hashWithoutPrefix = hash.startsWith("#") ? hash.slice(1) : hash;
	if (hashWithoutPrefix.length === 0) {
		return hashWithoutPrefix;
	}
	try {
		return decodeURIComponent(hashWithoutPrefix);
	} catch {
		return hashWithoutPrefix;
	}
}

function applyScrollState(scrollState?: ScrollState): void {
	if (!scrollState) {
		const normalizedHash = normalizeHashForElementLookup(
			window.location.hash,
		);
		if (normalizedHash.length > 0) {
			const hashElement = document.getElementById(normalizedHash);
			hashElement?.scrollIntoView();
		}
		return;
	}
	if ("hash" in scrollState) {
		const normalizedHash = normalizeHashForElementLookup(scrollState.hash);
		const hashElement = document.getElementById(normalizedHash);
		hashElement?.scrollIntoView();
		return;
	}
	window.scrollTo(scrollState.x, scrollState.y);
}

const MAX_STORED_SCROLL_STATE_ENTRY_COUNT = 50;

type StoredScrollState = { x: number; y: number };
type StoredScrollStateEntry = [string, StoredScrollState];

function isFiniteNumber(value: unknown): value is number {
	return typeof value === "number" && Number.isFinite(value);
}

function isStoredScrollStateShape(value: unknown): value is StoredScrollState {
	if (!value || typeof value !== "object") {
		return false;
	}
	const maybeState = value as {
		x?: unknown;
		y?: unknown;
	};
	return isFiniteNumber(maybeState.x) && isFiniteNumber(maybeState.y);
}

function isStoredScrollStateEntryShape(
	value: unknown,
): value is StoredScrollStateEntry {
	if (!Array.isArray(value) || value.length !== 2) {
		return false;
	}
	const [historyKey, state] = value;
	return typeof historyKey === "string" && isStoredScrollStateShape(state);
}

function getScrollStateEntriesFromStorage(): StoredScrollStateEntry[] {
	try {
		const raw = window.sessionStorage.getItem(SCROLL_STATE_STORAGE_KEY);
		if (!raw) {
			return [];
		}
		const parsed = JSON.parse(raw);
		if (!Array.isArray(parsed)) {
			return [];
		}
		return parsed.filter(isStoredScrollStateEntryShape).map((entry) => [
			entry[0],
			{
				x: entry[1].x,
				y: entry[1].y,
			},
		]);
	} catch {
		return [];
	}
}

function setScrollStateEntriesInStorage(
	entries: StoredScrollStateEntry[],
): void {
	try {
		window.sessionStorage.setItem(
			SCROLL_STATE_STORAGE_KEY,
			JSON.stringify(entries),
		);
	} catch {
		// No-op by design for storage failures.
	}
}

function saveStoredScrollStateForHistoryKey(
	historyKey: string,
	state: StoredScrollState,
): void {
	if (!historyKey) {
		return;
	}
	const nextEntries = getScrollStateEntriesFromStorage().filter(
		(entry) => entry[0] !== historyKey,
	);
	nextEntries.push([historyKey, { x: state.x, y: state.y }]);
	if (nextEntries.length > MAX_STORED_SCROLL_STATE_ENTRY_COUNT) {
		nextEntries.splice(
			0,
			nextEntries.length - MAX_STORED_SCROLL_STATE_ENTRY_COUNT,
		);
	}
	setScrollStateEntriesInStorage(nextEntries);
}

function getCurrentHistoryStorageKey(): string | null {
	const state = window.history.state as AnyRecord | null;
	const key = state?.key;
	return typeof key === "string" ? key : null;
}

function saveStoredScrollState(): void {
	const historyKey = getCurrentHistoryStorageKey();
	if (!historyKey) {
		return;
	}
	saveStoredScrollStateForHistoryKey(historyKey, {
		x: window.scrollX,
		y: window.scrollY,
	});
}

function getStoredScrollStateForHistoryKey(
	historyKey: string,
): StoredScrollState | undefined {
	for (const [
		storedHistoryKey,
		storedState,
	] of getScrollStateEntriesFromStorage()) {
		if (storedHistoryKey === historyKey) {
			return storedState;
		}
	}
	return undefined;
}

function resolveStoredScrollStateForCurrentHistoryKeyOrTop(): StoredScrollState {
	const currentHistoryKey = getCurrentHistoryStorageKey();
	if (!currentHistoryKey) {
		return {
			x: 0,
			y: 0,
		};
	}
	return (
		getStoredScrollStateForHistoryKey(currentHistoryKey) ?? {
			x: 0,
			y: 0,
		}
	);
}

function restoreRecentPageRefreshScrollState(): void {
	try {
		const raw = window.sessionStorage.getItem(
			PAGE_REFRESH_SCROLL_STATE_STORAGE_KEY,
		);
		if (!raw) {
			return;
		}
		window.sessionStorage.removeItem(PAGE_REFRESH_SCROLL_STATE_STORAGE_KEY);
		const parsed = JSON.parse(raw) as {
			x: number;
			y: number;
			unix: number;
			href: string;
		};
		if (
			typeof parsed?.x !== "number" ||
			typeof parsed?.y !== "number" ||
			typeof parsed?.unix !== "number" ||
			typeof parsed?.href !== "string"
		) {
			return;
		}
		const classification = classifyNavigationTargetAgainstCurrentLocation({
			targetHref: parsed.href,
			currentHref: window.location.href,
		});
		if (classification !== "same-document-noop") {
			return;
		}
		if (Date.now() - parsed.unix > PAGE_REFRESH_SCROLL_STATE_MAX_AGE_MS) {
			return;
		}
		window.requestAnimationFrame(() => {
			window.scrollTo(parsed.x, parsed.y);
		});
	} catch {
		// No-op by design for storage failures.
	}
}

function savePageRefreshScrollStateOnBeforeUnload(): void {
	try {
		window.sessionStorage.setItem(
			PAGE_REFRESH_SCROLL_STATE_STORAGE_KEY,
			JSON.stringify({
				x: window.scrollX,
				y: window.scrollY,
				unix: Date.now(),
				href: window.location.href,
			}),
		);
	} catch {
		// No-op by design for storage failures.
	}
}

function setHistoryScrollRestorationToManualWhenSupported(): void {
	try {
		window.history.scrollRestoration = "manual";
	} catch {
		// No-op by design for unsupported or locked environments.
	}
}

function registerBeforeUnloadScrollPersistenceListenerOnce(): void {
	const globalState = ensureGlobalState();
	if (globalState.hasRegisteredBeforeUnloadListener) {
		return;
	}
	window.addEventListener("beforeunload", () => {
		savePageRefreshScrollStateOnBeforeUnload();
	});
	globalState.hasRegisteredBeforeUnloadListener = true;
}

/////////////////////////////////////////////////////////////////////
/////// Platform Safety
/////////////////////////////////////////////////////////////////////

function assertProgrammaticSameOriginOrThrow(props: {
	absoluteHref: string;
	apiName: string;
}): void {
	const absoluteURL = new URL(props.absoluteHref);
	if (absoluteURL.origin !== window.location.origin) {
		throw new Error(
			`${props.apiName} only supports same-origin targets. Received: "${props.absoluteHref}".`,
		);
	}
}

const VORMA_ERROR_SENTINEL_PREFIX = "__vorma_error__:";

const VORMA_ABORT_REASON_CODE = {
	SupersededByNewNavigation: "superseded_by_new_navigation",
	SupersededByNewRevalidation: "superseded_by_new_revalidation",
	SupersededBySubmitDedupe: "superseded_by_submit_dedupe",
	ClearAll: "clear_all",
	PrefetchStopped: "prefetch_stopped",
} as const;

type VormaAbortReasonCode =
	(typeof VORMA_ABORT_REASON_CODE)[keyof typeof VORMA_ABORT_REASON_CODE];

const VORMA_ABORT_REASON_CODE_SET = new Set<string>(
	Object.values(VORMA_ABORT_REASON_CODE),
);

function encodeVormaAbortReason(
	reasonCode: VormaAbortReasonCode,
): `${typeof VORMA_ERROR_SENTINEL_PREFIX}${VormaAbortReasonCode}` {
	return `${VORMA_ERROR_SENTINEL_PREFIX}${reasonCode}`;
}

function decodeVormaAbortReason(error: unknown): VormaAbortReasonCode | null {
	if (typeof error !== "string") {
		return null;
	}
	if (!error.startsWith(VORMA_ERROR_SENTINEL_PREFIX)) {
		return null;
	}
	const reasonCode = error.slice(VORMA_ERROR_SENTINEL_PREFIX.length);
	if (!VORMA_ABORT_REASON_CODE_SET.has(reasonCode)) {
		return null;
	}
	return reasonCode as VormaAbortReasonCode;
}

function isAbortError(error: unknown): boolean {
	if (decodeVormaAbortReason(error) !== null) {
		return true;
	}
	if (!error || typeof error !== "object") {
		return false;
	}
	return (
		(error as AnyRecord).name === "AbortError" ||
		String((error as AnyRecord).message ?? "")
			.toLowerCase()
			.includes("abort")
	);
}

function panic(message: string): never {
	throw new Error(message);
}

function setAcceptsClientRedirectHeader(headers: Headers): void {
	headers.set(ACCEPTS_CLIENT_REDIRECT_HEADER_NAME, "1");
}

/////////////////////////////////////////////////////////////////////
/////// URL Resolution
/////////////////////////////////////////////////////////////////////

type RouteResolutionCoreProps<Pattern extends string = string> = {
	pattern: Pattern;
	params?: Record<string, string>;
	splatValues?: string[];
	search?: string;
	hash?: string;
	replace?: boolean;
	scrollToTop?: boolean;
	state?: unknown;
	input?: unknown;
	options?: SubmitOptions;
	requestInit?: APIRequestInitOverrides;
};

type PathResolutionConfigCore = Pick<
	VormaAppConfig,
	| "actionsDynamicRune"
	| "actionsSplatRune"
	| "loadersDynamicRune"
	| "loadersSplatRune"
	| "loadersExplicitIndexSegmentIdentifier"
>;

export type APIClientHelperOpts = {
	type: "query" | "mutation" | "loader";
	vormaAppConfig: VormaAppConfig;
	pattern: string;
	params?: Record<string, string>;
	splatValues?: string[];
	input?: unknown;
	options?: SubmitOptions;
	requestInit?: APIRequestInitOverrides;
};

export type PathResolutionProps = RouteResolutionCoreProps<string>;
export type ResolvePathInput = {
	vormaAppConfig: PathResolutionConfigCore;
	type: "query" | "mutation" | "loader";
	props: PathResolutionProps;
};

export type URLBuildProps = RouteResolutionCoreProps<string>;
export type URLBuildConfig = PathResolutionConfigCore &
	Pick<VormaAppConfig, "actionsRouterMountRoot">;
export type URLBuildInput = {
	vormaAppConfig: URLBuildConfig;
	type: "query" | "mutation" | "loader";
	props: URLBuildProps;
};

function stripTrailingSlashPreservingRoot(path: string): string {
	return path === "/" ? path : stripTrailingSlash(path);
}

function encodeSplatValues(splatValues: string[]): string {
	return splatValues.map((value) => encodeURIComponent(value)).join("/");
}

function replaceDynamicParam(props: {
	pathSegment: string;
	params: Record<string, string>;
	dynamicRune: string;
}): string {
	if (!props.pathSegment.startsWith(props.dynamicRune)) {
		return props.pathSegment;
	}
	const paramKey = props.pathSegment.slice(props.dynamicRune.length);
	return encodeURIComponent(props.params[paramKey] as string);
}

function resolvePatternPath(props: {
	pattern: string;
	params?: Record<string, string>;
	splatValues?: string[];
	dynamicRune: string;
	splatRune: string;
	explicitIndexSegmentIdentifier: string;
}): string {
	const params = props.params ?? {};
	const normalizedPattern =
		props.pattern === `/${props.explicitIndexSegmentIdentifier}`
			? "/"
			: props.pattern;
	const patternSegments = normalizedPattern
		.split("/")
		.filter((segment) => segment.length > 0)
		.filter((segment) => segment !== props.explicitIndexSegmentIdentifier);
	const splatValues = props.splatValues ?? [];
	const pathSegments = patternSegments.flatMap((segment) => {
		if (segment === props.splatRune) {
			return encodeSplatValues(splatValues)
				.split("/")
				.filter((part) => part.length > 0);
		}
		return [
			replaceDynamicParam({
				pathSegment: segment,
				params,
				dynamicRune: props.dynamicRune,
			}),
		];
	});

	const nextPath = "/" + pathSegments.join("/");
	return stripTrailingSlashPreservingRoot(
		nextPath.length === 0 ? "/" : nextPath,
	);
}

function resolveVormaPath(input: ResolvePathInput): string {
	if (input.type === "query" || input.type === "mutation") {
		return resolvePatternPath({
			pattern: input.props.pattern,
			params: input.props.params,
			splatValues: input.props.splatValues,
			dynamicRune: input.vormaAppConfig.actionsDynamicRune,
			splatRune: input.vormaAppConfig.actionsSplatRune,
			explicitIndexSegmentIdentifier:
				input.vormaAppConfig.loadersExplicitIndexSegmentIdentifier,
		});
	}
	return resolvePatternPath({
		pattern: input.props.pattern,
		params: input.props.params,
		splatValues: input.props.splatValues,
		dynamicRune: input.vormaAppConfig.loadersDynamicRune,
		splatRune: input.vormaAppConfig.loadersSplatRune,
		explicitIndexSegmentIdentifier:
			input.vormaAppConfig.loadersExplicitIndexSegmentIdentifier,
	});
}

function getCurrentOrigin(): string {
	return window.location.origin;
}

function buildVormaURL(input: URLBuildInput): URL {
	const pathname = resolveVormaPath({
		vormaAppConfig: input.vormaAppConfig,
		type: input.type,
		props: input.props,
	});
	const actionsPathname =
		input.type === "loader"
			? pathname
			: stripTrailingSlashPreservingRoot(
					input.vormaAppConfig.actionsRouterMountRoot,
				) + (pathname === "/" ? "" : pathname);
	const url = new URL(actionsPathname, getCurrentOrigin());
	if (input.props.search !== undefined) {
		url.search = input.props.search;
	}
	if (input.props.hash !== undefined) {
		url.hash = input.props.hash;
	}
	return url;
}

function resolveVormaRequestBody(input: unknown): BodyInit | null | undefined {
	if (
		input === undefined ||
		input === null ||
		typeof input === "string" ||
		input instanceof ReadableStream ||
		input instanceof FormData ||
		input instanceof URLSearchParams ||
		input instanceof Blob ||
		input instanceof ArrayBuffer
	) {
		return input;
	}
	if (ArrayBuffer.isView(input)) {
		const view = input as ArrayBufferView;
		if (view.buffer instanceof ArrayBuffer) {
			return view as ArrayBufferView<ArrayBuffer>;
		}
	}
	return JSON.stringify(input);
}

export function buildQueryURL(
	vormaAppConfig: URLBuildConfig,
	props: URLBuildProps,
): URL {
	const url = buildVormaURL({ vormaAppConfig, props, type: "query" });
	if (props.input && typeof props.input === "object") {
		url.search = serializeToSearchParams(props.input).toString();
	}
	return url;
}

export function buildMutationURL(
	vormaAppConfig: URLBuildConfig,
	props: URLBuildProps,
): URL {
	return buildVormaURL({ vormaAppConfig, props, type: "mutation" });
}

export function resolveBody(props: {
	input?: unknown;
}): BodyInit | null | undefined {
	return resolveVormaRequestBody(props.input);
}

export function resolvePath(opts: APIClientHelperOpts): string {
	const resolvedPath = resolveVormaPath({
		vormaAppConfig: opts.vormaAppConfig,
		type: opts.type,
		props: {
			pattern: opts.pattern,
			params: opts.params,
			splatValues: opts.splatValues,
		},
	});
	if (opts.type === "loader") {
		return resolvedPath;
	}
	const mountRoot = stripTrailingSlashPreservingRoot(
		opts.vormaAppConfig.actionsRouterMountRoot,
	);
	return mountRoot + (resolvedPath === "/" ? "" : resolvedPath);
}

/////////////////////////////////////////////////////////////////////
/////// Network Decode
/////////////////////////////////////////////////////////////////////

function decodeHTMLStringToText(html: string): string {
	const decoder = document.createElement("textarea");
	decoder.innerHTML = html;
	return decoder.value;
}

function resolveDecodedTitleFromRouteDataPayload(
	payload: RuntimeRouteDataJSONPayload,
): string {
	if (payload.title === undefined || payload.title === null) {
		return "";
	}
	return decodeHTMLStringToText(payload.title.dangerousInnerHTML);
}

function decodeRouteDataPayload(
	payload: RuntimeRouteDataJSONPayload,
): RuntimeRouteSnapshot {
	const previousSnapshot = getRuntimeRouteSnapshot();
	const nextSnapshot: RuntimeRouteSnapshot = {
		outermostServerError: payload.outermostServerError,
		outermostServerErrorIdx: payload.outermostServerErrorIdx,
		outermostClientError: undefined,
		outermostClientErrorIdx: undefined,
		outermostError: payload.outermostServerError,
		outermostErrorIdx: payload.outermostServerErrorIdx,
		matchedPatterns:
			payload.matchedPatterns === undefined
				? []
				: payload.matchedPatterns,
		loadersData:
			payload.loadersData === undefined ? [] : payload.loadersData,
		importURLs: payload.importURLs === undefined ? [] : payload.importURLs,
		exportKeys: payload.exportKeys === undefined ? [] : payload.exportKeys,
		errorExportKeys:
			payload.errorExportKeys === undefined
				? []
				: payload.errorExportKeys,
		hasRootData:
			payload.hasRootData === undefined ? false : payload.hasRootData,
		params: payload.params === undefined ? {} : payload.params,
		splatValues:
			payload.splatValues === undefined ? [] : payload.splatValues,
		buildID: previousSnapshot.buildID,
		rootElementID: previousSnapshot.rootElementID,
		activeComponents: [],
		activeErrorBoundary: undefined,
		clientLoadersData: [],
		title: resolveDecodedTitleFromRouteDataPayload(payload),
		metaHeadEls:
			payload.metaHeadEls === undefined ? [] : payload.metaHeadEls,
		restHeadEls:
			payload.restHeadEls === undefined ? [] : payload.restHeadEls,
		deps: payload.deps === undefined ? [] : payload.deps,
		cssBundles: payload.cssBundles === undefined ? [] : payload.cssBundles,
	};
	return nextSnapshot;
}

type RouteDataFetchResult =
	| {
			status: "success";
			routeSnapshot: RuntimeRouteSnapshot;
			buildID: string;
	  }
	| {
			status: "redirect";
			href: string;
			buildID: string;
			isHardReload: boolean;
	  };

function resolveNativeFetchRedirectTarget(props: {
	requestURL: URL;
	response: Response;
}): string | undefined {
	if (!props.response.redirected || !props.response.url) {
		return undefined;
	}
	const redirectedHref = new URL(props.response.url, props.requestURL.href)
		.href;
	return redirectedHref === props.requestURL.href
		? undefined
		: redirectedHref;
}

function resolveRedirectTargetHref(props: {
	redirectTarget: string;
	requestURL: URL;
}): string {
	return new URL(props.redirectTarget, props.requestURL.href).href;
}

const MAX_REDIRECT_CHAIN_FOLLOWS = 10;

function assertRedirectHrefUsesHTTPProtocol(redirectHref: string): void {
	const protocol = new URL(redirectHref, window.location.href).protocol;
	if (protocol === "http:" || protocol === "https:") {
		return;
	}
	throw new Error(
		`Redirect target must use http(s) protocol: ${redirectHref}`,
	);
}

function isSameOriginHref(href: string): boolean {
	return (
		new URL(href, window.location.href).origin === window.location.origin
	);
}

function resolveHardReloadHref(props: {
	targetHref: string;
	buildID: string;
}): string {
	const hardReloadURL = new URL(props.targetHref, window.location.href);
	hardReloadURL.searchParams.set("vorma_reload", props.buildID);
	return hardReloadURL.href;
}

function performHardRedirect(href: string): void {
	const globalState = getGlobalStateOrThrow();
	if (import.meta.env.MODE === "test") {
		if (typeof globalState.hardRedirectForTesting === "function") {
			globalState.hardRedirectForTesting(href);
			return;
		}
	}
	window.location.assign(href);
}

async function followRedirectWithHardFallback(props: {
	redirectHref: string;
	buildID: string;
	isHardReload: boolean;
	store?: NavigationRuntimeStore;
	redirectHopCount?: number;
	skipGlobalLoadingIndicator?: boolean;
}): Promise<{ didNavigate: boolean }> {
	assertRedirectHrefUsesHTTPProtocol(props.redirectHref);

	if (props.isHardReload) {
		performHardRedirect(
			resolveHardReloadHref({
				targetHref: props.redirectHref,
				buildID: props.buildID,
			}),
		);
		return {
			didNavigate: false,
		};
	}
	if (!isSameOriginHref(props.redirectHref)) {
		performHardRedirect(props.redirectHref);
		return {
			didNavigate: false,
		};
	}
	if (
		classifyNavigationTargetAgainstCurrentLocation({
			targetHref: props.redirectHref,
			currentHref: window.location.href,
		}) === "same-document-noop"
	) {
		return {
			didNavigate: false,
		};
	}
	const redirectHopCount = props.redirectHopCount ?? 0;
	if (redirectHopCount >= MAX_REDIRECT_CHAIN_FOLLOWS - 1) {
		console.error("Vorma:", "Too many redirects");
		return {
			didNavigate: false,
		};
	}

	if (!props.store) {
		return vormaNavigate(props.redirectHref, {
			replace: true,
		});
	}

	return executeNavigationOperation({
		store: props.store,
		navigationProps: {
			href: props.redirectHref,
			replace: true,
			intent: "navigate",
			redirectHopCount: redirectHopCount + 1,
			skipGlobalLoadingIndicator: props.skipGlobalLoadingIndicator,
		},
	});
}

async function fetchRouteDataSnapshot(props: {
	targetUrl: string;
	signal: AbortSignal;
}): Promise<RouteDataFetchResult> {
	const globalState = getGlobalStateOrThrow();
	const requestURL = new URL(props.targetUrl);
	requestURL.searchParams.set(
		"vorma_json",
		getRuntimeRouteSnapshot().buildID,
	);
	const requestHeaders = new Headers();
	setAcceptsClientRedirectHeader(requestHeaders);
	const deploymentID = globalState.deploymentID;
	if (deploymentID) {
		requestURL.searchParams.set("dpl", deploymentID);
	}
	const response = await window.fetch(requestURL, {
		method: "GET",
		headers: requestHeaders,
		signal: props.signal,
	});
	const hardReloadHeader = response.headers.get("X-Wave-Framework-Reload");
	const redirectHeader = response.headers.get("X-Client-Redirect");
	const buildIDFromResponseHeader = response.headers.get(
		"X-Wave-Framework-Build-Id",
	) as string;
	if (hardReloadHeader) {
		return {
			status: "redirect",
			href: resolveRedirectTargetHref({
				redirectTarget: hardReloadHeader,
				requestURL,
			}),
			buildID: buildIDFromResponseHeader,
			isHardReload: true,
		};
	}
	if (redirectHeader) {
		return {
			status: "redirect",
			href: resolveRedirectTargetHref({
				redirectTarget: redirectHeader,
				requestURL,
			}),
			buildID: buildIDFromResponseHeader,
			isHardReload: false,
		};
	}
	const nativeRedirectTarget = resolveNativeFetchRedirectTarget({
		requestURL,
		response,
	});
	if (nativeRedirectTarget) {
		return {
			status: "redirect",
			href: nativeRedirectTarget,
			buildID: buildIDFromResponseHeader,
			isHardReload: false,
		};
	}
	if (!response.ok) {
		const errorBody = await response.text();
		panic(`Navigation request failed (${response.status}): ${errorBody}.`);
	}
	const routeDataResponseJSON: unknown = await response.json();
	const routeDataPayload =
		parseRuntimeRouteDataJSONPayloadFromResponseBodyOrThrow(
			routeDataResponseJSON,
		);
	const routeSnapshot = decodeRouteDataPayload(routeDataPayload);
	preloadRouteDataAssets(routeSnapshot);
	const buildID = buildIDFromResponseHeader;
	return {
		status: "success",
		routeSnapshot: {
			...routeSnapshot,
			buildID,
		},
		buildID,
	};
}

function joinBaseHrefWithAssetPath(props: {
	baseHref: string;
	assetPath: string;
}): string {
	if (!props.baseHref) {
		return props.assetPath;
	}
	if (props.assetPath.startsWith("/")) {
		return `${props.baseHref}${props.assetPath}`;
	}
	return `${props.baseHref}/${props.assetPath}`;
}

function hasStylesheetForBundle(bundlePath: string): boolean {
	const stylesheets = Array.from(
		document.head.querySelectorAll<HTMLLinkElement>(
			'link[rel="stylesheet"][data-vorma-css-bundle]',
		),
	);
	return stylesheets.some(
		(node) => node.getAttribute("data-vorma-css-bundle") === bundlePath,
	);
}

function hasPreloadForCSSBundle(bundlePath: string): boolean {
	const preloads = Array.from(
		document.head.querySelectorAll<HTMLLinkElement>(
			'link[rel="preload"][as="style"][data-vorma-css-preload-bundle]',
		),
	);
	return preloads.some(
		(node) =>
			node.getAttribute("data-vorma-css-preload-bundle") === bundlePath,
	);
}

function markCSSPreloadNodeAsSettled(node: HTMLLinkElement): void {
	node.setAttribute("data-vorma-css-preload-settled", "1");
}

function getCSSPreloadNodeForBundle(
	bundlePath: string,
): HTMLLinkElement | undefined {
	return Array.from(
		document.head.querySelectorAll<HTMLLinkElement>(
			'link[rel="preload"][as="style"][data-vorma-css-preload-bundle]',
		),
	).find(
		(node) =>
			node.getAttribute("data-vorma-css-preload-bundle") === bundlePath,
	);
}

function ensureCSSPreloadLinksFromRouteDataSnapshot(
	routeSnapshot: RuntimeRouteSnapshot,
): void {
	const cssBundles = new Set(routeSnapshot.cssBundles);
	for (const cssBundlePath of cssBundles) {
		if (hasPreloadForCSSBundle(cssBundlePath)) {
			continue;
		}
		const preloadNode = document.createElement("link");
		preloadNode.setAttribute(
			"data-vorma-css-preload-bundle",
			cssBundlePath,
		);
		preloadNode.setAttribute("rel", "preload");
		preloadNode.setAttribute("as", "style");
		preloadNode.setAttribute("data-vorma-css-preload-managed", "1");
		preloadNode.setAttribute("data-vorma-css-preload-settled", "0");
		preloadNode.addEventListener("load", () => {
			markCSSPreloadNodeAsSettled(preloadNode);
		});
		preloadNode.addEventListener("error", () => {
			markCSSPreloadNodeAsSettled(preloadNode);
		});
		preloadNode.setAttribute("href", cssBundlePath);
		document.head.appendChild(preloadNode);
	}
}

async function waitForCSSPreloadsToSettleForRouteDataSnapshot(props: {
	routeSnapshot: RuntimeRouteSnapshot;
	signal: AbortSignal;
}): Promise<void> {
	const settlePromises: Promise<void>[] = [];
	const cssBundles = new Set(props.routeSnapshot.cssBundles);
	for (const cssBundlePath of cssBundles) {
		const preloadNode = getCSSPreloadNodeForBundle(cssBundlePath);
		if (!preloadNode) {
			continue;
		}
		if (
			preloadNode.getAttribute("data-vorma-css-preload-managed") !== "1"
		) {
			continue;
		}
		if (
			preloadNode.getAttribute("data-vorma-css-preload-settled") === "1"
		) {
			continue;
		}
		settlePromises.push(
			new Promise<void>((resolve) => {
				if (props.signal.aborted) {
					resolve();
					return;
				}
				const settle = () => {
					markCSSPreloadNodeAsSettled(preloadNode);
					cleanup();
					resolve();
				};
				const onAbort = () => {
					cleanup();
					resolve();
				};
				const cleanup = () => {
					preloadNode.removeEventListener("load", settle);
					preloadNode.removeEventListener("error", settle);
					props.signal.removeEventListener("abort", onAbort);
				};
				preloadNode.addEventListener("load", settle, {
					once: true,
				});
				preloadNode.addEventListener("error", settle, {
					once: true,
				});
				props.signal.addEventListener("abort", onAbort, {
					once: true,
				});
			}),
		);
	}
	if (settlePromises.length === 0) {
		return;
	}
	await Promise.all(settlePromises);
}

function ensureModulePreloadsFromRouteDataSnapshot(
	routeSnapshot: RuntimeRouteSnapshot,
): void {
	if (import.meta.env.DEV) {
		return;
	}
	const uniqueDeps = new Set(routeSnapshot.deps);
	for (const depPath of uniqueDeps) {
		const existingPreload = Array.from(
			document.head.querySelectorAll<HTMLLinkElement>(
				'link[rel="modulepreload"]',
			),
		).some((node) => node.getAttribute("href") === depPath);
		if (existingPreload) {
			continue;
		}
		const preloadNode = document.createElement("link");
		preloadNode.setAttribute("rel", "modulepreload");
		preloadNode.setAttribute("href", depPath);
		document.head.appendChild(preloadNode);
	}
}

function preloadRouteDataAssets(routeSnapshot: RuntimeRouteSnapshot): void {
	ensureModulePreloadsFromRouteDataSnapshot(routeSnapshot);
	ensureCSSPreloadLinksFromRouteDataSnapshot(routeSnapshot);
}

function applyCommittedCSSBundlesFromRouteDataSnapshot(
	routeSnapshot: RuntimeRouteSnapshot,
): void {
	const cssBundles = new Set(routeSnapshot.cssBundles);
	for (const cssBundlePath of cssBundles) {
		if (hasStylesheetForBundle(cssBundlePath)) {
			continue;
		}
		const stylesheetNode = document.createElement("link");
		stylesheetNode.setAttribute("rel", "stylesheet");
		stylesheetNode.setAttribute("data-vorma-css-bundle", cssBundlePath);
		stylesheetNode.setAttribute("href", cssBundlePath);
		document.head.appendChild(stylesheetNode);
	}
}

/////////////////////////////////////////////////////////////////////
/////// Component Loading
/////////////////////////////////////////////////////////////////////

export type ModuleExports = Record<string, unknown>;
export type ComponentModulesMap = Map<string, ModuleExports>;

function resolveImportURLForRuntime(importURL: string): string {
	const globalState = getGlobalStateOrThrow();
	if (globalState.viteDevURL) {
		return joinBaseHrefWithAssetPath({
			baseHref: stripTrailingSlash(globalState.viteDevURL),
			assetPath: importURL,
		});
	}
	return importURL;
}

async function loadComponentModules(
	urls: string[],
): Promise<ComponentModulesMap> {
	const modulesMap: ComponentModulesMap = new Map();
	const uniqueImportURLs = Array.from(new Set(urls));
	await Promise.all(
		uniqueImportURLs.map(async (url) => {
			const resolvedImportURL = resolveImportURLForRuntime(url);
			const loadedModule = (await import(
				/* @vite-ignore */ resolvedImportURL
			)) as ModuleExports;
			modulesMap.set(url, loadedModule);
		}),
	);
	return modulesMap;
}

function resolveModuleExportForKey(props: {
	moduleExports: ModuleExports;
	exportKey: string;
}): unknown {
	return props.moduleExports[props.exportKey];
}

function buildActiveComponentsFromModules(props: {
	importURLs: string[];
	exportKeys: string[];
	modulesMap: ComponentModulesMap;
}): unknown[] {
	const activeComponents: unknown[] = [];
	for (let index = 0; index < props.importURLs.length; index += 1) {
		const importURL = props.importURLs[index] as string;
		const exportKey = props.exportKeys[index] as string;
		activeComponents.push(
			resolveModuleExportForKey({
				moduleExports: props.modulesMap.get(importURL) as ModuleExports,
				exportKey,
			}),
		);
	}
	return activeComponents;
}

function resolveActiveErrorBoundaryFromModules(props: {
	importURLs: string[];
	errorExportKeys: string[];
	outermostErrorIdx: Nullable<number>;
	modulesMap: ComponentModulesMap;
}): unknown {
	if (props.outermostErrorIdx == null) {
		return undefined;
	}
	const importURL = props.importURLs[props.outermostErrorIdx] as string;
	const errorExportKey = props.errorExportKeys[
		props.outermostErrorIdx
	] as string;
	if (errorExportKey === "") {
		return getGlobalStateOrThrow().defaultErrorBoundary;
	}
	const resolvedBoundary = resolveModuleExportForKey({
		moduleExports: props.modulesMap.get(importURL) as ModuleExports,
		exportKey: errorExportKey,
	});
	return resolvedBoundary;
}

function shouldReuseCurrentActiveComponentsForRevalidateCommit(props: {
	intent: RuntimeNavigationIntent;
	currentSnapshot: RuntimeRouteSnapshot;
	nextSnapshot: RuntimeRouteSnapshot;
}): boolean {
	if (props.intent !== "revalidate") {
		return false;
	}
	return (
		jsonDeepEquals(
			props.currentSnapshot.importURLs,
			props.nextSnapshot.importURLs,
		) &&
		jsonDeepEquals(
			props.currentSnapshot.exportKeys,
			props.nextSnapshot.exportKeys,
		) &&
		jsonDeepEquals(
			props.currentSnapshot.errorExportKeys,
			props.nextSnapshot.errorExportKeys,
		) &&
		props.currentSnapshot.outermostServerErrorIdx ===
			props.nextSnapshot.outermostServerErrorIdx
	);
}

/////////////////////////////////////////////////////////////////////
/////// Head Management
/////////////////////////////////////////////////////////////////////

type HeadSection = "meta" | "rest";

type HeadSectionCommentPair = {
	startComment?: Comment;
	endComment?: Comment;
};

function getHeadSectionStartToken(type: HeadSection): string {
	return `data-vorma="${type}-start"`;
}

function getHeadSectionEndToken(type: HeadSection): string {
	return `data-vorma="${type}-end"`;
}

function getStartAndEndComments(type: HeadSection): HeadSectionCommentPair {
	const startToken = getHeadSectionStartToken(type);
	const endToken = getHeadSectionEndToken(type);
	let startComment: Comment | undefined;
	for (const node of Array.from(document.head.childNodes)) {
		if (node.nodeType !== Node.COMMENT_NODE) {
			continue;
		}
		const commentNode = node as Comment;
		const commentValue = commentNode.nodeValue?.trim();
		if (commentValue === startToken) {
			startComment = commentNode;
			continue;
		}
		if (commentValue === endToken && startComment) {
			return {
				startComment,
				endComment: commentNode,
			};
		}
	}
	return {};
}

function resolveHeadSectionCommentPairOrThrow(props: { type: HeadSection }): {
	startComment: Comment;
	endComment: Comment;
} {
	const pair = getStartAndEndComments(props.type);
	if (!pair.startComment || !pair.endComment) {
		panic(
			`Managed head section markers for '${props.type}' are missing or invalid.`,
		);
	}
	return {
		startComment: pair.startComment,
		endComment: pair.endComment,
	};
}

function buildBlockAttributeMap(block: HeadEl): Record<string, string> {
	const attributeMap = {
		...block.attributesKnownSafe,
	} as Record<string, string>;
	for (const booleanAttributeName of block.booleanAttributes ?? []) {
		attributeMap[booleanAttributeName] = "";
	}
	return attributeMap;
}

function buildElementAttributeMap(element: Element): Record<string, string> {
	const attributeMap: Record<string, string> = {};
	for (const attribute of Array.from(element.attributes)) {
		attributeMap[attribute.name] = attribute.value;
	}
	return attributeMap;
}

function buildAttributeFingerprint(
	attributeMap: Record<string, string>,
): string {
	return Object.entries(attributeMap)
		.sort(([leftName], [rightName]) => leftName.localeCompare(rightName))
		.map(([name, value]) => `${name}=${value}`)
		.join("|");
}

function buildHeadBlockFingerprint(block: HeadEl): string {
	return [
		block.tag.toLowerCase(),
		buildAttributeFingerprint(buildBlockAttributeMap(block)),
		block.dangerousInnerHTML ?? "",
	].join("::");
}

function buildHeadElementFingerprint(element: Element): string {
	return [
		element.tagName.toLowerCase(),
		buildAttributeFingerprint(buildElementAttributeMap(element)),
		element.innerHTML,
	].join("::");
}

function applyHeadBlockToElement(props: {
	element: Element;
	block: HeadEl;
}): void {
	const desiredAttributes = buildBlockAttributeMap(props.block);
	const currentAttributes = buildElementAttributeMap(props.element);
	for (const attributeName of Object.keys(currentAttributes)) {
		if (attributeName in desiredAttributes) {
			continue;
		}
		props.element.removeAttribute(attributeName);
	}
	for (const [attributeName, attributeValue] of Object.entries(
		desiredAttributes,
	)) {
		if (props.element.getAttribute(attributeName) === attributeValue) {
			continue;
		}
		props.element.setAttribute(attributeName, attributeValue);
	}
	const desiredInnerHTML = props.block.dangerousInnerHTML ?? "";
	if (props.element.innerHTML !== desiredInnerHTML) {
		props.element.innerHTML = desiredInnerHTML;
	}
}

function resolveExistingManagedSectionElements(props: {
	startComment: Comment;
	endComment: Comment;
}): Element[] {
	const elements: Element[] = [];
	let currentNode: Node | null = props.startComment.nextSibling;
	while (currentNode && currentNode !== props.endComment) {
		if (currentNode.nodeType === Node.ELEMENT_NODE) {
			elements.push(currentNode as Element);
		}
		currentNode = currentNode.nextSibling;
	}
	return elements;
}

function resolveExactMatchElementForBlock(props: {
	existingElements: Element[];
	usedElements: Set<Element>;
	block: HeadEl;
}): Element | undefined {
	const blockFingerprint = buildHeadBlockFingerprint(props.block);
	return props.existingElements.find(
		(element) =>
			!props.usedElements.has(element) &&
			buildHeadElementFingerprint(element) === blockFingerprint,
	);
}

function resolveTagMatchElementForBlock(props: {
	existingElements: Element[];
	usedElements: Set<Element>;
	block: HeadEl;
}): Element | undefined {
	const tagName = props.block.tag.toLowerCase();
	return props.existingElements.find(
		(element) =>
			!props.usedElements.has(element) &&
			element.tagName.toLowerCase() === tagName,
	);
}

function buildHeadElementFromBlock(block: HeadEl): Element {
	const element = document.createElement(block.tag);
	for (const [attributeName, attributeValue] of Object.entries(
		buildBlockAttributeMap(block),
	)) {
		element.setAttribute(attributeName, String(attributeValue));
	}
	element.innerHTML = block.dangerousInnerHTML ?? "";
	return element;
}

function removeUnmanagedNodesBetweenComments(props: {
	startComment: Comment;
	endComment: Comment;
	managedElements: Set<Element>;
}): void {
	let currentNode: Node | null = props.startComment.nextSibling;
	while (currentNode && currentNode !== props.endComment) {
		const nextNode = currentNode.nextSibling;
		if (
			currentNode.nodeType === Node.ELEMENT_NODE &&
			props.managedElements.has(currentNode as Element)
		) {
			currentNode = nextNode;
			continue;
		}
		currentNode.parentNode?.removeChild(currentNode);
		currentNode = nextNode;
	}
}

function reconcileHeadElements(type: HeadSection, blocks: HeadEl[]): void {
	const { startComment, endComment } = resolveHeadSectionCommentPairOrThrow({
		type,
	});
	const existingElements = resolveExistingManagedSectionElements({
		startComment,
		endComment,
	});
	const usedElements = new Set<Element>();
	const nextElementsInOrder: Element[] = [];
	for (const block of blocks) {
		const matchedElement =
			resolveExactMatchElementForBlock({
				existingElements,
				usedElements,
				block,
			}) ??
			resolveTagMatchElementForBlock({
				existingElements,
				usedElements,
				block,
			});
		const nextElement = matchedElement ?? buildHeadElementFromBlock(block);
		applyHeadBlockToElement({
			element: nextElement,
			block,
		});
		usedElements.add(nextElement);
		nextElementsInOrder.push(nextElement);
	}
	for (const nextElement of nextElementsInOrder) {
		document.head.insertBefore(nextElement, endComment);
	}
	removeUnmanagedNodesBetweenComments({
		startComment,
		endComment,
		managedElements: new Set(nextElementsInOrder),
	});
}

function updateHeadEls(type: HeadSection, blocks: HeadEl[]): void {
	reconcileHeadElements(type, blocks);
}

function applyRouteHeadAndTitle(snapshot: RuntimeRouteSnapshot): void {
	if (typeof snapshot.title === "string") {
		document.title = snapshot.title;
	}
	updateHeadEls("meta", snapshot.metaHeadEls);
	updateHeadEls("rest", snapshot.restHeadEls);
}

/////////////////////////////////////////////////////////////////////
/////// Client Loader Runtime
/////////////////////////////////////////////////////////////////////

function buildAbortErrorForClientLoaderServerData(): Error {
	const abortError = new Error(
		"Client loader serverDataPromise aborted because scoped server data is unavailable.",
	);
	(abortError as AnyRecord).name = "AbortError";
	return abortError;
}

const unhandledPromiseRejectionGuardSet = new WeakSet<Promise<unknown>>();

function suppressUnhandledPromiseRejection(promise: Promise<unknown>): void {
	if (unhandledPromiseRejectionGuardSet.has(promise)) {
		return;
	}
	unhandledPromiseRejectionGuardSet.add(promise);
	void promise.catch(() => undefined);
}

type ClientLoaderPrestart = {
	matchedPattern: string;
	clientLoaderResultPromise: Promise<unknown>;
	resolveFromSnapshot: (snapshot: RuntimeRouteSnapshot) => void;
	abortIfPending: () => void;
};

type MatchedClientLoaderTarget = {
	matchedPattern: string;
	params: Record<string, string>;
	splatValues: string[];
	waitFn: ClientLoaderWaitFn;
};

function resolveMatchedClientLoaderTarget(props: {
	targetUrl: string;
}): MatchedClientLoaderTarget | null {
	const globalState = getGlobalStateOrThrow();
	const targetPathname = new URL(props.targetUrl, window.location.href)
		.pathname;
	const match = findBestMatch(globalState.patternRegistry, targetPathname);
	if (!match) {
		return null;
	}
	const matchedPattern = match.registeredPattern.originalPattern;
	const waitFn = globalState.patternToWaitFnMap[matchedPattern];
	if (!waitFn) {
		return null;
	}
	return {
		matchedPattern,
		params: match.params,
		splatValues: match.splatValues,
		waitFn,
	};
}

function invokeClientLoaderWaitFnWithoutUnhandledRejection(props: {
	waitFn: ClientLoaderWaitFn;
	params: Record<string, string>;
	splatValues: string[];
	serverDataPromise: Promise<{
		matchedPatterns: string[];
		rootData: unknown;
		loaderData: unknown;
		buildID: string;
	}>;
	signal: AbortSignal;
}): Promise<unknown> {
	const waitFnPromise = Promise.resolve(
		props.waitFn({
			params: props.params,
			splatValues: props.splatValues,
			serverDataPromise: props.serverDataPromise,
			signal: props.signal,
		}),
	);
	suppressUnhandledPromiseRejection(waitFnPromise);
	return waitFnPromise;
}

function createClientLoaderPrestart(props: {
	targetUrl: string;
	signal: AbortSignal;
}): ClientLoaderPrestart | null {
	const matchedTarget = resolveMatchedClientLoaderTarget({
		targetUrl: props.targetUrl,
	});
	if (!matchedTarget) {
		return null;
	}
	const matchedPatternKey = matchedTarget.matchedPattern;

	let settled = false;
	let resolveServerDataPromise: (value: {
		matchedPatterns: string[];
		rootData: unknown;
		loaderData: unknown;
		buildID: string;
	}) => void;
	let rejectServerDataPromise: (error: unknown) => void;

	const serverDataPromise = new Promise<{
		matchedPatterns: string[];
		rootData: unknown;
		loaderData: unknown;
		buildID: string;
	}>((resolve, reject) => {
		resolveServerDataPromise = resolve;
		rejectServerDataPromise = reject;
	});
	suppressUnhandledPromiseRejection(serverDataPromise);

	const clientLoaderResultPromise =
		invokeClientLoaderWaitFnWithoutUnhandledRejection({
			waitFn: matchedTarget.waitFn,
			params: matchedTarget.params,
			splatValues: matchedTarget.splatValues,
			serverDataPromise,
			signal: props.signal,
		});

	const abortIfPending = () => {
		if (settled) {
			return;
		}
		settled = true;
		rejectServerDataPromise(buildAbortErrorForClientLoaderServerData());
	};

	props.signal.addEventListener("abort", abortIfPending, {
		once: true,
	});

	return {
		matchedPattern: matchedPatternKey,
		clientLoaderResultPromise,
		resolveFromSnapshot: (snapshot) => {
			if (settled) {
				return;
			}
			const matchedIndex =
				snapshot.matchedPatterns.indexOf(matchedPatternKey);
			settled = true;
			resolveServerDataPromise({
				matchedPatterns: snapshot.matchedPatterns,
				rootData: snapshot.hasRootData ? snapshot.loadersData[0] : null,
				loaderData: snapshot.loadersData[matchedIndex],
				buildID: snapshot.buildID,
			});
		},
		abortIfPending,
	};
}

async function completeClientLoaders(props: {
	nextSnapshot: RuntimeRouteSnapshot;
	signal: AbortSignal;
	prestartedClientLoaderResultPromiseByPattern?: Record<
		string,
		Promise<unknown>
	>;
}): Promise<{
	clientLoadersData: unknown[];
	outermostClientError: unknown;
	outermostClientErrorIdx: Nullable<number>;
}> {
	const globalState = getGlobalStateOrThrow();
	const patternToWaitFnMap = globalState.patternToWaitFnMap;
	const prestartedClientLoaderResultPromiseByPattern =
		props.prestartedClientLoaderResultPromiseByPattern ?? {};
	const clientLoadersData: unknown[] = Array.from(
		{
			length: props.nextSnapshot.matchedPatterns.length,
		},
		() => undefined,
	);
	let outermostClientError: unknown = undefined;
	let outermostClientErrorIdx: Nullable<number> = undefined;

	await Promise.all(
		props.nextSnapshot.matchedPatterns.map(async (pattern, index) => {
			const prestartedClientLoaderResultPromise =
				prestartedClientLoaderResultPromiseByPattern[pattern];
			if (prestartedClientLoaderResultPromise !== undefined) {
				try {
					clientLoadersData[index] =
						await prestartedClientLoaderResultPromise;
				} catch (error) {
					if (outermostClientErrorIdx == null) {
						outermostClientError = error;
						outermostClientErrorIdx = index;
					}
				}
				return;
			}
			const waitFn = patternToWaitFnMap[pattern];
			if (!waitFn) {
				return;
			}
			const serverDataPromise = Promise.resolve({
				matchedPatterns: props.nextSnapshot.matchedPatterns,
				rootData: props.nextSnapshot.hasRootData
					? props.nextSnapshot.loadersData[0]
					: null,
				loaderData: props.nextSnapshot.loadersData[index],
				buildID: props.nextSnapshot.buildID,
			});

			try {
				clientLoadersData[index] = await waitFn({
					params: props.nextSnapshot.params,
					splatValues: props.nextSnapshot.splatValues,
					serverDataPromise,
					signal: props.signal,
				});
			} catch (error) {
				if (outermostClientErrorIdx == null) {
					outermostClientError = error;
					outermostClientErrorIdx = index;
				}
			}
		}),
	);

	return {
		clientLoadersData,
		outermostClientError,
		outermostClientErrorIdx,
	};
}

export function registerClientLoaderForAdapter(
	props: VormaTypedAdapterAddClientLoaderProps<any, any, any, any>,
): void {
	const globalState = getGlobalStateOrThrow();
	globalState.patternToWaitFnMap[props.pattern] =
		props.clientLoader as ClientLoaderWaitFn;
	registerPattern(globalState.patternRegistry, props.pattern);
	if (import.meta.env.DEV && props.reRunOnModuleChange) {
		void import("vorma/client/__internal/hmr_dev")
			.then(
				({ registerClientLoaderModuleForHMRRerunFromAdapterProps }) => {
					registerClientLoaderModuleForHMRRerunFromAdapterProps({
						pattern: props.pattern,
						reRunOnModuleChange: props.reRunOnModuleChange,
					});
				},
			)
			.catch(() => undefined);
	}
}

/////////////////////////////////////////////////////////////////////
/////// Route Outlet Runtime
/////////////////////////////////////////////////////////////////////

function buildRouteOutletRouteKey(props: {
	pattern: string;
	importURL: string;
	exportKey: string;
}): string {
	return `${props.pattern}::${props.importURL}::${props.exportKey}`;
}

function buildRouteOutletBranchInputStateFromSnapshot(
	snapshot: RuntimeRouteSnapshot,
): RouteOutletBranchInputState {
	const routeKeys: string[] = [];
	for (let index = 0; index < snapshot.matchedPatterns.length; index += 1) {
		const pattern = snapshot.matchedPatterns[index] as string;
		const importURL = snapshot.importURLs[index] as string;
		const exportKey = snapshot.exportKeys[index] as string;
		routeKeys.push(
			buildRouteOutletRouteKey({ pattern, importURL, exportKey }),
		);
	}
	return {
		routeKeys,
		components: snapshot.activeComponents,
		errorComponents: snapshot.errorExportKeys.map(
			(errorExportKey, index) => {
				if (!errorExportKey) {
					return undefined;
				}
				if (snapshot.outermostErrorIdx !== index) {
					return undefined;
				}
				return snapshot.activeErrorBoundary;
			},
		),
		matchedPatterns: snapshot.matchedPatterns,
		outermostErrorIdx: snapshot.outermostErrorIdx,
	};
}

function buildCurrentRouteOutletNavigationState(): RouteOutletNavigationState {
	const snapshot = getRuntimeRouteSnapshot();
	return {
		loadersData: snapshot.loadersData,
		clientLoadersData: snapshot.clientLoadersData,
		routerData: buildRouterDataSnapshotView(snapshot),
		outermostError: snapshot.outermostError,
		outermostErrorIdx: snapshot.outermostErrorIdx,
		activeComponents: snapshot.activeComponents,
		activeErrorBoundary: snapshot.activeErrorBoundary,
		importURLs: snapshot.importURLs,
		exportKeys: snapshot.exportKeys,
		matchedPatterns: snapshot.matchedPatterns,
	};
}

export function buildInitialRouteOutletStoreState(): RouteOutletStoreState {
	const snapshot = getRuntimeRouteSnapshot();
	return {
		navigation: buildCurrentRouteOutletNavigationState(),
		routeOutletBranchInputState:
			buildRouteOutletBranchInputStateFromSnapshot(snapshot),
		location: getLocation(),
	};
}

function buildNextRouteOutletStoreStateFromRuntime(
	currentStoreState: RouteOutletStoreState,
): RouteOutletStoreState {
	const snapshot = getRuntimeRouteSnapshot();
	const nextNavigationState = buildCurrentRouteOutletNavigationState();
	const nextRouteOutletBranchInputState =
		buildRouteOutletBranchInputStateFromSnapshot(snapshot);
	const nextLocationState = getLocation();
	const resolvedNavigationState = jsonDeepEquals(
		currentStoreState.navigation,
		nextNavigationState,
	)
		? currentStoreState.navigation
		: nextNavigationState;
	const resolvedRouteOutletBranchInputState = jsonDeepEquals(
		currentStoreState.routeOutletBranchInputState,
		nextRouteOutletBranchInputState,
	)
		? currentStoreState.routeOutletBranchInputState
		: nextRouteOutletBranchInputState;
	const resolvedLocationState = jsonDeepEquals(
		currentStoreState.location,
		nextLocationState,
	)
		? currentStoreState.location
		: nextLocationState;
	const nextStoreState: RouteOutletStoreState = {
		navigation: resolvedNavigationState,
		routeOutletBranchInputState: resolvedRouteOutletBranchInputState,
		location: resolvedLocationState,
	};
	if (
		nextStoreState.navigation === currentStoreState.navigation &&
		nextStoreState.routeOutletBranchInputState ===
			currentStoreState.routeOutletBranchInputState &&
		nextStoreState.location === currentStoreState.location
	) {
		return currentStoreState;
	}
	return nextStoreState;
}

function syncRouteOutletStoreStateFromRuntime(props: {
	getCurrentStoreState: () => RouteOutletStoreState;
	applyNextStoreState: (nextStoreState: RouteOutletStoreState) => void;
}): void {
	const nextStoreState = buildNextRouteOutletStoreStateFromRuntime(
		props.getCurrentStoreState(),
	);
	if (nextStoreState !== props.getCurrentStoreState()) {
		props.applyNextStoreState(nextStoreState);
	}
}

export function shouldRemountRouteOutletComponentMount(props: {
	idx: number;
	previousRouteKey: string | undefined;
	nextRouteKey: string;
	previousRouteComponent: unknown;
	nextRouteComponent: unknown;
	previousMatchedPattern: string | undefined;
	nextMatchedPattern: string;
}): boolean {
	void props.idx;
	return (
		props.previousRouteKey !== props.nextRouteKey ||
		props.previousRouteComponent !== props.nextRouteComponent ||
		props.previousMatchedPattern !== props.nextMatchedPattern
	);
}

export function resolveRouteOutletComponentMountKey(props: {
	routeKey: string;
	matchedPattern: string;
}): string {
	return `${props.routeKey}::${props.matchedPattern}`;
}

export type RouteOutletAdapterSyncHost = {
	syncRootOutletMount: (props: { idx: number }) => void;
	syncRootOutletUnmount: () => void;
};

export function createRouteOutletAdapterSyncHost(props: {
	getCurrentStoreState: () => RouteOutletStoreState;
	applyNextStoreState: (nextStoreState: RouteOutletStoreState) => void;
}): RouteOutletAdapterSyncHost {
	let isRootMounted = false;
	let hostRuntimeGlobalState: VormaClientGlobal | undefined;
	let hasInitializedListeners = false;
	let removeRouteChangeListener: (() => void) | undefined;
	let removeLocationListener: (() => void) | undefined;
	const captureHostRuntimeGlobalState = () => {
		hostRuntimeGlobalState = (globalThis as AnyPropertyRecord)[
			VORMA_SYMBOL
		] as VormaClientGlobal | undefined;
	};
	const isHostRuntimeStillActive = (): boolean => {
		if (!hostRuntimeGlobalState) {
			return false;
		}
		return (
			(globalThis as AnyPropertyRecord)[VORMA_SYMBOL] ===
			hostRuntimeGlobalState
		);
	};
	const syncStore = () => {
		if (!isHostRuntimeStillActive()) {
			cleanupListeners();
			isRootMounted = false;
			return;
		}
		syncRouteOutletStoreStateFromRuntime({
			getCurrentStoreState: props.getCurrentStoreState,
			applyNextStoreState: props.applyNextStoreState,
		});
	};
	const initializeListeners = () => {
		if (hasInitializedListeners) {
			return;
		}
		hasInitializedListeners = true;
		removeRouteChangeListener = addRouteChangeListener(() => {
			if (!isHostRuntimeStillActive()) {
				cleanupListeners();
				isRootMounted = false;
				return;
			}
			if (!isRootMounted) {
				return;
			}
			syncStore();
		});
		removeLocationListener = addLocationListener(() => {
			if (!isHostRuntimeStillActive()) {
				cleanupListeners();
				isRootMounted = false;
				return;
			}
			if (!isRootMounted) {
				return;
			}
			syncStore();
		});
	};
	const cleanupListeners = () => {
		removeRouteChangeListener?.();
		removeRouteChangeListener = undefined;
		removeLocationListener?.();
		removeLocationListener = undefined;
		hasInitializedListeners = false;
	};

	return {
		syncRootOutletMount: () => {
			captureHostRuntimeGlobalState();
			if (isRootMounted) {
				syncStore();
				return;
			}
			isRootMounted = true;
			initializeListeners();
			syncStore();
		},
		syncRootOutletUnmount: () => {
			if (!isRootMounted) {
				return;
			}
			isRootMounted = false;
		},
	};
}

export type RouteOutletAdapterRenderModel =
	| {
			renderKind: "component";
			currentComponent: unknown;
			currentRouteKey: string;
			currentRouteMountKey: string;
			nextRouteKey: string;
			matchedPattern: string;
	  }
	| {
			renderKind: "error";
			errorComponent: unknown;
			currentRouteKey: string;
			nextRouteKey: string;
			outermostError: unknown;
	  }
	| {
			renderKind: "missing-component";
			currentRouteKey: string;
			nextRouteKey: string;
	  }
	| {
			renderKind: "fallback";
			nextRouteKey: string;
	  }
	| {
			renderKind: "empty";
			nextRouteKey: string;
	  };

export function resolveRouteOutletAdapterRenderModel(props: {
	routeOutletBranchInputState: RouteOutletBranchInputState;
	outermostError: unknown;
	idx: number;
}): RouteOutletAdapterRenderModel {
	const currentRouteKey =
		props.routeOutletBranchInputState.routeKeys[props.idx] ??
		`__route_${props.idx}_empty`;
	const nextRouteKey =
		props.routeOutletBranchInputState.routeKeys[props.idx + 1] ??
		`__route_${props.idx + 1}_empty`;
	const routeCount = props.routeOutletBranchInputState.routeKeys.length;
	if (props.idx > routeCount) {
		return {
			renderKind: "empty",
			nextRouteKey,
		};
	}
	if (props.idx === routeCount) {
		return {
			renderKind: "fallback",
			nextRouteKey,
		};
	}
	const outermostErrorIdx =
		props.routeOutletBranchInputState.outermostErrorIdx;
	if (outermostErrorIdx != null && props.idx >= outermostErrorIdx) {
		return {
			renderKind: "error",
			errorComponent:
				props.routeOutletBranchInputState.errorComponents[props.idx],
			currentRouteKey,
			nextRouteKey,
			outermostError: props.outermostError,
		};
	}
	const currentComponent =
		props.routeOutletBranchInputState.components[props.idx];
	if (!currentComponent) {
		return {
			renderKind: "missing-component",
			currentRouteKey,
			nextRouteKey,
		};
	}
	const matchedPattern = props.routeOutletBranchInputState.matchedPatterns[
		props.idx
	] as string;
	return {
		renderKind: "component",
		currentComponent,
		currentRouteKey,
		currentRouteMountKey: resolveRouteOutletComponentMountKey({
			routeKey: currentRouteKey,
			matchedPattern,
		}),
		nextRouteKey,
		matchedPattern,
	};
}

export function renderRouteOutletAdapterRenderModel<RenderOutput>(props: {
	routeOutletRenderModel: RouteOutletAdapterRenderModel;
	renderErrorWithBoundary: (props: {
		errorComponent: unknown;
		outermostError: unknown;
	}) => RenderOutput;
	renderErrorWithoutBoundary: (props: {
		outermostError: unknown;
	}) => RenderOutput;
	renderComponent: (props: {
		currentComponent: unknown;
		currentRouteKey: string;
		currentRouteMountKey: string;
		matchedPattern: string;
	}) => RenderOutput;
	renderMissingComponent: () => RenderOutput;
	renderFallback: (props: { nextRouteKey: string }) => RenderOutput;
	renderEmpty: () => RenderOutput;
}): RenderOutput {
	const routeOutletRenderModel = props.routeOutletRenderModel;
	switch (routeOutletRenderModel.renderKind) {
		case "component":
			return props.renderComponent({
				currentComponent: routeOutletRenderModel.currentComponent,
				currentRouteKey: routeOutletRenderModel.currentRouteKey,
				currentRouteMountKey:
					routeOutletRenderModel.currentRouteMountKey,
				matchedPattern: routeOutletRenderModel.matchedPattern,
			});
		case "error":
			if (routeOutletRenderModel.errorComponent) {
				return props.renderErrorWithBoundary({
					errorComponent: routeOutletRenderModel.errorComponent,
					outermostError: routeOutletRenderModel.outermostError,
				});
			}
			return props.renderErrorWithoutBoundary({
				outermostError: routeOutletRenderModel.outermostError,
			});
		case "missing-component":
			return props.renderMissingComponent();
		case "fallback":
			return props.renderFallback({
				nextRouteKey: routeOutletRenderModel.nextRouteKey,
			});
		case "empty":
			return props.renderEmpty();
	}
}

/////////////////////////////////////////////////////////////////////
/////// Typed Adapter Route Scope
/////////////////////////////////////////////////////////////////////

type TypedAdapterRouteScope = {
	index: number;
	readonly matchedPattern: string;
};

const TYPED_ADAPTER_ROUTE_SCOPE_KEY = "__vorma_internal_route_scope";
const typedAdapterRouteScopeRecordsByIndex: TypedAdapterRouteScope[] = [];

function getOrCreateTypedAdapterRouteScopeRecord(props: {
	routePropsIndex: number;
}): TypedAdapterRouteScope {
	const existingRecord =
		typedAdapterRouteScopeRecordsByIndex[props.routePropsIndex];
	if (existingRecord !== undefined) {
		return existingRecord;
	}
	const nextRecord: TypedAdapterRouteScope = {
		index: props.routePropsIndex,
		get matchedPattern() {
			return getRuntimeRouteSnapshot().matchedPatterns[
				props.routePropsIndex
			] as string;
		},
	};
	typedAdapterRouteScopeRecordsByIndex[props.routePropsIndex] = nextRecord;
	return nextRecord;
}

function getTypedAdapterRouteScope(
	routeProps: Record<string, unknown>,
): TypedAdapterRouteScope {
	return routeProps[TYPED_ADAPTER_ROUTE_SCOPE_KEY] as TypedAdapterRouteScope;
}

export function buildTypedAdapterRouteComponentMountProps(props: {
	routePropsIndex: number;
	matchedPattern: string;
}): Record<string, unknown> {
	const routeScopeRecord = getOrCreateTypedAdapterRouteScopeRecord({
		routePropsIndex: props.routePropsIndex,
	});
	void props.matchedPattern;
	return {
		[TYPED_ADAPTER_ROUTE_SCOPE_KEY]: routeScopeRecord,
	};
}

export function resolveTypedAdapterLoaderDataForRoutePropsOrThrow<Data>(props: {
	loadersData: unknown[];
	matchedPatterns: string[];
	routeProps: Record<string, unknown>;
}): Data {
	const routeScope = getTypedAdapterRouteScope(props.routeProps);
	const expectedMatchedPattern = routeScope.matchedPattern;
	const actualMatchedPattern = props.matchedPatterns[routeScope.index];
	if (actualMatchedPattern !== expectedMatchedPattern) {
		panic(
			`Route-scoped loader data access failed: expected matched pattern "${expectedMatchedPattern}" at index ${routeScope.index}, received "${actualMatchedPattern ?? "undefined"}".`,
		);
	}
	return props.loadersData[routeScope.index] as Data;
}

function resolvePatternCandidates(props: {
	pattern: string;
	explicitIndexSegmentIdentifier: string;
}): string[] {
	const explicitIndexPattern = `/${props.explicitIndexSegmentIdentifier}`;
	if (props.pattern === "/" || props.pattern.endsWith(explicitIndexPattern)) {
		return [props.pattern];
	}
	return [props.pattern, `${props.pattern}${explicitIndexPattern}`];
}

export function resolveTypedAdapterIndexedDataForPattern<Data>(props: {
	indexedData: unknown[];
	matchedPatterns: string[];
	pattern: string;
	explicitIndexSegmentIdentifier?: string;
}): Data | undefined {
	const globalState = getGlobalStateOrThrow();
	const explicitIndexSegmentIdentifier =
		props.explicitIndexSegmentIdentifier ??
		globalState.vormaAppConfig.loadersExplicitIndexSegmentIdentifier;
	const patternCandidates = resolvePatternCandidates({
		pattern: props.pattern,
		explicitIndexSegmentIdentifier,
	});
	const matchedPatternIndex = props.matchedPatterns.findIndex(
		(matchedPattern) => patternCandidates.includes(matchedPattern),
	);
	if (matchedPatternIndex < 0) {
		return undefined;
	}
	return props.indexedData[matchedPatternIndex] as Data | undefined;
}

export function resolveTypedAdapterClientLoaderDataForPatternOrRouteProps<
	Data,
>(props: {
	clientLoadersData: unknown[];
	matchedPatterns: string[];
	pattern: string;
	routeProps?: Record<string, unknown>;
	explicitIndexSegmentIdentifier?: string;
}): Data | undefined {
	if (props.routeProps) {
		return resolveTypedAdapterLoaderDataForRoutePropsOrThrow<Data>({
			loadersData: props.clientLoadersData,
			matchedPatterns: props.matchedPatterns,
			routeProps: props.routeProps,
		});
	}
	return resolveTypedAdapterIndexedDataForPattern<Data>({
		indexedData: props.clientLoadersData,
		matchedPatterns: props.matchedPatterns,
		pattern: props.pattern,
		explicitIndexSegmentIdentifier: props.explicitIndexSegmentIdentifier,
	});
}

export function createTypedAdapterValueHookFactories<
	App extends ExtractApp<VormaAppConfig>,
>(props: {
	useLoadersData: () => unknown[];
	useMatchedPatterns: () => string[];
	useClientLoadersData: () => unknown[];
	useMemoizedPatternLoaderData?: <Data>(props: {
		resolvePatternLoaderData: () => Data;
		dependencies: unknown[];
	}) => Data;
}) {
	function makeUseLoaderDataHook() {
		return function useLoaderData<Pattern extends VormaLoaderPattern<App>>(
			routeProps: VormaRoutePropsGeneric<any, App, Pattern>,
		): VormaLoaderOutput<App, Pattern> {
			return resolveTypedAdapterLoaderDataForRoutePropsOrThrow({
				loadersData: props.useLoadersData(),
				matchedPatterns: props.useMatchedPatterns(),
				routeProps: routeProps as Record<string, unknown>,
			}) as VormaLoaderOutput<App, Pattern>;
		};
	}

	function makeUsePatternLoaderDataHook() {
		return function usePatternLoaderData<
			Pattern extends VormaLoaderPattern<App>,
		>(pattern: Pattern): VormaLoaderOutput<App, Pattern> | undefined {
			const indexedLoadersData = props.useLoadersData();
			const matchedPatterns = props.useMatchedPatterns();
			const resolvePatternLoaderData = () =>
				resolveTypedAdapterIndexedDataForPattern<
					VormaLoaderOutput<App, Pattern>
				>({
					indexedData: indexedLoadersData,
					matchedPatterns,
					pattern,
				});
			if (props.useMemoizedPatternLoaderData) {
				return props.useMemoizedPatternLoaderData({
					resolvePatternLoaderData,
					dependencies: [
						pattern,
						indexedLoadersData,
						matchedPatterns,
					],
				});
			}
			return resolvePatternLoaderData();
		};
	}

	function makeAddClientLoaderHook() {
		return function addClientLoader<
			Pattern extends VormaLoaderPattern<App>,
			LoaderData extends VormaLoaderOutput<App, Pattern>,
			ResultData,
		>(
			addClientLoaderProps: VormaTypedAdapterAddClientLoaderProps<
				App,
				Pattern,
				LoaderData,
				ResultData
			>,
		) {
			registerClientLoaderForAdapter(addClientLoaderProps);
			const useClientLoaderData = (
				routeProps?: VormaRoutePropsGeneric<any, App, Pattern>,
			) => {
				return resolveTypedAdapterClientLoaderDataForPatternOrRouteProps<ResultData>(
					{
						clientLoadersData: props.useClientLoadersData(),
						matchedPatterns: props.useMatchedPatterns(),
						pattern: addClientLoaderProps.pattern,
						routeProps: routeProps as
							| Record<string, unknown>
							| undefined,
					},
				);
			};
			return useClientLoaderData as {
				(props: VormaRoutePropsGeneric<any, App, Pattern>): ResultData;
				(): ResultData | undefined;
			};
		};
	}

	return {
		makeUseLoaderDataHook,
		makeUsePatternLoaderDataHook,
		makeAddClientLoaderHook,
	};
}

/////////////////////////////////////////////////////////////////////
/////// Navigation Link Runtime
/////////////////////////////////////////////////////////////////////

function normalizedHashFragmentFromHash(hash: string): string {
	const fragment = hash.startsWith("#") ? hash.slice(1) : hash;
	try {
		return decodeURIComponent(fragment);
	} catch {
		return fragment;
	}
}

function hrefWithoutHash(href: string): string {
	const url = new URL(href, window.location.href);
	url.hash = "";
	return url.href;
}

function classifyNavigationTargetAgainstCurrentLocation(props: {
	targetHref: string;
	currentHref: string;
}): "same-document-noop" | "same-document-hash-change" | "requires-fetch" {
	const targetURL = new URL(props.targetHref, props.currentHref);
	const currentURL = new URL(props.currentHref);
	const targetWithoutHash = hrefWithoutHash(targetURL.href);
	const currentWithoutHash = hrefWithoutHash(currentURL.href);
	if (targetWithoutHash !== currentWithoutHash) {
		return "requires-fetch";
	}
	return normalizedHashFragmentFromHash(targetURL.hash) ===
		normalizedHashFragmentFromHash(currentURL.hash)
		? "same-document-noop"
		: "same-document-hash-change";
}

type NavigateProps = {
	href: string | URL;
	replace?: boolean;
	scrollToTop?: boolean;
	state?: unknown;
	intent?: RuntimeNavigationIntent;
	skipGlobalLoadingIndicator?: boolean;
	skipHistoryCommit?: boolean;
	currentHrefForClassification?: string;
};

type InternalNavigateProps = NavigateProps & {
	redirectHopCount?: number;
};

function getTargetDataKey(targetHref: string): string {
	const targetURL = new URL(targetHref, window.location.href);
	return `${targetURL.pathname}${targetURL.search}`;
}

function hasRevalidationTargetOwnership(props: {
	intent: RuntimeNavigationIntent;
	targetUrl: string;
}): boolean {
	if (props.intent !== "revalidate") {
		return true;
	}
	const currentDataKey = getTargetDataKey(window.location.href);
	const targetDataKey = getTargetDataKey(props.targetUrl);
	return currentDataKey === targetDataKey;
}

function getCurrentRootElementID(): string {
	return getRuntimeRouteSnapshot().rootElementID ?? "vorma-root";
}

function emitStatusIfChanged(store: NavigationRuntimeStore): void {
	const nextStatus: StatusEventDetail = {
		isNavigating: store.navigateOperation !== null,
		isSubmitting:
			resolveShouldRunSubmittingGlobalLoadingIndicatorFromStore(store),
		isRevalidating: store.revalidateOperation !== null,
	};
	if (jsonDeepEquals(store.lastStatus, nextStatus)) {
		return;
	}
	store.lastStatus = nextStatus;
	dispatchStatusEvent(nextStatus);
}

function resolveShouldRunSubmittingGlobalLoadingIndicatorFromStore(
	store: NavigationRuntimeStore,
): boolean {
	for (const submissionOperationID of store.activeSubmissionOperationIDs) {
		if (
			!store.skippedGlobalLoadingIndicatorSubmissionOperationIDs.has(
				submissionOperationID,
			)
		) {
			return true;
		}
	}
	return false;
}

function resolveGlobalLoadingIndicatorStatusFromStore(
	store: NavigationRuntimeStore,
): StatusEventDetail {
	return {
		isNavigating:
			store.navigateOperation !== null &&
			!store.skippedGlobalLoadingIndicatorNavigationOperationIDs.has(
				store.navigateOperation.id,
			),
		isSubmitting:
			resolveShouldRunSubmittingGlobalLoadingIndicatorFromStore(store),
		isRevalidating:
			store.revalidateOperation !== null &&
			!store.skippedGlobalLoadingIndicatorNavigationOperationIDs.has(
				store.revalidateOperation.id,
			),
	};
}

function createNavigationRuntimeStore(): NavigationRuntimeStore {
	return {
		navigateOperation: null,
		revalidateOperation: null,
		queuedRevalidateTargetDataKey: null,
		queuedRevalidateSettledPromise: null,
		prefetchOperationsByDataTarget: new Map(),
		prefetchCacheByDataTarget: new Map(),
		skippedGlobalLoadingIndicatorNavigationOperationIDs: new Set(),
		skippedGlobalLoadingIndicatorSubmissionOperationIDs: new Set(),
		nextNavigationOperationID: 1,
		nextSubmissionOperationID: 1,
		latestStartedSubmissionOperationID: 0,
		activeSubmissionOperationIDs: new Set(),
		submissionAbortControllerByOperationID: new Map(),
		submissionOperationIDByDedupeKey: new Map(),
		lastStatus: {
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		},
		lastNavOrRevalidateTimestampMS: Date.now(),
	};
}

function getNavigationStore(): NavigationRuntimeStore {
	const globalState = getGlobalStateOrThrow();
	const navigationStateManager = globalState.navigationStateManager;
	if (!navigationStateManager) {
		panic("Navigation runtime is not initialized.");
	}
	return navigationStateManager.getUnsafeNavigationStore();
}

function runHistoryCommit(props: {
	targetUrl: string;
	replace?: boolean;
	state?: unknown;
}): void {
	const history = getOrCreateHistoryInstance();
	saveStoredScrollState();
	if (props.replace) {
		history.replace(props.targetUrl, props.state);
	} else {
		history.push(props.targetUrl, props.state);
	}
	dispatchLocationEvent(getLocation());
}

function syncBuildIDIfChanged(buildID: string): void {
	setRuntimeBuildID({
		nextBuildID: buildID,
	});
}

function runNavigationCommitSideEffectsWithOptionalViewTransition(props: {
	runCommitSideEffects: () => void;
	shouldUseViewTransition: boolean;
}): void {
	const globalState = getGlobalStateOrThrow();
	const documentWithViewTransition = document as Document & {
		startViewTransition?: (callback: () => void) => unknown;
	};
	if (
		!props.shouldUseViewTransition ||
		!globalState.useViewTransitions ||
		typeof documentWithViewTransition.startViewTransition !== "function"
	) {
		props.runCommitSideEffects();
		return;
	}
	documentWithViewTransition.startViewTransition(() => {
		props.runCommitSideEffects();
	});
}

async function commitSuccessfulNavigation(props: {
	store: NavigationRuntimeStore;
	navigationProps: NavigateProps;
	targetUrl: string;
	routeSnapshot: RuntimeRouteSnapshot;
	buildID: string;
	intent: RuntimeNavigationIntent;
	shouldCommitHistory: boolean;
	signal: AbortSignal;
	modulesMapOverride?: ComponentModulesMap;
	prestartedClientLoaderResultPromiseByPattern?: Record<
		string,
		Promise<unknown>
	>;
}): Promise<void> {
	const currentSnapshot = getRuntimeRouteSnapshot();
	const shouldReuseCurrentActiveComponents =
		shouldReuseCurrentActiveComponentsForRevalidateCommit({
			intent: props.intent,
			currentSnapshot,
			nextSnapshot: props.routeSnapshot,
		});
	let activeComponents: unknown[];
	let activeErrorBoundary: unknown;
	if (shouldReuseCurrentActiveComponents) {
		activeComponents = currentSnapshot.activeComponents;
		activeErrorBoundary = currentSnapshot.activeErrorBoundary;
	} else {
		const modulesMap =
			props.modulesMapOverride ??
			(await loadComponentModules(props.routeSnapshot.importURLs));
		activeComponents = buildActiveComponentsFromModules({
			importURLs: props.routeSnapshot.importURLs,
			exportKeys: props.routeSnapshot.exportKeys,
			modulesMap,
		});
		activeErrorBoundary = resolveActiveErrorBoundaryFromModules({
			importURLs: props.routeSnapshot.importURLs,
			errorExportKeys: props.routeSnapshot.errorExportKeys,
			outermostErrorIdx: props.routeSnapshot.outermostServerErrorIdx,
			modulesMap,
		});
	}
	const clientLoaderResult = await completeClientLoaders({
		nextSnapshot: props.routeSnapshot,
		signal: props.signal,
		prestartedClientLoaderResultPromiseByPattern:
			props.prestartedClientLoaderResultPromiseByPattern,
	});
	await waitForCSSPreloadsToSettleForRouteDataSnapshot({
		routeSnapshot: props.routeSnapshot,
		signal: props.signal,
	});

	const effectiveOutermostError =
		clientLoaderResult.outermostClientError ??
		props.routeSnapshot.outermostServerError;
	const effectiveOutermostErrorIdx =
		clientLoaderResult.outermostClientErrorIdx ??
		props.routeSnapshot.outermostServerErrorIdx;

	syncBuildIDIfChanged(props.buildID);
	const committedSnapshot = setRuntimeRouteSnapshot({
		...props.routeSnapshot,
		buildID: getRuntimeRouteSnapshot().buildID,
		activeComponents,
		activeErrorBoundary,
		clientLoadersData: clientLoaderResult.clientLoadersData,
		outermostClientError: clientLoaderResult.outermostClientError,
		outermostClientErrorIdx: clientLoaderResult.outermostClientErrorIdx,
		outermostError: effectiveOutermostError,
		outermostErrorIdx: effectiveOutermostErrorIdx,
	});
	runNavigationCommitSideEffectsWithOptionalViewTransition({
		shouldUseViewTransition: props.intent === "navigate",
		runCommitSideEffects: () => {
			applyRouteHeadAndTitle(committedSnapshot);
			applyCommittedCSSBundlesFromRouteDataSnapshot(committedSnapshot);

			if (props.shouldCommitHistory && props.intent === "navigate") {
				runHistoryCommit({
					targetUrl: props.targetUrl,
					replace: props.navigationProps.replace,
					state: props.navigationProps.state,
				});
			}

			const hashFragment = new URL(props.targetUrl, window.location.href)
				.hash;
			const scrollState = props.navigationProps.skipHistoryCommit
				? resolveNavigationScrollStateForTargetHref(props.targetUrl)
				: hashFragment
					? { hash: hashFragment }
					: props.navigationProps.scrollToTop === false
						? undefined
						: { x: 0, y: 0 };
			applyScrollState(scrollState);
			dispatchRouteChangeEvent({
				__scrollState: scrollState,
			});
		},
	});

	props.store.lastNavOrRevalidateTimestampMS = Date.now();
}

async function getRouteDataForNavigation(props: {
	store: NavigationRuntimeStore;
	intent: RuntimeNavigationIntent;
	targetUrl: string;
	signal: AbortSignal;
}): Promise<
	| {
			status: "ready";
			routeSnapshot: RuntimeRouteSnapshot;
			buildID: string;
			modulesMap?: ComponentModulesMap;
	  }
	| {
			status: "redirected";
			redirectHref: string;
			buildID: string;
			isHardReload: boolean;
	  }
> {
	const targetDataKey = getTargetDataKey(props.targetUrl);
	if (props.intent !== "prefetch") {
		const prefetchCache =
			props.store.prefetchCacheByDataTarget.get(targetDataKey);
		if (prefetchCache) {
			return {
				status: "ready",
				routeSnapshot: prefetchCache.routeData,
				buildID: prefetchCache.buildID,
				modulesMap: prefetchCache.modulesMap,
			};
		}
	}

	const routeDataFetchResult = await fetchRouteDataSnapshot({
		targetUrl: props.targetUrl,
		signal: props.signal,
	});
	if (routeDataFetchResult.status === "redirect") {
		return {
			status: "redirected",
			redirectHref: routeDataFetchResult.href,
			buildID: routeDataFetchResult.buildID,
			isHardReload: routeDataFetchResult.isHardReload,
		};
	}
	return {
		status: "ready",
		routeSnapshot: routeDataFetchResult.routeSnapshot,
		buildID: routeDataFetchResult.buildID,
	};
}

function isActiveOperation(props: {
	store: NavigationRuntimeStore;
	operation: NavigationOperation;
}): boolean {
	if (props.operation.intent === "navigate") {
		return props.store.navigateOperation?.id === props.operation.id;
	}
	if (props.operation.intent === "revalidate") {
		return props.store.revalidateOperation?.id === props.operation.id;
	}
	const dataKey = getTargetDataKey(props.operation.targetUrl);
	return (
		props.store.prefetchOperationsByDataTarget.get(dataKey)?.id ===
		props.operation.id
	);
}

function resolveNavigationScrollStateForTargetHref(
	targetHref: string,
): ScrollState {
	const targetHash = new URL(targetHref, window.location.href).hash;
	if (normalizeHashForElementLookup(targetHash).length > 0) {
		return {
			hash: targetHash,
		};
	}
	return resolveStoredScrollStateForCurrentHistoryKeyOrTop();
}

function resolveScrollStateForSameDocumentNoopLinkClick(props: {
	targetHref: string;
	scrollToTop?: boolean;
}): ScrollState | undefined {
	const targetHash = new URL(props.targetHref, window.location.href).hash;
	if (normalizeHashForElementLookup(targetHash).length > 0) {
		return {
			hash: targetHash,
		};
	}
	if (props.scrollToTop === false) {
		return undefined;
	}
	return {
		x: 0,
		y: 0,
	};
}

function clearOperation(props: {
	store: NavigationRuntimeStore;
	operation: NavigationOperation;
}): void {
	props.store.skippedGlobalLoadingIndicatorNavigationOperationIDs.delete(
		props.operation.id,
	);
	if (props.operation.intent === "navigate") {
		if (props.store.navigateOperation?.id === props.operation.id) {
			props.store.navigateOperation = null;
		}
		return;
	}
	if (props.operation.intent === "revalidate") {
		if (props.store.revalidateOperation?.id === props.operation.id) {
			props.store.revalidateOperation = null;
		}
		return;
	}
	const dataKey = getTargetDataKey(props.operation.targetUrl);
	const currentOperation =
		props.store.prefetchOperationsByDataTarget.get(dataKey);
	if (currentOperation?.id === props.operation.id) {
		props.store.prefetchOperationsByDataTarget.delete(dataKey);
	}
}

async function executeNavigationOperation(props: {
	store: NavigationRuntimeStore;
	navigationProps: InternalNavigateProps;
}): Promise<{ didNavigate: boolean }> {
	const intent = props.navigationProps.intent ?? "navigate";
	const targetUrl = resolveAbsoluteHref({
		href: props.navigationProps.href,
	});
	assertProgrammaticSameOriginOrThrow({
		absoluteHref: targetUrl,
		apiName: "vormaNavigate(...)",
	});

	const currentHref =
		props.navigationProps.currentHrefForClassification ??
		window.location.href;
	const targetClassification = classifyNavigationTargetAgainstCurrentLocation(
		{
			targetHref: targetUrl,
			currentHref,
		},
	);
	if (
		intent !== "revalidate" &&
		targetClassification === "same-document-noop"
	) {
		if (
			props.navigationProps.replace &&
			!props.navigationProps.skipHistoryCommit
		) {
			runHistoryCommit({
				targetUrl,
				replace: true,
				state: props.navigationProps.state,
			});
		}
		return {
			didNavigate: false,
		};
	}
	if (
		intent === "prefetch" &&
		targetClassification === "same-document-hash-change"
	) {
		return {
			didNavigate: false,
		};
	}
	if (
		intent === "navigate" &&
		targetClassification === "same-document-hash-change"
	) {
		if (!props.navigationProps.skipHistoryCommit) {
			runHistoryCommit({
				targetUrl,
				replace: props.navigationProps.replace,
				state: props.navigationProps.state,
			});
		}
		const scrollState =
			resolveNavigationScrollStateForTargetHref(targetUrl);
		applyScrollState(scrollState);
		dispatchRouteChangeEvent({
			__scrollState: scrollState,
		});
		return {
			didNavigate: true,
		};
	}

	const targetDataKey = getTargetDataKey(targetUrl);
	if (intent === "navigate") {
		const activeNavigateOperation = props.store.navigateOperation;
		if (
			activeNavigateOperation &&
			getTargetDataKey(activeNavigateOperation.targetUrl) ===
				targetDataKey
		) {
			await activeNavigateOperation.settledPromise;
			return executeNavigationOperation({
				store: props.store,
				navigationProps: props.navigationProps,
			});
		}
	}
	if (intent === "revalidate") {
		const activeRevalidateOperation = props.store.revalidateOperation;
		if (
			activeRevalidateOperation &&
			getTargetDataKey(activeRevalidateOperation.targetUrl) ===
				targetDataKey
		) {
			if (!activeRevalidateOperation.allowTrailingRevalidatePass) {
				await activeRevalidateOperation.settledPromise;
				return {
					didNavigate: false,
				};
			}
			if (
				props.store.queuedRevalidateTargetDataKey !== targetDataKey ||
				!props.store.queuedRevalidateSettledPromise
			) {
				props.store.queuedRevalidateTargetDataKey = targetDataKey;
				props.store.queuedRevalidateSettledPromise =
					activeRevalidateOperation.settledPromise.then(async () => {
						if (
							props.store.queuedRevalidateTargetDataKey !==
							targetDataKey
						) {
							return;
						}
						props.store.queuedRevalidateTargetDataKey = null;
						props.store.queuedRevalidateSettledPromise = null;
						await executeNavigationOperation({
							store: props.store,
							navigationProps: props.navigationProps,
						});
					});
			}
			await props.store.queuedRevalidateSettledPromise;
			return {
				didNavigate: false,
			};
		}
		props.store.queuedRevalidateTargetDataKey = null;
		props.store.queuedRevalidateSettledPromise = null;
	}
	if (intent === "prefetch") {
		if (props.store.prefetchOperationsByDataTarget.has(targetDataKey)) {
			return {
				didNavigate: false,
			};
		}
		if (props.store.prefetchCacheByDataTarget.has(targetDataKey)) {
			return {
				didNavigate: false,
			};
		}
	}

	let notifySettled = () => {};
	const settledPromise = new Promise<void>((resolve) => {
		notifySettled = resolve;
	});
	const operation: NavigationOperation = {
		id: props.store.nextNavigationOperationID,
		intent,
		targetUrl,
		allowTrailingRevalidatePass: false,
		abortController: new AbortController(),
		settledPromise,
		notifySettled,
	};
	props.store.nextNavigationOperationID += 1;
	if (props.navigationProps.skipGlobalLoadingIndicator) {
		props.store.skippedGlobalLoadingIndicatorNavigationOperationIDs.add(
			operation.id,
		);
	}

	if (intent === "navigate") {
		props.store.navigateOperation?.abortController.abort(
			encodeVormaAbortReason(
				VORMA_ABORT_REASON_CODE.SupersededByNewNavigation,
			),
		);
		props.store.revalidateOperation?.abortController.abort(
			encodeVormaAbortReason(
				VORMA_ABORT_REASON_CODE.SupersededByNewNavigation,
			),
		);
		props.store.revalidateOperation = null;
		props.store.queuedRevalidateTargetDataKey = null;
		props.store.queuedRevalidateSettledPromise = null;
		for (const [
			prefetchDataTargetKey,
			prefetchOperation,
		] of props.store.prefetchOperationsByDataTarget.entries()) {
			if (prefetchDataTargetKey === targetDataKey) {
				continue;
			}
			prefetchOperation.abortController.abort(
				encodeVormaAbortReason(
					VORMA_ABORT_REASON_CODE.SupersededByNewNavigation,
				),
			);
			props.store.prefetchOperationsByDataTarget.delete(
				prefetchDataTargetKey,
			);
		}
		props.store.navigateOperation = operation;
	} else if (intent === "revalidate") {
		props.store.revalidateOperation?.abortController.abort(
			encodeVormaAbortReason(
				VORMA_ABORT_REASON_CODE.SupersededByNewRevalidation,
			),
		);
		props.store.queuedRevalidateTargetDataKey = null;
		props.store.queuedRevalidateSettledPromise = null;
		props.store.revalidateOperation = operation;
		queueMicrotask(() => {
			if (props.store.revalidateOperation?.id === operation.id) {
				operation.allowTrailingRevalidatePass = true;
			}
		});
	} else {
		props.store.prefetchOperationsByDataTarget.set(
			targetDataKey,
			operation,
		);
	}
	emitStatusIfChanged(props.store);
	const clientLoaderPrestart = createClientLoaderPrestart({
		targetUrl,
		signal: operation.abortController.signal,
	});

	try {
		if (intent !== "prefetch") {
			const sharedPrefetchOperation =
				props.store.prefetchOperationsByDataTarget.get(targetDataKey);
			if (sharedPrefetchOperation) {
				await sharedPrefetchOperation.settledPromise;
				if (!isActiveOperation({ store: props.store, operation })) {
					return {
						didNavigate: false,
					};
				}
			}
		}

		const routeDataResult = await getRouteDataForNavigation({
			store: props.store,
			intent,
			targetUrl,
			signal: operation.abortController.signal,
		});
		if (!isActiveOperation({ store: props.store, operation })) {
			clientLoaderPrestart?.abortIfPending();
			return {
				didNavigate: false,
			};
		}
		if (
			!hasRevalidationTargetOwnership({
				intent,
				targetUrl,
			})
		) {
			clientLoaderPrestart?.abortIfPending();
			return {
				didNavigate: false,
			};
		}

		if (routeDataResult.status === "redirected") {
			if (intent === "prefetch") {
				clientLoaderPrestart?.abortIfPending();
				return {
					didNavigate: false,
				};
			}
			clientLoaderPrestart?.abortIfPending();
			syncBuildIDIfChanged(routeDataResult.buildID);
			const redirectResult = await followRedirectWithHardFallback({
				redirectHref: routeDataResult.redirectHref,
				buildID: routeDataResult.buildID,
				isHardReload: routeDataResult.isHardReload,
				store: props.store,
				redirectHopCount: props.navigationProps.redirectHopCount,
				skipGlobalLoadingIndicator:
					props.navigationProps.skipGlobalLoadingIndicator,
			});
			return {
				didNavigate: redirectResult.didNavigate,
			};
		}

		if (intent === "prefetch") {
			clientLoaderPrestart?.resolveFromSnapshot(
				routeDataResult.routeSnapshot,
			);
			syncBuildIDIfChanged(routeDataResult.buildID);
			props.store.prefetchCacheByDataTarget.set(targetDataKey, {
				targetDataKey,
				targetUrl,
				routeData: routeDataResult.routeSnapshot,
				buildID: routeDataResult.buildID,
				modulesMap: routeDataResult.modulesMap,
			});
			return {
				didNavigate: false,
			};
		}

		clientLoaderPrestart?.resolveFromSnapshot(
			routeDataResult.routeSnapshot,
		);
		await commitSuccessfulNavigation({
			store: props.store,
			navigationProps: props.navigationProps,
			targetUrl,
			routeSnapshot: routeDataResult.routeSnapshot,
			buildID: routeDataResult.buildID,
			intent,
			shouldCommitHistory: !props.navigationProps.skipHistoryCommit,
			signal: operation.abortController.signal,
			modulesMapOverride: routeDataResult.modulesMap,
			prestartedClientLoaderResultPromiseByPattern: clientLoaderPrestart
				? {
						[clientLoaderPrestart.matchedPattern]:
							clientLoaderPrestart.clientLoaderResultPromise,
					}
				: undefined,
		});
		return {
			didNavigate: intent === "navigate",
		};
	} catch (error) {
		clientLoaderPrestart?.abortIfPending();
		if (isAbortError(error)) {
			return {
				didNavigate: false,
			};
		}
		throw error;
	} finally {
		clientLoaderPrestart?.abortIfPending();
		clearOperation({
			store: props.store,
			operation,
		});
		emitStatusIfChanged(props.store);
		operation.notifySettled();
	}
}

function resolveNormalizedSubmitMethod(
	requestMethod: string | undefined,
): string {
	return (requestMethod ?? "GET").toUpperCase();
}

function shouldOmitRequestBodyForSubmitMethod(method: string): boolean {
	return method === "GET" || method === "HEAD";
}

function shouldSerializeSubmitBodyAsJSON(body: unknown): boolean {
	if (!body || typeof body !== "object") {
		return false;
	}
	if (
		body instanceof ReadableStream ||
		body instanceof FormData ||
		body instanceof URLSearchParams ||
		body instanceof Blob ||
		body instanceof ArrayBuffer ||
		ArrayBuffer.isView(body)
	) {
		return false;
	}
	return true;
}

function buildSubmitRequestInit(props: {
	requestInit?: RequestInit;
	headers: Headers;
}): RequestInit {
	const normalizedMethod = resolveNormalizedSubmitMethod(
		props.requestInit?.method,
	);
	const submitRequestInit: RequestInit = {
		...props.requestInit,
		method: normalizedMethod,
		headers: props.headers,
	};
	if (shouldOmitRequestBodyForSubmitMethod(normalizedMethod)) {
		delete submitRequestInit.body;
		return submitRequestInit;
	}
	const submitBody = submitRequestInit.body;
	if (shouldSerializeSubmitBodyAsJSON(submitBody)) {
		submitRequestInit.body = JSON.stringify(submitBody);
		if (!props.headers.has("content-type")) {
			props.headers.set("content-type", "application/json");
		}
	}
	return submitRequestInit;
}

function shouldAutoRevalidateForSubmit(props: {
	options?: SubmitOptions;
	method: string;
}): boolean {
	if (props.options?.revalidate !== undefined) {
		return props.options.revalidate;
	}
	return !shouldOmitRequestBodyForSubmitMethod(props.method);
}

function isLatestStartedSubmissionOperation(props: {
	store: NavigationRuntimeStore;
	submissionOperationID: number;
}): boolean {
	return (
		props.store.latestStartedSubmissionOperationID ===
		props.submissionOperationID
	);
}

function resolveSupersededDedupedSubmitAbortResult(props: {
	store: NavigationRuntimeStore;
	submissionOperationID: number;
	dedupeKey?: string;
}): SubmitResult<never> | null {
	if (!props.dedupeKey) {
		return null;
	}
	if (
		props.store.submissionOperationIDByDedupeKey.get(props.dedupeKey) !==
		props.submissionOperationID
	) {
		return {
			success: false,
			error: "Aborted",
		};
	}
	return null;
}

function resolveSubmitErrorMessageFromHTTPFailure(props: {
	responseStatus: number;
	responseStatusText: string;
	responseBodyText: string;
}): string {
	const trimmedResponseBodyText = props.responseBodyText.trim();
	if (trimmedResponseBodyText === "") {
		return String(props.responseStatus);
	}
	if (
		props.responseStatusText !== "" &&
		trimmedResponseBodyText === props.responseStatusText
	) {
		return String(props.responseStatus);
	}
	return trimmedResponseBodyText;
}

async function executeSubmitOperation<T>(props: {
	store: NavigationRuntimeStore;
	url: string | URL;
	requestInit?: RequestInit;
	options?: SubmitOptions;
}): Promise<SubmitResult<T>> {
	const globalState = getGlobalStateOrThrow();
	const submissionOperationID = props.store.nextSubmissionOperationID;
	props.store.nextSubmissionOperationID += 1;
	props.store.latestStartedSubmissionOperationID = submissionOperationID;
	if (props.options?.skipGlobalLoadingIndicator) {
		props.store.skippedGlobalLoadingIndicatorSubmissionOperationIDs.add(
			submissionOperationID,
		);
	}
	const submitAbortController = new AbortController();
	const dedupeKey = props.options?.dedupeKey;
	if (dedupeKey) {
		const previousOperationID =
			props.store.submissionOperationIDByDedupeKey.get(dedupeKey);
		if (previousOperationID !== undefined) {
			const previousAbortController =
				props.store.submissionAbortControllerByOperationID.get(
					previousOperationID,
				);
			previousAbortController?.abort(
				encodeVormaAbortReason(
					VORMA_ABORT_REASON_CODE.SupersededBySubmitDedupe,
				),
			);
		}
		props.store.submissionOperationIDByDedupeKey.set(
			dedupeKey,
			submissionOperationID,
		);
	}
	props.store.activeSubmissionOperationIDs.add(submissionOperationID);
	props.store.submissionAbortControllerByOperationID.set(
		submissionOperationID,
		submitAbortController,
	);
	emitStatusIfChanged(props.store);

	try {
		const submitURL = new URL(resolveAbsoluteHref({ href: props.url }));
		assertProgrammaticSameOriginOrThrow({
			absoluteHref: submitURL.href,
			apiName: "submit(...)",
		});
		const submitHeaders = new Headers(
			props.requestInit?.headers ?? undefined,
		);
		setAcceptsClientRedirectHeader(submitHeaders);
		if (globalState.deploymentID) {
			submitHeaders.set("x-deployment-id", globalState.deploymentID);
		}
		const submitRequestInit = buildSubmitRequestInit({
			requestInit: props.requestInit,
			headers: submitHeaders,
		});
		const response = await window.fetch(submitURL, {
			...submitRequestInit,
			signal: submitAbortController.signal,
		});
		const supersededDedupedSubmitResult =
			resolveSupersededDedupedSubmitAbortResult({
				store: props.store,
				submissionOperationID,
				dedupeKey,
			});
		if (supersededDedupedSubmitResult) {
			return supersededDedupedSubmitResult;
		}
		const isCurrentSubmissionSideEffectOwner =
			isLatestStartedSubmissionOperation({
				store: props.store,
				submissionOperationID,
			});
		const buildIDFromResponseHeader = response.headers.get(
			"X-Wave-Framework-Build-Id",
		) as string;
		const hardReloadHeader = response.headers.get(
			"X-Wave-Framework-Reload",
		);
		const redirectHref = response.headers.get("X-Client-Redirect");
		if (hardReloadHeader) {
			if (!isCurrentSubmissionSideEffectOwner) {
				const supersededResult =
					resolveSupersededDedupedSubmitAbortResult({
						store: props.store,
						submissionOperationID,
						dedupeKey,
					});
				if (supersededResult) {
					return supersededResult;
				}
				return {
					success: true,
					data: undefined as T,
				};
			}
			syncBuildIDIfChanged(buildIDFromResponseHeader);
			try {
				await followRedirectWithHardFallback({
					redirectHref: resolveRedirectTargetHref({
						redirectTarget: hardReloadHeader,
						requestURL: submitURL,
					}),
					buildID: buildIDFromResponseHeader,
					isHardReload: true,
					store: props.store,
					redirectHopCount: 0,
				});
			} catch {
				return {
					success: false,
					error: "Redirect failed",
				};
			}
			return {
				success: true,
				data: undefined as T,
			};
		}
		if (redirectHref) {
			if (!isCurrentSubmissionSideEffectOwner) {
				const supersededResult =
					resolveSupersededDedupedSubmitAbortResult({
						store: props.store,
						submissionOperationID,
						dedupeKey,
					});
				if (supersededResult) {
					return supersededResult;
				}
				return {
					success: true,
					data: undefined as T,
				};
			}
			syncBuildIDIfChanged(buildIDFromResponseHeader);
			try {
				await followRedirectWithHardFallback({
					redirectHref: resolveRedirectTargetHref({
						redirectTarget: redirectHref,
						requestURL: submitURL,
					}),
					buildID: buildIDFromResponseHeader,
					isHardReload: false,
					store: props.store,
					redirectHopCount: 0,
				});
			} catch {
				return {
					success: false,
					error: "Redirect failed",
				};
			}
			return {
				success: true,
				data: undefined as T,
			};
		}
		const nativeRedirectTarget = resolveNativeFetchRedirectTarget({
			requestURL: submitURL,
			response,
		});
		if (nativeRedirectTarget) {
			if (!isCurrentSubmissionSideEffectOwner) {
				const supersededResult =
					resolveSupersededDedupedSubmitAbortResult({
						store: props.store,
						submissionOperationID,
						dedupeKey,
					});
				if (supersededResult) {
					return supersededResult;
				}
				return {
					success: true,
					data: undefined as T,
				};
			}
			syncBuildIDIfChanged(buildIDFromResponseHeader);
			try {
				await followRedirectWithHardFallback({
					redirectHref: nativeRedirectTarget,
					buildID: buildIDFromResponseHeader,
					isHardReload: false,
					store: props.store,
					redirectHopCount: 0,
				});
			} catch {
				return {
					success: false,
					error: "Redirect failed",
				};
			}
			return {
				success: true,
				data: undefined as T,
			};
		}
		if (!response.ok) {
			const responseBodyText = await response.text();
			const supersededResult = resolveSupersededDedupedSubmitAbortResult({
				store: props.store,
				submissionOperationID,
				dedupeKey,
			});
			if (supersededResult) {
				return supersededResult;
			}
			return {
				success: false,
				error: resolveSubmitErrorMessageFromHTTPFailure({
					responseStatus: response.status,
					responseStatusText: response.statusText,
					responseBodyText,
				}),
			};
		}
		let parsedData: unknown = undefined;
		const contentType = response.headers.get("content-type");
		const hasNoContent = response.status === 204;
		if (!hasNoContent) {
			if (contentType?.toLowerCase().includes("json")) {
				parsedData = await response.json();
			} else {
				const responseText = await response.text();
				parsedData = responseText.length > 0 ? responseText : undefined;
			}
		}
		const supersededResult = resolveSupersededDedupedSubmitAbortResult({
			store: props.store,
			submissionOperationID,
			dedupeKey,
		});
		if (supersededResult) {
			return supersededResult;
		}
		if (
			isCurrentSubmissionSideEffectOwner &&
			shouldAutoRevalidateForSubmit({
				options: props.options,
				method: submitRequestInit.method as string,
			})
		) {
			await revalidate();
		}
		return {
			success: true,
			data: parsedData as T,
		};
	} catch (error) {
		if (
			String((error as AnyRecord)?.message ?? "").includes(
				"only supports same-origin targets",
			)
		) {
			throw error;
		}
		if (isAbortError(error)) {
			return {
				success: false,
				error: "Aborted",
			};
		}
		return {
			success: false,
			error: String((error as AnyRecord)?.message ?? error),
		};
	} finally {
		props.store.activeSubmissionOperationIDs.delete(submissionOperationID);
		props.store.submissionAbortControllerByOperationID.delete(
			submissionOperationID,
		);
		props.store.skippedGlobalLoadingIndicatorSubmissionOperationIDs.delete(
			submissionOperationID,
		);
		if (
			dedupeKey &&
			props.store.submissionOperationIDByDedupeKey.get(dedupeKey) ===
				submissionOperationID
		) {
			props.store.submissionOperationIDByDedupeKey.delete(dedupeKey);
		}
		emitStatusIfChanged(props.store);
	}
}

function createNavigationStateManager(): NavigationStateManager {
	const store = createNavigationRuntimeStore();
	return {
		navigate: (navigationProps) => {
			return executeNavigationOperation({
				store,
				navigationProps,
			});
		},
		submit: (url, requestInit, options) => {
			return executeSubmitOperation({
				store,
				url,
				requestInit,
				options,
			});
		},
		getStatus: () => ({
			isNavigating: store.navigateOperation !== null,
			isSubmitting:
				resolveShouldRunSubmittingGlobalLoadingIndicatorFromStore(
					store,
				),
			isRevalidating: store.revalidateOperation !== null,
		}),
		clearAll: () => {
			store.navigateOperation?.abortController.abort(
				encodeVormaAbortReason(VORMA_ABORT_REASON_CODE.ClearAll),
			);
			store.revalidateOperation?.abortController.abort(
				encodeVormaAbortReason(VORMA_ABORT_REASON_CODE.ClearAll),
			);
			for (const operation of store.prefetchOperationsByDataTarget.values()) {
				operation.abortController.abort(
					encodeVormaAbortReason(VORMA_ABORT_REASON_CODE.ClearAll),
				);
			}
			for (const abortController of store.submissionAbortControllerByOperationID.values()) {
				abortController.abort(
					encodeVormaAbortReason(VORMA_ABORT_REASON_CODE.ClearAll),
				);
			}
			store.prefetchOperationsByDataTarget.clear();
			store.prefetchCacheByDataTarget.clear();
			store.navigateOperation = null;
			store.revalidateOperation = null;
			store.queuedRevalidateTargetDataKey = null;
			store.queuedRevalidateSettledPromise = null;
			store.submissionAbortControllerByOperationID.clear();
			store.submissionOperationIDByDedupeKey.clear();
			store.skippedGlobalLoadingIndicatorNavigationOperationIDs.clear();
			store.skippedGlobalLoadingIndicatorSubmissionOperationIDs.clear();
			store.activeSubmissionOperationIDs.clear();
			store.latestStartedSubmissionOperationID = 0;
			emitStatusIfChanged(store);
		},
		getUnsafeNavigationStore: () => store,
	};
}

/////////////////////////////////////////////////////////////////////
/////// Runtime Context APIs
/////////////////////////////////////////////////////////////////////

export function createVormaRuntimeContext(): VormaRuntimeContext {
	const globalState = ensureGlobalState();
	const navigationStateManager =
		globalState.navigationStateManager ?? createNavigationStateManager();
	globalState.navigationStateManager = navigationStateManager;
	return {
		navigationStateManager,
	};
}

export function getDefaultVormaRuntimeContext(): VormaRuntimeContext {
	return createVormaRuntimeContext();
}

export function setDefaultVormaRuntimeContext(
	nextRuntimeContext: VormaRuntimeContext,
): void {
	const globalState = ensureGlobalState();
	globalState.navigationStateManager =
		nextRuntimeContext.navigationStateManager;
}

export function setRuntimeGlobalStateForDefaultContext(
	nextGlobalState: VormaClientGlobal,
): void {
	(globalThis as AnyPropertyRecord)[VORMA_SYMBOL] = nextGlobalState;
	createVormaRuntimeContext();
}

/////////////////////////////////////////////////////////////////////
/////// Public Client API
/////////////////////////////////////////////////////////////////////

export function defaultErrorBoundary(props: { error: unknown }): string {
	return `Route Error: ${String(props.error)}`;
}

export function formatOutermostErrorForRendering(
	outermostError: unknown,
): string {
	let e = "unknown";
	if (outermostError instanceof Error) {
		e = outermostError.message || "unknown";
	} else if (typeof outermostError === "string") {
		e = outermostError;
	} else if (outermostError != null) {
		const serializedOutermostError = JSON.stringify(outermostError);
		if (serializedOutermostError !== undefined) {
			e = serializedOutermostError;
		}
	}
	return "Error: " + e;
}

export function getStatus(): StatusEventDetail {
	return getDefaultVormaRuntimeContext().navigationStateManager.getStatus();
}

export function getLocation(): RouteOutletLocationState {
	const history = getOrCreateHistoryInstance();
	return {
		pathname: history.location.pathname,
		search: history.location.search,
		hash: history.location.hash,
		state: history.location.state,
	};
}

export function getBuildID(): string {
	return getRuntimeRouteSnapshot().buildID;
}

export function getRootEl(): HTMLElement {
	const rootElementID = getCurrentRootElementID();
	const rootElement = document.getElementById(rootElementID);
	if (!rootElement) {
		panic(`Expected element with id "${rootElementID}" to exist`);
	}
	return rootElement;
}

export function getHistoryInstance(): BrowserHistory {
	return getOrCreateHistoryInstance();
}

export async function vormaNavigate(
	href: string | URL,
	options: Omit<NavigateProps, "href"> = {},
): Promise<{ didNavigate: boolean }> {
	return getDefaultVormaRuntimeContext().navigationStateManager.navigate({
		href,
		replace: options.replace,
		scrollToTop: options.scrollToTop,
		state: options.state,
		intent: options.intent ?? "navigate",
		skipGlobalLoadingIndicator: options.skipGlobalLoadingIndicator,
	});
}

export async function revalidate(): Promise<{ didNavigate: boolean }> {
	const currentHref = window.location.href;
	return getDefaultVormaRuntimeContext().navigationStateManager.navigate({
		href: currentHref,
		intent: "revalidate",
		replace: true,
		scrollToTop: false,
	});
}

function getLastTriggeredNavOrRevalidateTimestampMS(): number {
	return getNavigationStore().lastNavOrRevalidateTimestampMS;
}

export async function submit<T = unknown>(
	url: string | URL,
	requestInit?: RequestInit,
	options?: SubmitOptions,
): Promise<SubmitResult<T>> {
	return getDefaultVormaRuntimeContext().navigationStateManager.submit<T>(
		url,
		requestInit,
		options,
	);
}

/////////////////////////////////////////////////////////////////////
/////// API Client Helpers
/////////////////////////////////////////////////////////////////////

export function makeTypedAPIClient<C extends VormaAppConfig>(
	vormaAppConfig: C,
	apiRequestInitDecorator?: APIRequestInitDecorator<ExtractApp<C>>,
): TypedAPIClient<ExtractApp<C>> {
	type App = ExtractApp<C>;

	const mergeRequestInit = (props: {
		baseRequestInit: RequestInit;
		overrideRequestInit?: RequestInit;
	}): RequestInit => {
		const mergedHeaders = new Headers(
			props.baseRequestInit.headers ?? undefined,
		);
		const overrideHeaders = new Headers(
			props.overrideRequestInit?.headers ?? undefined,
		);
		overrideHeaders.forEach((value, key) => {
			mergedHeaders.set(key, value);
		});
		return {
			...props.baseRequestInit,
			...props.overrideRequestInit,
			headers: mergedHeaders,
		};
	};

	const resolveRequestInit = async (props: {
		requestContext: APIRequestInitDecoratorContext<App>;
		fallbackRequestInit: RequestInit;
	}): Promise<RequestInit> => {
		const decoratedRequestInit = apiRequestInitDecorator
			? await apiRequestInitDecorator(props.requestContext)
			: undefined;
		const requestInitWithDecorator = mergeRequestInit({
			baseRequestInit: props.fallbackRequestInit,
			overrideRequestInit: decoratedRequestInit,
		});
		return mergeRequestInit({
			baseRequestInit: requestInitWithDecorator,
			overrideRequestInit: props.requestContext.requestInit,
		});
	};

	return {
		query: async <P extends VormaQueryPattern<App>>(
			queryProps: VormaQueryProps<App, P>,
		): Promise<SubmitResult<VormaQueryOutput<App, P>>> => {
			const queryPropsAny = queryProps as AnyRecord;
			const requestInit = await resolveRequestInit({
				requestContext: {
					type: "query",
					pattern: queryProps.pattern as VormaQueryPattern<App>,
					requestInit: queryProps.requestInit,
					input: queryProps.input,
				},
				fallbackRequestInit: {
					method: "GET",
				},
			});
			return submit<VormaQueryOutput<App, P>>(
				buildQueryURL(vormaAppConfig, {
					pattern: queryProps.pattern,
					params: queryPropsAny.params,
					splatValues: queryPropsAny.splatValues,
					input: queryProps.input,
				}),
				requestInit,
				queryProps.options,
			);
		},
		mutate: async <P extends VormaMutationPattern<App>>(
			mutationProps: VormaMutationProps<App, P>,
		): Promise<SubmitResult<VormaMutationOutput<App, P>>> => {
			const mutationPropsAny = mutationProps as AnyRecord;
			const requestInit = await resolveRequestInit({
				requestContext: {
					type: "mutation",
					pattern: mutationProps.pattern as VormaMutationPattern<App>,
					requestInit: mutationProps.requestInit,
					input: mutationProps.input,
				},
				fallbackRequestInit: {
					method: "POST",
					body: resolveBody({
						input: mutationProps.input,
					}),
				},
			});
			return submit<VormaMutationOutput<App, P>>(
				buildMutationURL(vormaAppConfig, {
					pattern: mutationProps.pattern,
					params: mutationPropsAny.params,
					splatValues: mutationPropsAny.splatValues,
				}),
				requestInit,
				mutationProps.options,
			);
		},
	};
}

export function makeTypedNavigate<C extends VormaAppConfig>(vormaAppConfig: C) {
	type App = ExtractApp<C>;
	type TypedNavigateOptions<Pattern extends VormaLoaderPattern<App>> =
		PermissivePatternBasedProps<App, Pattern> & {
			replace?: boolean;
			scrollToTop?: boolean;
			search?: string;
			hash?: string;
			state?: unknown;
		};
	return async <Pattern extends VormaLoaderPattern<App>>(
		props: TypedNavigateOptions<Pattern>,
	): Promise<{ didNavigate: boolean }> => {
		const propsAny = props as AnyRecord;
		const href = resolveAbsoluteHrefWithOptionalSearchAndHash({
			href: resolveVormaPath({
				vormaAppConfig,
				type: "loader",
				props: {
					pattern: props.pattern,
					params: propsAny.params as
						| Record<string, string>
						| undefined,
					splatValues: propsAny.splatValues as string[] | undefined,
				},
			}),
			search: props.search,
			hash: props.hash,
		});
		return vormaNavigate(href, {
			replace: props.replace,
			scrollToTop: props.scrollToTop,
			state: props.state,
		});
	};
}

/////////////////////////////////////////////////////////////////////
/////// Loading Indicator And Focus Revalidation
/////////////////////////////////////////////////////////////////////

export type GlobalLoadingIndicatorConfig = {
	start: () => void;
	stop: () => void;
	isRunning: () => boolean;
	include?: "all" | Array<"navigations" | "submissions" | "revalidations">;
	startDelayMS?: number;
	stopDelayMS?: number;
};

const DEFAULT_GLOBAL_LOADING_INDICATOR_DELAY_MS = 12;

type ParsedGlobalLoadingIndicatorConfig = {
	start: () => void;
	stop: () => void;
	isRunning: () => boolean;
	includesNavigations: boolean;
	includesSubmissions: boolean;
	includesRevalidations: boolean;
	startDelayMS: number;
	stopDelayMS: number;
};

function resolveIncludesOption(
	config: GlobalLoadingIndicatorConfig,
	includeOption: "navigations" | "submissions" | "revalidations",
): boolean {
	return (
		Array.isArray(config.include) && config.include.includes(includeOption)
	);
}

function parseGlobalLoadingIndicatorConfig(
	config: GlobalLoadingIndicatorConfig,
): ParsedGlobalLoadingIndicatorConfig {
	const includesAll =
		config.include === undefined || config.include === "all";
	return {
		start: config.start,
		stop: config.stop,
		isRunning: config.isRunning,
		includesNavigations:
			includesAll || resolveIncludesOption(config, "navigations"),
		includesSubmissions:
			includesAll || resolveIncludesOption(config, "submissions"),
		includesRevalidations:
			includesAll || resolveIncludesOption(config, "revalidations"),
		startDelayMS:
			config.startDelayMS ?? DEFAULT_GLOBAL_LOADING_INDICATOR_DELAY_MS,
		stopDelayMS:
			config.stopDelayMS ?? DEFAULT_GLOBAL_LOADING_INDICATOR_DELAY_MS,
	};
}

export function setupGlobalLoadingIndicator(
	config: GlobalLoadingIndicatorConfig,
): () => void {
	const runtimeContext = getDefaultVormaRuntimeContext();
	const parsedConfig = parseGlobalLoadingIndicatorConfig(config);
	let pendingStartTimerID: number | null = null;
	let pendingStopTimerID: number | null = null;
	const clearPendingStartTimer = () => {
		if (pendingStartTimerID !== null) {
			window.clearTimeout(pendingStartTimerID);
			pendingStartTimerID = null;
		}
	};
	const clearPendingStopTimer = () => {
		if (pendingStopTimerID !== null) {
			window.clearTimeout(pendingStopTimerID);
			pendingStopTimerID = null;
		}
	};
	const resolveShouldRunIndicator = () => {
		const status = resolveGlobalLoadingIndicatorStatusFromStore(
			runtimeContext.navigationStateManager.getUnsafeNavigationStore(),
		);
		return (
			(parsedConfig.includesNavigations && status.isNavigating) ||
			(parsedConfig.includesSubmissions && status.isSubmitting) ||
			(parsedConfig.includesRevalidations && status.isRevalidating)
		);
	};
	const syncLoadingIndicator = () => {
		const shouldRunIndicator = resolveShouldRunIndicator();
		if (shouldRunIndicator) {
			clearPendingStopTimer();
			if (parsedConfig.isRunning() || pendingStartTimerID !== null) {
				return;
			}
			pendingStartTimerID = window.setTimeout(() => {
				pendingStartTimerID = null;
				if (!resolveShouldRunIndicator() || parsedConfig.isRunning()) {
					return;
				}
				parsedConfig.start();
			}, parsedConfig.startDelayMS);
			return;
		}
		clearPendingStartTimer();
		if (!parsedConfig.isRunning() || pendingStopTimerID !== null) {
			return;
		}
		pendingStopTimerID = window.setTimeout(() => {
			pendingStopTimerID = null;
			if (resolveShouldRunIndicator() || !parsedConfig.isRunning()) {
				return;
			}
			parsedConfig.stop();
		}, parsedConfig.stopDelayMS);
	};
	const removeStatusListener = addStatusListener(() => {
		syncLoadingIndicator();
	});
	syncLoadingIndicator();
	return () => {
		removeStatusListener();
		clearPendingStartTimer();
		clearPendingStopTimer();
		if (parsedConfig.isRunning()) {
			parsedConfig.stop();
		}
	};
}

export function revalidateOnWindowFocus(options?: {
	staleTimeMS?: number;
}): () => void {
	const staleTimeMS = options?.staleTimeMS ?? 0;
	return addOnWindowFocusListener(() => {
		const nowTimestampMS = Date.now();
		const lastNavOrRevalidateTimestampMS =
			getLastTriggeredNavOrRevalidateTimestampMS();
		if (nowTimestampMS - lastNavOrRevalidateTimestampMS >= staleTimeMS) {
			void revalidate();
		}
	});
}

/////////////////////////////////////////////////////////////////////
/////// Link Click Lifecycle
/////////////////////////////////////////////////////////////////////

async function navigateWithLinkLifecycleCallbacks<LinkEvent>(props: {
	href: string;
	event: LinkEvent;
	replace?: boolean;
	scrollToTop?: boolean;
	state?: unknown;
	beforeBegin?: (event: LinkEvent) => void | Promise<void>;
	beforeRender?: (event: LinkEvent) => void | Promise<void>;
	afterRender?: (event: LinkEvent) => void | Promise<void>;
}): Promise<{ didNavigate: boolean }> {
	await props.beforeBegin?.(props.event);
	await props.beforeRender?.(props.event);
	const navigationResult = await vormaNavigate(props.href, {
		replace: props.replace,
		scrollToTop: props.scrollToTop,
		state: props.state,
	});
	if (navigationResult.didNavigate) {
		await props.afterRender?.(props.event);
	}
	return navigationResult;
}

function hasStartedPrefetchForDataTargetHref(targetHref: string): boolean {
	const targetDataKey = getTargetDataKey(targetHref);
	const navigationStore = getNavigationStore();
	return (
		navigationStore.prefetchOperationsByDataTarget.has(targetDataKey) ||
		navigationStore.prefetchCacheByDataTarget.has(targetDataKey)
	);
}

function createPrefetchHandlersForHref(props: {
	href: string;
	prefetchDelayMs?: number;
	beforeBegin?: (event: unknown) => void | Promise<void>;
}): {
	start: (event: unknown) => void;
	stop: (options?: {
		clearCache?: boolean;
		abortInFlightOperation?: boolean;
	}) => void;
} | null {
	const hrefDetails = getHrefDetails(props.href);
	if (!hrefDetails.isHTTP || !hrefDetails.isInternal) {
		return null;
	}
	const targetHref = hrefDetails.absoluteURL;
	let prefetchTimer: number | undefined;

	const stop = (options?: {
		clearCache?: boolean;
		abortInFlightOperation?: boolean;
	}) => {
		if (prefetchTimer !== undefined) {
			window.clearTimeout(prefetchTimer);
			prefetchTimer = undefined;
		}
		const navigationStore = getNavigationStore();
		const shouldClearCache = options?.clearCache ?? true;
		const shouldAbortInFlightOperation =
			options?.abortInFlightOperation ?? true;
		const targetDataKey = getTargetDataKey(targetHref);
		const activeNavigateOperation = navigationStore.navigateOperation;
		if (
			activeNavigateOperation &&
			getTargetDataKey(activeNavigateOperation.targetUrl) ===
				targetDataKey
		) {
			if (shouldClearCache) {
				navigationStore.prefetchCacheByDataTarget.delete(targetDataKey);
			}
			return;
		}
		const prefetchOperation =
			navigationStore.prefetchOperationsByDataTarget.get(targetDataKey);
		if (shouldAbortInFlightOperation) {
			prefetchOperation?.abortController.abort(
				encodeVormaAbortReason(VORMA_ABORT_REASON_CODE.PrefetchStopped),
			);
			navigationStore.prefetchOperationsByDataTarget.delete(
				targetDataKey,
			);
		}
		if (shouldClearCache) {
			navigationStore.prefetchCacheByDataTarget.delete(targetDataKey);
		}
		emitStatusIfChanged(navigationStore);
	};

	return {
		start: (event) => {
			if (
				classifyNavigationTargetAgainstCurrentLocation({
					targetHref: targetHref,
					currentHref: window.location.href,
				}) !== "requires-fetch"
			) {
				return;
			}
			if (prefetchTimer !== undefined) {
				window.clearTimeout(prefetchTimer);
			}
			prefetchTimer = window.setTimeout(async () => {
				prefetchTimer = undefined;
				try {
					await props.beforeBegin?.(event);
					await vormaNavigate(targetHref, {
						intent: "prefetch",
						skipGlobalLoadingIndicator: true,
					});
				} catch (error) {
					console.error("Vorma:", "Prefetch start failed", error);
				}
			}, props.prefetchDelayMs ?? 100);
		},
		stop,
	};
}

function createPrefetchHandlers<E = any>(props: {
	href: string;
	prefetch?: "intent" | "none";
	prefetchDelayMs?: number;
	beforeBegin?: (event: E) => void | Promise<void>;
	eventProxyFactory?: (callback: (event: E) => void) => (event: E) => void;
}): {
	onPointerEnter?: (event: E) => void;
	onFocus?: (event: E) => void;
	onPointerLeave?: (event: E) => void;
	onBlur?: (event: E) => void;
	onTouchCancel?: (event: E) => void;
	cancelPendingForClick?: () => void;
} {
	if ((props.prefetch ?? "intent") !== "intent") {
		return {};
	}
	const prefetchHandlers = createPrefetchHandlersForHref({
		href: props.href,
		prefetchDelayMs: props.prefetchDelayMs,
		beforeBegin: props.beforeBegin as
			| ((event: unknown) => void | Promise<void>)
			| undefined,
	});
	if (!prefetchHandlers) {
		return {};
	}
	const wrapEvent =
		props.eventProxyFactory ??
		((callback) => {
			return (event) => callback(event);
		});
	const shouldStopPrefetchFromPointerLeave = (): boolean => {
		return !ensureGlobalState().isTouchInputModalityActive;
	};
	return {
		onPointerEnter: wrapEvent((event) => prefetchHandlers.start(event)),
		onFocus: wrapEvent((event) => prefetchHandlers.start(event)),
		onPointerLeave: wrapEvent(() => {
			if (!shouldStopPrefetchFromPointerLeave()) {
				return;
			}
			prefetchHandlers.stop();
		}),
		onBlur: wrapEvent(() => prefetchHandlers.stop()),
		onTouchCancel: wrapEvent(() => prefetchHandlers.stop()),
		cancelPendingForClick: () =>
			prefetchHandlers.stop({
				clearCache: false,
				abortInFlightOperation: false,
			}),
	};
}

function isSupportedInternalNavigationTarget(target: unknown): boolean {
	if (typeof target !== "string") {
		return true;
	}
	if (target === "" || target === "_self") {
		return true;
	}
	return false;
}

export const navigationInternalLinkPropKeysForAnchors = [
	"prefetch",
	"prefetchDelayMs",
	"replace",
	"scrollToTop",
	"state",
	"beforeBegin",
	"beforeRender",
	"afterRender",
	"pattern",
	"params",
	"splatValues",
	"search",
	"hash",
] as const;

function stripNavigationInternalLinkPropsForAnchor<AnchorProps extends object>(
	props: AnchorProps,
): Omit<
	AnchorProps,
	(typeof navigationInternalLinkPropKeysForAnchors)[number]
> {
	const clonedProps: AnyRecord = {
		...props,
	};
	for (const key of navigationInternalLinkPropKeysForAnchors) {
		delete clonedProps[key];
	}
	return clonedProps as Omit<
		AnchorProps,
		(typeof navigationInternalLinkPropKeysForAnchors)[number]
	>;
}

export function makeFinalLinkProps<LinkEvent>(
	linkProps: { href?: string } & VormaLinkPropsBase<LinkEvent>,
): {
	dataExternal?: boolean;
	safeAnchorProps: Record<string, unknown>;
	onPointerEnter?: (event: LinkEvent) => void;
	onFocus?: (event: LinkEvent) => void;
	onPointerLeave?: (event: LinkEvent) => void;
	onBlur?: (event: LinkEvent) => void;
	onTouchCancel?: (event: LinkEvent) => void;
	onClick?: (event: LinkEvent) => void | Promise<void>;
} {
	const href = linkProps.href ?? "";
	const hrefDetails = getHrefDetails(href);
	const isExternal = hrefDetails.isHTTP ? hrefDetails.isExternal : true;
	const safeAnchorProps =
		stripNavigationInternalLinkPropsForAnchor(linkProps);
	const consumerOnClick = (linkProps as AnyRecord).onClick as
		| ((event: LinkEvent) => void | Promise<void>)
		| undefined;
	if (isExternal) {
		return {
			dataExternal: true,
			safeAnchorProps,
			onClick: async (event: LinkEvent) => {
				try {
					await consumerOnClick?.(event);
					await linkProps.beforeBegin?.(event);
				} catch {
					// DOM event handlers should not leak unhandled rejections.
				}
			},
		};
	}
	const prefetchHandlers = createPrefetchHandlers<LinkEvent>({
		href,
		prefetch: linkProps.prefetch ?? "intent",
		prefetchDelayMs: linkProps.prefetchDelayMs,
		beforeBegin: linkProps.beforeBegin,
		eventProxyFactory: (callback) => (event) => {
			callback(event);
		},
	});

	return {
		dataExternal: undefined,
		safeAnchorProps,
		onPointerEnter: prefetchHandlers.onPointerEnter,
		onFocus: prefetchHandlers.onFocus,
		onPointerLeave: prefetchHandlers.onPointerLeave,
		onBlur: prefetchHandlers.onBlur,
		onTouchCancel: prefetchHandlers.onTouchCancel,
		onClick: async (event: LinkEvent) => {
			try {
				const clickEvent = event as AnyRecord;
				await consumerOnClick?.(event);
				if (clickEvent.defaultPrevented) {
					return;
				}
				if (getIsModifiedNavigationClick(clickEvent)) {
					return;
				}
				if (!getIsPrimaryNavigationClick(clickEvent)) {
					return;
				}
				if (
					!isSupportedInternalNavigationTarget(
						(linkProps as AnyRecord).target,
					)
				) {
					return;
				}

				prefetchHandlers.cancelPendingForClick?.();
				const shouldRunBeforeBeginForClick =
					!hasStartedPrefetchForDataTargetHref(href);

				const targetClassification =
					classifyNavigationTargetAgainstCurrentLocation({
						targetHref: href,
						currentHref: window.location.href,
					});
				if (targetClassification === "same-document-noop") {
					clickEvent.preventDefault?.();
					applyScrollState(
						resolveScrollStateForSameDocumentNoopLinkClick({
							targetHref: href,
							scrollToTop: linkProps.scrollToTop,
						}),
					);
					return;
				}
				if (targetClassification === "same-document-hash-change") {
					clickEvent.preventDefault?.();
					await vormaNavigate(href, {
						replace: linkProps.replace,
						scrollToTop: linkProps.scrollToTop,
						state: linkProps.state,
					});
					return;
				}

				clickEvent.preventDefault?.();
				await navigateWithLinkLifecycleCallbacks({
					href,
					event,
					replace: linkProps.replace,
					scrollToTop: linkProps.scrollToTop,
					state: linkProps.state,
					beforeBegin: shouldRunBeforeBeginForClick
						? linkProps.beforeBegin
						: undefined,
					beforeRender: linkProps.beforeRender,
					afterRender: linkProps.afterRender,
				});
			} catch (error) {
				console.error("Vorma:", "Link click navigation failed", error);
			}
		},
	};
}

export function buildNavigationLinkAnchorRenderProps<
	AnchorProps extends { href?: string },
	LinkEvent,
>(props: AnchorProps & VormaLinkPropsBase<LinkEvent>) {
	return makeFinalLinkProps<LinkEvent>({
		...props,
		href: props.href,
	});
}

/////////////////////////////////////////////////////////////////////
/////// Typed Adapter Link Factory
/////////////////////////////////////////////////////////////////////

type TypedLinkRouteResolutionInput<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
> = PermissivePatternBasedProps<App, Pattern> & {
	search?: string;
	hash?: string;
};

type TypedLinkResolvedNonRouteProps<AnchorProps, LinkEvent> = Omit<
	AnchorProps,
	"href" | "pattern"
> &
	VormaLinkPropsBase<LinkEvent>;

function stripTypedLinkRouteResolutionProps<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
	AnchorProps extends object,
	LinkEvent,
>(
	props: TypedAdapterLinkProps<App, Pattern, AnchorProps, LinkEvent>,
): TypedLinkResolvedNonRouteProps<AnchorProps, LinkEvent> {
	const nextProps: AnyRecord = {
		...props,
	};
	delete nextProps.pattern;
	delete nextProps.params;
	delete nextProps.splatValues;
	delete nextProps.search;
	delete nextProps.hash;
	return nextProps as TypedLinkResolvedNonRouteProps<AnchorProps, LinkEvent>;
}

function buildTypedLinkHrefForRouteResolution<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
>(props: {
	vormaAppConfig: VormaAppConfig;
	routeResolutionInput: TypedLinkRouteResolutionInput<App, Pattern>;
}): string {
	const routeResolutionInput = props.routeResolutionInput as AnyRecord;
	return resolveAbsoluteHrefWithOptionalSearchAndHash({
		href: resolveVormaPath({
			vormaAppConfig: props.vormaAppConfig,
			type: "loader",
			props: {
				pattern: props.routeResolutionInput.pattern,
				params: routeResolutionInput.params,
				splatValues: routeResolutionInput.splatValues,
			},
		}),
		search: props.routeResolutionInput.search,
		hash: props.routeResolutionInput.hash,
	});
}

function buildTypedLinkResolvedProps<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
	AnchorProps extends object,
	LinkEvent,
>(props: {
	vormaAppConfig: VormaAppConfig;
	rawLinkProps: TypedAdapterLinkProps<App, Pattern, AnchorProps, LinkEvent>;
	defaultLinkProps?: TypedAdapterLinkDefaultProps<
		App,
		AnchorProps,
		LinkEvent
	>;
}): {
	href: string;
	state: unknown;
	linkProps: TypedLinkResolvedNonRouteProps<AnchorProps, LinkEvent>;
} {
	const mergedLinkProps = {
		...props.defaultLinkProps,
		...props.rawLinkProps,
	} as TypedAdapterLinkProps<App, Pattern, AnchorProps, LinkEvent>;
	const href = buildTypedLinkHrefForRouteResolution({
		vormaAppConfig: props.vormaAppConfig,
		routeResolutionInput: mergedLinkProps,
	});
	const linkProps = stripTypedLinkRouteResolutionProps(mergedLinkProps);
	return {
		href,
		state: mergedLinkProps.state,
		linkProps: linkProps as TypedLinkResolvedNonRouteProps<
			AnchorProps,
			LinkEvent
		>,
	};
}

export function buildTypedLinkDisplayName(props: {
	namePrefix?: string;
}): string {
	return `${props.namePrefix ?? "Vorma"}TypedLink`;
}

export function createTypedAdapterLinkFactory<
	App extends VormaAppBase,
	AnchorProps extends object,
	LinkEvent,
	RenderOutput,
>(props: {
	vormaAppConfig: VormaAppConfig;
	defaultProps?: TypedAdapterLinkDefaultProps<App, AnchorProps, LinkEvent>;
	renderTypedLink: <Pattern extends VormaLoaderPattern<App>>(props: {
		resolveTypedLinkProps: () => {
			href: string;
			state: unknown;
			linkProps: TypedLinkResolvedNonRouteProps<AnchorProps, LinkEvent>;
		};
		rawLinkProps: TypedAdapterLinkProps<
			App,
			Pattern,
			AnchorProps,
			LinkEvent
		>;
	}) => RenderOutput;
}): <Pattern extends VormaLoaderPattern<App>>(
	linkProps: TypedAdapterLinkProps<App, Pattern, AnchorProps, LinkEvent>,
) => RenderOutput {
	const TypedAdapterLink = <Pattern extends VormaLoaderPattern<App>>(
		rawLinkProps: TypedAdapterLinkProps<
			App,
			Pattern,
			AnchorProps,
			LinkEvent
		>,
	): RenderOutput => {
		return props.renderTypedLink({
			resolveTypedLinkProps: () =>
				buildTypedLinkResolvedProps({
					vormaAppConfig: props.vormaAppConfig,
					rawLinkProps,
					defaultLinkProps: props.defaultProps,
				}),
			rawLinkProps,
		});
	};
	(TypedAdapterLink as AnyRecord).displayName = buildTypedLinkDisplayName({});
	return TypedAdapterLink;
}

/////////////////////////////////////////////////////////////////////
/////// Init Client
/////////////////////////////////////////////////////////////////////

type InitClientOptions = {
	isDev?: boolean;
	viteDevURL?: string;
	publicPathPrefix?: string;
	vormaAppConfig?: VormaAppConfig;
	routeManifestURL?: string;
	renderFn?: () => void | Promise<void>;
	defaultErrorBoundary?: typeof defaultErrorBoundary;
	useViewTransitions?: boolean;
	rootElementID?: string;
};

export type InitClientInput = InitClientOptions;

function initializePatternRegistryForGlobalState(
	globalState: VormaClientGlobal,
): void {
	const patternRegistry = createPatternRegistryForAppConfig(
		globalState.vormaAppConfig,
	);
	for (const pattern of Object.keys(globalState.patternToWaitFnMap)) {
		registerPattern(patternRegistry, pattern);
	}
	globalState.patternRegistry = patternRegistry;
}

function applyRouteManifest(props: {
	globalState: VormaClientGlobal;
	manifest: RouteManifestRecord;
}): void {
	for (const pattern of Object.keys(props.manifest)) {
		registerPattern(props.globalState.patternRegistry, pattern);
	}
	props.globalState.routeManifest = props.manifest;
}

function initializePatternRegistryFromPrecompiledRouteManifest(props: {
	globalState: VormaClientGlobal;
}): boolean {
	const precompiledRouteManifest = props.globalState.routeManifest;
	if (!precompiledRouteManifest) {
		return false;
	}
	applyRouteManifest({
		globalState: props.globalState,
		manifest: precompiledRouteManifest,
	});
	return true;
}

async function loadRouteManifestProgressively(props: {
	globalState: VormaClientGlobal;
}): Promise<void> {
	const routeManifestURL = props.globalState.routeManifestURL;
	if (!routeManifestURL) {
		return;
	}
	const progressiveLoadID =
		props.globalState.nextRouteManifestProgressiveLoadID + 1;
	props.globalState.nextRouteManifestProgressiveLoadID = progressiveLoadID;
	const initialPatternRegistry = props.globalState.patternRegistry;
	let response: Response;
	try {
		response = await window.fetch(routeManifestURL, {
			method: "GET",
		});
	} catch (error) {
		console.warn("Failed to load route manifest:", error);
		return;
	}
	if (!response.ok) {
		console.warn(
			"Failed to load route manifest:",
			new Error(
				`Route manifest request failed with status ${response.status}.`,
			),
		);
		return;
	}
	const manifest = (await response.json()) as RouteManifestRecord;
	if (
		props.globalState.nextRouteManifestProgressiveLoadID !==
		progressiveLoadID
	) {
		return;
	}
	if (props.globalState.patternRegistry !== initialPatternRegistry) {
		return;
	}
	applyRouteManifest({
		globalState: props.globalState,
		manifest,
	});
}

function setTouchInputModalityActive(props: {
	globalState: VormaClientGlobal;
}): void {
	if (props.globalState.isTouchInputModalityActive) {
		return;
	}
	props.globalState.isTouchInputModalityActive = true;
}

function setFinePointerInputModalityActive(props: {
	globalState: VormaClientGlobal;
}): void {
	if (!props.globalState.isTouchInputModalityActive) {
		return;
	}
	props.globalState.isTouchInputModalityActive = false;
}

function onPointerModalityChanged(event: Event): void {
	const globalState = ensureGlobalState();
	const pointerType = (
		event as Event & {
			pointerType?: unknown;
		}
	).pointerType;
	if (typeof pointerType !== "string") {
		return;
	}
	const normalizedPointerType = pointerType.toLowerCase();
	if (normalizedPointerType === "touch") {
		setTouchInputModalityActive({
			globalState,
		});
		return;
	}
	if (normalizedPointerType === "mouse" || normalizedPointerType === "pen") {
		setFinePointerInputModalityActive({
			globalState,
		});
	}
}

function registerInputModalityDetectionListenersIfNeeded(props: {
	globalState: VormaClientGlobal;
}): void {
	if (props.globalState.hasRegisteredInputModalityDetectionListeners) {
		return;
	}
	addWindowEventListener({
		eventName: "touchstart",
		listener: () => {
			setTouchInputModalityActive({
				globalState: ensureGlobalState(),
			});
		},
	});
	addWindowEventListener({
		eventName: "pointerdown",
		listener: onPointerModalityChanged,
	});
	addWindowEventListener({
		eventName: "pointermove",
		listener: onPointerModalityChanged,
	});
	addWindowEventListener({
		eventName: "pointerenter",
		listener: onPointerModalityChanged,
	});
	props.globalState.hasRegisteredInputModalityDetectionListeners = true;
}

async function loadRouteManifestFromURL(props: {
	globalState: VormaClientGlobal;
}): Promise<void> {
	const didInitializePatternRegistryFromPrecompiledManifest =
		initializePatternRegistryFromPrecompiledRouteManifest({
			globalState: props.globalState,
		});
	if (didInitializePatternRegistryFromPrecompiledManifest) {
		return;
	}
	await loadRouteManifestProgressively({
		globalState: props.globalState,
	});
}

async function bootstrapInitialRuntimeFromSnapshot(
	globalState: VormaClientGlobal,
): Promise<void> {
	const snapshot = globalState.runtimeRouteSnapshot;
	const modulesMap = await loadComponentModules(snapshot.importURLs);
	const activeComponents = buildActiveComponentsFromModules({
		importURLs: snapshot.importURLs,
		exportKeys: snapshot.exportKeys,
		modulesMap,
	});
	const activeErrorBoundary = resolveActiveErrorBoundaryFromModules({
		importURLs: snapshot.importURLs,
		errorExportKeys: snapshot.errorExportKeys,
		outermostErrorIdx: snapshot.outermostServerErrorIdx,
		modulesMap,
	});
	const initialClientLoaderResult = await completeClientLoaders({
		nextSnapshot: snapshot,
		signal: new AbortController().signal,
	});

	setRuntimeRouteSnapshot({
		...snapshot,
		activeComponents,
		activeErrorBoundary,
		clientLoadersData: initialClientLoaderResult.clientLoadersData,
		outermostClientError: initialClientLoaderResult.outermostClientError,
		outermostClientErrorIdx:
			initialClientLoaderResult.outermostClientErrorIdx,
		outermostError:
			initialClientLoaderResult.outermostClientError ??
			snapshot.outermostServerError,
		outermostErrorIdx:
			initialClientLoaderResult.outermostClientErrorIdx ??
			snapshot.outermostServerErrorIdx,
	});
}

function cleanVormaReloadFromCurrentURLIfPresent(): void {
	const currentURL = new URL(window.location.href);
	if (!currentURL.searchParams.has("vorma_reload")) {
		return;
	}
	currentURL.searchParams.delete("vorma_reload");
	window.history.replaceState(
		window.history.state,
		"",
		`${currentURL.pathname}${currentURL.search}${currentURL.hash}`,
	);
}

function exposeDevRevalidateHandleOnWindow(): void {
	(window as AnyPropertyRecord).__waveRevalidate = revalidate;
}

export async function initClient(options: InitClientInput): Promise<void> {
	const globalState = ensureGlobalState();
	cleanVormaReloadFromCurrentURLIfPresent();
	globalState.isDev = options.isDev ?? globalState.isDev;
	globalState.viteDevURL = options.viteDevURL ?? globalState.viteDevURL;
	globalState.publicPathPrefix =
		options.publicPathPrefix ?? globalState.publicPathPrefix;
	globalState.defaultErrorBoundary =
		options.defaultErrorBoundary ?? globalState.defaultErrorBoundary;
	globalState.useViewTransitions =
		options.useViewTransitions ?? globalState.useViewTransitions;
	if (options.vormaAppConfig) {
		globalState.vormaAppConfig = options.vormaAppConfig;
	}
	if (options.routeManifestURL !== undefined) {
		globalState.routeManifestURL = options.routeManifestURL;
	}
	if (options.rootElementID !== undefined) {
		updateRuntimeRouteSnapshot({
			patch: {
				rootElementID: options.rootElementID,
			},
		});
	}

	initializePatternRegistryForGlobalState(globalState);
	await loadRouteManifestFromURL({
		globalState,
	});
	registerInputModalityDetectionListenersIfNeeded({
		globalState,
	});
	if (import.meta.env.DEV) {
		void import("vorma/client/__internal/hmr_dev").then(
			({ registerViteAfterUpdateListenerIfNeeded }) => {
				registerViteAfterUpdateListenerIfNeeded(import.meta.hot);
			},
		);
	}

	const runtimeContext = createVormaRuntimeContext();
	setDefaultVormaRuntimeContext(runtimeContext);
	getOrCreateHistoryInstance();
	setHistoryScrollRestorationToManualWhenSupported();
	registerBeforeUnloadScrollPersistenceListenerOnce();
	await bootstrapInitialRuntimeFromSnapshot(globalState);
	if (options.renderFn) {
		await options.renderFn();
	}
	exposeDevRevalidateHandleOnWindow();
	dispatchLocationEvent(getLocation());
	dispatchRouteChangeEvent({});
	dispatchStatusEvent(getStatus());
	restoreRecentPageRefreshScrollState();
}
