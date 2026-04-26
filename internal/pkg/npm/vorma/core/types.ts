import type { SubmitError } from "./api_client.ts";

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
	kind?: ActionKind;
	__I?: unknown;
	__O?: unknown;
};

export type ActionKind = "query" | "mutation";

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
	| "init"
	| "navigation"
	| "popstate"
	| "revalidation";

export type BeforeRouteTransitionArgs = {
	trigger: Exclude<RouteUpdateReason, "init">;
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

export type SubmitResult<T> =
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

type __ResolvedActionKind<Act> = Act extends {
	kind: infer T extends ActionKind;
}
	? T
	: Act extends {
				method: "GET" | "HEAD";
		  }
		? "query"
		: "mutation";

type __ActionKindField<Act> = Act extends {
	kind: infer T extends ActionKind;
}
	? { kind: T }
	: {};

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
		? Extract<Parent, ToLoaderPattern<A>>
		: never;

type __LoaderInputWithParents<
	A extends AppConfig,
	P extends ToLoaderPattern<A>,
> = (
	P | __LoaderParents<A, P> extends infer Pattern
		? Pattern extends ToLoaderPattern<A>
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
	P extends ToLoaderPattern<A>,
> = P extends `${infer Prefix}/_index`
	? P | (Prefix extends "" ? "/" : Prefix)
	: P;

/////// PUBLIC PATTERN TYPES

export type ToLoaderPattern<A extends AppConfig> = __Loader<A>["pattern"];
export type ToActionMethod<A extends AppConfig> = __Action<A>["method"];
export type ToActionPattern<
	A extends AppConfig,
	M extends ToActionMethod<A> = ToActionMethod<A>,
> = Extract<__Action<A>, { method: M }>["pattern"];

/////// PUBLIC I/O TYPES

export type ToLoaderOutput<A extends AppConfig, P extends ToLoaderPattern<A>> =
	__LoaderByPattern<A, P> extends { __O: infer O } ? O : never;

export type ToLoaderInput<A extends AppConfig, P extends ToLoaderPattern<A>> =
	__LoaderByPattern<A, P> extends { __I: infer I } ? I : never;

export type ToActionInput<
	A extends AppConfig,
	M extends ToActionMethod<A>,
	P extends ToActionPattern<A, M>,
> = __ActionByMethodAndPattern<A, M, P> extends { __I: infer I } ? I : never;

export type ToActionOutput<
	A extends AppConfig,
	M extends ToActionMethod<A>,
	P extends ToActionPattern<A, M>,
> = __ActionByMethodAndPattern<A, M, P> extends { __O: infer O } ? O : never;

export type ToActionKind<
	A extends AppConfig,
	M extends ToActionMethod<A>,
	P extends ToActionPattern<A, M>,
> = __ResolvedActionKind<__ActionByMethodAndPattern<A, M, P>>;

/////// ROUTE TARGETS

export type ToRouteDestination<
	A extends AppConfig,
	P extends ToLoaderPattern<A>,
> = {
	href?: never;
	pattern: __PermissiveLoaderPattern<A, P>;
	search?: __LoaderInputWithParents<A, P>;
	hash?: string;
} & __ConditionalLoaderParams<A, P> &
	__ConditionalSplat<P>;

export type ToNavigationTarget<
	A extends AppConfig,
	P extends ToLoaderPattern<A>,
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
	P extends ToLoaderPattern<A>,
> = ToNavigationTarget<A, P> & {
	replace?: boolean;
	scrollToTop?: boolean;
	state?: unknown;
	skipProgressIndicator?: boolean;
};

export type ToRouteSyncArgs<
	A extends AppConfig,
	P extends ToLoaderPattern<A>,
> = ToRouteDestination<A, P> & {
	enabled?: boolean;
	debounceMs?: number;
	replace?: boolean;
	scrollToTop?: boolean;
};

/////// ACTION SUBMIT PROPS

type __ActionSubmitPropsForAction<A extends AppConfig, Act> = Act extends {
	method: infer M;
	pattern: infer P;
	__I?: infer Input;
}
	? M extends ToActionMethod<A>
		? P extends ToActionPattern<A, M>
			? Omit<RequestInit, "body" | "method"> & {
					dedupeKey?: string;
					pattern: P;
					revalidate?: boolean;
					skipProgressIndicator?: boolean;
				} & __ActionMethodField<A, P, M> &
					__ActionKindField<Act> &
					__ConditionalActionParams<Act> &
					__ConditionalSplat<P> &
					__ActionInputField<Input>
			: never
		: never
	: never;

export type ToActionSubmitArgs<A extends AppConfig> =
	__Action<A> extends infer Act
		? Act extends unknown
			? __ActionSubmitPropsForAction<A, Act>
			: never
		: never;

export type ToActionSubmitArgsByKind<
	A extends AppConfig,
	T extends ActionKind,
> =
	__Action<A> extends infer Act
		? Act extends unknown
			? __ResolvedActionKind<Act> extends T
				? __ActionSubmitPropsForAction<A, Act>
				: never
			: never
		: never;

export type ToActionSubmitOutput<
	A extends AppConfig,
	Args extends ToActionSubmitArgs<A>,
> = Args extends {
	method: infer M;
	pattern: infer P;
}
	? M extends ToActionMethod<A>
		? P extends ToActionPattern<A, M>
			? ToActionOutput<A, M, P>
			: never
		: never
	: Args extends {
				pattern: infer P;
		  }
		? "GET" extends ToActionMethod<A>
			? P extends ToActionPattern<A, "GET">
				? ToActionOutput<A, "GET", P>
				: never
			: never
		: never;

export type ToActionSubmitError<
	A extends AppConfig,
	Args extends ToActionSubmitArgs<A>,
> = SubmitError<ToActionSubmitOutput<A, Args>>;

/////// ROUTE COMPONENT PROPS

export type ToRouteComponentProps<
	A extends AppConfig,
	P extends ToLoaderPattern<A>,
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
	P extends ToLoaderPattern<A>,
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
	P extends ToLoaderPattern<A>,
> = {
	trigger: "init" | "navigation" | "revalidation" | "prefetch";
	href: string;
	historyState: unknown;
	pattern: P;
	params: __LoaderParamsRecord<A, P>;
	splatValues: string[];
	input: ToLoaderInput<A, P>;
	knownMatches: ClientLoaderKnownMatch[];
	serverPromise: Promise<ClientLoaderServerState<A, P>>;
	signal: AbortSignal;
};

/////// DEFINE ROUTE INPUT

export type ToDefineRouteArgs<
	A extends AppConfig,
	P extends ToLoaderPattern<A>,
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
	P extends ToLoaderPattern<A>,
> = LinkPropsBase &
	ToNavigationTarget<A, P> & {
		state?: unknown;
	};

/////// API CLIENT TYPES

export type ToAPIDecoratorContext<A extends AppConfig> =
	ToActionSubmitArgs<A> extends infer Args
		? Args extends ToActionSubmitArgs<A>
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
	submit: <Args extends ToActionSubmitArgs<A>>(
		args: Args,
	) => Promise<SubmitResult<ToActionSubmitOutput<A, Args>>>;
	submitOrThrow: <Args extends ToActionSubmitArgs<A>>(
		args: Args,
	) => Promise<ToActionSubmitOutput<A, Args>>;
	toIdentityArray: <Args extends ToActionSubmitArgs<A>>(
		args: Args,
	) => unknown[];
};
