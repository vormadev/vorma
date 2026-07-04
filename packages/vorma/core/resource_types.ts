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

/**
 * Result of {@link ToApiClient.query} — never throws; check `success` to
 * discriminate. `revalidationPromise` resolves once the route-data
 * revalidation this call may have triggered (see {@link ToApiClient} for the
 * auto-revalidation default) settles.
 */
export type QueryResult<T> = ApiResultBase<T>;

/**
 * Result of {@link ToApiClient.mutate} — never throws; check `success` to
 * discriminate. `revalidationPromise` resolves once the route-data
 * revalidation this call triggers by default (see {@link ToApiClient})
 * settles.
 */
export type MutationResult<T> = ApiResultBase<T>;

/////// RESOURCE PATTERNS

/** A resolved-`query`-kind resource's HTTP method, narrowed to `A`'s routes. */
export type ToQueryMethod<A extends AppConfig> = ResourceMethodByKind<A, "query">;

/** A resolved-`query`-kind resource's pattern for a given method (or every query pattern if `M` is left to its default). */
export type ToQueryPattern<
	A extends AppConfig,
	M extends ToQueryMethod<A> = ToQueryMethod<A>,
> = ResourcePatternByKind<A, "query", M>;

/** A resolved-`mutation`-kind resource's HTTP method, narrowed to `A`'s routes. */
export type ToMutationMethod<A extends AppConfig> = ResourceMethodByKind<A, "mutation">;

/** A resolved-`mutation`-kind resource's pattern for a given method (or every mutation pattern if `M` is left to its default). */
export type ToMutationPattern<
	A extends AppConfig,
	M extends ToMutationMethod<A> = ToMutationMethod<A>,
> = ResourcePatternByKind<A, "mutation", M>;

/////// RESOURCE I/O

/**
 * One query resource's typed input — the exact route named by `M`+`P`,
 * unlike {@link ToQueryArgs} which produces the whole call-args union.
 * Useful for a helper function's own signature that needs one route's
 * input type without threading full args through it.
 */
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

/** One query resource's typed output — see {@link ToQueryInput} for the input twin. */
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

/** One mutation resource's typed input — see {@link ToQueryInput} for the query twin. */
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

/** One mutation resource's typed output — see {@link ToQueryInput} for the query twin. */
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

/**
 * The response payload type a given `apiClient` call's args produce — the
 * type behind `queryOrThrow(args)`/`mutateOrThrow(args)`'s return value and
 * `query(args)`/`mutate(args)`'s `data` field on success.
 */
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

/**
 * Call args for {@link ToApiClient.query}/{@link ToApiClient.queryOrThrow} —
 * the full args union across every query resource in `A` (narrow to one
 * route by supplying `M`/`P`, the same per-route keying as
 * {@link ToQueryInput}/{@link ToQueryOutput}). `method` is optional here
 * because GET is implicit for queries; every other field
 * (`RequestInit` members, `params`, `splatValues`, `input`) follows the
 * matched route's own shape. `args` — not `[method, pattern, ...]` — is
 * also the cache identity {@link ToApiClient.toIdentityArray} derives from,
 * which is what makes it the natural `queryKey` input for a react-query-style
 * wrapper hook.
 */
export type ToQueryArgs<
	A extends AppConfig,
	M extends ToQueryMethod<A> = ToQueryMethod<A>,
	P extends ToQueryPattern<A, M> = ToQueryPattern<A, M>,
> = Extract<
	ApiClientArgsByKind<A, "query">,
	{ pattern: P; method: M } | { pattern: P; method?: M }
>;

/** {@link QueryError} narrowed to one call's typed output — the type a `catch` on `queryOrThrow(args)` actually throws. */
export type ToQueryError<A extends AppConfig, Args extends ToQueryArgs<A>> = QueryError<
	ApiClientOutput<A, Args>
>;

/**
 * Call args for {@link ToApiClient.mutate}/{@link ToApiClient.mutateOrThrow}
 * — the mutation twin of {@link ToQueryArgs}. `method` is always required
 * (mutations always name their method, even for a kind-overridden GET
 * mutation, so the call stays self-documenting) and the narrowing filter is
 * therefore exact rather than optional-method like the query side.
 */
export type ToMutationArgs<
	A extends AppConfig,
	M extends ToMutationMethod<A> = ToMutationMethod<A>,
	P extends ToMutationPattern<A, M> = ToMutationPattern<A, M>,
> = Extract<ApiClientArgsByKind<A, "mutation">, { method: M; pattern: P }>;

/** {@link MutationError} narrowed to one call's typed output — the type a `catch` on `mutateOrThrow(args)` actually throws. */
export type ToMutationError<
	A extends AppConfig,
	Args extends ToMutationArgs<A>,
> = MutationError<ApiClientOutput<A, Args>>;

/////// API CLIENT TYPES

/** The typed context {@link ToApiDecorator} receives for a given `apiClient` call. */
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

/**
 * `createVormaClient(config, { apiDecorator })` — runs before every
 * `apiClient` request and returns `RequestInit` fields (typically `headers`)
 * to merge onto the outgoing request. Typical use: attaching an
 * app-minted CSRF-style token header on mutations, read from a cookie the
 * server set. Returning `undefined` (or a promise resolving to it) applies
 * nothing.
 */
export type ToApiDecorator<A extends AppConfig> = (
	context: ToApiDecoratorContext<A>,
) =>
	| Omit<RequestInit, "method" | "body">
	| undefined
	| Promise<Omit<RequestInit, "method" | "body"> | undefined>;

/**
 * The typed `apiClient` object every adapter's `createVormaClient(...)`
 * returns, for calling Vorma resources (query/mutation routes) outside the
 * normal view-data/client-loader flow — e.g. from an event handler, a
 * background effect, or a `@tanstack/react-query` wrapper hook.
 *
 * **Auto-revalidation.** Every `mutate`/`mutateOrThrow` call
 * revalidates the current route's data by default once it succeeds —
 * the same revalidation a navigation or a `revalidate()` call triggers, so
 * whatever the page renders picks up the mutation's effect with no extra
 * wiring. `query`/`queryOrThrow` calls do NOT revalidate by default (a
 * query has no side effect to reflect). Both directions are overridable
 * per call via `{ ...args, revalidate: false | true }`.
 *
 * Do NOT call `client.revalidate()` manually after a mutation — that is
 * the anti-pattern this default exists to eliminate. A hand-rolled
 * `mutateOrThrow(args).then(() => client.revalidate())` double-schedules a
 * revalidation the framework was already going to run, and if the intent
 * was actually to suppress it, the correct spelling is `revalidate: false`,
 * not skipping the call.
 *
 * - `query`/`mutate` never throw; check `result.success` to discriminate
 *   ({@link QueryResult}/{@link MutationResult}).
 * - `queryOrThrow`/`mutateOrThrow` return the typed data directly and throw
 *   {@link QueryError}/{@link MutationError} on failure — the shape a
 *   `@tanstack/react-query` `queryFn`/`mutationFn` wants.
 * - `toIdentityArray(args)` derives a stable, serializable cache key from a
 *   call's full args (method, pattern, params, splat values, and input all
 *   feed the key) — built specifically to be `queryKey`-shaped for
 *   react-query and similar cache libraries: `useQuery({ queryKey:
 *   apiClient.toIdentityArray(args), queryFn: () =>
 *   apiClient.queryOrThrow(args) })`. Two calls with the same args produce
 *   the same array (deep-equal, not reference-equal); anything that varies
 *   the response belongs in `args`.
 */
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
