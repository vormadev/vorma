import type { MutationError, QueryError } from "./api_client.ts";

/////// PRIMITIVE TYPES

type ViewBase = {
	params?: ReadonlyArray<string>;
	parents?: ReadonlyArray<string>;
	pattern: string;
	__i?: unknown;
	__o?: unknown;
};

type ApiRouteBase = {
	method: string;
	params?: ReadonlyArray<string>;
	pattern: string;
	kind?: ApiRouteKind;
	__i?: unknown;
	__o?: unknown;
};

export type ApiRouteKind = "query" | "mutation";

export type RevalidationResult =
	| { ok: true }
	| { ok: false; reason: "build_skew" | "max_retries_exhausted" };

export type RouteErrorState = {
	idx: number;
	error: unknown;
	source: "server" | "clientLoader";
};

export type RouteMatchState = {
	pattern: string;
	input: unknown;
	loaderData: unknown;
	clientLoaderData: unknown;
};

export type RouteState = {
	href: string;
	historyState: unknown;
	clientBuildId: string;
	params: Record<string, string>;
	splatValues: string[];
	matches: RouteMatchState[];
	error: RouteErrorState | null;
};

export type RouteUpdateReason = "boot" | "navigation" | "popstate" | "revalidation";

export type BeforeRouteTransitionArgs = {
	trigger: Exclude<RouteUpdateReason, "boot">;
	signal: AbortSignal;
	current: RouteState;
	next: RouteState;
};

export type BeforeRouteCommitFn = (
	args: BeforeRouteTransitionArgs,
) => void | Promise<void>;

export type BeforeRouteYieldFn = (
	args: BeforeRouteTransitionArgs,
) => void | Promise<void>;

type ApiResultBase<T> =
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

export type LinkAttributeMatchRules =
	| {
			skip?: false;
			includeSearch?: boolean;
			includeHash?: boolean;
	  }
	| {
			skip: true;
			includeSearch?: never;
			includeHash?: never;
	  };

export type LinkPropsBase = {
	prefetch?: "intent" | "none";
	prefetchDelayMs?: number;
	attributeMatchRules?: LinkAttributeMatchRules;
	visitOnPointerDown?: boolean;
	replace?: boolean;
	scrollToTop?: boolean;
	skipWorkIndicator?: boolean;
};

/////// APP CONFIG

export type AppConfig = {
	apiMountRoot: string;
	__vorma_views: readonly ViewBase[];
	__vorma_api_routes: readonly ApiRouteBase[];
};

/////// APP TYPE EXTRACTORS

type AppView<A extends AppConfig> = A["__vorma_views"][number];
type AppApiRoute<A extends AppConfig> = A["__vorma_api_routes"][number];

type ViewByPattern<A extends AppConfig, P extends string> = Extract<
	AppView<A>,
	{ pattern: P }
>;
type ApiRouteByMethodAndPattern<
	A extends AppConfig,
	M extends string,
	P extends string,
> = Extract<
	AppApiRoute<A>,
	{
		method: M;
		pattern: P;
	}
>;

type ResolvedApiRouteKind<Act> = Act extends {
	kind: infer T extends ApiRouteKind;
}
	? T
	: Act extends {
				method: "GET" | "HEAD";
		  }
		? "query"
		: "mutation";

/////// SPLAT DETECTION (pattern-based, no distribution issues)

type IsSplat<P extends string> = P extends `${string}/*` ? true : false;

type ConditionalSplat<P extends string> =
	IsSplat<P> extends true ? { splatValues: Array<string> } : {};

/////// CONDITIONAL PARAMS (non-distributive — uses the resolved Extract
/////// result directly in the extends clause so it does NOT distribute
/////// when P is a union of patterns)

type ConditionalViewParams<A extends AppConfig, P extends string> =
	ViewByPattern<A, P> extends { params: ReadonlyArray<infer Params> }
		? Params extends string
			? { params: { [K in Params]: string } }
			: {}
		: {};

type ConditionalApiRouteParams<Act> = Act extends {
	params: ReadonlyArray<infer Params>;
}
	? Params extends string
		? { params: { [K in Params]: string } }
		: {}
	: {};

/////// PARAMS RECORD (for client loader props and router data)

type ViewParamsRecord<A extends AppConfig, P extends string> =
	ViewByPattern<A, P> extends { params: ReadonlyArray<infer Params> }
		? Params extends string
			? { [K in Params]: string }
			: Record<string, string>
		: Record<string, string>;

/////// LOADER INPUT FROM PARENT VIEWS

type ViewParents<A extends AppConfig, P extends string> =
	ViewByPattern<A, P> extends {
		parents: ReadonlyArray<infer Parent>;
	}
		? Extract<Parent, ToViewPattern<A>>
		: never;

type ViewInputWithParentViews<A extends AppConfig, P extends ToViewPattern<A>> = (
	P | ViewParents<A, P> extends infer Pattern
		? Pattern extends ToViewPattern<A>
			? (input: ToLoaderInput<A, Pattern>) => void
			: never
		: never
) extends (input: infer Input) => void
	? Input
	: never;

/////// INPUT EMPTINESS

type IsEmptyInput<T> = [T] extends [null | undefined] ? true : false;

type IsUnion<T, U = T> = [T] extends [never]
	? false
	: T extends unknown
		? [U] extends [T]
			? false
			: true
		: false;

type ApiRouteInputField<Input> =
	IsEmptyInput<Input> extends true ? { input?: Input } : { input: Input };

type ApiRouteMethodsForPattern<A extends AppConfig, P extends string> = Extract<
	AppApiRoute<A>,
	{ pattern: P }
>["method"];

type ApiRouteMethodField<A extends AppConfig, P extends string, M extends string> =
	ApiRouteMethodsForPattern<A, P> extends "GET"
		? IsUnion<ApiRouteMethodsForPattern<A, P>> extends true
			? { method: M }
			: { method?: M }
		: { method: M };

/////// PERMISSIVE PATTERN (for navigate / link _index shorthand)

type PermissiveViewPattern<
	A extends AppConfig,
	P extends ToViewPattern<A>,
> = P extends `${infer Prefix}/_index` ? P | (Prefix extends "" ? "/" : Prefix) : P;

/////// PUBLIC PATTERN TYPES

export type ToViewPattern<A extends AppConfig> = AppView<A>["pattern"];

type ApiRouteMethod<A extends AppConfig> = AppApiRoute<A>["method"];
type ApiRoutePattern<
	A extends AppConfig,
	M extends ApiRouteMethod<A> = ApiRouteMethod<A>,
> = Extract<AppApiRoute<A>, { method: M }>["pattern"];

type ApiRoutesByKind<A extends AppConfig, T extends ApiRouteKind> =
	AppApiRoute<A> extends infer Route
		? Route extends unknown
			? ResolvedApiRouteKind<Route> extends T
				? Route
				: never
			: never
		: never;

type ApiRouteMethodByKind<A extends AppConfig, T extends ApiRouteKind> =
	ApiRoutesByKind<A, T> extends infer Route
		? Route extends { method: infer M extends string }
			? M
			: never
		: never;

type ApiRoutePatternByKind<
	A extends AppConfig,
	T extends ApiRouteKind,
	M extends ApiRouteMethodByKind<A, T> = ApiRouteMethodByKind<A, T>,
> =
	Extract<ApiRoutesByKind<A, T>, { method: M }> extends infer Route
		? Route extends { pattern: infer P extends string }
			? P
			: never
		: never;

type ApiRouteByKindMethodAndPattern<
	A extends AppConfig,
	T extends ApiRouteKind,
	M extends ApiRouteMethodByKind<A, T>,
	P extends ApiRoutePatternByKind<A, T, M>,
> = Extract<ApiRoutesByKind<A, T>, { method: M; pattern: P }>;

export type ToQueryMethod<A extends AppConfig> = ApiRouteMethodByKind<A, "query">;
export type ToQueryPattern<
	A extends AppConfig,
	M extends ToQueryMethod<A> = ToQueryMethod<A>,
> = ApiRoutePatternByKind<A, "query", M>;

export type ToMutationMethod<A extends AppConfig> = ApiRouteMethodByKind<A, "mutation">;
export type ToMutationPattern<
	A extends AppConfig,
	M extends ToMutationMethod<A> = ToMutationMethod<A>,
> = ApiRoutePatternByKind<A, "mutation", M>;

/////// PUBLIC I/O TYPES

export type ToLoaderOutput<A extends AppConfig, P extends ToViewPattern<A>> =
	ViewByPattern<A, P> extends { __o: infer O } ? O : never;

export type ToLoaderInput<A extends AppConfig, P extends ToViewPattern<A>> =
	ViewByPattern<A, P> extends { __i: infer I } ? I : never;

export type ToQueryInput<
	A extends AppConfig,
	M extends ToQueryMethod<A>,
	P extends ToQueryPattern<A, M>,
> =
	ApiRouteByKindMethodAndPattern<A, "query", M, P> extends {
		__i: infer I;
	}
		? I
		: never;

export type ToQueryOutput<
	A extends AppConfig,
	M extends ToQueryMethod<A>,
	P extends ToQueryPattern<A, M>,
> =
	ApiRouteByKindMethodAndPattern<A, "query", M, P> extends {
		__o: infer O;
	}
		? O
		: never;

export type ToMutationInput<
	A extends AppConfig,
	M extends ToMutationMethod<A>,
	P extends ToMutationPattern<A, M>,
> =
	ApiRouteByKindMethodAndPattern<A, "mutation", M, P> extends {
		__i: infer I;
	}
		? I
		: never;

export type ToMutationOutput<
	A extends AppConfig,
	M extends ToMutationMethod<A>,
	P extends ToMutationPattern<A, M>,
> =
	ApiRouteByKindMethodAndPattern<A, "mutation", M, P> extends {
		__o: infer O;
	}
		? O
		: never;

/////// ROUTE TARGETS

export type ToRouteDestination<A extends AppConfig, P extends ToViewPattern<A>> = {
	href?: never;
	pattern: PermissiveViewPattern<A, P>;
	search?: ViewInputWithParentViews<A, P>;
	hash?: string;
} & ConditionalViewParams<A, P> &
	ConditionalSplat<P>;

export type ToNavigationTarget<A extends AppConfig, P extends ToViewPattern<A>> =
	| {
			href: string;
			pattern?: never;
			params?: never;
			splatValues?: never;
			search?: never;
			hash?: never;
	  }
	| ToRouteDestination<A, P>;

export type ToNavigateArgs<
	A extends AppConfig,
	P extends ToViewPattern<A>,
> = ToNavigationTarget<A, P> & {
	replace?: boolean;
	scrollToTop?: boolean;
	state?: unknown;
	skipWorkIndicator?: boolean;
};

export type ToRouteSyncArgs<
	A extends AppConfig,
	P extends ToViewPattern<A>,
> = ToRouteDestination<A, P> & {
	enabled?: boolean;
	debounceMs?: number;
	replace?: boolean;
	scrollToTop?: boolean;
};

/////// API CLIENT ARGS

type ApiClientArgsForApiRoute<A extends AppConfig, Act> = Act extends {
	method: infer M;
	pattern: infer P;
	__i?: infer Input;
}
	? M extends ApiRouteMethod<A>
		? P extends ApiRoutePattern<A, M>
			? Omit<RequestInit, "body" | "method"> & {
					dedupeKey?: string;
					pattern: P;
					revalidate?: boolean;
					skipWorkIndicator?: boolean;
				} & ApiRouteMethodField<A, P, M> &
					ConditionalApiRouteParams<Act> &
					ConditionalSplat<P> &
					ApiRouteInputField<Input>
			: never
		: never
	: never;

type ApiClientArgs<A extends AppConfig> =
	AppApiRoute<A> extends infer Act
		? Act extends unknown
			? ApiClientArgsForApiRoute<A, Act>
			: never
		: never;

type ApiClientArgsByKind<A extends AppConfig, T extends ApiRouteKind> =
	AppApiRoute<A> extends infer Act
		? Act extends unknown
			? ResolvedApiRouteKind<Act> extends T
				? ApiClientArgsForApiRoute<A, Act>
				: never
			: never
		: never;

export type ApiClientOutput<
	A extends AppConfig,
	Args extends ApiClientArgs<A>,
> = Args extends {
	method: infer M;
	pattern: infer P;
}
	? M extends ApiRouteMethod<A>
		? P extends ApiRoutePattern<A, M>
			? ApiRouteByMethodAndPattern<A, M, P> extends { __o: infer O }
				? O
				: never
			: never
		: never
	: Args extends {
				pattern: infer P;
		  }
		? "GET" extends ApiRouteMethod<A>
			? P extends ApiRoutePattern<A, "GET">
				? ApiRouteByMethodAndPattern<A, "GET", P> extends {
						__o: infer O;
					}
					? O
					: never
				: never
			: never
		: never;

export type ToQueryArgs<A extends AppConfig> = ApiClientArgsByKind<A, "query">;

export type ToQueryError<A extends AppConfig, Args extends ToQueryArgs<A>> = QueryError<
	ApiClientOutput<A, Args>
>;

export type ToMutationArgs<A extends AppConfig> = ApiClientArgsByKind<A, "mutation">;

export type ToMutationError<
	A extends AppConfig,
	Args extends ToMutationArgs<A>,
> = MutationError<ApiClientOutput<A, Args>>;

export type QueryResult<T> = ApiResultBase<T>;
export type MutationResult<T> = ApiResultBase<T>;

/////// ROUTE COMPONENT PROPS

export type ToRouteComponentProps<
	A extends AppConfig,
	P extends ToViewPattern<A>,
	ClientLoaderData = unknown,
> = {
	idx: number;
	Outlet: (local?: Record<string, unknown>) => any;
	__phantom_pattern?: P;
	__phantom_client_loader_data?: ClientLoaderData;
};

/////// CLIENT LOADER PROPS

export type ClientLoaderKnownMatch = {
	pattern: string;
	input: unknown;
};

export type ClientLoaderServerState<A extends AppConfig, P extends ToViewPattern<A>> = {
	clientBuildId: string;
	matches: Array<{
		pattern: string;
		input: unknown;
		loaderData: unknown;
	}>;
	outermostServerError: null | {
		idx: number;
		error: unknown;
	};
	loaderData: ToLoaderOutput<A, P>;
};

export type ToClientLoaderArgs<A extends AppConfig, P extends ToViewPattern<A>> = {
	trigger: "boot" | "navigation" | "revalidation" | "prefetch";
	href: string;
	historyState: unknown;
	pattern: P;
	params: ViewParamsRecord<A, P>;
	splatValues: string[];
	input: ToLoaderInput<A, P>;
	knownMatches: ClientLoaderKnownMatch[];
	serverPromise: Promise<ClientLoaderServerState<A, P>>;
	signal: AbortSignal;
};

/////// DEFINE VIEW INPUT

export type ToDefineViewArgs<
	A extends AppConfig,
	P extends ToViewPattern<A>,
	T = any,
	Element = unknown,
> = {
	pattern: P;
	component: (props: ToRouteComponentProps<A, P, T>) => Element;
	errorBoundary?: (props: { error: unknown }) => Element;
	clientLoader?: (args: ToClientLoaderArgs<A, P>) => Promise<T>;
	beforeRouteCommit?: BeforeRouteCommitFn;
	beforeRouteYield?: BeforeRouteYieldFn;
	runClientLoaderOnHmr?: boolean;
};

/////// LINK PROPS

export type ToLinkProps<A extends AppConfig, P extends ToViewPattern<A>> = LinkPropsBase &
	ToNavigationTarget<A, P> & {
		state?: unknown;
	};

/////// API CLIENT TYPES

export type ToApiDecoratorContext<A extends AppConfig> =
	ApiClientArgs<A> extends infer Args
		? Args extends ApiClientArgs<A>
			? {
					input?: Args extends { input: infer Input }
						? Input
						: Args extends { input?: infer Input }
							? Input
							: never;
					method: Args extends { method: infer M extends string } ? M : "GET";
					pattern: Args["pattern"];
					requestInit: Omit<RequestInit, "body" | "method">;
				}
			: never
		: never;

export type ToApiDecorator<A extends AppConfig> = (
	context: ToApiDecoratorContext<A>,
) =>
	| Omit<RequestInit, "method" | "body">
	| undefined
	| Promise<Omit<RequestInit, "method" | "body"> | undefined>;

export type ToApiClient<A extends AppConfig> = {
	query: <Args extends ToQueryArgs<A>>(
		args: Args,
	) => Promise<QueryResult<ApiClientOutput<A, Args>>>;
	queryOrThrow: <Args extends ToQueryArgs<A>>(
		args: Args,
	) => Promise<ApiClientOutput<A, Args>>;
	mutate: <Args extends ToMutationArgs<A>>(
		args: Args,
	) => Promise<MutationResult<ApiClientOutput<A, Args>>>;
	mutateOrThrow: <Args extends ToMutationArgs<A>>(
		args: Args,
	) => Promise<ApiClientOutput<A, Args>>;
	toIdentityArray: <Args extends ApiClientArgs<A>>(args: Args) => unknown[];
};
