import type { Action, BrowserHistory, Location, Update } from "history";
import { createBrowserHistory } from "history";
import { debounce } from "vorma/kit/debounce";
import { jsonDeepEquals, serializeToSearchParams } from "vorma/kit/json";
import { addOnWindowFocusListener } from "vorma/kit/listeners";
import { findNestedMatches } from "vorma/kit/matcher/find-nested";
import type { PatternRegistry } from "vorma/kit/matcher/register";
import {
	createPatternRegistry,
	registerPattern,
} from "vorma/kit/matcher/register";
import type { HrefDetails } from "vorma/kit/url";
import {
	getAnchorDetailsFromEvent,
	getHrefDetails,
	getIsGETRequest,
	resolveAbsoluteHref,
	resolveAbsoluteHrefWithOptionalSearchAndHash,
} from "vorma/kit/url";

/////////////////////////////////////////////////////////////////////
/////// BUILDTIME
/////////////////////////////////////////////////////////////////////

export type BuildtimeImportPromise = Promise<Record<string, any>>;
export type BuildtimeImportKey<T extends BuildtimeImportPromise> =
	keyof Awaited<T>;

/////////////////////////////////////////////////////////////////////
/////// RUNTIME EVENTS
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

/////////////////////////////////////////////////////////////////////
/////// NAVIGATION / SUBMIT
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
/////// APP / ROUTE TYPING
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

export type VormaRouteParams<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
> = GetParams<App, Pattern>;

export type HasParams<App extends VormaAppBase, Pattern extends string> =
	GetParams<App, Pattern> extends never ? false : true;

export type IsSplat<App extends VormaAppBase, Pattern extends string> =
	RouteMetadata<App, Pattern> extends { isSplat: true } ? true : false;

export type IsEmptyInput<T> = [T] extends [null | undefined | never]
	? true
	: false;

export type QueryInputContractViolation = {
	__queryInputContractViolation: "Query input root must be an object, null, or undefined.";
};

export type EnforceQueryInputRootContract<Input> = [Input] extends [
	null | undefined | never,
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
/////// API CLIENT
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
	requestContext: APIRequestInitDecoratorContext<App>,
) => APIRequestInitOverrides | undefined;

export type TypedAPIClient<App extends VormaAppBase> = {
	query: <Pattern extends VormaQueryPattern<App>>(
		props: VormaQueryProps<App, Pattern>,
	) => Promise<SubmitResult<VormaQueryOutput<App, Pattern>>>;
	mutate: <Pattern extends VormaMutationPattern<App>>(
		props: VormaMutationProps<App, Pattern>,
	) => Promise<SubmitResult<VormaMutationOutput<App, Pattern>>>;
};

/////////////////////////////////////////////////////////////////////
/////// CLIENT LOADER
/////////////////////////////////////////////////////////////////////

export type ClientLoaderAwaitedServerData<RootData, LoaderData> = {
	matchedPatterns: string[];
	loaderData: LoaderData;
	rootData: RootData;
	buildID: string;
};

/////////////////////////////////////////////////////////////////////
/////// LINK / ROUTER HOOK TYPES
/////////////////////////////////////////////////////////////////////

export type VormaLinkPropsBase<LinkEvent = unknown> = {
	href?: string;
	prefetch?: "intent";
	prefetchDelayMs?: number;
	beforeBegin?: LinkOnClickCallback<LinkEvent>;
	beforeRender?: LinkOnClickCallback<LinkEvent>;
	afterRender?: LinkOnClickCallback<LinkEvent>;
	scrollToTop?: boolean;
	replace?: boolean;
	state?: unknown;
};

export type VormaRouteGeneric<
	JSXElement,
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = (props: VormaRoutePropsGeneric<JSXElement, App, Pattern>) => JSXElement;

export type ParamsForPattern<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
> = VormaRouteParams<App, Pattern>;

export type BaseRouterData<RootData, Params extends string> = {
	buildID: string;
	matchedPatterns: string[];
	splatValues: string[];
	params: Record<Params, string>;
	rootData: RootData;
};

export type UseRouterDataWrapper<
	UseAccessor extends boolean,
	WrappedValue,
> = UseAccessor extends false ? WrappedValue : () => WrappedValue;

export type UseRouterDataFunction<
	App extends VormaAppBase,
	UseAccessor extends boolean = false,
> = {
	<Pattern extends VormaLoaderPattern<App>>(
		props: VormaRoutePropsGeneric<unknown, App, Pattern>,
	): UseRouterDataWrapper<
		UseAccessor,
		BaseRouterData<App["rootData"], ParamsForPattern<App, Pattern>>
	>;
	<Pattern extends VormaLoaderPattern<App>>(): UseRouterDataWrapper<
		UseAccessor,
		BaseRouterData<App["rootData"], ParamsForPattern<App, Pattern>>
	>;
	(): UseRouterDataWrapper<
		UseAccessor,
		BaseRouterData<App["rootData"], string>
	>;
};

/////////////////////////////////////////////////////////////////////
/////// ROUTE OUTLET STORE
/////////////////////////////////////////////////////////////////////

export type RouteOutletRouterDataState = {
	buildID: string;
	matchedPatterns: string[];
	splatValues: string[];
	params: Record<string, string>;
	rootData: unknown;
};

export type RouteOutletNavigationState = {
	loadersData: Array<unknown>;
	clientLoadersData: Array<unknown>;
	routerData: RouteOutletRouterDataState;
	outermostError: string | undefined;
	outermostErrorIdx: number | undefined;
	activeComponents: Array<unknown> | null;
	activeErrorBoundary: unknown | undefined;
	importURLs: Array<string>;
	exportKeys: Array<string>;
};

export type RouteOutletLocationState = {
	pathname: string;
	search: string;
	hash: string;
	state: unknown;
};

export type RouteOutletBranchInputState = Pick<
	RouteOutletNavigationState,
	| "outermostErrorIdx"
	| "activeComponents"
	| "activeErrorBoundary"
	| "importURLs"
	| "exportKeys"
> & {
	loaderCount: number;
	matchedPatterns: readonly string[];
};

export type RouteOutletStoreState = {
	navigation: RouteOutletNavigationState;
	routeOutletBranchInputState: RouteOutletBranchInputState;
	location: RouteOutletLocationState;
};

/////////////////////////////////////////////////////////////////////
/////// TYPED ADAPTER CLIENT LOADER
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
};

/////////////////////////////////////////////////////////////////////
/////// TYPED ADAPTER LINK
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
/////// Shared Runtime Domain Types
/////////////////////////////////////////////////////////////////////

export type GlobalCtorName =
	| "FormData"
	| "URLSearchParams"
	| "Blob"
	| "ArrayBuffer"
	| "ReadableStream";

export type NavigationTargetClassificationAgainstCurrentLocation =
	| "same-document-noop"
	| "hash-change"
	| "navigate";

export type HistoryLocationPrelude = Pick<
	Location,
	"key" | "pathname" | "search"
>;

export type PageRefreshSnapshot = {
	x: number;
	y: number;
	unix: number;
	href: string;
};

export type BuildIDEventDetail = { oldID: string; newID: string };

/////////////////////////////////////////////////////////////////////
/////// Request Body Transport
/////////////////////////////////////////////////////////////////////

export type RequestBodyTransportResolution = {
	body: BodyInit | null | undefined;
	didSerializeJSON: boolean;
};

/////////////////////////////////////////////////////////////////////
/////// Redirect Handling
/////////////////////////////////////////////////////////////////////

export type RedirectData = { href: string; hrefDetails: HrefDetails } & (
	| {
			status: "did";
	  }
	| {
			status: "should";
			shouldRedirectStrategy: "hard" | "soft";
			latestBuildID: string;
	  }
);

export type HTTPHrefDetails = Extract<HrefDetails, { isHTTP: true }>;

export type ShouldRedirectData = Extract<RedirectData, { status: "should" }>;

export type RedirectNavigationState = {
	getNavigations: () => Map<string, NavigationEntry>;
	removeNavigation: (targetUrl: string) => void;
	navigate: (props: NavigateProps) => Promise<{ didNavigate: boolean }>;
};

export type RedirectRequestFlowInput = {
	abortController: AbortController;
	url: URL;
	requestInit?: RequestInit;
	redirectCount?: number;
};

export type RedirectRequestFlowResult =
	| {
			kind: "too_many_redirects";
	  }
	| {
			kind: "ok";
			response: Response;
	  };

/////////////////////////////////////////////////////////////////////
/////// Server Route Data Fetch Contracts
/////////////////////////////////////////////////////////////////////

export type ServerSuccessPreloadPlan = {
	moduleDependencies: string[];
	cssBundles: string[];
};

export type RouteMatch = {
	registeredPattern: {
		originalPattern: string;
	};
};

export type ServerRouteDataResult = {
	redirectData: RedirectData | null;
	response?: Response;
	json?: CanonicalRouteDataPayload;
};

export type ResolvedServerRouteDataResult =
	| {
			type: "outcome";
			outcome:
				| Extract<NavigationOutcome, { type: "aborted" }>
				| Extract<NavigationOutcome, { type: "redirect" }>;
	  }
	| {
			type: "success";
			response: Response;
			json: CanonicalRouteDataPayload;
	  };

/////////////////////////////////////////////////////////////////////
/////// Navigation Core Types
/////////////////////////////////////////////////////////////////////

/**
 * Canonical route-data payload after ingress decode/validation.
 * Runtime navigation paths must only consume this decoded shape.
 */
export type CanonicalRouteDataPayload = RouteDataState &
	Meta & {
		deps: Array<string>;
		cssBundles: Array<string>;
	};

export type VormaNavigationType =
	| "browserHistory"
	| "userNavigation"
	| "revalidation"
	| "redirect"
	| "prefetch"
	| "action";

export type NavigateProps = {
	href: string;
	state?: unknown;
	navigationType: VormaNavigationType;
	scrollStateToRestore?: ScrollState;
	replace?: boolean;
	redirectCount?: number;
	scrollToTop?: boolean;
};

export type NavigationOutcome =
	| { type: "aborted" }
	| { type: "redirect"; redirectData: RedirectData; props: NavigateProps }
	| {
			type: "success";
			response: Response;
			json: CanonicalRouteDataPayload;
			preloadPlan: ServerSuccessPreloadPlan;
			waitFnPromise: Promise<ClientLoadersResult> | undefined;
			props: NavigateProps;
	  };

export type SuccessfulNavigationOutcome = Extract<
	NavigationOutcome,
	{ type: "success" }
>;

export type NavigationControl = {
	abortController: AbortController | undefined;
	promise: Promise<NavigationOutcome>;
	operationID?: number;
};

export type NavigationLifecyclePhase =
	| "idle"
	| "fetching"
	| "waiting"
	| "rendering"
	| "complete";

export type NavigationPhase = Exclude<NavigationLifecyclePhase, "idle">;

export type NavigationIntent = "none" | "navigate" | "revalidate";

export type NavigationEntry = {
	operationID: number;
	control: NavigationControl;
	type: VormaNavigationType;
	intent: NavigationIntent;
	startTime: number;
	targetUrl: string;
	originUrl: string;
	scrollToTop?: boolean;
	replace?: boolean;
	state?: unknown;
};

export type SubmissionEntry = {
	operationID: number;
	control: {
		abortController: AbortController | undefined;
		promise: Promise<unknown>;
	};
	startTime: number;
	skipGlobalLoadingIndicator?: boolean;
};

export type RuntimeLanes = {
	active: NavigationEntry | null;
	revalidation: NavigationEntry | null;
	prefetch: Map<string, NavigationEntry>;
	submissions: Map<string | symbol, SubmissionEntry>;
};

export type NavigationLanes = Omit<RuntimeLanes, "submissions">;

export type FindNavigationEntryByTargetUrl = (
	targetUrl: string,
) => NavigationEntry | undefined;

export type DeleteNavigationByTargetUrl = (props: {
	targetUrl: string;
	reason: string;
	causedByOperationID?: number | null;
}) => boolean;

export type ProcessSuccessfulNavigationOutcome = (
	outcome: SuccessfulNavigationOutcome,
	entry: NavigationEntry,
) => Promise<void>;

export type NavigationStateManager = {
	_submissions: Map<string | symbol, SubmissionEntry>;
	navigate: (props: NavigateProps) => Promise<{ didNavigate: boolean }>;
	beginNavigation: (props: NavigateProps) => NavigationControl;
	processSuccessfulNavigation: ProcessSuccessfulNavigationOutcome;
	submit: <T = unknown>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: SubmitOptions,
	) => Promise<SubmitResult<T>>;
	removeNavigation: (targetUrl: string) => void;
	getNavigation: (targetUrl: string) => NavigationEntry | undefined;
	hasNavigation: (targetUrl: string) => boolean;
	getNavigationsSize: () => number;
	getNavigations: () => Map<string, NavigationEntry>;
	getStatus: () => StatusEventDetail;
	clearAll: () => void;
};

/////////////////////////////////////////////////////////////////////
/////// Begin Navigation Planning
/////////////////////////////////////////////////////////////////////

export type BeginNavigationAbortInstruction =
	| {
			slot: "active";
			entry: NavigationEntry;
	  }
	| {
			slot: "revalidation";
			entry: NavigationEntry;
	  }
	| {
			slot: "prefetch";
			key: string;
			entry: NavigationEntry;
	  };

export type BeginNavigationPromotion = {
	targetUrl: string;
	type: NavigationEntry["type"];
	intent: NavigationIntent;
	scrollToTop: NavigateProps["scrollToTop"];
	replace: NavigateProps["replace"];
	state: NavigateProps["state"];
};

export type BeginNavigationReuseInstruction = {
	sourceSlot: "active" | "revalidation" | "prefetch";
	sourcePrefetchKey: string | null;
	entry: NavigationEntry;
	promotion: BeginNavigationPromotion | null;
};

export type BeginNavigationCreateInstruction =
	| {
			slot: "active";
	  }
	| {
			slot: "prefetch";
			targetUrl: string;
	  }
	| {
			slot: "revalidation";
			revalidationHref: string;
	  };

export type BeginNavigationExecutionPlanBase = {
	abortInstructions: BeginNavigationAbortInstruction[];
};

export type BeginNavigationExecutionPlan =
	| (BeginNavigationExecutionPlanBase & {
			type: "reuse";
			reuseInstruction: BeginNavigationReuseInstruction;
	  })
	| (BeginNavigationExecutionPlanBase & {
			type: "create";
			createInstruction: BeginNavigationCreateInstruction;
	  })
	| (BeginNavigationExecutionPlanBase & {
			type: "immediateAbort";
	  });

export type BeginNavigationLaneSnapshot = {
	active: NavigationEntry | null;
	revalidation: NavigationEntry | null;
	prefetch: Map<string, NavigationEntry>;
};

/////////////////////////////////////////////////////////////////////
/////// Begin Navigation Runtime Commands
/////////////////////////////////////////////////////////////////////

export type BeginNavigationFetchRouteDataFn = (
	controller: AbortController,
	props: NavigateProps,
) => Promise<NavigationOutcome>;

export type BeginNavigationContext = {
	getActiveNavigation: () => NavigationEntry | null;
	setActiveNavigation: (entry: NavigationEntry | null) => void;
	getRevalidationNavigation: () => NavigationEntry | null;
	setRevalidationNavigation: (entry: NavigationEntry | null) => void;
	prefetchNavigationsByTargetUrl: Map<string, NavigationEntry>;
	scheduleStatusUpdate: () => void;
	fetchRouteData: BeginNavigationFetchRouteDataFn;
	deleteNavigation: DeleteNavigationByTargetUrl;
	allocateNavigationOperationID: () => number;
};

export type BeginNavigationRuntimeCommand =
	| {
			type: "abort_navigation_entry";
			abortInstruction: BeginNavigationAbortInstruction;
	  }
	| {
			type: "apply_reuse_instruction";
			reuseInstruction: BeginNavigationReuseInstruction;
	  }
	| {
			type: "schedule_status_update_if_changed";
	  };

export type BeginNavigationRuntimeCommandPlanTerminalResult =
	| {
			type: "return_reused_control";
	  }
	| {
			type: "return_immediately_aborted_control";
	  }
	| {
			type: "create_navigation_control";
			createInstruction: BeginNavigationCreateInstruction;
			navigationProps: NavigateProps;
	  };

export type BeginNavigationRuntimeCommandPlan = {
	commands: BeginNavigationRuntimeCommand[];
	terminalResult: BeginNavigationRuntimeCommandPlanTerminalResult;
};

/////////////////////////////////////////////////////////////////////
/////// Navigation Lanes And Status Slots
/////////////////////////////////////////////////////////////////////

export type NavigationLaneMatch =
	| {
			lane: "active";
			entry: NavigationEntry;
	  }
	| {
			lane: "prefetch";
			key: string;
			entry: NavigationEntry;
	  }
	| {
			lane: "revalidation";
			entry: NavigationEntry;
	  };

/////////////////////////////////////////////////////////////////////
/////// Navigation Runtime Reason Tags
/////////////////////////////////////////////////////////////////////

type NavigationOutcomeExecutionReasonTags =
	typeof navigationOutcomeExecutionReason;
type SuccessfulNavigationLifecycleReasonTags =
	typeof successfulNavigationLifecycleReason;

export type NavigationOutcomeStopReason =
	NavigationOutcomeExecutionReasonTags["stop"][keyof NavigationOutcomeExecutionReasonTags["stop"]];

export type NavigationOutcomeDeleteAndStopReason =
	NavigationOutcomeExecutionReasonTags["deleteAndStop"][keyof NavigationOutcomeExecutionReasonTags["deleteAndStop"]];

export type NavigationOutcomeRedirectReason =
	NavigationOutcomeExecutionReasonTags["redirect"]["effectuate"];

export type NavigationOutcomeSuccessReason =
	NavigationOutcomeExecutionReasonTags["success"]["process"];

export type SuccessfulNavigationPreWaitingStopReason =
	SuccessfulNavigationLifecycleReasonTags["preWaiting"]["nonCurrentEntry"];

export type SuccessfulNavigationPreWaitingDeleteAndStopReason =
	SuccessfulNavigationLifecycleReasonTags["preWaiting"]["staleRevalidationPreWaiting"];

export type SuccessfulNavigationPreWaitingContinueReason =
	SuccessfulNavigationLifecycleReasonTags["preWaiting"]["entryCurrentAndFresh"];

export type SuccessfulNavigationPostWaitingStopReason =
	SuccessfulNavigationLifecycleReasonTags["postWaiting"]["nonCurrentEntry"];

export type SuccessfulNavigationPostWaitingContinueReason =
	SuccessfulNavigationLifecycleReasonTags["postWaiting"]["entryCurrentAndFresh"];

export type SuccessfulNavigationPostAssetStopReason =
	| SuccessfulNavigationLifecycleReasonTags["postAsset"]["entryLost"]
	| SuccessfulNavigationLifecycleReasonTags["postAsset"]["staleRevalidation"];

export type SuccessfulNavigationPostAssetCompleteWithoutRenderReason =
	SuccessfulNavigationLifecycleReasonTags["postAsset"]["idlePrefetch"];

export type SuccessfulNavigationPostAssetRenderReason =
	SuccessfulNavigationLifecycleReasonTags["postAsset"]["render"];

export type SuccessfulNavigationCleanupDeleteReason =
	SuccessfulNavigationLifecycleReasonTags["cleanup"]["successfulNavigation"];

export type SuccessfulNavigationCleanupSkipReason =
	SuccessfulNavigationLifecycleReasonTags["cleanup"]["skippedIdlePrefetchOrNonCurrentEntry"];

export type NavigationPromiseRejectedReason =
	SuccessfulNavigationLifecycleReasonTags["internalNavigateResult"]["navigatePromiseRejected"];

export type InternalNavigateResult =
	| {
			type: "committed";
			didNavigate: boolean;
	  }
	| {
			type: "cancelled";
			reason: NavigationOutcomeExecutionPlan["reason"];
	  }
	| {
			type: "failed";
			reason: NavigationPromiseRejectedReason;
	  };

export type ExecuteSubmitRuntimeCommandListWithStaleOwnershipResult<
	TSubmitResultData,
> =
	| {
			type: "stale";
			result: SubmitResult<TSubmitResultData>;
	  }
	| {
			type: "executed";
			redirectResult: RedirectData | null | undefined;
	  };

export type SuccessfulNavigationRuntimeDeleteReason =
	| SuccessfulNavigationPreWaitingDeleteAndStopReason
	| SuccessfulNavigationCleanupDeleteReason;

export type SuccessfulNavigationRuntimeCommand =
	| {
			type: "transition_to_waiting";
	  }
	| {
			type: "transition_to_complete";
	  }
	| {
			type: "delete_navigation";
			targetUrl: string;
			reason: SuccessfulNavigationRuntimeDeleteReason;
	  }
	| {
			type: "sync_build_id_from_response";
	  }
	| {
			type: "render_navigation";
	  };

export type SuccessfulNavigationRuntimeCommandPlan = {
	shouldStop: boolean;
	commands: SuccessfulNavigationRuntimeCommand[];
};

export type NavigationEntryLifecycleState =
	| "non_current"
	| "idle_prefetch"
	| "stale_revalidation"
	| "current_fresh";

export type NavigationOutcomeExecutionPlan =
	| {
			type: "stop";
			reason: NavigationOutcomeStopReason;
	  }
	| {
			type: "deleteAndStop";
			targetUrl: string;
			reason: NavigationOutcomeDeleteAndStopReason;
	  }
	| {
			type: "redirect";
			entry: NavigationEntry;
			outcome: Extract<NavigationOutcome, { type: "redirect" }>;
			reason: NavigationOutcomeRedirectReason;
	  }
	| {
			type: "success";
			entry: NavigationEntry;
			outcome: SuccessfulNavigationOutcome;
			didNavigate: boolean;
			reason: NavigationOutcomeSuccessReason;
	  };

export type NavigationOutcomeRuntimeCommandReason =
	| NavigationOutcomeDeleteAndStopReason
	| NavigationOutcomeRedirectReason;

export type InternalCancelledNavigateReason = Extract<
	InternalNavigateResult,
	{ type: "cancelled" }
>["reason"];

export type NavigationOutcomeRedirectOutcome = Extract<
	NavigationOutcome,
	{ type: "redirect" }
>;

export type NavigationOutcomeSuccessOutcome = Extract<
	NavigationOutcome,
	{ type: "success" }
>;

export type NavigationOutcomeRuntimeCommand =
	| {
			type: "delete_navigation";
			targetUrl: string;
			reason: NavigationOutcomeRuntimeCommandReason;
	  }
	| {
			type: "sync_redirect_build_id";
			redirectOutcome: NavigationOutcomeRedirectOutcome;
	  }
	| {
			type: "effectuate_redirect";
			redirectOutcome: NavigationOutcomeRedirectOutcome;
			redirectCount: number;
			navigationProps: NavigateProps;
	  }
	| {
			type: "process_successful_navigation";
			successOutcome: NavigationOutcomeSuccessOutcome;
			entry: NavigationEntry;
	  };

export type NavigationOutcomeRuntimeCommandPlan =
	| {
			terminalResult: {
				type: "cancelled";
				reason: InternalCancelledNavigateReason;
			};
			commands: NavigationOutcomeRuntimeCommand[];
	  }
	| {
			terminalResult: {
				type: "committed";
				didNavigate: boolean;
			};
			commands: NavigationOutcomeRuntimeCommand[];
	  }
	| {
			terminalResult: {
				type: "committed_from_redirect";
			};
			commands: NavigationOutcomeRuntimeCommand[];
	  };

export type NavigationPassRejectionExecutionPlan =
	| {
			type: "deleteAndReport";
			targetUrl: string;
			ownedEntry: NavigationEntry;
	  }
	| {
			type: "report";
			targetUrl: string;
	  };

export type NavigationPassRuntimeCommand = {
	type: "delete_navigation";
	targetUrl: string;
	reason: NavigationPromiseRejectedReason;
};

export type NavigationPassRuntimeCommandPlan = {
	commands: NavigationPassRuntimeCommand[];
	report: {
		targetUrl: string;
		ownedEntry: NavigationEntry | undefined;
	};
};

export type NavigationPassReducerState = {
	targetUrl: string;
	currentHref: string;
	entry: NavigationEntry | undefined;
	expectedOperationID: number | undefined;
};

export type NavigationPassReducerEvent =
	| {
			type: "outcome_resolved";
			outcome: NavigationOutcome;
			navigationProps: NavigateProps;
	  }
	| {
			type: "outcome_rejected";
	  };

export type NavigationPassReducerTransition =
	| {
			type: "resolved";
			executionPlan: NavigationOutcomeExecutionPlan;
			commandPlan: NavigationOutcomeRuntimeCommandPlan;
	  }
	| {
			type: "rejected";
			executionPlan: NavigationPassRejectionExecutionPlan;
			commandPlan: NavigationPassRuntimeCommandPlan;
	  };

export type NavigationPassResolvedReducerTransition = Extract<
	NavigationPassReducerTransition,
	{ type: "resolved" }
>;

export type NavigationPassRejectedReducerTransition = Extract<
	NavigationPassReducerTransition,
	{ type: "rejected" }
>;

type NavigationEntryOwnershipState = {
	entry: NavigationEntry;
	isCurrentEntry: boolean;
};

type NavigationEntryOwnershipStateWithCurrentHref =
	NavigationEntryOwnershipState & {
		currentHref: string;
	};

type ExecutionPlanStop<Reason extends string> = {
	type: "stop";
	reason: Reason;
};

type ExecutionPlanContinue<Reason extends string> = {
	type: "continue";
	reason: Reason;
};

type ExecutionPlanDeleteAndStop<Reason extends string> = {
	type: "deleteAndStop";
	targetUrl: string;
	reason: Reason;
};

type ExecutionPlanDeleteNavigation<Reason extends string> = {
	type: "deleteNavigation";
	targetUrl: string;
	reason: Reason;
};

type ExecutionPlanSkip<Reason extends string> = {
	type: "skip";
	reason: Reason;
};

export type SuccessfulNavigationPreWaitingExecutionPlan =
	| ExecutionPlanStop<SuccessfulNavigationPreWaitingStopReason>
	| ExecutionPlanDeleteAndStop<SuccessfulNavigationPreWaitingDeleteAndStopReason>
	| ExecutionPlanContinue<SuccessfulNavigationPreWaitingContinueReason>;

export type SuccessfulNavigationPostWaitingExecutionPlan =
	| ExecutionPlanStop<SuccessfulNavigationPostWaitingStopReason>
	| ExecutionPlanContinue<SuccessfulNavigationPostWaitingContinueReason>;

export type SuccessfulNavigationPostAssetExecutionPlan =
	| ExecutionPlanStop<SuccessfulNavigationPostAssetStopReason>
	| {
			type: "completeWithoutRender";
			reason: SuccessfulNavigationPostAssetCompleteWithoutRenderReason;
	  }
	| {
			type: "render";
			reason: SuccessfulNavigationPostAssetRenderReason;
	  };

export type SuccessfulNavigationCleanupExecutionPlan =
	| ExecutionPlanDeleteNavigation<SuccessfulNavigationCleanupDeleteReason>
	| ExecutionPlanSkip<SuccessfulNavigationCleanupSkipReason>;

type SuccessfulNavigationCheckpointExecutionPlanByCheckpoint = {
	pre_waiting: {
		checkpoint: "pre_waiting";
		plan: SuccessfulNavigationPreWaitingExecutionPlan;
	};
	post_waiting: {
		checkpoint: "post_waiting";
		plan: SuccessfulNavigationPostWaitingExecutionPlan;
	};
	post_asset: {
		checkpoint: "post_asset";
		plan: SuccessfulNavigationPostAssetExecutionPlan;
	};
	cleanup: {
		checkpoint: "cleanup";
		plan: SuccessfulNavigationCleanupExecutionPlan;
	};
};

type SuccessfulNavigationCheckpointExecutionPlanPropsByCheckpoint = {
	pre_waiting: {
		checkpoint: "pre_waiting";
	} & NavigationEntryOwnershipStateWithCurrentHref;
	post_waiting: {
		checkpoint: "post_waiting";
	} & NavigationEntryOwnershipStateWithCurrentHref;
	post_asset: {
		checkpoint: "post_asset";
	} & NavigationEntryOwnershipStateWithCurrentHref;
	cleanup: {
		checkpoint: "cleanup";
	} & NavigationEntryOwnershipState;
};

type SuccessfulNavigationCheckpointReducerEventByCheckpoint = {
	pre_waiting: SuccessfulNavigationCheckpointExecutionPlanPropsByCheckpoint["pre_waiting"];
	post_waiting: SuccessfulNavigationCheckpointExecutionPlanPropsByCheckpoint["post_waiting"];
	pre_asset_wait: {
		checkpoint: "pre_asset_wait";
		shouldSyncBuildIDBeforeAssetWait: boolean;
	};
	post_asset: SuccessfulNavigationCheckpointExecutionPlanPropsByCheckpoint["post_asset"] & {
		shouldSyncBuildIDAfterAssetWaitIfNotStopped: boolean;
	};
	cleanup: SuccessfulNavigationCheckpointExecutionPlanPropsByCheckpoint["cleanup"];
};

export type SuccessfulNavigationCheckpointReducerEvent =
	SuccessfulNavigationCheckpointReducerEventByCheckpoint[keyof SuccessfulNavigationCheckpointReducerEventByCheckpoint];

export type SuccessfulNavigationCheckpointExecutionPlan =
	SuccessfulNavigationCheckpointExecutionPlanByCheckpoint[keyof SuccessfulNavigationCheckpointExecutionPlanByCheckpoint];

export type SuccessfulNavigationCheckpointExecutionPlanProps =
	SuccessfulNavigationCheckpointExecutionPlanPropsByCheckpoint[keyof SuccessfulNavigationCheckpointExecutionPlanPropsByCheckpoint];

/////////////////////////////////////////////////////////////////////
/////// Navigation Pass Runtime
/////////////////////////////////////////////////////////////////////

export type HandleNavigationOutcomeProps = {
	findNavigationEntry: FindNavigationEntryByTargetUrl;
	deleteNavigation: DeleteNavigationByTargetUrl;
	processSuccessfulNavigation: ProcessSuccessfulNavigationOutcome;
	navigationProps: NavigateProps;
	outcome: NavigationOutcome;
	expectedOperationID: number | undefined;
};

export type ExecuteNavigationSinglePassProps = {
	navigationProps: NavigateProps;
	beginNavigation: (props: NavigateProps) => NavigationControl;
	findNavigationEntry: FindNavigationEntryByTargetUrl;
	deleteNavigation: DeleteNavigationByTargetUrl;
	processSuccessfulNavigation: ProcessSuccessfulNavigationOutcome;
	onLifecycleEvent?: (props: NavigationSinglePassLifecycleEvent) => void;
	onNavigationPromiseRejected: (props: {
		targetUrl: string;
		ownedEntry: NavigationEntry | undefined;
	}) => void;
};

type NavigationSinglePassLifecycleSharedProps = {
	navigationProps: NavigateProps;
	targetUrl: string;
	operationID: number | undefined;
};

export type NavigationSinglePassLifecycleEvent =
	| (NavigationSinglePassLifecycleSharedProps & {
			type: "fetch-resolve";
			outcomeType: NavigationOutcome["type"];
	  })
	| (NavigationSinglePassLifecycleSharedProps & {
			type: "redirect";
	  })
	| (NavigationSinglePassLifecycleSharedProps & {
			type: "wait-resolve";
	  })
	| (NavigationSinglePassLifecycleSharedProps & {
			type: "render-finish";
	  })
	| (NavigationSinglePassLifecycleSharedProps & {
			type: "abort";
			reason: string;
	  });

export type ProcessSuccessfulNavigationContext = {
	transitionPhase: (props: {
		targetUrl: string;
		phase: NavigationPhase;
		reason: string;
	}) => void;
	findNavigationEntry: FindNavigationEntryByTargetUrl;
	deleteNavigation: DeleteNavigationByTargetUrl;
	onSuccessfulNavigationCommitted?: (props: {
		entry: NavigationEntry;
		outcome: SuccessfulNavigationOutcome;
	}) => void;
};

export type SuccessfulNavigationClientLoadersResult =
	| Awaited<SuccessfulNavigationOutcome["waitFnPromise"]>
	| undefined;

export type SuccessfulNavigationCheckpointStateEnvelope = {
	context: ProcessSuccessfulNavigationContext;
	outcome: SuccessfulNavigationOutcome;
	entry: NavigationEntry;
};

/////////////////////////////////////////////////////////////////////
/////// Revalidation Lane Runtime
/////////////////////////////////////////////////////////////////////

export type RevalidationNavigateResult = Promise<{ didNavigate: boolean }>;

export type RevalidationLaneState = {
	inFlightPromise: RevalidationNavigateResult | null;
	isTrailingEligible: boolean;
	shouldRunTrailingPass: boolean;
	trailingPromise: RevalidationNavigateResult | null;
	resolveTrailingPromise: ((result: { didNavigate: boolean }) => void) | null;
};

export type RevalidationLaneReducerState = {
	hasInFlightPass: boolean;
	isTrailingEligible: boolean;
	inFlightTargetMatchesCurrentHref: boolean;
};

export type RevalidationLaneReducerEvent = {
	type: "revalidation_requested";
};

export type RevalidationLaneExecutionPlan =
	| {
			type: "start_new_pass";
	  }
	| {
			type: "restart_after_target_mismatch";
	  }
	| {
			type: "reuse_in_flight_pass";
	  }
	| {
			type: "schedule_trailing_pass";
	  };

export type DeterministicRevalidationLane = {
	runRevalidation: (props: {
		navigateSinglePass: (
			props: NavigateProps,
		) => RevalidationNavigateResult;
	}) => RevalidationNavigateResult;
	clearQueuedTrailingRequest: () => void;
	reset: () => void;
};

/////////////////////////////////////////////////////////////////////
/////// Runtime Engine State Machine
/////////////////////////////////////////////////////////////////////

export type NavigationRuntimeEngineNavigationLane =
	| "navigate"
	| "prefetch"
	| "revalidate";

export type NavigationRuntimeEngineLane =
	| NavigationRuntimeEngineNavigationLane
	| "submit";

export type NavigationRuntimeEngineSubmitPhase =
	| "idle"
	| "submitting"
	| "complete";

export type OperationOwnershipState = "none" | "current" | "stale";

export type NavigationRuntimeEngineNavigationLaneState = {
	targetUrl: string | null;
	operationID: number | null;
	phase: NavigationLifecyclePhase;
	ownership: OperationOwnershipState;
};

export type NavigationRuntimeEngineSubmitLaneState = {
	targetUrl: string | null;
	operationID: number | null;
	phase: NavigationRuntimeEngineSubmitPhase;
	ownership: OperationOwnershipState;
};

export type NavigationRuntimeEngineState = {
	lanes: {
		navigate: NavigationRuntimeEngineNavigationLaneState;
		prefetch: Map<string, NavigationRuntimeEngineNavigationLaneState>;
		revalidate: NavigationRuntimeEngineNavigationLaneState;
		submit: Map<number, NavigationRuntimeEngineSubmitLaneState>;
	};
};

export type NavigationRuntimeEngineCommand =
	| {
			type: "fetch";
			lane: NavigationRuntimeEngineNavigationLane;
			targetUrl: string;
			operationID: number | null;
	  }
	| {
			type: "start-client-loaders";
			lane: NavigationRuntimeEngineNavigationLane;
			targetUrl: string;
			operationID: number | null;
	  }
	| {
			type: "preload-assets";
			lane: NavigationRuntimeEngineNavigationLane;
			targetUrl: string;
			operationID: number | null;
	  }
	| {
			type: "commit-snapshot";
			lane: NavigationRuntimeEngineNavigationLane;
			targetUrl: string;
			operationID: number | null;
	  }
	| {
			type: "history-write";
			lane: NavigationRuntimeEngineNavigationLane;
			targetUrl: string;
			operationID: number | null;
	  }
	| {
			type: "redirect";
			lane: NavigationRuntimeEngineNavigationLane;
			targetUrl: string;
			operationID: number | null;
	  }
	| {
			type: "sync-build-id";
			lane: NavigationRuntimeEngineNavigationLane;
			targetUrl: string;
			operationID: number | null;
	  }
	| {
			type: "emit-events";
			eventName: string;
			lane?: NavigationRuntimeEngineLane;
			targetUrl?: string;
			operationID?: number | null;
			detail?: string;
	  }
	| {
			type: "cleanup-entry";
			lane: NavigationRuntimeEngineNavigationLane;
			targetUrl: string;
			operationID: number | null;
			reason: string;
			causedByOperationID?: number | null;
	  }
	| {
			type: "log-error";
			message: string;
			lane?: NavigationRuntimeEngineLane;
			targetUrl?: string;
			operationID?: number | null;
	  }
	| {
			type: "clear-queued-revalidation-request";
	  }
	| {
			type: "commit-same-document-hash-navigation-without-fetch";
			navigationProps: NavigateProps;
			targetUrl: string;
	  }
	| {
			type: "reset-revalidation-lane";
	  }
	| {
			type: "clear-runtime-lanes";
	  };

export type NavigationRuntimeEngineTerminalResult =
	| {
			type: "run-single-pass";
	  }
	| {
			type: "run-revalidation-lane";
	  }
	| {
			type: "return-without-fetch";
			didNavigate: boolean;
	  }
	| {
			type: "none";
	  };

export type NavigationRuntimeEngineTransition = {
	state: NavigationRuntimeEngineState;
	commands: NavigationRuntimeEngineCommand[];
	terminalResult: NavigationRuntimeEngineTerminalResult;
};

export type NavigationRuntimeEngineEvent =
	| {
			type: "start";
			lane: NavigationRuntimeEngineNavigationLane;
			navigationProps: NavigateProps;
			targetUrl: string;
			currentHref: string;
			operationID?: number;
	  }
	| {
			type: "fetch-resolve";
			lane: NavigationRuntimeEngineNavigationLane;
			targetUrl: string;
			operationID: number | undefined;
			outcomeType: NavigationOutcome["type"];
	  }
	| {
			type: "wait-resolve";
			lane: NavigationRuntimeEngineNavigationLane;
			targetUrl: string;
			operationID: number | undefined;
	  }
	| {
			type: "redirect";
			lane: NavigationRuntimeEngineNavigationLane;
			targetUrl: string;
			operationID: number | undefined;
	  }
	| {
			type: "abort";
			lane: NavigationRuntimeEngineNavigationLane;
			targetUrl: string;
			operationID: number | undefined;
			reason: string;
	  }
	| {
			type: "render-finish";
			lane: NavigationRuntimeEngineNavigationLane;
			targetUrl: string;
			operationID: number | undefined;
	  }
	| {
			type: "timeout";
			lane: "submit";
			operationID: number;
			targetUrl: string;
			reason: string;
	  }
	| {
			type: "external-location-change";
			currentHref: string;
	  }
	| {
			type: "remove-navigation-requested";
			targetUrl: string;
			reason: string;
			causedByOperationID?: number | null;
	  }
	| {
			type: "clear-all-requested";
			targetUrl: string;
	  }
	| {
			type: "submission-state-transitioned";
			operationID: number;
			targetUrl: string;
			fromState: string;
			toState: string;
			reason: string;
			causedByOperationID?: number | null;
	  };

export type ExecuteNavigationRuntimeEngineCommandsContext = {
	clearQueuedRevalidationRequest: () => void;
	commitSameDocumentHashNavigationWithoutFetch: (props: {
		navigationProps: NavigateProps;
		targetUrl: string;
	}) => void;
	cleanupNavigationEntry: (props: {
		targetUrl: string;
		reason: string;
		causedByOperationID?: number | null;
	}) => void;
	resetRevalidationLane: () => void;
	clearRuntimeLanes: () => void;
	onPlannedSideEffectCommand?: (props: {
		command: Extract<
			NavigationRuntimeEngineCommand,
			| { type: "fetch" }
			| { type: "start-client-loaders" }
			| { type: "preload-assets" }
			| { type: "commit-snapshot" }
			| { type: "history-write" }
			| { type: "redirect" }
			| { type: "sync-build-id" }
		>;
	}) => void;
	emitEvent?: (props: {
		eventName: string;
		lane?: NavigationRuntimeEngineLane;
		targetUrl?: string;
		operationID?: number | null;
		detail?: string;
	}) => void;
	logError?: (props: {
		message: string;
		lane?: NavigationRuntimeEngineLane;
		targetUrl?: string;
		operationID?: number | null;
	}) => void;
	isNavigationOperationCurrent?: (props: {
		lane: NavigationRuntimeEngineNavigationLane;
		targetUrl: string;
		operationID: number | null;
	}) => boolean;
};

export type SameDocumentNoFetchNavigationDecision =
	| "none"
	| "same-document-noop"
	| "hash-change";

/////////////////////////////////////////////////////////////////////
/////// Submission Runtime
/////////////////////////////////////////////////////////////////////

export type SubmissionLifecycle = {
	abortController: AbortController;
	isCurrent: () => boolean;
	begin: () => void;
	finish: () => void;
};

export type SubmitExecutionContext = {
	submissions: Map<string | symbol, SubmissionEntry>;
	scheduleStatusUpdate: () => void;
	allocateSubmissionOperationID: () => number;
	onSubmissionStateTransition?: (props: {
		submissionEntry: SubmissionEntry;
		fromState: string;
		toState: string;
		reason: string;
		causedByOperationID?: number | null;
	}) => void;
	navigate: (props: NavigateProps) => Promise<{
		didNavigate: boolean;
	}>;
};

export type SubmissionLifecycleTransitionCommand = {
	type: "emit_submission_state_transition";
	submissionEntry: SubmissionEntry;
	fromState: string;
	toState: string;
	reason: string;
	causedByOperationID?: number | null;
};

export type SubmissionLifecycleCommand =
	| {
			type: "abort_submission";
			submissionEntry: SubmissionEntry;
			abortReason: string;
	  }
	| {
			type: "set_submission";
			submissionKey: string | symbol;
			submissionEntry: SubmissionEntry;
	  }
	| {
			type: "delete_submission";
			submissionKey: string | symbol;
	  }
	| SubmissionLifecycleTransitionCommand
	| {
			type: "schedule_status_update";
	  };

export type SubmissionLifecycleCommandPlan = {
	commands: SubmissionLifecycleCommand[];
};

export type SubmitRuntimeCommand =
	| {
			type: "sync_build_id_from_response";
			response: Response;
	  }
	| {
			type: "effectuate_redirect";
			redirectData: RedirectData;
	  }
	| {
			type: "auto_revalidate_navigation";
			href: string;
	  };

export type SubmitRuntimeReducerState = {
	phase:
		| "awaiting_request_resolution"
		| "awaiting_response_classification"
		| "awaiting_success_payload_ownership_checkpoint"
		| "awaiting_post_classification_command_execution"
		| "completed";
	ownership: "current" | "stale";
};

export type SubmitRuntimeReducerEvent<TSubmitResultData> =
	| {
			type: "request_resolved";
			response: Response;
			staleSubmitResult: SubmitResult<TSubmitResultData> | null;
	  }
	| {
			type: "response_classified";
			response: Response;
			redirectData: RedirectData | null;
			requestInit?: RequestInit;
			options?: SubmitOptions;
			currentHref: string;
			staleSubmitResult: SubmitResult<TSubmitResultData> | null;
	  }
	| {
			type: "success_payload_parsed";
			staleSubmitResult: SubmitResult<TSubmitResultData> | null;
	  };

export type SubmitRuntimeReducerTerminalResult<TSubmitResultData> =
	| {
			type: "continue";
	  }
	| {
			type: "stale";
			result: SubmitResult<TSubmitResultData>;
	  }
	| {
			type: "error";
			error: string;
	  };

export type SubmitRuntimeReducerTransition<TSubmitResultData> = {
	state: SubmitRuntimeReducerState;
	commands: SubmitRuntimeCommand[];
	postClassificationCommandPlan:
		| SubmitPostClassificationRuntimeCommandPlan
		| undefined;
	shouldReadSuccessPayload: boolean;
	terminalResult: SubmitRuntimeReducerTerminalResult<TSubmitResultData>;
};

export type SubmitPostClassificationExecutionPlan =
	| {
			type: "redirect";
			redirectData: RedirectData;
	  }
	| {
			type: "error";
			error: string;
	  }
	| {
			type: "success";
			shouldAutoRevalidate: boolean;
	  };

export type SubmitPostClassificationRuntimeCommandPlan =
	| {
			terminalResult: {
				type: "redirect";
			};
			commands: Array<{
				type: "effectuate_redirect";
				redirectData: RedirectData;
			}>;
	  }
	| {
			terminalResult: {
				type: "error";
				error: string;
			};
			commands: [];
	  }
	| {
			terminalResult: {
				type: "success";
			};
			commands: Array<{
				type: "auto_revalidate_navigation";
				href: string;
			}>;
	  };

export type ExecuteSubmitRuntimeCommandWithStaleOwnershipResult<
	TSubmitResultData,
	TCommandResult,
> =
	| {
			type: "stale";
			result: SubmitResult<TSubmitResultData>;
	  }
	| {
			type: "executed";
			result: TCommandResult;
	  };

/////////////////////////////////////////////////////////////////////
/////// Navigation Runtime Composition
/////////////////////////////////////////////////////////////////////

export type CreateNavigationRuntimeOptions = {
	// Called after a navigate/revalidate intent commits successfully.
	onNavigationIntentResolved?: () => void;
};

export type VormaRuntimeContext = {
	history: {
		instance: BrowserHistory | undefined;
		lastKnownLocation: Location | undefined;
		cleanupListener: (() => void) | null;
		latestListenerSequenceIssued: number;
		listenerProcessingTail: Promise<void>;
	};
	navigationStateAccess: NavigationStateAccess | null;
	navigationStateManager: NavigationStateManager | null;
	revalidationTriggerTimestampRuntime: RevalidationTriggerTimestampRuntime;
	runtimeGlobalState: VormaClientGlobal | null;
};

/////////////////////////////////////////////////////////////////////
/////// Global Runtime Snapshot Types
/////////////////////////////////////////////////////////////////////

/**
 * Serialized head element shape transferred between server/runtime boundaries.
 */
export type HeadEl = {
	tag?: string;
	attributesKnownSafe?: Record<string, string>;
	booleanAttributes?: Array<string>;
	dangerousInnerHTML?: string;
};

export type Meta = {
	title: HeadEl | null | undefined;
	metaHeadEls: Array<HeadEl> | null | undefined;
	restHeadEls: Array<HeadEl> | null | undefined;
};

export type RouteDataState = {
	outermostServerError?: string;
	outermostServerErrorIdx?: number;

	matchedPatterns: Array<string>;
	loadersData: Array<unknown>;
	importURLs: Array<string>;
	exportKeys: Array<string>;
	errorExportKeys: string[];
	hasRootData: boolean;

	params: Record<string, string>;
	splatValues: Array<string>;
};

export type RuntimeRouteSnapshot = RouteDataState & {
	outermostClientError?: string;
	outermostClientErrorIdx?: number;
	outermostError?: string;
	outermostErrorIdx?: number;

	buildID: string;
	rootElementID?: string;

	activeComponents: Array<unknown> | null;
	activeErrorBoundary?: unknown;
	clientLoadersData: Array<unknown>;
};

/**
 * Contract for route-level error boundary components.
 */
export type RouteErrorComponent = (props: { error: string }) => unknown;

/**
 * Client loader wait function contract, keyed by route pattern.
 * Wait functions are speculative: the runtime may invoke them before final
 * navigation ownership checks complete, and their result may be discarded if
 * that navigation is superseded. Implementations should treat side effects as
 * idempotent and honor `signal` for cancellation-aware cleanup.
 */
export type PatternWaitFn = (props: {
	params: Record<string, string>;
	splatValues: string[];
	serverDataPromise: Promise<ClientLoaderAwaitedServerData<unknown, unknown>>;
	signal: AbortSignal;
}) => Promise<unknown>;

export type VormaClientGlobalNonSnapshotState = {
	isDev: boolean;
	viteDevURL: string;
	publicPathPrefix: string;
	// Tracks current pointer modality, not hardware capability.
	isTouchInputModalityActive: boolean;
	patternToWaitFnMap: Record<string, PatternWaitFn>;
	defaultErrorBoundary: RouteErrorComponent;
	useViewTransitions: boolean;
	deploymentID: string;
	vormaAppConfig: VormaAppConfig;
	routeManifestURL: string;
	routeManifest: Record<string, number> | undefined;
	patternRegistry: PatternRegistry;
};

export type VormaClientGlobalNonSnapshotKey =
	keyof VormaClientGlobalNonSnapshotState;

export type VormaClientGlobalGetKey =
	| VormaClientGlobalNonSnapshotKey
	| "runtimeRouteSnapshot";

export type VormaClientGlobalSetKey =
	| VormaClientGlobalNonSnapshotKey
	| "runtimeRouteSnapshot";

export type VormaClientGlobalGetValueForKey<K extends VormaClientGlobalGetKey> =
	K extends "runtimeRouteSnapshot"
		? RuntimeRouteSnapshot
		: K extends VormaClientGlobalNonSnapshotKey
			? VormaClientGlobalNonSnapshotState[K]
			: never;

export type VormaClientGlobalSetValueForKey<K extends VormaClientGlobalSetKey> =
	K extends "runtimeRouteSnapshot"
		? RuntimeRouteSnapshot
		: K extends VormaClientGlobalNonSnapshotKey
			? VormaClientGlobalNonSnapshotState[K]
			: never;

/**
 * Global runtime state container mounted on `globalThis[VORMA_SYMBOL]`.
 */
export type VormaClientGlobal = VormaClientGlobalNonSnapshotState & {
	runtimeRouteSnapshot: RuntimeRouteSnapshot;
};

export type VormaGlobalThis = typeof globalThis & {
	[key: symbol]: unknown;
};

export type ClientRuntimeRenderState = Pick<
	RuntimeRouteSnapshot,
	| "loadersData"
	| "clientLoadersData"
	| "outermostError"
	| "outermostErrorIdx"
	| "activeComponents"
	| "activeErrorBoundary"
	| "importURLs"
	| "exportKeys"
>;

/**
 * Minimal navigation runtime surface exposed to modules that should not depend
 * on the full navigation manager implementation.
 */
export type NavigationStateAccess = Pick<
	NavigationStateManager,
	"navigate" | "removeNavigation" | "getNavigations"
> & {
	navigate: (props: NavigateProps) => Promise<{ didNavigate: boolean }>;
};

export type RegisterClientLoaderForAdapterProps = {
	pattern: string;
	waitFn: PatternWaitFn;
	onRegistrationError?: (error: unknown) => void;
};

export type GlobalLoadingIndicatorIncludesOption =
	| "navigations"
	| "submissions"
	| "revalidations";

export type GlobalLoadingIndicatorConfig = {
	start: () => void;
	stop: () => void;
	isRunning: () => boolean;
	include?: "all" | Array<GlobalLoadingIndicatorIncludesOption>;
	startDelayMS?: number;
	stopDelayMS?: number;
};

export type ParsedGlobalLoadingIndicatorConfig = {
	includesAll: boolean;
	includesNavigations: boolean;
	includesSubmissions: boolean;
	includesRevalidations: boolean;
	startDelayMS: number;
	stopDelayMS: number;
};

/////////////////////////////////////////////////////////////////////
/////// Runtime Location And Revalidation Timestamp
/////////////////////////////////////////////////////////////////////

export type RevalidationTriggerTimestampState = {
	lastTriggeredNavOrRevalidateTimestampMS: number;
};

export type RevalidationTriggerTimestampEvent = {
	type: "navigation_or_revalidation_intent_committed";
	committedTimestampMS: number;
};

export type RevalidationTriggerTimestampRuntime = {
	recordNavigationOrRevalidationIntentCommitted: () => void;
	getLastTriggeredNavOrRevalidateTimestampMS: () => number;
};

/////////////////////////////////////////////////////////////////////
/////// URL And API Helpers
/////////////////////////////////////////////////////////////////////

export type Props = {
	pattern: string;
	params?: Record<string, string>;
	splatValues?: Array<string>;
	options?: SubmitOptions;
	requestInit?: RequestInit;
	input?: unknown;
};

export type APIClientHelperOpts = {
	vormaAppConfig: VormaAppConfig;
	type: "loader" | "query" | "mutation";
	props: Props;
};

type RouteResolutionCoreProps<Params extends Record<string, unknown>> = {
	pattern: string;
	params?: Params;
	splatValues?: Array<string>;
};

export type PathResolutionProps = RouteResolutionCoreProps<
	Record<string, unknown>
>;

type PathResolutionConfigCore = Pick<
	VormaAppConfig,
	| "actionsDynamicRune"
	| "actionsSplatRune"
	| "loadersDynamicRune"
	| "loadersSplatRune"
	| "loadersExplicitIndexSegmentIdentifier"
>;

type RouteResolutionInput<Config, RouteProps> = {
	vormaAppConfig: Config;
	type: "loader" | "query" | "mutation";
	props: RouteProps;
};

export type ResolvePathInput = RouteResolutionInput<
	PathResolutionConfigCore,
	PathResolutionProps
>;

export type URLBuildProps = RouteResolutionCoreProps<
	Record<string, unknown>
> & {
	input?: unknown;
};

export type URLBuildConfig = PathResolutionConfigCore &
	Pick<VormaAppConfig, "actionsRouterMountRoot">;

export type URLBuildInput = RouteResolutionInput<URLBuildConfig, URLBuildProps>;

/////////////////////////////////////////////////////////////////////
/////// Link Click Lifecycle
/////////////////////////////////////////////////////////////////////

export type LinkOnClickCallback<LinkEvent = unknown> = (
	event: LinkEvent,
) => void | Promise<void>;

export type LinkOnClickCallbacks<LinkEvent = unknown> = {
	beforeBegin?: LinkOnClickCallback<LinkEvent>;
	beforeRender?: LinkOnClickCallback<LinkEvent>;
	afterRender?: LinkOnClickCallback<LinkEvent>;
};

export type ClickNavigationOptions = {
	scrollToTop?: boolean;
	replace?: boolean;
	state?: unknown;
};

export type EligibleInternalAnchorDetails = Exclude<
	ReturnType<typeof getAnchorDetailsFromEvent>,
	null
>;

export type CreatePrefetchHandlersInput<E extends Event> =
	LinkOnClickCallbacks<E> & {
		href: string;
		delayMs?: number;
		scrollToTop?: boolean;
		replace?: boolean;
		state?: unknown;
	};

export type HandlerKeys = {
	onPointerEnter: string;
	onFocus: string;
	onPointerLeave: string;
	onBlur: string;
	onTouchCancel: string;
	onClick: string;
};

export type UnknownFn = (...args: ReadonlyArray<unknown>) => unknown;

export type TypedNavigateOptions<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
> = PermissivePatternBasedProps<App, Pattern> & {
	replace?: boolean;
	scrollToTop?: boolean;
	search?: string;
	hash?: string;
	state?: unknown;
};

/////////////////////////////////////////////////////////////////////
/////// Client Init Contracts
/////////////////////////////////////////////////////////////////////

export type InitClientOptions = {
	defaultErrorBoundary?: RouteErrorComponent;
	useViewTransitions?: boolean;
};

export type InitClientInput = InitClientOptions & {
	vormaAppConfig: VormaAppConfig;
	renderFn: () => void;
};

export type RouteManifestRecord = NonNullable<
	VormaClientGlobal["routeManifest"]
>;

/////////////////////////////////////////////////////////////////////
/////// Component Module Runtime
/////////////////////////////////////////////////////////////////////

export type ModuleExports = Record<string, unknown>;

export type ComponentModulesMap = Map<string, ModuleExports | undefined>;

/////////////////////////////////////////////////////////////////////
/////// Client Loader Render Runtime
/////////////////////////////////////////////////////////////////////

export type PartialWaitFnJSON = Pick<
	CanonicalRouteDataPayload,
	| "matchedPatterns"
	| "splatValues"
	| "params"
	| "hasRootData"
	| "loadersData"
	| "outermostServerErrorIdx"
	| "importURLs"
>;

export type ClientLoadersResult = {
	data: Array<unknown>;
	errorMessage?: string;
};

export type ClientLoaderWorkItems = {
	loaderPromises: Array<Promise<unknown>>;
	abortControllers: Array<AbortController | null>;
};

export type RenderingHistoryOptions = {
	href: string;
	scrollStateToRestore?: ScrollState;
	replace?: boolean;
	scrollToTop?: boolean;
	state?: unknown;
};

export type RerenderAppProps = {
	json: CanonicalRouteDataPayload;
	navigationType: VormaNavigationType;
	clientLoadersResult?: ClientLoadersResult;
	runHistoryOptions?: RenderingHistoryOptions;
	shouldCommit?: () => boolean;
	onFinish: () => void;
};

/////////////////////////////////////////////////////////////////////
/////// Route Outlet Listener Runtime
/////////////////////////////////////////////////////////////////////

export type RouteOutletRuntimeStoreSyncFunction = () => void;

export type RouteOutletRuntimeListenerInitializer = () => void;

export type RouteOutletBranchState = {
	currentRouteKey: string;
	nextRouteKey: string;
	isErrorIdx: boolean;
	currentComponent: unknown;
	errorComponent: unknown;
	shouldFallbackOutlet: boolean;
};

export type RouteOutletBranchRouteKeys = Pick<
	RouteOutletBranchState,
	"currentRouteKey" | "nextRouteKey"
>;

export type RouteOutletBranchRenderState =
	| (RouteOutletBranchRouteKeys & {
			renderKind: "error";
			errorComponent: unknown;
	  })
	| (RouteOutletBranchRouteKeys & {
			renderKind: "component";
			currentComponent: unknown;
	  })
	| (RouteOutletBranchRouteKeys & {
			renderKind: "fallback";
	  })
	| (RouteOutletBranchRouteKeys & {
			renderKind: "empty";
	  });

export type VormaTypedAdapterRoutePropsWithIndex = {
	idx: number;
	__vorma_internal_route_scope?: unknown;
};

export type TypedAdapterRouteScope = {
	[marker: symbol]: true | undefined;
	boundRoutePropsIndex: number;
	boundMatchedPattern: string;
};

/////////////////////////////////////////////////////////////////////
/////// Route Outlet Adapter Host Runtime
/////////////////////////////////////////////////////////////////////

export type RouteOutletAdapterSyncHost = {
	syncStoreState: () => void;
	initUIListeners: () => void;
	syncRootOutletMount: (props: { idx: number }) => void;
};

type RouteOutletAdapterRenderModelFromBranch<
	BranchRenderState extends RouteOutletBranchRenderState,
> = BranchRenderState extends {
	renderKind: "component";
}
	? BranchRenderState & {
			matchedPattern: string;
			outermostError: unknown;
		}
	: BranchRenderState & {
			outermostError: unknown;
		};

export type RouteOutletAdapterRenderModel =
	RouteOutletAdapterRenderModelFromBranch<RouteOutletBranchRenderState>;

/////////////////////////////////////////////////////////////////////
/////// Typed Adapter Link Route Resolution
/////////////////////////////////////////////////////////////////////

export type TypedLinkRouteResolutionInput = {
	pattern: string;
	params?: Record<string, string>;
	splatValues?: string[];
	search?: string;
	hash?: string;
};

export type TypedLinkMergedPropsBase = TypedLinkRouteResolutionInput & {
	state?: unknown;
};

export type TypedLinkResolvedNonRouteProps<
	MergedProps extends TypedLinkMergedPropsBase,
> = Omit<MergedProps, keyof TypedLinkRouteResolutionInput | "state">;

/////////////////////////////////////////////////////////////////////
/////// Typed Adapter Link Runtime
/////////////////////////////////////////////////////////////////////

export type NavigationInternalLinkPropKey =
	(typeof navigationInternalLinkPropKeysForAnchors)[number];

/////////////////////////////////////////////////////////////////////
/////// vorma/client package runtime (single-file package boundary)
/////////////////////////////////////////////////////////////////////

declare global {
	interface ImportMetaEnv {
		readonly DEV: boolean;
	}

	interface ImportMeta {
		readonly env: ImportMetaEnv;
	}
}

/////////////////////////////////////////////////////////////////////
/////// Route Data Contract Keys
/////////////////////////////////////////////////////////////////////

export const REQUIRED_RUNTIME_ROUTE_DATA_ARRAY_KEYS = [
	"matchedPatterns",
	"loadersData",
	"importURLs",
	"exportKeys",
	"errorExportKeys",
	"splatValues",
] as const;

export const REQUIRED_RUNTIME_ROUTE_DATA_KEYS = [
	...REQUIRED_RUNTIME_ROUTE_DATA_ARRAY_KEYS,
	"hasRootData",
	"params",
] as const;

export const REQUIRED_SERVER_ROUTE_DATA_ARRAY_KEYS = [
	"matchedPatterns",
	"loadersData",
	"importURLs",
	"exportKeys",
] as const;

const OPTIONAL_SERVER_ROUTE_DATA_ARRAY_KEYS = [
	"errorExportKeys",
	"splatValues",
	"deps",
	"cssBundles",
	"clientLoadersData",
	"metaHeadEls",
	"restHeadEls",
] as const;

/////////////////////////////////////////////////////////////////////
/////// Platform Safety
/////////////////////////////////////////////////////////////////////

export function logInfo(message?: unknown, ...optionalParams: Array<unknown>) {
	console.log("Vorma:", message, ...optionalParams);
}

export function logError(message?: unknown, ...optionalParams: Array<unknown>) {
	console.error("Vorma:", message, ...optionalParams);
}

export function isAbortError(error: unknown) {
	return error instanceof Error && error.name === "AbortError";
}

export function panic(msg?: string): never {
	logError("Panic");
	throw new Error(msg ?? "panic");
}

const objectTagByGlobalCtorName: Record<GlobalCtorName, string> = {
	FormData: "[object FormData]",
	URLSearchParams: "[object URLSearchParams]",
	Blob: "[object Blob]",
	ArrayBuffer: "[object ArrayBuffer]",
	ReadableStream: "[object ReadableStream]",
};

function isCrossRealmInstanceOfGlobal(
	value: unknown,
	ctorName: GlobalCtorName,
): boolean {
	if (
		(typeof value !== "object" && typeof value !== "function") ||
		value === null
	) {
		return false;
	}

	const expectedTag = objectTagByGlobalCtorName[ctorName];
	return Object.prototype.toString.call(value) === expectedTag;
}

export function isInstanceOfGlobal(
	value: unknown,
	ctorName: GlobalCtorName,
): boolean {
	const ctor = (globalThis as Record<string, unknown>)[ctorName];
	if (typeof ctor !== "function") {
		return false;
	}

	const typedCtor = ctor as new (...args: Array<never>) => object;
	if (value instanceof typedCtor) {
		return true;
	}

	return isCrossRealmInstanceOfGlobal(value, ctorName);
}

export function isArrayBufferView(value: unknown): value is ArrayBufferView {
	return typeof ArrayBuffer !== "undefined" && ArrayBuffer.isView(value);
}

export function observePromiseRejection<T>(promise: Promise<T>): Promise<T> {
	void promise.catch(() => {});
	return promise;
}

/////////////////////////////////////////////////////////////////////
/////// URL Parsing And Classification
/////////////////////////////////////////////////////////////////////

export const VORMA_HARD_RELOAD_QUERY_PARAM = "vorma_reload";

function stripHashPrefix(hash: string): string {
	return hash.startsWith("#") ? hash.slice(1) : hash;
}

export function hrefWithoutHash(props: {
	href: string;
	baseHref?: string;
}): string {
	const { href, baseHref = window.location.href } = props;
	const url = new URL(href, baseHref);
	url.hash = "";
	return url.href;
}

export function hasSameDataTarget(props: {
	firstHref: string;
	secondHref: string;
	baseHref?: string;
}): boolean {
	const { firstHref, secondHref, baseHref = window.location.href } = props;
	return (
		hrefWithoutHash({ href: firstHref, baseHref }) ===
		hrefWithoutHash({ href: secondHref, baseHref })
	);
}

export function findMapEntryByNavigationTarget<T>(props: {
	map: ReadonlyMap<string, T>;
	targetHref: string;
	baseHref?: string;
}): [string, T] | undefined {
	const { map, targetHref, baseHref = window.location.href } = props;
	if (map.has(targetHref)) {
		return [targetHref, map.get(targetHref)!];
	}

	for (const [key, value] of map.entries()) {
		if (
			hasSameDataTarget({
				firstHref: key,
				secondHref: targetHref,
				baseHref,
			})
		) {
			return [key, value];
		}
	}

	return undefined;
}

export function decodeHashFragment(hashFragment: string): string {
	try {
		return decodeURIComponent(hashFragment);
	} catch {
		return hashFragment;
	}
}

export function normalizedHashFragmentFromHash(hash: string): string {
	return decodeHashFragment(hashFragmentFromHash(hash));
}

export function hashFragmentFromHash(hash: string): string {
	return stripHashPrefix(hash);
}

export function normalizedHashFragmentFromHref(href: string): string {
	return decodeHashFragment(hashFragmentFromHref(href));
}

export function hashFragmentFromHref(href: string): string {
	const url = new URL(href, window.location.href);
	return hashFragmentFromHash(url.hash);
}

export function classifyNavigationTargetAgainstCurrentLocation(props: {
	targetHref: string;
	currentHref?: string;
}): NavigationTargetClassificationAgainstCurrentLocation {
	const { targetHref, currentHref = window.location.href } = props;
	const sharesDataTarget = hasSameDataTarget({
		firstHref: targetHref,
		secondHref: currentHref,
		baseHref: currentHref,
	});
	if (!sharesDataTarget) {
		return "navigate";
	}

	const targetHash = normalizedHashFragmentFromHref(targetHref);
	const currentHash = normalizedHashFragmentFromHref(currentHref);
	if (targetHash === currentHash) {
		return "same-document-noop";
	}

	return "hash-change";
}

function isAbsoluteOrProtocolRelativeHref(href: string): boolean {
	return href.startsWith("//") || /^[a-zA-Z][a-zA-Z\d+\-.]*:/.test(href);
}

export function resolvePublicHref(relativeHref: string): string {
	if (isAbsoluteOrProtocolRelativeHref(relativeHref)) {
		return relativeHref;
	}

	let baseURL = __vormaClientGlobal.get("viteDevURL");
	if (!baseURL) {
		baseURL = __vormaClientGlobal.get("publicPathPrefix");
	}
	if (baseURL.endsWith("/")) {
		baseURL = baseURL.slice(0, -1);
	}
	const final = relativeHref.startsWith("/")
		? baseURL + relativeHref
		: baseURL + "/" + relativeHref;
	return final;
}

export function assertProgrammaticSameOriginOrThrow(props: {
	absoluteHref: string;
	apiName: string;
	currentHref?: string;
}): void {
	const { absoluteHref, apiName, currentHref = window.location.href } = props;
	let targetURL: URL;
	let currentURL: URL;
	try {
		targetURL = new URL(absoluteHref, currentHref);
		currentURL = new URL(currentHref);
	} catch {
		throw new Error(
			`${apiName} received an invalid URL target: ${JSON.stringify(absoluteHref)}`,
		);
	}

	if (targetURL.origin === currentURL.origin) {
		return;
	}

	throw new Error(
		`${apiName} only supports same-origin targets. Received ${JSON.stringify(targetURL.origin)} while current origin is ${JSON.stringify(currentURL.origin)}.`,
	);
}

/////////////////////////////////////////////////////////////////////
/////// History And Scroll State
/////////////////////////////////////////////////////////////////////

function getRuntimeHistoryState(): VormaRuntimeContext["history"] {
	return getDefaultVormaRuntimeContext().history;
}

function issueHistoryListenerSequence(): number {
	const historyState = getRuntimeHistoryState();
	historyState.latestListenerSequenceIssued += 1;
	return historyState.latestListenerSequenceIssued;
}

function isLatestHistoryListenerSequence(sequence: number): boolean {
	return sequence === getRuntimeHistoryState().latestListenerSequenceIssued;
}

function getHistoryInstance(): BrowserHistory {
	const historyState = getRuntimeHistoryState();
	if (!historyState.instance) {
		historyState.instance = createBrowserHistory();
		historyState.lastKnownLocation = historyState.instance.location;
	}
	return historyState.instance;
}

function getLastKnownHistoryLocation(): Location {
	const historyState = getRuntimeHistoryState();
	if (!historyState.lastKnownLocation) {
		historyState.lastKnownLocation = getHistoryInstance().location;
	}
	return historyState.lastKnownLocation;
}

function setLastKnownHistoryLocation(location: Location): void {
	getRuntimeHistoryState().lastKnownLocation = location;
}

export function analyzeHistoryListenerPrelude(props: {
	action: Action;
	location: HistoryLocationPrelude;
	lastKnownLocation: HistoryLocationPrelude;
}): {
	didLocationKeyChange: boolean;
	popWithinSameDoc: boolean;
	shouldSaveScrollState: boolean;
} {
	const { action, location, lastKnownLocation } = props;
	const didLocationKeyChange = location.key !== lastKnownLocation.key;
	const popWithinSameDoc =
		action === "POP" &&
		hasSameDataTarget({
			firstHref: resolveAbsoluteHref({
				href: `${location.pathname}${location.search}`,
				baseHref: window.location.origin,
			}),
			secondHref: resolveAbsoluteHref({
				href: `${lastKnownLocation.pathname}${lastKnownLocation.search}`,
				baseHref: window.location.origin,
			}),
		});

	return {
		didLocationKeyChange,
		popWithinSameDoc,
		shouldSaveScrollState: !popWithinSameDoc,
	};
}

function applyHashDrivenPopScrollState(props: {
	location: Location;
	lastKnownLocation: Location;
	popWithinSameDoc: boolean;
}): void {
	const { location, lastKnownLocation, popWithinSameDoc } = props;
	const locationHashTarget = normalizedHashFragmentFromHash(location.hash);
	const lastKnownHashTarget = normalizedHashFragmentFromHash(
		lastKnownLocation.hash,
	);
	const hasLocationHashTarget = locationHashTarget !== "";
	const hasLastKnownHashTarget = lastKnownHashTarget !== "";
	const hashTargetChanged = locationHashTarget !== lastKnownHashTarget;
	const removingHash =
		popWithinSameDoc && hasLastKnownHashTarget && !hasLocationHashTarget;
	const addingHash =
		popWithinSameDoc && !hasLastKnownHashTarget && hasLocationHashTarget;
	const updatingHash =
		popWithinSameDoc && hasLocationHashTarget && hashTargetChanged;

	if (addingHash || updatingHash) {
		applyScrollState({
			hash: hashFragmentFromHash(location.hash),
		});
	}

	if (removingHash) {
		const stored = scrollStateManager.getState(location.key);
		applyScrollState(stored ?? { x: 0, y: 0 });
	}
}

function attemptHardReloadAfterFailedPopNavigation(): void {
	if (/jsdom/i.test(navigator.userAgent)) {
		return;
	}

	try {
		window.location.reload();
	} catch (error) {
		logError(
			"Browser POP hard reload failed after client navigation fallback.",
			error,
		);
	}
}

async function navigateCrossDocumentPop(location: Location): Promise<boolean> {
	const result = await getNavigationStateAccess().navigate({
		href: resolveAbsoluteHref({
			href: `${location.pathname}${location.search}${location.hash}`,
			baseHref: window.location.origin,
		}),
		navigationType: "browserHistory",
		scrollStateToRestore: scrollStateManager.getState(location.key),
	});

	if (result.didNavigate) {
		return true;
	}

	logError(
		"Browser POP navigation failed, attempting hard reload of the destination.",
	);

	attemptHardReloadAfterFailedPopNavigation();
	return false;
}

async function handlePopNavigationForHistoryUpdate(props: {
	location: Location;
	lastKnownLocation: Location;
	popWithinSameDoc: boolean;
}): Promise<boolean> {
	const { location, lastKnownLocation, popWithinSameDoc } = props;
	applyHashDrivenPopScrollState({
		location,
		lastKnownLocation,
		popWithinSameDoc,
	});

	if (popWithinSameDoc) {
		return true;
	}

	return navigateCrossDocumentPop(location);
}

function setManualScrollRestoration(): void {
	if (history.scrollRestoration && history.scrollRestoration !== "manual") {
		history.scrollRestoration = "manual";
	}
}

function initHistory(): void {
	const historyState = getRuntimeHistoryState();
	const instance = getHistoryInstance();
	historyState.cleanupListener?.();
	historyState.cleanupListener = instance.listen((update) => {
		void customHistoryListener(update);
	});
	setManualScrollRestoration();
}

export const HistoryManager = {
	getInstance: getHistoryInstance,
	getLastKnownLocation: getLastKnownHistoryLocation,
	updateLastKnownLocation: setLastKnownHistoryLocation,
	init: initHistory,
};

async function processHistoryUpdate({
	action,
	location,
}: Update): Promise<void> {
	const listenerSequence = issueHistoryListenerSequence();
	const lastKnownLocation = getLastKnownHistoryLocation();
	const { didLocationKeyChange, popWithinSameDoc, shouldSaveScrollState } =
		analyzeHistoryListenerPrelude({
			action,
			location,
			lastKnownLocation,
		});

	if (didLocationKeyChange) {
		dispatchLocationEvent();
	}

	if (shouldSaveScrollState) {
		saveScrollState();
	}

	let navigationSucceeded = true;
	if (action === "POP") {
		navigationSucceeded = await handlePopNavigationForHistoryUpdate({
			location,
			lastKnownLocation,
			popWithinSameDoc,
		});
	}

	if (
		navigationSucceeded &&
		isLatestHistoryListenerSequence(listenerSequence)
	) {
		setLastKnownHistoryLocation(location);
	}
}

export function customHistoryListener(update: Update): Promise<void> {
	const historyState = getRuntimeHistoryState();
	const queuedHistoryUpdate = historyState.listenerProcessingTail.then(
		() => processHistoryUpdate(update),
		() => processHistoryUpdate(update),
	);
	historyState.listenerProcessingTail = queuedHistoryUpdate.then(
		() => undefined,
		() => undefined,
	);
	return queuedHistoryUpdate;
}

const STORAGE_KEY = "__vorma__scrollStateMap";
const MAX_ENTRIES = 50;
const PAGE_REFRESH_KEY = "__vorma__pageRefreshScrollState";

function safeSessionStorageGetItem(key: string): string | null {
	try {
		return sessionStorage.getItem(key);
	} catch {
		return null;
	}
}

function safeSessionStorageSetItem(key: string, value: string): void {
	try {
		sessionStorage.setItem(key, value);
	} catch {
		// Ignore sessionStorage write failures to keep navigation functional.
	}
}

function safeSessionStorageRemoveItem(key: string): void {
	try {
		sessionStorage.removeItem(key);
	} catch {
		// Ignore sessionStorage remove failures to keep navigation functional.
	}
}

function getStoredScrollStateMap(): Map<string, ScrollState> {
	const stored = safeSessionStorageGetItem(STORAGE_KEY);
	if (!stored) return new Map();

	try {
		return new Map(JSON.parse(stored));
	} catch {
		safeSessionStorageRemoveItem(STORAGE_KEY);
		return new Map();
	}
}

function setStoredScrollStateMap(map: Map<string, ScrollState>): void {
	safeSessionStorageSetItem(
		STORAGE_KEY,
		JSON.stringify(Array.from(map.entries())),
	);
}

export function saveStoredScrollState(key: string, state: ScrollState): void {
	const map = getStoredScrollStateMap();
	map.set(key, state);

	if (map.size > MAX_ENTRIES) {
		const firstKey = map.keys().next().value;
		if (firstKey !== undefined) map.delete(firstKey);
	}

	setStoredScrollStateMap(map);
}

export function getStoredScrollState(key: string): ScrollState | undefined {
	return getStoredScrollStateMap().get(key);
}

function isValidSnapshot(value: unknown): value is PageRefreshSnapshot {
	if (!value || typeof value !== "object") return false;
	const snapshot = value as Partial<PageRefreshSnapshot>;
	return (
		typeof snapshot.href === "string" &&
		typeof snapshot.unix === "number" &&
		Number.isFinite(snapshot.unix) &&
		typeof snapshot.x === "number" &&
		Number.isFinite(snapshot.x) &&
		typeof snapshot.y === "number" &&
		Number.isFinite(snapshot.y)
	);
}

function savePageRefreshScrollStateSnapshot(): void {
	const state = {
		x: window.scrollX,
		y: window.scrollY,
		unix: Date.now(),
		href: window.location.href,
	};
	safeSessionStorageSetItem(PAGE_REFRESH_KEY, JSON.stringify(state));
}

export function restoreRecentPageRefreshScrollState(
	applyState: (props: { x: number; y: number }) => void,
): void {
	const stored = safeSessionStorageGetItem(PAGE_REFRESH_KEY);
	if (!stored) return;

	try {
		const state = JSON.parse(stored);
		if (!isValidSnapshot(state)) {
			safeSessionStorageRemoveItem(PAGE_REFRESH_KEY);
			return;
		}

		const isRecentSnapshot = Date.now() - state.unix < 5000;
		const isCurrentLocation =
			classifyNavigationTargetAgainstCurrentLocation({
				targetHref: state.href,
				currentHref: window.location.href,
			}) === "same-document-noop";
		if (!isCurrentLocation || !isRecentSnapshot) {
			safeSessionStorageRemoveItem(PAGE_REFRESH_KEY);
			return;
		}

		safeSessionStorageRemoveItem(PAGE_REFRESH_KEY);
		window.requestAnimationFrame(() => {
			applyState({ x: state.x, y: state.y });
		});
	} catch {
		safeSessionStorageRemoveItem(PAGE_REFRESH_KEY);
	}
}

export const scrollStateManager = {
	saveState: saveStoredScrollState,
	getState: getStoredScrollState,
	savePageRefreshState: savePageRefreshScrollStateSnapshot,
	restorePageRefreshState: () => {
		restoreRecentPageRefreshScrollState(({ x, y }) => {
			applyScrollState({ x, y });
		});
	},
};

export function applyScrollState(state?: ScrollState): void {
	if (!state) {
		const id = normalizedHashFragmentFromHash(window.location.hash);
		if (id) {
			document.getElementById(id)?.scrollIntoView();
		}
		return;
	}

	if ("hash" in state) {
		const hash = normalizedHashFragmentFromHash(state.hash);
		if (hash) {
			document.getElementById(hash)?.scrollIntoView();
		}
	} else {
		window.scrollTo(state.x, state.y);
	}
}

export function saveScrollState(): void {
	const lastKnownLocation = HistoryManager.getLastKnownLocation();
	scrollStateManager.saveState(lastKnownLocation.key, {
		x: window.scrollX,
		y: window.scrollY,
	});
}

/////////////////////////////////////////////////////////////////////
/////// Runtime Events
/////////////////////////////////////////////////////////////////////

// Route Change Event
export const VORMA_ROUTE_CHANGE_EVENT_KEY = "vorma:route-change";
export const addRouteChangeListener = makeListenerAdder<RouteChangeEventDetail>(
	VORMA_ROUTE_CHANGE_EVENT_KEY,
);
export function dispatchRouteChangeEvent(detail: RouteChangeEventDetail): void {
	dispatchCustomEvent({
		detail,
		eventKey: VORMA_ROUTE_CHANGE_EVENT_KEY,
	});
}

// Status Event
const STATUS_EVENT_KEY = "vorma:status";
export function dispatchStatusEvent(detail: StatusEventDetail): void {
	dispatchCustomEvent({
		detail,
		eventKey: STATUS_EVENT_KEY,
	});
}
export const addStatusListener =
	makeListenerAdder<StatusEventDetail>(STATUS_EVENT_KEY);

// Build ID Event
const BUILD_ID_EVENT_KEY = "vorma:build-id";
export function dispatchBuildIDEvent(detail: BuildIDEventDetail): void {
	dispatchCustomEvent({
		detail,
		eventKey: BUILD_ID_EVENT_KEY,
	});
}
export const addBuildIDListener =
	makeListenerAdder<BuildIDEventDetail>(BUILD_ID_EVENT_KEY);

// Location Event
const LOCATION_EVENT_KEY = "vorma:location";
export function dispatchLocationEvent(): void {
	dispatchCustomEvent({ eventKey: LOCATION_EVENT_KEY });
}
export const addLocationListener = makeListenerAdder<void>(LOCATION_EVENT_KEY);

function getWindowEventTargetOrNull(): Window | null {
	return typeof window !== "undefined" &&
		typeof window.dispatchEvent === "function" &&
		typeof window.addEventListener === "function" &&
		typeof window.removeEventListener === "function"
		? window
		: null;
}

// Helper to create listener adders
function dispatchCustomEvent<T>(props: { eventKey: string; detail?: T }): void {
	const eventTarget = getWindowEventTargetOrNull();
	if (!eventTarget) {
		return;
	}

	if (props.detail === undefined) {
		eventTarget.dispatchEvent(new CustomEvent(props.eventKey));
		return;
	}

	eventTarget.dispatchEvent(
		new CustomEvent(props.eventKey, { detail: props.detail }),
	);
}

function makeListenerAdder<T>(key: string) {
	return function addListener(
		listener: (event: CustomEvent<T>) => void,
	): () => void {
		const eventTarget = getWindowEventTargetOrNull();
		if (!eventTarget) {
			return () => {};
		}

		const wrappedListener: EventListener = (event) => {
			listener(event as CustomEvent<T>);
		};
		eventTarget.addEventListener(key, wrappedListener);
		return () => eventTarget.removeEventListener(key, wrappedListener);
	};
}

/////////////////////////////////////////////////////////////////////
/////// Request Body Transport
/////////////////////////////////////////////////////////////////////

function normalizeArrayBufferViewBody(
	arrayBufferView: ArrayBufferView<ArrayBufferLike>,
): BodyInit {
	if (
		typeof ArrayBuffer !== "undefined" &&
		arrayBufferView.buffer instanceof ArrayBuffer
	) {
		return arrayBufferView as ArrayBufferView<ArrayBuffer>;
	}

	// SharedArrayBuffer-backed views are cloned into an ArrayBuffer-backed
	// Uint8Array so they remain valid body payloads under strict DOM typings.
	const clonedArrayBufferView = new Uint8Array(arrayBufferView.byteLength);
	clonedArrayBufferView.set(
		new Uint8Array(
			arrayBufferView.buffer,
			arrayBufferView.byteOffset,
			arrayBufferView.byteLength,
		),
	);

	return clonedArrayBufferView;
}

function isBodyTransportValueWithoutJSONSerialization(
	input: unknown,
): input is BodyInit | null | undefined {
	return (
		input == null ||
		typeof input === "string" ||
		isInstanceOfGlobal(input, "Blob") ||
		isInstanceOfGlobal(input, "FormData") ||
		isInstanceOfGlobal(input, "URLSearchParams") ||
		isInstanceOfGlobal(input, "ReadableStream") ||
		isInstanceOfGlobal(input, "ArrayBuffer")
	);
}

export function resolveRequestBodyForTransport(props: {
	input: unknown;
}): RequestBodyTransportResolution {
	const { input } = props;

	if (isArrayBufferView(input)) {
		return {
			body: normalizeArrayBufferViewBody(input),
			didSerializeJSON: false,
		};
	}

	if (isBodyTransportValueWithoutJSONSerialization(input)) {
		return {
			body: input,
			didSerializeJSON: false,
		};
	}

	return {
		body: JSON.stringify(input),
		didSerializeJSON: true,
	};
}

/////////////////////////////////////////////////////////////////////
/////// Redirect Handling
/////////////////////////////////////////////////////////////////////

function resolveHTTPRedirectTarget(props: {
	href: string;
	source: string;
	baseHref?: string;
}): {
	hrefDetails: Extract<HrefDetails, { isHTTP: true }>;
} {
	const { href, source, baseHref } = props;
	let absoluteHref: string;
	try {
		absoluteHref = resolveAbsoluteHref({ href, baseHref });
	} catch {
		throw new Error(
			`${source} has invalid redirect target ${JSON.stringify(href)}`,
		);
	}

	const hrefDetails = getHrefDetails(absoluteHref);
	if (!hrefDetails.isHTTP) {
		throw new Error(
			`${source} redirect target ${JSON.stringify(href)} must be an HTTP(S) URL`,
		);
	}
	return { hrefDetails };
}

function getRedirectStrategy(
	hrefDetails: HTTPHrefDetails,
): ShouldRedirectData["shouldRedirectStrategy"] {
	return hrefDetails.isInternal ? "soft" : "hard";
}

function buildShouldRedirectData(props: {
	href: string;
	hrefDetails: HTTPHrefDetails;
	latestBuildID: string;
	shouldRedirectStrategy?: ShouldRedirectData["shouldRedirectStrategy"];
}): ShouldRedirectData {
	return {
		hrefDetails: props.hrefDetails,
		status: "should",
		href: props.href,
		shouldRedirectStrategy:
			props.shouldRedirectStrategy ??
			getRedirectStrategy(props.hrefDetails),
		latestBuildID: props.latestBuildID,
	};
}

function buildShouldRedirectFromHref(props: {
	href: string;
	latestBuildID: string;
	shouldRedirectStrategy?: ShouldRedirectData["shouldRedirectStrategy"];
	normalizeToAbsoluteHref?: boolean;
	baseHref?: string;
	source: string;
}): ShouldRedirectData {
	const resolvedTarget = resolveHTTPRedirectTarget({
		href: props.href,
		baseHref: props.baseHref,
		source: props.source,
	});

	const href = props.normalizeToAbsoluteHref
		? resolvedTarget.hrefDetails.absoluteURL
		: props.href;

	return buildShouldRedirectData({
		href,
		hrefDetails: resolvedTarget.hrefDetails,
		latestBuildID: props.latestBuildID,
		shouldRedirectStrategy: props.shouldRedirectStrategy,
	});
}

function parseBrowserRedirect(
	response: Response,
	latestBuildID: string,
): RedirectData | null {
	if (!response.redirected) {
		return null;
	}

	const shouldRedirectData = buildShouldRedirectFromHref({
		href: response.url,
		latestBuildID,
		normalizeToAbsoluteHref: true,
		source: "redirected fetch response URL",
	});
	const isCurrent =
		classifyNavigationTargetAgainstCurrentLocation({
			targetHref: shouldRedirectData.href,
			currentHref: window.location.href,
		}) === "same-document-noop";
	if (isCurrent) {
		return {
			hrefDetails: shouldRedirectData.hrefDetails,
			status: "did",
			href: shouldRedirectData.href,
		};
	}
	return shouldRedirectData;
}

function parseHeaderRedirect(props: {
	response: Response;
	headerName: "X-Vorma-Reload" | "X-Client-Redirect";
	latestBuildID: string;
	baseHref: string;
	shouldRedirectStrategy?: ShouldRedirectData["shouldRedirectStrategy"];
	normalizeToAbsoluteHref?: boolean;
	shortCircuitCurrentLocation?: boolean;
}): RedirectData | null {
	const headerValue = props.response.headers.get(props.headerName);
	if (!headerValue) {
		return null;
	}

	const shouldRedirectData = buildShouldRedirectFromHref({
		href: headerValue,
		latestBuildID: props.latestBuildID,
		baseHref: props.baseHref,
		shouldRedirectStrategy: props.shouldRedirectStrategy,
		normalizeToAbsoluteHref: props.normalizeToAbsoluteHref,
		source: props.headerName,
	});
	if (!props.shortCircuitCurrentLocation) {
		return shouldRedirectData;
	}

	const isCurrent =
		classifyNavigationTargetAgainstCurrentLocation({
			targetHref: shouldRedirectData.href,
			currentHref: window.location.href,
		}) === "same-document-noop";
	if (isCurrent) {
		return toDidRedirectData(shouldRedirectData);
	}

	return shouldRedirectData;
}

function canIncludeBodyForMethod(method: string | undefined): boolean {
	const normalizedMethod = method?.toUpperCase();
	if (!normalizedMethod) {
		return false;
	}
	return normalizedMethod !== "GET" && normalizedMethod !== "HEAD";
}

/**
 * Builds request init for redirect-aware fetches.
 * It preserves caller options, injects client-redirect acceptance,
 * and JSON-serializes plain object bodies.
 */
export function buildRedirectRequestInit(
	requestInit: RequestInit | undefined,
	signal: AbortSignal,
): RequestInit {
	const {
		body: rawBody,
		headers: rawHeaders,
		signal: _requestSignal,
		...rest
	} = requestInit ?? {};

	const shouldAttachBody =
		rawBody !== undefined && canIncludeBodyForMethod(requestInit?.method);
	let shouldSetJSONContentType = false;
	const bodyParentObj: Pick<RequestInit, "body"> = {};
	if (shouldAttachBody) {
		const requestBodyResolution = resolveRequestBodyForTransport({
			input: rawBody,
		});
		bodyParentObj.body = requestBodyResolution.body;
		shouldSetJSONContentType = requestBodyResolution.didSerializeJSON;
	}

	const headers = new Headers(rawHeaders);
	if (shouldSetJSONContentType && !headers.has("content-type")) {
		headers.set("Content-Type", "application/json");
	}
	// To temporarily test traditional server redirect behavior,
	// you can set this to "0" instead of "1"
	headers.set("X-Accepts-Client-Redirect", "1");

	return {
		...rest,
		headers,
		signal,
		...bodyParentObj,
	};
}

async function executeRedirectRequestFlow(
	input: RedirectRequestFlowInput,
): Promise<RedirectRequestFlowResult> {
	const MAX_REDIRECTS = 10;
	const redirectCount = input.redirectCount || 0;

	if (redirectCount >= MAX_REDIRECTS) {
		logError("Too many redirects");
		return { kind: "too_many_redirects" };
	}

	const requestInit = buildRedirectRequestInit(
		input.requestInit,
		input.abortController.signal,
	);
	const response = await fetch(input.url, requestInit);

	return {
		kind: "ok",
		response,
	};
}

function toDidRedirectData(redirectData: ShouldRedirectData): RedirectData {
	return {
		hrefDetails: redirectData.hrefDetails,
		status: "did",
		href: redirectData.href,
	};
}

// cleanupRedirectRelatedNavigations aborts and removes redirect/revalidation lanes
// before applying a new redirect decision.
function cleanupRedirectRelatedNavigations(
	navigationState: RedirectNavigationState,
): void {
	const navEntries = navigationState.getNavigations().entries();
	for (const [targetUrl, nav] of navEntries) {
		if (nav.type === "redirect" || nav.type === "revalidation") {
			nav.control.abortController?.abort();
			navigationState.removeNavigation(targetUrl);
		}
	}
}

// For internal targets we force a hard reload query marker so the server can
// correlate reload intent against the latest client-known build.
function effectuateHardRedirect(
	redirectData: ShouldRedirectData,
): RedirectData | null {
	if (!redirectData.hrefDetails.isHTTP) {
		return null;
	}

	const absoluteTargetHref = redirectData.hrefDetails.absoluteURL;
	if (redirectData.hrefDetails.isExternal) {
		window.location.href = absoluteTargetHref;
	} else {
		const url = new URL(absoluteTargetHref);
		url.searchParams.set(
			VORMA_HARD_RELOAD_QUERY_PARAM,
			redirectData.latestBuildID,
		);
		window.location.href = url.href;
	}

	return toDidRedirectData(redirectData);
}

async function effectuateSoftRedirect(
	navigationState: RedirectNavigationState,
	redirectData: ShouldRedirectData,
	redirectCount: number,
	originalProps?: NavigateProps,
): Promise<RedirectData | null> {
	const navigationResult = await navigationState.navigate({
		href: redirectData.href,
		navigationType: "redirect",
		redirectCount: redirectCount + 1,
		state: originalProps?.state,
		replace: originalProps?.replace,
		scrollToTop: originalProps?.scrollToTop,
	});
	if (!navigationResult.didNavigate) {
		return null;
	}

	return toDidRedirectData(redirectData);
}

// getBuildIDFromResponse extracts the build id header used for client/runtime sync.
export function getBuildIDFromResponse(response: Response | undefined): string {
	return response?.headers.get("X-Vorma-Build-Id") || "";
}

export function syncRuntimeBuildIDIfChanged(props: {
	nextBuildID: string;
}): void {
	const oldID = getRuntimeRouteSnapshot().buildID;
	const newID = props.nextBuildID;
	if (!newID || newID === oldID) {
		return;
	}

	setRuntimeBuildID({
		buildID: newID,
	});
	dispatchBuildIDEvent({ newID, oldID });
}

// effectuateRedirectDataResult applies redirect instructions and returns the final redirect status.
export async function effectuateRedirectDataResult(
	redirectData: RedirectData,
	redirectCount: number,
	originalProps?: NavigateProps,
): Promise<RedirectData | null> {
	if (redirectData.status !== "should") {
		return null;
	}

	const navigationState = getNavigationStateAccess();
	cleanupRedirectRelatedNavigations(navigationState);

	if (redirectData.shouldRedirectStrategy === "hard") {
		return effectuateHardRedirect(redirectData);
	}

	return effectuateSoftRedirect(
		navigationState,
		redirectData,
		redirectCount,
		originalProps,
	);
}

/////////////////////////////////////////////////////////////////////
/////// Server Route Data Fetch
/////////////////////////////////////////////////////////////////////

// handleRedirects executes fetch-with-redirect-detection and returns parsed redirect metadata.
export async function handleRedirects(props: {
	abortController: AbortController;
	url: URL;
	requestInit?: RequestInit;
	redirectCount?: number;
}): Promise<{ redirectData: RedirectData | null; response?: Response }> {
	const requestFlow = await executeRedirectRequestFlow(props);
	if (requestFlow.kind === "too_many_redirects") {
		return { redirectData: null, response: undefined };
	}

	const latestBuildID = getBuildIDFromResponse(requestFlow.response);
	const redirectBaseHref =
		requestFlow.response.url.trim().length > 0
			? requestFlow.response.url
			: props.url.href;
	const redirectData =
		parseHeaderRedirect({
			response: requestFlow.response,
			headerName: "X-Vorma-Reload",
			latestBuildID,
			baseHref: redirectBaseHref,
			shouldRedirectStrategy: "hard",
		}) ??
		parseBrowserRedirect(requestFlow.response, latestBuildID) ??
		parseHeaderRedirect({
			response: requestFlow.response,
			headerName: "X-Client-Redirect",
			latestBuildID,
			baseHref: redirectBaseHref,
			normalizeToAbsoluteHref: true,
			shortCircuitCurrentLocation: true,
		});
	return { redirectData, response: requestFlow.response };
}

function isNonArrayObject(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function assertRouteDataTopLevelContractOrThrow(props: {
	jsonObject: Record<string, unknown>;
}): void {
	for (const key of REQUIRED_SERVER_ROUTE_DATA_ARRAY_KEYS) {
		if (!(key in props.jsonObject)) {
			throw new Error(
				`Route data JSON contract violated: missing required key "${key}".`,
			);
		}
		if (!Array.isArray(props.jsonObject[key])) {
			throw new Error(
				`Route data JSON contract violated: "${key}" must be an array.`,
			);
		}
	}

	for (const key of OPTIONAL_SERVER_ROUTE_DATA_ARRAY_KEYS) {
		const value = props.jsonObject[key];
		if (value !== undefined && value !== null && !Array.isArray(value)) {
			throw new Error(
				`Route data JSON contract violated: "${key}" must be an array when present.`,
			);
		}
	}

	if (
		props.jsonObject.hasRootData !== undefined &&
		typeof props.jsonObject.hasRootData !== "boolean"
	) {
		throw new Error(
			'Route data JSON contract violated: "hasRootData" must be a boolean.',
		);
	}
	if (
		props.jsonObject.params !== undefined &&
		!isNonArrayObject(props.jsonObject.params)
	) {
		throw new Error(
			'Route data JSON contract violated: "params" must be an object.',
		);
	}
}

function deepFreezeRecursively<Value>(props: { value: Value }): Value {
	const seenValues = new WeakSet<object>();
	const freezeValue = (value: unknown): void => {
		if (typeof value !== "object" || value === null) {
			return;
		}
		const objectValue = value as object;
		if (seenValues.has(objectValue)) {
			return;
		}
		seenValues.add(objectValue);
		const childValues = Array.isArray(value)
			? value
			: Object.values(value as Record<string, unknown>);
		for (const childValue of childValues) {
			freezeValue(childValue);
		}
		Object.freeze(objectValue);
	};
	freezeValue(props.value);
	return props.value;
}

export function decodeServerRouteDataJSONOrThrow(props: {
	json: unknown;
}): CanonicalRouteDataPayload {
	const { json } = props;
	if (!isNonArrayObject(json)) {
		throw new Error(
			"Route data JSON contract violated: route-data payload must be an object.",
		);
	}
	assertRouteDataTopLevelContractOrThrow({
		jsonObject: json,
	});
	const canonicalJSON: CanonicalRouteDataPayload = {
		...(json as Partial<CanonicalRouteDataPayload>),
		matchedPatterns: (json.matchedPatterns as string[]).slice(),
		loadersData: (json.loadersData as unknown[]).slice(),
		importURLs: (json.importURLs as string[]).slice(),
		exportKeys: (json.exportKeys as string[]).slice(),
		errorExportKeys: (
			(json.errorExportKeys as string[] | undefined) ?? []
		).slice(),
		splatValues: ((json.splatValues as string[] | undefined) ?? []).slice(),
		deps: ((json.deps as string[] | undefined) ?? []).slice(),
		cssBundles: ((json.cssBundles as string[] | undefined) ?? []).slice(),
		title: (json.title as HeadEl | null | undefined) ?? undefined,
		metaHeadEls:
			(json.metaHeadEls as Array<HeadEl> | null | undefined) ?? undefined,
		restHeadEls:
			(json.restHeadEls as Array<HeadEl> | null | undefined) ?? undefined,
		hasRootData: (json.hasRootData as boolean | undefined) ?? false,
		params: (json.params as Record<string, string> | undefined) ?? {},
	};
	return deepFreezeRecursively({
		value: canonicalJSON,
	});
}

function getMatchedPatternOrThrow(props: {
	match: RouteMatch | undefined;
	index: number;
	context: string;
}): string {
	const { match, index, context } = props;
	if (!match) {
		throw new Error(
			`${context} returned a sparse matches array at index ${index}.`,
		);
	}

	const pattern = match.registeredPattern.originalPattern;
	if (!pattern) {
		throw new Error(
			`${context} returned an empty route pattern at index ${index}.`,
		);
	}

	return pattern;
}

export function getMatchedPatternsOrThrow(props: {
	matches: Array<RouteMatch | undefined>;
	context: string;
}): string[] {
	const { matches, context } = props;
	const matchedPatterns: string[] = [];
	for (let i = 0; i < matches.length; i++) {
		matchedPatterns.push(
			getMatchedPatternOrThrow({
				match: matches[i],
				index: i,
				context,
			}),
		);
	}

	return matchedPatterns;
}

export function buildServerSuccessPreloadPlan(props: {
	signalAborted: boolean;
	isDev: boolean;
	importURLs: string[];
	deps: string[];
	cssBundles: string[];
}): ServerSuccessPreloadPlan {
	if (props.signalAborted) {
		return {
			moduleDependencies: [],
			cssBundles: [],
		};
	}

	const moduleDependenciesToPreload = props.isDev
		? [...new Set(props.importURLs)]
		: props.deps;
	const moduleDependencies: string[] = [];
	for (const dependency of moduleDependenciesToPreload) {
		if (dependency.length === 0) {
			continue;
		}
		moduleDependencies.push(dependency);
	}

	const cssBundles: string[] = [];
	for (const bundle of props.cssBundles) {
		if (bundle.length === 0) {
			continue;
		}
		cssBundles.push(bundle);
	}

	return {
		moduleDependencies,
		cssBundles,
	};
}

export function buildRouteDataRequestURL(props: {
	targetHref: string;
	navigationType: NavigateProps["navigationType"];
}): URL {
	const buildID = getRuntimeRouteSnapshot().buildID || "1";
	const deploymentID = __vormaClientGlobal.get("deploymentID");
	const url = new URL(props.targetHref);
	// Reserved internal marker for Vorma route-data requests.
	url.searchParams.set("vorma_json", buildID);

	if (props.navigationType === "revalidation") {
		if (deploymentID) {
			// Reserved deployment-routing key for skew-protection revalidation.
			url.searchParams.set("dpl", deploymentID);
		}
	}

	return url;
}

export async function createServerRouteDataPromise(props: {
	abortController: AbortController;
	url: URL;
	redirectCount?: number;
}): Promise<ServerRouteDataResult> {
	const result = await handleRedirects(props);

	if (result.response && result.response.ok && !result.redirectData?.status) {
		let json: CanonicalRouteDataPayload;
		try {
			const rawJSON = await result.response.json();
			json = decodeServerRouteDataJSONOrThrow({
				json: rawJSON,
			});
		} catch (error) {
			props.abortController.abort();
			throw error;
		}
		return { ...result, json };
	}

	return { ...result, json: undefined };
}

export function resolveServerRouteDataResult(props: {
	controller: AbortController;
	navigationProps: NavigateProps;
	serverResult: ServerRouteDataResult;
}): ResolvedServerRouteDataResult {
	const { controller, navigationProps, serverResult } = props;
	const { redirectData, response, json } = serverResult;

	const redirected = redirectData?.status === "did";
	const responseNotOK = !response?.ok;

	if (redirected || !response) {
		controller.abort();
		return { type: "outcome", outcome: { type: "aborted" } };
	}

	if (response.status === 304) {
		controller.abort();
		throw new Error("Fetch returned 304 without route JSON payload.");
	}

	if (redirectData?.status === "should") {
		controller.abort();
		return {
			type: "outcome",
			outcome: { type: "redirect", redirectData, props: navigationProps },
		};
	}

	if (responseNotOK) {
		controller.abort();
		throw new Error(`Fetch failed with status ${response.status}`);
	}

	if (!json) {
		controller.abort();
		throw new Error("No JSON response");
	}

	return { type: "success", response, json };
}

function buildServerDataByMatchedPattern(props: {
	matchedPatterns: string[];
	serverResult: ServerRouteDataResult;
}): Map<string, ClientLoaderAwaitedServerData<unknown, unknown>> {
	const serverDataByPattern = new Map<
		string,
		ClientLoaderAwaitedServerData<unknown, unknown>
	>();
	const { response, json } = props.serverResult;
	if (!response || !response.ok || !json) {
		return serverDataByPattern;
	}

	const buildID = getBuildIDFromResponse(response) || "1";
	for (const pattern of props.matchedPatterns) {
		if (!json.matchedPatterns.includes(pattern)) {
			continue;
		}
		const serverData = buildClientLoaderServerData({
			pattern,
			matchedPatterns: json.matchedPatterns,
			loadersData: json.loadersData,
			hasRootData: json.hasRootData,
			buildID,
		});
		if (serverData) {
			serverDataByPattern.set(pattern, serverData);
		}
	}

	return serverDataByPattern;
}

export async function startParallelClientLoaders(props: {
	pathname: string;
	serverPromise: Promise<ServerRouteDataResult>;
	signal: AbortSignal;
}): Promise<Map<string, Promise<unknown>>> {
	const matchResult = await findPartialMatchesOnClient(props.pathname);
	const patternToWaitFnMap: VormaClientGlobal["patternToWaitFnMap"] =
		__vormaClientGlobal.get("patternToWaitFnMap") || {};
	const runningLoaders = new Map<string, Promise<unknown>>();

	if (!matchResult) {
		return runningLoaders;
	}

	const { params, splatValues, matches } = matchResult;
	const matchedPatterns = getMatchedPatternsOrThrow({
		matches: matches as Array<RouteMatch | undefined>,
		context: "Partial route matcher",
	});
	const serverDataByPatternPromise = props.serverPromise.then(
		(serverResult) =>
			buildServerDataByMatchedPattern({
				matchedPatterns,
				serverResult,
			}),
		() => {
			throw createUnavailableServerDataError();
		},
	);

	for (const pattern of matchedPatterns) {
		const loaderFn = patternToWaitFnMap[pattern];

		if (!loaderFn) {
			continue;
		}

		const serverDataPromise = serverDataByPatternPromise.then(
			(serverDataByPattern) => {
				const serverData = serverDataByPattern.get(pattern);
				if (serverData) {
					return serverData;
				}
				throw createUnavailableServerDataError();
			},
		);

		const loaderPromise = loaderFn({
			params,
			splatValues,
			serverDataPromise,
			signal: props.signal,
		});
		observePromiseRejection(loaderPromise);

		runningLoaders.set(pattern, loaderPromise);
	}

	return runningLoaders;
}

export function buildServerSuccessOutcome(props: {
	response: Response;
	json: CanonicalRouteDataPayload;
	navigationProps: NavigateProps;
	runningLoaders: Map<string, Promise<unknown>>;
	signal: AbortSignal;
}): Extract<NavigationOutcome, { type: "success" }> {
	const { response, json, navigationProps, runningLoaders, signal } = props;
	const preloadPlan = buildServerSuccessPreloadPlan({
		signalAborted: signal.aborted,
		isDev: import.meta.env.DEV,
		importURLs: json.importURLs,
		deps: json.deps,
		cssBundles: json.cssBundles,
	});
	const buildID = getBuildIDFromResponse(response);

	const waitFnPromise = completeClientLoaders(
		json,
		buildID,
		runningLoaders,
		signal,
	);
	observePromiseRejection(waitFnPromise);

	return {
		type: "success",
		response,
		json,
		props: navigationProps,
		preloadPlan,
		waitFnPromise,
	};
}

export async function fetchRouteData(
	controller: AbortController,
	props: NavigateProps,
): Promise<NavigationOutcome> {
	try {
		const targetURL = new URL(props.href, window.location.href);
		const requestURL = buildRouteDataRequestURL({
			targetHref: targetURL.href,
			navigationType: props.navigationType,
		});

		const serverPromise = createServerRouteDataPromise({
			abortController: controller,
			url: requestURL,
			redirectCount: props.redirectCount,
		});

		const runningLoaders = await startParallelClientLoaders({
			pathname: requestURL.pathname,
			serverPromise,
			signal: controller.signal,
		});

		const resolvedServerResult = resolveServerRouteDataResult({
			controller,
			navigationProps: props,
			serverResult: await serverPromise,
		});
		if (resolvedServerResult.type === "outcome") {
			return resolvedServerResult.outcome;
		}
		const { response, json } = resolvedServerResult;

		return buildServerSuccessOutcome({
			response,
			json,
			navigationProps: props,
			runningLoaders,
			signal: controller.signal,
		});
	} catch (error) {
		if (!isAbortError(error)) {
			logError("Navigation failed", error);
		}
		throw error;
	}
}

/////////////////////////////////////////////////////////////////////
/////// Navigation Ownership And Entry
/////////////////////////////////////////////////////////////////////

function classifyOperationOwnershipByOperationID(props: {
	actualOperationID: number | null | undefined;
	expectedOperationID: number | undefined;
}): OperationOwnershipState {
	if (typeof props.expectedOperationID !== "number") {
		return "none";
	}
	if (typeof props.actualOperationID !== "number") {
		return "none";
	}

	return props.actualOperationID === props.expectedOperationID
		? "current"
		: "stale";
}

function classifyOperationOwnershipForEntry<
	Entry extends {
		operationID: number;
	},
>(props: {
	entry: Entry | null | undefined;
	expectedOperationID: number | undefined;
}): OperationOwnershipState {
	return classifyOperationOwnershipByOperationID({
		actualOperationID: props.entry?.operationID,
		expectedOperationID: props.expectedOperationID,
	});
}

export function resolveOwnedOperationEntry<
	Entry extends {
		operationID: number;
	},
>(props: {
	entry: Entry | null | undefined;
	expectedOperationID: number | undefined;
}): Entry | undefined {
	return classifyOperationOwnershipForEntry(props) === "current"
		? (props.entry ?? undefined)
		: undefined;
}

function hasEntryWithSameNavigationTarget(props: {
	entry: Pick<NavigationEntry, "targetUrl"> | null;
	targetUrl: string;
}): boolean {
	const { entry, targetUrl } = props;
	return (
		!!entry &&
		hasSameDataTarget({
			firstHref: entry.targetUrl,
			secondHref: targetUrl,
		})
	);
}

function buildActiveLanePromotion(props: {
	navigationProps: NavigateProps;
	targetUrl: string;
}): BeginNavigationPromotion {
	return {
		targetUrl: props.targetUrl,
		type: props.navigationProps.navigationType,
		intent: "navigate",
		scrollToTop: props.navigationProps.scrollToTop,
		replace: props.navigationProps.replace,
		state: props.navigationProps.state,
	};
}

function decideActiveLaneBeginExecutionPlan(props: {
	navigationProps: NavigateProps;
	targetUrl: string;
	lanes: BeginNavigationLaneSnapshot;
}): BeginNavigationExecutionPlan {
	const { navigationProps, targetUrl, lanes } = props;
	const active = lanes.active;
	const revalidation = lanes.revalidation;
	const activeHasSameNavigationTarget = hasEntryWithSameNavigationTarget({
		entry: active,
		targetUrl,
	});
	const revalidationHasSameNavigationTarget =
		hasEntryWithSameNavigationTarget({
			entry: revalidation,
			targetUrl,
		});
	const prefetchMatch = activeHasSameNavigationTarget
		? undefined
		: findMapEntryByNavigationTarget({
				map: lanes.prefetch,
				targetHref: targetUrl,
			});

	const abortInstructions: BeginNavigationAbortInstruction[] = [];
	if (active && !activeHasSameNavigationTarget) {
		abortInstructions.push({
			slot: "active",
			entry: active,
		});
	}

	for (const [key, prefetchEntry] of lanes.prefetch.entries()) {
		if (key !== prefetchMatch?.[0]) {
			abortInstructions.push({
				slot: "prefetch",
				key,
				entry: prefetchEntry,
			});
		}
	}

	if (revalidation && !revalidationHasSameNavigationTarget) {
		abortInstructions.push({
			slot: "revalidation",
			entry: revalidation,
		});
	}

	const promotion = buildActiveLanePromotion({
		navigationProps,
		targetUrl,
	});

	if (active && activeHasSameNavigationTarget) {
		return {
			type: "reuse",
			abortInstructions,
			reuseInstruction: {
				sourceSlot: "active",
				sourcePrefetchKey: null,
				entry: active,
				promotion,
			},
		};
	}

	if (prefetchMatch) {
		const [prefetchMatchKey, prefetchMatchEntry] = prefetchMatch;
		return {
			type: "reuse",
			abortInstructions,
			reuseInstruction: {
				sourceSlot: "prefetch",
				sourcePrefetchKey: prefetchMatchKey,
				entry: prefetchMatchEntry,
				promotion,
			},
		};
	}

	if (revalidation && revalidationHasSameNavigationTarget) {
		return {
			type: "reuse",
			abortInstructions,
			reuseInstruction: {
				sourceSlot: "revalidation",
				sourcePrefetchKey: null,
				entry: revalidation,
				promotion,
			},
		};
	}

	return {
		type: "create",
		abortInstructions,
		createInstruction: {
			slot: "active",
		},
	};
}

function decidePrefetchBeginExecutionPlan(props: {
	targetUrl: string;
	currentHref: string;
	lanes: BeginNavigationLaneSnapshot;
}): BeginNavigationExecutionPlan {
	const { targetUrl, currentHref, lanes } = props;
	const active = lanes.active;
	const revalidation = lanes.revalidation;
	const prefetchMatch = findMapEntryByNavigationTarget({
		map: lanes.prefetch,
		targetHref: targetUrl,
	});

	if (
		active &&
		hasEntryWithSameNavigationTarget({
			entry: active,
			targetUrl,
		})
	) {
		return {
			type: "reuse",
			abortInstructions: [],
			reuseInstruction: {
				sourceSlot: "active",
				sourcePrefetchKey: null,
				entry: active,
				promotion: null,
			},
		};
	}

	if (prefetchMatch) {
		const [prefetchMatchKey, prefetchMatchEntry] = prefetchMatch;
		return {
			type: "reuse",
			abortInstructions: [],
			reuseInstruction: {
				sourceSlot: "prefetch",
				sourcePrefetchKey: prefetchMatchKey,
				entry: prefetchMatchEntry,
				promotion: null,
			},
		};
	}

	if (
		revalidation &&
		hasEntryWithSameNavigationTarget({
			entry: revalidation,
			targetUrl,
		})
	) {
		return {
			type: "reuse",
			abortInstructions: [],
			reuseInstruction: {
				sourceSlot: "revalidation",
				sourcePrefetchKey: null,
				entry: revalidation,
				promotion: null,
			},
		};
	}

	if (
		hasSameDataTarget({
			firstHref: currentHref,
			secondHref: targetUrl,
		})
	) {
		return {
			type: "immediateAbort",
			abortInstructions: [],
		};
	}

	return {
		type: "create",
		abortInstructions: [],
		createInstruction: {
			slot: "prefetch",
			targetUrl,
		},
	};
}

function decideRevalidationBeginExecutionPlan(props: {
	currentHref: string;
	lanes: BeginNavigationLaneSnapshot;
}): BeginNavigationExecutionPlan {
	const targetUrl = resolveAbsoluteHref({ href: props.currentHref });
	const revalidation = props.lanes.revalidation;
	if (
		revalidation &&
		hasEntryWithSameNavigationTarget({
			entry: revalidation,
			targetUrl,
		})
	) {
		return {
			type: "reuse",
			abortInstructions: [],
			reuseInstruction: {
				sourceSlot: "revalidation",
				sourcePrefetchKey: null,
				entry: revalidation,
				promotion: null,
			},
		};
	}

	return {
		type: "create",
		abortInstructions: revalidation
			? [
					{
						slot: "revalidation",
						entry: revalidation,
					},
				]
			: [],
		createInstruction: {
			slot: "revalidation",
			revalidationHref: props.currentHref,
		},
	};
}

/////////////////////////////////////////////////////////////////////
/////// Begin Navigation
/////////////////////////////////////////////////////////////////////

export function resolveBeginNavigationTargetURL(props: {
	navigationProps: NavigateProps;
	currentHref: string;
}): string {
	if (props.navigationProps.navigationType === "revalidation") {
		return resolveAbsoluteHref({ href: props.currentHref });
	}

	return resolveAbsoluteHref({ href: props.navigationProps.href });
}

export function decideBeginNavigationExecutionPlan(props: {
	navigationProps: NavigateProps;
	currentHref: string;
	lanes: BeginNavigationLaneSnapshot;
}): BeginNavigationExecutionPlan {
	const { navigationProps, currentHref, lanes } = props;
	const targetUrl = resolveBeginNavigationTargetURL({
		navigationProps,
		currentHref,
	});

	switch (navigationProps.navigationType) {
		case "userNavigation":
		case "browserHistory":
		case "redirect":
		case "action":
			return decideActiveLaneBeginExecutionPlan({
				navigationProps,
				targetUrl,
				lanes,
			});
		case "prefetch":
			return decidePrefetchBeginExecutionPlan({
				targetUrl,
				currentHref,
				lanes,
			});
		case "revalidation":
			return decideRevalidationBeginExecutionPlan({
				currentHref,
				lanes,
			});
	}
}

function createNavigationEntryControl(props: {
	context: BeginNavigationContext;
	navigationProps: NavigateProps;
	onFetchError: (error: unknown) => void;
}): {
	abortController: AbortController;
	promise: Promise<NavigationOutcome>;
	operationID: number;
} {
	const abortController = new AbortController();
	const operationID = props.context.allocateNavigationOperationID();
	return {
		abortController,
		promise: observePromiseRejection(
			props.context
				.fetchRouteData(abortController, props.navigationProps)
				.catch((error) => {
					props.onFetchError(error);
					throw error;
				}),
		),
		operationID,
	};
}

function createNavigationEntry(props: {
	context: BeginNavigationContext;
	navigationProps: NavigateProps;
	type: NavigationEntry["type"];
	intent: NavigationIntent;
	targetUrl: string;
	onFetchError: (error: unknown) => void;
}): NavigationEntry {
	const control = createNavigationEntryControl({
		context: props.context,
		navigationProps: props.navigationProps,
		onFetchError: props.onFetchError,
	});
	return {
		operationID: control.operationID,
		control,
		type: props.type,
		intent: props.intent,
		startTime: Date.now(),
		targetUrl: props.targetUrl,
		originUrl: window.location.href,
		scrollToTop: props.navigationProps.scrollToTop,
		replace: props.navigationProps.replace,
		state: props.navigationProps.state,
	};
}

function deleteNavigationIfOwnedOperation(props: {
	candidateEntry: NavigationEntry | null | undefined;
	expectedOperationID: number | undefined;
	targetUrl: string;
	reason: string;
	deleteNavigation: BeginNavigationContext["deleteNavigation"];
}): void {
	if (
		!resolveOwnedOperationEntry({
			entry: props.candidateEntry,
			expectedOperationID: props.expectedOperationID,
		})
	) {
		return;
	}
	props.deleteNavigation({
		targetUrl: props.targetUrl,
		reason: props.reason,
	});
}

function createActiveNavigationControl(props: {
	context: BeginNavigationContext;
	navigationProps: NavigateProps;
	intent: NavigationIntent;
}): NavigationControl {
	const targetUrl = resolveAbsoluteHref({ href: props.navigationProps.href });
	let entry: NavigationEntry;
	entry = createNavigationEntry({
		context: props.context,
		navigationProps: props.navigationProps,
		type: props.navigationProps.navigationType,
		intent: props.intent,
		targetUrl,
		onFetchError: () => {
			deleteNavigationIfOwnedOperation({
				candidateEntry: props.context.getActiveNavigation(),
				expectedOperationID: entry.operationID,
				targetUrl,
				reason: "active_navigation_fetch_rejected",
				deleteNavigation: props.context.deleteNavigation,
			});
		},
	});
	props.context.setActiveNavigation(entry);
	props.context.scheduleStatusUpdate();
	return entry.control;
}

function createPrefetchNavigationControl(props: {
	context: BeginNavigationContext;
	navigationProps: NavigateProps;
	targetUrl: string;
}): NavigationControl {
	let entry: NavigationEntry;
	entry = createNavigationEntry({
		context: props.context,
		navigationProps: props.navigationProps,
		type: "prefetch",
		intent: "none",
		targetUrl: props.targetUrl,
		onFetchError: () => {
			deleteNavigationIfOwnedOperation({
				candidateEntry:
					props.context.prefetchNavigationsByTargetUrl.get(
						props.targetUrl,
					),
				expectedOperationID: entry.operationID,
				targetUrl: props.targetUrl,
				reason: "prefetch_fetch_rejected",
				deleteNavigation: props.context.deleteNavigation,
			});
		},
	});
	props.context.prefetchNavigationsByTargetUrl.set(props.targetUrl, entry);
	return entry.control;
}

function createRevalidationNavigationControl(props: {
	context: BeginNavigationContext;
	navigationProps: NavigateProps;
}): NavigationControl {
	const targetUrl = resolveAbsoluteHref({ href: props.navigationProps.href });
	let entry: NavigationEntry;
	entry = createNavigationEntry({
		context: props.context,
		navigationProps: props.navigationProps,
		type: "revalidation",
		intent: "revalidate",
		targetUrl,
		onFetchError: () => {
			deleteNavigationIfOwnedOperation({
				candidateEntry: props.context.getRevalidationNavigation(),
				expectedOperationID: entry.operationID,
				targetUrl,
				reason: "revalidation_fetch_rejected",
				deleteNavigation: props.context.deleteNavigation,
			});
		},
	});
	props.context.setRevalidationNavigation(entry);
	props.context.scheduleStatusUpdate();
	return entry.control;
}

function promoteEntryToActiveLane(props: {
	entry: NavigationEntry;
	promotion: BeginNavigationPromotion;
}): void {
	const { entry, promotion } = props;
	entry.targetUrl = promotion.targetUrl;
	entry.scrollToTop = promotion.scrollToTop;
	entry.replace = promotion.replace;
	entry.state = promotion.state;
	entry.type = promotion.type;
	entry.intent = promotion.intent;
}

function executeAbortInstruction(props: {
	context: BeginNavigationContext;
	abortInstruction: BeginNavigationAbortInstruction;
}): { changedStatusRelevantLane: boolean } {
	const { context, abortInstruction } = props;
	abortInstruction.entry.control.abortController?.abort();

	switch (abortInstruction.slot) {
		case "active":
			if (context.getActiveNavigation() === abortInstruction.entry) {
				context.deleteNavigation({
					targetUrl: abortInstruction.entry.targetUrl,
					reason: "begin_navigation_abort_instruction_active",
				});
				return { changedStatusRelevantLane: true };
			}
			return { changedStatusRelevantLane: false };
		case "revalidation":
			if (
				context.getRevalidationNavigation() === abortInstruction.entry
			) {
				context.deleteNavigation({
					targetUrl: abortInstruction.entry.targetUrl,
					reason: "begin_navigation_abort_instruction_revalidation",
				});
				return { changedStatusRelevantLane: true };
			}
			return { changedStatusRelevantLane: false };
		case "prefetch":
			if (
				context.prefetchNavigationsByTargetUrl.get(
					abortInstruction.key,
				) === abortInstruction.entry
			) {
				context.deleteNavigation({
					targetUrl: abortInstruction.entry.targetUrl,
					reason: "begin_navigation_abort_instruction_prefetch",
				});
			}
			return { changedStatusRelevantLane: false };
	}
}

function createImmediatelyAbortedNavigationControl(): NavigationControl {
	const abortController = new AbortController();
	abortController.abort("navigation_not_started");
	return {
		abortController,
		promise: Promise.resolve({ type: "aborted" as const }),
	};
}

function shouldScheduleStatusUpdateThroughCommandPlan(props: {
	executionPlan: BeginNavigationExecutionPlan;
}): boolean {
	if (props.executionPlan.type !== "reuse") {
		return false;
	}

	const hasStatusRelevantAbort = props.executionPlan.abortInstructions.some(
		(abortInstruction) => {
			return (
				abortInstruction.slot === "active" ||
				abortInstruction.slot === "revalidation"
			);
		},
	);

	return (
		hasStatusRelevantAbort ||
		(!!props.executionPlan.reuseInstruction.promotion &&
			props.executionPlan.reuseInstruction.sourceSlot !== "active")
	);
}

export function buildBeginNavigationRuntimeCommandPlan(props: {
	navigationProps: NavigateProps;
	executionPlan: BeginNavigationExecutionPlan;
}): BeginNavigationRuntimeCommandPlan {
	const commands: BeginNavigationRuntimeCommand[] =
		props.executionPlan.abortInstructions.map((abortInstruction) => {
			return {
				type: "abort_navigation_entry",
				abortInstruction,
			};
		});

	switch (props.executionPlan.type) {
		case "reuse":
			commands.push({
				type: "apply_reuse_instruction",
				reuseInstruction: props.executionPlan.reuseInstruction,
			});
			break;
		case "immediateAbort":
		case "create":
			break;
	}

	if (
		shouldScheduleStatusUpdateThroughCommandPlan({
			executionPlan: props.executionPlan,
		})
	) {
		commands.push({
			type: "schedule_status_update_if_changed",
		});
	}

	switch (props.executionPlan.type) {
		case "reuse":
			return {
				commands,
				terminalResult: {
					type: "return_reused_control",
				},
			};
		case "immediateAbort":
			return {
				commands,
				terminalResult: {
					type: "return_immediately_aborted_control",
				},
			};
		case "create":
			return {
				commands,
				terminalResult: {
					type: "create_navigation_control",
					createInstruction: props.executionPlan.createInstruction,
					navigationProps: props.navigationProps,
				},
			};
	}
}

function executeBeginNavigationReuseInstruction(props: {
	context: BeginNavigationContext;
	reuseInstruction: BeginNavigationReuseInstruction;
}): { control: NavigationControl; changedStatusRelevantLane: boolean } {
	const { context, reuseInstruction } = props;
	let changedStatusRelevantLane = false;

	if (reuseInstruction.promotion) {
		promoteEntryToActiveLane({
			entry: reuseInstruction.entry,
			promotion: reuseInstruction.promotion,
		});
	}

	switch (reuseInstruction.sourceSlot) {
		case "active":
			return {
				control: reuseInstruction.entry.control,
				changedStatusRelevantLane,
			};
		case "prefetch":
			if (reuseInstruction.promotion) {
				if (
					reuseInstruction.sourcePrefetchKey &&
					context.prefetchNavigationsByTargetUrl.get(
						reuseInstruction.sourcePrefetchKey,
					) === reuseInstruction.entry
				) {
					context.prefetchNavigationsByTargetUrl.delete(
						reuseInstruction.sourcePrefetchKey,
					);
				}
				context.setActiveNavigation(reuseInstruction.entry);
				changedStatusRelevantLane = true;
			}
			return {
				control: reuseInstruction.entry.control,
				changedStatusRelevantLane,
			};
		case "revalidation":
			if (reuseInstruction.promotion) {
				if (
					context.getRevalidationNavigation() ===
					reuseInstruction.entry
				) {
					context.setRevalidationNavigation(null);
				}
				context.setActiveNavigation(reuseInstruction.entry);
				changedStatusRelevantLane = true;
			}
			return {
				control: reuseInstruction.entry.control,
				changedStatusRelevantLane,
			};
	}
}

function executeSynchronousRuntimeCommandList<Command>(props: {
	commands: readonly Command[];
	executeCommand: (props: { command: Command }) => void;
}): void {
	for (const command of props.commands) {
		props.executeCommand({
			command,
		});
	}
}

async function executeAsynchronousRuntimeCommandList<Command>(props: {
	commands: readonly Command[];
	executeCommand: (props: {
		command: Command;
	}) =>
		| void
		| { shouldStop: boolean }
		| Promise<void | { shouldStop: boolean }>;
}): Promise<void> {
	for (const command of props.commands) {
		const commandResult = await props.executeCommand({
			command,
		});
		if (commandResult?.shouldStop) {
			break;
		}
	}
}

function executeBeginNavigationRuntimeCommandPlan(props: {
	context: BeginNavigationContext;
	commandPlan: BeginNavigationRuntimeCommandPlan;
}): NavigationControl {
	const { context, commandPlan } = props;
	let didChangeStatusRelevantLane = false;
	let reusedControl: NavigationControl | undefined;

	for (const command of commandPlan.commands) {
		switch (command.type) {
			case "abort_navigation_entry": {
				const abortResult = executeAbortInstruction({
					context,
					abortInstruction: command.abortInstruction,
				});
				if (abortResult.changedStatusRelevantLane) {
					didChangeStatusRelevantLane = true;
				}
				break;
			}
			case "apply_reuse_instruction": {
				const reuseResult = executeBeginNavigationReuseInstruction({
					context,
					reuseInstruction: command.reuseInstruction,
				});
				reusedControl = reuseResult.control;
				if (reuseResult.changedStatusRelevantLane) {
					didChangeStatusRelevantLane = true;
				}
				break;
			}
			case "schedule_status_update_if_changed":
				if (didChangeStatusRelevantLane) {
					context.scheduleStatusUpdate();
				}
				break;
		}
	}

	switch (commandPlan.terminalResult.type) {
		case "return_reused_control":
			if (!reusedControl) {
				throw new Error(
					"Begin navigation runtime command plan violated: reused control was not produced.",
				);
			}
			return reusedControl;
		case "return_immediately_aborted_control":
			return createImmediatelyAbortedNavigationControl();
		case "create_navigation_control":
			switch (commandPlan.terminalResult.createInstruction.slot) {
				case "active":
					return createActiveNavigationControl({
						context,
						navigationProps:
							commandPlan.terminalResult.navigationProps,
						intent: "navigate",
					});
				case "prefetch":
					return createPrefetchNavigationControl({
						context,
						navigationProps:
							commandPlan.terminalResult.navigationProps,
						targetUrl:
							commandPlan.terminalResult.createInstruction
								.targetUrl,
					});
				case "revalidation":
					return createRevalidationNavigationControl({
						context,
						navigationProps: {
							...commandPlan.terminalResult.navigationProps,
							href: commandPlan.terminalResult.createInstruction
								.revalidationHref,
						},
					});
			}
	}
}

export function executeBeginNavigation(
	context: BeginNavigationContext,
	props: NavigateProps,
): NavigationControl {
	const executionPlan = decideBeginNavigationExecutionPlan({
		navigationProps: props,
		currentHref: window.location.href,
		lanes: {
			active: context.getActiveNavigation(),
			revalidation: context.getRevalidationNavigation(),
			prefetch: context.prefetchNavigationsByTargetUrl,
		},
	});
	const commandPlan = buildBeginNavigationRuntimeCommandPlan({
		navigationProps: props,
		executionPlan,
	});

	return executeBeginNavigationRuntimeCommandPlan({
		context,
		commandPlan,
	});
}

export const beginNavigation = executeBeginNavigation;

/////////////////////////////////////////////////////////////////////
/////// Navigation Lanes And Status
/////////////////////////////////////////////////////////////////////

export function createRuntimeLanes(): RuntimeLanes {
	return {
		active: null,
		revalidation: null,
		prefetch: new Map<string, NavigationEntry>(),
		submissions: new Map<string | symbol, SubmissionEntry>(),
	};
}

function isRuntimeEngineNavigationLanePending(props: {
	laneState: NavigationRuntimeEngineNavigationLaneState;
}): boolean {
	return (
		props.laneState.ownership === "current" &&
		props.laneState.phase !== "idle" &&
		props.laneState.phase !== "complete"
	);
}

export function computeNavigationStatus(props: {
	runtimeEngineState: NavigationRuntimeEngineState;
	submissions: Map<string | symbol, SubmissionEntry>;
}): StatusEventDetail {
	const { runtimeEngineState, submissions } = props;

	const isNavigating = isRuntimeEngineNavigationLanePending({
		laneState: runtimeEngineState.lanes.navigate,
	});

	const isRevalidating = isRuntimeEngineNavigationLanePending({
		laneState: runtimeEngineState.lanes.revalidate,
	});

	let isSubmitting = false;
	for (const submission of submissions.values()) {
		if (!submission.skipGlobalLoadingIndicator) {
			isSubmitting = true;
			break;
		}
	}

	return { isNavigating, isSubmitting, isRevalidating };
}

export function createStatusSignaler(props: {
	getStatus: () => StatusEventDetail;
	dispatchStatusEvent: (status: StatusEventDetail) => void;
	debounceMS?: number;
}): () => void {
	const { getStatus, dispatchStatusEvent, debounceMS = 8 } = props;
	let lastDispatchedStatus: StatusEventDetail | null = null;

	const scheduleStatusUpdate = debounce(() => {
		const newStatus = getStatus();
		if (jsonDeepEquals(lastDispatchedStatus, newStatus)) {
			return;
		}
		lastDispatchedStatus = newStatus;
		dispatchStatusEvent(newStatus);
	}, debounceMS);

	return scheduleStatusUpdate;
}

export function getNavigationsSizeFromNavigationLanes(props: {
	lanes: NavigationLanes;
}): number {
	const { lanes } = props;
	let size = 0;
	if (lanes.active) size++;
	size += lanes.prefetch.size;
	if (lanes.revalidation) size++;
	return size;
}

export function buildNavigationsMapFromNavigationLanes(props: {
	lanes: NavigationLanes;
}): Map<string, NavigationEntry> {
	const { lanes } = props;
	const map = new Map<string, NavigationEntry>();
	if (lanes.active) {
		map.set(lanes.active.targetUrl, lanes.active);
	}
	for (const [key, entry] of lanes.prefetch) {
		map.set(key, entry);
	}
	if (lanes.revalidation) {
		map.set(lanes.revalidation.targetUrl, lanes.revalidation);
	}
	return map;
}

export function matchNavigationLaneByTargetURL(props: {
	lanes: NavigationLanes;
	targetUrl: string;
}): NavigationLaneMatch | undefined {
	const { lanes, targetUrl } = props;
	if (
		lanes.active &&
		hasSameDataTarget({
			firstHref: lanes.active.targetUrl,
			secondHref: targetUrl,
		})
	) {
		return {
			lane: "active",
			entry: lanes.active,
		};
	}

	const prefetchLaneKey = findMapEntryByNavigationTarget({
		map: lanes.prefetch,
		targetHref: targetUrl,
	})?.[0];
	if (prefetchLaneKey) {
		return {
			lane: "prefetch",
			key: prefetchLaneKey,
			entry: lanes.prefetch.get(prefetchLaneKey)!,
		};
	}

	if (
		lanes.revalidation &&
		hasSameDataTarget({
			firstHref: lanes.revalidation.targetUrl,
			secondHref: targetUrl,
		})
	) {
		return {
			lane: "revalidation",
			entry: lanes.revalidation,
		};
	}

	return undefined;
}

export function deleteNavigationFromNavigationLanes(props: {
	lanes: NavigationLanes;
	targetUrl: string;
	onStatusRelevantChange: () => void;
}): boolean {
	const { lanes, targetUrl, onStatusRelevantChange } = props;
	const matchedLane = matchNavigationLaneByTargetURL({
		lanes,
		targetUrl,
	});
	if (!matchedLane) {
		return false;
	}

	switch (matchedLane.lane) {
		case "active":
			lanes.active = null;
			onStatusRelevantChange();
			return true;
		case "prefetch":
			lanes.prefetch.delete(matchedLane.key);
			return true;
		case "revalidation":
			lanes.revalidation = null;
			onStatusRelevantChange();
			return true;
	}
}

export function clearRuntimeLanes(props: {
	lanes: RuntimeLanes;
	onStatusRelevantChange: () => void;
}): void {
	const { lanes, onStatusRelevantChange } = props;
	if (lanes.active) {
		lanes.active.control.abortController?.abort();
		lanes.active = null;
	}

	for (const prefetchEntry of lanes.prefetch.values()) {
		prefetchEntry.control.abortController?.abort();
	}
	lanes.prefetch.clear();

	if (lanes.revalidation) {
		lanes.revalidation.control.abortController?.abort();
		lanes.revalidation = null;
	}

	for (const submissionEntry of lanes.submissions.values()) {
		submissionEntry.control.abortController?.abort();
	}
	lanes.submissions.clear();

	onStatusRelevantChange();
}

/////////////////////////////////////////////////////////////////////
/////// Navigation Outcome Reasons
/////////////////////////////////////////////////////////////////////

export const navigationOutcomeExecutionReason = {
	stop: {
		entryNotFound: "entry_not_found",
		staleControlOwnership: "stale_control_ownership",
		nonCurrentEntry: "non_current_entry",
		postAssetEntryLost: "post_asset_entry_lost",
		postAssetStaleRevalidation: "post_asset_stale_revalidation",
	},
	deleteAndStop: {
		outcomeAborted: "outcome_aborted",
		staleRevalidationPreWaiting: "stale_revalidation_pre_waiting",
		redirectIgnoredForPrefetchOrStaleRevalidation:
			"redirect_ignored_for_prefetch_or_stale_revalidation",
	},
	redirect: {
		effectuate: "redirect_effectuate",
	},
	success: {
		process: "success_process",
	},
} as const;

export const successfulNavigationLifecycleReason = {
	preWaiting: {
		nonCurrentEntry: navigationOutcomeExecutionReason.stop.nonCurrentEntry,
		staleRevalidationPreWaiting:
			navigationOutcomeExecutionReason.deleteAndStop
				.staleRevalidationPreWaiting,
		entryCurrentAndFresh: "entry_current_and_fresh",
	},
	postWaiting: {
		nonCurrentEntry: "post_waiting_non_current_entry",
		entryCurrentAndFresh: "post_waiting_entry_current_and_fresh",
	},
	postAsset: {
		entryLost: navigationOutcomeExecutionReason.stop.postAssetEntryLost,
		staleRevalidation:
			navigationOutcomeExecutionReason.stop.postAssetStaleRevalidation,
		idlePrefetch: "post_asset_idle_prefetch",
		render: "post_asset_render",
	},
	cleanup: {
		successfulNavigation: "successful_navigation_cleanup",
		skippedIdlePrefetchOrNonCurrentEntry:
			"cleanup_skipped_idle_prefetch_or_non_current_entry",
	},
	internalNavigateResult: {
		navigatePromiseRejected: "navigate_promise_rejected",
	},
} as const;

/////////////////////////////////////////////////////////////////////
/////// Navigation Entry Lifecycle Classification
/////////////////////////////////////////////////////////////////////

export function isIdlePrefetchNavigationEntry(entry: NavigationEntry): boolean {
	return entry.type === "prefetch" && entry.intent === "none";
}

export function isStaleRevalidationNavigationEntry(props: {
	entry: NavigationEntry;
	currentHref: string;
}): boolean {
	const { entry, currentHref } = props;
	return (
		entry.type === "revalidation" &&
		!hasSameDataTarget({
			firstHref: currentHref,
			secondHref: entry.originUrl,
		})
	);
}

export function resolveNavigationEntryLifecycleState(props: {
	entry: NavigationEntry;
	isCurrentEntry: boolean;
	currentHref: string;
}): NavigationEntryLifecycleState {
	const { entry, isCurrentEntry, currentHref } = props;
	if (!isCurrentEntry) {
		return "non_current";
	}

	if (isIdlePrefetchNavigationEntry(entry)) {
		return "idle_prefetch";
	}

	if (
		isStaleRevalidationNavigationEntry({
			entry,
			currentHref,
		})
	) {
		return "stale_revalidation";
	}

	return "current_fresh";
}

/////////////////////////////////////////////////////////////////////
/////// Navigation Pass Planning
/////////////////////////////////////////////////////////////////////

export function decideNavigationPassRejectionExecutionPlan(props: {
	targetUrl: string;
	candidateEntry: NavigationEntry | undefined;
	expectedOperationID: number | undefined;
}): NavigationPassRejectionExecutionPlan {
	const { targetUrl, candidateEntry, expectedOperationID } = props;
	const ownedEntry = resolveOwnedOperationEntry({
		entry: candidateEntry,
		expectedOperationID,
	});
	if (!ownedEntry) {
		return {
			type: "report",
			targetUrl,
		};
	}

	return {
		type: "deleteAndReport",
		targetUrl,
		ownedEntry,
	};
}

export function buildNavigationPassRejectionRuntimeCommandPlan(props: {
	executionPlan: NavigationPassRejectionExecutionPlan;
}): NavigationPassRuntimeCommandPlan {
	if (props.executionPlan.type === "report") {
		return {
			commands: [],
			report: {
				targetUrl: props.executionPlan.targetUrl,
				ownedEntry: undefined,
			},
		};
	}

	return {
		commands: [
			{
				type: "delete_navigation",
				targetUrl: props.executionPlan.targetUrl,
				reason: "navigate_promise_rejected",
			},
		],
		report: {
			targetUrl: props.executionPlan.targetUrl,
			ownedEntry: props.executionPlan.ownedEntry,
		},
	};
}

export function reduceNavigationPassEvent(props: {
	state: NavigationPassReducerState;
	event: Extract<NavigationPassReducerEvent, { type: "outcome_resolved" }>;
}): NavigationPassResolvedReducerTransition;
export function reduceNavigationPassEvent(props: {
	state: NavigationPassReducerState;
	event: Extract<NavigationPassReducerEvent, { type: "outcome_rejected" }>;
}): NavigationPassRejectedReducerTransition;
export function reduceNavigationPassEvent(props: {
	state: NavigationPassReducerState;
	event: NavigationPassReducerEvent;
}): NavigationPassReducerTransition {
	const { state, event } = props;

	if (event.type === "outcome_rejected") {
		const rejectionExecutionPlan =
			decideNavigationPassRejectionExecutionPlan({
				targetUrl: state.targetUrl,
				candidateEntry: state.entry,
				expectedOperationID: state.expectedOperationID,
			});
		return {
			type: "rejected",
			executionPlan: rejectionExecutionPlan,
			commandPlan: buildNavigationPassRejectionRuntimeCommandPlan({
				executionPlan: rejectionExecutionPlan,
			}),
		};
	}

	const outcomeExecutionPlan = decideNavigationOutcomeExecutionPlan({
		outcome: event.outcome,
		targetUrl: state.targetUrl,
		entry: state.entry,
		expectedOperationID: state.expectedOperationID,
		currentHref: state.currentHref,
	});
	return {
		type: "resolved",
		executionPlan: outcomeExecutionPlan,
		commandPlan: buildNavigationOutcomeRuntimeCommandPlan({
			executionPlan: outcomeExecutionPlan,
			resolvedTargetUrl: state.targetUrl,
			navigationProps: event.navigationProps,
		}),
	};
}

/////////////////////////////////////////////////////////////////////
/////// Navigation Outcome Planning
/////////////////////////////////////////////////////////////////////

export function decideNavigationOutcomeExecutionPlan(props: {
	outcome: NavigationOutcome;
	targetUrl: string;
	entry: NavigationEntry | undefined;
	expectedOperationID: number | undefined;
	currentHref: string;
}): NavigationOutcomeExecutionPlan {
	const { outcome, targetUrl, entry, expectedOperationID, currentHref } =
		props;

	const ownedEntry = resolveOwnedOperationEntry({
		entry,
		expectedOperationID,
	});
	if (!ownedEntry) {
		return {
			type: "stop",
			reason: entry
				? navigationOutcomeExecutionReason.stop.staleControlOwnership
				: navigationOutcomeExecutionReason.stop.entryNotFound,
		};
	}

	if (outcome.type === "aborted") {
		return {
			type: "deleteAndStop",
			targetUrl,
			reason: navigationOutcomeExecutionReason.deleteAndStop
				.outcomeAborted,
		};
	}

	const currentOwnedEntryLifecycleState =
		resolveNavigationEntryLifecycleState({
			entry: ownedEntry,
			isCurrentEntry: true,
			currentHref,
		});

	if (outcome.type === "redirect") {
		const shouldIgnoreRedirect =
			currentOwnedEntryLifecycleState === "idle_prefetch" ||
			currentOwnedEntryLifecycleState === "stale_revalidation";
		if (shouldIgnoreRedirect) {
			return {
				type: "deleteAndStop",
				targetUrl,
				reason: navigationOutcomeExecutionReason.deleteAndStop
					.redirectIgnoredForPrefetchOrStaleRevalidation,
			};
		}

		return {
			type: "redirect",
			entry: ownedEntry,
			outcome,
			reason: navigationOutcomeExecutionReason.redirect.effectuate,
		};
	}

	return {
		type: "success",
		entry: ownedEntry,
		outcome,
		didNavigate: currentOwnedEntryLifecycleState !== "idle_prefetch",
		reason: navigationOutcomeExecutionReason.success.process,
	};
}

/////////////////////////////////////////////////////////////////////
/////// Navigation Outcome Command Plans
/////////////////////////////////////////////////////////////////////

export function buildNavigationOutcomeRuntimeCommandPlan(props: {
	executionPlan: NavigationOutcomeExecutionPlan;
	resolvedTargetUrl: string;
	navigationProps: NavigateProps;
}): NavigationOutcomeRuntimeCommandPlan {
	const { executionPlan, resolvedTargetUrl, navigationProps } = props;
	switch (executionPlan.type) {
		case "stop":
			return {
				terminalResult: {
					type: "cancelled",
					reason: executionPlan.reason,
				},
				commands: [],
			};
		case "deleteAndStop":
			return {
				terminalResult: {
					type: "cancelled",
					reason: executionPlan.reason,
				},
				commands: [
					{
						type: "delete_navigation",
						targetUrl: executionPlan.targetUrl,
						reason: executionPlan.reason,
					},
				],
			};
		case "redirect":
			return {
				terminalResult: {
					type: "committed_from_redirect",
				},
				commands: [
					{
						type: "sync_redirect_build_id",
						redirectOutcome: executionPlan.outcome,
					},
					{
						type: "delete_navigation",
						targetUrl: resolvedTargetUrl,
						reason: executionPlan.reason,
					},
					{
						type: "effectuate_redirect",
						redirectOutcome: executionPlan.outcome,
						redirectCount: navigationProps.redirectCount || 0,
						navigationProps: {
							...navigationProps,
							href: executionPlan.entry.targetUrl,
							navigationType: executionPlan.entry.type,
							scrollToTop: executionPlan.entry.scrollToTop,
							replace: executionPlan.entry.replace,
							state: executionPlan.entry.state,
						},
					},
				],
			};
		case "success":
			return {
				terminalResult: {
					type: "committed",
					didNavigate: executionPlan.didNavigate,
				},
				commands: [
					{
						type: "process_successful_navigation",
						successOutcome: executionPlan.outcome,
						entry: executionPlan.entry,
					},
				],
			};
	}
}

/////////////////////////////////////////////////////////////////////
/////// Successful Navigation Checkpoint Planning
/////////////////////////////////////////////////////////////////////

export function decideSuccessfulNavigationPreWaitingExecutionPlan(props: {
	entry: NavigationEntry;
	isCurrentEntry: boolean;
	currentHref: string;
}): SuccessfulNavigationPreWaitingExecutionPlan {
	const { entry, isCurrentEntry, currentHref } = props;
	const entryLifecycleState = resolveNavigationEntryLifecycleState({
		entry,
		isCurrentEntry,
		currentHref,
	});

	switch (entryLifecycleState) {
		case "non_current":
			return {
				type: "stop",
				reason: successfulNavigationLifecycleReason.preWaiting
					.nonCurrentEntry,
			};
		case "stale_revalidation":
			return {
				type: "deleteAndStop",
				targetUrl: entry.targetUrl,
				reason: successfulNavigationLifecycleReason.preWaiting
					.staleRevalidationPreWaiting,
			};
		case "idle_prefetch":
		case "current_fresh":
			return {
				type: "continue",
				reason: successfulNavigationLifecycleReason.preWaiting
					.entryCurrentAndFresh,
			};
	}
}

function decideSuccessfulNavigationPostWaitingExecutionPlan(props: {
	isCurrentEntry: boolean;
}): SuccessfulNavigationPostWaitingExecutionPlan {
	if (!props.isCurrentEntry) {
		return {
			type: "stop",
			reason: successfulNavigationLifecycleReason.postWaiting
				.nonCurrentEntry,
		};
	}

	return {
		type: "continue",
		reason: successfulNavigationLifecycleReason.postWaiting
			.entryCurrentAndFresh,
	};
}

export function decideSuccessfulNavigationPostAssetExecutionPlan(props: {
	entry: NavigationEntry;
	isCurrentEntry: boolean;
	currentHref: string;
}): SuccessfulNavigationPostAssetExecutionPlan {
	const { entry, isCurrentEntry, currentHref } = props;
	const entryLifecycleState = resolveNavigationEntryLifecycleState({
		entry,
		isCurrentEntry,
		currentHref,
	});

	switch (entryLifecycleState) {
		case "non_current":
			return {
				type: "stop",
				reason: successfulNavigationLifecycleReason.postAsset.entryLost,
			};
		case "idle_prefetch":
			return {
				type: "completeWithoutRender",
				reason: successfulNavigationLifecycleReason.postAsset
					.idlePrefetch,
			};
		case "stale_revalidation":
			return {
				type: "stop",
				reason: successfulNavigationLifecycleReason.postAsset
					.staleRevalidation,
			};
		case "current_fresh":
			return {
				type: "render",
				reason: successfulNavigationLifecycleReason.postAsset.render,
			};
	}
}

export function decideSuccessfulNavigationCleanupExecutionPlan(props: {
	entry: NavigationEntry;
	isCurrentEntry: boolean;
}): SuccessfulNavigationCleanupExecutionPlan {
	const { entry, isCurrentEntry } = props;
	if (!isCurrentEntry || isIdlePrefetchNavigationEntry(entry)) {
		return {
			type: "skip",
			reason: successfulNavigationLifecycleReason.cleanup
				.skippedIdlePrefetchOrNonCurrentEntry,
		};
	}

	return {
		type: "deleteNavigation",
		targetUrl: entry.targetUrl,
		reason: successfulNavigationLifecycleReason.cleanup
			.successfulNavigation,
	};
}

/////////////////////////////////////////////////////////////////////
/////// Successful Navigation Checkpoint Command Plans
/////////////////////////////////////////////////////////////////////

export function buildSuccessfulNavigationPreWaitingRuntimeCommandPlan(props: {
	executionPlan: SuccessfulNavigationPreWaitingExecutionPlan;
}): SuccessfulNavigationRuntimeCommandPlan {
	switch (props.executionPlan.type) {
		case "stop":
			return {
				shouldStop: true,
				commands: [],
			};
		case "deleteAndStop":
			return {
				shouldStop: true,
				commands: [
					{
						type: "delete_navigation",
						targetUrl: props.executionPlan.targetUrl,
						reason: props.executionPlan.reason,
					},
				],
			};
		case "continue":
			return {
				shouldStop: false,
				commands: [{ type: "transition_to_waiting" }],
			};
	}
}

export function buildSuccessfulNavigationPostWaitingRuntimeCommandPlan(props: {
	executionPlan: SuccessfulNavigationPostWaitingExecutionPlan;
}): SuccessfulNavigationRuntimeCommandPlan {
	return {
		shouldStop: props.executionPlan.type === "stop",
		commands: [],
	};
}

export function buildSuccessfulNavigationPreAssetWaitRuntimeCommandPlan(props: {
	shouldSyncBuildIDBeforeAssetWait: boolean;
}): SuccessfulNavigationRuntimeCommandPlan {
	if (props.shouldSyncBuildIDBeforeAssetWait) {
		return {
			shouldStop: false,
			commands: [{ type: "sync_build_id_from_response" }],
		};
	}

	return {
		shouldStop: false,
		commands: [],
	};
}

export function buildSuccessfulNavigationPostAssetRuntimeCommandPlan(props: {
	executionPlan: SuccessfulNavigationPostAssetExecutionPlan;
	shouldSyncBuildIDAfterAssetWaitIfNotStopped: boolean;
}): SuccessfulNavigationRuntimeCommandPlan {
	const commands: SuccessfulNavigationRuntimeCommand[] = [];
	const shouldSyncBuildIDAfterAssetWait =
		props.shouldSyncBuildIDAfterAssetWaitIfNotStopped &&
		props.executionPlan.type !== "stop";
	if (shouldSyncBuildIDAfterAssetWait) {
		commands.push({ type: "sync_build_id_from_response" });
	}

	switch (props.executionPlan.type) {
		case "stop":
			return {
				shouldStop: true,
				commands,
			};
		case "completeWithoutRender":
			return {
				shouldStop: false,
				commands: [...commands, { type: "transition_to_complete" }],
			};
		case "render":
			return {
				shouldStop: false,
				commands: [...commands, { type: "render_navigation" }],
			};
	}
}

export function buildSuccessfulNavigationCleanupRuntimeCommandPlan(props: {
	executionPlan: SuccessfulNavigationCleanupExecutionPlan;
}): SuccessfulNavigationRuntimeCommandPlan {
	if (props.executionPlan.type === "skip") {
		return {
			shouldStop: false,
			commands: [],
		};
	}

	return {
		shouldStop: false,
		commands: [
			{
				type: "delete_navigation",
				targetUrl: props.executionPlan.targetUrl,
				reason: props.executionPlan.reason,
			},
		],
	};
}

/////////////////////////////////////////////////////////////////////
/////// Successful Navigation Checkpoint Reducer
/////////////////////////////////////////////////////////////////////

export function reduceSuccessfulNavigationCheckpointEvent(props: {
	event: SuccessfulNavigationCheckpointReducerEvent;
}): SuccessfulNavigationRuntimeCommandPlan {
	const { event } = props;
	if (event.checkpoint === "pre_asset_wait") {
		return buildSuccessfulNavigationPreAssetWaitRuntimeCommandPlan({
			shouldSyncBuildIDBeforeAssetWait:
				event.shouldSyncBuildIDBeforeAssetWait,
		});
	}

	if (event.checkpoint === "cleanup") {
		const checkpointExecutionPlan =
			decideSuccessfulNavigationCheckpointExecutionPlan({
				checkpoint: "cleanup",
				entry: event.entry,
				isCurrentEntry: event.isCurrentEntry,
			});
		return buildSuccessfulNavigationCleanupRuntimeCommandPlan({
			executionPlan: checkpointExecutionPlan.plan,
		});
	}

	if (event.checkpoint === "pre_waiting") {
		const checkpointExecutionPlan =
			decideSuccessfulNavigationCheckpointExecutionPlan({
				checkpoint: "pre_waiting",
				entry: event.entry,
				isCurrentEntry: event.isCurrentEntry,
				currentHref: event.currentHref,
			});
		return buildSuccessfulNavigationPreWaitingRuntimeCommandPlan({
			executionPlan: checkpointExecutionPlan.plan,
		});
	}

	if (event.checkpoint === "post_waiting") {
		const checkpointExecutionPlan =
			decideSuccessfulNavigationCheckpointExecutionPlan({
				checkpoint: "post_waiting",
				entry: event.entry,
				isCurrentEntry: event.isCurrentEntry,
				currentHref: event.currentHref,
			});
		return buildSuccessfulNavigationPostWaitingRuntimeCommandPlan({
			executionPlan: checkpointExecutionPlan.plan,
		});
	}

	if (event.checkpoint === "post_asset") {
		const checkpointExecutionPlan =
			decideSuccessfulNavigationCheckpointExecutionPlan({
				checkpoint: "post_asset",
				entry: event.entry,
				isCurrentEntry: event.isCurrentEntry,
				currentHref: event.currentHref,
			});
		return buildSuccessfulNavigationPostAssetRuntimeCommandPlan({
			executionPlan: checkpointExecutionPlan.plan,
			shouldSyncBuildIDAfterAssetWaitIfNotStopped:
				event.shouldSyncBuildIDAfterAssetWaitIfNotStopped,
		});
	}

	throw new Error(
		"Successful navigation checkpoint reduction violated: unsupported checkpoint.",
	);
}

export function decideSuccessfulNavigationCheckpointExecutionPlan(props: {
	checkpoint: "pre_waiting";
	entry: NavigationEntry;
	isCurrentEntry: boolean;
	currentHref: string;
}): {
	checkpoint: "pre_waiting";
	plan: SuccessfulNavigationPreWaitingExecutionPlan;
};
export function decideSuccessfulNavigationCheckpointExecutionPlan(props: {
	checkpoint: "post_waiting";
	entry: NavigationEntry;
	isCurrentEntry: boolean;
	currentHref: string;
}): {
	checkpoint: "post_waiting";
	plan: SuccessfulNavigationPostWaitingExecutionPlan;
};
export function decideSuccessfulNavigationCheckpointExecutionPlan(props: {
	checkpoint: "post_asset";
	entry: NavigationEntry;
	isCurrentEntry: boolean;
	currentHref: string;
}): {
	checkpoint: "post_asset";
	plan: SuccessfulNavigationPostAssetExecutionPlan;
};
export function decideSuccessfulNavigationCheckpointExecutionPlan(props: {
	checkpoint: "cleanup";
	entry: NavigationEntry;
	isCurrentEntry: boolean;
}): {
	checkpoint: "cleanup";
	plan: SuccessfulNavigationCleanupExecutionPlan;
};
export function decideSuccessfulNavigationCheckpointExecutionPlan(
	props: SuccessfulNavigationCheckpointExecutionPlanProps,
): SuccessfulNavigationCheckpointExecutionPlan {
	switch (props.checkpoint) {
		case "pre_waiting":
			return {
				checkpoint: "pre_waiting",
				plan: decideSuccessfulNavigationPreWaitingExecutionPlan({
					entry: props.entry,
					isCurrentEntry: props.isCurrentEntry,
					currentHref: props.currentHref,
				}),
			};
		case "post_waiting":
			return {
				checkpoint: "post_waiting",
				plan: decideSuccessfulNavigationPostWaitingExecutionPlan({
					isCurrentEntry: props.isCurrentEntry,
				}),
			};
		case "post_asset": {
			const postAssetExecutionPlan =
				decideSuccessfulNavigationPostAssetExecutionPlan({
					entry: props.entry,
					isCurrentEntry: props.isCurrentEntry,
					currentHref: props.currentHref,
				});
			return {
				checkpoint: "post_asset",
				plan: postAssetExecutionPlan,
			};
		}
		case "cleanup":
			return {
				checkpoint: "cleanup",
				plan: decideSuccessfulNavigationCleanupExecutionPlan({
					entry: props.entry,
					isCurrentEntry: props.isCurrentEntry,
				}),
			};
	}
}

/////////////////////////////////////////////////////////////////////
/////// Navigation Outcome Runtime Execution
/////////////////////////////////////////////////////////////////////

export function toPublicNavigateResult(props: {
	internalResult: InternalNavigateResult;
}): { didNavigate: boolean } {
	if (props.internalResult.type === "committed") {
		return { didNavigate: props.internalResult.didNavigate };
	}

	return { didNavigate: false };
}

export function resolveSuccessfulEntryBuildIDSyncPolicy(props: {
	entry: NavigationEntry;
}): {
	shouldSyncBuildIDBeforeAssetWait: boolean;
	shouldSyncBuildIDAfterAssetWaitIfNotStopped: boolean;
} {
	const shouldSyncBuildIDBeforeAssetWait = isIdlePrefetchNavigationEntry(
		props.entry,
	);
	return {
		shouldSyncBuildIDBeforeAssetWait,
		shouldSyncBuildIDAfterAssetWaitIfNotStopped:
			!shouldSyncBuildIDBeforeAssetWait,
	};
}

function buildNavigationPassReducerState(props: {
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	targetUrl: string;
	expectedOperationID: number | undefined;
}) {
	return {
		targetUrl: props.targetUrl,
		currentHref: window.location.href,
		entry: props.findNavigationEntry(props.targetUrl),
		expectedOperationID: props.expectedOperationID,
	};
}

/////////////////////////////////////////////////////////////////////
/////// Navigation Outcome Runtime Entry Points
/////////////////////////////////////////////////////////////////////

/**
 * Applies a completed navigation outcome and returns whether navigation committed.
 */
export async function handleNavigationOutcome(
	props: HandleNavigationOutcomeProps,
): Promise<{ didNavigate: boolean }> {
	const internalResult =
		await handleNavigationOutcomeWithInternalResult(props);

	return toPublicNavigateResult({
		internalResult,
	});
}

/**
 * Internal outcome handler that returns a richer result for runtime state machines.
 */
export async function handleNavigationOutcomeWithInternalResult(
	props: HandleNavigationOutcomeProps,
): Promise<InternalNavigateResult> {
	const {
		findNavigationEntry,
		deleteNavigation,
		processSuccessfulNavigation,
		navigationProps,
		outcome,
		expectedOperationID,
	} = props;
	const targetUrl = resolveAbsoluteHref({ href: navigationProps.href });
	const reducerTransition = reduceNavigationPassEvent({
		state: buildNavigationPassReducerState({
			findNavigationEntry,
			targetUrl,
			expectedOperationID,
		}),
		event: {
			type: "outcome_resolved",
			outcome,
			navigationProps,
		},
	});

	return executeNavigationOutcomeRuntimeCommandPlan({
		commandPlan: reducerTransition.commandPlan,
		deleteNavigation,
		processSuccessfulNavigation,
	});
}

async function executeNavigationOutcomeRuntimeCommandPlan(props: {
	commandPlan: NavigationOutcomeRuntimeCommandPlan;
	deleteNavigation: (props: { targetUrl: string; reason: string }) => boolean;
	processSuccessfulNavigation: (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	) => Promise<void>;
}): Promise<InternalNavigateResult> {
	const { commandPlan, deleteNavigation, processSuccessfulNavigation } =
		props;
	let didNavigateFromRedirect = false;

	for (const command of commandPlan.commands) {
		switch (command.type) {
			case "delete_navigation":
				deleteNavigation({
					targetUrl: command.targetUrl,
					reason: command.reason,
				});
				break;
			case "sync_redirect_build_id": {
				const redirectData = command.redirectOutcome.redirectData;
				if (redirectData.status === "should") {
					syncRuntimeBuildIDIfChanged({
						nextBuildID: redirectData.latestBuildID,
					});
				}
				break;
			}
			case "effectuate_redirect": {
				const redirectResult = await effectuateRedirectDataResult(
					command.redirectOutcome.redirectData,
					command.redirectCount,
					command.navigationProps,
				);
				didNavigateFromRedirect = redirectResult?.status === "did";
				break;
			}
			case "process_successful_navigation":
				await processSuccessfulNavigation(
					command.successOutcome,
					command.entry,
				);
				break;
		}
	}

	switch (commandPlan.terminalResult.type) {
		case "cancelled":
			return {
				type: "cancelled",
				reason: commandPlan.terminalResult.reason,
			};
		case "committed":
			return {
				type: "committed",
				didNavigate: commandPlan.terminalResult.didNavigate,
			};
		case "committed_from_redirect":
			return {
				type: "committed",
				didNavigate: didNavigateFromRedirect,
			};
	}
}

/////////////////////////////////////////////////////////////////////
/////// Navigation Pass Runtime Execution
/////////////////////////////////////////////////////////////////////

function executeNavigationPassRuntimeCommandPlan(props: {
	commandPlan: NavigationPassRuntimeCommandPlan;
	deleteNavigation: (props: { targetUrl: string; reason: string }) => boolean;
}): void {
	const { commandPlan, deleteNavigation } = props;
	executeSynchronousRuntimeCommandList({
		commands: commandPlan.commands,
		executeCommand: ({ command }) => {
			switch (command.type) {
				case "delete_navigation":
					deleteNavigation({
						targetUrl: command.targetUrl,
						reason: command.reason,
					});
					break;
			}
		},
	});
}

/////////////////////////////////////////////////////////////////////
/////// Navigation Single-Pass Orchestration
/////////////////////////////////////////////////////////////////////

/**
 * Runs a single navigation pass and centralizes rejected-promise ownership cleanup.
 */
export async function executeNavigationSinglePass(
	props: ExecuteNavigationSinglePassProps,
): Promise<{ didNavigate: boolean }> {
	const {
		navigationProps,
		beginNavigation,
		findNavigationEntry,
		deleteNavigation,
		processSuccessfulNavigation,
		onLifecycleEvent,
		onNavigationPromiseRejected,
	} = props;
	const targetUrl = resolveAbsoluteHref({ href: navigationProps.href });
	const control = beginNavigation(navigationProps);

	try {
		const outcome = await control.promise;
		onLifecycleEvent?.({
			type: "fetch-resolve",
			navigationProps,
			targetUrl,
			operationID: control.operationID,
			outcomeType: outcome.type,
		});
		if (outcome.type === "redirect") {
			onLifecycleEvent?.({
				type: "redirect",
				navigationProps,
				targetUrl,
				operationID: control.operationID,
			});
		}
		if (outcome.type === "aborted") {
			onLifecycleEvent?.({
				type: "abort",
				navigationProps,
				targetUrl,
				operationID: control.operationID,
				reason: "navigation_outcome_aborted",
			});
		}
		const navigationResult = await handleNavigationOutcome({
			findNavigationEntry,
			deleteNavigation,
			processSuccessfulNavigation,
			navigationProps,
			outcome,
			expectedOperationID: control.operationID,
		});

		return navigationResult;
	} catch {
		const rejectedTransition = reduceNavigationPassEvent({
			state: buildNavigationPassReducerState({
				findNavigationEntry,
				targetUrl,
				expectedOperationID: control.operationID,
			}),
			event: {
				type: "outcome_rejected",
			},
		});
		executeNavigationPassRuntimeCommandPlan({
			commandPlan: rejectedTransition.commandPlan,
			deleteNavigation,
		});
		onLifecycleEvent?.({
			type: "abort",
			navigationProps,
			targetUrl,
			operationID: control.operationID,
			reason: "navigate_promise_rejected",
		});
		onNavigationPromiseRejected(rejectedTransition.commandPlan.report);
		return { didNavigate: false };
	}
}

/////////////////////////////////////////////////////////////////////
/////// Successful Navigation Runtime
/////////////////////////////////////////////////////////////////////

const successfulNavigationPhaseReason = {
	waiting: "process_successful_navigation_waiting",
	rendering: "process_successful_navigation_rendering",
	complete: "process_successful_navigation_complete",
} as const;

function isCurrentNavigationEntry(props: {
	context: ProcessSuccessfulNavigationContext;
	entry: NavigationEntry;
}): boolean {
	const { context, entry } = props;
	return !!resolveOwnedOperationEntry({
		entry: context.findNavigationEntry(entry.targetUrl),
		expectedOperationID: entry.operationID,
	});
}

function transitionPhaseForCurrentEntry(props: {
	context: ProcessSuccessfulNavigationContext;
	entry: NavigationEntry;
	phase: NavigationPhase;
	reason: string;
}): void {
	const { context, entry, phase, reason } = props;
	if (!isCurrentNavigationEntry({ context, entry })) {
		return;
	}

	context.transitionPhase({
		targetUrl: entry.targetUrl,
		phase,
		reason,
	});
}

async function waitForSuccessfulNavigationAssets(
	outcome: SuccessfulNavigationOutcome,
): Promise<SuccessfulNavigationClientLoadersResult> {
	const { waitFnPromise, preloadPlan } = outcome;
	const cssBundlePromises: Array<Promise<unknown>> = [];
	for (const dependency of preloadPlan.moduleDependencies) {
		if (dependency) {
			AssetManager.preloadModule(dependency);
		}
	}
	for (const cssBundle of preloadPlan.cssBundles) {
		cssBundlePromises.push(AssetManager.preloadCSS(cssBundle));
	}

	const clientLoadersResult = await waitFnPromise;

	if (cssBundlePromises.length > 0) {
		try {
			await Promise.all(cssBundlePromises);
		} catch (error) {
			logError("Error preloading CSS bundles:", error);
		}
	}

	return clientLoadersResult;
}

function buildRunHistoryOptions(
	entry: NavigationEntry,
	props: SuccessfulNavigationOutcome["props"],
) {
	if (entry.intent !== "navigate") {
		return undefined;
	}

	return {
		href: entry.targetUrl,
		scrollStateToRestore: props.scrollStateToRestore,
		replace: entry.replace || props.replace,
		scrollToTop: entry.scrollToTop,
		state: entry.state,
	};
}

async function renderSuccessfulNavigation(
	context: ProcessSuccessfulNavigationContext,
	outcome: SuccessfulNavigationOutcome,
	entry: NavigationEntry,
	clientLoadersResult: SuccessfulNavigationClientLoadersResult,
): Promise<void> {
	transitionPhaseForCurrentEntry({
		context,
		entry,
		phase: "rendering",
		reason: successfulNavigationPhaseReason.rendering,
	});

	try {
		await __reRenderApp({
			json: outcome.json,
			navigationType: entry.type,
			clientLoadersResult,
			runHistoryOptions: buildRunHistoryOptions(entry, outcome.props),
			shouldCommit: () =>
				isCurrentNavigationEntry({
					context,
					entry,
				}),
			onFinish: () => {
				transitionPhaseForCurrentEntry({
					context,
					entry,
					phase: "complete",
					reason: successfulNavigationPhaseReason.complete,
				});
			},
		});
	} catch (error) {
		transitionPhaseForCurrentEntry({
			context,
			entry,
			phase: "complete",
			reason: successfulNavigationPhaseReason.complete,
		});
		if (!isAbortError(error)) {
			logError("Error completing navigation", error);
		}
		throw error;
	}
}

function readSuccessfulNavigationCheckpointEnvelope(props: {
	context: ProcessSuccessfulNavigationContext;
	entry: NavigationEntry;
}): {
	isCurrentEntry: boolean;
	currentHref: string;
} {
	const { context, entry } = props;
	return {
		isCurrentEntry: isCurrentNavigationEntry({ context, entry }),
		currentHref: window.location.href,
	};
}

async function executeSuccessfulNavigationRuntimeCommands(props: {
	envelope: SuccessfulNavigationCheckpointStateEnvelope;
	commands: SuccessfulNavigationRuntimeCommand[];
	clientLoadersResult?: SuccessfulNavigationClientLoadersResult;
}): Promise<void> {
	const { envelope, commands, clientLoadersResult } = props;
	const { context, outcome, entry } = envelope;

	await executeAsynchronousRuntimeCommandList({
		commands,
		executeCommand: async ({ command }) => {
			switch (command.type) {
				case "transition_to_waiting":
					transitionPhaseForCurrentEntry({
						context,
						entry,
						phase: "waiting",
						reason: successfulNavigationPhaseReason.waiting,
					});
					break;
				case "transition_to_complete":
					transitionPhaseForCurrentEntry({
						context,
						entry,
						phase: "complete",
						reason: successfulNavigationPhaseReason.complete,
					});
					break;
				case "delete_navigation":
					context.deleteNavigation({
						targetUrl: command.targetUrl,
						reason: command.reason,
					});
					break;
				case "sync_build_id_from_response":
					syncRuntimeBuildIDIfChanged({
						nextBuildID: getBuildIDFromResponse(outcome.response),
					});
					break;
				case "render_navigation":
					await renderSuccessfulNavigation(
						context,
						outcome,
						entry,
						clientLoadersResult,
					);
					break;
			}
		},
	});
}

async function runSuccessfulNavigationCheckpoint(props: {
	envelope: SuccessfulNavigationCheckpointStateEnvelope;
	event: SuccessfulNavigationCheckpointReducerEvent;
	clientLoadersResult?: SuccessfulNavigationClientLoadersResult;
}): Promise<boolean> {
	const commandPlan = reduceSuccessfulNavigationCheckpointEvent({
		event: props.event,
	});
	await executeSuccessfulNavigationRuntimeCommands({
		envelope: props.envelope,
		commands: commandPlan.commands,
		clientLoadersResult: props.clientLoadersResult,
	});
	return commandPlan.shouldStop;
}

export async function processSuccessfulNavigationRuntime(
	context: ProcessSuccessfulNavigationContext,
	outcome: SuccessfulNavigationOutcome,
	entry: NavigationEntry,
): Promise<void> {
	let didCommitSuccessfulNavigation = false;
	const envelope: SuccessfulNavigationCheckpointStateEnvelope = {
		context,
		outcome,
		entry,
	};

	try {
		const preWaitingCheckpointEnvelope =
			readSuccessfulNavigationCheckpointEnvelope({
				context,
				entry,
			});
		if (
			await runSuccessfulNavigationCheckpoint({
				envelope,
				event: {
					checkpoint: "pre_waiting",
					entry,
					isCurrentEntry: preWaitingCheckpointEnvelope.isCurrentEntry,
					currentHref: preWaitingCheckpointEnvelope.currentHref,
				},
			})
		) {
			return;
		}

		const postWaitingCheckpointEnvelope =
			readSuccessfulNavigationCheckpointEnvelope({
				context,
				entry,
			});
		if (
			await runSuccessfulNavigationCheckpoint({
				envelope,
				event: {
					checkpoint: "post_waiting",
					entry,
					isCurrentEntry:
						postWaitingCheckpointEnvelope.isCurrentEntry,
					currentHref: postWaitingCheckpointEnvelope.currentHref,
				},
			})
		) {
			return;
		}

		const buildIDSyncPolicy = resolveSuccessfulEntryBuildIDSyncPolicy({
			entry,
		});
		await runSuccessfulNavigationCheckpoint({
			envelope,
			event: {
				checkpoint: "pre_asset_wait",
				shouldSyncBuildIDBeforeAssetWait:
					buildIDSyncPolicy.shouldSyncBuildIDBeforeAssetWait,
			},
		});

		const clientLoadersResult =
			await waitForSuccessfulNavigationAssets(outcome);

		const postAssetCheckpointEnvelope =
			readSuccessfulNavigationCheckpointEnvelope({
				context,
				entry,
			});
		const shouldStopAfterPostAsset =
			await runSuccessfulNavigationCheckpoint({
				envelope,
				event: {
					checkpoint: "post_asset",
					entry,
					isCurrentEntry: postAssetCheckpointEnvelope.isCurrentEntry,
					currentHref: postAssetCheckpointEnvelope.currentHref,
					shouldSyncBuildIDAfterAssetWaitIfNotStopped:
						buildIDSyncPolicy.shouldSyncBuildIDAfterAssetWaitIfNotStopped,
				},
				clientLoadersResult,
			});
		if (shouldStopAfterPostAsset) {
			return;
		}

		didCommitSuccessfulNavigation = true;
	} finally {
		const cleanupCheckpointEnvelope =
			readSuccessfulNavigationCheckpointEnvelope({
				context,
				entry,
			});
		await runSuccessfulNavigationCheckpoint({
			envelope,
			event: {
				checkpoint: "cleanup",
				entry,
				isCurrentEntry: cleanupCheckpointEnvelope.isCurrentEntry,
			},
		});
	}

	if (didCommitSuccessfulNavigation) {
		context.onSuccessfulNavigationCommitted?.({
			entry,
			outcome,
		});
	}
}

/////////////////////////////////////////////////////////////////////
/////// Revalidation Lane Runtime
/////////////////////////////////////////////////////////////////////

export function reduceRevalidationLaneEvent(props: {
	state: RevalidationLaneReducerState;
	event: RevalidationLaneReducerEvent;
}): RevalidationLaneExecutionPlan {
	switch (props.event.type) {
		case "revalidation_requested":
			if (!props.state.hasInFlightPass) {
				return {
					type: "start_new_pass",
				};
			}
			if (!props.state.inFlightTargetMatchesCurrentHref) {
				return {
					type: "restart_after_target_mismatch",
				};
			}
			if (!props.state.isTrailingEligible) {
				return {
					type: "reuse_in_flight_pass",
				};
			}
			return {
				type: "schedule_trailing_pass",
			};
	}
}

function executeRevalidationLaneExecutionPlan(props: {
	executionPlan: RevalidationLaneExecutionPlan;
	notifyInFlightTargetMismatch: () => void;
	clearQueuedTrailingRequest: () => void;
	startPass: () => RevalidationNavigateResult;
	scheduleTrailingPass: () => RevalidationNavigateResult;
	getInFlightPromise: () => RevalidationNavigateResult;
}): RevalidationNavigateResult {
	switch (props.executionPlan.type) {
		case "start_new_pass":
			return props.startPass();
		case "restart_after_target_mismatch":
			props.notifyInFlightTargetMismatch();
			props.clearQueuedTrailingRequest();
			return props.startPass();
		case "reuse_in_flight_pass":
			return props.getInFlightPromise();
		case "schedule_trailing_pass":
			return props.scheduleTrailingPass();
	}
}

export function createDeterministicRevalidationLane(props: {
	getCurrentHref: () => string;
	getHasInFlightPass: () => boolean;
	getInFlightTargetUrl: () => string | null;
	onInFlightTargetMismatch: () => void;
}): DeterministicRevalidationLane {
	const state: RevalidationLaneState = {
		inFlightPromise: null,
		isTrailingEligible: false,
		shouldRunTrailingPass: false,
		trailingPromise: null,
		resolveTrailingPromise: null,
	};

	function resolveAndClearTrailingPromise(props?: {
		result?: { didNavigate: boolean };
	}): void {
		const result = props?.result || { didNavigate: false };
		state.resolveTrailingPromise?.(result);
		state.trailingPromise = null;
		state.resolveTrailingPromise = null;
	}

	function clearQueuedTrailingRequest(): void {
		state.shouldRunTrailingPass = false;
		resolveAndClearTrailingPromise();
	}

	function startPass(startNavigateProps: {
		navigateSinglePass: (
			props: NavigateProps,
		) => RevalidationNavigateResult;
	}): RevalidationNavigateResult {
		const { navigateSinglePass } = startNavigateProps;
		const revalidationHref = props.getCurrentHref();
		const revalidationProps: NavigateProps = {
			href: revalidationHref,
			navigationType: "revalidation",
		};
		const passPromise = navigateSinglePass(revalidationProps);
		state.inFlightPromise = passPromise;
		state.isTrailingEligible = false;

		queueMicrotask(() => {
			if (state.inFlightPromise === passPromise) {
				state.isTrailingEligible = true;
			}
		});

		void passPromise.finally(() => {
			if (state.inFlightPromise !== passPromise) {
				return;
			}

			state.inFlightPromise = null;
			state.isTrailingEligible = false;

			if (!state.shouldRunTrailingPass) {
				clearQueuedTrailingRequest();
				return;
			}

			state.shouldRunTrailingPass = false;
			const resolveTrailingPromise = state.resolveTrailingPromise;
			state.trailingPromise = null;
			state.resolveTrailingPromise = null;

			void startPass({ navigateSinglePass }).then(
				(result) => resolveTrailingPromise?.(result),
				() => resolveTrailingPromise?.({ didNavigate: false }),
			);
		});

		return passPromise;
	}

	function scheduleTrailingPass(): RevalidationNavigateResult {
		state.shouldRunTrailingPass = true;

		if (state.trailingPromise) {
			return state.trailingPromise;
		}

		state.trailingPromise = new Promise((resolve) => {
			state.resolveTrailingPromise = resolve;
		});
		return state.trailingPromise;
	}

	function hasInFlightTargetMismatch(): boolean {
		const inFlightTargetUrl = props.getInFlightTargetUrl();
		if (!inFlightTargetUrl) {
			return false;
		}

		return !hasSameDataTarget({
			firstHref: inFlightTargetUrl,
			secondHref: props.getCurrentHref(),
		});
	}

	function runRevalidation(runProps: {
		navigateSinglePass: (
			props: NavigateProps,
		) => RevalidationNavigateResult;
	}): RevalidationNavigateResult {
		const executionPlan = reduceRevalidationLaneEvent({
			state: {
				hasInFlightPass: props.getHasInFlightPass(),
				isTrailingEligible: state.isTrailingEligible,
				inFlightTargetMatchesCurrentHref: !hasInFlightTargetMismatch(),
			},
			event: {
				type: "revalidation_requested",
			},
		});
		return executeRevalidationLaneExecutionPlan({
			executionPlan,
			notifyInFlightTargetMismatch: props.onInFlightTargetMismatch,
			clearQueuedTrailingRequest,
			startPass: () => startPass(runProps),
			scheduleTrailingPass,
			getInFlightPromise: () => {
				if (!state.inFlightPromise) {
					throw new Error(
						"Revalidation lane runtime command plan violated: expected an in-flight promise.",
					);
				}
				return state.inFlightPromise;
			},
		});
	}

	function reset(): void {
		state.inFlightPromise = null;
		state.isTrailingEligible = false;
		clearQueuedTrailingRequest();
	}

	return {
		runRevalidation,
		clearQueuedTrailingRequest,
		reset,
	};
}

/////////////////////////////////////////////////////////////////////
/////// Runtime Engine State Machine
/////////////////////////////////////////////////////////////////////

function createIdleNavigationLaneState(): NavigationRuntimeEngineNavigationLaneState {
	return {
		targetUrl: null,
		operationID: null,
		phase: "idle",
		ownership: "none",
	};
}

export function createInitialNavigationRuntimeEngineState(): NavigationRuntimeEngineState {
	return {
		lanes: {
			navigate: createIdleNavigationLaneState(),
			prefetch: new Map(),
			revalidate: createIdleNavigationLaneState(),
			submit: new Map(),
		},
	};
}

export function resolveNavigationRuntimeEngineLaneForNavigateProps(props: {
	navigationProps: NavigateProps;
}): NavigationRuntimeEngineNavigationLane {
	if (props.navigationProps.navigationType === "prefetch") {
		return "prefetch";
	}
	if (props.navigationProps.navigationType === "revalidation") {
		return "revalidate";
	}
	return "navigate";
}

function resolveNavigationRuntimeEngineLaneForNavigationEntry(props: {
	entry: NavigationEntry;
}): NavigationRuntimeEngineNavigationLane {
	if (props.entry.type === "prefetch") {
		return "prefetch";
	}
	if (props.entry.type === "revalidation") {
		return "revalidate";
	}
	return "navigate";
}

function cloneNavigationRuntimeEngineState(props: {
	state: NavigationRuntimeEngineState;
}): NavigationRuntimeEngineState {
	return {
		lanes: {
			navigate: { ...props.state.lanes.navigate },
			prefetch: new Map(
				[...props.state.lanes.prefetch.entries()].map(
					([targetUrl, laneState]) => [targetUrl, { ...laneState }],
				),
			),
			revalidate: { ...props.state.lanes.revalidate },
			submit: new Map(
				[...props.state.lanes.submit.entries()].map(
					([operationID, laneState]) => [
						operationID,
						{ ...laneState },
					],
				),
			),
		},
	};
}

function resolveSameDocumentNoFetchNavigationDecision(props: {
	navigationProps: NavigateProps;
	targetUrl: string;
	currentHref: string;
}): SameDocumentNoFetchNavigationDecision {
	const { targetUrl, currentHref } = props;
	const targetClassification = classifyNavigationTargetAgainstCurrentLocation(
		{
			targetHref: targetUrl,
			currentHref,
		},
	);
	if (targetClassification === "hash-change") {
		return "hash-change";
	}
	if (targetClassification === "same-document-noop") {
		return "same-document-noop";
	}

	return "none";
}

function readNavigationLaneState(props: {
	state: NavigationRuntimeEngineState;
	lane: NavigationRuntimeEngineNavigationLane;
	targetUrl: string;
}): NavigationRuntimeEngineNavigationLaneState | undefined {
	const { state, lane, targetUrl } = props;
	switch (lane) {
		case "navigate":
			return state.lanes.navigate;
		case "revalidate":
			return state.lanes.revalidate;
		case "prefetch":
			return state.lanes.prefetch.get(targetUrl);
	}
}

function writeNavigationLaneState(props: {
	state: NavigationRuntimeEngineState;
	lane: NavigationRuntimeEngineNavigationLane;
	targetUrl: string;
	laneState: NavigationRuntimeEngineNavigationLaneState;
}): void {
	switch (props.lane) {
		case "navigate":
			props.state.lanes.navigate = props.laneState;
			return;
		case "revalidate":
			props.state.lanes.revalidate = props.laneState;
			return;
		case "prefetch":
			props.state.lanes.prefetch.set(props.targetUrl, props.laneState);
			return;
	}
}

function clearNavigationLaneState(props: {
	state: NavigationRuntimeEngineState;
	lane: NavigationRuntimeEngineNavigationLane;
	targetUrl: string;
}): void {
	switch (props.lane) {
		case "navigate":
			props.state.lanes.navigate = createIdleNavigationLaneState();
			return;
		case "revalidate":
			props.state.lanes.revalidate = createIdleNavigationLaneState();
			return;
		case "prefetch":
			props.state.lanes.prefetch.delete(props.targetUrl);
			return;
	}
}

function appendSupersededCleanupCommand(props: {
	commands: NavigationRuntimeEngineCommand[];
	lane: NavigationRuntimeEngineNavigationLane;
	laneState: NavigationRuntimeEngineNavigationLaneState;
	nextTargetUrl: string;
	nextOperationID: number | undefined;
}): void {
	const hasCurrentEntry =
		props.laneState.targetUrl !== null &&
		props.laneState.ownership === "current" &&
		props.laneState.phase !== "idle" &&
		props.laneState.phase !== "complete";
	if (!hasCurrentEntry) {
		return;
	}

	const sameTarget = hasSameDataTarget({
		firstHref: props.laneState.targetUrl!,
		secondHref: props.nextTargetUrl,
	});
	const sameOperation =
		typeof props.nextOperationID !== "number" ||
		props.laneState.operationID === null ||
		props.laneState.operationID === props.nextOperationID;
	if (sameTarget && sameOperation) {
		return;
	}

	props.commands.push({
		type: "cleanup-entry",
		lane: props.lane,
		targetUrl: props.laneState.targetUrl!,
		operationID: props.laneState.operationID,
		reason: "superseded_by_new_start",
	});
}

function isStaleEventOwnership(props: {
	laneState: NavigationRuntimeEngineNavigationLaneState;
	operationID: number | undefined;
}): boolean {
	if (typeof props.operationID !== "number") {
		return false;
	}

	return (
		classifyOperationOwnershipByOperationID({
			actualOperationID: props.laneState.operationID,
			expectedOperationID: props.operationID,
		}) !== "current"
	);
}

function reduceStartEvent(props: {
	state: NavigationRuntimeEngineState;
	event: Extract<NavigationRuntimeEngineEvent, { type: "start" }>;
}): NavigationRuntimeEngineTransition {
	const nextState = cloneNavigationRuntimeEngineState({ state: props.state });
	const commands: NavigationRuntimeEngineCommand[] = [];
	const authoritativeOperationID = props.event.operationID;

	if (props.event.lane !== "revalidate") {
		commands.push({
			type: "clear-queued-revalidation-request",
		});
	}

	if (typeof authoritativeOperationID === "number") {
		const laneState =
			readNavigationLaneState({
				state: nextState,
				lane: props.event.lane,
				targetUrl: props.event.targetUrl,
			}) ?? createIdleNavigationLaneState();
		appendSupersededCleanupCommand({
			commands,
			lane: props.event.lane,
			laneState,
			nextTargetUrl: props.event.targetUrl,
			nextOperationID: authoritativeOperationID,
		});

		const nextLaneState: NavigationRuntimeEngineNavigationLaneState = {
			targetUrl: props.event.targetUrl,
			operationID: authoritativeOperationID,
			phase: "fetching",
			ownership: "current",
		};
		writeNavigationLaneState({
			state: nextState,
			lane: props.event.lane,
			targetUrl: props.event.targetUrl,
			laneState: nextLaneState,
		});

		commands.push({
			type: "emit-events",
			eventName: "start",
			lane: props.event.lane,
			targetUrl: props.event.targetUrl,
			operationID: authoritativeOperationID,
		});
	}

	if (props.event.lane === "revalidate") {
		commands.push({
			type: "fetch",
			lane: props.event.lane,
			targetUrl: props.event.targetUrl,
			operationID: props.event.operationID ?? null,
		});
		return {
			state: nextState,
			commands,
			terminalResult: {
				type: "run-revalidation-lane",
			},
		};
	}

	const sameDocumentDecision = resolveSameDocumentNoFetchNavigationDecision({
		navigationProps: props.event.navigationProps,
		targetUrl: props.event.targetUrl,
		currentHref: props.event.currentHref,
	});
	if (sameDocumentDecision === "hash-change") {
		commands.push({
			type: "commit-same-document-hash-navigation-without-fetch",
			navigationProps: props.event.navigationProps,
			targetUrl: props.event.targetUrl,
		});
		commands.push({
			type: "history-write",
			lane: props.event.lane,
			targetUrl: props.event.targetUrl,
			operationID: props.event.operationID ?? null,
		});
		return {
			state: nextState,
			commands,
			terminalResult: {
				type: "return-without-fetch",
				didNavigate: true,
			},
		};
	}
	if (sameDocumentDecision === "same-document-noop") {
		return {
			state: nextState,
			commands,
			terminalResult: {
				type: "return-without-fetch",
				didNavigate: false,
			},
		};
	}

	commands.push({
		type: "fetch",
		lane: props.event.lane,
		targetUrl: props.event.targetUrl,
		operationID: props.event.operationID ?? null,
	});
	return {
		state: nextState,
		commands,
		terminalResult: {
			type: "run-single-pass",
		},
	};
}

function reduceFetchResolveEvent(props: {
	state: NavigationRuntimeEngineState;
	event: Extract<NavigationRuntimeEngineEvent, { type: "fetch-resolve" }>;
}): NavigationRuntimeEngineTransition {
	const nextState = cloneNavigationRuntimeEngineState({ state: props.state });
	const laneState =
		readNavigationLaneState({
			state: nextState,
			lane: props.event.lane,
			targetUrl: props.event.targetUrl,
		}) ?? createIdleNavigationLaneState();
	if (
		isStaleEventOwnership({
			laneState,
			operationID: props.event.operationID,
		})
	) {
		return {
			state: nextState,
			commands: [
				{
					type: "emit-events",
					eventName: "fetch-resolve-stale",
					lane: props.event.lane,
					targetUrl: props.event.targetUrl,
					operationID: props.event.operationID ?? null,
					detail: "ignored_stale_fetch_resolve",
				},
			],
			terminalResult: {
				type: "none",
			},
		};
	}

	const nextLaneState: NavigationRuntimeEngineNavigationLaneState = {
		targetUrl: props.event.targetUrl,
		operationID: props.event.operationID ?? laneState.operationID,
		phase: props.event.outcomeType === "success" ? "waiting" : "complete",
		ownership: "current",
	};
	writeNavigationLaneState({
		state: nextState,
		lane: props.event.lane,
		targetUrl: props.event.targetUrl,
		laneState: nextLaneState,
	});

	const commands: NavigationRuntimeEngineCommand[] = [
		{
			type: "emit-events",
			eventName: "fetch-resolve",
			lane: props.event.lane,
			targetUrl: props.event.targetUrl,
			operationID: props.event.operationID ?? null,
			detail: props.event.outcomeType,
		},
	];

	if (props.event.outcomeType === "success") {
		commands.push({
			type: "sync-build-id",
			lane: props.event.lane,
			targetUrl: props.event.targetUrl,
			operationID: props.event.operationID ?? null,
		});
		commands.push({
			type: "start-client-loaders",
			lane: props.event.lane,
			targetUrl: props.event.targetUrl,
			operationID: props.event.operationID ?? null,
		});
		commands.push({
			type: "preload-assets",
			lane: props.event.lane,
			targetUrl: props.event.targetUrl,
			operationID: props.event.operationID ?? null,
		});
	} else if (props.event.outcomeType === "redirect") {
		commands.push({
			type: "sync-build-id",
			lane: props.event.lane,
			targetUrl: props.event.targetUrl,
			operationID: props.event.operationID ?? null,
		});
		commands.push({
			type: "redirect",
			lane: props.event.lane,
			targetUrl: props.event.targetUrl,
			operationID: props.event.operationID ?? null,
		});
	} else {
		commands.push({
			type: "cleanup-entry",
			lane: props.event.lane,
			targetUrl: props.event.targetUrl,
			operationID: props.event.operationID ?? null,
			reason: "fetch_resolved_aborted",
		});
	}

	return {
		state: nextState,
		commands,
		terminalResult: {
			type: "none",
		},
	};
}

function reduceWaitResolveEvent(props: {
	state: NavigationRuntimeEngineState;
	event: Extract<NavigationRuntimeEngineEvent, { type: "wait-resolve" }>;
}): NavigationRuntimeEngineTransition {
	const nextState = cloneNavigationRuntimeEngineState({ state: props.state });
	const laneState =
		readNavigationLaneState({
			state: nextState,
			lane: props.event.lane,
			targetUrl: props.event.targetUrl,
		}) ?? createIdleNavigationLaneState();

	if (
		isStaleEventOwnership({
			laneState,
			operationID: props.event.operationID,
		})
	) {
		return {
			state: nextState,
			commands: [],
			terminalResult: {
				type: "none",
			},
		};
	}

	if (laneState.phase !== "waiting") {
		return {
			state: nextState,
			commands: [
				{
					type: "log-error",
					message:
						"Navigation runtime engine wait-resolve event received outside waiting phase.",
					lane: props.event.lane,
					targetUrl: props.event.targetUrl,
					operationID: props.event.operationID ?? null,
				},
			],
			terminalResult: {
				type: "none",
			},
		};
	}

	const nextLaneState: NavigationRuntimeEngineNavigationLaneState = {
		...laneState,
		phase: "rendering",
	};
	writeNavigationLaneState({
		state: nextState,
		lane: props.event.lane,
		targetUrl: props.event.targetUrl,
		laneState: nextLaneState,
	});

	return {
		state: nextState,
		commands: [
			{
				type: "commit-snapshot",
				lane: props.event.lane,
				targetUrl: props.event.targetUrl,
				operationID: props.event.operationID ?? null,
			},
			{
				type: "history-write",
				lane: props.event.lane,
				targetUrl: props.event.targetUrl,
				operationID: props.event.operationID ?? null,
			},
			{
				type: "emit-events",
				eventName: "wait-resolve",
				lane: props.event.lane,
				targetUrl: props.event.targetUrl,
				operationID: props.event.operationID ?? null,
			},
		],
		terminalResult: {
			type: "none",
		},
	};
}

function reduceRenderFinishEvent(props: {
	state: NavigationRuntimeEngineState;
	event: Extract<NavigationRuntimeEngineEvent, { type: "render-finish" }>;
}): NavigationRuntimeEngineTransition {
	const nextState = cloneNavigationRuntimeEngineState({ state: props.state });
	const laneState =
		readNavigationLaneState({
			state: nextState,
			lane: props.event.lane,
			targetUrl: props.event.targetUrl,
		}) ?? createIdleNavigationLaneState();
	if (
		isStaleEventOwnership({
			laneState,
			operationID: props.event.operationID,
		})
	) {
		return {
			state: nextState,
			commands: [],
			terminalResult: {
				type: "none",
			},
		};
	}

	const nextLaneState: NavigationRuntimeEngineNavigationLaneState = {
		...laneState,
		phase: "complete",
	};
	writeNavigationLaneState({
		state: nextState,
		lane: props.event.lane,
		targetUrl: props.event.targetUrl,
		laneState: nextLaneState,
	});

	const commands: NavigationRuntimeEngineCommand[] = [
		{
			type: "emit-events",
			eventName: "render-finish",
			lane: props.event.lane,
			targetUrl: props.event.targetUrl,
			operationID: props.event.operationID ?? null,
		},
	];

	return {
		state: nextState,
		commands,
		terminalResult: {
			type: "none",
		},
	};
}

function reduceRedirectEvent(props: {
	state: NavigationRuntimeEngineState;
	event: Extract<NavigationRuntimeEngineEvent, { type: "redirect" }>;
}): NavigationRuntimeEngineTransition {
	const nextState = cloneNavigationRuntimeEngineState({ state: props.state });
	const laneState =
		readNavigationLaneState({
			state: nextState,
			lane: props.event.lane,
			targetUrl: props.event.targetUrl,
		}) ?? createIdleNavigationLaneState();
	if (
		isStaleEventOwnership({
			laneState,
			operationID: props.event.operationID,
		})
	) {
		return {
			state: nextState,
			commands: [],
			terminalResult: {
				type: "none",
			},
		};
	}

	const nextLaneState: NavigationRuntimeEngineNavigationLaneState = {
		...laneState,
		phase: "complete",
	};
	writeNavigationLaneState({
		state: nextState,
		lane: props.event.lane,
		targetUrl: props.event.targetUrl,
		laneState: nextLaneState,
	});

	return {
		state: nextState,
		commands: [
			{
				type: "sync-build-id",
				lane: props.event.lane,
				targetUrl: props.event.targetUrl,
				operationID: props.event.operationID ?? null,
			},
			{
				type: "redirect",
				lane: props.event.lane,
				targetUrl: props.event.targetUrl,
				operationID: props.event.operationID ?? null,
			},
		],
		terminalResult: {
			type: "none",
		},
	};
}

function reduceAbortEvent(props: {
	state: NavigationRuntimeEngineState;
	event: Extract<NavigationRuntimeEngineEvent, { type: "abort" }>;
}): NavigationRuntimeEngineTransition {
	const nextState = cloneNavigationRuntimeEngineState({ state: props.state });
	const laneState =
		readNavigationLaneState({
			state: nextState,
			lane: props.event.lane,
			targetUrl: props.event.targetUrl,
		}) ?? createIdleNavigationLaneState();
	if (
		isStaleEventOwnership({
			laneState,
			operationID: props.event.operationID,
		})
	) {
		return {
			state: nextState,
			commands: [],
			terminalResult: {
				type: "none",
			},
		};
	}

	const nextLaneState: NavigationRuntimeEngineNavigationLaneState = {
		...laneState,
		phase: "complete",
		ownership: "stale",
	};
	writeNavigationLaneState({
		state: nextState,
		lane: props.event.lane,
		targetUrl: props.event.targetUrl,
		laneState: nextLaneState,
	});

	return {
		state: nextState,
		commands: [
			{
				type: "cleanup-entry",
				lane: props.event.lane,
				targetUrl: props.event.targetUrl,
				operationID: props.event.operationID ?? null,
				reason: props.event.reason,
			},
			{
				type: "emit-events",
				eventName: "abort",
				lane: props.event.lane,
				targetUrl: props.event.targetUrl,
				operationID: props.event.operationID ?? null,
				detail: props.event.reason,
			},
		],
		terminalResult: {
			type: "none",
		},
	};
}

function reduceTimeoutEvent(props: {
	state: NavigationRuntimeEngineState;
	event: Extract<NavigationRuntimeEngineEvent, { type: "timeout" }>;
}): NavigationRuntimeEngineTransition {
	const nextState = cloneNavigationRuntimeEngineState({ state: props.state });
	nextState.lanes.submit.delete(props.event.operationID);
	return {
		state: nextState,
		commands: [
			{
				type: "log-error",
				message: `Submission timeout: ${props.event.reason}`,
				lane: "submit",
				targetUrl: props.event.targetUrl,
				operationID: props.event.operationID,
			},
			{
				type: "emit-events",
				eventName: "timeout",
				lane: "submit",
				targetUrl: props.event.targetUrl,
				operationID: props.event.operationID,
				detail: props.event.reason,
			},
		],
		terminalResult: {
			type: "none",
		},
	};
}

function reduceExternalLocationChangeEvent(props: {
	state: NavigationRuntimeEngineState;
	event: Extract<
		NavigationRuntimeEngineEvent,
		{ type: "external-location-change" }
	>;
}): NavigationRuntimeEngineTransition {
	const nextState = cloneNavigationRuntimeEngineState({ state: props.state });
	const revalidationLane = nextState.lanes.revalidate;
	if (
		revalidationLane.targetUrl === null ||
		revalidationLane.phase === "idle" ||
		revalidationLane.phase === "complete" ||
		hasSameDataTarget({
			firstHref: revalidationLane.targetUrl,
			secondHref: props.event.currentHref,
		})
	) {
		return {
			state: nextState,
			commands: [],
			terminalResult: {
				type: "none",
			},
		};
	}

	nextState.lanes.revalidate = {
		...revalidationLane,
		phase: "complete",
		ownership: "stale",
	};
	return {
		state: nextState,
		commands: [
			{
				type: "cleanup-entry",
				lane: "revalidate",
				targetUrl: revalidationLane.targetUrl,
				operationID: revalidationLane.operationID,
				reason: "external_location_change",
			},
			{
				type: "emit-events",
				eventName: "external-location-change",
				lane: "revalidate",
				targetUrl: revalidationLane.targetUrl,
				operationID: revalidationLane.operationID,
			},
		],
		terminalResult: {
			type: "none",
		},
	};
}

function clearNavigationTargetsMatchingHref(props: {
	state: NavigationRuntimeEngineState;
	targetUrl: string;
}): Array<{
	lane: NavigationRuntimeEngineNavigationLane;
	targetUrl: string;
	operationID: number | null;
}> {
	const removed: Array<{
		lane: NavigationRuntimeEngineNavigationLane;
		targetUrl: string;
		operationID: number | null;
	}> = [];
	if (
		props.state.lanes.navigate.targetUrl &&
		hasSameDataTarget({
			firstHref: props.state.lanes.navigate.targetUrl,
			secondHref: props.targetUrl,
		})
	) {
		removed.push({
			lane: "navigate",
			targetUrl: props.state.lanes.navigate.targetUrl,
			operationID: props.state.lanes.navigate.operationID,
		});
		clearNavigationLaneState({
			state: props.state,
			lane: "navigate",
			targetUrl: props.targetUrl,
		});
	}
	if (
		props.state.lanes.revalidate.targetUrl &&
		hasSameDataTarget({
			firstHref: props.state.lanes.revalidate.targetUrl,
			secondHref: props.targetUrl,
		})
	) {
		removed.push({
			lane: "revalidate",
			targetUrl: props.state.lanes.revalidate.targetUrl,
			operationID: props.state.lanes.revalidate.operationID,
		});
		clearNavigationLaneState({
			state: props.state,
			lane: "revalidate",
			targetUrl: props.targetUrl,
		});
	}
	for (const [prefetchTargetUrl, prefetchLaneState] of props.state.lanes
		.prefetch) {
		if (
			hasSameDataTarget({
				firstHref: prefetchTargetUrl,
				secondHref: props.targetUrl,
			})
		) {
			removed.push({
				lane: "prefetch",
				targetUrl: prefetchTargetUrl,
				operationID: prefetchLaneState.operationID,
			});
			props.state.lanes.prefetch.delete(prefetchTargetUrl);
		}
	}

	return removed;
}

function reduceNavigationRemovalEvent(props: {
	state: NavigationRuntimeEngineState;
	event: Extract<
		NavigationRuntimeEngineEvent,
		{ type: "remove-navigation-requested" }
	>;
}): NavigationRuntimeEngineTransition {
	const nextState = cloneNavigationRuntimeEngineState({ state: props.state });
	const removedEntries = clearNavigationTargetsMatchingHref({
		state: nextState,
		targetUrl: props.event.targetUrl,
	});
	return {
		state: nextState,
		commands: removedEntries.map((removedEntry) => {
			return {
				type: "cleanup-entry",
				lane: removedEntry.lane,
				targetUrl: removedEntry.targetUrl,
				operationID: removedEntry.operationID,
				reason: props.event.reason,
				causedByOperationID: props.event.causedByOperationID,
			};
		}),
		terminalResult: {
			type: "none",
		},
	};
}

function reduceClearAllEvent(props: {
	state: NavigationRuntimeEngineState;
	event: Extract<
		NavigationRuntimeEngineEvent,
		{ type: "clear-all-requested" }
	>;
}): NavigationRuntimeEngineTransition {
	return {
		state: createInitialNavigationRuntimeEngineState(),
		commands: [
			{
				type: "reset-revalidation-lane",
			},
			{
				type: "clear-runtime-lanes",
			},
			{
				type: "emit-events",
				eventName: "clear-all",
				targetUrl: props.event.targetUrl,
			},
		],
		terminalResult: {
			type: "none",
		},
	};
}

function reduceSubmissionStateTransitionEvent(props: {
	state: NavigationRuntimeEngineState;
	event: Extract<
		NavigationRuntimeEngineEvent,
		{ type: "submission-state-transitioned" }
	>;
}): NavigationRuntimeEngineTransition {
	const nextState = cloneNavigationRuntimeEngineState({ state: props.state });
	if (props.event.toState === "submitting") {
		nextState.lanes.submit.set(props.event.operationID, {
			targetUrl: props.event.targetUrl,
			operationID: props.event.operationID,
			phase: "submitting",
			ownership: "current",
		});
	} else if (
		props.event.toState === "aborted" ||
		props.event.toState === "removed"
	) {
		nextState.lanes.submit.delete(props.event.operationID);
	}

	return {
		state: nextState,
		commands: [
			{
				type: "emit-events",
				eventName: "submission-state-transitioned",
				lane: "submit",
				targetUrl: props.event.targetUrl,
				operationID: props.event.operationID,
				detail: `${props.event.fromState}->${props.event.toState}:${props.event.reason}`,
			},
		],
		terminalResult: {
			type: "none",
		},
	};
}

export function reduceNavigationRuntimeEngineEvent(props: {
	state: NavigationRuntimeEngineState;
	event: NavigationRuntimeEngineEvent;
}): NavigationRuntimeEngineTransition {
	switch (props.event.type) {
		case "start":
			return reduceStartEvent({
				state: props.state,
				event: props.event,
			});
		case "fetch-resolve":
			return reduceFetchResolveEvent({
				state: props.state,
				event: props.event,
			});
		case "wait-resolve":
			return reduceWaitResolveEvent({
				state: props.state,
				event: props.event,
			});
		case "redirect":
			return reduceRedirectEvent({
				state: props.state,
				event: props.event,
			});
		case "abort":
			return reduceAbortEvent({
				state: props.state,
				event: props.event,
			});
		case "render-finish":
			return reduceRenderFinishEvent({
				state: props.state,
				event: props.event,
			});
		case "timeout":
			return reduceTimeoutEvent({
				state: props.state,
				event: props.event,
			});
		case "external-location-change":
			return reduceExternalLocationChangeEvent({
				state: props.state,
				event: props.event,
			});
		case "remove-navigation-requested":
			return reduceNavigationRemovalEvent({
				state: props.state,
				event: props.event,
			});
		case "clear-all-requested":
			return reduceClearAllEvent({
				state: props.state,
				event: props.event,
			});
		case "submission-state-transitioned":
			return reduceSubmissionStateTransitionEvent({
				state: props.state,
				event: props.event,
			});
	}
}

function shouldExecuteCleanupCommandWithOwnershipGuard(props: {
	command: Extract<NavigationRuntimeEngineCommand, { type: "cleanup-entry" }>;
	context: ExecuteNavigationRuntimeEngineCommandsContext;
}): boolean {
	if (props.command.operationID === null) {
		return true;
	}
	if (!props.context.isNavigationOperationCurrent) {
		return true;
	}

	return props.context.isNavigationOperationCurrent({
		lane: props.command.lane,
		targetUrl: props.command.targetUrl,
		operationID: props.command.operationID,
	});
}

export function executeNavigationRuntimeEngineCommands(props: {
	commands: NavigationRuntimeEngineCommand[];
	context: ExecuteNavigationRuntimeEngineCommandsContext;
}): void {
	executeSynchronousRuntimeCommandList({
		commands: props.commands,
		executeCommand: ({ command }) => {
			switch (command.type) {
				case "fetch":
				case "start-client-loaders":
				case "preload-assets":
				case "commit-snapshot":
				case "history-write":
				case "redirect":
				case "sync-build-id":
					props.context.onPlannedSideEffectCommand?.({
						command,
					});
					break;
				case "emit-events":
					props.context.emitEvent?.({
						eventName: command.eventName,
						lane: command.lane,
						targetUrl: command.targetUrl,
						operationID: command.operationID,
						detail: command.detail,
					});
					break;
				case "cleanup-entry":
					if (
						!shouldExecuteCleanupCommandWithOwnershipGuard({
							command,
							context: props.context,
						})
					) {
						break;
					}
					props.context.cleanupNavigationEntry({
						targetUrl: command.targetUrl,
						reason: command.reason,
						causedByOperationID: command.causedByOperationID,
					});
					break;
				case "log-error":
					props.context.logError?.({
						message: command.message,
						lane: command.lane,
						targetUrl: command.targetUrl,
						operationID: command.operationID,
					});
					break;
				case "clear-queued-revalidation-request":
					props.context.clearQueuedRevalidationRequest();
					break;
				case "commit-same-document-hash-navigation-without-fetch":
					props.context.commitSameDocumentHashNavigationWithoutFetch({
						navigationProps: command.navigationProps,
						targetUrl: command.targetUrl,
					});
					break;
				case "reset-revalidation-lane":
					props.context.resetRevalidationLane();
					break;
				case "clear-runtime-lanes":
					props.context.clearRuntimeLanes();
					break;
			}
		},
	});
}

const submissionLifecycleReason = {
	dedupedByNewerSubmission: "submission_deduped_by_newer_submission",
	started: "submission_started",
	finished: "submission_finished",
} as const;

function assertSubmitRuntimeReducerPhase(props: {
	state: SubmitRuntimeReducerState;
	expectedPhase: SubmitRuntimeReducerState["phase"];
	eventType: SubmitRuntimeReducerEvent<unknown>["type"];
}): void {
	if (props.state.phase === props.expectedPhase) {
		return;
	}

	throw new Error(
		`Submit runtime reducer event "${props.eventType}" cannot run from phase "${props.state.phase}". Expected "${props.expectedPhase}".`,
	);
}

function getStaleSubmitReducerTransition<TSubmitResultData>(props: {
	staleSubmitResult: SubmitResult<TSubmitResultData>;
}): SubmitRuntimeReducerTransition<TSubmitResultData> {
	return {
		state: {
			phase: "completed",
			ownership: "stale",
		},
		commands: [],
		postClassificationCommandPlan: undefined,
		shouldReadSuccessPayload: false,
		terminalResult: {
			type: "stale",
			result: props.staleSubmitResult,
		},
	};
}

export function createInitialSubmitRuntimeReducerState(): SubmitRuntimeReducerState {
	return {
		phase: "awaiting_request_resolution",
		ownership: "current",
	};
}

export function reduceSubmitRuntimeEvent<TSubmitResultData>(props: {
	state: SubmitRuntimeReducerState;
	event: SubmitRuntimeReducerEvent<TSubmitResultData>;
}): SubmitRuntimeReducerTransition<TSubmitResultData> {
	switch (props.event.type) {
		case "request_resolved": {
			assertSubmitRuntimeReducerPhase({
				state: props.state,
				expectedPhase: "awaiting_request_resolution",
				eventType: props.event.type,
			});

			if (props.event.staleSubmitResult) {
				return getStaleSubmitReducerTransition({
					staleSubmitResult: props.event.staleSubmitResult,
				});
			}

			return {
				state: {
					phase: "awaiting_response_classification",
					ownership: "current",
				},
				commands: [
					{
						type: "sync_build_id_from_response",
						response: props.event.response,
					},
				],
				postClassificationCommandPlan: undefined,
				shouldReadSuccessPayload: false,
				terminalResult: {
					type: "continue",
				},
			};
		}
		case "response_classified": {
			assertSubmitRuntimeReducerPhase({
				state: props.state,
				expectedPhase: "awaiting_response_classification",
				eventType: props.event.type,
			});

			if (props.event.staleSubmitResult) {
				return getStaleSubmitReducerTransition({
					staleSubmitResult: props.event.staleSubmitResult,
				});
			}

			const postClassificationExecutionPlan =
				decideSubmitPostClassificationExecutionPlan({
					response: props.event.response,
					redirectData: props.event.redirectData,
					requestInit: props.event.requestInit,
					options: props.event.options,
				});
			const postClassificationCommandPlan =
				buildSubmitPostClassificationRuntimeCommandPlan({
					executionPlan: postClassificationExecutionPlan,
					currentHref: props.event.currentHref,
				});

			if (postClassificationCommandPlan.terminalResult.type === "error") {
				return {
					state: {
						phase: "completed",
						ownership: "current",
					},
					commands: [],
					postClassificationCommandPlan,
					shouldReadSuccessPayload: false,
					terminalResult: {
						type: "error",
						error: postClassificationCommandPlan.terminalResult
							.error,
					},
				};
			}

			return {
				state: {
					phase:
						postClassificationCommandPlan.terminalResult.type ===
						"success"
							? "awaiting_success_payload_ownership_checkpoint"
							: "awaiting_post_classification_command_execution",
					ownership: "current",
				},
				commands: [],
				postClassificationCommandPlan,
				shouldReadSuccessPayload:
					postClassificationCommandPlan.terminalResult.type ===
					"success",
				terminalResult: {
					type: "continue",
				},
			};
		}
		case "success_payload_parsed": {
			assertSubmitRuntimeReducerPhase({
				state: props.state,
				expectedPhase: "awaiting_success_payload_ownership_checkpoint",
				eventType: props.event.type,
			});

			if (props.event.staleSubmitResult) {
				return getStaleSubmitReducerTransition({
					staleSubmitResult: props.event.staleSubmitResult,
				});
			}

			return {
				state: {
					phase: "awaiting_post_classification_command_execution",
					ownership: "current",
				},
				commands: [],
				postClassificationCommandPlan: undefined,
				shouldReadSuccessPayload: false,
				terminalResult: {
					type: "continue",
				},
			};
		}
	}
}

function emitSubmissionStateTransition(props: {
	context: SubmitExecutionContext;
	submissionEntry: SubmissionEntry;
	fromState: string;
	toState: string;
	reason: string;
	causedByOperationID?: number | null;
}): void {
	props.context.onSubmissionStateTransition?.({
		submissionEntry: props.submissionEntry,
		fromState: props.fromState,
		toState: props.toState,
		reason: props.reason,
		causedByOperationID: props.causedByOperationID ?? null,
	});
}

function executeSubmissionLifecycleCommandPlan(props: {
	context: SubmitExecutionContext;
	commandPlan: SubmissionLifecycleCommandPlan;
}): void {
	const { context, commandPlan } = props;
	executeSynchronousRuntimeCommandList({
		commands: commandPlan.commands,
		executeCommand: ({ command }) => {
			switch (command.type) {
				case "abort_submission":
					command.submissionEntry.control.abortController?.abort(
						command.abortReason,
					);
					break;
				case "set_submission":
					context.submissions.set(
						command.submissionKey,
						command.submissionEntry,
					);
					break;
				case "delete_submission":
					context.submissions.delete(command.submissionKey);
					break;
				case "emit_submission_state_transition":
					emitSubmissionStateTransition({
						context,
						submissionEntry: command.submissionEntry,
						fromState: command.fromState,
						toState: command.toState,
						reason: command.reason,
						causedByOperationID: command.causedByOperationID,
					});
					break;
				case "schedule_status_update":
					context.scheduleStatusUpdate();
					break;
			}
		},
	});
}

export function buildBeginSubmissionLifecycleCommandPlan(props: {
	submissionKey: string | symbol;
	submissionEntry: SubmissionEntry;
	existingSubmissionEntry: SubmissionEntry | undefined;
}): SubmissionLifecycleCommandPlan {
	const commands: SubmissionLifecycleCommand[] = [];
	if (props.existingSubmissionEntry) {
		commands.push({
			type: "abort_submission",
			submissionEntry: props.existingSubmissionEntry,
			abortReason: "deduped",
		});
		commands.push({
			type: "emit_submission_state_transition",
			submissionEntry: props.existingSubmissionEntry,
			fromState: "submitting",
			toState: "aborted",
			reason: submissionLifecycleReason.dedupedByNewerSubmission,
			causedByOperationID: props.submissionEntry.operationID,
		});
	}
	commands.push({
		type: "set_submission",
		submissionKey: props.submissionKey,
		submissionEntry: props.submissionEntry,
	});
	commands.push({
		type: "emit_submission_state_transition",
		submissionEntry: props.submissionEntry,
		fromState: "none",
		toState: "submitting",
		reason: submissionLifecycleReason.started,
		causedByOperationID: null,
	});
	commands.push({
		type: "schedule_status_update",
	});
	return { commands };
}

export function beginSubmissionLifecycle(props: {
	context: SubmitExecutionContext;
	submissionKey: string | symbol;
	submissionEntry: SubmissionEntry;
	existingSubmissionEntry: SubmissionEntry | undefined;
}): void {
	executeSubmissionLifecycleCommandPlan({
		context: props.context,
		commandPlan: buildBeginSubmissionLifecycleCommandPlan({
			submissionKey: props.submissionKey,
			submissionEntry: props.submissionEntry,
			existingSubmissionEntry: props.existingSubmissionEntry,
		}),
	});
}

export function buildFinishSubmissionLifecycleCommandPlan(props: {
	submissionKey: string | symbol;
	submissionEntry: SubmissionEntry;
	shouldRemoveSubmissionEntry: boolean;
}): SubmissionLifecycleCommandPlan {
	const commands: SubmissionLifecycleCommand[] = [];
	if (props.shouldRemoveSubmissionEntry) {
		commands.push({
			type: "delete_submission",
			submissionKey: props.submissionKey,
		});
		commands.push({
			type: "emit_submission_state_transition",
			submissionEntry: props.submissionEntry,
			fromState: "submitting",
			toState: "removed",
			reason: submissionLifecycleReason.finished,
			causedByOperationID: null,
		});
	}
	commands.push({
		type: "schedule_status_update",
	});
	return { commands };
}

export function finishSubmissionLifecycle(props: {
	context: SubmitExecutionContext;
	submissionKey: string | symbol;
	submissionEntry: SubmissionEntry;
	shouldRemoveSubmissionEntry: boolean;
}): void {
	executeSubmissionLifecycleCommandPlan({
		context: props.context,
		commandPlan: buildFinishSubmissionLifecycleCommandPlan({
			submissionKey: props.submissionKey,
			submissionEntry: props.submissionEntry,
			shouldRemoveSubmissionEntry: props.shouldRemoveSubmissionEntry,
		}),
	});
}

function createSubmissionLifecycle(
	context: SubmitExecutionContext,
	options?: SubmitOptions,
): SubmissionLifecycle {
	const abortController = new AbortController();
	const submissionEntry: SubmissionEntry = {
		operationID: context.allocateSubmissionOperationID(),
		control: {
			abortController,
			promise: Promise.resolve() as Promise<unknown>,
		},
		startTime: Date.now(),
		skipGlobalLoadingIndicator: options?.skipGlobalLoadingIndicator,
	};
	const submissionKey = options?.dedupeKey
		? `submission:${options.dedupeKey}`
		: Symbol("submission");

	const isCurrent = (): boolean =>
		!!resolveOwnedOperationEntry({
			entry: context.submissions.get(submissionKey),
			expectedOperationID: submissionEntry.operationID,
		});

	const begin = (): void => {
		const existingSubmissionEntry =
			typeof submissionKey === "string"
				? context.submissions.get(submissionKey)
				: undefined;
		beginSubmissionLifecycle({
			context,
			submissionKey,
			submissionEntry,
			existingSubmissionEntry,
		});
	};

	const finish = (): void => {
		finishSubmissionLifecycle({
			context,
			submissionKey,
			submissionEntry,
			shouldRemoveSubmissionEntry: isCurrent(),
		});
	};

	return {
		abortController,
		isCurrent,
		begin,
		finish,
	};
}

function getAbortedSubmitResult<T>(): SubmitResult<T> {
	return { success: false, error: "Aborted" };
}

function getSubmitErrorResult<T>(error: string): SubmitResult<T> {
	return { success: false, error };
}

function getStaleSubmitResultIfNotCurrent<T>(props: {
	isSubmissionCurrent: () => boolean;
}): SubmitResult<T> | null {
	if (props.isSubmissionCurrent()) {
		return null;
	}

	return getAbortedSubmitResult<T>();
}

function shouldAutoRevalidateSubmitResult(props: {
	requestInit?: RequestInit;
	redirectData: RedirectData | null;
	options?: SubmitOptions;
}): boolean {
	const { requestInit, redirectData, options } = props;
	const isGET = getIsGETRequest(requestInit);
	const redirected = redirectData?.status === "did";
	return !isGET && !redirected && options?.revalidate !== false;
}

export function decideSubmitPostClassificationExecutionPlan(props: {
	response: Response;
	redirectData: RedirectData | null;
	requestInit?: RequestInit;
	options?: SubmitOptions;
}): SubmitPostClassificationExecutionPlan {
	if (props.redirectData?.status === "should") {
		return {
			type: "redirect",
			redirectData: props.redirectData,
		};
	}
	if (!props.response.ok) {
		return {
			type: "error",
			error: String(props.response.status),
		};
	}
	return {
		type: "success",
		shouldAutoRevalidate: shouldAutoRevalidateSubmitResult({
			requestInit: props.requestInit,
			redirectData: props.redirectData,
			options: props.options,
		}),
	};
}

export function buildSubmitPostClassificationRuntimeCommandPlan(props: {
	executionPlan: SubmitPostClassificationExecutionPlan;
	currentHref: string;
}): SubmitPostClassificationRuntimeCommandPlan {
	switch (props.executionPlan.type) {
		case "redirect":
			return {
				terminalResult: {
					type: "redirect",
				},
				commands: [
					{
						type: "effectuate_redirect",
						redirectData: props.executionPlan.redirectData,
					},
				],
			};
		case "error":
			return {
				terminalResult: {
					type: "error",
					error: props.executionPlan.error,
				},
				commands: [],
			};
		case "success":
			return {
				terminalResult: {
					type: "success",
				},
				commands: props.executionPlan.shouldAutoRevalidate
					? [
							{
								type: "auto_revalidate_navigation",
								href: props.currentHref,
							},
						]
					: [],
			};
	}
}

function hasNoContentResponseBody(response: Response): boolean {
	if (response.status === 204 || response.status === 205) {
		return true;
	}

	return response.headers.get("content-length") === "0";
}

function responseDeclaresJSON(response: Response): boolean {
	const contentType = response.headers.get("content-type");
	if (!contentType) {
		return false;
	}

	const normalizedContentType = contentType.toLowerCase();
	return (
		normalizedContentType.includes("application/json") ||
		normalizedContentType.includes("+json")
	);
}

function responseDeclaresContentType(response: Response): boolean {
	return response.headers.has("content-type");
}

async function readSubmitSuccessResponseData(
	response: Response,
): Promise<unknown> {
	if (hasNoContentResponseBody(response)) {
		return undefined;
	}

	const maybeTextFn = (
		response as Response & {
			text?: () => Promise<string>;
		}
	).text;
	if (typeof maybeTextFn === "function") {
		const text = await maybeTextFn.call(response);
		if (text === "") {
			return undefined;
		}
		if (responseDeclaresJSON(response)) {
			return JSON.parse(text);
		}
		if (!responseDeclaresContentType(response)) {
			try {
				return JSON.parse(text);
			} catch {
				return text;
			}
		}
		return text;
	}

	const maybeJSONFn = (
		response as Response & { json?: () => Promise<unknown> }
	).json;
	if (
		typeof maybeJSONFn === "function" &&
		(responseDeclaresJSON(response) ||
			!responseDeclaresContentType(response))
	) {
		return maybeJSONFn.call(response);
	}

	return undefined;
}

function getSubmitRuntimeErrorResult<T>(props: {
	error: unknown;
	abortSignal: AbortSignal;
}): SubmitResult<T> {
	const { error, abortSignal } = props;
	if (isAbortError(error) || abortSignal.aborted) {
		return getAbortedSubmitResult<T>();
	}

	if (error instanceof Error) {
		logError(error);
		return getSubmitErrorResult<T>(error.message);
	}

	logError(error);
	return getSubmitErrorResult<T>("Unknown error");
}

/////////////////////////////////////////////////////////////////////
/////// Submission Runtime Commands
/////////////////////////////////////////////////////////////////////

function executeSubmitRuntimeCommand(props: {
	context: SubmitExecutionContext;
	command: {
		type: "sync_build_id_from_response";
		response: Response;
	};
}): Promise<void>;
function executeSubmitRuntimeCommand(props: {
	context: SubmitExecutionContext;
	command: {
		type: "effectuate_redirect";
		redirectData: RedirectData;
	};
}): ReturnType<typeof effectuateRedirectDataResult>;
function executeSubmitRuntimeCommand(props: {
	context: SubmitExecutionContext;
	command: {
		type: "auto_revalidate_navigation";
		href: string;
	};
}): ReturnType<SubmitExecutionContext["navigate"]>;
function executeSubmitRuntimeCommand(props: {
	context: SubmitExecutionContext;
	command: SubmitRuntimeCommand;
}): Promise<unknown>;
async function executeSubmitRuntimeCommand(props: {
	context: SubmitExecutionContext;
	command: SubmitRuntimeCommand;
}): Promise<unknown> {
	switch (props.command.type) {
		case "sync_build_id_from_response":
			syncRuntimeBuildIDIfChanged({
				nextBuildID: getBuildIDFromResponse(props.command.response),
			});
			return undefined;
		case "effectuate_redirect":
			return effectuateRedirectDataResult(props.command.redirectData, 0);
		case "auto_revalidate_navigation":
			return props.context.navigate({
				href: props.command.href,
				navigationType: "revalidation",
			});
	}
}

async function executeSubmitRuntimeCommandWithStaleOwnership<
	TSubmitResultData,
>(props: {
	context: SubmitExecutionContext;
	command: SubmitRuntimeCommand;
	getStaleSubmitResult: () => SubmitResult<TSubmitResultData> | null;
}): Promise<
	ExecuteSubmitRuntimeCommandWithStaleOwnershipResult<
		TSubmitResultData,
		unknown
	>
> {
	const staleBeforeCommand = props.getStaleSubmitResult();
	if (staleBeforeCommand) {
		return {
			type: "stale",
			result: staleBeforeCommand,
		};
	}
	const commandResult = await executeSubmitRuntimeCommand({
		context: props.context,
		command: props.command,
	});
	const staleAfterCommand = props.getStaleSubmitResult();
	if (staleAfterCommand) {
		return {
			type: "stale",
			result: staleAfterCommand,
		};
	}
	return {
		type: "executed",
		result: commandResult,
	};
}

async function executeSubmitRuntimeCommandListWithStaleOwnership<
	TSubmitResultData,
>(props: {
	context: SubmitExecutionContext;
	commands: SubmitRuntimeCommand[];
	getStaleSubmitResult: () => SubmitResult<TSubmitResultData> | null;
}): Promise<
	ExecuteSubmitRuntimeCommandListWithStaleOwnershipResult<TSubmitResultData>
> {
	const commandExecutionState: {
		redirectResult:
			| Awaited<ReturnType<typeof effectuateRedirectDataResult>>
			| undefined;
		staleResult: SubmitResult<TSubmitResultData> | null;
	} = {
		redirectResult: undefined,
		staleResult: null,
	};

	await executeAsynchronousRuntimeCommandList({
		commands: props.commands,
		executeCommand: async ({ command }) => {
			if (commandExecutionState.staleResult) {
				return {
					shouldStop: true,
				};
			}

			const commandExecutionResult =
				await executeSubmitRuntimeCommandWithStaleOwnership({
					context: props.context,
					command,
					getStaleSubmitResult: props.getStaleSubmitResult,
				});
			if (commandExecutionResult.type === "stale") {
				commandExecutionState.staleResult =
					commandExecutionResult.result;
				return {
					shouldStop: true,
				};
			}
			if (command.type === "effectuate_redirect") {
				commandExecutionState.redirectResult =
					commandExecutionResult.result as Awaited<
						ReturnType<typeof effectuateRedirectDataResult>
					>;
			}
			return;
		},
	});

	if (commandExecutionState.staleResult) {
		return {
			type: "stale",
			result: commandExecutionState.staleResult,
		};
	}

	return {
		type: "executed",
		redirectResult: commandExecutionState.redirectResult,
	};
}

export async function executeSubmitRuntime<T = unknown>(
	context: SubmitExecutionContext,
	url: string | URL,
	requestInit?: RequestInit,
	options?: SubmitOptions,
): Promise<SubmitResult<T>> {
	const submissionLifecycle = createSubmissionLifecycle(context, options);
	submissionLifecycle.begin();
	let submitRuntimeReducerState = createInitialSubmitRuntimeReducerState();
	const getStaleSubmitResult = (): SubmitResult<T> | null =>
		getStaleSubmitResultIfNotCurrent<T>({
			isSubmissionCurrent: submissionLifecycle.isCurrent,
		});

	try {
		const submitRequestURL = new URL(resolveAbsoluteHref({ href: url }));
		assertProgrammaticSameOriginOrThrow({
			absoluteHref: submitRequestURL.href,
			apiName: "submit(...)",
		});
		const submitRequestHeaders = new Headers(requestInit?.headers);
		const deploymentID = __vormaClientGlobal.get("deploymentID");
		if (deploymentID) {
			submitRequestHeaders.set("x-deployment-id", deploymentID);
		}
		const submitRequestInit: RequestInit = {
			...requestInit,
			headers: submitRequestHeaders,
			signal: submissionLifecycle.abortController.signal,
		};

		const submitRequestResult = await handleRedirects({
			abortController: submissionLifecycle.abortController,
			url: submitRequestURL,
			redirectCount: 0,
			requestInit: submitRequestInit,
		});
		if (!submitRequestResult.response) {
			throw new Error("Submit request completed without a response.");
		}
		const { redirectData, response } = submitRequestResult;

		const requestResolvedTransition = reduceSubmitRuntimeEvent({
			state: submitRuntimeReducerState,
			event: {
				type: "request_resolved",
				response,
				staleSubmitResult: getStaleSubmitResult(),
			},
		});
		submitRuntimeReducerState = requestResolvedTransition.state;
		if (requestResolvedTransition.terminalResult.type === "stale") {
			return requestResolvedTransition.terminalResult.result;
		}

		const requestCommandExecutionResult =
			await executeSubmitRuntimeCommandListWithStaleOwnership({
				context,
				commands: requestResolvedTransition.commands,
				getStaleSubmitResult,
			});
		if (requestCommandExecutionResult.type === "stale") {
			return requestCommandExecutionResult.result;
		}

		const responseClassifiedTransition = reduceSubmitRuntimeEvent({
			state: submitRuntimeReducerState,
			event: {
				type: "response_classified",
				response,
				redirectData,
				requestInit,
				options,
				currentHref: window.location.href,
				staleSubmitResult: getStaleSubmitResult(),
			},
		});
		submitRuntimeReducerState = responseClassifiedTransition.state;
		if (responseClassifiedTransition.terminalResult.type === "stale") {
			return responseClassifiedTransition.terminalResult.result;
		}
		const staleAfterResponseClassification = getStaleSubmitResult();
		if (staleAfterResponseClassification) {
			return staleAfterResponseClassification;
		}
		if (responseClassifiedTransition.terminalResult.type === "error") {
			return getSubmitErrorResult<T>(
				responseClassifiedTransition.terminalResult.error,
			);
		}
		const postClassificationCommandPlan =
			responseClassifiedTransition.postClassificationCommandPlan;
		if (!postClassificationCommandPlan) {
			throw new Error(
				"Submit runtime reducer did not produce a post-classification command plan.",
			);
		}

		let submitData: T | undefined;
		if (responseClassifiedTransition.shouldReadSuccessPayload) {
			submitData = (await readSubmitSuccessResponseData(response)) as T;
			const successPayloadParsedTransition = reduceSubmitRuntimeEvent({
				state: submitRuntimeReducerState,
				event: {
					type: "success_payload_parsed",
					staleSubmitResult: getStaleSubmitResult(),
				},
			});
			submitRuntimeReducerState = successPayloadParsedTransition.state;
			if (
				successPayloadParsedTransition.terminalResult.type === "stale"
			) {
				return successPayloadParsedTransition.terminalResult.result;
			}
			if (
				successPayloadParsedTransition.terminalResult.type === "error"
			) {
				return getSubmitErrorResult<T>(
					successPayloadParsedTransition.terminalResult.error,
				);
			}
		}

		const postClassificationCommandExecutionResult =
			await executeSubmitRuntimeCommandListWithStaleOwnership({
				context,
				commands: postClassificationCommandPlan.commands,
				getStaleSubmitResult,
			});
		if (postClassificationCommandExecutionResult.type === "stale") {
			return postClassificationCommandExecutionResult.result;
		}

		if (postClassificationCommandPlan.terminalResult.type === "redirect") {
			const redirectResult =
				postClassificationCommandExecutionResult.redirectResult;
			if (!redirectResult || redirectResult.status !== "did") {
				return getSubmitErrorResult<T>("Redirect failed");
			}
			return { success: true, data: undefined as T };
		}

		return { success: true, data: submitData as T };
	} catch (error) {
		return getSubmitRuntimeErrorResult<T>({
			error,
			abortSignal: submissionLifecycle.abortController.signal,
		});
	} finally {
		submissionLifecycle.finish();
	}
}

/////////////////////////////////////////////////////////////////////
/////// Navigation Runtime Composition
/////////////////////////////////////////////////////////////////////

function commitSameDocumentHashNavigationWithoutServerFetch(props: {
	navigationProps: NavigateProps;
	targetUrl: string;
}): { didNavigate: boolean } {
	const { navigationProps, targetUrl } = props;
	const history = HistoryManager.getInstance();
	saveScrollState();
	const isSameLocation =
		classifyNavigationTargetAgainstCurrentLocation({
			targetHref: targetUrl,
			currentHref: window.location.href,
		}) === "same-document-noop";
	if (!isSameLocation && !navigationProps.replace) {
		history.push(targetUrl, navigationProps.state);
	} else {
		history.replace(targetUrl, navigationProps.state);
	}

	const hash = hashFragmentFromHref(targetUrl);
	dispatchRouteChangeEvent({
		__scrollState: hash
			? { hash }
			: navigationProps.scrollToTop !== false
				? { x: 0, y: 0 }
				: undefined,
	});

	return { didNavigate: true };
}

/**
 * Creates the client navigation runtime that coordinates active navigation,
 * prefetch, revalidation, and submissions over shared slot state.
 */
export function createNavigationRuntime(
	options: CreateNavigationRuntimeOptions = {},
): NavigationStateManager {
	const { onNavigationIntentResolved } = options;

	const lanes = createRuntimeLanes();
	let nextNavigationOperationID = 1;
	let nextSubmissionOperationID = 1;
	let runtimeEngineState = createInitialNavigationRuntimeEngineState();

	function getStatus(): StatusEventDetail {
		return computeNavigationStatus({
			runtimeEngineState,
			submissions: lanes.submissions,
		});
	}
	const scheduleStatusUpdate = createStatusSignaler({
		getStatus,
		dispatchStatusEvent,
	});

	const getActiveNavigation = (): NavigationEntry | null => lanes.active;
	const setActiveNavigation = (entry: NavigationEntry | null): void => {
		lanes.active = entry;
	};

	const getRevalidationNavigation = (): NavigationEntry | null =>
		lanes.revalidation;
	const setRevalidationNavigation = (entry: NavigationEntry | null): void => {
		lanes.revalidation = entry;
	};
	const findNavigationEntry = (
		targetUrl: string,
	): NavigationEntry | undefined =>
		matchNavigationLaneByTargetURL({
			lanes,
			targetUrl,
		})?.entry;
	const deleteNavigationFromLanes = (props: {
		targetUrl: string;
		reason: string;
		causedByOperationID?: number | null;
	}): boolean => {
		void props.causedByOperationID;
		findNavigationEntry(props.targetUrl)?.control.abortController?.abort(
			props.reason,
		);
		return deleteNavigationFromNavigationLanes({
			lanes,
			targetUrl: props.targetUrl,
			onStatusRelevantChange: scheduleStatusUpdate,
		});
	};
	const deleteNavigation = (props: {
		targetUrl: string;
		reason: string;
		causedByOperationID?: number | null;
	}): boolean => {
		const transition = dispatchRuntimeEngineEvent({
			type: "remove-navigation-requested",
			targetUrl: props.targetUrl,
			reason: props.reason,
			causedByOperationID: props.causedByOperationID,
		});
		return transition.commands.some(
			(command) => command.type === "cleanup-entry",
		);
	};
	const transitionPhase = (props: {
		targetUrl: string;
		phase: NavigationPhase;
		reason: string;
	}): void => {
		void props.reason;
		const entry = findNavigationEntry(props.targetUrl);
		if (!entry) {
			return;
		}

		if (props.phase === "rendering" || props.phase === "complete") {
			dispatchRuntimeEngineEvent({
				type:
					props.phase === "rendering"
						? "wait-resolve"
						: "render-finish",
				lane: resolveNavigationRuntimeEngineLaneForNavigationEntry({
					entry,
				}),
				targetUrl: entry.targetUrl,
				operationID: entry.operationID,
			});
		}
	};

	const isNavigationOperationCurrent = (props: {
		lane: "navigate" | "prefetch" | "revalidate";
		targetUrl: string;
		operationID: number | null;
	}): boolean => {
		if (props.operationID === null) {
			return true;
		}
		const ownedEntry = resolveOwnedOperationEntry({
			entry: findNavigationEntry(props.targetUrl),
			expectedOperationID: props.operationID,
		});
		if (!ownedEntry) {
			return false;
		}

		if (props.lane === "prefetch") {
			return ownedEntry.type === "prefetch";
		}
		if (props.lane === "revalidate") {
			return ownedEntry.type === "revalidation";
		}
		return (
			ownedEntry.type !== "prefetch" && ownedEntry.type !== "revalidation"
		);
	};

	const assertRuntimeSlotStateMatchesRuntimeEngineState = (): void => {
		const navigateLane = runtimeEngineState.lanes.navigate;
		const activeNavigation = lanes.active;
		const doesEngineExpectActiveNavigation =
			navigateLane.ownership === "current" &&
			navigateLane.phase !== "idle" &&
			navigateLane.phase !== "complete";
		if (doesEngineExpectActiveNavigation) {
			if (!activeNavigation) {
				throw new Error(
					"Navigation runtime invariant violated: engine navigate lane is current but active navigation slot is empty.",
				);
			}
			if (activeNavigation.operationID !== navigateLane.operationID) {
				throw new Error(
					"Navigation runtime invariant violated: active navigation operation does not match engine navigate lane operation.",
				);
			}
			if (
				navigateLane.targetUrl &&
				!hasSameDataTarget({
					firstHref: activeNavigation.targetUrl,
					secondHref: navigateLane.targetUrl,
				})
			) {
				throw new Error(
					"Navigation runtime invariant violated: active navigation target does not match engine navigate lane target.",
				);
			}
		}

		const revalidateLane = runtimeEngineState.lanes.revalidate;
		const revalidationNavigation = lanes.revalidation;
		const doesEngineExpectRevalidationNavigation =
			revalidateLane.ownership === "current" &&
			revalidateLane.phase !== "idle" &&
			revalidateLane.phase !== "complete";
		if (doesEngineExpectRevalidationNavigation) {
			if (!revalidationNavigation) {
				throw new Error(
					"Navigation runtime invariant violated: engine revalidate lane is current but revalidation slot is empty.",
				);
			}
			if (
				revalidationNavigation.operationID !==
				revalidateLane.operationID
			) {
				throw new Error(
					"Navigation runtime invariant violated: revalidation navigation operation does not match engine revalidate lane operation.",
				);
			}
			if (
				revalidateLane.targetUrl &&
				!hasSameDataTarget({
					firstHref: revalidationNavigation.targetUrl,
					secondHref: revalidateLane.targetUrl,
				})
			) {
				throw new Error(
					"Navigation runtime invariant violated: revalidation navigation target does not match engine revalidate lane target.",
				);
			}
		}
	};

	const dispatchRuntimeEngineEvent = (
		event: NavigationRuntimeEngineEvent,
	) => {
		const previousStatus = getStatus();
		const transition = reduceNavigationRuntimeEngineEvent({
			state: runtimeEngineState,
			event,
		});
		runtimeEngineState = transition.state;
		executeNavigationRuntimeEngineCommands({
			commands: transition.commands,
			context: {
				clearQueuedRevalidationRequest: () => {
					deterministicRevalidationLane.clearQueuedTrailingRequest();
				},
				commitSameDocumentHashNavigationWithoutFetch: ({
					navigationProps,
					targetUrl,
				}) => {
					commitSameDocumentHashNavigationWithoutServerFetch({
						navigationProps,
						targetUrl,
					});
				},
				cleanupNavigationEntry: ({
					targetUrl,
					reason,
					causedByOperationID,
				}) => {
					findNavigationEntry(
						targetUrl,
					)?.control.abortController?.abort(reason);
					deleteNavigationFromLanes({
						targetUrl,
						reason,
						causedByOperationID,
					});
				},
				resetRevalidationLane: () => {
					deterministicRevalidationLane.reset();
				},
				clearRuntimeLanes: () => {
					clearRuntimeLanes({
						lanes,
						onStatusRelevantChange: scheduleStatusUpdate,
					});
				},
				isNavigationOperationCurrent,
			},
		});
		assertRuntimeSlotStateMatchesRuntimeEngineState();
		const nextStatus = getStatus();
		if (!jsonDeepEquals(previousStatus, nextStatus)) {
			scheduleStatusUpdate();
		}

		return transition;
	};

	const deterministicRevalidationLane = createDeterministicRevalidationLane({
		getCurrentHref: () => window.location.href,
		getHasInFlightPass: () => {
			const revalidationPhase = runtimeEngineState.lanes.revalidate.phase;
			return (
				revalidationPhase !== "idle" && revalidationPhase !== "complete"
			);
		},
		getInFlightTargetUrl: () =>
			runtimeEngineState.lanes.revalidate.targetUrl,
		onInFlightTargetMismatch: () => {
			dispatchRuntimeEngineEvent({
				type: "external-location-change",
				currentHref: window.location.href,
			});
		},
	});

	const removeNavigation = (targetUrl: string): void => {
		deleteNavigation({
			targetUrl,
			reason: "remove_navigation",
		});
	};

	const getNavigation = findNavigationEntry;
	const hasNavigation = (targetUrl: string): boolean =>
		findNavigationEntry(targetUrl) !== undefined;
	const getNavigationsSize = (): number =>
		getNavigationsSizeFromNavigationLanes({
			lanes,
		});
	const getNavigations = (): Map<string, NavigationEntry> =>
		buildNavigationsMapFromNavigationLanes({
			lanes,
		});

	const beginNavigationContext: BeginNavigationContext = {
		getActiveNavigation,
		setActiveNavigation,
		getRevalidationNavigation,
		setRevalidationNavigation,
		prefetchNavigationsByTargetUrl: lanes.prefetch,
		scheduleStatusUpdate,
		fetchRouteData,
		deleteNavigation: ({ targetUrl, reason, causedByOperationID }) =>
			deleteNavigation({
				targetUrl,
				reason,
				causedByOperationID,
			}),
		allocateNavigationOperationID: () => {
			const operationID = nextNavigationOperationID;
			nextNavigationOperationID += 1;
			return operationID;
		},
	};

	const processSuccessfulNavigation = async (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	): Promise<void> =>
		processSuccessfulNavigationRuntime(
			{
				transitionPhase,
				findNavigationEntry,
				deleteNavigation,
				onSuccessfulNavigationCommitted: ({
					entry: committedEntry,
				}): void => {
					if (
						committedEntry.intent === "navigate" ||
						committedEntry.intent === "revalidate"
					) {
						onNavigationIntentResolved?.();
					}
				},
			},
			outcome,
			entry,
		);

	const beginNavigation = (props: NavigateProps) => {
		const targetUrl = resolveBeginNavigationTargetURL({
			navigationProps: props,
			currentHref: window.location.href,
		});
		const control = executeBeginNavigation(beginNavigationContext, props);
		dispatchRuntimeEngineEvent({
			type: "start",
			lane: resolveNavigationRuntimeEngineLaneForNavigateProps({
				navigationProps: props,
			}),
			navigationProps: props,
			targetUrl,
			currentHref: window.location.href,
			operationID: control.operationID,
		});
		return control;
	};

	const navigateSinglePass = async (
		props: NavigateProps,
	): Promise<{ didNavigate: boolean }> =>
		executeNavigationSinglePass({
			navigationProps: props,
			beginNavigation,
			findNavigationEntry,
			deleteNavigation,
			processSuccessfulNavigation,
			onLifecycleEvent: (eventProps) => {
				const lane = resolveNavigationRuntimeEngineLaneForNavigateProps(
					{
						navigationProps: eventProps.navigationProps,
					},
				);
				switch (eventProps.type) {
					case "fetch-resolve":
						dispatchRuntimeEngineEvent({
							type: "fetch-resolve",
							lane,
							targetUrl: eventProps.targetUrl,
							operationID: eventProps.operationID,
							outcomeType: eventProps.outcomeType,
						});
						return;
					case "redirect":
						dispatchRuntimeEngineEvent({
							type: "redirect",
							lane,
							targetUrl: eventProps.targetUrl,
							operationID: eventProps.operationID,
						});
						return;
					case "wait-resolve":
						dispatchRuntimeEngineEvent({
							type: "wait-resolve",
							lane,
							targetUrl: eventProps.targetUrl,
							operationID: eventProps.operationID,
						});
						return;
					case "render-finish":
						dispatchRuntimeEngineEvent({
							type: "render-finish",
							lane,
							targetUrl: eventProps.targetUrl,
							operationID: eventProps.operationID,
						});
						return;
					case "abort":
						dispatchRuntimeEngineEvent({
							type: "abort",
							lane,
							targetUrl: eventProps.targetUrl,
							operationID: eventProps.operationID,
							reason: eventProps.reason,
						});
						return;
				}
			},
			onNavigationPromiseRejected: ({ targetUrl, ownedEntry }) => {
				void targetUrl;
				void ownedEntry;
			},
		});

	const navigate = async (
		props: NavigateProps,
	): Promise<{ didNavigate: boolean }> => {
		const targetUrl = resolveBeginNavigationTargetURL({
			navigationProps: props,
			currentHref: window.location.href,
		});
		const runtimeEngineTransition = dispatchRuntimeEngineEvent({
			type: "start",
			lane: resolveNavigationRuntimeEngineLaneForNavigateProps({
				navigationProps: props,
			}),
			navigationProps: props,
			targetUrl,
			currentHref: window.location.href,
		});

		switch (runtimeEngineTransition.terminalResult.type) {
			case "return-without-fetch":
				return {
					didNavigate:
						runtimeEngineTransition.terminalResult.didNavigate,
				};
			case "run-single-pass":
				return navigateSinglePass(props);
			case "run-revalidation-lane":
				return deterministicRevalidationLane.runRevalidation({
					navigateSinglePass,
				});
			case "none":
				return {
					didNavigate: false,
				};
		}
	};

	const submit = <T = unknown>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: SubmitOptions,
	): Promise<SubmitResult<T>> =>
		executeSubmitRuntime(
			{
				submissions: lanes.submissions,
				scheduleStatusUpdate,
				allocateSubmissionOperationID: () => {
					const operationID = nextSubmissionOperationID;
					nextSubmissionOperationID += 1;
					return operationID;
				},
				onSubmissionStateTransition: ({
					submissionEntry,
					fromState,
					toState,
					reason,
					causedByOperationID,
				}) => {
					dispatchRuntimeEngineEvent({
						type: "submission-state-transitioned",
						operationID: submissionEntry.operationID,
						targetUrl: window.location.href,
						fromState,
						toState,
						reason,
						causedByOperationID,
					});
				},
				navigate,
			},
			url,
			requestInit,
			options,
		);

	function clearAll(): void {
		dispatchRuntimeEngineEvent({
			type: "clear-all-requested",
			targetUrl: window.location.href,
		});
	}

	return {
		_submissions: lanes.submissions,
		navigate,
		beginNavigation,
		processSuccessfulNavigation,
		submit,
		removeNavigation,
		getNavigation,
		hasNavigation,
		getNavigationsSize,
		getNavigations,
		getStatus,
		clearAll,
	};
}

/////////////////////////////////////////////////////////////////////
/////// Global Runtime Snapshot Store
/////////////////////////////////////////////////////////////////////

export const runtimeRouteSnapshotFieldKeys = [
	"outermostServerError",
	"outermostServerErrorIdx",
	...REQUIRED_RUNTIME_ROUTE_DATA_KEYS,
	"outermostClientError",
	"outermostClientErrorIdx",
	"outermostError",
	"outermostErrorIdx",
	"buildID",
	"rootElementID",
	"activeComponents",
	"activeErrorBoundary",
	"clientLoadersData",
] as const satisfies ReadonlyArray<keyof RuntimeRouteSnapshot>;

const runtimeRouteSnapshotCanonicalizableFieldKeys = [
	"matchedPatterns",
	"loadersData",
	"importURLs",
	"exportKeys",
	"errorExportKeys",
	"params",
	"splatValues",
	"activeComponents",
	"clientLoadersData",
] as const satisfies ReadonlyArray<keyof RuntimeRouteSnapshot>;
type RuntimeRouteSnapshotCanonicalizableFieldKey =
	(typeof runtimeRouteSnapshotCanonicalizableFieldKeys)[number];
const runtimeRouteSnapshotNullishDefaultValueByFieldKey: {
	[K in RuntimeRouteSnapshotCanonicalizableFieldKey]: () => RuntimeRouteSnapshot[K];
} = {
	matchedPatterns: () => [],
	loadersData: () => [],
	importURLs: () => [],
	exportKeys: () => [],
	errorExportKeys: () => [],
	params: () => ({}),
	splatValues: () => [],
	activeComponents: () => null,
	clientLoadersData: () => [],
};

function setRuntimeRouteSnapshotCanonicalizableFieldValue<
	Key extends RuntimeRouteSnapshotCanonicalizableFieldKey,
>(props: {
	snapshot: RuntimeRouteSnapshot;
	fieldKey: Key;
	value: RuntimeRouteSnapshot[Key];
}): void {
	props.snapshot[props.fieldKey] = props.value;
}

/**
 * Process-wide symbol key used to store runtime state on globalThis.
 * Vorma currently supports one runtime app/version per browser window.
 */
export const VORMA_SYMBOL = Symbol.for("__vorma_internal__");
const vormaClientGlobalNonSnapshotKeys = [
	"isDev",
	"viteDevURL",
	"publicPathPrefix",
	"isTouchInputModalityActive",
	"patternToWaitFnMap",
	"defaultErrorBoundary",
	"useViewTransitions",
	"deploymentID",
	"vormaAppConfig",
	"routeManifestURL",
	"routeManifest",
	"patternRegistry",
] as const satisfies ReadonlyArray<keyof VormaClientGlobalNonSnapshotState>;
const vormaClientGlobalNonSnapshotKeySet = new Set<string>(
	vormaClientGlobalNonSnapshotKeys,
);

function normalizeRuntimeRouteSnapshot(
	snapshot: RuntimeRouteSnapshot,
): RuntimeRouteSnapshot {
	const normalizedSnapshot: RuntimeRouteSnapshot = { ...snapshot };
	for (const fieldKey of runtimeRouteSnapshotCanonicalizableFieldKeys) {
		if (
			normalizedSnapshot[fieldKey] === undefined ||
			normalizedSnapshot[fieldKey] === null
		) {
			setRuntimeRouteSnapshotCanonicalizableFieldValue({
				snapshot: normalizedSnapshot,
				fieldKey,
				value: runtimeRouteSnapshotNullishDefaultValueByFieldKey[
					fieldKey
				](),
			});
		}
	}
	normalizedSnapshot.hasRootData = normalizedSnapshot.hasRootData === true;
	normalizedSnapshot.buildID = normalizedSnapshot.buildID || "";
	return normalizedSnapshot;
}

function getGlobalStateOrThrow(): VormaClientGlobal {
	const runtimeGlobalState =
		getDefaultVormaRuntimeContext().runtimeGlobalState;
	if (runtimeGlobalState) {
		return runtimeGlobalState;
	}
	const dangerousGlobalThis = globalThis as VormaGlobalThis;
	const maybeGlobalState = dangerousGlobalThis[VORMA_SYMBOL];
	if (typeof maybeGlobalState !== "object" || maybeGlobalState === null) {
		throw new Error(
			'Vorma client runtime state is not initialized on globalThis[Symbol.for("__vorma_internal__")]. Initialize runtime globals before calling Vorma client APIs.',
		);
	}
	return maybeGlobalState as VormaClientGlobal;
}

function isVormaClientGlobalNonSnapshotKey(
	key: unknown,
): key is VormaClientGlobalNonSnapshotKey {
	return (
		typeof key === "string" && vormaClientGlobalNonSnapshotKeySet.has(key)
	);
}

function formatVormaClientGlobalKeyForError(key: unknown): string {
	return typeof key === "string" ? key : String(key);
}

function canonicalizeRuntimeRouteSnapshotField<T>(props: {
	previousValue: T;
	nextValue: T;
}): T {
	if (jsonDeepEquals(props.previousValue, props.nextValue)) {
		return props.previousValue;
	}
	return props.nextValue;
}

function setCanonicalizedRuntimeRouteSnapshotField<
	Key extends RuntimeRouteSnapshotCanonicalizableFieldKey,
>(props: {
	canonicalizedSnapshot: RuntimeRouteSnapshot;
	previousSnapshot: RuntimeRouteSnapshot;
	nextSnapshot: RuntimeRouteSnapshot;
	fieldKey: Key;
}): void {
	const { canonicalizedSnapshot, previousSnapshot, nextSnapshot, fieldKey } =
		props;
	setRuntimeRouteSnapshotCanonicalizableFieldValue({
		snapshot: canonicalizedSnapshot,
		fieldKey,
		value: canonicalizeRuntimeRouteSnapshotField({
			previousValue: previousSnapshot[fieldKey],
			nextValue: nextSnapshot[fieldKey],
		}),
	});
}

function areRuntimeRouteSnapshotsReferenceEqual(props: {
	firstSnapshot: RuntimeRouteSnapshot;
	secondSnapshot: RuntimeRouteSnapshot;
}): boolean {
	const { firstSnapshot, secondSnapshot } = props;
	for (const snapshotFieldKey of runtimeRouteSnapshotFieldKeys) {
		if (
			!Object.is(
				firstSnapshot[snapshotFieldKey],
				secondSnapshot[snapshotFieldKey],
			)
		) {
			return false;
		}
	}
	return true;
}

function canonicalizeRuntimeRouteSnapshotAgainstPrevious(props: {
	previousSnapshot: RuntimeRouteSnapshot | undefined;
	nextSnapshot: RuntimeRouteSnapshot;
}): RuntimeRouteSnapshot {
	const { previousSnapshot, nextSnapshot } = props;
	if (!previousSnapshot) {
		return nextSnapshot;
	}

	const canonicalizedSnapshot: RuntimeRouteSnapshot = { ...nextSnapshot };
	for (const fieldKey of runtimeRouteSnapshotCanonicalizableFieldKeys) {
		setCanonicalizedRuntimeRouteSnapshotField({
			canonicalizedSnapshot,
			previousSnapshot,
			nextSnapshot,
			fieldKey,
		});
	}

	if (
		areRuntimeRouteSnapshotsReferenceEqual({
			firstSnapshot: previousSnapshot,
			secondSnapshot: canonicalizedSnapshot,
		})
	) {
		return previousSnapshot;
	}

	return canonicalizedSnapshot;
}

function freezeRuntimeRouteSnapshot(
	snapshot: RuntimeRouteSnapshot,
): RuntimeRouteSnapshot {
	for (const fieldKey of runtimeRouteSnapshotCanonicalizableFieldKeys) {
		const fieldValue = snapshot[fieldKey];
		if (fieldValue !== null) {
			Object.freeze(fieldValue);
		}
	}
	return Object.freeze(snapshot);
}

function finalizeRuntimeRouteSnapshot(props: {
	previousSnapshot: RuntimeRouteSnapshot | undefined;
	nextSnapshot: RuntimeRouteSnapshot;
}): RuntimeRouteSnapshot {
	const normalizedNextSnapshot = normalizeRuntimeRouteSnapshot(
		props.nextSnapshot,
	);
	const canonicalizedSnapshot =
		canonicalizeRuntimeRouteSnapshotAgainstPrevious({
			previousSnapshot: props.previousSnapshot,
			nextSnapshot: normalizedNextSnapshot,
		});
	if (
		props.previousSnapshot &&
		canonicalizedSnapshot === props.previousSnapshot
	) {
		return props.previousSnapshot;
	}
	if (Object.isFrozen(canonicalizedSnapshot)) {
		return canonicalizedSnapshot;
	}
	return freezeRuntimeRouteSnapshot(canonicalizedSnapshot);
}

function setRuntimeRouteSnapshotOnGlobalState(props: {
	globalState: VormaClientGlobal;
	nextSnapshot: RuntimeRouteSnapshot;
}): RuntimeRouteSnapshot {
	const previousSnapshot = props.globalState.runtimeRouteSnapshot;
	const finalizedSnapshot = finalizeRuntimeRouteSnapshot({
		previousSnapshot,
		nextSnapshot: props.nextSnapshot,
	});
	props.globalState.runtimeRouteSnapshot = finalizedSnapshot;
	return finalizedSnapshot;
}

/**
 * Returns typed `get`/`set` accessors for the shared global runtime state.
 */
export function __getVormaClientGlobal() {
	function get<K extends VormaClientGlobalGetKey>(
		key: K,
	): VormaClientGlobalGetValueForKey<K> {
		const globalState = getGlobalStateOrThrow();
		if (key === "runtimeRouteSnapshot") {
			return getRuntimeRouteSnapshot() as VormaClientGlobalGetValueForKey<K>;
		}
		if (isVormaClientGlobalNonSnapshotKey(key)) {
			return globalState[key] as VormaClientGlobalGetValueForKey<K>;
		}
		throw new Error(
			`Vorma client global get contract violated: unsupported key "${formatVormaClientGlobalKeyForError(key)}". Read route snapshot fields from "runtimeRouteSnapshot".`,
		);
	}

	function set<K extends VormaClientGlobalSetKey>(
		key: K,
		value: VormaClientGlobalSetValueForKey<K>,
	) {
		const globalState = getGlobalStateOrThrow();
		if (key === "runtimeRouteSnapshot") {
			const runtimeRouteSnapshotValue = value as RuntimeRouteSnapshot;
			setRuntimeRouteSnapshotOnGlobalState({
				globalState,
				nextSnapshot: runtimeRouteSnapshotValue,
			});
			return;
		}
		if (!isVormaClientGlobalNonSnapshotKey(key)) {
			throw new Error(
				`Vorma client global set contract violated: unsupported key "${formatVormaClientGlobalKeyForError(key)}". Write route snapshot fields through "runtimeRouteSnapshot".`,
			);
		}
		const nonSnapshotKey = key as VormaClientGlobalNonSnapshotKey;
		const nonSnapshotState = globalState as Record<string, unknown>;
		nonSnapshotState[nonSnapshotKey] = value as unknown;
	}
	return { get, set };
}

export const __vormaClientGlobal = __getVormaClientGlobal();

export function getRuntimeRouteSnapshot(): RuntimeRouteSnapshot {
	const globalState = getGlobalStateOrThrow();
	const existingSnapshot = globalState.runtimeRouteSnapshot;
	if (!existingSnapshot) {
		throw new Error(
			'Vorma runtime invariant violated: global runtime route snapshot is missing at globalThis[Symbol.for("__vorma_internal__")].runtimeRouteSnapshot.',
		);
	}
	if (Object.isFrozen(existingSnapshot)) {
		return existingSnapshot;
	}
	return setRuntimeRouteSnapshotOnGlobalState({
		globalState,
		nextSnapshot: existingSnapshot,
	});
}

export function setRuntimeRouteSnapshot(
	nextSnapshot: RuntimeRouteSnapshot,
): RuntimeRouteSnapshot {
	return setRuntimeRouteSnapshotOnGlobalState({
		globalState: getGlobalStateOrThrow(),
		nextSnapshot,
	});
}

export function updateRuntimeRouteSnapshot(props: {
	updater: (snapshot: RuntimeRouteSnapshot) => RuntimeRouteSnapshot;
}): RuntimeRouteSnapshot {
	const previousSnapshot = getRuntimeRouteSnapshot();
	return setRuntimeRouteSnapshot(props.updater(previousSnapshot));
}

export function setRuntimeBuildID(props: {
	buildID: string;
}): RuntimeRouteSnapshot {
	return updateRuntimeRouteSnapshot({
		updater: (runtimeRouteSnapshot) => ({
			...runtimeRouteSnapshot,
			buildID: props.buildID,
		}),
	});
}

type RouterDataSnapshotView = {
	buildID: string;
	matchedPatterns: string[];
	splatValues: string[];
	params: Record<string, string>;
	rootData: unknown;
};

let latestRouterDataSnapshot: RuntimeRouteSnapshot | undefined;
let latestRouterDataSnapshotView: RouterDataSnapshotView | undefined;

function buildRouterDataSnapshotView(
	runtimeRouteSnapshot: RuntimeRouteSnapshot,
): RouterDataSnapshotView {
	const rootData = runtimeRouteSnapshot.hasRootData
		? runtimeRouteSnapshot.loadersData[0]
		: null;
	return {
		buildID: runtimeRouteSnapshot.buildID,
		matchedPatterns: runtimeRouteSnapshot.matchedPatterns,
		splatValues: runtimeRouteSnapshot.splatValues,
		params: runtimeRouteSnapshot.params,
		rootData,
	};
}

/**
 * Returns router data snapshot for application consumption.
 */
export function getRouterData<
	T = unknown,
	P extends Record<string, string> = Record<string, string>,
>() {
	const runtimeRouteSnapshot = getRuntimeRouteSnapshot();
	if (
		!latestRouterDataSnapshotView ||
		latestRouterDataSnapshot !== runtimeRouteSnapshot
	) {
		latestRouterDataSnapshot = runtimeRouteSnapshot;
		latestRouterDataSnapshotView =
			buildRouterDataSnapshotView(runtimeRouteSnapshot);
	}
	return latestRouterDataSnapshotView as {
		buildID: string;
		matchedPatterns: string[];
		splatValues: string[];
		params: P;
		rootData: T;
	};
}

/**
 * Returns render-state fields required by route outlet runtime reconciliation.
 */
export function getClientRuntimeRenderState(): ClientRuntimeRenderState {
	const runtimeRouteSnapshot = getRuntimeRouteSnapshot();
	return {
		loadersData: runtimeRouteSnapshot.loadersData,
		clientLoadersData: runtimeRouteSnapshot.clientLoadersData,
		outermostError: runtimeRouteSnapshot.outermostError,
		outermostErrorIdx: runtimeRouteSnapshot.outermostErrorIdx,
		activeComponents: runtimeRouteSnapshot.activeComponents,
		activeErrorBoundary: runtimeRouteSnapshot.activeErrorBoundary,
		importURLs: runtimeRouteSnapshot.importURLs,
		exportKeys: runtimeRouteSnapshot.exportKeys,
	};
}

/**
 * Registers a client loader wait function for a route pattern.
 */
export function setClientLoaderWaitFn(
	pattern: string,
	waitFn: PatternWaitFn,
): void {
	const currentMap = __vormaClientGlobal.get("patternToWaitFnMap") || {};
	__vormaClientGlobal.set("patternToWaitFnMap", {
		...currentMap,
		[pattern]: waitFn,
	});
}

/////////////////////////////////////////////////////////////////////
/////// Navigation Runtime Access Bridge
/////////////////////////////////////////////////////////////////////

/**
 * Sets the shared navigation runtime access object.
 */
export function setNavigationStateAccess(access: NavigationStateAccess): void {
	getDefaultVormaRuntimeContext().navigationStateAccess = access;
}

/**
 * Returns the shared navigation runtime access object.
 */
export function getNavigationStateAccess(): NavigationStateAccess {
	const navigationStateAccess =
		getDefaultVormaRuntimeContext().navigationStateAccess;
	if (!navigationStateAccess) {
		throw new Error("Navigation state access has not been initialized.");
	}
	return navigationStateAccess;
}

/////////////////////////////////////////////////////////////////////
/////// Runtime Location And Revalidation Timestamp
/////////////////////////////////////////////////////////////////////

/**
 * Returns a normalized browser location snapshot consumed by client adapters.
 */
export function getRuntimeLocationState(): RouteOutletLocationState {
	return {
		pathname: window.location.pathname,
		search: window.location.search,
		hash: window.location.hash,
		state: HistoryManager.getInstance().location.state,
	};
}

export function reduceRevalidationTriggerTimestampState(props: {
	state: RevalidationTriggerTimestampState;
	event: RevalidationTriggerTimestampEvent;
}): RevalidationTriggerTimestampState {
	switch (props.event.type) {
		case "navigation_or_revalidation_intent_committed":
			return {
				...props.state,
				lastTriggeredNavOrRevalidateTimestampMS:
					props.event.committedTimestampMS,
			};
	}
}

export function createRevalidationTriggerTimestampRuntime(props?: {
	getNowTimestampMS?: () => number;
	initialState?: RevalidationTriggerTimestampState;
}): RevalidationTriggerTimestampRuntime {
	const getNowTimestampMS = props?.getNowTimestampMS ?? (() => Date.now());
	let state: RevalidationTriggerTimestampState = props?.initialState ?? {
		lastTriggeredNavOrRevalidateTimestampMS: getNowTimestampMS(),
	};

	return {
		recordNavigationOrRevalidationIntentCommitted: () => {
			state = reduceRevalidationTriggerTimestampState({
				state,
				event: {
					type: "navigation_or_revalidation_intent_committed",
					committedTimestampMS: getNowTimestampMS(),
				},
			});
		},
		getLastTriggeredNavOrRevalidateTimestampMS: () =>
			state.lastTriggeredNavOrRevalidateTimestampMS,
	};
}

export function createVormaRuntimeContext(): VormaRuntimeContext {
	return {
		history: {
			instance: undefined,
			lastKnownLocation: undefined,
			cleanupListener: null,
			latestListenerSequenceIssued: 0,
			listenerProcessingTail: Promise.resolve(),
		},
		navigationStateAccess: null,
		navigationStateManager: null,
		revalidationTriggerTimestampRuntime:
			createRevalidationTriggerTimestampRuntime(),
		runtimeGlobalState: null,
	};
}

let defaultVormaRuntimeContext: VormaRuntimeContext | null = null;

export function getDefaultVormaRuntimeContext(): VormaRuntimeContext {
	if (!defaultVormaRuntimeContext) {
		defaultVormaRuntimeContext = createVormaRuntimeContext();
	}
	return defaultVormaRuntimeContext;
}

export function setDefaultVormaRuntimeContext(
	context: VormaRuntimeContext,
): void {
	if (!context.navigationStateAccess && context.navigationStateManager) {
		context.navigationStateAccess = context.navigationStateManager;
	}
	if (
		defaultVormaRuntimeContext &&
		defaultVormaRuntimeContext !== context &&
		defaultVormaRuntimeContext.history.cleanupListener
	) {
		defaultVormaRuntimeContext.history.cleanupListener();
		defaultVormaRuntimeContext.history.cleanupListener = null;
	}
	defaultVormaRuntimeContext = context;
}

export function setRuntimeGlobalStateForDefaultContext(
	runtimeGlobalState: VormaClientGlobal | null,
): void {
	getDefaultVormaRuntimeContext().runtimeGlobalState = runtimeGlobalState;
}

function getNavigationStateManagerForContext(
	context: VormaRuntimeContext,
): NavigationStateManager {
	if (context.navigationStateManager) {
		return context.navigationStateManager;
	}

	context.navigationStateManager = createNavigationRuntime({
		onNavigationIntentResolved: () => {
			context.revalidationTriggerTimestampRuntime.recordNavigationOrRevalidationIntentCommitted();
		},
	});
	context.navigationStateAccess = context.navigationStateManager;
	return context.navigationStateManager;
}

function getNavigationStateManager(): NavigationStateManager {
	return getNavigationStateManagerForContext(getDefaultVormaRuntimeContext());
}

/**
 * Ensures the singleton navigation runtime is initialized.
 */
export function ensureNavigationRuntimeInitialized(): void {
	getNavigationStateManager();
}

// Global singleton runtime proxy that defers concrete runtime construction
// until the first method/property access.
export const navigationStateManager: NavigationStateManager = {
	get _submissions() {
		return getNavigationStateManager()._submissions;
	},
	navigate(props: NavigateProps): Promise<{ didNavigate: boolean }> {
		return getNavigationStateManager().navigate(props);
	},
	beginNavigation(props: NavigateProps): NavigationControl {
		return getNavigationStateManager().beginNavigation(props);
	},
	processSuccessfulNavigation(
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	): Promise<void> {
		return getNavigationStateManager().processSuccessfulNavigation(
			outcome,
			entry,
		);
	},
	submit<T = unknown>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: SubmitOptions,
	): Promise<SubmitResult<T>> {
		return getNavigationStateManager().submit<T>(url, requestInit, options);
	},
	removeNavigation(targetUrl: string): void {
		getNavigationStateManager().removeNavigation(targetUrl);
	},
	getNavigation(targetUrl: string): NavigationEntry | undefined {
		return getNavigationStateManager().getNavigation(targetUrl);
	},
	hasNavigation(targetUrl: string): boolean {
		return getNavigationStateManager().hasNavigation(targetUrl);
	},
	getNavigationsSize(): number {
		return getNavigationStateManager().getNavigationsSize();
	},
	getNavigations(): Map<string, NavigationEntry> {
		return getNavigationStateManager().getNavigations();
	},
	getStatus(): StatusEventDetail {
		return getNavigationStateManager().getStatus();
	},
	clearAll(): void {
		getNavigationStateManager().clearAll();
	},
};

/////////////////////////////////////////////////////////////////////
/////// Public Client API
/////////////////////////////////////////////////////////////////////

/**
 * Navigates to a route and drives the full client navigation lifecycle.
 */
export async function vormaNavigate(
	href: string,
	options?: {
		replace?: boolean;
		scrollToTop?: boolean;
		search?: string;
		hash?: string;
		state?: unknown;
	},
): Promise<void> {
	const targetHref = resolveAbsoluteHrefWithOptionalSearchAndHash({
		href,
		search: options?.search,
		hash: options?.hash,
	});
	assertProgrammaticSameOriginOrThrow({
		absoluteHref: targetHref,
		apiName: "vormaNavigate(...)",
	});

	await navigationStateManager.navigate({
		href: targetHref,
		navigationType: "userNavigation",
		replace: options?.replace,
		scrollToTop: options?.scrollToTop,
		state: options?.state,
	});
}

/**
 * Returns the last navigation/revalidation trigger timestamp used by client
 * state machines to reason about staleness.
 */
export function getLastTriggeredNavOrRevalidateTimestampMS(): number {
	return getDefaultVormaRuntimeContext().revalidationTriggerTimestampRuntime.getLastTriggeredNavOrRevalidateTimestampMS();
}

/**
 * Revalidates the current location without changing history.
 */
export async function revalidate() {
	await navigationStateManager.navigate({
		href: window.location.href,
		navigationType: "revalidation",
	});
}

/**
 * Submits an action request through the shared navigation runtime.
 */
export async function submit<T = unknown>(
	url: string | URL,
	requestInit?: RequestInit,
	options?: SubmitOptions,
): Promise<SubmitResult<T>> {
	assertProgrammaticSameOriginOrThrow({
		absoluteHref: String(url),
		apiName: "submit(...)",
	});
	return navigationStateManager.submit(url, requestInit, options);
}

/**
 * Starts a controllable navigation lifecycle operation.
 */
export function beginNavigationFromClientRuntime(
	props: NavigateProps,
): NavigationControl {
	return navigationStateManager.beginNavigation(props);
}

/**
 * Returns current navigation status event detail.
 */
export function getStatus(): StatusEventDetail {
	return navigationStateManager.getStatus();
}

/**
 * Returns a normalized browser location snapshot used by adapters.
 */
export function getLocation() {
	return getRuntimeLocationState();
}

/**
 * Returns the current runtime build id.
 */
export function getBuildID(): string {
	return getRuntimeRouteSnapshot().buildID;
}

function getClientRootElementID(): string {
	const rootElementID = getRuntimeRouteSnapshot().rootElementID;
	if (typeof rootElementID === "string" && rootElementID.trim().length > 0) {
		return rootElementID;
	}
	return "vorma-root";
}

/**
 * Returns the client root element and validates both existence and element
 * type.
 */
export function getRootEl(): HTMLElement {
	const rootElementID = getClientRootElementID();
	const rootEl = document.getElementById(rootElementID);
	if (rootEl === null) {
		throw new Error(`Expected element with id "${rootElementID}" to exist`);
	}
	if (!(rootEl instanceof HTMLElement)) {
		throw new Error(
			`Expected element with id "${rootElementID}" to be an HTMLElement`,
		);
	}
	return rootEl;
}

/**
 * Returns the raw singleton `history` instance (`npm:history`) as an unsafe
 * escape hatch.
 *
 * Reach for this only when you explicitly need low-level history integration
 * that Vorma does not provide, such as:
 *
 * - reading raw `history.location.state` snapshots,
 * - wiring analytics/telemetry listeners directly to history updates, or
 * - coordinating with non-Vorma URL consumers in the same window.
 *
 * Do not use this instance to perform Vorma route navigation (`push`/`replace`
 * to Vorma routes). That bypasses Vorma's route-data fetch + render lifecycle
 * and can leave runtime state out of sync with the URL. Use `vormaNavigate(...)`
 * for navigation and `revalidate()` for data refreshes.
 */
export function getUnsafeHistoryInstance(): BrowserHistory {
	ensureNavigationRuntimeInitialized();
	return HistoryManager.getInstance();
}

/////////////////////////////////////////////////////////////////////
/////// URL And API Helpers
/////////////////////////////////////////////////////////////////////

function escapeRegex(value: string): string {
	return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function replaceDynamicParam(props: {
	path: string;
	token: string;
	value: string;
}): string {
	const { path, token, value } = props;
	const tokenRegex = new RegExp(`${escapeRegex(token)}(?=/|$)`, "g");
	return path.replace(tokenRegex, encodeURIComponent(value));
}

function encodeSplatValues(splatValues: Array<string>): string {
	return splatValues.map((segment) => encodeURIComponent(segment)).join("/");
}

function resolveRequiredDynamicParamKeysForPattern(props: {
	pattern: string;
	dynamicParamPrefixRune: string;
}): Array<string> {
	const requiredDynamicParamKeys: Array<string> = [];
	const dynamicParamRegex = new RegExp(
		`(?:^|/)${escapeRegex(props.dynamicParamPrefixRune)}([^/]+)(?=/|$)`,
		"g",
	);
	let dynamicParamMatch: RegExpExecArray | null = dynamicParamRegex.exec(
		props.pattern,
	);
	while (dynamicParamMatch) {
		const dynamicParamKey = dynamicParamMatch[1];
		if (
			typeof dynamicParamKey === "string" &&
			!requiredDynamicParamKeys.includes(dynamicParamKey)
		) {
			requiredDynamicParamKeys.push(dynamicParamKey);
		}
		dynamicParamMatch = dynamicParamRegex.exec(props.pattern);
	}
	return requiredDynamicParamKeys;
}

function hasRequiredSplatSegmentToken(props: {
	pattern: string;
	splatSegmentRune: string;
}): boolean {
	const splatTokenRegex = new RegExp(
		`(?:^|/)${escapeRegex(props.splatSegmentRune)}(?=/|$)`,
	);
	return splatTokenRegex.test(props.pattern);
}

function assertPathResolutionInputsMatchPatternOrThrow(props: {
	pattern: string;
	params: Record<string, unknown> | undefined;
	splatValues: Array<string> | undefined;
	dynamicParamPrefixRune: string;
	splatSegmentRune: string;
}): void {
	const requiredDynamicParamKeys = resolveRequiredDynamicParamKeysForPattern({
		pattern: props.pattern,
		dynamicParamPrefixRune: props.dynamicParamPrefixRune,
	});

	if (
		props.params !== undefined &&
		(typeof props.params !== "object" ||
			props.params === null ||
			Array.isArray(props.params))
	) {
		throw new Error(
			`Route params for pattern "${props.pattern}" must be an object when provided.`,
		);
	}

	const providedParams = props.params ?? {};
	const unresolvedDynamicParamKeys = requiredDynamicParamKeys.filter(
		(requiredDynamicParamKey) =>
			!(requiredDynamicParamKey in providedParams),
	);
	if (unresolvedDynamicParamKeys.length > 0) {
		throw new Error(
			`Missing required route params for pattern "${props.pattern}": ${unresolvedDynamicParamKeys.join(", ")}`,
		);
	}

	const invalidDynamicParamKeys = requiredDynamicParamKeys.filter(
		(requiredDynamicParamKey) => {
			const value = providedParams[requiredDynamicParamKey];
			return typeof value !== "string";
		},
	);
	if (invalidDynamicParamKeys.length > 0) {
		throw new Error(
			`Invalid required route params for pattern "${props.pattern}": ${invalidDynamicParamKeys.join(", ")} (expected string values).`,
		);
	}

	const unexpectedDynamicParamKeys = Object.keys(providedParams).filter(
		(providedDynamicParamKey) =>
			!requiredDynamicParamKeys.includes(providedDynamicParamKey),
	);
	if (unexpectedDynamicParamKeys.length > 0) {
		throw new Error(
			`Unexpected route params for pattern "${props.pattern}": ${unexpectedDynamicParamKeys.join(", ")}`,
		);
	}

	if (props.splatValues !== undefined && !Array.isArray(props.splatValues)) {
		throw new Error(
			`Splat values for pattern "${props.pattern}" must be an array when provided.`,
		);
	}

	const hasRequiredSplat = hasRequiredSplatSegmentToken({
		pattern: props.pattern,
		splatSegmentRune: props.splatSegmentRune,
	});
	if (hasRequiredSplat && props.splatValues === undefined) {
		throw new Error(
			`Missing required splat values for pattern "${props.pattern}"`,
		);
	}
	if (!hasRequiredSplat && props.splatValues !== undefined) {
		throw new Error(
			`Unexpected splat values for pattern "${props.pattern}"`,
		);
	}

	if (props.splatValues === undefined) {
		return;
	}

	const hasInvalidSplatSegment = props.splatValues.some(
		(splatSegment) => typeof splatSegment !== "string",
	);
	if (hasInvalidSplatSegment) {
		throw new Error(
			`Invalid splat values for pattern "${props.pattern}": expected string segments.`,
		);
	}
}

export function resolveVormaPath(input: ResolvePathInput): string {
	const { props, vormaAppConfig } = input;
	let path = props.pattern;

	let dynamicParamPrefixRune = vormaAppConfig.actionsDynamicRune;
	let splatSegmentRune = vormaAppConfig.actionsSplatRune;

	if (input.type === "loader") {
		dynamicParamPrefixRune = vormaAppConfig.loadersDynamicRune;
		splatSegmentRune = vormaAppConfig.loadersSplatRune;
	}

	assertPathResolutionInputsMatchPatternOrThrow({
		pattern: props.pattern,
		params: props.params,
		splatValues: props.splatValues,
		dynamicParamPrefixRune,
		splatSegmentRune,
	});

	if ("params" in props && props.params) {
		for (const [key, value] of Object.entries(props.params)) {
			path = replaceDynamicParam({
				path,
				token: `${dynamicParamPrefixRune}${key}`,
				value: value as string,
			});
		}
	}

	if ("splatValues" in props && props.splatValues) {
		const splatPath = encodeSplatValues(props.splatValues);
		path = path.replace(splatSegmentRune, splatPath);
	}

	// Strip explicit index segment
	if (
		input.type === "loader" &&
		vormaAppConfig.loadersExplicitIndexSegmentIdentifier
	) {
		const indexSegment = `/${vormaAppConfig.loadersExplicitIndexSegmentIdentifier}`;
		if (path.endsWith(indexSegment)) {
			path = path.slice(0, -indexSegment.length) || "/";
		}
	}

	return path;
}

function getCurrentOrigin(): string {
	return new URL(window.location.href).origin;
}

function stripTrailingSlash(path: string): string {
	return path.endsWith("/") ? path.slice(0, -1) : path;
}

function assertQueryInputRootIsObjectOrNil(
	input: unknown,
): asserts input is Record<string, unknown> | null | undefined {
	if (input === undefined || input === null) {
		return;
	}

	if (typeof input !== "object" || Array.isArray(input)) {
		throw new Error(
			"Query input root must be an object, null, or undefined.",
		);
	}
}

function buildVormaURL(input: URLBuildInput): URL {
	const basePath = stripTrailingSlash(
		input.vormaAppConfig.actionsRouterMountRoot,
	);
	const resolvedPath = resolveVormaPath(input);
	const url = new URL(basePath + resolvedPath, getCurrentOrigin());

	if (input.type === "query") {
		assertQueryInputRootIsObjectOrNil(input.props.input);
		if (input.props.input !== undefined && input.props.input !== null) {
			url.search = serializeToSearchParams(input.props.input).toString();
		}
	}

	return url;
}

export function resolveVormaRequestBody(
	input: unknown,
): BodyInit | null | undefined {
	return resolveRequestBodyForTransport({
		input,
	}).body;
}

export function buildQueryURL(
	vormaAppConfig: VormaAppConfig,
	props: Props,
): URL {
	return buildVormaURL({ vormaAppConfig, props, type: "query" });
}

export function buildMutationURL(
	vormaAppConfig: VormaAppConfig,
	props: Props,
): URL {
	return buildVormaURL({ vormaAppConfig, props, type: "mutation" });
}

export function resolveBody(props: Props): BodyInit | null | undefined {
	return resolveVormaRequestBody(props.input);
}

function mergeRequestInitWithHeaders(input: {
	baseRequestInit: RequestInit;
	overrideRequestInit?: RequestInit;
}): RequestInit {
	const { baseRequestInit, overrideRequestInit } = input;
	if (overrideRequestInit === undefined) {
		return baseRequestInit;
	}

	const mergedHeaders = new Headers(baseRequestInit.headers ?? undefined);
	const overrideHeaders = new Headers(
		overrideRequestInit.headers ?? undefined,
	);
	overrideHeaders.forEach((value, key) => {
		mergedHeaders.set(key, value);
	});

	return {
		...baseRequestInit,
		...overrideRequestInit,
		headers: mergedHeaders,
	};
}

export function makeTypedAPIClient<C extends VormaAppConfig>(
	vormaAppConfig: C,
	decorateRequestInit?: APIRequestInitDecorator<ExtractApp<C>>,
): TypedAPIClient<ExtractApp<C>> {
	const requestInitDecorator = decorateRequestInit;

	const resolveRequestInit = (props: {
		requestContext: APIRequestInitDecoratorContext<ExtractApp<C>>;
		fallbackRequestInit: RequestInit;
	}): RequestInit => {
		const decoratedRequestInit = requestInitDecorator?.(
			props.requestContext,
		);
		const baseRequestInit = mergeRequestInitWithHeaders({
			baseRequestInit: props.fallbackRequestInit,
			overrideRequestInit: decoratedRequestInit,
		});
		return mergeRequestInitWithHeaders({
			baseRequestInit,
			overrideRequestInit: props.requestContext.requestInit,
		});
	};

	const query = async <P extends VormaQueryPattern<ExtractApp<C>>>(
		queryProps: VormaQueryProps<ExtractApp<C>, P>,
	): Promise<SubmitResult<VormaQueryOutput<ExtractApp<C>, P>>> => {
		const requestInit = resolveRequestInit({
			requestContext: {
				type: "query",
				pattern: queryProps.pattern,
				requestInit: queryProps.requestInit,
				input: queryProps.input,
			},
			fallbackRequestInit: { method: "GET" },
		});
		return await submit<VormaQueryOutput<ExtractApp<C>, P>>(
			buildQueryURL(vormaAppConfig, queryProps),
			requestInit,
			queryProps.options,
		);
	};

	const mutate = async <P extends VormaMutationPattern<ExtractApp<C>>>(
		mutationProps: VormaMutationProps<ExtractApp<C>, P>,
	): Promise<SubmitResult<VormaMutationOutput<ExtractApp<C>, P>>> => {
		const requestInit = resolveRequestInit({
			requestContext: {
				type: "mutation",
				pattern: mutationProps.pattern,
				requestInit: mutationProps.requestInit,
				input: mutationProps.input,
			},
			fallbackRequestInit: {
				method: "POST",
				body: resolveBody(mutationProps),
			},
		});
		return await submit<VormaMutationOutput<ExtractApp<C>, P>>(
			buildMutationURL(vormaAppConfig, mutationProps),
			requestInit,
			mutationProps.options,
		);
	};

	return {
		query,
		mutate,
	};
}

export function resolvePath(opts: APIClientHelperOpts): string {
	return resolveVormaPath(opts);
}

/////////////////////////////////////////////////////////////////////
/////// Client Runtime Extras (Focus + HMR)
/////////////////////////////////////////////////////////////////////

export function shouldTriggerFocusRevalidation(props: {
	status: StatusEventDetail;
	nowTimestampMS: number;
	lastTriggeredNavOrRevalidateTimestampMS: number;
	staleTimeMS: number;
}): boolean {
	if (
		props.status.isNavigating ||
		props.status.isSubmitting ||
		props.status.isRevalidating
	) {
		return false;
	}

	if (
		props.nowTimestampMS - props.lastTriggeredNavOrRevalidateTimestampMS <
		props.staleTimeMS
	) {
		return false;
	}

	return true;
}

export function registerClientLoaderForAdapter(
	props: RegisterClientLoaderForAdapterProps,
): void {
	const { pattern, waitFn, onRegistrationError } = props;

	try {
		registerClientLoaderPatternOrThrow(pattern);
	} catch (error) {
		if (onRegistrationError) {
			onRegistrationError(error);
			return;
		}
		const reason = error instanceof Error ? error.message : String(error);
		throw new Error(
			`Failed to register client loader pattern "${pattern}": ${reason}`,
		);
	}

	setClientLoaderWaitFn(pattern, waitFn);
}

const DEFAULT_DELAY = 12;

function parseGlobalLoadingIndicatorConfig(
	config: GlobalLoadingIndicatorConfig,
): ParsedGlobalLoadingIndicatorConfig {
	const includesAll = !config.include || config.include === "all";
	const includeList =
		!includesAll && Array.isArray(config.include) ? config.include : [];

	return {
		includesAll,
		includesNavigations: includesAll || includeList.includes("navigations"),
		includesSubmissions: includesAll || includeList.includes("submissions"),
		includesRevalidations:
			includesAll || includeList.includes("revalidations"),
		startDelayMS: config.startDelayMS ?? DEFAULT_DELAY,
		stopDelayMS: config.stopDelayMS ?? DEFAULT_DELAY,
	};
}

export function setupGlobalLoadingIndicator(
	config: GlobalLoadingIndicatorConfig,
) {
	let gliDebounceStartTimer: number | null = null;
	let gliDebounceStopTimer: number | null = null;
	const pc = parseGlobalLoadingIndicatorConfig(config);
	function clearStartTimer() {
		if (gliDebounceStartTimer !== null) {
			window.clearTimeout(gliDebounceStartTimer);
			gliDebounceStartTimer = null;
		}
	}
	function clearStopTimer() {
		if (gliDebounceStopTimer !== null) {
			window.clearTimeout(gliDebounceStopTimer);
			gliDebounceStopTimer = null;
		}
	}
	function clearTimers() {
		clearStartTimer();
		clearStopTimer();
	}
	function handleStatusChange(e?: StatusEvent) {
		const shouldBeWorking = getIsWorking(pc, e);
		if (shouldBeWorking) {
			clearStopTimer();
			if (gliDebounceStartTimer === null) {
				gliDebounceStartTimer = window.setTimeout(() => {
					gliDebounceStartTimer = null;
					if (!config.isRunning() && getIsWorking(pc)) {
						config.start();
					}
				}, pc.startDelayMS);
			}
		} else {
			clearStartTimer();
			if (gliDebounceStopTimer === null) {
				gliDebounceStopTimer = window.setTimeout(() => {
					gliDebounceStopTimer = null;
					if (config.isRunning() && !getIsWorking(pc)) {
						config.stop();
					}
				}, pc.stopDelayMS);
			}
		}
	}
	handleStatusChange();
	const removeStatusListenerCallback = addStatusListener(handleStatusChange);
	return () => {
		removeStatusListenerCallback();
		clearTimers();
		if (config.isRunning()) {
			config.stop();
		}
	};
}

function getIsWorking(
	pc: ParsedGlobalLoadingIndicatorConfig,
	e?: StatusEvent,
): boolean {
	const status = e?.detail ?? getStatus();
	if (pc.includesAll) {
		return (
			status.isNavigating || status.isSubmitting || status.isRevalidating
		);
	}
	if (pc.includesNavigations && status.isNavigating) {
		return true;
	}
	if (pc.includesSubmissions && status.isSubmitting) {
		return true;
	}
	if (pc.includesRevalidations && status.isRevalidating) {
		return true;
	}
	return false;
}

/**
 * If called, will setup listeners to revalidate the current route when
 * the window regains focus and at least `staleTimeMS` has passed since
 * the last revalidation. The `staleTimeMS` option defaults to 5,000
 * (5 seconds). Returns a cleanup function.
 */
export function revalidateOnWindowFocus(options?: { staleTimeMS?: number }) {
	const staleTimeMS = options?.staleTimeMS ?? 5_000;
	return addOnWindowFocusListener(() => {
		const shouldRevalidate = shouldTriggerFocusRevalidation({
			status: getStatus(),
			nowTimestampMS: Date.now(),
			lastTriggeredNavOrRevalidateTimestampMS:
				getLastTriggeredNavOrRevalidateTimestampMS(),
			staleTimeMS,
		});
		if (shouldRevalidate) {
			revalidate();
		}
	});
}

export function classifyEligibleAnchorTarget(
	anchorDetails: EligibleInternalAnchorDetails,
): NavigationTargetClassificationAgainstCurrentLocation {
	return classifyNavigationTargetAgainstCurrentLocation({
		targetHref: anchorDetails.anchor.href,
		currentHref: window.location.href,
	});
}

export function getEligibleInternalAnchorDetails(
	event: Event,
): EligibleInternalAnchorDetails | null {
	if (event.defaultPrevented) return null;

	const anchorDetails = getAnchorDetailsFromEvent(
		event as unknown as MouseEvent,
	);
	if (!anchorDetails) return null;

	if (
		!anchorDetails.isEligibleForDefaultPrevention ||
		!anchorDetails.isInternal
	) {
		return null;
	}

	return anchorDetails;
}

export async function navigateEligibleInternalAnchorClick<E extends Event>(
	props: LinkOnClickCallbacks<E> & {
		event: E;
		anchorDetails: EligibleInternalAnchorDetails;
		scrollToTop?: boolean;
		replace?: boolean;
		state?: unknown;
		shouldRunBeforeBegin?: boolean;
	},
): Promise<void> {
	const {
		event,
		anchorDetails,
		beforeBegin,
		beforeRender,
		afterRender,
		scrollToTop,
		replace,
		state,
		shouldRunBeforeBegin = true,
	} = props;
	const targetType = classifyEligibleAnchorTarget(anchorDetails);
	const isCrossDocumentNavigation = targetType === "navigate";

	event.preventDefault();

	if (isCrossDocumentNavigation && shouldRunBeforeBegin) {
		await beforeBegin?.(event);
	}
	if (isCrossDocumentNavigation) {
		await beforeRender?.(event);
	}

	try {
		const navigationResult = await navigationStateManager.navigate({
			href: anchorDetails.anchor.href,
			navigationType: "userNavigation",
			scrollToTop,
			replace,
			state,
		});
		if (isCrossDocumentNavigation && navigationResult.didNavigate) {
			await afterRender?.(event);
		}
	} catch (error) {
		logError("Link navigation failed", error);
	}
}

export function createLinkOnClickFn<E extends Event>(
	callbacks: LinkOnClickCallbacks<E> & {
		scrollToTop?: boolean;
		replace?: boolean;
		state?: unknown;
	},
) {
	return async (event: E) => {
		const anchorDetails = getEligibleInternalAnchorDetails(event);
		if (!anchorDetails) return;

		await navigateEligibleInternalAnchorClick({
			event,
			anchorDetails,
			beforeBegin: callbacks.beforeBegin,
			beforeRender: callbacks.beforeRender,
			afterRender: callbacks.afterRender,
			scrollToTop: callbacks.scrollToTop,
			replace: callbacks.replace,
			state: callbacks.state,
		});
	};
}

/////////////////////////////////////////////////////////////////////
/////// Link Prefetch Lifecycle
/////////////////////////////////////////////////////////////////////

function findIdlePrefetchNavigationByDataTarget(
	targetHref: string,
): ReturnType<typeof navigationStateManager.getNavigation> {
	const exact = navigationStateManager.getNavigation(targetHref);
	if (exact && exact.type === "prefetch" && exact.intent === "none") {
		return exact;
	}

	for (const nav of navigationStateManager.getNavigations().values()) {
		if (
			nav.type === "prefetch" &&
			nav.intent === "none" &&
			hasSameDataTarget({
				firstHref: nav.targetUrl,
				secondHref: targetHref,
			})
		) {
			return nav;
		}
	}

	return undefined;
}

function hasIdlePrefetchNavigation(targetHref: string): boolean {
	return !!findIdlePrefetchNavigationByDataTarget(targetHref);
}

async function startPrefetchNavigation(props: {
	targetHref: string;
	state?: unknown;
}): Promise<void> {
	await navigationStateManager.navigate({
		href: props.targetHref,
		navigationType: "prefetch",
		state: props.state,
	});
}

function abortIdlePrefetchNavigation(targetHref: string): void {
	const nav = findIdlePrefetchNavigationByDataTarget(targetHref);
	if (!nav) return;

	nav.control.abortController?.abort();
	navigationStateManager.removeNavigation(nav.targetUrl);
}

async function handlePrefetchClick<E extends Event>(props: {
	event: E;
	prefetchStarted: boolean;
	clearPendingTimer: () => void;
	callbacks: LinkOnClickCallbacks<E>;
	navigationOptions: ClickNavigationOptions;
}): Promise<void> {
	const {
		event,
		prefetchStarted,
		clearPendingTimer,
		callbacks,
		navigationOptions,
	} = props;
	const anchorDetails = getEligibleInternalAnchorDetails(event);
	if (!anchorDetails) return;

	clearPendingTimer();
	await navigateEligibleInternalAnchorClick({
		event,
		anchorDetails,
		beforeBegin: callbacks.beforeBegin,
		beforeRender: callbacks.beforeRender,
		afterRender: callbacks.afterRender,
		scrollToTop: navigationOptions.scrollToTop,
		replace: navigationOptions.replace,
		state: navigationOptions.state,
		shouldRunBeforeBegin: !prefetchStarted,
	});
}

export function createPrefetchHandlers<E extends Event>(
	input: CreatePrefetchHandlersInput<E>,
) {
	const hrefDetails = getHrefDetails(input.href);
	if (!hrefDetails.isHTTP) {
		return;
	}

	const { relativeURL } = hrefDetails;
	if (!relativeURL || hrefDetails.isExternal) {
		return;
	}

	let timer: number | undefined;
	let prefetchStarted = false;
	const delayMs = input.delayMs ?? 100;
	const targetHref = hrefDetails.absoluteURL;

	function clearPendingTimer(): void {
		if (timer === undefined) {
			return;
		}

		clearTimeout(timer);
		timer = undefined;
	}

	function hasActiveIdlePrefetch(): boolean {
		return hasIdlePrefetchNavigation(targetHref);
	}

	async function prefetch(event: E): Promise<void> {
		prefetchStarted = true;

		try {
			if (input.beforeBegin) {
				await input.beforeBegin(event);
			}
			await startPrefetchNavigation({ targetHref, state: input.state });
			prefetchStarted = hasActiveIdlePrefetch();
		} catch (error) {
			prefetchStarted = false;
			logError(
				"Prefetch start failed; allowing subsequent retries.",
				error,
			);
		}
	}

	function start(event: E): void {
		if (timer !== undefined) return;
		if (prefetchStarted && hasActiveIdlePrefetch()) return;
		prefetchStarted = false;
		timer = window.setTimeout(() => {
			timer = undefined;
			void prefetch(event);
		}, delayMs);
	}

	function stop(): void {
		clearPendingTimer();
		abortIdlePrefetchNavigation(targetHref);
		prefetchStarted = false;
	}

	async function onClick(event: E): Promise<void> {
		await handlePrefetchClick({
			event,
			prefetchStarted,
			clearPendingTimer,
			callbacks: {
				beforeBegin: input.beforeBegin,
				beforeRender: input.beforeRender,
				afterRender: input.afterRender,
			},
			navigationOptions: {
				scrollToTop: input.scrollToTop,
				replace: input.replace,
				state: input.state,
			},
		});
	}

	return {
		...hrefDetails,
		start,
		stop,
		onClick,
	};
}

/////////////////////////////////////////////////////////////////////
/////// Link Props Helpers
/////////////////////////////////////////////////////////////////////

export const defaultErrorBoundary: RouteErrorComponent = (props: {
	error: string;
}) => {
	return "Route Error: " + props.error;
};

function adaptDOMEventCallback<LinkEvent>(
	callback: ((event: LinkEvent) => void | Promise<void>) | undefined,
): ((event: Event) => void | Promise<void>) | undefined {
	if (!callback) return undefined;
	return (event: Event) => callback(event as unknown as LinkEvent);
}

function linkPropsToPrefetchObj<LinkEvent>(
	props: VormaLinkPropsBase<LinkEvent>,
) {
	if (!props.href || props.prefetch !== "intent") {
		return undefined;
	}

	return createPrefetchHandlers({
		href: props.href,
		delayMs: props.prefetchDelayMs,
		beforeBegin: adaptDOMEventCallback(props.beforeBegin),
		beforeRender: adaptDOMEventCallback(props.beforeRender),
		afterRender: adaptDOMEventCallback(props.afterRender),
		scrollToTop: props.scrollToTop,
		replace: props.replace,
		state: props.state,
	});
}

function linkPropsToOnClickFn<LinkEvent>(props: VormaLinkPropsBase<LinkEvent>) {
	return createLinkOnClickFn({
		beforeBegin: adaptDOMEventCallback(props.beforeBegin),
		beforeRender: adaptDOMEventCallback(props.beforeRender),
		afterRender: adaptDOMEventCallback(props.afterRender),
		scrollToTop: props.scrollToTop,
		replace: props.replace,
		state: props.state,
	});
}

const standardCamelHandlerKeys = {
	onPointerEnter: "onPointerEnter",
	onFocus: "onFocus",
	onPointerLeave: "onPointerLeave",
	onBlur: "onBlur",
	onTouchCancel: "onTouchCancel",
	onClick: "onClick",
} satisfies HandlerKeys;

function isFn(fn: unknown): fn is UnknownFn {
	return typeof fn === "function";
}

function isDefaultPreventedEventLike(event: unknown): boolean {
	if (typeof event !== "object" || event === null) {
		return false;
	}

	return (event as { defaultPrevented?: unknown }).defaultPrevented === true;
}

export function makeFinalLinkProps<LinkEvent>(
	props: VormaLinkPropsBase<LinkEvent>,
	keys: HandlerKeys = standardCamelHandlerKeys,
) {
	const prefetchObj = linkPropsToPrefetchObj(props);
	const hrefDetails = props.href ? getHrefDetails(props.href) : null;
	const propsBag = props as Record<string, unknown>;

	function callOriginalHandlerIfPresent(
		handlerKey: string,
		event: LinkEvent,
	): void {
		const maybeHandler = propsBag[handlerKey];
		if (isFn(maybeHandler)) {
			maybeHandler(event);
		}
	}

	return {
		dataExternal:
			hrefDetails?.isHTTP && hrefDetails.isExternal ? true : undefined,
		onPointerEnter: (event: LinkEvent) => {
			prefetchObj?.start(event as Event);
			callOriginalHandlerIfPresent(keys.onPointerEnter, event);
		},
		onFocus: (event: LinkEvent) => {
			prefetchObj?.start(event as Event);
			callOriginalHandlerIfPresent(keys.onFocus, event);
		},
		onPointerLeave: (event: LinkEvent) => {
			if (!__vormaClientGlobal.get("isTouchInputModalityActive")) {
				prefetchObj?.stop();
			}
			callOriginalHandlerIfPresent(keys.onPointerLeave, event);
		},
		onBlur: (event: LinkEvent) => {
			prefetchObj?.stop();
			callOriginalHandlerIfPresent(keys.onBlur, event);
		},
		onTouchCancel: (event: LinkEvent) => {
			prefetchObj?.stop();
			callOriginalHandlerIfPresent(keys.onTouchCancel, event);
		},
		onClick: async (event: LinkEvent) => {
			callOriginalHandlerIfPresent(keys.onClick, event);
			if (isDefaultPreventedEventLike(event)) {
				return;
			}
			if (prefetchObj) {
				await prefetchObj.onClick(event as Event);
			} else {
				await linkPropsToOnClickFn(props)(event as Event);
			}
		},
	};
}

export function resolveTypedLinkHref(props: {
	vormaAppConfig: VormaAppConfig;
	pattern: string;
	params?: Record<string, string>;
	splatValues?: Array<string>;
	search?: string;
	hash?: string;
}): string {
	const href = resolvePath({
		vormaAppConfig: props.vormaAppConfig,
		type: "loader",
		props: {
			pattern: props.pattern,
			...(props.params && { params: props.params }),
			...(props.splatValues && { splatValues: props.splatValues }),
		},
	});

	return resolveAbsoluteHrefWithOptionalSearchAndHash({
		href,
		search: props.search,
		hash: props.hash,
		baseHref: window.location.origin,
	});
}

export function makeTypedNavigate<C extends VormaAppConfig>(vormaAppConfig: C) {
	type App = ExtractApp<C>;

	return async function typedNavigate<
		Pattern extends VormaLoaderPattern<App>,
	>(options: TypedNavigateOptions<App, Pattern>): Promise<void> {
		const { pattern, replace, scrollToTop, search, hash, state } = options;
		const params = "params" in options ? options.params : undefined;
		const splatValues =
			"splatValues" in options ? options.splatValues : undefined;

		const href = resolvePath({
			vormaAppConfig,
			type: "loader",
			props: {
				pattern,
				...(params && { params }),
				...(splatValues && { splatValues }),
			},
		});

		return vormaNavigate(href, {
			replace,
			scrollToTop,
			search,
			hash,
			state,
		});
	};
}

/////////////////////////////////////////////////////////////////////
/////// Client Initialization
/////////////////////////////////////////////////////////////////////

let beforeUnloadRegistered = false;
let inputModalityDetectionRegistered = false;
let latestRouteManifestProgressiveLoadID = 0;

function onBeforeUnload(): void {
	scrollStateManager.savePageRefreshState();
}

function setTouchInputModalityActive(): void {
	if (__vormaClientGlobal.get("isTouchInputModalityActive")) {
		return;
	}
	__vormaClientGlobal.set("isTouchInputModalityActive", true);
}

function setFinePointerInputModalityActive(): void {
	if (!__vormaClientGlobal.get("isTouchInputModalityActive")) {
		return;
	}
	__vormaClientGlobal.set("isTouchInputModalityActive", false);
}

function onPointerModalityChanged(event: Event): void {
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
		setTouchInputModalityActive();
		return;
	}

	if (normalizedPointerType === "mouse" || normalizedPointerType === "pen") {
		setFinePointerInputModalityActive();
	}
}

function registerBeforeUnloadScrollStatePersistence(): void {
	if (beforeUnloadRegistered) return;
	window.addEventListener("beforeunload", onBeforeUnload);
	beforeUnloadRegistered = true;
}

function applyInitClientOptions(options: InitClientOptions): void {
	if (options.defaultErrorBoundary) {
		__vormaClientGlobal.set(
			"defaultErrorBoundary",
			options.defaultErrorBoundary,
		);
	} else {
		__vormaClientGlobal.set("defaultErrorBoundary", defaultErrorBoundary);
	}

	__vormaClientGlobal.set(
		"useViewTransitions",
		options.useViewTransitions === true,
	);
}

function initializeClientPatternRegistry(vormaAppConfig: VormaAppConfig): void {
	const patternRegistry = createPatternRegistry({
		dynamicParamPrefixRune: vormaAppConfig.loadersDynamicRune,
		splatSegmentRune: vormaAppConfig.loadersSplatRune,
		explicitIndexSegment:
			vormaAppConfig.loadersExplicitIndexSegmentIdentifier,
	});
	__vormaClientGlobal.set("patternRegistry", patternRegistry);
}

function parseRouteManifestPayloadOrThrow(
	manifestPayload: unknown,
): RouteManifestRecord {
	if (
		typeof manifestPayload !== "object" ||
		manifestPayload === null ||
		Array.isArray(manifestPayload)
	) {
		throw new Error(
			"Route manifest must be a non-null object with pattern keys.",
		);
	}

	const parsedManifest: RouteManifestRecord = {};
	for (const [pattern, loaderFlag] of Object.entries(manifestPayload)) {
		if (loaderFlag !== 0 && loaderFlag !== 1) {
			throw new Error(
				`Route manifest value for pattern '${pattern}' must be 0 or 1.`,
			);
		}
		parsedManifest[pattern] = loaderFlag;
	}

	return parsedManifest;
}

function registerManifestPatterns(props: {
	manifest: RouteManifestRecord;
	patternRegistry: VormaClientGlobal["patternRegistry"];
}): void {
	const { manifest, patternRegistry } = props;
	for (const pattern of Object.keys(manifest)) {
		registerPattern(patternRegistry, pattern);
	}
}

function clonePatternRegistry(props: {
	patternRegistry: VormaClientGlobal["patternRegistry"];
}): VormaClientGlobal["patternRegistry"] {
	const { dynamicParamPrefixRune, splatSegmentRune, explicitIndexSegment } =
		props.patternRegistry.config;
	const nextPatternRegistry = createPatternRegistry({
		dynamicParamPrefixRune,
		splatSegmentRune,
		explicitIndexSegment,
	});

	for (const registeredPattern of props.patternRegistry.staticPatterns.values()) {
		registerPattern(nextPatternRegistry, registeredPattern.originalPattern);
	}
	for (const registeredPattern of props.patternRegistry.dynamicPatterns.values()) {
		registerPattern(nextPatternRegistry, registeredPattern.originalPattern);
	}

	return nextPatternRegistry;
}

function applyRouteManifestAtomicallyOrThrow(props: {
	manifest: RouteManifestRecord;
	patternRegistry: VormaClientGlobal["patternRegistry"];
}): void {
	const nextPatternRegistry = clonePatternRegistry({
		patternRegistry: props.patternRegistry,
	});
	registerManifestPatterns({
		manifest: props.manifest,
		patternRegistry: nextPatternRegistry,
	});
	__vormaClientGlobal.set("patternRegistry", nextPatternRegistry);
	__vormaClientGlobal.set("routeManifest", props.manifest);
}

function readPrecompiledRouteManifestOrNull(): RouteManifestRecord | null {
	const precompiledRouteManifest = __vormaClientGlobal.get("routeManifest");
	if (!precompiledRouteManifest) {
		return null;
	}

	return parseRouteManifestPayloadOrThrow(precompiledRouteManifest);
}

function initializePatternRegistryFromPrecompiledRouteManifest(): boolean {
	const manifest = readPrecompiledRouteManifestOrNull();
	if (!manifest) {
		return false;
	}

	const patternRegistry = __vormaClientGlobal.get("patternRegistry");
	applyRouteManifestAtomicallyOrThrow({
		manifest,
		patternRegistry,
	});
	return true;
}

export async function loadRouteManifestProgressively(): Promise<void> {
	const manifestURL = __vormaClientGlobal.get("routeManifestURL");
	if (!manifestURL) {
		return;
	}

	const patternRegistry = __vormaClientGlobal.get("patternRegistry");
	const routeManifestProgressiveLoadID =
		++latestRouteManifestProgressiveLoadID;
	let response: Response;
	try {
		response = await fetch(manifestURL);
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

	const manifestPayload = await response.json();
	const manifest = parseRouteManifestPayloadOrThrow(manifestPayload);

	if (
		routeManifestProgressiveLoadID !== latestRouteManifestProgressiveLoadID
	) {
		return;
	}

	if (__vormaClientGlobal.get("patternRegistry") !== patternRegistry) {
		return;
	}

	applyRouteManifestAtomicallyOrThrow({ manifest, patternRegistry });
}

function cleanupHardReloadQueryParam(): void {
	const url = new URL(window.location.href);
	if (!url.searchParams.has(VORMA_HARD_RELOAD_QUERY_PARAM)) {
		return;
	}

	url.searchParams.delete(VORMA_HARD_RELOAD_QUERY_PARAM);
	HistoryManager.getInstance().replace(url.href);
}

function registerInputModalityDetection(): void {
	if (inputModalityDetectionRegistered) {
		return;
	}

	window.addEventListener("touchstart", setTouchInputModalityActive);
	window.addEventListener("pointerdown", onPointerModalityChanged);
	window.addEventListener("pointermove", onPointerModalityChanged);
	window.addEventListener("pointerenter", onPointerModalityChanged);
	inputModalityDetectionRegistered = true;
}

async function bootstrapInitialClientRuntime(
	importURLs: Array<string> | undefined,
): Promise<void> {
	const modulesMap = await ComponentLoader.handleComponents(importURLs);
	await setupClientLoaders();
	await ComponentLoader.handleErrorBoundaryComponent(importURLs, modulesMap);
}

export async function initClient(options: InitClientInput): Promise<void> {
	registerBeforeUnloadScrollStatePersistence();

	__vormaClientGlobal.set("vormaAppConfig", options.vormaAppConfig);
	initializeClientPatternRegistry(options.vormaAppConfig);

	const didInitializePatternRegistryFromPrecompiledManifest =
		initializePatternRegistryFromPrecompiledRouteManifest();
	if (!didInitializePatternRegistryFromPrecompiledManifest) {
		void loadRouteManifestProgressively();
	}
	applyInitClientOptions(options);

	ensureNavigationRuntimeInitialized();
	HistoryManager.init();
	cleanupHardReloadQueryParam();

	const importURLs = getRuntimeRouteSnapshot().importURLs;
	await bootstrapInitialClientRuntime(importURLs);

	options.renderFn();
	scrollStateManager.restorePageRefreshState();
	registerInputModalityDetection();
}

/////////////////////////////////////////////////////////////////////
/////// Component Module Runtime
/////////////////////////////////////////////////////////////////////

export function getEffectiveErrorDataFromSnapshot(props: {
	outermostServerErrorIdx: number | undefined;
	outermostClientErrorIdx: number | undefined;
	outermostServerError: string | undefined;
	outermostClientError: string | undefined;
}): {
	index: number | undefined;
	error: string | undefined;
} {
	const {
		outermostServerErrorIdx: serverErrorIdx,
		outermostClientErrorIdx: clientErrorIdx,
		outermostServerError: serverError,
		outermostClientError: clientError,
	} = props;
	let errorIdx: number | undefined;
	if (serverErrorIdx != null && clientErrorIdx != null) {
		errorIdx = Math.min(serverErrorIdx, clientErrorIdx);
	} else {
		errorIdx = serverErrorIdx ?? clientErrorIdx;
	}

	if (errorIdx == null) {
		return {
			index: undefined,
			error: undefined,
		};
	}

	let error: string | undefined;
	if (serverErrorIdx != null && clientErrorIdx != null) {
		if (serverErrorIdx === clientErrorIdx) {
			error = serverError ?? clientError;
		} else {
			error = errorIdx === serverErrorIdx ? serverError : clientError;
		}
	} else {
		error = errorIdx === serverErrorIdx ? serverError : clientError;
	}

	return {
		index: errorIdx,
		error,
	};
}

function applyEffectiveErrorDataToRuntimeRouteSnapshot(
	snapshot: RuntimeRouteSnapshot,
): RuntimeRouteSnapshot {
	const effectiveErrorData = getEffectiveErrorDataFromSnapshot({
		outermostServerErrorIdx: snapshot.outermostServerErrorIdx,
		outermostClientErrorIdx: snapshot.outermostClientErrorIdx,
		outermostServerError: snapshot.outermostServerError,
		outermostClientError: snapshot.outermostClientError,
	});
	return {
		...snapshot,
		outermostErrorIdx: effectiveErrorData.index,
		outermostError: effectiveErrorData.error,
	};
}

async function loadComponentModules(
	importURLs: string[] = [],
): Promise<ComponentModulesMap> {
	const dedupedURLs = [...new Set(importURLs)];
	const modules = await Promise.all(
		dedupedURLs.map(async (url) => {
			if (!url) return undefined;
			return import(/* @vite-ignore */ resolvePublicHref(url));
		}),
	);
	return new Map(dedupedURLs.map((url, i) => [url, modules[i]]));
}

export function buildActiveComponentsFromModules(props: {
	importURLs: string[];
	exportKeys: string[];
	modulesMap: ComponentModulesMap;
}): Array<unknown> {
	const { importURLs, exportKeys, modulesMap } = props;
	return importURLs.map((url, i) => {
		const module = modulesMap.get(url);
		const key = exportKeys[i] ?? "default";
		return module?.[key] ?? null;
	});
}

export function resolveErrorBoundaryComponentFromModules(props: {
	errorIdx: number;
	importURLs: string[];
	errorExportKeys: Array<string> | undefined;
	modulesMap: ComponentModulesMap;
	defaultErrorBoundary: unknown;
}): unknown {
	const {
		errorIdx,
		importURLs,
		errorExportKeys,
		modulesMap,
		defaultErrorBoundary,
	} = props;
	const errorModuleURL = importURLs[errorIdx];
	let errorComponent;

	if (errorModuleURL) {
		const errorModule = modulesMap.get(errorModuleURL);
		const errorKey = errorExportKeys ? errorExportKeys[errorIdx] : null;
		if (errorKey && errorModule) {
			errorComponent = errorModule[errorKey];
		}
	}

	return errorComponent ?? defaultErrorBoundary;
}

export async function loadComponents(
	importURLs?: string[],
): Promise<ComponentModulesMap> {
	return loadComponentModules(importURLs);
}

export function setActiveComponentsFromModules(props: {
	importURLs: Array<string> | undefined;
	exportKeys: Array<string> | undefined;
	modulesMap: ComponentModulesMap;
}): void {
	const snapshot = getRuntimeRouteSnapshot();
	const newActiveComponents = buildActiveComponentsFromModules({
		importURLs: props.importURLs ?? [],
		exportKeys: props.exportKeys ?? [],
		modulesMap: props.modulesMap,
	});

	if (!jsonDeepEquals(newActiveComponents, snapshot.activeComponents)) {
		updateRuntimeRouteSnapshot({
			updater: (previousSnapshot) => ({
				...previousSnapshot,
				activeComponents: newActiveComponents,
			}),
		});
	}
}

export function setActiveErrorBoundaryFromModules(props: {
	importURLs: Array<string> | undefined;
	errorExportKeys: Array<string> | undefined;
	modulesMap: ComponentModulesMap;
}): void {
	const snapshot = getRuntimeRouteSnapshot();
	const errorIdx = getEffectiveErrorDataFromSnapshot({
		outermostServerErrorIdx: snapshot.outermostServerErrorIdx,
		outermostClientErrorIdx: snapshot.outermostClientErrorIdx,
		outermostServerError: snapshot.outermostServerError,
		outermostClientError: snapshot.outermostClientError,
	}).index;
	if (errorIdx == null) {
		return;
	}

	const newErrorBoundary = resolveErrorBoundaryComponentFromModules({
		errorIdx,
		importURLs: props.importURLs ?? [],
		errorExportKeys: props.errorExportKeys,
		modulesMap: props.modulesMap,
		defaultErrorBoundary: __vormaClientGlobal.get("defaultErrorBoundary"),
	});

	if (snapshot.activeErrorBoundary !== newErrorBoundary) {
		updateRuntimeRouteSnapshot({
			updater: (previousSnapshot) => ({
				...previousSnapshot,
				activeErrorBoundary: newErrorBoundary,
			}),
		});
	}
}

export async function handleComponents(
	importURLs?: string[],
): Promise<ComponentModulesMap> {
	const modulesMap = await loadComponents(importURLs);
	const snapshot = getRuntimeRouteSnapshot();
	setActiveComponentsFromModules({
		importURLs: snapshot.importURLs,
		exportKeys: snapshot.exportKeys,
		modulesMap,
	});
	return modulesMap;
}

export async function handleErrorBoundaryComponent(
	importURLs?: string[],
	modulesMapOverride?: ComponentModulesMap,
): Promise<void> {
	const modulesMap = modulesMapOverride ?? (await loadComponents(importURLs));
	const snapshot = getRuntimeRouteSnapshot();
	setActiveErrorBoundaryFromModules({
		importURLs: snapshot.importURLs,
		errorExportKeys: snapshot.errorExportKeys,
		modulesMap,
	});
}

export const ComponentLoader = {
	loadComponents,
	handleComponents,
	handleErrorBoundaryComponent,
};

/////////////////////////////////////////////////////////////////////
/////// Client Loader Render Runtime
/////////////////////////////////////////////////////////////////////

export function createUnavailableServerDataError(): Error {
	const error = new Error(
		"Server loader data is unavailable for abandoned or failed navigation.",
	);
	error.name = "AbortError";
	return error;
}

function patternRequiresServerData(pattern: string): boolean {
	const routeManifest = __vormaClientGlobal.get("routeManifest");
	return routeManifest?.[pattern] === 1;
}

export function buildClientLoaderServerData(props: {
	pattern: string;
	matchedPatterns: Array<string>;
	loadersData: Array<unknown>;
	hasRootData: boolean;
	buildID: string;
}): ClientLoaderAwaitedServerData<unknown, unknown> | null {
	const { pattern, matchedPatterns, loadersData, hasRootData, buildID } =
		props;
	const serverIdx = matchedPatterns.indexOf(pattern);

	if (serverIdx === -1) {
		return null;
	}

	const loaderData = loadersData[serverIdx];
	const rootData = hasRootData ? loadersData[0] : null;

	if (patternRequiresServerData(pattern) && loaderData === undefined) {
		return null;
	}

	if (hasRootData && rootData === undefined) {
		return null;
	}

	return {
		matchedPatterns,
		loaderData,
		rootData,
		buildID,
	};
}

function buildClientLoaderWorkItems(props: {
	matchedPatterns: Array<string>;
	loadersData: Array<unknown>;
	params: Record<string, string>;
	splatValues: Array<string>;
	hasRootData: boolean;
	patternToWaitFnMap: VormaClientGlobal["patternToWaitFnMap"];
	outermostServerErrorIdx: number | undefined;
	runningLoaders?: Map<string, Promise<unknown>>;
	signal: AbortSignal;
	buildID: string;
}): ClientLoaderWorkItems {
	const {
		matchedPatterns,
		loadersData,
		params,
		splatValues,
		hasRootData,
		patternToWaitFnMap,
		outermostServerErrorIdx,
		runningLoaders,
		signal,
		buildID,
	} = props;

	const loaderPromises: Array<Promise<unknown>> = [];
	const abortControllers: Array<AbortController | null> = [];

	let i = 0;
	for (const pattern of matchedPatterns) {
		if (
			outermostServerErrorIdx !== undefined &&
			i === outermostServerErrorIdx
		) {
			loaderPromises.push(Promise.resolve());
			abortControllers.push(null);
			i++;
			continue;
		}

		if (runningLoaders?.has(pattern)) {
			loaderPromises.push(runningLoaders.get(pattern)!);
			abortControllers.push(null);
		} else if (patternToWaitFnMap[pattern]) {
			const controller = new AbortController();
			abortControllers.push(controller);

			if (signal.aborted) {
				controller.abort();
			} else {
				signal.addEventListener("abort", () => controller.abort(), {
					once: true,
				});
			}

			const serverData = buildClientLoaderServerData({
				pattern,
				matchedPatterns,
				loadersData,
				hasRootData,
				buildID,
			});
			const serverDataPromise = serverData
				? Promise.resolve(serverData)
				: Promise.reject(createUnavailableServerDataError());

			const loaderPromise = patternToWaitFnMap[pattern]({
				params,
				splatValues,
				serverDataPromise,
				signal: controller.signal,
			});
			loaderPromises.push(loaderPromise);
		} else {
			loaderPromises.push(Promise.resolve());
			abortControllers.push(null);
		}
		i++;
	}

	return { loaderPromises, abortControllers };
}

function wrapLoaderPromisesWithChildAbort(props: {
	loaderPromises: Array<Promise<unknown>>;
	abortControllers: Array<AbortController | null>;
}): Array<Promise<unknown>> {
	const { loaderPromises, abortControllers } = props;
	return loaderPromises.map(async (promise, index) => {
		return promise.catch((error) => {
			if (!isAbortError(error)) {
				for (let j = index + 1; j < abortControllers.length; j++) {
					abortControllers[j]?.abort();
				}
			}
			throw error;
		});
	});
}

function processSettledClientLoaderResults(props: {
	results: Array<PromiseSettledResult<unknown>>;
	matchedPatterns: Array<string>;
}): {
	data: Array<unknown>;
	errorMessage: string | undefined;
} {
	const { results, matchedPatterns } = props;
	const data: Array<unknown> = [];
	let errorMessage: string | undefined;

	for (const [resultIndex, result] of results.entries()) {
		if (result.status === "fulfilled") {
			data.push(result.value);
		} else {
			if (!isAbortError(result.reason)) {
				const pattern = matchedPatterns[resultIndex];
				logError(
					`Client loader error for pattern ${pattern}:`,
					result.reason,
				);
				errorMessage =
					result.reason instanceof Error
						? result.reason.message
						: String(result.reason);
			}
			data.push(undefined);
			break;
		}
	}

	return { data, errorMessage };
}

async function executeClientLoaders(
	json: PartialWaitFnJSON,
	buildID: string,
	signal: AbortSignal,
	runningLoaders?: Map<string, Promise<unknown>>,
): Promise<ClientLoadersResult> {
	await ComponentLoader.loadComponents(json.importURLs);

	const matchedPatterns = json.matchedPatterns;
	const loadersData = json.loadersData;
	const params = json.params;
	const splatValues = json.splatValues;
	const hasRootData = json.hasRootData;
	const patternToWaitFnMap =
		__vormaClientGlobal.get("patternToWaitFnMap") || {};
	const outermostServerErrorIdx = json.outermostServerErrorIdx;

	const { loaderPromises, abortControllers } = buildClientLoaderWorkItems({
		matchedPatterns,
		loadersData,
		params,
		splatValues,
		hasRootData,
		patternToWaitFnMap,
		outermostServerErrorIdx,
		runningLoaders,
		signal,
		buildID,
	});

	const wrappedPromises = wrapLoaderPromisesWithChildAbort({
		loaderPromises,
		abortControllers,
	});
	const results = await Promise.allSettled(wrappedPromises);

	const { data, errorMessage } = processSettledClientLoaderResults({
		results,
		matchedPatterns,
	});

	return { data, errorMessage };
}

function buildClientLoaderSnapshotFromGlobal(): PartialWaitFnJSON {
	const runtimeRouteSnapshot = getRuntimeRouteSnapshot();
	return {
		hasRootData: runtimeRouteSnapshot.hasRootData,
		importURLs: runtimeRouteSnapshot.importURLs,
		loadersData: runtimeRouteSnapshot.loadersData,
		matchedPatterns: runtimeRouteSnapshot.matchedPatterns,
		outermostServerErrorIdx: runtimeRouteSnapshot.outermostServerErrorIdx,
		params: runtimeRouteSnapshot.params,
		splatValues: runtimeRouteSnapshot.splatValues,
	};
}

export function setClientLoadersState(
	clientLoadersResult: ClientLoadersResult | undefined,
): void {
	if (!clientLoadersResult) {
		return;
	}

	const normalizedClientLoaderData = clientLoadersResult.data;
	updateRuntimeRouteSnapshot({
		updater: (runtimeRouteSnapshot) => {
			const outermostClientErrorIdx = clientLoadersResult.errorMessage
				? normalizedClientLoaderData.length > 0
					? normalizedClientLoaderData.length - 1
					: undefined
				: undefined;
			return applyEffectiveErrorDataToRuntimeRouteSnapshot({
				...runtimeRouteSnapshot,
				clientLoadersData: normalizedClientLoaderData,
				outermostClientErrorIdx,
				outermostClientError: clientLoadersResult.errorMessage,
			});
		},
	});
}

export async function setupClientLoaders(): Promise<void> {
	const clientLoadersResult = await executeClientLoaders(
		buildClientLoaderSnapshotFromGlobal(),
		getRuntimeRouteSnapshot().buildID,
		new AbortController().signal,
	);

	setClientLoadersState(clientLoadersResult);
}

export function registerClientLoaderPatternOrThrow(pattern: string): void {
	const patternRegistry = __vormaClientGlobal.get("patternRegistry");
	if (!patternRegistry) {
		throw new Error("Pattern registry has not been initialized.");
	}
	registerPattern(patternRegistry, pattern);
}

export async function registerClientLoaderPattern(
	pattern: string,
): Promise<void> {
	registerClientLoaderPatternOrThrow(pattern);
}

export async function findPartialMatchesOnClient(pathname: string) {
	const patternRegistry = __vormaClientGlobal.get("patternRegistry");
	const patternToWaitFnMap =
		__vormaClientGlobal.get("patternToWaitFnMap") || {};
	if (!patternRegistry) {
		return null;
	}

	if (Object.keys(patternToWaitFnMap).length === 0) {
		return null;
	}

	const fullResult = findNestedMatches(patternRegistry, pathname);
	if (fullResult) {
		return fullResult;
	}

	const segments = pathname.split("/").filter(Boolean);
	for (let i = segments.length; i >= 0; i--) {
		const partialPath =
			i === 0 ? "/" : "/" + segments.slice(0, i).join("/");
		const result = findNestedMatches(patternRegistry, partialPath);
		if (result) {
			return result;
		}
	}

	return null;
}

export async function completeClientLoaders(
	json: PartialWaitFnJSON,
	buildID: string,
	runningLoaders: Map<string, Promise<unknown>>,
	signal: AbortSignal,
): Promise<ClientLoadersResult> {
	return executeClientLoaders(json, buildID, signal, runningLoaders);
}

/////////////////////////////////////////////////////////////////////
/////// Head Element Reconciliation
/////////////////////////////////////////////////////////////////////

function findNearestManagedSectionBoundaryComments(type: "meta" | "rest"): {
	startComment: Comment | null;
	endComment: Comment | null;
} {
	const startMarker = `data-vorma="${type}-start"`;
	const endMarker = `data-vorma="${type}-end"`;
	let nearestStartComment: Comment | null = null;
	let nodePointer: Node | null = document.head.firstChild;

	while (nodePointer != null) {
		if (nodePointer.nodeType !== Node.COMMENT_NODE) {
			nodePointer = nodePointer.nextSibling;
			continue;
		}

		const commentNode = nodePointer as Comment;
		const commentText = commentNode.nodeValue?.trim();
		if (commentText === startMarker) {
			nearestStartComment = commentNode;
			nodePointer = nodePointer.nextSibling;
			continue;
		}

		if (commentText === endMarker && nearestStartComment) {
			return {
				startComment: nearestStartComment,
				endComment: commentNode,
			};
		}

		nodePointer = nodePointer.nextSibling;
	}

	return {
		startComment: null,
		endComment: null,
	};
}

export function getStartAndEndComments(type: "meta" | "rest"): {
	startComment: Comment | null;
	endComment: Comment | null;
} {
	return findNearestManagedSectionBoundaryComments(type);
}

function getManagedSectionParentIfValid(props: {
	startComment: Comment | null;
	endComment: Comment | null;
}): Node | null {
	const { startComment, endComment } = props;
	if (!startComment || !endComment) {
		return null;
	}

	const startParent = startComment.parentNode;
	const endParent = endComment.parentNode;
	if (!startParent || startParent !== endParent) {
		return null;
	}

	let nodePtr = startComment.nextSibling;
	while (nodePtr != null) {
		if (nodePtr === endComment) {
			return startParent;
		}
		nodePtr = nodePtr.nextSibling;
	}

	return null;
}

function createElementFingerprint(element: Element): string {
	const attributes: Array<string> = [];
	for (let i = 0; i < element.attributes.length; i++) {
		const attr = element.attributes[i];
		if (!attr) {
			continue;
		}
		const value =
			element.hasAttribute(attr.name) && attr.value === ""
				? ""
				: attr.value;
		attributes.push(`${attr.name}="${value}"`);
	}
	attributes.sort();
	return `${element.tagName.toUpperCase()}|${attributes.join(",")}|${(element.innerHTML || "").trim()}`;
}

function buildDedupedElementsFromBlocks(blocks: Array<HeadEl>): Array<Element> {
	const newElements: Array<Element> = [];
	const newElementFingerprints = new Map<string, Element>();

	for (const block of blocks) {
		if (!block.tag) {
			continue;
		}
		const newEl = document.createElement(block.tag);
		if (block.attributesKnownSafe) {
			for (const key of Object.keys(block.attributesKnownSafe)) {
				const value = block.attributesKnownSafe[key];
				if (value === null || value === undefined) {
					panic(
						`Attribute value for '${key}' in tag '${block.tag}' cannot be null or undefined.`,
					);
				}
				newEl.setAttribute(key, value);
			}
		}
		if (block.booleanAttributes) {
			for (const key of block.booleanAttributes) {
				newEl.setAttribute(key, "");
			}
		}
		if (block.dangerousInnerHTML) {
			newEl.innerHTML = block.dangerousInnerHTML;
		}

		const fingerprint = createElementFingerprint(newEl);
		if (newElementFingerprints.has(fingerprint)) {
			const elementToRemove = newElementFingerprints.get(fingerprint);
			if (elementToRemove) {
				const indexToRemove = newElements.indexOf(elementToRemove);
				if (indexToRemove > -1) {
					newElements.splice(indexToRemove, 1);
				}
			}
		}
		newElements.push(newEl);
		newElementFingerprints.set(fingerprint, newEl);
	}

	return newElements;
}

function buildCurrentElementsMap(
	currentElements: Array<Element>,
): Map<string, Array<Element>> {
	const currentElementsMap = new Map<string, Array<Element>>();

	for (const el of currentElements) {
		const fingerprint = createElementFingerprint(el);
		if (!currentElementsMap.has(fingerprint)) {
			currentElementsMap.set(fingerprint, []);
		}
		currentElementsMap.get(fingerprint)?.push(el);
	}

	return currentElementsMap;
}

function reconcileHeadElements(props: {
	currentElements: Array<Element>;
	newElements: Array<Element>;
}): { finalElements: Array<Element>; usedCurrentElements: Set<Element> } {
	const { currentElements, newElements } = props;
	const currentElementsMap = buildCurrentElementsMap(currentElements);
	const finalElements: Array<Element> = [];
	const usedCurrentElements = new Set<Element>();

	for (const newEl of newElements) {
		const fingerprint = createElementFingerprint(newEl);
		const matchingCurrentElementsList =
			currentElementsMap.get(fingerprint) || [];

		const matchingElement = matchingCurrentElementsList.find(
			(el) => !usedCurrentElements.has(el),
		);

		if (matchingElement) {
			usedCurrentElements.add(matchingElement);
			finalElements.push(matchingElement);
		} else {
			finalElements.push(newEl);
		}
	}

	return { finalElements, usedCurrentElements };
}

function removeStaleManagedNodes(
	parent: Node,
	currentNodes: Array<Node>,
	usedCurrentElements: Set<Element>,
): void {
	for (const currentNode of currentNodes) {
		if (currentNode.nodeType !== Node.ELEMENT_NODE) {
			parent.removeChild(currentNode);
			continue;
		}

		const currentElement = currentNode as Element;
		if (!usedCurrentElements.has(currentElement)) {
			parent.removeChild(currentElement);
		}
	}
}

function placeReconciledHeadElements(props: {
	parent: Node;
	startComment: Comment;
	endComment: Comment;
	finalElements: Array<Element>;
	usedCurrentElements: Set<Element>;
}): void {
	const {
		parent,
		startComment,
		endComment,
		finalElements,
		usedCurrentElements,
	} = props;
	let lastProcessedElement: Element | null = null;

	for (let i = 0; i < finalElements.length; i++) {
		const element = finalElements[i];
		if (!element) {
			continue;
		}
		const isExistingElement = usedCurrentElements.has(element);

		if (isExistingElement) {
			const nextElementInDOM = (
				lastProcessedElement
					? lastProcessedElement.nextElementSibling
					: startComment.nextElementSibling
			) as Element | null;

			if (nextElementInDOM !== element) {
				parent.insertBefore(element, nextElementInDOM || endComment);
			}

			lastProcessedElement = element;
		} else {
			const insertBefore = lastProcessedElement
				? lastProcessedElement.nextSibling
				: startComment.nextSibling;

			parent.insertBefore(element, insertBefore || endComment);
			lastProcessedElement = element;
		}
	}
}

export function updateHeadEls(type: "meta" | "rest", blocks: Array<HeadEl>) {
	const { startComment, endComment } = getStartAndEndComments(type);
	const parent = getManagedSectionParentIfValid({
		startComment,
		endComment,
	});
	if (!parent || !startComment || !endComment) {
		panic(
			`Managed head section markers for '${type}' are missing or invalid.`,
		);
	}

	const currentNodes: Array<Node> = [];
	let nodePtr = startComment.nextSibling;
	while (nodePtr != null && nodePtr !== endComment) {
		currentNodes.push(nodePtr);
		nodePtr = nodePtr.nextSibling;
	}
	const currentElements = currentNodes.filter(
		(node): node is Element => node.nodeType === Node.ELEMENT_NODE,
	);

	const newElements = buildDedupedElementsFromBlocks(blocks);
	const { finalElements, usedCurrentElements } = reconcileHeadElements({
		currentElements,
		newElements,
	});
	removeStaleManagedNodes(parent, currentNodes, usedCurrentElements);
	placeReconciledHeadElements({
		parent,
		startComment,
		endComment,
		finalElements,
		usedCurrentElements,
	});
}

/////////////////////////////////////////////////////////////////////
/////// Render Commit Runtime
/////////////////////////////////////////////////////////////////////

const inFlightCSSPreloadPromiseByHref = new Map<string, Promise<void>>();

function preloadModule(url: string): void {
	const href = resolvePublicHref(url);
	if (
		document.querySelector(
			`link[rel="modulepreload"][href="${CSS.escape(href)}"]`,
		)
	) {
		return;
	}

	const link = document.createElement("link");
	link.rel = "modulepreload";
	link.href = href;
	document.head.appendChild(link);
}

function preloadCSS(url: string): Promise<void> {
	const href = resolvePublicHref(url);
	const existingInFlightPromise = inFlightCSSPreloadPromiseByHref.get(href);
	if (existingInFlightPromise) {
		return existingInFlightPromise;
	}

	if (
		document.querySelector(
			`link[rel="preload"][href="${CSS.escape(href)}"]`,
		)
	) {
		return Promise.resolve();
	}

	const link = document.createElement("link");
	link.rel = "preload";
	link.setAttribute("as", "style");
	link.href = href;

	const preloadPromise = new Promise<void>((resolve, reject) => {
		link.onload = () => {
			inFlightCSSPreloadPromiseByHref.delete(href);
			resolve();
		};
		link.onerror = (event) => {
			inFlightCSSPreloadPromiseByHref.delete(href);
			reject(event);
		};
	});
	inFlightCSSPreloadPromiseByHref.set(href, preloadPromise);
	document.head.appendChild(link);

	return preloadPromise;
}

function applyCSS(bundles: string[]): void {
	window.requestAnimationFrame(() => {
		for (const bundle of bundles) {
			if (
				document.querySelector(
					`link[data-vorma-css-bundle="${bundle}"]`,
				)
			) {
				continue;
			}

			const link = document.createElement("link");
			link.rel = "stylesheet";
			link.href = resolvePublicHref(bundle);
			link.setAttribute("data-vorma-css-bundle", bundle);
			document.head.appendChild(link);
		}
	});
}

export const AssetManager = {
	preloadModule,
	preloadCSS,
	applyCSS,
};

function runHistoryAndDeriveScrollState(props: {
	navigationType: VormaNavigationType;
	runHistoryOptions?: RenderingHistoryOptions;
}): ScrollState | undefined {
	const { navigationType, runHistoryOptions } = props;
	let scrollStateToDispatch: ScrollState | undefined;

	if (runHistoryOptions) {
		const { href, scrollStateToRestore, replace, scrollToTop } =
			runHistoryOptions;
		const hash = hashFragmentFromHref(href);
		const history = HistoryManager.getInstance();

		if (
			navigationType === "userNavigation" ||
			navigationType === "redirect"
		) {
			const currentHref = window.location.href;
			const isSameLocation =
				classifyNavigationTargetAgainstCurrentLocation({
					targetHref: href,
					currentHref: currentHref,
				}) === "same-document-noop";

			if (!isSameLocation && !replace) {
				history.push(href, runHistoryOptions.state);
			} else {
				history.replace(href, runHistoryOptions.state);
			}

			scrollStateToDispatch = hash
				? { hash }
				: scrollToTop !== false
					? { x: 0, y: 0 }
					: undefined;
		}

		if (navigationType === "browserHistory") {
			scrollStateToDispatch =
				scrollStateToRestore ?? (hash ? { hash } : undefined);
		}
	}

	return scrollStateToDispatch;
}

function applyRouteDocumentTitle(
	title: CanonicalRouteDataPayload["title"],
): void {
	if (title === undefined) {
		return;
	}

	const tempTxt = document.createElement("textarea");
	tempTxt.innerHTML = title?.dangerousInnerHTML || "";
	if (document.title !== tempTxt.value) {
		document.title = tempTxt.value;
	}
}

function applyRouteHeadElements(json: CanonicalRouteDataPayload): void {
	if (json.metaHeadEls !== undefined) {
		updateHeadEls("meta", json.metaHeadEls ?? []);
	}
	if (json.restHeadEls !== undefined) {
		updateHeadEls("rest", json.restHeadEls ?? []);
	}
}

function canCommitRender(props: {
	shouldCommit: RerenderAppProps["shouldCommit"];
}): boolean {
	if (!props.shouldCommit) {
		return true;
	}
	return props.shouldCommit();
}

function buildSnapshotWithCommittedClientLoaders(props: {
	previousSnapshot: RuntimeRouteSnapshot;
	clientLoadersResult: ClientLoadersResult | undefined;
}): RuntimeRouteSnapshot {
	const { previousSnapshot, clientLoadersResult } = props;
	if (!clientLoadersResult) {
		return previousSnapshot;
	}

	const normalizedClientLoaderData = clientLoadersResult.data;
	const outermostClientErrorIdx = clientLoadersResult.errorMessage
		? normalizedClientLoaderData.length > 0
			? normalizedClientLoaderData.length - 1
			: undefined
		: undefined;

	return {
		...previousSnapshot,
		clientLoadersData: normalizedClientLoaderData,
		outermostClientErrorIdx,
		outermostClientError: clientLoadersResult.errorMessage,
	};
}

function buildCommittedRuntimeRouteSnapshot(props: {
	previousSnapshot: RuntimeRouteSnapshot;
	json: CanonicalRouteDataPayload;
	modulesMap: ComponentModulesMap;
	clientLoadersResult: ClientLoadersResult | undefined;
}): RuntimeRouteSnapshot {
	const snapshotWithCommittedClientLoaders =
		buildSnapshotWithCommittedClientLoaders({
			previousSnapshot: props.previousSnapshot,
			clientLoadersResult: props.clientLoadersResult,
		});

	const baseCommittedSnapshot: RuntimeRouteSnapshot = {
		...snapshotWithCommittedClientLoaders,
		buildID: props.previousSnapshot.buildID || "",
		outermostServerError: props.json.outermostServerError,
		outermostServerErrorIdx: props.json.outermostServerErrorIdx,
		errorExportKeys: props.json.errorExportKeys,
		matchedPatterns: props.json.matchedPatterns,
		loadersData: props.json.loadersData,
		importURLs: props.json.importURLs,
		exportKeys: props.json.exportKeys,
		hasRootData: props.json.hasRootData,
		params: props.json.params,
		splatValues: props.json.splatValues,
		activeErrorBoundary: undefined,
		activeComponents: buildActiveComponentsFromModules({
			importURLs: props.json.importURLs,
			exportKeys: props.json.exportKeys,
			modulesMap: props.modulesMap,
		}),
	};
	const snapshotWithEffectiveErrors =
		applyEffectiveErrorDataToRuntimeRouteSnapshot(baseCommittedSnapshot);
	if (snapshotWithEffectiveErrors.outermostErrorIdx == null) {
		return {
			...snapshotWithEffectiveErrors,
			activeErrorBoundary: undefined,
		};
	}

	return {
		...snapshotWithEffectiveErrors,
		activeErrorBoundary: resolveErrorBoundaryComponentFromModules({
			errorIdx: snapshotWithEffectiveErrors.outermostErrorIdx,
			importURLs: props.json.importURLs,
			errorExportKeys: props.json.errorExportKeys,
			modulesMap: props.modulesMap,
			defaultErrorBoundary: __vormaClientGlobal.get(
				"defaultErrorBoundary",
			),
		}),
	};
}

function executeRenderCommitPipeline(props: {
	json: CanonicalRouteDataPayload;
	navigationType: VormaNavigationType;
	clientLoadersResult: ClientLoadersResult | undefined;
	runHistoryOptions?: RenderingHistoryOptions;
	modulesMap: ComponentModulesMap;
	onFinish: () => void;
}): void {
	const previousSnapshot = getRuntimeRouteSnapshot();
	const committedSnapshot = buildCommittedRuntimeRouteSnapshot({
		previousSnapshot,
		json: props.json,
		modulesMap: props.modulesMap,
		clientLoadersResult: props.clientLoadersResult,
	});
	setRuntimeRouteSnapshot(committedSnapshot);
	const scrollStateToDispatch = runHistoryAndDeriveScrollState({
		navigationType: props.navigationType,
		runHistoryOptions: props.runHistoryOptions,
	});
	applyRouteDocumentTitle(props.json.title);
	AssetManager.applyCSS(props.json.cssBundles);
	dispatchRouteChangeEvent({
		__scrollState: scrollStateToDispatch,
	});
	applyRouteHeadElements(props.json);
	props.onFinish();
}

export async function __reRenderApp(props: RerenderAppProps): Promise<void> {
	const shouldUseViewTransitions =
		__vormaClientGlobal.get("useViewTransitions") &&
		!!document.startViewTransition &&
		props.navigationType !== "prefetch" &&
		props.navigationType !== "revalidation";

	if (shouldUseViewTransitions) {
		const transition = document.startViewTransition(async () => {
			await __reRenderAppInner(props);
		});
		await transition.finished;
	} else {
		await __reRenderAppInner(props);
	}
}

async function __reRenderAppInner(props: RerenderAppProps): Promise<void> {
	const {
		json,
		navigationType,
		runHistoryOptions,
		shouldCommit,
		clientLoadersResult,
	} = props;

	if (
		!canCommitRender({
			shouldCommit,
		})
	) {
		return;
	}

	const modulesMap = await ComponentLoader.loadComponents(json.importURLs);

	if (
		!canCommitRender({
			shouldCommit,
		})
	) {
		return;
	}

	executeRenderCommitPipeline({
		json,
		navigationType,
		clientLoadersResult,
		runHistoryOptions,
		modulesMap,
		onFinish: props.onFinish,
	});
}

/**
 * Builds a once-only listener initializer for route and location updates.
 *
 * Each adapter keeps its own singleton initializer so remounts don't duplicate
 * global listeners.
 */
export function createRouteOutletRuntimeListenerInitializer(props: {
	syncStoreState: RouteOutletRuntimeStoreSyncFunction;
}): RouteOutletRuntimeListenerInitializer {
	const { syncStoreState } = props;
	let isInitialized = false;

	return () => {
		if (isInitialized) {
			return;
		}
		isInitialized = true;

		addRouteChangeListener((event: RouteChangeEvent) => {
			syncStoreState();
			window.requestAnimationFrame(() => {
				applyScrollState(event.detail.__scrollState);
			});
		});

		addLocationListener(() => {
			syncStoreState();
		});
	};
}

function areRouteOutletNavigationStatesReferenceEqual(props: {
	previousNavigationState: RouteOutletNavigationState;
	nextNavigationState: RouteOutletNavigationState;
}): boolean {
	const { previousNavigationState, nextNavigationState } = props;
	return (
		nextNavigationState.loadersData ===
			previousNavigationState.loadersData &&
		nextNavigationState.clientLoadersData ===
			previousNavigationState.clientLoadersData &&
		nextNavigationState.routerData === previousNavigationState.routerData &&
		nextNavigationState.outermostError ===
			previousNavigationState.outermostError &&
		nextNavigationState.outermostErrorIdx ===
			previousNavigationState.outermostErrorIdx &&
		nextNavigationState.activeComponents ===
			previousNavigationState.activeComponents &&
		nextNavigationState.activeErrorBoundary ===
			previousNavigationState.activeErrorBoundary &&
		nextNavigationState.importURLs === previousNavigationState.importURLs &&
		nextNavigationState.exportKeys === previousNavigationState.exportKeys
	);
}

export function buildCurrentRouteOutletNavigationState(): RouteOutletNavigationState {
	const runtimeRenderState = getClientRuntimeRenderState();
	return {
		loadersData: runtimeRenderState.loadersData,
		clientLoadersData: runtimeRenderState.clientLoadersData,
		routerData: getRouterData(),
		outermostError: runtimeRenderState.outermostError,
		outermostErrorIdx: runtimeRenderState.outermostErrorIdx,
		activeComponents: runtimeRenderState.activeComponents,
		activeErrorBoundary: runtimeRenderState.activeErrorBoundary,
		importURLs: runtimeRenderState.importURLs,
		exportKeys: runtimeRenderState.exportKeys,
	};
}

export function buildInitialRouteOutletNavigationState(): RouteOutletNavigationState {
	return buildCurrentRouteOutletNavigationState();
}

export function buildNextRouteOutletNavigationState(
	previousNavigationState: RouteOutletNavigationState,
): RouteOutletNavigationState {
	const nextNavigationState = buildCurrentRouteOutletNavigationState();

	if (
		areRouteOutletNavigationStatesReferenceEqual({
			previousNavigationState,
			nextNavigationState,
		})
	) {
		return previousNavigationState;
	}

	return nextNavigationState;
}

export function areRouteOutletLocationsEqual(props: {
	firstLocationState: RouteOutletLocationState;
	secondLocationState: RouteOutletLocationState;
}): boolean {
	const { firstLocationState, secondLocationState } = props;
	return (
		Object.is(firstLocationState.pathname, secondLocationState.pathname) &&
		Object.is(firstLocationState.search, secondLocationState.search) &&
		Object.is(firstLocationState.hash, secondLocationState.hash) &&
		Object.is(firstLocationState.state, secondLocationState.state)
	);
}

export function buildCurrentRouteOutletLocationState(): RouteOutletLocationState {
	return getRuntimeLocationState();
}

export function buildRouteOutletRouteKey(props: {
	importURLs: ReadonlyArray<string> | null | undefined;
	exportKeys: ReadonlyArray<string> | null | undefined;
	matchedPatterns: ReadonlyArray<string> | null | undefined;
	idx: number;
}): string {
	const importURL = props.importURLs?.[props.idx] || "";
	const exportKey = props.exportKeys?.[props.idx] || "";
	const matchedPattern = props.matchedPatterns?.[props.idx] || "";
	return JSON.stringify([props.idx, importURL, exportKey, matchedPattern]);
}

export function buildRouteOutletBranchInputState(
	navigationState: RouteOutletNavigationState,
): RouteOutletBranchInputState {
	return {
		loaderCount: navigationState.loadersData?.length ?? 0,
		outermostErrorIdx: navigationState.outermostErrorIdx,
		activeComponents: navigationState.activeComponents,
		activeErrorBoundary: navigationState.activeErrorBoundary,
		importURLs: navigationState.importURLs,
		exportKeys: navigationState.exportKeys,
		matchedPatterns: navigationState.routerData.matchedPatterns,
	};
}

export function areRouteOutletBranchInputsEqualByIdentity(props: {
	firstInputState: RouteOutletBranchInputState;
	secondInputState: RouteOutletBranchInputState;
}): boolean {
	const { firstInputState, secondInputState } = props;
	return (
		firstInputState.loaderCount === secondInputState.loaderCount &&
		Object.is(
			firstInputState.outermostErrorIdx,
			secondInputState.outermostErrorIdx,
		) &&
		firstInputState.activeComponents ===
			secondInputState.activeComponents &&
		Object.is(
			firstInputState.activeErrorBoundary,
			secondInputState.activeErrorBoundary,
		) &&
		firstInputState.importURLs === secondInputState.importURLs &&
		firstInputState.exportKeys === secondInputState.exportKeys &&
		firstInputState.matchedPatterns === secondInputState.matchedPatterns
	);
}

export function buildInitialRouteOutletStoreState(): RouteOutletStoreState {
	const initialNavigationState = buildInitialRouteOutletNavigationState();
	return {
		navigation: initialNavigationState,
		routeOutletBranchInputState: buildRouteOutletBranchInputState(
			initialNavigationState,
		),
		location: buildCurrentRouteOutletLocationState(),
	};
}

function resolveNextRouteOutletBranchInputState(props: {
	previousStoreState: RouteOutletStoreState;
	nextNavigationState: RouteOutletNavigationState;
}): RouteOutletBranchInputState {
	const { previousStoreState, nextNavigationState } = props;
	const previousNavigationState = previousStoreState.navigation;
	const previousRouteOutletBranchInputState =
		previousStoreState.routeOutletBranchInputState;

	if (nextNavigationState === previousNavigationState) {
		return previousRouteOutletBranchInputState;
	}

	const nextRouteOutletBranchInputState =
		buildRouteOutletBranchInputState(nextNavigationState);
	if (
		areRouteOutletBranchInputsEqualByIdentity({
			firstInputState: previousRouteOutletBranchInputState,
			secondInputState: nextRouteOutletBranchInputState,
		})
	) {
		return previousRouteOutletBranchInputState;
	}

	return nextRouteOutletBranchInputState;
}

function resolveNextRouteOutletLocationState(props: {
	previousLocationState: RouteOutletLocationState;
}): RouteOutletLocationState {
	const { previousLocationState } = props;
	const nextLocationState = buildCurrentRouteOutletLocationState();
	if (
		areRouteOutletLocationsEqual({
			firstLocationState: previousLocationState,
			secondLocationState: nextLocationState,
		})
	) {
		return previousLocationState;
	}

	return nextLocationState;
}

export function buildNextRouteOutletStoreStateFromRuntime(
	previousStoreState: RouteOutletStoreState,
): RouteOutletStoreState {
	const nextNavigationState = buildNextRouteOutletNavigationState(
		previousStoreState.navigation,
	);

	const nextRouteOutletBranchInputState =
		resolveNextRouteOutletBranchInputState({
			previousStoreState,
			nextNavigationState,
		});
	const nextLocationState = resolveNextRouteOutletLocationState({
		previousLocationState: previousStoreState.location,
	});

	if (
		nextNavigationState === previousStoreState.navigation &&
		nextRouteOutletBranchInputState ===
			previousStoreState.routeOutletBranchInputState &&
		nextLocationState === previousStoreState.location
	) {
		return previousStoreState;
	}

	return {
		navigation: nextNavigationState,
		routeOutletBranchInputState: nextRouteOutletBranchInputState,
		location: nextLocationState,
	};
}

export function syncRouteOutletStoreStateFromRuntime(props: {
	getCurrentStoreState: () => RouteOutletStoreState;
	applyNextStoreState: (nextStoreState: RouteOutletStoreState) => void;
}): RouteOutletStoreState {
	const previousStoreState = props.getCurrentStoreState();
	const nextStoreState =
		buildNextRouteOutletStoreStateFromRuntime(previousStoreState);
	if (nextStoreState !== previousStoreState) {
		props.applyNextStoreState(nextStoreState);
	}
	return nextStoreState;
}

export function buildRouteOutletBranchState(props: {
	navigationState: RouteOutletBranchInputState;
	idx: number;
}): RouteOutletBranchState {
	const { navigationState, idx } = props;
	const { importURLs, exportKeys } = navigationState;
	const isErrorIdx = idx === navigationState.outermostErrorIdx;
	const currentComponent = isErrorIdx
		? undefined
		: navigationState.activeComponents?.[idx];
	const errorComponent = isErrorIdx
		? navigationState.activeErrorBoundary
		: undefined;
	const loaderCount = navigationState.loaderCount;
	const shouldFallbackOutlet =
		!isErrorIdx && !currentComponent && idx + 1 < loaderCount;

	return {
		currentRouteKey: buildRouteOutletRouteKey({
			importURLs,
			exportKeys,
			matchedPatterns: navigationState.matchedPatterns,
			idx,
		}),
		nextRouteKey: buildRouteOutletRouteKey({
			importURLs,
			exportKeys,
			matchedPatterns: navigationState.matchedPatterns,
			idx: idx + 1,
		}),
		isErrorIdx,
		currentComponent,
		errorComponent,
		shouldFallbackOutlet,
	};
}

export function resolveRouteOutletBranchRenderState(props: {
	branchState: RouteOutletBranchState;
}): RouteOutletBranchRenderState {
	const { branchState } = props;
	const routeKeys: RouteOutletBranchRouteKeys = {
		currentRouteKey: branchState.currentRouteKey,
		nextRouteKey: branchState.nextRouteKey,
	};

	if (branchState.isErrorIdx) {
		return {
			renderKind: "error",
			errorComponent: branchState.errorComponent,
			...routeKeys,
		};
	}

	if (branchState.currentComponent) {
		return {
			renderKind: "component",
			currentComponent: branchState.currentComponent,
			...routeKeys,
		};
	}

	if (branchState.shouldFallbackOutlet) {
		return {
			renderKind: "fallback",
			...routeKeys,
		};
	}

	return {
		renderKind: "empty",
		...routeKeys,
	};
}

export function shouldRemountRouteOutletComponentMount(props: {
	idx: number;
	previousRouteKey: string | undefined;
	nextRouteKey: string;
	previousRouteComponent: unknown;
	nextRouteComponent: unknown;
}): boolean {
	const {
		idx,
		previousRouteKey,
		nextRouteKey,
		previousRouteComponent,
		nextRouteComponent,
	} = props;
	void idx;
	const didRouteComponentIdentityChange =
		previousRouteComponent !== undefined &&
		previousRouteComponent !== nextRouteComponent;
	if (didRouteComponentIdentityChange) {
		return true;
	}

	const didRouteKeyChange =
		typeof previousRouteKey === "string" &&
		previousRouteKey !== nextRouteKey;
	return didRouteKeyChange;
}

/////////////////////////////////////////////////////////////////////
/////// Typed Adapter Route Data Helpers
/////////////////////////////////////////////////////////////////////

const typedAdapterRouteScopeMarker = Symbol("vorma_typed_adapter_route_scope");

export const typedAdapterInternalRoutePropsRouteScopePropName =
	"__vorma_internal_route_scope";

function resolveTypedAdapterMatchedPatternIndex(props: {
	matchedPatterns: readonly string[];
	pattern: string;
}): number {
	const { matchedPatterns, pattern } = props;
	return matchedPatterns.findIndex(
		(matchedPattern) => matchedPattern === pattern,
	);
}

function isTypedAdapterRouteScope(
	value: unknown,
): value is TypedAdapterRouteScope {
	if (typeof value !== "object" || value === null) {
		return false;
	}
	return (
		(value as Record<PropertyKey, unknown>)[
			typedAdapterRouteScopeMarker
		] === true
	);
}

function resolveTypedAdapterRouteScopeOrThrow(props: {
	hookName: "useLoaderData" | "useClientLoaderData";
	routeProps: VormaTypedAdapterRoutePropsWithIndex;
}): TypedAdapterRouteScope {
	const maybeRouteScope =
		props.routeProps[typedAdapterInternalRoutePropsRouteScopePropName];
	if (!isTypedAdapterRouteScope(maybeRouteScope)) {
		throw new Error(
			`${props.hookName}(routeProps) contract violated: route scope is missing or invalid.`,
		);
	}

	const routePropsIndex = props.routeProps.idx;
	if (
		!Number.isInteger(routePropsIndex) ||
		routePropsIndex !== maybeRouteScope.boundRoutePropsIndex
	) {
		throw new Error(
			`${props.hookName}(routeProps) contract violated: routeProps.idx changed after route scope binding.`,
		);
	}

	return maybeRouteScope;
}

export function createTypedAdapterRouteScope(props: {
	routePropsIndex: number;
	matchedPattern: string;
}): unknown {
	const routePropsIndex = props.routePropsIndex;
	if (!Number.isInteger(routePropsIndex) || routePropsIndex < 0) {
		throw new Error(
			"Vorma route scope initialization violated: route index must be a non-negative integer.",
		);
	}
	if (
		typeof props.matchedPattern !== "string" ||
		props.matchedPattern === ""
	) {
		throw new Error(
			"Vorma route scope initialization violated: matched pattern is missing at route index.",
		);
	}

	return Object.freeze({
		[typedAdapterRouteScopeMarker]: true,
		boundRoutePropsIndex: routePropsIndex,
		boundMatchedPattern: props.matchedPattern,
	} satisfies TypedAdapterRouteScope);
}

export function buildTypedAdapterRoutePropsWithInternalRouteScope(props: {
	routeScope: unknown;
}): Record<typeof typedAdapterInternalRoutePropsRouteScopePropName, unknown> {
	return {
		[typedAdapterInternalRoutePropsRouteScopePropName]: props.routeScope,
	};
}

export function resolveTypedAdapterLoaderDataForRoutePropsOrThrow<Data>(props: {
	routeProps: VormaTypedAdapterRoutePropsWithIndex;
	loadersData: readonly unknown[];
}): Data {
	const routeScope = resolveTypedAdapterRouteScopeOrThrow({
		hookName: "useLoaderData",
		routeProps: props.routeProps,
	});

	const routePropsIndex = routeScope.boundRoutePropsIndex;
	if (routePropsIndex >= props.loadersData.length) {
		throw new Error(
			"useLoaderData(routeProps) contract violated: no route-scoped loader snapshot is available.",
		);
	}

	return props.loadersData[routePropsIndex] as Data;
}

export function resolveTypedAdapterIndexedDataForPattern<Data>(props: {
	pattern: string;
	matchedPatterns: readonly string[];
	indexedData: readonly unknown[];
}): Data | undefined {
	const idx = resolveTypedAdapterMatchedPatternIndex({
		matchedPatterns: props.matchedPatterns,
		pattern: props.pattern,
	});
	if (idx === -1) {
		return undefined;
	}
	return props.indexedData[idx] as Data | undefined;
}

export function resolveTypedAdapterClientLoaderDataForPatternOrRouteProps<
	Data,
>(props: {
	pattern: string;
	routeProps?: VormaTypedAdapterRoutePropsWithIndex;
	matchedPatterns: readonly string[];
	clientLoadersData: readonly unknown[];
}): Data | undefined {
	if (!props.routeProps) {
		return resolveTypedAdapterIndexedDataForPattern<Data>({
			pattern: props.pattern,
			matchedPatterns: props.matchedPatterns,
			indexedData: props.clientLoadersData,
		});
	}

	const routeScope = resolveTypedAdapterRouteScopeOrThrow({
		hookName: "useClientLoaderData",
		routeProps: props.routeProps,
	});

	if (routeScope.boundMatchedPattern !== props.pattern) {
		throw new Error(
			`useClientLoaderData(routeProps) contract violated for pattern "${props.pattern}": route scope is bound to pattern "${routeScope.boundMatchedPattern}".`,
		);
	}

	const routePropsIndex = routeScope.boundRoutePropsIndex;
	if (routePropsIndex >= props.clientLoadersData.length) {
		throw new Error(
			`useClientLoaderData(routeProps) contract violated for pattern "${props.pattern}": no route-scoped client-loader snapshot is available.`,
		);
	}

	return props.clientLoadersData[routePropsIndex] as Data | undefined;
}

/**
 * Creates shared typed hook factories used by UI adapters that read current
 * loader/client-loader snapshots from adapter-managed stores.
 */
export function createTypedAdapterValueHookFactories<
	App extends VormaAppBase,
>(props: {
	useLoadersData: () => readonly unknown[];
	useMatchedPatterns: () => readonly string[];
	useClientLoadersData: () => readonly unknown[];
	useMemoizedPatternLoaderData?: <Data>(props: {
		resolvePatternLoaderData: () => Data;
		dependencies: readonly unknown[];
	}) => Data;
}) {
	const makeUseLoaderDataHook = () => {
		return function useLoaderData<Pattern extends VormaLoaderPattern<App>>(
			routeProps: VormaTypedAdapterRoutePropsWithIndex,
		): VormaLoaderOutput<App, Pattern> {
			return resolveTypedAdapterLoaderDataForRoutePropsOrThrow<
				VormaLoaderOutput<App, Pattern>
			>({
				routeProps,
				loadersData: props.useLoadersData(),
			});
		};
	};

	const makeUsePatternLoaderDataHook = () => {
		return function usePatternLoaderData<
			Pattern extends VormaLoaderPattern<App>,
		>(pattern: Pattern): VormaLoaderOutput<App, Pattern> | undefined {
			const loadersData = props.useLoadersData();
			const matchedPatterns = props.useMatchedPatterns();
			const resolvePatternLoaderData = () => {
				return resolveTypedAdapterIndexedDataForPattern<
					VormaLoaderOutput<App, Pattern>
				>({
					pattern,
					matchedPatterns,
					indexedData: loadersData,
				});
			};
			if (!props.useMemoizedPatternLoaderData) {
				return resolvePatternLoaderData();
			}
			return props.useMemoizedPatternLoaderData({
				resolvePatternLoaderData,
				dependencies: [loadersData, pattern, matchedPatterns],
			});
		};
	};

	const makeAddClientLoaderHook = () => {
		return function addClientLoader<
			Pattern extends VormaLoaderPattern<App>,
			LoaderData extends VormaLoaderOutput<App, Pattern>,
			T = any,
		>(
			clientLoaderProps: VormaTypedAdapterAddClientLoaderProps<
				App,
				Pattern,
				LoaderData,
				T
			>,
		) {
			// Re-registering keeps the latest adapter-provided client loader authoritative.
			registerClientLoaderForAdapter({
				pattern: clientLoaderProps.pattern as string,
				waitFn: clientLoaderProps.clientLoader as any,
			});
			type Res = Awaited<
				ReturnType<typeof clientLoaderProps.clientLoader>
			>;
			const useClientLoaderData = (
				routeProps?: VormaTypedAdapterRoutePropsWithIndex,
			): Res | undefined => {
				return resolveTypedAdapterClientLoaderDataForPatternOrRouteProps<Res>(
					{
						pattern: clientLoaderProps.pattern,
						routeProps,
						matchedPatterns: props.useMatchedPatterns(),
						clientLoadersData: props.useClientLoadersData(),
					},
				);
			};
			return useClientLoaderData as {
				(props: VormaTypedAdapterRoutePropsWithIndex): Res;
				(): Res | undefined;
			};
		};
	};

	return {
		makeUseLoaderDataHook,
		makeUsePatternLoaderDataHook,
		makeAddClientLoaderHook,
	};
}

/////////////////////////////////////////////////////////////////////
/////// Route Outlet Adapter Host Runtime
/////////////////////////////////////////////////////////////////////

/**
 * Returns whether a root-outlet mount should run shared listener/store sync.
 */
export function shouldSyncRouteOutletRootMount(props: {
	idx: number;
}): boolean {
	return props.idx === 0;
}

/**
 * Creates shared runtime-sync wiring used by all UI adapters.
 * This keeps listener initialization and store synchronization behavior identical.
 */
export function createRouteOutletAdapterSyncHost(props: {
	getCurrentStoreState: () => RouteOutletStoreState;
	applyNextStoreState: (nextStoreState: RouteOutletStoreState) => void;
}): RouteOutletAdapterSyncHost {
	const syncStoreState = (): void => {
		syncRouteOutletStoreStateFromRuntime({
			getCurrentStoreState: props.getCurrentStoreState,
			applyNextStoreState: props.applyNextStoreState,
		});
	};
	const initUIListeners = createRouteOutletRuntimeListenerInitializer({
		syncStoreState,
	});
	const syncRootOutletMount = (rootOutletMount: { idx: number }): void => {
		const shouldSyncRootOutletMount = shouldSyncRouteOutletRootMount({
			idx: rootOutletMount.idx,
		});
		if (!shouldSyncRootOutletMount) {
			return;
		}
		initUIListeners();
		syncStoreState();
	};
	return {
		syncStoreState,
		initUIListeners,
		syncRootOutletMount,
	};
}

/**
 * Resolves adapter render model for one outlet index.
 * This unifies branch kind resolution, matched-pattern selection, and error payload wiring.
 */
export function resolveRouteOutletAdapterRenderModel(props: {
	routeOutletBranchInputState: RouteOutletBranchInputState;
	outermostError: unknown;
	idx: number;
}): RouteOutletAdapterRenderModel {
	const branchState = buildRouteOutletBranchState({
		navigationState: props.routeOutletBranchInputState,
		idx: props.idx,
	});
	const branchRenderState = resolveRouteOutletBranchRenderState({
		branchState,
	});
	if (branchRenderState.renderKind === "component") {
		const matchedPattern =
			props.routeOutletBranchInputState.matchedPatterns[props.idx];
		if (typeof matchedPattern !== "string" || matchedPattern.length === 0) {
			throw new Error(
				`Route outlet adapter render model contract violated: missing matched pattern at index ${props.idx}.`,
			);
		}
		return {
			...branchRenderState,
			matchedPattern,
			outermostError: props.outermostError,
		};
	}
	return {
		...branchRenderState,
		outermostError: props.outermostError,
	};
}

/**
 * Renders one adapter outlet branch from the shared render model.
 * Adapters provide framework-specific render callbacks only.
 */
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
		matchedPattern: string;
	}) => RenderOutput;
	renderMissingComponent: () => RenderOutput;
	renderFallback: (props: { nextRouteKey: string }) => RenderOutput;
	renderEmpty: () => RenderOutput;
}): RenderOutput {
	if (props.routeOutletRenderModel.renderKind === "error") {
		const maybeErrorComponent = props.routeOutletRenderModel.errorComponent;
		if (maybeErrorComponent) {
			return props.renderErrorWithBoundary({
				errorComponent: maybeErrorComponent,
				outermostError: props.routeOutletRenderModel.outermostError,
			});
		}
		return props.renderErrorWithoutBoundary({
			outermostError: props.routeOutletRenderModel.outermostError,
		});
	}

	if (props.routeOutletRenderModel.renderKind === "component") {
		const maybeCurrentComponent =
			props.routeOutletRenderModel.currentComponent;
		if (!maybeCurrentComponent) {
			return props.renderMissingComponent();
		}
		return props.renderComponent({
			currentComponent: maybeCurrentComponent,
			currentRouteKey: props.routeOutletRenderModel.currentRouteKey,
			matchedPattern: props.routeOutletRenderModel.matchedPattern,
		});
	}

	if (props.routeOutletRenderModel.renderKind === "fallback") {
		return props.renderFallback({
			nextRouteKey: props.routeOutletRenderModel.nextRouteKey,
		});
	}

	return props.renderEmpty();
}

/**
 * Builds typed route-props payload bound to one route scope.
 * All adapter mounts must use this shared binding seam.
 */
export function buildTypedAdapterRouteComponentMountProps(props: {
	routePropsIndex: number;
	matchedPattern: string;
}): Record<string, unknown> {
	const routeScope = createTypedAdapterRouteScope({
		routePropsIndex: props.routePropsIndex,
		matchedPattern: props.matchedPattern,
	});
	return buildTypedAdapterRoutePropsWithInternalRouteScope({
		routeScope,
	});
}

export function buildTypedLinkHrefForRouteResolution(props: {
	vormaAppConfig: VormaAppConfig;
	routeResolutionInput: TypedLinkRouteResolutionInput;
}): string {
	const { routeResolutionInput } = props;

	return resolveTypedLinkHref({
		vormaAppConfig: props.vormaAppConfig,
		pattern: routeResolutionInput.pattern,
		...(routeResolutionInput.params && {
			params: routeResolutionInput.params,
		}),
		...(routeResolutionInput.splatValues && {
			splatValues: routeResolutionInput.splatValues,
		}),
		search: routeResolutionInput.search,
		hash: routeResolutionInput.hash,
	});
}

export function buildTypedLinkResolvedProps<
	MergedProps extends TypedLinkMergedPropsBase,
>(props: {
	vormaAppConfig: VormaAppConfig;
	mergedProps: MergedProps;
}): {
	href: string;
	state: MergedProps["state"];
	linkProps: TypedLinkResolvedNonRouteProps<MergedProps>;
} {
	const { pattern, params, splatValues, search, hash, state, ...linkProps } =
		props.mergedProps;

	const href = buildTypedLinkHrefForRouteResolution({
		vormaAppConfig: props.vormaAppConfig,
		routeResolutionInput: {
			pattern,
			params,
			splatValues,
			search,
			hash,
		},
	});

	return {
		href,
		state,
		linkProps,
	};
}

export function buildTypedLinkDisplayName(props: {
	defaultProps: Record<string, unknown> | undefined;
}): string {
	return `TypedLink(${Object.keys(props.defaultProps || {}).join(", ")})`;
}

/////////////////////////////////////////////////////////////////////
/////// Typed Adapter Link Props
/////////////////////////////////////////////////////////////////////

export const navigationInternalLinkPropKeysForAnchors = [
	"prefetch",
	"prefetchDelayMs",
	"beforeBegin",
	"beforeRender",
	"afterRender",
	"scrollToTop",
	"replace",
	"state",
] as const;

function stripNavigationInternalLinkPropsForAnchor<AnchorProps extends object>(
	props: AnchorProps,
): Omit<AnchorProps, NavigationInternalLinkPropKey> {
	const nextProps = { ...(props as Record<string, unknown>) };
	for (const key of navigationInternalLinkPropKeysForAnchors) {
		delete nextProps[key];
	}
	return nextProps as Omit<AnchorProps, NavigationInternalLinkPropKey>;
}

export function buildNavigationLinkAnchorRenderProps<
	AnchorProps extends object,
	LinkEvent,
>(
	props: AnchorProps & VormaLinkPropsBase<LinkEvent>,
): {
	safeAnchorProps: Omit<AnchorProps, NavigationInternalLinkPropKey>;
	dataExternal: boolean | undefined;
	onPointerEnter: (event: LinkEvent) => void;
	onFocus: (event: LinkEvent) => void;
	onPointerLeave: (event: LinkEvent) => void;
	onBlur: (event: LinkEvent) => void;
	onTouchCancel: (event: LinkEvent) => void;
	onClick: (event: LinkEvent) => Promise<void>;
} {
	const finalLinkProps = makeFinalLinkProps(props);
	const safeAnchorProps = stripNavigationInternalLinkPropsForAnchor(props);
	return {
		safeAnchorProps,
		dataExternal: finalLinkProps.dataExternal,
		onPointerEnter: finalLinkProps.onPointerEnter,
		onFocus: finalLinkProps.onFocus,
		onPointerLeave: finalLinkProps.onPointerLeave,
		onBlur: finalLinkProps.onBlur,
		onTouchCancel: finalLinkProps.onTouchCancel,
		onClick: finalLinkProps.onClick,
	};
}

/////////////////////////////////////////////////////////////////////
/////// Typed Adapter Link Factory
/////////////////////////////////////////////////////////////////////

/**
 * Creates one typed-link component factory shared by all UI adapters.
 * Adapters provide only the final render glue for their framework.
 */
export function createTypedAdapterLinkFactory<
	App extends VormaAppBase,
	AnchorProps,
	LinkEvent,
	RenderOutput,
>(props: {
	vormaAppConfig: VormaAppConfig;
	defaultProps:
		| TypedAdapterLinkDefaultProps<App, AnchorProps, LinkEvent>
		| undefined;
	renderTypedLink: <Pattern extends VormaLoaderPattern<App>>(props: {
		linkProps: TypedAdapterLinkProps<App, Pattern, AnchorProps, LinkEvent>;
		resolveTypedLinkProps: () => {
			href: string;
			state: unknown;
			linkProps: TypedLinkResolvedNonRouteProps<
				TypedAdapterLinkProps<App, Pattern, AnchorProps, LinkEvent>
			>;
		};
	}) => RenderOutput;
}): <Pattern extends VormaLoaderPattern<App>>(
	linkProps: TypedAdapterLinkProps<App, Pattern, AnchorProps, LinkEvent>,
) => RenderOutput {
	const TypedLink = <Pattern extends VormaLoaderPattern<App>>(
		linkProps: TypedAdapterLinkProps<App, Pattern, AnchorProps, LinkEvent>,
	): RenderOutput => {
		const resolveTypedLinkProps = () => {
			const mergedProps = {
				...props.defaultProps,
				...linkProps,
			} as TypedAdapterLinkProps<App, Pattern, AnchorProps, LinkEvent>;
			return buildTypedLinkResolvedProps({
				vormaAppConfig: props.vormaAppConfig,
				mergedProps,
			});
		};

		return props.renderTypedLink({
			linkProps,
			resolveTypedLinkProps,
		});
	};

	(TypedLink as any).displayName = buildTypedLinkDisplayName({
		defaultProps: props.defaultProps as Record<string, unknown> | undefined,
	});

	return TypedLink;
}
