import { jsonStringifyStable } from "vorma/kit/json";
import { API_IDENTITY_ARRAY_PREFIX } from "./constants.ts";
import type {
	ActionKind,
	AppConfig,
	SubmitResult,
	ToActionSubmitArgs,
	ToActionSubmitOutput,
	ToAPIClient,
	ToAPIDecorator,
	ToAPIDecoratorContext,
} from "./types.ts";
import { build_action_url, resolve_body } from "./url.ts";

type SubmitFn = <T>(
	url: string | URL,
	requestInit?: RequestInit,
	options?: {
		actionKind?: ActionKind;
		dedupeKey?: string;
		revalidate?: boolean;
		skipProgressIndicator?: boolean;
	},
) => Promise<SubmitResult<T>>;

function normalize_action_method(raw_method: string | undefined): string {
	return (raw_method ?? "GET").trim().toUpperCase();
}

function normalize_action_pattern(pattern: string): string {
	return pattern.trim();
}

function resolve_action_kind(
	method: string,
	kind: ActionKind | undefined,
): ActionKind {
	if (kind !== undefined) {
		return kind;
	}
	if (method === "GET" || method === "HEAD") {
		return "query";
	}
	return "mutation";
}

function stringify_identity_value(value: unknown): string {
	const res = jsonStringifyStable(value);
	if (!res.ok) {
		throw new Error(res.err);
	}
	return res.val;
}

export class SubmitError<T = never> extends Error {
	result: Extract<SubmitResult<T>, { success: false }>;

	constructor(result: Extract<SubmitResult<T>, { success: false }>) {
		super(result.error);
		this.name = "SubmitError";
		this.result = result;
		Object.setPrototypeOf(this, new.target.prototype);
	}
}

export function create_typed_api_client<A extends AppConfig>(
	actions_mount_root: string,
	submit_fn: SubmitFn,
	decorator?: ToAPIDecorator<A>,
): ToAPIClient<A> {
	async function submit<Args extends ToActionSubmitArgs<A>>(
		args: Args,
	): Promise<SubmitResult<ToActionSubmitOutput<A, Args>>> {
		const {
			dedupeKey,
			input,
			kind,
			method: raw_method,
			params,
			pattern,
			revalidate,
			skipProgressIndicator,
			splatValues,
			...request_init
		} = args as any;
		const method = normalize_action_method(raw_method);
		const action_pattern = normalize_action_pattern(pattern);
		const action_kind = resolve_action_kind(method, kind);
		const is_get = method === "GET" || method === "HEAD";
		const url = build_action_url(
			actions_mount_root,
			action_pattern,
			params,
			splatValues,
			is_get ? input : undefined,
		);
		const ctx = {
			input,
			method,
			pattern: action_pattern,
			requestInit: request_init,
		} as ToAPIDecoratorContext<A>;
		const decorated = decorator
			? ((await (decorator as any)(ctx)) ?? {})
			: {};
		const init: RequestInit = { ...decorated, ...request_init };
		const headers = new Headers(decorated.headers ?? undefined);
		new Headers(request_init.headers ?? undefined).forEach((v, k) => {
			headers.set(k, v);
		});
		init.headers = headers;
		init.method = method;
		if (is_get) {
			delete init.body;
		} else {
			init.body = resolve_body(input);
		}
		const options: {
			actionKind?: ActionKind;
			dedupeKey?: string;
			revalidate?: boolean;
			skipProgressIndicator?: boolean;
		} = {
			actionKind: action_kind,
		};
		if (dedupeKey !== undefined) {
			options.dedupeKey = dedupeKey;
		}
		if (revalidate !== undefined) {
			options.revalidate = revalidate;
		}
		if (skipProgressIndicator !== undefined) {
			options.skipProgressIndicator = skipProgressIndicator;
		}
		return submit_fn<ToActionSubmitOutput<A, Args>>(url, init, options);
	}

	return {
		toIdentityArray: <Args extends ToActionSubmitArgs<A>>(
			args: Args,
		): unknown[] => {
			const {
				input,
				method: raw_method,
				params,
				pattern,
				splatValues,
			} = args as any;
			const method = normalize_action_method(raw_method);
			const action_pattern = normalize_action_pattern(pattern);
			return [
				API_IDENTITY_ARRAY_PREFIX,
				actions_mount_root,
				method,
				action_pattern,
				stringify_identity_value(params ?? null),
				stringify_identity_value(splatValues ?? []),
				stringify_identity_value(input ?? null),
			];
		},
		submit,
		submitOrThrow: async <Args extends ToActionSubmitArgs<A>>(
			args: Args,
		): Promise<ToActionSubmitOutput<A, Args>> => {
			const result = await submit(args);
			if (!result.success) {
				throw new SubmitError(result);
			}
			return result.data;
		},
	};
}
