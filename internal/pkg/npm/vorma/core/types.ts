/////// PRIMITIVE TYPES

type LoaderBase = {
	params?: ReadonlyArray<string>;
	parents?: ReadonlyArray<string>;
	pattern: string;
	__I?: unknown;
	__O?: unknown;
};

type ActionBase = {
	method: string;
	params?: ReadonlyArray<string>;
	pattern: string;
	__I?: unknown;
	__O?: unknown;
};

export type SubmitOptions = {
	dedupeKey?: string;
	revalidate?: boolean;
	skipProgressIndicator?: boolean;
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
};

/////// APP CONFIG

export type AppConfig = {
	actionsMountRoot: string;
	__phantom_loaders: readonly LoaderBase[];
	__phantom_actions: readonly ActionBase[];
};

/////// ROUTE-TYPE EXTRACTORS

type __Loader<A extends AppConfig> = A["__phantom_loaders"][number];
type __Action<A extends AppConfig> = A["__phantom_actions"][number];

type __LoaderByPattern<A extends AppConfig, P extends string> = Extract<
	__Loader<A>,
	{ pattern: P }
>;
type __ActionByMethodAndPattern<
	A extends AppConfig,
	M extends string,
	P extends string,
> = Extract<
	__Action<A>,
	{
		method: M;
		pattern: P;
	}
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

type __ConditionalActionParams<Act> = Act extends {
	params: ReadonlyArray<infer Params>;
}
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

/////// LOADER PARENT INPUT

type __LoaderParents<A extends AppConfig, P extends string> =
	__LoaderByPattern<A, P> extends {
		parents: ReadonlyArray<infer Parent>;
	}
		? Extract<Parent, MakeTypedLoaderPattern<A>>
		: never;

type __LoaderInputWithParents<
	A extends AppConfig,
	P extends MakeTypedLoaderPattern<A>,
> = (
	P | __LoaderParents<A, P> extends infer Pattern
		? Pattern extends MakeTypedLoaderPattern<A>
			? (input: MakeTypedLoaderInput<A, Pattern>) => void
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

type __ActionInputField<Input> =
	__IsEmptyInput<Input> extends true ? { input?: Input } : { input: Input };

type __ActionMethodsForPattern<A extends AppConfig, P extends string> = Extract<
	__Action<A>,
	{ pattern: P }
>["method"];

type __ActionMethodField<
	A extends AppConfig,
	P extends string,
	M extends string,
> =
	__ActionMethodsForPattern<A, P> extends "GET"
		? __IsUnion<__ActionMethodsForPattern<A, P>> extends true
			? { method: M }
			: { method?: M }
		: { method: M };

/////// PERMISSIVE PATTERN (for navigate / link _index shorthand)

type __PermissiveLoaderPattern<
	A extends AppConfig,
	P extends MakeTypedLoaderPattern<A>,
> = P extends `${infer Prefix}/_index`
	? P | (Prefix extends "" ? "/" : Prefix)
	: P;

/////// PUBLIC PATTERN TYPES

export type MakeTypedLoaderPattern<A extends AppConfig> =
	__Loader<A>["pattern"];
export type MakeTypedActionMethod<A extends AppConfig> = __Action<A>["method"];
export type MakeTypedActionPattern<
	A extends AppConfig,
	M extends MakeTypedActionMethod<A> = MakeTypedActionMethod<A>,
> = Extract<__Action<A>, { method: M }>["pattern"];

/////// PUBLIC I/O TYPES

export type MakeTypedLoaderOutput<
	A extends AppConfig,
	P extends MakeTypedLoaderPattern<A>,
> = __LoaderByPattern<A, P> extends { __O: infer O } ? O : never;

export type MakeTypedLoaderInput<
	A extends AppConfig,
	P extends MakeTypedLoaderPattern<A>,
> = __LoaderByPattern<A, P> extends { __I: infer I } ? I : never;

export type MakeTypedActionInput<
	A extends AppConfig,
	M extends MakeTypedActionMethod<A>,
	P extends MakeTypedActionPattern<A, M>,
> = __ActionByMethodAndPattern<A, M, P> extends { __I: infer I } ? I : never;

export type MakeTypedActionOutput<
	A extends AppConfig,
	M extends MakeTypedActionMethod<A>,
	P extends MakeTypedActionPattern<A, M>,
> = __ActionByMethodAndPattern<A, M, P> extends { __O: infer O } ? O : never;

/////// ROUTE TARGETS

export type MakeTypedRouteDestination<
	A extends AppConfig,
	P extends MakeTypedLoaderPattern<A>,
> = {
	href?: never;
	pattern: __PermissiveLoaderPattern<A, P>;
	search?: __LoaderInputWithParents<A, P>;
	hash?: string;
} & __ConditionalLoaderParams<A, P> &
	__ConditionalSplat<P>;

export type MakeTypedNavTarget<
	A extends AppConfig,
	P extends MakeTypedLoaderPattern<A>,
> =
	| {
			href: string;
			pattern?: never;
			params?: never;
			splatValues?: never;
			search?: never;
			hash?: never;
	  }
	| MakeTypedRouteDestination<A, P>;

export type MakeTypedNavProps<
	A extends AppConfig,
	P extends MakeTypedLoaderPattern<A>,
> = MakeTypedNavTarget<A, P> & {
	replace?: boolean;
	scrollToTop?: boolean;
	state?: unknown;
	skipProgressIndicator?: boolean;
};

/////// ACTION SUBMIT PROPS

type __ActionSubmitPropsForAction<A extends AppConfig, Act> = Act extends {
	method: infer M;
	pattern: infer P;
	__I?: infer Input;
}
	? M extends MakeTypedActionMethod<A>
		? P extends MakeTypedActionPattern<A, M>
			? Omit<RequestInit, "body" | "method"> & {
					dedupeKey?: string;
					pattern: P;
					revalidate?: boolean;
					skipProgressIndicator?: boolean;
				} & __ActionMethodField<A, P, M> &
					__ConditionalActionParams<Act> &
					__ConditionalSplat<P> &
					__ActionInputField<Input>
			: never
		: never
	: never;

export type MakeTypedActionSubmitProps<A extends AppConfig> =
	__Action<A> extends infer Act
		? Act extends unknown
			? __ActionSubmitPropsForAction<A, Act>
			: never
		: never;

export type MakeTypedActionSubmitOutput<
	A extends AppConfig,
	Props extends MakeTypedActionSubmitProps<A>,
> = Props extends {
	method: infer M;
	pattern: infer P;
}
	? M extends MakeTypedActionMethod<A>
		? P extends MakeTypedActionPattern<A, M>
			? MakeTypedActionOutput<A, M, P>
			: never
		: never
	: Props extends {
				pattern: infer P;
		  }
		? "GET" extends MakeTypedActionMethod<A>
			? P extends MakeTypedActionPattern<A, "GET">
				? MakeTypedActionOutput<A, "GET", P>
				: never
			: never
		: never;

/////// ROUTE COMPONENT PROPS

export type MakeTypedRouteProps<
	A extends AppConfig,
	P extends MakeTypedLoaderPattern<A>,
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
	P extends MakeTypedLoaderPattern<A>,
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
	loaderData: MakeTypedLoaderOutput<A, P>;
};

export type MakeTypedClientLoaderProps<
	A extends AppConfig,
	P extends MakeTypedLoaderPattern<A>,
> = {
	trigger: "init" | "navigation" | "revalidation" | "prefetch";
	href: string;
	historyState: unknown;
	pattern: P;
	params: __LoaderParamsRecord<A, P>;
	splatValues: string[];
	input: MakeTypedLoaderInput<A, P>;
	knownMatches: ClientLoaderKnownMatch[];
	serverPromise: Promise<ClientLoaderServerState<A, P>>;
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
> = LinkPropsBase &
	MakeTypedNavTarget<A, P> & {
		state?: unknown;
	};

/////// API CLIENT TYPES

export type MakeTypedAPIDecoratorContext<A extends AppConfig> =
	MakeTypedActionSubmitProps<A> extends infer Props
		? Props extends MakeTypedActionSubmitProps<A>
			? {
					input?: Props extends { input: infer Input }
						? Input
						: Props extends { input?: infer Input }
							? Input
							: never;
					method: Props extends { method: infer M extends string }
						? M
						: "GET";
					pattern: Props["pattern"];
					requestInit: Omit<RequestInit, "body" | "method">;
				}
			: never
		: never;

export type MakeTypedAPIDecorator<A extends AppConfig> = (
	context: MakeTypedAPIDecoratorContext<A>,
) =>
	| Omit<RequestInit, "method" | "body">
	| undefined
	| Promise<Omit<RequestInit, "method" | "body"> | undefined>;

export type MakeTypedAPIClient<A extends AppConfig> = {
	submit: <Props extends MakeTypedActionSubmitProps<A>>(
		props: Props,
	) => Promise<SubmitResult<MakeTypedActionSubmitOutput<A, Props>>>;
};
