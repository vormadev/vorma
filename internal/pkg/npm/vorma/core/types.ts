/////// PRIMITIVE TYPES

type LoaderBase = {
	params?: ReadonlyArray<string>;
	pattern: string;
	__O?: unknown;
};

type ActionBase = {
	method?: string;
	params?: ReadonlyArray<string>;
	pattern: string;
	__I?: unknown;
	__O?: unknown;
};

export type SubmitOptions = {
	dedupeKey?: string;
	revalidate?: boolean;
	skipGlobalLoadingIndicator?: boolean;
};

export type RevalidationResult =
	| { ok: true }
	| { ok: false; reason: "max_retries_exhausted" };

export type SubmitResult<T> =
	| {
			success: true;
			data: T;
			revalidationPromise: Promise<RevalidationResult>;
	  }
	| {
			success: false;
			error: string;
			revalidationPromise: Promise<RevalidationResult>;
	  };

export type LinkPropsBase = {
	href?: string;
	prefetch?: "intent" | "none";
	prefetchDelayMs?: number;
	visitOnPointerDown?: boolean;
	replace?: boolean;
	scrollToTop?: boolean;
};

/////// APP CONFIG

export type AppConfig = {
	actionsMountRoot: string;
	__phantom_loaders: readonly LoaderBase[];
	__phantom_actions: readonly ActionBase[];
};

/////// ROUTE-TYPE EXTRACTORS (scoped per route category)

type __Loader<A extends AppConfig> = A["__phantom_loaders"][number];
type __Query<A extends AppConfig> = Extract<
	A["__phantom_actions"][number],
	{ method: "GET" }
>;
type __Mutation<A extends AppConfig> = Exclude<
	A["__phantom_actions"][number],
	{ method: "GET" }
>;

type __LoaderByPattern<A extends AppConfig, P extends string> = Extract<
	__Loader<A>,
	{ pattern: P }
>;
type __QueryByPattern<A extends AppConfig, P extends string> = Extract<
	__Query<A>,
	{ pattern: P }
>;
type __MutationByPattern<A extends AppConfig, P extends string> = Extract<
	__Mutation<A>,
	{ pattern: P }
>;

/////// SPLAT DETECTION (pattern-based, no distribution issues)

type __IsSplat<P extends string> = P extends `${string}/*` ? true : false;

type __ConditionalSplat<P extends string> =
	__IsSplat<P> extends true ? { splatValues: Array<string> } : {};

/////// CONDITIONAL PARAMS (non-distributive — uses the resolved Extract
/////// result directly in the extends clause so it does NOT distribute
/////// when P is a union of patterns)

type __ConditionalLoaderParams<A extends AppConfig, P extends string> =
	__LoaderByPattern<A, P> extends { params: ReadonlyArray<infer Params> }
		? Params extends string
			? { params: { [K in Params]: string } }
			: {}
		: {};

type __ConditionalQueryParams<A extends AppConfig, P extends string> =
	__QueryByPattern<A, P> extends { params: ReadonlyArray<infer Params> }
		? Params extends string
			? { params: { [K in Params]: string } }
			: {}
		: {};

type __ConditionalMutationParams<A extends AppConfig, P extends string> =
	__MutationByPattern<A, P> extends { params: ReadonlyArray<infer Params> }
		? Params extends string
			? { params: { [K in Params]: string } }
			: {}
		: {};

/////// PARAMS RECORD (for client loader props and router data)

type __LoaderParamsRecord<A extends AppConfig, P extends string> =
	__LoaderByPattern<A, P> extends { params: ReadonlyArray<infer Params> }
		? Params extends string
			? { [K in Params]: string }
			: Record<string, string>
		: Record<string, string>;

/////// INPUT EMPTINESS

type __IsEmptyInput<T> = [T] extends [null | undefined] ? true : false;

/////// ROOT DATA

type __ExtractRootData<A extends AppConfig> =
	__LoaderByPattern<A, "/"> extends { __O: infer T } ? T : never;

/////// PERMISSIVE PATTERN (for navigate / link _index shorthand)

type __PermissiveLoaderPattern<
	A extends AppConfig,
	P extends MakeTypedLoaderPattern<A>,
> = P extends `${infer Prefix}/_index`
	? P | (Prefix extends "" ? "/" : Prefix)
	: P;

/////// MUTATION METHOD

type __MutationMethod<
	A extends AppConfig,
	P extends MakeTypedMutationPattern<A>,
> =
	__MutationByPattern<A, P> extends { method: infer M }
		? M extends string
			? M
			: string
		: string;

/////// PUBLIC PATTERN TYPES

export type MakeTypedLoaderPattern<A extends AppConfig> =
	__Loader<A>["pattern"];
export type MakeTypedQueryPattern<A extends AppConfig> = __Query<A>["pattern"];
export type MakeTypedMutationPattern<A extends AppConfig> =
	__Mutation<A>["pattern"];

/////// PUBLIC I/O TYPES (scoped to their own route category)

export type MakeTypedLoaderOutput<
	A extends AppConfig,
	P extends MakeTypedLoaderPattern<A>,
> = __LoaderByPattern<A, P> extends { __O: infer O } ? O : never;

export type MakeTypedQueryInput<
	A extends AppConfig,
	P extends MakeTypedQueryPattern<A>,
> = __QueryByPattern<A, P> extends { __I: infer I } ? I : never;

export type MakeTypedQueryOutput<
	A extends AppConfig,
	P extends MakeTypedQueryPattern<A>,
> = __QueryByPattern<A, P> extends { __O: infer O } ? O : never;

export type MakeTypedMutationInput<
	A extends AppConfig,
	P extends MakeTypedMutationPattern<A>,
> = __MutationByPattern<A, P> extends { __I: infer I } ? I : never;

export type MakeTypedMutationOutput<
	A extends AppConfig,
	P extends MakeTypedMutationPattern<A>,
> = __MutationByPattern<A, P> extends { __O: infer O } ? O : never;

/////// ROUTER DATA

export type MakeTypedRouterData<
	A extends AppConfig,
	P extends string = string,
> = {
	clientBuildID: string;
	matchedPatterns: string[];
	splatValues: string[];
	params: __LoaderParamsRecord<A, P>;
	historyState: unknown;
	rootData: __ExtractRootData<A>;
};

/////// NAVIGATE PROPS

export type MakeTypedNavigateProps<
	A extends AppConfig,
	P extends MakeTypedLoaderPattern<A>,
> = {
	pattern: __PermissiveLoaderPattern<A, P>;
	state?: unknown;
} & __ConditionalLoaderParams<A, P> &
	__ConditionalSplat<P>;

/////// QUERY PROPS

export type MakeTypedQueryProps<
	A extends AppConfig,
	P extends MakeTypedQueryPattern<A>,
> = {
	pattern: P;
	options?: SubmitOptions;
	requestInit?: Omit<RequestInit, "method"> & { method?: "GET" };
} & __ConditionalQueryParams<A, P> &
	__ConditionalSplat<P> &
	(__IsEmptyInput<MakeTypedQueryInput<A, P>> extends true
		? { input?: MakeTypedQueryInput<A, P> }
		: { input: MakeTypedQueryInput<A, P> });

/////// MUTATION PROPS

export type MakeTypedMutationProps<
	A extends AppConfig,
	P extends MakeTypedMutationPattern<A>,
> = {
	pattern: P;
	options?: SubmitOptions;
	requestInit: RequestInit & { method: __MutationMethod<A, P> };
} & __ConditionalMutationParams<A, P> &
	__ConditionalSplat<P> &
	(__IsEmptyInput<MakeTypedMutationInput<A, P>> extends true
		? { input?: MakeTypedMutationInput<A, P> }
		: { input: MakeTypedMutationInput<A, P> });

/////// ROUTE COMPONENT PROPS

export type MakeTypedRouteProps<
	A extends AppConfig,
	P extends MakeTypedLoaderPattern<A>,
	ClientData = unknown,
> = {
	idx: number;
	Outlet: (local?: Record<string, unknown>) => any;
	__phantom_pattern?: P;
	__phantom_client_data?: ClientData;
};

/////// CLIENT LOADER PROPS

export type MakeTypedClientLoaderProps<
	A extends AppConfig,
	P extends MakeTypedLoaderPattern<A>,
> = {
	params: __LoaderParamsRecord<A, P>;
	splatValues: string[];
	serverDataPromise: Promise<{
		matchedPatterns: string[];
		rootData: __ExtractRootData<A>;
		loaderData: MakeTypedLoaderOutput<A, P>;
		clientBuildID: string;
	}>;
	signal: AbortSignal;
};

/////// DEFINE ROUTE INPUT

export type MakeTypedDefineRouteInput<
	A extends AppConfig,
	P extends MakeTypedLoaderPattern<A>,
	T = any,
	Element = unknown,
> = {
	pattern: P;
	component: (props: MakeTypedRouteProps<A, P, T>) => Element;
	errorBoundary?: (props: { error: unknown }) => Element;
	clientLoader?: (props: MakeTypedClientLoaderProps<A, P>) => Promise<T>;
	runClientLoaderOnHMR?: boolean;
};

/////// LINK PROPS

export type MakeTypedLinkProps<
	A extends AppConfig,
	P extends MakeTypedLoaderPattern<A>,
> = LinkPropsBase & {
	pattern: __PermissiveLoaderPattern<A, P>;
	search?: string;
	hash?: string;
	state?: unknown;
} & __ConditionalLoaderParams<A, P> &
	__ConditionalSplat<P>;

/////// API CLIENT TYPES

export type MakeTypedAPIDecoratorContext<A extends AppConfig> =
	| {
			type: "query";
			pattern: MakeTypedQueryPattern<A>;
			requestInit?: RequestInit;
			input?: unknown;
	  }
	| {
			type: "mutation";
			pattern: MakeTypedMutationPattern<A>;
			requestInit?: RequestInit;
			input?: unknown;
	  };

export type MakeTypedAPIDecorator<A extends AppConfig> = (
	context: MakeTypedAPIDecoratorContext<A>,
) =>
	| Omit<RequestInit, "method" | "body">
	| undefined
	| Promise<Omit<RequestInit, "method" | "body"> | undefined>;

export type MakeTypedAPIClient<A extends AppConfig> = {
	query: <P extends MakeTypedQueryPattern<A>>(
		props: MakeTypedQueryProps<A, P>,
	) => Promise<SubmitResult<MakeTypedQueryOutput<A, P>>>;
	mutate: <P extends MakeTypedMutationPattern<A>>(
		props: MakeTypedMutationProps<A, P>,
	) => Promise<SubmitResult<MakeTypedMutationOutput<A, P>>>;
};
