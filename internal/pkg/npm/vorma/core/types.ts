import type { MutationError, QueryError } from "./api_client.ts";

/////// PRIMITIVE TYPES

type ViewBase = {
	params?: ReadonlyArray<string>;
	parents?: ReadonlyArray<string>;
	pattern: string;
	__I?: unknown;
	__O?: unknown;
};

type APIRouteBase = {
	method: string;
	params?: ReadonlyArray<string>;
	pattern: string;
	kind?: APIRouteKind;
	__I?: unknown;
	__O?: unknown;
};

export type APIRouteKind = "query" | "mutation";

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
	clientBuildID: string;
	params: Record<string, string>;
	splatValues: string[];
	matches: RouteMatchState[];
	error: RouteErrorState | null;
};

export type RouteUpdateReason =
	| "boot"
	| "navigation"
	| "popstate"
	| "revalidation";

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

type APIResultBase<T> =
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
	__vormaViews: readonly ViewBase[];
	__vormaAPIRoutes: readonly APIRouteBase[];
};

/////// APP TYPE EXTRACTORS

type __View<A extends AppConfig> = A["__vormaViews"][number];
type __APIRoute<A extends AppConfig> = A["__vormaAPIRoutes"][number];

type __ViewByPattern<A extends AppConfig, P extends string> = Extract<
	__View<A>,
	{ pattern: P }
>;
type __APIRouteByMethodAndPattern<
	A extends AppConfig,
	M extends string,
	P extends string,
> = Extract<
	__APIRoute<A>,
	{
		method: M;
		pattern: P;
	}
>;

type __ResolvedAPIRouteKind<Act> = Act extends {
	kind: infer T extends APIRouteKind;
}
	? T
	: Act extends {
				method: "GET" | "HEAD";
		  }
		? "query"
		: "mutation";

/////// SPLAT DETECTION (pattern-based, no distribution issues)

type __IsSplat<P extends string> = P extends `${string}/*` ? true : false;

type __ConditionalSplat<P extends string> =
	__IsSplat<P> extends true ? { splatValues: Array<string> } : {};

/////// CONDITIONAL PARAMS (non-distributive — uses the resolved Extract
/////// result directly in the extends clause so it does NOT distribute
/////// when P is a union of patterns)

type __ConditionalViewParams<A extends AppConfig, P extends string> =
	__ViewByPattern<A, P> extends { params: ReadonlyArray<infer Params> }
		? Params extends string
			? { params: { [K in Params]: string } }
			: {}
		: {};

type __ConditionalAPIRouteParams<Act> = Act extends {
	params: ReadonlyArray<infer Params>;
}
	? Params extends string
		? { params: { [K in Params]: string } }
		: {}
	: {};

/////// PARAMS RECORD (for client loader props and router data)

type __ViewParamsRecord<A extends AppConfig, P extends string> =
	__ViewByPattern<A, P> extends { params: ReadonlyArray<infer Params> }
		? Params extends string
			? { [K in Params]: string }
			: Record<string, string>
		: Record<string, string>;

/////// LOADER INPUT FROM PARENT VIEWS

type __ViewParents<A extends AppConfig, P extends string> =
	__ViewByPattern<A, P> extends {
		parents: ReadonlyArray<infer Parent>;
	}
		? Extract<Parent, ToViewPattern<A>>
		: never;

type __ViewInputWithParentViews<
	A extends AppConfig,
	P extends ToViewPattern<A>,
> = (
	P | __ViewParents<A, P> extends infer Pattern
		? Pattern extends ToViewPattern<A>
			? (input: ToLoaderInput<A, Pattern>) => void
			: never
		: never
) extends (input: infer Input) => void
	? Input
	: never;

/////// INPUT EMPTINESS

type __IsEmptyInput<T> = [T] extends [null | undefined] ? true : false;

type __IsUnion<T, U = T> = [T] extends [never]
	? false
	: T extends unknown
		? [U] extends [T]
			? false
			: true
		: false;

type __APIRouteInputField<Input> =
	__IsEmptyInput<Input> extends true ? { input?: Input } : { input: Input };

type __APIRouteMethodsForPattern<
	A extends AppConfig,
	P extends string,
> = Extract<__APIRoute<A>, { pattern: P }>["method"];

type __APIRouteMethodField<
	A extends AppConfig,
	P extends string,
	M extends string,
> =
	__APIRouteMethodsForPattern<A, P> extends "GET"
		? __IsUnion<__APIRouteMethodsForPattern<A, P>> extends true
			? { method: M }
			: { method?: M }
		: { method: M };

/////// PERMISSIVE PATTERN (for navigate / link _index shorthand)

type __PermissiveViewPattern<
	A extends AppConfig,
	P extends ToViewPattern<A>,
> = P extends `${infer Prefix}/_index`
	? P | (Prefix extends "" ? "/" : Prefix)
	: P;

/////// PUBLIC PATTERN TYPES

export type ToViewPattern<A extends AppConfig> = __View<A>["pattern"];

type __APIRouteMethod<A extends AppConfig> = __APIRoute<A>["method"];
type __APIRoutePattern<
	A extends AppConfig,
	M extends __APIRouteMethod<A> = __APIRouteMethod<A>,
> = Extract<__APIRoute<A>, { method: M }>["pattern"];

type __APIRoutesByKind<A extends AppConfig, T extends APIRouteKind> =
	__APIRoute<A> extends infer Route
		? Route extends unknown
			? __ResolvedAPIRouteKind<Route> extends T
				? Route
				: never
			: never
		: never;

type __APIRouteMethodByKind<A extends AppConfig, T extends APIRouteKind> =
	__APIRoutesByKind<A, T> extends infer Route
		? Route extends { method: infer M extends string }
			? M
			: never
		: never;

type __APIRoutePatternByKind<
	A extends AppConfig,
	T extends APIRouteKind,
	M extends __APIRouteMethodByKind<A, T> = __APIRouteMethodByKind<A, T>,
> =
	Extract<__APIRoutesByKind<A, T>, { method: M }> extends infer Route
		? Route extends { pattern: infer P extends string }
			? P
			: never
		: never;

type __APIRouteByKindMethodAndPattern<
	A extends AppConfig,
	T extends APIRouteKind,
	M extends __APIRouteMethodByKind<A, T>,
	P extends __APIRoutePatternByKind<A, T, M>,
> = Extract<__APIRoutesByKind<A, T>, { method: M; pattern: P }>;

export type ToQueryMethod<A extends AppConfig> = __APIRouteMethodByKind<
	A,
	"query"
>;
export type ToQueryPattern<
	A extends AppConfig,
	M extends ToQueryMethod<A> = ToQueryMethod<A>,
> = __APIRoutePatternByKind<A, "query", M>;

export type ToMutationMethod<A extends AppConfig> = __APIRouteMethodByKind<
	A,
	"mutation"
>;
export type ToMutationPattern<
	A extends AppConfig,
	M extends ToMutationMethod<A> = ToMutationMethod<A>,
> = __APIRoutePatternByKind<A, "mutation", M>;

/////// PUBLIC I/O TYPES

export type ToLoaderOutput<A extends AppConfig, P extends ToViewPattern<A>> =
	__ViewByPattern<A, P> extends { __O: infer O } ? O : never;

export type ToLoaderInput<A extends AppConfig, P extends ToViewPattern<A>> =
	__ViewByPattern<A, P> extends { __I: infer I } ? I : never;

export type ToQueryInput<
	A extends AppConfig,
	M extends ToQueryMethod<A>,
	P extends ToQueryPattern<A, M>,
> =
	__APIRouteByKindMethodAndPattern<A, "query", M, P> extends {
		__I: infer I;
	}
		? I
		: never;

export type ToQueryOutput<
	A extends AppConfig,
	M extends ToQueryMethod<A>,
	P extends ToQueryPattern<A, M>,
> =
	__APIRouteByKindMethodAndPattern<A, "query", M, P> extends {
		__O: infer O;
	}
		? O
		: never;

export type ToMutationInput<
	A extends AppConfig,
	M extends ToMutationMethod<A>,
	P extends ToMutationPattern<A, M>,
> =
	__APIRouteByKindMethodAndPattern<A, "mutation", M, P> extends {
		__I: infer I;
	}
		? I
		: never;

export type ToMutationOutput<
	A extends AppConfig,
	M extends ToMutationMethod<A>,
	P extends ToMutationPattern<A, M>,
> =
	__APIRouteByKindMethodAndPattern<A, "mutation", M, P> extends {
		__O: infer O;
	}
		? O
		: never;

/////// ROUTE TARGETS

export type ToRouteDestination<
	A extends AppConfig,
	P extends ToViewPattern<A>,
> = {
	href?: never;
	pattern: __PermissiveViewPattern<A, P>;
	search?: __ViewInputWithParentViews<A, P>;
	hash?: string;
} & __ConditionalViewParams<A, P> &
	__ConditionalSplat<P>;

export type ToNavigationTarget<
	A extends AppConfig,
	P extends ToViewPattern<A>,
> =
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

type __APIClientArgsForAPIRoute<A extends AppConfig, Act> = Act extends {
	method: infer M;
	pattern: infer P;
	__I?: infer Input;
}
	? M extends __APIRouteMethod<A>
		? P extends __APIRoutePattern<A, M>
			? Omit<RequestInit, "body" | "method"> & {
					dedupeKey?: string;
					pattern: P;
					revalidate?: boolean;
					skipWorkIndicator?: boolean;
				} & __APIRouteMethodField<A, P, M> &
					__ConditionalAPIRouteParams<Act> &
					__ConditionalSplat<P> &
					__APIRouteInputField<Input>
			: never
		: never
	: never;

type __APIClientArgs<A extends AppConfig> =
	__APIRoute<A> extends infer Act
		? Act extends unknown
			? __APIClientArgsForAPIRoute<A, Act>
			: never
		: never;

type __APIClientArgsByKind<A extends AppConfig, T extends APIRouteKind> =
	__APIRoute<A> extends infer Act
		? Act extends unknown
			? __ResolvedAPIRouteKind<Act> extends T
				? __APIClientArgsForAPIRoute<A, Act>
				: never
			: never
		: never;

export type __APIClientOutput<
	A extends AppConfig,
	Args extends __APIClientArgs<A>,
> = Args extends {
	method: infer M;
	pattern: infer P;
}
	? M extends __APIRouteMethod<A>
		? P extends __APIRoutePattern<A, M>
			? __APIRouteByMethodAndPattern<A, M, P> extends { __O: infer O }
				? O
				: never
			: never
		: never
	: Args extends {
				pattern: infer P;
		  }
		? "GET" extends __APIRouteMethod<A>
			? P extends __APIRoutePattern<A, "GET">
				? __APIRouteByMethodAndPattern<A, "GET", P> extends {
						__O: infer O;
					}
					? O
					: never
				: never
			: never
		: never;

export type ToQueryArgs<A extends AppConfig> = __APIClientArgsByKind<
	A,
	"query"
>;

export type ToQueryError<
	A extends AppConfig,
	Args extends ToQueryArgs<A>,
> = QueryError<__APIClientOutput<A, Args>>;

export type ToMutationArgs<A extends AppConfig> = __APIClientArgsByKind<
	A,
	"mutation"
>;

export type ToMutationError<
	A extends AppConfig,
	Args extends ToMutationArgs<A>,
> = MutationError<__APIClientOutput<A, Args>>;

export type QueryResult<T> = APIResultBase<T>;
export type MutationResult<T> = APIResultBase<T>;

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

export type ClientLoaderServerState<
	A extends AppConfig,
	P extends ToViewPattern<A>,
> = {
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
	loaderData: ToLoaderOutput<A, P>;
};

export type ToClientLoaderArgs<
	A extends AppConfig,
	P extends ToViewPattern<A>,
> = {
	trigger: "boot" | "navigation" | "revalidation" | "prefetch";
	href: string;
	historyState: unknown;
	pattern: P;
	params: __ViewParamsRecord<A, P>;
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
	runClientLoaderOnHMR?: boolean;
};

/////// LINK PROPS

export type ToLinkProps<
	A extends AppConfig,
	P extends ToViewPattern<A>,
> = LinkPropsBase &
	ToNavigationTarget<A, P> & {
		state?: unknown;
	};

/////// API CLIENT TYPES

export type ToAPIDecoratorContext<A extends AppConfig> =
	__APIClientArgs<A> extends infer Args
		? Args extends __APIClientArgs<A>
			? {
					input?: Args extends { input: infer Input }
						? Input
						: Args extends { input?: infer Input }
							? Input
							: never;
					method: Args extends { method: infer M extends string }
						? M
						: "GET";
					pattern: Args["pattern"];
					requestInit: Omit<RequestInit, "body" | "method">;
				}
			: never
		: never;

export type ToAPIDecorator<A extends AppConfig> = (
	context: ToAPIDecoratorContext<A>,
) =>
	| Omit<RequestInit, "method" | "body">
	| undefined
	| Promise<Omit<RequestInit, "method" | "body"> | undefined>;

export type ToAPIClient<A extends AppConfig> = {
	query: <Args extends ToQueryArgs<A>>(
		args: Args,
	) => Promise<QueryResult<__APIClientOutput<A, Args>>>;
	queryOrThrow: <Args extends ToQueryArgs<A>>(
		args: Args,
	) => Promise<__APIClientOutput<A, Args>>;
	mutate: <Args extends ToMutationArgs<A>>(
		args: Args,
	) => Promise<MutationResult<__APIClientOutput<A, Args>>>;
	mutateOrThrow: <Args extends ToMutationArgs<A>>(
		args: Args,
	) => Promise<__APIClientOutput<A, Args>>;
	toIdentityArray: <Args extends __APIClientArgs<A>>(args: Args) => unknown[];
};
