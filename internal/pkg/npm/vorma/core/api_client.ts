import { Data, Effect } from "effect";
import { jsonStringifyStable } from "vorma/kit/json";
import { API_IDENTITY_ARRAY_PREFIX } from "./constants.ts";
import type {
	APIRouteKind,
	AppConfig,
	MutationResult,
	QueryResult,
	ToAPIClient,
	ToAPIDecorator,
	ToAPIDecoratorContext,
	ToMutationArgs,
	ToQueryArgs,
	__APIClientOutput,
} from "./types.ts";
import { build_action_url, resolve_body } from "./url.ts";

type SubmitFn = <T>(
	url: string | URL,
	requestInit?: RequestInit,
	options?: {
		apiRouteKind?: APIRouteKind;
		dedupeKey?: string;
		revalidate?: boolean;
		skipWorkIndicator?: boolean;
	},
) => Promise<QueryResult<T> | MutationResult<T>>;

type APIClientSubmitOptions = {
	apiRouteKind?: APIRouteKind;
	dedupeKey?: string;
	revalidate?: boolean;
	skipWorkIndicator?: boolean;
};

type PreparedAPISubmit = {
	url: URL;
	init: RequestInit;
	options: APIClientSubmitOptions;
};

class APIIdentityStringifyFailed extends Data.TaggedError(
	"APIIdentityStringifyFailed",
)<{
	readonly reason: string;
}> {}

class APIRequestBuildFailed extends Data.TaggedError("APIRequestBuildFailed")<{
	readonly error: unknown;
	readonly reason: string;
}> {}

class APIDecoratorFailed extends Data.TaggedError("APIDecoratorFailed")<{
	readonly error: unknown;
	readonly reason: string;
}> {}

class APISubmitFailed extends Data.TaggedError("APISubmitFailed")<{
	readonly error: unknown;
	readonly reason: string;
}> {}

function normalize_api_method(raw_method: string | undefined): string {
	return (raw_method ?? "GET").trim().toUpperCase();
}

function normalize_api_pattern(pattern: string): string {
	return pattern.trim();
}

function api_error_message(error: unknown): string {
	if (error instanceof Error) {
		return error.message;
	}
	return String(error);
}

function stringify_identity_value(
	value: unknown,
): Effect.Effect<string, APIIdentityStringifyFailed> {
	const res = jsonStringifyStable(value);
	if (!res.ok) {
		return Effect.fail(
			new APIIdentityStringifyFailed({
				reason: res.err,
			}),
		);
	}
	return Effect.succeed(res.val);
}

class APIErrorBase<T = never> extends Error {
	result: Extract<QueryResult<T> | MutationResult<T>, { success: false }>;

	constructor(
		result: Extract<QueryResult<T> | MutationResult<T>, { success: false }>,
	) {
		super(result.error);
		this.result = result;
		Object.setPrototypeOf(this, new.target.prototype);
	}
}

export class QueryError<T = never> extends APIErrorBase<T> {
	constructor(result: Extract<QueryResult<T>, { success: false }>) {
		super(result);
		this.name = "QueryError";
	}
}

export class MutationError<T = never> extends APIErrorBase<T> {
	constructor(result: Extract<MutationResult<T>, { success: false }>) {
		super(result);
		this.name = "MutationError";
	}
}

function prepare_api_submit<A extends AppConfig>(
	actions_mount_root: string,
	decorator: ToAPIDecorator<A> | undefined,
	args: ToQueryArgs<A> | ToMutationArgs<A>,
	api_route_kind: APIRouteKind,
): Effect.Effect<
	PreparedAPISubmit,
	APIRequestBuildFailed | APIDecoratorFailed
> {
	return Effect.gen(function* () {
		const {
			dedupeKey,
			input,
			method: raw_method,
			params,
			pattern,
			revalidate,
			skipWorkIndicator,
			splatValues,
			...request_init
		} = args as any;
		const method = normalize_api_method(raw_method);
		const api_pattern = normalize_api_pattern(pattern);
		const is_get = method === "GET" || method === "HEAD";
		const url = yield* Effect.try({
			try: () => {
				return build_action_url(
					actions_mount_root,
					api_pattern,
					params,
					splatValues,
					is_get ? input : undefined,
				);
			},
			catch: (error) => {
				return new APIRequestBuildFailed({
					error,
					reason: api_error_message(error),
				});
			},
		});
		const ctx = {
			input,
			method,
			pattern: api_pattern,
			requestInit: request_init,
		} as ToAPIDecoratorContext<A>;
		let decorated: Omit<RequestInit, "method" | "body"> = {};
		if (decorator) {
			decorated = yield* Effect.tryPromise({
				try: async () => {
					return ((await (decorator as any)(ctx)) ?? {}) as Omit<
						RequestInit,
						"method" | "body"
					>;
				},
				catch: (error) => {
					return new APIDecoratorFailed({
						error,
						reason: api_error_message(error),
					});
				},
			});
		}

		const init = yield* Effect.try({
			try: () => {
				const init: RequestInit = { ...decorated, ...request_init };
				const headers = new Headers(decorated.headers ?? undefined);
				new Headers(request_init.headers ?? undefined).forEach(
					(v, k) => {
						headers.set(k, v);
					},
				);
				init.headers = headers;
				init.method = method;
				if (is_get) {
					delete init.body;
				} else {
					init.body = resolve_body(input);
				}
				return init;
			},
			catch: (error) => {
				return new APIRequestBuildFailed({
					error,
					reason: api_error_message(error),
				});
			},
		});
		const options: APIClientSubmitOptions = {
			apiRouteKind: api_route_kind,
		};
		if (dedupeKey !== undefined) {
			options.dedupeKey = dedupeKey;
		}
		if (revalidate !== undefined) {
			options.revalidate = revalidate;
		}
		if (skipWorkIndicator !== undefined) {
			options.skipWorkIndicator = skipWorkIndicator;
		}
		return { url, init, options };
	});
}

export function create_typed_api_client<A extends AppConfig>(
	actions_mount_root: string,
	submit_fn: SubmitFn,
	decorator?: ToAPIDecorator<A>,
): ToAPIClient<A> {
	function submit<Args extends ToQueryArgs<A> | ToMutationArgs<A>>(
		args: Args,
		api_route_kind: APIRouteKind,
	): Promise<
		| QueryResult<__APIClientOutput<A, Args>>
		| MutationResult<__APIClientOutput<A, Args>>
	> {
		return Effect.runPromise(
			Effect.gen(function* () {
				const prepared = yield* prepare_api_submit(
					actions_mount_root,
					decorator,
					args,
					api_route_kind,
				);
				return yield* Effect.tryPromise({
					try: () => {
						return submit_fn<__APIClientOutput<A, Args>>(
							prepared.url,
							prepared.init,
							prepared.options,
						);
					},
					catch: (error) => {
						return new APISubmitFailed({
							error,
							reason: api_error_message(error),
						});
					},
				});
			}).pipe(
				Effect.mapError((error) => {
					return error.error;
				}),
			),
		);
	}

	return {
		toIdentityArray: <Args extends ToQueryArgs<A> | ToMutationArgs<A>>(
			args: Args,
		): unknown[] => {
			const {
				input,
				method: raw_method,
				params,
				pattern,
				splatValues,
			} = args as any;
			const method = normalize_api_method(raw_method);
			const api_pattern = normalize_api_pattern(pattern);
			return Effect.runSync(
				Effect.gen(function* () {
					return [
						API_IDENTITY_ARRAY_PREFIX,
						actions_mount_root,
						method,
						api_pattern,
						yield* stringify_identity_value(params ?? null),
						yield* stringify_identity_value(splatValues ?? []),
						yield* stringify_identity_value(input ?? null),
					];
				}).pipe(
					Effect.mapError((error) => {
						return new Error(error.reason);
					}),
				),
			);
		},
		mutate: <Args extends ToMutationArgs<A>>(
			args: Args,
		): Promise<MutationResult<__APIClientOutput<A, Args>>> => {
			return submit(args, "mutation") as Promise<
				MutationResult<__APIClientOutput<A, Args>>
			>;
		},
		mutateOrThrow: <Args extends ToMutationArgs<A>>(
			args: Args,
		): Promise<__APIClientOutput<A, Args>> => {
			return Effect.runPromise(
				Effect.gen(function* () {
					const result = yield* Effect.tryPromise({
						try: () => {
							return submit(args, "mutation") as Promise<
								MutationResult<__APIClientOutput<A, Args>>
							>;
						},
						catch: (error) => {
							return error;
						},
					});
					if (!result.success) {
						return yield* Effect.fail(new MutationError(result));
					}
					return result.data;
				}),
			);
		},
		query: <Args extends ToQueryArgs<A>>(
			args: Args,
		): Promise<QueryResult<__APIClientOutput<A, Args>>> => {
			return submit(args, "query") as Promise<
				QueryResult<__APIClientOutput<A, Args>>
			>;
		},
		queryOrThrow: <Args extends ToQueryArgs<A>>(
			args: Args,
		): Promise<__APIClientOutput<A, Args>> => {
			return Effect.runPromise(
				Effect.gen(function* () {
					const result = yield* Effect.tryPromise({
						try: () => {
							return submit(args, "query") as Promise<
								QueryResult<__APIClientOutput<A, Args>>
							>;
						},
						catch: (error) => {
							return error;
						},
					});
					if (!result.success) {
						return yield* Effect.fail(new QueryError(result));
					}
					return result.data;
				}),
			);
		},
	};
}
