import type { MutationError, QueryError } from "./api_client.ts";
import type {
	AppConfig,
	AppResource,
	ConditionalResourceParams,
	ConditionalSplat,
	ResolvedResourceKind,
	ResourceByKindMethodAndPattern,
	ResourceByMethodAndPattern,
	ResourceInputField,
	ResourceKind,
	ResourceMethod,
	ResourceMethodByKind,
	ResourceMethodField,
	ResourcePattern,
	ResourcePatternByKind,
} from "./generated_contract_types.ts";
import type { RevalidationResult } from "./route_types.ts";

/////// API RESULT

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

export type QueryResult<T> = ApiResultBase<T>;
export type MutationResult<T> = ApiResultBase<T>;

/////// RESOURCE PATTERNS

export type ToQueryMethod<A extends AppConfig> = ResourceMethodByKind<A, "query">;

export type ToQueryPattern<
	A extends AppConfig,
	M extends ToQueryMethod<A> = ToQueryMethod<A>,
> = ResourcePatternByKind<A, "query", M>;

export type ToMutationMethod<A extends AppConfig> = ResourceMethodByKind<A, "mutation">;

export type ToMutationPattern<
	A extends AppConfig,
	M extends ToMutationMethod<A> = ToMutationMethod<A>,
> = ResourcePatternByKind<A, "mutation", M>;

/////// RESOURCE I/O

export type ToQueryInput<
	A extends AppConfig,
	M extends ToQueryMethod<A>,
	P extends ToQueryPattern<A, M>,
> =
	ResourceByKindMethodAndPattern<A, "query", M, P> extends {
		__i: infer I;
	}
		? I
		: never;

export type ToQueryOutput<
	A extends AppConfig,
	M extends ToQueryMethod<A>,
	P extends ToQueryPattern<A, M>,
> =
	ResourceByKindMethodAndPattern<A, "query", M, P> extends {
		__o: infer O;
	}
		? O
		: never;

export type ToMutationInput<
	A extends AppConfig,
	M extends ToMutationMethod<A>,
	P extends ToMutationPattern<A, M>,
> =
	ResourceByKindMethodAndPattern<A, "mutation", M, P> extends {
		__i: infer I;
	}
		? I
		: never;

export type ToMutationOutput<
	A extends AppConfig,
	M extends ToMutationMethod<A>,
	P extends ToMutationPattern<A, M>,
> =
	ResourceByKindMethodAndPattern<A, "mutation", M, P> extends {
		__o: infer O;
	}
		? O
		: never;

/////// API CLIENT ARGS

type ApiClientArgsForResource<A extends AppConfig, Resource> = Resource extends {
	method: infer M;
	pattern: infer P;
	__i?: infer Input;
}
	? M extends ResourceMethod<A>
		? P extends ResourcePattern<A, M>
			? Omit<RequestInit, "body" | "method"> & {
					dedupeKey?: string;
					pattern: P;
					revalidate?: boolean;
					skipWorkIndicator?: boolean;
				} & (ResolvedResourceKind<Resource> extends "mutation"
						? /*
						GET is implicit only for QUERIES. Mutations always name
						their method — side-effectful calls stay self-documenting
						even for a kind-overridden GET mutation.
						*/
							{ method: M }
						: ResourceMethodField<A, P, M>) &
					ConditionalResourceParams<Resource> &
					ConditionalSplat<P> &
					ResourceInputField<Input>
			: never
		: never
	: never;

type ApiClientArgs<A extends AppConfig> =
	AppResource<A> extends infer Resource
		? Resource extends unknown
			? ApiClientArgsForResource<A, Resource>
			: never
		: never;

type ApiClientArgsByKind<A extends AppConfig, T extends ResourceKind> =
	AppResource<A> extends infer Resource
		? Resource extends unknown
			? ResolvedResourceKind<Resource> extends T
				? ApiClientArgsForResource<A, Resource>
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
	? M extends ResourceMethod<A>
		? P extends ResourcePattern<A, M>
			? ResourceByMethodAndPattern<A, M, P> extends { __o: infer O }
				? O
				: never
			: never
		: never
	: Args extends {
				pattern: infer P;
		  }
		? "GET" extends ResourceMethod<A>
			? P extends ResourcePattern<A, "GET">
				? ResourceByMethodAndPattern<A, "GET", P> extends {
						__o: infer O;
					}
					? O
					: never
				: never
			: never
		: never;

/*
One-param = the full args union; add method + pattern to narrow to one
route's call args (same per-route keying as ToQueryInput/ToQueryOutput).
The optional-method arm exists because GET is implicit for queries.
*/
export type ToQueryArgs<
	A extends AppConfig,
	M extends ToQueryMethod<A> = ToQueryMethod<A>,
	P extends ToQueryPattern<A, M> = ToQueryPattern<A, M>,
> = Extract<
	ApiClientArgsByKind<A, "query">,
	{ pattern: P; method: M } | { pattern: P; method?: M }
>;

export type ToQueryError<A extends AppConfig, Args extends ToQueryArgs<A>> = QueryError<
	ApiClientOutput<A, Args>
>;

/*
Mutations always name their method, so the narrowing filter is exact.
*/
export type ToMutationArgs<
	A extends AppConfig,
	M extends ToMutationMethod<A> = ToMutationMethod<A>,
	P extends ToMutationPattern<A, M> = ToMutationPattern<A, M>,
> = Extract<ApiClientArgsByKind<A, "mutation">, { method: M; pattern: P }>;

export type ToMutationError<
	A extends AppConfig,
	Args extends ToMutationArgs<A>,
> = MutationError<ApiClientOutput<A, Args>>;

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
