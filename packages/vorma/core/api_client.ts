import { jsonStringifyStable } from "vorma/kit/json";
import { API_IDENTITY_ARRAY_PREFIX } from "./constants.ts";
import type {
	ApiClientOutput,
	AppConfig,
	MutationResult,
	QueryResult,
	ResourceKind,
	ToApiClient,
	ToApiDecorator,
	ToApiDecoratorContext,
	ToMutationArgs,
	ToQueryArgs,
} from "./types.ts";
import { build_resource_url, resolve_body } from "./url.ts";

type SubmitFn = <T>(
	url: string | URL,
	request_init?: RequestInit,
	options?: {
		resourceKind?: ResourceKind;
		dedupeKey?: string;
		revalidate?: boolean;
		skipWorkIndicator?: boolean;
	},
) => Promise<QueryResult<T> | MutationResult<T>>;

function normalize_api_method(raw_method: string | undefined): string {
	return (raw_method ?? "GET").trim().toUpperCase();
}

function normalize_api_pattern(pattern: string): string {
	return pattern.trim();
}

function stringify_identity_value(value: unknown): string {
	const res = jsonStringifyStable(value);
	if (!res.ok) {
		throw new Error(res.err);
	}
	return res.val;
}

class ApiErrorBase<T = never> extends Error {
	result: Extract<QueryResult<T> | MutationResult<T>, { success: false }>;

	constructor(result: Extract<QueryResult<T> | MutationResult<T>, { success: false }>) {
		super(result.error);
		this.result = result;
		Object.setPrototypeOf(this, new.target.prototype);
	}
}

export class QueryError<T = never> extends ApiErrorBase<T> {
	constructor(result: Extract<QueryResult<T>, { success: false }>) {
		super(result);
		this.name = "QueryError";
	}
}

export class MutationError<T = never> extends ApiErrorBase<T> {
	constructor(result: Extract<MutationResult<T>, { success: false }>) {
		super(result);
		this.name = "MutationError";
	}
}

export function create_typed_api_client<A extends AppConfig>(
	api_mount_root: string,
	submit_fn: SubmitFn,
	decorator?: ToApiDecorator<A>,
): ToApiClient<A> {
	async function submit<Args extends ToQueryArgs<A> | ToMutationArgs<A>>(
		args: Args,
		resource_kind: ResourceKind,
	): Promise<
		QueryResult<ApiClientOutput<A, Args>> | MutationResult<ApiClientOutput<A, Args>>
	> {
		const {
			dedupeKey,
			input,
			method,
			params,
			pattern,
			revalidate,
			skipWorkIndicator,
			splatValues,
			...request_init
		} = args as any;
		const normalized_method = normalize_api_method(method);
		const api_pattern = normalize_api_pattern(pattern);
		const is_get = normalized_method === "GET" || normalized_method === "HEAD";
		const url = build_resource_url(
			api_mount_root,
			api_pattern,
			params,
			splatValues,
			is_get ? input : undefined,
		);
		const ctx = {
			input,
			method: normalized_method,
			pattern: api_pattern,
			requestInit: request_init,
		} as ToApiDecoratorContext<A>;
		const decorated = decorator ? ((await (decorator as any)(ctx)) ?? {}) : {};
		const init: RequestInit = { ...decorated, ...request_init };
		const headers = new Headers(decorated.headers ?? undefined);
		new Headers(request_init.headers ?? undefined).forEach((v, k) => {
			headers.set(k, v);
		});
		init.headers = headers;
		init.method = normalized_method;
		if (is_get) {
			delete init.body;
		} else {
			init.body = resolve_body(input);
		}
		const options: {
			resourceKind?: ResourceKind;
			dedupeKey?: string;
			revalidate?: boolean;
			skipWorkIndicator?: boolean;
		} = {
			resourceKind: resource_kind,
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
		return submit_fn<ApiClientOutput<A, Args>>(url, init, options);
	}

	return {
		toIdentityArray: <Args extends ToQueryArgs<A> | ToMutationArgs<A>>(
			args: Args,
		): unknown[] => {
			const { input, method, params, pattern, splatValues } = args as any;
			const normalized_method = normalize_api_method(method);
			const api_pattern = normalize_api_pattern(pattern);
			return [
				API_IDENTITY_ARRAY_PREFIX,
				api_mount_root,
				normalized_method,
				api_pattern,
				stringify_identity_value(params ?? null),
				stringify_identity_value(splatValues ?? []),
				stringify_identity_value(input ?? null),
			];
		},
		mutate: <Args extends ToMutationArgs<A>>(
			args: Args,
		): Promise<MutationResult<ApiClientOutput<A, Args>>> => {
			return submit(args, "mutation") as Promise<
				MutationResult<ApiClientOutput<A, Args>>
			>;
		},
		mutateOrThrow: async <Args extends ToMutationArgs<A>>(
			args: Args,
		): Promise<ApiClientOutput<A, Args>> => {
			const result = (await submit(args, "mutation")) as MutationResult<
				ApiClientOutput<A, Args>
			>;
			if (!result.success) {
				throw new MutationError(result);
			}
			return result.data;
		},
		query: <Args extends ToQueryArgs<A>>(
			args: Args,
		): Promise<QueryResult<ApiClientOutput<A, Args>>> => {
			return submit(args, "query") as Promise<
				QueryResult<ApiClientOutput<A, Args>>
			>;
		},
		queryOrThrow: async <Args extends ToQueryArgs<A>>(
			args: Args,
		): Promise<ApiClientOutput<A, Args>> => {
			const result = (await submit(args, "query")) as QueryResult<
				ApiClientOutput<A, Args>
			>;
			if (!result.success) {
				throw new QueryError(result);
			}
			return result.data;
		},
	};
}
